package controlplane

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

func browserP2Headers(session BrowserSessionCreate, key string) http.Header {
	headers := p2MutationHeaders()
	headers.Set("Idempotency-Key", key)
	headers.Set("Cookie", browserSessionCookieName+"="+base64.RawURLEncoding.EncodeToString(session.RawToken))
	headers.Set("X-CSRF-Token", base64.RawURLEncoding.EncodeToString(session.RawCSRF))
	return headers
}

func TestPrepareP2MutationClearsRetainedBuffersOnValidationFailure(t *testing.T) {
	server, _ := testServer(t)
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/recovery/verify", strings.NewReader(`{"code":"ABCD-EFGH-IJKL-MNOP"}`))
	request.Header = p2MutationHeaders()
	var body generatedapi.RecoveryVerifyRequestV1
	want := errors.New("injected validation failure")
	prepared, err := server.prepareP2Mutation(request, AuthOperationRecoveryVerify, 4096, &body, func() error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
	if prepared.CanonicalBody != nil || prepared.RawSession != nil || prepared.RawCSRF != nil || prepared.BootstrapToken != "" {
		t.Fatalf("validation failure retained prepared buffers: canonical=%t session=%t csrf=%t bootstrap=%t", prepared.CanonicalBody != nil, prepared.RawSession != nil, prepared.RawCSRF != nil, prepared.BootstrapToken != "")
	}
}

func TestBootstrapEphemeralIsOneShotAndRequiredForOptionsReplay(t *testing.T) {
	server, store := testServer(t)
	now := time.Date(2030, 3, 1, 0, 0, 0, 123456789, time.UTC)
	server.Now = func() time.Time { return now }
	token, created, err := store.EnsureBootstrapToken(t.Context(), now)
	if err != nil || !created {
		t.Fatalf("bootstrap token created=%v err=%v", created, err)
	}
	_, _, _, descriptor := remoteBootstrapFixture(t, normalizeServerTime(now))
	optionsBody, err := json.Marshal(generatedapi.BootstrapOptionsRequestV1{DisplayName: "Owner", Anchor: descriptor.Anchor})
	if err != nil {
		t.Fatal(err)
	}
	optionsHeaders := p2MutationHeaders()
	optionsHeaders.Set("Authorization", "Bootstrap "+token)
	optionsHeaders.Set("Idempotency-Key", "bootstrap-ephemeral-options")
	first := request(t, server, http.MethodPost, "/v1/bootstrap/options", string(optionsBody), optionsHeaders)
	if first.Code != http.StatusOK {
		t.Fatalf("bootstrap options failed: status=%d body_bytes=%d", first.Code, first.Body.Len())
	}
	var ceremonyID string
	if err := store.db.QueryRow(`SELECT ceremony_id FROM auth_idempotency_operations WHERE operation='bootstrap_options'`).Scan(&ceremonyID); err != nil {
		t.Fatal(err)
	}
	if !server.bootstrap.hasEphemeral(ceremonyID) {
		t.Fatal("successful bootstrap options lost process-local private material")
	}

	verify := generatedapi.BootstrapVerifyRequestV1{
		CeremonyId:    ceremonyID,
		Credential:    structurallyValidRegistrationCredential(),
		SigningProof:  base64.RawURLEncoding.EncodeToString(make([]byte, 64)),
		ExchangeProof: base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
	}
	verifyBody, err := json.Marshal(verify)
	if err != nil {
		t.Fatal(err)
	}
	verifyHeaders := p2MutationHeaders()
	verifyHeaders.Set("Authorization", "Bootstrap "+token)
	verifyHeaders.Set("Idempotency-Key", "bootstrap-ephemeral-verify")
	failed := request(t, server, http.MethodPost, "/v1/bootstrap/verify", string(verifyBody), verifyHeaders)
	if failed.Code == http.StatusOK {
		t.Fatalf("invalid bootstrap verification succeeded: body_bytes=%d", failed.Body.Len())
	}
	if server.bootstrap.hasEphemeral(ceremonyID) {
		t.Fatal("failed bootstrap verification retained one-shot private material")
	}

	replay := request(t, server, http.MethodPost, "/v1/bootstrap/options", string(optionsBody), optionsHeaders)
	if replay.Code != http.StatusConflict || !strings.Contains(replay.Body.String(), `"code":"ceremony_restart_required"`) {
		t.Fatalf("bootstrap options replay invalid: status=%d body_bytes=%d restart_code=%t", replay.Code, replay.Body.Len(), strings.Contains(replay.Body.String(), `"code":"ceremony_restart_required"`))
	}
}

func TestBootstrapEphemeralExpiryAndClearZeroPrivateMaterial(t *testing.T) {
	server, _ := testServer(t)
	now := time.Date(2030, 3, 1, 0, 0, 0, 0, time.UTC)
	server.bootstrap.Now = func() time.Time { return now }
	private := bytes.Repeat([]byte{0x5a}, 32)
	server.bootstrap.rememberEphemeral("expired", private, now.Add(time.Minute))
	retained := server.bootstrap.ephemeral["expired"].private
	now = now.Add(time.Minute)
	if server.bootstrap.hasEphemeral("expired") {
		t.Fatal("expired bootstrap private material remained available")
	}
	if !bytes.Equal(retained, make([]byte, len(retained))) {
		t.Fatal("expired bootstrap private material was not zeroed")
	}

	server.bootstrap.rememberEphemeral("shutdown", private, now.Add(time.Minute))
	retained = server.bootstrap.ephemeral["shutdown"].private
	server.bootstrap.clearEphemeral()
	if server.bootstrap.hasEphemeral("shutdown") || !bytes.Equal(retained, make([]byte, len(retained))) {
		t.Fatal("bootstrap shutdown cleanup did not zero private material")
	}
}

func waitForEphemeralRemoval(t *testing.T, service *BootstrapService, ceremonyID string, retained []byte) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		service.ephemeralMu.Lock()
		_, exists := service.ephemeral[ceremonyID]
		zeroed := bytes.Equal(retained, make([]byte, len(retained)))
		service.ephemeralMu.Unlock()
		if !exists && zeroed {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("ephemeral %q exists=%v zeroed=%v", ceremonyID, exists, zeroed)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestBootstrapEphemeralTimerExpiresWithoutLazyMapOperation(t *testing.T) {
	service := &BootstrapService{Now: time.Now}
	private := bytes.Repeat([]byte{0x41}, 32)
	service.rememberEphemeral("timed", private, time.Now().Add(20*time.Millisecond))
	service.ephemeralMu.Lock()
	retained := service.ephemeral["timed"].private
	service.ephemeralMu.Unlock()
	// No has/claim/remember sweep follows: the scheduled callback itself must
	// remove and zero the retained private material.
	waitForEphemeralRemoval(t, service, "timed", retained)
}

func TestBootstrapEphemeralReplacementClaimAndShutdownCancelTimers(t *testing.T) {
	service := &BootstrapService{Now: time.Now}
	private := bytes.Repeat([]byte{0x42}, 32)
	service.rememberEphemeral("replace", private, time.Now().Add(20*time.Millisecond))
	service.ephemeralMu.Lock()
	oldRetained := service.ephemeral["replace"].private
	service.ephemeralMu.Unlock()
	service.rememberEphemeral("replace", bytes.Repeat([]byte{0x43}, 32), time.Now().Add(time.Hour))
	if !bytes.Equal(oldRetained, make([]byte, len(oldRetained))) {
		t.Fatal("replacement did not zero prior private material")
	}
	time.Sleep(40 * time.Millisecond)
	service.ephemeralMu.Lock()
	replacement := service.ephemeral["replace"]
	service.ephemeralMu.Unlock()
	if replacement == nil || replacement.timer == nil {
		t.Fatal("stopped prior timer removed or disarmed the replacement")
	}

	service.rememberEphemeral("claim", private, time.Now().Add(20*time.Millisecond))
	service.ephemeralMu.Lock()
	claimRetained := service.ephemeral["claim"].private
	service.ephemeralMu.Unlock()
	claimed, found := service.claimEphemeral("claim")
	if !found || claimed == nil {
		t.Fatal("claim did not return private material")
	}
	defer zeroBytes(claimed)
	time.Sleep(40 * time.Millisecond)
	service.ephemeralMu.Lock()
	claimedEntry := service.ephemeral["claim"]
	service.ephemeralMu.Unlock()
	if claimedEntry == nil || !claimedEntry.claimed || claimedEntry.timer != nil {
		t.Fatal("claim did not cancel its expiry timer while verification owns the material")
	}
	service.forgetEphemeral("claim")
	if !bytes.Equal(claimRetained, make([]byte, len(claimRetained))) {
		t.Fatal("claim finalizer did not zero retained private material")
	}

	replacementRetained := replacement.private
	service.clearEphemeral()
	if !bytes.Equal(replacementRetained, make([]byte, len(replacementRetained))) {
		t.Fatal("shutdown did not zero replacement private material")
	}
}

func TestBootstrapEphemeralDeferredTimerArmsOnlyAfterCommitHook(t *testing.T) {
	service := &BootstrapService{Now: time.Now}
	private := bytes.Repeat([]byte{0x44}, 32)
	service.rememberEphemeralDeferred("commit", private, time.Now().Add(-time.Second))
	service.ephemeralMu.Lock()
	entry := service.ephemeral["commit"]
	service.ephemeralMu.Unlock()
	if entry == nil || entry.armed || entry.timer != nil {
		t.Fatal("transactional ephemeral material armed before outer commit")
	}
	service.armEphemeral("commit")
	waitForEphemeralRemoval(t, service, "commit", entry.private)
}

func assertBrowserActivity(t *testing.T, store *Store, sessionID string, wantActivity int64, wantLastSeen time.Time) {
	t.Helper()
	var activity int64
	var lastSeen string
	if err := store.db.QueryRow(`SELECT activity_revision,last_seen_at FROM browser_sessions WHERE id=?`, sessionID).Scan(&activity, &lastSeen); err != nil {
		t.Fatal(err)
	}
	if activity != wantActivity || lastSeen != formatServerTime(wantLastSeen) {
		t.Fatalf("activity=%d lastSeen=%q wantActivity=%d wantLastSeen=%q", activity, lastSeen, wantActivity, formatServerTime(wantLastSeen))
	}
}

func assertNoAuthIdempotencyWinner(t *testing.T, store *Store) {
	t.Helper()
	var count int
	if err := store.db.QueryRow(`SELECT count(*) FROM auth_idempotency_operations`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("auth idempotency rows=%d err=%v", count, err)
	}
}

func TestBrowserWinnerTouchFailureOrdering(t *testing.T) {
	t.Run("recent-uv-product-failure-persists-touch", func(t *testing.T) {
		server, store := testServer(t)
		_, passkey, session := seedPasskeySession(t, store, 0)
		now := session.LastSeenAt.Add(6 * time.Minute)
		server.Now = func() time.Time { return now }
		headers := browserP2Headers(session, "touch-recent-uv-failure")
		headers.Set("If-Match", `"rev-1"`)
		response := request(t, server, http.MethodDelete, "/v1/auth/passkeys/"+passkey.ID, `{}`, headers)
		if response.Code == http.StatusOK || !strings.Contains(response.Body.String(), "recent user verification") {
			t.Fatalf("unexpected response: status=%d body_bytes=%d", response.Code, response.Body.Len())
		}
		assertBrowserActivity(t, store, session.ID, 2, now)
		assertNoAuthIdempotencyWinner(t, store)
		var active int
		if err := store.db.QueryRow(`SELECT active FROM passkeys WHERE id=?`, passkey.ID).Scan(&active); err != nil || active != 1 {
			t.Fatalf("passkey active=%d err=%v", active, err)
		}
	})

	t.Run("session-conflict-persists-boundary-touch", func(t *testing.T) {
		server, store := testServer(t)
		_, _, session := seedPasskeySession(t, store, 0)
		now := session.LastSeenAt.Add(5 * time.Minute)
		server.Now = func() time.Time { return now }
		headers := browserP2Headers(session, "touch-session-conflict")
		headers.Set("If-Match", `"rev-2"`)
		response := request(t, server, http.MethodDelete, "/v1/auth/sessions/"+session.ID, `{}`, headers)
		if response.Code != http.StatusPreconditionFailed || !strings.Contains(response.Body.String(), `"code":"session_revision_conflict"`) {
			t.Fatalf("unexpected response: status=%d body_bytes=%d", response.Code, response.Body.Len())
		}
		assertBrowserActivity(t, store, session.ID, 2, now)
		assertNoAuthIdempotencyWinner(t, store)
	})

	t.Run("recovery-permission-failure-persists-touch", func(t *testing.T) {
		server, store := testServer(t)
		user, _, passkeySession := seedPasskeySession(t, store, 0)
		recoverySession, err := NewBrowserSessionCreate(user.ID, "recovery", "", server.config.PublicOrigin, passkeySession.AuthenticatedAt)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.CreateBrowserSession(t.Context(), recoverySession, recoverySession.AuthenticatedAt); err != nil {
			t.Fatal(err)
		}
		now := recoverySession.LastSeenAt.Add(5 * time.Minute)
		server.Now = func() time.Time { return now }
		response := request(t, server, http.MethodPost, "/v1/auth/uv/options", `{}`, browserP2Headers(recoverySession, "touch-recovery-permission"))
		if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"permission_denied"`) {
			t.Fatalf("unexpected response: status=%d body_bytes=%d", response.Code, response.Body.Len())
		}
		assertBrowserActivity(t, store, recoverySession.ID, 2, now)
		assertNoAuthIdempotencyWinner(t, store)
	})

	t.Run("wrong-32-byte-csrf-revokes-without-touch", func(t *testing.T) {
		server, store := testServer(t)
		_, _, session := seedPasskeySession(t, store, 0)
		now := session.LastSeenAt.Add(5 * time.Minute)
		server.Now = func() time.Time { return now }
		headers := browserP2Headers(session, "touch-wrong-csrf")
		headers.Set("X-CSRF-Token", base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x7f}, 32)))
		response := request(t, server, http.MethodPost, "/v1/auth/uv/options", `{}`, headers)
		if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), `"code":"session_integrity_invalid"`) {
			t.Fatalf("unexpected response: status=%d body_bytes=%d", response.Code, response.Body.Len())
		}
		assertBrowserActivity(t, store, session.ID, 1, session.LastSeenAt)
		assertNoAuthIdempotencyWinner(t, store)
		var revoked *string
		var revision int64
		if err := store.db.QueryRow(`SELECT revoked_at,revision FROM browser_sessions WHERE id=?`, session.ID).Scan(&revoked, &revision); err != nil || revoked == nil || revision != 2 {
			t.Fatalf("revoked=%v revision=%d err=%v", revoked, revision, err)
		}
	})

	for _, malformed := range []struct {
		name  string
		value *string
	}{
		{"missing-csrf", nil},
		{"malformed-csrf", func() *string { value := "not-base64url"; return &value }()},
		{"wrong-length-csrf", func() *string { value := base64.RawURLEncoding.EncodeToString(make([]byte, 31)); return &value }()},
	} {
		t.Run(malformed.name+"-does-not-revoke-or-touch", func(t *testing.T) {
			server, store := testServer(t)
			_, _, session := seedPasskeySession(t, store, 0)
			now := session.LastSeenAt.Add(5 * time.Minute)
			server.Now = func() time.Time { return now }
			headers := browserP2Headers(session, "touch-"+malformed.name)
			if malformed.value == nil {
				headers.Del("X-CSRF-Token")
			} else {
				headers.Set("X-CSRF-Token", *malformed.value)
			}
			response := request(t, server, http.MethodPost, "/v1/auth/uv/options", `{}`, headers)
			if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"csrf_invalid"`) {
				t.Fatalf("unexpected response: status=%d body_bytes=%d", response.Code, response.Body.Len())
			}
			assertBrowserActivity(t, store, session.ID, 1, session.LastSeenAt)
			assertNoAuthIdempotencyWinner(t, store)
			var revoked *string
			var revision int64
			if err := store.db.QueryRow(`SELECT revoked_at,revision FROM browser_sessions WHERE id=?`, session.ID).Scan(&revoked, &revision); err != nil || revoked != nil || revision != 1 {
				t.Fatalf("revoked=%v revision=%d err=%v", revoked, revision, err)
			}
		})
	}

	t.Run("committed-key-mismatch-does-not-touch", func(t *testing.T) {
		server, store := testServer(t)
		_, _, session := seedPasskeySession(t, store, 0)
		now := session.LastSeenAt.Add(time.Minute)
		server.Now = func() time.Time { return now }
		headers := browserP2Headers(session, "touch-committed-key-mismatch")
		first := request(t, server, http.MethodPost, "/v1/auth/uv/options", `{}`, headers)
		if first.Code != http.StatusOK {
			t.Fatalf("first UV options failed: status=%d body_bytes=%d", first.Code, first.Body.Len())
		}
		now = session.LastSeenAt.Add(5 * time.Minute)
		mismatch := request(t, server, http.MethodPost, "/v1/auth/logout", `{}`, headers)
		if mismatch.Code != http.StatusConflict || !strings.Contains(mismatch.Body.String(), `"code":"idempotency_key_reused"`) {
			t.Fatalf("idempotency mismatch response invalid: status=%d body_bytes=%d reuse_code=%t", mismatch.Code, mismatch.Body.Len(), strings.Contains(mismatch.Body.String(), `"code":"idempotency_key_reused"`))
		}
		assertBrowserActivity(t, store, session.ID, 1, session.LastSeenAt)
	})
}

func TestBrowserBeginReplayRequiresLiveOriginalSessionAndCSRFGeneration(t *testing.T) {
	for _, transition := range []string{"revoked", "csrf-rotated"} {
		t.Run(transition, func(t *testing.T) {
			server, store := testServer(t)
			_, _, created := seedPasskeySession(t, store, 0)
			now := created.AuthenticatedAt.Add(time.Second)
			server.Now = func() time.Time { return now }
			headers := browserP2Headers(created, "browser-begin-replay-"+transition)
			first := request(t, server, http.MethodPost, "/v1/auth/passkeys/registration/options", `{}`, headers)
			if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"ceremonyId"`) {
				t.Fatalf("first bootstrap-options response invalid: status=%d body_bytes=%d ceremony_present=%t", first.Code, first.Body.Len(), strings.Contains(first.Body.String(), `"ceremonyId"`))
			}

			if transition == "revoked" {
				if _, err := store.RevokeBrowserSession(t.Context(), created.ID, 1, now.Add(time.Second)); err != nil {
					t.Fatal(err)
				}
			} else {
				stored, err := store.BrowserSessionByToken(t.Context(), created.RawToken, now)
				if err != nil {
					t.Fatal(err)
				}
				rotated, _, _, err := store.RotateSessionCSRF(t.Context(), stored, created.RawToken, server.config.PublicOrigin, now.Add(time.Second))
				if err != nil {
					t.Fatal(err)
				}
				zeroBytes(rotated)
			}
			now = now.Add(2 * time.Second)
			replay := request(t, server, http.MethodPost, "/v1/auth/passkeys/registration/options", `{}`, headers)
			if transition == "revoked" {
				if replay.Code != http.StatusConflict || replay.Header().Get("Retry-After") != "" || replay.Header().Get("Set-Cookie") != "" || !strings.Contains(replay.Body.String(), `"code":"ceremony_restart_required"`) {
					t.Fatalf("revoked replay invalid: status=%d body_bytes=%d restart_code=%t", replay.Code, replay.Body.Len(), strings.Contains(replay.Body.String(), `"code":"ceremony_restart_required"`))
				}
			} else {
				if replay.Code != http.StatusUnauthorized || !strings.Contains(replay.Body.String(), `"code":"session_integrity_invalid"`) {
					t.Fatalf("rotated-CSRF replay invalid: status=%d body_bytes=%d integrity_code=%t", replay.Code, replay.Body.Len(), strings.Contains(replay.Body.String(), `"code":"session_integrity_invalid"`))
				}
				var revokedAt *string
				var revision int64
				if err := store.db.QueryRow(`SELECT revoked_at,revision FROM browser_sessions WHERE id=?`, created.ID).Scan(&revokedAt, &revision); err != nil || revokedAt == nil || revision != 3 {
					t.Fatalf("rotated CSRF mismatch revoked=%v revision=%d err=%v", revokedAt, revision, err)
				}
			}
		})
	}
}

func TestBrowserReplayCSRFIntegrityOrWrongSubmittedRawRevokes(t *testing.T) {
	for _, corruptStored := range []bool{false, true} {
		name := "wrong-submitted"
		if corruptStored {
			name = "stored-integrity"
		}
		t.Run(name, func(t *testing.T) {
			server, store := testServer(t)
			_, _, created := seedPasskeySession(t, store, 0)
			now := created.AuthenticatedAt.Add(time.Second)
			server.Now = func() time.Time { return now }
			headers := browserP2Headers(created, "browser-replay-integrity-"+name)
			first := request(t, server, http.MethodPost, "/v1/auth/passkeys/registration/options", `{}`, headers)
			if first.Code != http.StatusOK {
				t.Fatalf("first registration options failed: status=%d body_bytes=%d", first.Code, first.Body.Len())
			}
			if corruptStored {
				if _, err := store.db.Exec(`UPDATE browser_sessions SET csrf_digest=? WHERE id=?`, bytes.Repeat([]byte{0x7f}, 32), created.ID); err != nil {
					t.Fatal(err)
				}
			} else {
				headers.Set("X-CSRF-Token", base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x7f}, 32)))
			}
			replay := request(t, server, http.MethodPost, "/v1/auth/passkeys/registration/options", `{}`, headers)
			if replay.Code != http.StatusUnauthorized || !strings.Contains(replay.Body.String(), `"code":"session_integrity_invalid"`) {
				t.Fatalf("CSRF-integrity replay invalid: status=%d body_bytes=%d integrity_code=%t", replay.Code, replay.Body.Len(), strings.Contains(replay.Body.String(), `"code":"session_integrity_invalid"`))
			}
			var revokedAt *string
			var revision int64
			if err := store.db.QueryRow(`SELECT revoked_at,revision FROM browser_sessions WHERE id=?`, created.ID).Scan(&revokedAt, &revision); err != nil {
				t.Fatal(err)
			}
			if revokedAt == nil || revision != 2 {
				t.Fatalf("CSRF mismatch did not revoke: revoked=%v revision=%d", revokedAt, revision)
			}
		})
	}
}

func TestBrowserReplayIntegrityRevocationSurvivesCanceledContext(t *testing.T) {
	server, store := testServer(t)
	_, _, created := seedPasskeySession(t, store, 0)
	digest := sha256.Sum256(created.RawToken)
	prepared := preparedP2Mutation{
		Request:    AuthIdempotencyRequest{ActorIdentity: digest},
		RawSession: append([]byte(nil), created.RawToken...),
		RawCSRF:    bytes.Repeat([]byte{0x7f}, 32),
	}
	defer prepared.zero()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := server.validateBrowserReplayCSRF(ctx, &prepared); !errors.Is(err, ErrSessionIntegrityInvalid) {
		t.Fatalf("replay integrity error=%v", err)
	}
	var revokedAt *string
	var revision int64
	if err := store.db.QueryRow(`SELECT revoked_at,revision FROM browser_sessions WHERE id=?`, created.ID).Scan(&revokedAt, &revision); err != nil || revokedAt == nil || revision != 2 {
		t.Fatalf("canceled replay revoked=%v revision=%d err=%v", revokedAt, revision, err)
	}
}

func TestAuthCurrentIntegrityRevocationSurvivesCanceledContext(t *testing.T) {
	server, store := testServer(t)
	_, _, created := seedPasskeySession(t, store, 0)
	if _, err := store.db.Exec(`UPDATE browser_sessions SET csrf_digest=? WHERE id=?`, bytes.Repeat([]byte{0x7f}, 32), created.ID); err != nil {
		t.Fatal(err)
	}
	headers := p2ReadHeaders()
	headers.Set("Cookie", browserSessionCookieName+"="+base64.RawURLEncoding.EncodeToString(created.RawToken))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/current", nil).WithContext(ctx)
	req.Header = headers
	server.http.Handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), `"code":"session_integrity_invalid"`) {
		t.Fatalf("current-auth integrity response invalid: status=%d body_bytes=%d integrity_code=%t", recorder.Code, recorder.Body.Len(), strings.Contains(recorder.Body.String(), `"code":"session_integrity_invalid"`))
	}
	var revokedAt *string
	var revision int64
	if err := store.db.QueryRow(`SELECT revoked_at,revision FROM browser_sessions WHERE id=?`, created.ID).Scan(&revokedAt, &revision); err != nil || revokedAt == nil || revision != 2 {
		t.Fatalf("canceled current revoked=%v revision=%d err=%v", revokedAt, revision, err)
	}
}

func TestRecoveryRotationReceiptHasNoCookieActionAndReplayNeverSetsCookie(t *testing.T) {
	server, store := testServer(t)
	_, _, created := seedPasskeySession(t, store, 0)
	now := created.AuthenticatedAt.Add(time.Second)
	server.Now = func() time.Time { return now }
	headers := browserP2Headers(created, "recovery-rotation-receipt")
	first := request(t, server, http.MethodPost, "/v1/auth/recovery-codes/rotate", `{}`, headers)
	if first.Code != http.StatusOK || first.Header().Get("Set-Cookie") != "" {
		t.Fatalf("first response invalid: status=%d cookie_present=%t body_bytes=%d", first.Code, first.Header().Get("Set-Cookie") != "", first.Body.Len())
	}
	var cookieAction, receipt string
	if err := store.db.QueryRow(`SELECT cookie_action,public_receipt_json FROM auth_idempotency_operations WHERE operation='recovery_codes_rotate'`).Scan(&cookieAction, &receipt); err != nil {
		t.Fatal(err)
	}
	if cookieAction != "none" || !strings.Contains(receipt, `"recoveryCodesOutcome":"issued_not_replayable"`) {
		t.Fatalf("durable recovery receipt invalid: cookie_action=%q receipt_bytes=%d", cookieAction, len(receipt))
	}
	replay := request(t, server, http.MethodPost, "/v1/auth/recovery-codes/rotate", `{}`, headers)
	if replay.Code != http.StatusConflict || replay.Header().Get("Set-Cookie") != "" || replay.Header().Get("Retry-After") != "" || !strings.Contains(replay.Body.String(), `"code":"one_time_result_unavailable"`) {
		t.Fatalf("replay response invalid: status=%d cookie_present=%t retry_present=%t body_bytes=%d", replay.Code, replay.Header().Get("Set-Cookie") != "", replay.Header().Get("Retry-After") != "", replay.Body.Len())
	}
}

func addSecondPasskey(t *testing.T, store *Store, userID string, now time.Time) string {
	t.Helper()
	passkeyID, err := transport.NewUUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	credential := webauthn.Credential{ID: []byte("credential-two")}
	encoded, err := json.Marshal(credential)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO passkeys(id,user_id,credential_id,credential_json,name,transports_json,sign_count,credential_revision,active,created_at,last_used_at,updated_at) VALUES(?,?,?,?,?,'[]',0,1,1,?,NULL,?)`, passkeyID, userID, credential.ID, encoded, "Second", formatServerTime(now), formatServerTime(now)); err != nil {
		t.Fatal(err)
	}
	return passkeyID
}

func responseDataAndRequestID(t *testing.T, response *httptest.ResponseRecorder) ([]byte, string) {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
		Meta struct {
			RequestID string `json:"requestId"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || len(envelope.Data) == 0 {
		t.Fatalf("decode response envelope: err_present=%t body_bytes=%d", err != nil, response.Body.Len())
	}
	if envelope.Meta.RequestID != response.Header().Get("X-Request-ID") {
		t.Fatalf("response meta requestId=%q header=%q", envelope.Meta.RequestID, response.Header().Get("X-Request-ID"))
	}
	return append([]byte(nil), envelope.Data...), envelope.Meta.RequestID
}

func TestPublicAuthMutationReplayAllowsRevokedActorAndRepeatsDataAndCookieAction(t *testing.T) {
	type fixture struct {
		server       *Server
		store        *Store
		actor        BrowserSessionCreate
		method       string
		path         string
		operation    AuthIdempotencyOperation
		ifMatch      string
		postFirst    func()
		cookieAction string
	}
	cases := []struct {
		name  string
		setup func(t *testing.T) fixture
	}{
		{
			name: "logout",
			setup: func(t *testing.T) fixture {
				server, store := testServer(t)
				_, _, actor := seedPasskeySession(t, store, 0)
				return fixture{server: server, store: store, actor: actor, method: http.MethodPost, path: "/v1/auth/logout", operation: AuthOperationLogout, cookieAction: "clear"}
			},
		},
		{
			name: "session-self",
			setup: func(t *testing.T) fixture {
				server, store := testServer(t)
				_, _, actor := seedPasskeySession(t, store, 0)
				return fixture{server: server, store: store, actor: actor, method: http.MethodDelete, path: "/v1/auth/sessions/" + actor.ID, operation: AuthOperationSessionDelete, ifMatch: `"rev-1"`, cookieAction: "clear"}
			},
		},
		{
			name: "session-other",
			setup: func(t *testing.T) fixture {
				server, store := testServer(t)
				user, passkey, actor := seedPasskeySession(t, store, 0)
				target, err := NewBrowserSessionCreate(user.ID, "passkey", passkey.ID, server.config.PublicOrigin, actor.AuthenticatedAt)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.CreateBrowserSession(t.Context(), target, target.AuthenticatedAt); err != nil {
					t.Fatal(err)
				}
				return fixture{
					server: server, store: store, actor: actor, method: http.MethodDelete,
					path: "/v1/auth/sessions/" + target.ID, operation: AuthOperationSessionDelete,
					ifMatch: `"rev-1"`, cookieAction: "none",
					postFirst: func() {
						if _, err := store.RevokeBrowserSession(t.Context(), actor.ID, 1, actor.AuthenticatedAt.Add(2*time.Second)); err != nil {
							t.Fatal(err)
						}
					},
				}
			},
		},
		{
			name: "passkey-current",
			setup: func(t *testing.T) fixture {
				server, store := testServer(t)
				user, passkey, actor := seedPasskeySession(t, store, 0)
				addSecondPasskey(t, store, user.ID, actor.AuthenticatedAt)
				return fixture{server: server, store: store, actor: actor, method: http.MethodDelete, path: "/v1/auth/passkeys/" + passkey.ID, operation: AuthOperationPasskeyDelete, ifMatch: `"rev-1"`, cookieAction: "clear"}
			},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			value := test.setup(t)
			now := value.actor.AuthenticatedAt.Add(time.Second)
			value.server.Now = func() time.Time { return now }
			headers := browserP2Headers(value.actor, "public-replay-"+test.name)
			if value.ifMatch != "" {
				headers.Set("If-Match", value.ifMatch)
			}
			first := request(t, value.server, value.method, value.path, `{}`, headers)
			if first.Code != http.StatusOK {
				t.Fatalf("first response invalid: status=%d body_bytes=%d", first.Code, first.Body.Len())
			}
			firstData, firstRequestID := responseDataAndRequestID(t, first)
			firstCookie := first.Header().Get("Set-Cookie")
			if value.cookieAction == "clear" {
				if firstCookie == "" {
					t.Fatal("clear-cookie action emitted no Set-Cookie")
				}
			} else if firstCookie != "" {
				t.Fatalf("none cookie action emitted a cookie: bytes=%d", len(firstCookie))
			}
			var storedReceipt, storedCookieAction string
			if err := value.store.db.QueryRow(`SELECT public_receipt_json,cookie_action FROM auth_idempotency_operations WHERE operation=?`, value.operation).Scan(&storedReceipt, &storedCookieAction); err != nil {
				t.Fatal(err)
			}
			if storedCookieAction != value.cookieAction {
				t.Fatalf("stored cookie_action=%q want=%q", storedCookieAction, value.cookieAction)
			}
			var durable struct {
				Receipt json.RawMessage `json:"receipt"`
				Result  json.RawMessage `json:"result"`
			}
			if err := json.Unmarshal([]byte(storedReceipt), &durable); err != nil || !bytes.Equal(durable.Result, firstData) {
				t.Fatalf("durable public result mismatch: durable_bytes=%d first_bytes=%d err_present=%t", len(durable.Result), len(firstData), err != nil)
			}
			wantCookieOutcome := `"cookieOutcome":"none"`
			if value.cookieAction == "clear" {
				wantCookieOutcome = `"cookieOutcome":"cleared"`
			}
			if !bytes.Contains(durable.Receipt, []byte(wantCookieOutcome)) {
				t.Fatalf("durable receipt cookie outcome mismatch: receipt_bytes=%d", len(durable.Receipt))
			}
			if value.postFirst != nil {
				value.postFirst()
			}
			replay := request(t, value.server, value.method, value.path, `{}`, headers)
			if replay.Code != http.StatusOK {
				t.Fatalf("replay response invalid: status=%d body_bytes=%d", replay.Code, replay.Body.Len())
			}
			replayData, replayRequestID := responseDataAndRequestID(t, replay)
			if !bytes.Equal(firstData, replayData) {
				t.Fatalf("public replay data changed: first_bytes=%d replay_bytes=%d", len(firstData), len(replayData))
			}
			if firstRequestID == replayRequestID {
				t.Fatalf("replay reused outer requestId %q", firstRequestID)
			}
			if replay.Header().Get("Set-Cookie") != firstCookie {
				t.Fatalf("replay cookie mismatch: replay_bytes=%d first_bytes=%d", len(replay.Header().Get("Set-Cookie")), len(firstCookie))
			}
			var replayReceipt, replayCookieAction string
			if err := value.store.db.QueryRow(`SELECT public_receipt_json,cookie_action FROM auth_idempotency_operations WHERE operation=?`, value.operation).Scan(&replayReceipt, &replayCookieAction); err != nil {
				t.Fatal(err)
			}
			if replayReceipt != storedReceipt || replayCookieAction != storedCookieAction {
				t.Fatalf("replay changed durable receipt/action: receipt=%v action=%q", replayReceipt != storedReceipt, replayCookieAction)
			}
		})
	}
}

func TestSessionCookieRawContract(t *testing.T) {
	server := &Server{}
	issued := httptest.NewRecorder()
	server.setSessionCookie(issued, make([]byte, 32), time.Date(2030, 3, 1, 12, 0, 0, 0, time.UTC))
	wantIssued := "__Host-mad_session=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; Path=/; Expires=Fri, 01 Mar 2030 12:00:00 GMT; HttpOnly; Secure; SameSite=Strict"
	if got := issued.Header().Get("Set-Cookie"); got != wantIssued || strings.Contains(strings.ToLower(got), "domain=") {
		t.Fatalf("issued Set-Cookie invalid: bytes=%d domain_present=%t", len(got), strings.Contains(strings.ToLower(got), "domain="))
	}
	cleared := httptest.NewRecorder()
	server.clearSessionCookie(cleared)
	wantCleared := "__Host-mad_session=; Path=/; Expires=Thu, 01 Jan 1970 00:00:01 GMT; Max-Age=0; HttpOnly; Secure; SameSite=Strict"
	if got := cleared.Header().Get("Set-Cookie"); got != wantCleared || strings.Contains(strings.ToLower(got), "domain=") {
		t.Fatalf("cleared Set-Cookie invalid: bytes=%d domain_present=%t", len(got), strings.Contains(strings.ToLower(got), "domain="))
	}
}

func TestP2RetryAfterIsOnlyEmittedForLiveInProgress(t *testing.T) {
	server, store := testServer(t)
	user, passkey, actor := seedPasskeySession(t, store, 0)
	now := actor.AuthenticatedAt.Add(time.Second)
	server.Now = func() time.Time { return now }
	headers := browserP2Headers(actor, "retry-after-in-progress")
	preparedRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", strings.NewReader(`{}`))
	preparedRequest.Header = headers.Clone()
	prepared, err := server.prepareP2Mutation(preparedRequest, AuthOperationLogout, 16, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.zero()
	operationID, err := transport.NewUUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO auth_idempotency_operations(key_digest,request_identity_digest,actor_class,actor_identity_digest,method,canonical_path,body_digest,canonical_if_match,operation_id,operation,state,server_boot_epoch,public_receipt_json,cookie_action,ceremony_id,public_options_digest,created_at,committed_at,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,'in_progress',?,NULL,'none',NULL,NULL,?,NULL,?)`,
		prepared.Request.KeyDigest[:], prepared.Request.RequestIdentity[:], prepared.Request.ActorClass, prepared.Request.ActorIdentity[:], prepared.Request.Method, prepared.Request.CanonicalPath, prepared.Request.BodyDigest[:], prepared.Request.CanonicalIfMatch, operationID, prepared.Request.Operation, prepared.Request.ServerBootEpoch, formatServerTime(now), formatServerTime(now.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	inProgress := request(t, server, http.MethodPost, "/v1/auth/logout", `{}`, headers)
	if inProgress.Code != http.StatusConflict || inProgress.Header().Get("Retry-After") != "1" || inProgress.Header().Get("Set-Cookie") != "" || !strings.Contains(inProgress.Body.String(), `"code":"idempotency_in_progress"`) {
		t.Fatalf("in-progress response invalid: status=%d retry_present=%t cookie_present=%t body_bytes=%d", inProgress.Code, inProgress.Header().Get("Retry-After") != "", inProgress.Header().Get("Set-Cookie") != "", inProgress.Body.Len())
	}

	otherActor, err := NewBrowserSessionCreate(user.ID, "passkey", passkey.ID, server.config.PublicOrigin, actor.AuthenticatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateBrowserSession(t.Context(), otherActor, otherActor.AuthenticatedAt); err != nil {
		t.Fatal(err)
	}
	reusedHeaders := browserP2Headers(otherActor, "retry-after-in-progress")
	keyReused := request(t, server, http.MethodPost, "/v1/auth/logout", `{}`, reusedHeaders)
	if keyReused.Code != http.StatusConflict || keyReused.Header().Get("Retry-After") != "" || keyReused.Header().Get("Set-Cookie") != "" || !strings.Contains(keyReused.Body.String(), `"code":"idempotency_key_reused"`) {
		t.Fatalf("key-reused response invalid: status=%d retry_present=%t cookie_present=%t body_bytes=%d", keyReused.Code, keyReused.Header().Get("Retry-After") != "", keyReused.Header().Get("Set-Cookie") != "", keyReused.Body.Len())
	}
}

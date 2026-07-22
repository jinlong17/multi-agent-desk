package controlplane

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

func seedPasskeySession(t *testing.T, store *Store, signCount uint32) (StoredUser, StoredPasskey, BrowserSessionCreate) {
	t.Helper()
	now := time.Now().UTC()
	userID, _ := transport.NewUUIDv7()
	passkeyID, _ := transport.NewUUIDv7()
	handle := []byte("0123456789abcdef0123456789abcdef")
	if _, err := store.db.Exec(`INSERT INTO users(id,singleton,user_handle,display_name,revision,created_at,updated_at) VALUES(?,1,?,'Owner',1,?,?)`, userID, handle, formatServerTime(now), formatServerTime(now)); err != nil {
		t.Fatal(err)
	}
	credential := webauthn.Credential{ID: []byte("credential-one"), Authenticator: webauthn.Authenticator{SignCount: signCount}}
	credentialJSON, _ := json.Marshal(credential)
	if _, err := store.db.Exec(`INSERT INTO passkeys(id,user_id,credential_id,credential_json,name,transports_json,sign_count,credential_revision,active,created_at,last_used_at,updated_at) VALUES(?,?,?,?,?,'[]',?,1,1,?,NULL,?)`, passkeyID, userID, credential.ID, credentialJSON, "Primary", signCount, formatServerTime(now), formatServerTime(now)); err != nil {
		t.Fatal(err)
	}
	session, err := NewBrowserSessionCreate(userID, "passkey", passkeyID, "https://control.example.test", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateBrowserSession(t.Context(), session, now); err != nil {
		t.Fatal(err)
	}
	return StoredUser{ID: userID, Handle: handle, DisplayName: "Owner"}, StoredPasskey{ID: passkeyID, UserID: userID, Credential: credential, CredentialRevision: 1, SignCount: signCount, Active: true}, session
}

func TestBrowserSessionTouchCoalescesActivityWithoutChangingStateRevision(t *testing.T) {
	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	_, _, created := seedPasskeySession(t, store, 0)
	stored, err := store.BrowserSessionByToken(t.Context(), created.RawToken, created.AuthenticatedAt)
	if err != nil {
		t.Fatal(err)
	}
	beforeIdle := stored.IdleExpiresAt
	coalesced, err := store.TouchBrowserSession(t.Context(), stored.ID, stored.ActivityRevision, stored.LastSeenAt.Add(5*time.Minute-time.Nanosecond))
	if err != nil {
		t.Fatal(err)
	}
	if coalesced.Revision != 1 || coalesced.ActivityRevision != 1 || !coalesced.LastSeenAt.Equal(stored.LastSeenAt) || !coalesced.IdleExpiresAt.Equal(beforeIdle) {
		t.Fatalf("coalesced touch changed state: %+v", coalesced)
	}
	boundary := stored.LastSeenAt.Add(5 * time.Minute)
	touched, err := store.TouchBrowserSession(t.Context(), stored.ID, stored.ActivityRevision, boundary)
	if err != nil {
		t.Fatal(err)
	}
	wantIdle := boundary.Add(browserIdleLifetime)
	if wantIdle.After(stored.ExpiresAt) {
		wantIdle = stored.ExpiresAt
	}
	if touched.Revision != 1 || touched.ActivityRevision != 2 || !touched.LastSeenAt.Equal(boundary) || !touched.IdleExpiresAt.Equal(wantIdle) {
		t.Fatalf("boundary touch=%+v wantIdle=%v", touched, wantIdle)
	}
	// A stale activity CAS reloads the winner and does not invalidate an item
	// state precondition.
	reloaded, err := store.TouchBrowserSession(t.Context(), stored.ID, stored.ActivityRevision, boundary)
	if err != nil || reloaded.Revision != 1 || reloaded.ActivityRevision != 2 {
		t.Fatalf("touch loser=%+v err=%v", reloaded, err)
	}
}

func TestBrowserSessionRevokeReportsItemRevisionConflict(t *testing.T) {
	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	_, _, created := seedPasskeySession(t, store, 0)
	now := created.AuthenticatedAt.Add(time.Second)
	if _, err := store.RevokeBrowserSession(t.Context(), created.ID, 1, now); err != nil {
		t.Fatal(err)
	}
	_, err := store.RevokeBrowserSession(t.Context(), created.ID, 1, now.Add(time.Second))
	var conflict *SessionRevisionConflictError
	if !errors.As(err, &conflict) || conflict.SessionID != created.ID || conflict.ExpectedRevision != 1 || conflict.CurrentRevision != 2 {
		t.Fatalf("conflict=%+v err=%v", conflict, err)
	}
}

func TestBrowserSessionCSRFBindsOriginSessionAndGeneration(t *testing.T) {
	raw := bytes.Repeat([]byte{0x5a}, 32)
	sessionID := "018f47a0-7b1c-7cc2-8000-000000000021"
	first := deriveSessionCSRF(raw, "https://control.example.test", sessionID, 1)
	if len(first) != sha256.Size {
		t.Fatalf("CSRF length=%d", len(first))
	}
	for _, changed := range [][]byte{
		deriveSessionCSRF(raw, "https://other.example.test", sessionID, 1),
		deriveSessionCSRF(raw, "https://control.example.test", "018f47a0-7b1c-7cc2-8000-000000000022", 1),
		deriveSessionCSRF(raw, "https://control.example.test", sessionID, 2),
	} {
		if bytes.Equal(first, changed) {
			t.Fatal("CSRF derivation omitted an identity field")
		}
	}
}

func seedCeremonyForCommit(t *testing.T, store *Store, kind ceremonyKind, now time.Time) string {
	t.Helper()
	id, err := transport.NewUUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO webauthn_ceremonies(id,kind,payload_json,expires_at,created_at) VALUES(?,?,?,?,?)`, id, string(kind), []byte("{}"), formatServerTime(now.Add(time.Minute)), formatServerTime(now)); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPasskeyCounterCASAndRegressionRevokesDerivedSessions(t *testing.T) {
	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	_, passkey, firstSession := seedPasskeySession(t, store, 0)
	now := time.Now().UTC().Add(time.Second)
	secondSession, _ := NewBrowserSessionCreate(passkey.UserID, "passkey", passkey.ID, "https://control.example.test", now)
	if _, err := store.CommitPasskeyAssertion(t.Context(), PasskeyAssertionCommit{CeremonyID: seedCeremonyForCommit(t, store, ceremonyPasskeyLogin, now), PasskeyID: passkey.ID, ExpectedCredentialRevision: 1, ObservedSignCount: 5, Credential: passkey.Credential, NewSession: secondSession, At: now}); err != nil {
		t.Fatal(err)
	}
	stored, err := store.PasskeyByCredentialID(t.Context(), passkey.Credential.ID)
	if err != nil || stored.SignCount != 5 || stored.CredentialRevision != 2 {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
	staleSession, _ := NewBrowserSessionCreate(passkey.UserID, "passkey", passkey.ID, "https://control.example.test", now.Add(time.Second))
	if _, err := store.CommitPasskeyAssertion(t.Context(), PasskeyAssertionCommit{CeremonyID: seedCeremonyForCommit(t, store, ceremonyPasskeyLogin, now.Add(time.Second)), PasskeyID: passkey.ID, ExpectedCredentialRevision: 1, ObservedSignCount: 6, Credential: stored.Credential, NewSession: staleSession, At: now.Add(time.Second)}); err != nil {
		t.Fatalf("stale CAS loser did not reload and re-evaluate: %v", err)
	}
	stored, err = store.PasskeyByCredentialID(t.Context(), passkey.Credential.ID)
	if err != nil || stored.SignCount != 6 || stored.CredentialRevision != 3 {
		t.Fatalf("re-evaluated stored=%+v err=%v", stored, err)
	}
	thirdSession, _ := NewBrowserSessionCreate(passkey.UserID, "passkey", passkey.ID, "https://control.example.test", now.Add(2*time.Second))
	regressionCeremony := seedCeremonyForCommit(t, store, ceremonyPasskeyLogin, now.Add(2*time.Second))
	_, err = store.CommitPasskeyAssertion(t.Context(), PasskeyAssertionCommit{CeremonyID: regressionCeremony, PasskeyID: passkey.ID, ExpectedCredentialRevision: 2, ObservedSignCount: 6, Credential: stored.Credential, NewSession: thirdSession, At: now.Add(2 * time.Second)})
	if !errors.Is(err, ErrPasskeyCounterRegressed) {
		t.Fatalf("regression err=%v", err)
	}
	var ceremonyRows int
	if err := store.db.QueryRow(`SELECT count(*) FROM webauthn_ceremonies WHERE id=?`, regressionCeremony).Scan(&ceremonyRows); err != nil || ceremonyRows != 0 {
		t.Fatalf("counter-failure ceremony rows=%d err=%v", ceremonyRows, err)
	}
	for _, raw := range [][]byte{firstSession.RawToken, secondSession.RawToken, staleSession.RawToken} {
		if _, err := store.BrowserSessionByToken(t.Context(), raw, now.Add(3*time.Second)); err == nil {
			t.Fatal("derived session survived counter regression")
		}
	}
}

func TestPasskeyDeletionRequiresAnotherActiveCredential(t *testing.T) {
	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	user, passkey, session := seedPasskeySession(t, store, 0)
	if _, err := store.DeletePasskeyCAS(t.Context(), user.ID, passkey.ID, session.ID, 1, time.Now().UTC()); !errors.Is(err, ErrLastPasskeyRequired) {
		t.Fatalf("last Passkey deletion err=%v", err)
	}
	secondID, _ := transport.NewUUIDv7()
	credential := webauthn.Credential{ID: []byte("credential-two")}
	encoded, _ := json.Marshal(credential)
	now := time.Now().UTC()
	if _, err := store.db.Exec(`INSERT INTO passkeys(id,user_id,credential_id,credential_json,name,transports_json,sign_count,credential_revision,active,created_at,last_used_at,updated_at) VALUES(?,?,?,?,?,'[]',0,1,1,?,NULL,?)`, secondID, user.ID, credential.ID, encoded, "Second", formatServerTime(now), formatServerTime(now)); err != nil {
		t.Fatal(err)
	}
	result, err := store.DeletePasskeyCAS(t.Context(), user.ID, passkey.ID, session.ID, 1, now)
	if err != nil || result.RevokedSessionCount != 1 || !result.CurrentSessionRevoked {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

package controlplane

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

const authIdempotencyTestBoot = "018f47a0-7b1c-7cc2-8000-000000000011"

func authIdempotencyTestRequest(t *testing.T, key string, operation AuthIdempotencyOperation, boot string) AuthIdempotencyRequest {
	t.Helper()
	contract := authOperationContracts[operation]
	normalized, err := transport.NormalizeIdempotencyKeyV1(key)
	if err != nil {
		t.Fatal(err)
	}
	keyDigest, err := transport.AuthIdempotencyKeyDigestV1(normalized)
	if err != nil {
		t.Fatal(err)
	}
	bodyDigest, err := transport.AuthIdempotencyBodyDigestV1([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	actor := sha256.Sum256([]byte("test-actor"))
	path := contract.Path
	if path == "" {
		path = contract.DynamicPath + "018f47a0-7b1c-7cc2-8000-000000000012"
	}
	ifMatch := ""
	if contract.IfMatch {
		ifMatch = `"rev-1"`
	}
	identity, err := transport.AuthIdempotencyRequestIdentityDigestV1("https://control.example.test", string(contract.Actor), actor[:], string(operation), contract.Method, path, bodyDigest, ifMatch)
	if err != nil {
		t.Fatal(err)
	}
	return AuthIdempotencyRequest{KeyDigest: keyDigest, RequestIdentity: identity, ActorClass: contract.Actor, ActorIdentity: actor, Method: contract.Method, CanonicalPath: path, BodyDigest: bodyDigest, CanonicalIfMatch: ifMatch, Operation: operation, ServerBootEpoch: boot}
}

func authIdempotencyTestCommit(operationID string, at time.Time, ceremony bool) AuthIdempotencyCommit {
	commit := AuthIdempotencyCommit{
		OperationID: operationID,
		Receipt: AuthOperationReceiptV1{
			OperationId: operationID, State: generatedapi.Committed, CommittedAt: at,
			CookieOutcome: generatedapi.AuthOperationReceiptV1CookieOutcomeNone, CsrfOutcome: generatedapi.AuthOperationReceiptV1CsrfOutcomeNone,
			RecoveryCodesOutcome: generatedapi.AuthOperationReceiptV1RecoveryCodesOutcomeNone, NextAction: generatedapi.AuthOperationReceiptV1NextActionNone,
		},
		CookieAction: "none", At: at,
	}
	if ceremony {
		commit.CeremonyID = "018f47a0-7b1c-7cc2-8000-000000000013"
		commit.PublicOptionsDigest = bytes.Repeat([]byte{0x42}, sha256.Size)
	}
	return commit
}

func TestAuthIdempotencyAtomicCommitReplayMismatchRestartAndExpiry(t *testing.T) {
	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	now := time.Date(2030, 3, 1, 0, 0, 0, 0, time.UTC)
	request := authIdempotencyTestRequest(t, "0123456789abcdef", AuthOperationPasskeyLoginOptions, authIdempotencyTestBoot)
	var actionCalls atomic.Int64
	action := func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		actionCalls.Add(1)
		if err := tx.BeginProduct(t.Context()); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		conn := tx.Conn
		if _, err := conn.ExecContext(t.Context(), `INSERT INTO pre_user_audit_events(id,action,decision,created_at) VALUES(?,?,?,?)`, "018f47a0-7b1c-7cc2-8000-000000000014", "idempotency.test", "allowed", formatServerTime(now)); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		return authIdempotencyTestCommit(operationID, now, true), nil
	}
	record, replay, err := store.WithAuthIdempotency(t.Context(), request, now, action)
	if err != nil || replay || record.OperationID == "" || record.State != "committed" || actionCalls.Load() != 1 {
		t.Fatalf("first record=%+v replay=%v calls=%d err=%v", record, replay, actionCalls.Load(), err)
	}
	replayed, replay, err := store.WithAuthIdempotency(t.Context(), request, now.Add(time.Minute), func(*AuthIdempotencyTx, string) (AuthIdempotencyCommit, error) {
		t.Fatal("replay executed the product action")
		return AuthIdempotencyCommit{}, nil
	})
	if err != nil || !replay || replayed.OperationID != record.OperationID || !bytes.Equal(replayed.PublicReceiptJSON, record.PublicReceiptJSON) {
		t.Fatalf("replay record=%+v replay=%v err=%v", replayed, replay, err)
	}
	mismatch := request
	mismatch.BodyDigest[0] ^= 0xff
	if _, _, err := store.WithAuthIdempotency(t.Context(), mismatch, now.Add(2*time.Minute), action); !errors.Is(err, ErrAuthIdempotencyKeyReused) {
		t.Fatalf("mismatch err=%v", err)
	}
	restarted := request
	restarted.ServerBootEpoch = "018f47a0-7b1c-7cc2-8000-000000000015"
	if _, replay, err := store.WithAuthIdempotency(t.Context(), restarted, now.Add(3*time.Minute), action); !replay || !errors.Is(err, ErrCeremonyRestartRequired) {
		t.Fatalf("restart replay=%v err=%v", replay, err)
	}
	// Fixed expiry frees only the global key; it does not slide on replay.
	fresh := mismatch
	fresh.RequestIdentity[0] ^= 0x55
	if _, replay, err := store.WithAuthIdempotency(t.Context(), fresh, now.Add(24*time.Hour), func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		if err := tx.BeginProduct(t.Context()); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		return authIdempotencyTestCommit(operationID, now.Add(24*time.Hour), true), nil
	}); err != nil || replay {
		t.Fatalf("expiry replay=%v err=%v", replay, err)
	}
}

func TestAuthIdempotencyActionFailureRollsBackGateAndProduct(t *testing.T) {
	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	now := time.Date(2030, 3, 1, 0, 0, 0, 0, time.UTC)
	request := authIdempotencyTestRequest(t, "fedcba9876543210", AuthOperationLogout, authIdempotencyTestBoot)
	want := errors.New("simulated crash before receipt")
	_, _, err := store.WithAuthIdempotency(t.Context(), request, now, func(tx *AuthIdempotencyTx, _ string) (AuthIdempotencyCommit, error) {
		if err := tx.BeginProduct(t.Context()); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		conn := tx.Conn
		if _, err := conn.ExecContext(t.Context(), `INSERT INTO pre_user_audit_events(id,action,decision,created_at) VALUES(?,?,?,?)`, "018f47a0-7b1c-7cc2-8000-000000000016", "idempotency.rollback", "allowed", formatServerTime(now)); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		return AuthIdempotencyCommit{}, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("action err=%v", err)
	}
	for table, wantCount := range map[string]int{"auth_idempotency_operations": 0, "pre_user_audit_events": 0} {
		var count int
		if err := store.db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != wantCount {
			t.Fatalf("%s count=%d err=%v", table, count, err)
		}
	}
}

func TestAuthIdempotencyFailureFinalizationPreservesPrefixAndConsumesCeremony(t *testing.T) {
	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	now := time.Date(2030, 3, 1, 0, 0, 0, 123000000, time.UTC)
	request := authIdempotencyTestRequest(t, "failure-finalization-key", AuthOperationPasskeyLoginVerify, authIdempotencyTestBoot)
	ceremonyID := seedCeremonyForCommit(t, store, ceremonyPasskeyLogin, now)
	ctx, cancel := context.WithCancel(context.Background())
	want := errors.New("verification failed after ceremony consume")
	_, _, err := store.WithAuthIdempotency(ctx, request, now, func(tx *AuthIdempotencyTx, _ string) (AuthIdempotencyCommit, error) {
		if _, err := tx.Conn.ExecContext(ctx, `INSERT INTO pre_user_audit_events(id,action,decision,created_at) VALUES(?,?,?,?)`, "018f47a0-7b1c-7cc2-8000-000000000021", "auth.prefix", "allowed", formatServerTime(now)); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		if err := tx.BeginProduct(ctx); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		if err := tx.ConsumeCeremonyOnFailure(ceremonyID, ceremonyPasskeyLogin); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		if err := consumeWebAuthnCeremony(ctx, tx.Conn, ceremonyID, ceremonyPasskeyLogin); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		if _, err := tx.Conn.ExecContext(ctx, `INSERT INTO pre_user_audit_events(id,action,decision,created_at) VALUES(?,?,?,?)`, "018f47a0-7b1c-7cc2-8000-000000000022", "auth.product", "allowed", formatServerTime(now)); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		cancel()
		return AuthIdempotencyCommit{}, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("failure err=%v", err)
	}
	for query, wantCount := range map[string]int{
		`SELECT count(*) FROM pre_user_audit_events WHERE action='auth.prefix'`:  1,
		`SELECT count(*) FROM pre_user_audit_events WHERE action='auth.product'`: 0,
		`SELECT count(*) FROM auth_idempotency_operations`:                       0,
		`SELECT count(*) FROM webauthn_ceremonies WHERE id='` + ceremonyID + `'`: 0,
	} {
		var count int
		if err := store.db.QueryRow(query).Scan(&count); err != nil || count != wantCount {
			t.Fatalf("query=%q count=%d want=%d err=%v", query, count, wantCount, err)
		}
	}
}

func TestAuthIdempotencyReceiptFailureRollsBackProductAndConsumesCeremony(t *testing.T) {
	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	now := time.Date(2030, 3, 1, 0, 0, 0, 456000000, time.UTC)
	request := authIdempotencyTestRequest(t, "receipt-finalization-key", AuthOperationLogout, authIdempotencyTestBoot)
	ceremonyID := seedCeremonyForCommit(t, store, ceremonyPasskeyLogin, now)
	_, _, err := store.WithAuthIdempotency(t.Context(), request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		if err := tx.BeginProduct(t.Context()); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		if err := tx.ConsumeCeremonyOnFailure(ceremonyID, ceremonyPasskeyLogin); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		if err := consumeWebAuthnCeremony(t.Context(), tx.Conn, ceremonyID, ceremonyPasskeyLogin); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		if _, err := tx.Conn.ExecContext(t.Context(), `INSERT INTO pre_user_audit_events(id,action,decision,created_at) VALUES(?,?,?,?)`, "018f47a0-7b1c-7cc2-8000-000000000023", "auth.receipt-product", "allowed", formatServerTime(now)); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		commit := authIdempotencyTestCommit(operationID, now, false)
		commit.PublicResult = make(chan int)
		return commit, nil
	})
	if err == nil || !strings.Contains(err.Error(), "receipt") {
		t.Fatalf("receipt err=%v", err)
	}
	for query := range map[string]struct{}{
		`SELECT count(*) FROM pre_user_audit_events WHERE action='auth.receipt-product'`: {},
		`SELECT count(*) FROM auth_idempotency_operations`:                               {},
		`SELECT count(*) FROM webauthn_ceremonies WHERE id='` + ceremonyID + `'`:         {},
	} {
		var count int
		if err := store.db.QueryRow(query).Scan(&count); err != nil || count != 0 {
			t.Fatalf("query=%q count=%d err=%v", query, count, err)
		}
	}
}

func TestAuthIdempotencySecurityFailureFinalizesAfterRequestCancellation(t *testing.T) {
	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	now := time.Date(2030, 3, 1, 0, 0, 0, 789000000, time.UTC)
	request := authIdempotencyTestRequest(t, "security-finalization-key", AuthOperationLogout, authIdempotencyTestBoot)
	ctx, cancel := context.WithCancel(context.Background())
	want := ErrSessionIntegrityInvalid
	_, _, err := store.WithAuthIdempotency(ctx, request, now, func(tx *AuthIdempotencyTx, _ string) (AuthIdempotencyCommit, error) {
		if _, err := tx.Conn.ExecContext(ctx, `INSERT INTO pre_user_audit_events(id,action,decision,created_at) VALUES(?,?,?,?)`, "018f47a0-7b1c-7cc2-8000-000000000024", "auth.security", "denied", formatServerTime(now)); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		cancel()
		return AuthIdempotencyCommit{}, commitAuthSecurityFailure(want)
	})
	if !errors.Is(err, want) {
		t.Fatalf("security err=%v", err)
	}
	for table, wantCount := range map[string]int{"pre_user_audit_events": 1, "auth_idempotency_operations": 0} {
		var count int
		if err := store.db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != wantCount {
			t.Fatalf("%s count=%d err=%v", table, count, err)
		}
	}
}

func TestAuthIdempotencySuccessFinalizesAfterRequestCancellation(t *testing.T) {
	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	now := time.Date(2030, 3, 1, 0, 0, 0, 123456789, time.UTC)
	request := authIdempotencyTestRequest(t, "success-finalization-key", AuthOperationLogout, authIdempotencyTestBoot)
	ctx, cancel := context.WithCancel(context.Background())
	record, replay, err := store.WithAuthIdempotency(ctx, request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		if err := tx.BeginProduct(ctx); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		if _, err := tx.Conn.ExecContext(ctx, `INSERT INTO pre_user_audit_events(id,action,decision,created_at) VALUES(?,?,?,?)`, "018f47a0-7b1c-7cc2-8000-000000000025", "auth.success", "allowed", formatServerTime(now)); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		cancel()
		return authIdempotencyTestCommit(operationID, now, false), nil
	})
	if err != nil || replay || record.State != "committed" {
		t.Fatalf("record=%+v replay=%v err=%v", record, replay, err)
	}
	for query, wantCount := range map[string]int{
		`SELECT count(*) FROM pre_user_audit_events WHERE action='auth.success'`:   1,
		`SELECT count(*) FROM auth_idempotency_operations WHERE state='committed'`: 1,
	} {
		var count int
		if err := store.db.QueryRow(query).Scan(&count); err != nil || count != wantCount {
			t.Fatalf("query=%q count=%d want=%d err=%v", query, count, wantCount, err)
		}
	}
}

func TestAuthIdempotencyHooksFollowTransactionOutcome(t *testing.T) {
	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	now := time.Date(2030, 3, 1, 0, 0, 0, 0, time.UTC)
	var rollbackCalls, commitCalls, finishCalls atomic.Int64
	request := authIdempotencyTestRequest(t, "transaction-hook-failure", AuthOperationLogout, authIdempotencyTestBoot)
	want := errors.New("fail after process-local allocation")
	_, _, err := store.WithAuthIdempotency(t.Context(), request, now, func(tx *AuthIdempotencyTx, _ string) (AuthIdempotencyCommit, error) {
		if err := tx.BeginProduct(t.Context()); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		if err := tx.OnRollback(func() { rollbackCalls.Add(1) }); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		if err := tx.OnCommit(func() { commitCalls.Add(1) }); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		if err := tx.OnFinish(func() { finishCalls.Add(1) }); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		return AuthIdempotencyCommit{}, want
	})
	if !errors.Is(err, want) || rollbackCalls.Load() != 1 || commitCalls.Load() != 0 || finishCalls.Load() != 1 {
		t.Fatalf("err=%v rollback=%d commit=%d finish=%d", err, rollbackCalls.Load(), commitCalls.Load(), finishCalls.Load())
	}

	rollbackCalls.Store(0)
	commitCalls.Store(0)
	finishCalls.Store(0)
	request = authIdempotencyTestRequest(t, "transaction-hook-success", AuthOperationLogout, authIdempotencyTestBoot)
	if _, replay, err := store.WithAuthIdempotency(t.Context(), request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		if err := tx.BeginProduct(t.Context()); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		if err := tx.OnRollback(func() { rollbackCalls.Add(1) }); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		if err := tx.OnCommit(func() {
			var state string
			if err := tx.Conn.QueryRowContext(t.Context(), `SELECT state FROM auth_idempotency_operations WHERE operation_id=?`, operationID).Scan(&state); err == nil && state == "committed" {
				commitCalls.Add(1)
			}
		}); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		if err := tx.OnFinish(func() { finishCalls.Add(1) }); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		return authIdempotencyTestCommit(operationID, now, false), nil
	}); err != nil || replay {
		t.Fatalf("success replay=%v err=%v", replay, err)
	}
	if rollbackCalls.Load() != 0 || commitCalls.Load() != 1 || finishCalls.Load() != 1 {
		t.Fatalf("success rollback=%d commit=%d finish=%d", rollbackCalls.Load(), commitCalls.Load(), finishCalls.Load())
	}
}

func TestAuthIdempotencySecretOperationRejectsPublicResult(t *testing.T) {
	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	now := time.Date(2030, 3, 1, 0, 0, 0, 0, time.UTC)
	request := authIdempotencyTestRequest(t, "secret-public-result", AuthOperationRecoveryVerify, authIdempotencyTestBoot)
	_, _, err := store.WithAuthIdempotency(t.Context(), request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		if err := tx.BeginProduct(t.Context()); err != nil {
			return AuthIdempotencyCommit{}, err
		}
		commit := authIdempotencyTestCommit(operationID, now, false)
		commit.PublicResult = map[string]string{"secret": "must-not-persist"}
		return commit, nil
	})
	if err == nil || !strings.Contains(err.Error(), "public result class") {
		t.Fatalf("secret public result err=%v", err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT count(*) FROM auth_idempotency_operations`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("idempotency rows=%d err=%v", count, err)
	}
}

func TestAuthIdempotencyClosedOperationActorPathAndIfMatchMap(t *testing.T) {
	if len(authIdempotencyOperations) != 13 || len(authOperationContracts) != len(authIdempotencyOperations) {
		t.Fatalf("operation closure=%d contracts=%d", len(authIdempotencyOperations), len(authOperationContracts))
	}
	seen := map[AuthIdempotencyOperation]bool{}
	for _, operation := range authIdempotencyOperations {
		if seen[operation] || !operation.Valid() {
			t.Fatalf("invalid or duplicate operation %q", operation)
		}
		seen[operation] = true
		request := authIdempotencyTestRequest(t, "0123456789abcdef", operation, authIdempotencyTestBoot)
		if err := validateAuthIdempotencyRequest(request); err != nil {
			t.Fatalf("%s: %v", operation, err)
		}
		wrong := request
		wrong.ActorClass = AuthActorPreauthBrowser
		if wrong.ActorClass == request.ActorClass {
			wrong.ActorClass = AuthActorBrowserSession
		}
		if err := validateAuthIdempotencyRequest(wrong); err == nil {
			t.Fatalf("%s accepted wrong actor", operation)
		}
		wrong = request
		if wrong.CanonicalIfMatch == "" {
			wrong.CanonicalIfMatch = `"rev-1"`
		} else {
			wrong.CanonicalIfMatch = ""
		}
		if err := validateAuthIdempotencyRequest(wrong); err == nil {
			t.Fatalf("%s accepted wrong If-Match class", operation)
		}
	}
}

func TestAuthIdempotencyConcurrentLoserIsBounded(t *testing.T) {
	directory := privateTestDirectory(t)
	path := filepath.Join(directory, "server.sqlite")
	first := openTestStore(t, path)
	second, err := OpenStore(t.Context(), StoreOptions{Path: path, BusyTimeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	now := time.Date(2030, 3, 1, 0, 0, 0, 0, time.UTC)
	request := authIdempotencyTestRequest(t, "1111111111111111", AuthOperationLogout, authIdempotencyTestBoot)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, _, err := first.WithAuthIdempotency(context.Background(), request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
			if err := tx.BeginProduct(context.Background()); err != nil {
				return AuthIdempotencyCommit{}, err
			}
			close(entered)
			<-release
			return authIdempotencyTestCommit(operationID, now, false), nil
		})
		done <- err
	}()
	<-entered
	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()
	_, _, loserErr := second.WithAuthIdempotency(ctx, request, now, func(*AuthIdempotencyTx, string) (AuthIdempotencyCommit, error) {
		t.Fatal("concurrent loser executed product action")
		return AuthIdempotencyCommit{}, nil
	})
	if !errors.Is(loserErr, ErrAuthIdempotencyInProgress) {
		t.Fatalf("loser err=%v", loserErr)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

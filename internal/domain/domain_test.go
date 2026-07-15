package domain

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func testID(prefix, suffix string) ID {
	return ID(prefix + "_" + suffix)
}

const fixedHex = "00112233445566778899aabbccddeeff"

func baseSession(t *testing.T) Session {
	t.Helper()
	session, err := NewSession(Session{
		ID:                   testID("session", fixedHex),
		DeviceID:             testID("device", fixedHex),
		Provider:             "fake",
		CredentialInstanceID: testID("credential", fixedHex),
		RuntimeProfileID:     testID("profile", fixedHex),
		WorkspaceID:          testID("workspace", fixedHex),
		Status:               SessionStarting,
		StartedAt:            time.Unix(100, 0).UTC(),
		CapabilitySnapshot:   []Capability{CapabilitySessionResume, CapabilityMetadataRead, CapabilityMetadataRead},
	})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestSessionTransitions(t *testing.T) {
	valid := []struct {
		from SessionStatus
		to   SessionStatus
	}{
		{SessionStarting, SessionRunning},
		{SessionStarting, SessionFailed},
		{SessionStarting, SessionKilled},
		{SessionRunning, SessionStopping},
		{SessionRunning, SessionFailed},
		{SessionRunning, SessionKilled},
		{SessionStopping, SessionExited},
		{SessionStopping, SessionFailed},
		{SessionStopping, SessionKilled},
	}
	for _, tc := range valid {
		t.Run(string(tc.from)+"_to_"+string(tc.to), func(t *testing.T) {
			session := baseSession(t)
			session.Status = tc.from
			failure := ""
			if tc.to == SessionFailed {
				failure = "provider_failed"
			}
			if err := session.Transition(tc.to, time.Unix(200, 0), nil, failure); err != nil {
				t.Fatalf("expected valid transition: %v", err)
			}
			if tc.to.Terminal() && session.EndedAt == nil {
				t.Fatal("terminal transition did not set end time")
			}
		})
	}
}

func TestSessionRejectsIllegalAndTerminalTransitions(t *testing.T) {
	statuses := []SessionStatus{SessionStarting, SessionRunning, SessionStopping, SessionExited, SessionFailed, SessionKilled}
	for _, from := range statuses {
		for _, to := range statuses {
			_, allowed := sessionTransitions[from][to]
			if allowed {
				continue
			}
			t.Run(string(from)+"_to_"+string(to), func(t *testing.T) {
				session := baseSession(t)
				session.Status = from
				before := session
				if err := session.Transition(to, time.Unix(200, 0), nil, ""); CodeOf(err) != CodeInvalidTransition {
					t.Fatalf("got %v, want invalid transition", err)
				}
				if !reflect.DeepEqual(session, before) {
					t.Fatal("rejected transition mutated session")
				}
			})
		}
	}
}

func TestResumeCreatesNewStartingRecord(t *testing.T) {
	source := baseSession(t)
	source.Status = SessionStopping
	if err := source.Transition(SessionExited, time.Unix(200, 0), nil, ""); err != nil {
		t.Fatal(err)
	}
	original := source
	newID := testID("session", "ffeeddccbbaa99887766554433221100")
	resumed, err := source.Resume(newID, time.Unix(300, 0))
	if err != nil {
		t.Fatal(err)
	}
	if resumed.ID != newID || resumed.ResumedFromSessionID != source.ID || resumed.Status != SessionStarting {
		t.Fatalf("unexpected resumed session: %+v", resumed)
	}
	if resumed.EndedAt != nil || resumed.ExitCode != nil {
		t.Fatal("new session retained terminal metadata")
	}
	if !reflect.DeepEqual(source, original) {
		t.Fatal("resume mutated source session")
	}
}

func TestResumeRequiresCapabilityAndMonotonicTime(t *testing.T) {
	source := baseSession(t)
	source.Status = SessionStopping
	if err := source.Transition(SessionExited, time.Unix(200, 0), nil, ""); err != nil {
		t.Fatal(err)
	}
	newID := testID("session", "ffeeddccbbaa99887766554433221100")
	withoutCapability := source
	withoutCapability.CapabilitySnapshot = []Capability{CapabilityMetadataRead}
	if _, err := withoutCapability.Resume(newID, time.Unix(300, 0)); CodeOf(err) != CodePermissionDenied {
		t.Fatalf("got %v, want permission denied", err)
	}
	if _, err := source.Resume(newID, time.Unix(199, 0)); CodeOf(err) != CodeInvalidArgument {
		t.Fatalf("got %v, want invalid argument", err)
	}
}

func TestControllerLeaseLifecycle(t *testing.T) {
	sessionID := testID("session", fixedHex)
	holderA := testID("client", fixedHex)
	holderB := testID("client", "ffeeddccbbaa99887766554433221100")
	now := time.Unix(100, 0).UTC()

	lease, err := AcquireControllerLease(nil, sessionID, holderA, now, DefaultLeaseDuration)
	if err != nil {
		t.Fatal(err)
	}
	if lease.Revision != 1 || !lease.Active(now) {
		t.Fatalf("unexpected lease: %+v", lease)
	}
	if _, err := AcquireControllerLease(&lease, sessionID, holderB, now.Add(time.Second), DefaultLeaseDuration); CodeOf(err) != CodeLeaseHeld {
		t.Fatalf("got %v, want lease held", err)
	}
	if err := lease.RequireControl(holderB, lease.Revision, now.Add(time.Second)); CodeOf(err) != CodePermissionDenied {
		t.Fatalf("observer controlled session: %v", err)
	}
	if err := lease.RequireControl(holderA, lease.Revision+1, now.Add(time.Second)); CodeOf(err) != CodeStaleLease {
		t.Fatalf("got %v, want stale lease", err)
	}

	heartbeat, err := lease.Heartbeat(holderA, lease.Revision, now.Add(10*time.Second), DefaultLeaseDuration)
	if err != nil {
		t.Fatal(err)
	}
	if !heartbeat.ExpiresAt.Equal(now.Add(40*time.Second)) || heartbeat.Revision != lease.Revision {
		t.Fatalf("unexpected heartbeat: %+v", heartbeat)
	}
	if _, err := heartbeat.Heartbeat(holderA, heartbeat.Revision, now.Add(9*time.Second), DefaultLeaseDuration); CodeOf(err) != CodeInvalidArgument {
		t.Fatalf("non-monotonic heartbeat got %v", err)
	}
	released, err := heartbeat.Release(holderA, heartbeat.Revision, now.Add(11*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if released.Active(now.Add(11*time.Second)) || released.Revision != 2 {
		t.Fatalf("unexpected release: %+v", released)
	}
	if _, err := AcquireControllerLease(&released, sessionID, holderB, now.Add(10*time.Second), DefaultLeaseDuration); CodeOf(err) != CodeInvalidArgument {
		t.Fatalf("non-monotonic acquire got %v", err)
	}
	next, err := AcquireControllerLease(&released, sessionID, holderB, now.Add(12*time.Second), DefaultLeaseDuration)
	if err != nil {
		t.Fatal(err)
	}
	if next.Revision != 3 || next.HolderDeviceID != holderB {
		t.Fatalf("unexpected replacement: %+v", next)
	}
}

func TestCanonicalCapabilities(t *testing.T) {
	got, err := CanonicalCapabilities([]Capability{CapabilitySessionResume, CapabilityMetadataRead, CapabilitySessionResume})
	if err != nil {
		t.Fatal(err)
	}
	want := []Capability{CapabilityMetadataRead, CapabilitySessionResume}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if _, err := CanonicalCapabilities([]Capability{"root"}); CodeOf(err) != CodeInvalidArgument {
		t.Fatalf("got %v, want invalid argument", err)
	}
}

func TestValidateIDChecksCompleteBoundedGrammar(t *testing.T) {
	valid := []ID{
		testID("session", fixedHex),
		testID("phase_one", fixedHex),
		testID("_", fixedHex),
	}
	for _, id := range valid {
		if err := ValidateID(id); err != nil {
			t.Fatalf("valid id %q rejected: %v", id, err)
		}
	}
	invalid := []ID{
		"",
		fixedHex,
		ID("_" + fixedHex),
		ID("UPPER_" + fixedHex),
		ID("bad!_" + fixedHex),
		ID(strings.Repeat("a", 25) + "_" + fixedHex),
		"session_0011",
		"session_zz112233445566778899aabbccddeeff",
	}
	for _, id := range invalid {
		if err := ValidateID(id); CodeOf(err) != CodeInvalidArgument {
			t.Fatalf("invalid id %q got %v", id, err)
		}
	}
}

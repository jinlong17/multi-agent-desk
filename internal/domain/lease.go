package domain

import "time"

const (
	DefaultLeaseDuration  = 30 * time.Second
	DefaultLeaseHeartbeat = 10 * time.Second
)

type ControllerLease struct {
	SessionID      ID
	HolderDeviceID ID
	Revision       int64
	ExpiresAt      time.Time
	LastHeartbeat  time.Time
	ReleasedAt     *time.Time
}

func (l ControllerLease) Active(at time.Time) bool {
	return l.ReleasedAt == nil && l.Revision > 0 && at.Before(l.ExpiresAt)
}

// AcquireControllerLease creates a monotonically revisioned lease. An expired
// lease may be replaced; an active lease is never stolen, including by its
// current holder.
func AcquireControllerLease(current *ControllerLease, sessionID, holderID ID, at time.Time, duration time.Duration) (ControllerLease, error) {
	if err := ValidateID(sessionID); err != nil {
		return ControllerLease{}, err
	}
	if err := ValidateID(holderID); err != nil {
		return ControllerLease{}, err
	}
	if at.IsZero() || duration <= 0 {
		return ControllerLease{}, NewError(CodeInvalidArgument, "lease requires a valid time and duration")
	}
	revision := int64(1)
	if current != nil {
		if current.SessionID != sessionID {
			return ControllerLease{}, NewError(CodeConflict, "lease belongs to another session")
		}
		if current.Active(at) {
			return ControllerLease{}, NewError(CodeLeaseHeld, "controller lease is already held")
		}
		revision = current.Revision + 1
	}
	at = at.UTC()
	return ControllerLease{
		SessionID:      sessionID,
		HolderDeviceID: holderID,
		Revision:       revision,
		ExpiresAt:      at.Add(duration),
		LastHeartbeat:  at,
	}, nil
}

func (l ControllerLease) Heartbeat(holderID ID, revision int64, at time.Time, duration time.Duration) (ControllerLease, error) {
	if err := l.require(holderID, revision, at); err != nil {
		return ControllerLease{}, err
	}
	if duration <= 0 {
		return ControllerLease{}, NewError(CodeInvalidArgument, "lease heartbeat requires a duration")
	}
	at = at.UTC()
	l.LastHeartbeat = at
	l.ExpiresAt = at.Add(duration)
	return l, nil
}

func (l ControllerLease) Release(holderID ID, revision int64, at time.Time) (ControllerLease, error) {
	if err := l.require(holderID, revision, at); err != nil {
		return ControllerLease{}, err
	}
	at = at.UTC()
	l.Revision++
	l.ExpiresAt = at
	l.LastHeartbeat = at
	l.ReleasedAt = &at
	return l, nil
}

// RequireControl validates both the holder identity and current revision.
func (l ControllerLease) RequireControl(holderID ID, revision int64, at time.Time) error {
	return l.require(holderID, revision, at)
}

func (l ControllerLease) require(holderID ID, revision int64, at time.Time) error {
	if l.Revision == 0 {
		return NewError(CodeLeaseRequired, "controller lease is required")
	}
	if revision != l.Revision {
		return NewError(CodeStaleLease, "controller lease revision is stale")
	}
	if l.HolderDeviceID != holderID {
		return NewError(CodePermissionDenied, "controller lease belongs to another client")
	}
	if at.IsZero() || at.Before(l.LastHeartbeat) {
		return NewError(CodeInvalidArgument, "lease operation time is invalid")
	}
	if !l.Active(at) {
		return NewError(CodeLeaseRequired, "controller lease is expired or released")
	}
	return nil
}

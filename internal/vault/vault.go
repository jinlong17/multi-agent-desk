package vault

import (
	"sync"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type State string

const (
	StateLocked   State = "locked"
	StateUnlocked State = "unlocked"
)

// Manager is the Phase 1 fake Vault boundary. It intentionally retains only
// unlock state, never a password or secret. Production key wrapping belongs
// to the later security-owned Vault implementation.
type Manager struct {
	mu          sync.RWMutex
	state       State
	unlockEpoch uint64
}

func NewManager() *Manager { return &Manager{state: StateLocked} }

func (m *Manager) Status() State {
	if m == nil {
		return StateLocked
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state == "" {
		return StateLocked
	}
	return m.state
}

func (m *Manager) Unlock(secret []byte) error {
	if m == nil || len(secret) == 0 || len(secret) > 4096 {
		return domain.NewError(domain.CodeInvalidArgument, "vault unlock input is invalid")
	}
	m.mu.Lock()
	m.state = StateUnlocked
	m.unlockEpoch++
	m.mu.Unlock()
	return nil
}

func (m *Manager) Lock() error {
	if m == nil {
		return domain.NewError(domain.CodeInvalidArgument, "vault is unavailable")
	}
	m.mu.Lock()
	m.state = StateLocked
	m.mu.Unlock()
	return nil
}

func (m *Manager) RequireUnlocked() error {
	if m == nil || m.Status() != StateUnlocked {
		return domain.NewError(domain.CodeVaultLocked, "vault is locked")
	}
	return nil
}

func (m *Manager) Epoch() uint64 {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.unlockEpoch
}

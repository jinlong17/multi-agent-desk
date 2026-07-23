package controlplane

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/transport"
	"golang.org/x/crypto/argon2"
)

const (
	recoveryCodeCount       = 10
	recoveryCodeEntropySize = 20
	recoverySaltSize        = 16
	recoveryArgonTime       = 3
	recoveryArgonMemoryKiB  = 64 * 1024
	recoveryArgonThreads    = 1
	recoveryHashSize        = 32
)

var (
	ErrRecoveryInvalidOrRateLimited = errors.New("recovery_invalid_or_rate_limited")
	ErrRecoveryConsumed             = errors.New("recovery_consumed")
	ErrRecoveryBatchReplaced        = errors.New("recovery_batch_replaced")
	ErrOneTimeResultUnavailable     = errors.New("one_time_result_unavailable")
	recoveryHashSlots               = make(chan struct{}, 2)
)

type RecoveryCodeSet struct {
	BatchID     string
	Plaintext   []string
	Hashes      []RecoveryCodeHash
	GeneratedAt time.Time
}

func GenerateRecoveryCodeSet(now time.Time) (RecoveryCodeSet, error) {
	now = normalizeServerTime(now)
	select {
	case recoveryHashSlots <- struct{}{}:
		defer func() { <-recoveryHashSlots }()
	default:
		return RecoveryCodeSet{}, fmt.Errorf("recovery hashing capacity is exhausted")
	}
	batchID, err := transport.NewUUIDv7()
	if err != nil {
		return RecoveryCodeSet{}, err
	}
	result := RecoveryCodeSet{BatchID: batchID, GeneratedAt: now, Plaintext: make([]string, 0, recoveryCodeCount), Hashes: make([]RecoveryCodeHash, 0, recoveryCodeCount)}
	for ordinal := 1; ordinal <= recoveryCodeCount; ordinal++ {
		entropy := make([]byte, recoveryCodeEntropySize)
		if _, err := rand.Read(entropy); err != nil {
			return RecoveryCodeSet{}, fmt.Errorf("generate recovery code: %w", err)
		}
		code := formatRecoveryCode(entropy)
		zeroBytes(entropy)
		salt := make([]byte, recoverySaltSize)
		if _, err := rand.Read(salt); err != nil {
			return RecoveryCodeSet{}, fmt.Errorf("generate recovery salt: %w", err)
		}
		hash := hashRecoveryCode([]byte(code), salt)
		id, err := transport.NewUUIDv7()
		if err != nil {
			return RecoveryCodeSet{}, err
		}
		result.Plaintext = append(result.Plaintext, code)
		result.Hashes = append(result.Hashes, RecoveryCodeHash{ID: id, Ordinal: ordinal, Salt: salt, Hash: hash})
	}
	return result, nil
}

func (s *RecoveryCodeSet) ZeroPlaintext() {
	if s == nil {
		return
	}
	for index := range s.Plaintext {
		s.Plaintext[index] = ""
	}
	s.Plaintext = nil
}

func formatRecoveryCode(entropy []byte) string {
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(entropy)
	var builder strings.Builder
	builder.Grow(len("MAD-RC1-") + len(encoded) + 7)
	builder.WriteString("MAD-RC1-")
	for offset := 0; offset < len(encoded); offset += 4 {
		if offset != 0 {
			builder.WriteByte('-')
		}
		builder.WriteString(encoded[offset : offset+4])
	}
	return builder.String()
}

func ParseRecoveryCode(input string) (string, error) {
	if len(input) != 47 {
		return "", ErrRecoveryInvalidOrRateLimited
	}
	for _, value := range []int{3, 7, 12, 17, 22, 27, 32, 37, 42} {
		if input[value] != '-' {
			return "", ErrRecoveryInvalidOrRateLimited
		}
	}
	upper := strings.ToUpper(input)
	if upper[:8] != "MAD-RC1-" {
		return "", ErrRecoveryInvalidOrRateLimited
	}
	compact := strings.ReplaceAll(upper[8:], "-", "")
	if len(compact) != 32 {
		return "", ErrRecoveryInvalidOrRateLimited
	}
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(compact)
	if err != nil || len(decoded) != recoveryCodeEntropySize || formatRecoveryCode(decoded) != upper {
		zeroBytes(decoded)
		return "", ErrRecoveryInvalidOrRateLimited
	}
	zeroBytes(decoded)
	return upper, nil
}

func hashRecoveryCode(code, salt []byte) []byte {
	return argon2.IDKey(code, salt, recoveryArgonTime, recoveryArgonMemoryKiB, recoveryArgonThreads, recoveryHashSize)
}

type RecoveryCandidate struct {
	ID     string
	Salt   []byte
	Hash   []byte
	Status string
}

func MatchRecoveryCode(ctx context.Context, canonical string, candidates []RecoveryCandidate) (string, error) {
	select {
	case recoveryHashSlots <- struct{}{}:
		defer func() { <-recoveryHashSlots }()
	case <-ctx.Done():
		return "", ErrRecoveryInvalidOrRateLimited
	}
	matched := ""
	consumed := false
	for _, candidate := range candidates {
		if len(candidate.Salt) != recoverySaltSize || len(candidate.Hash) != recoveryHashSize {
			return "", fmt.Errorf("stored recovery code is corrupt")
		}
		calculated := hashRecoveryCode([]byte(canonical), candidate.Salt)
		equal := subtle.ConstantTimeCompare(calculated, candidate.Hash) == 1
		zeroBytes(calculated)
		if equal {
			matched = candidate.ID
			consumed = candidate.Status != "active"
		}
	}
	if matched == "" {
		return "", ErrRecoveryInvalidOrRateLimited
	}
	if consumed {
		return "", ErrRecoveryConsumed
	}
	return matched, nil
}

type attemptBucket struct {
	window time.Time
	count  int
}

// RecoveryLimiter is deliberately process-local. Restart drops only denial
// counters, never recovery-code state; the expensive Argon2 semaphore remains
// the hard global resource bound.
type RecoveryLimiter struct {
	mu      sync.Mutex
	sources map[string]attemptBucket
	global  attemptBucket
	Now     func() time.Time
}

func (l *RecoveryLimiter) Allow(source string) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.sources == nil {
		l.sources = make(map[string]attemptBucket)
	}
	now := time.Now().UTC()
	if l.Now != nil {
		now = l.Now().UTC()
	}
	window := now.Truncate(time.Minute)
	if !l.global.window.Equal(window) {
		l.global = attemptBucket{window: window}
		for key, value := range l.sources {
			if !value.window.Equal(window) {
				delete(l.sources, key)
			}
		}
	}
	entry, exists := l.sources[source]
	if !exists && len(l.sources) >= maxRateLimitSources {
		return false
	}
	if !entry.window.Equal(window) {
		entry = attemptBucket{window: window}
	}
	if l.global.count >= 30 || entry.count >= 5 {
		return false
	}
	l.global.count++
	entry.count++
	l.sources[source] = entry
	return true
}

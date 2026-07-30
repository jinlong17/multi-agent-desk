package controlplane

import (
	"sync"
	"time"
)

const maxRateLimitSources = 4096

// RequestLimiter is a bounded process-local fixed-window limiter used before
// authentication. Durable authority never depends on it; it only limits work
// that an unauthenticated network peer can make the process perform.
type RequestLimiter struct {
	mu        sync.Mutex
	sources   map[string]attemptBucket
	global    attemptBucket
	Now       func() time.Time
	PerSource int
	Global    int
}

func (l *RequestLimiter) Allow(source string) bool {
	if l == nil || l.PerSource < 1 || l.Global < 1 {
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
	if l.global.count >= l.Global || entry.count >= l.PerSource {
		return false
	}
	l.global.count++
	entry.count++
	l.sources[source] = entry
	return true
}

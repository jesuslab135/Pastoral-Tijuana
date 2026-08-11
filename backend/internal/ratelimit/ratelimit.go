// Package ratelimit is a fixed-window in-memory limiter, sufficient for the
// single-instance deployment described in the spec.
package ratelimit

import (
	"sync"
	"time"
)

const sweepThreshold = 4096

type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]entry
}

type entry struct {
	count int
	start time.Time
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, hits: make(map[string]entry)}
}

// Allow reports whether the key may proceed, counting this call against its
// current window.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	e, ok := l.hits[key]
	if !ok || now.Sub(e.start) >= l.window {
		// Sweep expired keys occasionally so an attacker cycling addresses
		// cannot grow the map without bound.
		if len(l.hits) > sweepThreshold {
			for k, v := range l.hits {
				if now.Sub(v.start) >= l.window {
					delete(l.hits, k)
				}
			}
		}
		l.hits[key] = entry{count: 1, start: now}
		return true
	}
	if e.count >= l.limit {
		return false
	}
	e.count++
	l.hits[key] = e
	return true
}

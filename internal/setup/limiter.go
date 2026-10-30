package setup

import (
	"sync"
	"time"
)

// limiter counts the failed attempts of each client in fixed windows, it lives in memory since Redis may not be configured yet.
type limiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	now      func() time.Time
	failures map[string]*failureWindow
}

type failureWindow struct {
	count   int
	resetAt time.Time
}

// pruneThreshold bounds the memory used by the clients that never come back.
const pruneThreshold = 1024

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, now: time.Now, failures: map[string]*failureWindow{}}
}

// Allow reports whether the client has failed fewer than max times in the current window.
func (l *limiter) Allow(client string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	w, ok := l.failures[client]
	if !ok {
		return true
	}
	if !l.now().Before(w.resetAt) {
		delete(l.failures, client)
		return true
	}
	return w.count < l.max
}

// Fail records a failed attempt of the client.
func (l *limiter) Fail(client string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if len(l.failures) >= pruneThreshold {
		for key, w := range l.failures {
			if !now.Before(w.resetAt) {
				delete(l.failures, key)
			}
		}
	}

	w, ok := l.failures[client]
	if !ok || !now.Before(w.resetAt) {
		w = &failureWindow{resetAt: now.Add(l.window)}
		l.failures[client] = w
	}
	w.count++
}

// Package probe performs bounded, rate-limited network probing of LAN
// hosts.
package probe

import "context"

const (
	// DefaultConcurrency is the dial concurrency used when the caller
	// passes a non-positive value to NewLimiter.
	DefaultConcurrency = 128
	// MaxConcurrency is the hard cap on concurrent dials. A bug in the
	// caller must not be able to flood the LAN.
	MaxConcurrency = 256
)

// Limiter bounds the number of functions running concurrently through Do.
// It is a weighted semaphore implemented as a buffered channel; the zero
// value is not usable — construct with NewLimiter.
type Limiter struct {
	sem chan struct{}
}

// NewLimiter returns a Limiter allowing at most n concurrent executions.
// n <= 0 selects the default of 128; n > 256 is clamped down to 256.
func NewLimiter(n int) *Limiter {
	if n <= 0 {
		n = DefaultConcurrency
	}
	if n > MaxConcurrency {
		n = MaxConcurrency
	}
	return &Limiter{sem: make(chan struct{}, n)}
}

// Do runs f once a concurrency slot is available, holding the slot for the
// duration of f. If ctx is cancelled while waiting for a slot, Do returns
// ctx.Err() without running f. A panic in f still releases the slot and
// propagates to the caller of Do.
func (l *Limiter) Do(ctx context.Context, f func()) error {
	select {
	case l.sem <- struct{}{}:
		defer func() { <-l.sem }()
		f()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

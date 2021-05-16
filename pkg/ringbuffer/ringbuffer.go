package ringbuffer

import (
	"sync"
	"time"
)

// Based this on https://github.com/mholt/caddy-ratelimit/tree/master as it uses windows & events
// ringBufferRateLimiter uses a ring to enforce rate limits
// consisting of a maximum number of events within a single
// sliding window of a given duration. An empty value is
// not valid; always call initialize() before using.
type RingBufferRateLimiter struct {
	Mu     sync.Mutex
	Windows time.Duration
	Ring   []time.Time // len(ring) == maxEvents
	Cursor int         // always points to the oldest timestamp
}

// initialize sets up the rate limiter if it isn't already, allowing maxEvents
// in a sliding window of size window. If maxEvents is 0, no events are
// allowed. If window is 0, all events are allowed. It panics if maxEvents or
// window are less than zero. This method is idempotent.
func (r *RingBufferRateLimiter) Initialize(maxEvents int, window time.Duration) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	if r.Windows != 0 || r.Ring != nil {
		return
	}
	if maxEvents < 0 {
		panic("maxEvents cannot be less than zero")
	}
	if window < 0 {
		panic("window cannot be less than zero")
	}
	r.Windows = window
	r.Ring = make([]time.Time, maxEvents)
}

// reserve claims the current spot in the ring buffer
// and advances the cursor.
// It is NOT safe for concurrent use, so it must
// be called inside a lock on r.Mu.
func (r *RingBufferRateLimiter) Reserve(timestamp time.Time) {
	r.Ring[r.Cursor] = timestamp
	r.Advance()
}

// advance moves the cursor to the next position.
// It is NOT safe for concurrent use, so it must
// be called inside a lock on r.Mu.
func (r *RingBufferRateLimiter) Advance() {
	r.Cursor++
	if r.Cursor >= len(r.Ring) {
		r.Cursor = 0
	}
}

// MaxEvents returns the maximum number of events that
// are allowed within the sliding window.
func (r *RingBufferRateLimiter) MaxEvents() int {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	return len(r.Ring)
}

// SetMaxEvents changes the maximum number of events that are
// allowed in the sliding window. If the new limit is lower,
// the oldest events will be forgotten. If the new limit is
// higher, the window will suddenly have capacity for new
// reservations. It panics if maxEvents is less than 0.
func (r *RingBufferRateLimiter) SetMaxEvents(maxEvents int) {
	if maxEvents < 0 {
		panic("maxEvents cannot be less than zero")
	}

	r.Mu.Lock()
	defer r.Mu.Unlock()

	// only make a change if the new limit is different
	if maxEvents == len(r.Ring) {
		return
	}

	newRing := make([]time.Time, maxEvents)

	// the new ring may be smaller; fast-forward to the
	// oldest timestamp that will be kept in the new
	// ring so the oldest ones are forgotten and the
	// newest ones will be remembered
	sizeDiff := len(r.Ring) - maxEvents
	for i := 0; i < sizeDiff; i++ {
		r.Advance()
	}

	if len(r.Ring) > 0 {
		// copy timestamps into the new ring until we
		// have either copied all of them or have reached
		// the capacity of the new ring
		startCursor := r.Cursor
		for i := 0; i < len(newRing); i++ {
			newRing[i] = r.Ring[r.Cursor]
			r.Advance()
			if r.Cursor == startCursor {
				// new ring is larger than old one;
				// "we've come full circle"
				break
			}
		}
	}

	r.Ring = newRing
	r.Cursor = 0
}

// Window returns the size of the sliding window.
func (r *RingBufferRateLimiter) Window() time.Duration {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	return r.Windows
}

// SetWindow changes r's sliding window duration to window.
// It panics if window is less than zero.
func (r *RingBufferRateLimiter) SetWindow(window time.Duration) {
	if window < 0 {
		panic("window cannot be less than zero")
	}
	r.Mu.Lock()
	r.Windows = window
	r.Mu.Unlock()
}

// Count counts how many events are in the window from the reference time.
func (r *RingBufferRateLimiter) Count(ref time.Time) int {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	return r.CountUnsynced(ref)
}

// countUnsycned counts how many events are in the window from the reference time.
// It is NOT safe to use without a lock on r.Mu.
// TODO: this is currently O(n) but could probably become O(log n) if we switch to some weird, custom binary search modulo ring length around the cursor.
func (r *RingBufferRateLimiter) CountUnsynced(ref time.Time) int {
	beginningOfWindow := ref.Add(-r.Windows)

	// This loop is a little gnarly, I know. We start at one before the cursor because that's
	// the newest event, and we're trying to count how many events are in the window; so
	// iterating backwards from the cursor and wrapping around to the end of the ring is the
	// same as iterating events in reverse chronological order starting with most recent. When
	// we encounter the first element that's outside the window, then eventsInWindow has the
	// correct count of events within the window.
	for eventsInWindow := 0; eventsInWindow < len(r.Ring); eventsInWindow++ {
		// start at cursor, add difference between ring length and offset (eventsInWindow),
		// then subtract 1 because we want to start 1 before cursor (newest event), then
		// modulus the ring length to wrap around if necessary
		i := (r.Cursor + (len(r.Ring) - eventsInWindow - 1)) % len(r.Ring)
		if r.Ring[i].Before(beginningOfWindow) {
			return eventsInWindow
		}
	}

	// if we looped the entire ring, all events are within the window
	return len(r.Ring)
}

// Current time function, to be substituted by tests
var now = time.Now
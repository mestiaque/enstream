package auth

import (
	"sync"
	"time"
)

// Throttle slows down PIN guessing per client IP: after FreeAttempts failures every further
// failure locks that IP out, doubling from BaseLockout up to MaxLockout.
type Throttle struct {
	FreeAttempts int
	BaseLockout  time.Duration
	MaxLockout   time.Duration
	now          func() time.Time

	mu      sync.Mutex
	entries map[string]*attempts
}

type attempts struct {
	fails int
	until time.Time
	last  time.Time
}

func NewThrottle() *Throttle {
	return &Throttle{FreeAttempts: 5, BaseLockout: 30 * time.Second, MaxLockout: 15 * time.Minute, now: time.Now, entries: map[string]*attempts{}}
}

// Wait is how long ip must wait before trying again (0 = may try now).
func (t *Throttle) Wait(ip string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if e := t.entries[ip]; e != nil {
		if d := e.until.Sub(t.now()); d > 0 {
			return d
		}
	}
	return 0
}

// Fail records a wrong PIN and returns the new lockout (0 while attempts are still free).
func (t *Throttle) Fail(ip string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	t.prune(now)
	e := t.entries[ip]
	if e == nil {
		e = &attempts{}
		t.entries[ip] = e
	}
	e.fails++
	e.last = now
	if e.fails <= t.FreeAttempts {
		return 0
	}
	d := t.BaseLockout << min(e.fails-t.FreeAttempts-1, 10)
	if d > t.MaxLockout || d <= 0 {
		d = t.MaxLockout
	}
	e.until = now.Add(d)
	return d
}

// Success forgets the failures of ip.
func (t *Throttle) Success(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, ip)
}

// prune drops entries idle for a day so the map can't grow without bound.
func (t *Throttle) prune(now time.Time) {
	if len(t.entries) < 1000 {
		return
	}
	for ip, e := range t.entries {
		if now.Sub(e.last) > 24*time.Hour {
			delete(t.entries, ip)
		}
	}
}

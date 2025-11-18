package metrics

import (
	"sync"
	"time"
)

type RateMeter struct {
	mu      sync.Mutex
	window  time.Duration
	last    time.Time
	count   int
	current float64
}

func NewRateMeter(window time.Duration) *RateMeter {
	return &RateMeter{
		window: window,
		last:   time.Now(),
	}
}

func (r *RateMeter) Tick() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.count++
	now := time.Now()
	elapsed := now.Sub(r.last)
	if elapsed >= r.window {
		r.current = float64(r.count) / elapsed.Seconds()
		r.count = 0
		r.last = now
	}
	return r.current
}

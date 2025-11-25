package ratelimit

import (
	"sync"
	"time"
)

// RateLimiter manages per-tenant rate limits using token bucket algorithm
type RateLimiter struct {
	mu              sync.Mutex
	tenantTokens    map[string]int
	tenantLastReset map[string]time.Time
	maxJobsPerMin   int
}

// New creates a new RateLimiter
func New(maxJobsPerMin int) *RateLimiter {
	return &RateLimiter{
		tenantTokens:    make(map[string]int),
		tenantLastReset: make(map[string]time.Time),
		maxJobsPerMin:   maxJobsPerMin,
	}
}

// Allow checks if a tenant is allowed to submit a job
func (rl *RateLimiter) Allow(tenantID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	lastReset, exists := rl.tenantLastReset[tenantID]

	// Reset tokens if a minute has passed
	if !exists || now.Sub(lastReset) > time.Minute {
		rl.tenantTokens[tenantID] = rl.maxJobsPerMin
		rl.tenantLastReset[tenantID] = now
	}

	// Check and consume token
	if rl.tenantTokens[tenantID] > 0 {
		rl.tenantTokens[tenantID]--
		return true
	}

	return false
}


package main

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter tracks request counts per IP address
type RateLimiter struct {
	requests map[string]*requestInfo
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

type requestInfo struct {
	count     int
	resetTime time.Time
}

// NewRateLimiter creates a new rate limiter
// limit: maximum requests allowed per window
// window: time window for rate limiting (e.g., 1 minute)
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*requestInfo),
		limit:    limit,
		window:   window,
	}

	// Start cleanup goroutine to remove old entries
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given IP should be allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Get or create request info for this IP
	info, exists := rl.requests[ip]
	if !exists || now.After(info.resetTime) {
		// New IP or window expired - reset counter
		rl.requests[ip] = &requestInfo{
			count:     1,
			resetTime: now.Add(rl.window),
		}
		return true
	}

	// Check if limit exceeded
	if info.count >= rl.limit {
		return false
	}

	// Increment counter
	info.count++
	return true
}

// cleanup removes expired entries every minute
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, info := range rl.requests {
			if now.After(info.resetTime) {
				delete(rl.requests, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimitMiddleware wraps an http.Handler with rate limiting
func RateLimitMiddleware(rl *RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract IP address using shared helper
		ip := getIPAddress(r)

		// Check rate limit
		if !rl.Allow(ip) {
			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		// Allow request to proceed
		next.ServeHTTP(w, r)
	})
}

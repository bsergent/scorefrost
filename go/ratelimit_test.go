package main

import (
	"testing"
	"time"
)

func TestRateLimiterCreation(t *testing.T) {
	// Test that rate limiter can be created without external dependencies
	limiter := NewRateLimiter(10, 1*time.Minute) // 10 requests per minute

	if limiter == nil {
		t.Error("Rate limiter should not be nil")
	}

	// Test that we can check if an IP is allowed (this doesn't hit external services)
	allowed := limiter.Allow("127.0.0.1")
	if !allowed {
		t.Error("First request should be allowed")
	}
}

func TestRateLimiterBasicFunctionality(t *testing.T) {
	// Create a strict rate limiter for testing
	limiter := NewRateLimiter(2, 1*time.Minute) // Only 2 requests per minute

	ip := "192.168.1.1"

	// First two requests should be allowed
	if !limiter.Allow(ip) {
		t.Error("First request should be allowed")
	}

	if !limiter.Allow(ip) {
		t.Error("Second request should be allowed")
	}

	// Third request should be denied
	if limiter.Allow(ip) {
		t.Error("Third request should be denied")
	}
}

//go:build integration

package main

import (
	"database/sql"
	"net/http"
	"time"
)

// setupTestRoutes creates HTTP handler with routes and test-friendly rate limiting
func setupTestRoutes(db *sql.DB) http.Handler {
	// Get shared routes
	mux := setupRoutes(db)

	// Create more permissive rate limiter for testing
	rateLimiter := NewRateLimiter(1000, 1*time.Minute)

	// Wrap with rate limiting middleware
	return RateLimitMiddleware(rateLimiter, mux)
}

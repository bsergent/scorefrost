package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Unit tests that don't require external dependencies

func TestHealthEndpoint(t *testing.T) {
	// Create a request to the health endpoint
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	
	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(health)
	
	// Call the handler with our request and recorder
	handler.ServeHTTP(rr, req)
	
	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Health handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	// Check the content type
	if contentType := rr.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Health handler returned wrong content type: got %v want %v", contentType, "application/json")
	}
	
	// Check that response contains expected JSON structure
	body := rr.Body.String()
	if !strings.Contains(body, `"status":"ok"`) {
		t.Errorf("Health handler response missing status: %s", body)
	}
	if !strings.Contains(body, `"time":`) {
		t.Errorf("Health handler response missing time: %s", body)
	}
}

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

func TestAPIKeyHashing(t *testing.T) {
	// Test API key hashing function
	apiKey := "test-api-key-123"
	hash1 := hashAPIKey(apiKey)
	hash2 := hashAPIKey(apiKey)
	
	// Same input should produce same hash
	if hash1 != hash2 {
		t.Errorf("API key hashing is not deterministic: %s != %s", hash1, hash2)
	}
	
	// Hash should not be empty
	if hash1 == "" {
		t.Error("API key hash should not be empty")
	}
	
	// Hash should not be the same as input
	if hash1 == apiKey {
		t.Error("API key hash should be different from input")
	}
}

func TestDisplayNameGeneration(t *testing.T) {
	// Test that display name generation works
	name1, err := generateRandomDisplayName()
	if err != nil {
		t.Fatalf("Failed to generate display name: %v", err)
	}
	
	name2, err := generateRandomDisplayName()
	if err != nil {
		t.Fatalf("Failed to generate second display name: %v", err)
	}
	
	// Names should not be empty
	if name1 == "" {
		t.Error("Generated display name should not be empty")
	}
	
	if name2 == "" {
		t.Error("Generated display name should not be empty")
	}
	
	// Names should be different (very likely)
	if name1 == name2 {
		t.Log("Generated names are the same (this is possible but unlikely)")
	}
}

func TestFriendCodeGeneration(t *testing.T) {
	// Test friend code generation
	code1, err := generateFriendCode()
	if err != nil {
		t.Fatalf("Failed to generate friend code: %v", err)
	}
	
	code2, err := generateFriendCode()
	if err != nil {
		t.Fatalf("Failed to generate second friend code: %v", err)
	}
	
	// Check format (XXXX-XXXX)
	if len(code1) != 9 {
		t.Errorf("Friend code should be 9 characters, got %d", len(code1))
	}
	
	if code1[4] != '-' {
		t.Errorf("Friend code should have dash at position 4, got %c", code1[4])
	}
	
	// Codes should be different (very likely)
	if code1 == code2 {
		t.Log("Generated friend codes are the same (this is possible but unlikely)")
	}
}

func TestAPIKeyGeneration(t *testing.T) {
	// Test API key generation
	key1, err := generateAPIKey()
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}
	
	key2, err := generateAPIKey()
	if err != nil {
		t.Fatalf("Failed to generate second API key: %v", err)
	}
	
	// Keys should not be empty
	if key1 == "" {
		t.Error("Generated API key should not be empty")
	}
	
	if key2 == "" {
		t.Error("Generated API key should not be empty")
	}
	
	// Keys should be different
	if key1 == key2 {
		t.Error("Generated API keys should be different")
	}
	
	// Keys should be base64 encoded (basic check)
	if len(key1) < 20 {
		t.Error("API key seems too short")
	}
}

func TestLeaderboardRouteRegistered(t *testing.T) {
	// Test that the leaderboard route is properly registered
	// We'll use a nil DB for this test since we're just checking route registration
	mux := setupRoutes(nil)
	
	// Create a test request to the leaderboard endpoint
	req, err := http.NewRequest("GET", "/score/leaderboard?levels=test&scope=global", nil)
	if err != nil {
		t.Fatal(err)
	}
	
	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	
	// Call the handler
	mux.ServeHTTP(rr, req)
	
	// The endpoint should be registered (even if it returns an error due to no auth/DB)
	// A 404 would indicate the route is not registered
	if rr.Code == http.StatusNotFound {
		t.Error("Leaderboard route is not registered - got 404")
	}
	
	// We expect some other status (like 401 Unauthorized due to missing auth)
	// This confirms the route exists and the handler is being called
}
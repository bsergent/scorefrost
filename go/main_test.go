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

func TestParseLevelsParameter(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCount int
		expectedFirst map[string]any
		expectedError bool
	}{
		{
			name:          "Valid single level",
			input:         "test.1",
			expectedCount: 1,
			expectedFirst: map[string]any{"level_id": "test", "level_version": 1},
			expectedError: false,
		},
		{
			name:          "Valid multiple levels",
			input:         "test.1,level2.2",
			expectedCount: 2,
			expectedFirst: map[string]any{"level_id": "test", "level_version": 1},
			expectedError: false,
		},
		{
			name:          "Valid latest version",
			input:         "test.-1",
			expectedCount: 1,
			expectedFirst: map[string]any{"level_id": "test", "level_version": -1},
			expectedError: false,
		},
		{
			name:          "Valid level without version (defaults to latest)",
			input:         "test",
			expectedCount: 1,
			expectedFirst: map[string]any{"level_id": "test", "level_version": -1},
			expectedError: false,
		},
		{
			name:          "Invalid format - empty",
			input:         "",
			expectedError: true,
		},
		{
			name:          "Invalid format - non-numeric version",
			input:         "test.abc",
			expectedError: true,
		},
		{
			name:          "Mixed valid formats",
			input:         "test.1,level2",
			expectedCount: 2,
			expectedFirst: map[string]any{"level_id": "test", "level_version": 1},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseLevelsParameter(tt.input)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d levels, got %d", tt.expectedCount, len(result))
				return
			}

			if tt.expectedCount > 0 {
				first := result[0]
				if first["level_id"] != tt.expectedFirst["level_id"] {
					t.Errorf("Expected level_id %v, got %v", tt.expectedFirst["level_id"], first["level_id"])
				}
				if first["level_version"] != tt.expectedFirst["level_version"] {
					t.Errorf("Expected level_version %v, got %v", tt.expectedFirst["level_version"], first["level_version"])
				}
			}
		})
	}
}

func TestLeaderboardRouteExists(t *testing.T) {
	// Test that the leaderboard route is properly registered
	// We'll use a nil DB for this test since we're just checking route registration
	mux := setupRoutes(nil)

	// Create a test request to the leaderboard endpoint
	req, err := http.NewRequest("GET", "/score/leaderboard?levels=test.1&scope=global", nil)
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

func TestLeaderboardResponseStructure(t *testing.T) {
	// Test the leaderboard response structure types
	entry := LeaderboardEntry{
		Rank:         1,
		LevelID:      "test",
		LevelVersion: 1,
		ScoreType:    "time_ms",
		BestScore:    15000,
		UserID:       "user123",
		DisplayName:  "Test User",
		FriendCode:   "ABC-123",
	}

	if entry.Rank != 1 {
		t.Errorf("Expected rank 1, got %d", entry.Rank)
	}
	if entry.ScoreType != "time_ms" {
		t.Errorf("Expected score type 'time_ms', got '%s'", entry.ScoreType)
	}

	// Test pagination structure
	pagination := PaginationInfo{
		Offset: 0,
		Size:   50,
		Total:  100,
	}

	if pagination.Total != 100 {
		t.Errorf("Expected total 100, got %d", pagination.Total)
	}

	// Test full response structure
	response := LeaderboardResponse{
		Scores:     []LeaderboardEntry{entry},
		Count:      1,
		Pagination: pagination,
	}

	if response.Count != 1 {
		t.Errorf("Expected count 1, got %d", response.Count)
	}
	if len(response.Scores) != 1 {
		t.Errorf("Expected 1 score, got %d", len(response.Scores))
	}
}

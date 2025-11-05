package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Unit tests for main.go - health endpoint and route registration

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

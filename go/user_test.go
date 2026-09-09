package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

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

func TestUUIDValidation(t *testing.T) {
	// Test UUID validation in login request processing
	testCases := []struct {
		uuid  string
		valid bool
	}{
		{"123e4567-e89b-12d3-a456-426614174000", true}, // Valid UUID
		{"00000000-0000-0000-0000-000000000001", true}, // Valid UUID (all zeros except last digit)
		{"invalid-uuid-format", false},                 // Invalid format
		{"123e4567-e89b-12d3-a456-42661417400", false}, // Too short
		{"", false}, // Empty
	}

	for _, tc := range testCases {
		t.Run(tc.uuid, func(t *testing.T) {
			// We'll use uuid.Parse which is the same as what the code uses
			_, err := uuid.Parse(tc.uuid)
			isValid := err == nil
			if isValid != tc.valid {
				t.Errorf("UUID %q: expected valid=%v, got valid=%v", tc.uuid, tc.valid, isValid)
			}
		})
	}
}

func TestLoginUserHandler_MissingUserIDWithAPIKey(t *testing.T) {
	handler := loginUserHandler(nil)

	requestBody := map[string]string{
		"game_id":      "com.company.testgame",
		"game_version": "1.0.0",
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, APIBasePath+"/user", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer some-api-key")

	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "user_id is required") {
		t.Fatalf("Expected error body to contain %q, got %q", "user_id is required", rr.Body.String())
	}
}

func TestLoginUserHandler_InvalidUserIDWithAPIKey(t *testing.T) {
	handler := loginUserHandler(nil)

	requestBody := map[string]string{
		"game_id":      "com.company.testgame",
		"game_version": "1.0.0",
		"user_id":      "not-a-uuid",
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, APIBasePath+"/user", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer some-api-key")

	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "Invalid request body") {
		t.Fatalf("Expected error body to contain %q, got %q", "Invalid request body", rr.Body.String())
	}
}

func TestLoginUserHandler_NoAPIKey_IgnoresUserIDAndCreatesUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create sqlmock DB: %v", err)
	}
	defer db.Close()

	// create_user call from tryCreateUser
	mock.ExpectQuery(`SELECT create_user\(\$1, \$2, \$3, \$4, \$5\)`).
		WillReturnRows(sqlmock.NewRows([]string{"create_user"}).AddRow("generated"))

	// fetchUserFullObject call
	mock.ExpectQuery(`(?s)SELECT\s+date_time_created_utc.*FROM "user".*WHERE id = \$1`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"date_time_created_utc",
			"date_time_active_utc",
			"game_version",
			"display_name",
			"friend_code",
			"get_user_play_time_ms",
		}).AddRow(
			"2026-09-01T00:00:00Z",
			"2026-09-01T00:00:00Z",
			"1.0.0",
			"Test Name",
			"ABCD-EFGH",
			123,
		))

	// touch_user_active_time call
	mock.ExpectExec(`SELECT touch_user_active_time\(\$1, \$2\)`).
		WithArgs(sqlmock.AnyArg(), "1.0.0").
		WillReturnResult(sqlmock.NewResult(1, 1))

	handler := loginUserHandler(db)

	requestBody := map[string]string{
		"game_id":      "com.company.testgame",
		"game_version": "1.0.0",
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, APIBasePath+"/user", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d; body=%s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var resp UserFull
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.APIKey == "" {
		t.Fatalf("Expected API key for created user")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("Unmet SQL expectations: %v", err)
	}
}

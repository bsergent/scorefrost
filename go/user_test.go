package main

import (
	"testing"
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
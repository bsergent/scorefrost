package main

import (
	"strings"
	"testing"
)

func TestSanitizeDisplayName_ValidAndTrimmed(t *testing.T) {
	input := DisplayName("  Cool_Gamer-42  ")
	original := input

	sanitized, err := sanitizeDisplayName(input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if sanitized != "Cool_Gamer-42" {
		t.Fatalf("Expected sanitized display name %q, got %q", "Cool_Gamer-42", sanitized)
	}

	if input != original {
		t.Fatalf("Expected source value to be unchanged, got %q", input)
	}
}

func TestSanitizeDisplayName_Empty(t *testing.T) {
	_, err := sanitizeDisplayName("")
	if err == nil {
		t.Fatal("Expected error for empty display name")
	}
}

func TestSanitizeDisplayName_InvalidCases(t *testing.T) {
	testCases := []struct {
		name  string
		input DisplayName
	}{
		{name: "too short", input: "ab"},
		{name: "invalid characters", input: "name!"},
		{name: "too long", input: DisplayName(strings.Repeat("a", 33))},
		{name: "only spaces becomes empty", input: "    "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := tc.input
			_, err := sanitizeDisplayName(input)
			if err == nil {
				t.Fatalf("Expected error for case %q with input %q", tc.name, tc.input)
			}
		})
	}
}

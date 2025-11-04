//go:build integration

package main

import (
	"net/http/httptest"
	"testing"
)

// Integration tests for ScoreFrost API

func TestIntegrationUserCreation(t *testing.T) {
	db := mustConnectToIntegrationDB()
	defer db.Close()

	server := httptest.NewServer(setupTestRoutes(db))
	defer server.Close()

	// Test creating a new user (no input required)
	user, err := createIntegrationTestUser(server.URL)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Verify user has all required fields
	if user.ID == "" {
		t.Error("User ID is empty")
	}
	if user.FriendCode == "" {
		t.Error("Friend code is empty")
	}
	if user.DisplayName == "" {
		t.Error("Display name is empty")
	}
	if user.APIKey == "" {
		t.Error("API key is empty")
	}

	// Verify friend code format (XXXX-XXXX)
	if len(user.FriendCode) != 9 || user.FriendCode[4] != '-' {
		t.Errorf("Invalid friend code format: %s", user.FriendCode)
	}
}

func TestIntegrationScoreSubmission(t *testing.T) {
	db := mustConnectToIntegrationDB()
	defer db.Close()

	server := httptest.NewServer(setupTestRoutes(db))
	defer server.Close()

	// Prepare test data
	solution := "SGVsbG8gV29ybGQ=" // "Hello World" in base64
	solutionHash := calculateIntegrationSolutionHash(solution)

	request := IntegrationScoreSubmissionRequest{
		Solution:     solution,
		SolutionHash: solutionHash,
		LevelID:      "test_level_001",
		LevelVersion: 1,
		GameVersion:  "1.0.0",
		Scores: map[string]int{
			"time_ms":  15000,
			"striping": 8,
			"fuel":     45,
		},
	}

	// Submit score for first user
	response, err := submitIntegrationScore(server, integrationConfig.TestAPIKey, request)
	if err != nil {
		t.Fatalf("Failed to submit score: %v", err)
	}

	// Verify response
	if !response.Success {
		t.Errorf("Score submission failed: %s", response.Message)
	}
	if response.SolutionID <= 0 {
		t.Error("Invalid solution ID returned")
	}

	// Submit a better score for second user
	betterRequest := request
	betterRequest.Scores = map[string]int{
		"time_ms":  12000,
		"striping": 10,
		"fuel":     35,
	}

	betterResponse, err := submitIntegrationScore(server, integrationConfig.TestAPIKey2, betterRequest)
	if err != nil {
		t.Fatalf("Failed to submit better score: %v", err)
	}

	if !betterResponse.Success {
		t.Errorf("Better score submission failed: %s", betterResponse.Message)
	}
}

func TestIntegrationBestScoresGlobal(t *testing.T) {
	db := mustConnectToIntegrationDB()
	defer db.Close()

	server := httptest.NewServer(setupTestRoutes(db))
	defer server.Close()

	// First submit some scores to ensure we have data
	solution := "SGVsbG8gV29ybGQ=" // "Hello World" in base64
	solutionHash := calculateIntegrationSolutionHash(solution)

	request1 := IntegrationScoreSubmissionRequest{
		Solution:     solution,
		SolutionHash: solutionHash,
		LevelID:      "test_level_002",
		LevelVersion: 1,
		GameVersion:  "1.0.0",
		Scores: map[string]int{
			"time_ms": 20000,
			"stars":   2,
		},
	}

	request2 := request1
	request2.Scores = map[string]int{
		"time_ms": 18000, // Better time
		"stars":   3,     // Better stars
	}

	// Submit scores for both users
	_, err := submitIntegrationScore(server, integrationConfig.TestAPIKey, request1)
	if err != nil {
		t.Fatalf("Failed to submit first score: %v", err)
	}

	_, err = submitIntegrationScore(server, integrationConfig.TestAPIKey2, request2)
	if err != nil {
		t.Fatalf("Failed to submit second score: %v", err)
	}

	// Get best scores
	response, err := getIntegrationBestScores(server, integrationConfig.TestAPIKey, []string{"test_level_002.1"}, "global")
	if err != nil {
		t.Fatalf("Failed to get best scores: %v", err)
	}

	// Verify response structure
	if response.Scope != "global" {
		t.Errorf("Expected scope 'global', got '%s'", response.Scope)
	}

	// Verify count field matches array length
	assertIntegrationScoreCount(t, response, len(response.Scores))

	// Verify we have the better scores
	assertIntegrationScoreExists(t, response.Scores, "test_level_002", "time_ms", 18000)
	assertIntegrationScoreExists(t, response.Scores, "test_level_002", "stars", 3)

	// Verify the better scores belong to user 2
	assertIntegrationScoreUser(t, response.Scores, "test_level_002", "time_ms", integrationConfig.TestUserID2)
	assertIntegrationScoreUser(t, response.Scores, "test_level_002", "stars", integrationConfig.TestUserID2)
}

func TestIntegrationBestScoresPersonal(t *testing.T) {
	db := mustConnectToIntegrationDB()
	defer db.Close()

	server := httptest.NewServer(setupTestRoutes(db))
	defer server.Close()

	// Submit scores for user 1
	solution := "VGVzdCBTb2x1dGlvbg==" // "Test Solution" in base64
	solutionHash := calculateIntegrationSolutionHash(solution)

	request := IntegrationScoreSubmissionRequest{
		Solution:     solution,
		SolutionHash: solutionHash,
		LevelID:      "test_level_003",
		LevelVersion: 1,
		GameVersion:  "1.0.0",
		Scores: map[string]int{
			"time_ms": 25000,
			"fuel":    60,
		},
	}

	_, err := submitIntegrationScore(server, integrationConfig.TestAPIKey, request)
	if err != nil {
		t.Fatalf("Failed to submit score: %v", err)
	}

	// Get personal best scores for user 1
	response, err := getIntegrationBestScores(server, integrationConfig.TestAPIKey, []string{"test_level_003.1"}, "personal")
	if err != nil {
		t.Fatalf("Failed to get personal best scores: %v", err)
	}

	// Verify response
	if response.Scope != "personal" {
		t.Errorf("Expected scope 'personal', got '%s'", response.Scope)
	}

	assertIntegrationScoreCount(t, response, len(response.Scores))

	// All scores should belong to user 1
	for _, score := range response.Scores {
		if score.UserID != integrationConfig.TestUserID {
			t.Errorf("Expected all scores to belong to user %s, found score for user %s", integrationConfig.TestUserID, score.UserID)
		}
	}

	assertIntegrationScoreExists(t, response.Scores, "test_level_003", "time_ms", 25000)
	assertIntegrationScoreExists(t, response.Scores, "test_level_003", "fuel", 60)
}

func TestIntegrationBestScoresMultipleLevels(t *testing.T) {
	db := mustConnectToIntegrationDB()
	defer db.Close()

	server := httptest.NewServer(setupTestRoutes(db))
	defer server.Close()

	// Submit scores for multiple levels
	solution := "SGVsbG8gV29ybGQ=" // "Hello World" in base64
	solutionHash := calculateIntegrationSolutionHash(solution)

	// Level A
	requestA := IntegrationScoreSubmissionRequest{
		Solution:     solution,
		SolutionHash: solutionHash,
		LevelID:      "test_level_A",
		LevelVersion: 1,
		GameVersion:  "1.0.0",
		Scores: map[string]int{
			"time_ms": 10000,
		},
	}

	// Level B
	requestB := IntegrationScoreSubmissionRequest{
		Solution:     solution,
		SolutionHash: solutionHash,
		LevelID:      "test_level_B",
		LevelVersion: 1,
		GameVersion:  "1.0.0",
		Scores: map[string]int{
			"time_ms": 15000,
		},
	}

	_, err := submitIntegrationScore(server, integrationConfig.TestAPIKey, requestA)
	if err != nil {
		t.Fatalf("Failed to submit score A: %v", err)
	}

	_, err = submitIntegrationScore(server, integrationConfig.TestAPIKey, requestB)
	if err != nil {
		t.Fatalf("Failed to submit score B: %v", err)
	}

	// Get best scores for both levels
	response, err := getIntegrationBestScores(server, integrationConfig.TestAPIKey, []string{"test_level_A.1", "test_level_B.1"}, "global")
	if err != nil {
		t.Fatalf("Failed to get best scores for multiple levels: %v", err)
	}

	// Should have scores for both levels
	foundA := false
	foundB := false

	for _, score := range response.Scores {
		if score.LevelID == "test_level_A" {
			foundA = true
		}
		if score.LevelID == "test_level_B" {
			foundB = true
		}
	}

	if !foundA {
		t.Error("Missing scores for test_level_A")
	}
	if !foundB {
		t.Error("Missing scores for test_level_B")
	}

	assertIntegrationScoreCount(t, response, len(response.Scores))
}

func TestIntegrationLatestVersionDetection(t *testing.T) {
	db := mustConnectToIntegrationDB()
	defer db.Close()

	server := httptest.NewServer(setupTestRoutes(db))
	defer server.Close()

	// Submit score for version 1
	solution := "SGVsbG8gV29ybGQ=" // "Hello World" in base64
	solutionHash := calculateIntegrationSolutionHash(solution)

	requestV1 := IntegrationScoreSubmissionRequest{
		Solution:     solution,
		SolutionHash: solutionHash,
		LevelID:      "test_level_v",
		LevelVersion: 1,
		GameVersion:  "1.0.0",
		Scores: map[string]int{
			"time_ms": 30000,
		},
	}

	// Submit score for version 2 (higher version)
	requestV2 := IntegrationScoreSubmissionRequest{
		Solution:     solution,
		SolutionHash: solutionHash,
		LevelID:      "test_level_v",
		LevelVersion: 2,
		GameVersion:  "1.0.0",
		Scores: map[string]int{
			"time_ms": 25000,
		},
	}

	_, err := submitIntegrationScore(server, integrationConfig.TestAPIKey, requestV1)
	if err != nil {
		t.Fatalf("Failed to submit score v1: %v", err)
	}

	_, err = submitIntegrationScore(server, integrationConfig.TestAPIKey, requestV2)
	if err != nil {
		t.Fatalf("Failed to submit score v2: %v", err)
	}

	// Query without specifying version (should get latest = version 2)
	response, err := getIntegrationBestScores(server, integrationConfig.TestAPIKey, []string{"test_level_v"}, "global")
	if err != nil {
		t.Fatalf("Failed to get best scores with latest version: %v", err)
	}

	// Should return version 2 scores only
	for _, score := range response.Scores {
		if score.LevelVersion != 2 {
			t.Errorf("Expected version 2, got version %d", score.LevelVersion)
		}
	}

	assertIntegrationScoreExists(t, response.Scores, "test_level_v", "time_ms", 25000)
}

func TestIntegrationLeaderboard(t *testing.T) {
	db := mustConnectToIntegrationDB()
	defer db.Close()

	server := httptest.NewServer(setupTestRoutes(db))
	defer server.Close()

	// Submit multiple scores to create a leaderboard
	solution := "SGVsbG8gV29ybGQ=" // "Hello World" in base64
	solutionHash := calculateIntegrationSolutionHash(solution)

	// User 1: Good score
	request1 := IntegrationScoreSubmissionRequest{
		Solution:     solution,
		SolutionHash: solutionHash,
		LevelID:      "leaderboard_test",
		LevelVersion: 1,
		GameVersion:  "1.0.0",
		Scores: map[string]int{
			"time_ms": 20000,
		},
	}

	// User 2: Better score
	request2 := IntegrationScoreSubmissionRequest{
		Solution:     solution,
		SolutionHash: solutionHash,
		LevelID:      "leaderboard_test",
		LevelVersion: 1,
		GameVersion:  "1.0.0",
		Scores: map[string]int{
			"time_ms": 15000,
		},
	}

	_, err := submitIntegrationScore(server, integrationConfig.TestAPIKey, request1)
	if err != nil {
		t.Fatalf("Failed to submit score for user 1: %v", err)
	}

	_, err = submitIntegrationScore(server, integrationConfig.TestAPIKey2, request2)
	if err != nil {
		t.Fatalf("Failed to submit score for user 2: %v", err)
	}

	// Test global leaderboard with pagination
	response, err := getIntegrationLeaderboard(server, integrationConfig.TestAPIKey, []string{"leaderboard_test.1"}, "global", 0, 10)
	if err != nil {
		t.Fatalf("Failed to get leaderboard: %v", err)
	}

	// Verify response structure
	if response.Scope != "global" {
		t.Errorf("Expected scope 'global', got '%s'", response.Scope)
	}

	if response.Pagination.Offset != 0 {
		t.Errorf("Expected offset 0, got %d", response.Pagination.Offset)
	}

	if response.Pagination.Size != 10 {
		t.Errorf("Expected size 10, got %d", response.Pagination.Size)
	}

	if response.Count != len(response.Scores) {
		t.Errorf("Count mismatch: expected %d, got %d", len(response.Scores), response.Count)
	}

	// Verify ranking: User 2 should be rank 1 (better time), User 1 should be rank 2
	for _, score := range response.Scores {
		if score.UserID == integrationConfig.TestUserID2 && score.Rank != 1 {
			t.Errorf("User 2 should be rank 1 (best score), got rank %d", score.Rank)
		}
		if score.UserID == integrationConfig.TestUserID && score.Rank != 2 {
			t.Errorf("User 1 should be rank 2, got rank %d", score.Rank)
		}
	}

	// Test personal leaderboard
	personalResponse, err := getIntegrationLeaderboard(server, integrationConfig.TestAPIKey, []string{"leaderboard_test.1"}, "personal", 0, 10)
	if err != nil {
		t.Fatalf("Failed to get personal leaderboard: %v", err)
	}

	if personalResponse.Scope != "personal" {
		t.Errorf("Expected scope 'personal', got '%s'", personalResponse.Scope)
	}

	// Personal leaderboard should only contain scores for the authenticated user
	for _, score := range personalResponse.Scores {
		if score.UserID != integrationConfig.TestUserID {
			t.Errorf("Personal leaderboard should only contain scores for user %s, found %s", integrationConfig.TestUserID, score.UserID)
		}
	}

	// Test pagination
	paginatedResponse, err := getIntegrationLeaderboard(server, integrationConfig.TestAPIKey, []string{"leaderboard_test.1"}, "global", 1, 1)
	if err != nil {
		t.Fatalf("Failed to get paginated leaderboard: %v", err)
	}

	if paginatedResponse.Pagination.Offset != 1 {
		t.Errorf("Expected offset 1, got %d", paginatedResponse.Pagination.Offset)
	}

	if paginatedResponse.Pagination.Size != 1 {
		t.Errorf("Expected size 1, got %d", paginatedResponse.Pagination.Size)
	}

	if len(paginatedResponse.Scores) > 1 {
		t.Errorf("Expected at most 1 score with size=1, got %d", len(paginatedResponse.Scores))
	}
}

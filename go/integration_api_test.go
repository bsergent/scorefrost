//go:build integration

package main

import (
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
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

func TestIntegrationUserAuthentication(t *testing.T) {
	db := mustConnectToIntegrationDB()
	defer db.Close()

	server := httptest.NewServer(setupTestRoutes(db))
	defer server.Close()

	// First create a new user
	newUser, err := createIntegrationTestUser(server.URL)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Now authenticate with the API key from the new user
	authenticatedUser, err := authenticateIntegrationTestUser(server.URL, newUser.APIKey)
	if err != nil {
		t.Fatalf("Failed to authenticate user: %v", err)
	}

	// Verify the authenticated user matches the original user
	if authenticatedUser.ID != newUser.ID {
		t.Errorf("Authenticated user ID mismatch: got %s, want %s", authenticatedUser.ID, newUser.ID)
	}
	if authenticatedUser.FriendCode != newUser.FriendCode {
		t.Errorf("Authenticated user friend code mismatch: got %s, want %s", authenticatedUser.FriendCode, newUser.FriendCode)
	}
	if authenticatedUser.DisplayName != newUser.DisplayName {
		t.Errorf("Authenticated user display name mismatch: got %s, want %s", authenticatedUser.DisplayName, newUser.DisplayName)
	}

	// API key should not be included in authentication response
	if authenticatedUser.APIKey != "" {
		t.Error("API key should not be included in authentication response")
	}

	// Test authentication with invalid API key
	_, err = authenticateIntegrationTestUser(server.URL, "invalid-api-key")
	if err == nil {
		t.Error("Authentication with invalid API key should fail")
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
	if response.SolutionID == "" {
		t.Error("Empty solution ID returned")
	} else if _, err := uuid.Parse(response.SolutionID); err != nil {
		t.Errorf("Invalid UUID solution ID returned: %s", response.SolutionID)
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

	if betterResponse.SolutionID == "" {
		t.Error("Empty solution ID returned for better score")
	} else if _, err := uuid.Parse(betterResponse.SolutionID); err != nil {
		t.Errorf("Invalid UUID solution ID returned for better score: %s", betterResponse.SolutionID)
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
	response, err := getIntegrationLeaderboard(server, integrationConfig.TestAPIKey, []string{"leaderboard_test.1"}, "global", "", 0, 10)
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
	personalResponse, err := getIntegrationLeaderboard(server, integrationConfig.TestAPIKey, []string{"leaderboard_test.1"}, "personal", "", 0, 10)
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
	paginatedResponse, err := getIntegrationLeaderboard(server, integrationConfig.TestAPIKey, []string{"leaderboard_test.1"}, "global", "", 1, 1)
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

func TestIntegrationLeaderboardScoreTypeFiltering(t *testing.T) {
	db := mustConnectToIntegrationDB()
	defer db.Close()

	server := httptest.NewServer(setupTestRoutes(db))
	defer server.Close()

	solution := "dGVzdCBzb2x1dGlvbiBmb3Igc2NvcmUgdHlwZSBmaWx0ZXJpbmc=" // "test solution for score type filtering" in base64
	solutionHash := calculateIntegrationSolutionHash(solution)

	// Submit scores with multiple score types for user 1
	request1 := IntegrationScoreSubmissionRequest{
		Solution:     solution,
		SolutionHash: solutionHash,
		LevelID:      "score_type_test",
		LevelVersion: 1,
		GameVersion:  "1.0.0",
		Scores: map[string]int{
			"time_ms":  20000, // Better time than user 2 (lower is better)
			"striping": 5,     // Worse striping than user 2 (higher is better)
		},
	}

	// Submit scores with multiple score types for user 2
	request2 := IntegrationScoreSubmissionRequest{
		Solution:     solution,
		SolutionHash: solutionHash,
		LevelID:      "score_type_test",
		LevelVersion: 1,
		GameVersion:  "1.0.0",
		Scores: map[string]int{
			"time_ms":  25000, // Worse time than user 1 (lower is better)
			"striping": 8,     // Better striping than user 1 (higher is better)
		},
	}

	// Submit scores for both users
	_, err := submitIntegrationScore(server, integrationConfig.TestAPIKey, request1)
	if err != nil {
		t.Fatalf("Failed to submit score for user 1: %v", err)
	}

	_, err = submitIntegrationScore(server, integrationConfig.TestAPIKey2, request2)
	if err != nil {
		t.Fatalf("Failed to submit score for user 2: %v", err)
	}

	// Test filtering by time_ms - should return only time scores
	timeResponse, err := getIntegrationLeaderboard(server, integrationConfig.TestAPIKey, []string{"score_type_test.1"}, "global", "time_ms", 0, 10)
	if err != nil {
		t.Fatalf("Failed to get time_ms leaderboard: %v", err)
	}

	// Verify all returned scores are time_ms type
	for _, score := range timeResponse.Scores {
		if score.ScoreType != "time_ms" {
			t.Errorf("Expected score_type 'time_ms', got '%s'", score.ScoreType)
		}
	}

	// Verify ranking: User 1 should be rank 1 (better/lower time)
	if len(timeResponse.Scores) >= 2 {
		user1Found := false
		user2Found := false
		for _, score := range timeResponse.Scores {
			if score.UserID == integrationConfig.TestUserID {
				user1Found = true
				if score.Rank != 1 {
					t.Errorf("User 1 should be rank 1 for time_ms, got rank %d", score.Rank)
				}
			}
			if score.UserID == integrationConfig.TestUserID2 {
				user2Found = true
				if score.Rank != 2 {
					t.Errorf("User 2 should be rank 2 for time_ms, got rank %d", score.Rank)
				}
			}
		}
		if !user1Found || !user2Found {
			t.Error("Both users should appear in time_ms leaderboard")
		}
	}

	// Test filtering by striping - should return only striping scores
	stripingResponse, err := getIntegrationLeaderboard(server, integrationConfig.TestAPIKey, []string{"score_type_test.1"}, "global", "striping", 0, 10)
	if err != nil {
		t.Fatalf("Failed to get striping leaderboard: %v", err)
	}

	// Verify all returned scores are striping type
	for _, score := range stripingResponse.Scores {
		if score.ScoreType != "striping" {
			t.Errorf("Expected score_type 'striping', got '%s'", score.ScoreType)
		}
	}

	// Verify ranking: User 2 should be rank 1 (higher striping is better)
	if len(stripingResponse.Scores) >= 2 {
		user1Found := false
		user2Found := false
		for _, score := range stripingResponse.Scores {
			if score.UserID == integrationConfig.TestUserID2 {
				user2Found = true
				if score.Rank != 1 {
					t.Errorf("User 2 should be rank 1 for striping, got rank %d", score.Rank)
				}
			}
			if score.UserID == integrationConfig.TestUserID {
				user1Found = true
				if score.Rank != 2 {
					t.Errorf("User 1 should be rank 2 for striping, got rank %d", score.Rank)
				}
			}
		}
		if !user1Found || !user2Found {
			t.Error("Both users should appear in striping leaderboard")
		}
	}

	// Test without score_type filter - should return all scores
	allResponse, err := getIntegrationLeaderboard(server, integrationConfig.TestAPIKey, []string{"score_type_test.1"}, "global", "", 0, 10)
	if err != nil {
		t.Fatalf("Failed to get all scores leaderboard: %v", err)
	}

	// Should contain both time_ms and striping scores - one from each user showing their best
	timeScoresFound := 0
	stripingScoresFound := 0
	for _, score := range allResponse.Scores {
		if score.ScoreType == "time_ms" {
			timeScoresFound++
		} else if score.ScoreType == "striping" {
			stripingScoresFound++
		}
	}

	if timeScoresFound != 2 {
		t.Errorf("Expected 2 time_ms scores (one per user), got %d", timeScoresFound)
	}
	if stripingScoresFound != 2 {
		t.Errorf("Expected 2 striping scores (one per user), got %d", stripingScoresFound)
	}
}

package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// ScoreSubmissionRequest represents the JSON body for score submission
type ScoreSubmissionRequest struct {
	LevelID      string         `json:"level_id"`
	LevelVersion int            `json:"level_version"`
	GameVersion  string         `json:"game_version"`
	Solution     string         `json:"solution"`      // base64 encoded solution bytes
	SolutionHash string         `json:"solution_hash"` // salted hash of solution
	Scores       map[string]int `json:"scores"`        // score type -> score value
}

// ScoreSubmissionResponse represents the JSON response for score submission
type ScoreSubmissionResponse struct {
	Success    bool   `json:"success"`
	SolutionID int    `json:"solution_id,omitempty"`
	Message    string `json:"message,omitempty"`
}

// submitScoreHandler handles POST /score/submit requests
// Uses a stored procedure for atomic solution and score insertion
func submitScoreHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var req ScoreSubmissionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("Failed to parse score submission request: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if req.LevelID == "" {
			http.Error(w, "Level ID is required", http.StatusBadRequest)
			return
		}

		if req.GameVersion == "" {
			http.Error(w, "Game version is required", http.StatusBadRequest)
			return
		}

		if req.Solution == "" {
			http.Error(w, "Solution is required", http.StatusBadRequest)
			return
		}

		if req.SolutionHash == "" {
			http.Error(w, "Solution hash is required", http.StatusBadRequest)
			return
		}

		// Get authenticated user ID from context (set by authMiddleware)
		userID, ok := GetUserID(r)
		if !ok {
			log.Printf("User ID not found in request context")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Verify solution hash
		if !verifySolutionHash(req.Solution, req.SolutionHash) {
			log.Printf("Solution hash verification failed for user %s", userID)
			http.Error(w, "Solution hash verification failed", http.StatusBadRequest)
			return
		}

		// Validate base64 solution
		if _, err := base64.StdEncoding.DecodeString(req.Solution); err != nil {
			http.Error(w, "Invalid base64 solution format", http.StatusBadRequest)
			return
		}

		// Convert scores map to JSON array format expected by the stored procedure
		var scoresJSON []map[string]interface{}
		for scoreType, scoreValue := range req.Scores {
			scoresJSON = append(scoresJSON, map[string]interface{}{
				"type":  scoreType,
				"value": scoreValue,
			})
		}

		scoresJSONBytes, err := json.Marshal(scoresJSON)
		if err != nil {
			log.Printf("Failed to marshal scores to JSON: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Call stored procedure to submit solution with scores
		var solutionID int
		err = db.QueryRow(`
			SELECT submit_solution_with_scores($1, $2, $3, $4, $5, $6, $7)
		`, userID, req.LevelID, req.LevelVersion, req.GameVersion, req.Solution, 1, string(scoresJSONBytes)).
			Scan(&solutionID)

		if err != nil {
			log.Printf("Failed to submit solution with scores: %v", err)
			// Check if it's a score type validation error
			if strings.Contains(err.Error(), "Invalid score type:") {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, "Failed to save solution", http.StatusInternalServerError)
			return
		}

		log.Printf("Score submitted successfully: solution_id=%d, user=%s, level=%s",
			solutionID, userID, req.LevelID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ScoreSubmissionResponse{
			Success:    true,
			SolutionID: solutionID,
			Message:    "Score submitted successfully",
		})
	}
}

// verifySolutionHash verifies that the provided hash matches the solution with the secret salt
func verifySolutionHash(solution, providedHash string) bool {
	salt := os.Getenv("SOLUTION_SALT")
	if salt == "" {
		log.Printf("Warning: SOLUTION_SALT not configured")
		return false
	}

	// Create the salted solution string
	saltedSolution := solution + salt

	// Calculate hash
	hash := sha256.Sum256([]byte(saltedSolution))
	expectedHash := fmt.Sprintf("%x", hash)

	// Compare hashes (case-insensitive)
	return strings.EqualFold(expectedHash, providedHash)
}

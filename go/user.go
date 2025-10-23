package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// Word lists for random display name generation
var adjectives = []string{
	"Awesome", "Blazing", "Bouncy", "Brave", "Bright", "Cheerful", "Clever", "Cool",
	"Cosmic", "Crafty", "Daring", "Dazzling", "Epic", "Fearless", "Friendly", "Funky",
	"Frosty", "Gentle", "Giggly", "Glowing", "Golden", "Happy", "Heroic", "Jolly",
	"Jumpy", "Legendary", "Lightning", "Lucky", "Magical", "Majestic", "Mighty", "Mystic",
	"Noble", "Nimble", "Peppy", "Playful", "Powerful", "Quick", "Radiant", "Royal",
	"Shiny", "Silly", "Smooth", "Snappy", "Sparkly", "Speedy", "Stellar", "Super",
	"Swift", "Turbo", "Ultimate", "Vibrant", "Wild", "Zippy", "Zany",
}

var nouns = []string{
	"Archer", "Adventurer", "Bear", "Comet", "Cactus", "Dragon", "Dreamer", "Eagle",
	"Explorer", "Falcon", "Flame", "Fox", "Gamer", "Glacier", "Hero", "Hunter",
	"Knight", "Legend", "Lion", "Mage", "Meteor", "Ninja", "Otter", "Panda",
	"Penguin", "Phoenix", "Pirate", "Player", "Racer", "Ranger", "Rebel", "Rocket",
	"Samurai", "Scout", "Shadow", "Shark", "Sloth", "Spirit", "Star", "Storm",
	"Tiger", "Titan", "Viking", "Warrior", "Wizard", "Wolf", "Wonder", "Yeti",
}

// CreateUserResponse represents the JSON response for POST /user
type CreateUserResponse struct {
	ID          string `json:"id"`
	FriendCode  string `json:"friend_code"`
	DisplayName string `json:"display_name"`
	APIKey      string `json:"api_key"`
}

// GetUserResponse represents the JSON response for GET /user/{id}
type GetUserResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	FriendCode  string `json:"friend_code"`
}

// UpdateDisplayNameRequest represents the JSON request body for PUT /user/{id}/name
type UpdateDisplayNameRequest struct {
	DisplayName string `json:"display_name"`
}

// UpdateDisplayNameResponse represents the JSON response for PUT /user/{id}/name
type UpdateDisplayNameResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Message     string `json:"message"`
}

// createUserHandler handles POST /user requests
func createUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Generate new UUID for the user
		userID := uuid.New()

		// Generate a random 256-bit API key (32 bytes)
		apiKey, err := generateAPIKey()
		if err != nil {
			log.Printf("Failed to generate API key: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Hash the API key for storage
		apiKeyHash := hashAPIKey(apiKey)

		// Generate random display name
		displayName, err := generateRandomDisplayName()
		if err != nil {
			log.Printf("Failed to generate display name: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Generate a unique friend code with retry logic
		const maxRetries = 10
		var friendCode string
		var returnedID string

		for i := range maxRetries {
			// Generate friend code
			friendCode, err = generateFriendCode()
			if err != nil {
				log.Printf("Failed to generate friend code: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			// Try to insert user into database
			err = db.QueryRow(
				"SELECT create_user($1, $2, $3, $4)",
				userID.String(),
				friendCode,
				displayName,
				apiKeyHash,
			).Scan(&returnedID)

			// If successful, break out of retry loop
			if err == nil {
				break
			}

			// Check if error is due to duplicate friend code
			// If it's a different error, return immediately
			if !isDuplicateKeyError(err) {
				log.Printf("Failed to create user: %v", err)
				http.Error(w, "Failed to create user", http.StatusInternalServerError)
				return
			}

			// If last retry, return error
			if i == maxRetries-1 {
				log.Printf("Failed to generate unique friend code after %d attempts", maxRetries)
				http.Error(w, "Failed to create user", http.StatusInternalServerError)
				return
			}

			// Otherwise, retry with new friend code
			log.Printf("Friend code collision, retrying... (attempt %d/%d)", i+1, maxRetries)
		}

		// Prepare response
		response := CreateUserResponse{
			ID:          userID.String(),
			FriendCode:  friendCode,
			DisplayName: displayName,
			APIKey:      apiKey,
		}

		// Send JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

// getUserHandler handles GET /user/{id} and GET /user/{friend_code} requests
func getUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract identifier from URL path parameter (Go 1.22+)
		identifier := r.PathValue("id")

		if identifier == "" {
			http.Error(w, "User ID or friend code is required", http.StatusBadRequest)
			return
		}

		// Try to determine if it's a UUID or friend code
		var id, displayName, friendCode string
		var err error

		// Check if it's a valid UUID format
		if _, uuidErr := uuid.Parse(identifier); uuidErr == nil {
			// It's a UUID - query by ID
			err = db.QueryRow(`
				SELECT id, COALESCE(display_name, ''), friend_code
				FROM "user"
				WHERE id = $1
			`, identifier).Scan(&id, &displayName, &friendCode)
		} else {
			// Not a UUID - treat as friend code
			// Friend codes are in format XXXX-XXXX (9 characters including dash)
			err = db.QueryRow(`
				SELECT id, COALESCE(display_name, ''), friend_code
				FROM "user"
				WHERE friend_code = $1
			`, identifier).Scan(&id, &displayName, &friendCode)
		}

		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		if err != nil {
			log.Printf("Failed to fetch user: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Prepare response
		response := GetUserResponse{
			ID:          id,
			DisplayName: displayName,
			FriendCode:  friendCode,
		}

		// Send JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

// generateAPIKey generates a random 256-bit key and returns it as a base64 string
func generateAPIKey() (string, error) {
	// 256 bits = 32 bytes
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// hashAPIKey hashes an API key using SHA-256
func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return fmt.Sprintf("%x", hash)
}

// generateRandomDisplayName creates a random "Adjective Noun" display name
func generateRandomDisplayName() (string, error) {
	// Generate random index for adjective
	adjIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(adjectives))))
	if err != nil {
		return "", err
	}

	// Generate random index for noun
	nounIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(nouns))))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s %s", adjectives[adjIndex.Int64()], nouns[nounIndex.Int64()]), nil
}

// generateFriendCode creates a random 8-character alphanumeric friend code in format XXXX-XXXX
func generateFriendCode() (string, error) {
	const charset = "ABCDEFGHJKMNPQRSTUVWXYZ123456789"
	const codeLength = 8

	code := make([]byte, codeLength)
	for i := range codeLength {
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		code[i] = charset[randomIndex.Int64()]
	}

	// Insert dash in the middle: XXXX-XXXX
	return fmt.Sprintf("%s-%s", string(code[:4]), string(code[4:])), nil
}

// isDuplicateKeyError checks if the error is a PostgreSQL unique constraint violation
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// PostgreSQL duplicate key error code is 23505
	errMsg := err.Error()
	return strings.Contains(errMsg, "duplicate key") ||
		strings.Contains(errMsg, "unique constraint") ||
		strings.Contains(errMsg, "23505")
}

// updateDisplayNameHandler handles PUT /user/{id}/name requests
// Requires authentication via authMiddleware
func updateDisplayNameHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get authenticated user ID from context
		authenticatedUserID, ok := GetUserID(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Extract target user ID from URL path parameter (Go 1.22+)
		targetUserID := r.PathValue("id")

		if targetUserID == "" {
			http.Error(w, "User ID is required", http.StatusBadRequest)
			return
		}

		// Validate UUID format
		if _, err := uuid.Parse(targetUserID); err != nil {
			http.Error(w, "Invalid user ID format", http.StatusBadRequest)
			return
		}

		// Verify the authenticated user is updating their own name
		if authenticatedUserID != targetUserID {
			http.Error(w, "Forbidden: You can only update your own display name", http.StatusForbidden)
			return
		}

		// Parse request body
		var req UpdateDisplayNameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Validate display name
		newDisplayName := strings.TrimSpace(req.DisplayName)
		if newDisplayName == "" {
			http.Error(w, "Display name cannot be empty", http.StatusBadRequest)
			return
		}

		if len(newDisplayName) > 64 {
			http.Error(w, "Display name must be 64 characters or less", http.StatusBadRequest)
			return
		}

		// Update display name in database (sets display_name_pending)
		_, err := db.Exec(`
			UPDATE "user"
			SET display_name_pending = $1,
			    display_name_status = 0
			WHERE id = $2
		`, newDisplayName, authenticatedUserID)

		if err != nil {
			log.Printf("Failed to update display name: %v", err)
			http.Error(w, "Failed to update display name", http.StatusInternalServerError)
			return
		}

		// Prepare response
		response := UpdateDisplayNameResponse{
			ID:          authenticatedUserID,
			DisplayName: newDisplayName,
			Message:     "Display name updated and pending approval",
		}

		// Send JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

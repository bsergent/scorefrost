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
	"regexp"
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

// User represents the JSON response for GET /user/{id}
type User struct {
	ApiResponse
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	FriendCode  string `json:"friend_code"`
}

// UserFull represents detailed user information returned by login
type UserFull struct {
	User
	DateTimeCreatedUTC string `json:"date_time_created_utc"`
	DateTimeActiveUTC  string `json:"date_time_active_utc"`
	GameVersion        string `json:"game_version"`
	PlayTimeMs         int    `json:"play_time_ms"`
	APIKey             string `json:"api_key,omitempty"` // Only included for new users
}

// LoginRequest represents the JSON request body for POST {APIBasePath}/user (login/create)
type LoginRequest struct {
	GameID      string `json:"game_id"`
	GameVersion string `json:"game_version"`
}

// UpdateDisplayNameRequest represents the JSON request body for PUT /user/{id}/name
type UpdateDisplayNameRequest struct {
	DisplayName string `json:"display_name"`
}

// loginUserHandler handles POST {APIBasePath}/user requests (login/create)
func loginUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Validate required fields
		if strings.TrimSpace(req.GameID) == "" {
			http.Error(w, "game_id is required", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.GameVersion) == "" {
			http.Error(w, "game_version is required", http.StatusBadRequest)
			return
		}

		// Get Authorization header
		auth := r.Header.Get("Authorization")
		var apiKey string
		var isNewUser bool

		// Check if API key is provided
		if auth != "" && strings.HasPrefix(auth, "Bearer ") {
			apiKey = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		}

		var userFull *UserFull
		var err error

		if apiKey == "" {
			// No API key provided - create new user
			userFull, err = createNewUser(db, req.GameVersion)
			if err != nil {
				log.Printf("Failed to create new user: %v", err)
				http.Error(w, "Failed to create user", http.StatusInternalServerError)
				return
			}
			isNewUser = true
		} else {
			// API key provided - authenticate existing user or return error
			userFull, err = authenticateUser(db, apiKey)
			if err != nil {
				if err.Error() == "user not found" {
					log.Printf("Authentication failed: Invalid API key from IP %s", getIPAddress(r))
					http.Error(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
					return
				}
				log.Printf("Authentication error: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		// Update user's active time
		if err := updateUserActiveTime(db, userFull.ID, req.GameVersion); err != nil {
			log.Printf("Failed to update user active time: %v", err)
			// Don't fail the request, just log the error
		}

		// Send JSON response
		w.Header().Set("Content-Type", "application/json")
		statusCode := http.StatusOK
		if isNewUser {
			statusCode = http.StatusCreated
		}
		w.WriteHeader(statusCode)
		if err := json.NewEncoder(w).Encode(userFull); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

// getUserHandler handles GET /user/{id} and GET /user/{friend_code} requests
func getUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract identifier from URL path parameter (Go 1.22+)
		identifier := r.PathValue("user_id")

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
		response := User{
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

		// Parse request body
		var req UpdateDisplayNameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Validate display name length
		newDisplayName := strings.TrimSpace(req.DisplayName)
		const minLength = 3
		const maxLength = 32
		if len(newDisplayName) < minLength || len(newDisplayName) > maxLength {
			http.Error(w,
				fmt.Sprintf("Display name must be between %d and %d characters", minLength, maxLength),
				http.StatusBadRequest)
			return
		}

		// Validate display name characters
		validChars := regexp.MustCompile(`^[a-zA-Z0-9 \-_]+$`)
		if !validChars.MatchString(newDisplayName) {
			http.Error(w,
				"Display name can only contain letters, numbers, spaces, hyphens, and underscores",
				http.StatusBadRequest)
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
			http.Error(w,
				"Failed to update display name",
				http.StatusInternalServerError)
			return
		}

		// Prepare response
		response := User{
			ApiResponse: ApiResponse{
				Message: "Display name updated and pending approval",
			},
			ID:          authenticatedUserID,
			DisplayName: newDisplayName,
		}

		// Send JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

// Helper function to create a new user and return UserFull details
func createNewUser(db *sql.DB, gameVersion string) (*UserFull, error) {
	// Generate new UUID for the user
	userID := uuid.New()

	// Generate a random 256-bit API key (32 bytes)
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Hash the API key for storage
	apiKeyHash := hashAPIKey(apiKey)

	// Generate random display name
	displayName, err := generateRandomDisplayName()
	if err != nil {
		return nil, fmt.Errorf("failed to generate display name: %w", err)
	}

	// Generate a unique friend code with retry logic
	const maxRetries = 10
	var friendCode string
	var returnedID string

	for i := range maxRetries {
		// Generate friend code
		friendCode, err = generateFriendCode()
		if err != nil {
			return nil, fmt.Errorf("failed to generate friend code: %w", err)
		}

		// Try to insert user into database
		err = db.QueryRow(
			"SELECT create_user($1, $2, $3, $4, $5)",
			userID.String(),
			friendCode,
			displayName,
			apiKeyHash,
			gameVersion,
		).Scan(&returnedID)

		// If successful, break out of retry loop
		if err == nil {
			break
		}

		// Check if error is due to duplicate friend code
		if !isDuplicateKeyError(err) {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}

		// If last retry, return error
		if i == maxRetries-1 {
			return nil, fmt.Errorf("failed to generate unique friend code after %d attempts", maxRetries)
		}
	}

	// Create UserFull response with the new API key
	return fetchUserFullObject(db, userID.String(), apiKey)
}

// Helper function to authenticate existing user and return UserFull details
func authenticateUser(db *sql.DB, apiKey string) (*UserFull, error) {
	// Hash the provided API key
	hashedKey := hashAPIKey(apiKey)

	// Look up user by API key hash
	var userID string
	err := db.QueryRow(`
		SELECT id
		FROM "user"
		WHERE api_key_hash = $1
	`, hashedKey).Scan(&userID)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Create UserFull response without API key
	return fetchUserFullObject(db, userID, "")
}

// Helper function to create UserFull struct with calculated play time
func fetchUserFullObject(db *sql.DB, userID, apiKey string) (*UserFull, error) {
	// Query user details with calculated play time
	var createdTime, activeTime, gameVersion, displayName, friendCode sql.NullString
	var playTime int

	err := db.QueryRow(`
		SELECT 
			date_time_created_utc,
			COALESCE(date_time_active_utc, date_time_created_utc),
			COALESCE(game_version, ''),
			COALESCE(display_name, ''),
			friend_code,
			get_user_play_time_ms($1)
		FROM "user"
		WHERE id = $1
	`, userID).Scan(&createdTime, &activeTime, &gameVersion, &displayName, &friendCode, &playTime)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch user details: %w", err)
	}

	userFull := &UserFull{
		User: User{
			ID:          userID,
			FriendCode:  friendCode.String,
			DisplayName: displayName.String,
		},
		DateTimeCreatedUTC: createdTime.String,
		DateTimeActiveUTC:  activeTime.String,
		GameVersion:        gameVersion.String,
		PlayTimeMs:         playTime,
	}

	// Include API key only for new users
	if apiKey != "" {
		userFull.APIKey = apiKey
	}

	return userFull, nil
}

// Helper function to update user's active time using stored procedure
func updateUserActiveTime(db *sql.DB, userID, gameVersion string) error {
	_, err := db.Exec("SELECT touch_user_active_time($1, $2)", userID, gameVersion)
	return err
}

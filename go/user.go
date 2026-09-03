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

// User represents the base user payload used across responses.
type User struct {
	ApiResponse
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"display_name"`
	FriendCode  string `json:"friend_code"`
}

// AuthenticatedUser represents the private user payload returned by POST /user.
// It includes the internal user ID, which is intentionally private outside login flows.
type AuthenticatedUser struct {
	User
	ID string `json:"id"`
}

// UserFull represents detailed user information returned by login
type UserFull struct {
	AuthenticatedUser
	DateTimeCreatedUTC string `json:"date_time_created_utc"`
	DateTimeActiveUTC  string `json:"date_time_active_utc"`
	GameVersion        string `json:"game_version"`
	PlayTimeMs         int    `json:"play_time_ms"`
	APIKey             string `json:"api_key,omitempty"` // Only included for new users
}

// LoginRequest represents the JSON request body for POST {APIBasePath}/user (login/create)
type LoginRequest struct {
	GameID      string  `json:"game_id"`
	GameVersion string  `json:"game_version"`
	UserID      *string `json:"user_id,omitempty"`
}

// UpdateDisplayNameRequest represents the JSON request body for PUT /user/name
type UpdateDisplayNameRequest struct {
	DisplayName string `json:"display_name"`
}

// loginUserHandler handles POST {APIBasePath}/user requests (login/create)
// Create new user - API key must be empty or omitted, user ID is ignored
// - API key must be empty or omitted
// - User ID is ignored
// Reclaim lost user
// - User ID must be provided. Must be valid UUID. Cannot exist in database.
// - API key must be provided but will be ignored. A new API key will be generated and returned in the response.
// Authenticate existing user
// - User ID must be provided. Must be valid UUID. Must exist in database.
// - API key must be provided and must match the stored hash for the user ID.
func loginUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Validate game information fields
		if strings.TrimSpace(req.GameID) == "" {
			http.Error(w, "game_id is required", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.GameVersion) == "" {
			http.Error(w, "game_version is required", http.StatusBadRequest)
			return
		}

		// Get api key from authorization header
		auth := r.Header.Get("Authorization")
		var apiKey string
		if auth != "" && strings.HasPrefix(auth, "Bearer ") {
			apiKey = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		}

		// Helper function to send JSON user response and update active time
		sendUserResponse := func(user *UserFull, statusCode int) {
			// Update user's active time
			if err := updateUserActiveTime(db, user.ID, req.GameVersion); err != nil {
				log.Printf("Failed to update user active time: %v", err)
				// Don't fail the request, just log the error
			}

			// Send JSON response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			if err := json.NewEncoder(w).Encode(user); err != nil {
				log.Printf("Failed to encode response: %v", err)
			}
		}

		var userFull *UserFull
		var err error

		if apiKey == "" {
			// Create new user with random UUID and random API key
			userFull, err = createNewUser(db, req.GameVersion)
			if err != nil {
				log.Printf("Failed to create new user: %v", err)
				http.Error(w, "Failed to create user", http.StatusInternalServerError)
				return
			}

			sendUserResponse(userFull, http.StatusCreated)
			log.Printf("Created user: %s (%s)", userFull.DisplayName, userFull.FriendCode)
			return
		}

		if req.UserID == nil || strings.TrimSpace(*req.UserID) == "" {
			http.Error(w, "user_id is required", http.StatusBadRequest)
			return
		}

		// Validate user ID (must be included, must be valid UUID)
		requestedUserID := strings.TrimSpace(*req.UserID)
		if _, err := uuid.Parse(requestedUserID); err != nil {
			http.Error(w, "Invalid user_id format. Must be a valid UUID", http.StatusBadRequest)
			return
		}

		// Authenticate with the provided user ID and API key
		userFull, err = authenticateUser(db, requestedUserID, apiKey)
		if err == nil {
			sendUserResponse(userFull, http.StatusOK)
			log.Printf("Authenticated user: %s (%s)", userFull.DisplayName, userFull.FriendCode)
			return
		}

		// User not in database, reclaim with given UUID and new API key
		if err.Error() == "user not found" {
			userFull, err = reclaimUserWithID(db, requestedUserID, req.GameVersion)

			if err != nil {
				// Unknown error
				log.Printf("Failed to reclaim user: %v", err)
				http.Error(w, "Failed to reclaim user", http.StatusInternalServerError)
				return
			}

			// Reclaimed lost user
			sendUserResponse(userFull, http.StatusCreated)
			log.Printf("Reclaimed user: %s (%s)", userFull.DisplayName, userFull.FriendCode)
			return
		}

		// Failed authentication
		if err.Error() == "invalid api key" {
			log.Printf("Invalid API key for user %s from IP %s", requestedUserID, getIPAddress(r))
			http.Error(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
			return
		}

		// Unknown error
		log.Printf("Authentication error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// getUserHandler handles GET /user/{friend_code} requests.
func getUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract friend code from URL path parameter (Go 1.22+)
		friendCode := strings.TrimSpace(r.PathValue("friend_code"))

		if friendCode == "" {
			http.Error(w, "friend_code is required", http.StatusBadRequest)
			return
		}

		// Friend codes are public identifiers in format XXXX-XXXX.
		var displayName string
		err := db.QueryRow(`
			SELECT COALESCE(display_name, '')
			FROM "user"
			WHERE friend_code = $1
		`, friendCode).Scan(&displayName)

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

// tryCreateUser repeatedly attempts to create a user until the database accepts
// the generated friend code or the retry limit is reached.
func tryCreateUser(db *sql.DB, userID, apiKey, gameVersion string) (*UserFull, error) {
	const maxRetries = 10

	// Generate the API key hash once per user creation attempt sequence.
	// The same API key is reused across retries while friend code changes.
	apiKeyHash := hashAPIKey(apiKey)

	// Generate random display name once per sequence.
	// It's okay if the display name is not unique.
	displayName, err := generateRandomDisplayName()
	if err != nil {
		return nil, fmt.Errorf("failed to generate display name: %w", err)
	}

	for range maxRetries {
		friendCode, err := generateFriendCode()
		if err != nil {
			return nil, fmt.Errorf("failed to generate friend code: %w", err)
		}

		var returnedID string
		err = db.QueryRow(
			"SELECT create_user($1, $2, $3, $4, $5)",
			userID,
			friendCode,
			displayName,
			apiKeyHash,
			gameVersion,
		).Scan(&returnedID)
		if err == nil {
			return fetchUserFullObject(db, userID, apiKey)
		}

		if !isDuplicateKeyError(err) {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	return nil, fmt.Errorf("failed to create user after %d attempts", maxRetries)
}

// isDuplicateKeyError checks if the error is a PostgreSQL unique constraint violation.
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

// updateDisplayNameHandler handles PUT /user/name requests
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
		friendCode, _ := GetFriendCode(r)

		response := User{
			ApiResponse: ApiResponse{
				Message: "Display name updated and pending approval",
			},
			DisplayName: newDisplayName,
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

// Helper function to create a new user and return UserFull details
func createNewUser(db *sql.DB, gameVersion string) (*UserFull, error) {
	// Generate new UUID for the user
	userID := uuid.New()

	// Generate a random 256-bit API key (32 bytes)
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Try to create the user, retrying only when the database reports a duplicate
	// friend code.
	return tryCreateUser(db, userID.String(), apiKey, gameVersion)
}

// Helper function to reclaim a user with a given ID (for disaster recovery)
func reclaimUserWithID(db *sql.DB, userID, gameVersion string) (*UserFull, error) {
	// Generate a new API key
	// We intentionally do generate a new API key instead of reusing the provided one
	// as we cannot guarantee that the provided key is cryptographically random. By
	// generating a new key here, we maintain the invariant that every api key in our
	// database is cryptographically random. The user will receive the new API key in
	// the response and can use it for future authentication.
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Try to reclaim the user, retrying only when the database reports a duplicate
	// friend code.
	return tryCreateUser(db, userID, apiKey, gameVersion)
}

// Helper function to authenticate user by ID and API key
func authenticateUser(db *sql.DB, userID string, apiKey string) (*UserFull, error) {
	// Hash the provided API key
	hashedKey := hashAPIKey(apiKey)

	// Look up user by ID and verify API key
	var storedHash string
	err := db.QueryRow(`
		SELECT api_key_hash
		FROM "user"
		WHERE id = $1
	`, userID).Scan(&storedHash)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Check if the provided API key matches
	if hashedKey != storedHash {
		return nil, fmt.Errorf("invalid api key")
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
		AuthenticatedUser: AuthenticatedUser{
			User: User{
				FriendCode:  friendCode.String,
				DisplayName: displayName.String,
			},
			ID: userID,
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

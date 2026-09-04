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

// Private user identifier, should not be exposed outside of login flows. Use FriendCode for public identifiers.
// Type alias so we can scan directly into a uuid.UUID from the database
type UserID = uuid.UUID

// Friend codes are public identifiers in format ABCD-EF01.
type FriendCode string

// User represents the base user payload used across responses.
type User struct {
	ApiResponse
	ID          *UserID     `json:"id,omitempty"`
	DisplayName DisplayName `json:"display_name"`
	FriendCode  FriendCode  `json:"friend_code"`
}

// AuthenticatedUser represents the private user payload returned by POST /user.
// It includes the internal user ID, which is intentionally private outside login flows.
type AuthenticatedUser struct {
	User
	ID UserID `json:"id"`
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
	GameID      string       `json:"game_id"`
	GameVersion string       `json:"game_version"`
	UserID      *UserID      `json:"user_id,omitempty"`
	FriendCode  *FriendCode  `json:"friend_code,omitempty"`
	DisplayName *DisplayName `json:"display_name,omitempty"`
}

// UpdateDisplayNameRequest represents the JSON request body for PUT /user/name
type UpdateDisplayNameRequest struct {
	DisplayName DisplayName `json:"display_name"`
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

		if req.UserID == nil {
			http.Error(w, "user_id is required", http.StatusBadRequest)
			return
		}

		// Authenticate with the provided user ID and API key
		userFull, err = authenticateUser(db, *req.UserID, apiKey)
		if err == nil {
			sendUserResponse(userFull, http.StatusOK)
			log.Printf("Authenticated user: %s (%s)", userFull.DisplayName, userFull.FriendCode)
			return
		}

		// User not in database, reclaim with given UUID and new API key
		if err.Error() == "user not found" {
			userFull, err = reclaimUserWithID(db, *req.UserID, req.GameVersion, req.FriendCode)

			if err != nil {
				// Unknown error
				log.Printf("Failed to reclaim user: %v", err)
				http.Error(w, "Failed to reclaim user", http.StatusInternalServerError)
				return
			}

			// Request display name, if provided
			if req.DisplayName != nil {
				if requestedDisplayName, err := sanitizeDisplayName(*req.DisplayName); err == nil {
					if err := requestDisplayName(db, userFull.ID, requestedDisplayName); err != nil {
						log.Printf("Failed to request display name during reclaim: %v", err)
					}
				}
			}

			// Reclaimed lost user
			sendUserResponse(userFull, http.StatusCreated)
			log.Printf("Reclaimed user: %s (%s)", userFull.DisplayName, userFull.FriendCode)
			return
		}

		// Failed authentication
		if err.Error() == "invalid api key" {
			log.Printf("Invalid API key for user %s from IP %s", *req.UserID, getIPAddress(r))
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
		friendCode := FriendCode(strings.TrimSpace(r.PathValue("friend_code")))

		if friendCode == "" {
			http.Error(w, "friend_code is required", http.StatusBadRequest)
			return
		}

		var displayName, err = getDisplayNameByFriendCode(db, friendCode)
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

// Definition of what a friend code must look like, e.g. ABCD-EF01
var friendCodeRegex = regexp.MustCompile(`^[A-Z0-9]{4}-[A-Z0-9]{4}$`)

// generateFriendCode creates a random 8-character alphanumeric friend code in format XXXX-XXXX
func generateFriendCode() (FriendCode, error) {
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
	return FriendCode(fmt.Sprintf("%s-%s", string(code[:4]), string(code[4:]))), nil
}

// tryCreateUser repeatedly attempts to create a user until the database accepts
// the generated friend code or the retry limit is reached.
func tryCreateUser(db *sql.DB, userID UserID, apiKey, gameVersion string, friendCode *FriendCode) (*UserFull, error) {
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

	if friendCode != nil {
		// Apply basic formatting
		*friendCode = FriendCode(strings.ToUpper(strings.TrimSpace(string(*friendCode))))

		// Invalidate provided friend code if it doesn't match the required format.
		if !friendCodeRegex.MatchString(string(*friendCode)) {
			friendCode = nil
		}
	}

	for range maxRetries {
		// Generate a new friend code if none pending
		if friendCode == nil {
			generatedFriendCode, genErr := generateFriendCode()
			if genErr != nil {
				return nil, fmt.Errorf("failed to generate friend code: %w", genErr)
			}
			friendCode = &generatedFriendCode
		}

		var returnedID string
		err = db.QueryRow(
			"SELECT create_user($1, $2, $3, $4, $5)",
			userID,
			*friendCode,
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

		// Reset friend code to generate a new one on the next iteration
		friendCode = nil
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

		newDisplayName, validationErr := sanitizeDisplayName(req.DisplayName)
		if validationErr != nil {
			http.Error(w, validationErr.Error(), http.StatusBadRequest)
			return
		}

		err := requestDisplayName(db, authenticatedUserID, newDisplayName)

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
	userID := UserID(uuid.New())

	// Generate a random 256-bit API key (32 bytes)
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Try to create the user, retrying only when the database reports a duplicate
	// friend code.
	return tryCreateUser(db, userID, apiKey, gameVersion, nil)
}

// Helper function to reclaim a user with a given ID (for disaster recovery)
func reclaimUserWithID(db *sql.DB, userID UserID, gameVersion string, requestedFriendCode *FriendCode) (*UserFull, error) {
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
	return tryCreateUser(db, userID, apiKey, gameVersion, requestedFriendCode)
}

// Helper function to authenticate user by ID and API key
func authenticateUser(db *sql.DB, userID UserID, apiKey string) (*UserFull, error) {
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
func fetchUserFullObject(db *sql.DB, userID UserID, apiKey string) (*UserFull, error) {
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
				FriendCode:  FriendCode(friendCode.String),
				DisplayName: DisplayName(displayName.String),
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
func updateUserActiveTime(db *sql.DB, userID UserID, gameVersion string) error {
	_, err := db.Exec("SELECT touch_user_active_time($1, $2)", userID, gameVersion)
	return err
}

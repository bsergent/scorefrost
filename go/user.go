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
	DisplayName string `json:"display_name"`
	APIKey      string `json:"api_key"`
}

// createUserHandler handles POST /user requests
func createUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept POST requests
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

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

		// Insert user into database
		var returnedID string
		err = db.QueryRow(
			"SELECT create_user($1, $2, $3)",
			userID.String(),
			displayName,
			apiKeyHash,
		).Scan(&returnedID)

		if err != nil {
			log.Printf("Failed to create user: %v", err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		// Prepare response
		response := CreateUserResponse{
			ID:          userID.String(),
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

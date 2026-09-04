package main

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// Word lists for random display name generation
// TODO Load these from a file or database instead of hardcoding
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

type DisplayName string
type DisplayNameStatus int

const (
	DisplayNameStatusPending DisplayNameStatus = iota
	DisplayNameStatusApproved
	DisplayNameStatusRejected
)

// PendingDisplayName represents a pending display name change
type PendingDisplayName struct {
	UserID             UserID            `json:"user_id"`
	FriendCode         FriendCode        `json:"friend_code"`
	CurrentDisplayName DisplayName       `json:"current_display_name"`
	PendingDisplayName DisplayName       `json:"pending_display_name"`
	DisplayNameStatus  DisplayNameStatus `json:"display_name_status"`
}

// Definition of what a display name must look like, e.g. "Spirited-Rival 67_"
var displayNameRegex = regexp.MustCompile(`^[a-zA-Z0-9 \-_]{3,32}$`)

func sanitizeDisplayName(displayName *DisplayName) (DisplayName, error) {
	if displayName == nil {
		return "", fmt.Errorf("Display name cannot be nil")
	}

	*displayName = DisplayName(strings.TrimSpace(string(*displayName)))

	if !displayNameRegex.MatchString(string(*displayName)) {
		return "", fmt.Errorf("Display name can only contain letters, numbers, spaces, hyphens, and underscores")
	}

	return DisplayName(*displayName), nil
}

// Get the display name for a user by their user ID
// Returns sql.ErrNoRows if the user does not exist
func getDisplayNameByUserID(db *sql.DB, userId UserID) (DisplayName, error) {
	var displayName DisplayName
	err := db.QueryRow(`
		SELECT COALESCE(display_name, '')
		FROM "user"
		WHERE id = $1
	`, userId).Scan(&displayName)
	return displayName, err
}

// Get the display name for a user by their friend code
// Returns sql.ErrNoRows if the user does not exist
func getDisplayNameByFriendCode(db *sql.DB, friendCode FriendCode) (DisplayName, error) {
	var displayName DisplayName
	err := db.QueryRow(`
		SELECT COALESCE(display_name, '')
		FROM "user"
		WHERE friend_code = $1
	`, friendCode).Scan(&displayName)
	return displayName, err
}

func getPendingDisplayNames(db *sql.DB) ([]PendingDisplayName, error) {
	// Call stored procedure to get all pending display names
	rows, err := db.Query(`SELECT * FROM get_pending_display_names()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Collect all pending names
	var pendingNames []PendingDisplayName
	for rows.Next() {
		var pending PendingDisplayName
		if err := rows.Scan(
			&pending.UserID,
			&pending.FriendCode,
			&pending.CurrentDisplayName,
			&pending.PendingDisplayName,
			&pending.DisplayNameStatus,
		); err != nil {
			log.Printf("Failed to scan pending display name: %v", err)
			continue
		}
		pendingNames = append(pendingNames, pending)
	}

	return pendingNames, rows.Err()
}

func requestDisplayName(db *sql.DB, userId UserID, displayName DisplayName) error {
	_, err := db.Exec(`
		UPDATE "user"
		SET display_name_pending = $1,
		    display_name_status = 0
		WHERE id = $2
	`, displayName, userId)
	return err
}

func approveDisplayName(db *sql.DB, userId UserID, status DisplayNameStatus) (DisplayName, error) {
	var finalDisplayName DisplayName
	var newStatus DisplayNameStatus
	err := db.QueryRow(`SELECT * FROM approve_display_name($1)`, userId).
		Scan(&finalDisplayName, &newStatus)
	return finalDisplayName, err
}

func rejectDisplayName(db *sql.DB, userId UserID) (DisplayName, error) {
	var finalDisplayName DisplayName
	var newStatus DisplayNameStatus
	err := db.QueryRow(`SELECT * FROM reject_display_name($1)`, userId).
		Scan(&finalDisplayName, &newStatus)
	return finalDisplayName, err
}

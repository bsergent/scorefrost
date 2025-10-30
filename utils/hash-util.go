package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
)

// This utility helps calculate the correct solution hash for testing
func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run hash-util.go <solution_base64> <salt>")
		fmt.Println("Example: go run hash-util.go SGVsbG8gV29ybGQ= your_secret_salt_change_in_production")
		os.Exit(1)
	}

	solution := os.Args[1]
	salt := os.Args[2]

	// Verify base64
	if _, err := base64.StdEncoding.DecodeString(solution); err != nil {
		fmt.Printf("Error: Invalid base64 solution: %v\n", err)
		os.Exit(1)
	}

	// Calculate hash
	saltedSolution := solution + salt
	hash := sha256.Sum256([]byte(saltedSolution))
	expectedHash := fmt.Sprintf("%x", hash)

	fmt.Printf("Solution (base64): %s\n", solution)
	fmt.Printf("Salt: %s\n", salt)
	fmt.Printf("Salted string: %s\n", saltedSolution)
	fmt.Printf("SHA256 hash: %s\n", expectedHash)
}
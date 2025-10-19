package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

type Server struct {
	db     *sql.DB
	router *http.ServeMux
}

type Score struct {
	ID        int       `json:"id"`
	PlayerID  string    `json:"player_id"`
	GameID    string    `json:"game_id"`
	Score     int       `json:"score"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	// Get database connection string from environment
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "scorefrost")
	dbPassword := getEnv("DB_PASSWORD", "scorefrost")
	dbName := getEnv("DB_NAME", "scorefrost")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	// Connect to database with retries
	var db *sql.DB
	var err error
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		db, err = sql.Open("postgres", connStr)
		if err != nil {
			log.Printf("Failed to open database connection: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		err = db.Ping()
		if err != nil {
			log.Printf("Failed to ping database (attempt %d/%d): %v", i+1, maxRetries, err)
			time.Sleep(2 * time.Second)
			continue
		}

		log.Println("Successfully connected to database")
		break
	}

	if err != nil {
		log.Fatalf("Could not connect to database after %d attempts: %v", maxRetries, err)
	}
	defer db.Close()

	// Initialize database schema
	if err := initDB(db); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create server
	server := &Server{
		db:     db,
		router: http.NewServeMux(),
	}

	// Register routes
	server.routes()

	// Get server port
	port := getEnv("PORT", "8080")
	addr := ":" + port

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on %s", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func (s *Server) routes() {
	s.router.HandleFunc("/health", s.handleHealth())
	s.router.HandleFunc("/api/scores/", s.handleScoreByID())
	s.router.HandleFunc("/api/scores", s.handleScores())
	s.router.HandleFunc("/api/leaderboard", s.handleLeaderboard())
}

func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check database connection
		if err := s.db.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "unhealthy",
				"error":  err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
		})
	}
}

func (s *Server) handleScores() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			s.getScores(w, r)
		case http.MethodPost:
			s.createScore(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (s *Server) getScores(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Query().Get("game_id")
	playerID := r.URL.Query().Get("player_id")

	query := "SELECT id, player_id, game_id, score, created_at FROM scores WHERE 1=1"
	args := []interface{}{}
	argCount := 1

	if gameID != "" {
		query += fmt.Sprintf(" AND game_id = $%d", argCount)
		args = append(args, gameID)
		argCount++
	}

	if playerID != "" {
		query += fmt.Sprintf(" AND player_id = $%d", argCount)
		args = append(args, playerID)
		argCount++
	}

	query += " ORDER BY created_at DESC LIMIT 100"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	scores := []Score{}
	for rows.Next() {
		var score Score
		if err := rows.Scan(&score.ID, &score.PlayerID, &score.GameID, &score.Score, &score.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		scores = append(scores, score)
	}

	json.NewEncoder(w).Encode(scores)
}

func (s *Server) createScore(w http.ResponseWriter, r *http.Request) {
	var score Score
	if err := json.NewDecoder(r.Body).Decode(&score); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if score.PlayerID == "" || score.GameID == "" {
		http.Error(w, "player_id and game_id are required", http.StatusBadRequest)
		return
	}

	query := "INSERT INTO scores (player_id, game_id, score, created_at) VALUES ($1, $2, $3, $4) RETURNING id, created_at"
	err := s.db.QueryRow(query, score.PlayerID, score.GameID, score.Score, time.Now()).Scan(&score.ID, &score.CreatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(score)
}

func (s *Server) handleScoreByID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Extract ID from path - trim prefix and check for valid ID
		path := r.URL.Path
		prefix := "/api/scores/"
		if !strings.HasPrefix(path, prefix) {
			http.Error(w, "Invalid path", http.StatusNotFound)
			return
		}

		id := strings.TrimPrefix(path, prefix)
		if id == "" {
			http.Error(w, "Score ID is required", http.StatusBadRequest)
			return
		}

		scoreID, err := strconv.Atoi(id)
		if err != nil {
			http.Error(w, "Invalid score ID", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			s.getScoreByID(w, r, scoreID)
		case http.MethodDelete:
			s.deleteScore(w, r, scoreID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (s *Server) getScoreByID(w http.ResponseWriter, r *http.Request, id int) {
	var score Score
	query := "SELECT id, player_id, game_id, score, created_at FROM scores WHERE id = $1"
	err := s.db.QueryRow(query, id).Scan(&score.ID, &score.PlayerID, &score.GameID, &score.Score, &score.CreatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "Score not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(score)
}

func (s *Server) deleteScore(w http.ResponseWriter, r *http.Request, id int) {
	query := "DELETE FROM scores WHERE id = $1"
	result, err := s.db.Exec(query, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "Score not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLeaderboard() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		gameID := r.URL.Query().Get("game_id")
		if gameID == "" {
			http.Error(w, "game_id is required", http.StatusBadRequest)
			return
		}

		limitStr := r.URL.Query().Get("limit")
		limit := 10
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		query := `
			SELECT id, player_id, game_id, score, created_at
			FROM scores
			WHERE game_id = $1
			ORDER BY score DESC, created_at ASC
			LIMIT $2
		`

		rows, err := s.db.Query(query, gameID, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		scores := []Score{}
		for rows.Next() {
			var score Score
			if err := rows.Scan(&score.ID, &score.PlayerID, &score.GameID, &score.Score, &score.CreatedAt); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			scores = append(scores, score)
		}

		json.NewEncoder(w).Encode(scores)
	}
}

func initDB(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS scores (
		id SERIAL PRIMARY KEY,
		player_id VARCHAR(255) NOT NULL,
		game_id VARCHAR(255) NOT NULL,
		score INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_scores_game_id ON scores(game_id);
	CREATE INDEX IF NOT EXISTS idx_scores_player_id ON scores(player_id);
	CREATE INDEX IF NOT EXISTS idx_scores_game_score ON scores(game_id, score DESC);
	`

	_, err := db.Exec(schema)
	return err
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

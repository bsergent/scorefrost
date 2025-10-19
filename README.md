# scorefrost
Lightweight leaderboard and analytics solution for game jams.

## Overview

Scorefrost is a lightweight REST API server written in Go with PostgreSQL database support, designed to provide leaderboard and scoring functionality for game jams. The entire application runs as Docker containers for easy deployment.

## Features

- ✅ RESTful API with JSON responses
- ✅ PostgreSQL database with automatic schema initialization
- ✅ Docker and Docker Compose support
- ✅ Health check endpoint
- ✅ CRUD operations for scores
- ✅ Leaderboard queries
- ✅ Graceful shutdown handling

## Prerequisites

- Docker
- Docker Compose

## Quick Start

1. Clone the repository:
```bash
git clone https://github.com/bsergent/scorefrost.git
cd scorefrost
```

2. Start the services:
```bash
docker compose up --build
```

The API will be available at `http://localhost:8080`

3. To stop the services:
```bash
docker compose down
```

To also remove the database volume:
```bash
docker compose down -v
```

## API Endpoints

### Health Check
```bash
GET /health
```

Returns the health status of the API and database connection.

**Example:**
```bash
curl http://localhost:8080/health
```

### Create Score
```bash
POST /api/scores
Content-Type: application/json

{
  "player_id": "player123",
  "game_id": "game456",
  "score": 1000
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/api/scores \
  -H "Content-Type: application/json" \
  -d '{"player_id":"player123","game_id":"game456","score":1000}'
```

### Get Scores
```bash
GET /api/scores?game_id=game456&player_id=player123
```

Query parameters:
- `game_id` (optional): Filter by game ID
- `player_id` (optional): Filter by player ID

**Example:**
```bash
curl http://localhost:8080/api/scores?game_id=game456
```

### Get Score by ID
```bash
GET /api/scores/{id}
```

**Example:**
```bash
curl http://localhost:8080/api/scores/1
```

### Delete Score
```bash
DELETE /api/scores/{id}
```

**Example:**
```bash
curl -X DELETE http://localhost:8080/api/scores/1
```

### Get Leaderboard
```bash
GET /api/leaderboard?game_id=game456&limit=10
```

Query parameters:
- `game_id` (required): Game ID to get leaderboard for
- `limit` (optional): Number of results (default: 10, max: 100)

**Example:**
```bash
curl http://localhost:8080/api/leaderboard?game_id=game456&limit=10
```

## Development

### Running Locally (without Docker)

1. Start PostgreSQL:
```bash
docker run -d \
  -e POSTGRES_USER=scorefrost \
  -e POSTGRES_PASSWORD=scorefrost \
  -e POSTGRES_DB=scorefrost \
  -p 5432:5432 \
  postgres:16-alpine
```

2. Run the application:
```bash
go run main.go
```

### Building the Docker Image
```bash
docker build -t scorefrost .
```

### Environment Variables

The API supports the following environment variables:

- `DB_HOST`: Database host (default: localhost)
- `DB_PORT`: Database port (default: 5432)
- `DB_USER`: Database user (default: scorefrost)
- `DB_PASSWORD`: Database password (default: scorefrost)
- `DB_NAME`: Database name (default: scorefrost)
- `PORT`: API server port (default: 8080)

## Database Schema

The application automatically creates the following table on startup:

```sql
CREATE TABLE scores (
    id SERIAL PRIMARY KEY,
    player_id VARCHAR(255) NOT NULL,
    game_id VARCHAR(255) NOT NULL,
    score INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

Indexes are created on `game_id`, `player_id`, and `(game_id, score)` for optimized queries.

## License

MIT

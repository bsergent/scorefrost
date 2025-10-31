# ScoreFrost
Lightweight leaderboard and analytics solution for game jams.

## Setup

### Environment Variables
Copy `.env.example` to `.env` and configure:
```bash
cp .env.example .env
```

Key variables:
- `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDB` - PostgreSQL connection
- `API_PORT` - API server port (default: 8080)
- `DEV_API_KEY` - Admin authentication key for dev user
- `SOLUTION_SALT` - Secret salt for solution hash verification

**Important:** Change `DEV_API_KEY` and `SOLUTION_SALT` in production!

## Developing Locally
Only start database and run API locally from the root directory:
`docker-compose up -d db`
`go run ./go`

Stop database
`docker-compose down`

## Deploying
`docker-compose up --build`

`docker build -t scoreforst:dev .`

`docker-compose up -d` Run the volume with a detached head.

`docker-compose up --build` Rebuild the GoAPI and run both the database and API.

`docker-compose down` Shut database and API down.

`docker ps` Check for running containers.

`docker stop <id>`

`docker system prune`

## Testing

### Unit Tests (Fast)
Standard Go unit tests that don't require external dependencies:
```bash
go test ./go -v
```

### Integration Tests (Full Database)
Integration tests that use Docker database for complete API testing:
```bash
# Start database first
docker-compose up -d db

# Run integration tests
go test -tags=integration ./go -v
```

**From VS Code:**
- **Ctrl+Shift+P** → "Tasks: Run Task" → "Run Unit Tests" (fast, no setup)
- **Ctrl+Shift+P** → "Tasks: Run Task" → "Run Integration Tests" (full database testing)
- **Ctrl+Shift+P** → "Tasks: Run Task" → "Run All Tests" (runs both unit and integration)

**Debug Support:**
- **F5** → "Debug Unit Tests" (debug unit tests with breakpoints)
- **F5** → "Debug Integration Tests" (debug integration tests with database)

### Test Structure
Following Go conventions, tests are located alongside the code:
```
go/
├── main.go
├── main_test.go              # Unit tests for core functions
├── routes.go                 # Shared route setup (used by main + tests)
├── integration_test.go       # Integration test setup (//go:build integration)
├── integration_utils.go      # Integration test utilities (//go:build integration)
├── integration_api_test.go   # Full API integration tests (//go:build integration)
└── test_setup.go            # Test-specific route setup (//go:build integration)
```

**Test Coverage:**
- ✅ Unit tests: Health endpoint, rate limiting, API key generation, display names
- ✅ Integration tests: User creation, score submission, best scores retrieval, multiple levels, version detection
- ✅ Database integration: Real PostgreSQL with cleanup between tests
- ✅ HTTP testing: Uses httptest.NewServer for in-process testing

## API Endpoints

### Public Endpoints
- `GET /health` - Health check
- `POST /user` - Create new user, returns API key (store this!)
- `GET /user/{id}` - Get user info by UUID or friend code

### Authenticated Endpoints
Requires `Authorization: Bearer {api_key}` header

- `PUT /user/{id}/name` - Update display name (sets pending, requires approval)
- `POST /score/submit` - Submit solution with scores for a level
- `GET /score/best` - Get best scores for specified levels

#### Score Submission
Submit a completed level solution with multiple score types.

**Request:**
```json
POST /score/submit
Authorization: Bearer {user_api_key}
Content-Type: application/json

{
  "level_id": "level_001",
  "level_version": 1,
  "game_version": "1.0.0",
  "solution": "SGVsbG8gV29ybGQ=",
  "solution_hash": "47b1ccfc46209749ca88f8ee4556ef7de42cbd94e297a79b3f8efd04ce663588",
  "scores": {
    "time_ms": 12500,
    "striping": 85,
    "fuel": 750,
    "stars": 3
  }
}
```

**Response:**
```json
{
  "success": true,
  "solution_id": 42,
  "message": "Score submitted successfully"
}
```

**Score Submission Fields:**
- `level_id` (string) - Unique identifier for the level
- `level_version` (int) - Version number of the level
- `game_version` (string) - Version of the game client
- `solution` (string) - Base64 encoded solution data
- `solution_hash` (string) - SHA256 hash of solution + secret salt for integrity verification
- `scores` (object) - Map of score type to score value

**Available Score Types:**
- `time_ms` - Completion time in milliseconds (lower is better)
- `striping` - Coverage/striping percentage (higher is better)
- `fuel` - Fuel consumption (lower is better)
- `stars` - Star rating achieved (higher is better)

**Security:**
- Solution integrity is verified using a salted hash
- All operations are atomic via stored procedure
- Invalid score types are rejected

**Solution Hash Calculation:**
```javascript
// Client-side (example)
const solutionBase64 = btoa(solutionData); // Base64 encode solution
const saltedSolution = solutionBase64 + SECRET_SALT;
const solutionHash = sha256(saltedSolution); // SHA256 hash
```

```bash
# Utility for testing hash calculation
go run utils/hash-util.go "SGVsbG8gV29ybGQ=" "your_secret_salt"
```

#### Best Scores Retrieval
Get the best scores for specified levels within a given scope.

**Request:**
```bash
GET /score/best?levels=level_001.1,level_002&scope=global
Authorization: Bearer {user_api_key}
```

**Response:**
```json
{
  "scores": [
    {
      "level_id": "level_001",
      "level_version": 1,
      "score_type": "time_ms",
      "best_score": 12500,
      "user_id": "uuid-here",
      "display_name": "PlayerName",
      "friend_code": "ABCD-1234"
    }
  ],
  "count": 1,
  "scope": "global"
}
```

**Parameters:**
- `levels` - Comma-separated list of level specifications:
  - `level_001.1` - Specific level and version
  - `level_001` - Latest version of level (automatically determined)
  - Mixed: `level_001,level_002.1,level_003.2`
- `scope` - Score scope (defaults to `global`):
  - `personal` - User's own best scores
  - `friends` - Best among user's friends (not yet implemented)
  - `regional` - Regional leaderboards (not yet implemented)
  - `global` - Worldwide best scores

### Admin Endpoints
Requires dev user authentication (`DEV_API_KEY`)

- `GET /admin/names` - List all pending display name changes
- `PUT /admin/names/{user_id}` - Approve/reject display name change
  - Request body: `{"approve": true}` or `{"approve": false}`

### Default Users
- **Anonymous** (`00000000-0000-0000-0000-000000000000`) - Friend code: `0000-0000`
- **Dev** (`00000000-0000-0000-0000-000000000001`) - Friend code: `0000-0001`
  - Admin access for display name moderation
  - Authenticate using `DEV_API_KEY` environment variable


# ScoreFrost
Lightweight leaderboard and analytics solution for game jams.

OpenAPI Specification: [openapi.yaml](openapi.yaml)

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

ScoreFrost follows Go testing conventions with well-organized test files:

### Test Structure
Tests are organized by source file following Go conventions:
```
go/
├── main.go → main_test.go                    # Health endpoint, route registration
├── auth.go → auth_test.go                    # IP extraction, context functions  
├── user.go → user_test.go                    # API keys, display names, friend codes
├── ratelimit.go → ratelimit_test.go         # Rate limiting functionality
├── score.go → score_test.go                 # Level parsing, response structures
├── admin.go → admin_test.go                 # Admin middleware, display name management
├── integration_api_test.go                  # Full API integration tests
├── integration_test.go                      # Integration test setup
└── integration_utils.go                     # Integration test helpers
```

### Unit Tests (Fast, No Dependencies)
Standard Go unit tests that don't require external services:
```bash
# Run all unit tests
go test ./go -v

# Run specific test file
go test ./go -run TestHealth

# Run with coverage
go test ./go -cover

# Run short tests only (excludes slow tests)
go test ./go -short
```

### Integration Tests (Full Database)
Integration tests that use Docker database for complete API testing:
```bash
# Start database first
docker-compose up -d db

# Run integration tests
go test -tags=integration ./go -v

# Run specific integration test
go test -tags=integration ./go -run TestIntegrationUserCreation
```

**From VS Code:**
- **Ctrl+Shift+P** → "Tasks: Run Task" → "Run Unit Tests" (fast, no setup)
- **Ctrl+Shift+P** → "Tasks: Run Task" → "Run Integration Tests" (full database testing)
- **Ctrl+Shift+P** → "Tasks: Run Task" → "Run All Tests" (runs both unit and integration)

**Debug Support:**
- **F5** → "Debug Unit Tests" (debug unit tests with breakpoints)
- **F5** → "Debug Integration Tests" (debug integration tests with database)

### Test Coverage Summary
- ✅ **44 Unit Tests**: All core functions covered without external dependencies
- ✅ **8 Integration Tests**: Complete API workflows with real database
- ✅ **Database Integration**: Real PostgreSQL with cleanup between tests
- ✅ **HTTP Testing**: Uses httptest.NewServer for in-process testing
- ✅ **Authentication**: API key generation, hashing, and validation
- ✅ **Rate Limiting**: IP-based request throttling
- ✅ **Score Processing**: Level parsing, leaderboards, and score submission
- ✅ **User Management**: User creation, display names, friend codes
- ✅ **Admin Functionality**: Admin middleware, display name approval/rejection, authorization

## Security

- Solution integrity is verified using a salted hash
- All operations are atomic via stored procedure
- Invalid score types are rejected

### Solution Hash Calculation
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

## Default Users
- **Anonymous**
  - Id: `00000000-0000-0000-0000-000000000000`
  - Friend code: `0000-0000`
- **Dev**
  - Id: `00000000-0000-0000-0000-000000000001`
  - Friend code: `0000-0001`
  - Admin access for display name moderation
  - Authenticate using `DEV_API_KEY` environment variable


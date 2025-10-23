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

**Important:** Change `DEV_API_KEY` in production!

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

## API Endpoints

### Public Endpoints
- `GET /health` - Health check
- `POST /user` - Create new user, returns API key (store this!)
- `GET /user/{id}` - Get user info by UUID or friend code

### Authenticated Endpoints
Requires `Authorization: Bearer {api_key}` header

- `PUT /user/{id}/name` - Update display name (sets pending, requires approval)

### Admin Endpoints
Requires dev user authentication (`DEV_API_KEY`)

- `GET /admin/pending-display-names` - List all pending display name changes
- `PUT /admin/display-names/{user_id}` - Approve/reject display name change
  - Request body: `{"approve": true}` or `{"approve": false}`

### Default Users
- **Anonymous** (`00000000-0000-0000-0000-000000000000`) - Friend code: `0000-0000`
- **Dev** (`00000000-0000-0000-0000-000000000001`) - Friend code: `0000-0001`
  - Admin access for display name moderation
  - Authenticate using `DEV_API_KEY` environment variable


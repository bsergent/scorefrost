# Score Submission API

## POST /score/submit

Submits a player's solution and scores for a specific level.

### Request Body

```json
{
  "level_id": "string",           // Unique identifier for the level
  "level_version": 1,             // Version number of the level
  "game_version": "string",       // Version of the game client
  "api_key": "string",            // User's API/secret key for authentication
  "solution": "string",           // Base64 encoded solution bytes
  "solution_hash": "string",      // SHA256 hash of (solution + salt)
  "scores": {                     // Map of score type to score value
    "time": 12500,               // Time in milliseconds (lower is better)
    "striping": 85,              // Striping score (higher is better)
    "fuel": 750,                 // Fuel consumed (lower is better)
    "stars": 3                   // Stars earned (higher is better)
  }
}
```

### Authentication

The endpoint uses API key authentication. The `api_key` field in the request body is hashed using SHA256 and compared against stored user credentials.

### Solution Integrity

The `solution_hash` must be the SHA256 hash of the concatenated string: `solution + SOLUTION_SALT` where `SOLUTION_SALT` is a server-side secret configured via environment variable.

#### Calculating Solution Hash

```bash
# Example using the hash utility
go run utils/hash-util.go <base64_solution> <salt>

# Example
go run utils/hash-util.go SGVsbG8gV29ybGQ= your_secret_salt_change_in_production
```

### Score Types

Currently supported score types:
- `time` - Completion time in milliseconds (lower is better)
- `striping` - Striping optimization score (higher is better)  
- `fuel` - Fuel consumption (lower is better)
- `stars` - Stars earned (higher is better)

### Response

#### Success (201 Created)
```json
{
  "success": true,
  "solution_id": 123,
  "message": "Score submitted successfully"
}
```

#### Error (400/401/500)
```json
{
  "success": false,
  "message": "Error description"
}
```

### Error Cases

- **400 Bad Request**: Missing required fields, invalid JSON, invalid base64 solution, solution hash verification failed, invalid score types
- **401 Unauthorized**: Invalid API key
- **500 Internal Server Error**: Database errors, transaction failures

### Environment Variables

- `SOLUTION_SALT` - Secret salt used for solution hash verification (required)

### Database Tables

The endpoint inserts data into:
- `solution` table - Stores the user's solution attempt
- `score` table - Stores individual score values by type

### Security Features

1. **API Key Authentication** - Users must provide valid API keys
2. **Solution Hash Verification** - Prevents tampering with solution data
3. **Transaction Safety** - All database operations are atomic
4. **Input Validation** - Validates all input fields and data types
5. **Score Type Validation** - Only allows submission of registered score types
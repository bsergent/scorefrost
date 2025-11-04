# Leaderboard API Endpoint

## GET /score/leaderboard

Returns paginated leaderboard scores for specified levels within the given scope.

### Authentication
Requires a valid API key in the `X-API-Key` header.

### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `levels` | string | Yes | Comma-separated list of level specifications in format `levelId` or `levelId.version` |
| `scope` | string | No | Scope of leaderboard. Options: `personal`, `friends`, `regional`, `global` (default: `global`) |
| `offset` | integer | No | Number of records to skip for pagination (default: `0`, min: `0`) |
| `size` | integer | No | Number of records to return per page (default: `20`, min: `1`, max: `100`) |

### Level Format
- `levelId` - Gets scores for the latest version of the level
- `levelId.version` - Gets scores for the specific version of the level
- Examples: `level1`, `level1.2`, `puzzle_easy.1`

### Response Format

```json
{
  "scores": [
    {
      "rank": 1,
      "level_id": "level1",
      "level_version": 1,
      "score_type": "time_ms",
      "best_score": 12500,
      "user_id": "123e4567-e89b-12d3-a456-426614174000",
      "display_name": "Speedy Runner",
      "friend_code": "AB12-CD34"
    }
  ],
  "count": 10,
  "scope": "global",
  "pagination": {
    "offset": 0,
    "size": 20,
    "total": 150
  }
}
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `scores` | array | Array of leaderboard entries |
| `count` | integer | Number of scores returned in this response |
| `scope` | string | The scope that was queried |
| `pagination` | object | Pagination metadata |

#### Pagination Object

| Field | Type | Description |
|-------|------|-------------|
| `offset` | integer | The offset used for pagination |
| `size` | integer | The page size requested |
| `total` | integer | Total number of scores available (for pagination) |

#### Score Entry Fields

| Field | Type | Description |
|-------|------|-------------|
| `rank` | integer | Position in the leaderboard (1-based) |
| `level_id` | string | Identifier of the level |
| `level_version` | integer | Version of the level |
| `score_type` | string | Type of score (e.g., "time_ms", "points") |
| `best_score` | integer | The best score value |
| `user_id` | string | UUID of the user who achieved the score |
| `display_name` | string | User's display name |
| `friend_code` | string | User's friend code |

### Example Requests

#### Basic Global Leaderboard
```http
GET /score/leaderboard?levels=level1,level2.3&scope=global&offset=0&size=10
X-API-Key: your-api-key-here
```

#### Personal Leaderboard with Pagination
```http
GET /score/leaderboard?levels=level1.1,level2.2&scope=personal&offset=10&size=5
X-API-Key: your-api-key-here
```

#### Regional Leaderboard
```http
GET /score/leaderboard?levels=puzzle_easy,puzzle_hard.5&scope=regional&offset=0&size=20
X-API-Key: your-api-key-here
```

### Error Responses

| Status Code | Description |
|-------------|-------------|
| `400` | Invalid parameters (missing levels, invalid scope, invalid pagination) |
| `401` | Missing or invalid API key |
| `500` | Internal server error |
| `501` | Leaderboard functionality not yet implemented in database |

### Differences from `/score/best`

| Feature | `/score/best` | `/score/leaderboard` |
|---------|---------------|---------------------|
| Ranking | No rank information | Includes rank for each entry |
| Pagination | Returns all results | Supports offset/size pagination |
| Total count | Returns count of results | Returns count + total available |
| Response size | Can be large | Controlled by pagination |
| Use case | Get user's best scores | View competitive rankings |

### Notes

- The `personal` scope returns only the authenticated user's scores
- The `friends` scope is planned for future implementation (friend system)
- The `regional` scope is planned for future implementation (geographical regions)
- The `global` scope returns scores from all users
- Pagination helps manage large result sets efficiently
- Rankings are calculated server-side for consistency
# Admin Endpoints Testing Guide

## Prerequisites
1. Ensure `.env` file has `DEV_API_KEY` set
2. Server must be running with database initialized
3. Dev user API key hash is automatically set on startup

## Test Flow

### 1. Create a test user
```http
POST http://localhost:8080/user
```
Save the returned `api_key`, private `id`, and public `friend_code` for next steps.

### 2. Update display name (creates pending request)
```http
PUT http://localhost:8080/api/v2/user/name
Authorization: Bearer {api_key_from_step_1}
Content-Type: application/json

{
  "display_name": "TestUser123"
}
```

### 3. Get pending display names (as admin)
```http
GET http://localhost:8080/admin/names
Authorization: Bearer dev_key_change_in_production
```

Expected response:
```json
{
  "pending_names": [
    {
      "user_id": "...",
      "friend_code": "...",
      "current_display_name": "...",
      "pending_display_name": "TestUser123",
      "display_name_status": 0
    }
  ],
  "count": 1
}
```

### 4. Approve the display name (as admin)
```http
PUT http://localhost:8080/admin/names/{user_id_from_step_1}
Authorization: Bearer dev_key_change_in_production
Content-Type: application/json

{
  "approve": true
}
```

Expected response:
```json
{
  "user_id": "...",
  "display_name": "TestUser123",
  "status": 1,
  "status_message": "approved"
}
```

### 5. Verify display name was approved
```http
GET http://localhost:8080/user/{user_id_from_step_1}
```

Should show `display_name: "TestUser123"`

### Test Rejection Flow

Follow steps 1-3, then:

```http
PUT http://localhost:8080/admin/names/{user_id}
Authorization: Bearer dev_key_change_in_production
Content-Type: application/json

{
  "approve": false
}
```

Expected: Display name reverts to previous value, pending is cleared.

## Security Tests

### Non-admin user cannot access admin endpoints
```http
GET http://localhost:8080/admin/names
Authorization: Bearer {regular_user_api_key}
```
Expected: 403 Forbidden

### No authentication
```http
GET http://localhost:8080/admin/names
```
Expected: 401 Unauthorized

### Wrong admin key
```http
GET http://localhost:8080/admin/names
Authorization: Bearer wrong_key
```
Expected: 401 Unauthorized

## Display Name Status Codes
- `0` = Pending (awaiting admin approval)
- `1` = Approved (display name is active)
- `2` = Rejected (display name change was denied)

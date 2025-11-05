# Test Organization

This project now follows Go testing conventions properly:

## Test File Structure

### Unit Tests (No external dependencies)
- `main_test.go` - Tests for `main.go` (health endpoint, route registration)
- `auth_test.go` - Tests for `auth.go` (IP extraction, context functions)
- `user_test.go` - Tests for `user.go` (API key generation, hashing, display names, friend codes)
- `ratelimit_test.go` - Tests for `ratelimit.go` (rate limiter functionality)
- `score_test.go` - Tests for `score.go` (level parameter parsing, response structures)

### Integration Tests (Require database)
- `integration_api_test.go` - Full API integration tests with database
- `integration_test.go` - Integration test setup and utilities
- `integration_utils.go` - Helper functions for integration tests

## Go Testing Conventions Followed

1. **File Naming**: Test files are named `<source>_test.go` to match the source file they test
2. **Package**: All tests are in the same package (`main`) as the source code
3. **Function Naming**: Test functions follow `TestFunctionName` pattern
4. **Separation of Concerns**: Unit tests vs integration tests are clearly separated
5. **Table-Driven Tests**: Complex test cases use the standard Go table-driven pattern
6. **No Mixed Dependencies**: Unit tests don't require external dependencies (database, network)

## Test Coverage

### Files with Unit Tests
- ✅ `main.go` → `main_test.go`
- ✅ `auth.go` → `auth_test.go`  
- ✅ `user.go` → `user_test.go`
- ✅ `ratelimit.go` → `ratelimit_test.go`
- ✅ `score.go` → `score_test.go`

### Files Covered by Integration Tests
- ✅ `admin.go` - Admin endpoints tested in integration tests
- ✅ `database.go` - Database initialization tested through integration tests
- ✅ `routes.go` - Route setup tested in `main_test.go` and integration tests

## Running Tests

```bash
# Run all tests
go test ./go/

# Run only unit tests (fast)
go test ./go/ -short

# Run integration tests
go test ./go/ -tags=integration

# Run specific test file
go test ./go/ -run TestHealth

# Run with coverage
go test ./go/ -cover
```
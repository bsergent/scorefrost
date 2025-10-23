package main

import (
	"context"
	"net/http"
	"strings"
)

// PathParams extracts path parameters from a URL pattern
// Example: extractPathParam("/user/123/name", "/user/{id}/name") returns {"id": "123"}
func extractPathParams(path string, pattern string) map[string]string {
	params := make(map[string]string)

	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")

	if len(pathParts) != len(patternParts) {
		return params
	}

	for i, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			// Extract parameter name from {name}
			paramName := strings.Trim(part, "{}")
			params[paramName] = pathParts[i]
		} else if part != pathParts[i] {
			// Path doesn't match pattern
			return make(map[string]string)
		}
	}

	return params
}

// matchesPattern checks if a path matches a pattern with parameters
func matchesPattern(path string, pattern string) bool {
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")

	if len(pathParts) != len(patternParts) {
		return false
	}

	for i, part := range patternParts {
		// {param} matches anything
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			continue
		}
		// Literal parts must match exactly
		if part != pathParts[i] {
			return false
		}
	}

	return true
}

// RouteHandler wraps a handler with path parameter extraction
type RouteHandler struct {
	pattern string
	handler http.HandlerFunc
}

// ServeHTTP extracts path parameters and adds them to request context
func (rh *RouteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	params := extractPathParams(r.URL.Path, rh.pattern)

	// Add parameters to request context
	ctx := r.Context()
	for key, value := range params {
		ctx = contextWithValue(ctx, "path_"+key, value)
	}

	rh.handler(w, r.WithContext(ctx))
}

// GetPathParam extracts a path parameter from the request context
func GetPathParam(r *http.Request, name string) string {
	if val := r.Context().Value(contextKey("path_" + name)); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// Helper to add values to context with proper key type
func contextWithValue(ctx context.Context, key string, value string) context.Context {
	return context.WithValue(ctx, contextKey(key), value)
}

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	httpSwagger "github.com/swaggo/http-swagger/v2"
	"gopkg.in/yaml.v2"
)

// serveOpenAPISpec serves the OpenAPI specification in JSON format
func serveOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	// Read the OpenAPI YAML file
	yamlData, err := os.ReadFile("openapi.yaml")
	if err != nil {
		log.Printf("Failed to read openapi.yaml: %v", err)
		http.Error(w, "Failed to read OpenAPI specification", http.StatusInternalServerError)
		return
	}

	// Parse YAML to interface{} first
	var yamlSpec any
	if err := yaml.Unmarshal(yamlData, &yamlSpec); err != nil {
		log.Printf("Failed to parse YAML: %v", err)
		http.Error(w, "Failed to parse OpenAPI specification", http.StatusInternalServerError)
		return
	}

	// Convert to JSON-compatible format by converting through JSON
	jsonData, err := json.Marshal(convertToJSONCompatible(yamlSpec))
	if err != nil {
		log.Printf("Failed to marshal to JSON: %v", err)
		http.Error(w, "Failed to encode OpenAPI specification", http.StatusInternalServerError)
		return
	}

	// Parse back to map[string]interface{} to ensure proper structure
	var spec map[string]interface{}
	if err := json.Unmarshal(jsonData, &spec); err != nil {
		log.Printf("Failed to unmarshal JSON: %v", err)
		http.Error(w, "Failed to encode OpenAPI specification", http.StatusInternalServerError)
		return
	}

	// Convert to JSON and serve
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(spec); err != nil {
		log.Printf("Failed to encode JSON: %v", err)
		http.Error(w, "Failed to encode OpenAPI specification", http.StatusInternalServerError)
		return
	}
}

// convertToJSONCompatible converts YAML interface{} to JSON-compatible format
func convertToJSONCompatible(i any) any {
	switch x := i.(type) {
	case map[any]any:
		m2 := map[string]any{}
		for k, v := range x {
			m2[k.(string)] = convertToJSONCompatible(v)
		}
		return m2
	case []any:
		for i, v := range x {
			x[i] = convertToJSONCompatible(v)
		}
	}
	return i
}

// setupDocsRoutes adds documentation endpoints to the mux
func setupDocsRoutes(mux *http.ServeMux) {
	// Serve OpenAPI spec at /openapi.json
	mux.HandleFunc("GET /openapi.json", serveOpenAPISpec)

	// Serve Swagger UI at /docs
	mux.Handle("/docs/", httpSwagger.Handler(
		httpSwagger.URL("/openapi.json"), // Point to our OpenAPI spec
		httpSwagger.DocExpansion("list"),
		httpSwagger.DeepLinking(true),
	))

	// Redirect /docs to /docs/ for convenience
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/", http.StatusMovedPermanently)
	})
}

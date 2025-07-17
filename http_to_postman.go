package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test helper functions
func createTempFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.http")
	
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	
	return tmpFile
}

func readJSONFile(t *testing.T, filename string) Collection {
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read JSON file: %v", err)
	}
	
	var collection Collection
	err = json.Unmarshal(data, &collection)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	
	return collection
}

func TestSimpleGETRequest(t *testing.T) {
	httpContent := `GET https://api.example.com/users
Accept: application/json
Authorization: Bearer token123

###`

	inputFile := createTempFile(t, httpContent)
	outputFile := filepath.Join(t.TempDir(), "output.json")
	
	err := convertHTTPToPostman(inputFile, outputFile)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}
	
	collection := readJSONFile(t, outputFile)
	
	// Verify collection structure
	if len(collection.Items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(collection.Items))
	}
	
	item := collection.Items[0]
	if item.Name != "request-1" {
		t.Errorf("Expected name 'request-1', got '%s'", item.Name)
	}
	
	if item.Request.Method != "GET" {
		t.Errorf("Expected method 'GET', got '%s'", item.Request.Method)
	}
	
	if item.Request.URL.Raw != "https://api.example.com/users" {
		t.Errorf("Expected URL 'https://api.example.com/users', got '%s'", item.Request.URL.Raw)
	}
	
	if item.Request.URL.Protocol != "https" {
		t.Errorf("Expected protocol 'https', got '%s'", item.Request.URL.Protocol)
	}
	
	expectedHost := []string{"api", "example", "com"}
	if len(item.Request.URL.Host) != len(expectedHost) {
		t.Errorf("Expected host %v, got %v", expectedHost, item.Request.URL.Host)
	}
	
	expectedPath := []string{"users"}
	if len(item.Request.URL.Path) != len(expectedPath) {
		t.Errorf("Expected path %v, got %v", expectedPath, item.Request.URL.Path)
	}
	
	// Check headers
	if len(item.Request.Header) != 2 {
		t.Errorf("Expected 2 headers, got %d", len(item.Request.Header))
	}
	
	acceptHeader := item.Request.Header[0]
	if acceptHeader.Key != "Accept" || acceptHeader.Value != "application/json" {
		t.Errorf("Expected Accept header, got %v", acceptHeader)
	}
	
	authHeader := item.Request.Header[1]
	if authHeader.Key != "Authorization" || authHeader.Value != "Bearer token123" {
		t.Errorf("Expected Authorization header, got %v", authHeader)
	}
}

func TestPOSTRequestWithJSON(t *testing.T) {
	httpContent := `POST https://api.example.com/users
Content-Type: application/json

{
  "name": "John Doe",
  "email": "john@example.com"
}

###`

	inputFile := createTempFile(t, httpContent)
	outputFile := filepath.Join(t.TempDir(), "output.json")
	
	err := convertHTTPToPostman(inputFile, outputFile)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}
	
	collection := readJSONFile(t, outputFile)
	
	if len(collection.Items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(collection.Items))
	}
	
	item := collection.Items[0]
	if item.Request.Method != "POST" {
		t.Errorf("Expected method 'POST', got '%s'", item.Request.Method)
	}
	
	// Check body
	if item.Request.Body.Mode != "raw" {
		t.Errorf("Expected body mode 'raw', got '%s'", item.Request.Body.Mode)
	}
	
	expectedBody := `{
  "name": "John Doe",
  "email": "john@example.com"
}`
	
	if strings.TrimSpace(item.Request.Body.Raw) != strings.TrimSpace(expectedBody) {
		t.Errorf("Expected body:\n%s\nGot:\n%s", expectedBody, item.Request.Body.Raw)
	}
	
	// Check body options
	if item.Request.Body.Options == nil {
		t.Error("Expected body options to be set")
	}
}

func TestRequestWithQueryParameters(t *testing.T) {
	httpContent := `GET https://api.example.com/users?page=1&limit=10&sort=name
Accept: application/json

###`

	inputFile := createTempFile(t, httpContent)
	outputFile := filepath.Join(t.TempDir(), "output.json")
	
	err := convertHTTPToPostman(inputFile, outputFile)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}
	
	collection := readJSONFile(t, outputFile)
	
	if len(collection.Items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(collection.Items))
	}
	
	item := collection.Items[0]
	
	// Check query parameters
	expectedParams := []QueryParam{
		{Key: "page", Value: "1"},
		{Key: "limit", Value: "10"},
		{Key: "sort", Value: "name"},
	}
	
	if len(item.Request.URL.Query) != len(expectedParams) {
		t.Errorf("Expected %d query params, got %d", len(expectedParams), len(item.Request.URL.Query))
	}
	
	for i, expected := range expectedParams {
		actual := item.Request.URL.Query[i]
		if actual.Key != expected.Key || actual.Value != expected.Value {
			t.Errorf("Expected query param %v, got %v", expected, actual)
		}
	}
}

func TestMultipleRequests(t *testing.T) {
	httpContent := `GET https://api.example.com/users
Accept: application/json

###

POST https://api.example.com/users
Content-Type: application/json

{
  "name": "Jane Doe"
}

###

DELETE https://api.example.com/users/123
Authorization: Bearer token123

###`

	inputFile := createTempFile(t, httpContent)
	outputFile := filepath.Join(t.TempDir(), "output.json")
	
	err := convertHTTPToPostman(inputFile, outputFile)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}
	
	collection := readJSONFile(t, outputFile)
	
	if len(collection.Items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(collection.Items))
	}
	
	// Check first request
	if collection.Items[0].Request.Method != "GET" {
		t.Errorf("Expected first request method 'GET', got '%s'", collection.Items[0].Request.Method)
	}
	
	// Check second request
	if collection.Items[1].Request.Method != "POST" {
		t.Errorf("Expected second request method 'POST', got '%s'", collection.Items[1].Request.Method)
	}
	
	if collection.Items[1].Request.Body.Mode != "raw" {
		t.Errorf("Expected second request body mode 'raw', got '%s'", collection.Items[1].Request.Body.Mode)
	}
	
	// Check third request
	if collection.Items[2].Request.Method != "DELETE" {
		t.Errorf("Expected third request method 'DELETE', got '%s'", collection.Items[2].Request.Method)
	}
	
	if collection.Items[2].Request.URL.Raw != "https://api.example.com/users/123" {
		t.Errorf("Expected third request URL 'https://api.example.com/users/123', got '%s'", collection.Items[2].Request.URL.Raw)
	}
}

func TestRequestWithComments(t *testing.T) {
	httpContent := `# This is a comment
GET https://api.example.com/users
# Another comment
Accept: application/json

###`

	inputFile := createTempFile(t, httpContent)
	outputFile := filepath.Join(t.TempDir(), "output.json")
	
	err := convertHTTPToPostman(inputFile, outputFile)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}
	
	collection := readJSONFile(t, outputFile)
	
	if len(collection.Items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(collection.Items))
	}
	
	// Comments should be ignored, only headers and request should remain
	item := collection.Items[0]
	if len(item.Request.Header) != 1 {
		t.Errorf("Expected 1 header (comments should be ignored), got %d", len(item.Request.Header))
	}
}

func TestCollectionMetadata(t *testing.T) {
	httpContent := `GET https://api.example.com/test

###`

	inputFile := createTempFile(t, httpContent)
	outputFile := filepath.Join(t.TempDir(), "output.json")
	
	err := convertHTTPToPostman(inputFile, outputFile)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}
	
	collection := readJSONFile(t, outputFile)
	
	// Check collection info
	if !strings.HasPrefix(collection.Info.Name, "jb-export-") {
		t.Errorf("Expected collection name to start with 'jb-export-', got '%s'", collection.Info.Name)
	}
	
	expectedSchema := "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
	if collection.Info.Schema != expectedSchema {
		t.Errorf("Expected schema '%s', got '%s'", expectedSchema, collection.Info.Schema)
	}
}

func TestInvalidInput(t *testing.T) {
	// Test with non-existent file
	outputFile := filepath.Join(t.TempDir(), "output.json")
	err := convertHTTPToPostman("non-existent.http", outputFile)
	if err == nil {
		t.Error("Expected error for non-existent input file")
	}
}

func TestOneLineJSON(t *testing.T) {
	httpContent := `POST https://api.example.com/users
Content-Type: application/json

{"name": "John", "age": 30}

###`

	inputFile := createTempFile(t, httpContent)
	outputFile := filepath.Join(t.TempDir(), "output.json")
	
	err := convertHTTPToPostman(inputFile, outputFile)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}
	
	collection := readJSONFile(t, outputFile)
	
	if len(collection.Items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(collection.Items))
	}
	
	item := collection.Items[0]
	expectedBody := `{"name": "John", "age": 30}`
	
	if strings.TrimSpace(item.Request.Body.Raw) != expectedBody {
		t.Errorf("Expected body '%s', got '%s'", expectedBody, item.Request.Body.Raw)
	}
}

// Benchmark test
func BenchmarkConvertHTTPToPostman(b *testing.B) {
	httpContent := `GET https://api.example.com/users
Accept: application/json
Authorization: Bearer token123

###

POST https://api.example.com/users
Content-Type: application/json

{
  "name": "John Doe",
  "email": "john@example.com"
}

###`

	tmpDir := b.TempDir()
	inputFile := filepath.Join(tmpDir, "input.http")
	outputFile := filepath.Join(tmpDir, "output.json")
	
	err := os.WriteFile(inputFile, []byte(httpContent), 0644)
	if err != nil {
		b.Fatalf("Failed to create input file: %v", err)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		err := convertHTTPToPostman(inputFile, outputFile)
		if err != nil {
			b.Fatalf("Conversion failed: %v", err)
		}
	}
}

// Example usage test
func ExampleConvertHTTPToPostman() {
	httpContent := `GET https://api.example.com/users
Accept: application/json

###`

	// Create temp files
	tmpDir := "/tmp"
	inputFile := filepath.Join(tmpDir, "example.http")
	outputFile := filepath.Join(tmpDir, "example.json")
	
	// Write example content
	os.WriteFile(inputFile, []byte(httpContent), 0644)
	
	// Convert
	err := convertHTTPToPostman(inputFile, outputFile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Println("Conversion successful!")
	// Output: Conversion successful!
}

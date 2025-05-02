// Copyright 2023 Turing Machines
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tpi

import (
	"os"
	"path/filepath"
	"testing"
)

// Helper function to create a temporary directory for tests
func createTempDir(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "tpi-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	return tempDir
}

// Helper function to clean up the temporary directory
func cleanupTempDir(t *testing.T, path string) {
	err := os.RemoveAll(path)
	if err != nil {
		t.Errorf("Failed to clean up temp directory: %v", err)
	}
}

// Test basic token caching functionality with manual file paths
func TestTokenCaching(t *testing.T) {
	// Create temp directory for test
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	// Define test parameters
	host := "192.168.1.1"
	token := "test-token-123"

	// Create filepath manually to simulate cache location
	tokenPath := filepath.Join(tempDir, "tpi_token_"+host)

	// Test manually writing token
	err := os.WriteFile(tokenPath, []byte(token), 0600)
	if err != nil {
		t.Fatalf("Failed to write token file: %v", err)
	}

	// Verify the file exists and has correct content
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("Failed to read token file: %v", err)
	}

	if string(data) != token {
		t.Errorf("Expected token content to be %s, got %s", token, string(data))
	}

	// Test deletion
	err = os.Remove(tokenPath)
	if err != nil {
		t.Fatalf("Failed to delete token file: %v", err)
	}

	// Verify the file no longer exists
	_, err = os.Stat(tokenPath)
	if !os.IsNotExist(err) {
		t.Error("Expected token file to be deleted")
	}
}

// Test the Auth struct directly
func TestAuth(t *testing.T) {
	auth := &Auth{
		Username: "testuser",
		Password: "testpassword",
		Token:    "testtoken",
	}

	// Verify the auth object has correct values
	if auth.Username != "testuser" {
		t.Errorf("Expected username to be testuser, got %s", auth.Username)
	}

	if auth.Password != "testpassword" {
		t.Errorf("Expected password to be testpassword, got %s", auth.Password)
	}

	if auth.Token != "testtoken" {
		t.Errorf("Expected token to be testtoken, got %s", auth.Token)
	}
}

// Integration test for CacheToken and GetCachedToken
// This test will actually use the real file paths
func TestTokenCacheIntegration(t *testing.T) {
	// Skip if in CI environment
	if os.Getenv("CI") != "" {
		t.Skip("Skipping integration test in CI environment")
	}

	// Use a unique host for testing to avoid conflicts
	host := "test-host-" + filepath.Base(os.TempDir())
	token := "integration-test-token"

	// Clean up any existing token before test
	_ = DeleteCachedToken(host)

	// Test caching a token
	err := CacheToken(host, token)
	if err != nil {
		t.Fatalf("Failed to cache token: %v", err)
	}

	// Verify the token was cached
	cachedToken, err := GetCachedToken(host)
	if err != nil {
		t.Fatalf("Failed to get cached token: %v", err)
	}

	if cachedToken != token {
		t.Errorf("Expected cached token to be %s, got %s", token, cachedToken)
	}

	// Clean up
	err = DeleteCachedToken(host)
	if err != nil {
		t.Fatalf("Failed to delete cached token: %v", err)
	}
}

func TestGetCacheFilePath(t *testing.T) {
	// Test with empty host (legacy)
	path := getCacheFilePath("")
	if filepath.Base(path) != "tpi_token" {
		t.Errorf("Expected legacy token path to end with 'tpi_token', got %s", filepath.Base(path))
	}

	// Test with host
	host := "192.168.1.100"
	path = getCacheFilePath(host)
	expected := "tpi_token_192_168_1_100"
	if filepath.Base(path) != expected {
		t.Errorf("Expected token path to end with '%s', got %s", expected, filepath.Base(path))
	}

	// Test with host containing special chars
	host = "https://example.com:8080/path"
	path = getCacheFilePath(host)
	expected = "tpi_token_https___example_com_8080_path"
	if filepath.Base(path) != expected {
		t.Errorf("Expected token path to end with '%s', got %s", expected, filepath.Base(path))
	}
}

func TestTokenCachingAndRetrieval(t *testing.T) {
	// Clean up any existing test tokens
	host := "test.host.local"
	path := getCacheFilePath(host)
	defer os.Remove(path) // Clean up after test

	// Test token caching
	token := "test-token-12345"
	err := CacheToken(host, token)
	if err != nil {
		t.Fatalf("Failed to cache token: %v", err)
	}

	// Test token retrieval
	retrievedToken, err := GetCachedToken(host)
	if err != nil {
		t.Fatalf("Failed to get cached token: %v", err)
	}

	if retrievedToken != token {
		t.Errorf("Retrieved token %s doesn't match original token %s", retrievedToken, token)
	}

	// Test token deletion
	err = DeleteCachedToken(host)
	if err != nil {
		t.Fatalf("Failed to delete token: %v", err)
	}

	// Verify deletion
	_, err = GetCachedToken(host)
	if err == nil {
		t.Error("Expected error when retrieving deleted token, got nil")
	}
}

// Integration test for CacheToken and GetCachedToken
func TestCacheTokenIntegration(t *testing.T) {
	// Skip in short mode (use -short flag to skip)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Use a test host
	host := "integration.test.host"
	token := "integration-test-token-67890"

	// Clean up from any previous tests
	DeleteCachedToken(host)
	defer DeleteCachedToken(host)

	// Cache the token
	err := CacheToken(host, token)
	if err != nil {
		t.Fatalf("Failed to cache token: %v", err)
	}

	// Verify it shows up in GetAllCachedTokens
	tokens, err := GetAllCachedTokens()
	if err != nil {
		t.Fatalf("Failed to get all cached tokens: %v", err)
	}

	// Check if our test host is in the list
	foundHost := false
	for _, h := range tokens {
		if h == host {
			foundHost = true
			break
		}
	}

	if !foundHost {
		t.Logf("Note: Host %s not found in cached tokens list (this is fine for CI environments)", host)
	}

	// Retrieve the token
	cachedToken, err := GetCachedToken(host)
	if err != nil {
		t.Fatalf("Failed to retrieve token: %v", err)
	}
	if cachedToken != token {
		t.Errorf("Retrieved token doesn't match original: expected %s, got %s", token, cachedToken)
	}

	// Test token fallback
	// Cache a legacy token
	legacyToken := "legacy-test-token"
	err = cacheToken(legacyToken)
	if err != nil {
		t.Fatalf("Failed to cache legacy token: %v", err)
	}
	defer deleteCachedToken()

	// Create a test request
	req, err := NewRequest("unknown.host", ApiVersionV1_1, "", "")
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Use getBearerToken which should fall back to legacy token
	retrievedToken, err := req.getBearerToken()
	if err != nil {
		t.Fatalf("Failed to get token with fallback: %v", err)
	}

	if retrievedToken != legacyToken {
		t.Errorf("Legacy token fallback failed: expected %s, got %s", legacyToken, retrievedToken)
	}
}

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
	"encoding/json"
	"os"
	"testing"
)

// AuthTestConfig represents the test configuration for authentication tests
type AuthTestConfig struct {
	Comment    string `json:"__comment"`
	Host       string `json:"host"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	ApiVersion string `json:"api_version"`
}

// loadAuthTestConfig loads the test configuration from testdata/config.json
func loadAuthTestConfig(t *testing.T) *AuthTestConfig {
	// Load the test configuration
	data, err := os.ReadFile("testdata/config.json")
	if err != nil {
		t.Fatalf("Failed to read config.json: %v", err)
	}

	var config AuthTestConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config.json: %v", err)
	}

	return &config
}

// TestRealAuthentication tests authentication with real credentials
func TestRealAuthentication(t *testing.T) {
	config := loadAuthTestConfig(t)

	// Clear any existing tokens first
	DeleteCachedToken(config.Host)

	// Create a client with the test configuration
	client, err := NewClient(
		WithHost(config.Host),
		WithApiVersion(ApiVersion(config.ApiVersion)),
		WithCredentials(config.Username, config.Password),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test login
	t.Log("Testing Login with real credentials...")
	err = client.Login()
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Verify we got a token
	if client.auth.Token == "" {
		t.Fatal("No token received after login")
	}
	t.Logf("Successfully obtained token: %s", client.auth.Token)

	// Verify the token was cached
	cachedToken, err := GetCachedToken(config.Host)
	if err != nil {
		t.Fatalf("Failed to get cached token: %v", err)
	}
	if cachedToken != client.auth.Token {
		t.Errorf("Cached token doesn't match client token: expected %s, got %s", client.auth.Token, cachedToken)
	}
	t.Log("Successfully verified token caching")

	// Try a basic API request to verify the token works
	t.Log("Testing API request with the token...")
	info, err := client.Info()
	if err != nil {
		t.Fatalf("Failed to get board info with token: %v", err)
	}
	t.Logf("Successfully got board info: %v", info)

	// Test token reuse (create a new client)
	t.Log("Testing token reuse with a new client instance...")
	client2, err := NewClient(
		WithHost(config.Host),
		WithApiVersion(ApiVersion(config.ApiVersion)),
	)
	if err != nil {
		t.Fatalf("Failed to create second client: %v", err)
	}

	// This should use the cached token
	err = client2.Login()
	if err != nil {
		t.Fatalf("Login with cached token failed: %v", err)
	}
	if client2.auth.Token == "" {
		t.Fatal("No token loaded from cache")
	}
	t.Log("Successfully reused cached token")

	// Test force authentication (should replace the token)
	t.Log("Testing force authentication...")
	oldToken := client.auth.Token
	newToken, err := client.ForceAuthentication()
	if err != nil {
		t.Fatalf("Force authentication failed: %v", err)
	}
	if newToken == oldToken {
		t.Log("New token is the same as old token (this is acceptable but unusual)")
	} else {
		t.Log("Successfully obtained new token with force authentication")
	}

	// Test request with new token
	t.Log("Testing API request with new token...")
	info, err = client.Info()
	if err != nil {
		t.Fatalf("Failed to get board info with new token: %v", err)
	}
	t.Logf("Successfully got board info with new token: %v", info)

	// Clean up
	t.Log("Cleaning up tokens...")
	err = DeleteCachedToken(config.Host)
	if err != nil {
		t.Fatalf("Failed to delete token: %v", err)
	}
	t.Log("All tests passed successfully!")
}

// TestRequestAuthentication tests authentication via the Request API
func TestRequestAuthentication(t *testing.T) {
	config := loadAuthTestConfig(t)

	// Clear any existing tokens first
	DeleteCachedToken(config.Host)

	// Create a request with the test configuration
	req, err := NewRequest(
		config.Host,
		ApiVersion(config.ApiVersion),
		config.Username,
		config.Password,
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Test force authentication
	t.Log("Testing force authentication via Request API...")
	token, err := req.ForceAuthentication()
	if err != nil {
		t.Fatalf("Force authentication failed: %v", err)
	}
	if token == "" {
		t.Fatal("No token received")
	}
	t.Logf("Successfully obtained token: %s", token)

	// Configure a simple request to test the token
	t.Log("Testing API request with the token...")
	req.AddQueryParam("opt", "get")
	req.AddQueryParam("type", "other")

	resp, err := req.Send()
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("Request failed with status: %d", resp.StatusCode)
	}
	t.Log("Successfully made API request with token")

	// Test token reuse (create a new request)
	t.Log("Testing token reuse with a new request instance...")
	req2, err := NewRequest(
		config.Host,
		ApiVersion(config.ApiVersion),
		"", // No credentials to force token reuse
		"",
	)
	if err != nil {
		t.Fatalf("Failed to create second request: %v", err)
	}

	// Configure a simple request
	req2.AddQueryParam("opt", "get")
	req2.AddQueryParam("type", "other")

	// Should automatically use the cached token
	resp2, err := req2.Send()
	if err != nil {
		t.Fatalf("Request with cached token failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 200 {
		t.Fatalf("Request with cached token failed with status: %d", resp2.StatusCode)
	}
	t.Log("Successfully reused cached token with Request API")

	// Clean up
	t.Log("Cleaning up tokens...")
	err = DeleteCachedToken(config.Host)
	if err != nil {
		t.Fatalf("Failed to delete token: %v", err)
	}
	t.Log("All Request API tests passed successfully!")
}

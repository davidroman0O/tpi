package tpi

import (
	"encoding/json"
	"os"
	"testing"
)

// Config represents the test configuration for connecting to real hardware
type TestConfig struct {
	Host       string `json:"host"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	ApiVersion string `json:"api_version"`
}

// loadTestConfig loads the test configuration from testdata/config.json
func loadTestConfig(t *testing.T) TestConfig {
	t.Helper()

	// Check if the config file exists
	configPath := "testdata/config.json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skipf("Test config file not found at %s, skipping hardware test", configPath)
	}

	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Skipf("Failed to read test config: %v", err)
	}

	// Parse the config
	var config TestConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Skipf("Failed to parse test config: %v", err)
	}

	// Validate required fields
	if config.Host == "" {
		t.Skip("Test config missing host field")
	}

	return config
}

// createTestClient creates a client using the test configuration
func createTestClient(t *testing.T) *Client {
	t.Helper()

	config := loadTestConfig(t)

	// Create options
	options := []Option{
		WithHost(config.Host),
	}

	// Add credentials if provided
	if config.Username != "" || config.Password != "" {
		options = append(options, WithCredentials(config.Username, config.Password))
	}

	// Add API version if specified
	if config.ApiVersion != "" {
		options = append(options, WithApiVersion(ApiVersion(config.ApiVersion)))
	}

	// Create the client
	client, err := NewClient(options...)
	if err != nil {
		t.Skipf("Failed to create test client: %v", err)
	}

	// Force authentication to ensure we have a fresh token
	req, err := client.newRequest()
	if err != nil {
		t.Skipf("Failed to create request: %v", err)
	}

	// Force token refresh
	_, err = req.ForceAuthentication()
	if err != nil {
		t.Skipf("Failed to authenticate: %v", err)
	}

	return client
}

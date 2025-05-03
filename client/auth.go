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
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Auth handles authentication and token caching
type Auth struct {
	Username string
	Password string
	Token    string
}

// HasCredentials checks if the Auth struct has valid credentials
func (a *Auth) HasCredentials() bool {
	return a.Username != "" && a.Password != ""
}

// ForceAuthentication forces authentication and token caching
func (c *Client) ForceAuthentication() (string, error) {
	// Delete any existing token for this host
	if err := DeleteCachedToken(c.Host); err != nil {
		Debug("Failed to delete existing token: %v", err)
	}

	// Get a new token
	token, err := c.requestToken()
	if err != nil {
		return "", err
	}

	// Cache the token
	if err := CacheToken(c.Host, token); err != nil {
		Debug("Failed to cache token: %v", err)
	}

	return token, nil
}

// requestToken requests a new authentication token
func (c *Client) requestToken() (string, error) {
	// Use the credentials from the client
	username := c.auth.Username
	password := c.auth.Password

	// Debug information
	Debug("Auth attempt with user: %s to URL: %s", username, c.Host)

	// Construct authentication URL
	baseURL := fmt.Sprintf("%s://%s", c.ApiVersion.GetScheme(), c.Host)
	authURL := fmt.Sprintf("%s/api/bmc/authenticate", baseURL)

	Debug("Auth URL: %s", authURL)

	// Create request body with username and password
	requestBody := map[string]string{
		"username": username,
		"password": password,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth request: %w", err)
	}

	Debug("Auth request body: %s", string(jsonBody))

	// Create a POST request with JSON body
	req, err := http.NewRequest(http.MethodPost, authURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}

	// Set Content-Type to application/json - this is critical
	req.Header.Set("Content-Type", "application/json")

	// Set User-Agent header
	osInfo := runtime.GOOS
	osVersion := runtime.Version()
	userAgent := fmt.Sprintf("TPI (%s;%s)", osInfo, osVersion)
	req.Header.Set("User-Agent", userAgent)

	// Create a client that ignores SSL certificate errors
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Skip certificate verification
		},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   3 * time.Second,
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send auth request: %w", err)
	}
	defer resp.Body.Close()

	// Handle non-200 response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		Debug("Auth failed with status: %d, body: %s", resp.StatusCode, string(body))

		if resp.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("authentication failed: invalid credentials")
		}

		return "", fmt.Errorf("authentication failed: %s", string(body))
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to parse auth response: %w", err)
	}

	Debug("Auth response: %+v", response)

	// Look for token in the "id" field
	tokenVal, ok := response["id"]
	if !ok {
		return "", fmt.Errorf("invalid auth response: missing id field")
	}

	token, ok := tokenVal.(string)
	if !ok {
		return "", fmt.Errorf("invalid auth response: id is not a string")
	}

	Debug("Successfully got auth token: %s", token)

	// Save token to cache
	if err := CacheToken(c.Host, token); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cache token for host %s: %v\n", c.Host, err)
	}

	return token, nil
}

// Login authenticates with the BMC and caches the token for future use
func (c *Client) Login() error {
	// Already have a token?
	if c.auth.Token != "" {
		return nil
	}

	// Try to use cached token for this specific host, if available
	token, err := GetCachedToken(c.Host)
	if err == nil && token != "" {
		c.auth.Token = token
		return nil
	}

	// If host-specific token not found, try legacy token (for backward compatibility)
	if c.Host != "" {
		legacyToken, legacyErr := getCachedToken()
		if legacyErr == nil {
			c.auth.Token = legacyToken
			return nil
		}
	}

	// Force authentication to get a new token
	token, err = c.ForceAuthentication()
	if err != nil {
		return err
	}

	// Store token in client
	c.auth.Token = token
	return nil
}

// getCacheFilePath returns the path to the cache file for a specific host
func getCacheFilePath(host string) string {
	var cacheDir string

	// Get cache directory based on OS
	switch runtime.GOOS {
	case "windows":
		cacheDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "tpi")
	case "darwin":
		homeDir, err := os.UserHomeDir()
		if err == nil {
			cacheDir = filepath.Join(homeDir, "Library", "Caches", "tpi")
		}
	default: // Linux and others
		homeDir, err := os.UserHomeDir()
		if err == nil {
			cacheDir = filepath.Join(homeDir, ".cache", "tpi")
		}
	}

	// Fallback to current directory if we couldn't determine the cache directory
	if cacheDir == "" {
		cacheDir = "."
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		// If we can't create the directory, just use current directory
		cacheDir = "."
	}

	// If no host is specified, use the default token path (for backward compatibility)
	if host == "" {
		return filepath.Join(cacheDir, "tpi_token")
	}

	// Create a sanitized version of the host for the filename
	safeHost := strings.ReplaceAll(host, ":", "_")
	safeHost = strings.ReplaceAll(safeHost, "/", "_")
	safeHost = strings.ReplaceAll(safeHost, ".", "_")

	return filepath.Join(cacheDir, fmt.Sprintf("tpi_token_%s", safeHost))
}

// CacheToken caches the token for a specific host
func CacheToken(host, token string) error {
	path := getCacheFilePath(host)
	err := os.WriteFile(path, []byte(token), 0600)
	if err != nil {
		return fmt.Errorf("failed to write token: %w", err)
	}
	return nil
}

// GetCachedToken returns the cached token for a specific host
func GetCachedToken(host string) (string, error) {
	path := getCacheFilePath(host)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DeleteCachedToken deletes the cached token for a specific host
func DeleteCachedToken(host string) error {
	path := getCacheFilePath(host)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// GetAllCachedTokens returns a list of all hosts with cached tokens
func GetAllCachedTokens() ([]string, error) {
	var cacheDir string

	// Get cache directory based on OS
	switch runtime.GOOS {
	case "windows":
		cacheDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "tpi")
	case "darwin":
		homeDir, err := os.UserHomeDir()
		if err == nil {
			cacheDir = filepath.Join(homeDir, "Library", "Caches", "tpi")
		}
	default: // Linux and others
		homeDir, err := os.UserHomeDir()
		if err == nil {
			cacheDir = filepath.Join(homeDir, ".cache", "tpi")
		}
	}

	// If we couldn't determine the cache directory or it doesn't exist, return empty list
	if cacheDir == "" {
		return []string{}, nil
	}

	files, err := os.ReadDir(cacheDir)
	if err != nil {
		// If the directory doesn't exist, return empty list
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var hosts []string
	for _, file := range files {
		name := file.Name()
		// Check for legacy token file
		if name == "tpi_token" {
			hosts = append(hosts, "default")
			continue
		}

		// Check for host-specific token files
		if strings.HasPrefix(name, "tpi_token_") {
			// Extract and un-sanitize host from filename
			host := strings.TrimPrefix(name, "tpi_token_")
			// We've sanitized dots, but for display we can't fully reverse it
			// Just show the sanitized name for consistency
			hosts = append(hosts, host)
		}
	}

	return hosts, nil
}

// DeleteAllCachedTokens deletes all cached tokens
func DeleteAllCachedTokens() error {
	hosts, err := GetAllCachedTokens()
	if err != nil {
		return err
	}

	var lastErr error
	for _, host := range hosts {
		hostToDelete := ""
		if host != "default" {
			hostToDelete = host
		}
		if err := DeleteCachedToken(hostToDelete); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// getCachedToken retrieves the cached token for the default host (legacy compatibility)
func getCachedToken() (string, error) {
	return GetCachedToken("")
}

// deleteCachedToken deletes the token for the default host (legacy compatibility)
func deleteCachedToken() error {
	return DeleteCachedToken("")
}

// cacheToken caches the token for the default host (legacy compatibility)
func cacheToken(token string) error {
	return CacheToken("", token)
}

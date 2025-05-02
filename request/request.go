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

package request

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/davidroman0O/tpi/cli"
)

type Request struct {
	URL         *url.URL
	Host        string
	Version     cli.ApiVersion
	Credentials struct {
		Username string
		Password string
	}
	Method        string
	Headers       map[string]string
	QueryParams   url.Values
	MultipartForm *bytes.Buffer
	ContentType   string
	UserAgent     string
}

// NewRequest creates a new request with the given host and API version
func NewRequest(host string, version cli.ApiVersion, username, password string) (*Request, error) {
	scheme := version.GetScheme()

	// Construct the URL
	urlStr := fmt.Sprintf("%s://%s/api/bmc", scheme, host)
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Get system info for user agent
	osInfo := runtime.GOOS
	osVersion := runtime.Version()
	userAgent := fmt.Sprintf("TPI (%s;%s)", osInfo, osVersion)

	req := &Request{
		URL:         parsedURL,
		Host:        host,
		Version:     version,
		Method:      http.MethodGet,
		Headers:     make(map[string]string),
		QueryParams: url.Values{},
		UserAgent:   userAgent,
	}

	req.Credentials.Username = username
	req.Credentials.Password = password

	// Set default headers
	req.Headers["User-Agent"] = userAgent

	return req, nil
}

// Clone creates a deep copy of the request
func (r *Request) Clone() *Request {
	clone := &Request{
		URL:         &url.URL{},
		Host:        r.Host,
		Version:     r.Version,
		Method:      r.Method,
		Headers:     make(map[string]string),
		QueryParams: url.Values{},
		UserAgent:   r.UserAgent,
	}

	// Clone URL
	*clone.URL = *r.URL

	// Clone credentials
	clone.Credentials.Username = r.Credentials.Username
	clone.Credentials.Password = r.Credentials.Password

	// Clone headers
	for k, v := range r.Headers {
		clone.Headers[k] = v
	}

	// Clone query params
	for k, v := range r.QueryParams {
		clone.QueryParams[k] = v
	}

	// Clone multipart form if present
	if r.MultipartForm != nil {
		clone.MultipartForm = bytes.NewBuffer(r.MultipartForm.Bytes())
		clone.ContentType = r.ContentType
	}

	return clone
}

// ToPost converts the request to a POST request
func (r *Request) ToPost() *Request {
	clone := r.Clone()
	clone.Method = http.MethodPost
	return clone
}

// SetMultipartForm sets the request's body to a multipart form
func (r *Request) SetMultipartForm(form *bytes.Buffer, contentType string) {
	r.MultipartForm = form
	r.ContentType = contentType
}

// GetURL returns the request's URL with query parameters
func (r *Request) GetURL() string {
	u := *r.URL
	u.RawQuery = r.QueryParams.Encode()
	return u.String()
}

// AddQueryParam adds a query parameter to the request
func (r *Request) AddQueryParam(key, value string) {
	r.QueryParams.Add(key, value)
}

// Send sends the request and returns the response
func (r *Request) Send() (*http.Response, error) {
	// Check if we already have a cached token for this host
	// and authenticate immediately if so
	authenticated := false
	_, tokenErr := GetCachedTokenForHost(r.Host)
	if tokenErr == nil {
		// We already have a token, use it right away
		authenticated = true
		fmt.Printf("DEBUG: Found cached token for %s, using it for first request\n", r.Host)
	}

	fmt.Printf("DEBUG: Send request to URL: %s\n", r.GetURL())
	fmt.Printf("DEBUG: Request headers: %v\n", r.Headers)
	fmt.Printf("DEBUG: Request method: %s\n", r.Method)

	// Create a client that ignores SSL certificate errors
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Skip certificate verification
		},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   3 * time.Second, // Reduced timeout for better user experience
	}

	var resp *http.Response

	for {
		// Create a new request
		var reqBody io.Reader
		if r.MultipartForm != nil {
			reqBody = r.MultipartForm
		}

		req, err := http.NewRequest(r.Method, r.GetURL(), reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers
		for k, v := range r.Headers {
			req.Header.Set(k, v)
		}

		// Set content type if specified
		if r.ContentType != "" {
			req.Header.Set("Content-Type", r.ContentType)
		}

		// Add authentication if required
		if authenticated {
			token, err := r.getBearerToken()
			if err != nil {
				return nil, fmt.Errorf("failed to get bearer token: %w", err)
			}

			// Set Authorization header with Bearer prefix - match Rust implementation
			// The Rust code uses builder.bearer_auth(token) which adds the "Bearer " prefix
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			fmt.Printf("DEBUG: Setting Authorization header with Bearer prefix: Bearer %s\n", token)
		}

		// Send the request
		resp, err = client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}

		fmt.Printf("DEBUG: Response status: %d\n", resp.StatusCode)

		// If unauthorized and not already authenticated, try again with authentication
		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()

			if authenticated {
				// We got a 401 despite using a token, so the token is likely invalid
				fmt.Printf("DEBUG: Got 401 Unauthorized with a token, token may be expired. Deleting cached token.\n")
				DeleteCachedTokenForHost(r.Host)
			}

			if !authenticated {
				fmt.Printf("DEBUG: Got 401 Unauthorized, trying again with authentication\n")
				authenticated = true
				continue
			} else {
				// We already tried with authentication and still got 401
				return resp, nil
			}
		}

		break
	}

	return resp, nil
}

// getBearerToken retrieves the bearer token for authentication
func (r *Request) getBearerToken() (string, error) {
	// First try to use cached token for this specific host, if available
	token, err := GetCachedTokenForHost(r.Host)
	if err == nil {
		return token, nil
	}

	// If host-specific token not found, try legacy token (for backward compatibility)
	if r.Host != "" {
		legacyToken, legacyErr := getCachedToken()
		if legacyErr == nil {
			return legacyToken, nil
		}
	}

	// If credentials are explicitly provided, use them
	if r.Credentials.Username != "" && r.Credentials.Password != "" {
		return r.requestToken()
	}

	// Try with default credentials as a last resort
	originalUsername := r.Credentials.Username
	originalPassword := r.Credentials.Password

	// Try common default credentials
	credentialPairs := []struct{ username, password string }{
		{"root", ""},             // Empty password
		{"root", "turing"},       // Default Turing Pi password
		{"root", "root"},         // Common default
		{"admin", "admin"},       // Common default
		{"turingpi", "turingpi"}, // Product-specific
	}

	var lastErr error
	for _, creds := range credentialPairs {
		r.Credentials.Username = creds.username
		r.Credentials.Password = creds.password

		token, err := r.requestToken()
		if err == nil {
			return token, nil
		}
		lastErr = err
	}

	// Restore original credentials
	r.Credentials.Username = originalUsername
	r.Credentials.Password = originalPassword

	return "", fmt.Errorf("failed to authenticate with any credentials: %w", lastErr)
}

// requestToken requests a new authentication token
func (r *Request) requestToken() (string, error) {
	// Use the credentials that were already set
	username := r.Credentials.Username
	password := r.Credentials.Password

	// Debug information - print auth attempt details
	fmt.Printf("DEBUG: Auth attempt with user: %s to URL: %s\n", username, r.Host)

	// Construct authentication URL - MATCH EXACTLY with Rust implementation
	// The Rust code:
	// 1. Calls url_from_host which sets path to "api/bmc"
	// 2. Then adds "authenticate" to that path
	baseURL := fmt.Sprintf("%s://%s", r.Version.GetScheme(), r.Host)
	authURL := fmt.Sprintf("%s/api/bmc/authenticate", baseURL)

	fmt.Printf("DEBUG: Auth URL: %s\n", authURL)

	// Create request body exactly as in Rust, with just username and password
	requestBody := map[string]string{
		"username": username,
		"password": password,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth request: %w", err)
	}

	fmt.Printf("DEBUG: Auth request body: %s\n", string(jsonBody))

	// Create a POST request with JSON body
	req, err := http.NewRequest(http.MethodPost, authURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}

	// Set Content-Type to application/json - this is critical
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", r.UserAgent)

	// Create a client that ignores SSL certificate errors
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Skip certificate verification
		},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   3 * time.Second, // Reduced timeout for better user experience
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send auth request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("DEBUG: Auth failed with status: %d, body: %s\n", resp.StatusCode, string(body))

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

	fmt.Printf("DEBUG: Auth response: %+v\n", response)

	// Look for token in the "id" field like Rust
	tokenVal, ok := response["id"]
	if !ok {
		return "", fmt.Errorf("invalid auth response: missing id field")
	}

	token, ok := tokenVal.(string)
	if !ok {
		return "", fmt.Errorf("invalid auth response: id is not a string")
	}

	fmt.Printf("DEBUG: Successfully got auth token: %s\n", token)

	// Save token to cache - use host-specific caching
	if err := CacheTokenForHost(r.Host, token); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cache token for host %s: %v\n", r.Host, err)
	}

	return token, nil
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

// getCachedToken retrieves the cached token for the current host
func getCachedToken() (string, error) {
	return GetCachedTokenForHost("")
}

// GetCachedTokenForHost retrieves the cached token for the specified host
func GetCachedTokenForHost(host string) (string, error) {
	path := getCacheFilePath(host)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// deleteCachedToken deletes the cached token for the current host
func deleteCachedToken() {
	DeleteCachedTokenForHost("")
}

// DeleteCachedTokenForHost deletes the cached token for the specified host
func DeleteCachedTokenForHost(host string) error {
	path := getCacheFilePath(host)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// cacheToken caches the token for the current host
func cacheToken(token string) error {
	return CacheTokenForHost("", token)
}

// CacheTokenForHost caches the token for the specified host
func CacheTokenForHost(host, token string) error {
	path := getCacheFilePath(host)
	err := os.WriteFile(path, []byte(token), 0600)
	if err != nil {
		return fmt.Errorf("failed to write token: %w", err)
	}
	return nil
}

// GetAllCachedTokenHosts returns a list of all hosts that have cached tokens
func GetAllCachedTokenHosts() ([]string, error) {
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
	hosts, err := GetAllCachedTokenHosts()
	if err != nil {
		return err
	}

	var lastErr error
	for _, host := range hosts {
		hostToDelete := ""
		if host != "default" {
			hostToDelete = host
		}
		if err := DeleteCachedTokenForHost(hostToDelete); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// ForceAuthentication forces authentication and token caching
func (r *Request) ForceAuthentication() (string, error) {
	// Delete any existing token
	DeleteCachedTokenForHost(r.Host)

	// Get and cache a new token
	token, err := r.requestToken()
	if err != nil {
		return "", err
	}

	// Cache the token with the specific host
	if err := CacheTokenForHost(r.Host, token); err != nil {
		fmt.Printf("DEBUG: Failed to cache token: %v\n", err)
	}

	return token, nil
}

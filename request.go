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
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"time"
)

// Debug logs a debug message if debugging is enabled
// Only prints when TPI_DEBUG env var is set to "true"
func Debug(format string, args ...interface{}) {
	if os.Getenv("TPI_DEBUG") == "true" {
		fmt.Printf("DEBUG: "+format+"\n", args...)
	}
}

// Request represents an HTTP request for the Turing Pi API
type Request struct {
	URL         *url.URL
	Host        string
	Version     ApiVersion
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
	Timeout       time.Duration   // Custom timeout for this request
	Context       context.Context // Context for the request
}

// NewRequest creates a new request with the given host and API version
func NewRequest(host string, version ApiVersion, username, password string) (*Request, error) {
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
		Timeout:     0, // Use default timeout
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
		Timeout:     r.Timeout, // Copy timeout
		Context:     r.Context, // Copy context
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

// Debug logs a debug message if debugging is enabled
func (r *Request) Debug(format string, args ...interface{}) {
	// Use the package-level Debug function
	Debug(format, args...)
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
	_, tokenErr := GetCachedToken(r.Host)
	if tokenErr == nil {
		// We already have a token, use it right away
		authenticated = true
		r.Debug("Found cached token for %s, using it for first request", r.Host)
	}

	r.Debug("Send request to URL: %s", r.GetURL())
	r.Debug("Request headers: %v", r.Headers)
	r.Debug("Request method: %s", r.Method)

	// Create a client that ignores SSL certificate errors
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Skip certificate verification
		},
	}

	// Use custom timeout if set, otherwise use default
	timeout := 3 * time.Second // Default timeout
	if r.Timeout > 0 {
		timeout = r.Timeout
	}

	if r.Timeout > 0 {
		r.Debug("Using custom timeout of %s", r.Timeout)
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	var resp *http.Response

	for {
		// Create a new request
		var reqBody io.Reader
		if r.MultipartForm != nil {
			reqBody = r.MultipartForm
		}

		var req *http.Request
		var err error

		// Use context if available
		if r.Context != nil {
			req, err = http.NewRequestWithContext(r.Context, r.Method, r.GetURL(), reqBody)
			r.Debug("Creating request with context")
		} else {
			req, err = http.NewRequest(r.Method, r.GetURL(), reqBody)
		}

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

			// Set Authorization header with Bearer prefix
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			r.Debug("Setting Authorization header with Bearer prefix: Bearer %s", token)
		}

		// Send the request
		resp, err = client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}

		r.Debug("Response status: %d", resp.StatusCode)

		// If unauthorized and not already authenticated, try again with authentication
		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()

			if authenticated {
				// We got a 401 despite using a token, so the token is likely invalid
				r.Debug("Got 401 Unauthorized with a token, token may be expired. Deleting cached token.")
				DeleteCachedToken(r.Host)
			}

			if !authenticated {
				r.Debug("Got 401 Unauthorized, trying again with authentication")
				authenticated = true
				continue
			} else {
				// We already tried with authentication and still got 401
				return resp, nil
			}
		}

		// No further authentication needed, return the response
		return resp, nil
	}
}

// getBearerToken retrieves the bearer token for authentication
func (r *Request) getBearerToken() (string, error) {
	// First try to use cached token for this specific host, if available
	token, err := GetCachedToken(r.Host)
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

	// Debug information
	r.Debug("Auth attempt with user: %s to URL: %s", username, r.Host)

	// Construct authentication URL
	baseURL := fmt.Sprintf("%s://%s", r.Version.GetScheme(), r.Host)
	authURL := fmt.Sprintf("%s/api/bmc/authenticate", baseURL)

	r.Debug("Auth URL: %s", authURL)

	// Create request body with username and password
	requestBody := map[string]string{
		"username": username,
		"password": password,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth request: %w", err)
	}

	r.Debug("Auth request body: %s", string(jsonBody))

	// Create a POST request with JSON body
	req, err := http.NewRequest(http.MethodPost, authURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}

	// Set Content-Type to application/json
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
		Timeout:   3 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send auth request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		r.Debug("Auth failed with status: %d, body: %s", resp.StatusCode, string(body))

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

	r.Debug("Auth response: %+v", response)

	// Look for token in the "id" field
	tokenVal, ok := response["id"]
	if !ok {
		return "", fmt.Errorf("invalid auth response: missing id field")
	}

	token, ok := tokenVal.(string)
	if !ok {
		return "", fmt.Errorf("invalid auth response: id is not a string")
	}

	r.Debug("Successfully got auth token: %s", token)

	// Save token to cache
	if err := CacheToken(r.Host, token); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cache token for host %s: %v\n", r.Host, err)
	}

	return token, nil
}

// ForceAuthentication forces authentication and token caching
func (r *Request) ForceAuthentication() (string, error) {
	// Delete any existing token
	DeleteCachedToken(r.Host)

	// Get and cache a new token
	token, err := r.requestToken()
	if err != nil {
		return "", err
	}

	// Cache the token with the specific host
	if err := CacheToken(r.Host, token); err != nil {
		fmt.Printf("DEBUG: Failed to cache token: %v\n", err)
	}

	return token, nil
}

// requestBuilder builds HTTP requests
type requestBuilder struct {
	client      *Client
	path        string
	queryParams map[string]string
	err         error
}

// setPath sets the path for the request
func (rb *requestBuilder) setPath(path string) *requestBuilder {
	rb.path = path
	return rb
}

// addQueryParam adds a query parameter to the request
func (rb *requestBuilder) addQueryParam(key, value string) *requestBuilder {
	if rb.queryParams == nil {
		rb.queryParams = make(map[string]string)
	}
	rb.queryParams[key] = value
	return rb
}

// build builds the HTTP request
func (rb *requestBuilder) build() (*http.Request, error) {
	if rb.err != nil {
		return nil, rb.err
	}

	// Construct URL
	scheme := rb.client.ApiVersion.GetScheme()
	urlStr := fmt.Sprintf("%s://%s%s", scheme, rb.client.Host, rb.path)

	// Add query parameters if any
	if len(rb.queryParams) > 0 {
		params := url.Values{}
		for k, v := range rb.queryParams {
			params.Add(k, v)
		}
		urlStr = fmt.Sprintf("%s?%s", urlStr, params.Encode())
	}

	// Create request
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}

	// Add token if available
	if rb.client.auth.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", rb.client.auth.Token))
	}

	return req, nil
}

// extractResultObject extracts the result object from the response
func extractResultObject(resp *http.Response) (map[string]interface{}, error) {
	// Try to parse as the common expected structure
	var result struct {
		Result map[string]interface{} `json:"result"`
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Since we read the body, create a new reader for additional parsing attempts
	resp.Body = ioutil.NopCloser(bytes.NewReader(body))

	// First try the simple structure
	if err := json.Unmarshal(body, &result); err == nil && len(result.Result) > 0 {
		return result.Result, nil
	}

	// If that fails, try the nested structure used by the power status endpoint
	var nestedResult struct {
		Response []struct {
			Result []map[string]interface{} `json:"result"`
		} `json:"response"`
	}

	if err := json.Unmarshal(body, &nestedResult); err != nil {
		return nil, fmt.Errorf("failed to decode nested response: %w", err)
	}

	// Check if we have a valid nested structure
	if len(nestedResult.Response) > 0 && len(nestedResult.Response[0].Result) > 0 {
		// Return the first result object
		return nestedResult.Response[0].Result[0], nil
	}

	// Fall back to an empty map if we couldn't parse anything useful
	log.Printf("WARNING: Could not extract result from response: %s", string(body))
	return map[string]interface{}{}, nil
}

// extractResultValue extracts a specific value from the result
func extractResultValue(resp *http.Response, key string) (interface{}, error) {
	result, err := extractResultObject(resp)
	if err != nil {
		return nil, err
	}

	value, ok := result[key]
	if !ok {
		return nil, fmt.Errorf("key %s not found in result", key)
	}

	return value, nil
}

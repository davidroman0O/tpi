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

// Package tpi provides a client for interacting with Turing Pi BMC API
package tpi

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	// DefaultTimeout is the default timeout for HTTP requests
	DefaultTimeout = 10 * time.Second

	// DefaultRetries is the default number of retries for HTTP requests
	DefaultRetries = 3

	// DefaultRetryWait is the default wait time between retries
	DefaultRetryWait = 1 * time.Second
)

// Client is the main interface for interacting with a Turing Pi board
type Client struct {
	Host       string
	ApiVersion ApiVersion
	httpClient *http.Client
	auth       *Auth
	mu         sync.Mutex
}

// NewClient creates a new Turing Pi client with the provided options
func NewClient(options ...Option) (*Client, error) {
	// Default client options
	client := &Client{
		ApiVersion: ApiVersionV1_1, // Default to v1-1
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // Skip cert verification
				},
			},
		},
		auth: &Auth{},
	}

	// Apply options
	for _, option := range options {
		option(client)
	}

	// Validate client configuration
	if client.Host == "" {
		return nil, fmt.Errorf("host is required")
	}

	return client, nil
}

// Option is a function that configures a Client
type Option func(*Client)

// WithHost sets the client host
func WithHost(host string) Option {
	return func(c *Client) {
		c.Host = host
	}
}

// WithApiVersion sets the API version
func WithApiVersion(version ApiVersion) Option {
	return func(c *Client) {
		c.ApiVersion = version
	}
}

// WithCredentials sets the credentials for authentication
func WithCredentials(username, password string) Option {
	return func(c *Client) {
		c.auth.Username = username
		c.auth.Password = password
	}
}

// WithTimeout sets the client timeout
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// newRequest creates a new HTTP request
func (c *Client) newRequest() (*Request, error) {
	// Check if we have a cached token for this host
	hasCachedToken := false
	if c.Host != "" {
		_, err := GetCachedToken(c.Host)
		if err == nil {
			hasCachedToken = true
			Debug("Found cached token for host %s", c.Host)
		}
	}

	// Only require explicit credentials if we don't have a cached token
	if !hasCachedToken && (c.auth == nil || !c.auth.HasCredentials()) {
		return nil, fmt.Errorf("no credentials provided")
	}

	// Create a new request
	req, err := NewRequest(c.Host, c.ApiVersion, c.auth.Username, c.auth.Password)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// checkResponseError checks if a response contains an error
func checkResponseError(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Try to decode response to check for error field
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// If we can't decode the response, assume it's not an error
		return nil
	}

	// Check if there's an error in the response
	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Errorf("server returned error: %s", errMsg)
	}

	return nil
}

// Info returns the basic information about the Turing Pi
func (c *Client) Info() (map[string]string, error) {
	req, err := c.newRequest()
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "get")
	req.AddQueryParam("type", "other")

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Extract the result
	result, err := extractResultObject(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to extract result: %w", err)
	}

	// Convert the result to a map[string]string
	info := make(map[string]string)
	for key, value := range result {
		if strVal, ok := value.(string); ok {
			info[key] = strVal
		}
	}

	return info, nil
}

// Reboot reboots the BMC. Warning: Nodes will lose power until booted!
func (c *Client) Reboot() error {
	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "reboot")

	// Send the request with auto-retry on auth failures
	var resp *http.Response

	// First try with any cached token
	resp, err = req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// If we get unauthorized, try to force authentication and retry
	if resp.StatusCode == http.StatusUnauthorized {
		// Delete the cached token which is causing the 401
		DeleteCachedToken(c.Host)

		// Force re-authentication
		req, err = c.newRequest()
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.AddQueryParam("opt", "set")
		req.AddQueryParam("type", "reboot")

		// Force authentication before sending
		if _, authErr := req.ForceAuthentication(); authErr != nil {
			return fmt.Errorf("authentication failed: %w", authErr)
		}

		// Retry the request with the new token
		resp, err = req.Send()
		if err != nil {
			return fmt.Errorf("failed to send request after re-authentication: %w", err)
		}
		defer resp.Body.Close()
	}

	// Check for errors in the response
	if err := checkResponseError(resp); err != nil {
		return fmt.Errorf("reboot failed: %w", err)
	}

	return nil
}

// RebootAndWait reboots the BMC and waits for it to come back online.
// It uses exponential backoff when checking the BMC status.
// The timeout is in seconds.
func (c *Client) RebootAndWait(timeout int) error {
	// First reboot the BMC
	if err := c.Reboot(); err != nil {
		return err
	}

	// Wait a bit before starting to check
	time.Sleep(5 * time.Second)

	// Start time
	startTime := time.Now()
	timeoutDuration := time.Duration(timeout) * time.Second

	// Retry interval starts at 1 second, will gradually increase
	retryInterval := time.Second

	// Setup progress indicator
	progressChar := "."
	attempts := 0
	lastProgressUpdate := time.Now()
	progressInterval := 1 * time.Second

	for {
		// Check if we've exceeded the timeout
		if time.Since(startTime) > timeoutDuration {
			return fmt.Errorf("timeout reached: BMC did not respond within %d seconds", timeout)
		}

		// Print progress indicator at regular intervals
		if time.Since(lastProgressUpdate) >= progressInterval {
			fmt.Print(progressChar)
			lastProgressUpdate = time.Now()
		}

		// Try to connect to the BMC
		_, err := c.Info()
		if err == nil {
			return nil // BMC is back online
		}

		// Exponential backoff with a maximum of 5 seconds
		retryInterval = time.Duration(float64(retryInterval) * 1.5)
		if retryInterval > 5*time.Second {
			retryInterval = 5 * time.Second
		}

		attempts++
		time.Sleep(retryInterval)
	}
}

// About returns detailed information about the BMC daemon
func (c *Client) About() (map[string]string, error) {
	req, err := c.newRequest()
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters for the about endpoint
	req.AddQueryParam("opt", "get")
	req.AddQueryParam("type", "about")

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Handle the specific response format for the about endpoint
	var responseData struct {
		Response []struct {
			Result map[string]string `json:"result"`
		} `json:"response"`
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the JSON
	if err := json.Unmarshal(body, &responseData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if we have valid data
	if len(responseData.Response) == 0 || responseData.Response[0].Result == nil {
		return nil, fmt.Errorf("invalid response format")
	}

	// Return the result map
	return responseData.Response[0].Result, nil
}

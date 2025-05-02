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
	"time"
)

// Client is the main interface for interacting with a Turing Pi board
type Client struct {
	Host       string
	ApiVersion ApiVersion
	httpClient *http.Client
	auth       *Auth
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

// newRequest creates a new Request object for this client
func (c *Client) newRequest() (*Request, error) {
	return NewRequest(c.Host, c.ApiVersion, c.auth.Username, c.auth.Password)
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

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors in the response
	if err := checkResponseError(resp); err != nil {
		return fmt.Errorf("reboot failed: %w", err)
	}

	return nil
}

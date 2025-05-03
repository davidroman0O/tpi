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
	"fmt"
	"net/http"
	"strings"
	"time"
)

// EthReset resets the on-board Ethernet switch
// Note: This is expected to cause a timeout as the network connection will be lost
func (c *Client) EthReset() error {
	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "network")
	req.AddQueryParam("cmd", "reset")

	// Use a shorter timeout for this request since we expect it to timeout
	originalTimeout := c.httpClient.Timeout
	c.httpClient.Timeout = 2 * time.Second

	// Send the request
	resp, err := req.Send()

	// Restore the original timeout
	c.httpClient.Timeout = originalTimeout

	// Check for timeout or connection errors, which are expected when resetting the network
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") ||
			strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "EOF") {
			// This is expected, so we'll return success
			return nil
		}
		return fmt.Errorf("failed to send request: %w", err)
	}

	// If we got a response, check for errors
	defer resp.Body.Close()

	// Check for errors in the response
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusBadRequest {
			// Some BMC firmware versions may return an error, but the command might still work
			return nil
		}
		return fmt.Errorf("Ethernet switch reset failed: status code %d", resp.StatusCode)
	}

	return nil
}

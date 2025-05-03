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
	"strconv"
	"strings"
	"time"
)

// SetNodeNormalMode sets the specified node to normal mode (clears any advanced mode)
// and resets the node
func (c *Client) SetNodeNormalMode(node int) error {
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d (must be 1-4)", node)
	}

	// First, clear USB boot
	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "clear_usb_boot")
	req.AddQueryParam("node", strconv.Itoa(node-1)) // BMC uses 0-based indexing

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors in the response
	if err := checkResponseError(resp); err != nil {
		return fmt.Errorf("failed to clear USB boot: %w", err)
	}

	// Then, reset the node to apply changes
	return c.PowerReset(node)
}

// SetNodeMsdMode puts the specified node into Mass Storage Device mode
// This reboots supported compute modules and exposes its eMMC storage as a mass storage device
func (c *Client) SetNodeMsdMode(node int) error {
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d (must be 1-4)", node)
	}

	// Create a request with a longer timeout specifically for MSD mode
	// which takes longer to complete
	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Increase the timeout for this operation, as it can take longer
	req.Timeout = 60 * time.Second

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "node_to_msd")
	req.AddQueryParam("node", strconv.Itoa(node-1)) // BMC uses 0-based indexing

	// Send the request with auto-retry on auth failures
	var resp *http.Response

	// First try with any cached token
	resp, err = req.Send()
	if err != nil {
		// If send failed, try force authentication and retry once
		if isTimeoutError(err) {
			fmt.Printf("MSD mode operation taking longer than expected. Retrying with longer timeout...\n")

			// Create a new request with even longer timeout
			req, err = c.newRequest()
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}

			// Try with a much longer timeout
			req.Timeout = 120 * time.Second

			req.AddQueryParam("opt", "set")
			req.AddQueryParam("type", "node_to_msd")
			req.AddQueryParam("node", strconv.Itoa(node-1))

			// Force authentication to ensure we have a valid token
			if _, authErr := req.ForceAuthentication(); authErr != nil {
				return fmt.Errorf("authentication failed: %w", authErr)
			}

			// Retry the request
			resp, err = req.Send()
			if err != nil {
				return fmt.Errorf("failed to send request: %w", err)
			}
		} else {
			return fmt.Errorf("failed to send request: %w", err)
		}
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

		// Maintain the longer timeout
		req.Timeout = 60 * time.Second

		req.AddQueryParam("opt", "set")
		req.AddQueryParam("type", "node_to_msd")
		req.AddQueryParam("node", strconv.Itoa(node-1))

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
		return fmt.Errorf("failed to set MSD mode: %w", err)
	}

	fmt.Println("Setting node to MSD mode. This may take up to a minute...")
	return nil
}

// Helper function to determine if an error is a timeout error
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "deadline exceeded")
}

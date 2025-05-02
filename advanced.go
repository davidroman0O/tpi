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
	"strconv"
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

	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "node_to_msd")
	req.AddQueryParam("node", strconv.Itoa(node-1)) // BMC uses 0-based indexing

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors in the response
	if err := checkResponseError(resp); err != nil {
		return fmt.Errorf("failed to set MSD mode: %w", err)
	}

	return nil
}

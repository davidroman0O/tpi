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
	"strings"
)

// PowerStatus returns the power status of all nodes
func (c *Client) PowerStatus() (map[int]bool, error) {
	req, err := c.newRequest()
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "get")
	req.AddQueryParam("type", "power")

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	result, err := extractResultObject(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to extract result: %w", err)
	}

	// The result is a map of node to power status
	// node1, node2, node3, node4 => 0 or 1
	status := make(map[int]bool)

	for key, value := range result {
		// Check if this is a node key
		if strings.HasPrefix(key, "node") {
			// Extract the node number
			nodeStr := strings.TrimPrefix(key, "node")
			nodeNum, err := strconv.Atoi(nodeStr)
			if err != nil {
				continue // Skip invalid node numbers
			}

			// Convert value to bool: 1 = on, 0 = off
			powerOn := false
			switch v := value.(type) {
			case float64:
				powerOn = v > 0
			case int:
				powerOn = v > 0
			case string:
				powerOn = v == "1" || strings.ToLower(v) == "on"
			}

			status[nodeNum] = powerOn
		}
	}

	return status, nil
}

// PowerOn turns on the specified node
func (c *Client) PowerOn(node int) error {
	return c.setPowerState(node, true)
}

// PowerOff turns off the specified node
func (c *Client) PowerOff(node int) error {
	return c.setPowerState(node, false)
}

// PowerReset resets the specified node
func (c *Client) PowerReset(node int) error {
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d (must be between 1 and 4)", node)
	}

	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "reset")
	req.AddQueryParam("node", strconv.Itoa(node-1)) // API expects 0-based index

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors in the response
	if err := checkResponseError(resp); err != nil {
		return fmt.Errorf("reset failed: %w", err)
	}

	return nil
}

// setPowerState sets the power state of the specified node
func (c *Client) setPowerState(node int, powerOn bool) error {
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d (must be between 1 and 4)", node)
	}

	// Set power state
	powerState := "0"
	if powerOn {
		powerState = "1"
	}

	// Build the node parameter name (node1, node2, etc.)
	nodeParam := fmt.Sprintf("node%d", node)

	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "power")
	req.AddQueryParam(nodeParam, powerState)

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors in the response
	if err := checkResponseError(resp); err != nil {
		return fmt.Errorf("power state change failed: %w", err)
	}

	return nil
}

// PowerOnAll turns on all nodes
func (c *Client) PowerOnAll() error {
	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "power")
	req.AddQueryParam("node1", "1")
	req.AddQueryParam("node2", "1")
	req.AddQueryParam("node3", "1")
	req.AddQueryParam("node4", "1")

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors in the response
	if err := checkResponseError(resp); err != nil {
		return fmt.Errorf("power on all failed: %w", err)
	}

	return nil
}

// PowerOffAll turns off all nodes
func (c *Client) PowerOffAll() error {
	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "power")
	req.AddQueryParam("node1", "0")
	req.AddQueryParam("node2", "0")
	req.AddQueryParam("node3", "0")
	req.AddQueryParam("node4", "0")

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors in the response
	if err := checkResponseError(resp); err != nil {
		return fmt.Errorf("power off all failed: %w", err)
	}

	return nil
}

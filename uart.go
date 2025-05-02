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
	"encoding/json"
	"fmt"
	"strconv"
)

// GetUartOutput gets the UART output from the specified node
func (c *Client) GetUartOutput(node int) (string, error) {
	if node < 1 || node > 4 {
		return "", fmt.Errorf("invalid node number: %d (must be 1-4)", node)
	}

	req, err := c.newRequest()
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "get")
	req.AddQueryParam("type", "uart")
	req.AddQueryParam("node", strconv.Itoa(node-1)) // BMC uses 0-based indexing

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// For UART output, we need to parse the response differently
	// as it has a "response" key with an array that contains the output
	var respData struct {
		Response []interface{} `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if we have any data
	if len(respData.Response) == 0 {
		return "", nil
	}

	// Extract the output from the response
	// It could be in different formats depending on the API version
	if str, ok := respData.Response[0].(string); ok {
		return str, nil
	}

	// If it's not a string, try to extract from an object
	if obj, ok := respData.Response[0].(map[string]interface{}); ok {
		if output, ok := obj["output"].(string); ok {
			return output, nil
		}
	}

	// If we can't extract the output, try to convert the whole response to JSON
	outputJSON, err := json.Marshal(respData.Response)
	if err != nil {
		return "", fmt.Errorf("failed to extract UART output: %w", err)
	}

	return string(outputJSON), nil
}

// SendUartCommand sends a command to the specified node over UART
func (c *Client) SendUartCommand(node int, command string) error {
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d (must be 1-4)", node)
	}

	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "uart")
	req.AddQueryParam("node", strconv.Itoa(node-1)) // BMC uses 0-based indexing
	req.AddQueryParam("cmd", command)

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors in the response
	if err := checkResponseError(resp); err != nil {
		return fmt.Errorf("UART command failed: %w", err)
	}

	return nil
}

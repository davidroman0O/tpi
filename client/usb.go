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
	"net/http"
	"strconv"
)

// extractResultArray extracts an array result from the response
func extractResultArray(resp *http.Response) ([]interface{}, error) {
	// Parse the response
	var respData struct {
		Result []interface{} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return respData.Result, nil
}

// UsbGetStatus returns the current USB configuration
func (c *Client) UsbGetStatus() (*UsbStatusInfo, error) {
	req, err := c.newRequest()
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "get")
	req.AddQueryParam("type", "usb")

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	resultArray, err := extractResultArray(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to extract result: %w", err)
	}

	if len(resultArray) == 0 {
		return nil, fmt.Errorf("no USB status information available")
	}

	// Extract the USB status from the first entry in the array
	statusMap, ok := resultArray[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid USB status entry format")
	}

	// Extract node, mode, and route
	node, ok := statusMap["node"].(string)
	if !ok {
		return nil, fmt.Errorf("missing node in USB status")
	}

	mode, ok := statusMap["mode"].(string)
	if !ok {
		return nil, fmt.Errorf("missing mode in USB status")
	}

	route, ok := statusMap["route"].(string)
	if !ok {
		return nil, fmt.Errorf("missing route in USB status")
	}

	return &UsbStatusInfo{
		Node:  node,
		Mode:  mode,
		Route: route,
	}, nil
}

// UsbSetHost configures the specified node as USB host
func (c *Client) UsbSetHost(node int, bmc bool) error {
	return c.usbSetMode(node, UsbHost, bmc)
}

// UsbSetDevice configures the specified node as USB device
func (c *Client) UsbSetDevice(node int, bmc bool) error {
	return c.usbSetMode(node, UsbDevice, bmc)
}

// UsbSetFlash configures the specified node in flash mode
func (c *Client) UsbSetFlash(node int, bmc bool) error {
	return c.usbSetMode(node, UsbFlash, bmc)
}

// usbSetMode configures the USB mode for the specified node
func (c *Client) usbSetMode(node int, mode UsbCmd, bmc bool) error {
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d (must be between 1 and 4)", node)
	}

	// Convert mode to numeric value
	var modeVal uint8
	switch mode {
	case UsbHost:
		modeVal = 0
	case UsbDevice:
		modeVal = 1
	case UsbFlash:
		modeVal = 2
	default:
		return fmt.Errorf("invalid USB mode: %s", mode)
	}

	// Add BMC bit if needed
	if bmc {
		modeVal |= 1 << 2
	}

	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "usb")
	req.AddQueryParam("node", strconv.Itoa(node-1)) // API expects 0-based index
	req.AddQueryParam("mode", strconv.Itoa(int(modeVal)))

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors in the response
	if err := checkResponseError(resp); err != nil {
		return fmt.Errorf("USB configuration failed: %w", err)
	}

	return nil
}

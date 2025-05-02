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

// CoolingDevice represents a cooling device (fan)
type CoolingDevice struct {
	Name     string
	Speed    uint
	MaxSpeed uint
}

// GetCoolingStatus returns the status of all cooling devices
func (c *Client) GetCoolingStatus() ([]CoolingDevice, error) {
	req, err := c.newRequest()
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "get")
	req.AddQueryParam("type", "cooling")

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Parse the response
	var respData struct {
		Response []map[string]interface{} `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if we have any data
	if len(respData.Response) == 0 {
		return []CoolingDevice{}, nil
	}

	// Parse the cooling devices
	var devices []CoolingDevice

	for _, item := range respData.Response {
		// Extract device name
		name, ok := item["name"].(string)
		if !ok {
			continue
		}

		// Extract speed
		var speed uint
		speedVal, ok := item["speed"]
		if ok {
			// Convert to float64 first, as JSON numbers are decoded as float64
			if speedFloat, ok := speedVal.(float64); ok {
				speed = uint(speedFloat)
			}
		}

		// Extract max speed
		var maxSpeed uint
		maxSpeedVal, ok := item["max_speed"]
		if ok {
			// Convert to float64 first, as JSON numbers are decoded as float64
			if maxSpeedFloat, ok := maxSpeedVal.(float64); ok {
				maxSpeed = uint(maxSpeedFloat)
			}
		}

		devices = append(devices, CoolingDevice{
			Name:     name,
			Speed:    speed,
			MaxSpeed: maxSpeed,
		})
	}

	return devices, nil
}

// SetCoolingSpeed sets the speed of a cooling device
func (c *Client) SetCoolingSpeed(device string, speed uint) error {
	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "cooling")
	req.AddQueryParam("name", device)
	req.AddQueryParam("speed", strconv.FormatUint(uint64(speed), 10))

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors in the response
	if err := checkResponseError(resp); err != nil {
		return fmt.Errorf("setting cooling speed failed: %w", err)
	}

	return nil
}

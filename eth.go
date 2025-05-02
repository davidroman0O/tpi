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
)

// EthReset resets the on-board Ethernet switch
func (c *Client) EthReset() error {
	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "eth")
	req.AddQueryParam("cmd", "reset")

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors in the response
	if err := checkResponseError(resp); err != nil {
		return fmt.Errorf("Ethernet switch reset failed: %w", err)
	}

	return nil
}

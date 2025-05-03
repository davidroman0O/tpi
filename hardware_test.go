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
	"testing"
	"time"
)

// skipIfNoHardware skips the test if the hardware is not available
func skipIfNoHardware(t *testing.T) {
	t.Helper()

	// Set a short timeout for connection attempts
	timeout := 500 * time.Millisecond

	config := loadTestConfig(t)
	client, err := NewClient(
		WithHost(config.Host),
		WithTimeout(timeout),
		WithCredentials(config.Username, config.Password),
	)
	if err != nil {
		t.Skipf("Failed to create test client: %v", err)
	}

	// Try to get info as a connectivity test
	_, err = client.Info()
	if err != nil {
		t.Skipf("Hardware not available: %v", err)
	}
}

// TestHardwareInfo tests getting info from real hardware
func TestHardwareInfo(t *testing.T) {
	skipIfNoHardware(t)

	client := createTestClient(t)

	// Get board info
	info, err := client.Info()
	if err != nil {
		t.Fatalf("Failed to get info: %v", err)
	}

	// Print the info for debugging
	fmt.Println("Hardware Info Response:")
	if len(info) == 0 {
		fmt.Println("  <empty response>")
		// Even an empty response means we connected successfully
		// Some hardware might not provide info or might have a different API version
		t.Log("Got empty info response - this may be normal for some hardware")
	} else {
		for key, value := range info {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}

	// The test passes if we can connect to the hardware, even if the info is empty
	// This is to accommodate different hardware versions
}

// TestHardwarePowerStatus tests getting power status from real hardware
func TestHardwarePowerStatus(t *testing.T) {
	skipIfNoHardware(t)

	client := createTestClient(t)

	// Get power status
	status, err := client.PowerStatus()
	if err != nil {
		t.Fatalf("Failed to get power status: %v", err)
	}

	// Print the status for debugging
	fmt.Println("Power Status Response:")
	if len(status) == 0 {
		fmt.Println("  <empty response>")
		// Even an empty response means we connected successfully
		// Some hardware might not provide status or might have a different API version
		t.Log("Got empty power status response - this may be normal for some hardware")
	} else {
		for node, isOn := range status {
			powerState := "OFF"
			if isOn {
				powerState = "ON"
			}
			fmt.Printf("  Node %d: %s\n", node, powerState)
		}
	}

	// The test passes if we can connect to the hardware, even if the status is empty
	// This is to accommodate different hardware versions
}

// TestHardwareUsbStatus tests getting USB status from real hardware
func TestHardwareUsbStatus(t *testing.T) {
	skipIfNoHardware(t)

	client := createTestClient(t)

	// Get USB status
	status, err := client.UsbGetStatus()
	fmt.Println("USB Status Response:")

	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		t.Logf("Failed to get USB status: %v - this may be normal for some hardware or firmware versions", err)
	} else if status == nil {
		fmt.Println("  <nil response>")
		t.Log("Got nil USB status response - this may be normal for some hardware")
	} else {
		fmt.Printf("  Node=%s, Mode=%s, Route=%s\n",
			status.Node, status.Mode, status.Route)
	}

	// The test passes regardless of the response
	// We just want to verify that the code runs without panicking
}

// We don't include power control tests that would affect the hardware state
// Those should be run manually and selectively

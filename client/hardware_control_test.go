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
	"os"
	"testing"
	"time"
)

// TestHardwarePowerControl tests power control operations on real hardware
// This test is disabled by default since it affects the hardware state
// To run this test, set the environment variable TPI_TEST_POWER=1
func TestHardwarePowerControl(t *testing.T) {
	// Skip if not explicitly enabled
	if os.Getenv("TPI_TEST_POWER") != "1" {
		t.Skip("Skipping power control test (enable with TPI_TEST_POWER=1)")
	}

	skipIfNoHardware(t)
	client := createTestClient(t)

	// For safety, we'll only operate on Node 1
	// You can change this if needed, but be careful
	const testNode = 1

	// Check initial power state
	initialStatus, err := client.PowerStatus()
	if err != nil {
		t.Fatalf("Failed to get initial power status: %v", err)
	}

	fmt.Println("Initial Power Status:")
	initialPowerOn := false
	if status, exists := initialStatus[testNode]; exists {
		initialPowerOn = status
		fmt.Printf("  Node %d: %s\n", testNode, getPowerStateString(status))
	} else {
		fmt.Printf("  Node %d: unknown\n", testNode)
	}

	// Try to toggle the power state
	fmt.Printf("Toggling power state for Node %d...\n", testNode)
	if initialPowerOn {
		// If on, turn it off
		if err := client.PowerOff(testNode); err != nil {
			t.Fatalf("Failed to power off Node %d: %v", testNode, err)
		}
		fmt.Printf("  Powered OFF Node %d\n", testNode)
	} else {
		// If off, turn it on
		if err := client.PowerOn(testNode); err != nil {
			t.Fatalf("Failed to power on Node %d: %v", testNode, err)
		}
		fmt.Printf("  Powered ON Node %d\n", testNode)
	}

	// Wait a moment for the state change to take effect
	time.Sleep(2 * time.Second)

	// Check the new power state
	newStatus, err := client.PowerStatus()
	if err != nil {
		t.Fatalf("Failed to get updated power status: %v", err)
	}

	fmt.Println("Updated Power Status:")
	newPowerOn := false
	if status, exists := newStatus[testNode]; exists {
		newPowerOn = status
		fmt.Printf("  Node %d: %s\n", testNode, getPowerStateString(status))
	} else {
		fmt.Printf("  Node %d: unknown\n", testNode)
	}

	// Restore original state
	fmt.Printf("Restoring original power state for Node %d...\n", testNode)
	if initialPowerOn && !newPowerOn {
		if err := client.PowerOn(testNode); err != nil {
			t.Fatalf("Failed to restore power ON for Node %d: %v", testNode, err)
		}
		fmt.Printf("  Restored power ON for Node %d\n", testNode)
	} else if !initialPowerOn && newPowerOn {
		if err := client.PowerOff(testNode); err != nil {
			t.Fatalf("Failed to restore power OFF for Node %d: %v", testNode, err)
		}
		fmt.Printf("  Restored power OFF for Node %d\n", testNode)
	} else {
		fmt.Printf("  No restoration needed for Node %d\n", testNode)
	}
}

// TestHardwareUsbControl tests USB configuration on real hardware
// This test is disabled by default since it affects the hardware state
// To run this test, set the environment variable TPI_TEST_USB=1
func TestHardwareUsbControl(t *testing.T) {
	// Skip if not explicitly enabled
	if os.Getenv("TPI_TEST_USB") != "1" {
		t.Skip("Skipping USB control test (enable with TPI_TEST_USB=1)")
	}

	skipIfNoHardware(t)
	client := createTestClient(t)

	// For testing, we'll use Node 2
	// Adjust as needed for your hardware setup
	const testNode = 2

	// Get current USB status
	initialStatus, err := client.UsbGetStatus()
	if err != nil {
		t.Fatalf("Failed to get initial USB status: %v", err)
	}

	fmt.Println("Initial USB Status:")
	fmt.Printf("  Node=%s, Mode=%s, Route=%s\n",
		initialStatus.Node, initialStatus.Mode, initialStatus.Route)

	// Determine what change to make
	var originallyInDeviceMode bool
	if initialStatus.Node == fmt.Sprintf("Node %d", testNode) && initialStatus.Mode == "Device" {
		originallyInDeviceMode = true
		fmt.Printf("Node %d is currently in Device mode, switching to Host mode...\n", testNode)
	} else {
		originallyInDeviceMode = false
		fmt.Printf("Setting Node %d to Device mode...\n", testNode)
	}

	// Make the change
	if originallyInDeviceMode {
		// Switch to Host mode
		if err := client.UsbSetHost(testNode, false); err != nil {
			t.Fatalf("Failed to set Node %d to Host mode: %v", testNode, err)
		}
	} else {
		// Switch to Device mode
		if err := client.UsbSetDevice(testNode, false); err != nil {
			t.Fatalf("Failed to set Node %d to Device mode: %v", testNode, err)
		}
	}

	// Wait a moment for the change to take effect
	time.Sleep(2 * time.Second)

	// Get the new USB status
	newStatus, err := client.UsbGetStatus()
	if err != nil {
		t.Fatalf("Failed to get updated USB status: %v", err)
	}

	fmt.Println("Updated USB Status:")
	fmt.Printf("  Node=%s, Mode=%s, Route=%s\n",
		newStatus.Node, newStatus.Mode, newStatus.Route)

	// Verify the change
	if originallyInDeviceMode {
		if newStatus.Mode != "Host" {
			t.Errorf("Failed to change USB mode to Host, still in %s mode", newStatus.Mode)
		}
	} else {
		if newStatus.Mode != "Device" {
			t.Errorf("Failed to change USB mode to Device, in %s mode", newStatus.Mode)
		}
	}

	// Restore original state
	fmt.Println("Restoring original USB configuration...")
	if originallyInDeviceMode {
		if err := client.UsbSetDevice(testNode, false); err != nil {
			t.Fatalf("Failed to restore Device mode for Node %d: %v", testNode, err)
		}
		fmt.Printf("  Restored Device mode for Node %d\n", testNode)
	} else {
		// We were not originally in device mode, so if we're in device mode now, change back
		if newStatus.Mode == "Device" && newStatus.Node == fmt.Sprintf("Node %d", testNode) {
			if err := client.UsbSetHost(testNode, false); err != nil {
				t.Fatalf("Failed to restore Host mode for Node %d: %v", testNode, err)
			}
			fmt.Printf("  Restored Host mode for Node %d\n", testNode)
		} else {
			fmt.Println("  No restoration needed")
		}
	}
}

// Helper function to convert power state to string
func getPowerStateString(isOn bool) string {
	if isOn {
		return "ON"
	}
	return "OFF"
}

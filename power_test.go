package tpi

import "testing"

func TestPowerCycleAll(t *testing.T) {
	client := createTestClient(t)

	// Get the current power state
	err := client.PowerOffAll()
	if err != nil {
		t.Fatalf("Failed to power off all nodes: %v", err)
	}

	// Power on all nodes
	powerState, err := client.PowerStatus()
	if err != nil {
		t.Fatalf("Failed to get power state: %v", err)
	}

	// Check that all nodes are powered off
	for node, powered := range powerState {
		if powered {
			t.Fatalf("Node %d is powered on", node)
		}
	}

	// Check that all nodes are powered off
	err = client.PowerOnAll()
	if err != nil {
		t.Fatalf("Failed to power on all nodes: %v", err)
	}

	// Check that all nodes are powered on
	powerState, err = client.PowerStatus()
	if err != nil {
		t.Fatalf("Failed to get power state: %v", err)
	}

	// Check that all nodes are powered on
	for node, powered := range powerState {
		if !powered {
			t.Fatalf("Node %d is powered off", node)
		}
	}
}

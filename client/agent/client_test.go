package agent

import (
	"testing"
)

// TestAgentClient tests connecting to an agent server
// This assumes the agent server is already running from TestAgentServer
// Run with: go test -v ./agent -run TestAgentClient
func TestAgentClient(t *testing.T) {
	// Skip in automated testing
	if testing.Short() {
		t.Skip("Skipping agent client test in short mode")
	}

	// Create the agent client using options
	client, err := NewAgentClientFromOptions(
		WithAgentHost("localhost"),
		WithAgentPort(9977),
		WithAgentSecret("test-secret"),
	)
	if err != nil {
		t.Fatalf("Failed to create agent client: %v", err)
	}

	// Test Info command
	t.Log("Testing Info command...")
	info, err := client.Info()
	if err != nil {
		t.Fatalf("Failed to get info: %v", err)
	}
	t.Logf("Info result: %+v", info)

	// Test About command
	t.Log("Testing About command...")
	about, err := client.About()
	if err != nil {
		t.Fatalf("Failed to get about: %v", err)
	}
	t.Logf("About result: %+v", about)

	// Test PowerStatus command
	t.Log("Testing PowerStatus command...")
	status, err := client.PowerStatus()
	if err != nil {
		t.Fatalf("Failed to get power status: %v", err)
	}
	t.Logf("Power status: %+v", status)

	// Test UsbGetStatus command
	t.Log("Testing UsbGetStatus command...")
	usbStatus, err := client.UsbGetStatus()
	if err != nil {
		t.Fatalf("Failed to get USB status: %v", err)
	}
	t.Logf("USB status: Node=%s, Mode=%s, Route=%s",
		usbStatus.Node, usbStatus.Mode, usbStatus.Route)

	// Test GetUartOutput command
	t.Log("Testing GetUartOutput command...")
	uart, err := client.GetUartOutput(1)
	if err != nil {
		t.Fatalf("Failed to get UART output: %v", err)
	}
	t.Logf("UART output for node 1: %s", uart)

	t.Log("Agent client test completed successfully!")
}

package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	tpi "github.com/davidroman0O/tpi/client"
)

// TestAgentServer tests running an agent server
// This is more of an example than an automated test
// Run with: go test -v ./agent -run TestAgentServer
func TestAgentServer(t *testing.T) {
	// Skip in automated testing
	if testing.Short() {
		t.Skip("Skipping agent server test in short mode")
	}

	// Create a mock HTTP server that will respond to BMC requests
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate BMC responses
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("type") {
		case "other":
			w.Write([]byte(`{"result": {"version": "mock-version", "build": "mock-build", "status": "OK"}}`))
		case "node":
			w.Write([]byte(`{"result": {"1": true, "2": false, "3": true, "4": false}}`))
		default:
			w.Write([]byte(`{"result": "success"}`))
		}
	}))
	defer mockServer.Close()

	// Create a real client pointing to our mock server
	client, err := tpi.NewClient(
		tpi.WithHost(mockServer.URL),
		tpi.WithCredentials("test", "test"),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create agent config
	agentConfig := AgentConfig{
		Port: 9977, // Default port
		Auth: AgentAuthConfig{
			Secret: "test-secret", // Use a simple test secret
		},
	}

	// Create the agent
	agentServer, err := NewAgent(agentConfig, client)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Create a context with cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the agent in a goroutine
	go func() {
		if err := agentServer.Start(ctx); err != nil {
			t.Logf("Agent server stopped: %v", err)
		}
	}()

	// Log server information
	t.Logf("Agent server started on port %d", agentConfig.Port)
	t.Logf("Use auth secret: %s", agentConfig.Auth.Secret)

	// Let the server run for a while (adjust as needed for manual testing)
	time.Sleep(5 * time.Second)

	// Cancel the context to stop the server
	cancel()
	t.Log("Test complete, agent server stopped")
}

// MockTransport is a mock implementation of the transport interface for testing
type MockTransport struct{}

func (m *MockTransport) SendRequest(endpoint string, method string, params map[string]interface{}) (map[string]interface{}, error) {
	// Simulate responses based on the endpoint
	switch endpoint {
	case "info":
		return map[string]interface{}{
			"version": "mock-version",
			"build":   "mock-build",
			"status":  "OK",
		}, nil
	case "about":
		return map[string]interface{}{
			"name":    "mock-bmc",
			"version": "mock-version",
		}, nil
	case "power/status":
		return map[string]interface{}{
			"1": true,
			"2": false,
			"3": true,
			"4": false,
		}, nil
	case "usb/status":
		return map[string]interface{}{
			"node":  "1",
			"mode":  "host",
			"route": "bmc",
		}, nil
	case "uart/output":
		return map[string]interface{}{
			"output": "Mock UART output",
		}, nil
	default:
		// For any other endpoint, just return success
		return map[string]interface{}{"success": true}, nil
	}
}

// MockTpiClient is a mock implementation of the TPI client for testing
type MockTpiClient struct{}

// Info mocks the Info method
func (m *MockTpiClient) Info() (map[string]string, error) {
	return map[string]string{
		"version": "mock-version",
		"build":   "mock-build",
		"status":  "OK",
	}, nil
}

// About mocks the About method
func (m *MockTpiClient) About() (map[string]string, error) {
	return map[string]string{
		"name":    "mock-bmc",
		"version": "mock-version",
	}, nil
}

// Reboot mocks the Reboot method
func (m *MockTpiClient) Reboot() error {
	return nil
}

// RebootAndWait mocks the RebootAndWait method
func (m *MockTpiClient) RebootAndWait(timeout int) error {
	time.Sleep(1 * time.Second) // Simulate a short wait
	return nil
}

// PowerStatus mocks the PowerStatus method
func (m *MockTpiClient) PowerStatus() (map[int]bool, error) {
	return map[int]bool{
		1: true,
		2: false,
		3: true,
		4: false,
	}, nil
}

// PowerOn mocks the PowerOn method
func (m *MockTpiClient) PowerOn(node int) error {
	return nil
}

// PowerOff mocks the PowerOff method
func (m *MockTpiClient) PowerOff(node int) error {
	return nil
}

// PowerReset mocks the PowerReset method
func (m *MockTpiClient) PowerReset(node int) error {
	return nil
}

// PowerOnAll mocks the PowerOnAll method
func (m *MockTpiClient) PowerOnAll() error {
	return nil
}

// PowerOffAll mocks the PowerOffAll method
func (m *MockTpiClient) PowerOffAll() error {
	return nil
}

// SetNodeNormalMode mocks the SetNodeNormalMode method
func (m *MockTpiClient) SetNodeNormalMode(node int) error {
	return nil
}

// SetNodeMsdMode mocks the SetNodeMsdMode method
func (m *MockTpiClient) SetNodeMsdMode(node int) error {
	return nil
}

// UsbGetStatus mocks the UsbGetStatus method
func (m *MockTpiClient) UsbGetStatus() (*tpi.UsbStatusInfo, error) {
	return &tpi.UsbStatusInfo{
		Node:  "1",
		Mode:  "host",
		Route: "bmc",
	}, nil
}

// UsbSetHost mocks the UsbSetHost method
func (m *MockTpiClient) UsbSetHost(node int, bmc bool) error {
	return nil
}

// UsbSetDevice mocks the UsbSetDevice method
func (m *MockTpiClient) UsbSetDevice(node int, bmc bool) error {
	return nil
}

// UsbSetFlash mocks the UsbSetFlash method
func (m *MockTpiClient) UsbSetFlash(node int, bmc bool) error {
	return nil
}

// GetUartOutput mocks the GetUartOutput method
func (m *MockTpiClient) GetUartOutput(node int) (string, error) {
	return "Mock UART output", nil
}

// SendUartCommand mocks the SendUartCommand method
func (m *MockTpiClient) SendUartCommand(node int, command string) error {
	return nil
}

// EthReset mocks the EthReset method
func (m *MockTpiClient) EthReset() error {
	return nil
}

// FlashNode mocks the FlashNode method
func (m *MockTpiClient) FlashNode(node int, options *tpi.FlashOptions) error {
	return nil
}

// FlashNodeLocal mocks the FlashNodeLocal method
func (m *MockTpiClient) FlashNodeLocal(node int, imagePath string) error {
	return nil
}

// UpgradeFirmware mocks the UpgradeFirmware method
func (m *MockTpiClient) UpgradeFirmware(filePath string, providedSha256 string) error {
	return nil
}

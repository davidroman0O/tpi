package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	tpi "github.com/davidroman0O/tpi/client"
	"github.com/davidroman0O/tpi/client/agent"
)

// This example shows how to use the agent mode
// There are two parts:
// 1. Running an agent server on the machine with direct BMC access
// 2. Connecting to that server from a remote machine

func main() {
	// Check if we should run as agent server or client
	if len(os.Args) > 1 && os.Args[1] == "server" {
		runAgentServer()
	} else {
		runAgentClient()
	}
}

// runAgentServer demonstrates how to run an agent server
func runAgentServer() {
	fmt.Println("Starting TPI Agent Server...")

	// Create a client to talk to the local BMC
	// Replace with your actual BMC IP and credentials
	client, err := tpi.NewClient(
		tpi.WithHost("192.168.1.1"),           // Replace with your BMC IP
		tpi.WithCredentials("root", "turing"), // Replace with your credentials
	)
	if err != nil {
		log.Fatalf("Failed to create TPI client: %v", err)
	}

	// Create agent config
	agentConfig := agent.AgentConfig{
		Port: 9977, // Default port
		// Optional IP whitelist
		AllowedClients: []string{}, // Empty means allow all
		Auth: agent.AgentAuthConfig{
			Secret: "your-shared-secret", // Replace with secure secret
		},
	}

	// Set up context with signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived shutdown signal, stopping agent server...")
		cancel()
	}()

	// Create the agent
	agentServer, err := agent.NewAgent(agentConfig, client)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Printf("Agent server started on port %d\n", agentConfig.Port)
	fmt.Println("Press Ctrl+C to stop the server")

	// Start the agent server (this will block until the context is canceled)
	if err := agentServer.Start(ctx); err != nil {
		log.Fatalf("Agent server error: %v", err)
	}

	fmt.Println("Agent server stopped")
}

// runAgentClient demonstrates how to connect to an agent server
func runAgentClient() {
	fmt.Println("Starting TPI Agent Client...")

	// Create the agent client
	// Replace with your actual agent server IP and port
	client, err := agent.NewAgentClientFromOptions(
		agent.WithAgentHost("your-server-ip"), // Replace with your server IP
		agent.WithAgentPort(9977),
		agent.WithAgentSecret("your-shared-secret"), // Must match server
	)
	if err != nil {
		log.Fatalf("Failed to create agent client: %v", err)
	}

	// Get power status
	fmt.Println("Getting power status...")
	status, err := client.PowerStatus()
	if err != nil {
		log.Fatalf("Failed to get power status: %v", err)
	}

	// Display power status
	fmt.Println("Power status:")
	for node, isOn := range status {
		powerState := "OFF"
		if isOn {
			powerState = "ON"
		}
		fmt.Printf("  Node %d: %s\n", node, powerState)
	}

	// Get system info
	fmt.Println("\nGetting system info...")
	info, err := client.Info()
	if err != nil {
		log.Fatalf("Failed to get system info: %v", err)
	}
	fmt.Println("System info:")
	for key, value := range info {
		fmt.Printf("  %s: %s\n", key, value)
	}

	// Turn on a node
	nodeToControl := 1 // Replace with the node you want to control
	fmt.Printf("\nTurning on node %d...\n", nodeToControl)
	err = client.PowerOn(nodeToControl)
	if err != nil {
		log.Fatalf("Failed to turn on node %d: %v", nodeToControl, err)
	}
	fmt.Printf("Node %d powered on successfully\n", nodeToControl)

	// Wait a bit before checking status again
	time.Sleep(2 * time.Second)

	// Check power status again
	fmt.Println("\nChecking power status again...")
	status, err = client.PowerStatus()
	if err != nil {
		log.Fatalf("Failed to get power status: %v", err)
	}

	// Display updated power status
	fmt.Println("Updated power status:")
	for node, isOn := range status {
		powerState := "OFF"
		if isOn {
			powerState = "ON"
		}
		fmt.Printf("  Node %d: %s\n", node, powerState)
	}

	fmt.Println("\nAgent client demo completed successfully!")
}

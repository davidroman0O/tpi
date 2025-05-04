# Turing Pi Agent Mode

This package implements an agent mode for the Turing Pi client, enabling remote control of a Turing Pi BMC without SSH tunnels or VPN connections.

## Architecture

The agent mode consists of two main components:

1. **Agent Server**: Runs on a machine with direct access to the BMC
2. **Agent Client**: Connects to the agent server from a remote machine

## Usage

### Setting up the Agent Server

To run an agent server on a machine with direct BMC access:

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	tpi "github.com/davidroman0O/tpi/client"
	"github.com/davidroman0O/tpi/client/agent"
)

func main() {
	// Create a client to talk to the local BMC
	client, err := tpi.NewClient(
		tpi.WithHost("192.168.1.1"),           // Your BMC IP
		tpi.WithCredentials("root", "turing"), // Your credentials
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Configure the agent
	agentConfig := agent.AgentConfig{
		Port: 9977, // Default port
		Auth: agent.AgentAuthConfig{
			Secret: "your-shared-secret", // Use a strong, unique secret
		},
	}

	// Create a context with cancel for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Create and start the agent
	agentServer, err := agent.NewAgent(agentConfig, client)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	log.Printf("Agent server started on port %d\n", agentConfig.Port)
	if err := agentServer.Start(ctx); err != nil {
		log.Fatalf("Agent server error: %v", err)
	}
}
```

### Connecting with the Agent Client

To connect to an agent server from a remote machine:

```go
package main

import (
	"fmt"
	"log"

	"github.com/davidroman0O/tpi/client/agent"
)

func main() {
	// Create the agent client
	client, err := agent.NewAgentClientFromOptions(
		agent.WithAgentHost("agent-server-ip"), // IP of the server
		agent.WithAgentPort(9977),              // Port (default: 9977)
		agent.WithAgentSecret("your-shared-secret"), // Same secret as server
	)
	if err != nil {
		log.Fatalf("Failed to create agent client: %v", err)
	}

	// Use the client to control the Turing Pi
	// Same API as the regular client
	status, err := client.PowerStatus()
	if err != nil {
		log.Fatalf("Failed to get power status: %v", err)
	}
	
	fmt.Println("Power status:")
	for node, isOn := range status {
		fmt.Printf("Node %d: %v\n", node, isOn)
	}
}
```

## Security Considerations

1. **Authentication**: Always use a strong, unique secret for agent authentication.
2. **Network Security**: Consider restricting access to the agent port using a firewall.
3. **TLS**: For production use, enable TLS by configuring certificates.
4. **IP Allowlist**: Restrict which IPs can connect using the `AllowedClients` config option.

## Testing

To test the agent functionality:

1. Run the agent server test:
   ```bash
   cd client
   go test -v ./agent -run TestAgentServer
   ```

2. In another terminal, run the agent client test:
   ```bash
   cd client
   go test -v ./agent -run TestAgentClient
   ```

The tests use mock implementations so no actual BMC is required.

## Complete Example

See `client/examples/agent_example.go` for a complete example with both server and client implementations. 
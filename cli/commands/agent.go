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

package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/davidroman0O/tpi/client/agent"
	"github.com/spf13/cobra"
)

// newAgentCommand creates the agent command
func newAgentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Control Turing Pi remotely via agent mode",
		Long:  "Run an agent server or connect to a remote agent to control Turing Pi.",
	}

	// Add subcommands
	cmd.AddCommand(newAgentServerCommand())
	cmd.AddCommand(newAgentClientCommand())

	return cmd
}

// newAgentServerCommand creates the agent server subcommand
func newAgentServerCommand() *cobra.Command {
	var port int
	var secret string
	var allowedIPs []string
	var tlsEnabled bool
	var tlsCertFile string
	var tlsKeyFile string

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run an agent server",
		Long:  "Run an agent server to allow remote control of a Turing Pi.",
		Example: `  # Run an agent server on the default port
  tpi agent server --host=192.168.1.91 --user=root --password=turing

  # Run with a custom port and authentication
  tpi agent server --host=192.168.1.91 --port=9977 --secret=mysecret`,
		Run: func(cmd *cobra.Command, args []string) {
			// Create a client
			client, err := getClient(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Create agent config
			agentConfig := agent.AgentConfig{
				Port:           port,
				AllowedClients: allowedIPs,
				Auth: agent.AgentAuthConfig{
					Secret: secret,
				},
				TLSEnabled:  tlsEnabled,
				TLSCertFile: tlsCertFile,
				TLSKeyFile:  tlsKeyFile,
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
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Print server info
			host, _ := cmd.Flags().GetString("host")
			fmt.Printf("Agent server started for Turing Pi at %s\n", host)
			fmt.Printf("Listening on port: %d\n", port)
			if secret != "" {
				fmt.Println("Authentication enabled")
			}
			fmt.Println("Press Ctrl+C to stop the server")

			// Start the agent server (this will block until the context is canceled)
			if err := agentServer.Start(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Agent server stopped")
		},
	}

	// Add flags - without shorthand flags to avoid conflicts with global flags
	cmd.Flags().IntVar(&port, "port", 9977, "Port to listen on")
	cmd.Flags().StringVar(&secret, "secret", "", "Secret for authentication")
	cmd.Flags().StringSliceVar(&allowedIPs, "allowed-ips", nil, "List of allowed client IPs (empty for all)")
	cmd.Flags().BoolVar(&tlsEnabled, "tls", false, "Enable TLS")
	cmd.Flags().StringVar(&tlsCertFile, "cert", "", "TLS certificate file")
	cmd.Flags().StringVar(&tlsKeyFile, "key", "", "TLS key file")

	return cmd
}

// newAgentClientCommand creates the agent client subcommand
func newAgentClientCommand() *cobra.Command {
	var agentHost string
	var agentPort int
	var secret string
	var command string
	var node int
	var tlsEnabled bool
	var skipVerify bool
	var localPath string
	var remotePath string
	var execCommand string

	cmd := &cobra.Command{
		Use:   "client",
		Short: "Connect to an agent server",
		Long:  "Connect to a remote agent server to control a Turing Pi.",
		Example: `  # Connect to an agent server
  tpi agent client --agent-host=192.168.1.100 --secret=mysecret
  
  # Run a specific command through the agent
  tpi agent client --agent-host=192.168.1.100 --secret=mysecret --command=power-status
  
  # Upload a file to the remote system
  tpi agent client --agent-host=192.168.1.100 --secret=mysecret --command=upload --local-path=./local-file.txt --remote-path=/tmp/remote-file.txt
  
  # Download a file from the remote system
  tpi agent client --agent-host=192.168.1.100 --secret=mysecret --command=download --remote-path=/tmp/remote-file.txt --local-path=./local-file.txt
  
  # List files in a remote directory
  tpi agent client --agent-host=192.168.1.100 --secret=mysecret --command=list --remote-path=/tmp
  
  # Execute a command on the remote system
  tpi agent client --agent-host=192.168.1.100 --secret=mysecret --command=execute --exec="ls -la /tmp"`,
		Run: func(cmd *cobra.Command, args []string) {
			// Check required flags
			if agentHost == "" {
				fmt.Fprintln(os.Stderr, "Error: agent host is required")
				os.Exit(1)
			}

			// Create agent client options
			clientOptions := []agent.AgentOption{
				agent.WithAgentHost(agentHost),
				agent.WithAgentPort(agentPort),
			}

			// Add authentication if provided
			if secret != "" {
				clientOptions = append(clientOptions, agent.WithAgentSecret(secret))
			}

			// Add TLS if enabled
			if tlsEnabled {
				clientOptions = append(clientOptions, agent.WithAgentTLS(true, skipVerify))
			}

			// Create the agent client
			client, err := agent.NewAgentClientFromOptions(clientOptions...)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Handle commands
			if command == "" || command == "info" {
				// Display system info
				info, err := client.Info()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				fmt.Println("System Information:")
				fmt.Println("|---------------|----------------------------|")
				fmt.Println("|      key      |           value            |")
				fmt.Println("|---------------|----------------------------|")
				for key, val := range info {
					fmt.Printf("| %-14s | %-28s |\n", key, val)
				}
				fmt.Println("|---------------|----------------------------|")
			} else if command == "power-status" {
				// Get power status
				status, err := client.PowerStatus()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				fmt.Println("Power Status:")
				for node, isOn := range status {
					state := "OFF"
					if isOn {
						state = "ON"
					}
					fmt.Printf("Node %d: %s\n", node, state)
				}
			} else if command == "power-on" {
				// Power on node
				if node < 1 || node > 4 {
					fmt.Fprintf(os.Stderr, "Error: node must be between 1 and 4\n")
					os.Exit(1)
				}

				if err := client.PowerOn(node); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Node %d powered on\n", node)
			} else if command == "power-off" {
				// Power off node
				if node < 1 || node > 4 {
					fmt.Fprintf(os.Stderr, "Error: node must be between 1 and 4\n")
					os.Exit(1)
				}

				if err := client.PowerOff(node); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Node %d powered off\n", node)
			} else if command == "reboot" {
				// Reboot BMC
				if err := client.Reboot(); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("BMC is rebooting...")
			} else if command == "upload" {
				// Upload file to remote system
				if localPath == "" || remotePath == "" {
					fmt.Fprintf(os.Stderr, "Error: local-path and remote-path are required for upload\n")
					os.Exit(1)
				}

				// Check if local file exists
				if _, err := os.Stat(localPath); os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "Error: local file %s does not exist\n", localPath)
					os.Exit(1)
				}

				fmt.Printf("Uploading %s to %s...\n", localPath, remotePath)
				if err := client.UploadFile(localPath, remotePath); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("Upload complete")
			} else if command == "download" {
				// Download file from remote system
				if localPath == "" || remotePath == "" {
					fmt.Fprintf(os.Stderr, "Error: local-path and remote-path are required for download\n")
					os.Exit(1)
				}

				// Create local directory if it doesn't exist
				localDir := filepath.Dir(localPath)
				if localDir != "." {
					if err := os.MkdirAll(localDir, 0755); err != nil {
						fmt.Fprintf(os.Stderr, "Error creating local directory: %v\n", err)
						os.Exit(1)
					}
				}

				fmt.Printf("Downloading %s to %s...\n", remotePath, localPath)
				if err := client.DownloadFile(remotePath, localPath); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("Download complete")
			} else if command == "list" {
				// List directory contents
				if remotePath == "" {
					fmt.Fprintf(os.Stderr, "Error: remote-path is required for listing\n")
					os.Exit(1)
				}

				fmt.Printf("Listing contents of %s:\n", remotePath)
				files, err := client.ListDirectory(remotePath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				// Print directory contents in a table format
				fmt.Println()
				fmt.Printf("%-10s %-12s %-20s %s\n", "Type", "Size", "Modified", "Name")
				fmt.Println(strings.Repeat("-", 80))

				for _, file := range files {
					fileType := "file"
					if file.IsDir {
						fileType = "directory"
					}

					// Format size
					sizeStr := fmt.Sprintf("%d", file.Size)
					if file.Size > 1024*1024*1024 {
						sizeStr = fmt.Sprintf("%.2f GB", float64(file.Size)/(1024*1024*1024))
					} else if file.Size > 1024*1024 {
						sizeStr = fmt.Sprintf("%.2f MB", float64(file.Size)/(1024*1024))
					} else if file.Size > 1024 {
						sizeStr = fmt.Sprintf("%.2f KB", float64(file.Size)/1024)
					}

					// Format time
					timeStr := file.ModTime.Format("2006-01-02 15:04:05")

					fmt.Printf("%-10s %-12s %-20s %s\n", fileType, sizeStr, timeStr, file.Name)
				}
				fmt.Println()
			} else if command == "execute" {
				// Execute command on remote system
				if execCommand == "" {
					fmt.Fprintf(os.Stderr, "Error: exec parameter is required for execute command\n")
					os.Exit(1)
				}

				fmt.Printf("Executing command: %s\n", execCommand)
				output, err := client.ExecuteCommand(execCommand)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				fmt.Println("Command output:")
				fmt.Println(strings.Repeat("-", 80))
				fmt.Println(output)
				fmt.Println(strings.Repeat("-", 80))
			} else if command == "interactive" {
				// Interactive mode with multiple commands
				fmt.Printf("Connected to agent at %s:%d\n", agentHost, agentPort)
				fmt.Println("Interactive mode - press Ctrl+C to exit")

				// First show the power status
				status, err := client.PowerStatus()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error getting power status: %v\n", err)
				} else {
					fmt.Println("\nCurrent Power Status:")
					for node, isOn := range status {
						state := "OFF"
						if isOn {
							state = "ON"
						}
						fmt.Printf("Node %d: %s\n", node, state)
					}
				}

				// Then show system info
				info, err := client.Info()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error getting system info: %v\n", err)
				} else {
					fmt.Println("\nSystem Information:")
					for key, val := range info {
						fmt.Printf("%s: %s\n", key, val)
					}
				}

				// Keep the connection alive
				sigCh := make(chan os.Signal, 1)
				signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
				<-sigCh
				fmt.Println("\nDisconnected from agent")
			} else {
				fmt.Fprintf(os.Stderr, "Error: unknown command: %s\n", command)
				os.Exit(1)
			}
		},
	}

	// Add flags - without shorthand flags to avoid conflicts with global flags
	cmd.Flags().StringVar(&agentHost, "agent-host", "", "Agent server hostname or IP")
	cmd.Flags().IntVar(&agentPort, "agent-port", 9977, "Agent server port")
	cmd.Flags().StringVar(&secret, "secret", "", "Authentication secret")
	cmd.Flags().StringVar(&command, "command", "", "Command to execute [info, power-status, power-on, power-off, reboot, upload, download, list, execute, interactive]")
	cmd.Flags().IntVar(&node, "node", 0, "Node number for node-specific commands [1-4]")
	cmd.Flags().BoolVar(&tlsEnabled, "tls", false, "Enable TLS")
	cmd.Flags().BoolVar(&skipVerify, "skip-verify", true, "Skip TLS certificate verification")
	cmd.Flags().StringVar(&localPath, "local-path", "", "Local file path for upload or download")
	cmd.Flags().StringVar(&remotePath, "remote-path", "", "Remote file path for upload, download or list")
	cmd.Flags().StringVar(&execCommand, "exec", "", "Command to execute on the remote system")

	// Mark required flags
	cmd.MarkFlagRequired("agent-host")

	return cmd
}

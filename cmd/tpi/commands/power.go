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
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

// newPowerCommand creates the power command
func newPowerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "power [command] [node]",
		Short: "Power on/off or reset specific nodes",
		Long:  "Power on/off or reset specific nodes.",
		Example: `  # Power on node 1
  tpi power on 1 --host=192.168.1.91
  
  # Power off all nodes
  tpi power off --host=192.168.1.91
  
  # Check power status of all nodes
  tpi power status --host=192.168.1.91`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("requires a command (on, off, reset, status)")
			}

			validCommands := map[string]bool{
				"on":     true,
				"off":    true,
				"reset":  true,
				"status": true,
			}

			if !validCommands[args[0]] {
				return fmt.Errorf("invalid command: %s (must be on, off, reset, or status)", args[0])
			}

			// If a node is specified, validate it
			if len(args) > 1 {
				nodeNum, err := strconv.Atoi(args[1])
				if err != nil {
					return fmt.Errorf("node must be a number: %v", err)
				}

				if nodeNum < 1 || nodeNum > 4 {
					return fmt.Errorf("node number must be between 1 and 4, got %d", nodeNum)
				}
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Create a client
			client, err := getClient(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Get the command (args[0]) and node number (args[1], if present)
			command := args[0]
			var nodeNum int

			if len(args) > 1 {
				nodeNum, _ = strconv.Atoi(args[1]) // Already validated in Args
			} else if command != "status" {
				// For on/off/reset without a specified node, use all nodes
				if command == "on" {
					err = client.PowerOnAll()
				} else if command == "off" {
					err = client.PowerOffAll()
				} else if command == "reset" {
					fmt.Fprintf(os.Stderr, "Error: reset command requires a node number\n")
					os.Exit(1)
				}

				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				return
			}

			// Execute the command
			switch command {
			case "status":
				// Get power status
				status, err := client.PowerStatus()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				// Print the status
				fmt.Println("Power Status:")
				fmt.Println("-----------------")

				// If a node was specified, only print that node
				if nodeNum > 0 {
					if powerOn, ok := status[nodeNum]; ok {
						printNodeStatus(nodeNum, powerOn)
					} else {
						fmt.Fprintf(os.Stderr, "Error: node %d not found\n", nodeNum)
						os.Exit(1)
					}
				} else {
					// Print all nodes
					for i := 1; i <= 4; i++ {
						if powerOn, ok := status[i]; ok {
							printNodeStatus(i, powerOn)
						}
					}
				}
			case "on":
				if err := client.PowerOn(nodeNum); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Node %d powered on\n", nodeNum)
			case "off":
				if err := client.PowerOff(nodeNum); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Node %d powered off\n", nodeNum)
			case "reset":
				if err := client.PowerReset(nodeNum); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Node %d reset\n", nodeNum)
			}
		},
	}

	// Add flags
	cmd.Flags().StringP("cmd", "c", "", "Specify command [on, off, reset, status]")
	cmd.Flags().IntP("node", "n", 0, "Node number [1-4]. Not specifying a node selects all nodes")

	return cmd
}

// printNodeStatus prints the status of a node
func printNodeStatus(node int, powerOn bool) {
	status := "OFF"
	if powerOn {
		status = "ON"
	}
	fmt.Printf("Node %d: %s\n", node, status)
}

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

	"github.com/spf13/cobra"
)

// newUartCommand creates the UART command
func newUartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uart [action] [node]",
		Short: "Read or write over UART",
		Long:  "Read or write over UART.",
		Example: `  # Get UART output from node 1
  tpi uart get 1 --host=192.168.1.91
  
  # Send a command to node 2 over UART
  tpi uart set 2 --cmd "ls -la" --host=192.168.1.91`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("requires an action (get, set)")
			}

			validActions := map[string]bool{
				"get": true,
				"set": true,
			}

			if !validActions[args[0]] {
				return fmt.Errorf("invalid action: %s (must be get or set)", args[0])
			}

			if len(args) < 2 {
				return fmt.Errorf("requires a node number")
			}

			// Node validation is done in the Run function

			// For set action, a command is required
			if args[0] == "set" {
				cmdFlag, err := cmd.Flags().GetString("cmd")
				if err != nil || cmdFlag == "" {
					return fmt.Errorf("set action requires a command (--cmd)")
				}
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Get node
			nodeNum, err := parseNodeArg(args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Create client
			client, err := getClient(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Handle action
			action := args[0]
			if action == "get" {
				// Get UART output
				output, err := client.GetUartOutput(nodeNum)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Print(output)
			} else if action == "set" {
				// Send UART command
				cmdStr, _ := cmd.Flags().GetString("cmd")
				if cmdStr == "" {
					fmt.Fprintln(os.Stderr, "Error: command is required for set action")
					os.Exit(1)
				}

				if err := client.SendUartCommand(nodeNum, cmdStr); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Command sent to node %d\n", nodeNum)
			}
		},
	}

	// Add flags
	cmd.Flags().StringP("cmd", "c", "", "Command to send over UART")

	return cmd
}

// parseNodeArg parses and validates the node argument
func parseNodeArg(arg string) (int, error) {
	var nodeNum int
	_, err := fmt.Sscanf(arg, "%d", &nodeNum)
	if err != nil {
		return 0, fmt.Errorf("node must be a number: %v", err)
	}

	if nodeNum < 1 || nodeNum > 4 {
		return 0, fmt.Errorf("node number must be between 1 and 4, got %d", nodeNum)
	}

	return nodeNum, nil
}

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

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	// Define styles for power status display
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	nodeStyle = lipgloss.NewStyle().
			Padding(0, 1)

	powerOnStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("10")). // Green
			Padding(0, 1)

	powerOffStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")). // Gray
			Padding(0, 1)

	tableStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)
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

			// Get command flags
			cmdFlag, _ := cmd.Flags().GetString("cmd")
			nodeFlag, _ := cmd.Flags().GetInt("node")

			// Get the command (args[0]) and node number (args[1], if present)
			command := args[0]
			var nodeNum int

			if len(args) > 1 {
				nodeNum, _ = strconv.Atoi(args[1]) // Already validated in Args
			} else if nodeFlag > 0 {
				// Use node from flag if provided
				nodeNum = nodeFlag
			} else if command != "status" {
				// For on/off/reset without a specified node, use all nodes
				if command == "on" {
					err = client.PowerOnAll()
					if err == nil {
						fmt.Println("✅ All nodes powered on\n")

						// Show current power status
						fmt.Println("Current power status:")
						status, _ := client.PowerStatus()
						printStyledPowerStatus(status, 0)
					}
				} else if command == "off" {
					err = client.PowerOffAll()
					if err == nil {
						fmt.Println("✅ All nodes powered off\n")

						// Show current power status
						fmt.Println("Current power status:")
						status, _ := client.PowerStatus()
						printStyledPowerStatus(status, 0)
					}
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
				// Check if the --cmd flag was also used
				if cmdFlag != "" && cmdFlag != "status" {
					fmt.Printf("⚠️  Warning: Ignoring --cmd=%s flag in favor of 'status' argument\n", cmdFlag)
				}

				// Get power status
				status, err := client.PowerStatus()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				// Print the status with nice styling
				printStyledPowerStatus(status, nodeNum)

			case "on":
				// Check if the --cmd flag was also used
				if cmdFlag != "" && cmdFlag != "on" {
					fmt.Printf("⚠️  Warning: Ignoring --cmd=%s flag in favor of 'on' argument\n", cmdFlag)
				}

				if err := client.PowerOn(nodeNum); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("✅ Node %d powered on\n", nodeNum)

				// Show the current power status
				fmt.Println("\nCurrent power status:")
				status, _ := client.PowerStatus()
				printStyledPowerStatus(status, 0)

			case "off":
				// Check if the --cmd flag was also used
				if cmdFlag != "" && cmdFlag != "off" {
					fmt.Printf("⚠️  Warning: Ignoring --cmd=%s flag in favor of 'off' argument\n", cmdFlag)
				}

				if err := client.PowerOff(nodeNum); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("✅ Node %d powered off\n", nodeNum)

				// Show the current power status
				fmt.Println("\nCurrent power status:")
				status, _ := client.PowerStatus()
				printStyledPowerStatus(status, 0)

			case "reset":
				// Check if the --cmd flag was also used
				if cmdFlag != "" && cmdFlag != "reset" {
					fmt.Printf("⚠️  Warning: Ignoring --cmd=%s flag in favor of 'reset' argument\n", cmdFlag)
				}

				if err := client.PowerReset(nodeNum); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("✅ Node %d reset\n", nodeNum)

				// Show the current power status
				fmt.Println("\nCurrent power status:")
				status, _ := client.PowerStatus()
				printStyledPowerStatus(status, 0)
			}
		},
	}

	// Add flags
	cmd.Flags().StringP("cmd", "c", "", "Specify command [on, off, reset, status]")
	cmd.Flags().IntP("node", "n", 0, "Node number [1-4]. Not specifying a node selects all nodes")

	return cmd
}

// printStyledPowerStatus prints the status with nice lipgloss styling
func printStyledPowerStatus(status map[int]bool, specificNode int) {
	// Header
	header := headerStyle.Render("NODE") + headerStyle.Render("STATUS")

	// Rows
	rows := []string{}

	// If a specific node is requested, only show that one
	if specificNode > 0 {
		if powerOn, ok := status[specificNode]; ok {
			rows = append(rows, renderNodeRow(specificNode, powerOn))
		} else {
			fmt.Fprintf(os.Stderr, "Error: node %d not found\n", specificNode)
			os.Exit(1)
		}
	} else {
		// Otherwise show all nodes in order
		for i := 1; i <= 4; i++ {
			if powerOn, ok := status[i]; ok {
				rows = append(rows, renderNodeRow(i, powerOn))
			}
		}
	}

	// Combine rows
	table := header
	for _, row := range rows {
		table += "\n" + row
	}

	// Print the table with border
	fmt.Println(tableStyle.Render(table))
}

// renderNodeRow renders a single row in the power status table
func renderNodeRow(node int, powerOn bool) string {
	nodeStr := nodeStyle.Render(fmt.Sprintf("Node %d", node))

	var statusStr string
	if powerOn {
		statusStr = powerOnStyle.Render("● ON")
	} else {
		statusStr = powerOffStyle.Render("○ OFF")
	}

	return nodeStr + statusStr
}

// printNodeStatus prints the status of a node (DEPRECATED - using the styled version now)
func printNodeStatus(node int, powerOn bool) {
	status := "OFF"
	if powerOn {
		status = "ON"
	}
	fmt.Printf("Node %d: %s\n", node, status)
}

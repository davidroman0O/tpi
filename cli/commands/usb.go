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

// newUsbCommand creates the USB command
func newUsbCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usb [mode] [node]",
		Short: "Change the USB device/host configuration",
		Long:  "Change the USB device/host configuration. The USB-bus can only be routed to one node simultaneously.",
		Example: `  # Configure node 2 as USB device
  tpi usb device 2 --host=192.168.1.91
  
  # Configure node 1 as USB host
  tpi usb host 1 --host=192.168.1.91
  
  # Check USB status
  tpi usb status --host=192.168.1.91`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("requires a mode (device, host, flash, status)")
			}

			validModes := map[string]bool{
				"device": true,
				"host":   true,
				"flash":  true,
				"status": true,
			}

			if !validModes[args[0]] {
				return fmt.Errorf("invalid mode: %s (must be device, host, flash, or status)", args[0])
			}

			// For status, no node is needed
			if args[0] == "status" && len(args) > 1 {
				return fmt.Errorf("status mode does not require a node")
			}

			// For other modes, a node is required
			if args[0] != "status" && len(args) < 2 {
				return fmt.Errorf("mode %s requires a node number", args[0])
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

			// Get the mode and node number
			mode := args[0]
			var nodeNum int
			if len(args) > 1 {
				nodeNum, _ = strconv.Atoi(args[1]) // Already validated in Args
			}

			// Get BMC flag
			bmcFlag, _ := cmd.Flags().GetBool("bmc")

			// Execute the command
			switch mode {
			case "status":
				// Get USB status
				status, err := client.UsbGetStatus()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				// Print status
				fmt.Println("    USB Host    -->    USB Device    ")
				fmt.Println("---------------    ---------------")

				var host, device string
				if status.Mode == "host" {
					host = status.Node
					device = status.Route
				} else {
					host = status.Route
					device = status.Node
				}

				fmt.Printf("    %-12s -->    %-12s\n", host, device)

			case "device":
				if err := client.UsbSetDevice(nodeNum, bmcFlag); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Node %d configured as USB device\n", nodeNum)

			case "host":
				if err := client.UsbSetHost(nodeNum, bmcFlag); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Node %d configured as USB host\n", nodeNum)

			case "flash":
				if err := client.UsbSetFlash(nodeNum, bmcFlag); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Node %d configured in USB flash mode\n", nodeNum)
			}
		},
	}

	// Add flags
	cmd.Flags().StringP("mode", "m", "", "Specify mode [device, host, flash, status]")
	cmd.Flags().IntP("node", "n", 0, "Node number [1-4]")
	cmd.Flags().BoolP("bmc", "b", false, "Instead of USB-A, route the USB-bus to the BMC chip")

	return cmd
}

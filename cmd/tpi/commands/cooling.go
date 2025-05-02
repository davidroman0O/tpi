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

// newCoolingCommand creates the cooling command
func newCoolingCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cooling",
		Short: "Configure the cooling devices",
		Long:  "Configure the cooling devices.",
		Run: func(cmd *cobra.Command, args []string) {
			// Get command
			cmdStr, _ := cmd.Flags().GetString("cmd")
			if cmdStr == "" {
				fmt.Fprintf(os.Stderr, "Error: command is required\n")
				os.Exit(1)
			}

			// Make sure the command is valid
			if cmdStr != "set" && cmdStr != "status" {
				fmt.Fprintf(os.Stderr, "Error: invalid command: %s (must be set or status)\n", cmdStr)
				os.Exit(1)
			}

			// Create a client
			client, err := getClient(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Handle command
			if cmdStr == "status" {
				// Get cooling status
				devices, err := client.GetCoolingStatus()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				// Print status
				if len(devices) == 0 {
					fmt.Println("No cooling devices found")
					return
				}

				// Print the header
				fmt.Println("|---------------|-------|-----------|")
				fmt.Println("|     Device    | Speed | Max Speed |")
				fmt.Println("|---------------|-------|-----------|")

				// Print each device
				for _, device := range devices {
					fmt.Printf("|%-15s|%7d|%11d|\n", device.Name, device.Speed, device.MaxSpeed)
				}

				fmt.Println("|---------------|-------|-----------|")
			} else if cmdStr == "set" {
				// Get device and speed
				device, _ := cmd.Flags().GetString("device")
				if device == "" {
					fmt.Fprintf(os.Stderr, "Error: device is required for set command\n")
					os.Exit(1)
				}

				speed, _ := cmd.Flags().GetUint("speed")
				if speed == 0 {
					fmt.Fprintf(os.Stderr, "Error: speed is required for set command\n")
					os.Exit(1)
				}

				// Set cooling speed
				if err := client.SetCoolingSpeed(device, speed); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				fmt.Printf("Set cooling device %s to speed %d\n", device, speed)
			}
		},
	}

	// Add flags
	cmd.Flags().StringP("cmd", "c", "", "Specify command [set, status]")
	cmd.Flags().String("device", "", "Specify the cooling device")
	cmd.Flags().UintP("speed", "s", 0, "Specify the cooling device speed")
	cmd.MarkFlagRequired("cmd")

	return cmd
}

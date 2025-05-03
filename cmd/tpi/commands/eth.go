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

// newEthCommand creates the ethernet command
func newEthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "eth",
		Short: "Configure the on-board Ethernet switch",
		Long:  "Configure the on-board Ethernet switch.",
		Run: func(cmd *cobra.Command, args []string) {
			// Get command
			cmdStr, _ := cmd.Flags().GetString("cmd")
			if cmdStr == "" {
				fmt.Fprintln(os.Stderr, "Error: command is required")
				os.Exit(1)
			}

			// Make sure the command is valid
			if cmdStr != "reset" {
				fmt.Fprintf(os.Stderr, "Error: invalid command: %s (must be reset)\n", cmdStr)
				os.Exit(1)
			}

			// Create a client
			client, err := getClient(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Execute the command based on the command string
			switch cmdStr {
			case "reset":
				fmt.Println("Resetting Ethernet switch...")
				if err := client.EthReset(); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("ok")
			}
		},
	}

	// Add flags
	cmd.Flags().StringP("cmd", "c", "", "Specify command [reset]")
	cmd.MarkFlagRequired("cmd")

	return cmd
}

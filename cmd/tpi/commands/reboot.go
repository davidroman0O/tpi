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

// newRebootCommand creates the reboot command
func newRebootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reboot",
		Short: "Reboot the BMC chip",
		Long:  "Reboot the BMC chip. Nodes will lose power until booted!",
		Run: func(cmd *cobra.Command, args []string) {
			// Create a client
			client, err := getClient(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Get confirmation
			fmt.Println("WARNING: Rebooting the BMC will cause all nodes to lose power until the BMC boots up again.")
			fmt.Print("Are you sure you want to continue? [y/N] ")

			var response string
			fmt.Scanln(&response)

			if response != "y" && response != "Y" {
				fmt.Println("Reboot cancelled.")
				return
			}

			// Reboot
			if err := client.Reboot(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("BMC is rebooting...")
		},
	}

	return cmd
}

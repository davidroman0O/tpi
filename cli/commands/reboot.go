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
	var waitForBoot bool
	var waitTimeout int
	var skipConfirmation bool
	var showDebug bool

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

			// Get confirmation unless skipped
			if !skipConfirmation {
				fmt.Println("WARNING: Rebooting the BMC will cause all nodes to lose power until the BMC boots up again.")
				fmt.Print("Are you sure you want to continue? [y/N] ")

				var response string
				fmt.Scanln(&response)

				if response != "y" && response != "Y" {
					fmt.Println("Reboot cancelled.")
					return
				}
			}

			// If wait is requested, use RebootAndWait
			if waitForBoot {
				fmt.Println("BMC is rebooting...")
				fmt.Printf("Waiting for BMC to come back online (timeout: %d seconds)\n", waitTimeout)

				// Store original stdout if we need to hide debug output
				var originalStdout *os.File
				var null *os.File

				if !showDebug {
					// Temporarily redirect debug output to /dev/null
					originalStdout = os.Stdout
					var err error
					null, err = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
					if err == nil {
						os.Stdout = null
						defer func() {
							os.Stdout = originalStdout
							null.Close()
						}()
					}
				}

				// Call the reboot method
				err := client.RebootAndWait(waitTimeout)

				// If we redirected debug output, restore stdout before printing progress
				if !showDebug && originalStdout != nil {
					os.Stdout = originalStdout
				}

				// Print final result
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				fmt.Println("BMC is back online!")
			} else {
				// Just reboot without waiting
				if err := client.Reboot(); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("BMC is rebooting...")
			}
		},
	}

	// Add flags
	cmd.Flags().BoolVarP(&waitForBoot, "wait", "w", false, "Wait for the BMC to come back online after reboot")
	cmd.Flags().IntVarP(&waitTimeout, "timeout", "t", 120, "Timeout in seconds when waiting for BMC to come back online")
	cmd.Flags().BoolVarP(&skipConfirmation, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().BoolVarP(&showDebug, "debug", "d", false, "Show debug output during wait")

	return cmd
}

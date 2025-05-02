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
	"path/filepath"

	"github.com/spf13/cobra"
)

// newFirmwareCommand creates the firmware command
func newFirmwareCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "firmware",
		Short: "Upgrade the firmware of the BMC",
		Long:  "Upgrade the firmware of the BMC.",
		Run: func(cmd *cobra.Command, args []string) {
			// Get required flags
			file, _ := cmd.Flags().GetString("file")
			if file == "" {
				fmt.Fprintln(os.Stderr, "Error: firmware file is required")
				os.Exit(1)
			}

			// Check if file exists
			if _, err := os.Stat(file); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Error: firmware file does not exist: %s\n", file)
				os.Exit(1)
			}

			// Get optional SHA256 checksum
			sha256, _ := cmd.Flags().GetString("sha256")

			// Create a client
			client, err := getClient(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Get file name for display
			fileName := filepath.Base(file)
			fmt.Printf("Upgrading firmware with %s...\n", fileName)

			// Upload firmware
			if err := client.UpgradeFirmware(file, sha256); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Firmware upgrade completed successfully")
		},
	}

	// Add flags
	cmd.Flags().StringP("file", "f", "", "Firmware file path")
	cmd.Flags().String("sha256", "", "SHA256 checksum for verification")
	cmd.MarkFlagRequired("file")

	return cmd
}

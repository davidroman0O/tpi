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

	tpi "github.com/davidroman0O/tpi/client"
	"github.com/spf13/cobra"
)

// newFlashCommand creates the flash command
func newFlashCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flash",
		Short: "Flash a given node",
		Long:  "Flash a given node with an OS image.",
		Run: func(cmd *cobra.Command, args []string) {
			// Get flags
			local, _ := cmd.Flags().GetBool("local")
			imagePath, _ := cmd.Flags().GetString("image-path")
			if imagePath == "" {
				fmt.Fprintln(os.Stderr, "Error: image path is required")
				os.Exit(1)
			}

			node, _ := cmd.Flags().GetInt("node")
			if node < 1 || node > 4 {
				fmt.Fprintf(os.Stderr, "Error: node number must be between 1 and 4, got %d\n", node)
				os.Exit(1)
			}

			sha256, _ := cmd.Flags().GetString("sha256")
			skipCrc, _ := cmd.Flags().GetBool("skip-crc")

			// Create a client
			client, err := getClient(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// If local flag is set, use local flash
			if local {
				fmt.Printf("Flashing node %d from local file %s...\n", node, imagePath)
				if err := client.FlashNodeLocal(node, imagePath); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				return
			}

			// Otherwise, check if image file exists
			if _, err := os.Stat(imagePath); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Error: image file does not exist: %s\n", imagePath)
				os.Exit(1)
			}

			// Get file name for display
			fileName := filepath.Base(imagePath)
			fmt.Printf("Flashing node %d with %s...\n", node, fileName)

			// Flash the node
			options := &tpi.FlashOptions{
				ImagePath: imagePath,
				SHA256:    sha256,
				SkipCRC:   skipCrc,
			}

			if err := client.FlashNode(node, options); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Flash operation completed successfully")
		},
	}

	// Add flags
	cmd.Flags().BoolP("local", "l", false, "Update a node with an image accessible from the local filesystem")
	cmd.Flags().StringP("image-path", "i", "", "Update a node with the given image")
	cmd.Flags().IntP("node", "n", 0, "Node number [1-4]")
	cmd.Flags().String("sha256", "", "SHA256 checksum for verification")
	cmd.Flags().Bool("skip-crc", false, "Opt out of the CRC integrity check")
	cmd.MarkFlagRequired("image-path")
	cmd.MarkFlagRequired("node")

	return cmd
}

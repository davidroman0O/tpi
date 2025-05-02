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
	"errors"
	"fmt"

	"github.com/davidroman0O/tpi"
	"github.com/spf13/cobra"
)

// newAdvancedCommand creates a command for switching nodes between different modes
func newAdvancedCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "advanced",
		Short: "Advanced node modes",
		Long:  "Configure advanced node modes like normal or MSD (Mass Storage Device)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create client
			client, err := getClient(cmd)
			if err != nil {
				return err
			}

			// Get mode
			mode, err := cmd.Flags().GetString("mode")
			if err != nil {
				return err
			}
			if mode == "" {
				return errors.New("mode is required")
			}

			// Get node
			node, err := cmd.Flags().GetInt("node")
			if err != nil {
				return err
			}
			if node <= 0 || node > 4 {
				return fmt.Errorf("invalid node number: %d (must be 1-4)", node)
			}

			// Execute the appropriate command based on mode
			switch tpi.ModeCmd(mode) {
			case tpi.ModeNormal:
				// Set to normal mode
				if err := client.SetNodeNormalMode(node); err != nil {
					return fmt.Errorf("failed to set normal mode: %w", err)
				}
				fmt.Printf("Node %d set to normal mode\n", node)
			case tpi.ModeMsd:
				// Set to mass storage device mode
				if err := client.SetNodeMsdMode(node); err != nil {
					return fmt.Errorf("failed to set MSD mode: %w", err)
				}
				fmt.Printf("Node %d set to MSD (Mass Storage Device) mode\n", node)
			default:
				return fmt.Errorf("unsupported mode: %s (must be 'normal' or 'msd')", mode)
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringP("mode", "m", "", "Specify mode [normal, msd]")
	cmd.Flags().IntP("node", "n", 0, "Node number [1-4]")
	cmd.MarkFlagRequired("mode")
	cmd.MarkFlagRequired("node")

	return cmd
}

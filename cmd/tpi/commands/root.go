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
	"github.com/davidroman0O/tpi"
	"github.com/spf13/cobra"
)

// NewRootCommand creates a new root command
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "tpi",
		Short: "Command-line utility for controlling Turing Pi 2",
		Long: `Command-line utility for controlling Turing Pi 2.

Turing Pi 2 is a compact ARM cluster that can run up to 4 compute 
modules simultaneously. For more information, visit https://turingpi.com/`,
	}

	// Add persistent flags
	rootCmd.PersistentFlags().StringP("host", "H", "", "BMC hostname or IP address")
	rootCmd.PersistentFlags().StringP("user", "u", "", "BMC username")
	rootCmd.PersistentFlags().StringP("password", "p", "", "BMC password")
	rootCmd.PersistentFlags().StringP("api-version", "a", string(tpi.ApiVersionV1_1), "Force which version of the BMC API to use")

	// Add commands
	rootCmd.AddCommand(newPowerCommand())
	rootCmd.AddCommand(newUsbCommand())
	rootCmd.AddCommand(newInfoCommand())
	rootCmd.AddCommand(newRebootCommand())
	rootCmd.AddCommand(newFirmwareCommand())
	rootCmd.AddCommand(newFlashCommand())
	rootCmd.AddCommand(newEthCommand())
	rootCmd.AddCommand(newUartCommand())
	rootCmd.AddCommand(newAdvancedCommand())
	rootCmd.AddCommand(newCoolingCommand())
	rootCmd.AddCommand(newAuthCommand())

	return rootCmd
}

// getClient creates a client from command flags
func getClient(cmd *cobra.Command) (*tpi.Client, error) {
	// Get flags
	host, _ := cmd.Flags().GetString("host")
	user, _ := cmd.Flags().GetString("user")
	password, _ := cmd.Flags().GetString("password")
	apiVersionStr, _ := cmd.Flags().GetString("api-version")

	// Create options
	options := []tpi.Option{
		tpi.WithHost(host),
	}

	// Add API version if specified
	apiVersion := tpi.ApiVersion(apiVersionStr)
	if apiVersion != "" {
		options = append(options, tpi.WithApiVersion(apiVersion))
	}

	// Add credentials if provided
	if user != "" || password != "" {
		options = append(options, tpi.WithCredentials(user, password))
	}

	// Create client
	return tpi.NewClient(options...)
}

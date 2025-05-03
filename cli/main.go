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

package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/davidroman0O/tpi/cli/commands"
	"github.com/spf13/cobra"
)

// Version information set by build flags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Set by linker at build time
var debug string

// enableDebugging is a package-level variable to control debug output
var enableDebugging bool

func main() {
	// Check if we're running in debug mode
	if debug == "true" || os.Getenv("TPI_DEBUG") == "true" {
		enableDebugging = true
		fmt.Println("Running in debug mode")
	}

	// Export the debug flag to the request package
	// Ensure TPI_DEBUG is set properly even in go run
	if os.Getenv("TPI_DEBUG") == "true" {
		os.Setenv("TPI_DEBUG", "true")
		debug = "true"
	} else {
		os.Setenv("TPI_DEBUG", debug)
	}

	// Create a root command
	rootCmd := commands.NewRootCommand()

	// Add version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("TPI CLI %s\n", version)
			fmt.Printf("Commit: %s\n", commit)
			fmt.Printf("Built: %s\n", date)
		},
	})

	// Override cobra's default behavior for help text
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// If this is the help command or -h/--help flag is present, don't validate host
		if cmd.Name() == "help" || cmd.CommandPath() == "tpi" || cmd.Flags().Changed("help") {
			return nil
		}

		// Skip validation for auth commands
		if cmd.Name() == "auth" || cmd.Parent() != nil && cmd.Parent().Name() == "auth" {
			return nil
		}

		// Skip validation for version command
		if cmd.Name() == "version" {
			return nil
		}

		// Get options
		host, _ := cmd.Flags().GetString("host")

		// For all other commands, validate the host
		if host == "" {
			return fmt.Errorf("No host specified. Please provide the Turing Pi hostname with --host.\nExample: tpi --host=192.168.1.91 power status\n\nIf running on a Turing Pi, the host should be detected automatically.")
		}

		return nil
	}

	// Handle generating shell completions
	if len(os.Args) > 1 && os.Args[1] == "-gen" && len(os.Args) > 2 {
		shell := os.Args[2]
		var err error
		switch strings.ToLower(shell) {
		case "bash":
			err = rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			err = rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			err = rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			err = rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			fmt.Fprintf(os.Stderr, "Unknown shell type: %s\n", shell)
			os.Exit(1)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating completion: %s\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Set up host auto-detection
	cobra.OnInitialize(func() {
		if isLocalTuringPi() {
			// Running on a Turing Pi, set the host to localhost if not already set
			hostFlag := rootCmd.PersistentFlags().Lookup("host")
			if hostFlag != nil && hostFlag.Value.String() == "" {
				rootCmd.PersistentFlags().Set("host", "127.0.0.1")
				if debug == "true" {
					fmt.Println("Detected running on a Turing Pi, using 127.0.0.1 as host")
				}
			}
		}
	})

	// Execute the command - Cobra will handle all the argument parsing
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

// isLocalTuringPi tries to detect if we're running on a Turing Pi
func isLocalTuringPi() bool {
	// Try different local addresses that might indicate we're on a Turing Pi
	addresses := []string{"127.0.0.1:80", "127.0.0.1:443"}

	for _, address := range addresses {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
	}

	// Additional checks - try to identify Turing Pi specific hardware or files
	// This is a simple placeholder - we could add more sophisticated detection here
	_, err := os.Stat("/sys/class/gpio/export")
	if err == nil {
		return true
	}

	return false
}

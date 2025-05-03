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
	"sort"

	"github.com/spf13/cobra"
)

// newAboutCommand creates the about command
func newAboutCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "about",
		Short: "Display detailed information about the BMC daemon",
		Long:  "Display detailed daemon information including API version, daemon version, build details, etc.",
		Run: func(cmd *cobra.Command, args []string) {
			// Create a client
			client, err := getClient(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Get detailed daemon info
			about, err := client.About()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Print info in a table
			fmt.Println("\n--- BMC Daemon Information ---")
			fmt.Println("|-----------------|----------------------------|")
			fmt.Println("|       Key       |            Value           |")
			fmt.Println("|-----------------|----------------------------|")

			// Sort keys for consistent output
			keys := make([]string, 0, len(about))
			for key := range about {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			for _, key := range keys {
				fmt.Printf("| %-15s | %-28s |\n", key, about[key])
			}

			fmt.Println("|-----------------|----------------------------|")
		},
	}

	return cmd
}

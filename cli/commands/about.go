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
	"strings"

	"github.com/charmbracelet/glamour"
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

			// Print info using markdown/glamour for nicer formatting
			renderAboutInfo(about)
		},
	}

	return cmd
}

// renderAboutInfo renders the about information as nicely formatted output
func renderAboutInfo(about map[string]string) {
	// Sort keys for consistent output
	keys := make([]string, 0, len(about))
	for key := range about {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Build markdown content
	var md strings.Builder
	md.WriteString("# BMC Daemon Information\n\n")
	md.WriteString("| Key | Value |\n")
	md.WriteString("|-----|-------|\n")

	for _, key := range keys {
		md.WriteString(fmt.Sprintf("| **%s** | %s |\n", key, about[key]))
	}

	// Set up the renderer with the dark theme
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		// Fallback to plain text if renderer fails
		fmt.Println("\n--- BMC Daemon Information ---")
		fmt.Println("|-----------------|----------------------------|")
		fmt.Println("|       Key       |            Value           |")
		fmt.Println("|-----------------|----------------------------|")

		for _, key := range keys {
			fmt.Printf("| %-15s | %-28s |\n", key, about[key])
		}

		fmt.Println("|-----------------|----------------------------|")
		return
	}

	// Render and print the markdown
	out, err := renderer.Render(md.String())
	if err != nil {
		// Fallback to plain text if rendering fails
		fmt.Println(md.String())
		return
	}

	fmt.Println(out)
}

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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/davidroman0O/tpi"
	"github.com/spf13/cobra"
)

// newAuthCommand creates the auth command
func newAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication and token persistence",
		Long:  "Commands to manage authentication, login/logout, and token persistence",
	}

	// Add subcommands
	cmd.AddCommand(newAuthLoginCommand())
	cmd.AddCommand(newAuthLogoutCommand())
	cmd.AddCommand(newAuthStatusCommand())

	return cmd
}

// newAuthLoginCommand creates the login subcommand
func newAuthLoginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate and cache token",
		Long:  "Explicitly authenticate with the BMC and cache the token for future use",
		Example: `  # Login with specified credentials
  tpi auth login --host=192.168.1.91 --user=root --password=turing
  
  # Login with just a host (will try default credentials)
  tpi auth login --host=192.168.1.91`,
		Run: func(cmd *cobra.Command, args []string) {
			// Ensure host is specified
			host, _ := cmd.Flags().GetString("host")
			if host == "" {
				fmt.Fprintln(os.Stderr, "Error: host is required")
				os.Exit(1)
			}

			// Get user and password
			user, _ := cmd.Flags().GetString("user")
			password, _ := cmd.Flags().GetString("password")

			// Try direct HTTP approach first - this is most reliable
			if user != "" && password != "" {
				fmt.Println("Attempting direct HTTP authentication...")

				// Create JSON payload
				payload := fmt.Sprintf(`{"username":"%s","password":"%s"}`, user, password)

				// Create HTTP request
				req, err := http.NewRequest("POST",
					fmt.Sprintf("https://%s/api/bmc/authenticate", host),
					strings.NewReader(payload))
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
				} else {
					// Set headers
					req.Header.Set("Content-Type", "application/json")

					// Skip TLS verification
					tr := &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					}
					client := &http.Client{Transport: tr}

					// Send request
					fmt.Println("Sending request...")
					resp, err := client.Do(req)
					if err != nil {
						fmt.Fprintf(os.Stderr, "HTTP request failed: %v\n", err)
					} else {
						defer resp.Body.Close()

						// Read response
						body, err := io.ReadAll(resp.Body)
						if err != nil {
							fmt.Fprintf(os.Stderr, "Failed to read response: %v\n", err)
						} else {
							// Try to extract token
							var response map[string]interface{}
							if err := json.Unmarshal(body, &response); err != nil {
								fmt.Fprintf(os.Stderr, "Failed to parse response: %v\n%s\n", err, string(body))
							} else {
								// Look for token in id field
								if token, ok := response["id"].(string); ok {
									// Cache token
									if err := tpi.CacheToken(host, token); err != nil {
										fmt.Fprintf(os.Stderr, "Failed to cache token: %v\n", err)
									} else {
										fmt.Printf("Successfully authenticated to %s and cached token\n", host)
										return
									}
								} else {
									fmt.Fprintf(os.Stderr, "Token not found in response: %s\n", string(body))
								}
							}
						}
					}
				}

				// If we're here, direct HTTP approach failed
				fmt.Println("Direct HTTP approach failed, trying curl...")
			}

			// Try using curl as a fallback
			if user != "" && password != "" {
				// Build the curl command with more verbose output
				curlCmd := exec.Command("curl", "-v", "-k", "-X", "POST",
					"-H", "Content-Type: application/json",
					"-d", fmt.Sprintf(`{"username":"%s","password":"%s"}`, user, password),
					fmt.Sprintf("https://%s/api/bmc/authenticate", host))

				// Print the exact command being executed (without password)
				fmt.Println("Executing curl command:", strings.Replace(curlCmd.String(), password, "********", -1))

				// Capture output
				output, err := curlCmd.CombinedOutput()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: curl command failed: %v\n%s\n", err, string(output))
					os.Exit(1)
				}

				// Print raw output for debugging
				fmt.Println("Raw curl output:", string(output))

				// Try to parse response
				var response map[string]interface{}
				if err := json.Unmarshal(output, &response); err != nil {
					fmt.Fprintf(os.Stderr, "Error: failed to parse response: %v\n%s\n", err, string(output))
					os.Exit(1)
				}

				// Get token from id field
				tokenVal, ok := response["id"]
				if !ok {
					fmt.Fprintf(os.Stderr, "Error: token not found in response\n%s\n", string(output))
					os.Exit(1)
				}

				token, ok := tokenVal.(string)
				if !ok {
					fmt.Fprintf(os.Stderr, "Error: token is not a string\n%s\n", string(output))
					os.Exit(1)
				}

				// Cache the token
				if err := tpi.CacheToken(host, token); err != nil {
					fmt.Fprintf(os.Stderr, "Error: failed to cache token: %v\n", err)
					os.Exit(1)
				}

				fmt.Printf("Successfully authenticated to %s and cached token\n", host)
				return
			}

			// Fall back to our client implementation if no username/password
			client, err := getClient(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Log in
			if err := client.Login(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Successfully authenticated to %s and cached token\n", host)
		},
	}

	return cmd
}

// newAuthLogoutCommand creates the logout subcommand
func newAuthLogoutCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear cached token",
		Long:  "Remove cached authentication token",
		Example: `  # Logout from a specific host
  tpi auth logout --host=192.168.1.91
  
  # Logout from all hosts (clear all tokens)
  tpi auth logout`,
		Run: func(cmd *cobra.Command, args []string) {
			host, _ := cmd.Flags().GetString("host")

			if host == "" {
				// Clear all tokens
				if err := tpi.DeleteAllCachedTokens(); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("Successfully logged out - all token caches cleared")
			} else {
				// Clear token for specific host
				if err := tpi.DeleteCachedToken(host); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Successfully logged out from %s - token cache cleared\n", host)
			}
		},
	}

	return cmd
}

// newAuthStatusCommand creates the status subcommand
func newAuthStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check authentication status",
		Long:  "Check if there is a cached authentication token",
		Example: `  # Check auth status for a specific host
  tpi auth status --host=192.168.1.91
  
  # Check auth status for all hosts
  tpi auth status`,
		Run: func(cmd *cobra.Command, args []string) {
			host, _ := cmd.Flags().GetString("host")

			if host == "" {
				// List all cached tokens
				hosts, err := tpi.GetAllCachedTokens()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				if len(hosts) == 0 {
					fmt.Println("No cached authentication tokens found")
				} else {
					fmt.Println("Cached authentication tokens found for:")
					for _, h := range hosts {
						fmt.Printf("- %s\n", h)
					}
				}
			} else {
				// Check if token exists for specific host
				_, err := tpi.GetCachedToken(host)
				if err != nil {
					fmt.Printf("Not authenticated to %s (no cached token)\n", host)
				} else {
					fmt.Printf("Authenticated to %s (token is cached)\n", host)
				}
			}
		},
	}

	return cmd
}

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

package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

// Simple prompts for a string input with the given message
func Simple(msg string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s: ", msg)

	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	// Trim newline characters
	return strings.TrimSpace(input), nil
}

// Password prompts for a password with the given message (without echoing)
func Password(msg string) (string, error) {
	fmt.Printf("%s: ", msg)

	// Get file descriptor of standard input
	fd := int(syscall.Stdin)

	// Read password without echoing
	bytePassword, err := terminal.ReadPassword(fd)
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	// Print a newline after password input (since terminal.ReadPassword doesn't do it)
	fmt.Println()

	return string(bytePassword), nil
}

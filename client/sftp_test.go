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

package tpi

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSFTPFunctionality tests the SFTP file transfer functionality
func TestSFTPFunctionality(t *testing.T) {
	// Create a client with the settings from config.json
	client := createTestClient(t)

	// Get SSH credentials from the test config
	config := loadTestConfig(t)

	// VERBOSE DEBUG: Print connection details
	t.Logf("*** TEST CONNECTING TO: %s with user: %s ***", config.Host, config.Username)
	t.Logf("*** CHECKING FOR MOCK INTERFACES ***")

	// Create a unique timestamp for this test run
	timestamp := time.Now().Unix()
	t.Logf("*** TEST RUN TIMESTAMP: %d ***", timestamp)

	// Check if the test config has SSH credentials defined
	if config.Username == "" || config.Password == "" {
		t.Fatalf("SFTP test requires SSH credentials in testdata/config.json")
	}

	// Check if we can reach the SSH server
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:22", config.Host), 5*time.Second)
	if err != nil {
		t.Fatalf("Cannot reach SSH server at %s:22: %v - SFTP test requires a running SSH server", config.Host, err)
	}
	conn.Close()
	t.Logf("*** TCP CONNECTION SUCCESSFUL TO %s:22 ***", config.Host)

	// Generate a random file for testing to ensure we're not using cached data
	testFileSize := int64(1024 * 1024) // 1MB
	content := make([]byte, testFileSize)
	_, err = rand.Read(content)
	if err != nil {
		t.Fatalf("Failed to generate random content: %v", err)
	}

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "sftp-test-*.dat")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	t.Logf("*** CREATED LOCAL TEMP FILE: %s ***", tempFile.Name())

	// Write the content to the file
	_, err = tempFile.Write(content)
	if err != nil {
		t.Fatalf("Failed to write to temporary file: %v", err)
	}
	tempFile.Close()

	// Verify the content was written correctly
	writtenContent, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to read the written file: %v", err)
	}

	if !bytes.Equal(content, writtenContent) {
		t.Fatalf("Written content doesn't match original content")
	}

	// Create a directory for download
	downloadDir, err := os.MkdirTemp("", "sftp-download")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(downloadDir)
	t.Logf("*** CREATED LOCAL DOWNLOAD DIR: %s ***", downloadDir)

	// Test uploading a file
	t.Log("Testing file upload...")
	remotePath := fmt.Sprintf("/tmp/sftp-test-%d.dat", timestamp)
	t.Logf("*** ATTEMPTING TO UPLOAD TO REMOTE PATH: %s ***", remotePath)

	// Execute a command to create a marker file with our test timestamp
	markerOutput, markerErr := client.ExecuteCommand(
		fmt.Sprintf("echo 'Test run %d' > /tmp/sftp-verify-%d.txt", timestamp, timestamp),
		WithSSHCredentials(config.Username, config.Password),
		WithSSHPort(22),
	)
	if markerErr != nil {
		t.Logf("*** WARNING: Could not create marker file: %v ***", markerErr)
	} else {
		t.Logf("*** CREATED MARKER FILE WITH OUTPUT: %s ***", markerOutput)
	}

	err = client.UploadFile(tempFile.Name(), remotePath,
		WithSSHCredentials(config.Username, config.Password),
		WithSSHPort(22),
	)
	if err != nil {
		t.Fatalf("Failed to upload file: %v", err)
	}
	t.Logf("Uploaded file to %s", remotePath)

	// Test listing directory
	t.Log("Testing directory listing...")
	files, err := client.ListDirectory("/tmp",
		WithSSHCredentials(config.Username, config.Password),
		WithSSHPort(22),
	)
	if err != nil {
		t.Fatalf("Failed to list directory: %v", err)
	}

	t.Log("Files in /tmp:")
	found := false
	markerFound := false
	var remoteFileSize int64
	for _, file := range files {
		t.Logf("*** FOUND FILE: %s (%d bytes, isDir: %t) ***", file.Name, file.Size, file.IsDir)

		if file.Name == filepath.Base(remotePath) {
			found = true
			remoteFileSize = file.Size
			t.Logf("- %s (%d bytes, isDir: %t, mode: %s)",
				file.Name, file.Size, file.IsDir, file.Mode.String())
		}

		// Check if our marker file is present
		if file.Name == fmt.Sprintf("sftp-verify-%d.txt", timestamp) {
			markerFound = true
			t.Logf("*** MARKER FILE FOUND: %s ***", file.Name)
		}
	}

	if !found {
		t.Fatalf("Uploaded file not found in directory listing")
	}

	if !markerFound {
		t.Logf("*** WARNING: MARKER FILE NOT FOUND IN DIRECTORY LISTING ***")
	}

	// Verify the file size matches
	if remoteFileSize != testFileSize {
		t.Fatalf("Remote file size (%d) doesn't match original size (%d)",
			remoteFileSize, testFileSize)
	}

	// Test downloading a file
	t.Log("Testing file download...")
	downloadPath := filepath.Join(downloadDir, "downloaded.dat")
	t.Logf("*** ATTEMPTING TO DOWNLOAD TO LOCAL PATH: %s ***", downloadPath)
	err = client.DownloadFile(remotePath, downloadPath,
		WithSSHCredentials(config.Username, config.Password),
		WithSSHPort(22),
	)
	if err != nil {
		t.Fatalf("Failed to download file: %v", err)
	}
	t.Logf("Downloaded file to %s", downloadPath)

	// Verify download
	downloadedContent, err := os.ReadFile(downloadPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if !bytes.Equal(downloadedContent, content) {
		t.Fatalf("Downloaded content doesn't match original content")
	}
	t.Log("Downloaded content matches original content")

	// Test executing a command
	t.Log("Testing command execution...")
	verifyCmd := fmt.Sprintf("ls -la /tmp/sftp-test-%d.dat /tmp/sftp-verify-%d.txt", timestamp, timestamp)
	output, err := client.ExecuteCommand(verifyCmd,
		WithSSHCredentials(config.Username, config.Password),
		WithSSHPort(22),
	)
	if err != nil {
		t.Logf("*** WARNING: Verification command failed: %v ***", err)
	} else {
		t.Logf("*** VERIFICATION OUTPUT: %s ***", output)
	}

	output, err = client.ExecuteCommand("echo 'Hello from Turing Pi'",
		WithSSHCredentials(config.Username, config.Password),
		WithSSHPort(22),
	)
	if err != nil {
		t.Fatalf("Failed to execute command: %v", err)
	}
	t.Logf("Command output: %s", output)

	// Clean up the remote file
	t.Log("Cleaning up...")
	_, err = client.ExecuteCommand(fmt.Sprintf("rm %s /tmp/sftp-verify-%d.txt", remotePath, timestamp),
		WithSSHCredentials(config.Username, config.Password),
		WithSSHPort(22),
	)
	if err != nil {
		t.Fatalf("Failed to remove remote file: %v", err)
	}
	t.Log("Test completed successfully")
}

// TestSFTPExecuteCommand verifies the command execution functionality
func TestSFTPExecuteCommand(t *testing.T) {
	// Create a client with the settings from config.json
	client := createTestClient(t)

	// Get SSH credentials from the test config
	config := loadTestConfig(t)

	// VERBOSE DEBUG: Print connection details
	t.Logf("*** TEST CONNECTING TO: %s with user: %s ***", config.Host, config.Username)

	// Check if the test config has SSH credentials defined
	if config.Username == "" || config.Password == "" {
		t.Fatalf("SSH test requires credentials in testdata/config.json")
	}

	// Check if we can reach the SSH server
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:22", config.Host), 5*time.Second)
	if err != nil {
		t.Fatalf("Cannot reach SSH server at %s:22: %v - SSH test requires a running SSH server", config.Host, err)
	}
	conn.Close()
	t.Logf("*** TCP CONNECTION SUCCESSFUL TO %s:22 ***", config.Host)

	// Test executing various commands
	testCases := []struct {
		name    string
		command string
	}{
		{"Echo Test", "echo 'Hello from Turing Pi'"},
		{"Directory Listing", "ls -la /tmp"},
		{"Create and Verify File", "touch /tmp/test-file && ls -la /tmp/test-file && rm /tmp/test-file"},
		{"Process List", "ps aux | head -5"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("*** EXECUTING COMMAND: %s ***", tc.command)
			output, err := client.ExecuteCommand(tc.command,
				WithSSHCredentials(config.Username, config.Password),
				WithSSHPort(22),
			)
			if err != nil {
				t.Fatalf("Failed to execute command '%s': %v", tc.command, err)
			}
			t.Logf("Command output (%s): %s", tc.name, output)
		})
	}
}

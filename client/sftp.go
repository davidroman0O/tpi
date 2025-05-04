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
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHConfig holds the configuration for SSH connections
type SSHConfig struct {
	Host       string
	Port       int
	User       string
	Password   string
	PrivateKey string
	Timeout    time.Duration
}

// SSHOption is a function that configures an SSHConfig
type SSHOption func(*SSHConfig)

// WithSSHCredentials sets the SSH credentials
func WithSSHCredentials(username, password string) SSHOption {
	return func(c *SSHConfig) {
		c.User = username
		c.Password = password
	}
}

// WithSSHPrivateKey sets the SSH private key
func WithSSHPrivateKey(privateKey string) SSHOption {
	return func(c *SSHConfig) {
		c.PrivateKey = privateKey
	}
}

// WithSSHPort sets the SSH port
func WithSSHPort(port int) SSHOption {
	return func(c *SSHConfig) {
		c.Port = port
	}
}

// WithSSHTimeout sets the SSH connection timeout
func WithSSHTimeout(timeout time.Duration) SSHOption {
	return func(c *SSHConfig) {
		c.Timeout = timeout
	}
}

// FileInfo represents information about a file on the remote system
type FileInfo struct {
	Name    string
	Size    int64
	Mode    fs.FileMode
	ModTime time.Time
	IsDir   bool
}

// getSSHClient creates an SSH client connection
func (c *Client) getSSHClient(options ...SSHOption) (*ssh.Client, error) {
	// Default SSH configuration
	sshConfig := &SSHConfig{
		Host:    c.Host,
		Port:    22,
		User:    c.auth.Username,
		Timeout: 10 * time.Second,
	}

	// Apply options
	for _, option := range options {
		option(sshConfig)
	}

	// Create SSH config
	config := &ssh.ClientConfig{
		User:            sshConfig.User,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         sshConfig.Timeout,
	}

	// Add authentication methods
	if sshConfig.Password != "" {
		config.Auth = append(config.Auth, ssh.Password(sshConfig.Password))
	}

	if sshConfig.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(sshConfig.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", sshConfig.Host, sshConfig.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	return client, nil
}

// UploadFile uploads a local file to the remote system using SFTP
func (c *Client) UploadFile(localPath, remotePath string, options ...SSHOption) error {
	// Open the local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Get file stat
	stat, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if stat.IsDir() {
		return fmt.Errorf("cannot upload a directory, only files are supported")
	}

	// Get SSH client
	client, err := c.getSSHClient(options...)
	if err != nil {
		return fmt.Errorf("failed to establish SSH connection: %w", err)
	}
	defer client.Close()

	// Create new SFTP client
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Ensure the remote directory exists
	remoteDir := filepath.Dir(remotePath)
	if remoteDir != "." && remoteDir != "/" {
		if err := sftpClient.MkdirAll(remoteDir); err != nil {
			return fmt.Errorf("failed to create remote directory: %w", err)
		}
	}

	// Create remote file
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Set mode
	if err := sftpClient.Chmod(remotePath, stat.Mode()); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Copy file content
	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

// DownloadFile downloads a file from the remote system using SFTP
func (c *Client) DownloadFile(remotePath, localPath string, options ...SSHOption) error {
	// Get SSH client
	client, err := c.getSSHClient(options...)
	if err != nil {
		return fmt.Errorf("failed to establish SSH connection: %w", err)
	}
	defer client.Close()

	// Create new SFTP client
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Check if remote file exists and is not a directory
	remoteFileInfo, err := sftpClient.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("failed to stat remote file: %w", err)
	}

	if remoteFileInfo.IsDir() {
		return fmt.Errorf("cannot download a directory, only files are supported")
	}

	// Create the local directory if it doesn't exist
	localDir := filepath.Dir(localPath)
	if localDir != "." && localDir != "/" {
		if err := os.MkdirAll(localDir, 0755); err != nil {
			return fmt.Errorf("failed to create local directory: %w", err)
		}
	}

	// Open remote file
	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	// Create local file
	localFile, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, remoteFileInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	// Copy file content
	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

// ListDirectory lists the contents of a remote directory using SFTP
func (c *Client) ListDirectory(remotePath string, options ...SSHOption) ([]FileInfo, error) {
	// Get SSH client
	client, err := c.getSSHClient(options...)
	if err != nil {
		return nil, fmt.Errorf("failed to establish SSH connection: %w", err)
	}
	defer client.Close()

	// Create new SFTP client
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Check if path exists and is a directory
	pathInfo, err := sftpClient.Stat(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat remote path: %w", err)
	}

	if !pathInfo.IsDir() {
		return nil, fmt.Errorf("remote path must be a directory: %s", remotePath)
	}

	// Read directory contents
	entries, err := sftpClient.ReadDir(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Convert to our FileInfo format
	var files []FileInfo
	for _, entry := range entries {
		files = append(files, FileInfo{
			Name:    entry.Name(),
			Size:    entry.Size(),
			Mode:    entry.Mode(),
			ModTime: entry.ModTime(),
			IsDir:   entry.IsDir(),
		})
	}

	return files, nil
}

// ExecuteCommand executes a command on the remote system and returns the output
func (c *Client) ExecuteCommand(command string, options ...SSHOption) (string, error) {
	// Get SSH client
	client, err := c.getSSHClient(options...)
	if err != nil {
		return "", fmt.Errorf("failed to establish SSH connection: %w", err)
	}
	defer client.Close()

	// Create new session
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Execute the command and get the output
	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command execution failed: %w", err)
	}

	return string(output), nil
}

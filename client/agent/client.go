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

package agent

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	tpi "github.com/davidroman0O/tpi/client"
)

// AgentClient represents a client for the TPI agent
type AgentClient struct {
	config     AgentClientConfig
	httpClient *http.Client
	auth       AgentAuthConfig
}

// NewAgentClient creates a new agent client with the given configuration
func NewAgentClient(config AgentClientConfig) (*AgentClient, error) {
	// Validate configuration
	if config.Host == "" {
		return nil, fmt.Errorf("host is required")
	}

	if config.Port == 0 {
		config.Port = DefaultAgentPort
	}

	// Set default timeout if not specified
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	// Configure TLS
	transport := &http.Transport{}
	if config.TLSEnabled {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: config.SkipVerify,
		}
	}

	// Create HTTP client
	httpClient := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	// Generate a token if not provided but secret is
	auth := config.Auth
	if auth.Secret != "" && auth.Token == "" {
		token, err := generateRandomToken()
		if err != nil {
			return nil, fmt.Errorf("failed to generate token: %w", err)
		}
		auth.Token = token
	}

	return &AgentClient{
		config:     config,
		httpClient: httpClient,
		auth:       auth,
	}, nil
}

// AgentOption is a function that configures an AgentClientConfig
type AgentOption func(*AgentClientConfig)

// WithAgentHost sets the agent host
func WithAgentHost(host string) AgentOption {
	return func(cfg *AgentClientConfig) {
		cfg.Host = host
	}
}

// WithAgentPort sets the agent port
func WithAgentPort(port int) AgentOption {
	return func(cfg *AgentClientConfig) {
		cfg.Port = port
	}
}

// WithAgentTLS enables TLS for the agent connection
func WithAgentTLS(enabled bool, skipVerify bool) AgentOption {
	return func(cfg *AgentClientConfig) {
		cfg.TLSEnabled = enabled
		cfg.SkipVerify = skipVerify
	}
}

// WithAgentSecret sets the authentication secret for the agent
func WithAgentSecret(secret string) AgentOption {
	return func(cfg *AgentClientConfig) {
		cfg.Auth.Secret = secret
	}
}

// WithAgentToken sets an explicit authentication token for the agent
func WithAgentToken(token string) AgentOption {
	return func(cfg *AgentClientConfig) {
		cfg.Auth.Token = token
	}
}

// WithAgentTimeout sets the timeout for agent requests
func WithAgentTimeout(timeout time.Duration) AgentOption {
	return func(cfg *AgentClientConfig) {
		cfg.Timeout = timeout
	}
}

// NewAgentClientFromOptions creates a new agent client with the provided options
func NewAgentClientFromOptions(opts ...AgentOption) (*AgentClient, error) {
	// Create default configuration
	config := AgentClientConfig{
		Port: DefaultAgentPort,
	}

	// Apply options
	for _, opt := range opts {
		opt(&config)
	}

	// Create and return the client
	return NewAgentClient(config)
}

// generateRandomToken generates a random token for authentication
func generateRandomToken() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// sendCommand sends a command to the agent and returns the response
func (c *AgentClient) sendCommand(cmdType CommandType, args map[string]any) (interface{}, error) {
	// Create the command
	cmd := Command{
		Type: cmdType,
		Args: args,
		Auth: c.auth,
	}

	// Marshal the command to JSON
	cmdJSON, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	// Create request URL
	protocol := "http"
	if c.config.TLSEnabled {
		protocol = "https"
	}
	url := fmt.Sprintf("%s://%s:%d/api/agent", protocol, c.config.Host, c.config.Port)

	// Create the HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(cmdJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TPI-Agent-Client")

	// Send the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check if the response code is not 200
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if the command was successful
	if !response.Success {
		return nil, fmt.Errorf("command failed: %s", response.Error)
	}

	return response.Result, nil
}

// Info gets the basic information about the Turing Pi
func (c *AgentClient) Info() (map[string]string, error) {
	result, err := c.sendCommand(CmdInfo, nil)
	if err != nil {
		return nil, err
	}

	// Convert to map[string]string
	info := make(map[string]string)
	if resultMap, ok := result.(map[string]interface{}); ok {
		for k, v := range resultMap {
			if str, ok := v.(string); ok {
				info[k] = str
			} else {
				info[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	return info, nil
}

// About gets detailed information about the BMC daemon
func (c *AgentClient) About() (map[string]string, error) {
	result, err := c.sendCommand(CmdAbout, nil)
	if err != nil {
		return nil, err
	}

	// Convert to map[string]string
	info := make(map[string]string)
	if resultMap, ok := result.(map[string]interface{}); ok {
		for k, v := range resultMap {
			if str, ok := v.(string); ok {
				info[k] = str
			} else {
				info[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	return info, nil
}

// Reboot reboots the BMC
func (c *AgentClient) Reboot() error {
	_, err := c.sendCommand(CmdReboot, nil)
	return err
}

// RebootAndWait reboots the BMC and waits for it to come back online
func (c *AgentClient) RebootAndWait(timeout int) error {
	args := map[string]any{
		"timeout": timeout,
	}
	_, err := c.sendCommand(CmdRebootAndWait, args)
	return err
}

// PowerStatus gets the power status of all nodes
func (c *AgentClient) PowerStatus() (map[int]bool, error) {
	result, err := c.sendCommand(CmdPowerStatus, nil)
	if err != nil {
		return nil, err
	}

	// Convert to map[int]bool
	status := make(map[int]bool)
	if resultMap, ok := result.(map[string]interface{}); ok {
		for k, v := range resultMap {
			var node int
			if _, err := fmt.Sscanf(k, "%d", &node); err == nil {
				if b, ok := v.(bool); ok {
					status[node] = b
				}
			}
		}
	}

	return status, nil
}

// PowerOn turns on the specified node
func (c *AgentClient) PowerOn(node int) error {
	args := map[string]any{
		"node": node,
	}
	_, err := c.sendCommand(CmdPowerOn, args)
	return err
}

// PowerOff turns off the specified node
func (c *AgentClient) PowerOff(node int) error {
	args := map[string]any{
		"node": node,
	}
	_, err := c.sendCommand(CmdPowerOff, args)
	return err
}

// PowerReset resets the specified node
func (c *AgentClient) PowerReset(node int) error {
	args := map[string]any{
		"node": node,
	}
	_, err := c.sendCommand(CmdPowerReset, args)
	return err
}

// PowerOnAll turns on all nodes
func (c *AgentClient) PowerOnAll() error {
	_, err := c.sendCommand(CmdPowerOnAll, nil)
	return err
}

// PowerOffAll turns off all nodes
func (c *AgentClient) PowerOffAll() error {
	_, err := c.sendCommand(CmdPowerOffAll, nil)
	return err
}

// UsbGetStatus gets the USB mode of each node
func (c *AgentClient) UsbGetStatus() (map[int]bool, error) {
	result, err := c.sendCommand(CmdUsbGetStatus, nil)
	if err != nil {
		return nil, err
	}

	// Convert to map[int]bool
	status := make(map[int]bool)
	if resultMap, ok := result.(map[string]interface{}); ok {
		for k, v := range resultMap {
			var node int
			if _, err := fmt.Sscanf(k, "%d", &node); err == nil {
				if b, ok := v.(bool); ok {
					status[node] = b
				}
			}
		}
	}

	return status, nil
}

// UsbSetHost configures the specified node as USB host
func (c *AgentClient) UsbSetHost(node int, bmc bool) error {
	args := map[string]any{
		"node": node,
		"bmc":  bmc,
	}
	_, err := c.sendCommand(CmdUsbSetHost, args)
	return err
}

// UsbSetDevice configures the specified node as USB device
func (c *AgentClient) UsbSetDevice(node int, bmc bool) error {
	args := map[string]any{
		"node": node,
		"bmc":  bmc,
	}
	_, err := c.sendCommand(CmdUsbSetDevice, args)
	return err
}

// UsbSetFlash configures the specified node in flash mode
func (c *AgentClient) UsbSetFlash(node int, bmc bool) error {
	args := map[string]any{
		"node": node,
		"bmc":  bmc,
	}
	_, err := c.sendCommand(CmdUsbSetFlash, args)
	return err
}

// GetUartOutput gets the UART output from the specified node
func (c *AgentClient) GetUartOutput(node int) (string, error) {
	args := map[string]any{
		"node": node,
	}
	result, err := c.sendCommand(CmdGetUartOutput, args)
	if err != nil {
		return "", err
	}

	if str, ok := result.(string); ok {
		return str, nil
	}
	return fmt.Sprintf("%v", result), nil
}

// SendUartCommand sends a command to the specified node over UART
func (c *AgentClient) SendUartCommand(node int, command string) error {
	args := map[string]any{
		"node":    node,
		"command": command,
	}
	_, err := c.sendCommand(CmdSendUartCommand, args)
	return err
}

// EthReset resets the on-board Ethernet switch
func (c *AgentClient) EthReset() error {
	_, err := c.sendCommand(CmdEthReset, nil)
	return err
}

// FlashNode flashes the specified node with an OS image
func (c *AgentClient) FlashNode(node int, options *tpi.FlashOptions) error {
	args := map[string]any{
		"node":       node,
		"image_path": options.ImagePath,
	}

	if options.SHA256 != "" {
		args["sha256"] = options.SHA256
	}

	if options.SkipCRC {
		args["skip_crc"] = true
	}

	_, err := c.sendCommand(CmdFlashNode, args)
	return err
}

// FlashNodeLocal flashes a node with an image file accessible from the BMC
func (c *AgentClient) FlashNodeLocal(node int, imagePath string) error {
	args := map[string]any{
		"node":       node,
		"image_path": imagePath,
	}
	_, err := c.sendCommand(CmdFlashNodeLocal, args)
	return err
}

// UpgradeFirmware upgrades the BMC firmware with the given file
func (c *AgentClient) UpgradeFirmware(filePath string, providedSha256 string) error {
	args := map[string]any{
		"file_path": filePath,
	}

	if providedSha256 != "" {
		args["sha256"] = providedSha256
	}

	_, err := c.sendCommand(CmdUpgradeFirmware, args)
	return err
}

// UploadFile uploads a local file to the remote system through the agent
func (c *AgentClient) UploadFile(localPath, remotePath string) error {
	// Open the local file to get its contents
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file content: %w", err)
	}

	// Encode file content as base64 to send in the request
	args := map[string]any{
		"local_path":  localPath,
		"remote_path": remotePath,
		"content":     content,
	}

	_, err = c.sendCommand(CmdUploadFile, args)
	return err
}

// DownloadFile downloads a file from the remote system through the agent
func (c *AgentClient) DownloadFile(remotePath, localPath string) error {
	args := map[string]any{
		"remote_path": remotePath,
	}

	result, err := c.sendCommand(CmdDownloadFile, args)
	if err != nil {
		return err
	}

	// Extract file content from the response
	var fileContent []byte
	if content, ok := result.([]byte); ok {
		fileContent = content
	} else if contentMap, ok := result.(map[string]interface{}); ok {
		if content, ok := contentMap["content"].([]byte); ok {
			fileContent = content
		} else {
			return fmt.Errorf("invalid response format: content not found or not a byte array")
		}
	} else {
		return fmt.Errorf("invalid response format")
	}

	// Write the content to the local file
	if err := os.WriteFile(localPath, fileContent, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ListDirectory lists the contents of a remote directory through the agent
func (c *AgentClient) ListDirectory(remotePath string) ([]FileInfo, error) {
	args := map[string]any{
		"path": remotePath,
	}

	result, err := c.sendCommand(CmdListDirectory, args)
	if err != nil {
		return nil, err
	}

	// Convert the result to a slice of FileInfo
	var files []FileInfo
	if resultSlice, ok := result.([]interface{}); ok {
		for _, item := range resultSlice {
			if fileMap, ok := item.(map[string]interface{}); ok {
				file := FileInfo{}

				if name, ok := fileMap["name"].(string); ok {
					file.Name = name
				}

				if size, ok := fileMap["size"].(float64); ok {
					file.Size = int64(size)
				}

				if mode, ok := fileMap["mode"].(float64); ok {
					file.Mode = uint32(mode)
				}

				if isDir, ok := fileMap["is_dir"].(bool); ok {
					file.IsDir = isDir
				}

				// Parse time if present
				if modTimeStr, ok := fileMap["mod_time"].(string); ok {
					if modTime, err := time.Parse(time.RFC3339, modTimeStr); err == nil {
						file.ModTime = modTime
					}
				}

				files = append(files, file)
			}
		}
	}

	return files, nil
}

// ExecuteCommand executes a command on the remote system through the agent
func (c *AgentClient) ExecuteCommand(command string) (string, error) {
	args := map[string]any{
		"command": command,
	}

	result, err := c.sendCommand(CmdExecuteCommand, args)
	if err != nil {
		return "", err
	}

	if output, ok := result.(string); ok {
		return output, nil
	}

	return fmt.Sprintf("%v", result), nil
}

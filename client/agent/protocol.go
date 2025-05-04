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
	"time"
)

// Default port for the agent server
const DefaultAgentPort = 9977

// CommandType defines the type of command being sent
type CommandType string

const (
	// Basic commands
	CmdInfo          CommandType = "info"
	CmdAbout         CommandType = "about"
	CmdReboot        CommandType = "reboot"
	CmdRebootAndWait CommandType = "reboot_and_wait"

	// Power commands
	CmdPowerStatus CommandType = "power_status"
	CmdPowerOn     CommandType = "power_on"
	CmdPowerOff    CommandType = "power_off"
	CmdPowerReset  CommandType = "power_reset"
	CmdPowerOnAll  CommandType = "power_on_all"
	CmdPowerOffAll CommandType = "power_off_all"

	// Advanced mode commands
	CmdSetNodeNormalMode CommandType = "set_node_normal_mode"
	CmdSetNodeMsdMode    CommandType = "set_node_msd_mode"

	// USB commands
	CmdUsbGetStatus CommandType = "usb_get_status"
	CmdUsbSetHost   CommandType = "usb_set_host"
	CmdUsbSetDevice CommandType = "usb_set_device"
	CmdUsbSetFlash  CommandType = "usb_set_flash"

	// UART commands
	CmdGetUartOutput   CommandType = "get_uart_output"
	CmdSendUartCommand CommandType = "send_uart_command"

	// Ethernet commands
	CmdEthReset CommandType = "eth_reset"

	// Flash commands
	CmdFlashNode      CommandType = "flash_node"
	CmdFlashNodeLocal CommandType = "flash_node_local"

	// Firmware commands
	CmdUpgradeFirmware CommandType = "upgrade_firmware"
)

// Command represents a command sent from a client to the agent
type Command struct {
	Type CommandType     `json:"type"`
	Args map[string]any  `json:"args,omitempty"`
	Auth AgentAuthConfig `json:"auth,omitempty"`
}

// Response represents the response sent from the agent to the client
type Response struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// AgentConfig holds the configuration for the agent
type AgentConfig struct {
	Port           int             `json:"port"`
	AllowedClients []string        `json:"allowed_clients,omitempty"`
	Auth           AgentAuthConfig `json:"auth,omitempty"`
	TLSEnabled     bool            `json:"tls_enabled"`
	TLSCertFile    string          `json:"tls_cert_file,omitempty"`
	TLSKeyFile     string          `json:"tls_key_file,omitempty"`
}

// AgentAuthConfig holds authentication configuration
type AgentAuthConfig struct {
	Secret string        `json:"secret,omitempty"`
	Token  string        `json:"token,omitempty"`
	Expiry time.Duration `json:"expiry,omitempty"`
}

// AgentClientConfig holds the configuration for connecting to an agent
type AgentClientConfig struct {
	Host       string          `json:"host"`
	Port       int             `json:"port"`
	Auth       AgentAuthConfig `json:"auth,omitempty"`
	TLSEnabled bool            `json:"tls_enabled"`
	SkipVerify bool            `json:"skip_verify"`
	Timeout    time.Duration   `json:"timeout,omitempty"`
}

// FlashOptions contains options for flashing a node (used with CmdFlashNode)
type FlashOptions struct {
	ImagePath string `json:"image_path"`
	SHA256    string `json:"sha256,omitempty"`
	SkipCRC   bool   `json:"skip_crc,omitempty"`
}

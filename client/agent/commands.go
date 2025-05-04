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
	"fmt"
	"strconv"

	tpi "github.com/davidroman0O/tpi/client"
)

// executeCommand executes a command using the TPI client
func (a *Agent) executeCommand(cmd Command) (interface{}, error) {
	var result interface{}
	var err error

	// Execute the command based on its type
	switch cmd.Type {
	// Basic commands
	case CmdInfo:
		result, err = a.client.Info()
	case CmdAbout:
		result, err = a.client.About()
	case CmdReboot:
		err = a.client.Reboot()
	case CmdRebootAndWait:
		timeout, _ := getIntArg(cmd.Args, "timeout", 60)
		err = a.client.RebootAndWait(timeout)

	// Power commands
	case CmdPowerStatus:
		result, err = a.client.PowerStatus()
	case CmdPowerOn:
		node, _ := getIntArg(cmd.Args, "node", 0)
		err = validateNodeNumber(node)
		if err == nil {
			err = a.client.PowerOn(node)
		}
	case CmdPowerOff:
		node, _ := getIntArg(cmd.Args, "node", 0)
		err = validateNodeNumber(node)
		if err == nil {
			err = a.client.PowerOff(node)
		}
	case CmdPowerReset:
		node, _ := getIntArg(cmd.Args, "node", 0)
		err = validateNodeNumber(node)
		if err == nil {
			err = a.client.PowerReset(node)
		}
	case CmdPowerOnAll:
		err = a.client.PowerOnAll()
	case CmdPowerOffAll:
		err = a.client.PowerOffAll()

	// Advanced mode commands
	case CmdSetNodeNormalMode:
		node, _ := getIntArg(cmd.Args, "node", 0)
		err = validateNodeNumber(node)
		if err == nil {
			err = a.client.SetNodeNormalMode(node)
		}
	case CmdSetNodeMsdMode:
		node, _ := getIntArg(cmd.Args, "node", 0)
		err = validateNodeNumber(node)
		if err == nil {
			err = a.client.SetNodeMsdMode(node)
		}

	// USB commands
	case CmdUsbGetStatus:
		result, err = a.client.UsbGetStatus()
	case CmdUsbSetHost:
		node, _ := getIntArg(cmd.Args, "node", 0)
		bmc, _ := getBoolArg(cmd.Args, "bmc", false)
		err = validateNodeNumber(node)
		if err == nil {
			err = a.client.UsbSetHost(node, bmc)
		}
	case CmdUsbSetDevice:
		node, _ := getIntArg(cmd.Args, "node", 0)
		bmc, _ := getBoolArg(cmd.Args, "bmc", false)
		err = validateNodeNumber(node)
		if err == nil {
			err = a.client.UsbSetDevice(node, bmc)
		}
	case CmdUsbSetFlash:
		node, _ := getIntArg(cmd.Args, "node", 0)
		bmc, _ := getBoolArg(cmd.Args, "bmc", false)
		err = validateNodeNumber(node)
		if err == nil {
			err = a.client.UsbSetFlash(node, bmc)
		}

	// UART commands
	case CmdGetUartOutput:
		node, _ := getIntArg(cmd.Args, "node", 0)
		err = validateNodeNumber(node)
		if err == nil {
			result, err = a.client.GetUartOutput(node)
		}
	case CmdSendUartCommand:
		node, _ := getIntArg(cmd.Args, "node", 0)
		command, _ := getStringArg(cmd.Args, "command", "")
		err = validateNodeNumber(node)
		if err == nil && command != "" {
			err = a.client.SendUartCommand(node, command)
		} else if command == "" {
			err = fmt.Errorf("command is required for SendUartCommand")
		}

	// Ethernet commands
	case CmdEthReset:
		err = a.client.EthReset()

	// Flash commands
	case CmdFlashNode:
		node, _ := getIntArg(cmd.Args, "node", 0)
		err = validateNodeNumber(node)
		if err == nil {
			imagePath, _ := getStringArg(cmd.Args, "image_path", "")
			if imagePath == "" {
				err = fmt.Errorf("image_path is required for FlashNode")
				break
			}

			// Create flash options
			options := &tpi.FlashOptions{
				ImagePath: imagePath,
			}

			// Get optional parameters
			options.SHA256, _ = getStringArg(cmd.Args, "sha256", "")
			options.SkipCRC, _ = getBoolArg(cmd.Args, "skip_crc", false)

			err = a.client.FlashNode(node, options)
		}
	case CmdFlashNodeLocal:
		node, _ := getIntArg(cmd.Args, "node", 0)
		imagePath, _ := getStringArg(cmd.Args, "image_path", "")
		err = validateNodeNumber(node)
		if err == nil {
			if imagePath == "" {
				err = fmt.Errorf("image_path is required for FlashNodeLocal")
				break
			}
			err = a.client.FlashNodeLocal(node, imagePath)
		}

	// Firmware commands
	case CmdUpgradeFirmware:
		filePath, _ := getStringArg(cmd.Args, "file_path", "")
		sha256, _ := getStringArg(cmd.Args, "sha256", "")
		if filePath == "" {
			err = fmt.Errorf("file_path is required for UpgradeFirmware")
			break
		}
		err = a.client.UpgradeFirmware(filePath, sha256)

	default:
		err = fmt.Errorf("unknown command: %s", cmd.Type)
	}

	return result, err
}

// Helper functions for argument extraction

func getIntArg(args map[string]any, key string, defaultValue int) (int, bool) {
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case float64:
			return int(v), true
		case int:
			return v, true
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i, true
			}
		}
	}
	return defaultValue, false
}

func getStringArg(args map[string]any, key string, defaultValue string) (string, bool) {
	if val, ok := args[key]; ok {
		if str, ok := val.(string); ok {
			return str, true
		}
		// Try to convert to string
		return fmt.Sprintf("%v", val), true
	}
	return defaultValue, false
}

func getBoolArg(args map[string]any, key string, defaultValue bool) (bool, bool) {
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case bool:
			return v, true
		case string:
			if v == "true" {
				return true, true
			} else if v == "false" {
				return false, true
			}
		case float64:
			return v != 0, true
		case int:
			return v != 0, true
		}
	}
	return defaultValue, false
}

// validateNodeNumber validates that the node number is between 1 and 4
func validateNodeNumber(node int) error {
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d (must be between 1 and 4)", node)
	}
	return nil
}

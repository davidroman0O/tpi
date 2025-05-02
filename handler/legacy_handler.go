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

package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/davidroman0O/tpi/cli"
	"github.com/davidroman0O/tpi/request"
)

const (
	multipartBufferSize = 1024 * 32
)

// ResponsePrinter is a function that prints the response
type ResponsePrinter func(map[string]interface{}) error

// LegacyHandler handles the execution of commands
type LegacyHandler struct {
	request         *request.Request
	options         *cli.Options
	responsePrinter ResponsePrinter
	skipRequest     bool
}

// NewLegacyHandler creates a new legacy handler
func NewLegacyHandler(options *cli.Options) (*LegacyHandler, error) {
	// Create the request
	req, err := request.NewRequest(options.Host, options.ApiVersion, options.User, options.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return &LegacyHandler{
		request: req,
		options: options,
	}, nil
}

// Execute executes the command in the options
func Execute(options *cli.Options) error {
	handler, err := NewLegacyHandler(options)
	if err != nil {
		return err
	}

	return handler.HandleCommand()
}

// HandleCommand handles the command in the options
func (h *LegacyHandler) HandleCommand() error {
	// Make sure credentials are set if provided
	if h.options.User != "" {
		h.request.Credentials.Username = h.options.User
	}
	if h.options.Password != "" {
		h.request.Credentials.Password = h.options.Password
	}

	switch h.options.Command {
	case "power":
		if err := h.handlePower(); err != nil {
			return err
		}
	case "usb":
		if err := h.handleUsb(); err != nil {
			return err
		}
	case "firmware":
		if err := h.handleFirmware(); err != nil {
			return err
		}
	case "flash":
		if err := h.handleFlash(); err != nil {
			return err
		}
	case "eth":
		if err := h.handleEth(); err != nil {
			return err
		}
	case "uart":
		if err := h.handleUart(); err != nil {
			return err
		}
	case "advanced":
		if err := h.handleAdvanced(); err != nil {
			return err
		}
	case "cooling":
		if err := h.handleCooling(); err != nil {
			return err
		}
	case "info":
		h.handleInfo()
	case "reboot":
		h.handleReboot()
	case "auth.login":
		return h.handleAuthLogin()
	case "auth.logout":
		return h.handleAuthLogout()
	case "auth.status":
		return h.handleAuthStatus()
	default:
		return fmt.Errorf("unknown command: %s", h.options.Command)
	}

	if h.skipRequest {
		return nil
	}

	// Send the request
	resp, err := h.request.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, string(body))
	}

	// Parse the response
	var respData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// If JSON output is requested, print the raw response
	if h.options.JSON {
		jsonData, err := json.MarshalIndent(respData, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON response: %w", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	// Otherwise, process the response with a response printer
	response, ok := respData["response"]
	if !ok {
		return fmt.Errorf("invalid response: missing 'response' key")
	}

	responseArray, ok := response.([]interface{})
	if !ok || len(responseArray) == 0 {
		return fmt.Errorf("invalid response: 'response' is not an array or is empty")
	}

	extracted := responseArray[0]
	extractedMap, ok := extracted.(map[string]interface{})

	if h.responsePrinter != nil {
		if ok {
			if err := h.responsePrinter(extractedMap); err != nil {
				fmt.Println(extracted)
				fmt.Println(err)
			}
		} else {
			fmt.Println(extracted)
		}
	} else {
		fmt.Println(extracted)
	}

	return nil
}

// handlePower handles the power command
func (h *LegacyHandler) handlePower() error {
	cmdInterface, ok := h.options.CommandArgs["cmd"]
	if !ok {
		return fmt.Errorf("missing power command")
	}

	// Handle the case where cmd might be a string instead of PowerCmd
	var cmd cli.PowerCmd
	switch v := cmdInterface.(type) {
	case cli.PowerCmd:
		cmd = v
	case string:
		cmd = cli.PowerCmd(v)
	default:
		return fmt.Errorf("invalid power command type: %T", cmdInterface)
	}

	// Status command is a special case - no node needed
	if string(cmd) == "status" {
		h.request.AddQueryParam("opt", "get")
		h.request.AddQueryParam("type", "power")
		h.responsePrinter = printPowerStatus
		fmt.Printf("DEBUG: Power status request URL: %s\n", h.request.GetURL())
		return nil
	}

	// For all other commands, we need a node number
	nodeInterface, ok := h.options.CommandArgs["node"]
	if !ok || nodeInterface.(int) == 0 {
		fmt.Println("No node specified, defaulting to all nodes")
		// For non-status commands, use all nodes if not specified
		allNodes := []int{1, 2, 3, 4}
		// Create a fresh request each time to avoid parameter duplication
		for _, node := range allNodes {
			// Create a new request for each node to avoid parameter accumulation
			nodeReq, err := request.NewRequest(h.options.Host, h.options.ApiVersion, h.options.User, h.options.Password)
			if err != nil {
				return fmt.Errorf("failed to create request for node %d: %w", node, err)
			}

			// Configure the request for this specific node
			if err := configureNodePowerRequest(nodeReq, cmd, node); err != nil {
				return err
			}

			// Send the request for this node
			fmt.Printf("Sending power %s command for node %d\n", cmd, node)
			resp, err := nodeReq.Send()
			if err != nil {
				return fmt.Errorf("failed to send power command for node %d: %w", node, err)
			}
			defer resp.Body.Close()

			// Check response status
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("failed power command for node %d: %s: %s", node, resp.Status, string(body))
			}

			// Parse and print response
			var respData map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
				return fmt.Errorf("failed to parse response for node %d: %w", node, err)
			}

			// Print result
			fmt.Printf("Node %d: ", node)
			if h.options.JSON {
				jsonData, err := json.MarshalIndent(respData, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON response: %w", err)
				}
				fmt.Println(string(jsonData))
			} else {
				response, ok := respData["response"]
				if ok {
					responseArray, ok := response.([]interface{})
					if ok && len(responseArray) > 0 {
						fmt.Println(responseArray[0])
					} else {
						fmt.Println(response)
					}
				} else {
					fmt.Println(respData)
				}
			}
		}

		// Skip the regular request sending since we handled it manually for each node
		h.skipRequest = true
		return nil
	}

	// Handle single node
	node, ok := nodeInterface.(int)
	if !ok {
		return fmt.Errorf("invalid node type: %T", nodeInterface)
	}

	// Configure the request for the specific node
	if err := configureNodePowerRequest(h.request, cmd, node); err != nil {
		return err
	}

	h.responsePrinter = printResult
	fmt.Printf("DEBUG: Power %s node %d URL: %s\n", cmd, node, h.request.GetURL())
	return nil
}

// configureNodePowerRequest configures a request for a power command on a specific node
func configureNodePowerRequest(req *request.Request, cmd cli.PowerCmd, node int) error {
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d", node)
	}

	// If reset, handle differently
	if cmd == cli.ResetCmd {
		req.AddQueryParam("opt", "set")
		req.AddQueryParam("type", "reset")
		req.AddQueryParam("node", strconv.Itoa(node-1))
		return nil
	}

	// Handle on/off - set power only for the specific node
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "power")

	// Use API format for node parameters
	nodeStr := fmt.Sprintf("node%d", node)

	if cmd == cli.OnCmd {
		req.AddQueryParam(nodeStr, "1")
	} else {
		req.AddQueryParam(nodeStr, "0")
	}

	return nil
}

// handleUsb handles the USB command
func (h *LegacyHandler) handleUsb() error {
	modeInterface, ok := h.options.CommandArgs["mode"]
	if !ok {
		return fmt.Errorf("missing USB mode")
	}
	mode := modeInterface.(cli.UsbCmd)

	// If status, get USB status
	if mode == cli.StatusUsbCmd {
		h.request.AddQueryParam("opt", "get")
		h.request.AddQueryParam("type", "usb")
		h.responsePrinter = printUsbStatus
		return nil
	}

	// Otherwise, set USB mode
	nodeInterface, ok := h.options.CommandArgs["node"]
	if !ok {
		return fmt.Errorf("missing node number for USB command")
	}
	node := nodeInterface.(int)
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d", node)
	}

	h.request.AddQueryParam("opt", "set")
	h.request.AddQueryParam("type", "usb")
	h.request.AddQueryParam("node", strconv.Itoa(node-1))

	// Convert mode to numeric value
	var modeVal uint8
	switch mode {
	case cli.HostCmd:
		modeVal = 0
	case cli.DeviceCmd:
		modeVal = 1
	case cli.FlashCmd:
		modeVal = 2
	default:
		return fmt.Errorf("invalid USB mode: %s", mode)
	}

	// Add BMC bit if needed
	bmcInterface, ok := h.options.CommandArgs["bmc"]
	if ok && bmcInterface.(bool) {
		modeVal |= 1 << 2
	}

	h.request.AddQueryParam("mode", strconv.Itoa(int(modeVal)))
	h.responsePrinter = printResult

	return nil
}

// handleFirmware handles the firmware command
func (h *LegacyHandler) handleFirmware() error {
	fileInterface, ok := h.options.CommandArgs["file"]
	if !ok {
		return fmt.Errorf("missing firmware file")
	}
	filePath := fileInterface.(string)

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open firmware file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get firmware file info: %w", err)
	}
	fileSize := fileInfo.Size()
	fileName := filepath.Base(filePath)

	// Create base request
	h.request.AddQueryParam("opt", "set")
	h.request.AddQueryParam("type", "firmware")
	h.request.AddQueryParam("file", fileName)

	// Handle based on API version
	if h.options.ApiVersion == cli.ApiVersionV1 {
		h.skipRequest = true
		h.request.AddQueryParam("file", fileName)
		return h.handleFileUploadV1(file)
	} else {
		h.skipRequest = true
		h.request.AddQueryParam("length", strconv.FormatInt(fileSize, 10))

		// Add SHA256 if provided
		sha256Interface, ok := h.options.CommandArgs["sha256"]
		if ok {
			h.request.AddQueryParam("sha256", sha256Interface.(string))
		}

		return h.handleFileUploadV1_1(file, fileSize)
	}
}

// handleFlash handles the flash command
func (h *LegacyHandler) handleFlash() error {
	// Check if local flag is set
	localInterface, ok := h.options.CommandArgs["local"]
	if ok && localInterface.(bool) {
		return h.handleLocalFileUpload()
	}

	// Get image path
	imagePathInterface, ok := h.options.CommandArgs["image_path"]
	if !ok {
		return fmt.Errorf("missing image path")
	}
	imagePath := imagePathInterface.(string)

	// Open the file
	file, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get image file info: %w", err)
	}
	fileSize := fileInfo.Size()
	fileName := filepath.Base(imagePath)

	// Get node
	nodeInterface, ok := h.options.CommandArgs["node"]
	if !ok {
		return fmt.Errorf("missing node number")
	}
	node := nodeInterface.(int)
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d", node)
	}

	fmt.Printf("request flashing of %s to node %d\n", fileName, node)

	// Create base request
	h.request.AddQueryParam("opt", "set")
	h.request.AddQueryParam("type", "flash")
	h.request.AddQueryParam("file", fileName)
	h.request.AddQueryParam("length", strconv.FormatInt(fileSize, 10))
	h.request.AddQueryParam("node", strconv.Itoa(node-1))

	// Add SHA256 if provided
	sha256Interface, ok := h.options.CommandArgs["sha256"]
	if ok {
		h.request.AddQueryParam("sha256", sha256Interface.(string))
	}

	// Add skip CRC if provided
	skipCrcInterface, ok := h.options.CommandArgs["skip_crc"]
	if ok && skipCrcInterface.(bool) {
		h.request.AddQueryParam("skip_crc", "")
	}

	// Handle based on API version
	h.skipRequest = true
	if h.options.ApiVersion == cli.ApiVersionV1 {
		return h.handleFileUploadV1(file)
	} else {
		return h.handleFileUploadV1_1(file, fileSize)
	}
}

// handleLocalFileUpload handles local file uploads
func (h *LegacyHandler) handleLocalFileUpload() error {
	// Get image path
	imagePathInterface, ok := h.options.CommandArgs["image_path"]
	if !ok {
		return fmt.Errorf("missing image path")
	}
	imagePath := imagePathInterface.(string)

	// Get node
	nodeInterface, ok := h.options.CommandArgs["node"]
	if !ok {
		return fmt.Errorf("missing node number")
	}
	node := nodeInterface.(int)
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d", node)
	}

	// Create request
	h.request.AddQueryParam("opt", "set")
	h.request.AddQueryParam("type", "flash")
	h.request.AddQueryParam("local", "")
	h.request.AddQueryParam("file", imagePath)
	h.request.AddQueryParam("node", strconv.Itoa(node-1))

	// Send the request
	resp, err := h.request.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to begin flashing: %s", resp.Status)
	}

	// Parse the response
	var respData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Get the handle ID
	handleID, err := getJsonNum(respData, "handle")
	if err != nil {
		return fmt.Errorf("failed to get handle ID: %w", err)
	}

	fmt.Printf("Flashing from image file %s...\n", imagePath)

	// Watch the progress
	return h.watchFlashingProgress(int(handleID))
}

// handleFileUploadV1 handles file uploads for API version 1
func (h *LegacyHandler) handleFileUploadV1(file *os.File) error {
	fmt.Println("Warning: large files will very likely to fail to be uploaded in version 1")

	// Create multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Add file field
	fileName := filepath.Base(file.Name())
	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	// Copy file contents to form
	if _, err := io.Copy(fw, file); err != nil {
		return fmt.Errorf("failed to copy file to form: %w", err)
	}

	// Close the multipart writer
	w.Close()

	// Create post request
	postReq := h.request.ToPost()
	postReq.SetMultipartForm(&b, w.FormDataContentType())

	// Send the request
	resp, err := postReq.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// handleFileUploadV1_1 handles file uploads for API version 1.1
func (h *LegacyHandler) handleFileUploadV1_1(file *os.File, fileSize int64) error {
	// Send the initial request
	resp, err := h.request.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to initiate upload: %s: %s", resp.Status, string(body))
	}

	// Parse the response to get the handle
	var respData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	handle, ok := respData["handle"].(float64)
	if !ok {
		return fmt.Errorf("invalid response: missing handle")
	}

	fmt.Printf("Started transfer of %d bytes...\n", fileSize)

	// Create multipart form for file upload
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Add file field
	fileName := filepath.Base(file.Name())
	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	// Copy file contents to form with progress
	if _, err := io.Copy(fw, file); err != nil {
		return fmt.Errorf("failed to copy file to form: %w", err)
	}

	// Close the multipart writer
	w.Close()

	// Create upload URL
	uploadURL := fmt.Sprintf("%s://%s/api/bmc/upload/%d",
		h.options.ApiVersion.GetScheme(),
		h.options.Host,
		int(handle))

	// Create upload request
	uploadReq, err := request.NewRequest(h.options.Host, h.options.ApiVersion, h.options.User, h.options.Password)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	uploadReq = uploadReq.ToPost()

	// Parse the URL string into a url.URL object
	parsedURL, err := url.Parse(uploadURL)
	if err != nil {
		return fmt.Errorf("failed to parse upload URL: %w", err)
	}
	uploadReq.URL = parsedURL

	uploadReq.SetMultipartForm(&b, w.FormDataContentType())

	// Send the upload request
	uploadResp, err := uploadReq.Send()
	if err != nil {
		return fmt.Errorf("failed to send upload request: %w", err)
	}
	uploadResp.Body.Close()

	// Watch the progress
	return h.watchFlashingProgress(int(handle))
}

// watchFlashingProgress watches the flashing progress
func (h *LegacyHandler) watchFlashingProgress(handle int) error {
	// Initial delay
	time.Sleep(3 * time.Second)

	// Create progress request
	progressReq, err := request.NewRequest(h.options.Host, h.options.ApiVersion, h.options.User, h.options.Password)
	if err != nil {
		return fmt.Errorf("failed to create progress request: %w", err)
	}
	progressReq.AddQueryParam("opt", "get")
	progressReq.AddQueryParam("type", "flash")

	var fileSize int64
	verifying := false

	for {
		// Send progress request
		resp, err := progressReq.Send()
		if err != nil {
			return fmt.Errorf("failed to send progress request: %w", err)
		}

		// Parse response
		var respData map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to parse progress response: %w", err)
		}
		resp.Body.Close()

		// Check for transferring status
		if transferring, ok := respData["Transferring"].(map[string]interface{}); ok {
			id, err := getJsonNum(transferring, "id")
			if err != nil {
				return fmt.Errorf("failed to get transfer ID: %w", err)
			}

			if id != int64(handle) {
				return fmt.Errorf("invalid transfer handle: expected %d, got %d", handle, id)
			}

			size, err := getJsonNum(transferring, "size")
			if err != nil {
				return fmt.Errorf("failed to get file size: %w", err)
			}

			if fileSize == 0 {
				fileSize = size
				fmt.Printf("Total size: %d bytes\n", fileSize)
			}

			bytesWritten, err := getJsonNum(transferring, "bytes_written")
			if err != nil {
				return fmt.Errorf("failed to get bytes written: %w", err)
			}

			if bytesWritten >= fileSize {
				if !verifying {
					fmt.Println("Verifying checksum...")
					verifying = true
				}
			} else {
				progress := float64(bytesWritten) / float64(fileSize) * 100
				fmt.Printf("\rProgress: %.1f%% (%d/%d bytes)", progress, bytesWritten, fileSize)
			}

			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Check for completion
		if _, ok := respData["Done"]; ok {
			fmt.Println("\nDone")
			break
		}

		// Check for error
		if errMap, ok := respData["Error"].(map[string]interface{}); ok {
			return fmt.Errorf("error occurred during flashing: %v", errMap)
		}

		// Unexpected response
		return fmt.Errorf("unexpected response: %v", respData)
	}

	return nil
}

// handleEth handles the eth command
func (h *LegacyHandler) handleEth() error {
	cmdInterface, ok := h.options.CommandArgs["cmd"]
	if !ok {
		return fmt.Errorf("missing eth command")
	}
	cmd := cmdInterface.(cli.EthCmd)

	// Currently only reset is supported
	if cmd == cli.ResetEthCmd {
		h.request.AddQueryParam("opt", "set")
		h.request.AddQueryParam("type", "network")
		h.request.AddQueryParam("cmd", "reset")
		h.responsePrinter = printResult
	} else {
		return fmt.Errorf("unsupported eth command: %s", cmd)
	}

	return nil
}

// handleUart handles the uart command
func (h *LegacyHandler) handleUart() error {
	actionInterface, ok := h.options.CommandArgs["action"]
	if !ok {
		return fmt.Errorf("missing uart action")
	}
	action := actionInterface.(cli.GetSet)

	nodeInterface, ok := h.options.CommandArgs["node"]
	if !ok {
		return fmt.Errorf("missing node number")
	}
	node := nodeInterface.(int)
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d", node)
	}

	if action == cli.GetOperation {
		h.request.AddQueryParam("opt", "get")
		h.request.AddQueryParam("type", "uart")
		h.request.AddQueryParam("node", strconv.Itoa(node-1))
		h.responsePrinter = printUartOutput
	} else if action == cli.SetOperation {
		cmdInterface, ok := h.options.CommandArgs["cmd"]
		if !ok {
			return fmt.Errorf("missing uart command")
		}
		cmd := cmdInterface.(string)

		h.request.AddQueryParam("opt", "set")
		h.request.AddQueryParam("type", "uart")
		h.request.AddQueryParam("node", strconv.Itoa(node-1))
		h.request.AddQueryParam("cmd", cmd)
		h.responsePrinter = printResult
	} else {
		return fmt.Errorf("unsupported uart action: %s", action)
	}

	return nil
}

// handleAdvanced handles the advanced command
func (h *LegacyHandler) handleAdvanced() error {
	modeInterface, ok := h.options.CommandArgs["mode"]
	if !ok {
		return fmt.Errorf("missing advanced mode")
	}
	mode := modeInterface.(cli.ModeCmd)

	nodeInterface, ok := h.options.CommandArgs["node"]
	if !ok {
		return fmt.Errorf("missing node number")
	}
	node := nodeInterface.(int)
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d", node)
	}

	if mode == cli.NormalMode {
		// Clear USB boot
		h.request.AddQueryParam("opt", "set")
		h.request.AddQueryParam("type", "clear_usb_boot")
		h.request.AddQueryParam("node", strconv.Itoa(node-1))

		// Send the request
		resp, err := h.request.Send()
		if err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to execute Normal mode: %s", resp.Status)
		}

		// Reset the node
		h.request = nil
		powerOptions := &cli.Options{
			Host:        h.options.Host,
			Port:        h.options.Port,
			User:        h.options.User,
			Password:    h.options.Password,
			JSON:        h.options.JSON,
			ApiVersion:  h.options.ApiVersion,
			Command:     "power",
			CommandArgs: map[string]interface{}{"cmd": cli.ResetCmd, "node": node},
		}
		return Execute(powerOptions)
	} else if mode == cli.MsdMode {
		h.request.AddQueryParam("opt", "set")
		h.request.AddQueryParam("type", "node_to_msd")
		h.request.AddQueryParam("node", strconv.Itoa(node-1))
		h.responsePrinter = printResult
	} else {
		return fmt.Errorf("unsupported advanced mode: %s", mode)
	}

	return nil
}

// handleCooling handles the cooling command
func (h *LegacyHandler) handleCooling() error {
	cmdInterface, ok := h.options.CommandArgs["cmd"]
	if !ok {
		return fmt.Errorf("missing cooling command")
	}
	cmd := cmdInterface.(cli.CoolingCmd)

	if cmd == cli.StatusCoolingCmd {
		h.request.AddQueryParam("opt", "get")
		h.request.AddQueryParam("type", "cooling")
		h.responsePrinter = printCoolingStatus
	} else if cmd == cli.SetCoolingCmd {
		deviceInterface, ok := h.options.CommandArgs["device"]
		if !ok {
			return fmt.Errorf("missing cooling device")
		}
		device := deviceInterface.(string)

		speedInterface, ok := h.options.CommandArgs["speed"]
		if !ok {
			return fmt.Errorf("missing cooling speed")
		}
		speed := speedInterface.(uint)

		h.request.AddQueryParam("opt", "set")
		h.request.AddQueryParam("type", "cooling")
		h.request.AddQueryParam("device", device)
		h.request.AddQueryParam("speed", strconv.FormatUint(uint64(speed), 10))
		h.responsePrinter = printCoolingStatus
	} else {
		return fmt.Errorf("unsupported cooling command: %s", cmd)
	}

	return nil
}

// handleInfo handles the info command
func (h *LegacyHandler) handleInfo() {
	h.request.AddQueryParam("opt", "get")
	h.request.AddQueryParam("type", "other")
	h.responsePrinter = printInfo
}

// handleReboot handles the reboot command
func (h *LegacyHandler) handleReboot() {
	h.request.AddQueryParam("opt", "set")
	h.request.AddQueryParam("type", "reboot")
	h.responsePrinter = printResult
}

// handleAuthLogin handles the auth login command
func (h *LegacyHandler) handleAuthLogin() error {
	// Force authentication and cache the token
	_, err := h.request.ForceAuthentication()
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	fmt.Printf("Successfully authenticated to %s\n", h.options.Host)
	return nil
}

// handleAuthLogout handles the auth logout command
func (h *LegacyHandler) handleAuthLogout() error {
	if h.options.Host != "" {
		// Clear token for specific host
		if err := request.DeleteCachedTokenForHost(h.options.Host); err != nil {
			return fmt.Errorf("failed to clear token for host %s: %w", h.options.Host, err)
		}
	} else {
		// Clear all tokens
		if err := request.DeleteAllCachedTokens(); err != nil {
			return fmt.Errorf("failed to clear all tokens: %w", err)
		}
	}

	return nil
}

// handleAuthStatus handles the auth status command
func (h *LegacyHandler) handleAuthStatus() error {
	if h.options.Host != "" {
		// Check if token exists for specific host
		_, err := request.GetCachedTokenForHost(h.options.Host)
		if err != nil {
			fmt.Printf("Not authenticated to %s (no cached token)\n", h.options.Host)
		} else {
			fmt.Printf("Authenticated to %s (token is cached)\n", h.options.Host)
		}
	} else {
		// List all hosts with cached tokens
		hosts, err := request.GetAllCachedTokenHosts()
		if err != nil || len(hosts) == 0 {
			fmt.Println("No cached authentication tokens found")
		} else {
			fmt.Println("Cached authentication tokens found for:")
			for _, host := range hosts {
				if host == "default" {
					fmt.Println("- default (legacy token)")
				} else {
					fmt.Printf("- %s\n", host)
				}
			}
		}
	}

	return nil
}

// printPowerStatus prints the power status of the nodes
func printPowerStatus(result map[string]interface{}) error {
	fmt.Printf("DEBUG: Power status result: %v\n", result)

	// Check if we have a result value
	resultVal, ok := result["result"]
	if !ok {
		return fmt.Errorf("invalid power status response: missing result")
	}

	// Parse the node power status
	nodes := map[string]bool{}

	// Handle different result formats
	switch rv := resultVal.(type) {
	case []interface{}:
		// Format: result: [map[node1:1 node2:1 node3:1 node4:1]]
		if len(rv) > 0 {
			if nodeMap, ok := rv[0].(map[string]interface{}); ok {
				for node, val := range nodeMap {
					if strings.HasPrefix(node, "node") {
						// Convert value to bool: 1 = on, 0 = off
						powerOn := false
						switch v := val.(type) {
						case float64:
							powerOn = v > 0
						case int:
							powerOn = v > 0
						case string:
							powerOn = v == "1" || strings.ToLower(v) == "on"
						}
						nodes[node] = powerOn
					}
				}
			}
		}
	case map[string]interface{}:
		// Alternative format: result: map[node1:1 node2:1 node3:1 node4:1]
		for node, val := range rv {
			if strings.HasPrefix(node, "node") {
				// Convert value to bool: 1 = on, 0 = off
				powerOn := false
				switch v := val.(type) {
				case float64:
					powerOn = v > 0
				case int:
					powerOn = v > 0
				case string:
					powerOn = v == "1" || strings.ToLower(v) == "on"
				}
				nodes[node] = powerOn
			}
		}
	default:
		return fmt.Errorf("invalid power status format: %T", resultVal)
	}

	// Sort nodes by number
	nodeKeys := make([]string, 0, len(nodes))
	for k := range nodes {
		nodeKeys = append(nodeKeys, k)
	}
	sort.Strings(nodeKeys)

	// Print node status in a nice table
	fmt.Println("Power Status:")
	fmt.Println("-----------------")
	for _, node := range nodeKeys {
		status := "OFF"
		if nodes[node] {
			status = "ON"
		}

		// Extract node number (node1 -> 1)
		nodeNum := strings.TrimPrefix(node, "node")
		fmt.Printf("Node %s: %s\n", nodeNum, status)
	}

	return nil
}

// printResult prints the result
func printResult(result map[string]interface{}) error {
	resultStr, err := getJsonStr(result, "result")
	if err != nil {
		return err
	}

	fmt.Println(resultStr)
	return nil
}

// printInfo prints the info
func printInfo(result map[string]interface{}) error {
	fmt.Println("|---------------|----------------------------|")
	fmt.Println("|      key      |           value            |")
	fmt.Println("|---------------|----------------------------|")

	for key, value := range result {
		strValue, ok := value.(string)
		if !ok {
			return fmt.Errorf("invalid info value for %s", key)
		}

		fmt.Printf(" %-15s: %s\n", key, strValue)
	}

	fmt.Println("|---------------|----------------------------|")
	return nil
}

// printUsbStatus prints the USB status
func printUsbStatus(result map[string]interface{}) error {
	fmt.Printf("DEBUG: USB status result: %v\n", result)

	// Check if we have a result array
	resultVal, ok := result["result"]
	if !ok {
		return fmt.Errorf("invalid USB status response: missing result")
	}

	// The result is an array of status entries
	resultArray, ok := resultVal.([]interface{})
	if !ok {
		return fmt.Errorf("invalid USB status format: %T", resultVal)
	}

	if len(resultArray) == 0 {
		fmt.Println("No USB status information available")
		return nil
	}

	// Extract the USB status from the first entry in the array
	statusMap, ok := resultArray[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid USB status entry format")
	}

	// Extract node, mode, and route
	nodeVal, ok := statusMap["node"]
	if !ok {
		return fmt.Errorf("missing node in USB status")
	}
	node := fmt.Sprintf("%v", nodeVal)

	modeVal, ok := statusMap["mode"]
	if !ok {
		return fmt.Errorf("missing mode in USB status")
	}
	mode := fmt.Sprintf("%v", modeVal)

	routeVal, ok := statusMap["route"]
	if !ok {
		return fmt.Errorf("missing route in USB status")
	}
	route := fmt.Sprintf("%v", routeVal)

	fmt.Println("    USB Host    -->    USB Device    ")
	fmt.Println("---------------    ---------------")

	var host, device string
	mode = strings.ToLower(mode)
	if mode == "host" {
		host = node
		device = route
	} else {
		host = route
		device = node
	}

	fmt.Printf("    %-12s -->    %-12s\n", host, device)

	return nil
}

// printUartOutput prints the UART output
func printUartOutput(result map[string]interface{}) error {
	uart, err := getJsonStr(result, "uart")
	if err != nil {
		return err
	}

	fmt.Print(uart)
	return nil
}

// printCoolingStatus prints the cooling status
func printCoolingStatus(result map[string]interface{}) error {
	// Check if we have a simple result
	if resultStr, ok := result["result"].(string); ok {
		fmt.Println(resultStr)
		return nil
	}

	// Otherwise, we have a cooling device list
	resultArr, ok := result["result"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid cooling status result")
	}

	if len(resultArr) == 0 {
		fmt.Println("No cooling devices found")
		return nil
	}

	fmt.Println("|---------------|-------|-----------|")
	fmt.Println("|     Device    | Speed | Max Speed |")
	fmt.Println("|---------------|-------|-----------|")

	for _, device := range resultArr {
		deviceMap, ok := device.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid cooling device data")
		}

		name, err := getJsonStr(deviceMap, "device")
		if err != nil {
			return err
		}

		speed, err := getJsonNum(deviceMap, "speed")
		if err != nil {
			return err
		}

		maxSpeed, err := getJsonNum(deviceMap, "max_speed")
		if err != nil {
			return err
		}

		fmt.Printf("|%-15s|%7d|%11d|\n", name, speed, maxSpeed)
	}

	fmt.Println("|---------------|-------|-----------|")
	return nil
}

// getJsonStr gets a string value from a JSON map
func getJsonStr(m map[string]interface{}, key string) (string, error) {
	value, ok := m[key]
	if !ok {
		return "", fmt.Errorf("missing key: %s", key)
	}

	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("value for %s is not a string", key)
	}

	return strValue, nil
}

// getJsonNum gets a number value from a JSON map
func getJsonNum(m map[string]interface{}, key string) (int64, error) {
	value, ok := m[key]
	if !ok {
		return 0, fmt.Errorf("missing key: %s", key)
	}

	// Convert from float64 (JSON number) to int64
	floatValue, ok := value.(float64)
	if !ok {
		return 0, fmt.Errorf("value for %s is not a number", key)
	}

	return int64(floatValue), nil
}

// calculateSHA256 calculates the SHA256 hash of a file
func calculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

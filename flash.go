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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
)

// FlashOptions contains options for flashing a node
type FlashOptions struct {
	// Path to the image file
	ImagePath string
	// Optional SHA256 checksum for verification
	SHA256 string
	// Skip CRC check
	SkipCRC bool
}

// FlashNode flashes the specified node with an OS image
func (c *Client) FlashNode(node int, options *FlashOptions) error {
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d (must be 1-4)", node)
	}

	if options == nil || options.ImagePath == "" {
		return fmt.Errorf("image path is required")
	}

	// Verify file exists
	file, err := os.Open(options.ImagePath)
	if err != nil {
		return fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close()

	// If SHA256 is provided, verify the file
	if options.SHA256 != "" {
		// Calculate SHA256
		h := sha256.New()
		if _, err := io.Copy(h, file); err != nil {
			return fmt.Errorf("failed to calculate SHA256: %w", err)
		}
		calculatedSha256 := hex.EncodeToString(h.Sum(nil))

		// Verify checksum
		if calculatedSha256 != options.SHA256 {
			return fmt.Errorf("SHA256 checksum mismatch: provided %s, calculated %s",
				options.SHA256, calculatedSha256)
		}

		// Reset file position for upload
		if _, err := file.Seek(0, 0); err != nil {
			return fmt.Errorf("failed to reset file: %w", err)
		}
	}

	// Create a new request
	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Modify the URL to point to the update endpoint
	req.URL.Path = "/api/update"

	// Add query parameters
	req.AddQueryParam("node", strconv.Itoa(node-1)) // BMC uses 0-based indexing

	// Add skip CRC parameter if needed
	if options.SkipCRC {
		req.AddQueryParam("skip_crc", "1")
	}

	// Create a buffer for the multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Create the form file
	formFile := filepath.Base(options.ImagePath)
	part, err := writer.CreateFormFile("update", formFile)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	// Copy the file to the form
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file to form: %w", err)
	}

	// Close the writer to finalize the form data
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Set the multipart form data on the request
	req.Method = "POST"
	req.SetMultipartForm(body, writer.FormDataContentType())

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors in the response
	if err := checkResponseError(resp); err != nil {
		return fmt.Errorf("flash operation failed: %w", err)
	}

	return nil
}

// FlashNodeLocal flashes a node with an image file that is accessible from the BMC
func (c *Client) FlashNodeLocal(node int, imagePath string) error {
	if node < 1 || node > 4 {
		return fmt.Errorf("invalid node number: %d (must be 1-4)", node)
	}

	if imagePath == "" {
		return fmt.Errorf("image path is required")
	}

	// Create a new request
	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "update")
	req.AddQueryParam("node", strconv.Itoa(node-1)) // BMC uses 0-based indexing
	req.AddQueryParam("path", imagePath)

	// Send the request
	resp, err := req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors in the response
	if err := checkResponseError(resp); err != nil {
		return fmt.Errorf("flash operation failed: %w", err)
	}

	return nil
}

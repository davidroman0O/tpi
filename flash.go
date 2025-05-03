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
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
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

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get image file info: %w", err)
	}
	fileSize := fileInfo.Size()
	fileName := filepath.Base(options.ImagePath)

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

	// Step 1: Create a request to get the handle
	req, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	req.AddQueryParam("opt", "set")
	req.AddQueryParam("type", "flash")
	req.AddQueryParam("file", fileName)
	req.AddQueryParam("length", strconv.FormatInt(fileSize, 10))
	req.AddQueryParam("node", strconv.Itoa(node-1)) // BMC uses 0-based indexing

	// Add SHA256 if provided
	if options.SHA256 != "" {
		req.AddQueryParam("sha256", options.SHA256)
	}

	// Add skip CRC if specified
	if options.SkipCRC {
		req.AddQueryParam("skip_crc", "1")
	}

	// Send the request to get the handle
	resp, err := req.Send()
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to initiate flash operation: %s: %s", resp.Status, string(body))
	}

	// Parse the response to get the handle
	var respData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract the handle directly from the top level
	handle, ok := respData["handle"].(float64)
	if !ok {
		return fmt.Errorf("invalid response: missing handle")
	}

	fmt.Printf("Started transfer of %.2f GiB...\n", float64(fileSize)/(1024*1024*1024))

	// Step 2: Upload the file using the handle
	// Create a buffer for the multipart form data
	var formBuffer bytes.Buffer
	writer := multipart.NewWriter(&formBuffer)

	// Create the form file part
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	// Reset file pointer to the beginning
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to reset file: %w", err)
	}

	// Copy the file to the form
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file to form: %w", err)
	}

	// Close the writer to finalize the form data
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create upload URL
	uploadURLStr := fmt.Sprintf("%s://%s/api/bmc/upload/%d",
		c.ApiVersion.GetScheme(),
		c.Host,
		int(handle))

	// Parse the upload URL
	uploadURL, err := url.Parse(uploadURLStr)
	if err != nil {
		return fmt.Errorf("failed to parse upload URL: %w", err)
	}

	// Create upload request
	uploadReq, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	// Set the URL and method for the upload
	uploadReq.URL = uploadURL
	uploadReq.Method = "POST"
	uploadReq.SetMultipartForm(&formBuffer, writer.FormDataContentType())

	// Allow up to 60 minutes for the upload
	uploadReq.Timeout = 60 * time.Minute

	// Send the upload request
	uploadResp, err := uploadReq.Send()
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	defer uploadResp.Body.Close()

	// Step 3: Monitor the flashing progress
	return c.watchFlashingProgress(int(handle), fileSize)
}

// watchFlashingProgress watches the progress of a flashing operation
func (c *Client) watchFlashingProgress(handle int, fileSize int64) error {
	// Initial delay to allow the flashing to begin
	time.Sleep(3 * time.Second)

	// Create a new request to check progress
	progressReq, err := c.newRequest()
	if err != nil {
		return fmt.Errorf("failed to create progress request: %w", err)
	}

	// Add query parameters for the progress request
	progressReq.AddQueryParam("opt", "get")
	progressReq.AddQueryParam("type", "flash")

	// Set a longer timeout for progress monitoring
	progressReq.Timeout = 30 * time.Second

	var verifying bool
	startTime := time.Now()

	for {
		// Send the request
		resp, err := progressReq.Send()
		if err != nil {
			fmt.Printf("\nError checking progress: %v. Retrying in 5 seconds...\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// Parse the response
		var respData map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to parse progress response: %w", err)
		}
		resp.Body.Close()

		// Check if the transfer is still in progress
		if transferring, ok := respData["Transferring"].(map[string]interface{}); ok {
			// Extract the transfer ID
			var id float64
			if idFloat, ok := transferring["id"].(float64); ok {
				id = idFloat
			} else if idStr, ok := transferring["id"].(string); ok {
				idInt, err := strconv.ParseInt(idStr, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse transfer ID: %w", err)
				}
				id = float64(idInt)
			} else {
				return fmt.Errorf("unexpected transfer ID type")
			}

			// Verify the ID matches our handle
			if int(id) != handle {
				return fmt.Errorf("invalid transfer handle: expected %d, got %d", handle, int(id))
			}

			// Extract bytes written
			var bytesWritten int64
			if bytesFloat, ok := transferring["bytes_written"].(float64); ok {
				bytesWritten = int64(bytesFloat)
			} else if bytesStr, ok := transferring["bytes_written"].(string); ok {
				bytesWritten, err = strconv.ParseInt(bytesStr, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse bytes written: %w", err)
				}
			} else {
				return fmt.Errorf("unexpected bytes_written type")
			}

			// Calculate progress
			if bytesWritten >= fileSize {
				if !verifying {
					fmt.Println("\nVerifying checksum...")
					verifying = true
				}
			} else {
				progress := float64(bytesWritten) / float64(fileSize) * 100
				elapsed := time.Since(startTime)
				var eta time.Duration
				if bytesWritten > 0 {
					// Calculate ETA
					bytesPerSecond := float64(bytesWritten) / elapsed.Seconds()
					if bytesPerSecond > 0 {
						etaSeconds := float64(fileSize-bytesWritten) / bytesPerSecond
						eta = time.Duration(etaSeconds) * time.Second
					}
				}

				etaStr := "calculating..."
				if eta > 0 {
					etaStr = eta.Round(time.Second).String()
				}

				fmt.Printf("\rProgress: %.1f%% (%s / %s) - ETA: %s",
					progress,
					formatBytes(bytesWritten),
					formatBytes(fileSize),
					etaStr)
			}

			time.Sleep(1 * time.Second)
			continue
		}

		// Check if done
		if _, ok := respData["Done"]; ok {
			fmt.Println("\nFlashing completed successfully")
			break
		}

		// Check for errors
		if errMap, ok := respData["Error"].(map[string]interface{}); ok {
			return fmt.Errorf("error occurred during flashing: %v", errMap)
		}

		// If we don't recognize the response, wait and retry
		fmt.Printf("\rWaiting for flashing to complete...")
		time.Sleep(2 * time.Second)
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

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

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
	"context"
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
	"strings"
	"sync"
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

	// Send the request to get the handle with retry logic
	var handle float64
	for attempts := 0; attempts < 3; attempts++ {
		resp, err := req.Send()
		if err != nil {
			if attempts < 2 {
				fmt.Printf("Error initializing flash operation: %v. Retrying in 3 seconds...\n", err)
				time.Sleep(3 * time.Second)
				continue
			}
			return fmt.Errorf("failed to send request after retries: %w", err)
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			if attempts < 2 {
				fmt.Printf("Error initializing flash operation: %s. Retrying in 3 seconds...\n", resp.Status)
				time.Sleep(3 * time.Second)
				continue
			}
			return fmt.Errorf("failed to initiate flash operation: %s: %s", resp.Status, string(body))
		}

		// Parse the response to get the handle
		var respData map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
			if attempts < 2 {
				fmt.Printf("Error parsing response: %v. Retrying in 3 seconds...\n", err)
				time.Sleep(3 * time.Second)
				continue
			}
			return fmt.Errorf("failed to parse response: %w", err)
		}

		// Extract the handle directly from the top level
		var ok bool
		handle, ok = respData["handle"].(float64)
		if !ok {
			if attempts < 2 {
				fmt.Printf("Error extracting handle from response. Retrying in 3 seconds...\n")
				time.Sleep(3 * time.Second)
				continue
			}
			return fmt.Errorf("invalid response: missing handle")
		}

		// If we get here, we have a valid handle
		break
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

	// Send the upload request with retry logic
	for attempts := 0; attempts < 3; attempts++ {
		uploadResp, err := uploadReq.Send()
		if err != nil {
			if attempts < 2 {
				fmt.Printf("Error uploading file: %v. Retrying in 5 seconds...\n", err)
				time.Sleep(5 * time.Second)
				continue
			}
			return fmt.Errorf("failed to upload file after retries: %w", err)
		}
		defer uploadResp.Body.Close()

		// Check response status
		if uploadResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(uploadResp.Body)
			if attempts < 2 {
				fmt.Printf("Error uploading file: %s. Retrying in 5 seconds...\n", uploadResp.Status)
				time.Sleep(5 * time.Second)
				continue
			}
			return fmt.Errorf("failed to upload file: %s: %s", uploadResp.Status, string(body))
		}

		// If we get here, the upload was successful
		break
	}

	// Step 3: Monitor the flashing progress
	// Create a context with timeout for the entire operation
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Minute)
	defer cancel()

	return c.watchFlashingProgress(ctx, int(handle), fileSize)
}

// watchFlashingProgress watches the progress of a flashing operation with improved error handling
func (c *Client) watchFlashingProgress(ctx context.Context, handle int, fileSize int64) error {
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

	// Use a much longer timeout for progress checking as the BMC can be slow to respond
	progressReq.Timeout = 45 * time.Second

	// Variables for tracking progress
	var (
		verifying      bool
		startTime      = time.Now()
		consecutiveErr int
		maxRetries     = 20 // Increase max retries
		lastBytes      int64
		lastUpdateTime = startTime
		speedWindow    = []float64{}
		lastErrorMsg   string
	)

	// Use a ticker for consistent polling
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Use a mutex to protect shared data during updates
	var mu sync.Mutex

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Set a timeout just for this request
			reqCtx, reqCancel := context.WithTimeout(ctx, 45*time.Second)
			progressReq.SetContext(reqCtx)

			// Send the request
			resp, err := progressReq.Send()
			reqCancel() // Clean up the context immediately after the request

			if err != nil {
				consecutiveErr++

				// Only print error message if it's different from the last one
				// or if it's been a while since we printed an error
				errorMsg := err.Error()
				if errorMsg != lastErrorMsg || consecutiveErr%5 == 1 {
					if strings.Contains(errorMsg, "context deadline exceeded") {
						fmt.Printf("\nWaiting for BMC response... (%d/%d)", consecutiveErr, maxRetries)
					} else {
						fmt.Printf("\nError checking progress: %v. Retrying... (%d/%d)",
							err, consecutiveErr, maxRetries)
					}
					lastErrorMsg = errorMsg
				}

				if consecutiveErr >= maxRetries {
					return fmt.Errorf("too many consecutive errors (%d): %w", consecutiveErr, err)
				}

				// Use exponential backoff for retries but cap at 10 seconds
				backoff := time.Duration(consecutiveErr/2) * time.Second
				if backoff > 10*time.Second {
					backoff = 10 * time.Second
				}

				time.Sleep(backoff)
				continue
			}

			// Reset consecutive errors on success
			if consecutiveErr > 0 {
				fmt.Printf("\nResumed progress monitoring after %d errors", consecutiveErr)
				consecutiveErr = 0
				lastErrorMsg = ""
			}

			// Parse the response
			var respData map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
				resp.Body.Close()
				fmt.Printf("\nError parsing progress response: %v. Retrying...", err)
				continue
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
						continue
					}
					id = float64(idInt)
				} else {
					continue
				}

				// Verify the ID matches our handle - just log and continue if not
				if int(id) != handle {
					continue
				}

				// Extract bytes written
				var bytesWritten int64
				if bytesFloat, ok := transferring["bytes_written"].(float64); ok {
					bytesWritten = int64(bytesFloat)
				} else if bytesStr, ok := transferring["bytes_written"].(string); ok {
					bytesWritten, err = strconv.ParseInt(bytesStr, 10, 64)
					if err != nil {
						continue
					}
				} else {
					continue
				}

				// Protect updates with mutex
				mu.Lock()

				// Calculate progress
				if bytesWritten >= fileSize {
					if !verifying {
						fmt.Println("\nVerifying checksum...")
						verifying = true
					}
				} else {
					progress := float64(bytesWritten) / float64(fileSize) * 100

					// Calculate speed
					now := time.Now()
					elapsed := now.Sub(lastUpdateTime)

					var speed float64
					if elapsed.Seconds() > 0 && lastBytes > 0 {
						// Calculate current speed
						currentSpeed := float64(bytesWritten-lastBytes) / elapsed.Seconds()

						// Add to speed window (up to 5 samples)
						speedWindow = append(speedWindow, currentSpeed)
						if len(speedWindow) > 5 {
							speedWindow = speedWindow[1:]
						}

						// Calculate average speed from window
						var totalSpeed float64
						for _, s := range speedWindow {
							totalSpeed += s
						}
						speed = totalSpeed / float64(len(speedWindow))
					}

					// Update tracking variables
					lastBytes = bytesWritten
					lastUpdateTime = now

					// Calculate ETA
					totalElapsed := time.Since(startTime)
					var eta time.Duration

					if speed > 0 {
						remainingBytes := float64(fileSize - bytesWritten)
						etaSeconds := remainingBytes / speed
						eta = time.Duration(etaSeconds) * time.Second
					}

					etaStr := "calculating..."
					if eta > 0 {
						etaStr = eta.Round(time.Second).String()
					}

					speedStr := "calculating..."
					if speed > 0 {
						speedStr = formatBytes(int64(speed)) + "/s"
					}

					// Use a carriage return to overwrite the current line
					fmt.Printf("\rProgress: %.1f%% (%s / %s) • Speed: %s • Elapsed: %s • ETA: %s    ",
						progress,
						formatBytes(bytesWritten),
						formatBytes(fileSize),
						speedStr,
						totalElapsed.Round(time.Second),
						etaStr)
				}

				mu.Unlock()
				continue
			}

			// Check if done
			if _, ok := respData["Done"]; ok {
				fmt.Println("\nFlashing completed successfully")
				return nil
			}

			// Check for errors
			if errMap, ok := respData["Error"].(map[string]interface{}); ok {
				return fmt.Errorf("error occurred during flashing: %v", errMap)
			}

			// If we don't recognize the response, log and continue
			fmt.Printf("\rWaiting for flashing to complete...")
		}
	}
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

	// Send the request with retry logic
	for attempts := 0; attempts < 3; attempts++ {
		resp, err := req.Send()
		if err != nil {
			if attempts < 2 {
				fmt.Printf("Error sending request: %v. Retrying in 3 seconds...\n", err)
				time.Sleep(3 * time.Second)
				continue
			}
			return fmt.Errorf("failed to send request after retries: %w", err)
		}
		defer resp.Body.Close()

		// Check for errors in the response
		if err := checkResponseError(resp); err != nil {
			if attempts < 2 {
				fmt.Printf("Error in response: %v. Retrying in 3 seconds...\n", err)
				time.Sleep(3 * time.Second)
				continue
			}
			return fmt.Errorf("flash operation failed: %w", err)
		}

		// If we get here, the operation was successful
		fmt.Println("Flash operation completed successfully")
		return nil
	}

	return fmt.Errorf("failed to complete flash operation after maximum retries")
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

// SetContext adds a context to an existing request
func (r *Request) SetContext(ctx context.Context) {
	r.Context = ctx
}

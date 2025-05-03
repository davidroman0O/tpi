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
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	// Test case: client creation without a host should return an error
	client, err := NewClient()
	if err == nil {
		t.Error("Expected error when creating client without host, got nil")
	}
	if client != nil {
		t.Error("Expected nil client when creating without host")
	}

	// Test case: client creation with a host should succeed
	client, err = NewClient(WithHost("192.168.1.1"))
	if err != nil {
		t.Errorf("Unexpected error when creating client with host: %v", err)
	}
	if client == nil {
		t.Error("Expected non-nil client when creating with host")
	}
	if client.Host != "192.168.1.1" {
		t.Errorf("Expected host to be 192.168.1.1, got %s", client.Host)
	}
}

func TestWithHost(t *testing.T) {
	// Create a client with a host
	client, err := NewClient(WithHost("192.168.1.1"))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify the host was set correctly
	if client.Host != "192.168.1.1" {
		t.Errorf("Expected host to be 192.168.1.1, got %s", client.Host)
	}
}

func TestWithApiVersion(t *testing.T) {
	// Create a client with a custom API version
	client, err := NewClient(
		WithHost("192.168.1.1"),
		WithApiVersion(ApiVersionV1),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify the API version was set correctly
	if client.ApiVersion != ApiVersionV1 {
		t.Errorf("Expected API version to be %s, got %s", ApiVersionV1, client.ApiVersion)
	}
}

func TestWithCredentials(t *testing.T) {
	// Create a client with credentials
	username := "admin"
	password := "password"
	client, err := NewClient(
		WithHost("192.168.1.1"),
		WithCredentials(username, password),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify the credentials were set correctly
	if client.auth.Username != username {
		t.Errorf("Expected username to be %s, got %s", username, client.auth.Username)
	}
	if client.auth.Password != password {
		t.Errorf("Expected password to be %s, got %s", password, client.auth.Password)
	}
}

func TestWithTimeout(t *testing.T) {
	// Create a client with a custom timeout
	timeout := 20 * time.Second
	client, err := NewClient(
		WithHost("192.168.1.1"),
		WithTimeout(timeout),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify the timeout was set correctly
	if client.httpClient.Timeout != timeout {
		t.Errorf("Expected timeout to be %v, got %v", timeout, client.httpClient.Timeout)
	}
}

func TestDefaultApiVersion(t *testing.T) {
	// Create a client without specifying an API version
	client, err := NewClient(WithHost("192.168.1.1"))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify the default API version was set
	if client.ApiVersion != ApiVersionV1_1 {
		t.Errorf("Expected default API version to be %s, got %s", ApiVersionV1_1, client.ApiVersion)
	}
}

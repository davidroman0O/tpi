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
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	tpi "github.com/davidroman0O/tpi/client"
)

// Agent represents the TPI agent server
type Agent struct {
	config    AgentConfig
	client    *tpi.Client
	server    *http.Server
	router    *http.ServeMux
	authCache map[string]time.Time
	mu        sync.RWMutex
}

// NewAgent creates a new TPI agent server
func NewAgent(config AgentConfig, client *tpi.Client) (*Agent, error) {
	if config.Port == 0 {
		config.Port = DefaultAgentPort
	}

	router := http.NewServeMux()

	agent := &Agent{
		config:    config,
		client:    client,
		router:    router,
		authCache: make(map[string]time.Time),
	}

	// Register command handler
	router.HandleFunc("/", agent.handleCommand)

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: router,
	}

	agent.server = server

	return agent, nil
}

// Start starts the agent server
func (a *Agent) Start(ctx context.Context) error {
	var listener net.Listener
	var err error

	// Set up TLS if enabled
	if a.config.TLSEnabled {
		if a.config.TLSCertFile == "" || a.config.TLSKeyFile == "" {
			return fmt.Errorf("TLS enabled but certificate or key file not provided")
		}

		// Create TLS configuration
		cert, err := tls.LoadX509KeyPair(a.config.TLSCertFile, a.config.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS certificates: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		// Create TLS listener
		listener, err = tls.Listen("tcp", a.server.Addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to create TLS listener: %w", err)
		}
	} else {
		// Create non-TLS listener
		listener, err = net.Listen("tcp", a.server.Addr)
		if err != nil {
			return fmt.Errorf("failed to create listener: %w", err)
		}
	}

	log.Printf("TPI Agent started on %s", a.server.Addr)

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error shutting down server: %v", err)
		}
	}()

	// Start serving
	if err := a.server.Serve(listener); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// handleCommand handles incoming command requests
func (a *Agent) handleCommand(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check IP allowlist if configured
	if len(a.config.AllowedClients) > 0 {
		clientIP := strings.Split(r.RemoteAddr, ":")[0]
		allowed := false
		for _, allowedIP := range a.config.AllowedClients {
			if clientIP == allowedIP {
				allowed = true
				break
			}
		}
		if !allowed {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	// Decode the command
	var cmd Command
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&cmd); err != nil {
		sendErrorResponse(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Check authentication if enabled
	if a.config.Auth.Secret != "" {
		if !a.authenticateRequest(cmd.Auth) {
			sendErrorResponse(w, "Authentication failed", http.StatusUnauthorized)
			return
		}
	}

	// Execute the command
	result, err := a.executeCommand(cmd)
	if err != nil {
		sendErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send success response
	response := Response{
		Success: true,
		Result:  result,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// authenticateRequest verifies the authentication of an incoming request
func (a *Agent) authenticateRequest(auth AgentAuthConfig) bool {
	// Check if token-based authentication is used
	if auth.Token != "" {
		a.mu.RLock()
		timestamp, exists := a.authCache[auth.Token]
		a.mu.RUnlock()

		if exists {
			// Check if token has expired
			if a.config.Auth.Expiry > 0 && time.Since(timestamp) > a.config.Auth.Expiry {
				a.mu.Lock()
				delete(a.authCache, auth.Token)
				a.mu.Unlock()
				return false
			}
			return true
		}
	}

	// Otherwise check if secret matches
	if auth.Secret == a.config.Auth.Secret {
		// If secret is provided and correct, generate a token for future use
		if auth.Token == "" {
			// Token is usually generated by the client
			return true
		}

		// Cache the token with the current timestamp
		a.mu.Lock()
		a.authCache[auth.Token] = time.Now()
		a.mu.Unlock()
		return true
	}

	return false
}

// RunAgent runs the agent server as a standalone process
func RunAgent(config AgentConfig, clientOpts ...tpi.Option) error {
	// Create the TPI client
	client, err := tpi.NewClient(clientOpts...)
	if err != nil {
		return fmt.Errorf("failed to create TPI client: %w", err)
	}

	// Create the agent
	agent, err := NewAgent(config, client)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Set up context with signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal")
		cancel()
	}()

	// Start the agent
	if err := agent.Start(ctx); err != nil {
		return fmt.Errorf("agent error: %w", err)
	}

	return nil
}

// sendErrorResponse sends an error response to the client
func sendErrorResponse(w http.ResponseWriter, errMsg string, statusCode int) {
	response := Response{
		Success: false,
		Error:   errMsg,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

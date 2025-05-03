# Turing Pi Control Tools

This repository contains tools for controlling Turing Pi boards, split into two components:

## Components

### 1. [Client Library](/client)

A Go library for programmatically controlling Turing Pi boards via their BMC API.

```bash
go get github.com/davidroman0O/tpi/client
```

[Learn more about the client library →](/client)

### 2. [CLI Tool](/cli)

A command-line interface for controlling Turing Pi boards.

```bash
# Install pre-built binary (see releases)
# or build from source:
cd cli && go build
```

[Learn more about the CLI tool →](/cli)

## Repository Structure

```
tpi/
├── client/           # Client library (minimal dependencies)
│   ├── *.go          # Core functionality files
│   └── testdata/     # Test data
├── cli/              # Command-line interface 
│   ├── commands/     # CLI command implementations
│   └── main.go       # CLI entry point
└── .github/          # GitHub workflows and config
```

## Quick Start

### Using the CLI

```bash
# Power on node 1
./cli/tpi power on 1 --host=192.168.1.91

# Get power status
./cli/tpi power status --host=192.168.1.91
```

### Using the Client Library

```go
package main

import (
	"fmt"
	"os"

	"github.com/davidroman0O/tpi/client"
)

func main() {
	// Create a client
	c, err := client.NewClient(
		client.WithHost("192.168.1.91"),
		client.WithCredentials("root", "turing"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Power on node 1
	if err := c.PowerOn(1); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Node 1 powered on!")
}
```

## License

Apache License 2.0 
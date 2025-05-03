# Turing Pi Client Library

A Go client library for controlling Turing Pi boards via their BMC API.

## Features

- Power management (on/off/reset)
- USB configuration
- UART access
- Firmware management
- Network configuration
- Advanced node modes (MSD, normal)

## Installation

```bash
go get github.com/davidroman0O/tpi/client
```

## Usage

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
		fmt.Fprintf(os.Stderr, "Error creating client: %v\n", err)
		os.Exit(1)
	}

	// Get board info
	info, err := c.Info()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting info: %v\n", err)
		os.Exit(1)
	}

	// Print info
	for key, value := range info {
		fmt.Printf("%s: %s\n", key, value)
	}
}
```

## API Documentation

### Client Creation

```go
// Create a client with options
client, err := client.NewClient(
    client.WithHost("192.168.1.91"),
    client.WithCredentials("root", "turing"),
    client.WithApiVersion(client.ApiVersionV1_1),
    client.WithTimeout(5 * time.Second),
)
```

### Power Management

```go
// Get power status of all nodes
status, err := client.PowerStatus()

// Power on node 1
err := client.PowerOn(1)

// Power off node 1
err := client.PowerOff(1)

// Reset node 1
err := client.PowerReset(1)

// Power on all nodes
err := client.PowerOnAll()

// Power off all nodes
err := client.PowerOffAll()
```

See code documentation for more details on available functions.

## License

Apache License 2.0 
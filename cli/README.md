# Turing Pi CLI Tool

A command-line interface for controlling Turing Pi boards.

## Installation

### Download pre-built binaries

Download the appropriate binary for your platform from the [Releases](https://github.com/davidroman0O/tpi/releases) page.

### Build from source

```bash
git clone https://github.com/davidroman0O/tpi.git
cd tpi/cli
go build
```

## Usage

```
tpi [command] [flags]
```

### Examples

```bash
# Power on node 1
tpi power on 1 --host=192.168.1.91

# Check power status
tpi power status --host=192.168.1.91

# Get information about the board
tpi info --host=192.168.1.91

# Set USB device mode for node 2
tpi usb device 2 --host=192.168.1.91

# Login and cache credentials
tpi auth login --host=192.168.1.91
```

## Available Commands

- `about` - Display detailed information about the BMC daemon
- `advanced` - Configure advanced node modes (normal, MSD)
- `auth` - Manage authentication and token persistence
- `eth` - Configure the on-board Ethernet switch
- `firmware` - Upgrade the firmware of the BMC
- `flash` - Flash a given node with an OS image
- `info` - Print Turing Pi info
- `power` - Power on/off or reset specific nodes
- `reboot` - Reboot the BMC chip
- `uart` - Read or write over UART
- `usb` - Change the USB device/host configuration
- `version` - Print version information

## Global Flags

- `--host`, `-H` - BMC hostname or IP address
- `--user`, `-u` - BMC username
- `--password`, `-p` - BMC password
- `--api-version`, `-a` - Force which version of the BMC API to use

## Authentication

The CLI supports caching authentication tokens for convenience:

```bash
# Login and cache token
tpi auth login --host=192.168.1.91

# Check authentication status
tpi auth status

# Logout/clear token
tpi auth logout --host=192.168.1.91
```

## License

Apache License 2.0 
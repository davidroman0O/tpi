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

// ApiVersion represents the BMC API version
type ApiVersion string

const (
	ApiVersionV1   ApiVersion = "v1"
	ApiVersionV1_1 ApiVersion = "v1-1"
)

// GetScheme returns the HTTP scheme for the given API version
func (a ApiVersion) GetScheme() string {
	switch a {
	case ApiVersionV1:
		return "http"
	case ApiVersionV1_1, "":
		return "https"
	default:
		return "https"
	}
}

// PowerCmd represents power commands
type PowerCmd string

const (
	PowerOn     PowerCmd = "on"
	PowerOff    PowerCmd = "off"
	PowerReset  PowerCmd = "reset"
	PowerStatus PowerCmd = "status"
)

// UsbCmd represents USB commands
type UsbCmd string

const (
	// Configure the specified node as USB device. The BMC itself or USB-A port is USB host
	UsbDevice UsbCmd = "device"
	// Configure the specified node as USB Host. USB devices can be attached to the USB-A port on the board
	UsbHost UsbCmd = "host"
	// Turns the module into flashing mode and sets the USB_OTG into device mode
	UsbFlash UsbCmd = "flash"
	// Get status
	UsbStatus UsbCmd = "status"
)

// UsbStatusInfo represents the current USB configuration
type UsbStatusInfo struct {
	Node  string
	Mode  string
	Route string
}

// ModeCmd represents advanced mode commands
type ModeCmd string

const (
	// Clear any advanced mode
	ModeNormal ModeCmd = "normal"
	// Reboots supported compute modules and expose its eMMC storage as a mass storage device
	ModeMsd ModeCmd = "msd"
)

// EthCmd represents Ethernet commands
type EthCmd string

const (
	EthReset EthCmd = "reset"
)

// CoolingCmd represents cooling commands
type CoolingCmd string

const (
	CoolingSet    CoolingCmd = "set"
	CoolingStatus CoolingCmd = "status"
)

// GetSet represents get or set operations
type GetSet string

const (
	GetOperation GetSet = "get"
	SetOperation GetSet = "set"
)

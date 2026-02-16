// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package socket

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

// DeviceDiscovery handles network device discovery through DHCP monitoring
type DeviceDiscovery struct {
	// Configuration
	config *DeviceDiscoveryConfig

	// State
	mutex   sync.RWMutex
	enabled bool

	// Device database
	devices  map[string]*types.DeviceInfo
	devMutex sync.RWMutex

	// Statistics
	stats *DeviceDiscoveryStats

	// Event handlers
	newDeviceHandler    NewDeviceHandler
	deviceUpdateHandler DeviceUpdateHandler

	// Logger
	logger *logging.Logger

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// DeviceDiscoveryConfig holds configuration for device discovery
type DeviceDiscoveryConfig struct {
	// Discovery settings
	Enabled         bool `hcl:"enabled,optional"`
	TrackByMAC      bool `hcl:"track_by_mac,optional"`
	TrackByIP       bool `hcl:"track_by_ip,optional"`
	TrackByHostname bool `hcl:"track_by_hostname,optional"`

	// Database settings
	MaxDevices      int           `hcl:"max_devices,optional"`
	CleanupInterval time.Duration `hcl:"cleanup_interval,optional"`
	DeviceTimeout   time.Duration `hcl:"device_timeout,optional"`

	// Vendor detection
	LookupVendors  bool   `hcl:"lookup_vendors,optional"`
	VendorDatabase string `hcl:"vendor_database,optional"`

	// Device classification
	ClassifyDevices  bool     `hcl:"classify_devices,optional"`
	DeviceCategories []string `hcl:"device_categories,optional"`

	// Alerting
	AlertOnNewDevice bool `hcl:"alert_on_new_device,optional"`
	AlertOnUnknown   bool `hcl:"alert_on_unknown,optional"`
	AlertOnRogue     bool `hcl:"alert_on_rogue,optional"`

	// Integration
	ExportToIPS      bool `hcl:"export_to_ips,optional"`
	ExportToLearning bool `hcl:"export_to_learning,optional"`
}

// DeviceDiscoveryStats holds statistics for device discovery
type DeviceDiscoveryStats struct {
	DevicesDiscovered uint64    `json:"devices_discovered"`
	DevicesUpdated    uint64    `json:"devices_updated"`
	DevicesExpired    uint64    `json:"devices_expired"`
	VendorLookups     uint64    `json:"vendor_lookups"`
	VendorMatches     uint64    `json:"vendor_matches"`
	Classifications   uint64    `json:"classifications"`
	AlertsSent        uint64    `json:"alerts_sent"`
	DatabaseSize      uint64    `json:"database_size"`
	LastUpdate        time.Time `json:"last_update"`
}

// NewDeviceHandler handles new device discoveries
type NewDeviceHandler func(device *types.DeviceInfo) error

// DeviceUpdateHandler handles device updates
type DeviceUpdateHandler func(device *types.DeviceInfo) error

// DefaultDeviceDiscoveryConfig returns default configuration
func DefaultDeviceDiscoveryConfig() *DeviceDiscoveryConfig {
	return &DeviceDiscoveryConfig{
		Enabled:          true,
		TrackByMAC:       true,
		TrackByIP:        true,
		TrackByHostname:  true,
		MaxDevices:       10000,
		CleanupInterval:  1 * time.Hour,
		DeviceTimeout:    24 * time.Hour,
		LookupVendors:    true,
		ClassifyDevices:  true,
		AlertOnNewDevice: false,
		AlertOnUnknown:   false,
		AlertOnRogue:     false,
		ExportToIPS:      true,
		ExportToLearning: true,
	}
}

// NewDeviceDiscovery creates a new device discovery module
func NewDeviceDiscovery(logger *logging.Logger, config *DeviceDiscoveryConfig) *DeviceDiscovery {
	if config == nil {
		config = DefaultDeviceDiscoveryConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	dd := &DeviceDiscovery{
		config: config,
		stats: &DeviceDiscoveryStats{
			LastUpdate: time.Now(),
		},
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
		devices: make(map[string]*types.DeviceInfo),
	}

	return dd
}

// Start starts the device discovery module
func (dd *DeviceDiscovery) Start() error {
	dd.mutex.Lock()
	defer dd.mutex.Unlock()

	if !dd.config.Enabled {
		dd.logger.Info("Device discovery disabled")
		return nil
	}

	dd.logger.Info("Starting device discovery")

	// Start cleanup goroutine
	go dd.cleanupWorker()

	dd.enabled = true
	dd.logger.Info("Device discovery started")

	return nil
}

// Stop stops the device discovery module
func (dd *DeviceDiscovery) Stop() {
	dd.mutex.Lock()
	defer dd.mutex.Unlock()

	if !dd.enabled {
		return
	}

	dd.logger.Info("Stopping device discovery")

	// Cancel context
	dd.cancel()

	dd.enabled = false
	dd.logger.Info("Device discovery stopped")
}

// ProcessDHCPDiscover processes a DHCP discover to discover new devices
func (dd *DeviceDiscovery) ProcessDHCPDiscover(event *types.DHCPDiscoverEvent) error {
	if !dd.enabled {
		return nil
	}

	// Create device key
	var deviceKey string
	if dd.config.TrackByMAC {
		deviceKey = strings.ToLower(event.MACAddress)
	} else if dd.config.TrackByHostname && event.HostName != "" {
		deviceKey = strings.ToLower(event.HostName)
	} else {
		return nil // No tracking method configured
	}

	dd.devMutex.Lock()
	defer dd.devMutex.Unlock()

	now := time.Now()
	device, exists := dd.devices[deviceKey]

	if !exists {
		// New device discovered
		atomic.AddUint64(&dd.stats.DevicesDiscovered, 1)

		device = &types.DeviceInfo{
			MACAddress:  event.MACAddress,
			HostName:    event.HostName,
			Vendor:      "",
			DeviceType:  "Unknown",
			FirstSeen:   now,
			LastSeen:    now,
			DHCPOptions: make(map[string]interface{}),
		}

		// Add vendor class if present
		if event.VendorClass != "" {
			device.DHCPOptions["vendor_class"] = event.VendorClass
		}

		// Lookup vendor
		if dd.config.LookupVendors {
			if vendor := dd.lookupVendor(event.MACAddress); vendor != "" {
				device.Vendor = vendor
				atomic.AddUint64(&dd.stats.VendorMatches, 1)
			}
			atomic.AddUint64(&dd.stats.VendorLookups, 1)
		}

		// Classify device
		if dd.config.ClassifyDevices {
			device.DeviceType = dd.classifyDevice(device)
			atomic.AddUint64(&dd.stats.Classifications, 1)
		}

		// Add to database
		dd.devices[deviceKey] = device
		atomic.AddUint64(&dd.stats.DatabaseSize, 1)

		// Call new device handler
		if dd.newDeviceHandler != nil {
			if err := dd.newDeviceHandler(device); err != nil {
				dd.logger.Error("New device handler failed", "error", err)
			}
		}

		// Alert on new device
		if dd.config.AlertOnNewDevice {
			dd.sendAlert("NEW_DEVICE", fmt.Sprintf("New device discovered: %s (%s)",
				device.MACAddress, device.HostName))
			atomic.AddUint64(&dd.stats.AlertsSent, 1)
		}

		dd.logger.Info("New device discovered",
			"mac", device.MACAddress,
			"hostname", device.HostName,
			"vendor", device.Vendor,
			"type", device.DeviceType)

	} else {
		// Update existing device
		atomic.AddUint64(&dd.stats.DevicesUpdated, 1)
		device.LastSeen = now

		// Update hostname if changed
		if event.HostName != "" && device.HostName != event.HostName {
			device.HostName = event.HostName
		}

		// Call update handler
		if dd.deviceUpdateHandler != nil {
			if err := dd.deviceUpdateHandler(device); err != nil {
				dd.logger.Error("Device update handler failed", "error", err)
			}
		}
	}

	return nil
}

// ProcessDHCPAck processes a DHCP ACK to update device IP information
func (dd *DeviceDiscovery) ProcessDHCPAck(event *types.DHCPAckEvent) error {
	if !dd.enabled || !dd.config.TrackByIP {
		return nil
	}

	deviceKey := strings.ToLower(event.MACAddress)
	if deviceKey == "" {
		return nil
	}

	dd.devMutex.Lock()
	defer dd.devMutex.Unlock()

	device, exists := dd.devices[deviceKey]
	if !exists {
		// Device not found by MAC, try searching by other criteria if needed?
		// But usually MAC is best.
		return nil
	}

	if event.YourIP != nil {
		device.IPAddress = event.YourIP

		// Set lease expiry
		if event.LeaseTime > 0 {
			expiry := time.Now().Add(time.Duration(event.LeaseTime) * time.Second)
			device.LeaseExpiry = &expiry
		}

		dd.logger.Debug("Device IP updated",
			"mac", device.MACAddress,
			"ip", device.IPAddress,
			"lease_expiry", device.LeaseExpiry)

		// Call update handler
		if dd.deviceUpdateHandler != nil {
			if err := dd.deviceUpdateHandler(device); err != nil {
				dd.logger.Error("Device update handler failed", "error", err)
			}
		}
	}

	return nil
}

// lookupVendor looks up vendor information from MAC address
func (dd *DeviceDiscovery) lookupVendor(macAddr string) string {
	// In a real implementation, this would query a large OUI database.
	// We use a curated list of common network and device manufacturers.

	mac := strings.ToUpper(strings.ReplaceAll(macAddr, ":", ""))
	if len(mac) < 6 {
		return "Unknown"
	}
	oui := mac[:6]

	vendors := map[string]string{
		// Apple
		"000393": "Apple", "000502": "Apple", "000A27": "Apple", "000A95": "Apple",
		"000D93": "Apple", "0010FA": "Apple", "001124": "Apple", "001451": "Apple",
		"0016CB": "Apple", "0017F2": "Apple", "0019E3": "Apple", "001B63": "Apple",
		"001C42": "Apple", "001C25": "Apple", "001D4F": "Apple", "001E52": "Apple",
		"001F5B": "Apple", "0021E9": "Apple", "002241": "Apple", "002312": "Apple",
		"002332": "Apple", "00236C": "Apple", "002436": "Apple", "002500": "Apple",
		"00254B": "Apple", "0025BC": "Apple", "002608": "Apple", "00264A": "Apple",
		"0026B0": "Apple", "0026BB": "Apple", "D8004D": "Apple", "D81D72": "Apple",
		"D83062": "Apple", "D88F76": "Apple", "D89695": "Apple", "D8A25E": "Apple",
		"D8CF9C": "Apple", "D8D1CB": "Apple",

		// Google
		"001A11": "Google", "3C5AB4": "Google", "D8EB97": "Google", "DAA119": "Google",
		"E4F042": "Google", "F4F5D8": "Google",

		// Samsung
		"0000F0": "Samsung", "000278": "Samsung", "0007AB": "Samsung", "000D70": "Samsung",
		"000FB3": "Samsung", "001247": "Samsung", "0012FB": "Samsung", "001599": "Samsung",
		"00166B": "Samsung", "0017C9": "Samsung", "0017D4": "Samsung", "0018AF": "Samsung",
		"001901": "Samsung", "001A99": "Samsung", "001B98": "Samsung", "001C43": "Samsung",

		// Cisco / Linksys
		"00000C": "Cisco", "000142": "Cisco", "000143": "Cisco", "000163": "Cisco",
		"000164": "Cisco", "000196": "Cisco", "000197": "Cisco", "0001C7": "Cisco",
		"0001C9": "Cisco", "000216": "Cisco", "000217": "Cisco", "00024A": "Cisco",
		"00024B": "Cisco", "00027D": "Cisco", "0002B9": "Cisco", "0002FA": "Cisco",
		"000625": "Linksys", "000C41": "Linksys", "000F66": "Linksys", "001310": "Linksys",
		"0014BF": "Linksys", "001839": "Linksys", "001D7E": "Linksys", "002129": "Linksys",
		"00226B": "Linksys", "002369": "Linksys", "00259C": "Linksys",

		// Netgear
		"00095B": "Netgear", "000FB5": "Netgear", "00146C": "Netgear", "00184D": "Netgear",
		"001B2F": "Netgear", "001E2A": "Netgear", "001F33": "Netgear", "00223F": "Netgear",
		"0024B2": "Netgear", "0026F2": "Netgear", "204E71": "Netgear", "28C687": "Netgear",

		// Intel
		"0002B3": "Intel", "000347": "Intel", "000423": "Intel", "0007E9": "Intel",
		"0008A1": "Intel", "000CF1": "Intel", "001302": "Intel", "0013E8": "Intel",
		"001500": "Intel", "00166F": "Intel", "001676": "Intel", "0018DE": "Intel",
		"0019D1": "Intel", "001B21": "Intel", "001C23": "Intel",
		"001D71": "Intel", "001E64": "Intel", "001E65": "Intel", "001F3C": "Intel",

		// Dell
		"000874": "Dell", "000BDB": "Dell", "000D56": "Dell", "000F1F": "Dell",
		"001143": "Dell", "00123F": "Dell", "001372": "Dell", "001422": "Dell",
		"0015C5": "Dell", "00188B": "Dell", "0019B9": "Dell", "001AF1": "Dell",
		"001A4B": "Dell", "001D09": "Dell",

		// HP
		"0001E6": "HP", "000344": "HP", "0004EA": "HP", "00055D": "HP",
		"000802": "HP", "000BCD": "HP", "000D9D": "HP", "000E7F": "HP",
		"000F20": "HP", "001083": "HP", "00110A": "HP", "001185": "HP",
		"001279": "HP", "001321": "HP", "001438": "HP", "001560": "HP",

		// Microsoft
		"0003FF": "Microsoft", "00125A": "Microsoft", "00155D": "Microsoft",
		"0017FA": "Microsoft", "001D2D": "Microsoft", "001DD8": "Microsoft",
		"002248": "Microsoft", "0025AE": "Microsoft", "0050F2": "Microsoft",

		// Raspberry Pi
		"28CDC1": "Raspberry Pi", "3A3541": "Raspberry Pi", "B827EB": "Raspberry Pi",
		"D83ADD": "Raspberry Pi", "E45F01": "Raspberry Pi",

		// Ubiquiti
		"00156D": "Ubiquiti", "002722": "Ubiquiti", "0418D6": "Ubiquiti", "0418D7": "Ubiquiti",
		"24A43C": "Ubiquiti", "44D9E7": "Ubiquiti", "687251": "Ubiquiti", "706582": "Ubiquiti",
		"788A20": "Ubiquiti", "802AA8": "Ubiquiti", "8DE204": "Ubiquiti", "902106": "Ubiquiti",
		"B4FBE4": "Ubiquiti", "D8B377": "Ubiquiti", "F09FC2": "Ubiquiti", "FCECDA": "Ubiquiti",

		// TP-Link
		"0019E0": "TP-Link", "002127": "TP-Link", "00236A": "TP-Link", "002586": "TP-Link",
		"10FEED": "TP-Link", "14CC20": "TP-Link", "18A6C7": "TP-Link", "18D6C7": "TP-Link",
		"30B5C2": "TP-Link", "349672": "TP-Link", "40169E": "TP-Link", "4432C8": "TP-Link",
		"50C7BF": "TP-Link", "54E6FC": "TP-Link", "60E327": "TP-Link", "6466B3": "TP-Link",
		"704F57": "TP-Link", "74EA3A": "TP-Link", "784476": "TP-Link", "8416F9": "TP-Link",
		"8C210A": "TP-Link", "90F652": "TP-Link", "94103E": "TP-Link", "98DED0": "TP-Link",
		"A42BB0": "TP-Link", "B0487A": "TP-Link", "BC4699": "TP-Link", "C025E9": "TP-Link",
		"C04A00": "TP-Link", "D46E0E": "TP-Link", "D84732": "TP-Link", "E894F6": "TP-Link",
		"EC086B": "TP-Link", "F4F26D": "TP-Link", "F81A67": "TP-Link", "F8D111": "TP-Link",
	}

	if vendor, exists := vendors[oui]; exists {
		return vendor
	}

	return "Unknown"
}

// classifyDevice classifies a device based on its characteristics
func (dd *DeviceDiscovery) classifyDevice(device *types.DeviceInfo) string {
	vendor := strings.ToLower(device.Vendor)
	hostname := strings.ToLower(device.HostName)

	// DHCP options/vendor class heuristics
	vendorClass := ""
	if vc, ok := device.DHCPOptions["vendor_class"].(string); ok {
		vendorClass = strings.ToLower(vc)
	}

	// Mobile devices
	if strings.Contains(vendor, "apple") || strings.Contains(vendor, "samsung") ||
		strings.Contains(hostname, "iphone") || strings.Contains(hostname, "ipad") ||
		strings.Contains(hostname, "android") || strings.Contains(vendorClass, "android") {
		return "Mobile"
	}

	// Network equipment
	if strings.Contains(vendor, "cisco") || strings.Contains(vendor, "netgear") ||
		strings.Contains(vendor, "linksys") || strings.Contains(vendor, "ubiquiti") ||
		strings.Contains(vendor, "tp-link") || strings.Contains(hostname, "router") ||
		strings.Contains(hostname, "gateway") || strings.Contains(hostname, "ap-") ||
		strings.Contains(vendorClass, "ubnt") {
		return "Network Equipment"
	}

	// Printers
	if strings.Contains(hostname, "printer") || strings.Contains(vendor, "hp") ||
		strings.Contains(vendor, "brother") || strings.Contains(vendor, "canon") ||
		strings.Contains(vendor, "epson") || strings.Contains(vendor, "lexmark") ||
		strings.Contains(vendorClass, "ipp") {
		return "Printer"
	}

	// Smart Home / IoT
	if strings.Contains(hostname, "iot") || strings.Contains(hostname, "sensor") ||
		strings.Contains(hostname, "camera") || strings.Contains(hostname, "cam") ||
		strings.Contains(hostname, "smart") || strings.Contains(hostname, "light") ||
		strings.Contains(hostname, "plug") || strings.Contains(vendor, "raspberry pi") ||
		strings.Contains(vendor, "espressif") || strings.Contains(vendorClass, "esp8266") ||
		strings.Contains(vendorClass, "esp32") {
		return "IoT"
	}

	// Servers / Infrastructure
	if strings.Contains(hostname, "server") || strings.Contains(hostname, "nas") ||
		strings.Contains(hostname, "proxmox") || strings.Contains(hostname, "esxi") ||
		strings.Contains(vendor, "supermicro") || strings.Contains(vendor, "vmware") {
		return "Server"
	}

	// Workstations / Computers
	if strings.Contains(vendor, "dell") || strings.Contains(vendor, "hp") ||
		strings.Contains(vendor, "lenovo") || strings.Contains(vendor, "microsoft") ||
		strings.Contains(vendor, "intel") || strings.Contains(hostname, "pc") ||
		strings.Contains(hostname, "laptop") || strings.Contains(hostname, "desktop") {
		return "Computer"
	}

	return "Unknown"
}

// cleanupWorker periodically cleans up expired devices
func (dd *DeviceDiscovery) cleanupWorker() {
	ticker := time.NewTicker(dd.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-dd.ctx.Done():
			return
		case <-ticker.C:
			dd.cleanupExpiredDevices()
		}
	}
}

// cleanupExpiredDevices removes expired devices from the database
func (dd *DeviceDiscovery) cleanupExpiredDevices() {
	dd.devMutex.Lock()
	defer dd.devMutex.Unlock()

	now := time.Now()
	expired := []string{}

	for key, device := range dd.devices {
		// Check if device has expired
		if now.Sub(device.LastSeen) > dd.config.DeviceTimeout {
			expired = append(expired, key)
		}
	}

	// Remove expired devices
	for _, key := range expired {
		delete(dd.devices, key)
		atomic.AddUint64(&dd.stats.DevicesExpired, 1)
		atomic.AddUint64(&dd.stats.DatabaseSize, ^uint64(0)) // Decrement
	}

	if len(expired) > 0 {
		dd.logger.Debug("Cleaned up expired devices", "count", len(expired))
	}
}

// sendAlert sends an alert
func (dd *DeviceDiscovery) sendAlert(alertType, message string) {
	// Note: Alerts are primarily handled by the Manager through handlers.
	// This method provides a fallback for internal discovery events.
	dd.logger.Info("Device discovery alert",
		"type", alertType,
		"message", message)
}

// SetNewDeviceHandler sets the new device handler
func (dd *DeviceDiscovery) SetNewDeviceHandler(handler NewDeviceHandler) {
	dd.mutex.Lock()
	defer dd.mutex.Unlock()
	dd.newDeviceHandler = handler
}

// SetDeviceUpdateHandler sets the device update handler
func (dd *DeviceDiscovery) SetDeviceUpdateHandler(handler DeviceUpdateHandler) {
	dd.mutex.Lock()
	defer dd.mutex.Unlock()
	dd.deviceUpdateHandler = handler
}

// GetStatistics returns device discovery statistics
func (dd *DeviceDiscovery) GetStatistics() *DeviceDiscoveryStats {
	dd.mutex.RLock()
	defer dd.mutex.RUnlock()

	stats := *dd.stats
	stats.DatabaseSize = uint64(len(dd.devices))
	return &stats
}

// IsEnabled returns whether the discovery is enabled
func (dd *DeviceDiscovery) IsEnabled() bool {
	dd.mutex.RLock()
	defer dd.mutex.RUnlock()
	return dd.enabled
}

// GetDiscoveredDevices returns all discovered devices
func (dd *DeviceDiscovery) GetDiscoveredDevices() []types.DeviceInfo {
	dd.devMutex.RLock()
	defer dd.devMutex.RUnlock()

	devices := make([]types.DeviceInfo, 0, len(dd.devices))
	for _, device := range dd.devices {
		devices = append(devices, *device)
	}

	return devices
}

// GetDeviceByMAC returns device information by MAC address
func (dd *DeviceDiscovery) GetDeviceByMAC(macAddr string) (*types.DeviceInfo, bool) {
	dd.devMutex.RLock()
	defer dd.devMutex.RUnlock()

	device, exists := dd.devices[strings.ToLower(macAddr)]
	return device, exists
}

// GetDeviceByIP returns device information by IP address
func (dd *DeviceDiscovery) GetDeviceByIP(ip net.IP) (*types.DeviceInfo, bool) {
	dd.devMutex.RLock()
	defer dd.devMutex.RUnlock()

	for _, device := range dd.devices {
		if device.IPAddress != nil && device.IPAddress.Equal(ip) {
			return device, true
		}
	}

	return nil, false
}

// SearchDevices searches devices by various criteria
func (dd *DeviceDiscovery) SearchDevices(query string) []types.DeviceInfo {
	dd.devMutex.RLock()
	defer dd.devMutex.RUnlock()

	query = strings.ToLower(query)
	var results []types.DeviceInfo

	for _, device := range dd.devices {
		if strings.Contains(strings.ToLower(device.MACAddress), query) ||
			strings.Contains(strings.ToLower(device.HostName), query) ||
			strings.Contains(strings.ToLower(device.Vendor), query) ||
			strings.Contains(strings.ToLower(device.DeviceType), query) {
			results = append(results, *device)
		}
	}

	return results
}

// GetDevicesByType returns devices of a specific type
func (dd *DeviceDiscovery) GetDevicesByType(deviceType string) []types.DeviceInfo {
	dd.devMutex.RLock()
	defer dd.devMutex.RUnlock()

	var results []types.DeviceInfo
	for _, device := range dd.devices {
		if device.DeviceType == deviceType {
			results = append(results, *device)
		}
	}

	return results
}

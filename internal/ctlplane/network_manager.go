// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ctlplane

import (
	"fmt"
	"log"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/network"

	"runtime"
	"strconv"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// NetworkManager handles network interface configuration.
type NetworkManager struct {
	config *config.Config
	netLib network.NetworkManager
}

// NewNetworkManager creates a new network manager.
func NewNetworkManager(cfg *config.Config, netLib network.NetworkManager) *NetworkManager {
	return &NetworkManager{
		config: cfg,
		netLib: netLib,
	}
}

// UpdateConfig updates the configuration reference.
func (nm *NetworkManager) UpdateConfig(cfg *config.Config) {
	nm.config = cfg
}

// GetInterfaces returns the status of interfaces based on the provided configuration.
func (nm *NetworkManager) GetInterfaces(cfg *config.Config) ([]InterfaceStatus, error) {
	var interfaces []InterfaceStatus

	// Try to create ethtool manager for additional info
	linkMgr, err := NewLinkManager()
	if err != nil {
		log.Printf("[NM] Warning: ethtool not available: %v", err)
	}
	defer func() {
		if linkMgr != nil {
			linkMgr.Close()
		}
	}()

	for _, iface := range cfg.Interfaces {
		zoneName := iface.Zone
		// If zone is empty, try to find it in zone definitions (reverse lookup)
		if zoneName == "" {
			for _, z := range nm.config.Zones {
				for _, m := range z.Matches {
					if m.Interface == iface.Name {
						zoneName = z.Name
						break
					}
				}
				if zoneName != "" {
					break
				}
			}
		}

		status := InterfaceStatus{
			Name:           iface.Name,
			Type:           GetInterfaceType(iface.Name),
			Description:    iface.Description,
			Zone:           zoneName,
			ConfiguredIPv4: iface.IPv4,
			DHCPEnabled:    iface.DHCP,
			Gateway:        iface.Gateway,
			Disabled:       iface.Disabled,
		}

		// Check if interface is disabled in config
		if iface.Disabled {
			status.State = InterfaceStateDisabled
			status.AdminUp = false
			status.Carrier = false
			interfaces = append(interfaces, status)
			continue
		}

		// Try to get link from netlink
		link, err := netlink.LinkByName(iface.Name)
		if err != nil {
			// Interface configured but not found in system
			status.State = InterfaceStateMissing
			interfaces = append(interfaces, status)
			continue
		}

		attrs := link.Attrs()
		status.MAC = attrs.HardwareAddr.String()
		status.MTU = attrs.MTU
		status.AdminUp = attrs.Flags&unix.IFF_UP != 0

		// Read carrier from sysfs (more reliable than OperState for physical detection)
		carrier, _ := ReadCarrier(iface.Name)
		status.Carrier = carrier

		// Determine state based on admin status and carrier
		switch {
		case !status.AdminUp:
			status.State = InterfaceStateDown
		case !carrier && status.Type == "ethernet":
			status.State = InterfaceStateNoCarrier
		case attrs.OperState == netlink.OperUp:
			status.State = InterfaceStateUp
		default:
			status.State = InterfaceStateDown
		}

		// Get addresses
		addrs, err := netlink.AddrList(link, unix.AF_INET)
		if err == nil {
			for _, addr := range addrs {
				status.IPv4Addrs = append(status.IPv4Addrs, addr.IPNet.String())
			}
		}
		addrs, err = netlink.AddrList(link, unix.AF_INET6)
		if err == nil {
			for _, addr := range addrs {
				status.IPv6Addrs = append(status.IPv6Addrs, addr.IPNet.String())
			}
		}

		// Get interface stats from sysfs
		if stats, err := ReadInterfaceStats(iface.Name); err == nil {
			status.Stats = stats
		}

		// Handle bond-specific info
		if status.Type == "bond" {
			status.BondMode = ReadBondMode(iface.Name)

			// Get active slaves from system
			activeSlaves, _ := ReadBondActiveSlaves(iface.Name)
			status.BondActiveMembers = activeSlaves

			// Get configured members
			if iface.Bond != nil {
				status.BondMembers = iface.Bond.Interfaces

				// Check for missing members (degraded state)
				activeSet := make(map[string]bool)
				for _, s := range activeSlaves {
					activeSet[s] = true
				}
				for _, configured := range iface.Bond.Interfaces {
					if !activeSet[configured] {
						status.BondMissingMembers = append(status.BondMissingMembers, configured)
					}
				}

				// If we have missing members, mark as degraded
				if len(status.BondMissingMembers) > 0 && status.State == InterfaceStateUp {
					status.State = InterfaceStateDegraded
				}
			}
		}

		// Handle VLAN-specific info
		if status.Type == "vlan" {
			if parent, vlanID, err := ReadVLANInfo(iface.Name); err == nil {
				status.VLANParent = parent
				status.VLANID = vlanID
			}
		}

		// Get ethtool data if available
		// The ethtool wrapper handles virtual NIC detection and uses sysfs fallback
		if linkMgr != nil && (status.Type == "ethernet" || status.Type == "bond") {
			// Link info (speed, duplex, autoneg)
			if linkInfo, err := linkMgr.GetLinkInfo(iface.Name); err == nil {
				status.Speed = linkInfo.Speed
				status.Duplex = linkInfo.Duplex
				status.Autoneg = linkInfo.Autoneg
			}

			// Driver info
			if driverInfo, err := linkMgr.GetDriverInfo(iface.Name); err == nil {
				status.Driver = driverInfo.Driver
				status.DriverVersion = driverInfo.Version
				status.Firmware = driverInfo.Firmware
				status.BusInfo = driverInfo.BusInfo
			}

			// Offload features
			if offloads, err := linkMgr.GetOffloads(iface.Name); err == nil {
				status.Offloads = offloads
			}

			// Ring buffer settings
			if ringBuffer, err := linkMgr.GetRingBuffer(iface.Name); err == nil {
				status.RingBuffer = ringBuffer
			}

			// Coalesce settings
			if coalesce, err := linkMgr.GetCoalesce(iface.Name); err == nil {
				status.Coalesce = coalesce
			}
		}

		// Mock ethtool data for macOS (since we can't run ethtool)
		if runtime.GOOS == "darwin" && (status.Type == "ethernet" || status.Type == "bond") {
			status.Speed = 1000
			status.Duplex = "Full"
			status.Autoneg = true
			status.Stats = &InterfaceStats{
				RxBytes:   1024 * 1024 * 500, // 500 MB
				TxBytes:   1024 * 1024 * 120, // 120 MB
				RxPackets: 500000,
				TxPackets: 120000,
			}
		}

		interfaces = append(interfaces, status)
	}

	// Include WireGuard interfaces from VPN config
	if nm.config.VPN != nil {
		for _, wg := range nm.config.VPN.WireGuard {
			if !wg.Enabled {
				continue
			}

			ifaceName := wg.Interface
			if ifaceName == "" {
				ifaceName = "wg0"
			}

			status := InterfaceStatus{
				Name:           ifaceName,
				Type:           "wireguard",
				Description:    "WireGuard VPN: " + wg.Name,
				Zone:           wg.Zone,
				ConfiguredIPv4: wg.Address,
				// WireGuard typicaly doesn't use DHCP but handles IPs internally
			}

			// Try to get link from netlink
			link, err := netlink.LinkByName(ifaceName)
			if err != nil {
				status.State = InterfaceStateMissing
				interfaces = append(interfaces, status)
				continue
			} else {
				// Fill runtime info
				attrs := link.Attrs()
				status.MAC = attrs.HardwareAddr.String()
				status.MTU = attrs.MTU
				status.AdminUp = attrs.Flags&unix.IFF_UP != 0

				// WireGuard is virtual, so carrier detection is different
				// Usually if AdminUp, it is "Up" unless handshake fails (which we can't easily see here)
				// We'll treat AdminUp as Up for now
				if status.AdminUp {
					status.State = InterfaceStateUp
				} else {
					status.State = InterfaceStateDown
				}

				// Get addresses
				addrs, err := netlink.AddrList(link, unix.AF_INET)
				if err == nil {
					for _, addr := range addrs {
						status.IPv4Addrs = append(status.IPv4Addrs, addr.IPNet.String())
					}
				}
				addrs6, err := netlink.AddrList(link, unix.AF_INET6)
				if err == nil {
					for _, addr := range addrs6 {
						status.IPv6Addrs = append(status.IPv6Addrs, addr.IPNet.String())
					}
				}

				if stats, err := ReadInterfaceStats(ifaceName); err == nil {
					status.Stats = stats
				}
			}
			interfaces = append(interfaces, status)
		}
	}

	return interfaces, nil
}

// GetAvailableInterfaces returns all physical interfaces available for configuration.
func (nm *NetworkManager) GetAvailableInterfaces(cfg *config.Config) ([]AvailableInterface, error) {
	if runtime.GOOS == "darwin" {
		// Mock available interfaces for development on macOS
		mockInterfaces := []AvailableInterface{
			{Name: "eth0", MAC: "00:11:22:33:44:55", LinkUp: true, Driver: "e1000", Speed: "1000Mb/s"},
			{Name: "eth1", MAC: "00:11:22:33:44:56", LinkUp: true, Driver: "e1000", Speed: "1000Mb/s"},
			{Name: "eth2", MAC: "00:11:22:33:44:57", LinkUp: true, Driver: "e1000", Speed: "1000Mb/s"},
			{Name: "eth3", MAC: "00:11:22:33:44:58", LinkUp: true, Driver: "e1000", Speed: "1000Mb/s"},
			{Name: "eth4", MAC: "00:11:22:33:44:59", LinkUp: true, Driver: "e1000", Speed: "1000Mb/s"},
			{Name: "eth5", MAC: "00:11:22:33:44:60", LinkUp: false, Driver: "e1000", Speed: "1000Mb/s"},
		}

		// Calculate assigned status
		assigned := make(map[string]bool)
		bondMembers := make(map[string]string)
		for _, iface := range cfg.Interfaces {
			assigned[iface.Name] = true
			if iface.Bond != nil {
				for _, member := range iface.Bond.Interfaces {
					bondMembers[member] = iface.Name
				}
			}
		}

		for i := range mockInterfaces {
			name := mockInterfaces[i].Name
			mockInterfaces[i].Assigned = assigned[name]
			if bondName, ok := bondMembers[name]; ok {
				mockInterfaces[i].InBond = true
				mockInterfaces[i].BondName = bondName
			}
		}
		return mockInterfaces, nil
	}

	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w", err)
	}

	// Build a map of assigned interfaces from config
	assigned := make(map[string]bool)
	bondMembers := make(map[string]string) // interface -> bond name
	for _, iface := range cfg.Interfaces {
		assigned[iface.Name] = true
		if iface.Bond != nil {
			for _, member := range iface.Bond.Interfaces {
				bondMembers[member] = iface.Name
			}
		}
	}

	var interfaces []AvailableInterface
	for _, link := range links {
		name := link.Attrs().Name

		// Skip loopback and virtual interfaces
		if name == "lo" || IsVirtualInterface(name) {
			continue
		}

		iface := AvailableInterface{
			Name:     name,
			MAC:      link.Attrs().HardwareAddr.String(),
			LinkUp:   link.Attrs().OperState == netlink.OperUp,
			Assigned: assigned[name],
		}

		// Check if in a bond
		if bondName, ok := bondMembers[name]; ok {
			iface.InBond = true
			iface.BondName = bondName
		}

		// Get driver info
		iface.Driver = GetDriverName(name)
		iface.Speed = GetLinkSpeedString(name)

		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

// StageInterfaceUpdate updates an interface's configuration in the provided staged config.
func (nm *NetworkManager) StageInterfaceUpdate(cfg *config.Config, args *UpdateInterfaceArgs) error {
	auditLog("StageInterfaceUpdate", fmt.Sprintf("name=%s action=%s", args.Name, args.Action))
	if args.Name == "" {
		return fmt.Errorf("interface name is required")
	}

	// Find interface in config
	var ifaceIdx int = -1
	for i, iface := range cfg.Interfaces {
		if iface.Name == args.Name {
			ifaceIdx = i
			break
		}
	}

	switch args.Action {
	case ActionEnable:
		if ifaceIdx < 0 {
			cfg.Interfaces = append(cfg.Interfaces, config.Interface{Name: args.Name, Disabled: false})
		} else {
			cfg.Interfaces[ifaceIdx].Disabled = false
		}

	case ActionDisable:
		if ifaceIdx < 0 {
			cfg.Interfaces = append(cfg.Interfaces, config.Interface{Name: args.Name, Disabled: true})
		} else {
			cfg.Interfaces[ifaceIdx].Disabled = true
		}

	case ActionUpdate:
		if ifaceIdx < 0 {
			// Add new interface to config
			newIface := config.Interface{Name: args.Name}
			if args.Zone != nil {
				newIface.Zone = *args.Zone
			}
			if args.Description != nil {
				newIface.Description = *args.Description
			}
			if args.IPv4 != nil {
				newIface.IPv4 = args.IPv4
			}
			if args.DHCP != nil {
				newIface.DHCP = *args.DHCP
			}
			if args.MTU != nil {
				newIface.MTU = *args.MTU
			}
			if args.Disabled != nil {
				newIface.Disabled = *args.Disabled
			}
			cfg.Interfaces = append(cfg.Interfaces, newIface)
		} else {
			// Update existing interface
			if args.Zone != nil {
				cfg.Interfaces[ifaceIdx].Zone = *args.Zone
			}
			if args.Description != nil {
				cfg.Interfaces[ifaceIdx].Description = *args.Description
			}
			if args.IPv4 != nil {
				cfg.Interfaces[ifaceIdx].IPv4 = args.IPv4
			}
			if args.DHCP != nil {
				cfg.Interfaces[ifaceIdx].DHCP = *args.DHCP
			}
			if args.MTU != nil {
				cfg.Interfaces[ifaceIdx].MTU = *args.MTU
			}
			if args.Disabled != nil {
				cfg.Interfaces[ifaceIdx].Disabled = *args.Disabled
			}
			if args.Bond != nil && cfg.Interfaces[ifaceIdx].Bond != nil {
				if args.Bond.Mode != "" {
					cfg.Interfaces[ifaceIdx].Bond.Mode = args.Bond.Mode
				}
				if len(args.Bond.Interfaces) > 0 {
					cfg.Interfaces[ifaceIdx].Bond.Interfaces = args.Bond.Interfaces
				}
			}
		}

	case ActionDelete:
		if ifaceIdx < 0 {
			return fmt.Errorf("interface not in configuration")
		}
		// Referencial Integrity Check
		if err := nm.checkInterfaceDependencies(cfg, args.Name); err != nil {
			return err
		}
		// Remove from config
		cfg.Interfaces = append(cfg.Interfaces[:ifaceIdx], cfg.Interfaces[ifaceIdx+1:]...)

	default:
		return fmt.Errorf("unknown action: %s", args.Action)
	}

	return nil
}

// StageVLANCreate stages a VLAN interface creation in the provided staged config.
func (nm *NetworkManager) StageVLANCreate(cfg *config.Config, args *CreateVLANArgs) error {
	auditLog("StageVLANCreate", fmt.Sprintf("parent=%s vlan=%d", args.ParentInterface, args.VLANID))

	vlanIDStr := fmt.Sprintf("%d", args.VLANID)

	// Config: Add to config
	for i, iface := range cfg.Interfaces {
		if iface.Name == args.ParentInterface {
			// Check for duplicates
			found := false
			for j, v := range cfg.Interfaces[i].VLANs {
				if v.ID == vlanIDStr {
					log.Printf("[NM] Warning: VLAN %s.%s already exists in config, updating", args.ParentInterface, vlanIDStr)
					cfg.Interfaces[i].VLANs[j] = config.VLAN{
						ID:          vlanIDStr,
						Zone:        args.Zone,
						Description: args.Description,
						IPv4:        args.IPv4,
					}
					found = true
					break
				}
			}
			if !found {
				cfg.Interfaces[i].VLANs = append(cfg.Interfaces[i].VLANs, config.VLAN{
					ID:          vlanIDStr,
					Zone:        args.Zone,
					Description: args.Description,
					IPv4:        args.IPv4,
				})
			}
			return nil
		}
	}

	return fmt.Errorf("parent interface %s not found", args.ParentInterface)
}

// StageVLANDelete stages a VLAN interface deletion in the provided staged config.
func (nm *NetworkManager) StageVLANDelete(cfg *config.Config, interfaceName string) error {
	auditLog("StageVLANDelete", fmt.Sprintf("name=%s", interfaceName))

	found := false
	for i := range cfg.Interfaces {
		newVLANs := cfg.Interfaces[i].VLANs[:0]
		for _, vlan := range cfg.Interfaces[i].VLANs {
			expectedName := fmt.Sprintf("%s.%s", cfg.Interfaces[i].Name, vlan.ID)
			if expectedName != interfaceName {
				newVLANs = append(newVLANs, vlan)
			} else {
				found = true
			}
		}
		cfg.Interfaces[i].VLANs = newVLANs
		if found {
			return nil
		}
	}

	return fmt.Errorf("VLAN %s not found in config", interfaceName)
}

// StageBondCreate stages a bonded interface creation in the provided staged config.
func (nm *NetworkManager) StageBondCreate(cfg *config.Config, args *CreateBondArgs) error {
	auditLog("StageBondCreate", fmt.Sprintf("name=%s mode=%s", args.Name, args.Mode))

	// Validate members exist
	for _, member := range args.Interfaces {
		if _, err := netlink.LinkByName(member); err != nil {
			return fmt.Errorf("interface %s not found", member)
		}
	}

	// Check for duplicates
	duplicateIndex := -1
	for i, iface := range cfg.Interfaces {
		if iface.Name == args.Name {
			duplicateIndex = i
			break
		}
	}

	if duplicateIndex != -1 {
		cfg.Interfaces[duplicateIndex] = config.Interface{
			Name:        args.Name,
			Description: args.Description,
			Zone:        args.Zone,
			IPv4:        args.IPv4,
			DHCP:        args.DHCP,
			Bond: &config.Bond{
				Mode:       args.Mode,
				Interfaces: args.Interfaces,
			},
		}
	} else {
		cfg.Interfaces = append(cfg.Interfaces, config.Interface{
			Name:        args.Name,
			Description: args.Description,
			Zone:        args.Zone,
			IPv4:        args.IPv4,
			DHCP:        args.DHCP,
			Bond: &config.Bond{
				Mode:       args.Mode,
				Interfaces: args.Interfaces,
			},
		})
	}

	return nil
}

// StageBondDelete stages a bonded interface deletion in the provided staged config.
func (nm *NetworkManager) StageBondDelete(cfg *config.Config, name string) error {
	auditLog("StageBondDelete", fmt.Sprintf("name=%s", name))

	newInterfaces := cfg.Interfaces[:0]
	found := false
	for _, iface := range cfg.Interfaces {
		if iface.Name != name {
			newInterfaces = append(newInterfaces, iface)
		} else {
			found = true
		}
	}
	cfg.Interfaces = newInterfaces

	if !found {
		return fmt.Errorf("bond %s not found in config", name)
	}

	return nil
}

// ApplyConfig synchronizes the kernel state with the provided configuration.
func (nm *NetworkManager) ApplyConfig(newConfig *config.Config) error {
	auditLog("ApplyConfig", "Applying network configuration")

	linkMgr, err := NewLinkManager()
	if err != nil {
		return fmt.Errorf("failed to create link manager: %w", err)
	}
	defer linkMgr.Close()

	// Apply VRFs first so interfaces can be enslaved
	if nm.netLib != nil {
		if err := nm.netLib.ApplyVRFs(newConfig.VRFs); err != nil {
			log.Printf("[NM] Warning: failed to apply VRFs: %v", err)
			// Continue best-effort
		}
	}

	// 1. Identify deletions (In Running, but not in Staged)
	stagedNames := make(map[string]bool)
	for _, iface := range newConfig.Interfaces {
		stagedNames[iface.Name] = true
	}

	for _, iface := range nm.config.Interfaces {
		if !stagedNames[iface.Name] {
			// Found an interface that should be removed
			if iface.Bond != nil {
				linkMgr.DeleteBond(iface.Name)
			}
			// VLANs are handled as children of interfaces
			for _, vlan := range iface.VLANs {
				linkMgr.DeleteVLAN(fmt.Sprintf("%s.%s", iface.Name, vlan.ID))
			}
		} else {
			// Interface exists in both. Check for removed VLANs.
			// Find the new interface config to compare
			var newIfaceConfig *config.Interface
			for i := range newConfig.Interfaces {
				if newConfig.Interfaces[i].Name == iface.Name {
					newIfaceConfig = &newConfig.Interfaces[i]
					break
				}
			}

			if newIfaceConfig != nil {
				// Build set of new VLAN IDs
				newVLANs := make(map[string]bool)
				for _, v := range newIfaceConfig.VLANs {
					newVLANs[v.ID] = true
				}

				// Check if any old VLANs are missing in the new config
				for _, oldVLAN := range iface.VLANs {
					if !newVLANs[oldVLAN.ID] {
						// VLAN removed
						vlanName := fmt.Sprintf("%s.%s", iface.Name, oldVLAN.ID)
						log.Printf("[NM] Removing deleted VLAN %s", vlanName)
						if err := linkMgr.DeleteVLAN(vlanName); err != nil {
							log.Printf("[NM] Warning: failed to delete removed VLAN %s: %v", vlanName, err)
						}
					}
				}
			}
		}
	}

	// 2. Identify additions and updates
	for _, iface := range newConfig.Interfaces {
		if iface.Bond != nil {
			// Ensure bond exists
			linkMgr.CreateBond(&CreateBondArgs{
				Name:        iface.Name,
				Mode:        iface.Bond.Mode,
				Interfaces:  iface.Bond.Interfaces,
				Description: iface.Description,
				Zone:        iface.Zone,
				IPv4:        iface.IPv4,
				DHCP:        iface.DHCP,
			})
		}

		// Handle VLANs
		for _, vlan := range iface.VLANs {
			vlanID, _ := strconv.Atoi(vlan.ID)
			linkMgr.CreateVLAN(&CreateVLANArgs{
				ParentInterface: iface.Name,
				VLANID:          vlanID,
				Zone:            vlan.Zone,
				Description:     vlan.Description,
				IPv4:            vlan.IPv4,
			})
		}

		// Apply L3 config (IPs, DHCP, etc.)
		nm.applyInterfaceConfigWithConfig(iface.Name, newConfig)
	}

	// Update running config reference
	nm.config = newConfig
	return nil
}

// applyInterfaceConfigWithConfig applies configuration to a specific interface using a provided config object.
func (nm *NetworkManager) applyInterfaceConfigWithConfig(ifaceName string, cfg *config.Config) error {
	var ifaceCfg *config.Interface
	for i := range cfg.Interfaces {
		if cfg.Interfaces[i].Name == ifaceName {
			ifaceCfg = &cfg.Interfaces[i]
			break
		}
	}
	if ifaceCfg == nil {
		return fmt.Errorf("interface %s not in config", ifaceName)
	}

	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		if runtime.GOOS == "darwin" {
			return nil
		}
		return err
	}

	// Set MTU if specified
	if ifaceCfg.MTU > 0 {
		netlink.LinkSetMTU(link, ifaceCfg.MTU)
	}

	// Enslave to VRF if configured
	if ifaceCfg.VRF != "" {
		vrfLink, err := netlink.LinkByName(ifaceCfg.VRF)
		if err != nil {
			log.Printf("[NM] Warning: VRF %s not found for interface %s: %v", ifaceCfg.VRF, ifaceName, err)
		} else {
			if err := netlink.LinkSetMaster(link, vrfLink); err != nil {
				log.Printf("[NM] Warning: failed to enslave %s to VRF %s: %v", ifaceName, ifaceCfg.VRF, err)
			}
		}
	}

	// Flush existing addresses
	addrs, _ := netlink.AddrList(link, unix.AF_INET)
	for _, addr := range addrs {
		netlink.AddrDel(link, &addr)
	}

	// Add configured addresses
	if !ifaceCfg.DHCP {
		for _, ipStr := range ifaceCfg.IPv4 {
			addr, err := netlink.ParseAddr(ipStr)
			if err != nil {
				continue
			}
			netlink.AddrAdd(link, addr)
		}
		network.StopDHCPClient(ifaceName)
	} else {
		network.StartDHCPClient(ifaceName)
	}

	// Handle Admin State
	if ifaceCfg.Disabled {
		netlink.LinkSetDown(link)
	} else {
		netlink.LinkSetUp(link)
	}

	return nil
}

// SnapshotInterfaces returns a deep copy of the current interface configuration.
func (nm *NetworkManager) SnapshotInterfaces() ([]config.Interface, error) {
	clone := nm.config.Clone()
	return clone.Interfaces, nil
}

// UpdateInterface is a shim for the NetworkConfigurator interface, providing immediate application.
func (nm *NetworkManager) UpdateInterface(args *UpdateInterfaceArgs) error {
	staged := nm.config.Clone()
	if err := nm.StageInterfaceUpdate(staged, args); err != nil {
		return err
	}
	return nm.ApplyConfig(staged)
}

// RestoreInterfaces restores the interface configuration from a snapshot.
func (nm *NetworkManager) RestoreInterfaces(snapshot []config.Interface) error {
	cfg := &config.Config{Interfaces: snapshot}
	return nm.ApplyConfig(cfg)
}

// checkInterfaceDependencies checks if an interface is used by other components
func (nm *NetworkManager) checkInterfaceDependencies(cfg *config.Config, ifaceName string) error {
	// 1. Check DHCP Scopes
	if cfg.DHCP != nil {
		for _, scope := range cfg.DHCP.Scopes {
			if scope.Interface == ifaceName {
				return fmt.Errorf("interface %s is used by DHCP scope '%s'", ifaceName, scope.Name)
			}
		}
	}

	// 2. Check Zones
	for _, zone := range cfg.Zones {
		// Simple match
		if zone.Interface == ifaceName {
			return fmt.Errorf("interface %s is used by zone '%s'", ifaceName, zone.Name)
		}
		// Complex matches
		for _, match := range zone.Matches {
			if match.Interface == ifaceName {
				return fmt.Errorf("interface %s is used by zone '%s' match rule", ifaceName, zone.Name)
			}
		}
	}

	// 3. Check mDNS
	if cfg.MDNS != nil {
		for _, iface := range cfg.MDNS.Interfaces {
			if iface == ifaceName {
				return fmt.Errorf("interface %s is used by mDNS service", ifaceName)
			}
		}
	}

	return nil
}

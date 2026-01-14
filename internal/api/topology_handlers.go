package api

import (
	"encoding/json"
	"net/http"
)

// handleGetTopology returns the discovered network topology (LLDP neighbors)
func (s *Server) handleGetTopology(w http.ResponseWriter, r *http.Request) {
	neighbors, err := s.client.GetTopology()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Enrich Graph Nodes with Sentinel data
	if s.sentinel != nil {
		for i := range neighbors.Graph.Nodes {
			node := &neighbors.Graph.Nodes[i]
			// Only enrich end devices, leave routers/switches if already classified
			if node.Type == "device" || node.Type == "" || node.Type == "unknown" {
				// We attempt to use ID as MAC (common in our topology builder) and Label as Hostname
				res := s.sentinel.Analyze(node.ID, node.Label)

				// Set Icon
				if node.Icon == "" {
					node.Icon = res.Icon
				}

				// Refine Type if we found a better category
				if (node.Type == "device" || node.Type == "unknown") && res.Category != "unknown" {
					node.Type = res.Category
				}

				// Add specific detail to description if present
				if res.Detail != "" {
					if node.Description != "" {
						node.Description += " (" + res.Detail + ")"
					} else {
						node.Description = res.Detail
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(neighbors)
}

// handleGetNetworkDevices returns all discovered devices on the network
// Query Parameters:
//
//	details=full: Include mDNS/DHCP profiling data (default: summary only)
func (s *Server) handleGetNetworkDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := s.client.GetNetworkDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Enrich with Sentinel data
	if s.sentinel != nil {
		for i := range devices {
			dev := &devices[i]
			res := s.sentinel.Analyze(dev.MAC, dev.Hostname)

			if dev.Vendor == "" && res.Vendor != "" {
				dev.Vendor = res.Vendor
			}
			if dev.DeviceType == "" && res.Category != "unknown" {
				dev.DeviceType = res.Category
			}
			if dev.DeviceModel == "" && res.Detail != "" {
				dev.DeviceModel = res.Detail
			}
		}
	}

	// Filter details if not requested
	if r.URL.Query().Get("details") != "full" {
		for i := range devices {
			devices[i].MDNSServices = nil
			devices[i].MDNSTXTRecords = nil
			devices[i].DHCPOptions = nil
			devices[i].DHCPFingerprint = ""
			devices[i].DHCPVendorClass = ""
			devices[i].DHCPClientID = ""
			// Keep Hostname, Vendor, DeviceType, DeviceModel as they are useful summary info
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"devices": devices,
	})
}

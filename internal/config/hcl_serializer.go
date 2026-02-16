// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

// Package config provides HCL serialization for Config struct to HCL.
package config

import (
	"fmt"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// SyncConfigToHCL updates the HCL AST to match the current Config struct.
// This preserves comments and formatting for unchanged sections.
func (cf *ConfigFile) SyncConfigToHCL() error {
	body := cf.hclFile.Body()
	cfg := cf.Config

	// Sync top-level attributes
	if cfg.SchemaVersion != "" {
		body.SetAttributeValue("schema_version", cty.StringVal(cfg.SchemaVersion))
	}
	body.SetAttributeValue("ip_forwarding", cty.BoolVal(cfg.IPForwarding))
	if cfg.MSSClamping {
		body.SetAttributeValue("mss_clamping", cty.BoolVal(cfg.MSSClamping))
	}
	if cfg.EnableFlowOffload {
		body.SetAttributeValue("enable_flow_offload", cty.BoolVal(cfg.EnableFlowOffload))
	}
	if cfg.StateDir != "" {
		body.SetAttributeValue("state_dir", cty.StringVal(cfg.StateDir))
	}

	// Sync config blocks
	if err := cf.syncFeatures(); err != nil {
		return fmt.Errorf("sync features: %w", err)
	}
	if err := cf.syncAPI(); err != nil {
		return fmt.Errorf("sync api: %w", err)
	}
	if err := cf.syncSystem(); err != nil {
		return fmt.Errorf("sync system: %w", err)
	}
	if err := cf.syncInterfaces(); err != nil {
		return fmt.Errorf("sync interfaces: %w", err)
	}
	if err := cf.syncPolicies(); err != nil {
		return fmt.Errorf("sync policies: %w", err)
	}
	if err := cf.syncZones(); err != nil {
		return fmt.Errorf("sync zones: %w", err)
	}
	if err := cf.syncNAT(); err != nil {
		return fmt.Errorf("sync nat: %w", err)
	}
	if err := cf.syncIPSets(); err != nil {
		return fmt.Errorf("sync ipsets: %w", err)
	}
	if err := cf.syncRoutes(); err != nil {
		return fmt.Errorf("sync routes: %w", err)
	}
	if err := cf.syncServices(); err != nil {
		return fmt.Errorf("sync services: %w", err)
	}
	if err := cf.syncIntegrations(); err != nil {
		return fmt.Errorf("sync integrations: %w", err)
	}
	if err := cf.syncQoSPolicies(); err != nil {
		return fmt.Errorf("sync qos policies: %w", err)
	}
	if err := cf.syncFRR(); err != nil {
		return fmt.Errorf("sync frr: %w", err)
	}

	return nil
}

// syncFRR synchronizes the frr block
func (cf *ConfigFile) syncFRR() error {
	body := cf.hclFile.Body()

	// Remove existing frr blocks
	for _, block := range body.Blocks() {
		if block.Type() == "frr" {
			body.RemoveBlock(block)
		}
	}

	if cf.Config.FRR == nil {
		return nil
	}

	frr := cf.Config.FRR
	block := body.AppendNewBlock("frr", nil)
	b := block.Body()

	if frr.Enabled {
		b.SetAttributeValue("enabled", cty.BoolVal(frr.Enabled))
	}

	if frr.OSPF != nil {
		ospfBlock := b.AppendNewBlock("ospf", nil)
		ob := ospfBlock.Body()

		if frr.OSPF.RouterID != "" {
			ob.SetAttributeValue("router_id", cty.StringVal(frr.OSPF.RouterID))
		}
		if len(frr.OSPF.Networks) > 0 {
			ob.SetAttributeValue("networks", toCtyStringList(frr.OSPF.Networks))
		}

		for _, area := range frr.OSPF.Areas {
			ab := ob.AppendNewBlock("area", []string{area.ID})
			abb := ab.Body()
			if len(area.Networks) > 0 {
				abb.SetAttributeValue("networks", toCtyStringList(area.Networks))
			}
		}
	}

	return nil
}

// syncQoSPolicies synchronizes qos_policy blocks
func (cf *ConfigFile) syncQoSPolicies() error {
	body := cf.hclFile.Body()

	// Remove all existing qos_policy blocks
	for _, block := range body.Blocks() {
		if block.Type() == "qos_policy" {
			body.RemoveBlock(block)
		}
	}

	// Add policies from config
	for _, pol := range cf.Config.QoSPolicies {
		block := body.AppendNewBlock("qos_policy", []string{pol.Name})
		b := block.Body()

		if pol.Interface != "" {
			b.SetAttributeValue("interface", cty.StringVal(pol.Interface))
		}
		if pol.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(pol.Enabled))
		}
		if pol.UploadMbps > 0 {
			b.SetAttributeValue("upload_mbps", cty.NumberIntVal(int64(pol.UploadMbps)))
		}
		if pol.DownloadMbps > 0 {
			b.SetAttributeValue("download_mbps", cty.NumberIntVal(int64(pol.DownloadMbps)))
		}

		// Classes
		for _, class := range pol.Classes {
			cb := b.AppendNewBlock("class", []string{class.Name})
			cbb := cb.Body()
			if class.Rate != "" {
				cbb.SetAttributeValue("rate", cty.StringVal(class.Rate))
			}
			if class.Ceil != "" {
				cbb.SetAttributeValue("ceil", cty.StringVal(class.Ceil))
			}
			if class.Priority > 0 {
				cbb.SetAttributeValue("priority", cty.NumberIntVal(int64(class.Priority)))
			}
		}

		// Rules
		for _, rule := range pol.Rules {
			rb := b.AppendNewBlock("rule", []string{rule.Name})
			rbb := rb.Body()
			rbb.SetAttributeValue("class", cty.StringVal(rule.Class))
			if rule.Protocol != "" {
				rbb.SetAttributeValue("proto", cty.StringVal(rule.Protocol))
			}
			if rule.DestPort > 0 {
				rbb.SetAttributeValue("dest_port", cty.NumberIntVal(int64(rule.DestPort)))
			}
			if rule.SrcIP != "" {
				rbb.SetAttributeValue("src_ip", cty.StringVal(rule.SrcIP))
			}
		}
	}

	return nil
}

// syncFeatures synchronizes the features block
func (cf *ConfigFile) syncFeatures() error {
	body := cf.hclFile.Body()
	// Remove existing
	for _, block := range body.Blocks() {
		if block.Type() == "features" {
			body.RemoveBlock(block)
		}
	}

	if cf.Config.Features == nil {
		return nil
	}

	block := body.AppendNewBlock("features", nil)
	b := block.Body()

	f := cf.Config.Features
	if f.ThreatIntel {
		b.SetAttributeValue("threat_intel", cty.BoolVal(true))
	}
	if f.NetworkLearning {
		b.SetAttributeValue("network_learning", cty.BoolVal(true))
	}
	if f.QoS {
		b.SetAttributeValue("qos", cty.BoolVal(true))
	}
	if f.IntegrityMonitoring {
		b.SetAttributeValue("integrity_monitoring", cty.BoolVal(true))
	}
	return nil
}

// syncAPI synchronizes the api block
func (cf *ConfigFile) syncAPI() error {
	body := cf.hclFile.Body()

	// Remove existing api blocks
	for _, block := range body.Blocks() {
		if block.Type() == "api" {
			body.RemoveBlock(block)
		}
	}

	if cf.Config.API == nil {
		return nil
	}

	api := cf.Config.API
	block := body.AppendNewBlock("api", nil)
	b := block.Body()

	if api.Enabled {
		b.SetAttributeValue("enabled", cty.BoolVal(api.Enabled))
	}
	if api.Listen != "" {
		b.SetAttributeValue("listen", cty.StringVal(api.Listen))
	}
	if api.TLSListen != "" {
		b.SetAttributeValue("tls_listen", cty.StringVal(api.TLSListen))
	}
	if api.TLSCert != "" {
		b.SetAttributeValue("tls_cert", cty.StringVal(api.TLSCert))
	}
	if api.TLSKey != "" {
		b.SetAttributeValue("tls_key", cty.StringVal(api.TLSKey))
	}
	if api.DisableHTTPRedirect {
		b.SetAttributeValue("disable_http_redirect", cty.BoolVal(api.DisableHTTPRedirect))
	}
	if api.DisableSandbox {
		b.SetAttributeValue("disable_sandbox", cty.BoolVal(api.DisableSandbox))
	}
	if api.RequireAuth {
		b.SetAttributeValue("require_auth", cty.BoolVal(api.RequireAuth))
	}
	if api.BootstrapKey != "" {
		b.SetAttributeValue("bootstrap_key", cty.StringVal(api.BootstrapKey))
	}
	if api.KeyStorePath != "" {
		b.SetAttributeValue("key_store_path", cty.StringVal(api.KeyStorePath))
	}
	if len(api.CORSOrigins) > 0 {
		b.SetAttributeValue("cors_origins", toCtyStringList(api.CORSOrigins))
	}

	// Sync API Keys
	for _, key := range api.Keys {
		kBlock := b.AppendNewBlock("key", []string{key.Name})
		kb := kBlock.Body()
		kb.SetAttributeValue("key", cty.StringVal(key.Key))
		kb.SetAttributeValue("permissions", toCtyStringList(key.Permissions))
		if len(key.AllowedIPs) > 0 {
			kb.SetAttributeValue("allowed_ips", toCtyStringList(key.AllowedIPs))
		}
		if len(key.AllowedPaths) > 0 {
			kb.SetAttributeValue("allowed_paths", toCtyStringList(key.AllowedPaths))
		}
		if key.RateLimit > 0 {
			kb.SetAttributeValue("rate_limit", cty.NumberIntVal(int64(key.RateLimit)))
		}
		if key.Enabled {
			kb.SetAttributeValue("enabled", cty.BoolVal(key.Enabled))
		}
		if key.Description != "" {
			kb.SetAttributeValue("description", cty.StringVal(key.Description))
		}
	}

	// Sync Let's Encrypt
	if api.LetsEncrypt != nil {
		le := api.LetsEncrypt
		leBlock := b.AppendNewBlock("letsencrypt", nil)
		leb := leBlock.Body()
		if le.Enabled {
			leb.SetAttributeValue("enabled", cty.BoolVal(le.Enabled))
		}
		if le.Email != "" {
			leb.SetAttributeValue("email", cty.StringVal(le.Email))
		}
		if le.Domain != "" {
			leb.SetAttributeValue("domain", cty.StringVal(le.Domain))
		}
		if le.CacheDir != "" {
			leb.SetAttributeValue("cache_dir", cty.StringVal(le.CacheDir))
		}
		if le.Staging {
			leb.SetAttributeValue("staging", cty.BoolVal(le.Staging))
		}
	}

	return nil
}

// syncSystem synchronizes the system block
func (cf *ConfigFile) syncSystem() error {
	body := cf.hclFile.Body()
	// Remove existing
	for _, block := range body.Blocks() {
		if block.Type() == "system" {
			body.RemoveBlock(block)
		}
	}

	if cf.Config.System == nil {
		return nil
	}

	sys := cf.Config.System
	block := body.AppendNewBlock("system", nil)
	b := block.Body()

	if sys.SysctlProfile != "" {
		b.SetAttributeValue("sysctl_profile", cty.StringVal(sys.SysctlProfile))
	}

	if len(sys.Sysctl) > 0 {
		// Map support
		m := make(map[string]cty.Value)
		for k, v := range sys.Sysctl {
			m[k] = cty.StringVal(v)
		}
		b.SetAttributeValue("sysctl", cty.ObjectVal(m))
	}

	return nil
}

// syncIPSets synchronizes ipset blocks
func (cf *ConfigFile) syncIPSets() error {
	body := cf.hclFile.Body()

	// Remove all existing ipset blocks
	for _, block := range body.Blocks() {
		if block.Type() == "ipset" {
			body.RemoveBlock(block)
		}
	}

	// Add ipsets from config
	for _, set := range cf.Config.IPSets {
		block := body.AppendNewBlock("ipset", []string{set.Name})
		blockBody := block.Body()

		if set.Type != "" {
			blockBody.SetAttributeValue("type", cty.StringVal(set.Type))
		}

		if set.Description != "" {
			blockBody.SetAttributeValue("description", cty.StringVal(set.Description))
		}

		if len(set.Entries) > 0 {
			blockBody.SetAttributeValue("entries", toCtyStringList(set.Entries))
		}

		if set.URL != "" {
			blockBody.SetAttributeValue("url", cty.StringVal(set.URL))
		}

		if set.FireHOLList != "" {
			blockBody.SetAttributeValue("firehol_list", cty.StringVal(set.FireHOLList))
		}
	}

	return nil
}

// syncRoutes synchronizes routing-related blocks
func (cf *ConfigFile) syncRoutes() error {
	body := cf.hclFile.Body()

	// Clear existing blocks
	for _, block := range body.Blocks() {
		switch block.Type() {
		case "route", "routing_table", "policy_route", "mark_rule", "uid_routing":
			body.RemoveBlock(block)
		}
	}

	// Sync Routes (static)
	for _, route := range cf.Config.Routes {
		block := body.AppendNewBlock("route", []string{route.Destination})
		b := block.Body()
		if route.Name != "" {
			b.SetAttributeValue("name", cty.StringVal(route.Name))
		}
		if route.Gateway != "" {
			b.SetAttributeValue("gateway", cty.StringVal(route.Gateway))
		}
		if route.Interface != "" {
			b.SetAttributeValue("interface", cty.StringVal(route.Interface))
		}
		if route.MonitorIP != "" {
			b.SetAttributeValue("monitor_ip", cty.StringVal(route.MonitorIP))
		}
		if route.Table > 0 {
			b.SetAttributeValue("table", cty.NumberIntVal(int64(route.Table)))
		}
		if route.Metric > 0 {
			b.SetAttributeValue("metric", cty.NumberIntVal(int64(route.Metric)))
		}
	}

	// Sync RoutingTables
	for _, tbl := range cf.Config.RoutingTables {
		block := body.AppendNewBlock("routing_table", []string{tbl.Name})
		b := block.Body()
		b.SetAttributeValue("id", cty.NumberIntVal(int64(tbl.ID)))
		for _, r := range tbl.Routes {
			rb := b.AppendNewBlock("route", []string{r.Destination})
			rbb := rb.Body()
			if r.Gateway != "" {
				rbb.SetAttributeValue("gateway", cty.StringVal(r.Gateway))
			}
			// ... other route fields inside table if supported ...
		}
	}

	// Sync PolicyRoutes
	for _, pr := range cf.Config.PolicyRoutes {
		block := body.AppendNewBlock("policy_route", []string{pr.Name})
		b := block.Body()
		if pr.Priority > 0 {
			b.SetAttributeValue("priority", cty.NumberIntVal(int64(pr.Priority)))
		}
		if pr.Mark != "" {
			b.SetAttributeValue("mark", cty.StringVal(pr.Mark))
		}
		if pr.MarkMask != "" {
			b.SetAttributeValue("mark_mask", cty.StringVal(pr.MarkMask))
		}
		if pr.FromSource != "" {
			b.SetAttributeValue("from", cty.StringVal(pr.FromSource))
		}
		if pr.To != "" {
			b.SetAttributeValue("to", cty.StringVal(pr.To))
		}
		if pr.IIF != "" {
			b.SetAttributeValue("iif", cty.StringVal(pr.IIF))
		}
		if pr.OIF != "" {
			b.SetAttributeValue("oif", cty.StringVal(pr.OIF))
		}
		if pr.Table > 0 {
			b.SetAttributeValue("table", cty.NumberIntVal(int64(pr.Table)))
		}
		if pr.Blackhole {
			b.SetAttributeValue("blackhole", cty.BoolVal(pr.Blackhole))
		}
		if pr.Prohibit {
			b.SetAttributeValue("prohibit", cty.BoolVal(pr.Prohibit))
		}
		if pr.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(pr.Enabled))
		}
		if pr.Comment != "" {
			b.SetAttributeValue("comment", cty.StringVal(pr.Comment))
		}
	}

	// Sync MarkRules
	for _, mr := range cf.Config.MarkRules {
		block := body.AppendNewBlock("mark_rule", []string{mr.Name})
		b := block.Body()
		b.SetAttributeValue("mark", cty.StringVal(mr.Mark))
		if mr.Mask != "" {
			b.SetAttributeValue("mask", cty.StringVal(mr.Mask))
		}
		if mr.Protocol != "" {
			b.SetAttributeValue("proto", cty.StringVal(mr.Protocol))
		}
		if mr.SrcIP != "" {
			b.SetAttributeValue("src_ip", cty.StringVal(mr.SrcIP))
		}
		if mr.DstIP != "" {
			b.SetAttributeValue("dst_ip", cty.StringVal(mr.DstIP))
		}
		if mr.InInterface != "" {
			b.SetAttributeValue("in_interface", cty.StringVal(mr.InInterface))
		}
		if mr.IPSet != "" {
			b.SetAttributeValue("ipset", cty.StringVal(mr.IPSet))
		}
		// ... add other fields like ports/zones/conn_state if needed ...
	}

	// Sync UIDRouting
	for _, ur := range cf.Config.UIDRouting {
		block := body.AppendNewBlock("uid_routing", []string{ur.Name})
		b := block.Body()
		if ur.UID > 0 {
			b.SetAttributeValue("uid", cty.NumberIntVal(int64(ur.UID)))
		}
		if ur.Username != "" {
			b.SetAttributeValue("username", cty.StringVal(ur.Username))
		}
		b.SetAttributeValue("uplink", cty.StringVal(ur.Uplink))
		b.SetAttributeValue("vpn_link", cty.StringVal(ur.VPNLink))
		if ur.Interface != "" {
			b.SetAttributeValue("interface", cty.StringVal(ur.Interface))
		}
		if ur.SNATIP != "" {
			b.SetAttributeValue("snat_ip", cty.StringVal(ur.SNATIP))
		}
		if ur.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(ur.Enabled))
		}
	}

	return nil
}

// syncServices synchronizes service blocks
func (cf *ConfigFile) syncServices() error {
	body := cf.hclFile.Body()

	// Clear existing blocks
	for _, block := range body.Blocks() {
		switch block.Type() {
		case "dhcp", "dns", "dns_server", "mdns", "upnp", "ntp", "syslog", "ddns":
			body.RemoveBlock(block)
		}
	}

	// DHCP
	if cf.Config.DHCP != nil {
		dhcp := cf.Config.DHCP
		block := body.AppendNewBlock("dhcp", nil)
		b := block.Body()
		if dhcp.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(dhcp.Enabled))
		}
		if dhcp.Mode != "" {
			b.SetAttributeValue("mode", cty.StringVal(dhcp.Mode))
		}
		// Scopes
		for _, scope := range dhcp.Scopes {
			sb := b.AppendNewBlock("scope", []string{scope.Name})
			sbb := sb.Body()
			sbb.SetAttributeValue("interface", cty.StringVal(scope.Interface))
			sbb.SetAttributeValue("range_start", cty.StringVal(scope.RangeStart))
			sbb.SetAttributeValue("range_end", cty.StringVal(scope.RangeEnd))
			sbb.SetAttributeValue("router", cty.StringVal(scope.Router))
			if len(scope.DNS) > 0 {
				sbb.SetAttributeValue("dns", toCtyStringList(scope.DNS))
			}
			if scope.Domain != "" {
				sbb.SetAttributeValue("domain", cty.StringVal(scope.Domain))
			}
			if scope.LeaseTime != "" {
				sbb.SetAttributeValue("lease_time", cty.StringVal(scope.LeaseTime))
			}
			// Reservations
			for _, res := range scope.Reservations {
				rb := sbb.AppendNewBlock("reservation", []string{res.MAC})
				rbb := rb.Body()
				rbb.SetAttributeValue("ip", cty.StringVal(res.IP))
				if res.Hostname != "" {
					rbb.SetAttributeValue("hostname", cty.StringVal(res.Hostname))
				}
				if res.Description != "" {
					rbb.SetAttributeValue("description", cty.StringVal(res.Description))
				}
			}
		}
	}

	// DNS (New consolidated config)
	if cf.Config.DNS != nil {
		dns := cf.Config.DNS
		block := body.AppendNewBlock("dns", nil)
		b := block.Body()
		if dns.Mode != "" {
			b.SetAttributeValue("mode", cty.StringVal(dns.Mode))
		}
		if len(dns.Forwarders) > 0 {
			b.SetAttributeValue("forwarders", toCtyStringList(dns.Forwarders))
		}
		if dns.DNSSEC {
			b.SetAttributeValue("dnssec", cty.BoolVal(dns.DNSSEC))
		}
	}

	// DNSServer (Deprecated but supported)
	if cf.Config.DNSServer != nil {
		dns := cf.Config.DNSServer
		block := body.AppendNewBlock("dns_server", nil)
		b := block.Body()
		if dns.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(dns.Enabled))
		}
		if len(dns.ListenOn) > 0 {
			b.SetAttributeValue("listen_on", toCtyStringList(dns.ListenOn))
		}
		if dns.Mode != "" {
			b.SetAttributeValue("mode", cty.StringVal(dns.Mode))
		}
		if len(dns.Forwarders) > 0 {
			b.SetAttributeValue("forwarders", toCtyStringList(dns.Forwarders))
		}

		for _, host := range dns.Hosts {
			hb := b.AppendNewBlock("host", []string{host.IP})
			hbb := hb.Body()
			hbb.SetAttributeValue("hostnames", toCtyStringList(host.Hostnames))
		}
	}

	// mDNS
	if cf.Config.MDNS != nil {
		block := body.AppendNewBlock("mdns", nil)
		b := block.Body()
		if cf.Config.MDNS.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(cf.Config.MDNS.Enabled))
		}
		if len(cf.Config.MDNS.Interfaces) > 0 {
			b.SetAttributeValue("interfaces", toCtyStringList(cf.Config.MDNS.Interfaces))
		}
	}

	// UPnP
	if cf.Config.UPnP != nil {
		upnp := cf.Config.UPnP
		block := body.AppendNewBlock("upnp", nil)
		b := block.Body()
		if upnp.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(upnp.Enabled))
		}
		if upnp.ExternalIntf != "" {
			b.SetAttributeValue("external_interface", cty.StringVal(upnp.ExternalIntf))
		}
		if len(upnp.InternalIntfs) > 0 {
			b.SetAttributeValue("internal_interfaces", toCtyStringList(upnp.InternalIntfs))
		}
		if upnp.SecureMode {
			b.SetAttributeValue("secure_mode", cty.BoolVal(upnp.SecureMode))
		}
	}

	// NTP
	if cf.Config.NTP != nil {
		ntp := cf.Config.NTP
		block := body.AppendNewBlock("ntp", nil)
		b := block.Body()
		if ntp.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(ntp.Enabled))
		}
		if len(ntp.Servers) > 0 {
			b.SetAttributeValue("servers", toCtyStringList(ntp.Servers))
		}
		if ntp.Interval != "" {
			b.SetAttributeValue("interval", cty.StringVal(ntp.Interval))
		}
	}

	// Syslog
	if cf.Config.Syslog != nil {
		sl := cf.Config.Syslog
		block := body.AppendNewBlock("syslog", nil)
		b := block.Body()
		if sl.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(sl.Enabled))
		}
		b.SetAttributeValue("host", cty.StringVal(sl.Host))
		if sl.Port > 0 {
			b.SetAttributeValue("port", cty.NumberIntVal(int64(sl.Port)))
		}
	}

	// DDNS
	if cf.Config.DDNS != nil {
		ddns := cf.Config.DDNS
		block := body.AppendNewBlock("ddns", nil)
		b := block.Body()
		if ddns.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(ddns.Enabled))
		}
		b.SetAttributeValue("provider", cty.StringVal(ddns.Provider))
		b.SetAttributeValue("hostname", cty.StringVal(ddns.Hostname))
		if ddns.Token != "" {
			b.SetAttributeValue("token", cty.StringVal(ddns.Token))
		}
	}

	return nil
}

// syncIntegrations synchronizes integration blocks
func (cf *ConfigFile) syncIntegrations() error {
	body := cf.hclFile.Body()

	// Clear existing blocks
	for _, block := range body.Blocks() {
		switch block.Type() {
		case "vpn", "replication", "multi_wan", "uplink_group", "rule_learning",
			"anomaly_detection", "notifications", "scheduler", "scheduled_rule", "syslog", "ddns":
			body.RemoveBlock(block)
		}
	}

	// VPN
	if cf.Config.VPN != nil {
		vpn := cf.Config.VPN
		block := body.AppendNewBlock("vpn", nil)
		b := block.Body()

		// WireGuard
		for _, wg := range vpn.WireGuard {
			wb := b.AppendNewBlock("wireguard", []string{wg.Name})
			wbb := wb.Body()
			wbb.SetAttributeValue("private_key", cty.StringVal(string(wg.PrivateKey)))
			if wg.ListenPort > 0 {
				wbb.SetAttributeValue("listen_port", cty.NumberIntVal(int64(wg.ListenPort)))
			}
			if len(wg.Address) > 0 {
				wbb.SetAttributeValue("address", toCtyStringList(wg.Address))
			}
			// Peers
			for _, peer := range wg.Peers {
				pb := wbb.AppendNewBlock("peer", []string{peer.PublicKey})
				pbb := pb.Body()
				if peer.Endpoint != "" {
					pbb.SetAttributeValue("endpoint", cty.StringVal(peer.Endpoint))
				}
				if len(peer.AllowedIPs) > 0 {
					pbb.SetAttributeValue("allowed_ips", toCtyStringList(peer.AllowedIPs))
				}
				if peer.PresharedKey != "" {
					pbb.SetAttributeValue("preshared_key", cty.StringVal(string(peer.PresharedKey)))
				}
			}
		}
		// Tailscale
		for _, ts := range vpn.Tailscale {
			tsb := b.AppendNewBlock("tailscale", []string{ts.Name})
			tsbb := tsb.Body()
			if ts.Enabled {
				tsbb.SetAttributeValue("enabled", cty.BoolVal(ts.Enabled))
			}
			if ts.AuthKey != "" {
				tsbb.SetAttributeValue("auth_key", cty.StringVal(string(ts.AuthKey)))
			}
		}
	}

	// Replication
	if cf.Config.Replication != nil {
		rep := cf.Config.Replication
		block := body.AppendNewBlock("replication", nil)
		b := block.Body()
		b.SetAttributeValue("mode", cty.StringVal(rep.Mode))
		if rep.ListenAddr != "" {
			b.SetAttributeValue("listen_addr", cty.StringVal(rep.ListenAddr))
		}
		if rep.PrimaryAddr != "" {
			b.SetAttributeValue("primary_addr", cty.StringVal(rep.PrimaryAddr))
		}
		if rep.SecretKey != "" {
			b.SetAttributeValue("secret_key", cty.StringVal(rep.SecretKey))
		}
	}

	// MultiWAN
	if cf.Config.MultiWAN != nil {
		mw := cf.Config.MultiWAN
		block := body.AppendNewBlock("multi_wan", nil)
		b := block.Body()
		if mw.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(mw.Enabled))
		}
		if mw.Mode != "" {
			b.SetAttributeValue("mode", cty.StringVal(mw.Mode))
		}
		for _, conn := range mw.Connections {
			cb := b.AppendNewBlock("wan", []string{conn.Name})
			cbb := cb.Body()
			cbb.SetAttributeValue("interface", cty.StringVal(conn.Interface))
			cbb.SetAttributeValue("gateway", cty.StringVal(conn.Gateway))
			if conn.Weight > 0 {
				cbb.SetAttributeValue("weight", cty.NumberIntVal(int64(conn.Weight)))
			}
			// ...
		}
	}

	// UplinkGroups
	for _, ug := range cf.Config.UplinkGroups {
		block := body.AppendNewBlock("uplink_group", []string{ug.Name})
		b := block.Body()
		if len(ug.SourceNetworks) > 0 {
			b.SetAttributeValue("source_networks", toCtyStringList(ug.SourceNetworks))
		}
		for _, up := range ug.Uplinks {
			ub := b.AppendNewBlock("uplink", []string{up.Name})
			ubb := ub.Body()
			ubb.SetAttributeValue("interface", cty.StringVal(up.Interface))
			if up.Gateway != "" {
				ubb.SetAttributeValue("gateway", cty.StringVal(up.Gateway))
			}
			// ...
		}
	}

	// RuleLearning
	if cf.Config.RuleLearning != nil {
		rl := cf.Config.RuleLearning
		block := body.AppendNewBlock("rule_learning", nil)
		b := block.Body()
		if rl.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(rl.Enabled))
		}
		if rl.LearningMode {
			b.SetAttributeValue("learning_mode", cty.BoolVal(rl.LearningMode))
		}
	}

	// AnomalyConfig
	if cf.Config.AnomalyConfig != nil {
		ac := cf.Config.AnomalyConfig
		block := body.AppendNewBlock("anomaly_detection", nil)
		b := block.Body()
		if ac.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(ac.Enabled))
		}
	}

	// Notifications
	if cf.Config.Notifications != nil {
		nc := cf.Config.Notifications
		block := body.AppendNewBlock("notifications", nil)
		b := block.Body()
		if nc.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(nc.Enabled))
		}
		for _, ch := range nc.Channels {
			cb := b.AppendNewBlock("channel", []string{ch.Name})
			cbb := cb.Body()
			cbb.SetAttributeValue("type", cty.StringVal(ch.Type))
			if ch.WebhookURL != "" {
				cbb.SetAttributeValue("webhook_url", cty.StringVal(ch.WebhookURL))
			}
			// ... other channel fields
		}
	}

	// Scheduler
	if cf.Config.Scheduler != nil {
		sc := cf.Config.Scheduler
		block := body.AppendNewBlock("scheduler", nil)
		b := block.Body()
		if sc.Enabled {
			b.SetAttributeValue("enabled", cty.BoolVal(sc.Enabled))
		}
		if sc.BackupEnabled {
			b.SetAttributeValue("backup_enabled", cty.BoolVal(sc.BackupEnabled))
		}
	}

	// ScheduledRules
	for _, sr := range cf.Config.ScheduledRules {
		block := body.AppendNewBlock("scheduled_rule", []string{sr.Name})
		b := block.Body()
		b.SetAttributeValue("policy", cty.StringVal(sr.PolicyName))
		b.SetAttributeValue("schedule", cty.StringVal(sr.Schedule))
		if sr.EndSchedule != "" {
			b.SetAttributeValue("end_schedule", cty.StringVal(sr.EndSchedule))
		}
		// Rule block inside?
		// struct: Rule PolicyRule `hcl:"rule,block"`
		// We need to append rule block.
		cf.appendRuleBlock(b, &sr.Rule)
	}

	return nil
}

// syncPolicies synchronizes policy blocks to HCL
func (cf *ConfigFile) syncPolicies() error {
	body := cf.hclFile.Body()

	// Remove all existing policy blocks
	for _, block := range body.Blocks() {
		if block.Type() == "policy" {
			body.RemoveBlock(block)
		}
	}

	// Add policies from config
	for _, pol := range cf.Config.Policies {
		cf.appendPolicyBlock(body, &pol)
	}

	return nil
}

// appendPolicyBlock adds a policy block to the body
func (cf *ConfigFile) appendPolicyBlock(body *hclwrite.Body, pol *Policy) {
	// Policy uses From/To as labels
	block := body.AppendNewBlock("policy", []string{pol.From, pol.To})
	blockBody := block.Body()

	if pol.Name != "" {
		blockBody.SetAttributeValue("name", cty.StringVal(pol.Name))
	}
	if pol.Description != "" {
		blockBody.SetAttributeValue("description", cty.StringVal(pol.Description))
	}
	if pol.Priority != 0 {
		blockBody.SetAttributeValue("priority", cty.NumberIntVal(int64(pol.Priority)))
	}
	if pol.Disabled {
		blockBody.SetAttributeValue("disabled", cty.BoolVal(pol.Disabled))
	}
	if pol.Action != "" {
		blockBody.SetAttributeValue("action", cty.StringVal(pol.Action))
	}
	if pol.Masquerade != nil {
		blockBody.SetAttributeValue("masquerade", cty.BoolVal(*pol.Masquerade))
	}
	if pol.Log {
		blockBody.SetAttributeValue("log", cty.BoolVal(pol.Log))
	}
	if pol.LogPrefix != "" {
		blockBody.SetAttributeValue("log_prefix", cty.StringVal(pol.LogPrefix))
	}
	if pol.Inherits != "" {
		blockBody.SetAttributeValue("inherits", cty.StringVal(pol.Inherits))
	}

	// Add rules
	for _, rule := range pol.Rules {
		cf.appendRuleBlock(blockBody, &rule)
	}
}

// appendRuleBlock adds a rule block to a policy body
func (cf *ConfigFile) appendRuleBlock(body *hclwrite.Body, rule *PolicyRule) {
	block := body.AppendNewBlock("rule", []string{rule.Name})
	blockBody := block.Body()

	if rule.Description != "" {
		blockBody.SetAttributeValue("description", cty.StringVal(rule.Description))
	}
	if rule.Disabled {
		blockBody.SetAttributeValue("disabled", cty.BoolVal(rule.Disabled))
	}

	// Match conditions
	if rule.Protocol != "" {
		blockBody.SetAttributeValue("proto", cty.StringVal(rule.Protocol))
	}
	if rule.Service != "" {
		blockBody.SetAttributeValue("service", cty.StringVal(rule.Service))
	}
	if rule.DestPort != 0 {
		blockBody.SetAttributeValue("dest_port", cty.NumberIntVal(int64(rule.DestPort)))
	}
	if len(rule.DestPorts) > 0 {
		blockBody.SetAttributeValue("dest_ports", toCtyIntList(rule.DestPorts))
	}
	if rule.SrcPort != 0 {
		blockBody.SetAttributeValue("src_port", cty.NumberIntVal(int64(rule.SrcPort)))
	}
	if len(rule.SrcPorts) > 0 {
		blockBody.SetAttributeValue("src_ports", toCtyIntList(rule.SrcPorts))
	}
	if len(rule.Services) > 0 {
		blockBody.SetAttributeValue("services", toCtyStringList(rule.Services))
	}
	if rule.SrcIP != "" {
		blockBody.SetAttributeValue("src_ip", cty.StringVal(rule.SrcIP))
	}
	if rule.SrcIPSet != "" {
		blockBody.SetAttributeValue("src_ipset", cty.StringVal(rule.SrcIPSet))
	}
	if rule.DestIP != "" {
		blockBody.SetAttributeValue("dest_ip", cty.StringVal(rule.DestIP))
	}
	if rule.DestIPSet != "" {
		blockBody.SetAttributeValue("dest_ipset", cty.StringVal(rule.DestIPSet))
	}

	// Action (required)
	blockBody.SetAttributeValue("action", cty.StringVal(rule.Action))

	// Advanced match options
	if rule.InvertSrc {
		blockBody.SetAttributeValue("invert_src", cty.BoolVal(rule.InvertSrc))
	}
	if rule.InvertDest {
		blockBody.SetAttributeValue("invert_dest", cty.BoolVal(rule.InvertDest))
	}
	if rule.TCPFlags != "" {
		blockBody.SetAttributeValue("tcp_flags", cty.StringVal(rule.TCPFlags))
	}
	if rule.MaxConnections > 0 {
		blockBody.SetAttributeValue("max_connections", cty.NumberIntVal(int64(rule.MaxConnections)))
	}

	// Optional fields
	if rule.Log {
		blockBody.SetAttributeValue("log", cty.BoolVal(rule.Log))
	}
	if rule.Comment != "" {
		blockBody.SetAttributeValue("comment", cty.StringVal(rule.Comment))
	}
}

// syncZones synchronizes zone blocks to HCL
func (cf *ConfigFile) syncZones() error {
	body := cf.hclFile.Body()

	// Track which zones satisfy current config
	seen := make(map[string]bool)

	// Update or Add zones from config
	for _, zone := range cf.Config.Zones {
		seen[zone.Name] = true

		var block *hclwrite.Block
		// Find existing block
		for _, b := range body.Blocks() {
			if b.Type() == "zone" && len(b.Labels()) > 0 && b.Labels()[0] == zone.Name {
				block = b
				break
			}
		}

		if block == nil {
			block = body.AppendNewBlock("zone", []string{zone.Name})
		}

		blockBody := block.Body()

		if zone.Interface != "" {
			blockBody.SetAttributeValue("interface", cty.StringVal(zone.Interface))
		} else {
			blockBody.RemoveAttribute("interface")
		}

		// Never output deprecated interfaces field
		blockBody.RemoveAttribute("interfaces")

		if zone.Description != "" {
			blockBody.SetAttributeValue("description", cty.StringVal(zone.Description))
		} else {
			blockBody.RemoveAttribute("description")
		}

		if zone.External != nil {
			blockBody.SetAttributeValue("external", cty.BoolVal(*zone.External))
		} else {
			blockBody.RemoveAttribute("external")
		}

		// Remove existing match blocks
		for _, b := range blockBody.Blocks() {
			if b.Type() == "match" {
				blockBody.RemoveBlock(b)
			}
		}

		// Sync Matches
		for _, match := range zone.Matches {
			mb := blockBody.AppendNewBlock("match", nil)
			mbb := mb.Body()
			if match.Interface != "" {
				mbb.SetAttributeValue("interface", cty.StringVal(match.Interface))
			}
			if match.Src != "" {
				mbb.SetAttributeValue("src", cty.StringVal(match.Src))
			}
			if match.Dst != "" {
				mbb.SetAttributeValue("dst", cty.StringVal(match.Dst))
			}
			if match.VLAN > 0 {
				mbb.SetAttributeValue("vlan", cty.NumberIntVal(int64(match.VLAN)))
			}
		}

		// Zone Management (Block)
		if zone.Management != nil {
			// Find existing management block inside zone
			var mgmtBlock *hclwrite.Block
			for _, b := range blockBody.Blocks() {
				if b.Type() == "management" {
					mgmtBlock = b
					break
				}
			}
			if mgmtBlock == nil {
				mgmtBlock = blockBody.AppendNewBlock("management", nil)
			}
			mb := mgmtBlock.Body()
			mb.SetAttributeValue("ssh", cty.BoolVal(zone.Management.SSH))
			mb.SetAttributeValue("web", cty.BoolVal(zone.Management.Web))
			mb.SetAttributeValue("api", cty.BoolVal(zone.Management.API))
			mb.SetAttributeValue("icmp", cty.BoolVal(zone.Management.ICMP))
			mb.SetAttributeValue("snmp", cty.BoolVal(zone.Management.SNMP))
			mb.SetAttributeValue("syslog", cty.BoolVal(zone.Management.Syslog))
		} else {
			// Remove management block if it exists
			for _, b := range blockBody.Blocks() {
				if b.Type() == "management" {
					blockBody.RemoveBlock(b)
				}
			}
		}

		// Zone Services (Block)
		if zone.Services != nil {
			var svcBlock *hclwrite.Block
			for _, b := range blockBody.Blocks() {
				if b.Type() == "services" {
					svcBlock = b
					break
				}
			}
			if svcBlock == nil {
				svcBlock = blockBody.AppendNewBlock("services", nil)
			}
			sb := svcBlock.Body()
			sb.SetAttributeValue("dhcp", cty.BoolVal(zone.Services.DHCP))
			sb.SetAttributeValue("dns", cty.BoolVal(zone.Services.DNS))
			// ... add other services as needed
		} else {
			for _, b := range blockBody.Blocks() {
				if b.Type() == "services" {
					blockBody.RemoveBlock(b)
				}
			}
		}

	}

	// Remove zones not in config
	for _, block := range body.Blocks() {
		if block.Type() == "zone" {
			if len(block.Labels()) > 0 && !seen[block.Labels()[0]] {
				body.RemoveBlock(block)
			}
		}
	}

	return nil
}

// Exported wrapper for ctlplane
func (cf *ConfigFile) SyncZones() error {
	return cf.syncZones()
}
func (cf *ConfigFile) syncNAT() error {
	body := cf.hclFile.Body()

	// Remove all existing nat blocks
	for _, block := range body.Blocks() {
		if block.Type() == "nat" {
			body.RemoveBlock(block)
		}
	}

	// Add NAT rules from config
	for _, nat := range cf.Config.NAT {
		block := body.AppendNewBlock("nat", []string{nat.Name})
		blockBody := block.Body()

		// Type is required
		blockBody.SetAttributeValue("type", cty.StringVal(nat.Type))

		if nat.Description != "" {
			blockBody.SetAttributeValue("description", cty.StringVal(nat.Description))
		}
		if nat.Protocol != "" {
			blockBody.SetAttributeValue("proto", cty.StringVal(nat.Protocol))
		}
		if nat.OutInterface != "" {
			blockBody.SetAttributeValue("out_interface", cty.StringVal(nat.OutInterface))
		}
		if nat.InInterface != "" {
			blockBody.SetAttributeValue("in_interface", cty.StringVal(nat.InInterface))
		}
		if nat.SrcIP != "" {
			blockBody.SetAttributeValue("src_ip", cty.StringVal(nat.SrcIP))
		}
		if nat.DestIP != "" {
			blockBody.SetAttributeValue("dest_ip", cty.StringVal(nat.DestIP))
		}
		if nat.DestPort != "" {
			blockBody.SetAttributeValue("dest_port", cty.StringVal(nat.DestPort))
		}
		if nat.ToIP != "" {
			blockBody.SetAttributeValue("to_ip", cty.StringVal(nat.ToIP))
		}
		if nat.ToPort != "" {
			blockBody.SetAttributeValue("to_port", cty.StringVal(nat.ToPort))
		}
		if nat.SNATIP != "" {
			blockBody.SetAttributeValue("snat_ip", cty.StringVal(nat.SNATIP))
		}
		if nat.Hairpin {
			blockBody.SetAttributeValue("hairpin", cty.BoolVal(nat.Hairpin))
		}
	}

	return nil
}

// syncInterfaces synchronizes interface blocks to HCL
func (cf *ConfigFile) syncInterfaces() error {
	body := cf.hclFile.Body()

	// Track which interfaces satisfy current config
	seen := make(map[string]bool)

	// Update or Add interfaces from config
	for _, iface := range cf.Config.Interfaces {
		seen[iface.Name] = true

		var block *hclwrite.Block
		// Find existing block
		for _, b := range body.Blocks() {
			if b.Type() == "interface" && len(b.Labels()) > 0 && b.Labels()[0] == iface.Name {
				block = b
				break
			}
		}

		if block == nil {
			block = body.AppendNewBlock("interface", []string{iface.Name})
		}

		blockBody := block.Body()

		if iface.Description != "" {
			blockBody.SetAttributeValue("description", cty.StringVal(iface.Description))
		} else {
			blockBody.RemoveAttribute("description")
		}

		blockBody.SetAttributeValue("dhcp", cty.BoolVal(iface.DHCP))

		if len(iface.IPv4) > 0 {
			blockBody.SetAttributeValue("ipv4", toCtyStringList(iface.IPv4))
		} else {
			blockBody.RemoveAttribute("ipv4")
		}

		if len(iface.IPv6) > 0 {
			blockBody.SetAttributeValue("ipv6", toCtyStringList(iface.IPv6))
		} else {
			blockBody.RemoveAttribute("ipv6")
		}

		if iface.MTU > 0 {
			blockBody.SetAttributeValue("mtu", cty.NumberIntVal(int64(iface.MTU)))
		} else {
			blockBody.RemoveAttribute("mtu")
		}

		if iface.Zone != "" {
			blockBody.SetAttributeValue("zone", cty.StringVal(iface.Zone))
		} else {
			blockBody.RemoveAttribute("zone")
		}

		// Bond
		if iface.Bond != nil {
			// Find existing bond block or create new
			var bondBlock *hclwrite.Block
			for _, b := range blockBody.Blocks() {
				if b.Type() == "bond" {
					bondBlock = b
					break
				}
			}
			if bondBlock == nil {
				bondBlock = blockBody.AppendNewBlock("bond", nil)
			}
			bb := bondBlock.Body()
			if iface.Bond.Mode != "" {
				bb.SetAttributeValue("mode", cty.StringVal(iface.Bond.Mode))
			}
			if len(iface.Bond.Interfaces) > 0 {
				bb.SetAttributeValue("interfaces", toCtyStringList(iface.Bond.Interfaces))
			}
		} else {
			for _, b := range blockBody.Blocks() {
				if b.Type() == "bond" {
					blockBody.RemoveBlock(b)
				}
			}
		}

		// VLANs
		// Clear existing VLAN blocks to ensure order and correctness
		for _, b := range blockBody.Blocks() {
			if b.Type() == "vlan" {
				blockBody.RemoveBlock(b)
			}
		}
		for _, vlan := range iface.VLANs {
			vb := blockBody.AppendNewBlock("vlan", []string{vlan.ID})
			vbb := vb.Body()
			if vlan.Description != "" {
				vbb.SetAttributeValue("description", cty.StringVal(vlan.Description))
			}
			if vlan.Zone != "" {
				vbb.SetAttributeValue("zone", cty.StringVal(vlan.Zone))
			}
			if len(vlan.IPv4) > 0 {
				vbb.SetAttributeValue("ipv4", toCtyStringList(vlan.IPv4))
			}
			if len(vlan.IPv6) > 0 {
				vbb.SetAttributeValue("ipv6", toCtyStringList(vlan.IPv6))
			}
		}
	}

	// Remove interfaces not in config
	for _, block := range body.Blocks() {
		if block.Type() == "interface" {
			if len(block.Labels()) > 0 && !seen[block.Labels()[0]] {
				body.RemoveBlock(block)
			}
		}
	}

	return nil
}

// Exported wrapper for ctlplane
func (cf *ConfigFile) SyncInterfaces() error {
	return cf.syncInterfaces()
}

// toCtyStringList converts a []string to cty.Value list
func toCtyStringList(strs []string) cty.Value {
	if len(strs) == 0 {
		return cty.ListValEmpty(cty.String)
	}
	vals := make([]cty.Value, len(strs))
	for i, s := range strs {
		vals[i] = cty.StringVal(s)
	}
	return cty.ListVal(vals)
}

// toCtyIntList converts a []int to cty.Value list
func toCtyIntList(ints []int) cty.Value {
	if len(ints) == 0 {
		return cty.ListValEmpty(cty.Number)
	}
	vals := make([]cty.Value, len(ints))
	for i, n := range ints {
		vals[i] = cty.NumberIntVal(int64(n))
	}
	return cty.ListVal(vals)
}

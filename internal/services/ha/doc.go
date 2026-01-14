// Package ha provides high-availability failover between two Flywall nodes.
//
// # Overview
//
// The HA package implements active/passive failover with support for:
//   - Virtual IP (VIP) migration for LAN gateway addresses
//   - Virtual MAC (VMAC) migration for DHCP-assigned WAN interfaces
//   - Heartbeat-based peer health monitoring
//   - Automatic DHCP lease reclaim on failover
//
// # Architecture
//
// Two nodes are configured as primary and backup. The primary node:
//   - Holds all virtual IPs and virtual MACs
//   - Serves as the active router/firewall
//   - Sends heartbeat messages to backup
//
// The backup node:
//   - Monitors heartbeats from primary
//   - Maintains synchronized state via replication
//   - Takes over on primary failure
//
// # Failover Sequence
//
// When the backup detects primary failure:
//  1. Apply virtual MACs to local interfaces
//  2. Reclaim DHCP leases (if applicable)
//  3. Add virtual IPs to local interfaces
//  4. Transition to primary role
//  5. Start accepting traffic
//
// # Configuration
//
// See config.HAConfig for configuration options.
package ha

/**
 * Zones Aggregation Store
 *
 * Aggregates zone data from multiple sources:
 * - config.zones (zone definitions)
 * - config.interfaces (physical/virtual interfaces)
 * - config.dhcp (DHCP scopes per zone)
 * - leases (active DHCP leases for device counts)
 * - status (interface state, health)
 */

import { derived, get } from "svelte/store";
import { config, leases, status, networkDevices } from "./app";

// ============================================================================
// Types
// ============================================================================

export type ZoneType = "wan" | "lan" | "dmz" | "guest" | "mgmt";
export type StealthStatus = "dark" | "beacon" | "exposed";
export type LearningMode = "lockdown" | "tofu" | "approval" | null;

export interface ZoneDevice {
    ip: string;
    mac: string;
    hostname?: string;
    vendor?: string;
    anomalyScore?: number;
    isAnomalous?: boolean;
}

export interface OpenPort {
    port: number;
    service: string;
    protocol: "tcp" | "udp";
}

export interface AggregatedZone {
    // Identity
    name: string;
    type: ZoneType;
    interface: string;
    ip: string;
    ips: string[]; // Multiple IPs for WAN/aliases
    cidr?: string;

    // Status
    status: "active" | "connected" | "down" | "degraded";
    isWan: boolean;

    // WAN-specific
    stealthStatus: StealthStatus;
    pingEnabled: boolean;
    openPorts: OpenPort[];
    latencyMs?: number;

    // LAN-specific
    deviceCount: number;
    devices: ZoneDevice[];
    learningMode: LearningMode;

    // Services
    dhcpEnabled: boolean;
    dhcpScope?: { start: string; end: string };
    dnsEnabled: boolean;
    dnsMode?: "local" | "forward" | "off";

    // Policy
    ruleCount: number;
    natEnabled: boolean;
}

// ============================================================================
// Helper Functions
// ============================================================================

function detectZoneType(zone: any, iface: any): ZoneType {
    const name = (zone?.name || "").toLowerCase();
    const ifaceName = (iface?.Name || iface?.name || "").toLowerCase();

    if (name.includes("wan") || ifaceName.includes("wan") || ifaceName === "eth0") {
        return "wan";
    }
    if (name.includes("dmz")) return "dmz";
    if (name.includes("guest")) return "guest";
    if (name.includes("mgmt") || name.includes("management")) return "mgmt";
    return "lan";
}

function getZoneIPs(zone: any, iface: any): string[] {
    const ips: string[] = [];

    // From zone definition
    if (zone?.ip) ips.push(zone.ip);

    // From interface
    const ifaceIPs = iface?.IPv4 || iface?.ipv4 || [];
    for (const ip of ifaceIPs) {
        if (!ips.includes(ip)) ips.push(ip);
    }

    return ips;
}

function countDevicesInZone(zoneName: string, allLeases: any[]): number {
    return allLeases.filter((l) => l.zone === zoneName || l.interface === zoneName).length;
}

function getDevicesInZone(zoneName: string, allLeases: any[], allNetworkDevices: any[]): ZoneDevice[] {
    const deviceMap = new Map(allNetworkDevices.map(d => [d.mac, d]));

    return allLeases
        .filter((l) => l.zone === zoneName || l.interface === zoneName)
        .map((l) => {
            const mac = l.mac || l.hw_address;
            const netDev = deviceMap.get(mac);
            return {
                ip: l.ip || l.address,
                mac: mac,
                hostname: l.hostname || l.client_id,
                vendor: l.vendor,
                anomalyScore: netDev?.anomaly_score || 0,
                isAnomalous: netDev?.is_anomalous || false,
            };
        });
}

function detectStealthStatus(zone: any, policies: any[]): StealthStatus {
    // Get all accept policies for this zone
    const zonePolicies = policies.filter(
        (p) => p.destination_zone === zone.name && p.action === "accept"
    );

    // Check if any non-ICMP policies expose ports
    const hasExposedPorts = zonePolicies.some(
        (p) => p.protocol !== "icmp" && (p.destination_port || p.protocol === "tcp" || p.protocol === "udp")
    );

    if (hasExposedPorts) return "exposed";

    // Check if ICMP is allowed (beacon)
    const icmpAllowed = zonePolicies.some((p) => p.protocol === "icmp");

    if (icmpAllowed) return "beacon";
    return "dark";
}

function getOpenPorts(zone: any, policies: any[]): OpenPort[] {
    return policies
        .filter(
            (p) =>
                p.destination_zone === zone.name &&
                p.action === "accept" &&
                p.destination_port
        )
        .map((p) => ({
            port: parseInt(p.destination_port, 10),
            service: p.description || guessService(parseInt(p.destination_port, 10)),
            protocol: (p.protocol || "tcp") as "tcp" | "udp",
        }));
}

function guessService(port: number): string {
    const services: Record<number, string> = {
        22: "SSH",
        80: "HTTP",
        443: "HTTPS",
        53: "DNS",
        67: "DHCP",
        123: "NTP",
        51820: "WireGuard",
        1194: "OpenVPN",
    };
    return services[port] || `Port ${port}`;
}

// ============================================================================
// Derived Store
// ============================================================================

export const zones = derived(
    [config, leases, status, networkDevices],
    ([$config, $leases, $status, $networkDevices]): AggregatedZone[] => {
        if (!$config) return [];

        const configZones = $config.zones || [];
        const interfaces = $config.interfaces || [];
        const dhcpScopes = $config.dhcp?.scopes || [];
        const dnsConfig = $config.dns || {};
        const policies = $config.policies || [];

        const safeNetworkDevices = Array.isArray($networkDevices) ? $networkDevices : [];

        return configZones.map((zone: any) => {
            // Find matching interface
            const iface = interfaces.find(
                (i: any) => (i.Zone || i.zone) === zone.name || (i.Name || i.name) === zone.interface
            );

            const zoneType = detectZoneType(zone, iface);
            const isWan = zoneType === "wan";
            const ips = getZoneIPs(zone, iface);

            // DHCP scope for this zone
            const dhcpScope = dhcpScopes.find((s: any) => s.interface === zone.interface);

            // DNS enabled if zone has local DNS or forwarding
            const dnsEnabled = dnsConfig.enabled && (dnsConfig.zones || []).includes(zone.name);

            // Device count from leases
            const deviceCount = countDevicesInZone(zone.name, $leases);
            const devices = getDevicesInZone(zone.name, $leases, safeNetworkDevices);

            // WAN-specific
            const stealthStatus = isWan ? detectStealthStatus(zone, policies) : "dark";
            const openPorts = isWan ? getOpenPorts(zone, policies) : [];

            // Rule count for this zone
            const ruleCount = policies.filter(
                (p: any) => p.source_zone === zone.name || p.destination_zone === zone.name
            ).length;

            // Latency from monitors
            const monitor = ($status?.monitors || []).find(
                (m: any) => m.route_name === zone.name || m.target === zone.gateway || m.target === zone.ip
            );
            const latencyMs = monitor?.latency_ms;

            return {
                name: zone.name,
                type: zoneType,
                interface: zone.interface || iface?.Name || iface?.name || "",
                ip: zone.ip || ips[0] || "",
                ips,
                cidr: zone.cidr,
                status: isWan ? (monitor?.is_up ? "connected" : "down") : "active",
                isWan,
                stealthStatus,
                pingEnabled: stealthStatus === "beacon",
                openPorts,
                latencyMs,
                deviceCount,
                devices,
                learningMode: zone.learning_mode || null,
                dhcpEnabled: !!dhcpScope,
                dhcpScope: dhcpScope
                    ? { start: dhcpScope.range_start, end: dhcpScope.range_end }
                    : undefined,
                dnsEnabled,
                dnsMode: dnsEnabled ? "local" : "off",
                ruleCount,
                natEnabled: policies.some(
                    (p: any) => p.source_zone === zone.name && p.nat
                ),
            };
        });
    }
);

// Convenience selectors
export const wanZones = derived(zones, ($zones) =>
    $zones.filter((z) => z.isWan)
);

export const lanZones = derived(zones, ($zones) =>
    $zones.filter((z) => !z.isWan)
);

export const totalDevices = derived(zones, ($zones) =>
    $zones.reduce((sum, z) => sum + z.deviceCount, 0)
);

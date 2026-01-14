/**
 * Zones Store Tests
 * Tests for the zone aggregation store
 */

import { describe, it, expect, beforeEach } from "vitest";
import { get } from "svelte/store";
import { zones, wanZones, lanZones, totalDevices } from "$lib/stores/zones";
import { config, leases, status } from "$lib/stores/app";

describe("Zones Store", () => {
    beforeEach(() => {
        // Reset stores
        config.set(null);
        leases.set([]);
        status.set(null);
    });

    describe("initial state", () => {
        it("should return empty array when config is null", () => {
            expect(get(zones)).toEqual([]);
        });

        it("wanZones should be empty initially", () => {
            expect(get(wanZones)).toEqual([]);
        });

        it("lanZones should be empty initially", () => {
            expect(get(lanZones)).toEqual([]);
        });

        it("totalDevices should be 0 initially", () => {
            expect(get(totalDevices)).toBe(0);
        });
    });

    describe("zone aggregation", () => {
        it("should aggregate zones from config", () => {
            config.set({
                zones: [
                    { name: "LAN", ip: "192.168.1.1", interface: "vlan10" },
                    { name: "WAN", interface: "eth0" },
                ],
                interfaces: [],
                dhcp: { scopes: [] },
                policies: [],
            });

            const result = get(zones);
            expect(result).toHaveLength(2);
            expect(result[0].name).toBe("LAN");
            expect(result[1].name).toBe("WAN");
        });

        it("should detect WAN zones by name", () => {
            config.set({
                zones: [
                    { name: "WAN", interface: "eth0" },
                    { name: "LAN", interface: "vlan10" },
                ],
                interfaces: [],
                dhcp: { scopes: [] },
                policies: [],
            });

            expect(get(wanZones)).toHaveLength(1);
            expect(get(wanZones)[0].name).toBe("WAN");
            expect(get(lanZones)).toHaveLength(1);
            expect(get(lanZones)[0].name).toBe("LAN");
        });

        it("should merge IP from zone and interface", () => {
            config.set({
                zones: [{ name: "LAN", ip: "192.168.1.1", interface: "vlan10" }],
                interfaces: [{ Name: "vlan10", Zone: "LAN", IPv4: ["192.168.1.1/24"] }],
                dhcp: { scopes: [] },
                policies: [],
            });

            const result = get(zones);
            expect(result[0].ips).toContain("192.168.1.1");
            expect(result[0].ips).toContain("192.168.1.1/24");
        });

        it("should detect DHCP enabled from scope", () => {
            config.set({
                zones: [{ name: "LAN", interface: "vlan10" }],
                interfaces: [],
                dhcp: {
                    scopes: [
                        { interface: "vlan10", range_start: "192.168.1.100", range_end: "192.168.1.200" },
                    ],
                },
                policies: [],
            });

            const result = get(zones);
            expect(result[0].dhcpEnabled).toBe(true);
            expect(result[0].dhcpScope).toEqual({
                start: "192.168.1.100",
                end: "192.168.1.200",
            });
        });

        it("should count devices from leases", () => {
            config.set({
                zones: [{ name: "LAN", interface: "vlan10" }],
                interfaces: [],
                dhcp: { scopes: [] },
                policies: [],
            });

            leases.set([
                { zone: "LAN", ip: "192.168.1.10", mac: "AA:BB:CC:DD:EE:01" },
                { zone: "LAN", ip: "192.168.1.11", mac: "AA:BB:CC:DD:EE:02" },
                { zone: "Other", ip: "192.168.2.10", mac: "AA:BB:CC:DD:EE:03" },
            ]);

            const result = get(zones);
            expect(result[0].deviceCount).toBe(2);
            expect(get(totalDevices)).toBe(2);
        });
    });

    describe("WAN security status", () => {
        it("should default to dark when no policies expose ports", () => {
            config.set({
                zones: [{ name: "WAN", interface: "eth0" }],
                interfaces: [],
                dhcp: { scopes: [] },
                policies: [],
            });

            const result = get(zones);
            expect(result[0].stealthStatus).toBe("dark");
        });

        it("should be beacon when ICMP is allowed", () => {
            config.set({
                zones: [{ name: "WAN", interface: "eth0" }],
                interfaces: [],
                dhcp: { scopes: [] },
                policies: [{ destination_zone: "WAN", protocol: "icmp", action: "accept" }],
            });

            const result = get(zones);
            expect(result[0].stealthStatus).toBe("beacon");
        });

        it("should be exposed when ports are open", () => {
            config.set({
                zones: [{ name: "WAN", interface: "eth0" }],
                interfaces: [],
                dhcp: { scopes: [] },
                policies: [
                    { destination_zone: "WAN", destination_port: "443", action: "accept" },
                ],
            });

            const result = get(zones);
            expect(result[0].stealthStatus).toBe("exposed");
            expect(result[0].openPorts).toHaveLength(1);
            expect(result[0].openPorts[0].port).toBe(443);
        });
    });

    describe("rule counting", () => {
        it("should count rules for zone", () => {
            config.set({
                zones: [{ name: "LAN", interface: "vlan10" }],
                interfaces: [],
                dhcp: { scopes: [] },
                policies: [
                    { source_zone: "LAN", destination_zone: "WAN", action: "accept" },
                    { source_zone: "LAN", destination_zone: "DMZ", action: "accept" },
                    { source_zone: "WAN", destination_zone: "LAN", action: "drop" },
                ],
            });

            const result = get(zones);
            // LAN appears in 3 rules (2 as source, 1 as destination)
            expect(result[0].ruleCount).toBe(3);
        });
    });
});

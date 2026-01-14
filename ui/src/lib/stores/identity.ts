/**
 * Identity Derived Store
 * 
 * Enriches raw identity data with:
 * - Lease information (IP addresses, Hostnames)
 * - Group information (Name, Color)
 */

import { derived } from "svelte/store";
import { identities, groups, leases } from "./app";

export interface EnrichedIdentity {
    id: string;
    macs: string[];
    alias: string;
    owner: string;
    groupId: string;
    groupName?: string;
    groupColor?: string;
    tags: string[];
    firstSeen: string;
    lastSeen: string;

    // Derived
    ips: string[];
    hostnames: string[];
    vendors: string[];
    online: boolean; // Based on active lease or recent activity? For now, lease existence.
}

export const enrichedIdentities = derived(
    [identities, groups, leases],
    ([$identities, $groups, $leases]) => {
        if (!Array.isArray($identities)) return [];

        const groupMap = new Map($groups.map((g: any) => [g.id, g]));

        // Map MAC to Lease(s)
        const leaseMap = new Map<string, any[]>();
        if (Array.isArray($leases)) {
            for (const lease of $leases) {
                const mac = (lease.mac || lease.hw_address || "").toLowerCase();
                if (mac) {
                    if (!leaseMap.has(mac)) leaseMap.set(mac, []);
                    leaseMap.get(mac)?.push(lease);
                }
            }
        }

        return $identities.map((identity: any) => {
            const group = groupMap.get(identity.group_id);

            const ips = new Set<string>();
            const hostnames = new Set<string>();
            const vendors = new Set<string>();
            let online = false;

            // Check all MACs for leases
            for (const mac of identity.macs || []) {
                const macLower = mac.toLowerCase();
                const devLeases = leaseMap.get(macLower) || [];

                for (const l of devLeases) {
                    if (l.ip || l.address) ips.add(l.ip || l.address);
                    if (l.hostname) hostnames.add(l.hostname);
                    if (l.vendor) vendors.add(l.vendor);
                    // Assume lease existence means recently active? 
                    // Or check lease expiration? For simplicity, if in leases list, it's somewhat active.
                    online = true;
                }
            }

            return {
                id: identity.id,
                macs: identity.macs || [],
                alias: identity.alias || "",
                owner: identity.owner || "",
                groupId: identity.group_id || "",
                groupName: group?.name,
                groupColor: group?.color,
                tags: identity.tags || [],
                firstSeen: identity.first_seen,
                lastSeen: identity.last_seen,
                ips: Array.from(ips),
                hostnames: Array.from(hostnames),
                vendors: Array.from(vendors),
                online
            };
        });
    }
);

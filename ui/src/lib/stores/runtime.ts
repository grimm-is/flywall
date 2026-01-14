
/**
 * runtime.ts
 * 
 * Manages container runtime state (Docker).
 * Subscribes to 'runtime:containers' WebSocket topic.
 */

import { writable } from 'svelte/store';
import { subscribe, unsubscribe } from './websocket';
import { browser } from '$app/environment';

export interface Container {
    Id: string;
    Names: string[];
    Image: string;
    State: string;
    Status: string;
    NetworkSettings: {
        Networks: Record<string, NetworkEndpoint>;
    };
    Labels: Record<string, string>;
}

export interface NetworkEndpoint {
    IPAddress: string;
    Gateway: string;
    MacAddress: string;
    NetworkID: string;
    EndpointID: string;
}

// Active containers store
export const containers = writable<Container[]>([]);

// Derived store: Map of IP -> Container Name (for easy lookup)
// Useful for enriching flow logs
export const containerIPMap = writable<Record<string, string>>({});

// Initialize subscription
export function initRuntimeStore() {
    if (!browser) return;

    // Listen for WebSocket updates
    window.addEventListener('ws-runtime-containers', (event: any) => {
        const data = event.detail as Container[];
        // Sort by name for stability
        data.sort((a, b) => (a.Names[0] || '').localeCompare(b.Names[0] || ''));
        containers.set(data);

        // Update IP map
        const ipMap: Record<string, string> = {};
        for (const c of data) {
            const name = c.Names[0]?.replace(/^\//, '') || c.Id.slice(0, 12);
            for (const netName in c.NetworkSettings.Networks) {
                const endpoint = c.NetworkSettings.Networks[netName];
                if (endpoint.IPAddress) {
                    ipMap[endpoint.IPAddress] = name;
                }
            }
        }
        containerIPMap.set(ipMap);
    });

    // Subscribe to topic
    subscribe(['runtime:containers']);
}

// Cleanup
export function destroyRuntimeStore() {
    if (!browser) return;
    unsubscribe(['runtime:containers']);
}

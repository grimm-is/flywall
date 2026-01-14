/**
 * Flows Store - Connection/Flow tracking for Observatory
 *
 * Fetches from /api/flows and supports WebSocket updates via 'flows' topic
 */

import { writable, get } from "svelte/store";
import { subscribe as wsSubscribe, unsubscribe as wsUnsubscribe } from "./websocket";

// ============================================================================
// Types
// ============================================================================

export interface Flow {
    id: string;
    protocol: string;
    src_ip: string;
    src_port: number;
    dst_ip: string;
    dst_port: number;
    src_zone?: string;
    dst_zone?: string;
    src_hostname?: string;
    dst_hostname?: string;
    bytes_sent?: number;
    bytes_recv?: number;
    packets_sent?: number;
    packets_recv?: number;
    state?: string;
    age_seconds?: number;
    mark?: number;
}

export interface FlowsState {
    flows: Flow[];
    loading: boolean;
    error: string | null;
    lastUpdate: Date | null;
}

// ============================================================================
// Store
// ============================================================================

const initialState: FlowsState = {
    flows: [],
    loading: false,
    error: null,
    lastUpdate: null,
};

function createFlowsStore() {
    const { subscribe, set, update } = writable<FlowsState>(initialState);

    let pollInterval: ReturnType<typeof setInterval> | null = null;

    return {
        subscribe,

        /**
         * Fetch flows from API
         */
        async fetch() {
            update((s) => ({ ...s, loading: true, error: null }));

            try {
                const response = await fetch("/api/flows", {
                    credentials: "include",
                });

                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}`);
                }

                const data = await response.json();
                const flows = Array.isArray(data) ? data : data.flows || [];

                update((s) => ({
                    ...s,
                    flows,
                    loading: false,
                    lastUpdate: new Date(),
                }));

                return flows;
            } catch (e) {
                const error = e instanceof Error ? e.message : "Failed to fetch flows";
                update((s) => ({ ...s, loading: false, error }));
                throw e;
            }
        },

        /**
         * Kill a flow (temporary 5-minute block)
         */
        async kill(flowId: string) {
            try {
                const response = await fetch(`/api/flows?id=${flowId}`, {
                    method: "DELETE",
                    credentials: "include",
                });

                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}`);
                }

                // Remove from local state
                update((s) => ({
                    ...s,
                    flows: s.flows.filter((f) => f.id !== flowId),
                }));

                return true;
            } catch (e) {
                console.error("Failed to kill flow:", e);
                throw e;
            }
        },

        /**
         * Block a flow permanently (adds to policy)
         */
        async block(flow: Flow) {
            // TODO: Implement via /api/flows/deny or add policy rule
            try {
                const response = await fetch("/api/flows/deny", {
                    method: "POST",
                    credentials: "include",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                        src_ip: flow.src_ip,
                        dst_ip: flow.dst_ip,
                        dst_port: flow.dst_port,
                        protocol: flow.protocol,
                    }),
                });

                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}`);
                }

                // Remove from local state
                update((s) => ({
                    ...s,
                    flows: s.flows.filter((f) => f.id !== flow.id),
                }));

                return true;
            } catch (e) {
                console.error("Failed to block flow:", e);
                throw e;
            }
        },

        /**
         * Start polling for flow updates
         */
        startPolling(intervalMs = 2000) {
            this.stopPolling();
            this.fetch(); // Initial fetch
            pollInterval = setInterval(() => this.fetch(), intervalMs);

            // Also subscribe to WebSocket flows topic
            wsSubscribe(["flows"]);
        },

        /**
         * Stop polling
         */
        stopPolling() {
            if (pollInterval) {
                clearInterval(pollInterval);
                pollInterval = null;
            }
            wsUnsubscribe(["flows"]);
        },

        /**
         * Update from WebSocket message
         */
        updateFromWS(data: Flow[]) {
            update((s) => ({
                ...s,
                flows: data,
                lastUpdate: new Date(),
            }));
        },

        /**
         * Reset store
         */
        reset() {
            this.stopPolling();
            set(initialState);
        },
    };
}

export const flowsStore = createFlowsStore();

// Listen for WebSocket flow updates
if (typeof window !== "undefined") {
    window.addEventListener("ws-flows", ((e: CustomEvent) => {
        flowsStore.updateFromWS(e.detail);
    }) as EventListener);
}

// ============================================================================
// Helpers
// ============================================================================

/**
 * Format bytes to human readable
 */
export function formatBytes(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

/**
 * Format seconds to human readable age
 */
export function formatAge(seconds: number): string {
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
    if (seconds < 86400) return `${Math.floor(seconds / 3600)}h`;
    return `${Math.floor(seconds / 86400)}d`;
}

/**
 * Calculate rate from bytes over time
 */
export function formatRate(bytes: number, seconds: number): string {
    if (seconds <= 0) return "0 B/s";
    const rate = bytes / seconds;
    return formatBytes(rate) + "/s";
}

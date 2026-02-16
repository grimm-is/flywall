import { writable, derived, get } from 'svelte/store';
import { containerIPMap } from './runtime';

export interface Flow {
    id: string;
    src_ip: string;
    src_port: number;
    dest_ip: string;
    dest_port: number;
    protocol: string;
    bytes: number;
    packets: number;
    // Calculated fields
    bps: number;
    pps: number;
    timestamp: number;
    // Smoothing state (internal)
    _ewma_bps?: number;
    _ewma_pps?: number;
    // Identity context
    container_id?: string;
    process_name?: string;
}

export interface Snapshot {
    timestamp: number;
    flows: Map<string, Flow>;
    total_bps: number;
    total_pps: number;
}

// Configuration
const HISTORY_SECONDS = 300; // 5 minutes
const SNAPSHOT_INTERVAL = 1000; // 1 second (approx match to update rate)

// Internal Live Store
const _liveFlows = writable<Map<string, Flow>>(new Map());

// DVR Store
function createDVRStore() {
    const { subscribe, set, update } = writable({
        isLive: true,
        seekTime: 0, // Timestamp
        history: [] as Snapshot[], // Ring buffer
    });

    return {
        subscribe,

        // Add a new snapshot from the live feed
        pushSnapshot: (flowsMap: Map<string, Flow>) => {
            const now = Date.now();
            let total_bps = 0;
            let total_pps = 0;

            // Calculate aggregates
            for (const f of flowsMap.values()) {
                total_bps += f.bps;
                total_pps += f.pps;
            }

            update(state => {
                const newHistory = [...state.history, {
                    timestamp: now,
                    flows: new Map(flowsMap), // Clone map to freeze state
                    total_bps,
                    total_pps
                }];

                // Prune old history
                const cutoff = now - (HISTORY_SECONDS * 1000);
                while (newHistory.length > 0 && newHistory[0].timestamp < cutoff) {
                    newHistory.shift();
                }

                return {
                    ...state,
                    history: newHistory,
                    // Auto-advance seekTime if we were live?
                    // No, seekTime is only relevant if !isLive.
                };
            });
        },

        seek: (timestamp: number) => {
            update(s => ({ ...s, isLive: false, seekTime: timestamp }));
        },

        goLive: () => {
            update(s => ({ ...s, isLive: true }));
        },

        fetchHistory: async () => {
            try {
                const res = await fetch('/api/discovery/history');
                const data = await res.json();

                // Convert arrays back to Maps
                const snapshots: Snapshot[] = data.map((d: any) => ({
                    timestamp: d.timestamp,
                    flows: new Map(d.flows.map((f: any) => [f.id, f])),
                    total_bps: d.total_bps,
                    total_pps: d.total_pps
                }));

                update(s => {
                    // Merge history (prepend fetched history to existing)
                    // In a real implementation we'd handle overlaps more carefully
                    const merged = [...snapshots, ...s.history].sort((a, b) => a.timestamp - b.timestamp);
                    return { ...s, history: merged };
                });
            } catch (e) {
                console.error("Failed to fetch DVR history", e);
            }
        }
    };
}

export const dvr = createDVRStore();

// The Main Exported Store (polymorphic: Live or DVR)
export const flows = derived(
    [_liveFlows, dvr],
    ([$live, $dvr]) => {
        if ($dvr.isLive) {
            return $live;
        } else {
            // Find closest snapshot
            const snap = $dvr.history.find(s => Math.abs(s.timestamp - $dvr.seekTime) < 1500)
                || $dvr.history[$dvr.history.length - 1]; // fallback to latest
            return snap ? snap.flows : new Map<string, Flow>();
        }
    }
);

// Actions Controller
export const flowActions = {
    handleUpdate: (rawFlows: any[]) => {
        // Access current container map synchronously
        const ipMap = get(containerIPMap);

        _liveFlows.update(current => {
            const next = new Map<string, Flow>();
            const now = Date.now();

            rawFlows.forEach((f: any) => {
                const prev = current.get(f.id);
                let bps = 0;
                let pps = 0;
                let ewmaBps = 0;
                let ewmaPps = 0;

                if (prev) {
                    const timeDelta = Math.max((now - prev.timestamp) / 1000, 0.001);
                    const deltaBytes = f.bytes - prev.bytes;
                    const deltaPkts = f.packets - prev.packets;

                    if (deltaBytes >= 0) {
                        const instantBps = deltaBytes / timeDelta;
                        const instantPps = deltaPkts / timeDelta;

                        const alpha = 0.3;
                        ewmaBps = (instantBps * alpha) + ((prev._ewma_bps || instantBps) * (1 - alpha));
                        ewmaPps = (instantPps * alpha) + ((prev._ewma_pps || instantPps) * (1 - alpha));

                        bps = Math.round(ewmaBps);
                        pps = Math.round(ewmaPps);
                    } else {
                        ewmaBps = 0;
                        ewmaPps = 0;
                    }
                }

                // Identity Context Resolution
                const containerName = ipMap[f.src_ip];

                next.set(f.id, {
                    ...f,
                    timestamp: now,
                    bps,
                    pps,
                    _ewma_bps: ewmaBps,
                    _ewma_pps: ewmaPps,
                    process_name: containerName, // Enriched Identity
                    container_id: containerName ? 'container' : undefined // Sentinel flag
                });
            });

            // Push to DVR
            dvr.pushSnapshot(next);

            return next;
        });
    },

    kill: async (id: string) => {
        // Optimistic update (only affects live view really)
        _liveFlows.update(s => {
            const next = new Map(s);
            next.delete(id);
            return next;
        });

        try {
            await fetch(`/api/conntrack/${id}`, { method: 'DELETE' });
        } catch (e) {
            console.error("Failed to kill flow", e);
        }
    },

    block: async (ip: string) => {
        try {
            await fetch('/api/policy/block-temporary', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ ip, duration: '5m' })
            });
            // Optional: Notification or toast here
        } catch (e) {
            console.error("Failed to block IP", e);
        }
    }
};

// Derived store for Top Talkers
export const topTalkers = derived(flows, ($flows) => {
    return Array.from($flows.values())
        .sort((a, b) => b.bps - a.bps)
        .slice(0, 50);
});

// Helper for formatting bytes
export function formatBytes(bytes: number, decimals = 2) {
    if (!+bytes) return '0 B';

    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];

    const i = Math.floor(Math.log(bytes) / Math.log(k));

    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}

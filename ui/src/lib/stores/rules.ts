/**
 * Rules Store - State management for ClearPath Policy Editor
 * Handles API fetching with smart stats merging to avoid UI flickering
 */

import { writable, derived, get } from 'svelte/store';

// ============================================================================
// Types
// ============================================================================

export interface ResolvedAddress {
    display_name: string;
    type: string;
    description?: string;
    count: number;
    is_truncated?: boolean;
    preview?: string[];
}

export interface RuleStats {
    packets: number;
    bytes: number;
    sparkline_data: number[];
    // Rate fields (from WebSocket updates)
    packets_per_sec?: number;
    bytes_per_sec?: number;
    last_match_unix?: number;
}

// ============================================================================
// Heat Level Helpers (The Pulse)
// ============================================================================

/**
 * Get heat level (0-1) based on packets per second
 * Used for visual heatmap coloring on rule rows
 */
export function getHeatLevel(stats: RuleStats | undefined): number {
    if (!stats) return 0;
    const pps = stats.packets_per_sec || 0;
    if (pps <= 0) return 0;
    // Log scale: 1 PPS = 0.2, 10 PPS = 0.5, 100 PPS = 0.8, 1000+ = 1.0
    return Math.min(1, Math.log10(pps + 1) / 3);
}

/**
 * Check if a rule is "cold" (no traffic in last 5 minutes)
 */
export function isCold(stats: RuleStats | undefined): boolean {
    if (!stats?.last_match_unix) return true;
    const fiveMinutesAgo = Math.floor(Date.now() / 1000) - (5 * 60);
    return stats.last_match_unix < fiveMinutesAgo;
}

/**
 * Get CSS color for heat level (green -> yellow -> red)
 */
export function getHeatColor(heat: number): string {
    const hue = 120 - (heat * 120); // 120=green, 0=red
    return `hsl(${hue}, 70%, 50%)`;
}

/**
 * Format bytes per second for display
 */
export function formatBps(bps: number): string {
    if (bps <= 0) return '0 B/s';
    if (bps < 1024) return `${bps.toFixed(0)} B/s`;
    if (bps < 1024 * 1024) return `${(bps / 1024).toFixed(1)} KB/s`;
    if (bps < 1024 * 1024 * 1024) return `${(bps / (1024 * 1024)).toFixed(1)} MB/s`;
    return `${(bps / (1024 * 1024 * 1024)).toFixed(2)} GB/s`;
}

/**
 * Format packets per second for display
 */
export function formatPps(pps: number): string {
    if (pps <= 0) return '0 pps';
    if (pps < 1000) return `${pps.toFixed(1)} pps`;
    if (pps < 1000000) return `${(pps / 1000).toFixed(1)}K pps`;
    return `${(pps / 1000000).toFixed(1)}M pps`;
}

export interface RuleWithStats {
    id?: string;
    name?: string;
    description?: string;
    action: string;
    protocol?: string;
    src_ip?: string;
    src_ipset?: string;
    dest_ip?: string;
    dest_ipset?: string;
    dest_port?: number;
    services?: string[];
    disabled?: boolean;
    group?: string;
    tags?: string[];
    stats?: RuleStats;
    resolved_src?: ResolvedAddress;
    resolved_dest?: ResolvedAddress;
    nft_syntax?: string;
    policy_from?: string;
    policy_to?: string;
}

export interface PolicyWithStats {
    from: string;
    to: string;
    default_action?: string;
    description?: string;
    rules: RuleWithStats[];
}

export interface GroupInfo {
    name: string;
    count: number;
}

// ============================================================================
// Stores
// ============================================================================

export const policies = writable<PolicyWithStats[]>([]);
export const flatRules = writable<RuleWithStats[]>([]);
export const groups = writable<GroupInfo[]>([]);
export const selectedGroup = writable<string | null>(null);
export const isLoading = writable(false);
export const lastError = writable<string | null>(null);

// Derived store: filtered rules by group
export const filteredRules = derived(
    [flatRules, selectedGroup],
    ([$flatRules, $selectedGroup]) => {
        if (!$selectedGroup) return $flatRules;
        return $flatRules.filter(r => r.group === $selectedGroup);
    }
);

// ============================================================================
// API Methods
// ============================================================================

const API_BASE = '/api';

async function apiRequest(endpoint: string): Promise<any> {
    const response = await fetch(`${API_BASE}${endpoint}`, {
        credentials: 'include',
    });

    if (!response.ok) {
        const text = await response.text();
        throw new Error(text || `HTTP ${response.status}`);
    }

    return response.json();
}

/**
 * Smart merge: Only update stats field to avoid re-rendering entire rule rows
 */
function mergeStats(existing: RuleWithStats[], incoming: RuleWithStats[]): RuleWithStats[] {
    if (existing.length !== incoming.length) {
        return incoming; // Structure changed, full replace
    }

    return existing.map((rule, i) => {
        const newRule = incoming[i];

        // If IDs match, just merge stats
        if (rule.id === newRule.id) {
            return {
                ...rule,
                stats: newRule.stats,
                resolved_src: newRule.resolved_src,
                resolved_dest: newRule.resolved_dest,
            };
        }

        // Rule changed position/identity, full replace
        return newRule;
    });
}

export const rulesApi = {
    _pollInterval: null as ReturnType<typeof setInterval> | null,

    /**
     * Load all policies with rules (grouped view)
     */
    async loadPolicies(withStats = true) {
        isLoading.set(true);
        lastError.set(null);

        try {
            const url = withStats ? '/rules?with_stats=true' : '/rules';
            const data = await apiRequest(url);
            policies.set(data);
            return data;
        } catch (e) {
            lastError.set(e instanceof Error ? e.message : 'Failed to load policies');
            throw e;
        } finally {
            isLoading.set(false);
        }
    },

    /**
     * Load flat rules list (ungrouped view)
     */
    async loadFlatRules(withStats = true, group?: string) {
        isLoading.set(true);
        lastError.set(null);

        try {
            let url = '/rules/flat';
            const params = new URLSearchParams();
            if (withStats) params.set('with_stats', 'true');
            if (group) params.set('group', group);
            if (params.toString()) url += '?' + params.toString();

            const data = await apiRequest(url);

            // Smart merge to preserve UI state
            const current = get(flatRules);
            if (current.length > 0 && withStats) {
                flatRules.set(mergeStats(current, data));
            } else {
                flatRules.set(data);
            }

            return data;
        } catch (e) {
            lastError.set(e instanceof Error ? e.message : 'Failed to load rules');
            throw e;
        } finally {
            isLoading.set(false);
        }
    },

    /**
     * Load available group tags
     */
    async loadGroups() {
        try {
            const data = await apiRequest('/rules/groups');
            groups.set(data);
            return data;
        } catch (e) {
            console.error('Failed to load groups', e);
            return [];
        }
    },

    /**
     * Start polling for stats updates (every 2s)
     */
    startStatsPolling() {
        this.stopStatsPolling();

        // Initial load
        this.loadFlatRules(true);

        // Poll every 2 seconds
        this._pollInterval = setInterval(() => {
            this.loadFlatRules(true, get(selectedGroup) || undefined).catch(() => {
                // Ignore polling errors to prevent crash
            });
        }, 2000);
    },

    /**
     * Stop stats polling
     */
    stopStatsPolling() {
        if (this._pollInterval) {
            clearInterval(this._pollInterval);
            this._pollInterval = null;
        }
    },

    /**
     * Handle WebSocket stats updates (The Pulse)
     */
    updateStatsFromWS(statsMap: Record<string, any>) {
        const currentRules = get(flatRules);
        if (currentRules.length === 0) return;

        const updatedRules = currentRules.map(rule => {
            // Match by policy key (from_zone->to_zone) or rule id/name
            const policyKey = rule.policy_from && rule.policy_to
                ? `${rule.policy_from}->${rule.policy_to}`
                : '';
            const ruleStats = statsMap[policyKey] || statsMap[rule.id || ''] || statsMap[rule.name || ''];

            if (ruleStats) {
                // Update stats including rate fields for The Pulse
                return {
                    ...rule,
                    stats: {
                        packets: ruleStats.packets || 0,
                        bytes: ruleStats.bytes || 0,
                        packets_per_sec: ruleStats.packets_per_sec || 0,
                        bytes_per_sec: ruleStats.bytes_per_sec || 0,
                        last_match_unix: ruleStats.last_match_unix,
                        sparkline_data: rule.stats?.sparkline_data || []
                    },
                };
            }
            return rule;
        });

        flatRules.set(updatedRules);
    },

    /**
     * Initialize WebSocket listeners
     */
    initWebSocket() {
        if (typeof window === 'undefined') return;

        window.addEventListener('ws-stats-rules', ((e: CustomEvent) => {
            this.updateStatsFromWS(e.detail);
        }) as EventListener);
    },

    /**
     * Toggle rule enabled/disabled state
     */
    async toggleRule(ruleId: string, disabled: boolean) {
        const currentRules = get(flatRules);
        const ruleIndex = currentRules.findIndex(r => r.id === ruleId);
        if (ruleIndex === -1) return;

        // Optimistic update
        const updatedRules = [...currentRules];
        updatedRules[ruleIndex] = { ...updatedRules[ruleIndex], disabled };
        flatRules.set(updatedRules);

        try {
            // Convert flatRules to config.Policy format for API
            await this.savePolicies(updatedRules);
        } catch (e) {
            // Rollback on failure
            flatRules.set(currentRules);
            throw e;
        }
    },

    /**
     * Delete a rule by ID
     */
    async deleteRule(ruleId: string) {
        const currentRules = get(flatRules);
        const updatedRules = currentRules.filter(r => r.id !== ruleId);

        // Optimistic update
        flatRules.set(updatedRules);

        try {
            await this.savePolicies(updatedRules);
        } catch (e) {
            // Rollback on failure
            flatRules.set(currentRules);
            throw e;
        }
    },

    /**
     * Update a rule
     */
    async updateRule(ruleId: string, updates: Partial<RuleWithStats>) {
        const currentRules = get(flatRules);
        const ruleIndex = currentRules.findIndex(r => r.id === ruleId);
        if (ruleIndex === -1) return;

        const updatedRules = [...currentRules];
        updatedRules[ruleIndex] = { ...updatedRules[ruleIndex], ...updates };
        flatRules.set(updatedRules);

        try {
            await this.savePolicies(updatedRules);
        } catch (e) {
            flatRules.set(currentRules);
            throw e;
        }
    },

    /**
     * Create a new rule
     */
    async createRule(rule: Omit<RuleWithStats, 'id' | 'stats'>) {
        const currentRules = get(flatRules);
        const newRule: RuleWithStats = {
            ...rule,
            id: `rule_${Date.now()}`,
        };
        const updatedRules = [...currentRules, newRule];
        flatRules.set(updatedRules);

        try {
            await this.savePolicies(updatedRules);
            return newRule;
        } catch (e) {
            flatRules.set(currentRules);
            throw e;
        }
    },

    /**
     * Save policies to backend API
     */
    async savePolicies(rules: RuleWithStats[]) {
        // Convert RuleWithStats to config.Policy format
        const policies = rules.map(r => ({
            name: r.name || r.id,
            description: r.description,
            action: r.action,
            protocol: r.protocol,
            source_ip: r.src_ip,
            source_ipset: r.src_ipset,
            destination_ip: r.dest_ip,
            destination_ipset: r.dest_ipset,
            destination_port: r.dest_port,
            source_zone: r.policy_from,
            destination_zone: r.policy_to,
            disabled: r.disabled,
        }));

        const response = await fetch(`${API_BASE}/config/policies`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify(policies),
        });

        if (!response.ok) {
            const text = await response.text();
            throw new Error(text || `HTTP ${response.status}`);
        }

        return response.json();
    },

    /**
     * Select a group filter
     */
    selectGroup(group: string | null) {
        selectedGroup.set(group);
        this.loadFlatRules(true, group || undefined);
    },
};

// Initialize listeners immediately
rulesApi.initWebSocket();

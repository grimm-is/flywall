<script lang="ts">
    import { onMount } from "svelte";
    import BaseWidget from "./BaseWidget.svelte";
    import Icon from "$lib/components/Icon.svelte";
    import { api } from "$lib/stores/app";

    let { onremove } = $props();

    interface UplinkGroup {
        name: string;
        active_uplink: string;
        uplinks: Array<{
            name: string;
            enabled: boolean;
            healthy: boolean;
            interface: string;
            weight?: number;
        }>;
    }

    let uplinkGroups = $state<UplinkGroup[]>([]);

    async function loadUplinkGroups() {
        try {
            const res = await api.getUplinkGroups();
            uplinkGroups = res || [];
        } catch (e) {
            console.log("Uplinks not configured or unavailable");
            uplinkGroups = [];
        }
    }

    async function switchUplink(groupName: string, uplinkName: string) {
        try {
            await api.switchUplink(groupName, uplinkName);
            await loadUplinkGroups();
        } catch (e: any) {
            console.error("Failed to switch uplink:", e);
        }
    }

    async function toggleUplink(
        groupName: string,
        uplinkName: string,
        enabled: boolean,
    ) {
        try {
            await api.toggleUplink(groupName, uplinkName, enabled);
            await loadUplinkGroups();
        } catch (e: any) {
            console.error("Failed to toggle uplink:", e);
        }
    }

    onMount(() => {
        loadUplinkGroups();
    });
</script>

<BaseWidget title="Multi-WAN Uplinks" icon="alt_route" {onremove}>
    {#if !uplinkGroups || uplinkGroups.length === 0}
        <div class="empty-state">
            <span class="text-sm text-muted">No Uplink Groups Configured</span>
        </div>
    {:else}
        <div class="uplinks-container">
            {#each uplinkGroups as group}
                <div class="uplink-group">
                    <div class="group-header">
                        <span class="group-name">{group.name}</span>
                        <span class="active-badge">{group.active_uplink}</span>
                    </div>
                    <div class="uplink-list">
                        {#each group.uplinks as uplink}
                            <div
                                class="uplink-row"
                                class:active={uplink.name ===
                                    group.active_uplink}
                            >
                                <div class="uplink-id">
                                    <span
                                        class="status-dot"
                                        class:healthy={uplink.healthy}
                                    ></span>
                                    <span class="name">{uplink.name}</span>
                                </div>
                                <div class="uplink-actions">
                                    <button
                                        class="btn-xs"
                                        class:primary={uplink.name !==
                                            group.active_uplink}
                                        disabled={uplink.name ===
                                            group.active_uplink}
                                        onclick={() =>
                                            switchUplink(
                                                group.name,
                                                uplink.name,
                                            )}
                                    >
                                        {uplink.name === group.active_uplink
                                            ? "Active"
                                            : "Switch"}
                                    </button>
                                </div>
                            </div>
                        {/each}
                    </div>
                </div>
            {/each}
        </div>
    {/if}
</BaseWidget>

<style>
    .uplinks-container {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
    }

    .uplink-group {
        border: 1px solid var(--dashboard-border);
        border-radius: var(--radius-md);
        overflow: hidden;
    }

    .group-header {
        background: var(--dashboard-input);
        padding: var(--space-2) var(--space-3);
        display: flex;
        justify-content: space-between;
        align-items: center;
        font-size: var(--text-xs);
        font-weight: 600;
    }

    .uplink-list {
        display: flex;
        flex-direction: column;
    }

    .uplink-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: var(--space-2) var(--space-3);
        border-bottom: 1px solid var(--dashboard-border);
        font-size: var(--text-sm);
    }

    .uplink-row:last-child {
        border-bottom: none;
    }

    .uplink-row.active {
        background: rgba(var(--color-primary-rgb), 0.05);
    }

    .uplink-id {
        display: flex;
        align-items: center;
        gap: var(--space-2);
    }

    .status-dot {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        background: var(--color-destructive);
    }

    .status-dot.healthy {
        background: var(--color-success);
    }

    .btn-xs {
        padding: 2px 8px;
        font-size: 11px;
        border-radius: 4px;
        border: 1px solid var(--dashboard-border);
        background: transparent;
        cursor: pointer;
    }

    .btn-xs.primary {
        background: var(--color-primary);
        color: white;
        border: none;
    }

    .btn-xs:disabled {
        opacity: 0.5;
        cursor: default;
    }

    .empty-state {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100%;
        color: var(--dashboard-text-muted);
    }
</style>

<script lang="ts">
    import { t } from "svelte-i18n";
    import { onMount } from "svelte";
    import {
        zones,
        wanZones,
        lanZones,
        totalDevices,
        type AggregatedZone,
    } from "$lib/stores/zones";
    import { containers } from "$lib/stores/runtime";
    import { api } from "$lib/stores/app";
    import TopologyGraph from "$lib/components/TopologyGraph.svelte";
    import ZoneCard from "$lib/components/ZoneCard.svelte";
    import Icon from "$lib/components/Icon.svelte";

    // System stats
    let systemStats = $state<any>(null);
    let statsError = $state<string | null>(null);

    async function loadStats() {
        try {
            systemStats = await api.getSystemStats();
            statsError = null;
        } catch (e: any) {
            statsError = e.message || "Failed to load stats";
        }
    }

    function formatBytes(bytes: number): string {
        if (bytes === 0) return "0 B";
        const k = 1024;
        const sizes = ["B", "KB", "MB", "GB", "TB"];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
    }

    function formatUptime(seconds: number): string {
        const days = Math.floor(seconds / 86400);
        const hours = Math.floor((seconds % 86400) / 3600);
        const mins = Math.floor((seconds % 3600) / 60);
        if (days > 0) return `${days}d ${hours}h`;
        if (hours > 0) return `${hours}h ${mins}m`;
        return `${mins}m`;
    }

    // Hero visualization collapsed state (persisted)
    let heroCollapsed = $state(false);

    // Load from localStorage on mount
    $effect(() => {
        const saved = localStorage.getItem("dashboard:hero-collapsed");
        if (saved !== null) heroCollapsed = saved === "true";
    });

    function toggleHero() {
        heroCollapsed = !heroCollapsed;
        localStorage.setItem("dashboard:hero-collapsed", String(heroCollapsed));
    }

    // Uplink Groups (Multi-WAN)
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
    let uplinkLoading = $state(false);

    async function loadUplinkGroups() {
        uplinkLoading = true;
        try {
            uplinkGroups = await api.getUplinkGroups();
        } catch (e) {
            console.log("Uplinks not configured or unavailable");
            uplinkGroups = [];
        } finally {
            uplinkLoading = false;
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
        loadStats();
        loadUplinkGroups();
        const interval = setInterval(loadStats, 10000); // Refresh every 10s
        return () => clearInterval(interval);
    });

    // Build topology graph from aggregated zones
    let topologyGraph = $derived(() => {
        const nodes: any[] = [
            { id: "router", type: "router", label: "Router", ip: "" },
        ];
        const links: any[] = [];

        // Connect WAN zones to internet
        if ($wanZones.length > 0) {
            nodes.push({
                id: "internet",
                type: "cloud",
                label: "Internet",
                ip: "",
            });
            for (const zone of $wanZones) {
                nodes.push({
                    id: zone.name,
                    type: "wan",
                    label: zone.name.toUpperCase(),
                    ip: zone.ips[0] || "",
                });
                links.push({ source: "router", target: zone.name });
                links.push({ source: zone.name, target: "internet" });
            }
        }

        // LAN zones connect to router
        for (const zone of $lanZones) {
            nodes.push({
                id: zone.name,
                type: "switch",
                label: zone.name.toUpperCase(),
                ip: zone.ip || "",
                deviceCount: zone.deviceCount,
            });
            links.push({ source: "router", target: zone.name });
        }

        // Containers connect to router (Host)
        for (const c of $containers) {
            const name = c.Names[0]?.replace(/^\//, "") || c.Id.slice(0, 12);
            // Find primary IP
            let ip = "";
            const networks = Object.values(c.NetworkSettings.Networks);
            if (networks.length > 0) ip = networks[0].IPAddress;

            nodes.push({
                id: c.Id,
                type: "container",
                label: name,
                ip: ip,
                description: c.Image,
                icon: "container",
            });
            links.push({ source: "router", target: c.Id }); // Logic: Host runs containers
        }

        return { nodes, links };
    });
</script>

<div class="topology-page">
    <header class="page-header">
        <div class="page-title">
            <h1>Topology</h1>
            <span class="device-count">{$totalDevices} devices online</span>
        </div>
        <div class="header-actions">
            <button class="btn-secondary">
                <Icon name="radar" size={16} />
                Scan Network
            </button>
            <button
                class="btn-icon"
                onclick={toggleHero}
                aria-label="Toggle visualization"
            >
                <Icon
                    name={heroCollapsed ? "expand_more" : "expand_less"}
                    size={20}
                />
            </button>
        </div>
    </header>

    {#if !heroCollapsed}
        <section class="hero-visualization">
            <TopologyGraph graph={topologyGraph()} />
        </section>
    {/if}

    <!-- System Stats -->
    {#if systemStats}
        <section class="stats-grid">
            <div class="stat-card">
                <div class="stat-icon">
                    <Icon name="schedule" size={24} />
                </div>
                <div class="stat-content">
                    <span class="stat-label">Uptime</span>
                    <span class="stat-value"
                        >{formatUptime(systemStats.uptime_seconds || 0)}</span
                    >
                </div>
            </div>
            <div class="stat-card">
                <div class="stat-icon">
                    <Icon name="memory" size={24} />
                </div>
                <div class="stat-content">
                    <span class="stat-label">CPU</span>
                    <span class="stat-value"
                        >{(systemStats.cpu_percent || 0).toFixed(1)}%</span
                    >
                </div>
                <div
                    class="stat-bar"
                    role="progressbar"
                    aria-valuenow={Math.min(systemStats.cpu_percent || 0, 100)}
                    aria-valuemin="0"
                    aria-valuemax="100"
                    aria-label="CPU Usage"
                >
                    <div
                        class="stat-bar-fill"
                        style="width: {Math.min(
                            systemStats.cpu_percent || 0,
                            100,
                        )}%"
                    ></div>
                </div>
            </div>
            <div class="stat-card">
                <div class="stat-icon">
                    <Icon name="dns" size={24} />
                </div>
                <div class="stat-content">
                    <span class="stat-label">Memory</span>
                    <span class="stat-value"
                        >{formatBytes(systemStats.memory_used || 0)} / {formatBytes(
                            systemStats.memory_total || 0,
                        )}</span
                    >
                </div>
                <div
                    class="stat-bar"
                    role="progressbar"
                    aria-valuenow={systemStats.memory_total
                        ? Math.round(
                              (systemStats.memory_used /
                                  systemStats.memory_total) *
                                  100,
                          )
                        : 0}
                    aria-valuemin="0"
                    aria-valuemax="100"
                    aria-label="Memory Usage"
                >
                    <div
                        class="stat-bar-fill"
                        style="width: {systemStats.memory_total
                            ? (systemStats.memory_used /
                                  systemStats.memory_total) *
                              100
                            : 0}%"
                    ></div>
                </div>
            </div>
            <div class="stat-card">
                <div class="stat-icon">
                    <Icon name="hard_drive" size={24} />
                </div>
                <div class="stat-content">
                    <span class="stat-label">Disk</span>
                    <span class="stat-value"
                        >{formatBytes(systemStats.disk_used || 0)} / {formatBytes(
                            systemStats.disk_total || 0,
                        )}</span
                    >
                </div>
                <div
                    class="stat-bar"
                    role="progressbar"
                    aria-valuenow={systemStats.disk_total
                        ? Math.round(
                              (systemStats.disk_used / systemStats.disk_total) *
                                  100,
                          )
                        : 0}
                    aria-valuemin="0"
                    aria-valuemax="100"
                    aria-label="Disk Usage"
                >
                    <div
                        class="stat-bar-fill"
                        style="width: {systemStats.disk_total
                            ? (systemStats.disk_used / systemStats.disk_total) *
                              100
                            : 0}%"
                    ></div>
                </div>
            </div>
        </section>
    {/if}

    <!-- Uplink Groups (Multi-WAN) -->
    {#if uplinkGroups.length > 0}
        <section class="uplinks-section">
            <h2 class="section-title">Multi-WAN Uplinks</h2>
            <div class="uplinks-grid">
                {#each uplinkGroups as group}
                    <div class="uplink-group-card">
                        <div class="uplink-group-header">
                            <span class="uplink-group-name">{group.name}</span>
                            <span class="uplink-active"
                                >Active: {group.active_uplink || "None"}</span
                            >
                        </div>
                        <div class="uplink-list">
                            {#each group.uplinks as uplink}
                                <div
                                    class="uplink-item"
                                    class:active={uplink.name ===
                                        group.active_uplink}
                                >
                                    <div class="uplink-info">
                                        <span class="uplink-name"
                                            >{uplink.name}</span
                                        >
                                        <span class="uplink-interface"
                                            >({uplink.interface})</span
                                        >
                                        <span
                                            class="uplink-status"
                                            class:healthy={uplink.healthy}
                                            class:unhealthy={!uplink.healthy}
                                            title={uplink.healthy
                                                ? "Healthy"
                                                : "Unhealthy"}
                                            aria-label={uplink.healthy
                                                ? "Status: Healthy"
                                                : "Status: Unhealthy"}
                                        >
                                            {uplink.healthy ? "●" : "○"}
                                        </span>
                                    </div>
                                    <div class="uplink-actions">
                                        <button
                                            class="uplink-switch"
                                            class:selected={uplink.name ===
                                                group.active_uplink}
                                            onclick={() =>
                                                switchUplink(
                                                    group.name,
                                                    uplink.name,
                                                )}
                                            disabled={uplink.name ===
                                                group.active_uplink ||
                                                !uplink.enabled}
                                        >
                                            {uplink.name === group.active_uplink
                                                ? "Active"
                                                : "Switch"}
                                        </button>
                                        <button
                                            class="uplink-toggle"
                                            class:enabled={uplink.enabled}
                                            onclick={() =>
                                                toggleUplink(
                                                    group.name,
                                                    uplink.name,
                                                    !uplink.enabled,
                                                )}
                                        >
                                            {uplink.enabled ? "On" : "Off"}
                                        </button>
                                    </div>
                                </div>
                            {/each}
                        </div>
                    </div>
                {/each}
            </div>
        </section>
    {/if}

    <section class="zones-grid">
        <h2 class="section-title">Zones</h2>
        <div class="zones-cards">
            {#each $zones as zone (zone.name)}
                <ZoneCard {zone} />
            {:else}
                <div class="empty-state">
                    <Icon name="hub" size={48} />
                    <p>No zones configured</p>
                    <a href="/interfaces" class="btn-primary"
                        >Configure Interfaces</a
                    >
                </div>
            {/each}
        </div>
    </section>
</div>

<style>
    .topology-page {
        display: flex;
        flex-direction: column;
        gap: var(--space-6);
    }

    .page-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
    }

    .page-title {
        display: flex;
        align-items: baseline;
        gap: var(--space-3);
    }

    .page-title h1 {
        font-size: var(--text-2xl);
        font-weight: 600;
        color: var(--dashboard-text);
    }

    .device-count {
        font-size: var(--text-sm);
        color: var(--dashboard-text-muted);
    }

    .header-actions {
        display: flex;
        gap: var(--space-2);
    }

    .btn-secondary {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-2) var(--space-4);
        background: var(--dashboard-card);
        border: 1px solid var(--dashboard-border);
        border-radius: var(--radius-md);
        color: var(--dashboard-text);
        font-size: var(--text-sm);
        cursor: pointer;
        transition: all var(--transition-fast);
    }

    .btn-secondary:hover {
        background: var(--dashboard-input);
    }

    .btn-icon {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 36px;
        height: 36px;
        background: var(--dashboard-card);
        border: 1px solid var(--dashboard-border);
        border-radius: var(--radius-md);
        color: var(--dashboard-text-muted);
        cursor: pointer;
        transition: all var(--transition-fast);
    }

    .btn-icon:hover {
        background: var(--dashboard-input);
        color: var(--dashboard-text);
    }

    .hero-visualization {
        background: var(--dashboard-card);
        border: 1px solid var(--dashboard-border);
        border-radius: var(--radius-lg);
        height: 350px;
        overflow: hidden;
    }

    .section-title {
        font-size: var(--text-lg);
        font-weight: 600;
        color: var(--dashboard-text);
        margin-bottom: var(--space-4);
    }

    .zones-cards {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(380px, 1fr));
        gap: var(--space-4);
    }

    .empty-state {
        grid-column: 1 / -1;
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: var(--space-4);
        padding: var(--space-12);
        background: var(--dashboard-card);
        border: 1px dashed var(--dashboard-border);
        border-radius: var(--radius-lg);
        color: var(--dashboard-text-muted);
        text-align: center;
    }

    .btn-primary {
        padding: var(--space-2) var(--space-4);
        background: var(--color-primary);
        color: var(--color-primaryForeground);
        border: none;
        border-radius: var(--radius-md);
        font-size: var(--text-sm);
        cursor: pointer;
        text-decoration: none;
    }

    /* Mobile */
    @media (max-width: 768px) {
        .hero-visualization {
            height: 250px;
        }

        .zones-cards {
            grid-template-columns: 1fr;
        }

        .stats-grid {
            grid-template-columns: 1fr 1fr;
        }
    }

    /* Stats Grid */
    .stats-grid {
        display: grid;
        grid-template-columns: repeat(4, 1fr);
        gap: var(--space-4);
        margin-bottom: var(--space-6);
    }

    .stat-card {
        background: var(--dashboard-card);
        border: 1px solid var(--dashboard-border);
        border-radius: var(--radius-lg);
        padding: var(--space-4);
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
    }

    .stat-card .stat-icon {
        color: var(--color-primary);
        opacity: 0.8;
    }

    .stat-content {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
    }

    .stat-label {
        font-size: var(--text-xs);
        color: var(--dashboard-text-muted);
        text-transform: uppercase;
        letter-spacing: 0.05em;
    }

    .stat-value {
        font-size: var(--text-lg);
        font-weight: 600;
        color: var(--dashboard-text);
    }

    .stat-bar {
        height: 4px;
        background: var(--dashboard-border);
        border-radius: 2px;
        overflow: hidden;
    }

    .stat-bar-fill {
        height: 100%;
        background: var(--color-primary);
        border-radius: 2px;
        transition: width 0.3s ease;
    }

    /* Uplinks Section */
    .uplinks-section {
        margin-bottom: var(--space-6);
    }

    .uplinks-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
        gap: var(--space-4);
    }

    .uplink-group-card {
        background: var(--dashboard-card);
        border: 1px solid var(--dashboard-border);
        border-radius: var(--radius-lg);
        overflow: hidden;
    }

    .uplink-group-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: var(--space-3) var(--space-4);
        background: var(--color-backgroundSecondary);
        border-bottom: 1px solid var(--dashboard-border);
    }

    .uplink-group-name {
        font-weight: 600;
        color: var(--dashboard-text);
    }

    .uplink-active {
        font-size: var(--text-xs);
        color: var(--dashboard-text-muted);
    }

    .uplink-list {
        display: flex;
        flex-direction: column;
    }

    .uplink-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: var(--space-3) var(--space-4);
        border-bottom: 1px solid var(--dashboard-border);
    }

    .uplink-item:last-child {
        border-bottom: none;
    }

    .uplink-item.active {
        background: rgba(var(--color-primary-rgb, 59, 130, 246), 0.1);
    }

    .uplink-info {
        display: flex;
        align-items: center;
        gap: var(--space-2);
    }

    .uplink-name {
        font-weight: 500;
        color: var(--dashboard-text);
    }

    .uplink-interface {
        font-size: var(--text-xs);
        color: var(--dashboard-text-muted);
    }

    .uplink-status {
        font-size: var(--text-sm);
    }

    .uplink-status.healthy {
        color: var(--color-success, #22c55e);
    }

    .uplink-status.unhealthy {
        color: var(--color-muted);
    }

    .uplink-actions {
        display: flex;
        gap: var(--space-2);
    }

    .uplink-switch,
    .uplink-toggle {
        padding: var(--space-1) var(--space-2);
        font-size: var(--text-xs);
        border-radius: var(--radius-sm);
        cursor: pointer;
        transition: all 0.2s;
    }

    .uplink-switch {
        background: var(--color-backgroundSecondary);
        border: 1px solid var(--dashboard-border);
        color: var(--dashboard-text);
    }

    .uplink-switch.selected {
        background: var(--color-primary);
        color: white;
        border-color: var(--color-primary);
    }

    .uplink-switch:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .uplink-toggle {
        background: var(--color-destructive);
        border: none;
        color: white;
    }

    .uplink-toggle.enabled {
        background: var(--color-success, #22c55e);
    }
</style>

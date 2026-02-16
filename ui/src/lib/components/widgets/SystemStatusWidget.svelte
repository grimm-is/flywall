<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import BaseWidget from "./BaseWidget.svelte";
    import Icon from "$lib/components/Icon.svelte";
    import { api } from "$lib/stores/app";

    let { type = "cpu", onremove } = $props();

    // Type can be: 'cpu', 'memory', 'disk', 'uptime'
    // This widget handles one stat based on type, allowing granular grid placement

    let systemStats = $state<any>(null);
    let interval: any;

    async function loadStats() {
        try {
            systemStats = await api.getSystemStats();
        } catch (e) {
            console.error("Failed to load stats", e);
        }
    }

    onMount(() => {
        loadStats();
        interval = setInterval(loadStats, 5000);
    });

    onDestroy(() => {
        if (interval) clearInterval(interval);
    });

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

    // Config based on type
    const config = $derived(() => {
        switch (type) {
            case "stat-uptime":
                return {
                    title: "Uptime",
                    icon: "schedule",
                    value: systemStats
                        ? formatUptime(systemStats.uptime_seconds || 0)
                        : "-",
                    percent: 0,
                    showBar: false,
                };
            case "stat-cpu":
                return {
                    title: "CPU",
                    icon: "memory",
                    value: systemStats
                        ? `${(systemStats.cpu_percent || 0).toFixed(1)}%`
                        : "-",
                    percent: systemStats ? systemStats.cpu_percent || 0 : 0,
                    showBar: true,
                };
            case "stat-memory":
                return {
                    title: "Memory",
                    icon: "dns",
                    value: systemStats
                        ? `${formatBytes(systemStats.memory_used || 0)}`
                        : "-",
                    valueDetails: systemStats
                        ? `/ ${formatBytes(systemStats.memory_total || 0)}`
                        : "",
                    percent: systemStats?.memory_total
                        ? (systemStats.memory_used / systemStats.memory_total) *
                          100
                        : 0,
                    showBar: true,
                };
            case "stat-disk":
                return {
                    title: "Disk",
                    icon: "hard_drive",
                    value: systemStats
                        ? `${formatBytes(systemStats.disk_used || 0)}`
                        : "-",
                    valueDetails: systemStats
                        ? `/ ${formatBytes(systemStats.disk_total || 0)}`
                        : "",
                    percent: systemStats?.disk_total
                        ? (systemStats.disk_used / systemStats.disk_total) * 100
                        : 0,
                    showBar: true,
                };
            default:
                return {
                    title: "Unknown",
                    icon: "help",
                    value: "-",
                    percent: 0,
                    showBar: false,
                };
        }
    });
</script>

<BaseWidget title={config().title} icon={config().icon} {onremove}>
    <div class="stat-content">
        <div class="value-row">
            <span class="stat-value">{config().value}</span>
            {#if config().valueDetails}
                <span class="stat-details">{config().valueDetails}</span>
            {/if}
        </div>

        {#if config().showBar}
            <div
                class="stat-bar"
                role="progressbar"
                aria-valuenow={Math.round(config().percent)}
                aria-valuemin="0"
                aria-valuemax="100"
            >
                <div
                    class="stat-bar-fill"
                    style="width: {Math.min(config().percent, 100)}%"
                ></div>
            </div>
        {/if}
    </div>
</BaseWidget>

<style>
    .stat-content {
        display: flex;
        flex-direction: column;
        justify-content: center;
        height: 100%;
        gap: var(--space-2);
    }

    .value-row {
        display: flex;
        align-items: baseline;
        gap: var(--space-1);
    }

    .stat-value {
        font-size: var(--text-2xl);
        font-weight: 600;
        color: var(--dashboard-text);
    }

    .stat-details {
        font-size: var(--text-xs);
        color: var(--dashboard-text-muted);
    }

    .stat-bar {
        height: 6px;
        background: var(--dashboard-border);
        border-radius: 3px;
        overflow: hidden;
        margin-top: auto; /* Push to bottom */
    }

    .stat-bar-fill {
        height: 100%;
        background: var(--color-primary);
        border-radius: 3px;
        transition: width 0.5s ease;
    }
</style>

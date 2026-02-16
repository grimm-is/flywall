<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import BaseWidget from "./BaseWidget.svelte";
    import { api } from "$lib/stores/app";
    import Icon from "$lib/components/Icon.svelte";

    let { onremove } = $props();

    let info: any = $state({});
    let interval: any;

    async function loadStatus() {
        try {
            const res = await api.get("/api/status");
            info = res;
        } catch (e) {
            console.error("Failed to load system status", e);
        }
    }

    onMount(() => {
        loadStatus();
        interval = setInterval(loadStatus, 10000);
    });

    onDestroy(() => {
        if (interval) clearInterval(interval);
    });

    function getUptimeShort(uptime: string) {
        if (!uptime) return "-";
        // Parsed from Go string like "2h30m10s" or simple display
        // Just take the first part before dot if it has ms
        return uptime.toString().split(".")[0];
    }
</script>

<BaseWidget title="System Health" icon="monitor_heart" {onremove}>
    <div class="p-4 space-y-3">
        <!-- Firewall Status -->
        <div class="flex justify-between items-center">
            <span class="text-sm text-gray-500">Firewall</span>
            <div class="flex items-center gap-2">
                <div
                    class={`w-2 h-2 rounded-full ${info.firewall_active ? "bg-green-500" : "bg-red-500"}`}
                ></div>
                <span class="font-medium text-sm"
                    >{info.firewall_active ? "Active" : "Stopped"}</span
                >
            </div>
        </div>

        <!-- WAN IP -->
        <div class="flex justify-between items-center">
            <span class="text-sm text-gray-500">WAN IP</span>
            <span
                class="font-mono text-xs bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded select-all"
            >
                {info.wan_ip || "Unavailable"}
            </span>
        </div>

        <!-- Uptime -->
        <div class="flex justify-between items-center">
            <span class="text-sm text-gray-500">Uptime</span>
            <span class="text-sm font-medium"
                >{getUptimeShort(info.uptime)}</span
            >
        </div>

        <!-- Monitors -->
        {#if info.monitors && info.monitors.length > 0}
            <div class="pt-2 border-t border-gray-100 dark:border-gray-800">
                <div class="text-xs text-gray-400 mb-2 font-medium">
                    Connectivity
                </div>
                <div class="space-y-1">
                    {#each info.monitors as monitor}
                        <div class="flex items-center justify-between text-xs">
                            <span
                                class="text-gray-600 dark:text-gray-400 truncate max-w-[120px]"
                                title={monitor.target}
                                >{monitor.name || monitor.target}</span
                            >
                            <div
                                class={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${monitor.status === "up" ? "bg-green-500" : "bg-red-500"}`}
                            ></div>
                        </div>
                    {/each}
                </div>
            </div>
        {/if}
    </div>
</BaseWidget>

<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import BaseWidget from "./BaseWidget.svelte";
    import { api } from "$lib/stores/app";

    let { onremove } = $props();

    let stats = $state({ active_leases: 0, total_leases: 0, enabled: false });
    let interval: any;

    async function loadStats() {
        try {
            const res = await api.get("/api/monitoring/services");
            if (res.dhcp) {
                stats = res.dhcp;
            }
        } catch (e) {
            console.error("Failed to load DHCP stats", e);
        }
    }

    onMount(() => {
        loadStats();
        interval = setInterval(loadStats, 5000);
    });

    onDestroy(() => {
        if (interval) clearInterval(interval);
    });
</script>

<BaseWidget title="DHCP Server" icon="settings_ethernet" {onremove}>
    <div
        class="p-4 flex flex-col justify-center h-full items-center text-center relative"
    >
        {#if !stats.enabled}
            <div
                class="absolute inset-0 bg-white/50 dark:bg-black/50 flex items-center justify-center backdrop-blur-sm rounded-lg z-10"
            >
                <span
                    class="text-xs font-bold bg-gray-200 dark:bg-gray-700 px-2 py-1 rounded text-gray-500"
                    >DISABLED</span
                >
            </div>
        {/if}

        <div class="text-4xl font-bold text-gray-900 dark:text-white mb-1">
            {stats.active_leases}
        </div>
        <div class="text-sm text-gray-500 mb-4">Active Leases</div>

        <div
            class="w-full bg-gray-200 rounded-full h-1.5 dark:bg-gray-700 overflow-hidden"
        >
            <!-- Assuming ~250 as standard pool size for visualization context -->
            <div
                class="bg-indigo-500 h-1.5 transition-all duration-500"
                style="width: {Math.min(
                    (stats.active_leases / 250) * 100,
                    100,
                )}%"
            ></div>
        </div>
        <div class="text-[10px] text-gray-400 mt-2 uppercase tracking-wide">
            Total History: {stats.total_leases}
        </div>
    </div>
</BaseWidget>

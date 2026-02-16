<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import BaseWidget from "./BaseWidget.svelte";
    import { api } from "$lib/stores/app";

    let { onremove } = $props();

    let counts = $state({ total: 0, tcp: 0, udp: 0 });
    let interval: any;

    async function loadStats() {
        try {
            const res = await api.get("/api/flows?limit=1");
            if (res.total_counts) {
                counts = {
                    total: res.total_counts.total_flows || 0,
                    tcp: res.total_counts.tcp_flows || 0,
                    udp: res.total_counts.udp_flows || 0,
                };
            }
        } catch (e) {
            console.error("Failed to load flow stats", e);
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

<BaseWidget title="Active Connections" icon="hub" {onremove}>
    <div class="p-4 grid grid-cols-3 gap-2 text-center h-full items-center">
        <div class="flex flex-col p-2 bg-gray-50 dark:bg-gray-800 rounded-lg">
            <span class="text-xl font-bold text-gray-900 dark:text-white"
                >{counts.total}</span
            >
            <span
                class="text-[10px] text-gray-500 uppercase tracking-wider mt-1"
                >Total</span
            >
        </div>
        <div
            class="flex flex-col p-2 bg-blue-50 dark:bg-blue-900/20 rounded-lg"
        >
            <span class="text-xl font-bold text-blue-600 dark:text-blue-400"
                >{counts.tcp}</span
            >
            <span
                class="text-[10px] text-blue-500/80 uppercase tracking-wider mt-1"
                >TCP</span
            >
        </div>
        <div
            class="flex flex-col p-2 bg-orange-50 dark:bg-orange-900/20 rounded-lg"
        >
            <span class="text-xl font-bold text-orange-600 dark:text-orange-400"
                >{counts.udp}</span
            >
            <span
                class="text-[10px] text-orange-500/80 uppercase tracking-wider mt-1"
                >UDP</span
            >
        </div>
    </div>
</BaseWidget>

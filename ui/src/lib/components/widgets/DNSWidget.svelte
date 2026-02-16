<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import BaseWidget from "./BaseWidget.svelte";
    import { api } from "$lib/stores/app";

    let { onremove } = $props();

    let stats = $state({
        queries: 0,
        blocked: 0,
        cache_hits: 0,
        cache_misses: 0,
        forwarded: 0,
    });
    let interval: any;

    async function loadStats() {
        try {
            const res = await api.get("/api/monitoring/services");
            if (res.dns) {
                stats = res.dns;
            }
        } catch (e) {
            console.error("Failed to load DNS stats", e);
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

<BaseWidget title="DNS Activity" icon="dns" {onremove}>
    <div class="p-4 flex flex-col justify-between h-full">
        <div>
            <div class="flex justify-between items-center mb-2">
                <span class="text-sm text-gray-500">Total Queries</span>
                <span class="font-bold text-lg"
                    >{stats.queries.toLocaleString()}</span
                >
            </div>

            <div
                class="w-full bg-gray-200 rounded-full h-2.5 dark:bg-gray-700 mb-4 overflow-hidden flex"
            >
                <div
                    class="bg-green-500 h-2.5"
                    style="width: {stats.queries
                        ? (stats.cache_hits / stats.queries) * 100
                        : 0}%"
                    title="Cache Hits"
                ></div>
                <div
                    class="bg-red-500 h-2.5"
                    style="width: {stats.queries
                        ? (stats.blocked / stats.queries) * 100
                        : 0}%"
                    title="Blocked"
                ></div>
                <div
                    class="bg-blue-500 h-2.5"
                    style:flex="1"
                    title="Forwarded"
                ></div>
            </div>
        </div>

        <div class="space-y-2">
            <div class="flex justify-between items-center text-sm">
                <div class="flex items-center gap-2">
                    <div class="w-2 h-2 rounded-full bg-green-500"></div>
                    <span class="text-gray-600 dark:text-gray-400"
                        >Cache Hits</span
                    >
                </div>
                <span class="font-medium"
                    >{stats.cache_hits.toLocaleString()}</span
                >
            </div>

            <div class="flex justify-between items-center text-sm">
                <div class="flex items-center gap-2">
                    <div class="w-2 h-2 rounded-full bg-red-500"></div>
                    <span class="text-gray-600 dark:text-gray-400">Blocked</span
                    >
                </div>
                <span class="font-medium">{stats.blocked.toLocaleString()}</span
                >
            </div>

            <div class="flex justify-between items-center text-sm">
                <div class="flex items-center gap-2">
                    <div class="w-2 h-2 rounded-full bg-blue-500"></div>
                    <span class="text-gray-600 dark:text-gray-400"
                        >Forwarded</span
                    >
                </div>
                <span class="font-medium"
                    >{stats.forwarded.toLocaleString()}</span
                >
            </div>
        </div>
    </div>
</BaseWidget>

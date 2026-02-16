<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import BaseWidget from "./BaseWidget.svelte";
    import Sparkline from "$lib/components/Sparkline.svelte";
    import { api } from "$lib/stores/app";

    let { onremove } = $props();

    let history: number[] = $state([]);
    let currentRate: number = $state(0);
    // Responsive width state
    let width = $state(200);
    let interval: any;

    // State for rate calculation
    let prevBytes = 0;
    let prevTime = 0;

    async function loadStats() {
        try {
            const stats = await api.get("/api/traffic");
            // stats is map of interface stats
            let totalBytes = 0;
            // Sum up Rx+Tx for all interfaces
            // Filter loopback just in case (backend handles it usually)
            Object.values(stats || {}).forEach((iface: any) => {
                if (iface.name !== "lo") {
                    totalBytes += (iface.rx_bytes || 0) + (iface.tx_bytes || 0);
                }
            });

            const now = Date.now() / 1000;

            if (prevTime > 0) {
                const elapsed = now - prevTime;
                if (elapsed > 0) {
                    let diff = totalBytes - prevBytes;
                    // Handle counter reset or restart
                    if (diff < 0 || diff > 100 * 1024 * 1024 * 1024) {
                        // >100GB jump unlikely in 2s
                        diff = 0;
                    }

                    const rate = diff / elapsed; // Bytes per second

                    currentRate = rate;

                    // Update history (keep last 60 points)
                    // Create new array to trigger reactivity
                    const newHistory = [...history, rate];
                    if (newHistory.length > 50) {
                        newHistory.shift();
                    }
                    history = newHistory;
                }
            } else {
                // Initialize history with 0s if empty
                if (history.length === 0) {
                    history = new Array(50).fill(0);
                }
            }

            prevBytes = totalBytes;
            prevTime = now;
        } catch (e) {
            console.error("Failed to load bandwidth stats", e);
        }
    }

    onMount(() => {
        loadStats();
        interval = setInterval(loadStats, 2000);
    });

    onDestroy(() => {
        if (interval) clearInterval(interval);
    });

    function formatRate(bytesSec: number) {
        if (bytesSec === 0) return "0 B/s";
        const units = ["B/s", "KB/s", "MB/s", "GB/s"];
        const i = Math.floor(Math.log(bytesSec) / Math.log(1024)) || 0;
        return `${(bytesSec / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
    }
</script>

<BaseWidget title="Bandwidth" icon="activity" {onremove}>
    <div class="p-4 flex flex-col h-full justify-between">
        <div class="text-3xl font-bold text-gray-900 dark:text-white">
            {formatRate(currentRate)}
        </div>

        <div class="h-16 mt-4 opacity-80 w-full" bind:clientWidth={width}>
            <Sparkline
                data={history}
                color="#3B82F6"
                showArea={true}
                height={60}
                {width}
            />
        </div>

        <div class="text-xs text-gray-500 mt-2 flex justify-between">
            <span>Aggregated Traffic</span>
            <span class="text-emerald-500 font-medium">Real-time</span>
        </div>
    </div>
</BaseWidget>

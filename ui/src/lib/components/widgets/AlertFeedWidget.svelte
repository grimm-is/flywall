<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import BaseWidget from "./BaseWidget.svelte";
    import { api } from "$lib/stores/app";
    import Icon from "$lib/components/Icon.svelte";

    let { onremove } = $props();

    let alerts: any[] = $state([]);
    let interval: any;

    async function loadAlerts() {
        try {
            const res = await api.get("/api/alerts/history?limit=5");
            alerts = res || [];
        } catch (e) {
            console.error("Failed to load alerts", e);
        }
    }

    onMount(() => {
        loadAlerts();
        interval = setInterval(loadAlerts, 10000);
    });

    onDestroy(() => {
        if (interval) clearInterval(interval);
    });

    function getSeverityColor(severity: string) {
        switch (severity) {
            case "critical":
                return "text-red-600 dark:text-red-400";
            case "error":
                return "text-orange-600 dark:text-orange-400";
            case "warning":
                return "text-yellow-600 dark:text-yellow-400";
            default:
                return "text-blue-600 dark:text-blue-400";
        }
    }

    function formatTime(ts: string) {
        if (!ts) return "";
        const d = new Date(ts);
        return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    }
</script>

<BaseWidget title="Recent Alerts" icon="notifications" {onremove}>
    <div class="h-full overflow-y-auto p-0 scrollbar-thin">
        {#if alerts.length === 0}
            <div
                class="flex flex-col items-center justify-center h-full text-gray-400 p-4 min-h-[140px]"
            >
                <div class="mb-2 opacity-30">
                    <Icon name="check_circle" size={32} />
                </div>
                <span class="text-xs">No recent alerts</span>
            </div>
        {:else}
            <div class="divide-y divide-gray-100 dark:divide-gray-800">
                {#each alerts as alert}
                    <div
                        class="p-3 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors text-xs border-l-4 border-transparent pl-2 hover:border-gray-300 dark:hover:border-gray-600 block"
                    >
                        <div class="flex justify-between mb-1 items-center">
                            <span
                                class={`font-bold ${getSeverityColor(alert.severity)} uppercase text-[10px] tracking-wider`}
                            >
                                {alert.severity}
                            </span>
                            <span class="text-gray-400 text-[10px]"
                                >{formatTime(alert.timestamp)}</span
                            >
                        </div>
                        <div
                            class="font-medium text-gray-800 dark:text-gray-200 mb-0.5"
                        >
                            {alert.rule_name || "System Notification"}
                        </div>
                        <div
                            class="text-gray-500 dark:text-gray-400 leading-relaxed break-words"
                        >
                            {alert.message}
                        </div>
                    </div>
                {/each}
            </div>
        {/if}
    </div>
</BaseWidget>

<script lang="ts">
    /**
     * FlowTable.svelte
     * High-density connection monitoring with real-time updates and sparkline visualization.
     */
    import { flows, flowActions, type Flow } from "$lib/stores/flows";
    import { Badge, Button, Icon } from "$lib/components";
    import Sparkline from "$lib/components/Sparkline.svelte";
    import { onMount } from "svelte";

    let sortColumn = $state("bps");
    let sortDirection = $state("desc");
    let searchQuery = $state("");

    // Helper to format large numbers
    function formatRate(n: number, unit: "B" | "P") {
        if (n < 10) return "0";
        if (n < 1024) return Math.round(n) + (unit === "B" ? " B/s" : " pps");
        const k = 1000;
        const sizes =
            unit === "B" ? ["KB/s", "MB/s", "GB/s"] : ["Kpps", "Mpps", "Gpps"];
        const i = Math.floor(Math.log(n) / Math.log(k));
        return parseFloat((n / Math.pow(k, i)).toFixed(1)) + " " + sizes[i - 1];
    }

    // Mock history generation for sparklines (since backend doesn't provide history yet)
    let flowHistory = $state(new Map<string, number[]>());

    // Derived and Sorted Data
    const sortedFlows = $derived(
        Array.from($flows.values())
            .filter((f) => {
                if (!searchQuery) return true;
                const q = searchQuery.toLowerCase();
                return (
                    f.src_ip.includes(q) ||
                    f.dest_ip.includes(q) ||
                    f.protocol.toLowerCase().includes(q)
                );
            })
            .sort((a, b) => {
                let valA = a[sortColumn as keyof Flow] ?? 0; // Default to 0/empty if undefined
                let valB = b[sortColumn as keyof Flow] ?? 0;

                // Handle IPs numerically if possible, otherwise string
                if (sortColumn.includes("ip")) {
                    valA = String(valA);
                    valB = String(valB);
                }

                if (valA < valB) return sortDirection === "asc" ? -1 : 1;
                if (valA > valB) return sortDirection === "asc" ? 1 : -1;
                return 0;
            })
            .slice(0, 100), // Render limit for performance
    );

    // Update history loop
    onMount(() => {
        const interval = setInterval(() => {
            const now = Date.now();
            sortedFlows.forEach((f) => {
                let hist = flowHistory.get(f.id) || Array(30).fill(0);
                hist.push(f.bps);
                if (hist.length > 30) hist.shift();
                flowHistory.set(f.id, hist);
            });
            // Force reactivity by reassigning map
            flowHistory = new Map(flowHistory);
        }, 1000);
        return () => clearInterval(interval);
    });

    function handleSort(col: string) {
        if (sortColumn === col) {
            sortDirection = sortDirection === "asc" ? "desc" : "asc";
        } else {
            sortColumn = col;
            sortDirection = "desc";
        }
    }
</script>

<div class="flow-table-container glass-panel">
    <div
        class="toolbar p-4 flex justify-between items-center border-b border-subtle"
    >
        <div class="search relative">
            <Icon
                name="search"
                size={18}
                class="absolute left-3 top-1/2 -translate-y-1/2 text-muted"
            />
            <input
                type="text"
                placeholder="Filter IPs or Protocol..."
                bind:value={searchQuery}
                class="pl-9 pr-3 py-1.5 rounded-md bg-background border border-border focus:outline-none focus:ring-1 focus:ring-primary w-64 text-sm"
            />
        </div>
        <div class="metrics text-xs text-muted font-mono">
            Showing {sortedFlows.length} / {$flows.size} active flows
        </div>
    </div>

    <div class="table-wrapper overflow-auto" style="height: 600px;">
        <table class="w-full text-sm text-left border-collapse">
            <thead class="sticky top-0 bg-surface z-10 shadow-sm">
                <tr>
                    <th
                        class="p-3 font-semibold text-muted cursor-pointer"
                        onclick={() => handleSort("protocol")}>PROTO</th
                    >
                    <th
                        class="p-3 font-semibold text-muted cursor-pointer"
                        onclick={() => handleSort("src_ip")}>SOURCE</th
                    >
                    <th class="p-3 font-semibold text-muted"></th>
                    <!-- Arrow -->
                    <th
                        class="p-3 font-semibold text-muted cursor-pointer"
                        onclick={() => handleSort("dest_ip")}>DESTINATION</th
                    >
                    <th
                        class="p-3 font-semibold text-muted cursor-pointer text-right"
                        onclick={() => handleSort("bps")}>RATE (BPS)</th
                    >
                    <th class="p-3 font-semibold text-muted text-right w-32"
                        >ACTIVITY</th
                    >
                    <th class="p-3 font-semibold text-muted text-right w-24"
                        >ACT</th
                    >
                </tr>
            </thead>
            <tbody class="font-mono">
                {#each sortedFlows as flow (flow.id)}
                    {@const ratePercent = Math.min(
                        (flow.bps / 1000000) * 100,
                        100,
                    )}
                    <!-- Mock 1MB max for bar -->
                    <tr
                        class="border-b border-subtle hover:bg-surfaceHover transition-colors border-l-2"
                        class:border-transparent={!flow.container_id}
                        class:border-l-purple-500={!!flow.container_id}
                        class:bg-purple-500-10={!!flow.container_id}
                    >
                        <td class="p-3">
                            <Badge variant="outline" class="text-xs"
                                >{flow.protocol}</Badge
                            >
                        </td>
                        <td class="p-3">
                            <div class="flex flex-col">
                                {#if flow.process_name}
                                    <div
                                        class="flex items-center gap-1 text-purple-400 font-bold"
                                    >
                                        <Icon name="dns" size={14} />
                                        <!-- Use generic container/dns icon -->
                                        <span>{flow.process_name}</span>
                                    </div>
                                    <span class="text-xs text-muted"
                                        >{flow.src_ip}:{flow.src_port}</span
                                    >
                                {:else}
                                    <span class="text-foreground"
                                        >{flow.src_ip}</span
                                    >
                                    <span class="text-xs text-muted"
                                        >:{flow.src_port}</span
                                    >
                                {/if}
                            </div>
                        </td>
                        <td class="p-3 text-muted opacity-50">→</td>
                        <td class="p-3">
                            <div class="flex flex-col">
                                <span class="text-foreground"
                                    >{flow.dest_ip}</span
                                >
                                <span class="text-xs text-muted"
                                    >:{flow.dest_port}</span
                                >
                            </div>
                        </td>
                        <td class="p-3 text-right">
                            <div class="flex flex-col items-end">
                                <span
                                    class="font-bold relative"
                                    class:pulse={flow.bps > 100000}
                                >
                                    {formatRate(flow.bps, "B")}
                                    {#if flow.bps > 100000}<span
                                            class="pulse-dot"
                                        ></span>{/if}
                                </span>
                                <span class="text-xs text-muted"
                                    >{formatRate(flow.pps, "P")}</span
                                >
                            </div>
                            <!-- Rate Bar -->
                            <div
                                class="h-1 bg-border mt-1 rounded-full overflow-hidden w-full max-w-[100px] ml-auto"
                            >
                                <div
                                    class="h-full bg-primary transition-all duration-300"
                                    style="width: {ratePercent}%"
                                ></div>
                            </div>
                        </td>
                        <td class="p-3">
                            <Sparkline
                                data={flowHistory.get(flow.id) || []}
                                width={100}
                                height={24}
                                color={flow.bps > 1000000
                                    ? "#ef4444"
                                    : "#10b981"}
                                showArea={true}
                            />
                        </td>
                        <td class="p-3 text-right">
                            <div class="flex justify-end gap-1">
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    class="text-muted hover:text-warning h-8 w-8 p-0"
                                    onclick={() =>
                                        flowActions.block(flow.src_ip)}
                                    title="Block Source IP (5m)"
                                >
                                    <Icon name="shield" size={16} />
                                </Button>
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    class="text-muted hover:text-destructive h-8 w-8 p-0"
                                    onclick={() => flowActions.kill(flow.id)}
                                    title="Kill Flow"
                                >
                                    <Icon name="delete" size={16} />
                                </Button>
                            </div>
                        </td>
                    </tr>
                {/each}
            </tbody>
        </table>
    </div>
</div>

<style>
    .pulse {
        color: var(--color-success);
    }
    .pulse-dot {
        display: inline-block;
        width: 6px;
        height: 6px;
        border-radius: 50%;
        background-color: var(--color-success);
        position: absolute;
        right: -10px;
        top: 6px;
        animation: blink 1s infinite;
    }
    @keyframes blink {
        0%,
        100% {
            opacity: 1;
        }
        50% {
            opacity: 0;
        }
    }
    /* Semantic Purple for Entities */
    :global(.border-l-purple-500) {
        border-left-color: #a855f7 !important;
    }
    :global(.text-purple-400) {
        color: #c084fc !important;
    }
    :global(.bg-purple-500-10) {
        background-color: rgba(168, 85, 247, 0.05);
    }
</style>

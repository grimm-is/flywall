<script lang="ts">
    /**
     * DVRScrubber.svelte
     * Time-travel control for the Observatory.
     */
    import { dvr, type Snapshot } from "$lib/stores/flows";
    import { Button, Icon } from "$lib/components";
    import { onMount } from "svelte";

    let container: HTMLElement;
    let width = $state(0);
    const height = 60;

    // Subscribe to DVR state
    let history = $derived($dvr.history);
    let isLive = $derived($dvr.isLive);
    let seekTime = $derived($dvr.seekTime);

    // Helper for responsiveness
    onMount(() => {
        const resizeObserver = new ResizeObserver((entries) => {
            width = entries[0].contentRect.width;
        });
        if (container) resizeObserver.observe(container);
        return () => resizeObserver.disconnect();
    });

    // Compute SVG path
    let points = $derived.by(() => {
        if (history.length < 2 || width === 0) return "";

        // Find time range
        const end = history[history.length - 1].timestamp;
        const start = history[0].timestamp;
        const duration = end - start;
        if (duration <= 0) return "";

        // Find max BPS for Y-scale
        const maxBps = Math.max(...history.map((s) => s.total_bps), 1000); // min 1kbps

        return history
            .map((s) => {
                const x = ((s.timestamp - start) / duration) * width;
                const y = height - (s.total_bps / maxBps) * height;
                return `${x.toFixed(1)},${y.toFixed(1)}`;
            })
            .join(" ");
    });

    let areaPath = $derived.by(() => {
        if (!points) return "";
        return `M0,${height} L${points.split(" ").join(" L")} L${width},${height} Z`;
    });

    // Scrubber Position
    let scrubberX = $derived.by(() => {
        if (history.length < 2 || width === 0) return width;
        if (isLive) return width;

        const end = history[history.length - 1].timestamp;
        const start = history[0].timestamp;
        const duration = end - start;

        if (seekTime < start) return 0;
        if (seekTime > end) return width;

        return ((seekTime - start) / duration) * width;
    });

    function handleSeek(e: MouseEvent) {
        if (history.length < 2 || width === 0) return;

        const rect = container.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const percent = Math.max(0, Math.min(1, x / width));

        const end = history[history.length - 1].timestamp;
        const start = history[0].timestamp;
        const duration = end - start;

        const targetTime = start + duration * percent;
        dvr.seek(targetTime);
    }

    function formatTime(ts: number) {
        return new Date(ts).toLocaleTimeString();
    }
</script>

<div class="dvr-container glass-panel flex flex-col gap-2 p-2">
    <div class="dvr-header flex justify-between items-center text-xs">
        <div class="flex items-center gap-2">
            <span class="font-bold text-muted tracking-widest">TIMELINE</span>
            {#if !isLive}
                <span class="text-warning animate-pulse"
                    >REPLAYING: {formatTime(seekTime)}</span
                >
            {/if}
        </div>
        <div class="controls">
            <Button
                size="sm"
                variant={isLive ? "default" : "outline"}
                class="h-6 text-xs px-2"
                onclick={() => dvr.goLive()}
            >
                {#if isLive}<span class="animate-pulse mr-1">●</span>{/if} LIVE
            </Button>
        </div>
    </div>

    <!-- Timeline Graph -->
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
        class="timeline relative h-[60px] bg-background/50 rounded cursor-crosshair overflow-hidden"
        bind:this={container}
        onclick={handleSeek}
    >
        <svg {width} {height} class="absolute inset-0 pointer-events-none">
            <!-- Grid lines -->
            <line
                x1="0"
                y1={height / 2}
                x2={width}
                y2={height / 2}
                stroke="var(--color-border)"
                stroke-dasharray="2 4"
            />

            <!-- Graph -->
            {#if areaPath}
                <path
                    d={areaPath}
                    fill="var(--color-primary)"
                    fill-opacity="0.2"
                />
                <polyline
                    {points}
                    fill="none"
                    stroke="var(--color-primary)"
                    stroke-width="1.5"
                />
            {/if}
        </svg>

        <!-- Scrubber Head -->
        <div
            class="scrubber-head absolute top-0 bottom-0 w-[2px] bg-warning shadow-[0_0_10px_var(--color-warning)] pointer-events-none transition-all duration-75"
            style="left: {scrubberX}px"
            class:hidden={isLive && width > 0}
        ></div>
    </div>
</div>

<style>
    .timeline:hover .scrubber-head {
        /* Could show hover preview head */
        opacity: 0.8;
    }
</style>

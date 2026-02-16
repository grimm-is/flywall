<script lang="ts">
    import { onMount } from "svelte";
    import Icon from "$lib/components/Icon.svelte";
    import {
        dashboardLayout,
        isLayoutEditing,
        type WidgetLayout,
    } from "$lib/stores/ui";

    // Importing Widgets dynamically or statically? Static for now to ensure types.
    import TopologyHeroWidget from "$lib/components/widgets/TopologyHeroWidget.svelte";
    import SystemStatusWidget from "$lib/components/widgets/SystemStatusWidget.svelte";
    import UplinkWidget from "$lib/components/widgets/UplinkWidget.svelte";
    import ZonesWidget from "$lib/components/widgets/ZonesWidget.svelte";

    import BandwidthWidget from "$lib/components/widgets/BandwidthWidget.svelte";
    import ConnectionsWidget from "$lib/components/widgets/ConnectionsWidget.svelte";
    import DNSWidget from "$lib/components/widgets/DNSWidget.svelte";
    import DHCPWidget from "$lib/components/widgets/DHCPWidget.svelte";
    import ServiceHealthWidget from "$lib/components/widgets/ServiceHealthWidget.svelte";
    import AlertFeedWidget from "$lib/components/widgets/AlertFeedWidget.svelte";

    // Map types to components
    const widgetComponents: Record<string, any> = {
        "topology-hero": TopologyHeroWidget,
        "stat-uptime": SystemStatusWidget,
        "stat-cpu": SystemStatusWidget,
        "stat-memory": SystemStatusWidget,
        "stat-disk": SystemStatusWidget,
        uplinks: UplinkWidget,
        zones: ZonesWidget,
        // New Widgets
        bandwidth: BandwidthWidget,
        connections: ConnectionsWidget,
        dns: DNSWidget,
        dhcp: DHCPWidget,
        "service-health": ServiceHealthWidget,
        alerts: AlertFeedWidget,
    };

    // --- Drag & Drop State ---
    let draggedId = $state<string | null>(null);

    function handleDragStart(e: DragEvent, id: string) {
        if (!$isLayoutEditing) return;
        draggedId = id;
        e.dataTransfer?.setData("text/plain", id);
        if (e.dataTransfer) {
            e.dataTransfer.effectAllowed = "move";
        }
    }

    function handleDragOver(e: DragEvent, targetId: string) {
        if (!$isLayoutEditing || !draggedId || draggedId === targetId) return;
        e.preventDefault(); // Allow drop
        if (e.dataTransfer) {
            e.dataTransfer.dropEffect = "move";
        }
    }

    function handleDrop(e: DragEvent, targetId: string) {
        if (!$isLayoutEditing || !draggedId) return;
        e.preventDefault();

        // Swap positions in layout
        const fromIdx = $dashboardLayout.findIndex((w) => w.id === draggedId);
        const toIdx = $dashboardLayout.findIndex((w) => w.id === targetId);

        if (fromIdx !== -1 && toIdx !== -1) {
            const layout = [...$dashboardLayout];
            // Simple swap of order for now, assumes auto-flow grid
            // Ideally we'd swap x/y if we were manually positioning,
            // but CSS Grid auto-flow is easier for "masonry-like" behavior.
            // Let's swap the array order to change visual order.
            const temp = layout[fromIdx];
            layout.splice(fromIdx, 1);
            layout.splice(toIdx, 0, temp);

            $dashboardLayout = layout; // Store updates
        }

        draggedId = null;
    }

    function removeWidget(id: string) {
        $dashboardLayout = $dashboardLayout.filter((w) => w.id !== id);
    }

    function resetLayout() {
        $dashboardLayout = [
            // Row 1: Hero
            { id: "hero", type: "topology-hero", x: 0, y: 0, w: 4, h: 2 },

            // Row 2: Live Status & Activity
            { id: "health", type: "service-health", x: 0, y: 2, w: 1, h: 1 },
            { id: "bandwidth", type: "bandwidth", x: 1, y: 2, w: 1, h: 1 },
            { id: "conns", type: "connections", x: 2, y: 2, w: 1, h: 1 },
            { id: "alerts", type: "alerts", x: 3, y: 2, w: 1, h: 1 },

            // Row 3: Network Services & Hardware
            { id: "dns", type: "dns", x: 0, y: 3, w: 1, h: 1 },
            { id: "dhcp", type: "dhcp", x: 1, y: 3, w: 1, h: 1 },
            { id: "stats-cpu", type: "stat-cpu", x: 2, y: 3, w: 1, h: 1 },
            { id: "stats-mem", type: "stat-memory", x: 3, y: 3, w: 1, h: 1 },

            // Row 4: Uplinks
            { id: "uplinks", type: "uplinks", x: 0, y: 4, w: 4, h: 1 },

            // Row 5: Zones
            { id: "zones", type: "zones", x: 0, y: 5, w: 4, h: 2 },
        ];
    }
</script>

<div class="dashboard-page">
    <header class="page-header">
        <div class="page-title">
            <h1>Dashboard</h1>
        </div>
        <div class="header-actions">
            {#if $isLayoutEditing}
                <button class="btn-destructive" onclick={() => resetLayout()}>
                    <Icon name="refresh" size={16} /> Reset
                </button>
                <button
                    class="btn-primary"
                    onclick={() => ($isLayoutEditing = false)}
                >
                    <Icon name="check" size={16} /> Done
                </button>
            {:else}
                <button
                    class="btn-secondary"
                    onclick={() => ($isLayoutEditing = true)}
                >
                    <Icon name="edit" size={16} /> Customize
                </button>
            {/if}
        </div>
    </header>

    <div class="dashboard-grid" class:editing={$isLayoutEditing}>
        {#each $dashboardLayout as widget (widget.id)}
            <!--
                 Note: We use style mapping for grid-column/row spanning.
                 Example: w=2 -> grid-column: span 2
            -->
            <div
                class="widget-wrapper"
                style="grid-column: span {widget.w}; grid-row: span {widget.h};"
                draggable={$isLayoutEditing}
                ondragstart={(e) => handleDragStart(e, widget.id)}
                ondragover={(e) => handleDragOver(e, widget.id)}
                ondrop={(e) => handleDrop(e, widget.id)}
                role={$isLayoutEditing ? "button" : undefined}
                tabindex={$isLayoutEditing ? 0 : undefined}
            >
                <svelte:component
                    this={widgetComponents[widget.type]}
                    type={widget.type}
                    onremove={() => removeWidget(widget.id)}
                />
            </div>
        {/each}

        {#if $isLayoutEditing}
            <div class="add-widget-placeholder">
                <span>+ Add Widget (Coming Soon)</span>
            </div>
        {/if}
    </div>
</div>

<style>
    .dashboard-page {
        display: flex;
        flex-direction: column;
        gap: var(--space-6);
        max-width: 1600px; /* Prevent overstretching on ultra-wide */
        margin: 0 auto;
    }

    .page-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
    }

    .page-title h1 {
        font-size: var(--text-2xl);
        font-weight: 600;
        color: var(--dashboard-text);
    }

    .header-actions {
        display: flex;
        gap: var(--space-2);
    }

    .btn-secondary,
    .btn-primary,
    .btn-destructive {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-2) var(--space-4);
        border-radius: var(--radius-md);
        font-size: var(--text-sm);
        cursor: pointer;
        border: 1px solid transparent;
    }

    .btn-secondary {
        background: var(--dashboard-card);
        border-color: var(--dashboard-border);
        color: var(--dashboard-text);
    }

    .btn-primary {
        background: var(--color-primary);
        color: white;
    }

    .btn-destructive {
        background: transparent;
        border-color: var(--color-destructive);
        color: var(--color-destructive);
    }

    /* Grid Layout */
    .dashboard-grid {
        display: grid;
        grid-template-columns: repeat(4, 1fr); /* 4-column layout */
        gap: var(--space-4);
        auto-rows: minmax(180px, auto); /* Minimum row height */
        padding-bottom: 100px;
    }

    .widget-wrapper {
        height: 100%;
    }

    /* Mobile Responsive */
    @media (max-width: 1024px) {
        .dashboard-grid {
            grid-template-columns: repeat(2, 1fr); /* 2 columns on tablet */
        }
    }

    @media (max-width: 768px) {
        .dashboard-grid {
            display: flex; /* Stack on mobile */
            flex-direction: column;
        }

        .widget-wrapper {
            width: 100% !important; /* Force full width override */
            height: auto;
            min-height: 200px;
        }
    }

    .add-widget-placeholder {
        grid-column: span 1;
        border: 2px dashed var(--dashboard-border);
        border-radius: var(--radius-lg);
        display: flex;
        align-items: center;
        justify-content: center;
        color: var(--dashboard-text-muted);
        min-height: 180px;
    }
</style>

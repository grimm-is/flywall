<script lang="ts">
    /**
     * AddressPill - Smart badge for displaying resolved addresses with popovers
     * Dashboard-native styling using CSS variables
     */
    interface ResolvedAddress {
        display_name: string;
        type: string;
        description?: string;
        count: number;
        is_truncated?: boolean;
        preview?: string[];
    }

    interface Props {
        resolved?: ResolvedAddress | null;
        raw?: string;
        size?: "sm" | "md";
    }

    let { resolved = null, raw = "", size = "sm" }: Props = $props();

    let showTooltip = $state(false);
    let tooltipTimeout: ReturnType<typeof setTimeout> | null = null;

    function handleMouseEnter() {
        tooltipTimeout = setTimeout(() => {
            showTooltip = true;
        }, 300);
    }

    function handleMouseLeave() {
        if (tooltipTimeout) {
            clearTimeout(tooltipTimeout);
            tooltipTimeout = null;
        }
        showTooltip = false;
    }

    let displayName = $derived(resolved?.display_name || raw || "Any");
    let pillType = $derived(resolved?.type || "raw");
</script>

<div
    class="pill-container"
    onmouseenter={handleMouseEnter}
    onmouseleave={handleMouseLeave}
    role="button"
    tabindex="0"
>
    <!-- Pill Badge -->
    <div class="pill {pillType} {size}">
        <span class="pill-name">{displayName}</span>
        {#if resolved && resolved.count > 1}
            <span class="pill-count">({resolved.count})</span>
        {/if}
    </div>

    <!-- Tooltip Popover -->
    {#if showTooltip && resolved && resolved.type !== "any"}
        <div class="tooltip" role="tooltip">
            <div class="tooltip-title">{resolved.display_name}</div>
            <div class="tooltip-meta">
                <span class="tooltip-type"
                    >{resolved.type.replace("_", " ")}</span
                >
                {#if resolved.description}
                    <span class="tooltip-desc">{resolved.description}</span>
                {/if}
            </div>

            {#if resolved.preview && resolved.preview.length > 0}
                <div class="tooltip-preview">
                    {#each resolved.preview as item}
                        <div class="preview-item">{item}</div>
                    {/each}
                    {#if resolved.is_truncated}
                        <div class="preview-more">
                            ...and {resolved.count -
                                (resolved.preview?.length || 0)} more
                        </div>
                    {/if}
                </div>
            {/if}
        </div>
    {/if}
</div>

<style>
    .pill-container {
        position: relative;
        display: inline-block;
    }

    .pill {
        display: inline-flex;
        align-items: center;
        gap: var(--space-1);
        border-radius: var(--radius-sm);
        border: 1px solid;
        cursor: help;
        transition: filter var(--transition-fast);
    }

    .pill:hover {
        filter: brightness(1.1);
    }

    /* Sizes */
    .pill.sm {
        font-size: var(--text-xs);
        padding: 2px var(--space-2);
    }

    .pill.md {
        font-size: var(--text-sm);
        padding: var(--space-1) var(--space-3);
    }

    .pill-name {
        font-weight: 500;
    }

    .pill-count {
        opacity: 0.6;
    }

    /* Type-based colors using CSS variables */
    .pill.device_named {
        color: var(--color-primary);
        background: rgba(59, 130, 246, 0.15);
        border-color: rgba(59, 130, 246, 0.4);
    }

    .pill.device_auto {
        color: #06b6d4;
        background: rgba(6, 182, 212, 0.15);
        border-color: rgba(6, 182, 212, 0.4);
    }

    .pill.device_vendor,
    .pill.host,
    .pill.ip,
    .pill.raw {
        color: var(--dashboard-text);
        background: var(--dashboard-input);
        border-color: var(--dashboard-border);
    }

    .pill.alias,
    .pill.ipset {
        color: #a855f7;
        background: rgba(168, 85, 247, 0.15);
        border-color: rgba(168, 85, 247, 0.4);
    }

    .pill.zone {
        color: var(--color-warning);
        background: rgba(245, 158, 11, 0.15);
        border-color: rgba(245, 158, 11, 0.4);
    }

    .pill.cidr {
        color: var(--color-success);
        background: rgba(34, 197, 94, 0.15);
        border-color: rgba(34, 197, 94, 0.4);
    }

    .pill.service,
    .pill.port {
        color: #ec4899;
        background: rgba(236, 72, 153, 0.15);
        border-color: rgba(236, 72, 153, 0.4);
    }

    .pill.any {
        color: var(--dashboard-text-muted);
        background: var(--dashboard-canvas);
        border-color: var(--dashboard-border);
        font-style: italic;
    }

    /* Tooltip */
    .tooltip {
        position: absolute;
        z-index: var(--z-dropdown);
        bottom: 100%;
        left: 0;
        margin-bottom: var(--space-2);
        width: 16rem;
        background: var(--dashboard-card);
        border: 1px solid var(--dashboard-border);
        border-radius: var(--radius-lg);
        padding: var(--space-3);
        box-shadow: var(--shadow-lg);
        font-size: var(--text-xs);
        pointer-events: none;
    }

    .tooltip-title {
        font-weight: 700;
        color: var(--dashboard-text);
        margin-bottom: var(--space-1);
    }

    .tooltip-meta {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        color: var(--dashboard-text-muted);
        margin-bottom: var(--space-2);
    }

    .tooltip-type {
        padding: 2px var(--space-1);
        background: var(--dashboard-input);
        border-radius: var(--radius-sm);
        text-transform: uppercase;
        font-size: 10px;
        letter-spacing: 0.05em;
    }

    .tooltip-desc {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .tooltip-preview {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
        max-height: 8rem;
        overflow-y: auto;
    }

    .preview-item {
        font-family: var(--font-mono);
        padding: 2px var(--space-1);
        background: var(--dashboard-canvas);
        border-radius: var(--radius-sm);
        font-size: 11px;
        color: var(--dashboard-text);
    }

    .preview-more {
        color: var(--dashboard-text-muted);
        font-style: italic;
    }
</style>

<script lang="ts">
    import Icon from "$lib/components/Icon.svelte";
    import { isLayoutEditing } from "$lib/stores/ui";

    let { title, icon = "widgets", onremove, children } = $props();

    // Determine grid span classes based on props (handled by parent grid usually,
    // but we might want internal responsive logic)
</script>

<div class="widget-card" class:editing={$isLayoutEditing}>
    <header class="widget-header">
        <div class="widget-title">
            {#if icon}
                <Icon name={icon} size={18} />
            {/if}
            <span>{title}</span>
        </div>

        {#if $isLayoutEditing}
            <div class="widget-controls">
                <button
                    class="control-btn remove"
                    onclick={onremove}
                    aria-label="Remove widget"
                >
                    <Icon name="close" size={16} />
                </button>
                <div class="drag-handle" aria-label="Drag widget">
                    <Icon name="drag_indicator" size={16} />
                </div>
            </div>
        {/if}
    </header>

    <div class="widget-content">
        {@render children()}
    </div>
</div>

<style>
    .widget-card {
        background: var(--dashboard-card);
        border: 1px solid var(--dashboard-border);
        border-radius: var(--radius-lg);
        display: flex;
        flex-direction: column;
        height: 100%;
        overflow: hidden;
        transition:
            border-color 0.2s,
            box-shadow 0.2s;
        position: relative;
    }

    .widget-card.editing {
        border-style: dashed;
        border-color: var(--color-primary);
        background: var(--dashboard-canvas);
    }

    .widget-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: var(--space-3) var(--space-4);
        border-bottom: 1px solid var(--dashboard-border);
        background: var(--color-backgroundSecondary);
    }

    .widget-title {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        font-weight: 600;
        font-size: var(--text-sm);
        color: var(--dashboard-text);
    }

    .widget-content {
        flex: 1;
        overflow: auto;
        padding: var(--space-4);
        /* Scrollbar styling */
        scrollbar-width: thin;
        scrollbar-color: var(--dashboard-border) transparent;
    }

    /* Editing Controls */
    .widget-controls {
        display: flex;
        align-items: center;
        gap: var(--space-2);
    }

    .control-btn {
        background: none;
        border: none;
        color: var(--color-destructive);
        cursor: pointer;
        padding: 4px;
        border-radius: 4px;
        display: flex;
        transition: background 0.2s;
    }

    .control-btn:hover {
        background: rgba(239, 68, 68, 0.1);
    }

    .drag-handle {
        cursor: move;
        color: var(--dashboard-text-muted);
        padding: 4px;
        border-radius: 4px;
    }

    .drag-handle:hover {
        background: var(--dashboard-input);
        color: var(--dashboard-text);
    }
</style>

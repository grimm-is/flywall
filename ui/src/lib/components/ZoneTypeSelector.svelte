<script lang="ts">
    import { t } from "svelte-i18n";
    import Icon from "$lib/components/Icon.svelte";
    import { createEventDispatcher } from "svelte";

    export let value: boolean = false; // false = internal, true = external

    // The dispatch is kept as per the provided instruction snippet,
    // though it's not used in the new setType function.
    const dispatch = createEventDispatcher();

    function setType(isExternal: boolean) {
        // If the value changes, update it.
        // Svelte's two-way binding will handle prop updates to the parent.
        if (value !== isExternal) {
            value = isExternal;
            // If a custom 'change' event is still desired for parent components
            // that don't use two-way binding, it could be re-added here.
            // For now, it's removed as per the instruction's setType function.
        }
    }
</script>

<div class="zone-type-selector">
    <button
        type="button"
        class="type-option"
        class:active={!value}
        onclick={() => setType(false)}
    >
        <div class="icon-wrapper">
            <Icon name="home" size="md" />
        </div>
        <div class="text-wrapper">
            <span class="label">{$t("zones.internal_zone")}</span>
            <span class="description"
                >{$t("zones.internal_zone_desc", {
                    default: "Trusted local traffic",
                })}</span
            >
        </div>
    </button>

    <button
        type="button"
        class="type-option"
        class:active={value}
        onclick={() => setType(true)}
    >
        <div class="icon-wrapper">
            <Icon name="cloud" size="md" />
        </div>
        <div class="text-wrapper">
            <span class="label">{$t("zones.external_zone")}</span>
            <span class="description">{$t("zones.external_zone_desc")}</span>
        </div>
    </button>
</div>

<style>
    .zone-type-selector {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: var(--space-4);
        margin-bottom: var(--space-2);
    }

    .type-option {
        display: flex;
        align-items: flex-start;
        gap: var(--space-3);
        padding: var(--space-3);
        background: var(--color-surface);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        cursor: pointer;
        text-align: left;
        transition: all 0.2s ease;
    }

    .type-option:hover {
        border-color: var(--color-border-hover);
        background: var(--color-surface-hover);
    }

    .type-option.active {
        background: var(--color-primary-subtle);
        border-color: var(--color-primary);
        color: var(--color-primary);
    }

    .icon-wrapper {
        display: flex;
        align-items: center;
        justify-content: center;
        padding: var(--space-2);
        background: var(--color-background);
        border-radius: 50%;
        color: var(--color-muted-foreground);
    }

    .type-option.active .icon-wrapper {
        background: var(--color-background);
        color: var(--color-primary);
    }

    .text-wrapper {
        display: flex;
        flex-direction: column;
        gap: 2px;
    }

    .label {
        font-size: var(--text-sm);
        font-weight: 500;
    }

    .description {
        font-size: var(--text-xs);
        color: var(--color-muted-foreground);
        line-height: 1.3;
    }

    .type-option.active .description {
        color: var(--color-primary);
        opacity: 0.8;
    }
</style>

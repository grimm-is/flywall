<script lang="ts">
    import { t } from "svelte-i18n";
    import { Icon } from "$lib/components";

    export let value: string = "blue";
    export let disabled: boolean = false;

    const presets = [
        { name: "red", label: "Red", cssVar: "var(--zone-red)" },
        { name: "green", label: "Green", cssVar: "var(--zone-green)" },
        { name: "blue", label: "Blue", cssVar: "var(--zone-blue)" },
        { name: "orange", label: "Orange", cssVar: "var(--zone-orange)" },
        { name: "purple", label: "Purple", cssVar: "var(--zone-purple)" },
        { name: "cyan", label: "Cyan", cssVar: "var(--zone-cyan)" },
        { name: "gray", label: "Gray", cssVar: "var(--zone-gray)" },
    ];

    function isPreset(val: string) {
        return presets.some((p) => p.name === val);
    }

    // Initialize custom color to value if it's not a preset, otherwise default to black/white for picker
    let customColor = isPreset(value) ? "#000000" : value;

    // Watch for external value changes
    $: if (!isPreset(value) && value !== customColor) {
        customColor = value;
    }

    function selectPreset(name: string) {
        if (disabled) return;
        value = name;
    }

    function handleCustomChange(e: Event) {
        const target = e.target as HTMLInputElement;
        value = target.value;
    }
</script>

<div class="color-selector">
    <label class="selector-label">{$t("zones.color")}</label>
    <div class="swatches">
        {#each presets as preset}
            <button
                type="button"
                class="swatch"
                class:active={value === preset.name}
                style="--swatch-color: {preset.cssVar}"
                title={preset.label}
                onclick={() => selectPreset(preset.name)}
                {disabled}
            >
                {#if value === preset.name}
                    <Icon name="check" size="sm" class="check-icon" />
                {/if}
            </button>
        {/each}

        <div class="custom-wrapper" class:active={!isPreset(value)}>
            <input
                type="color"
                class="color-input"
                bind:value={customColor}
                oninput={handleCustomChange}
                {disabled}
                title={$t("common.custom")}
                aria-label={$t("zones.custom_color")}
            />
            {#if !isPreset(value)}
                <div class="check-overlay">
                    <Icon name="check" size="sm" />
                </div>
            {/if}
        </div>
    </div>
    <div class="current-value">
        {#if isPreset(value)}
            <span class="value-text"
                >{presets.find((p) => p.name === value)?.label}</span
            >
        {:else}
            <span class="value-text">{value}</span>
        {/if}
    </div>
</div>

<style>
    .color-selector {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
    }

    .selector-label {
        font-size: var(--text-sm);
        font-weight: 500;
        color: var(--color-foreground);
    }

    .swatches {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-2);
    }

    .swatch {
        width: 32px;
        height: 32px;
        border-radius: 50%;
        border: 2px solid transparent;
        background-color: var(--swatch-color);
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: all 0.2s;
        padding: 0;
    }

    .swatch:hover:not(:disabled) {
        transform: scale(1.1);
    }

    .swatch.active {
        border-color: var(--color-foreground);
        box-shadow:
            0 0 0 2px var(--color-background),
            0 0 0 4px var(--swatch-color);
    }

    .swatch:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    /* Target the icon inside */
    :global(.swatch .check-icon) {
        color: white;
        filter: drop-shadow(0 1px 1px rgba(0, 0, 0, 0.5));
    }

    .custom-wrapper {
        position: relative;
        width: 32px;
        height: 32px;
        border-radius: 50%;
        overflow: hidden;
        border: 2px solid var(--color-border);
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: pointer;
        background: conic-gradient(red, yellow, lime, cyan, blue, magenta, red);
    }

    .custom-wrapper.active {
        border-color: var(--color-foreground);
        box-shadow: 0 0 0 2px var(--color-background);
    }

    .color-input {
        position: absolute;
        top: -50%;
        left: -50%;
        width: 200%;
        height: 200%;
        padding: 0;
        margin: 0;
        border: none;
        cursor: pointer;
        opacity: 0; /* Hidden but clickable */
    }

    /* Make input visible only for the color value when active if desirable,
       but standard approach is usually invisible click target over a nice generic icon/gradient */

    .check-overlay {
        pointer-events: none;
        color: white;
        filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.8));
        z-index: 2;
    }

    .current-value {
        font-size: var(--text-xs);
        color: var(--color-muted);
        min-height: 1.2em;
    }
</style>

<script lang="ts">
    import BaseWidget from "./BaseWidget.svelte";
    import ZoneStatusCard from "$lib/components/ZoneStatusCard.svelte";
    import { zones } from "$lib/stores/zones";
    import Icon from "$lib/components/Icon.svelte";

    let { onremove } = $props();
</script>

<BaseWidget title="Active Zones" icon="shield" {onremove}>
    <div class="zones-grid">
        {#each $zones as zone (zone.name)}
            <div class="zone-wrapper">
                <ZoneStatusCard {zone} />
            </div>
        {:else}
            <div class="empty-state">
                <Icon name="hub" size={32} />
                <p>No zones configured</p>
            </div>
        {/each}
    </div>
</BaseWidget>

<style>
    .zones-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
        gap: var(--space-4);
        height: 100%;
    }

    .zone-wrapper {
        /* Wrapper to handle potential layout issues inside the widget scroll area */
    }

    .empty-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        height: 100%;
        color: var(--dashboard-text-muted);
        gap: var(--space-2);
    }
</style>

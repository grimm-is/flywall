<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { Icon } from "$lib/components";
    import { t } from "svelte-i18n";

    export let services: Record<string, boolean> = {};
    export let type: "management" | "network";

    const dispatch = createEventDispatcher();

    const mgmtItems = [
        { key: "web", icon: "web", label: "zones.svc.web" },
        { key: "ssh", icon: "ssh", label: "zones.svc.ssh" },
        { key: "api", icon: "api", label: "zones.svc.api" },
        { key: "icmp", icon: "icmp", label: "zones.svc.icmp" },
    ];

    const netItems = [
        { key: "dhcp", icon: "dhcp", label: "zones.svc.dhcp" },
        { key: "dns", icon: "dns", label: "zones.svc.dns" },
        { key: "ntp", icon: "ntp", label: "zones.svc.ntp" },
    ];

    const items = type === "management" ? mgmtItems : netItems;

    function toggle(key: string) {
        const newVal = !services[key];
        dispatch("change", { ...services, [key]: newVal });
    }
</script>

<div class="service-grid">
    {#each items as item}
        <button
            type="button"
            class="service-tile"
            class:active={services[item.key]}
            onclick={() => toggle(item.key)}
        >
            <div class="tile-header">
                <Icon name={item.icon} size={20} />
                <div class="status-indicator">
                    <div class="status-dot"></div>
                </div>
            </div>
            <span class="tile-label">{$t(item.label)}</span>
        </button>
    {/each}
</div>

<style>
    .service-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(80px, 1fr));
        gap: var(--space-2);
    }

    .service-tile {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        padding: var(--space-3);
        background-color: var(--color-background);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        cursor: pointer;
        color: var(--color-muted-foreground);
        transition: all 0.2s ease;
        height: 80px;
        justify-content: space-between;
    }

    .service-tile:hover {
        border-color: var(--color-primary);
        background-color: var(--color-surface-hover);
    }

    .service-tile.active {
        background-color: var(--color-primary-transparent);
        border-color: var(--color-primary);
    }

    .tile-header {
        display: flex;
        justify-content: space-between;
        width: 100%;
        align-items: flex-start;
        margin-bottom: var(--space-2);
    }

    .status-indicator {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        background-color: var(--color-border);
        transition: background-color 0.2s ease;
    }

    .service-tile.active .status-indicator {
        background-color: var(--color-primary);
        box-shadow: 0 0 5px var(--color-primary);
    }

    .tile-label {
        font-size: var(--text-xs);
        font-weight: 500;
        color: var(--color-foreground);
    }

    .service-tile.active .tile-label {
        color: var(--color-primary);
        font-weight: 600;
    }
</style>

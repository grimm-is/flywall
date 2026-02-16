<script lang="ts">
    import { config } from "$lib/stores/app";
    import { t } from "svelte-i18n";

    import { Icon } from "$lib/components";

    let { name, size = "sm" } = $props<{ name: string; size?: "sm" | "md" }>();

    const zones = $derived($config?.zones || []);

    const zone = $derived(zones.find((z: any) => z.name === name));

    function getZoneColorStyle(z: any): string {
        if (!z) return "--zone-color: var(--color-muted)";
        if (z.color?.startsWith("#")) {
            return `--zone-color: ${z.color}`;
        }
        return `--zone-color: var(--zone-${z.color}, var(--color-muted))`;
    }
</script>

<a
    href="/network?tab=zones#zone-{name}"
    class="zone-link {size}"
    style={getZoneColorStyle(zone)}
    title={$t("common.view_details")}
>
    <Icon name="domain" size={size === "md" ? 16 : 14} />
    {name || $t("common.none")}
</a>

<style>
    .zone-link {
        display: inline-flex;
        align-items: center;
        gap: var(--space-1);
        padding: var(--space-1) var(--space-2);
        background-color: var(--zone-color, var(--color-muted));
        color: white;
        font-weight: 500;
        font-size: var(--text-xs);
        border-radius: var(--radius-sm);
        text-decoration: none;
        transition: opacity 0.2s;
        line-height: 1;
    }

    .zone-link.md {
        font-size: var(--text-sm);
        padding: var(--space-1) var(--space-3);
    }

    /* Hover effect to show it's clickable */
    .zone-link:hover {
        opacity: 0.9;
        text-decoration: underline;
    }
</style>

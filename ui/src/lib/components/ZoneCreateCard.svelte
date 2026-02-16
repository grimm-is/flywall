<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { t } from "svelte-i18n";
    import { Card, Button, Input, Spinner, Badge } from "$lib/components";
    import ZoneColorSelector from "$lib/components/ZoneColorSelector.svelte";
    import ZoneSelectorEditor from "$lib/components/ZoneSelectorEditor.svelte";
    import ServiceTileGrid from "$lib/components/ServiceTileGrid.svelte";
    import ZoneTypeSelector from "$lib/components/ZoneTypeSelector.svelte";

    export let availableInterfaceNames: string[] = [];
    export let availableIPSetNames: string[] = [];
    export let loading = false;

    const dispatch = createEventDispatcher();

    // Form state
    let zoneName = "";
    let zoneColor = "blue";
    let zoneDescription = "";
    let zoneExternal = false;
    let zoneSelectors: any[] = [];

    let management = {
        web: false,
        ssh: false,
        api: false,
        icmp: false,
    };

    let services = {
        dhcp: false,
        dns: false,
        ntp: false,
    };

    function handleSave() {
        if (!zoneName) return;

        dispatch("save", {
            name: zoneName,
            color: zoneColor,
            description: zoneDescription,
            external: zoneExternal,
            matches: zoneSelectors,
            management,
            services,
        });
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card class="create-card border-l-4 border-l-primary">
    <div class="card-header">
        <h3>{$t("common.add_item", { values: { item: $t("item.zone") } })}</h3>
        <Badge variant="outline">New Zone</Badge>
    </div>

    <div class="form-stack">
        <div class="grid grid-cols-2 gap-4">
            <Input
                id="new-zone-name"
                label={$t("zones.zone_name")}
                bind:value={zoneName}
                placeholder="e.g., Guest"
                required
            />

            <ZoneColorSelector bind:value={zoneColor} />
        </div>

        <Input
            id="new-zone-desc"
            label={$t("common.description")}
            bind:value={zoneDescription}
            placeholder="e.g., Guest network for visitors"
        />

        <div class="selector-section">
            <h3 class="text-sm font-medium text-foreground mb-2">Selectors</h3>
            <ZoneSelectorEditor
                bind:matches={zoneSelectors}
                availableInterfaces={availableInterfaceNames}
                availableIPSets={availableIPSetNames}
            />
        </div>

        <div class="space-y-4">
            <h3 class="text-sm font-medium text-foreground">
                {$t("zones.zone_type")}
            </h3>
            <ZoneTypeSelector bind:value={zoneExternal} />
        </div>

        <div class="grid grid-cols-2 gap-6">
            <div class="space-y-3">
                <div class="flex flex-col gap-1">
                    <h3 class="text-sm font-medium text-foreground">
                        {$t("zones.management_access")}
                    </h3>
                    <span class="text-xs text-muted-foreground"
                        >Applies only to traffic matching this zone</span
                    >
                </div>
                <ServiceTileGrid
                    type="management"
                    on:change={(e) => (management = e.detail)}
                    services={management}
                />
            </div>

            <div class="space-y-3">
                <div class="flex flex-col gap-1">
                    <h3 class="text-sm font-medium text-foreground">
                        {$t("zones.network_services")}
                    </h3>
                    <span class="text-xs text-muted-foreground"
                        >Applies only to traffic matching this zone</span
                    >
                </div>
                <ServiceTileGrid
                    type="network"
                    on:change={(e) => (services = e.detail)}
                    {services}
                />
            </div>
        </div>

        <div class="actions">
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button onclick={handleSave} disabled={loading || !zoneName}>
                {#if loading}<Spinner size="sm" />{/if}
                {$t("common.save")}
            </Button>
        </div>
    </div>
</Card>

<style>
    .card-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: var(--space-4);
    }

    .card-header h3 {
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
    }

    .form-stack {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
    }

    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        padding-top: var(--space-4);
        border-top: 1px solid var(--color-border);
    }

    .selector-section {
        margin-bottom: var(--space-2);
    }
</style>

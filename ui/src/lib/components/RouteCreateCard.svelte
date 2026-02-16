<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { Card, Button, Input, Select, Spinner } from "$lib/components";
    import { t } from "svelte-i18n";

    export let loading = false;
    export let interfaces: any[] = [];

    const dispatch = createEventDispatcher();

    let destination = "";
    let gateway = "";
    let routeInterface = "";
    let metric = "100";

    // Pre-select first interface or empty
    $: if (!routeInterface && interfaces.length > 0) {
        // Default to nothing or first? Typically route interface is optional (auto) or explicit.
        // Keeping it text/select. `options` will include "Auto".
    }

    function handleSave() {
        if (!destination) return;
        if (!gateway && !routeInterface) {
            // Validation error: need gateway or interface or both
            // But some systems allow just dest? Usually static route needs gw or dev.
            // Existing logic in Routing.svelte: `if (!routeDestination || (!routeGateway && !routeInterface)) return;`
            return;
        }

        dispatch("save", {
            destination,
            gateway,
            interface: routeInterface,
            metric,
        });
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card>
    <div class="create-card">
        <div class="header">
            <h3>
                {$t("common.add_item", {
                    values: { item: $t("item.static_route") },
                })}
            </h3>
        </div>

        <div class="form-grid">
            <Input
                id="route-dest"
                label={$t("routing.destination_cidr")}
                bind:value={destination}
                placeholder="e.g., 10.0.0.0/8"
                required
            />

            <Input
                id="route-gateway"
                label={$t("common.gateway")}
                bind:value={gateway}
                placeholder="e.g., 192.168.1.254"
            />

            <Select
                id="route-interface"
                label={$t("common.interface")}
                bind:value={routeInterface}
                options={[
                    { value: "", label: $t("routing.auto") },
                    ...interfaces.map((i: any) => ({
                        value: i.Name,
                        label: i.Name,
                    })),
                ]}
            />

            <Input
                id="route-metric"
                label={$t("routing.metric")}
                bind:value={metric}
                placeholder="100"
                type="number"
            />
        </div>

        <div class="actions">
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button
                variant="default"
                onclick={handleSave}
                disabled={loading ||
                    !destination ||
                    (!gateway && !routeInterface)}
            >
                {#if loading}<Spinner size="sm" />{/if}
                {$t("common.add_item", {
                    values: { item: $t("item.static_route") },
                })}
            </Button>
        </div>
    </div>
</Card>

<style>
    .create-card {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
        padding: var(--space-2);
    }

    .header h3 {
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
    }

    .form-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
        gap: var(--space-4);
    }

    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        margin-top: var(--space-2);
        padding-top: var(--space-4);
        border-top: 1px solid var(--color-border);
    }
</style>

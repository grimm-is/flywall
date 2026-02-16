<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { Card, Button, Input, Select, Spinner } from "$lib/components";
    import { t } from "svelte-i18n";

    export let loading = false;
    export let route: any = null;
    export let interfaces: any[] = [];

    const dispatch = createEventDispatcher();

    let destination = route?.destination || "";
    let gateway = route?.gateway || "";
    let iface = route?.interface || "";
    let metric = route?.metric?.toString() || "";

    $: if (route) {
        destination = route.destination || "";
        gateway = route.gateway || "";
        iface = route.interface || "";
        metric = route.metric?.toString() || "";
    }

    function handleSave() {
        if (!destination) return;
        dispatch("save", {
            destination,
            gateway: gateway || undefined,
            interface: iface || undefined,
            metric: metric ? parseInt(metric) : undefined,
        });
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card>
    <div class="create-card">
        <div class="header">
            <h4>
                {route
                    ? $t("common.edit_item", {
                          values: { item: $t("item.static_route") },
                      })
                    : $t("common.add_item", {
                          values: { item: $t("item.static_route") },
                      })}
            </h4>
        </div>
        <div class="form-stack">
            <Input
                id="route-dest"
                label={$t("routing.destination_cidr")}
                bind:value={destination}
                placeholder="e.g., 10.0.0.0/8"
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
                bind:value={iface}
                options={[
                    { value: "", label: $t("routing.auto") },
                    ...interfaces.map((i) => ({
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
            <Button onclick={handleSave} disabled={loading || !destination}>
                {#if loading}<Spinner size="sm" />{/if}
                {$t("common.save")}
            </Button>
        </div>
    </div>
</Card>

<style>
    .create-card {
        display: flex;
        flex-direction: column;
        gap: var(--space-3);
        padding: var(--space-2);
    }
    .header h4 {
        margin: 0;
        font-size: var(--text-md);
        font-weight: 600;
    }
    .form-stack {
        display: flex;
        flex-direction: column;
        gap: var(--space-3);
    }
    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        border-top: 1px solid var(--color-border);
        padding-top: var(--space-3);
    }
</style>

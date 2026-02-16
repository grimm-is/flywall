<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { t } from "svelte-i18n";
    import {
        Card,
        Button,
        Input,
        Select,
        Spinner,
        Badge,
    } from "$lib/components";
    import StaticReservationsEditor from "$lib/components/StaticReservationsEditor.svelte";

    export let interfaces: any[] = [];
    export let loading = false;

    const dispatch = createEventDispatcher();

    // Form state
    let scopeName = "";
    let scopeInterface = "";
    let scopeRangeStart = "";
    let scopeRangeEnd = "";
    let scopeRouter = "";
    let scopeDns = "";
    let scopeLeaseTime = "24h";
    let scopeDomain = "";
    let scopeReservations: any[] = [];

    // Default interface
    $: if (interfaces.length > 0 && !scopeInterface) {
        scopeInterface = interfaces[0].Name;
    }

    function handleSave() {
        if (!scopeName || !scopeInterface || !scopeRangeStart || !scopeRangeEnd)
            return;

        dispatch("save", {
            name: scopeName,
            interface: scopeInterface,
            range_start: scopeRangeStart,
            range_end: scopeRangeEnd,
            router: scopeRouter || undefined,
            dns: scopeDns
                ? scopeDns
                      .split(",")
                      .map((s) => s.trim())
                      .filter(Boolean)
                : undefined,
            lease_time: scopeLeaseTime,
            domain: scopeDomain || undefined,
            reservations: scopeReservations,
        });
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card class="create-card border-l-4 border-l-primary">
    <div class="card-header">
        <h3>{$t("common.add_item", { values: { item: $t("item.scope") } })}</h3>
        <Badge variant="outline">New Scope</Badge>
    </div>

    <div class="form-stack">
        <Input
            id="new-scope-name"
            label={$t("dhcp.scope_name")}
            bind:value={scopeName}
            placeholder={$t("dhcp.scope_name_placeholder")}
            required
        />

        <Select
            id="new-scope-iface"
            label={$t("dhcp.interface")}
            bind:value={scopeInterface}
            options={interfaces.map((i: any) => ({
                value: i.Name,
                label: i.Name,
            }))}
            required
        />

        <div class="grid grid-cols-2 gap-4">
            <Input
                id="new-scope-start"
                label={$t("dhcp.range_start")}
                bind:value={scopeRangeStart}
                placeholder="192.168.1.100"
                required
            />
            <Input
                id="new-scope-end"
                label={$t("dhcp.range_end")}
                bind:value={scopeRangeEnd}
                placeholder="192.168.1.200"
                required
            />
        </div>

        <Input
            id="new-scope-router"
            label={$t("dhcp.router_optional")}
            bind:value={scopeRouter}
            placeholder="192.168.1.1"
        />

        <div class="grid grid-cols-2 gap-4">
            <Input
                id="new-scope-lease"
                label={$t("dhcp.lease_time")}
                bind:value={scopeLeaseTime}
                placeholder="24h"
            />
            <Input
                id="new-scope-domain"
                label={$t("dhcp.domain")}
                bind:value={scopeDomain}
                placeholder="lan"
            />
        </div>

        <Input
            id="new-scope-dns"
            label={$t("dhcp.dns_servers")}
            bind:value={scopeDns}
            placeholder="1.1.1.1, 8.8.8.8"
        />

        <StaticReservationsEditor bind:reservations={scopeReservations} />

        <div class="actions">
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button onclick={handleSave} disabled={loading || !scopeName}>
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
</style>

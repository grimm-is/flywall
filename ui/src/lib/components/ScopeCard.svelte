<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { t } from "svelte-i18n";
    import {
        Card,
        Button,
        Input,
        Select,
        Spinner,
        Icon,
    } from "$lib/components";
    import StaticReservationsEditor from "$lib/components/StaticReservationsEditor.svelte";

    export let scope: any;
    export let interfaces: any[] = [];
    export let loading = false;

    const dispatch = createEventDispatcher();

    let isEditing = false;

    // Edit form state
    let editName = "";
    let editInterface = "";
    let editRangeStart = "";
    let editRangeEnd = "";
    let editRouter = "";
    let editDns = "";
    let editLeaseTime = "";
    let editDomain = "";
    let editReservations: any[] = [];

    function startEdit() {
        editName = scope.name || "";
        editInterface = scope.interface || "";
        editRangeStart = scope.range_start || "";
        editRangeEnd = scope.range_end || "";
        editRouter = scope.router || "";
        editDns = (scope.dns || []).join(", ");
        editLeaseTime = scope.lease_time || "24h";
        editDomain = scope.domain || "";
        editReservations = scope.reservations
            ? JSON.parse(JSON.stringify(scope.reservations))
            : [];
        isEditing = true;
    }

    function cancelEdit() {
        isEditing = false;
    }

    function handleSave() {
        if (!editName || !editInterface || !editRangeStart || !editRangeEnd)
            return;

        dispatch("save", {
            name: editName,
            interface: editInterface,
            range_start: editRangeStart,
            range_end: editRangeEnd,
            router: editRouter || undefined,
            dns: editDns
                ? editDns
                      .split(",")
                      .map((s) => s.trim())
                      .filter(Boolean)
                : undefined,
            lease_time: editLeaseTime,
            domain: editDomain || undefined,
            reservations: editReservations,
        });
        isEditing = false;
    }

    function handleDelete() {
        dispatch("delete", scope);
    }
</script>

<Card>
    {#if isEditing}
        <div class="form-stack">
            <div class="edit-header">
                <h3>
                    {$t("common.edit_item", { values: { item: scope.name } })}
                </h3>
            </div>

            <Input
                id={`scope-name-${scope.name}`}
                label={$t("dhcp.scope_name")}
                bind:value={editName}
                placeholder={$t("dhcp.scope_name_placeholder")}
                required
            />

            <Select
                id={`scope-iface-${scope.name}`}
                label={$t("dhcp.interface")}
                bind:value={editInterface}
                options={interfaces.map((i: any) => ({
                    value: i.Name,
                    label: i.Name,
                }))}
                required
            />

            <div class="grid grid-cols-2 gap-4">
                <Input
                    id={`scope-start-${scope.name}`}
                    label={$t("dhcp.range_start")}
                    bind:value={editRangeStart}
                    placeholder="192.168.1.100"
                    required
                />
                <Input
                    id={`scope-end-${scope.name}`}
                    label={$t("dhcp.range_end")}
                    bind:value={editRangeEnd}
                    placeholder="192.168.1.200"
                    required
                />
            </div>

            <Input
                id={`scope-router-${scope.name}`}
                label={$t("dhcp.router_optional")}
                bind:value={editRouter}
                placeholder="192.168.1.1"
            />

            <div class="grid grid-cols-2 gap-4">
                <Input
                    id={`scope-lease-${scope.name}`}
                    label={$t("dhcp.lease_time")}
                    bind:value={editLeaseTime}
                    placeholder="24h"
                />
                <Input
                    id={`scope-domain-${scope.name}`}
                    label={$t("dhcp.domain")}
                    bind:value={editDomain}
                    placeholder="lan"
                />
            </div>

            <Input
                id={`scope-dns-${scope.name}`}
                label={$t("dhcp.dns_servers")}
                bind:value={editDns}
                placeholder="1.1.1.1, 8.8.8.8"
            />

            <StaticReservationsEditor bind:reservations={editReservations} />

            <div class="edit-actions">
                <Button variant="ghost" onclick={cancelEdit} disabled={loading}>
                    {$t("common.cancel")}
                </Button>
                <Button onclick={handleSave} disabled={loading}>
                    {#if loading}<Spinner size="sm" />{/if}
                    {$t("common.save")}
                </Button>
            </div>
        </div>
    {:else}
        <div class="scope-header">
            <h4>{scope.name}</h4>
            <div class="scope-actions">
                <Button variant="ghost" size="sm" onclick={startEdit}
                    ><Icon name="edit" size="sm" /></Button
                >
                <Button variant="ghost" size="sm" onclick={handleDelete}
                    ><Icon name="delete" size="sm" /></Button
                >
            </div>
        </div>
        <div class="scope-details">
            <div class="detail-row">
                <span class="detail-label">{$t("dhcp.interface")}:</span>
                <span class="detail-value">{scope.interface}</span>
            </div>
            <div class="detail-row">
                <span class="detail-label">{$t("dhcp.range")}:</span>
                <span class="detail-value mono"
                    >{scope.range_start} - {scope.range_end}</span
                >
            </div>
            {#if scope.router}
                <div class="detail-row">
                    <span class="detail-label">{$t("dhcp.router")}:</span>
                    <span class="detail-value mono">{scope.router}</span>
                </div>
            {/if}
            <div class="detail-row">
                <span class="detail-label">Lease Time:</span>
                <span class="detail-value">{scope.lease_time}</span>
            </div>
            {#if scope.reservations?.length > 0}
                <div class="detail-row">
                    <span class="detail-label">Reservations:</span>
                    <span class="detail-value">{scope.reservations.length}</span
                    >
                </div>
            {/if}
        </div>
    {/if}
</Card>

<style>
    .edit-header {
        margin-bottom: var(--space-4);
    }
    .edit-header h3 {
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
    }
    .form-stack {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
    }
    .edit-actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        margin-top: var(--space-4);
        padding-top: var(--space-4);
        border-top: 1px solid var(--color-border);
    }
    .scope-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-bottom: var(--space-3);
    }
    .scope-header h4 {
        font-size: var(--text-base);
        font-weight: 600;
        margin: 0;
    }
    .scope-actions {
        display: flex;
        gap: var(--space-1);
    }
    .scope-details {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
    }
    .detail-row {
        display: flex;
        justify-content: space-between;
        font-size: var(--text-sm);
    }
    .detail-label {
        color: var(--color-muted);
    }
    .detail-value {
        color: var(--color-foreground);
    }
    .mono {
        font-family: var(--font-mono);
    }
</style>

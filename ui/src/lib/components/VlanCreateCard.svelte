<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import {
        Card,
        Button,
        Input,
        Select,
        Spinner,
        Badge,
    } from "$lib/components";
    import { t } from "svelte-i18n";

    export let interfaces: any[] = [];
    export let zones: any[] = [];
    export let loading = false;

    const dispatch = createEventDispatcher();

    let vlanParent = "";
    let vlanId = "";
    let vlanZone = "";
    let vlanIp = "";
    let vlanError = "";

    // Set defaults
    $: if (interfaces.length > 0 && !vlanParent) {
        // Default to first non-virtual interface if possible, or just first
        const candidates = interfaces.filter((i) => !i.Name?.includes("."));
        if (candidates.length > 0) vlanParent = candidates[0].Name;
    }
    $: if (zones.length > 0 && !vlanZone) {
        vlanZone = zones[0].name;
    }

    function handleSave() {
        if (!vlanParent || !vlanId || !vlanZone) return;
        vlanError = "";

        // Validate IP if provided
        if (vlanIp) {
            const cidrRegex =
                /^(\d{1,3}\.){3}\d{1,3}\/(1?[0-9]|2[0-9]|3[0-2])$/;
            if (!cidrRegex.test(vlanIp)) {
                vlanError =
                    $t("interfaces.invalid_cidr") ||
                    "Invalid CIDR format (e.g. 192.168.100.1/24)";
                return;
            }
        }

        dispatch("save", {
            parent_interface: vlanParent,
            vlan_id: parseInt(vlanId),
            zone: vlanZone,
            ipv4: vlanIp ? [vlanIp] : [],
        });
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card class="create-card border-l-4 border-l-primary">
    <div class="card-header">
        <h3>{$t("common.add_item", { values: { item: $t("item.vlan") } })}</h3>
        <Badge variant="outline">New VLAN</Badge>
    </div>

    <div class="form-grid">
        <Select
            id="vlan-parent"
            label={$t("interfaces.parent_interface")}
            bind:value={vlanParent}
            options={interfaces
                .filter((i: any) => !i.Name?.includes("."))
                .map((i: any) => ({ value: i.Name, label: i.Name }))}
            required
        />

        <Input
            id="vlan-id"
            label={$t("interfaces.vlan_id")}
            bind:value={vlanId}
            placeholder="100"
            type="number"
            required
        />

        <Select
            id="vlan-zone"
            label={$t("item.zone")}
            bind:value={vlanZone}
            options={zones.map((z: any) => ({ value: z.name, label: z.name }))}
            required
        />

        <Input
            id="vlan-ip"
            label={$t("interfaces.ipv4_list")}
            bind:value={vlanIp}
            placeholder="192.168.100.1/24"
            error={vlanError}
        />
    </div>

    <div class="actions">
        <Button variant="ghost" onclick={handleCancel} disabled={loading}>
            {$t("common.cancel")}
        </Button>
        <Button
            onclick={handleSave}
            disabled={loading || !vlanParent || !vlanId}
        >
            {#if loading}<Spinner size="sm" />{/if}
            {$t("common.save")}
        </Button>
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

    .form-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
        gap: var(--space-4);
        margin-bottom: var(--space-4);
    }

    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        padding-top: var(--space-4);
        border-top: 1px solid var(--color-border);
    }
</style>

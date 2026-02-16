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

    export let zones: any[] = [];
    export let hardwareInterfaces: any[] = [];
    export let loading = false;

    const dispatch = createEventDispatcher();

    let bondName = "bond0";
    let bondZone = "";
    let bondMode = "balance-rr";
    let bondMembers: string[] = [];
    let bondError = "";

    // Set default zone
    $: if (zones.length > 0 && !bondZone) {
        bondZone = zones[0].name;
    }

    // Calculate available interfaces state for display
    $: availableInterfaces = hardwareInterfaces.filter((i) => i.isAvailable);
    $: isDegradedBond = bondMembers.length === 1;

    function toggleMember(ifaceName: string) {
        if (bondMembers.includes(ifaceName)) {
            bondMembers = bondMembers.filter((m) => m !== ifaceName);
        } else {
            bondMembers = [...bondMembers, ifaceName];
        }
    }

    function handleSave() {
        if (!bondName || !bondZone || bondMembers.length < 1) return;
        bondError = "";

        dispatch("save", {
            name: bondName,
            zone: bondZone,
            mode: bondMode,
            interfaces: bondMembers,
        });
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card class="create-card border-l-4 border-l-primary">
    <div class="card-header">
        <h3>{$t("common.add_item", { values: { item: $t("item.bond") } })}</h3>
        <Badge variant="outline">New Bond</Badge>
    </div>

    <div class="form-stack">
        <div class="row-2">
            <Input
                id="bond-name"
                label={$t("common.name")}
                bind:value={bondName}
                placeholder="bond0"
                required
            />

            <Select
                id="bond-zone"
                label={$t("item.zone")}
                bind:value={bondZone}
                options={zones.map((z: any) => ({
                    value: z.name,
                    label: z.name,
                }))}
                required
            />
        </div>

        <Select
            id="bond-mode"
            label={$t("interfaces.bond_mode")}
            bind:value={bondMode}
            options={[
                { value: "balance-rr", label: "Round Robin (balance-rr)" },
                {
                    value: "active-backup",
                    label: "Active Backup (active-backup)",
                },
                { value: "balance-xor", label: "XOR (balance-xor)" },
                { value: "broadcast", label: "Broadcast" },
                { value: "802.3ad", label: "LACP (802.3ad)" },
                { value: "balance-tlb", label: "Adaptive TLB (balance-tlb)" },
                { value: "balance-alb", label: "Adaptive ALB (balance-alb)" },
            ]}
        />

        <div class="member-selection">
            <span class="member-label">{$t("interfaces.select_members")}</span>
            <div class="member-list">
                {#each hardwareInterfaces as iface}
                    <label
                        class="member-item"
                        class:disabled={!iface.isAvailable}
                    >
                        <input
                            type="checkbox"
                            checked={bondMembers.includes(iface.Name)}
                            disabled={!iface.isAvailable}
                            onchange={() => toggleMember(iface.Name)}
                        />
                        <span class="member-name">{iface.Name}</span>
                        {#if !iface.isAvailable}
                            <span class="member-status"
                                >({iface.usageReason})</span
                            >
                        {/if}
                    </label>
                {/each}
                {#if hardwareInterfaces.length === 0}
                    <p class="member-warning">{$t("interfaces.no_hardware")}</p>
                {/if}
            </div>

            {#if availableInterfaces.length === 0}
                <p class="member-warning">
                    ⚠️ {$t("interfaces.no_available")}
                </p>
            {:else if availableInterfaces.length === 1}
                <p class="member-info">
                    ℹ️ {$t("interfaces.one_available")}
                </p>
            {/if}

            {#if isDegradedBond}
                <p class="member-warning degraded">
                    ⚠️ {$t("interfaces.degraded_bond")}
                </p>
            {/if}
        </div>

        <div class="actions">
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button
                onclick={handleSave}
                disabled={loading || bondMembers.length < 1}
            >
                {#if loading}<Spinner size="sm" />{/if}
                {$t("common.save")}
            </Button>
        </div>
        {#if bondError}
            <p class="error-msg">{bondError}</p>
        {/if}
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

    .row-2 {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: var(--space-4);
    }

    .member-selection {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
    }

    .member-label {
        font-size: var(--text-sm);
        font-weight: 500;
    }

    .member-list {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
        padding: var(--space-3);
        background-color: var(--color-backgroundSecondary);
        border-radius: var(--radius-md);
    }

    .member-item {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        cursor: pointer;
        font-size: var(--text-sm);
    }

    .member-item.disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .member-status {
        color: var(--color-muted);
        font-style: italic;
        font-size: var(--text-xs);
    }

    .member-warning {
        color: var(--color-warning, #f59e0b);
        font-size: var(--text-sm);
        margin: var(--space-2) 0 0 0;
    }

    .member-info {
        color: var(--color-muted);
        font-size: var(--text-sm);
        margin: var(--space-2) 0 0 0;
    }

    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        margin-top: var(--space-4);
        padding-top: var(--space-4);
        border-top: 1px solid var(--color-border);
    }

    .error-msg {
        color: var(--color-destructive);
        font-size: var(--text-sm);
        margin-top: var(--space-2);
    }
</style>

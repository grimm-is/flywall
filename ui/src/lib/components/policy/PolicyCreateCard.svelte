<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { Card, Button, Select, Spinner } from "$lib/components";
    import { t } from "svelte-i18n";

    export let loading = false;
    export let zoneNames: string[] = [];

    const dispatch = createEventDispatcher();

    let policyFrom = "";
    let policyTo = "";

    function handleSave() {
        if (!policyFrom || !policyTo) return;
        dispatch("save", { from: policyFrom, to: policyTo });
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card>
    <div class="create-card">
        <div class="header">
            <h4>Add Policy</h4>
        </div>
        <div class="form-stack">
            <Select
                id="policy-from"
                label={$t("firewall.from_zone")}
                bind:value={policyFrom}
                options={zoneNames.map((n) => ({ value: n, label: n }))}
            />
            <Select
                id="policy-to"
                label={$t("firewall.to_zone")}
                bind:value={policyTo}
                options={zoneNames.map((n) => ({ value: n, label: n }))}
            />
            <p class="form-hint">
                Traffic from <strong>{policyFrom || "?"}</strong> →
                <strong>{policyTo || "?"}</strong>
            </p>
        </div>
        <div class="actions">
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button
                onclick={handleSave}
                disabled={loading || !policyFrom || !policyTo}
            >
                {#if loading}<Spinner size="sm" />{/if}
                {$t("common.create_item", {
                    values: { item: $t("item.policy") },
                })}
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
    .form-hint {
        font-size: var(--text-sm);
        color: var(--color-muted);
        margin: 0;
    }
    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        border-top: 1px solid var(--color-border);
        padding-top: var(--space-3);
    }
</style>

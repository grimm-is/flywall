<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { Card, Button, Input, Spinner } from "$lib/components";
    import { t } from "svelte-i18n";

    export let loading = false;

    const dispatch = createEventDispatcher();

    let forwarderIp = "";

    function handleSave() {
        if (!forwarderIp.trim()) return;
        dispatch("save", forwarderIp.trim());
        forwarderIp = "";
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card>
    <div class="create-card">
        <div class="header">
            <h4>
                {$t("common.add_item", {
                    values: { item: $t("item.forwarder") },
                })}
            </h4>
        </div>
        <div class="form-row">
            <Input
                id="forwarder-ip"
                label={$t("dns.server_ip")}
                bind:value={forwarderIp}
                placeholder={$t("dns.server_ip_placeholder")}
            />
        </div>
        <div class="actions">
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button
                onclick={handleSave}
                disabled={loading || !forwarderIp.trim()}
            >
                {#if loading}<Spinner size="sm" />{/if}
                {$t("common.add")}
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
    .form-row {
        display: flex;
        gap: var(--space-4);
    }
    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        border-top: 1px solid var(--color-border);
        padding-top: var(--space-3);
    }
</style>

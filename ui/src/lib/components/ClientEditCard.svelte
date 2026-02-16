<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { Card, Button, Input, Spinner } from "$lib/components";
    import { t } from "svelte-i18n";

    export let loading = false;
    export let client: any = null;

    const dispatch = createEventDispatcher();

    let alias = client?.alias || "";
    let owner = client?.owner || "";
    let type = client?.type || "";
    let tags = client?.tags?.join(", ") || "";

    $: if (client) {
        alias = client.alias || "";
        owner = client.owner || "";
        type = client.type || "";
        tags = client.tags?.join(", ") || "";
    }

    function handleSave() {
        dispatch("save", {
            mac: client.mac,
            alias,
            owner,
            type,
            tags: tags
                .split(",")
                .map((t: string) => t.trim())
                .filter(Boolean),
        });
    }

    function handleCancel() {
        dispatch("cancel");
    }

    function handleUnlink() {
        dispatch("unlink", { device_id: client.device_id });
    }
</script>

<Card>
    <div class="edit-card">
        <div class="header">
            <h4>
                {$t("common.edit_item", {
                    values: { item: $t("item.device") },
                })}
            </h4>
        </div>
        {#if client}
            <div class="info-grid">
                <div class="info-row">
                    <strong>{$t("network.mac_address")}:</strong>
                    <code>{client.mac}</code>
                </div>
                {#if client.vendor}
                    <div class="info-row">
                        <strong>{$t("network.vendor")}:</strong>
                        <span>{client.vendor}</span>
                    </div>
                {/if}
                {#if client.dhcp_fingerprint}
                    <div class="info-row">
                        <strong>{$t("network.dhcp_fingerprint")}:</strong>
                        <code class="fingerprint"
                            >{client.dhcp_fingerprint}</code
                        >
                    </div>
                {/if}
                {#if client.mdns_hostname}
                    <div class="info-row">
                        <strong>{$t("network.mdns_host")}:</strong>
                        <span>{client.mdns_hostname}</span>
                    </div>
                {/if}
                {#if client.dhcp_vendor_class}
                    <div class="info-row">
                        <strong>{$t("network.dhcp_vendor_class")}:</strong>
                        <span>{client.dhcp_vendor_class}</span>
                    </div>
                {/if}
            </div>

            <div class="form-stack">
                <Input
                    id="alias"
                    label={$t("network.alias")}
                    bind:value={alias}
                    placeholder={$t("network.friendly_name")}
                />
                <Input
                    id="owner"
                    label={$t("network.owner")}
                    bind:value={owner}
                    placeholder={$t("network.owner_placeholder")}
                />
                <Input
                    id="type"
                    label={$t("network.type")}
                    bind:value={type}
                    placeholder={$t("network.type_placeholder")}
                />
                <Input
                    id="tags"
                    label={$t("network.tags")}
                    bind:value={tags}
                    placeholder={$t("network.tags_placeholder")}
                />
            </div>

            <details class="raw-details">
                <summary class="raw-summary"
                    >{$t("network.full_raw_data")}</summary
                >
                <pre class="raw-json">{JSON.stringify(client, null, 2)}</pre>
            </details>
        {/if}
        <div class="actions">
            {#if client?.device_id}
                <Button
                    variant="destructive"
                    onclick={handleUnlink}
                    disabled={loading}
                >
                    {$t("network.unlink_identity")}
                </Button>
            {/if}
            <div class="spacer"></div>
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button onclick={handleSave} disabled={loading}>
                {#if loading}<Spinner size="sm" />{/if}
                {$t("common.save")}
            </Button>
        </div>
    </div>
</Card>

<style>
    .edit-card {
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
    .info-grid {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
        background: var(--color-backgroundSecondary);
        padding: var(--space-3);
        border-radius: var(--radius-md);
    }
    .info-row {
        display: flex;
        gap: var(--space-2);
        font-size: var(--text-sm);
        align-items: center;
    }
    .info-row code {
        font-family: var(--font-mono);
        background: var(--color-background);
        padding: 0 var(--space-1);
        border-radius: var(--radius-sm);
    }
    .fingerprint {
        font-size: var(--text-xs);
    }
    .form-stack {
        display: flex;
        flex-direction: column;
        gap: var(--space-3);
    }
    .raw-details {
        margin-top: var(--space-2);
    }
    .raw-summary {
        cursor: pointer;
        color: var(--color-muted);
        font-size: var(--text-sm);
    }
    .raw-json {
        font-size: var(--text-xs);
        background: var(--color-backgroundSecondary);
        padding: var(--space-2);
        border-radius: var(--radius-md);
        overflow: auto;
        max-height: 200px;
    }
    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        border-top: 1px solid var(--color-border);
        padding-top: var(--space-3);
    }
    .spacer {
        flex: 1;
    }
</style>

<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { t } from "svelte-i18n";
    import {
        Card,
        Button,
        Input,
        Badge,
        Spinner,
        Icon,
        Toggle,
    } from "$lib/components";

    export let serve: any;
    export let loading = false;

    const dispatch = createEventDispatcher();

    let isEditing = false;

    // Edit form state
    let editZone = "";
    let editLocalDomain = "";
    let editExpandHosts = false;
    let editDhcp = false;
    let editCache = false;
    let editCacheSize = 10000;
    let editLogging = false;

    function startEdit() {
        editZone = serve.zone;
        editLocalDomain = serve.local_domain || "";
        editExpandHosts = serve.expand_hosts || false;
        editDhcp = serve.dhcp_integration || false;
        editCache = serve.cache_enabled || false;
        editCacheSize = serve.cache_size || 10000;
        editLogging = serve.query_logging || false;
        isEditing = true;
    }

    function cancelEdit() {
        isEditing = false;
    }

    function handleSave() {
        if (!editZone) return;

        dispatch("save", {
            zone: editZone,
            local_domain: editLocalDomain,
            expand_hosts: editExpandHosts,
            dhcp_integration: editDhcp,
            cache_enabled: editCache,
            cache_size: Number(editCacheSize),
            query_logging: editLogging,
        });
        isEditing = false;
    }

    function handleDelete() {
        dispatch("delete", serve.zone);
    }
</script>

<Card>
    {#if isEditing}
        <div class="form-stack">
            <div class="edit-header">
                <h3>
                    {$t("common.edit_item", { values: { item: serve.zone } })}
                </h3>
            </div>

            <div class="grid grid-cols-2 gap-4">
                <Input
                    id={`serve-zone-${serve.zone}`}
                    label={$t("dns.zone_name")}
                    bind:value={editZone}
                    placeholder={$t("dns.zone_name_placeholder")}
                    required
                    disabled={true}
                />
                <Input
                    id={`serve-domain-${serve.zone}`}
                    label={$t("dns.local_domain")}
                    bind:value={editLocalDomain}
                    placeholder={$t("dns.local_domain_placeholder")}
                />
            </div>

            <div class="p-4 bg-secondary/10 rounded-lg space-y-4">
                <h3 class="text-sm font-medium text-foreground">
                    {$t("dns.integration")}
                </h3>
                <Toggle
                    label={$t("dhcp.integration")}
                    bind:checked={editDhcp}
                />
                <p class="text-xs text-muted-foreground pb-2">
                    {$t("dns.integration_desc")}
                </p>

                <Toggle
                    label={$t("dns.expand_hosts")}
                    bind:checked={editExpandHosts}
                />
                <p class="text-xs text-muted-foreground">
                    {$t("dns.expand_hosts_desc")}
                </p>
            </div>

            <div class="p-4 bg-secondary/10 rounded-lg space-y-4">
                <div class="flex items-center justify-between">
                    <h3 class="text-sm font-medium text-foreground">
                        {$t("dns.caching")}
                    </h3>
                    <Toggle label="" bind:checked={editCache} />
                </div>

                {#if editCache}
                    <Input
                        id={`serve-cache-size-${serve.zone}`}
                        label={$t("dns.cache_size")}
                        type="number"
                        bind:value={editCacheSize}
                    />
                {/if}
            </div>

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
        <div class="serve-item">
            <div class="serve-info">
                <span class="zone-badge">{serve.zone}</span>
                <div class="serve-details">
                    {#if serve.local_domain}
                        <Badge variant="outline"
                            >{$t("dns.domain")}: {serve.local_domain}</Badge
                        >
                    {/if}
                    {#if serve.cache_enabled}
                        <Badge variant="secondary"
                            >{$t("dns.cache")}: {serve.cache_size}</Badge
                        >
                    {/if}
                    {#if serve.dhcp_integration}
                        <Badge variant="secondary"
                            >{$t("dns.dhcp_linked")}</Badge
                        >
                    {/if}
                </div>
            </div>
            <div class="serve-actions">
                <Button variant="ghost" onclick={startEdit}>
                    <Icon name="edit" />
                </Button>
                <Button variant="ghost" onclick={handleDelete}>
                    <Icon name="delete" />
                </Button>
            </div>
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

    .serve-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
    }

    .serve-info {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
    }

    .zone-badge {
        font-size: var(--text-lg);
        font-weight: 600;
        color: var(--color-foreground);
    }

    .serve-details {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-2);
    }

    .serve-actions {
        display: flex;
        gap: var(--space-1);
    }
</style>

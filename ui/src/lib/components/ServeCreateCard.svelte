<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { t } from "svelte-i18n";
    import {
        Card,
        Button,
        Input,
        Badge,
        Spinner,
        Toggle,
    } from "$lib/components";

    export let loading = false;

    const dispatch = createEventDispatcher();

    // Form state
    let newZone = "lan";
    let newLocalDomain = "lan";
    let newExpandHosts = true;
    let newDhcp = true;
    let newCache = true;
    let newCacheSize = 10000;
    let newLogging = false;

    function handleSave() {
        if (!newZone) return;

        dispatch("save", {
            zone: newZone,
            local_domain: newLocalDomain,
            expand_hosts: newExpandHosts,
            dhcp_integration: newDhcp,
            cache_enabled: newCache,
            cache_size: Number(newCacheSize),
            query_logging: newLogging,
        });
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card class="create-card border-l-4 border-l-primary">
    <div class="card-header">
        <h3>
            {$t("common.add_item", { values: { item: $t("item.config") } })}
        </h3>
        <Badge variant="outline">New Zone Config</Badge>
    </div>

    <div class="form-stack">
        <div class="grid grid-cols-2 gap-4">
            <Input
                id="new-serve-zone"
                label={$t("dns.zone_name")}
                bind:value={newZone}
                placeholder={$t("dns.zone_name_placeholder")}
                required
            />
            <Input
                id="new-serve-domain"
                label={$t("dns.local_domain")}
                bind:value={newLocalDomain}
                placeholder={$t("dns.local_domain_placeholder")}
            />
        </div>

        <div class="p-4 bg-secondary/10 rounded-lg space-y-4">
            <h3 class="text-sm font-medium text-foreground">
                {$t("dns.integration")}
            </h3>
            <Toggle label={$t("dhcp.integration")} bind:checked={newDhcp} />
            <p class="text-xs text-muted-foreground pb-2">
                {$t("dns.integration_desc")}
            </p>

            <Toggle
                label={$t("dns.expand_hosts")}
                bind:checked={newExpandHosts}
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
                <Toggle label="" bind:checked={newCache} />
            </div>

            {#if newCache}
                <Input
                    id="new-serve-cache-size"
                    label={$t("dns.cache_size")}
                    type="number"
                    bind:value={newCacheSize}
                />
            {/if}
        </div>

        <div class="actions">
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button onclick={handleSave} disabled={loading || !newZone}>
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

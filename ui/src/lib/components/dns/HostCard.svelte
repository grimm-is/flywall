<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { Card, Button, Input, Spinner } from "$lib/components";
    import { t } from "svelte-i18n";

    export let loading = false;
    export let host: any = null; // For editing

    const dispatch = createEventDispatcher();

    let ip = host?.ip || "";
    let hostnames = host?.hostnames?.join(", ") || "";

    $: if (host) {
        ip = host.ip || "";
        hostnames = host.hostnames?.join(", ") || "";
    }

    function handleSave() {
        if (!ip.trim() || !hostnames.trim()) return;
        dispatch("save", {
            ip: ip.trim(),
            hostnames: hostnames
                .split(",")
                .map((h: string) => h.trim())
                .filter(Boolean),
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
                {host
                    ? $t("common.edit_item", {
                          values: { item: $t("item.host") },
                      })
                    : $t("common.add_item", {
                          values: { item: $t("item.host") },
                      })}
            </h4>
        </div>
        <div class="form-stack">
            <Input
                id="host-ip"
                label={$t("dns.host_ip")}
                bind:value={ip}
                placeholder="192.168.1.100"
                disabled={!!host}
            />
            <Input
                id="host-hostnames"
                label={$t("dns.hostnames")}
                bind:value={hostnames}
                placeholder="myserver, myserver.lan, myserver.local"
            />
            <p class="hint">{$t("dns.hostnames_hint")}</p>
        </div>
        <div class="actions">
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button
                onclick={handleSave}
                disabled={loading || !ip.trim() || !hostnames.trim()}
            >
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
    .hint {
        font-size: var(--text-xs);
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

<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import {
        Card,
        Button,
        Input,
        Select,
        Toggle,
        Spinner,
    } from "$lib/components";
    import { t } from "svelte-i18n";

    export let loading = false;
    export let blocklist: any = null; // For editing

    const dispatch = createEventDispatcher();

    let name = blocklist?.name || "";
    let url = blocklist?.url || "";
    let format = blocklist?.format || "domains";
    let enabled = blocklist?.enabled !== false;
    let refreshHours = blocklist?.refresh_hours || 24;

    $: if (blocklist) {
        name = blocklist.name || "";
        url = blocklist.url || "";
        format = blocklist.format || "domains";
        enabled = blocklist.enabled !== false;
        refreshHours = blocklist.refresh_hours || 24;
    }

    function handleSave() {
        if (!name.trim() || !url.trim()) return;
        dispatch("save", {
            name: name.trim(),
            url: url.trim(),
            format,
            enabled,
            refresh_hours: refreshHours,
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
                {blocklist
                    ? $t("common.edit_item", {
                          values: { item: $t("item.blocklist") },
                      })
                    : $t("common.add_item", {
                          values: { item: $t("item.blocklist") },
                      })}
            </h4>
        </div>
        <div class="form-stack">
            <Input
                id="blocklist-name"
                label={$t("common.name")}
                bind:value={name}
                placeholder="e.g., ads, malware, tracking"
                disabled={!!blocklist}
            />
            <Input
                id="blocklist-url"
                label={$t("dns.blocklist_url")}
                bind:value={url}
                placeholder="https://example.com/blocklist.txt"
            />
            <div class="grid grid-cols-2 gap-4">
                <Select
                    id="blocklist-format"
                    label={$t("dns.blocklist_format")}
                    bind:value={format}
                    options={[
                        {
                            value: "domains",
                            label: $t("dns.blocklist_formats.domains"),
                        },
                        {
                            value: "hosts",
                            label: $t("dns.blocklist_formats.hosts"),
                        },
                        {
                            value: "adblock",
                            label: $t("dns.blocklist_formats.adblock"),
                        },
                    ]}
                />
                <Input
                    id="blocklist-refresh"
                    label={$t("dns.refresh_hours")}
                    type="number"
                    bind:value={refreshHours}
                    placeholder="24"
                />
            </div>
            <Toggle label={$t("common.enabled")} bind:checked={enabled} />
        </div>
        <div class="actions">
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button
                onclick={handleSave}
                disabled={loading || !name.trim() || !url.trim()}
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
    .grid {
        display: grid;
    }
    .grid-cols-2 {
        grid-template-columns: repeat(2, 1fr);
    }
    .gap-4 {
        gap: 1rem;
    }
    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        border-top: 1px solid var(--color-border);
        padding-top: var(--space-3);
    }
</style>

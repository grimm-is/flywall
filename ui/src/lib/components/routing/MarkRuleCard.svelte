<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { Card, Button, Input, Select, Spinner } from "$lib/components";
    import { t } from "svelte-i18n";

    export let loading = false;
    export let markRule: any = null;
    export let interfaces: any[] = [];

    const dispatch = createEventDispatcher();

    let name = markRule?.name || "";
    let mark = markRule?.mark?.toString() || "";
    let srcIP = markRule?.src_ip || "";
    let outInterface = markRule?.out_interface || "";

    $: if (markRule) {
        name = markRule.name || "";
        mark = markRule.mark?.toString() || "";
        srcIP = markRule.src_ip || "";
        outInterface = markRule.out_interface || "";
    }

    function handleSave() {
        if (!name || !mark) return;
        dispatch("save", {
            name,
            mark: parseInt(mark),
            src_ip: srcIP || undefined,
            out_interface: outInterface || undefined,
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
                {markRule
                    ? $t("common.edit_item", {
                          values: { item: $t("item.mark_rule") },
                      })
                    : $t("common.add_item", {
                          values: { item: $t("item.mark_rule") },
                      })}
            </h4>
        </div>
        <div class="form-stack">
            <Input id="mr-name" label={$t("common.name")} bind:value={name} />
            <Input
                id="mr-mark"
                label={$t("routing.mark_int")}
                bind:value={mark}
                type="number"
            />
            <Input
                id="mr-src"
                label={$t("routing.source_ip")}
                bind:value={srcIP}
            />
            <Select
                id="mr-iface"
                label={$t("common.interface")}
                bind:value={outInterface}
                options={[
                    { value: "", label: $t("routing.any") },
                    ...interfaces.map((i) => ({
                        value: i.Name,
                        label: i.Name,
                    })),
                ]}
            />
        </div>
        <div class="actions">
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button onclick={handleSave} disabled={loading || !name || !mark}>
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
    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        border-top: 1px solid var(--color-border);
        padding-top: var(--space-3);
    }
</style>

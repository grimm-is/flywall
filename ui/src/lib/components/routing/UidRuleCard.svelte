<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { Card, Button, Input, Spinner } from "$lib/components";
    import { t } from "svelte-i18n";

    export let loading = false;
    export let uidRule: any = null;

    const dispatch = createEventDispatcher();

    let name = uidRule?.name || "";
    let uid = uidRule?.uid?.toString() || "";
    let uplink = uidRule?.uplink || "";

    $: if (uidRule) {
        name = uidRule.name || "";
        uid = uidRule.uid?.toString() || "";
        uplink = uidRule.uplink || "";
    }

    function handleSave() {
        if (!name || !uid || !uplink) return;
        dispatch("save", {
            name,
            uid: parseInt(uid),
            uplink,
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
                {uidRule
                    ? $t("common.edit_item", {
                          values: { item: $t("item.uid_rule") },
                      })
                    : $t("common.add_item", {
                          values: { item: $t("item.uid_rule") },
                      })}
            </h4>
        </div>
        <div class="form-stack">
            <Input id="uid-name" label={$t("common.name")} bind:value={name} />
            <Input
                id="uid-uid"
                label={$t("routing.uid")}
                bind:value={uid}
                type="number"
            />
            <Input
                id="uid-uplink"
                label={$t("routing.uplink_name")}
                bind:value={uplink}
            />
        </div>
        <div class="actions">
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button
                onclick={handleSave}
                disabled={loading || !name || !uid || !uplink}
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
    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        border-top: 1px solid var(--color-border);
        padding-top: var(--space-3);
    }
</style>

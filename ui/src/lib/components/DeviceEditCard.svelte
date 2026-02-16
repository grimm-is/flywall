<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { Card, Button, Input, Select, Spinner } from "$lib/components";
    import { t } from "svelte-i18n";

    export let loading = false;
    export let identity: any;
    export let groupOptions: { value: string; label: string }[] = [];

    const dispatch = createEventDispatcher();

    let alias = identity?.alias || "";
    let owner = identity?.owner || "";
    let groupId = identity?.groupId || "";

    $: if (identity) {
        alias = identity.alias || "";
        owner = identity.owner || "";
        groupId = identity.groupId || "";
    }

    function handleSave() {
        dispatch("save", {
            id: identity.id,
            alias,
            owner,
            group_id: groupId,
            tags: identity.tags,
        });
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card>
    <div class="edit-card">
        <div class="header">
            <h3>Edit Device</h3>
        </div>

        <div class="form-stack">
            <div class="form-group">
                <Input
                    id="alias"
                    label="Alias / Name"
                    bind:value={alias}
                    placeholder="e.g. Dad's Laptop"
                />
                <span class="help">Friendly name for this device.</span>
            </div>

            <div class="form-group">
                <Input
                    id="owner"
                    label="Owner"
                    bind:value={owner}
                    placeholder="e.g. John Doe"
                />
                <span class="help">Who owns this device?</span>
            </div>

            <div class="form-group">
                <Select
                    id="group"
                    label="Group"
                    bind:value={groupId}
                    options={groupOptions}
                />
                <span class="help"
                    >Assign to a group for policy enforcement.</span
                >
            </div>

            {#if identity?.macs?.length > 0}
                <div class="mac-info">
                    <strong>Linked Hardware Addresses:</strong>
                    <ul>
                        {#each identity.macs as mac}
                            <li>{mac}</li>
                        {/each}
                    </ul>
                </div>
            {/if}

            <div class="actions">
                <Button
                    variant="ghost"
                    onclick={handleCancel}
                    disabled={loading}
                >
                    Cancel
                </Button>
                <Button onclick={handleSave} disabled={loading}>
                    {#if loading}<Spinner size="sm" />{/if}
                    Save Changes
                </Button>
            </div>
        </div>
    </div>
</Card>

<style>
    .edit-card {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
        padding: var(--space-2);
    }
    .header h3 {
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
    }
    .form-stack {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
    }
    .form-group {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
    }
    .help {
        font-size: var(--text-xs);
        color: var(--color-muted);
    }
    .mac-info {
        margin-top: var(--space-2);
        padding: var(--space-3);
        background: var(--color-backgroundSecondary);
        border-radius: var(--radius-md);
        font-size: var(--text-xs);
        color: var(--color-muted);
    }
    .mac-info ul {
        margin: var(--space-1) 0 0 var(--space-4);
        padding: 0;
    }
    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        margin-top: var(--space-2);
        border-top: 1px solid var(--color-border);
        padding-top: var(--space-4);
    }
</style>

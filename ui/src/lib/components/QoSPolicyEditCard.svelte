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
    export let policy: any = null;
    export let interfaces: any[] = [];

    const dispatch = createEventDispatcher();

    let policyName = policy?.name || "";
    let policyInterface = policy?.interface || "";
    let policyEnabled = policy?.enabled !== false;
    let policyDirection = policy?.direction || "both";
    let policyDownload = policy?.download_mbps?.toString() || "";
    let policyUpload = policy?.upload_mbps?.toString() || "";
    let formError = "";

    $: if (policy) {
        policyName = policy.name || "";
        policyInterface = policy.interface || "";
        policyEnabled = policy.enabled !== false;
        policyDirection = policy.direction || "both";
        policyDownload = policy.download_mbps?.toString() || "";
        policyUpload = policy.upload_mbps?.toString() || "";
    }

    function handleSave() {
        formError = "";
        if (!policyName) {
            formError = "Policy Name is required";
            return;
        }
        if (!policyInterface) {
            formError = "Interface is required";
            return;
        }

        dispatch("save", {
            name: policyName,
            interface: policyInterface,
            enabled: policyEnabled,
            direction: policyDirection,
            download_mbps: parseInt(policyDownload) || 0,
            upload_mbps: parseInt(policyUpload) || 0,
            classes: policy?.classes || [],
            rules: policy?.rules || [],
        });
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card>
    <div class="edit-card">
        <div class="header">
            <h4>{policy ? `Edit Policy: ${policy.name}` : "Add QoS Policy"}</h4>
        </div>

        {#if formError}
            <div class="error-alert">{formError}</div>
        {/if}

        <div class="form-stack">
            <Input
                id="policy-name"
                label="Policy Name"
                bind:value={policyName}
                placeholder="e.g., wan-shaping"
                disabled={!!policy}
            />

            <Select
                id="policy-interface"
                label="Interface"
                bind:value={policyInterface}
                options={interfaces.map((i) => ({
                    value: i.Name,
                    label: i.Name,
                }))}
                placeholder={interfaces.length === 0
                    ? "No interfaces available"
                    : "Select Interface"}
            />

            <Select
                id="policy-direction"
                label="Direction"
                bind:value={policyDirection}
                options={[
                    { value: "both", label: "Both (Ingress & Egress)" },
                    { value: "ingress", label: "Ingress (Download)" },
                    { value: "egress", label: "Egress (Upload)" },
                ]}
            />

            <div class="bandwidth-row">
                <Input
                    id="policy-download"
                    label="Download (Mbps)"
                    type="number"
                    bind:value={policyDownload}
                    placeholder="100"
                />
                <Input
                    id="policy-upload"
                    label="Upload (Mbps)"
                    type="number"
                    bind:value={policyUpload}
                    placeholder="20"
                />
            </div>

            <Toggle label="Policy Enabled" bind:checked={policyEnabled} />
        </div>

        <div class="actions">
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button
                onclick={handleSave}
                disabled={loading}
                data-testid="save-policy-btn"
            >
                {#if loading}<Spinner size="sm" />{/if}
                {policy ? "Save Changes" : "Create Policy"}
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
    .error-alert {
        background: var(--color-destructive);
        color: white;
        padding: var(--space-2) var(--space-3);
        border-radius: var(--radius-md);
        font-size: var(--text-sm);
    }
    .form-stack {
        display: flex;
        flex-direction: column;
        gap: var(--space-3);
    }
    .bandwidth-row {
        display: grid;
        grid-template-columns: 1fr 1fr;
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

<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import {
        Card,
        Button,
        Input,
        Select,
        Spinner,
        Toggle,
    } from "$lib/components";
    import { t } from "svelte-i18n";

    export let loading = false;
    export let interfaces: any[] = [];

    const dispatch = createEventDispatcher();

    let policyName = "";
    let policyInterface = "";
    let policyEnabled = true;
    let policyDirection = "both";
    let policyDownload = "";
    let policyUpload = "";

    let validationError = "";

    // Set default interface
    $: if (!policyInterface && interfaces.length > 0) {
        policyInterface = interfaces[0]?.Name || "";
    }

    function handleSave() {
        validationError = "";
        if (!policyName.trim()) {
            validationError = "Policy Name is required";
            return;
        }
        if (!policyInterface) {
            validationError = "Interface is required";
            return;
        }

        dispatch("save", {
            name: policyName,
            interface: policyInterface,
            enabled: policyEnabled,
            direction: policyDirection,
            download: policyDownload,
            upload: policyUpload,
        });
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card>
    <div class="create-card">
        <div class="header">
            <h3>Add QoS Policy</h3>
        </div>

        <div class="form-stack">
            <Input
                id="policy-name"
                label="Policy Name"
                bind:value={policyName}
                placeholder="e.g., wan-shaping"
                required
            />

            <div class="form-row">
                <Select
                    id="policy-interface"
                    label="Interface"
                    bind:value={policyInterface}
                    options={interfaces.map((i: any) => ({
                        value: i.Name,
                        label: i.Name,
                    }))}
                    placeholder={interfaces.length === 0
                        ? "No interfaces available"
                        : "Select Interface"}
                    required
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
            </div>

            <div class="form-row">
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

            {#if validationError}
                <div class="validation-error" role="alert" aria-live="polite">
                    {validationError}
                </div>
            {/if}

            <div class="actions">
                <Button
                    variant="ghost"
                    onclick={handleCancel}
                    disabled={loading}
                >
                    {$t("common.cancel")}
                </Button>
                <Button
                    variant="default"
                    onclick={handleSave}
                    disabled={loading}
                >
                    {#if loading}<Spinner size="sm" />{/if}
                    Create Policy
                </Button>
            </div>
        </div>
    </div>
</Card>

<style>
    .create-card {
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

    .form-row {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: var(--space-4);
    }

    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        margin-top: var(--space-2);
        padding-top: var(--space-4);
        border-top: 1px solid var(--color-border);
    }

    .validation-error {
        padding: var(--space-3);
        background-color: rgba(220, 38, 38, 0.1);
        border: 1px solid var(--color-destructive);
        border-radius: var(--radius-md);
        color: var(--color-destructive);
        font-size: var(--text-sm);
    }
</style>

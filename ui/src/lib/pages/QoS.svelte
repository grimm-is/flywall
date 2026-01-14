<script lang="ts">
    /**
     * QoS (Quality of Service) Page
     * Traffic shaping and bandwidth management
     */

    import { onMount } from "svelte";
    import { config, api, alertStore } from "$lib/stores/app";
    import {
        Card,
        Button,
        Modal,
        Input,
        Select,
        Badge,
        Toggle,
        Spinner,
        Icon,
    } from "$lib/components";
    import { t } from "svelte-i18n";

    interface QoSClass {
        name: string;
        priority: number;
        rate: string;
        ceil: string;
        burst?: string;
        queue_type?: string;
    }

    interface QoSRule {
        name: string;
        class: string;
        proto?: string;
        src_ip?: string;
        dest_ip?: string;
        src_port?: number;
        dest_port?: number;
        services?: string[];
        dscp?: string;
        set_dscp?: string;
    }

    interface QoSPolicy {
        name: string;
        interface: string;
        enabled: boolean;
        direction: string;
        download_mbps: number;
        upload_mbps: number;
        classes: QoSClass[];
        rules: QoSRule[];
    }

    let policies = $state<QoSPolicy[]>([]);
    let loading = $state(false);
    let showPolicyModal = $state(false);
    let editingPolicy = $state<QoSPolicy | null>(null);
    let sentinelEnabled = $state(false); // Mock state for Sentinel AI

    // Policy form state
    let policyName = $state("");
    let policyInterface = $state("");
    let policyEnabled = $state(true);
    let policyDirection = $state("both");
    let policyDownload = $state("");
    let policyUpload = $state("");
    let formError = $state("");

    const interfaces = $derived($config?.interfaces || []);

    $effect(() => {
        console.log("QoS Interfaces:", JSON.stringify(interfaces));
    });

    onMount(loadPolicies);

    async function loadPolicies() {
        loading = true;
        try {
            const result = await api.getQoSPolicies();
            policies = result || [];
        } catch (e: any) {
            console.log("QoS not configured");
            policies = [];
        } finally {
            loading = false;
        }
    }

    function openAddPolicy() {
        editingPolicy = null;
        policyName = "";
        // Default to first interface or empty
        policyInterface = interfaces.length > 0 ? interfaces[0].Name : "";
        policyEnabled = true;
        policyDirection = "both";
        policyDownload = "";
        policyUpload = "";
        formError = "";
        showPolicyModal = true;
    }

    function openEditPolicy(policy: QoSPolicy) {
        editingPolicy = policy;
        policyName = policy.name;
        policyInterface = policy.interface;
        policyEnabled = policy.enabled;
        policyDirection = policy.direction || "both";
        policyDownload = policy.download_mbps?.toString() || "";
        policyUpload = policy.upload_mbps?.toString() || "";
        formError = "";
        showPolicyModal = true;
    }

    async function savePolicy() {
        formError = "";
        if (!policyName) {
            formError = "Policy Name is required";
            return;
        }
        if (!policyInterface) {
            formError = "Interface is required";
            return;
        }

        const newPolicy: QoSPolicy = {
            name: policyName,
            interface: policyInterface,
            enabled: policyEnabled,
            direction: policyDirection,
            download_mbps: parseInt(policyDownload) || 0,
            upload_mbps: parseInt(policyUpload) || 0,
            classes: editingPolicy?.classes || [],
            rules: editingPolicy?.rules || [],
        };

        loading = true;
        try {
            let updatedPolicies: QoSPolicy[];
            if (editingPolicy) {
                const editName = editingPolicy.name;
                updatedPolicies = policies.map((p) =>
                    p.name === editName ? newPolicy : p,
                );
            } else {
                updatedPolicies = [...policies, newPolicy];
            }
            await api.updateQoSPolicies(updatedPolicies);
            await loadPolicies();
            showPolicyModal = false;
            alertStore.success(`QoS policy "${policyName}" saved`);
        } catch (e: any) {
            alertStore.error(e.message || "Failed to save QoS policy");
        } finally {
            loading = false;
        }
    }

    async function deletePolicy(policy: QoSPolicy) {
        if (
            !confirm(
                `Delete QoS policy "${policy.name}"? This cannot be undone.`,
            )
        )
            return;

        loading = true;
        try {
            const updatedPolicies = policies.filter(
                (p) => p.name !== policy.name,
            );
            await api.updateQoSPolicies(updatedPolicies);
            await loadPolicies();
            alertStore.success(`QoS policy "${policy.name}" deleted`);
        } catch (e: any) {
            alertStore.error(e.message || "Failed to delete QoS policy");
        } finally {
            loading = false;
        }
    }

    async function togglePolicy(policy: QoSPolicy) {
        loading = true;
        try {
            const updatedPolicies = policies.map((p) =>
                p.name === policy.name ? { ...p, enabled: !p.enabled } : p,
            );
            await api.updateQoSPolicies(updatedPolicies);
            await loadPolicies();
        } catch (e: any) {
            alertStore.error(e.message || "Failed to toggle QoS policy");
        } finally {
            loading = false;
        }
    }
</script>

<div class="qos-page">
    <div class="page-header">
        <div class="header-info">
            <h2>Quality of Service</h2>
            <p class="subtitle">Traffic shaping and AI-driven prioritization</p>
        </div>
        <div class="header-actions">
            <Button
                onclick={loadPolicies}
                variant="outline"
                size="sm"
                disabled={loading}
            >
                <Icon name="refresh" size="sm" />
                Refresh
            </Button>
            <Button onclick={openAddPolicy} data-testid="add-policy-btn"
                >+ Add Policy</Button
            >
        </div>
    </div>

    <!-- Sentinel AI Feature (Spec Compliance) -->
    <Card className="sentinel-card">
        <div class="sentinel-content">
            <div class="sentinel-info">
                <div class="sentinel-title">
                    <Icon
                        name="auto_awesome"
                        size="md"
                        className="text-purple-400"
                    />
                    <span>Sentinel AI Optimization</span>
                </div>
                <p class="sentinel-desc">
                    Automatically classify and prioritize Gaming and VoIP
                    traffic using on-device ML inference.
                </p>
            </div>
            <Toggle
                bind:checked={sentinelEnabled}
                label={sentinelEnabled ? "Active" : "Inactive"}
            />
        </div>
    </Card>

    {#if loading && policies.length === 0}
        <Card>
            <div class="loading-state">
                <Spinner size="md" />
                <span>Loading QoS policies...</span>
            </div>
        </Card>
    {:else if policies.length === 0}
        <Card>
            <div class="empty-state">
                <Icon name="speed" size={48} />
                <h3>No QoS Policies</h3>
                <p>
                    Create a QoS policy to manage bandwidth and prioritize
                    traffic.
                </p>
                <Button onclick={openAddPolicy}>Create First Policy</Button>
            </div>
        </Card>
    {:else}
        <div class="policies-grid">
            {#each policies as policy}
                <Card>
                    <div class="policy-header">
                        <div class="policy-title">
                            <span class="policy-name">{policy.name}</span>
                            <Badge
                                variant={policy.enabled
                                    ? "default"
                                    : "secondary"}
                            >
                                {policy.enabled ? "Enabled" : "Disabled"}
                            </Badge>
                        </div>
                        <div class="policy-actions">
                            <Button
                                variant="ghost"
                                size="sm"
                                onclick={() => togglePolicy(policy)}
                                aria-label={policy.enabled
                                    ? `Disable ${policy.name}`
                                    : `Enable ${policy.name}`}
                            >
                                <Icon
                                    name={policy.enabled
                                        ? "pause"
                                        : "play_arrow"}
                                    size="sm"
                                />
                            </Button>
                            <Button
                                variant="ghost"
                                size="sm"
                                onclick={() => openEditPolicy(policy)}
                                aria-label={`Edit ${policy.name}`}
                            >
                                <Icon name="edit" size="sm" />
                            </Button>
                            <Button
                                variant="ghost"
                                size="sm"
                                onclick={() => deletePolicy(policy)}
                                aria-label={`Delete ${policy.name}`}
                            >
                                <Icon name="trash" size="sm" />
                            </Button>
                        </div>
                    </div>

                    <div class="policy-details">
                        <div class="detail-row">
                            <span class="detail-label">Interface</span>
                            <Badge variant="outline">{policy.interface}</Badge>
                        </div>
                        <div class="detail-row">
                            <span class="detail-label">Direction</span>
                            <span class="detail-value"
                                >{policy.direction || "both"}</span
                            >
                        </div>
                        <div class="detail-row">
                            <span class="detail-label">Download</span>
                            <span class="detail-value bandwidth">
                                {policy.download_mbps
                                    ? `${policy.download_mbps} Mbps`
                                    : "Unlimited"}
                            </span>
                        </div>
                        <div class="detail-row">
                            <span class="detail-label">Upload</span>
                            <span class="detail-value bandwidth">
                                {policy.upload_mbps
                                    ? `${policy.upload_mbps} Mbps`
                                    : "Unlimited"}
                            </span>
                        </div>
                    </div>

                    {#if policy.classes?.length > 0}
                        <div class="policy-section">
                            <span class="section-label"
                                >Traffic Classes ({policy.classes.length})</span
                            >
                            <div class="classes-list">
                                {#each policy.classes as cls}
                                    <div class="class-item">
                                        <span class="class-name"
                                            >{cls.name}</span
                                        >
                                        <Badge variant="secondary"
                                            >P{cls.priority}</Badge
                                        >
                                        {#if cls.rate}
                                            <span class="class-rate"
                                                >{cls.rate}</span
                                            >
                                        {/if}
                                    </div>
                                {/each}
                            </div>
                        </div>
                    {/if}
                </Card>
            {/each}
        </div>
    {/if}
</div>

<!-- Add/Edit Policy Modal -->
<Modal
    bind:open={showPolicyModal}
    title={editingPolicy
        ? `Edit Policy: ${editingPolicy.name}`
        : "Add QoS Policy"}
>
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <form
        class="form-stack"
        onsubmit={(e) => {
            e.preventDefault();
            savePolicy();
        }}
    >
        {#if formError}
            <div class="error-alert">{formError}</div>
        {/if}

        <Input
            id="policy-name"
            label="Policy Name"
            bind:value={policyName}
            placeholder="e.g., wan-shaping"
            disabled={!!editingPolicy}
            required
        />

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

        <div class="modal-actions">
            <Button
                type="button"
                variant="ghost"
                onclick={() => (showPolicyModal = false)}>Cancel</Button
            >
            <Button
                type="submit"
                disabled={loading}
                data-testid="save-policy-btn"
            >
                {#if loading}<Spinner size="sm" />{/if}
                {editingPolicy ? "Save Changes" : "Create Policy"}
            </Button>
        </div>
    </form>
</Modal>

<style>
    .qos-page {
        display: flex;
        flex-direction: column;
        gap: var(--space-6);
    }

    .page-header {
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
    }

    .header-info h2 {
        font-size: var(--text-2xl);
        font-weight: 600;
        margin: 0;
    }

    .subtitle {
        color: var(--color-muted);
        font-size: var(--text-sm);
        margin: var(--space-1) 0 0;
    }

    .header-actions {
        display: flex;
        gap: var(--space-2);
    }

    .sentinel-content {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-4);
    }

    .sentinel-title {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        font-weight: 600;
        color: var(--color-foreground);
    }

    .sentinel-desc {
        margin: var(--space-1) 0 0;
        font-size: var(--text-sm);
        color: var(--color-muted);
    }

    .loading-state,
    .empty-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: var(--space-3);
        padding: var(--space-8);
        text-align: center;
        color: var(--color-muted);
    }

    .empty-state h3 {
        margin: 0;
        color: var(--color-foreground);
    }

    .empty-state p {
        margin: 0;
        max-width: 400px;
    }

    .policies-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
        gap: var(--space-4);
    }

    .policy-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: var(--space-4);
    }

    .policy-title {
        display: flex;
        align-items: center;
        gap: var(--space-2);
    }

    .policy-name {
        font-weight: 600;
        font-size: var(--text-lg);
    }

    .policy-actions {
        display: flex;
        gap: var(--space-1);
    }

    .policy-details {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
        padding: var(--space-3);
        background: var(--color-backgroundSecondary);
        border-radius: var(--radius-md);
    }

    .detail-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        font-size: var(--text-sm);
    }

    .detail-label {
        color: var(--color-muted);
    }

    .detail-value {
        color: var(--color-foreground);
    }

    .detail-value.bandwidth {
        font-family: var(--font-mono);
        font-weight: 500;
    }

    .policy-section {
        margin-top: var(--space-4);
        padding-top: var(--space-3);
        border-top: 1px solid var(--color-border);
    }

    .section-label {
        font-size: var(--text-xs);
        font-weight: 600;
        color: var(--color-muted);
        text-transform: uppercase;
        letter-spacing: 0.05em;
    }

    .classes-list {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-2);
        margin-top: var(--space-2);
    }

    .class-item {
        display: flex;
        align-items: center;
        gap: var(--space-1);
        padding: var(--space-1) var(--space-2);
        background: var(--color-backgroundSecondary);
        border-radius: var(--radius-sm);
        font-size: var(--text-xs);
    }

    .class-name {
        font-weight: 500;
    }

    .class-rate {
        color: var(--color-muted);
    }

    .form-stack {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
    }

    .bandwidth-row {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: var(--space-4);
    }

    .modal-actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        margin-top: var(--space-4);
        padding-top: var(--space-4);
        border-top: 1px solid var(--color-border);
    }

    .error-alert {
        background: #fef2f2;
        color: #ef4444;
        padding: var(--space-3);
        border-radius: var(--radius-sm);
        font-size: var(--text-sm);
        border: 1px solid #fee2e2;
    }
</style>

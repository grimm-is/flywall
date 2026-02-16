<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import {
        Card,
        Button,
        Input,
        Select,
        PillInput,
        Toggle,
        Badge,
        Spinner,
        NetworkInput,
        Icon,
    } from "$lib/components";
    import { t } from "svelte-i18n";
    import {
        SERVICE_GROUPS,
        getService,
        type ServiceDefinition,
    } from "$lib/data/common_services";

    export let loading = false;
    export let availableIPSets: string[] = [];
    export let zones: any[] = [];

    const dispatch = createEventDispatcher();

    let ruleAction = "accept";
    let ruleName = "";
    let policyFrom = "";
    let policyTo = "";
    // Protocol selection (array for PillInput)
    let protocols: string[] = [];
    let ruleDestPort = "";
    let ruleSrc = "";
    let ruleDest = "";
    let selectedService = "";

    // Advanced options state
    let showAdvanced = false;
    let invertSrc = false;
    let invertDest = false;
    let tcpFlagsArray: string[] = [];
    let maxConnections = "";

    // Protocol options for PillInput
    const PROTOCOL_OPTIONS = [
        { value: "tcp", label: "TCP" },
        { value: "udp", label: "UDP" },
        { value: "icmp", label: "ICMP" },
    ];

    // TCP Flags options for PillInput
    const TCP_FLAG_OPTIONS = [
        { value: "syn", label: "SYN" },
        { value: "ack", label: "ACK" },
        { value: "fin", label: "FIN" },
        { value: "rst", label: "RST" },
        { value: "psh", label: "PSH" },
        { value: "urg", label: "URG" },
    ];

    // Reactive: when service changes, update protocol and port
    $: if (selectedService) {
        const svc = getService(selectedService);
        if (svc) {
            // Set protocols array
            if (svc.protocol === "both") {
                protocols = ["tcp", "udp"];
            } else if (svc.protocol === "tcp") {
                protocols = ["tcp"];
            } else if (svc.protocol === "udp") {
                protocols = ["udp"];
            }
            ruleDestPort = svc.port?.toString() || "";
            // Auto-generate rule name if empty
            if (!ruleName) {
                ruleName = `Allow ${svc.label}`;
            }
        }
    }

    function handleSave() {
        if (!ruleName) return;

        dispatch("save", {
            action: ruleAction,
            name: ruleName,
            policyFrom,
            policyTo,
            protocols,
            destPort: ruleDestPort,
            src: ruleSrc,
            dest: ruleDest,
            invertSrc,
            invertDest,
            tcpFlags: tcpFlagsArray,
            maxConnections,
        });
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card>
    <div class="create-card">
        <div class="header">
            <h3>
                {$t("common.add_item", { values: { item: $t("item.rule") } })}
            </h3>
        </div>

        <div class="form-grid">
            <Input
                id="rule-name"
                label={$t("common.name")}
                bind:value={ruleName}
                placeholder={$t("firewall.rule_name_placeholder")}
                required
            />

            <Select
                id="rule-from"
                label="From Zone"
                bind:value={policyFrom}
                options={zones.map((z) => ({ value: z.name, label: z.name }))}
                required
            />

            <Select
                id="rule-to"
                label="To Zone"
                bind:value={policyTo}
                options={zones.map((z) => ({ value: z.name, label: z.name }))}
                required
            />

            <Select
                id="rule-action"
                label={$t("common.action")}
                bind:value={ruleAction}
                options={[
                    { value: "accept", label: $t("firewall.accept") },
                    { value: "drop", label: $t("firewall.drop") },
                    { value: "reject", label: $t("firewall.reject") },
                ]}
            />

            <Select
                id="rule-service"
                label={$t("firewall.quick_service")}
                bind:value={selectedService}
                options={[
                    { value: "", label: "-- Custom / Manual --" },
                    ...SERVICE_GROUPS.flatMap((group) =>
                        group.services.map((svc) => ({
                            value: svc.name,
                            label: `${group.label}: ${svc.label} (${svc.protocol === "both" ? "tcp+udp" : svc.protocol}/${svc.port})`,
                        })),
                    ),
                ]}
            />

            <div class="full-width">
                <PillInput
                    id="rule-protocols"
                    label={$t("common.protocol")}
                    bind:value={protocols}
                    options={PROTOCOL_OPTIONS}
                    placeholder={$t("firewall.protocol_placeholder")}
                />
            </div>

            {#if !protocols.includes("icmp") || protocols.includes("tcp") || protocols.includes("udp")}
                <Input
                    id="rule-port"
                    label={$t("firewall.dest_port")}
                    bind:value={ruleDestPort}
                    placeholder={$t("firewall.dest_port_placeholder")}
                    type="text"
                />
            {/if}

            <NetworkInput
                id="rule-src"
                label={$t("firewall.source")}
                bind:value={ruleSrc}
                suggestions={availableIPSets}
                placeholder="IP, CIDR, Host, or IPSet"
            />
            <NetworkInput
                id="rule-dest"
                label={$t("firewall.destination")}
                bind:value={ruleDest}
                suggestions={availableIPSets}
                placeholder="IP, CIDR, Host, or IPSet"
            />
        </div>

        <!-- Advanced Options (collapsible) -->
        <details class="advanced-options" bind:open={showAdvanced}>
            <summary>{$t("firewall.advanced_options")}</summary>
            <div class="advanced-content form-grid">
                <Toggle
                    label={$t("firewall.invert_src")}
                    bind:checked={invertSrc}
                />
                <Toggle
                    label={$t("firewall.invert_dest")}
                    bind:checked={invertDest}
                />

                <div class="full-width">
                    <PillInput
                        id="rule-tcp-flags"
                        label={$t("firewall.tcp_flags")}
                        bind:value={tcpFlagsArray}
                        options={TCP_FLAG_OPTIONS}
                        placeholder={$t("firewall.tcp_flags_placeholder")}
                    />
                </div>

                <Input
                    id="rule-max-connections"
                    label={$t("firewall.max_connections")}
                    bind:value={maxConnections}
                    placeholder={$t("firewall.max_connections_placeholder")}
                    type="number"
                />
            </div>
        </details>

        <div class="actions">
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button
                variant="default"
                onclick={handleSave}
                disabled={loading || !ruleName}
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
        gap: var(--space-4);
        padding: var(--space-2);
    }

    .header h3 {
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
    }

    .form-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
        gap: var(--space-4);
    }

    .full-width {
        grid-column: 1 / -1;
    }

    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        margin-top: var(--space-2);
        padding-top: var(--space-4);
        border-top: 1px solid var(--color-border);
    }

    /* Advanced Details Styling */
    details.advanced-options {
        background: var(--color-backgroundSecondary);
        border-radius: var(--radius-md);
        border: 1px solid var(--color-border);
    }

    details.advanced-options summary {
        padding: var(--space-3);
        cursor: pointer;
        font-weight: 500;
        color: var(--color-muted);
        list-style: none; /* Hide default triangle in some browsers */
    }

    /* Custom marker for details */
    details.advanced-options summary::-webkit-details-marker {
        display: none;
    }

    details.advanced-options summary::after {
        content: "+";
        float: right;
        font-weight: bold;
    }

    details.advanced-options[open] summary::after {
        content: "-";
    }

    .advanced-content {
        padding: var(--space-4);
        border-top: 1px solid var(--color-border);
    }
</style>

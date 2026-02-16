<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { Card, Button, Input, Select, Spinner } from "$lib/components";
    import { t } from "svelte-i18n";

    export let loading = false;
    export let interfaces: any[] = [];

    const dispatch = createEventDispatcher();

    // Rule form
    let ruleType: "dnat" | "masquerade" | "snat" = "dnat";
    let ruleProtocol = "tcp";
    let ruleDestPort = "";
    let ruleToAddress = "";
    let ruleToPort = "";

    // Advanced fields
    let ruleSrcIP = "";
    let ruleSNATIP = "";
    let ruleMark = "";

    let ruleDescription = "";
    let ruleInterface = "";

    // Validation error state
    let validationError = "";

    function validateIPv4(ip: string): boolean {
        if (!ip) return false;
        const parts = ip.split(".");
        if (parts.length !== 4) return false;
        return parts.every((p) => {
            const n = parseInt(p, 10);
            return !isNaN(n) && n >= 0 && n <= 255;
        });
    }

    function validatePort(port: string): boolean {
        if (!port) return false;
        const n = parseInt(port, 10);
        return !isNaN(n) && n >= 1 && n <= 65535;
    }

    // Set default interface when available
    $: if (!ruleInterface && interfaces.length > 0) {
        const wanIface = interfaces.find((i: any) => i.Zone === "WAN");
        ruleInterface = wanIface?.Name || interfaces[0]?.Name || "";
    }

    function handleSave() {
        validationError = "";

        // Validation
        if (ruleType === "dnat") {
            if (!validatePort(ruleDestPort)) {
                validationError = "Invalid port number (1-65535 required)";
                return;
            }
            if (!validateIPv4(ruleToAddress)) {
                validationError =
                    "Invalid IP address format (e.g., 192.168.1.10)";
                return;
            }
            if (ruleToPort && !validatePort(ruleToPort)) {
                validationError = "Invalid forward port number";
                return;
            }
        } else if (ruleType === "snat") {
            if (!validateIPv4(ruleSNATIP)) {
                validationError = "Invalid SNAT IP address";
                return;
            }
        }

        dispatch("save", {
            type: ruleType,
            proto: ruleProtocol,
            destPort: ruleDestPort,
            toIP: ruleToAddress,
            toPort: ruleToPort,
            srcIP: ruleSrcIP,
            snatIP: ruleSNATIP,
            mark: ruleMark,
            description: ruleDescription,
            outInterface: ruleInterface,
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

        <div class="form-stack">
            <Select
                id="rule-type"
                label="Rule Type"
                bind:value={ruleType}
                options={[
                    { value: "dnat", label: "Port Forward (DNAT)" },
                    { value: "masquerade", label: "Masquerade (Auto SNAT)" },
                    { value: "snat", label: "Static SNAT" },
                ]}
            />

            {#if ruleType === "masquerade" || ruleType === "snat"}
                <Select
                    id="rule-interface"
                    label="Outbound Interface"
                    bind:value={ruleInterface}
                    options={interfaces.map((i: any) => ({
                        value: i.Name,
                        label: `${i.Name} (${i.Zone})`,
                    }))}
                />

                {#if ruleType === "snat"}
                    <Input
                        id="rule-snat-ip"
                        label="SNAT IP Address"
                        bind:value={ruleSNATIP}
                        placeholder="e.g. 1.2.3.4"
                        required
                    />

                    <Input
                        id="rule-src-ip"
                        label="Source IP Match (Optional)"
                        bind:value={ruleSrcIP}
                        placeholder="e.g. 10.0.0.0/24"
                    />

                    <Input
                        id="rule-mark"
                        label="Firewall Mark Match (Optional)"
                        bind:value={ruleMark}
                        type="number"
                        placeholder="e.g. 10"
                    />
                {/if}
            {:else}
                <div class="form-row">
                    <Select
                        id="rule-protocol"
                        label={$t("common.protocol")}
                        bind:value={ruleProtocol}
                        options={[
                            { value: "tcp", label: "TCP" },
                            { value: "udp", label: "UDP" },
                        ]}
                    />

                    <Input
                        id="rule-dest"
                        label="External Port"
                        bind:value={ruleDestPort}
                        placeholder="e.g., 443"
                        type="number"
                        required
                    />
                </div>

                <div class="form-row">
                    <Input
                        id="rule-to-addr"
                        label="Forward to Address"
                        bind:value={ruleToAddress}
                        placeholder="e.g., 192.168.1.10"
                        required
                    />

                    <Input
                        id="rule-to-port"
                        label="Forward to Port (optional)"
                        bind:value={ruleToPort}
                        placeholder="Same as external if blank"
                        type="number"
                    />
                </div>

                <Input
                    id="rule-desc"
                    label="Description"
                    bind:value={ruleDescription}
                    placeholder="e.g., Web Server"
                />
            {/if}

            {#if validationError}
                <div class="validation-error" role="alert" aria-live="polite">
                    {validationError}
                </div>
            {/if}
        </div>

        <div class="actions">
            <Button variant="ghost" onclick={handleCancel} disabled={loading}>
                {$t("common.cancel")}
            </Button>
            <Button variant="default" onclick={handleSave} disabled={loading}>
                {#if loading}<Spinner size="sm" />{/if}
                {$t("nat.add_rule")}
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

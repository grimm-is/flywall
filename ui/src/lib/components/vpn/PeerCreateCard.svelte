<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { Card, Button, Input, Spinner } from "$lib/components";
    import { t } from "svelte-i18n";

    export let loading = false;
    export let peer: any = null;

    const dispatch = createEventDispatcher();

    let peerName = "";
    let peerPublicKey = "";
    let peerPresharedKey = "";
    let peerEndpoint = "";
    let peerAllowedIps = "";
    let peerKeepalive = "25";

    $: if (peer) {
        peerName = peer.name || "";
        peerPublicKey = peer.public_key || "";
        peerPresharedKey = peer.preshared_key || "";
        peerEndpoint = peer.endpoint || "";
        peerAllowedIps = (peer.allowed_ips || []).join(", ");
        peerKeepalive = String(peer.persistent_keepalive || "25");
    } else {
        // Reset when switching to add mode if peer becomes null
        peerName = "";
        peerPublicKey = "";
        peerPresharedKey = "";
        peerEndpoint = "";
        peerAllowedIps = "";
        peerKeepalive = "25";
    }

    let validationError = "";

    function handleSave() {
        validationError = "";
        if (!peerName) validationError = "Name is required";
        else if (!peerPublicKey) validationError = "Public Key is required";

        if (validationError) return;

        dispatch("save", {
            name: peerName,
            public_key: peerPublicKey,
            preshared_key: peerPresharedKey || undefined,
            endpoint: peerEndpoint || undefined,
            allowed_ips: peerAllowedIps
                .split(",")
                .map((s) => s.trim())
                .filter(Boolean),
            persistent_keepalive: Number(peerKeepalive),
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
                {peer
                    ? "Edit Peer"
                    : $t("common.add_item", {
                          values: { item: $t("item.peer") },
                      })}
            </h3>
        </div>

        <div class="form-stack">
            <Input
                id="peer-name"
                label={$t("common.name")}
                bind:value={peerName}
                required
            />
            <Input
                id="peer-pubkey"
                label={$t("vpn.public_key")}
                bind:value={peerPublicKey}
                required
                placeholder="base64..."
            />
            <Input
                id="peer-psk"
                label={$t("vpn.preshared_key")}
                bind:value={peerPresharedKey}
                type="password"
            />
            <Input
                id="peer-endpoint"
                label={$t("vpn.endpoint")}
                bind:value={peerEndpoint}
                placeholder="ip:port"
            />
            <Input
                id="peer-ips"
                label={$t("vpn.allowed_ips")}
                bind:value={peerAllowedIps}
                placeholder="0.0.0.0/0"
            />
            <Input
                id="peer-ka"
                label={$t("vpn.keepalive")}
                type="number"
                bind:value={peerKeepalive}
            />

            {#if validationError}
                <div class="validation-error">
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
                    onclick={handleSave}
                    disabled={loading || !peerName || !peerPublicKey}
                >
                    {$t("common.save")}
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
    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        margin-top: var(--space-2);
        border-top: 1px solid var(--color-border);
        padding-top: var(--space-4);
    }
    .validation-error {
        color: var(--color-destructive);
        font-size: var(--text-sm);
    }
</style>

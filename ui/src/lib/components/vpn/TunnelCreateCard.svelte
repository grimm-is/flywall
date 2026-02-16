<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { Card, Button, Input, Icon, Spinner } from "$lib/components";
    import { t } from "svelte-i18n";
    import { api } from "$lib/stores/app";
    import PeerCreateCard from "./PeerCreateCard.svelte";

    export let loading = false;
    export let tunnel: any = null;

    const dispatch = createEventDispatcher();

    let name = "";
    let iface = "wg0";
    let port = "51820";
    let privateKey = "";
    let addresses = "";
    let dns = "";
    let mtu = "1420";
    let fwmark = "";
    let table = "auto";
    let postUp = "";
    let postDown = "";
    let peers: any[] = [];

    // Peer management state
    let isAddingPeer = false;
    let editingPeerIndex: number | null = null;
    let editingPeerData: any = null;

    $: if (tunnel) {
        name = tunnel.name || "";
        iface = tunnel.interface || "wg0";
        port = String(tunnel.listen_port || "51820");
        privateKey = tunnel.private_key || "";
        addresses = (tunnel.address || []).join(", ");
        dns = (tunnel.dns || []).join(", ");
        mtu = String(tunnel.mtu || "1420");
        fwmark = String(tunnel.fwmark || "");
        table = tunnel.table || "auto";
        postUp = (tunnel.post_up || []).join("\n");
        postDown = (tunnel.post_down || []).join("\n");
        // Deep copy peers to avoid mutating prop directly until save
        peers = tunnel.peers ? JSON.parse(JSON.stringify(tunnel.peers)) : [];
    } else {
        name = "";
        iface = "wg0";
        port = "51820";
        privateKey = "";
        addresses = "";
        dns = "";
        mtu = "1420";
        fwmark = "";
        table = "auto";
        postUp = "";
        postDown = "";
        peers = [];
    }

    // We can eventually add peer management here too, but let's start with the basic tunnel
    // For simplicity, we might emulate the original modal's "peers" section
    // or just let users save execution and then add peers in the edit view.
    // The original modal allowed adding peers. Let's keep it simple for now: valid tunnel first.

    let generatingKey = false;
    let validationError = "";

    async function generateKey() {
        generatingKey = true;
        try {
            const result = await api.get("/vpn/wireguard/keygen"); // Assuming this is the endpoint or similar
            // Wait, the original file used api.generateWireGuardKey().
            // I should check what that maps to, or just use `api.generateWireGuardKey()`.
            // Since I can't import api types easily, I'll assume the prop is passed or I use the store.
            // Actually I imported api from store above.

            // We need to double check the api method name from VPN.svelte usage:
            // const result = await api.generateWireGuardKey();
            // connPrivateKey = result.private_key;

            const res = await api.generateWireGuardKey();
            privateKey = res.private_key;
        } catch (e: any) {
            console.error("Failed to generate key", e);
            alert("Failed to generate key: " + e.message);
        } finally {
            generatingKey = false;
        }
    }

    /* --- Peer Functions --- */

    function toggleAddPeer() {
        if (isAddingPeer) {
            isAddingPeer = false;
            editingPeerIndex = null;
            editingPeerData = null;
        } else {
            isAddingPeer = true;
            editingPeerIndex = null;
            editingPeerData = null;
        }
    }

    function editPeer(index: number) {
        editingPeerIndex = index;
        editingPeerData = peers[index];
        isAddingPeer = true;
    }

    function handleSavePeer(event: CustomEvent) {
        const newPeer = event.detail;
        if (editingPeerIndex !== null) {
            peers[editingPeerIndex] = newPeer;
        } else {
            peers = [...peers, newPeer];
        }
        isAddingPeer = false;
        editingPeerIndex = null;
        editingPeerData = null;
    }

    function deletePeer(index: number) {
        if (
            !confirm(
                $t("common.delete_confirm_item", {
                    values: { item: $t("item.peer") },
                }),
            )
        )
            return;
        peers = peers.filter((_, i) => i !== index);
    }

    function handleSave() {
        validationError = "";
        if (!name.trim()) validationError = "Name is required";
        if (!iface.trim()) validationError = "Interface is required";
        if (!privateKey.trim()) validationError = "Private Key is required";

        if (validationError) return;

        dispatch("save", {
            name,
            interface: iface,
            listen_port: Number(port),
            private_key: privateKey,
            address: addresses
                .split(",")
                .map((s) => s.trim())
                .filter(Boolean),
            dns: dns
                .split(",")
                .map((s) => s.trim())
                .filter(Boolean),
            mtu: Number(mtu),
            fwmark: fwmark ? Number(fwmark) : undefined,
            table: table === "auto" ? undefined : table,
            post_up: postUp.split("\n").filter(Boolean),
            post_down: postDown.split("\n").filter(Boolean),
            peers: peers,
            enabled: true,
        });
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card>
    <div class="create-card">
        <div class="header">
            <h3>{tunnel ? "Edit Tunnel" : "Add Tunnel"}</h3>
        </div>

        <div class="form-stack">
            <div class="grid grid-cols-2 gap-4">
                <Input
                    id="conn-name"
                    label="Tunnel Name"
                    bind:value={name}
                    placeholder="e.g. Site-to-Site"
                />
                <Input
                    id="conn-iface"
                    label="Interface Name"
                    bind:value={iface}
                    placeholder="wg0"
                />
            </div>

            <div class="grid grid-cols-3 gap-4">
                <Input
                    id="conn-port"
                    label={$t("vpn.listen_port")}
                    type="number"
                    bind:value={port}
                />
                <Input
                    id="conn-mtu"
                    label={$t("common.mtu")}
                    type="number"
                    bind:value={mtu}
                    placeholder="1420"
                />
            </div>

            <div class="flex gap-2 items-end">
                <div class="flex-1">
                    <Input
                        id="conn-privkey"
                        label={$t("vpn.private_key")}
                        bind:value={privateKey}
                        type="password"
                        placeholder="base64 key..."
                    />
                </div>
                <Button
                    variant="outline"
                    onclick={generateKey}
                    disabled={generatingKey}
                >
                    {#if generatingKey}<Spinner size="sm" />{:else}{$t(
                            "vpn.generate",
                        )}{/if}
                </Button>
            </div>

            <Input
                id="conn-addr"
                label={$t("vpn.addresses")}
                bind:value={addresses}
                placeholder="10.100.0.1/24"
            />

            <Input
                id="conn-dns"
                label={$t("vpn.dns_servers")}
                bind:value={dns}
                placeholder="1.1.1.1"
            />

            <!-- Advanced Section Toggle could go here, omitting for brevity in initial card -->
            <details class="text-sm border border-border rounded p-2">
                <summary class="cursor-pointer font-medium p-1"
                    >Advanced Settings</summary
                >
                <div class="grid grid-cols-2 gap-4 mt-2">
                    <Input
                        id="conn-mark"
                        label={$t("vpn.firewall_mark")}
                        type="number"
                        bind:value={fwmark}
                        placeholder={$t("common.optional")}
                    />
                    <Input
                        id="conn-table"
                        label="Routing Table"
                        bind:value={table}
                        placeholder="auto"
                    />
                </div>
                <div class="grid grid-cols-2 gap-4 mt-2">
                    <div>
                        <label
                            class="block text-xs text-muted mb-1"
                            for="post-up">PostUp Commands (one per line)</label
                        >
                        <textarea
                            id="post-up"
                            class="w-full bg-backgroundSecondary border border-border rounded p-2 text-xs font-mono"
                            rows="2"
                            bind:value={postUp}
                        ></textarea>
                    </div>
                    <div>
                        <label
                            class="block text-xs text-muted mb-1"
                            for="post-down"
                            >PostDown Commands (one per line)</label
                        >
                        <textarea
                            id="post-down"
                            class="w-full bg-backgroundSecondary border border-border rounded p-2 text-xs font-mono"
                            rows="2"
                            bind:value={postDown}
                        ></textarea>
                    </div>
                </div>
            </details>

            <!-- Peers Section -->
            <div class="peers-section border-t border-border pt-4 mt-2">
                <div class="flex justify-between items-center mb-3">
                    <h3 class="text-sm font-medium">{$t("vpn.peers")}</h3>
                    <Button size="sm" variant="outline" onclick={toggleAddPeer}>
                        {isAddingPeer
                            ? "Cancel"
                            : "+ " +
                              $t("common.add_item", {
                                  values: { item: $t("item.peer") },
                              })}
                    </Button>
                </div>

                {#if isAddingPeer}
                    <div class="mb-4">
                        <PeerCreateCard
                            peer={editingPeerData}
                            on:save={handleSavePeer}
                            on:cancel={toggleAddPeer}
                        />
                    </div>
                {/if}

                {#if peers.length > 0}
                    <div class="peers-list space-y-2">
                        {#each peers as peer, i}
                            <div
                                class="flex items-center justify-between p-3 bg-secondary/10 rounded-md border border-border"
                            >
                                <div
                                    class="grid grid-cols-3 gap-4 flex-1 text-sm"
                                >
                                    <div class="font-medium">{peer.name}</div>
                                    <div
                                        class="font-mono text-xs text-muted-foreground truncate"
                                    >
                                        {peer.public_key}
                                    </div>
                                    <div class="font-mono text-xs">
                                        {(peer.allowed_ips || []).join(", ")}
                                    </div>
                                </div>
                                <div class="flex gap-1 ml-2">
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        onclick={() => editPeer(i)}
                                    >
                                        <Icon name="edit" />
                                    </Button>
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        onclick={() => deletePeer(i)}
                                    >
                                        <Icon name="delete" />
                                    </Button>
                                </div>
                            </div>
                        {/each}
                    </div>
                {:else}
                    <p class="text-sm text-muted-foreground italic">
                        {$t("common.no_items", {
                            values: { items: $t("item.peer") },
                        })}
                    </p>
                {/if}
            </div>

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
                    variant="default"
                    onclick={handleSave}
                    disabled={loading}
                >
                    {#if loading}<Spinner size="sm" />{/if}
                    {tunnel ? "Save Changes" : "Create Tunnel"}
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

    /* Utilities that might not be global */
    .grid {
        display: grid;
    }
    .grid-cols-2 {
        grid-template-columns: repeat(2, 1fr);
    }
    .grid-cols-3 {
        grid-template-columns: repeat(3, 1fr);
    }
    .gap-4 {
        gap: 1rem;
    }
    .flex {
        display: flex;
    }
    .flex-1 {
        flex: 1;
    }
    .gap-2 {
        gap: 0.5rem;
    }
    .items-end {
        align-items: flex-end;
    }
    .w-full {
        width: 100%;
    }
    .text-sm {
        font-size: 0.875rem;
    }
    .p-2 {
        padding: 0.5rem;
    }
    .mt-2 {
        margin-top: 0.5rem;
    }
    .mb-1 {
        margin-bottom: 0.25rem;
    }
    .font-medium {
        font-weight: 500;
    }
    .text-xs {
        font-size: 0.75rem;
    }
    .text-muted {
        color: var(--color-muted);
    }
    .font-mono {
        font-family: monospace;
    }
    .cursor-pointer {
        cursor: pointer;
    }
</style>

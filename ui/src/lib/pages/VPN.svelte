<script lang="ts">
  /**
   * VPN Page
   * WireGuard connection and peer management
   */

  import { config, api } from "$lib/stores/app";
  import {
    Card,
    Button,
    Modal,
    Input,
    Badge,
    Spinner,
    Icon,
    Table,
  } from "$lib/components";
  import { t } from "svelte-i18n";

  let loading = $state(false);
  let showConnModal = $state(false);
  let showPeerModal = $state(false);

  // Connection State
  let editingConnIndex = $state<number | null>(null);
  let connName = $state("");
  let connInterface = $state("wg0");
  let connPort = $state("51820");
  let connPrivateKey = $state("");
  let connAddresses = $state("");
  let connDns = $state("");
  let connMtu = $state("1420");
  let connMark = $state("");
  let connTable = $state("auto");
  let connPostUp = $state("");
  let connPostDown = $state("");
  let connPeers = $state<any[]>([]);
  let connEnabled = $state(true);

  let fileInput: HTMLInputElement;

  // Peer State (nested)
  let editingPeerIndex = $state<number | null>(null);
  let peerName = $state("");
  let peerPublicKey = $state("");
  let peerPresharedKey = $state("");
  let peerEndpoint = $state("");
  let peerAllowedIps = $state("");
  let peerKeepalive = $state("25");

  const vpnConfig = $derived($config?.vpn || { wireguard: [] });
  const connections = $derived(vpnConfig.wireguard || []);

  /* --- Connection Management --- */

  function openAddConnection() {
    editingConnIndex = null;
    connName = "New Connection";
    connInterface = "wg0";
    connPort = "51820";
    connPrivateKey = "";
    connAddresses = "";
    connDns = "";
    connMtu = "1420";
    connMark = "";
    connTable = "auto";
    connPostUp = "";
    connPostDown = "";
    connPeers = [];
    connEnabled = true;
    showConnModal = true;
  }

  function openEditConnection(index: number) {
    editingConnIndex = index;
    const c = connections[index];
    connName = c.name || "";
    connInterface = c.interface || "wg0";
    connPort = String(c.listen_port || "51820");
    connPrivateKey = c.private_key || "";
    connAddresses = (c.address || []).join(", ");
    connDns = (c.dns || []).join(", ");
    connMtu = String(c.mtu || "1420");
    connMark = String(c.fwmark || "");
    connTable = c.table || "auto";
    connPostUp = (c.post_up || []).join("\n") || "";
    connPostDown = (c.post_down || []).join("\n") || "";
    connPeers = [...(c.peers || [])];
    connEnabled = c.enabled !== false;
    showConnModal = true;
  }

  async function saveConnection() {
    loading = true;
    try {
      const newConn = {
        name: connName,
        interface: connInterface,
        listen_port: Number(connPort),
        private_key: connPrivateKey,
        address: connAddresses
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
        dns: connDns
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
        mtu: Number(connMtu),
        fwmark: connMark ? Number(connMark) : undefined,
        table: connTable === "auto" ? undefined : connTable,
        post_up: connPostUp.split("\n").filter(Boolean),
        post_down: connPostDown.split("\n").filter(Boolean),
        peers: connPeers,
        enabled: connEnabled,
      };

      let updatedWg = [...connections];
      if (editingConnIndex !== null) {
        updatedWg[editingConnIndex] = {
          ...updatedWg[editingConnIndex],
          ...newConn,
        };
      } else {
        updatedWg.push(newConn);
      }

      await api.updateVPN({
        ...vpnConfig,
        wireguard: updatedWg,
      });
      showConnModal = false;
    } catch (e) {
      console.error("Failed to save connection:", e);
    } finally {
      loading = false;
    }
  }

  async function deleteConnection(index: number) {
    if (
      !confirm(
        $t("common.delete_confirm_item", {
          values: { item: $t("item.interface") },
        }),
      )
    )
      return;
    loading = true;
    try {
      const updatedWg = connections.filter((_: any, i: number) => i !== index);
      await api.updateVPN({
        ...vpnConfig,
        wireguard: updatedWg,
      });
    } catch (e) {
      console.error("Failed to delete connection:", e);
    } finally {
      loading = false;
    }
  }

  /* --- Peer Management --- */

  function openAddPeer() {
    editingPeerIndex = null;
    peerName = "";
    peerPublicKey = "";
    peerPresharedKey = "";
    peerEndpoint = "";
    peerAllowedIps = "";
    peerKeepalive = "25";
    showPeerModal = true;
  }

  function openEditPeer(index: number) {
    editingPeerIndex = index;
    const p = connPeers[index];
    peerName = p.name || "";
    peerPublicKey = p.public_key || "";
    peerPresharedKey = p.preshared_key || "";
    peerEndpoint = p.endpoint || "";
    peerAllowedIps = (p.allowed_ips || []).join(", ");
    peerKeepalive = String(p.persistent_keepalive || "25");
    showPeerModal = true;
  }

  function savePeer() {
    const newPeer = {
      name: peerName,
      public_key: peerPublicKey,
      preshared_key: peerPresharedKey || undefined,
      endpoint: peerEndpoint || undefined,
      allowed_ips: peerAllowedIps
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean),
      persistent_keepalive: Number(peerKeepalive),
    };

    if (editingPeerIndex !== null) {
      connPeers[editingPeerIndex] = newPeer;
    } else {
      connPeers = [...connPeers, newPeer];
    }
    showPeerModal = false;
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
    connPeers = connPeers.filter((_, i) => i !== index);
  }

  async function generateKey() {
    loading = true;
    try {
      const result = await api.generateWireGuardKey();
      connPrivateKey = result.private_key;
      alert(
        `Key generated!\n\nPublic Key: ${result.public_key}\n\n(Public key is shown for sharing with peers. Private key has been filled in.)`,
      );
    } catch (e: any) {
      alert(`Failed to generate key: ${e.message || e}`);
      console.error("Failed to generate key:", e);
      loading = false;
    }
  }

  async function handleImport(e: Event) {
    const target = e.target as HTMLInputElement;
    if (!target.files?.length) return;
    const file = target.files[0];

    try {
      loading = true;
      const imported = await api.importVPNConfig(file);

      // Open as new connection
      editingConnIndex = null;
      connName = file.name.replace(".conf", "") || "Imported Tunnel";
      connInterface = imported.interface || "wg0"; // Parser might not set interface name
      connPort = String(imported.listen_port || "0");
      connPrivateKey = imported.private_key || "";
      connAddresses = (imported.address || []).join(", ");
      connDns = (imported.dns || []).join(", ");
      connMtu = String(imported.mtu || "1420");
      connMark = String(imported.fwmark || "");
      connTable = imported.table || "auto";
      connEnabled = true;

      // Handle peers
      if (imported.peers) {
        connPeers = imported.peers.map((p: any, i: number) => ({
          name: p.name || `Peer ${i + 1}`,
          public_key: p.public_key,
          preshared_key: p.preshared_key,
          endpoint: p.endpoint,
          allowed_ips: p.allowed_ips || [],
          persistent_keepalive: p.persistent_keepalive,
        }));
      } else {
        connPeers = [];
      }

      showConnModal = true;
    } catch (e: any) {
      alert("Import failed: " + (e.message || e));
    } finally {
      loading = false;
      target.value = "";
    }
  }

  function triggerImport() {
    fileInput?.click();
  }
</script>

<div class="vpn-page">
  <div class="page-header">
    <div class="header-info">
      <h2>Tunnels</h2>
      <p class="subtitle">Secure remote access & site-to-site links</p>
    </div>
    <div class="header-actions">
      <input
        type="file"
        accept=".conf"
        class="hidden"
        bind:this={fileInput}
        onchange={handleImport}
      />
      <Button variant="outline" onclick={triggerImport} disabled={loading}>
        <Icon name="upload" /> Import
      </Button>
      <Button onclick={openAddConnection} data-testid="add-tunnel-btn"
        >+ Add Tunnel</Button
      >
    </div>
  </div>

  {#if connections.length === 0}
    <Card>
      <div class="empty-state">
        <Icon name="vpn_key" size={48} className="text-muted" />
        <p class="empty-message">
          No active tunnels. Create a WireGuard interface to get started.
        </p>
        <Button variant="outline" onclick={openAddConnection}
          >Create Tunnel</Button
        >
      </div>
    </Card>
  {:else}
    <div class="connections-list">
      {#each connections as conn, i}
        <Card>
          <div class="conn-header">
            <div class="conn-title-group">
              <div
                class="status-dot {conn.enabled ? 'active' : 'inactive'}"
              ></div>
              <h3 class="text-lg font-bold">{conn.name}</h3>
              <Badge variant="outline" className="font-mono"
                >{conn.interface}</Badge
              >
            </div>
            <div class="conn-actions">
              <Button variant="ghost" onclick={() => openEditConnection(i)}>
                <Icon name="edit" />
              </Button>
              <Button variant="ghost" onclick={() => deleteConnection(i)}>
                <Icon name="delete" />
              </Button>
            </div>
          </div>

          <div
            class="conn-details grid grid-cols-2 md:grid-cols-4 gap-4 text-sm"
          >
            <div>
              <span class="detail-label">{$t("vpn.port")}</span>
              <span class="font-mono">{conn.listen_port}</span>
            </div>
            <div>
              <span class="detail-label">{$t("vpn.address")}</span>
              <span class="font-mono">{(conn.address || []).join(", ")}</span>
            </div>
            <div>
              <span class="detail-label">Public Key</span>
              <span
                class="font-mono text-xs truncate max-w-[150px] block"
                title={conn.public_key || "Generating..."}
              >
                {conn.public_key || "N/A"}
              </span>
            </div>
            <div>
              <span class="detail-label"
                >{$t("vpn.peers")} ({conn.peers?.length || 0})</span
              >
            </div>
          </div>

          <!-- Peer Preview (High Density) -->
          {#if conn.peers && conn.peers.length > 0}
            <div class="peers-preview">
              {#each conn.peers as peer}
                <div class="peer-chip">
                  <div class="peer-status"></div>
                  <span class="peer-name">{peer.name}</span>
                  <span class="peer-ip">{peer.allowed_ips?.[0] || ""}</span>
                </div>
              {/each}
            </div>
          {/if}
        </Card>
      {/each}
    </div>
  {/if}
</div>

<!-- Connection Modal -->
<Modal
  bind:open={showConnModal}
  title={editingConnIndex !== null ? "Edit Tunnel" : "Add Tunnel"}
  size="lg"
>
  <div class="form-stack">
    <div class="grid grid-cols-2 gap-4">
      <Input
        id="conn-name"
        label="Tunnel Name"
        bind:value={connName}
        placeholder="e.g. Site-to-Site"
        data-testid="vpn-conn-name"
      />
      <Input
        id="conn-iface"
        label="Interface Name"
        bind:value={connInterface}
        placeholder="wg0"
        data-testid="vpn-conn-iface"
      />
    </div>

    <div class="grid grid-cols-3 gap-4">
      <Input
        id="conn-port"
        label={$t("vpn.listen_port")}
        type="number"
        bind:value={connPort}
        data-testid="vpn-conn-port"
      />
      <Input
        id="conn-mtu"
        label={$t("common.mtu")}
        type="number"
        bind:value={connMtu}
        placeholder="1420"
      />
    </div>

    <!-- Advanced Settings Collapsible or Just Section -->
    <details class="text-sm border border-border rounded p-2">
      <summary class="cursor-pointer font-medium p-1">Advanced Settings</summary
      >
      <div class="grid grid-cols-2 gap-4 mt-2">
        <Input
          id="conn-mark"
          label={$t("vpn.firewall_mark")}
          type="number"
          bind:value={connMark}
          placeholder={$t("common.optional")}
        />
        <Input
          id="conn-table"
          label="Routing Table"
          bind:value={connTable}
          placeholder="auto (or ID)"
          title="Use 'auto' for default behavior, 'off' to disable routes, or a number for specific table."
        />
      </div>
      <div class="grid grid-cols-2 gap-4 mt-2">
        <div>
          <label class="block text-xs text-muted mb-1" for="post-up"
            >PostUp Commands (one per line)</label
          >
          <textarea
            id="post-up"
            class="w-full bg-backgroundSecondary border border-border rounded p-2 text-xs font-mono"
            rows="2"
            bind:value={connPostUp}
          ></textarea>
        </div>
        <div>
          <label class="block text-xs text-muted mb-1" for="post-down"
            >PostDown Commands (one per line)</label
          >
          <textarea
            id="post-down"
            class="w-full bg-backgroundSecondary border border-border rounded p-2 text-xs font-mono"
            rows="2"
            bind:value={connPostDown}
          ></textarea>
        </div>
      </div>
    </details>

    <div class="flex gap-2 items-end">
      <div class="flex-1">
        <Input
          id="conn-privkey"
          label={$t("vpn.private_key")}
          bind:value={connPrivateKey}
          type="password"
          placeholder="base64 key..."
          data-testid="vpn-conn-privkey"
        />
      </div>
      <Button variant="outline" onclick={generateKey}
        >{$t("vpn.generate")}</Button
      >
    </div>

    <Input
      id="conn-addr"
      label={$t("vpn.addresses")}
      bind:value={connAddresses}
      placeholder="10.100.0.1/24"
      data-testid="vpn-conn-addr"
    />

    <Input
      id="conn-dns"
      label={$t("vpn.dns_servers")}
      bind:value={connDns}
      placeholder="1.1.1.1"
      data-testid="vpn-conn-dns"
    />

    <div class="peers-section border-t border-border pt-4 mt-2">
      <div class="flex justify-between items-center mb-3">
        <h3 class="text-sm font-medium">{$t("vpn.peers")}</h3>
        <Button size="sm" variant="outline" onclick={openAddPeer}
          >+ {$t("common.add_item", {
            values: { item: $t("item.peer") },
          })}</Button
        >
      </div>

      {#if connPeers.length > 0}
        <div class="peers-list space-y-2">
          {#each connPeers as peer, i}
            <div
              class="flex items-center justify-between p-3 bg-secondary/10 rounded-md border border-border"
            >
              <div class="grid grid-cols-3 gap-4 flex-1 text-sm">
                <div class="font-medium">{peer.name}</div>
                <div class="font-mono text-xs text-muted-foreground truncate">
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
                  onclick={() => openEditPeer(i)}
                >
                  <Icon name="edit" />
                </Button>
                <Button variant="ghost" size="sm" onclick={() => deletePeer(i)}>
                  <Icon name="delete" />
                </Button>
              </div>
            </div>
          {/each}
        </div>
      {:else}
        <p class="text-sm text-muted-foreground italic">
          {$t("common.no_items", { values: { items: $t("item.peer") } })}
        </p>
      {/if}
    </div>

    <div class="modal-actions">
      <Button
        variant="ghost"
        onclick={() => {
          showConnModal = false;
        }}>{$t("common.cancel")}</Button
      >
      <Button onclick={saveConnection} disabled={loading}
        >{$t("common.save_item", {
          values: { item: $t("item.interface") },
        })}</Button
      >
    </div>
  </div>
</Modal>

<!-- Peer Modal (Stacked) -->
{#if showPeerModal}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
  >
    <div
      class="bg-background border border-border rounded-lg shadow-xl w-full max-w-md p-6 m-4"
      role="dialog"
    >
      <h3 class="text-lg font-semibold mb-4">
        {editingPeerIndex !== null
          ? $t("common.edit_item", { values: { item: $t("item.peer") } })
          : $t("common.add_item", { values: { item: $t("item.peer") } })}
      </h3>

      <div class="form-stack space-y-4">
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

        <div class="flex justify-end gap-2 mt-6 pt-4 border-t border-border">
          <Button variant="ghost" onclick={() => (showPeerModal = false)}
            >{$t("common.cancel")}</Button
          >
          <Button onclick={savePeer} disabled={!peerName || !peerPublicKey}
            >{$t("common.save_item", {
              values: { item: $t("item.peer") },
            })}</Button
          >
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .vpn-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
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

  .connections-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .conn-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-bottom: var(--space-2);
    border-bottom: 1px solid var(--color-border);
  }

  .conn-title-group {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--color-muted);
  }

  .status-dot.active {
    background: var(--color-success);
    box-shadow: 0 0 8px var(--color-success);
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-3);
    padding: var(--space-8);
    text-align: center;
  }

  .empty-message {
    color: var(--color-muted);
    font-size: var(--text-sm);
    margin: 0;
  }

  .form-stack {
    display: flex;
    flex-direction: column;
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

  .detail-label {
    display: block;
    color: var(--color-muted);
    font-size: var(--text-xs);
    margin-bottom: 2px;
  }

  .peers-preview {
    margin-top: var(--space-4);
    padding-top: var(--space-3);
    border-top: 1px dashed var(--color-border);
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .peer-chip {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: 4px 8px;
    background: var(--color-backgroundSecondary);
    border-radius: var(--radius-sm);
    font-size: var(--text-xs);
    border: 1px solid var(--color-border);
  }

  .peer-status {
    width: 6px;
    height: 6px;
    background: var(--color-success);
    border-radius: 50%;
  }

  .peer-name {
    font-weight: 500;
  }

  .peer-ip {
    font-family: var(--font-mono);
    color: var(--color-muted);
  }
</style>

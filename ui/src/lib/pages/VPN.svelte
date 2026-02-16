<script lang="ts">
  /**
   * VPN Page
   * WireGuard connection and peer management
   */

  import { onMount, onDestroy } from "svelte";
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
  import TunnelCreateCard from "$lib/components/vpn/TunnelCreateCard.svelte";
  import { t } from "svelte-i18n";

  let loading = $state(false);

  let showEditConnModal = $state(false); // Kept for import modal only? No, import uses it.
  // Wait, handleImport uses showEditConnModal. I should refactor Import to use isAddingTunnel.
  // We'll remove showEditConnModal and use isAddingTunnel for import flow.
  let isAddingTunnel = $state(false);

  // Imported tunnel data if any
  let importedTunnel = $state<any>(null);

  // Real-time stats
  let vpnStats = $state<any>({});
  let statsLoading = $state(true);
  let pollInterval: any;

  // Connection State
  let editingConnIndex = $state<number | null>(null);
  // specific conn vars removed as TunnelCreateCard handles them
  let fileInput: HTMLInputElement;

  onMount(() => {
    loadStats();
    pollInterval = setInterval(loadStats, 5000);
  });

  onDestroy(() => {
    if (pollInterval) clearInterval(pollInterval);
  });

  async function loadStats() {
    try {
      vpnStats = await api.get("/vpn/status");
    } catch (e) {
      console.warn("Failed to load VPN stats", e);
    } finally {
      statsLoading = false;
    }
  }

  function getPeerStatus(iface: string, pubKey: string) {
    const s = vpnStats[iface]?.[pubKey];
    if (!s) return "unknown";
    // Active if handshake < 3 mins ago
    if (s.last_handshake_seconds >= 0 && s.last_handshake_seconds < 180)
      return "active";
    return "inactive";
  }

  function formatBytes(bytes: number) {
    if (!bytes) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  }

  const vpnConfig = $derived($config?.vpn || { wireguard: [] });
  const connections = $derived(vpnConfig.wireguard || []);

  /* --- Connection Management --- */

  function toggleAddTunnel() {
    isAddingTunnel = !isAddingTunnel;
    importedTunnel = null;
  }

  async function handleCreateTunnel(event: CustomEvent) {
    const data = event.detail;
    loading = true;
    try {
      const newConn = {
        name: data.name,
        interface: data.interface,
        listen_port: data.listen_port,
        private_key: data.private_key,
        address: data.address,
        dns: data.dns,
        mtu: data.mtu,
        fwmark: data.fwmark,
        table: data.table,
        post_up: data.post_up,
        post_down: data.post_down,
        peers: data.peers,
        enabled: data.enabled,
      };

      const updatedWg = [...connections, newConn];
      await api.updateVPN({
        ...vpnConfig,
        wireguard: updatedWg,
      });
      isAddingTunnel = false;
      importedTunnel = null;
    } catch (e: any) {
      console.error("Failed to create tunnel", e);
      alert("Failed to create tunnel: " + e.message);
    } finally {
      loading = false;
    }
  }

  async function handleUpdateTunnel(event: CustomEvent) {
    const data = event.detail;
    loading = true;
    try {
      const newConn = {
        name: data.name,
        interface: data.interface,
        listen_port: data.listen_port,
        private_key: data.private_key,
        address: data.address,
        dns: data.dns,
        mtu: data.mtu,
        fwmark: data.fwmark,
        table: data.table,
        post_up: data.post_up,
        post_down: data.post_down,
        peers: data.peers,
        enabled: data.enabled,
      };

      const updatedWg = [...connections];
      if (editingConnIndex !== null) {
        updatedWg[editingConnIndex] = {
          ...updatedWg[editingConnIndex],
          ...newConn,
        };
      } else {
        // Fallback if index lost? Should not happen in edit mode
        updatedWg.push(newConn);
      }

      await api.updateVPN({
        ...vpnConfig,
        wireguard: updatedWg,
      });
      editingConnIndex = null;
    } catch (e: any) {
      console.error("Failed to update tunnel", e);
      alert("Failed to update tunnel: " + e.message);
    } finally {
      loading = false;
    }
  }

  function openEditConnection(index: number) {
    editingConnIndex = index;
    // No modal to open, just inline
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

  async function handleImport(e: Event) {
    const target = e.target as HTMLInputElement;
    if (!target.files?.length) return;
    const file = target.files[0];

    try {
      loading = true;
      const imported = await api.importVPNConfig(file);

      // Setup imported tunnel data for CreateCard
      importedTunnel = {
        name: file.name.replace(".conf", "") || "Imported Tunnel",
        interface: imported.interface || "wg0",
        listen_port: imported.listen_port || 0,
        private_key: imported.private_key || "",
        address: imported.address || [],
        dns: imported.dns || [],
        mtu: imported.mtu || 1420,
        fwmark: imported.fwmark,
        table: imported.table || "auto",
        post_up: imported.post_up || [],
        post_down: imported.post_down || [],
        peers:
          imported.peers?.map((p: any, i: number) => ({
            name: p.name || `Peer ${i + 1}`,
            public_key: p.public_key,
            preshared_key: p.preshared_key,
            endpoint: p.endpoint,
            allowed_ips: p.allowed_ips || [],
            persistent_keepalive: p.persistent_keepalive,
          })) || [],
      };

      // Open Add Tunnel mode with prepopulated data
      editingConnIndex = null;
      isAddingTunnel = true;
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
      <Button onclick={toggleAddTunnel} data-testid="add-tunnel-btn"
        >{isAddingTunnel ? "Cancel" : "+ Add Tunnel"}</Button
      >
    </div>
  </div>

  {#if isAddingTunnel}
    <div class="mb-4">
      <TunnelCreateCard
        {loading}
        tunnel={importedTunnel}
        on:save={handleCreateTunnel}
        on:cancel={toggleAddTunnel}
      />
    </div>
  {/if}

  {#if connections.length === 0}
    <Card>
      <div class="empty-state">
        <Icon name="vpn_key" size={48} className="text-muted" />
        <p class="empty-message">
          No active tunnels. Create a WireGuard interface to get started.
        </p>
        <Button variant="outline" onclick={toggleAddTunnel}
          >Create Tunnel</Button
        >
      </div>
    </Card>
  {:else}
    <div class="connections-list">
      {#each connections as conn, i}
        {#each connections as conn, i}
          {#if editingConnIndex === i}
            <TunnelCreateCard
              {loading}
              tunnel={conn}
              on:save={handleUpdateTunnel}
              on:cancel={() => (editingConnIndex = null)}
            />
          {:else}
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
                  <span class="font-mono"
                    >{(conn.address || []).join(", ")}</span
                  >
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
                    {@const status = getPeerStatus(
                      conn.interface,
                      peer.public_key,
                    )}
                    {@const stats = vpnStats[conn.interface]?.[peer.public_key]}
                    <div
                      class="peer-chip"
                      title={stats
                        ? `Last handshake: ${stats.last_handshake_seconds}s ago\nRX: ${formatBytes(stats.transfer_rx)}\nTX: ${formatBytes(stats.transfer_tx)}`
                        : statsLoading
                          ? "Loading stats..."
                          : "No stats"}
                    >
                      <div class="peer-status {status}"></div>
                      <span class="peer-name">{peer.name}</span>
                      <span class="peer-ip">{peer.allowed_ips?.[0] || ""}</span>
                      {#if statsLoading}
                        <Spinner size="sm" />
                      {/if}
                    </div>
                  {/each}
                </div>
              {/if}
            </Card>
          {/if}
        {/each}
      {/each}
    </div>
  {/if}
</div>

<!-- Connection Modal -->

<!-- Peer Modal (Stacked) -->

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
    background: var(--color-muted);
    border-radius: 50%;
  }

  .peer-status.active {
    background: var(--color-success);
    box-shadow: 0 0 4px var(--color-success);
  }

  .peer-status.inactive {
    background: var(--color-muted);
  }
</style>

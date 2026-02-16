<script lang="ts">
  import { api } from "$lib/stores/app"; // For mutations
  import { enrichedIdentities } from "$lib/stores/identity";
  import { groups } from "$lib/stores/app";

  import Card from "$lib/components/Card.svelte";
  import Table from "$lib/components/Table.svelte";
  import Icon from "$lib/components/Icon.svelte";
  import Badge from "$lib/components/Badge.svelte";
  import Input from "$lib/components/Input.svelte";
  import Button from "$lib/components/Button.svelte";
  import DeviceEditCard from "$lib/components/DeviceEditCard.svelte";

  // --- State ---
  let searchQuery = "";
  let editingIdentityIndex: number | null = null;
  let loading = false;

  // --- Derived ---
  // Filter identities
  $: filteredIdentities = $enrichedIdentities
    .filter((d) => {
      if (!searchQuery) return true;
      const q = searchQuery.toLowerCase();
      return (
        d.alias.toLowerCase().includes(q) ||
        d.owner.toLowerCase().includes(q) ||
        (d.groupName || "").toLowerCase().includes(q) ||
        d.macs.some((m: string) => m.toLowerCase().includes(q)) ||
        d.ips.some((ip) => ip.includes(q)) ||
        d.hostnames.some((h) => h.toLowerCase().includes(q))
      );
    })
    .sort((a, b) => {
      // Sort by online status first, then alias/mac
      if (a.online !== b.online) return a.online ? -1 : 1;
      return (a.alias || a.macs[0]).localeCompare(b.alias || b.macs[0]);
    });

  $: groupOptions = [
    { value: "", label: "No Group" },
    ...$groups.map((g) => ({ value: g.id, label: g.name })),
  ];

  // --- Actions ---
  function openEdit(index: number) {
    editingIdentityIndex = index;
  }

  async function handleSaveDevice(event: CustomEvent) {
    loading = true;
    try {
      await api.updateIdentity(event.detail);
      editingIdentityIndex = null;
    } catch (e) {
      console.error(e);
      alert("Failed to update identity");
    } finally {
      loading = false;
    }
  }

  // Unlink MAC (Advanced)
  async function unlinkMAC(mac: string) {
    if (
      !confirm(
        `Are you sure you want to unlink MAC ${mac}? It will create a new identity.`,
      )
    )
      return;
    try {
      await api.unlinkMAC(mac);
      // Refresh auto-happens via store
    } catch (e) {
      console.error(e);
      alert("Failed to unlink MAC");
    }
  }
</script>

<div class="devices-page">
  <div class="controls">
    <Input
      bind:value={searchQuery}
      placeholder="Search devices (MAC, IP, Name)..."
      class="max-w-md"
    />
    <!-- Potential filters here -->
  </div>

  <!-- Device List -->
  <Card>
    <Table
      columns={[
        { label: "Device", key: "alias", width: "25%" },
        { label: "Group", key: "group", width: "15%" },
        { label: "Network", key: "ip", width: "25%" },
        { label: "Details", key: "details", width: "25%" },
        { label: "", key: "actions", width: "10%" },
      ]}
      data={filteredIdentities}
    >
      {#snippet children(row: any, i: number)}
        {#if editingIdentityIndex === i}
          <!-- Inline Edit Card -->
          <td colspan="5">
            <DeviceEditCard
              identity={row}
              {groupOptions}
              {loading}
              on:save={handleSaveDevice}
              on:cancel={() => (editingIdentityIndex = null)}
            />
          </td>
        {:else}
          <!-- Device Name & Status -->
          <td class="col-device">
            <div class="device-cell">
              <div class="status-indicator" class:online={row.online}></div>
              <div class="device-info">
                <span class="device-name">
                  {row.alias || row.hostnames[0] || "Unknown Device"}
                </span>
                <span class="device-sub">
                  {#if row.owner}
                    <Icon name="person" size={12} /> {row.owner}
                  {:else if row.vendors.length > 0}
                    {row.vendors[0]}
                  {:else}
                    Generic Device
                  {/if}
                </span>
              </div>
            </div>
          </td>

          <!-- Group -->
          <td class="col-group">
            {#if row.groupName}
              <Badge variant="secondary">
                {row.groupName}
              </Badge>
            {:else}
              <span class="text-muted">-</span>
            {/if}
          </td>

          <!-- Network (IPs) -->
          <td class="col-network">
            <div class="network-info">
              {#each row.ips as ip}
                <span class="ip-tag">{ip}</span>
              {/each}
              {#if row.ips.length === 0}
                <span class="text-muted text-sm">No IP</span>
              {/if}
            </div>
          </td>

          <!-- Details (MACs) -->
          <td class="col-details">
            <div class="mac-list">
              {#each row.macs as mac}
                <div class="mac-item">
                  <span class="mac-text">{mac}</span>
                  {#if row.macs.length > 1}
                    <button
                      class="unlink-btn"
                      title="Unlink MAC"
                      on:click|stopPropagation={() => unlinkMAC(mac)}
                    >
                      <Icon name="link_off" size={12} />
                    </button>
                  {/if}
                </div>
              {/each}
            </div>
          </td>

          <!-- Actions -->
          <td class="col-actions">
            <Button size="sm" variant="outline" onclick={() => openEdit(i)}>
              Edit
            </Button>
          </td>
        {/if}
      {/snippet}
    </Table>
  </Card>
</div>

<style>
  .devices-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .controls {
    display: flex;
    justify-content: space-between;
  }

  /* Device Cell */
  .device-cell {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .status-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background-color: var(--color-gray-400);
  }

  .status-indicator.online {
    background-color: var(--color-success);
    box-shadow: 0 0 8px var(--color-success-transparent);
  }

  .device-info {
    display: flex;
    flex-direction: column;
  }

  .device-name {
    font-weight: 500;
    color: var(--dashboard-text);
  }

  .device-sub {
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
    display: flex;
    align-items: center;
    gap: 4px;
  }

  /* IP Tags */
  .network-info {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  .ip-tag {
    background: var(--dashboard-surface);
    padding: 2px 6px;
    border-radius: 4px;
    font-size: var(--text-xs);
    font-family: var(--font-mono);
    color: var(--dashboard-text-muted);
  }

  /* MAC List */
  .mac-list {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .mac-item {
    display: flex;
    align-items: center;
    gap: 6px;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
  }

  .unlink-btn {
    background: none;
    border: none;
    cursor: pointer;
    padding: 0;
    color: var(--color-danger);
    opacity: 0.5;
    display: flex;
  }

  .unlink-btn:hover {
    opacity: 1;
  }

  /* Form */
  .form-grid {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .form-group label {
    font-size: var(--text-sm);
    font-weight: 500;
    color: var(--dashboard-text);
  }

  .help {
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
  }

  .mac-info {
    margin-top: var(--space-2);
    padding: var(--space-3);
    background: var(--dashboard-surface);
    border-radius: var(--radius-md);
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
  }

  .mac-info ul {
    margin: var(--space-1) 0 0 calc(var(--space-4));
    padding: 0;
  }

  .text-muted {
    color: var(--dashboard-text-muted);
  }

  .text-sm {
    font-size: var(--text-sm);
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    margin-top: var(--space-6);
  }
</style>

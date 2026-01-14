<script lang="ts">
  import { onMount } from "svelte";
  import { api } from "$lib/stores/app";
  import { _ } from "svelte-i18n";

  interface UplinkStatus {
    name: string;
    type: string;
    interface: string;
    gateway: string;
    public_ip: string;
    healthy: boolean;
    enabled: boolean;
    latency: string;
    packet_loss: number;
    throughput: number;
    tier: number;
    weight: number;
  }

  interface UplinkGroup {
    name: string;
    uplinks: UplinkStatus[];
    active_uplinks: string[];
    active_tier: number;
    failover_mode: string;
    load_balance_mode: string;
  }

  let groups: UplinkGroup[] = $state([]);
  let loading = $state(true);
  let testingUplink = $state<string | null>(null);

  onMount(async () => {
    await loadGroups();
  });

  async function loadGroups() {
    loading = true;
    try {
      groups = await api.getUplinkGroups();
    } catch (e) {
      console.error("Failed to load uplink groups", e);
    }
    loading = false;
  }

  async function handleToggle(
    groupName: string,
    uplinkName: string,
    enabled: boolean,
  ) {
    try {
      await api.toggleUplink(groupName, uplinkName, enabled);
      await loadGroups();
    } catch (e) {
      console.error("Failed to toggle uplink", e);
    }
  }

  async function handleSwitch(groupName: string, uplinkName: string) {
    try {
      await api.switchUplink(groupName, uplinkName);
      await loadGroups();
    } catch (e) {
      console.error("Failed to switch uplink", e);
    }
  }

  async function handleTest(groupName: string, uplinkName: string) {
    testingUplink = `${groupName}:${uplinkName}`;
    try {
      const result = await api.testUplink(groupName, uplinkName);
      console.log("Test result:", result);
      await loadGroups();
    } catch (e) {
      console.error("Failed to test uplink", e);
    }
    testingUplink = null;
  }

  function getHealthClass(uplink: UplinkStatus): string {
    if (!uplink.enabled) return "disabled";
    if (!uplink.healthy) return "unhealthy";
    if (uplink.packet_loss > 5) return "degraded";
    return "healthy";
  }

  function formatLatency(latency: string): string {
    if (!latency || latency === "0s") return "-";
    return latency;
  }
</script>

<div class="uplinks-page">
  <header class="page-header">
    <h1>{$_("uplinks.title", { default: "Multi-WAN Uplinks" })}</h1>
    <button class="btn btn-secondary" onclick={() => loadGroups()}>
      ↻ {$_("common.refresh", { default: "Refresh" })}
    </button>
  </header>

  {#if loading}
    <div class="loading">{$_("common.loading", { default: "Loading..." })}</div>
  {:else if groups.length === 0}
    <div class="empty-state">
      <p>{$_("uplinks.empty", { default: "No uplink groups configured" })}</p>
      <p class="hint">
        {$_("uplinks.emptyHint", {
          default: "Configure Multi-WAN in the HCL config to enable failover.",
        })}
      </p>
    </div>
  {:else}
    {#each groups as group}
      <div class="group-card">
        <div class="group-header">
          <h2>{group.name}</h2>
          <div class="group-meta">
            <span class="badge">{group.failover_mode}</span>
            {#if group.load_balance_mode}
              <span class="badge secondary">{group.load_balance_mode}</span>
            {/if}
          </div>
        </div>

        <div class="uplinks-grid">
          {#each group.uplinks as uplink}
            {@const isActive = group.active_uplinks.includes(uplink.name)}
            {@const isTesting =
              testingUplink === `${group.name}:${uplink.name}`}
            <div
              class="uplink-card {getHealthClass(uplink)}"
              class:active={isActive}
            >
              <div class="uplink-header">
                <span class="uplink-name">{uplink.name}</span>
                <span class="health-chip {getHealthClass(uplink)}">
                  {#if !uplink.enabled}
                    Disabled
                  {:else if uplink.healthy}
                    Healthy
                  {:else}
                    Unhealthy
                  {/if}
                </span>
              </div>

              <div class="uplink-details">
                <div class="detail-row">
                  <span class="label">Interface</span>
                  <span class="value">{uplink.interface}</span>
                </div>
                <div class="detail-row">
                  <span class="label">Gateway</span>
                  <span class="value">{uplink.gateway || "-"}</span>
                </div>
                <div class="detail-row">
                  <span class="label">Latency</span>
                  <span class="value">{formatLatency(uplink.latency)}</span>
                </div>
                <div class="detail-row">
                  <span class="label">Packet Loss</span>
                  <span class="value">{uplink.packet_loss.toFixed(1)}%</span>
                </div>
                <div class="detail-row">
                  <span class="label">Tier</span>
                  <span class="value">{uplink.tier}</span>
                </div>
              </div>

              <div class="uplink-actions">
                <label class="toggle-label">
                  <input
                    type="checkbox"
                    checked={uplink.enabled}
                    onchange={(e) =>
                      handleToggle(
                        group.name,
                        uplink.name,
                        e.currentTarget.checked,
                      )}
                  />
                  Enabled
                </label>
                <button
                  class="btn btn-sm"
                  disabled={isActive || !uplink.enabled}
                  onclick={() => handleSwitch(group.name, uplink.name)}
                >
                  Switch To
                </button>
                <button
                  class="btn btn-sm btn-secondary"
                  disabled={isTesting}
                  onclick={() => handleTest(group.name, uplink.name)}
                >
                  {isTesting ? "Testing..." : "Test"}
                </button>
              </div>

              {#if isActive}
                <div class="active-badge">ACTIVE</div>
              {/if}
            </div>
          {/each}
        </div>
      </div>
    {/each}
  {/if}
</div>

<style>
  .uplinks-page {
    padding: 1.5rem;
    max-width: 1200px;
    margin: 0 auto;
  }

  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1.5rem;
  }

  .page-header h1 {
    margin: 0;
    font-size: 1.5rem;
  }

  .loading,
  .empty-state {
    text-align: center;
    padding: 3rem;
    color: var(--text-muted, #888);
  }

  .empty-state .hint {
    font-size: 0.9rem;
    opacity: 0.7;
  }

  .group-card {
    background: var(--card-bg, #1a1a2e);
    border-radius: 12px;
    padding: 1.5rem;
    margin-bottom: 1.5rem;
    border: 1px solid var(--border-color, #333);
  }

  .group-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .group-header h2 {
    margin: 0;
    font-size: 1.2rem;
  }

  .group-meta {
    display: flex;
    gap: 0.5rem;
  }

  .badge {
    background: var(--accent-color, #6c5ce7);
    color: white;
    padding: 0.25rem 0.75rem;
    border-radius: 20px;
    font-size: 0.75rem;
    text-transform: uppercase;
  }

  .badge.secondary {
    background: var(--secondary-color, #444);
  }

  .uplinks-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 1rem;
  }

  .uplink-card {
    background: var(--card-inner-bg, #232342);
    border-radius: 8px;
    padding: 1rem;
    position: relative;
    border: 2px solid transparent;
    transition:
      border-color 0.2s,
      box-shadow 0.2s;
  }

  .uplink-card.active {
    border-color: var(--success-color, #00b894);
    box-shadow: 0 0 12px rgba(0, 184, 148, 0.2);
  }

  .uplink-card.unhealthy {
    border-color: var(--danger-color, #ff6b6b);
  }

  .uplink-card.degraded {
    border-color: var(--warning-color, #fdcb6e);
  }

  .uplink-card.disabled {
    opacity: 0.6;
  }

  .uplink-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.75rem;
  }

  .uplink-name {
    font-weight: 600;
    font-size: 1rem;
  }

  .health-chip {
    padding: 0.2rem 0.6rem;
    border-radius: 12px;
    font-size: 0.7rem;
    font-weight: 600;
    text-transform: uppercase;
  }

  .health-chip.healthy {
    background: rgba(0, 184, 148, 0.2);
    color: var(--success-color, #00b894);
  }

  .health-chip.unhealthy {
    background: rgba(255, 107, 107, 0.2);
    color: var(--danger-color, #ff6b6b);
  }

  .health-chip.degraded {
    background: rgba(253, 203, 110, 0.2);
    color: var(--warning-color, #fdcb6e);
  }

  .health-chip.disabled {
    background: rgba(136, 136, 136, 0.2);
    color: var(--text-muted, #888);
  }

  .uplink-details {
    margin-bottom: 0.75rem;
  }

  .detail-row {
    display: flex;
    justify-content: space-between;
    font-size: 0.85rem;
    padding: 0.2rem 0;
  }

  .detail-row .label {
    color: var(--text-muted, #888);
  }

  .uplink-actions {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    margin-top: 0.75rem;
    padding-top: 0.75rem;
    border-top: 1px solid var(--border-color, #333);
  }

  .toggle-label {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.85rem;
    cursor: pointer;
  }

  .btn {
    padding: 0.4rem 0.75rem;
    border-radius: 6px;
    border: none;
    cursor: pointer;
    font-size: 0.85rem;
    background: var(--accent-color, #6c5ce7);
    color: white;
    transition: opacity 0.2s;
  }

  .btn:hover:not(:disabled) {
    opacity: 0.9;
  }

  .btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .btn-sm {
    padding: 0.3rem 0.6rem;
    font-size: 0.8rem;
  }

  .btn-secondary {
    background: var(--secondary-color, #444);
  }

  .active-badge {
    position: absolute;
    top: 0.5rem;
    right: 0.5rem;
    background: var(--success-color, #00b894);
    color: white;
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
    font-size: 0.65rem;
    font-weight: 700;
  }
</style>

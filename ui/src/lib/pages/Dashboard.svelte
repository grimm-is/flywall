<script lang="ts">
  /**
   * Glass Chassis Dashboard
   * High-tech system overview with deep interactivity
   */

  import { onMount, onDestroy } from "svelte";
  import {
    config,
    status,
    leases,
    brand,
    api,
    logs,
    alertStore,
  } from "$lib/stores/app";
  import { t } from "svelte-i18n";
  import { Button, Badge, Spinner, Modal, Icon } from "$lib/components";
  import Sparkline from "$lib/components/Sparkline.svelte";

  let showRebootModal = $state(false);
  let rebooting = $state(false);
  let loading = $state(true);
  let interfaceStatus = $state<any[]>([]);
  let pollInterval: any;

  // Traffic Stats
  let currentRxRate = $state(0);
  let currentTxRate = $state(0);
  let peakRxRate = $state(0);
  let peakTxRate = $state(0);
  let totalPackets = $state(0);

  const activeLeases = $derived(
    $leases?.filter((l: any) => l.active).length || 0,
  );
  const totalLeases = $derived($leases?.length || 0);
  const isForwarding = $derived($config?.ip_forwarding ?? false);

  // Computed Stats
  const systemLoad = $derived($status?.load_average?.[0]?.toFixed(2) || "0.00");
  const uptime = $derived($status?.uptime || "Unknown");

  // Real-time Traffic History
  let trafficHistory = $state<number[]>(Array(50).fill(0));

  onMount(async () => {
    await loadData();
    pollInterval = setInterval(() => {
      loadStats();
    }, 2000);
  });

  onDestroy(() => {
    if (pollInterval) clearInterval(pollInterval);
  });

  async function loadData() {
    try {
      await Promise.all([loadInterfaces(), loadStats()]);
    } finally {
      loading = false;
    }
  }

  async function loadInterfaces() {
    try {
      // Load config primarily for names/status
      const res = await api.getInterfaces();
      const rawData = Array.isArray(res) ? res : res.interfaces || [];

      // We will merge stats in loadStats, but init here
      interfaceStatus = rawData.map((s: any) => ({
        ...s,
        stats: {
          rx_bytes: 0,
          tx_bytes: 0,
          rx_bytes_per_sec: 0,
          tx_bytes_per_sec: 0,
        },
      }));
    } catch (e) {
      console.error("Failed to load interfaces", e);
      alertStore.notify("Failed to load intefaces", "error");
    }
  }

  async function loadStats() {
    try {
      const statsMap = await api.get("/traffic"); // Returns map[string]InterfaceStats

      // Update interfaceStatus with real stats
      interfaceStatus = interfaceStatus.map((iface) => {
        const s = statsMap[iface.name];
        return {
          ...iface,
          stats: s || iface.stats,
        };
      });

      // Calculate Aggregates
      let rxBps = 0;
      let txBps = 0;
      let packets = 0;

      Object.values(statsMap).forEach((s: any) => {
        rxBps += s.rx_bytes_per_sec || 0;
        txBps += s.tx_bytes_per_sec || 0;
        packets += (s.rx_packets || 0) + (s.tx_packets || 0);
      });

      currentRxRate = rxBps;
      currentTxRate = txBps;
      totalPackets = packets;

      // Update Peaks
      if (currentRxRate > peakRxRate) peakRxRate = currentRxRate;
      if (currentTxRate > peakTxRate) peakTxRate = currentTxRate;

      // Update Graph (Total Throughput)
      trafficHistory = [
        ...trafficHistory.slice(1),
        currentRxRate + currentTxRate,
      ];
    } catch (e) {
      console.warn("Failed to load traffic stats", e);
      // Optional: Don't spam toasts on every poll failure, maybe check if connected?
      // For now, just log to console to avoid spamming the user every 2s if backend is down
    }
  }

  function formatRate(bytesPerSec: number) {
    if (!bytesPerSec) return "0 B/s";
    const k = 1024;
    const sizes = ["B/s", "KB/s", "MB/s", "GB/s"];
    const i = Math.floor(Math.log(bytesPerSec) / Math.log(k));
    return (
      parseFloat((bytesPerSec / Math.pow(k, i)).toFixed(1)) +
      " " +
      (sizes[i] || "B/s")
    );
  }

  function formatBytes(bytes: number, decimals = 1) {
    if (!bytes) return "0 B";
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + " " + sizes[i];
  }

  function formatCount(num: number) {
    if (num > 1000000) return (num / 1000000).toFixed(1) + "M";
    if (num > 1000) return (num / 1000).toFixed(1) + "K";
    return num.toString();
  }

  async function handleReboot() {
    rebooting = true;
    try {
      await api.reboot();
    } catch (e) {
      console.error("Reboot failed:", e);
      rebooting = false;
    }
  }

  async function toggleForwarding() {
    try {
      await api.setIPForwarding(!isForwarding);
      await api.reloadConfig();
    } catch (e) {
      console.error("Failed to toggle forwarding:", e);
    }
  }
</script>

<div class="glass-dashboard">
  {#if loading}
    <div class="loading-overlay">
      <Spinner size="lg" />
      <p>{$t("dashboard.initializing")}</p>
    </div>
  {/if}
  <!-- Header / HUD -->
  <header class="hud-header">
    <div class="system-ident">
      <h1 class="text-glow">{$brand?.name || "SYSTEM"}</h1>
      <Badge
        variant={isForwarding ? "success" : "destructive"}
        class="status-badge"
      >
        {isForwarding
          ? $t("dashboard.online").toUpperCase()
          : $t("dashboard.offline").toUpperCase()}
      </Badge>
    </div>
    <div class="hud-metrics">
      <div class="metric-group">
        <span class="metric-label">{$t("dashboard.uptime")}</span>
        <span class="metric-value">{uptime}</span>
      </div>
      <div class="metric-group">
        <span class="metric-label">{$t("dashboard.load")}</span>
        <span class="metric-value">{systemLoad}</span>
      </div>
      <div class="metric-group">
        <span class="metric-label">{$t("dashboard.version")}</span>
        <span class="metric-value">{$status?.version || "0.0.0"}</span>
      </div>
    </div>
  </header>

  <!-- Main Console Grid -->
  <div class="console-grid">
    <!-- Left: Traffic & Interfaces -->
    <div class="console-main">
      <!-- Traffic Graph -->
      <div class="glass-panel p-6 mb-6">
        <div class="panel-header">
          <h3>{$t("dashboard.traffic")}</h3>
          <span class="live-indicator">● {$t("dashboard.live")}</span>
        </div>
        <div class="traffic-chart">
          <Sparkline
            data={trafficHistory}
            width={600}
            height={60}
            color="var(--color-primary)"
            showArea={true}
          />
        </div>
        <div class="traffic-stats">
          <div class="t-stat">
            <span class="t-label">{$t("dashboard.rx_rate")}</span>
            <span class="metric-value">{formatRate(currentRxRate)}</span>
            <span class="text-xs text-muted"
              >{$t("dashboard.peak", {
                values: { rate: formatRate(peakRxRate) },
              })}</span
            >
          </div>
          <div class="t-stat">
            <span class="t-label">{$t("dashboard.tx_rate")}</span>
            <span class="metric-value">{formatRate(currentTxRate)}</span>
            <span class="text-xs text-muted"
              >{$t("dashboard.peak", {
                values: { rate: formatRate(peakTxRate) },
              })}</span
            >
          </div>
          <div class="t-stat">
            <span class="t-label">{$t("dashboard.packets")}</span>
            <span class="metric-value">{formatCount(totalPackets)}</span>
          </div>
        </div>
      </div>

      <!-- Active Interfaces Grid -->
      <div class="interfaces-grid">
        {#each interfaceStatus.slice(0, 6) as iface}
          <a
            href="/network?tab=interfaces"
            class="glass-panel glass-panel-hover iface-card"
          >
            <div class="iface-header">
              <span class="iface-name">{iface.name}</span>
              <div
                class="status-dot"
                class:active={iface.link_up || iface.state === "up"}
              ></div>
            </div>
            <div class="iface-details">
              <span class="detail"
                >{iface.ipv4_addrs?.[0]?.split("/")[0] || "No IP"}</span
              >
              <div class="flex flex-col mt-1">
                <span class="detail muted text-xs"
                  >RX: {formatRate(iface.stats?.rx_bytes_per_sec || 0)}</span
                >
                <span class="detail muted text-xs"
                  >TX: {formatRate(iface.stats?.tx_bytes_per_sec || 0)}</span
                >
              </div>
            </div>
          </a>
        {/each}
      </div>
    </div>

    <!-- Right: Quick Actions & Status -->
    <div class="console-sidebar">
      <div class="glass-panel p-4 action-panel">
        <h3>{$t("dashboard.quick_actions")}</h3>
        <div class="action-list">
          <Button
            variant="outline"
            class="w-full justify-start gap-2"
            onclick={toggleForwarding}
          >
            <Icon name={isForwarding ? "power_off" : "power"} size={18} />
            {isForwarding
              ? $t("dashboard.disable_forwarding")
              : $t("dashboard.enable_forwarding")}
          </Button>
          <Button
            variant="outline"
            class="w-full justify-start gap-2"
            href="/rules"
          >
            <Icon name="shield" size={18} />
            {$t("dashboard.firewall_rules")}
          </Button>
          <Button
            variant="outline"
            class="w-full justify-start gap-2"
            href="/network?tab=dhcp"
          >
            <Icon name="dns" size={18} />
            {$t("dashboard.dhcp_leases", { values: { n: activeLeases } })}
          </Button>
          <Button
            variant="destructive"
            class="w-full justify-start gap-2"
            onclick={() => (showRebootModal = true)}
          >
            <Icon name="restart_alt" size={18} />
            {$t("dashboard.reboot_system")}
          </Button>
        </div>
      </div>

      <!-- Alerts / Security Status -->
      <div class="glass-panel p-4 mt-4 security-panel">
        <h3>{$t("dashboard.system_status")}</h3>
        <div class="sec-stat">
          <span class="stat-label">{$t("dashboard.protection")}</span>
          <Badge
            variant={$config?.protection?.enabled ? "success" : "secondary"}
            >{$config?.protection?.enabled
              ? $t("common.enabled").toUpperCase()
              : $t("common.disabled").toUpperCase()}</Badge
          >
        </div>
        <div class="sec-stat">
          <span class="stat-label">{$t("dashboard.active_zones")}</span>
          <span class="metric-value">{$config?.zones?.length || 0}</span>
        </div>
        <div class="sec-stat">
          <span class="stat-label">{$t("dashboard.blocked_hosts")}</span>
          <span class="metric-value text-destructive"
            >{$status?.blocked_count || 0}</span
          >
        </div>
      </div>
    </div>
  </div>

  <!-- Bottom: Logs Terminal -->
  <div class="logs-terminal glass-panel p-4">
    <div class="terminal-header">
      <h3>{$t("dashboard.system_logs")}</h3>
      <Button variant="ghost" size="sm" href="/logs"
        >{$t("dashboard.view_all")}</Button
      >
    </div>
    <div class="log-window">
      {#each $logs.slice(-5).reverse() as log}
        <div class="log-line">
          <span class="log-time"
            >[{new Date(log.timestamp).toLocaleTimeString()}]</span
          >
          <span
            class="log-source"
            class:text-primary={log.source === "api"}
            class:text-warning={log.source === "firewall"}
            >{log.source.toUpperCase()}</span
          >
          <span class="log-msg">{log.message}</span>
        </div>
      {:else}
        <div class="log-line text-muted">{$t("dashboard.no_recent_logs")}</div>
      {/each}
    </div>
  </div>
</div>

<!-- Reboot Confirmation -->
<Modal bind:open={showRebootModal} title={$t("dashboard.reboot_system")}>
  <p>{$t("dashboard.reboot_warning")}</p>
  <div class="modal-actions">
    <Button variant="ghost" onclick={() => (showRebootModal = false)}
      >{$t("common.cancel")}</Button
    >
    <Button variant="destructive" onclick={handleReboot} disabled={rebooting}>
      {#if rebooting}<Spinner size="sm" />{/if}
      {$t("dashboard.reboot_now")}
    </Button>
  </div>
</Modal>

<style>
  .glass-dashboard {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    padding: var(--space-2);
    position: relative;
    min-height: 400px;
  }

  .loading-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-4);
    background: rgba(var(--color-background-rgb), 0.8);
    backdrop-filter: blur(8px);
    z-index: 50;
    border-radius: var(--radius-lg);
    color: var(--color-muted);
  }

  /* HUD Header */
  .hud-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-end;
    padding-bottom: var(--space-4);
    border-bottom: 1px solid var(--color-borderSubtle);
  }

  .system-ident {
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }

  .system-ident h1 {
    font-size: var(--text-3xl);
    font-weight: 700;
    margin: 0;
    letter-spacing: -0.05em;
  }

  .hud-metrics {
    display: flex;
    gap: var(--space-8);
  }

  .metric-group {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
  }

  .metric-label {
    font-size: var(--text-xs);
    color: var(--color-muted);
    font-weight: 600;
    letter-spacing: 0.1em;
  }

  .metric-value {
    font-size: var(--text-xl);
  }

  /* Console Grid */
  .console-grid {
    display: grid;
    grid-template-columns: 1fr 300px; /* Sidebar fixed width */
    gap: var(--space-6);
  }

  @media (max-width: 1024px) {
    .console-grid {
      grid-template-columns: 1fr;
    }
  }

  .panel-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: var(--space-4);
  }

  .panel-header h3 {
    font-size: var(--text-sm);
    font-weight: 600;
    letter-spacing: 0.1em;
    color: var(--color-muted);
    margin: 0;
  }

  .live-indicator {
    color: var(--color-success);
    font-size: var(--text-xs);
    font-weight: 600;
    animation: pulse 2s infinite;
  }

  /* Traffic Stats */
  .traffic-stats {
    display: flex;
    gap: var(--space-8);
    margin-top: var(--space-4);
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-borderSubtle);
  }

  .t-stat {
    display: flex;
    flex-direction: column;
  }

  .t-stat .t-label {
    font-size: var(--text-xs);
    color: var(--color-muted);
    font-weight: 600;
  }

  .t-stat .metric-value {
    font-size: var(--text-lg);
  }

  /* Interfaces Grid */
  .interfaces-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: var(--space-4);
  }

  .iface-card {
    padding: var(--space-4);
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    text-decoration: none;
    color: inherit;
    min-height: 100px;
  }

  .iface-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: var(--space-2);
  }

  .iface-name {
    font-family: var(--font-mono);
    font-weight: 600;
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background-color: var(--color-destructive);
  }

  .status-dot.active {
    background-color: var(--color-success);
    box-shadow: 0 0 8px var(--color-success);
  }

  .iface-details {
    display: flex;
    flex-direction: column;
    font-size: var(--text-sm);
  }

  .iface-details .muted {
    color: var(--color-muted);
    font-size: var(--text-xs);
    margin-top: 2px;
  }

  /* Sidebar */
  .action-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    margin-top: var(--space-4);
  }

  .sec-stat {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: var(--space-3) 0;
    border-bottom: 1px solid var(--color-borderSubtle);
  }

  .sec-stat:last-child {
    border-bottom: none;
  }

  .sec-stat .stat-label {
    font-size: var(--text-sm);
    color: var(--color-muted);
  }

  /* Logs Terminal */
  .logs-terminal {
    max-height: 300px;
    display: flex;
    flex-direction: column;
  }

  .terminal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: var(--space-2);
    padding-bottom: var(--space-2);
    border-bottom: 1px solid var(--color-borderSubtle);
  }

  .terminal-header h3 {
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--color-muted);
    margin: 0;
  }

  .log-window {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding-right: var(--space-2);
  }

  .log-line {
    display: grid;
    grid-template-columns: 80px 80px 1fr;
    gap: var(--space-4);
    padding: 2px 0;
    border-bottom: 1px solid transparent;
  }

  .log-line:hover {
    background: rgba(0, 0, 0, 0.02);
  }

  .log-time {
    color: var(--color-muted);
  }

  .log-source {
    font-weight: 600;
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    margin-top: var(--space-6);
  }

  @keyframes pulse {
    0% {
      opacity: 1;
    }
    50% {
      opacity: 0.5;
    }
    100% {
      opacity: 1;
    }
  }
</style>

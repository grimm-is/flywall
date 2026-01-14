<script lang="ts">
  import { page } from "$app/stores";
  import { onMount, onDestroy } from "svelte";
  import Icon from "$lib/components/Icon.svelte";
  import {
    flowsStore,
    formatBytes,
    formatAge,
    formatRate,
    type Flow,
  } from "$lib/stores/flows";
  import { api } from "$lib/stores/app";

  // Active view from URL or default
  let activeView = $derived(() => {
    const view = $page.url.searchParams.get("view");
    return view || "flows";
  });

  // Time range filter
  let timeRange = $state("1m");

  const views = [
    { id: "flows", label: "Flows", icon: "swap_horiz" },
    { id: "routes", label: "Route Table", icon: "route" },
    { id: "audit", label: "Audit Logs", icon: "history" },
    { id: "traffic", label: "Traffic Logs", icon: "traffic" },
    { id: "capture", label: "Packet Capture", icon: "radio_button_checked" },
  ];

  function setView(viewId: string) {
    const url = new URL(window.location.href);
    url.searchParams.set("view", viewId);
    window.history.pushState({}, "", url.toString());
  }

  // Subscribe to flows store
  let flowsState = $derived($flowsStore);
  let flows = $derived(flowsState.flows);

  // Routes from API
  let routes = $state<any[]>([]);
  let routesLoading = $state(false);

  // Audit logs from API
  let auditLogs = $state<any[]>([]);
  let auditLoading = $state(false);

  // Traffic logs
  let trafficLogs = $state<any[]>([]);
  let trafficLoading = $state(false);

  // Capture state
  let captureInterface = $state("any");
  let captureFilter = $state("");
  let captureDuration = $state(30);
  let captureCount = $state(1000);
  let captureStatus = $state<any>(null);
  let captureLoading = $state(false);
  let capturePollTimer = $state<number | null>(null);
  let interfaces = $state<any[]>([]);

  onMount(() => {
    // Start polling flows when component mounts
    flowsStore.startPolling(2000);
    loadRoutes();
    loadAuditLogs();
    loadInterfaces();
    checkCaptureStatus();
    loadTrafficLogs();
  });

  onDestroy(() => {
    // Stop polling when component unmounts
    flowsStore.stopPolling();
    if (capturePollTimer) clearInterval(capturePollTimer);
  });

  async function loadInterfaces() {
    try {
      const ifaces = await api.getInterfaces();
      interfaces = ifaces || [];
    } catch (e) {
      console.error("Failed to load interfaces", e);
    }
  }

  async function checkCaptureStatus() {
    try {
      captureStatus = await api.getCaptureStatus();

      if (captureStatus?.running) {
        if (!capturePollTimer) {
          capturePollTimer = setInterval(
            checkCaptureStatus,
            1000,
          ) as unknown as number;
        }
      } else {
        if (capturePollTimer) {
          clearInterval(capturePollTimer);
          capturePollTimer = null;
        }
      }
    } catch (e) {
      console.error("Failed to check capture status", e);
    }
  }

  async function startCapture() {
    captureLoading = true;
    try {
      await api.startCapture({
        interface: captureInterface,
        filter: captureFilter,
        duration: captureDuration,
        count: captureCount,
      });
      await checkCaptureStatus();
    } catch (e) {
      alert("Failed to start capture: " + (e as Error).message);
    } finally {
      captureLoading = false;
    }
  }

  async function stopCapture() {
    captureLoading = true;
    try {
      await api.stopCapture();
      await checkCaptureStatus();
    } catch (e) {
      alert("Failed to stop capture: " + (e as Error).message);
    } finally {
      captureLoading = false;
    }
  }

  function downloadCapture() {
    window.open("/api/debug/capture/download", "_blank");
  }

  async function loadRoutes() {
    routesLoading = true;
    try {
      const response = await fetch("/api/system/routes", {
        credentials: "include",
      });
      if (response.ok) {
        routes = await response.json();
      }
    } catch (e) {
      console.error("Failed to load routes:", e);
    } finally {
      routesLoading = false;
    }
  }

  async function loadAuditLogs() {
    auditLoading = true;
    try {
      const response = await fetch("/api/audit", {
        credentials: "include",
      });
      if (response.ok) {
        auditLogs = await response.json();
      }
    } catch (e) {
      console.error("Failed to load audit logs:", e);
    } finally {
      auditLoading = false;
    }
  }

  async function loadTrafficLogs() {
    trafficLoading = true;
    try {
      const result = await api.getLogs({ source: "nftables", limit: 100 });
      trafficLogs = result.entries || [];
    } catch (e) {
      console.error("Failed to load traffic logs:", e);
    } finally {
      trafficLoading = false;
    }
  }

  async function killFlow(flow: Flow) {
    if (
      confirm(
        `Kill connection from ${flow.src_hostname || flow.src_ip} to ${flow.dst_hostname || flow.dst_ip}?`,
      )
    ) {
      try {
        await flowsStore.kill(flow.id);
      } catch (e) {
        alert(
          "Failed to kill flow: " +
            (e instanceof Error ? e.message : "Unknown error"),
        );
      }
    }
  }

  async function blockFlow(flow: Flow) {
    if (confirm(`Permanently block ${flow.dst_ip}? This adds a policy rule.`)) {
      try {
        await flowsStore.block(flow);
      } catch (e) {
        alert(
          "Failed to block flow: " +
            (e instanceof Error ? e.message : "Unknown error"),
        );
      }
    }
  }

  // Format protocol display
  function formatProto(flow: Flow): string {
    const proto = flow.protocol?.toUpperCase() || "TCP";
    if (flow.dst_port) {
      return `${proto}:${flow.dst_port}`;
    }
    return proto;
  }

  // Format rate from flow data
  function getRate(flow: Flow): string {
    const bytes = (flow.bytes_sent || 0) + (flow.bytes_recv || 0);
    const age = flow.age_seconds || 1;
    return formatRate(bytes, age);
  }
</script>

<div class="observatory-page">
  <header class="page-header">
    <div class="header-left">
      <h1>Observatory</h1>
      {#if flowsState.loading}
        <span class="loading-indicator">⟳</span>
      {/if}
      {#if flowsState.lastUpdate}
        <span class="last-update"
          >Updated {flowsState.lastUpdate.toLocaleTimeString()}</span
        >
      {/if}
    </div>
    <div class="header-controls">
      <span class="flow-count">{flows.length} active flows</span>
      <select class="time-select" bind:value={timeRange}>
        <option value="1m">Last 1 min</option>
        <option value="5m">Last 5 min</option>
        <option value="15m">Last 15 min</option>
        <option value="1h">Last 1 hour</option>
      </select>
    </div>
  </header>

  <!-- View Tabs -->
  <nav class="view-bar">
    {#each views as view}
      <button
        class="view-btn"
        class:active={activeView() === view.id}
        onclick={() => setView(view.id)}
      >
        <Icon name={view.icon} size={16} />
        {view.label}
      </button>
    {/each}
  </nav>

  <!-- View Content -->
  <div class="view-content">
    {#if activeView() === "flows"}
      <!-- Flow Table -->
      {#if flowsState.error}
        <div class="error-banner">
          <Icon name="error" size={16} />
          {flowsState.error}
        </div>
      {/if}
      <table class="flow-table">
        <thead>
          <tr>
            <th>Source</th>
            <th>Destination</th>
            <th>Protocol</th>
            <th>Traffic</th>
            <th>Age</th>
            <th>Action</th>
          </tr>
        </thead>
        <tbody>
          {#each flows as flow (flow.id)}
            <tr>
              <td>
                <span class="host-name"
                  >{flow.src_hostname || flow.src_zone || "Unknown"}</span
                >
                <span class="host-ip font-mono"
                  >{flow.src_ip}:{flow.src_port}</span
                >
              </td>
              <td>
                <span class="host-name"
                  >{flow.dst_hostname || flow.dst_zone || "Unknown"}</span
                >
                <span class="host-ip font-mono"
                  >{flow.dst_ip}:{flow.dst_port}</span
                >
              </td>
              <td><span class="proto font-mono">{formatProto(flow)}</span></td>
              <td>
                <span class="rate">{getRate(flow)}</span>
                <span class="bytes font-mono"
                  >{formatBytes(
                    (flow.bytes_sent || 0) + (flow.bytes_recv || 0),
                  )}</span
                >
              </td>
              <td
                ><span class="age">{formatAge(flow.age_seconds || 0)}</span></td
              >
              <td class="action-cell">
                <button
                  class="btn-kill"
                  onclick={() => killFlow(flow)}
                  title="Kill connection (5min block)"
                >
                  <Icon name="close" size={14} />
                  Kill
                </button>
                <button
                  class="btn-block"
                  onclick={() => blockFlow(flow)}
                  title="Permanently block"
                >
                  <Icon name="block" size={14} />
                </button>
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="6" class="empty-row">
                {#if flowsState.loading}
                  Loading flows...
                {:else}
                  No active flows
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else if activeView() === "routes"}
      <!-- Route Table -->
      <table class="route-table">
        <thead>
          <tr>
            <th>Destination</th>
            <th>Gateway</th>
            <th>Interface</th>
            <th>Metric</th>
            <th>Protocol</th>
          </tr>
        </thead>
        <tbody>
          {#each routes as route}
            <tr>
              <td
                ><span class="font-mono">{route.destination || route.dest}</span
                ></td
              >
              <td><span class="font-mono">{route.gateway || "-"}</span></td>
              <td>{route.interface || route.iface}</td>
              <td>{route.metric || 0}</td>
              <td
                ><span class="proto-badge"
                  >{route.protocol || route.proto || "kernel"}</span
                ></td
              >
            </tr>
          {:else}
            <tr>
              <td colspan="5" class="empty-row">
                {#if routesLoading}
                  Loading routes...
                {:else}
                  No routes
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else if activeView() === "audit"}
      <!-- Audit Logs -->
      <table class="audit-table">
        <thead>
          <tr>
            <th>Time</th>
            <th>User</th>
            <th>Action</th>
            <th>Details</th>
          </tr>
        </thead>
        <tbody>
          {#each auditLogs as log}
            <tr>
              <td><span class="font-mono">{log.timestamp || log.time}</span></td
              >
              <td>{log.user || log.username}</td>
              <td><span class="action-badge">{log.action || log.type}</span></td
              >
              <td class="details">{log.details || log.message}</td>
            </tr>
          {:else}
            <tr>
              <td colspan="4" class="empty-row">
                {#if auditLoading}
                  Loading audit logs...
                {:else}
                  No audit logs
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else if activeView() === "traffic"}
      <!-- Traffic Logs -->
      <table class="traffic-table">
        <thead>
          <tr>
            <th>Time</th>
            <th>Class</th>
            <th>Source</th>
            <th>Destination</th>
            <th>Protocol</th>
            <th>Len</th>
          </tr>
        </thead>
        <tbody>
          {#each trafficLogs as log}
            <tr>
              <td><span class="font-mono">{formatAge(log.timestamp)}</span></td>
              <td>
                {#if log.class}
                  <span
                    class="badge class-{log.class
                      .toLowerCase()
                      .replace(/\s+/g, '-')}">{log.class}</span
                  >
                {:else}
                  <span class="badge class-unknown">Unknown</span>
                {/if}
              </td>
              <td>{log.extra?.SRC || "-"}:{log.extra?.SPT || ""}</td>
              <td>{log.extra?.DST || "-"}:{log.extra?.DPT || ""}</td>
              <td
                ><span class="proto-badge"
                  >{log.extra?.PROTO || log.protocol || "IP"}</span
                ></td
              >
              <td>{log.extra?.LEN || "-"}</td>
            </tr>
          {:else}
            <tr>
              <td colspan="6" class="empty-row">
                {#if trafficLoading}
                  Loading traffic logs...
                {:else}
                  No traffic logs found
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else if activeView() === "capture"}
      <div class="capture-panel">
        {#if captureStatus?.running}
          <div class="capture-running">
            <div class="capture-status-header">
              <span class="status-indicator running"></span>
              <h3>Capture in Progress</h3>
            </div>

            <div class="capture-metric-grid">
              <div class="metric-card">
                <span class="metric-label">Interface</span>
                <span class="metric-value">{captureStatus.interface}</span>
              </div>
              <div class="metric-card">
                <span class="metric-label">Filter</span>
                <span class="metric-value font-mono"
                  >{captureStatus.filter || "none"}</span
                >
              </div>
              <div class="metric-card">
                <span class="metric-label">File Size</span>
                <span class="metric-value"
                  >{formatBytes(captureStatus.size || 0)}</span
                >
              </div>
            </div>

            <button
              class="btn-stop"
              onclick={stopCapture}
              disabled={captureLoading}
            >
              <Icon name="stop" size={16} />
              Stop Capture
            </button>
          </div>
        {:else}
          <div class="capture-form">
            <div class="form-grid">
              <div class="form-group">
                <label for="iface">Interface</label>
                <select id="iface" bind:value={captureInterface}>
                  <option value="any">Any</option>
                  {#each interfaces as iface}
                    <option value={iface.name}>{iface.name}</option>
                  {/each}
                </select>
              </div>
              <div class="form-group">
                <label for="filter">Filter (tcpdump syntax)</label>
                <input
                  id="filter"
                  type="text"
                  bind:value={captureFilter}
                  placeholder="port 80 or host 10.0.0.1"
                />
              </div>
              <div class="form-group">
                <label for="duration">Duration (sec)</label>
                <input
                  id="duration"
                  type="number"
                  bind:value={captureDuration}
                  min="1"
                  max="300"
                />
              </div>
              <div class="form-group">
                <label for="count">Packet Count</label>
                <input
                  id="count"
                  type="number"
                  bind:value={captureCount}
                  min="1"
                  max="10000"
                />
              </div>
            </div>

            <div class="capture-actions">
              <button
                class="btn-start"
                onclick={startCapture}
                disabled={captureLoading}
              >
                {#if captureLoading}
                  <span class="spinner">⟳</span>
                {:else}
                  <Icon name="play_arrow" size={16} />
                {/if}
                Start Capture
              </button>

              {#if captureStatus?.size > 0}
                <button class="btn-download" onclick={downloadCapture}>
                  <Icon name="download" size={16} />
                  Download Last Capture ({formatBytes(captureStatus.size)})
                </button>
              {/if}
            </div>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .observatory-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .page-header h1 {
    font-size: var(--text-2xl);
    font-weight: 600;
    color: var(--dashboard-text);
  }

  .loading-indicator {
    animation: spin 1s linear infinite;
    color: var(--color-primary);
  }

  @keyframes spin {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }

  .last-update {
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
  }

  .header-controls {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .flow-count {
    font-size: var(--text-sm);
    color: var(--dashboard-text-muted);
  }

  .time-select {
    padding: var(--space-2) var(--space-3);
    background: var(--dashboard-card);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-md);
    color: var(--dashboard-text);
    font-size: var(--text-sm);
  }

  /* View Bar */
  .view-bar {
    display: flex;
    gap: var(--space-1);
    padding: var(--space-1);
    background: var(--dashboard-card);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-lg);
  }

  .view-btn {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    background: none;
    border: none;
    border-radius: var(--radius-md);
    color: var(--dashboard-text-muted);
    font-size: var(--text-sm);
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .view-btn:hover {
    background: var(--dashboard-input);
    color: var(--dashboard-text);
  }

  .view-btn.active {
    background: var(--color-primary);
    color: var(--color-primaryForeground);
  }

  /* Tables */
  .view-content {
    background: var(--dashboard-card);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-lg);
    overflow: hidden;
  }

  .error-banner {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3);
    background: var(--color-destructive);
    color: var(--color-destructiveForeground);
    font-size: var(--text-sm);
  }

  table {
    width: 100%;
    border-collapse: collapse;
  }

  th {
    text-align: left;
    padding: var(--space-3);
    background: var(--dashboard-input);
    font-size: var(--text-xs);
    font-weight: 600;
    color: var(--dashboard-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  td {
    padding: var(--space-3);
    border-top: 1px solid var(--dashboard-border);
    font-size: var(--text-sm);
    color: var(--dashboard-text);
  }

  tr:hover td {
    background: var(--dashboard-input);
  }

  .host-name {
    display: block;
    font-weight: 500;
  }

  .host-ip {
    display: block;
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
  }

  .proto {
    font-size: var(--text-xs);
    color: var(--color-primary);
  }

  .rate {
    display: block;
    font-weight: 600;
    color: var(--color-success);
  }

  .bytes {
    display: block;
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
  }

  .age {
    color: var(--dashboard-text-muted);
    font-size: var(--text-xs);
  }

  .action-cell {
    display: flex;
    gap: var(--space-1);
  }

  .btn-kill {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-1) var(--space-2);
    background: var(--color-destructive);
    color: var(--color-destructiveForeground);
    border: none;
    border-radius: var(--radius-sm);
    font-size: var(--text-xs);
    cursor: pointer;
  }

  .btn-block {
    display: flex;
    align-items: center;
    padding: var(--space-1);
    background: none;
    border: 1px solid var(--color-destructive);
    border-radius: var(--radius-sm);
    color: var(--color-destructive);
    cursor: pointer;
  }

  .proto-badge,
  .action-badge {
    display: inline-block;
    padding: var(--space-1) var(--space-2);
    background: var(--dashboard-input);
    border-radius: var(--radius-sm);
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
  }

  .details {
    color: var(--dashboard-text-muted);
    font-size: var(--text-xs);
  }

  .empty-row {
    text-align: center;
    color: var(--dashboard-text-muted);
    padding: var(--space-8);
  }

  /* Capture Panel */
  .capture-panel {
    padding: var(--space-6);
  }

  .capture-running {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-6);
    padding: var(--space-8);
    text-align: center;
  }

  .capture-status-header {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .capture-status-header h3 {
    margin: 0;
    font-size: var(--text-xl);
    font-weight: 600;
    color: var(--dashboard-text);
  }

  .status-indicator {
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background: var(--dashboard-border);
  }

  .status-indicator.running {
    background: var(--color-destructive);
    animation: pulse 1.5s infinite;
    box-shadow: 0 0 0 4px rgba(239, 68, 68, 0.2);
  }

  @keyframes pulse {
    0% {
      transform: scale(0.95);
      opacity: 0.8;
    }
    50% {
      transform: scale(1.05);
      opacity: 1;
    }
    100% {
      transform: scale(0.95);
      opacity: 0.8;
    }
  }

  .capture-metric-grid {
    display: flex;
    gap: var(--space-8);
  }

  .metric-card {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .metric-label {
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
    text-transform: uppercase;
    font-weight: 600;
  }

  .metric-value {
    font-size: var(--text-lg);
    font-weight: 500;
    color: var(--dashboard-text);
    font-family: var(--font-mono);
  }

  .capture-form {
    max-width: 600px;
    margin: 0 auto;
  }

  .form-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: var(--space-4);
    margin-bottom: var(--space-6);
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .form-group label {
    font-size: var(--text-xs);
    font-weight: 500;
    color: var(--dashboard-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .form-group input,
  .form-group select {
    padding: var(--space-2) var(--space-3);
    background: var(--dashboard-input);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-md);
    color: var(--dashboard-text);
    font-size: var(--text-sm);
    font-family: var(--font-mono);
  }

  .form-group input:focus,
  .form-group select:focus {
    outline: none;
    border-color: var(--color-primary);
    box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
  }

  .capture-actions {
    display: flex;
    gap: var(--space-4);
  }

  .btn-start,
  .btn-stop,
  .btn-download {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-6);
    border: none;
    border-radius: var(--radius-md);
    font-size: var(--text-sm);
    font-weight: 500;
    cursor: pointer;
    transition: all var(--transition-fast);
    min-width: 140px;
  }

  .btn-start {
    background: var(--color-primary);
    color: var(--color-primaryForeground);
    flex: 1;
  }

  .btn-stop {
    background: var(--color-destructive);
    color: var(--color-destructiveForeground);
  }

  .btn-download {
    background: var(--dashboard-card);
    border: 1px solid var(--dashboard-border);
    color: var(--dashboard-text);
  }

  .btn-start:hover,
  .btn-stop:hover {
    filter: brightness(1.1);
  }

  .btn-download:hover {
    background: var(--dashboard-input);
  }

  .btn-start:disabled,
  .btn-stop:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .spinner {
    animation: spin 1s linear infinite;
    display: inline-block;
  }

  /* Traffic Badges */
  .badge {
    display: inline-block;
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
    font-size: var(--text-xs);
    font-weight: 500;
  }
  .class-web {
    background: rgba(59, 130, 246, 0.1);
    color: #3b82f6;
  }
  .class-streaming {
    background: rgba(168, 85, 247, 0.1);
    color: #a855f7;
  }
  .class-gaming {
    background: rgba(239, 68, 68, 0.1);
    color: #ef4444;
  }
  .class-voip {
    background: rgba(34, 197, 94, 0.1);
    color: #22c55e;
  }
  .class-file-transfer {
    background: rgba(245, 158, 11, 0.1);
    color: #f59e0b;
  }
  .class-infrastructure {
    background: rgba(100, 116, 139, 0.1);
    color: #64748b;
  }
  .class-unknown {
    background: var(--dashboard-input);
    color: var(--dashboard-text-muted);
  }

  /* Mobile */
  @media (max-width: 768px) {
    .view-content {
      overflow-x: auto;
    }

    table {
      min-width: 600px;
    }
  }
</style>

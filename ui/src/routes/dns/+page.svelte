<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { api } from "$lib/stores/app";
  import Icon from "$lib/components/Icon.svelte";
  import { fade } from "svelte/transition";

  let loading = true;
  let error: string | null = null;
  let pollInterval: any;

  // Data
  let stats: any = null;
  let history: any[] = [];

  // Filters
  let limit = 100;
  let offset = 0;
  let search = "";
  let autoRefresh = true;

  // Status mapping
  const statusColors: Record<string, string> = {
    allowed: "text-green-500",
    blocked: "text-red-500",
    cache: "text-blue-500",
  };

  const statusIcons: Record<string, string> = {
    allowed: "check_circle",
    blocked: "block",
    cache: "cached",
  };

  async function loadData() {
    try {
      const [statsData, historyData] = await Promise.all([
        api.getDNSStats(),
        api.getDNSQueryHistory(limit, offset, search),
      ]);
      stats = statsData;
      history = historyData;
      error = null;
    } catch (e: any) {
      console.error("Failed to load DNS data", e);
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function startPolling() {
    if (autoRefresh && !pollInterval) {
      pollInterval = setInterval(loadData, 2000);
    }
  }

  function stopPolling() {
    if (pollInterval) {
      clearInterval(pollInterval);
      pollInterval = null;
    }
  }

  function toggleAutoRefresh() {
    autoRefresh = !autoRefresh;
    if (autoRefresh) {
      startPolling();
    } else {
      stopPolling();
    }
  }

  function formatTime(ts: string) {
    return new Date(ts).toLocaleTimeString();
  }

  function formatDuration(ms: number) {
    if (ms < 1) return "< 1ms";
    return `${ms.toFixed(1)}ms`;
  }

  onMount(() => {
    loadData();
    startPolling();
  });

  onDestroy(() => {
    stopPolling();
  });

  // Debounce search
  let searchTimer: any;
  function handleSearch() {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      offset = 0;
      loadData();
    }, 300);
  }
</script>

<div class="dns-page">
  <header class="page-header">
    <div class="header-content">
      <h1>DNS Query Log</h1>
      <p class="subtitle">Live monitor of DNS request traffic and blocking.</p>
    </div>
    <div class="actions">
      <button
        class="btn-icon"
        class:active={autoRefresh}
        on:click={toggleAutoRefresh}
        title="Auto-refresh"
      >
        <Icon name={autoRefresh ? "pause" : "play_arrow"} size={20} />
      </button>
      <button class="btn-primary" on:click={loadData} title="Refresh Now">
        <Icon name="refresh" size={20} />
      </button>
    </div>
  </header>

  {#if error}
    <div class="alert error" transition:fade>
      <Icon name="error" />
      {error}
    </div>
  {/if}

  <!-- Stats Cards -->
  <div class="stats-grid">
    <div class="stat-card">
      <div class="stat-icon info"><Icon name="dns" size={24} /></div>
      <div class="stat-content">
        <span class="label">Total Queries</span>
        <span class="value">{stats?.total_queries?.toLocaleString() || 0}</span>
        <span class="subtext">Last 24 hours</span>
      </div>
    </div>
    <div class="stat-card">
      <div class="stat-icon error"><Icon name="shield" size={24} /></div>
      <div class="stat-content">
        <span class="label">Blocked</span>
        <span class="value">{stats?.total_blocked?.toLocaleString() || 0}</span>
        <span class="subtext">
          {stats?.total_queries
            ? ((stats.total_blocked / stats.total_queries) * 100).toFixed(1)
            : 0}% of traffic
        </span>
      </div>
    </div>
    <div class="stat-card">
      <div class="stat-icon success">
        <Icon name="check_circle" size={24} />
      </div>
      <div class="stat-content">
        <span class="label">Allowed</span>
        <span class="value"
          >{(
            stats?.total_queries - (stats?.total_blocked || 0)
          ).toLocaleString()}</span
        >
      </div>
    </div>
  </div>

  <!-- Filters -->
  <div class="filters-bar">
    <div class="search-box">
      <Icon name="search" size={18} />
      <input
        type="text"
        placeholder="Search domains or IPs..."
        bind:value={search}
        on:input={handleSearch}
      />
    </div>
  </div>

  <!-- Log Table -->
  <div class="card table-container">
    <table>
      <thead>
        <tr>
          <th style="width: 100px;">Time</th>
          <th style="width: 40px;"></th>
          <th>Domain</th>
          <th>Client</th>
          <th style="width: 80px;">Type</th>
          <th style="text-align: right; width: 80px;">Latency</th>
        </tr>
      </thead>
      <tbody>
        {#if history.length === 0 && !loading}
          <tr>
            <td colspan="6" class="empty-state">
              <Icon name="history_toggle_off" size={32} />
              <p>No queries found</p>
            </td>
          </tr>
        {:else}
          {#each history as entry (entry.timestamp + entry.domain)}
            <tr class="row-entry">
              <td class="timestamp">{formatTime(entry.timestamp)}</td>
              <td>
                <div
                  class="status-icon {statusColors[entry.status] ||
                    'text-gray-500'}"
                  title={entry.status}
                >
                  <Icon name={statusIcons[entry.status] || "help"} size={18} />
                </div>
              </td>
              <td class="domain-cell">
                <span class="domain">{entry.domain}</span>
                {#if entry.status === "blocked"}
                  <span class="tag error">Blocked</span>
                {/if}
              </td>
              <td class="client">{entry.client_ip}</td>
              <td class="type">{entry.query_type}</td>
              <td class="latency">{formatDuration(entry.duration_ms)}</td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</div>

<style>
  .dns-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    padding: var(--space-6);
    max-width: 1200px;
    margin: 0 auto;
    width: 100%;
  }

  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
  }

  .header-content h1 {
    font-size: var(--text-2xl);
    font-weight: 600;
    color: var(--dashboard-text);
    margin: 0;
  }

  .subtitle {
    color: var(--dashboard-text-muted);
    margin-top: var(--space-1);
  }

  .actions {
    display: flex;
    gap: var(--space-2);
  }

  /* Stats Grid */
  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: var(--space-4);
  }

  .stat-card {
    background: var(--dashboard-card);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-lg);
    padding: var(--space-4);
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }

  .stat-icon {
    width: 48px;
    height: 48px;
    border-radius: var(--radius-full);
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--dashboard-bg);
  }

  .stat-icon.info {
    color: var(--color-info);
    background: rgba(59, 130, 246, 0.1);
  }
  .stat-icon.error {
    color: var(--color-danger);
    background: rgba(239, 68, 68, 0.1);
  }
  .stat-icon.success {
    color: var(--color-success);
    background: rgba(34, 197, 94, 0.1);
  }

  .stat-content {
    display: flex;
    flex-direction: column;
  }

  .stat-content .label {
    font-size: var(--text-sm);
    color: var(--dashboard-text-muted);
  }

  .stat-content .value {
    font-size: var(--text-2xl);
    font-weight: 700;
    color: var(--dashboard-text);
  }

  .stat-content .subtext {
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
  }

  /* Filters */
  .filters-bar {
    display: flex;
    gap: var(--space-4);
  }

  .search-box {
    position: relative;
    flex: 1;
    max-width: 400px;
  }

  .search-box :global(.icon) {
    position: absolute;
    left: var(--space-3);
    top: 50%;
    transform: translateY(-50%);
    color: var(--dashboard-text-muted);
    pointer-events: none;
  }

  .search-box input {
    width: 100%;
    padding: var(--space-2) var(--space-4) var(--space-2) var(--space-10);
    background: var(--dashboard-input);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-md);
    color: var(--dashboard-text);
    font-size: var(--text-sm);
  }

  /* Table */
  .table-container {
    overflow-x: auto;
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-lg);
    background: var(--dashboard-card);
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--text-sm);
  }

  th {
    text-align: left;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--dashboard-border);
    color: var(--dashboard-text-muted);
    font-weight: 500;
  }

  td {
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--dashboard-border);
    color: var(--dashboard-text);
  }

  tr:last-child td {
    border-bottom: none;
  }

  .domain-cell {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .domain {
    font-weight: 500;
    font-family: var(--font-mono);
  }

  .tag {
    font-size: var(--text-xs);
    padding: 2px 6px;
    border-radius: var(--radius-sm);
    font-weight: 500;
  }

  .tag.error {
    background: rgba(239, 68, 68, 0.1);
    color: var(--color-danger);
  }

  .timestamp {
    color: var(--dashboard-text-muted);
    white-space: nowrap;
  }

  .latency {
    text-align: right;
    font-family: var(--font-mono);
    color: var(--dashboard-text-muted);
  }

  .type {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
  }

  .empty-state {
    text-align: center;
    padding: var(--space-12);
    color: var(--dashboard-text-muted);
  }

  .empty-state p {
    margin-top: var(--space-2);
  }

  /* Utilities */
  .text-green-500 {
    color: var(--color-success);
  }
  .text-red-500 {
    color: var(--color-danger);
  }
  .text-blue-500 {
    color: var(--color-info);
  }
  .text-gray-500 {
    color: var(--dashboard-text-muted);
  }

  .btn-icon {
    background: none;
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-md);
    padding: var(--space-2);
    color: var(--dashboard-text-muted);
    cursor: pointer;
    transition: all var(--transition-fast);
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .btn-icon:hover {
    background: var(--dashboard-input);
    color: var(--dashboard-text);
  }

  .btn-icon.active {
    background: var(--color-primary);
    color: var(--color-primaryForeground);
    border-color: var(--color-primary);
  }
</style>

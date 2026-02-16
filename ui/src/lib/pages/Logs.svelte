<script lang="ts">
  /**
   * Logs Page
   * Live streaming logs viewer + Audit trail
   */

  import { onMount, tick } from "svelte";
  import { logs, api } from "$lib/stores/app";
  import {
    Card,
    Button,
    Select,
    Badge,
    Toggle,
    Spinner,
    Icon,
  } from "$lib/components";
  import { t } from "svelte-i18n";

  let activeTab = $state<"logs" | "audit">("logs");
  let autoScroll = $state(true);
  let levelFilter = $state("all");
  let sourceFilter = $state("all");
  let logContainer = $state<HTMLDivElement | null>(null);

  // Audit state
  interface AuditEvent {
    id: number;
    timestamp: string;
    action: string;
    user: string;
    target: string;
    details: string;
    ip: string;
  }

  let auditEvents = $state<AuditEvent[]>([]);
  let auditLoading = $state(false);
  let auditError = $state<string | null>(null);

  const LOG_SOURCES = [
    { value: "all", label: $t("logs.all_sources") },
    { value: "syslog", label: $t("logs.sources.syslog") },
    { value: "nftables", label: $t("logs.sources.nftables") },
    { value: "dhcp", label: $t("logs.sources.dhcp") },
    { value: "dns", label: $t("logs.sources.dns") },
    { value: "dmesg", label: $t("logs.sources.kernel") },
    { value: "api", label: $t("logs.sources.api") },
    { value: "firewall", label: $t("logs.sources.firewall") },
  ];

  const filteredLogs = $derived(
    $logs.filter((l) => {
      if (levelFilter !== "all" && l.level !== levelFilter) return false;
      if (sourceFilter !== "all" && l.source !== sourceFilter) return false;
      return true;
    }),
  );

  function scrollToBottom() {
    if (autoScroll && logContainer) {
      logContainer.scrollTop = logContainer.scrollHeight;
    }
  }

  function clearLogs() {
    logs.set([]);
  }

  function getLevelColor(level: string) {
    switch (level) {
      case "error":
        return "destructive";
      case "warn":
        return "warning";
      case "info":
        return "default";
      case "debug":
        return "secondary";
      default:
        return "outline";
    }
  }

  function getActionColor(action: string) {
    if (action.startsWith("create") || action.startsWith("add"))
      return "default";
    if (action.startsWith("delete") || action.startsWith("remove"))
      return "destructive";
    if (action.startsWith("update") || action.startsWith("modify"))
      return "warning";
    if (action === "login") return "secondary";
    return "outline";
  }

  async function loadAuditLog() {
    auditLoading = true;
    auditError = null;
    try {
      const result = await api.getAuditLog(100, 0);
      auditEvents = result.events || [];
    } catch (e: any) {
      auditError = e.message || "Failed to load audit log";
    } finally {
      auditLoading = false;
    }
  }

  function formatTimestamp(ts: string): string {
    return new Date(ts).toLocaleString();
  }

  // Auto-scroll when logs change
  $effect(() => {
    if (filteredLogs.length > 0 && autoScroll) {
      tick().then(scrollToBottom);
    }
  });

  // Load audit on tab switch
  $effect(() => {
    if (activeTab === "audit" && auditEvents.length === 0) {
      loadAuditLog();
    }
  });
</script>

<div class="logs-page">
  <!-- Tabs -->
  <div class="tabs">
    <button
      class:active={activeTab === "logs"}
      onclick={() => (activeTab = "logs")}
    >
      <Icon name="list" size="sm" /> Streaming Logs
    </button>
    <button
      class:active={activeTab === "audit"}
      onclick={() => (activeTab = "audit")}
    >
      <Icon name="history" size="sm" /> Audit Trail
    </button>
  </div>

  {#if activeTab === "logs"}
    <div class="page-header">
      <div class="header-controls">
        <Select
          id="source-filter"
          bind:value={sourceFilter}
          options={LOG_SOURCES}
        />
        <Select
          id="level-filter"
          bind:value={levelFilter}
          options={[
            { value: "all", label: $t("logs.all_levels") },
            { value: "error", label: $t("logs.errors") },
            { value: "warn", label: $t("logs.warnings") },
            { value: "info", label: $t("logs.info") },
            { value: "debug", label: $t("logs.debug") },
          ]}
        />
        <Toggle label={$t("logs.auto_scroll")} bind:checked={autoScroll} />
        <Button variant="ghost" size="sm" onclick={clearLogs}
          >{$t("logs.clear")}</Button
        >
      </div>
    </div>

    <Card>
      <div class="log-viewer" bind:this={logContainer}>
        {#if filteredLogs.length === 0}
          <p class="empty-message">
            {$t("common.no_items", { values: { items: $t("item.log") } })}
          </p>
        {:else}
          {#each filteredLogs as log}
            <div
              class="log-entry"
              class:error={log.level === "error"}
              class:warn={log.level === "warn"}
            >
              <span class="log-time">{log.timestamp}</span>
              <Badge variant={getLevelColor(log.level)}>{log.level}</Badge>
              {#if log.source}
                <span class="log-source">[{log.source}]</span>
              {/if}
              {#if log.source === "dns" && log.extra}
                <div class="dns-content">
                  <span class="dns-type">{log.extra.TYPE}</span>
                  <span class="dns-domain">{log.extra.DOMAIN}</span>
                  <span class="dns-arrow">→</span>
                  {#if log.extra.BLOCKED}
                    <Badge variant="destructive">BLOCKED</Badge>
                    <span class="dns-blocklist">({log.extra.BLOCKLIST})</span>
                  {:else}
                    <span class="dns-rcode">{log.extra.RCODE}</span>
                  {/if}
                  <span class="dns-meta"
                    >{log.extra.CLIENT} ({log.extra.DURATION}ms)</span
                  >
                </div>
              {:else}
                <span class="log-message">{log.message}</span>
              {/if}
            </div>
          {/each}
        {/if}
      </div>
    </Card>
  {:else}
    <!-- Audit Tab -->
    <div class="page-header">
      <div class="header-controls">
        <Button
          variant="outline"
          size="sm"
          onclick={loadAuditLog}
          disabled={auditLoading}
        >
          <Icon name="refresh" size="sm" /> Refresh
        </Button>
      </div>
    </div>

    <Card>
      <div class="audit-viewer">
        {#if auditLoading}
          <div class="loading-state">
            <Spinner size="md" />
            <span>Loading audit log...</span>
          </div>
        {:else if auditError}
          <div class="error-state">{auditError}</div>
        {:else if auditEvents.length === 0}
          <div class="empty-state">
            <Icon name="check_circle" size={48} />
            <p>No audit events</p>
            <span class="text-muted">User actions will be logged here</span>
          </div>
        {:else}
          <table class="audit-table">
            <thead>
              <tr>
                <th>Time</th>
                <th>User</th>
                <th>Action</th>
                <th>Target</th>
                <th>Details</th>
              </tr>
            </thead>
            <tbody>
              {#each auditEvents as event}
                <tr>
                  <td class="timestamp">{formatTimestamp(event.timestamp)}</td>
                  <td><Badge variant="secondary">{event.user || "-"}</Badge></td
                  >
                  <td
                    ><Badge variant={getActionColor(event.action)}
                      >{event.action}</Badge
                    ></td
                  >
                  <td><code>{event.target || "-"}</code></td>
                  <td class="details">{event.details || "-"}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>
    </Card>
  {/if}
</div>

<style>
  .logs-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    height: 100%;
  }

  .tabs {
    display: flex;
    gap: var(--space-2);
    border-bottom: 1px solid var(--color-border);
    padding-bottom: var(--space-2);
  }

  .tabs button {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    background: none;
    border: none;
    padding: var(--space-2) var(--space-4);
    border-radius: var(--radius-md);
    color: var(--color-muted);
    cursor: pointer;
    font-size: var(--text-sm);
    font-weight: 500;
    transition: all var(--transition-fast);
  }

  .tabs button:hover {
    color: var(--color-foreground);
  }

  .tabs button.active {
    background: var(--color-backgroundSecondary);
    color: var(--color-foreground);
  }

  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-4);
  }

  .header-controls {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .log-viewer {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    max-height: 500px;
    overflow-y: auto;
    background-color: var(--color-backgroundSecondary);
    border-radius: var(--radius-md);
    padding: var(--space-2);
  }

  .log-entry {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
  }

  .log-entry.error {
    background-color: rgba(220, 38, 38, 0.1);
  }

  .log-entry.warn {
    background-color: rgba(234, 179, 8, 0.1);
  }

  .log-time {
    color: var(--color-muted);
    flex-shrink: 0;
  }

  .log-source {
    color: var(--color-primary);
    flex-shrink: 0;
  }

  .log-message {
    color: var(--color-foreground);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* DNS Log Styles */
  .dns-content {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: 1;
    overflow: hidden;
  }

  .dns-type {
    font-weight: 600;
    color: var(--color-primary);
    width: 40px;
  }

  .dns-domain {
    font-weight: 500;
    color: var(--color-foreground);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .dns-arrow {
    color: var(--color-muted);
  }

  .dns-rcode {
    color: var(--color-success);
    font-size: var(--text-xs);
    font-weight: 600;
    text-transform: uppercase;
  }

  .dns-meta {
    margin-left: auto;
    color: var(--color-muted);
    font-size: var(--text-xs);
  }

  .dns-blocklist {
    color: var(--color-destructive);
    font-size: var(--text-xs);
  }

  .empty-message {
    color: var(--color-muted);
    text-align: center;
    padding: var(--space-6);
    margin: 0;
  }

  /* Audit styles */
  .audit-viewer {
    overflow-x: auto;
  }

  .audit-table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--text-sm);
  }

  .audit-table th,
  .audit-table td {
    padding: var(--space-3);
    text-align: left;
    border-bottom: 1px solid var(--color-border);
  }

  .audit-table th {
    font-weight: 600;
    color: var(--color-muted);
    font-size: var(--text-xs);
    text-transform: uppercase;
  }

  .audit-table td code {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    background: var(--color-backgroundSecondary);
    padding: var(--space-1);
    border-radius: var(--radius-sm);
  }

  .audit-table .timestamp {
    white-space: nowrap;
    color: var(--color-muted);
    font-size: var(--text-xs);
  }

  .audit-table .details {
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .loading-state {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-3);
    padding: var(--space-8);
    color: var(--color-muted);
  }

  .error-state {
    color: var(--color-destructive);
    text-align: center;
    padding: var(--space-4);
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-8);
    color: var(--color-muted);
    text-align: center;
  }

  .empty-state p {
    font-size: var(--text-lg);
    font-weight: 500;
    margin: 0;
  }

  .text-muted {
    color: var(--color-muted);
  }
</style>

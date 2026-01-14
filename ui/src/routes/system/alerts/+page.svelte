<script lang="ts">
  import { onMount } from "svelte";
  import { api } from "$lib/stores/app";
  import Icon from "$lib/components/Icon.svelte";

  let history: any[] = [];
  let rules: any[] = [];
  let loading = true;
  let activeTab = "history";

  let editingRule: any = null;

  async function reload() {
    loading = true;
    try {
      history = await api.getAlertHistory(100);
      rules = await api.getAlertRules();
    } catch (e) {
      console.error(e);
    }
    loading = false;
  }

  onMount(reload);

  async function saveRule() {
    if (!editingRule) return;
    try {
      await api.updateAlertRule(editingRule);
      editingRule = null;
      await reload();
    } catch (e) {
      alert("Failed to save rule: " + e);
    }
  }

  function editRule(rule: any) {
    editingRule = { ...rule };
  }

  function createRule() {
    editingRule = {
      Name: "",
      Enabled: true,
      Condition: "",
      Severity: "warning",
      Channels: ["log"],
      Cooldown: 300000000000, // 5m in nanoseconds
    };
  }

  function formatDate(dateStr: string) {
    const d = new Date(dateStr);
    return d.toLocaleString(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    });
  }

  function getLevelIcon(level: string) {
    switch (level.toLowerCase()) {
      case "critical":
        return "warning";
      case "warning":
        return "shield";
      default:
        return "info";
    }
  }

  function getLevelColor(level: string) {
    switch (level.toLowerCase()) {
      case "critical":
        return "#ef4444"; // red-500
      case "warning":
        return "#f59e0b"; // amber-500
      default:
        return "#3b82f6"; // blue-500
    }
  }
</script>

<div class="page-container">
  <header>
    <div class="title-row">
      <h1>Alerts & Notifications</h1>
      <div class="tabs">
        <button
          class:active={activeTab === "history"}
          on:click={() => (activeTab = "history")}
        >
          History
        </button>
        <button
          class:active={activeTab === "rules"}
          on:click={() => (activeTab = "rules")}
        >
          Rules
        </button>
      </div>
    </div>
    {#if activeTab === "rules"}
      <button class="btn-primary" on:click={createRule}>
        <Icon name="plus" size={16} /> New Rule
      </button>
    {/if}
  </header>

  <main>
    {#if loading}
      <div class="loading-state">
        <div class="spinner"></div>
        <p>Loading alerts...</p>
      </div>
    {:else if activeTab === "history"}
      <div class="history-table">
        <table>
          <thead>
            <tr>
              <th>Time</th>
              <th>Severity</th>
              <th>Rule</th>
              <th>Message</th>
            </tr>
          </thead>
          <tbody>
            {#each history as event}
              <tr>
                <td class="time">{formatDate(event.Timestamp)}</td>
                <td>
                  <span
                    class="badge"
                    style="background: {getLevelColor(event.Severity)}"
                  >
                    <Icon name={getLevelIcon(event.Severity)} size={12} />
                    {event.Severity}
                  </span>
                </td>
                <td>{event.RuleName}</td>
                <td class="message">{event.Message}</td>
              </tr>
            {:else}
              <tr>
                <td colspan="4" class="empty">No alerts found in history.</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <div class="rules-grid">
        {#each rules as rule}
          <div class="rule-card" class:disabled={!rule.Enabled}>
            <div class="rule-header">
              <h3>{rule.Name}</h3>
              <button class="btn-icon" on:click={() => editRule(rule)}
                >Edit</button
              >
            </div>
            <div class="rule-body">
              <p><strong>Condition:</strong> <code>{rule.Condition}</code></p>
              <p><strong>Severity:</strong> {rule.Severity}</p>
              <p><strong>Channels:</strong> {rule.Channels.join(", ")}</p>
            </div>
          </div>
        {:else}
          <div class="empty-state">
            <Icon name="bell" size={48} />
            <p>No alert rules configured.</p>
          </div>
        {/each}
      </div>
    {/if}
  </main>

  {#if editingRule}
    <div
      class="modal-overlay"
      on:click={() => (editingRule = null)}
      role="presentation"
    >
      <div
        class="modal-content"
        on:click|stopPropagation
        role="dialog"
        aria-modal="true"
      >
        <h2>{editingRule.Name ? "Edit Rule" : "New Alert Rule"}</h2>
        <form on:submit|preventDefault={saveRule}>
          <div class="form-group">
            <label for="rule-name">Rule Name</label>
            <input
              id="rule-name"
              type="text"
              bind:value={editingRule.Name}
              placeholder="e.g. WAN High Bandwidth"
            />
          </div>
          <div class="form-group">
            <label for="rule-condition">Condition</label>
            <input
              id="rule-condition"
              type="text"
              bind:value={editingRule.Condition}
              placeholder="e.g. bandwidth.wan > 100Mbps"
            />
            <small
              >Supported: <code>anomaly.any</code>,
              <code>bandwidth.[interface] > [value]</code></small
            >
          </div>
          <div class="row">
            <div class="form-group">
              <label for="rule-severity">Severity</label>
              <select id="rule-severity" bind:value={editingRule.Severity}>
                <option value="info">Info</option>
                <option value="warning">Warning</option>
                <option value="critical">Critical</option>
              </select>
            </div>
            <div class="form-group">
              <label for="rule-enabled">Enabled</label>
              <div class="toggle">
                <input
                  id="rule-enabled"
                  type="checkbox"
                  bind:checked={editingRule.Enabled}
                />
              </div>
            </div>
          </div>
          <div class="actions">
            <button
              type="button"
              class="btn-secondary"
              on:click={() => (editingRule = null)}>Cancel</button
            >
            <button type="submit" class="btn-primary">
              <Icon name="save" size={16} /> Save Rule
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}
</div>

<style>
  .page-container {
    padding: 2rem;
    display: flex;
    flex-direction: column;
    gap: 2rem;
    max-width: 1200px;
    margin: 0 auto;
  }

  header {
    display: flex;
    justify-content: space-between;
    align-items: flex-end;
  }

  .title-row {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }

  h1 {
    margin: 0;
    font-size: 1.75rem;
    font-weight: 700;
    background: linear-gradient(135deg, #fff 0%, #94a3b8 100%);
    -webkit-background-clip: text;
    background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .tabs {
    display: flex;
    background: rgba(15, 23, 42, 0.4);
    padding: 4px;
    border-radius: 10px;
    width: fit-content;
    border: 1px solid rgba(255, 255, 255, 0.05);
  }

  .tabs button {
    background: transparent;
    border: none;
    padding: 0.6rem 1.25rem;
    border-radius: 8px;
    color: #94a3b8;
    cursor: pointer;
    font-weight: 600;
    font-size: 0.875rem;
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .tabs button.active {
    background: rgba(59, 130, 246, 0.15);
    color: #60a5fa;
    box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
  }

  main {
    min-height: 400px;
  }

  table {
    width: 100%;
    border-collapse: separate;
    border-spacing: 0;
    background: rgba(30, 41, 59, 0.3);
    border-radius: 16px;
    overflow: hidden;
    border: 1px solid rgba(255, 255, 255, 0.05);
    backdrop-filter: blur(8px);
  }

  th {
    text-align: left;
    padding: 1.25rem 1.5rem;
    background: rgba(255, 255, 255, 0.03);
    color: #64748b;
    font-weight: 600;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  }

  td {
    padding: 1.25rem 1.5rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.03);
    color: #cbd5e1;
    font-size: 0.9375rem;
  }

  tr:last-child td {
    border-bottom: none;
  }

  .time {
    color: #64748b;
    font-family: "JetBrains Mono", monospace;
    font-size: 0.8125rem;
  }

  .badge {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.35rem 0.75rem;
    border-radius: 6px;
    font-size: 0.75rem;
    font-weight: 700;
    text-transform: uppercase;
    color: white;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  }

  .message {
    line-height: 1.5;
  }

  .empty,
  .empty-state {
    text-align: center;
    padding: 5rem 2rem;
    color: #475569;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1rem;
  }

  .rules-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
    gap: 1.5rem;
  }

  .rule-card {
    background: rgba(30, 41, 59, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 20px;
    padding: 1.75rem;
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    position: relative;
    overflow: hidden;
  }

  .rule-card::before {
    content: "";
    position: absolute;
    top: 0;
    left: 0;
    width: 4px;
    height: 100%;
    background: #3b82f6;
    opacity: 0.5;
  }

  .rule-card.disabled {
    opacity: 0.5;
    filter: grayscale(0.5);
  }

  .rule-card.disabled::before {
    background: #64748b;
  }

  .rule-card:hover {
    background: rgba(30, 41, 59, 0.6);
    border-color: rgba(59, 130, 246, 0.3);
    transform: translateY(-4px);
    box-shadow: 0 12px 24px -8px rgba(0, 0, 0, 0.3);
  }

  .rule-header h3 {
    margin: 0;
    font-size: 1.125rem;
    font-weight: 700;
    color: #f1f5f9;
  }

  .rule-body {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .rule-body p {
    margin: 0;
    color: #94a3b8;
    font-size: 0.875rem;
    display: flex;
    justify-content: space-between;
  }

  code {
    background: rgba(15, 23, 42, 0.4);
    padding: 0.2rem 0.4rem;
    border-radius: 4px;
    font-family: "JetBrains Mono", monospace;
    color: #60a5fa;
    font-size: 0.8125rem;
  }

  /* Modal */
  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(2, 6, 23, 0.85);
    backdrop-filter: blur(12px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 1rem;
  }

  .modal-content {
    background: #0f172a;
    padding: 2.5rem;
    border-radius: 24px;
    width: 100%;
    max-width: 540px;
    border: 1px solid rgba(255, 255, 255, 0.1);
    box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.7);
  }

  h2 {
    margin: 0 0 2rem 0;
    font-size: 1.5rem;
    font-weight: 800;
  }

  .form-group label {
    display: block;
    margin-bottom: 0.6rem;
    color: #94a3b8;
    font-weight: 600;
    font-size: 0.8125rem;
    text-transform: uppercase;
    letter-spacing: 0.025em;
  }

  input,
  select {
    width: 100%;
    background: rgba(15, 23, 42, 0.8);
    border: 1px solid rgba(255, 255, 255, 0.1);
    padding: 0.875rem 1rem;
    border-radius: 12px;
    color: #f8fafc;
    font-size: 0.9375rem;
    transition:
      border-color 0.2s,
      box-shadow 0.2s;
  }

  input:focus,
  select:focus {
    outline: none;
    border-color: #3b82f6;
    box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.1);
  }

  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 300px;
    color: #64748b;
    gap: 1.5rem;
  }

  .spinner {
    width: 32px;
    height: 32px;
    border: 3px solid rgba(59, 130, 246, 0.1);
    border-top-color: #3b82f6;
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .btn-primary {
    background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
    box-shadow: 0 4px 12px rgba(37, 99, 235, 0.3);
    color: white;
    border: none;
    padding: 0.75rem 1.25rem;
    border-radius: 8px;
    font-weight: 600;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .btn-primary:hover {
    transform: translateY(-1px);
    box-shadow: 0 6px 16px rgba(37, 99, 235, 0.4);
  }

  .btn-secondary {
    background: rgba(255, 255, 255, 0.05);
    color: #94a3b8;
    border: none;
    padding: 0.75rem 1.25rem;
    border-radius: 8px;
    font-weight: 600;
    cursor: pointer;
  }

  .btn-icon {
    background: transparent;
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: #94a3b8;
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-size: 0.75rem;
    cursor: pointer;
  }
</style>

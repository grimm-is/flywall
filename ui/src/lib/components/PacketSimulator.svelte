<script lang="ts">
  /**
   * PacketSimulator - Debug tool for simulating packet flow through firewall
   * Shows which policy and rule would match a hypothetical packet
   */
  import { api } from "$lib/stores/app";
  import { t } from "svelte-i18n";
  import Icon from "./Icon.svelte";

  interface Props {
    open?: boolean;
    onclose?: () => void;
  }

  let { open = $bindable(false), onclose = () => {} }: Props = $props();

  // Form state
  let srcIP = $state("192.168.1.100");
  let dstIP = $state("8.8.8.8");
  let dstPort = $state("443");
  let protocol = $state("tcp");

  // Result state
  let result = $state<any>(null);
  let loading = $state(false);
  let error = $state<string | null>(null);

  async function simulate() {
    loading = true;
    result = null;
    error = null;

    try {
      result = await api.simulatePacket({
        src_ip: srcIP,
        dst_ip: dstIP,
        dst_port: dstPort ? parseInt(dstPort) : undefined,
        protocol: protocol || "tcp",
      });
    } catch (e: any) {
      error = e.message || "Simulation failed";
    } finally {
      loading = false;
    }
  }

  function handleClose() {
    open = false;
    onclose();
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") handleClose();
    if (e.key === "Enter" && !loading) simulate();
  }
</script>

{#if open}
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <div
    class="modal-backdrop"
    onclick={handleClose}
    onkeydown={handleKeydown}
    role="dialog"
    aria-modal="true"
    tabindex="-1"
  >
    <!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
    <div class="modal-content" onclick={(e) => e.stopPropagation()}>
      <div class="modal-header">
        <h2>Packet Simulator</h2>
        <button class="close-btn" onclick={handleClose} aria-label="Close">
          <Icon name="close" size="sm" />
        </button>
      </div>

      <div class="modal-body">
        <p class="description">
          Test how a packet would be handled by the firewall. Enter source and
          destination details to see which policy and rule would match.
        </p>

        <div class="form-grid">
          <div class="form-group">
            <label for="src-ip">Source IP</label>
            <input
              id="src-ip"
              type="text"
              bind:value={srcIP}
              placeholder="192.168.1.100"
            />
          </div>
          <div class="form-group">
            <label for="dst-ip">Destination IP</label>
            <input
              id="dst-ip"
              type="text"
              bind:value={dstIP}
              placeholder="8.8.8.8"
            />
          </div>
          <div class="form-group">
            <label for="dst-port">Destination Port</label>
            <input
              id="dst-port"
              type="text"
              bind:value={dstPort}
              placeholder="443"
            />
          </div>
          <div class="form-group">
            <label for="protocol">Protocol</label>
            <select id="protocol" bind:value={protocol}>
              <option value="tcp">TCP</option>
              <option value="udp">UDP</option>
              <option value="icmp">ICMP</option>
            </select>
          </div>
        </div>

        <button class="simulate-btn" onclick={simulate} disabled={loading}>
          {#if loading}
            <Icon name="sync" size="sm" />
            Simulating...
          {:else}
            <Icon name="play_arrow" size="sm" />
            Simulate Packet
          {/if}
        </button>

        {#if error}
          <div class="result-panel error">
            <Icon name="error" size="sm" />
            <span>{error}</span>
          </div>
        {/if}

        {#if result}
          <div
            class="result-panel"
            class:accept={result.action === "accept"}
            class:drop={result.action === "drop" || result.action === "reject"}
          >
            <div class="verdict-header">
              <div
                class="verdict-badge"
                class:accept={result.action === "accept"}
                class:drop={result.action === "drop" ||
                  result.action === "reject"}
              >
                <Icon
                  name={result.action === "accept" ? "check_circle" : "block"}
                  size="md"
                />
                {result.action.toUpperCase()}
              </div>
            </div>

            <p class="verdict-text">{result.verdict}</p>

            <dl class="result-details">
              <div class="detail-row">
                <dt>Source Zone</dt>
                <dd>{result.src_zone || "unknown"}</dd>
              </div>
              <div class="detail-row">
                <dt>Destination Zone</dt>
                <dd>{result.dst_zone || "unknown"}</dd>
              </div>
              <div class="detail-row">
                <dt>Matched Policy</dt>
                <dd>{result.matched_policy || "none"}</dd>
              </div>
              <div class="detail-row">
                <dt>Matched Rule</dt>
                <dd>{result.matched_rule || "default"}</dd>
              </div>
            </dl>

            {#if result.rule_path?.length}
              <div class="rule-path">
                <span class="path-label">Evaluation Path:</span>
                <div class="path-steps">
                  {#each result.rule_path as step, i}
                    <code class="path-step">{step}</code>
                    {#if i < result.rule_path.length - 1}
                      <Icon name="arrow_forward" size="sm" />
                    {/if}
                  {/each}
                </div>
              </div>
            {/if}
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.7);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: var(--space-4);
  }

  .modal-content {
    background: var(--dashboard-card);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-lg);
    width: 100%;
    max-width: 600px;
    max-height: 90vh;
    overflow-y: auto;
    box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-4);
    border-bottom: 1px solid var(--dashboard-border);
  }

  .modal-header h2 {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: 600;
    color: var(--dashboard-text);
  }

  .close-btn {
    background: none;
    border: none;
    color: var(--dashboard-text-muted);
    cursor: pointer;
    padding: var(--space-2);
    border-radius: var(--radius-sm);
    transition: all var(--transition-fast);
  }

  .close-btn:hover {
    background: var(--dashboard-input);
    color: var(--dashboard-text);
  }

  .modal-body {
    padding: var(--space-4);
  }

  .description {
    color: var(--dashboard-text-muted);
    font-size: var(--text-sm);
    margin-bottom: var(--space-4);
  }

  .form-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: var(--space-4);
    margin-bottom: var(--space-4);
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

  .simulate-btn {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: var(--space-3);
    background: var(--color-primary);
    color: var(--color-primaryForeground);
    border: none;
    border-radius: var(--radius-md);
    font-size: var(--text-sm);
    font-weight: 500;
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .simulate-btn:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .simulate-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .result-panel {
    margin-top: var(--space-4);
    padding: var(--space-4);
    background: var(--dashboard-input);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-md);
  }

  .result-panel.error {
    background: rgba(239, 68, 68, 0.1);
    border-color: var(--color-destructive);
    color: var(--color-destructive);
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .result-panel.accept {
    border-color: var(--color-success);
    background: rgba(34, 197, 94, 0.1);
  }

  .result-panel.drop {
    border-color: var(--color-destructive);
    background: rgba(239, 68, 68, 0.1);
  }

  .verdict-header {
    display: flex;
    align-items: center;
    margin-bottom: var(--space-3);
  }

  .verdict-badge {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    border-radius: var(--radius-md);
    font-size: var(--text-lg);
    font-weight: 700;
    letter-spacing: 0.1em;
  }

  .verdict-badge.accept {
    background: var(--color-success);
    color: white;
  }

  .verdict-badge.drop {
    background: var(--color-destructive);
    color: white;
  }

  .verdict-text {
    color: var(--dashboard-text);
    font-size: var(--text-sm);
    margin-bottom: var(--space-4);
  }

  .result-details {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: var(--space-2);
    margin: 0;
  }

  .detail-row {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .detail-row dt {
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
    text-transform: uppercase;
    font-weight: 500;
  }

  .detail-row dd {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--dashboard-text);
    font-family: var(--font-mono);
  }

  .rule-path {
    margin-top: var(--space-4);
    padding-top: var(--space-3);
    border-top: 1px solid var(--dashboard-border);
  }

  .path-label {
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
    text-transform: uppercase;
    font-weight: 500;
    display: block;
    margin-bottom: var(--space-2);
  }

  .path-steps {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  .path-step {
    padding: var(--space-1) var(--space-2);
    background: var(--dashboard-card);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-sm);
    font-size: var(--text-xs);
    color: var(--color-success);
  }
</style>

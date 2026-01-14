<script lang="ts">
  /**
   * RuleRow - Single rule display with inline expansion
   * Dashboard-native styling using CSS variables
   */
  import { slide } from "svelte/transition";
  import AddressPill from "./AddressPill.svelte";
  import Sparkline from "../Sparkline.svelte";
  import { t } from "svelte-i18n";
  import Icon from "../Icon.svelte";

  // Types matching backend DTOs
  interface ResolvedAddress {
    display_name: string;
    type: string;
    description?: string;
    count: number;
    is_truncated?: boolean;
    preview?: string[];
  }

  interface RuleStats {
    packets: number;
    bytes: number;
    sparkline_data: number[];
    packets_per_sec?: number;
    bytes_per_sec?: number;
    last_match_unix?: number;
  }

  // Heat level calculation (0-1 scale)
  function getHeatLevel(stats: RuleStats | undefined): number {
    if (!stats) return 0;
    const pps = stats.packets_per_sec || 0;
    if (pps <= 0) return 0;
    return Math.min(1, Math.log10(pps + 1) / 3);
  }

  // Heat color: green -> yellow -> red
  function getHeatColor(heat: number): string {
    const hue = 120 - (heat * 120);
    return `hsl(${hue}, 70%, 50%)`;
  }

  // Format packets per second
  function formatPps(pps: number | undefined): string {
    if (!pps || pps <= 0) return '';
    if (pps < 1000) return `${pps.toFixed(0)} pps`;
    if (pps < 1000000) return `${(pps / 1000).toFixed(1)}K pps`;
    return `${(pps / 1000000).toFixed(1)}M pps`;
  }

  interface RuleWithStats {
    id?: string;
    name?: string;
    description?: string;
    action: string;
    protocol?: string;
    src_ip?: string;
    dest_ip?: string;
    dest_port?: number;
    services?: string[];
    disabled?: boolean;
    group?: string;
    stats?: RuleStats;
    resolved_src?: ResolvedAddress;
    resolved_dest?: ResolvedAddress;
    nft_syntax?: string;
    policy_from?: string;
    policy_to?: string;
  }

  interface Props {
    rule: RuleWithStats;
    isSelected?: boolean;
    onToggle?: ((id: string, disabled: boolean) => void) | null;
    onEdit?: ((rule: RuleWithStats) => void) | null;
    onDelete?: ((id: string) => void) | null;
    onDuplicate?: ((rule: RuleWithStats) => void) | null;
  }

  let {
    rule,
    isSelected = false,
    onToggle = null,
    onEdit = null,
    onDelete = null,
    onDuplicate = null,
  }: Props = $props();

  let expanded = $state(false);

  function toggleRule() {
    if (onToggle && rule.id) {
      onToggle(rule.id, !rule.disabled);
    }
  }

  function formatBytes(bytes: number): string {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  }

  let portDisplay = $derived(
    rule.services?.length
      ? rule.services.join(", ")
      : rule.dest_port
        ? String(rule.dest_port)
        : "Any",
  );

  let actionStyle = $derived.by(() => {
    const action = rule.action?.toLowerCase();
    if (action === "accept") return "action-accept";
    if (action === "drop") return "action-drop";
    if (action === "reject") return "action-reject";
    return "action-default";
  });
</script>

<div
  class="rule-row-container"
  class:selected={isSelected}
  class:disabled={rule.disabled}
>
  <!-- Main Row -->
  <div
    class="rule-row-main"
    onclick={() => (expanded = !expanded)}
    onkeydown={(e) => e.key === "Enter" && (expanded = !expanded)}
    role="button"
    tabindex="0"
  >
    <!-- Toggle Switch -->
    <button
      class="toggle-switch"
      class:enabled={!rule.disabled}
      onclick={(e) => {
        e.stopPropagation();
        toggleRule();
      }}
      title={rule.disabled ? "Enable rule" : "Disable rule"}
    >
      <span class="toggle-knob"></span>
    </button>

    <!-- Rule Sentence -->
    <div class="rule-sentence">
      <span class="label">{$t("policy.from")}</span>
      <AddressPill resolved={rule.resolved_src} raw={rule.src_ip || ""} />
      <span class="label">{$t("policy.to")}</span>
      <AddressPill resolved={rule.resolved_dest} raw={rule.dest_ip || ""} />
      <span class="label">{$t("policy.on")}</span>
      <span class="port-badge">{portDisplay}</span>
    </div>

    <!-- The Pulse: Sparkline + Heat Indicator -->
    <div class="pulse-container">
      <div class="sparkline-container">
        {#if rule.stats?.sparkline_data}
          <Sparkline
            data={rule.stats.sparkline_data}
            color={rule.action === "drop"
              ? "var(--color-destructive)"
              : "var(--color-success)"}
          />
        {/if}
      </div>
      {#if rule.stats?.packets_per_sec && rule.stats.packets_per_sec > 0}
        <div class="heat-indicator" title="{formatPps(rule.stats.packets_per_sec)}">
          <div 
            class="heat-bar" 
            style="width: {getHeatLevel(rule.stats) * 100}%; background: {getHeatColor(getHeatLevel(rule.stats))};"
          ></div>
        </div>
        <span class="rate-label">{formatPps(rule.stats.packets_per_sec)}</span>
      {/if}
    </div>

    <!-- Action Badge -->
    <div class="action-badge {actionStyle}">
      {rule.action}
    </div>

    <!-- Expand Indicator -->
    <div class="expand-indicator" class:expanded>
      <Icon name="expand_more" size="sm" />
    </div>
  </div>

  <!-- Expanded Details -->
  {#if expanded}
    <div transition:slide class="expanded-panel">
      <div class="details-grid">
        <!-- Left: Details -->
        <div class="details-left">
          <!-- Description -->
          <div class="detail-group">
            <span class="detail-label">{$t("policy.description")}</span>
            <span class="detail-value"
              >{rule.description ||
                rule.name ||
                $t("policy.no_description")}</span
            >
          </div>

          <!-- Stats -->
          {#if rule.stats}
            <div class="stats-row">
              <div class="detail-group">
                <span class="detail-label">{$t("policy.packets")}</span>
                <span class="detail-value mono"
                  >{rule.stats.packets?.toLocaleString() || 0}</span
                >
              </div>
              <div class="detail-group">
                <span class="detail-label">{$t("policy.bytes")}</span>
                <span class="detail-value mono"
                  >{formatBytes(rule.stats.bytes || 0)}</span
                >
              </div>
            </div>
          {/if}

          <!-- NFT Syntax -->
          {#if rule.nft_syntax}
            <div class="detail-group">
              <span class="detail-label">{$t("policy.generated_rule")}</span>
              <code class="nft-code">{rule.nft_syntax}</code>
            </div>
          {/if}
        </div>

        <!-- Right: Actions -->
        <div class="details-right">
          <button class="action-btn primary" onclick={() => onEdit?.(rule)}>
            <Icon name="edit" size="sm" />
            Edit Rule
          </button>
          <button class="action-btn" onclick={() => onDuplicate?.(rule)}>
            <Icon name="content_copy" size="sm" />
            {$t("policy.duplicate_rule")}
          </button>
          <button
            class="action-btn destructive"
            onclick={() => rule.id && onDelete?.(rule.id)}
          >
            <Icon name="delete" size="sm" />
            {$t("policy.delete_rule")}
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .rule-row-container {
    border-bottom: 1px solid var(--dashboard-border);
    background: var(--dashboard-card);
    transition: background var(--transition-fast);
  }

  .rule-row-container:hover {
    background: var(--dashboard-input);
  }

  .rule-row-container.selected {
    background: var(--color-primary);
  }

  .rule-row-container.disabled {
    opacity: 0.5;
  }

  .rule-row-main {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    cursor: pointer;
    min-height: 48px;
  }

  /* Toggle Switch */
  .toggle-switch {
    position: relative;
    width: 32px;
    height: 16px;
    border-radius: var(--radius-full);
    background: var(--dashboard-input);
    border: none;
    cursor: pointer;
    flex-shrink: 0;
    transition: background var(--transition-fast);
  }

  .toggle-switch.enabled {
    background: var(--color-success);
  }

  .toggle-knob {
    position: absolute;
    width: 12px;
    height: 12px;
    background: white;
    border-radius: var(--radius-full);
    top: 2px;
    left: 2px;
    transition: transform var(--transition-fast);
  }

  .toggle-switch.enabled .toggle-knob {
    transform: translateX(16px);
  }

  /* Rule Sentence */
  .rule-sentence {
    flex: 1;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    font-size: var(--text-sm);
  }

  .label {
    color: var(--dashboard-text-muted);
    font-size: var(--text-xs);
    text-transform: uppercase;
    font-weight: 600;
    letter-spacing: 0.05em;
    flex-shrink: 0;
  }

  .port-badge {
    padding: var(--space-1) var(--space-2);
    background: var(--dashboard-input);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-sm);
    font-size: var(--text-xs);
    color: var(--dashboard-text);
  }

  /* The Pulse: Sparkline + Heat Indicator */
  .pulse-container {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-shrink: 0;
  }

  .sparkline-container {
    width: 64px;
    height: 24px;
    opacity: 0.6;
  }

  .rule-row-main:hover .sparkline-container {
    opacity: 1;
  }

  .heat-indicator {
    width: 40px;
    height: 6px;
    background: var(--dashboard-input);
    border-radius: 3px;
    overflow: hidden;
  }

  .heat-bar {
    height: 100%;
    border-radius: 3px;
    transition: width 0.3s ease, background 0.3s ease;
  }

  .rate-label {
    font-size: var(--text-xs);
    font-family: var(--font-mono);
    color: var(--dashboard-text-muted);
    min-width: 60px;
    text-align: right;
  }

  .rule-row-main:hover .rate-label {
    color: var(--dashboard-text);
  }

  /* Action Badge */
  .action-badge {
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
    font-size: var(--text-xs);
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    flex-shrink: 0;
  }

  .action-accept {
    background: rgba(34, 197, 94, 0.15);
    color: var(--color-success);
    border: 1px solid rgba(34, 197, 94, 0.3);
  }

  .action-drop {
    background: rgba(239, 68, 68, 0.15);
    color: var(--color-destructive);
    border: 1px solid rgba(239, 68, 68, 0.3);
  }

  .action-reject {
    background: rgba(245, 158, 11, 0.15);
    color: var(--color-warning);
    border: 1px solid rgba(245, 158, 11, 0.3);
  }

  .action-default {
    background: var(--dashboard-input);
    color: var(--dashboard-text-muted);
    border: 1px solid var(--dashboard-border);
  }

  /* Expand Indicator */
  .expand-indicator {
    color: var(--dashboard-text-muted);
    transition: transform var(--transition-fast);
  }

  .expand-indicator.expanded {
    transform: rotate(180deg);
  }

  /* Expanded Panel */
  .expanded-panel {
    background: var(--dashboard-canvas);
    border-top: 1px solid var(--dashboard-border);
    padding: var(--space-4);
  }

  .details-grid {
    display: grid;
    grid-template-columns: 2fr 1fr;
    gap: var(--space-6);
  }

  .details-left {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .details-right {
    border-left: 1px solid var(--dashboard-border);
    padding-left: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .detail-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .detail-label {
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
    text-transform: uppercase;
    font-weight: 500;
  }

  .detail-value {
    font-size: var(--text-sm);
    color: var(--dashboard-text);
  }

  .detail-value.mono {
    font-family: var(--font-mono);
  }

  .stats-row {
    display: flex;
    gap: var(--space-6);
  }

  .nft-code {
    display: block;
    padding: var(--space-2);
    background: var(--dashboard-card);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-sm);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--color-success);
    overflow-x: auto;
  }

  /* Action Buttons */
  .action-btn {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: none;
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-md);
    font-size: var(--text-sm);
    color: var(--dashboard-text-muted);
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .action-btn:hover {
    background: var(--dashboard-input);
    color: var(--dashboard-text);
    border-color: var(--dashboard-text-muted);
  }

  .action-btn.destructive {
    color: var(--color-destructive);
  }

  .action-btn.destructive:hover {
    background: rgba(239, 68, 68, 0.1);
    border-color: var(--color-destructive);
  }

  .action-btn.primary {
    background: var(--color-primary);
    color: var(--color-primaryForeground);
    border-color: var(--color-primary);
  }

  .action-btn.primary:hover {
    filter: brightness(1.1);
  }
</style>

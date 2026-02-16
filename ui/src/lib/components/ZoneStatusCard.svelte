<script lang="ts">
    import Icon from "./Icon.svelte";
    import { goto } from "$app/navigation";
    import { api, alertStore } from "$lib/stores/app";
    import type { AggregatedZone } from "$lib/stores/zones";

    let { zone }: { zone: AggregatedZone } = $props();

    // Derived values from aggregated zone
    let isWan = $derived(zone.isWan);
    let deviceCount = $derived(zone.deviceCount);
    let dhcpEnabled = $derived(zone.dhcpEnabled);
    let dnsEnabled = $derived(zone.dnsEnabled);
    let learningMode = $derived(zone.learningMode);
    let stealthStatus = $derived(zone.stealthStatus);
    let openPorts = $derived(zone.openPorts);
    let pingEnabled = $derived(zone.pingEnabled);
    let ruleCount = $derived(zone.ruleCount);
    let anomalousCount = $derived(
        zone.devices ? zone.devices.filter((d) => d.isAnomalous).length : 0,
    );

    function navigateToPolicy() {
        goto(`/policy?zone=${zone.name}`);
    }

    function navigateToLogs() {
        goto(`/discovery?filter=zone:${zone.name}`);
    }

    async function scanSelf() {
        try {
            // Use the first IP or interface name
            const target = zone.ip || zone.interface || zone.name;
            await api.startScanNetwork(target, 30);
            alertStore.success(`Scanning ${zone.name}... Check Scanner module for results.`);
            goto("/scanner");
        } catch (e: any) {
            alertStore.error(`Failed to start scan: ${e.message}`);
        }
    }

    function startCapture() {
        // Navigate to Discovery with capture parameter for this interface
        // This enables deep-linking to start a capture on this interface
        const iface = zone.interface || zone.name;
        goto(`/discovery?capture=${encodeURIComponent(iface)}`);
    }

    // Status badge based on zone status
    let statusLabel = $derived(
        zone.status === "connected"
            ? "Connected"
            : zone.status === "down"
              ? "Down"
              : "Active",
    );
    let statusClass = $derived(zone.status);
</script>

<article class="zone-card" class:wan={isWan}>
    <header class="zone-header">
        <div class="zone-title">
            <h3>{zone.name}</h3>
            {#if zone.interface}
                <span class="zone-interface">({zone.interface})</span>
            {/if}
            <span class="zone-status {statusClass}">{statusLabel}</span>
            {#if !isWan}
                {#if learningMode === "lockdown"}
                    <span class="learning-badge" title="Lockdown mode">🔒</span>
                {:else if learningMode === "tofu"}
                    <span class="learning-badge" title="Trust on first use"
                        >🤝</span
                    >
                {:else if learningMode === "approval"}
                    <span class="learning-badge" title="Approval required"
                        >⏸️</span
                    >
                {/if}
            {/if}
        </div>
        <button
            class="btn-capture"
            aria-label="Start capture"
            onclick={startCapture}
            title="Capture traffic on {zone.interface || zone.name}"
        >
            <Icon name="radio_button_checked" size={16} />
        </button>
    </header>

    <div class="zone-grid">
        <!-- Identity (always shown) -->
        <div class="zone-section">
            <div class="section-label">Identity</div>
            <div class="section-content">
                {#if zone.ips && zone.ips.length > 0}
                    {#each zone.ips as ip}
                        <span class="ip-display font-mono">{ip}</span>
                    {/each}
                {:else}
                    <span class="ip-display font-mono"
                        >{zone.ip || "DHCP-Client"}</span
                    >
                {/if}
            </div>
        </div>

        {#if isWan}
            <!-- WAN: Surface (Attack Surface) -->
            <div class="zone-section">
                <div class="section-label">Surface</div>
                <div class="section-content">
                    <span class="stealth-status {stealthStatus}">
                        {#if stealthStatus === "dark"}
                            🌑 DARK
                        {:else if stealthStatus === "beacon"}
                            📡 BEACON
                        {:else}
                            🔓 EXPOSED
                        {/if}
                    </span>
                    <span class="meta">Ping: {pingEnabled ? "YES" : "NO"}</span>
                    {#if openPorts.length > 0}
                        <div class="open-ports">
                            {#each openPorts.slice(0, 3) as port}
                                <span class="port-item font-mono"
                                    >{port.port} ({port.service})</span
                                >
                            {/each}
                            {#if openPorts.length > 3}
                                <span class="meta"
                                    >+{openPorts.length - 3} more</span
                                >
                            {/if}
                        </div>
                    {/if}
                </div>
            </div>

            <!-- WAN: Health -->
            <div class="zone-section">
                <div class="section-label">Health</div>
                <div class="section-content">
                    <span class="health-status" class:good={zone.status === 'connected'}>
                        ● {zone.status === 'connected' ? 'Up' : 'Down'}
                    </span>
                    <span class="meta">Latency: {zone.latencyMs !== undefined ? `${zone.latencyMs}ms` : '---'}</span>
                </div>
            </div>
        {:else}
            <!-- LAN: Members -->
            <div class="zone-section">
                <div class="section-label">Members</div>
                <div class="section-content">
                    <span class="device-count">{deviceCount} devices</span>
                    {#if anomalousCount > 0}
                        <span
                            class="anomaly-alert"
                            title="{anomalousCount} devices showing anomalous traffic pattern"
                        >
                            ⚠️ {anomalousCount} anomalous
                        </span>
                    {/if}
                </div>
            </div>

            <!-- LAN: Services -->
            <div class="zone-section">
                <div class="section-label">Services</div>
                <div class="section-content services">
                    <span class="service" class:active={dhcpEnabled}>
                        DHCP: {dhcpEnabled ? "ON" : "OFF"}
                    </span>
                    <span class="service" class:active={dnsEnabled}>
                        DNS: {dnsEnabled ? "Local" : "OFF"}
                    </span>
                </div>
            </div>
        {/if}
    </div>

    <!-- Footer: Actions or Flow -->
    <footer class="zone-footer">
        {#if isWan}
            <div class="wan-actions">
                <button class="btn-small" onclick={scanSelf}>
                    <Icon name="radar" size={14} />
                    Scan Self
                </button>
                <button class="btn-small" onclick={navigateToLogs}>
                    <Icon name="description" size={14} />
                    Firewall Log
                </button>
            </div>
        {:else}
            <button class="flow-pipeline" onclick={navigateToPolicy}>
                <span class="flow-step">IN</span>
                <span class="flow-arrow">→</span>
                <span class="flow-step rules">0 Rules</span>
                <span class="flow-arrow">→</span>
                <span class="flow-step">WAN</span>
            </button>
        {/if}
    </footer>
</article>

<style>
    .zone-card {
        background: var(--dashboard-card);
        border: 1px solid var(--dashboard-border);
        border-radius: var(--radius-lg);
        overflow: hidden;
    }

    .zone-card.wan {
        border-color: var(--color-primary);
        border-width: 2px;
    }

    .zone-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: var(--space-3) var(--space-4);
        border-bottom: 1px solid var(--dashboard-border);
    }

    .zone-title {
        display: flex;
        align-items: center;
        gap: var(--space-2);
    }

    .zone-title h3 {
        font-size: var(--text-base);
        font-weight: 600;
        color: var(--dashboard-text);
        text-transform: uppercase;
    }

    .zone-interface {
        font-size: var(--text-sm);
        color: var(--dashboard-text-muted);
    }

    .zone-status {
        font-size: var(--text-xs);
        padding: var(--space-1) var(--space-2);
        border-radius: var(--radius-full);
    }

    .zone-status.active {
        background: var(--color-success);
        color: var(--color-successForeground);
    }

    .zone-status.connected {
        background: var(--color-primary);
        color: var(--color-primaryForeground);
    }

    .learning-badge {
        font-size: var(--text-sm);
    }

    .btn-capture {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 28px;
        height: 28px;
        background: none;
        border: 1px solid var(--color-destructive);
        border-radius: var(--radius-full);
        color: var(--color-destructive);
        cursor: pointer;
        transition: all var(--transition-fast);
    }

    .btn-capture:hover {
        background: var(--color-destructive);
        color: var(--color-destructiveForeground);
    }

    .zone-grid {
        display: grid;
        grid-template-columns: repeat(3, 1fr);
        border-bottom: 1px solid var(--dashboard-border);
    }

    .zone-section {
        padding: var(--space-3);
        border-right: 1px solid var(--dashboard-border);
    }

    .zone-section:last-child {
        border-right: none;
    }

    .section-label {
        font-size: var(--text-xs);
        font-weight: 600;
        color: var(--dashboard-text-muted);
        text-transform: uppercase;
        margin-bottom: var(--space-2);
    }

    .section-content {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
    }

    .ip-display {
        font-size: var(--text-sm);
        color: var(--dashboard-text);
    }

    .meta {
        font-size: var(--text-xs);
        color: var(--dashboard-text-muted);
    }

    .device-count {
        font-size: var(--text-sm);
        color: var(--dashboard-text);
    }

    .anomaly-alert {
        font-size: var(--text-xs);
        color: var(--color-warning);
        font-weight: 600;
        display: flex;
        align-items: center;
        gap: var(--space-1);
        animation: pulse 2s infinite;
    }

    @keyframes pulse {
        0% {
            opacity: 1;
        }
        50% {
            opacity: 0.7;
        }
        100% {
            opacity: 1;
        }
    }

    .services {
        gap: var(--space-1);
    }

    .service {
        font-size: var(--text-xs);
        color: var(--dashboard-text-muted);
    }

    .service.active {
        color: var(--color-success);
    }

    /* WAN-specific styles */
    .stealth-status {
        font-size: var(--text-sm);
        font-weight: 500;
    }

    .stealth-status.dark {
        color: var(--color-success);
    }

    .stealth-status.beacon {
        color: var(--color-warning);
    }

    .stealth-status.exposed {
        color: var(--color-destructive);
    }

    .open-ports {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
        margin-top: var(--space-1);
    }

    .port-item {
        font-size: var(--text-xs);
        color: var(--color-destructive);
    }

    .health-status {
        font-size: var(--text-sm);
    }

    .health-status.good {
        color: var(--color-success);
    }

    .zone-footer {
        padding: var(--space-2) var(--space-3);
    }

    .wan-actions {
        display: flex;
        gap: var(--space-2);
    }

    .btn-small {
        display: flex;
        align-items: center;
        gap: var(--space-1);
        padding: var(--space-2) var(--space-3);
        background: var(--dashboard-input);
        border: none;
        border-radius: var(--radius-md);
        color: var(--dashboard-text);
        font-size: var(--text-xs);
        cursor: pointer;
        transition: all var(--transition-fast);
    }

    .btn-small:hover {
        background: var(--dashboard-border);
    }

    .flow-pipeline {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        width: 100%;
        padding: var(--space-2);
        background: var(--dashboard-input);
        border: none;
        border-radius: var(--radius-md);
        cursor: pointer;
        transition: all var(--transition-fast);
    }

    .flow-pipeline:hover {
        background: var(--dashboard-border);
    }

    .flow-step {
        font-size: var(--text-xs);
        font-weight: 500;
        color: var(--dashboard-text-muted);
        padding: var(--space-1) var(--space-2);
        background: var(--dashboard-card);
        border-radius: var(--radius-sm);
    }

    .flow-step.rules {
        color: var(--color-primary);
    }

    .flow-arrow {
        color: var(--dashboard-text-muted);
        font-size: var(--text-xs);
    }

    /* Mobile stacking */
    @media (max-width: 640px) {
        .zone-grid {
            grid-template-columns: 1fr;
        }

        .zone-section {
            border-right: none;
            border-bottom: 1px solid var(--dashboard-border);
        }

        .zone-section:last-child {
            border-bottom: none;
        }

        .wan-actions {
            flex-direction: column;
        }
    }
</style>

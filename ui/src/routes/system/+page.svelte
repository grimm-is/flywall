<script lang="ts">
    import Icon from "$lib/components/Icon.svelte";
    import { config, status, api } from "$lib/stores/app";

    // System data
    let systemStatus = $derived($status || {});
    let uplinks = $derived($config?.uplinks || []);
    let users = $state([]);

    $effect(() => {
        api.getUsers().then((u) => {
            if (Array.isArray(u)) users = u;
        });
    });

    async function reboot() {
        if (confirm("Reboot the router? Active connections will be dropped.")) {
            try {
                await api.reboot();
                alert("Reboot command sent successfully.");
            } catch (e) {
                alert("Failed to reboot: " + e.message);
            }
        }
    }

    async function enterSafeMode() {
        if (confirm("Enter Safe Mode? This will disable all firewall rules.")) {
            try {
                await api.enterSafeMode();
                alert("Entering safe mode...");
            } catch (e) {
                alert("Failed to enter safe mode: " + e.message);
            }
        }
    }
</script>

<div class="system-page">
    <header class="page-header">
        <h1>System</h1>
    </header>

    <!-- Infrastructure Section -->
    <section class="grid-section">
        <h2 class="section-title">Infrastructure</h2>
        <div class="card-grid">
            <!-- Uplinks -->
            <article class="system-card">
                <header class="card-header">
                    <Icon name="public" size={20} />
                    <span>Uplinks (Multi-WAN)</span>
                </header>
                <div class="card-content">
                    {#each uplinks as uplink}
                        <div class="uplink-row">
                            <span class="uplink-name">{uplink.name}</span>
                            <span
                                class="uplink-status"
                                class:primary={uplink.primary}
                            >
                                {uplink.primary ? "Primary" : "Standby"}
                            </span>
                        </div>
                    {:else}
                        <p class="empty-text">Single WAN configured</p>
                    {/each}
                </div>
            </article>

            <!-- Users -->
            <article class="system-card">
                <header class="card-header">
                    <Icon name="people" size={20} />
                    <span>Users & Access</span>
                </header>
                <div class="card-content">
                    <p class="stat-line">
                        {users.length} User{users.length === 1 ? "" : "s"} configured
                    </p>
                    <a href="/system/users" class="btn-link">Manage Users →</a>
                </div>
            </article>

            <!-- Raw Config -->
            <article class="system-card">
                <header class="card-header">
                    <Icon name="code" size={20} />
                    <span>Raw Config</span>
                </header>
                <div class="card-content">
                    <p class="stat-line">Edit config.hcl directly</p>
                    <a href="/system/hcl" class="btn-link">Open Editor →</a>
                </div>
            </article>
        </div>
    </section>

    <!-- Maintenance Section -->
    <section class="grid-section">
        <h2 class="section-title">Maintenance</h2>
        <div class="card-grid">
            <!-- Backup/Restore -->
            <article class="system-card">
                <header class="card-header">
                    <Icon name="backup" size={20} />
                    <span>Disaster Recovery</span>
                </header>
                <div class="card-content">
                    <p class="stat-line">Manage system backups</p>
                    <a href="/system/backups" class="btn-link"
                        >Manage Backups →</a
                    >
                </div>
            </article>

            <!-- Audit Log -->
            <article class="system-card">
                <header class="card-header">
                    <Icon name="history" size={20} />
                    <span>Audit Log</span>
                </header>
                <div class="card-content">
                    <p class="stat-line">View activity history</p>
                    <a href="/system/audit" class="btn-link">View Logs →</a>
                </div>
            </article>

            <!-- Power -->
            <article class="system-card">
                <header class="card-header">
                    <Icon name="power_settings_new" size={20} />
                    <span>Power</span>
                </header>
                <div class="card-content">
                    Uptime: {systemStatus.uptime || "Unknown"}

                    <p class="stat-line">
                        Version: {systemStatus.version || "Unknown"}
                    </p>
                    <div class="button-group">
                        <button class="btn-warning" onclick={reboot}>
                            <Icon name="restart_alt" size={16} />
                            Reboot
                        </button>
                        <button class="btn-danger" onclick={enterSafeMode}>
                            <Icon name="warning" size={16} />
                            Safe Mode
                        </button>
                    </div>
                </div>
            </article>

            <!-- Hardware -->
            <article class="system-card">
                <header class="card-header">
                    <Icon name="memory" size={20} />
                    <span>Hardware</span>
                </header>
                <div class="card-content">
                    <div class="hw-stats">
                        <div class="hw-stat">
                            <span class="hw-label">CPU</span>
                            <span class="hw-value"
                                >{systemStatus.cpu_usage || 0}%</span
                            >
                        </div>
                        <div class="hw-stat">
                            <span class="hw-label">Memory</span>
                            <span class="hw-value"
                                >{systemStatus.memory_used_mb || 0} MB</span
                            >
                        </div>
                        <div class="hw-stat">
                            <span class="hw-label">Temp</span>
                            <span class="hw-value"
                                >{systemStatus.temperature || "-"}°C</span
                            >
                        </div>
                    </div>
                </div>
            </article>
        </div>
    </section>
</div>

<style>
    .system-page {
        display: flex;
        flex-direction: column;
        gap: var(--space-6);
    }

    .page-header h1 {
        font-size: var(--text-2xl);
        font-weight: 600;
        color: var(--dashboard-text);
    }

    .grid-section {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
    }

    .section-title {
        font-size: var(--text-lg);
        font-weight: 600;
        color: var(--dashboard-text);
    }

    .card-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
        gap: var(--space-4);
    }

    .system-card {
        background: var(--dashboard-card);
        border: 1px solid var(--dashboard-border);
        border-radius: var(--radius-lg);
        overflow: hidden;
    }

    .card-header {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-3) var(--space-4);
        background: var(--dashboard-input);
        border-bottom: 1px solid var(--dashboard-border);
        font-weight: 600;
        font-size: var(--text-sm);
        color: var(--dashboard-text);
    }

    .card-content {
        padding: var(--space-4);
    }

    .uplink-row {
        display: flex;
        justify-content: space-between;
        padding: var(--space-2) 0;
        border-bottom: 1px solid var(--dashboard-border);
    }

    .uplink-row:last-child {
        border-bottom: none;
    }

    .uplink-name {
        font-weight: 500;
        color: var(--dashboard-text);
    }

    .uplink-status {
        font-size: var(--text-xs);
        padding: var(--space-1) var(--space-2);
        background: var(--dashboard-input);
        border-radius: var(--radius-full);
        color: var(--dashboard-text-muted);
    }

    .uplink-status.primary {
        background: var(--color-success);
        color: var(--color-successForeground);
    }

    .stat-line {
        font-size: var(--text-sm);
        color: var(--dashboard-text-muted);
        margin-bottom: var(--space-3);
    }

    .btn-link {
        background: none;
        border: none;
        color: var(--color-primary);
        font-size: var(--text-sm);
        cursor: pointer;
        text-decoration: none;
    }

    .button-group {
        display: flex;
        gap: var(--space-2);
        flex-wrap: wrap;
    }

    .btn-secondary {
        display: flex;
        align-items: center;
        gap: var(--space-1);
        padding: var(--space-2) var(--space-3);
        background: var(--dashboard-input);
        border: 1px solid var(--dashboard-border);
        border-radius: var(--radius-md);
        color: var(--dashboard-text);
        font-size: var(--text-sm);
        cursor: pointer;
    }

    .btn-warning {
        display: flex;
        align-items: center;
        gap: var(--space-1);
        padding: var(--space-2) var(--space-3);
        background: var(--color-warning);
        border: none;
        border-radius: var(--radius-md);
        color: var(--color-warningForeground);
        font-size: var(--text-sm);
        cursor: pointer;
    }

    .btn-danger {
        display: flex;
        align-items: center;
        gap: var(--space-1);
        padding: var(--space-2) var(--space-3);
        background: var(--color-destructive);
        border: none;
        border-radius: var(--radius-md);
        color: var(--color-destructiveForeground);
        font-size: var(--text-sm);
        cursor: pointer;
    }

    .empty-text {
        color: var(--dashboard-text-muted);
        font-size: var(--text-sm);
    }

    .hw-stats {
        display: flex;
        justify-content: space-around;
    }

    .hw-stat {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: var(--space-1);
    }

    .hw-label {
        font-size: var(--text-xs);
        color: var(--dashboard-text-muted);
    }

    .hw-value {
        font-size: var(--text-lg);
        font-weight: 600;
        color: var(--dashboard-text);
    }
</style>

<script lang="ts">
    import { t } from "svelte-i18n";
    import { page } from "$app/stores";
    import Icon from "./Icon.svelte";
    import SearchPalette from "./SearchPalette.svelte";
    import PendingChangesBanner from "./PendingChangesBanner.svelte";
    import { api } from "$lib/stores/app";
    import { sidebarExpanded } from "$lib/stores/ui";

    let { children } = $props();

    // Navigation rail items
    const navItems = [
        { id: "topology", icon: "hub", label: $t("nav.topology"), path: "/" },
        {
            id: "network",
            icon: "lan",
            label: $t("nav.network"),
            path: "/network",
        },
        {
            id: "policy",
            icon: "shield",
            label: $t("nav.policy"),
            path: "/policy",
        },
        {
            id: "discovery",
            icon: "activity",
            label: $t("nav.discovery"),
            path: "/discovery",
        },
        {
            id: "tunnels",
            icon: "vpn_key",
            label: $t("nav.vpn"),
            path: "/tunnels",
        },
        {
            id: "alerts",
            icon: "notifications",
            label: $t("nav.alerts"),
            path: "/alerts",
        },
        {
            id: "system",
            icon: "settings",
            label: $t("nav.system"),
            path: "/system",
        },
    ];

    // Track active rail from current path
    let activeRail = $derived(() => {
        const path = $page.url.pathname;
        if (path.startsWith("/policy")) return "policy";
        if (path.startsWith("/network")) return "network";
        if (path.startsWith("/discovery")) return "discovery";
        if (path.startsWith("/tunnels")) return "tunnels";
        if (path.startsWith("/system")) return "system";
        return "topology";
    });

    // Command palette
    let showPalette = $state(false);

    function handleKeydown(e: KeyboardEvent) {
        if ((e.metaKey || e.ctrlKey) && e.key === "k") {
            e.preventDefault();
            showPalette = true;
        }
    }

    function toggleSidebar() {
        $sidebarExpanded = !$sidebarExpanded;
    }
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="dashboard-shell">
    <!-- Navigation Rail -->
    <nav
        class="dashboard-rail"
        class:expanded={$sidebarExpanded}
        aria-label="Main navigation"
    >
        <div class="rail-header">
            <button
                class="rail-toggle"
                onclick={toggleSidebar}
                aria-label={$sidebarExpanded
                    ? "Collapse sidebar"
                    : "Expand sidebar"}
            >
                <Icon name="menu" size={24} />
            </button>
            <div class="rail-logo">
                <Icon name="router" size={24} />
                <span class="logo-text">Flywall</span>
            </div>
        </div>

        <div class="rail-items">
            {#each navItems as item}
                <a
                    href={item.path}
                    class="rail-item"
                    class:active={activeRail() === item.id}
                    aria-current={activeRail() === item.id ? "page" : undefined}
                >
                    <Icon name={item.icon} size={20} />
                    <span class="rail-label">{item.label}</span>
                </a>
            {/each}
        </div>

        <div class="rail-footer">
            <a
                href="https://docs.flywall.dev"
                target="_blank"
                rel="noopener noreferrer"
                class="rail-item"
                aria-label="Documentation"
            >
                <Icon name="help" size={20} />
                <span class="rail-label">Docs</span>
            </a>
            <button
                class="rail-item"
                onclick={() => (showPalette = true)}
                aria-label="Open command palette"
            >
                <Icon name="search" size={20} />
                <span class="rail-label">Search</span>
            </button>
            <button
                class="rail-item"
                onclick={() => api.logout()}
                aria-label="Log Out"
            >
                <Icon name="logout" size={20} />
                <span class="rail-label">Log Out</span>
            </button>
        </div>
    </nav>

    <!-- Main Content Area -->
    <main class="dashboard-main" class:expanded={$sidebarExpanded}>
        {@render children()}
    </main>
</div>

{#if showPalette}
    <SearchPalette onclose={() => (showPalette = false)} />
{/if}

<style>
    .dashboard-shell {
        display: flex;
        min-height: 100vh;
        background: var(--dashboard-canvas);
    }

    /* Navigation Rail */
    .dashboard-rail {
        width: 64px;
        height: 100vh; /* Changed from min-height to constrain to viewport */
        background: var(--dashboard-card);
        border-right: 1px solid var(--dashboard-border);
        display: flex;
        flex-direction: column;
        transition: width var(--transition-fast);
        overflow: hidden;
        position: fixed;
        left: 0;
        top: 0;
        z-index: 50;
    }

    .dashboard-rail.expanded {
        width: 240px;
    }

    .rail-header {
        height: 64px;
        display: flex;
        align-items: center;
        padding: 0 var(--space-4); /* Center icon in collapsed mode */
        border-bottom: 1px solid var(--dashboard-border);
        white-space: nowrap;
        overflow: hidden;
    }

    .rail-toggle {
        background: none;
        border: none;
        color: var(--dashboard-text-muted);
        cursor: pointer;
        padding: var(--space-1);
        border-radius: var(--radius-sm);
        display: flex;
        align-items: center;
        justify-content: center;
        margin-right: var(--space-4);
        flex-shrink: 0; /* Prevent shrinking */
    }

    .rail-toggle:hover {
        color: var(--dashboard-text);
        background: var(--dashboard-input);
    }

    .rail-logo {
        display: flex;
        align-items: center;
        gap: var(--space-3);
        color: var(--color-primary);
        font-weight: 600;
        font-size: var(--text-lg);
        opacity: 0;
        transition: opacity var(--transition-fast);
    }

    .dashboard-rail.expanded .rail-logo {
        opacity: 1;
    }

    .rail-items {
        flex: 1;
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
        padding: var(--space-2);
        overflow-y: auto;
        overflow-x: hidden;
    }

    .rail-item {
        display: flex;
        align-items: center;
        gap: var(--space-3);
        padding: var(--space-3);
        border-radius: var(--radius-md);
        color: var(--dashboard-text-muted);
        text-decoration: none;
        transition: all var(--transition-fast);
        cursor: pointer;
        background: none;
        border: none;
        width: 100%;
        font-size: var(--text-sm);
        white-space: nowrap;
    }

    /* Collapsed state adjustments */
    .dashboard-rail:not(.expanded) .rail-item {
        flex-direction: column;
        gap: 4px;
        padding: var(--space-2) 0;
        justify-content: center;
    }

    .rail-item:hover {
        background: var(--dashboard-input);
        color: var(--dashboard-text);
    }

    .rail-item.active {
        background: var(--color-primary);
        color: var(--color-primaryForeground);
    }

    .rail-label {
        /* Default opacity is 1, but we adjust size/visibility based on state */
        white-space: nowrap;
        opacity: 1; /* Always visible but styled differently */
        transition: opacity var(--transition-fast);
        font-weight: 500;
    }

    .dashboard-rail:not(.expanded) .rail-label {
        font-size: 10px;
        line-height: 1;
        opacity: 0.8;
    }

    .dashboard-rail.expanded .rail-label {
        font-size: var(--text-sm);
        opacity: 1;
    }

    .rail-footer {
        padding: var(--space-2);
        border-top: 1px solid var(--dashboard-border);
    }

    /* Main Content */
    .dashboard-main {
        flex: 1;
        margin-left: 64px;
        padding: var(--space-6);
        min-height: 100vh;
        transition: margin-left var(--transition-fast);
    }

    .dashboard-main.expanded {
        margin-left: 240px;
    }

    /* Mobile: Bottom nav */
    @media (max-width: 768px) {
        .dashboard-rail {
            width: 100%;
            height: 64px;
            min-height: auto;
            flex-direction: row;
            position: fixed;
            bottom: 0;
            top: auto;
            left: 0;
            right: 0;
            border-right: none;
            border-top: 1px solid var(--dashboard-border);
            z-index: 100;
        }

        .dashboard-rail.expanded {
            width: 100%;
        }

        .rail-header {
            display: none;
        }

        .rail-items {
            flex-direction: row;
            justify-content: space-around;
            padding: var(--space-1);
            overflow-y: hidden;
        }

        .rail-item {
            flex-direction: column;
            padding: var(--space-2);
            gap: var(--space-1);
            justify-content: center;
        }

        .rail-label {
            display: none;
        }

        .rail-footer {
            display: none;
        }

        .dashboard-main {
            margin-left: 0 !important;
            padding: var(--space-4);
            padding-bottom: 80px; /* Space for bottom nav */
        }
    }
</style>

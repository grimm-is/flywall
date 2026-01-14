<script lang="ts">
    import { t } from "svelte-i18n";
    import { page } from "$app/stores";
    import Icon from "./Icon.svelte";
    import SearchPalette from "./SearchPalette.svelte";

    let { children } = $props();

    // Navigation rail items
    const navItems = [
        { id: "topology", icon: "hub", label: "Topology", path: "/" },
        {
            id: "network",
            icon: "lan",
            label: "Network",
            path: "/network",
        },
        {
            id: "policy",
            icon: "shield",
            label: "Policy",
            path: "/policy",
        },
        {
            id: "observatory",
            icon: "monitoring",
            label: "Observatory",
            path: "/observatory",
        },
        {
            id: "tunnels",
            icon: "vpn_key",
            label: "Tunnels",
            path: "/tunnels",
        },
        {
            id: "system",
            icon: "settings",
            label: "System",
            path: "/system",
        },
    ];

    // Track active rail from current path
    let activeRail = $derived(() => {
        const path = $page.url.pathname;
        if (path.startsWith("/policy")) return "policy";
        if (path.startsWith("/network")) return "network";
        if (path.startsWith("/observatory")) return "observatory";
        if (path.startsWith("/tunnels")) return "tunnels";
        if (path.startsWith("/system")) return "system";
        return "topology";
    });

    // Rail hover expansion
    let railExpanded = $state(false);

    // Command palette
    let showPalette = $state(false);

    function handleKeydown(e: KeyboardEvent) {
        if ((e.metaKey || e.ctrlKey) && e.key === "k") {
            e.preventDefault();
            showPalette = true;
        }
    }
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="dashboard-shell">
    <!-- Navigation Rail -->
    <nav
        class="dashboard-rail"
        class:expanded={railExpanded}
        onmouseenter={() => (railExpanded = true)}
        onmouseleave={() => (railExpanded = false)}
        aria-label="Main navigation"
    >
        <div class="rail-logo">
            <Icon name="router" size={24} />
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
            <button
                class="rail-item"
                onclick={() => (showPalette = true)}
                aria-label="Open command palette"
            >
                <Icon name="search" size={20} />
                <span class="rail-label">Search</span>
            </button>
        </div>
    </nav>

    <!-- Main Content Area -->
    <main class="dashboard-main">
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
        width: 56px;
        min-height: 100vh;
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
        width: 180px;
    }

    .rail-logo {
        padding: var(--space-4);
        display: flex;
        align-items: center;
        justify-content: center;
        border-bottom: 1px solid var(--dashboard-border);
        color: var(--color-primary);
    }

    .rail-items {
        flex: 1;
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
        padding: var(--space-2);
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
        white-space: nowrap;
        opacity: 0;
        transition: opacity var(--transition-fast);
    }

    .dashboard-rail.expanded .rail-label {
        opacity: 1;
    }

    .rail-footer {
        padding: var(--space-2);
        border-top: 1px solid var(--dashboard-border);
    }

    /* Main Content */
    .dashboard-main {
        flex: 1;
        margin-left: 56px;
        padding: var(--space-4);
        min-height: 100vh;
    }

    /* Mobile: Bottom nav */
    @media (max-width: 768px) {
        .dashboard-rail {
            width: 100%;
            height: 56px;
            min-height: auto;
            flex-direction: row;
            position: fixed;
            bottom: 0;
            top: auto;
            border-right: none;
            border-top: 1px solid var(--dashboard-border);
        }

        .dashboard-rail.expanded {
            width: 100%;
        }

        .rail-logo {
            display: none;
        }

        .rail-items {
            flex-direction: row;
            justify-content: space-around;
            padding: var(--space-2);
            flex: 1;
        }

        .rail-item {
            flex-direction: column;
            padding: var(--space-2);
            gap: var(--space-1);
        }

        .rail-label {
            display: none;
        }

        .rail-footer {
            display: none;
        }

        .dashboard-main {
            margin-left: 0;
            margin-bottom: 56px;
        }
    }
</style>

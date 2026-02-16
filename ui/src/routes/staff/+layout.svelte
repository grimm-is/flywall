    import { onMount } from "svelte";
    import { goto } from "$app/navigation";
    import { authStatus } from "$lib/stores/app";
    import { page } from "$app/stores";
    import Icon from "$lib/components/Icon.svelte";
    import { sidebarExpanded } from "$lib/stores/ui";

    let { children } = $props();

    onMount(() => {
        if (!$authStatus?.is_staff) {
            goto("/");
        }
    });

    // Staff Navigation Items
    const navItems = [
        {
            id: "organizations",
            icon: "building",
            label: "Organizations",
            path: "/staff/organizations",
        },
        { id: "plans", icon: "tag", label: "Plans", path: "/staff/plans" },
        {
            id: "invoices",
            icon: "file-text",
            label: "Invoices",
            path: "/staff/invoices",
        },
    ];

    // Active Rail Logic
    let activeRail = $derived(() => {
        const path = $page.url.pathname;
        if (path.startsWith("/staff/plans")) return "plans";
        if (path.startsWith("/staff/invoices")) return "invoices";
        return "organizations"; // Default
    });

    function toggleSidebar() {
        $sidebarExpanded = !$sidebarExpanded;
    }
</script>

<div class="staff-shell">
    <!-- Staff Navigation Rail -->
    <nav
        class="staff-rail"
        class:expanded={$sidebarExpanded}
        aria-label="Staff navigation"
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
                <Icon name="shield-alert" size={24} />
                <span class="logo-text">Staff Admin</span>
            </div>
        </div>

        <div class="rail-items">
            {#each navItems as item}
                <a
                    href={item.path}
                    class="rail-item"
                    class:active={activeRail() === item.id}
                >
                    <Icon name={item.icon} size={20} />
                    <span class="rail-label">{item.label}</span>
                </a>
            {/each}
        </div>

        <div class="rail-footer">
            <a href="/" class="rail-item">
                <Icon name="log-out" size={20} />
                <span class="rail-label">Exit Staff</span>
            </a>
        </div>
    </nav>

    <!-- Main Content -->
    <main class="staff-main" class:expanded={$sidebarExpanded}>
        {@render children()}
    </main>
</div>

<style>
    .staff-shell {
        display: flex;
        min-height: 100vh;
        background: var(--dashboard-canvas);
    }

    .staff-rail {
        width: 64px;
        background: #1e1e24; /* Darker than normal dashboard */
        color: #fff;
        display: flex;
        flex-direction: column;
        position: fixed;
        top: 0;
        bottom: 0;
        left: 0;
        transition: width 0.2s;
        overflow: hidden;
        z-index: 50;
    }

    .staff-rail.expanded {
        width: 240px;
    }

    .rail-header {
        height: 64px;
        display: flex;
        align-items: center;
        padding: 0 1rem;
        border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    }

    .rail-toggle {
        background: none;
        border: none;
        color: rgba(255, 255, 255, 0.7);
        cursor: pointer;
        padding: 0.5rem;
        margin-right: 1rem;
    }

    .rail-logo {
        display: flex;
        align-items: center;
        gap: 0.75rem;
        font-weight: 600;
        font-size: 1.1rem;
        opacity: 0;
        transition: opacity 0.2s;
        white-space: nowrap;
    }

    .staff-rail.expanded .rail-logo {
        opacity: 1;
    }

    .rail-items {
        flex: 1;
        padding: 1rem 0.5rem;
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
    }

    .rail-item {
        display: flex;
        align-items: center;
        gap: 0.75rem;
        padding: 0.75rem;
        border-radius: 0.375rem;
        color: rgba(255, 255, 255, 0.7);
        text-decoration: none;
        transition: all 0.2s;
        white-space: nowrap;
    }

    .rail-item:hover {
        background: rgba(255, 255, 255, 0.1);
        color: #fff;
    }

    .rail-item.active {
        background: var(--color-primary);
        color: #fff;
    }

    .rail-label {
        opacity: 0;
        transition: opacity 0.2s;
    }

    .staff-rail.expanded .rail-label {
        opacity: 1;
    }

    .rail-footer {
        padding: 1rem 0.5rem;
        border-top: 1px solid rgba(255, 255, 255, 0.1);
    }

    .staff-main {
        flex: 1;
        margin-left: 64px;
        padding: 2rem;
        transition: margin-left 0.2s;
    }

    .staff-main.expanded {
        margin-left: 240px;
    }
</style>

<script lang="ts">
    import { page } from "$app/stores";
    import Icon from "$lib/components/Icon.svelte";

    let { children } = $props();

    const tabs = [
        {
            id: "overview",
            label: "Subscription",
            path: "/billing",
            icon: "credit-card",
        },
        {
            id: "invoices",
            label: "Invoices",
            path: "/billing/invoices",
            icon: "file-text",
        },
        {
            id: "transactions",
            label: "Transactions",
            path: "/billing/transactions",
            icon: "activity",
        },
    ];

    let activeTab = $derived(() => {
        const path = $page.url.pathname;
        if (path.includes("/invoices")) return "invoices";
        if (path.includes("/transactions")) return "transactions";
        return "overview";
    });
</script>

<div class="billing-container">
    <div class="billing-header">
        <h1>Billing & Subscription</h1>
        <p class="subtitle">
            Manage your plan, payment methods, and billing history
        </p>
    </div>

    <nav class="billing-nav">
        {#each tabs as tab}
            <a
                href={tab.path}
                class="nav-item"
                class:active={activeTab() === tab.id}
            >
                <Icon name={tab.icon} size={18} />
                <span>{tab.label}</span>
            </a>
        {/each}
    </nav>

    <div class="billing-content">
        {@render children()}
    </div>
</div>

<style>
    .billing-container {
        max-width: 1000px;
        margin: 0 auto;
        padding: 2rem;
    }

    .billing-header {
        margin-bottom: 2rem;
    }

    h1 {
        font-size: 2rem;
        font-weight: 700;
        margin: 0 0 0.5rem 0;
        color: var(--color-foreground);
    }

    .subtitle {
        color: var(--color-muted);
        font-size: 1.1rem;
    }

    .billing-nav {
        display: flex;
        gap: 0.5rem;
        border-bottom: 2px solid var(--color-border);
        margin-bottom: 2rem;
    }

    .nav-item {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        padding: 0.75rem 1.5rem;
        text-decoration: none;
        color: var(--color-muted);
        font-weight: 500;
        border-bottom: 2px solid transparent;
        margin-bottom: -2px;
        transition: all 0.2s;
        border-radius: 0.375rem 0.375rem 0 0;
    }

    .nav-item:hover {
        color: var(--color-foreground);
        background: var(--color-surfaceHover);
    }

    .nav-item.active {
        color: var(--color-primary);
        border-bottom-color: var(--color-primary);
    }

    .billing-content {
        animation: fade-in 0.3s ease;
    }

    @keyframes fade-in {
        from {
            opacity: 0;
            transform: translateY(10px);
        }
        to {
            opacity: 1;
            transform: translateY(0);
        }
    }
</style>

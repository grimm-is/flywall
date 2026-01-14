<script lang="ts">
    import { page } from "$app/stores";
    import { goto } from "$app/navigation";
    import Icon from "$lib/components/Icon.svelte";
    import Policy from "$lib/pages/Policy.svelte";
    import NAT from "$lib/pages/NAT.svelte";
    import Routing from "$lib/pages/Routing.svelte";
    import QoS from "$lib/pages/QoS.svelte";
    import { t } from "svelte-i18n";

    // Active tab state
    let activeTab = $state("security");

    // Sync from URL
    $effect(() => {
        activeTab = $page.url.searchParams.get("tab") || "security";
    });

    // Tab definitions
    const tabs = [
        { id: "security", label: "Security", icon: "shield" },
        { id: "nat", label: "NAT", icon: "swap_horiz" },
        { id: "routing", label: "Routing", icon: "alt_route" },
        { id: "traffic", label: "Traffic", icon: "traffic" },
        { id: "objects", label: "Objects", icon: "category" },
    ];

    function setTab(tabId: string) {
        const url = new URL($page.url);
        url.searchParams.set("tab", tabId);
        goto(url.toString(), { replaceState: false, noScroll: true });
    }
</script>

<div class="policy-page">
    <nav class="tab-bar">
        {#each tabs as tab}
            <button
                class="tab-btn"
                class:active={activeTab === tab.id}
                onclick={() => setTab(tab.id)}
            >
                <Icon name={tab.icon} size={16} />
                {tab.label}
            </button>
        {/each}
    </nav>

    <div class="tab-content">
        {#if activeTab === "security"}
            <section class="content-section">
                <Policy />
            </section>
        {:else if activeTab === "nat"}
            <section class="content-section">
                <NAT />
            </section>
        {:else if activeTab === "routing"}
            <section class="content-section">
                <Routing />
            </section>
        {:else if activeTab === "traffic"}
            <section class="content-section">
                <QoS />
            </section>
        {:else if activeTab === "objects"}
            <section class="content-section">
                <div class="empty-state">
                    <Icon name="category" size={48} />
                    <p>Objects management coming soon</p>
                </div>
            </section>
        {/if}
    </div>
</div>

<style>
    .policy-page {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
        height: 100%;
    }

    /* Tab Bar */
    .tab-bar {
        display: flex;
        gap: var(--space-1);
        padding: var(--space-1);
        background: var(--dashboard-card);
        border: 1px solid var(--dashboard-border);
        border-radius: var(--radius-lg);
    }

    .tab-btn {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-2) var(--space-4);
        background: none;
        border: none;
        border-radius: var(--radius-md);
        color: var(--dashboard-text-muted);
        font-size: var(--text-sm);
        cursor: pointer;
        transition: all var(--transition-fast);
    }

    .tab-btn:hover {
        background: var(--dashboard-input);
        color: var(--dashboard-text);
    }

    .tab-btn.active {
        background: var(--color-primary);
        color: var(--color-primaryForeground);
    }

    /* Content */
    .tab-content {
        flex: 1;
        min-height: 0;
        display: flex;
        flex-direction: column;
    }

    .content-section {
        flex: 1;
        display: flex;
        flex-direction: column;
        min-height: 0;
    }

    .empty-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: var(--space-4);
        padding: var(--space-12);
        color: var(--dashboard-text-muted);
        text-align: center;
        background: var(--dashboard-card);
        border-radius: var(--radius-lg);
        border: 1px solid var(--dashboard-border);
    }
</style>

<script lang="ts">
  import { page } from "$app/stores";
  import { goto } from "$app/navigation";
  import Icon from "$lib/components/Icon.svelte";

  // Import legacy components to wrap them
  import Interfaces from "$lib/pages/Interfaces.svelte";
  import Zones from "$lib/pages/Zones.svelte";
  import DHCP from "$lib/pages/DHCP.svelte";
  import DNS from "$lib/pages/DNS.svelte";
  import Devices from "$lib/pages/Devices.svelte";

  // Active tab from URL or default
  let activeTab = $derived($page.url.searchParams.get("tab") || "interfaces");

  // Tab definitions
  const tabs = [
    { id: "interfaces", label: "Interfaces", icon: "settings_ethernet" },
    { id: "zones", label: "Zones", icon: "security" },
    { id: "devices", label: "Devices", icon: "devices" },
    { id: "dhcp", label: "DHCP Server", icon: "dns" }, // Using DNS icon for DHCP as it relates to addressing
    { id: "dns", label: "DNS Server", icon: "language" },
  ];

  function setTab(tabId: string) {
    const url = new URL($page.url);
    url.searchParams.set("tab", tabId);
    goto(url.toString(), { replaceState: false, noScroll: true });
  }
</script>

<div class="network-page">
  <header class="page-header">
    <h1>Network</h1>
  </header>

  <!-- Tab Navigation -->
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

  <!-- Tab Content -->
  <div class="tab-content">
    {#if activeTab === "interfaces"}
      <Interfaces />
    {:else if activeTab === "zones"}
      <Zones />
    {:else if activeTab === "devices"}
      <Devices />
    {:else if activeTab === "dhcp"}
      <DHCP />
    {:else if activeTab === "dns"}
      <DNS />
    {/if}
  </div>
</div>

<style>
  .network-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .page-header h1 {
    font-size: var(--text-2xl);
    font-weight: 600;
    color: var(--dashboard-text);
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
    /* Legacy components supply their own containers, but we might need to reset some styles if they conflict */
  }
</style>

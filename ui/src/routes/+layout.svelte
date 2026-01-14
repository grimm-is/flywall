<script lang="ts">
  import "../lib/styles/global.css";
  import "$lib/i18n";
  import { isLoading, t } from "svelte-i18n";
  import { onMount, onDestroy } from "svelte";
  import { api, currentView, brand } from "$lib/stores/app";
  import { connectWebSocket, disconnectWebSocket } from "$lib/stores/websocket";
  import { initRuntimeStore, destroyRuntimeStore } from "$lib/stores/runtime";

  import AlertModal from "$lib/components/AlertModal.svelte";
  import StagedChangesBar from "$lib/components/StagedChangesBar.svelte";
  import Login from "$lib/components/auth/Login.svelte";
  import Setup from "$lib/components/auth/Setup.svelte";
  import DashboardLayout from "$lib/components/DashboardLayout.svelte";

  let { children } = $props();
  let pendingCheckInterval: ReturnType<typeof setInterval> | null = null;

  onMount(async () => {
    // Load brand info
    await api.getBrand();

    // Init runtime store (moved from dashboard layout)
    initRuntimeStore();

    // Check auth status
    const authData = await api.checkAuth();

    if (authData?.setup_required) {
      currentView.set("setup");
    } else if (!authData?.authenticated) {
      currentView.set("login");
    } else {
      await api.loadDashboard();
      currentView.set("app");
      // Connect WS
      connectWebSocket(["status", "logs", "stats", "notification"]);
    }
  });

  onDestroy(() => {
    if (pendingCheckInterval) {
      clearInterval(pendingCheckInterval);
    }
    disconnectWebSocket();
    destroyRuntimeStore();
  });

  // Effect to ensure WS connects if view switches to app (e.g. after login)
  $effect(() => {
    if ($currentView === "app") {
      connectWebSocket(["status", "logs", "stats", "notification"]);
    }
  });
</script>

<svelte:head>
  <title>{$brand?.name || "Flywall"}</title>
  <meta
    name="description"
    content={$brand?.tagline || "Network learning firewall"}
  />
</svelte:head>

{#if $isLoading}
  <div class="loading-overlay">Loading...</div>
{:else if $currentView === "setup"}
  <Setup />
{:else if $currentView === "login"}
  <Login />
{:else if $currentView === "app"}
  <DashboardLayout>
    {@render children()}
  </DashboardLayout>
  <AlertModal />
  <StagedChangesBar />
{:else}
  <!-- Loading application state -->
  <div class="loading-overlay">
    <div class="loading-spinner"></div>
    <p>Initializing...</p>
  </div>
{/if}

<style>
  .loading-overlay {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100vh;
    width: 100vw;
    background-color: var(--color-background);
    color: var(--color-foreground);
  }
</style>

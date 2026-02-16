<script lang="ts">
  import "../lib/styles/global.css";
  import "$lib/i18n";
  import { isLoading, t } from "svelte-i18n";
  import { onMount, onDestroy } from "svelte";
  import {
    api,
    currentView,
    brand,
    isConnected,
    connectionError,
  } from "$lib/stores/app";
  import { connectWebSocket, disconnectWebSocket } from "$lib/stores/websocket";
  import { initRuntimeStore, destroyRuntimeStore } from "$lib/stores/runtime";

  import AlertModal from "$lib/components/AlertModal.svelte";
  import Toast from "$lib/components/Toast.svelte";
  import StagedChangesBar from "$lib/components/StagedChangesBar.svelte";
  import Login from "$lib/components/auth/Login.svelte";
  import Setup from "$lib/components/auth/Setup.svelte";
  import DashboardLayout from "$lib/components/DashboardLayout.svelte";
  import Spinner from "$lib/components/Spinner.svelte";

  let { children } = $props();
  let pendingCheckInterval: ReturnType<typeof setInterval> | null = null;
  async function initAuth() {
    try {
      const status = await api.checkAuth();
      if (status) {
        if (status.setup_required) {
          currentView.set("setup");
        } else if (status.authenticated) {
          currentView.set("app");
          // Load initial data
          api.loadDashboard();
        } else {
          currentView.set("login");
        }
      } else {
        // If checkAuth failed (e.g. network), but we are "connected", default to login
        currentView.set("login");
      }
    } catch (e) {
      console.error("Auth check failed:", e);
      currentView.set("login");
    }
  }

  onMount(async () => {
    initRuntimeStore();

    // Load brand immediately
    await api.getBrand();

    // Initial auth check
    await initAuth();

    // Connect WS
    connectWebSocket();

    // Poll for pending changes
    pendingCheckInterval = setInterval(async () => {
      // Only check if we are in the app view
      let view;
      currentView.subscribe((v) => (view = v))();
      if (view === "app") {
        await api.checkPendingChanges();
      }
    }, 10000);
  });

  onDestroy(() => {
    destroyRuntimeStore();
    disconnectWebSocket();
    if (pendingCheckInterval) clearInterval(pendingCheckInterval);
  });
</script>

<svelte:head>
  <title>{$brand?.name || "Flywall"}</title>
  <meta
    name="description"
    content={$brand?.tagline || "Network learning firewall"}
  />
</svelte:head>

{#if !$isConnected}
  <div class="connection-overlay">
    <div class="connection-content">
      <Spinner size="lg" />
      <h2>Connecting to {$brand?.name || "Flywall"}...</h2>
      {#if $connectionError}
        <p class="error-text">{$connectionError}</p>
      {/if}
      <p class="sub-text">Please wait while the connection is established.</p>
    </div>
  </div>
{/if}

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
  <Toast />
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

  .connection-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(0, 0, 0, 0.7);
    backdrop-filter: blur(4px);
    z-index: 9999;
    display: flex;
    align-items: center;
    justify-content: center;
    color: white;
  }

  .connection-content {
    background: var(--color-background);
    color: var(--color-foreground);
    padding: var(--space-8);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-xl);
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-4);
    max-width: 400px;
  }

  .error-text {
    color: var(--color-destructive);
    font-size: var(--text-sm);
  }

  .sub-text {
    color: var(--color-muted);
    font-size: var(--text-sm);
  }
</style>

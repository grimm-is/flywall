<script lang="ts">
  /**
   * DHCP Page
   * DHCP server settings and lease management
   */

  import { config, leases, api } from "$lib/stores/app";
  import {
    Card,
    Button,
    Modal,
    Input,
    Select,
    Badge,
    Table,
    Icon,
  } from "$lib/components";
  import ScopeCard from "$lib/components/ScopeCard.svelte";
  import ScopeCreateCard from "$lib/components/ScopeCreateCard.svelte";
  import { t } from "svelte-i18n";

  let loading = $state(false);
  let isAddingScope = $state(false);
  let showSettingsModal = $state(false);

  // Global Settings
  let dhcpMode = $state("builtin");
  let dhcpLeaseFile = $state("");

  const dhcpConfig = $derived($config?.dhcp || { enabled: false, scopes: [] });
  const activeLeases = $derived(($leases || []).filter((l: any) => l.active));
  const interfaces = $derived($config?.interfaces || []);

  const leaseColumns = $derived([
    { key: "ip", label: $t("common.ip_address") },
    { key: "alias", label: $t("item.device") },
    { key: "mac", label: $t("network.mac_address") },
    { key: "vendor", label: $t("network.vendor") },
    { key: "hostname", label: $t("common.hostname") },
    { key: "interface", label: $t("common.interface") },
  ]);

  async function toggleDHCP() {
    loading = true;
    try {
      await api.updateDHCP({
        ...dhcpConfig,
        enabled: !dhcpConfig.enabled,
      });
    } catch (e) {
      console.error("Failed to toggle DHCP:", e);
    } finally {
      loading = false;
    }
  }

  function openSettings() {
    dhcpMode = dhcpConfig.mode || "builtin";
    dhcpLeaseFile = dhcpConfig.external_lease_file || "";
    showSettingsModal = true;
  }

  async function saveSettings() {
    loading = true;
    try {
      await api.updateDHCP({
        ...dhcpConfig,
        mode: dhcpMode,
        external_lease_file: dhcpLeaseFile || undefined,
      });
      showSettingsModal = false;
    } catch (e) {
      console.error("Failed to save DHCP settings:", e);
    } finally {
      loading = false;
    }
  }

  function toggleAddScope() {
    isAddingScope = !isAddingScope;
  }

  async function handleAddScope(event: CustomEvent) {
    loading = true;
    try {
      const newScope = event.detail;
      const updatedScopes = [...(dhcpConfig.scopes || []), newScope];

      await api.updateDHCP({
        ...dhcpConfig,
        scopes: updatedScopes,
      });
      isAddingScope = false;
    } catch (e) {
      console.error("Failed to add DHCP scope:", e);
    } finally {
      loading = false;
    }
  }

  async function handleUpdateScope(event: CustomEvent, index: number) {
    loading = true;
    try {
      const updatedScope = event.detail;
      const updatedScopes = [...(dhcpConfig.scopes || [])];
      updatedScopes[index] = updatedScope;

      await api.updateDHCP({
        ...dhcpConfig,
        scopes: updatedScopes,
      });
    } catch (e) {
      console.error("Failed to update DHCP scope:", e);
    } finally {
      loading = false;
    }
  }

  async function handleDeleteScope(event: CustomEvent, index: number) {
    if (
      !confirm(
        $t("common.delete_confirm_item", {
          values: { item: $t("item.scope") },
        }),
      )
    )
      return;

    loading = true;
    try {
      const updatedScopes = dhcpConfig.scopes.filter(
        (_: any, i: number) => i !== index,
      );
      await api.updateDHCP({
        ...dhcpConfig,
        scopes: updatedScopes,
      });
    } catch (e) {
      console.error("Failed to delete scope:", e);
    } finally {
      loading = false;
    }
  }
</script>

<div class="dhcp-page">
  <div class="page-header">
    <div class="header-actions">
      <Button variant="outline" onclick={openSettings} disabled={loading}>
        <Icon name="settings" />
        {$t("common.settings")}
      </Button>
      <Button
        variant={dhcpConfig.enabled ? "destructive" : "default"}
        onclick={toggleDHCP}
        disabled={loading}
      >
        {dhcpConfig.enabled ? $t("common.disable") : $t("common.enable")}
      </Button>
    </div>
  </div>

  <!-- Status -->
  <Card>
    <div class="status-row">
      <span class="status-label">{$t("common.status")}:</span>
      <Badge variant={dhcpConfig.enabled ? "success" : "secondary"}>
        {dhcpConfig.enabled ? $t("common.running") : $t("common.stopped")}
      </Badge>
    </div>
    {#if dhcpConfig.mode && dhcpConfig.mode !== "builtin"}
      <div class="status-row mt-2">
        <span class="status-label">{$t("dhcp.mode")}:</span>
        <Badge variant="outline">{dhcpConfig.mode}</Badge>
      </div>
    {/if}
  </Card>

  <!-- Scopes -->
  <div class="section">
    <div class="section-header">
      <h3>{$t("dhcp.scopes")}</h3>
      <Button variant={isAddingScope ? "primary" : "outline"} size="sm" onclick={toggleAddScope}
        >+ {$t("common.add_item", {
          values: { item: $t("item.scope") },
        })}</Button
      >
    </div>

    {#if isAddingScope}
       <div class="mb-4">
           <ScopeCreateCard
             {interfaces}
             {loading}
             on:save={handleAddScope}
             on:cancel={() => isAddingScope = false}
            />
       </div>
    {/if}

    {#if dhcpConfig.scopes?.length > 0}
      <div class="scopes-grid">
        {#each dhcpConfig.scopes as scope, scopeIndex}
          <div style:width="100%">
             <ScopeCard
                {scope}
                {interfaces}
                {loading}
                on:save={(e) => handleUpdateScope(e, scopeIndex)}
                on:delete={(e) => handleDeleteScope(e, scopeIndex)}
              />
          </div>
        {/each}
      </div>
    {:else}
      <Card>
        <p class="empty-message">
          {$t("common.no_items", { values: { items: $t("item.scope") } })}
        </p>
      </Card>
    {/if}
  </div>

  <!-- Leases -->
  <div class="section">
    <div class="section-header">
      <h3>
        {$t("dhcp.active_leases", { values: { n: activeLeases.length } })}
      </h3>
    </div>

    <Card>
      <Table
        columns={leaseColumns}
        data={activeLeases}
        emptyMessage="No active DHCP leases"
      />
    </Card>
  </div>
</div>

<!-- Settings Modal (Only modal remaining) -->
<Modal bind:open={showSettingsModal} title={$t("dhcp.settings_title")}>
  <div class="form-stack">
    <Select
      id="dhcp-mode"
      label={$t("dhcp.server_mode")}
      options={[
        { value: "builtin", label: $t("dhcp.server_modes.builtin") },
        { value: "external", label: $t("dhcp.server_modes.external") },
        { value: "import", label: $t("dhcp.server_modes.import") },
      ]}
      bind:value={dhcpMode}
    />

    {#if dhcpMode === "import"}
      <Input
        id="lease-file"
        label={$t("dhcp.external_lease_file")}
        bind:value={dhcpLeaseFile}
        placeholder="/var/lib/misc/dnsmasq.leases"
      />
    {/if}

    <div class="modal-actions">
      <Button variant="ghost" onclick={() => (showSettingsModal = false)}
        >{$t("common.cancel")}</Button
      >
      <Button onclick={saveSettings} disabled={loading}>
        {#if loading}<Spinner size="sm" />{/if}
        {$t("common.save")}
      </Button>
    </div>
  </div>
</Modal>

<style>
  .dhcp-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .status-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .status-label {
    font-weight: 500;
    color: var(--color-foreground);
  }

  .section {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .section-header h3 {
    font-size: var(--text-lg);
    font-weight: 600;
    margin: 0;
    color: var(--color-foreground);
  }

  .scopes-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: var(--space-4);
  }

  .empty-message {
    color: var(--color-muted);
    text-align: center;
    margin: 0;
  }

  .form-stack {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    margin-top: var(--space-4);
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
  }

  .header-actions {
    display: flex;
    gap: var(--space-2);
  }

  .mb-4 { margin-bottom: var(--space-4); }
</style>
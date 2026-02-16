<script lang="ts">
  /**
   * DNS Page
   * DNS server settings and upstream configuration
   */

  import { config, api } from "$lib/stores/app";
  import {
    Card,
    Button,
    Input,
    Badge,
    Spinner,
    Icon,
    Toggle,
    Select,
  } from "$lib/components";
  import ServeCard from "$lib/components/ServeCard.svelte";
  import ServeCreateCard from "$lib/components/ServeCreateCard.svelte";
  import ForwarderCreateCard from "$lib/components/dns/ForwarderCreateCard.svelte";
  import BlocklistCard from "$lib/components/dns/BlocklistCard.svelte";
  import HostCard from "$lib/components/dns/HostCard.svelte";
  import { t } from "svelte-i18n";

  let loading = $state(false);
  let isAddingForwarder = $state(false);
  let isAddingServe = $state(false);

  const dnsConfig = $derived(
    $config?.dns ||
      $config?.dns_server || { enabled: false, forwarders: [], listen_on: [] },
  );

  const usingNewFormat = $derived(!!$config?.dns);

  async function toggleDNS() {
    loading = true;
    try {
      // Logic depends on legacy vs new.
      // For new format, often presence implies enabled, or we toggle specific services.
      // But preserving existing logic for now if it works.
      const field = usingNewFormat ? "dns" : "dns_server";
      await api.updateDNS({
        [field]: {
          ...dnsConfig,
          enabled: !dnsConfig.enabled,
        },
      });
    } catch (e) {
      console.error("Failed to toggle DNS:", e);
    } finally {
      loading = false;
    }
  }

  function toggleAddServe() {
    isAddingServe = !isAddingServe;
  }

  async function handleAddServe(event: CustomEvent) {
    loading = true;
    try {
      const serveData = event.detail;
      const updatedServe = [...(dnsConfig.serve || []), serveData];
      await api.updateDNS({
        dns: { ...dnsConfig, serve: updatedServe },
      });
      isAddingServe = false;
    } catch (e: any) {
      console.error("Failed to add serve:", e);
    } finally {
      loading = false;
    }
  }

  async function handleUpdateServe(event: CustomEvent) {
    loading = true;
    try {
      const serveData = event.detail;
      const currentServes = dnsConfig.serve || [];
      const updatedServe = currentServes.map((s: any) =>
        s.zone === serveData.zone ? { ...s, ...serveData } : s,
      );
      await api.updateDNS({
        dns: { ...dnsConfig, serve: updatedServe },
      });
    } catch (e: any) {
      console.error("Failed to update serve:", e);
    } finally {
      loading = false;
    }
  }

  async function handleDeleteServe(event: CustomEvent) {
    const zoneName = event.detail;
    if (
      !confirm(
        $t("common.delete_confirm_item", {
          values: { item: $t("item.config") },
        }),
      )
    )
      return;

    loading = true;
    try {
      const currentServes = dnsConfig.serve || [];
      const updatedServe = currentServes.filter(
        (s: any) => s.zone !== zoneName,
      );
      await api.updateDNS({
        dns: { ...dnsConfig, serve: updatedServe },
      });
    } catch (e: any) {
      console.error("Failed to delete serve config:", e);
    } finally {
      loading = false;
    }
  }

  async function handleAddForwarder(event: CustomEvent) {
    const newForwarder = event.detail;
    if (!newForwarder) return;

    loading = true;
    try {
      const field = usingNewFormat ? "dns" : "dns_server";
      await api.updateDNS({
        [field]: {
          ...dnsConfig,
          forwarders: [...(dnsConfig.forwarders || []), newForwarder],
        },
      });
      isAddingForwarder = false;
    } catch (e) {
      console.error("Failed to add forwarder:", e);
    } finally {
      loading = false;
    }
  }

  async function removeForwarder(ip: string) {
    loading = true;
    try {
      const field = usingNewFormat ? "dns" : "dns_server";
      await api.updateDNS({
        [field]: {
          ...dnsConfig,
          forwarders: dnsConfig.forwarders.filter((f: string) => f !== ip),
        },
      });
    } catch (e) {
      console.error("Failed to remove forwarder:", e);
    } finally {
      loading = false;
    }
  }

  // Blocklist management
  let isAddingBlocklist = $state(false);
  let editingBlocklistIndex = $state<number | null>(null);

  function openAddBlocklist() {
    editingBlocklistIndex = null;
    isAddingBlocklist = true;
  }

  function openEditBlocklist(index: number) {
    editingBlocklistIndex = index;
    isAddingBlocklist = true;
  }

  function closeBlocklistForm() {
    isAddingBlocklist = false;
    editingBlocklistIndex = null;
  }

  async function handleSaveBlocklist(event: CustomEvent) {
    const newBlocklist = event.detail;
    if (!newBlocklist.name || !newBlocklist.url) return;

    loading = true;
    try {
      const currentBlocklists = dnsConfig.blocklists || [];
      let updatedBlocklists: any[];

      if (editingBlocklistIndex !== null) {
        updatedBlocklists = currentBlocklists.map((b: any, i: number) =>
          i === editingBlocklistIndex ? newBlocklist : b,
        );
      } else {
        updatedBlocklists = [...currentBlocklists, newBlocklist];
      }

      await api.updateDNS({
        dns: { ...dnsConfig, blocklists: updatedBlocklists },
      });
      closeBlocklistForm();
    } catch (e: any) {
      alert(`Failed to save blocklist: ${e.message || e}`);
      console.error("Failed to save blocklist:", e);
    } finally {
      loading = false;
    }
  }

  async function deleteBlocklist(name: string) {
    if (!confirm(`Delete blocklist "${name}"?`)) return;

    loading = true;
    try {
      const updatedBlocklists = (dnsConfig.blocklists || []).filter(
        (b: any) => b.name !== name,
      );
      await api.updateDNS({
        dns: {
          ...dnsConfig,
          blocklists: updatedBlocklists,
        },
      });
    } catch (e) {
      console.error("Failed to delete blocklist:", e);
    } finally {
      loading = false;
    }
  }

  // Toggle blocklist enabled state
  async function toggleBlocklist(blocklist: any) {
    loading = true;
    try {
      const updatedBlocklists = (dnsConfig.blocklists || []).map((b: any) =>
        b.name === blocklist.name ? { ...b, enabled: !b.enabled } : b,
      );
      await api.updateDNS({
        dns: {
          ...dnsConfig,
          blocklists: updatedBlocklists,
        },
      });
    } catch (e) {
      console.error("Failed to toggle blocklist:", e);
    } finally {
      loading = false;
    }
  }

  // Local Host management
  let isAddingHost = $state(false);
  let editingHostIndex = $state<number | null>(null);

  function openAddHost() {
    editingHostIndex = null;
    isAddingHost = true;
  }

  function openEditHost(index: number) {
    editingHostIndex = index;
    isAddingHost = true;
  }

  function closeHostForm() {
    isAddingHost = false;
    editingHostIndex = null;
  }

  async function handleSaveHost(event: CustomEvent) {
    const newHost = event.detail;
    if (!newHost.ip || !newHost.hostnames?.length) return;

    loading = true;
    try {
      const currentHosts = dnsConfig.hosts || [];
      let updatedHosts: any[];

      if (editingHostIndex !== null) {
        updatedHosts = currentHosts.map((h: any, i: number) =>
          i === editingHostIndex ? newHost : h,
        );
      } else {
        updatedHosts = [...currentHosts, newHost];
      }

      await api.updateDNS({
        dns: { ...dnsConfig, hosts: updatedHosts },
      });
      closeHostForm();
    } catch (e: any) {
      alert(`Failed to save host: ${e.message || e}`);
      console.error("Failed to save host:", e);
    } finally {
      loading = false;
    }
  }

  async function deleteHost(ip: string) {
    if (!confirm(`Delete host entry for ${ip}?`)) return;

    loading = true;
    try {
      const updatedHosts = (dnsConfig.hosts || []).filter(
        (h: any) => h.ip !== ip,
      );
      await api.updateDNS({
        dns: {
          ...dnsConfig,
          hosts: updatedHosts,
        },
      });
    } catch (e) {
      console.error("Failed to delete host:", e);
    } finally {
      loading = false;
    }
  }
</script>

<div class="dns-page">
  <div class="page-header">
    <div class="header-actions">
      <Button
        variant={dnsConfig.enabled ? "destructive" : "default"}
        onclick={toggleDNS}
        disabled={loading}
      >
        {dnsConfig.enabled ? $t("common.disable") : $t("common.enable")}
      </Button>
    </div>
  </div>

  <!-- Status -->
  <Card>
    <div class="status-row">
      <span class="status-label">{$t("common.status")}:</span>
      <Badge variant={dnsConfig.enabled ? "success" : "secondary"}>
        {dnsConfig.enabled ? $t("common.running") : $t("common.stopped")}
      </Badge>
    </div>
    {#if usingNewFormat}
      {#each dnsConfig.serve || [] as serve}
        {#if serve.listen_on?.length > 0}
          <div class="status-row" style="margin-top: var(--space-2)">
            <span class="status-label"
              >{$t("dns.listening_on")} ({serve.zone}):</span
            >
            <span class="mono">{serve.listen_on.join(", ")}</span>
          </div>
        {/if}
      {/each}
    {:else if dnsConfig.listen_on?.length > 0}
      <div class="status-row" style="margin-top: var(--space-2)">
        <span class="status-label">{$t("dns.listening_on_generic")}:</span>
        <span class="mono">{dnsConfig.listen_on.join(", ")}</span>
      </div>
    {/if}
  </Card>

  <!-- Forwarders -->
  <div class="section">
    <div class="section-header">
      <h3>{$t("dns.upstream_forwarders")}</h3>
      <Button
        variant="outline"
        onclick={() => (isAddingForwarder = !isAddingForwarder)}
      >
        {isAddingForwarder
          ? "Cancel"
          : "+ " +
            $t("common.add_item", { values: { item: $t("item.forwarder") } })}
      </Button>
    </div>

    {#if isAddingForwarder}
      <div class="mb-4">
        <ForwarderCreateCard
          {loading}
          on:save={handleAddForwarder}
          on:cancel={() => (isAddingForwarder = false)}
        />
      </div>
    {/if}

    {#if dnsConfig.forwarders?.length > 0}
      <div class="forwarders-list">
        {#each dnsConfig.forwarders as forwarder}
          <Card>
            <div class="forwarder-item">
              <span class="forwarder-ip mono">{forwarder}</span>
              <Button
                variant="ghost"
                onclick={() => removeForwarder(forwarder)}
              >
                <Icon name="delete" />
              </Button>
            </div>
          </Card>
        {/each}
      </div>
    {:else}
      <Card>
        <p class="empty-message">
          {$t("common.no_items", { values: { items: $t("item.forwarder") } })}
        </p>
      </Card>
    {/if}
  </div>

  <!-- Zone Serving (New Format) -->
  {#if usingNewFormat}
    <div class="section">
      <div class="section-header">
        <h3>{$t("dns.zone_serving")}</h3>
        <Button
          variant={isAddingServe ? "default" : "outline"}
          onclick={toggleAddServe}
        >
          + {$t("common.add_item", { values: { item: $t("item.config") } })}
        </Button>
      </div>

      {#if isAddingServe}
        <div class="mb-4">
          <ServeCreateCard
            {loading}
            on:save={handleAddServe}
            on:cancel={() => (isAddingServe = false)}
          />
        </div>
      {/if}

      {#if dnsConfig.serve?.length > 0}
        <div class="serve-list">
          {#each dnsConfig.serve as serve}
            <div style="width: 100%">
              <ServeCard
                {serve}
                {loading}
                on:save={handleUpdateServe}
                on:delete={handleDeleteServe}
              />
            </div>
          {/each}
        </div>
      {:else}
        <Card>
          <p class="empty-message">
            {$t("common.no_items", { values: { items: $t("item.config") } })}
          </p>
        </Card>
      {/if}
    </div>
  {/if}

  <!-- DNS Inspection (Only shown if using new format) -->
  {#if usingNewFormat && dnsConfig.inspect?.length > 0}
    <div class="section">
      <div class="section-header">
        <h3>{$t("dns.inspect")}</h3>
      </div>
      <div class="inspect-list">
        {#each dnsConfig.inspect as inspect}
          <Card>
            <div class="inspect-item">
              <div class="inspect-info">
                <span class="zone-name"
                  >{$t("dns.zone")}: <strong>{inspect.zone}</strong></span
                >
                <Badge
                  variant={inspect.mode === "redirect"
                    ? "warning"
                    : "secondary"}
                >
                  {inspect.mode === "redirect"
                    ? $t("dns.inspect_mode.redirect")
                    : $t("dns.inspect_mode.passive")}
                </Badge>
              </div>
              {#if inspect.exclude_router}
                <span class="exclude-router-tag"
                  >{$t("dns.excluding_router")}</span
                >
              {/if}
            </div>
          </Card>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Blocklists Section -->
  {#if usingNewFormat}
    <div class="section">
      <div class="section-header">
        <h3>{$t("dns.blocklists")}</h3>
        <Button variant="outline" onclick={openAddBlocklist}>
          + {$t("common.add_item", { values: { item: $t("item.blocklist") } })}
        </Button>
      </div>

      {#if isAddingBlocklist}
        <div class="mb-4">
          <BlocklistCard
            blocklist={editingBlocklistIndex !== null
              ? dnsConfig.blocklists[editingBlocklistIndex]
              : null}
            {loading}
            on:save={handleSaveBlocklist}
            on:cancel={closeBlocklistForm}
          />
        </div>
      {/if}

      {#if dnsConfig.blocklists?.length > 0}
        <div class="blocklist-list">
          {#each dnsConfig.blocklists as blocklist}
            <Card>
              <div class="blocklist-item">
                <div class="blocklist-info">
                  <div class="blocklist-header">
                    <span class="blocklist-name">{blocklist.name}</span>
                    <Badge
                      variant={blocklist.enabled !== false
                        ? "success"
                        : "secondary"}
                    >
                      {blocklist.enabled !== false
                        ? $t("common.enabled")
                        : $t("common.disabled")}
                    </Badge>
                  </div>
                  <span class="blocklist-url mono">{blocklist.url}</span>
                </div>
                <div class="blocklist-actions">
                  <Button
                    variant="ghost"
                    size="sm"
                    onclick={() => toggleBlocklist(blocklist)}
                  >
                    <Icon
                      name={blocklist.enabled !== false ? "pause" : "play"}
                    />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onclick={() =>
                      openEditBlocklist(
                        dnsConfig.blocklists.indexOf(blocklist),
                      )}
                  >
                    <Icon name="edit" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onclick={() => deleteBlocklist(blocklist.name)}
                  >
                    <Icon name="delete" />
                  </Button>
                </div>
              </div>
            </Card>
          {/each}
        </div>
      {:else}
        <Card>
          <p class="empty-message">
            {$t("common.no_items", { values: { items: $t("item.blocklist") } })}
          </p>
        </Card>
      {/if}
    </div>
  {/if}

  <!-- Local Hosts Section -->
  {#if usingNewFormat}
    <div class="section">
      <div class="section-header">
        <h3>{$t("dns.local_hosts")}</h3>
        <Button variant="outline" onclick={openAddHost}>
          + {$t("common.add_item", { values: { item: $t("item.host") } })}
        </Button>
      </div>

      {#if isAddingHost}
        <div class="mb-4">
          <HostCard
            host={editingHostIndex !== null
              ? dnsConfig.hosts[editingHostIndex]
              : null}
            {loading}
            on:save={handleSaveHost}
            on:cancel={closeHostForm}
          />
        </div>
      {/if}

      {#if dnsConfig.hosts?.length > 0}
        <div class="hosts-list">
          {#each dnsConfig.hosts as host}
            <Card>
              <div class="host-item">
                <div class="host-info">
                  <span class="host-ip mono">{host.ip}</span>
                  <span class="host-names"
                    >{(host.hostnames || []).join(", ")}</span
                  >
                </div>
                <div class="host-actions">
                  <Button
                    variant="ghost"
                    size="sm"
                    onclick={() => openEditHost(dnsConfig.hosts.indexOf(host))}
                  >
                    <Icon name="edit" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onclick={() => deleteHost(host.ip)}
                  >
                    <Icon name="delete" />
                  </Button>
                </div>
              </div>
            </Card>
          {/each}
        </div>
      {:else}
        <Card>
          <p class="empty-message">
            {$t("common.no_items", { values: { items: $t("item.host") } })}
          </p>
        </Card>
      {/if}
    </div>
  {/if}
</div>

<style>
  .dns-page {
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

  .forwarders-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .forwarder-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .forwarder-ip {
    color: var(--color-foreground);
  }

  .inspect-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .inspect-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .inspect-info {
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }

  .zone-name {
    color: var(--color-foreground);
  }

  .exclude-router-tag {
    font-size: var(--text-xs);
    background: var(--color-surface-hover);
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
    color: var(--color-muted);
  }

  .mono {
    font-family: var(--font-mono);
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
  .serve-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .serve-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .serve-info {
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }

  .zone-badge {
    background-color: var(--color-primary);
    color: white;
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-md);
    font-weight: 600;
    font-size: var(--text-sm);
  }

  .serve-details {
    display: flex;
    gap: var(--space-2);
  }

  .serve-actions {
    display: flex;
    gap: var(--space-1);
  }

  /* Blocklist styles */
  .blocklist-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .blocklist-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .blocklist-info {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .blocklist-header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .blocklist-name {
    font-weight: 600;
    color: var(--color-foreground);
  }

  .blocklist-url {
    font-size: var(--text-sm);
    color: var(--color-muted);
    max-width: 400px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .blocklist-actions {
    display: flex;
    gap: var(--space-1);
  }

  /* Host styles */
  .hosts-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .host-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .host-info {
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }

  .host-ip {
    font-weight: 600;
    color: var(--color-foreground);
    min-width: 120px;
  }

  .host-names {
    color: var(--color-muted);
    font-size: var(--text-sm);
  }

  .host-actions {
    display: flex;
    gap: var(--space-1);
  }
</style>

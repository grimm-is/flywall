<script lang="ts">
  /**
   * Routing Page
   * Static routes management + Kernel routing table view
   */

  import { onMount } from "svelte";
  import { config, api } from "$lib/stores/app";
  import {
    Card,
    Button,
    Input,
    Select,
    Badge,
    Table,
    Spinner,
    Icon,
  } from "$lib/components";
  import RouteCreateCard from "$lib/components/RouteCreateCard.svelte";
  import StaticRouteCard from "$lib/components/routing/StaticRouteCard.svelte";
  import MarkRuleCard from "$lib/components/routing/MarkRuleCard.svelte";
  import UidRuleCard from "$lib/components/routing/UidRuleCard.svelte";
  import { t } from "svelte-i18n";

  let loading = $state(false);
  let isEditingRoute = $state(false);
  let isAddingRoute = $state(false);
  let editingRouteIndex = $state<number | null>(null);
  let isAddingMark = $state(false);
  let editingMarkIndex = $state<number | null>(null);
  let isAddingUid = $state(false);
  let editingUidIndex = $state<number | null>(null);

  let activeTab = $state<"kernel" | "routes" | "marks" | "uid">("kernel");

  // Kernel routes (live from system)
  let kernelRoutes = $state<any[]>([]);
  let kernelRoutesLoading = $state(false);
  let kernelRoutesError = $state<string | null>(null);

  onMount(() => {
    loadKernelRoutes();
  });

  async function loadKernelRoutes() {
    kernelRoutesLoading = true;
    kernelRoutesError = null;
    try {
      const result = await api.getSystemRoutes();
      kernelRoutes = result.routes || [];
    } catch (e: any) {
      kernelRoutesError = e.message || "Failed to load kernel routes";
    } finally {
      kernelRoutesLoading = false;
    }
  }

  // Route form
  let routeDestination = $state("");
  let routeGateway = $state("");
  let routeInterface = $state("");
  let routeMetric = $state("100");

  const routes = $derived($config?.routes || []);
  const markRules = $derived($config?.mark_rules || []);
  const uidRouting = $derived($config?.uid_routing || []);
  const interfaces = $derived($config?.interfaces || []);

  const routeColumns = [
    { key: "destination", label: "Destination" },
    { key: "gateway", label: "Gateway" },
    { key: "interface", label: "Interface" },
    { key: "metric", label: "Metric" },
  ];

  function toggleAddRoute() {
    isAddingRoute = !isAddingRoute;
  }

  async function handleCreateRoute(event: CustomEvent) {
    const data = event.detail;
    loading = true;
    try {
      const newRoute = {
        destination: data.destination,
        gateway: data.gateway || undefined,
        interface: data.interface || undefined,
        metric: parseInt(data.metric) || 100,
      };
      const updatedRoutes = [...routes, newRoute];
      await api.updateRoutes(updatedRoutes);
      isAddingRoute = false;
    } catch (e: any) {
      console.error("Failed to add route:", e);
    } finally {
      loading = false;
    }
  }

  function openEditRoute(index: number) {
    editingRouteIndex = index;
    const route = routes[index];
    routeDestination = route.destination || "";
    routeGateway = route.gateway || "";
    routeInterface = route.interface || "";
    routeMetric = route.metric?.toString() || "100";
    isEditingRoute = true;
  }

  async function saveRoute() {
    if (!routeDestination || (!routeGateway && !routeInterface)) return;

    loading = true;
    try {
      const newRoute = {
        destination: routeDestination,
        gateway: routeGateway || undefined,
        interface: routeInterface || undefined,
        metric: parseInt(routeMetric) || 100,
      };

      let updatedRoutes;
      if (editingRouteIndex !== null) {
        updatedRoutes = [...routes];
        updatedRoutes[editingRouteIndex] = newRoute;
      } else {
        updatedRoutes = [...routes, newRoute];
      }

      await api.updateRoutes(updatedRoutes);
      isEditingRoute = false;
      editingRouteIndex = null;
    } catch (e) {
      console.error("Failed to save route:", e);
    } finally {
      loading = false;
    }
  }

  async function deleteRoute(index: number) {
    loading = true;
    try {
      const updatedRoutes = routes.filter((_: any, i: number) => i !== index);
      await api.updateRoutes(updatedRoutes);
    } catch (e) {
      console.error("Failed to delete route:", e);
    } finally {
      loading = false;
    }
  }

  // --- Mark Rules Logic ---
  // Mark Rule Form
  let mrName = $state("");
  let mrMark = $state("");
  let mrSrcIP = $state("");
  let mrDstIP = $state("");
  let mrProtocol = $state("all");
  let mrOutInterface = $state("");
  let mrSaveMark = $state(false);
  let mrEnabled = $state(true);

  function openAddMarkRule() {
    editingMarkIndex = null;
    mrName = "";
    mrMark = "";
    mrSrcIP = "";
    mrDstIP = "";
    mrProtocol = "all";
    mrOutInterface = "";
    mrSaveMark = true;
    mrEnabled = true;
    isAddingMark = true;
  }

  function openEditMarkRule(index: number) {
    editingMarkIndex = index;
    const r = markRules[index];
    mrName = r.name || "";
    mrMark = r.mark?.toString() || "";
    mrSrcIP = r.src_ip || "";
    mrDstIP = r.dst_ip || "";
    mrProtocol = r.proto || "all";
    mrOutInterface = r.out_interface || "";
    mrSaveMark = r.save_mark ?? true;
    mrEnabled = r.enabled ?? true;
    isAddingMark = true;
  }

  async function saveMarkRule() {
    if (!mrName || !mrMark) return;
    loading = true;
    try {
      const rule = {
        name: mrName,
        mark: parseInt(mrMark) || 0,
        src_ip: mrSrcIP,
        dst_ip: mrDstIP,
        proto: mrProtocol,
        out_interface: mrOutInterface,
        save_mark: mrSaveMark,
        enabled: mrEnabled,
      };

      let updated = [...markRules];
      if (editingMarkIndex !== null) {
        updated[editingMarkIndex] = rule;
      } else {
        updated.push(rule);
      }

      await (api as any).updateMarkRules(updated);
      isAddingMark = false;
      editingMarkIndex = null;
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  async function deleteMarkRule(index: number) {
    if (
      !confirm(
        $t("common.delete_confirm_item", {
          values: { item: $t("item.mark_rule") },
        }),
      )
    )
      return;
    loading = true;
    try {
      const updated = markRules.filter((_: any, i: number) => i !== index);
      await (api as any).updateMarkRules(updated);
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  // --- UID Routing Logic ---
  let uidName = $state("");
  let uidUID = $state("");
  let uidUplink = $state("");
  let uidEnabled = $state(true);

  function openAddUIDRule() {
    editingUidIndex = null;
    uidName = "";
    uidUID = "";
    uidUplink = "";
    uidEnabled = true;
    isAddingUid = true;
  }

  function openEditUIDRule(index: number) {
    editingUidIndex = index;
    const r = uidRouting[index];
    uidName = r.name || "";
    uidUID = r.uid?.toString() || "";
    uidUplink = r.uplink || "";
    uidEnabled = r.enabled ?? true;
    isAddingUid = true;
  }

  async function saveUIDRule() {
    if (!uidName || !uidUID || !uidUplink) return;
    loading = true;
    try {
      const rule = {
        name: uidName,
        uid: parseInt(uidUID),
        uplink: uidUplink,
        enabled: uidEnabled,
      };
      let updated = [...uidRouting];
      if (editingUidIndex !== null) {
        updated[editingUidIndex] = rule;
      } else {
        updated.push(rule);
      }
      await (api as any).updateUIDRouting(updated);
      isAddingUid = false;
      editingUidIndex = null;
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  async function deleteUIDRule(index: number) {
    if (
      !confirm(
        $t("common.delete_confirm_item", {
          values: { item: $t("item.uid_rule") },
        }),
      )
    )
      return;
    loading = true;
    try {
      const updated = uidRouting.filter((_: any, i: number) => i !== index);
      await (api as any).updateUIDRouting(updated);
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }
</script>

<div class="routing-page">
  <div class="page-header"></div>

  <div class="tabs">
    <Button
      variant={activeTab === "kernel" ? "default" : "ghost"}
      onclick={() => (activeTab = "kernel")}
      aria-pressed={activeTab === "kernel"}>Kernel Routes</Button
    >
    <Button
      variant={activeTab === "routes" ? "default" : "ghost"}
      onclick={() => (activeTab = "routes")}
      aria-pressed={activeTab === "routes"}
      >{$t("routing.static_routes")}</Button
    >
    <Button
      variant={activeTab === "marks" ? "default" : "ghost"}
      onclick={() => (activeTab = "marks")}
      aria-pressed={activeTab === "marks"}>{$t("routing.mark_rules")}</Button
    >
    <Button
      variant={activeTab === "uid" ? "default" : "ghost"}
      onclick={() => (activeTab = "uid")}
      aria-pressed={activeTab === "uid"}>{$t("routing.user_routing")}</Button
    >
  </div>

  {#if activeTab === "kernel"}
    <div class="sub-header">
      <h3>Kernel Routing Table</h3>
      <Button
        onclick={loadKernelRoutes}
        size="sm"
        disabled={kernelRoutesLoading}
      >
        <Icon name="refresh" size="sm" />
        Refresh
      </Button>
    </div>
    <Card>
      {#if kernelRoutesLoading}
        <div class="loading-state">
          <Spinner size="md" />
          <span>Loading kernel routes...</span>
        </div>
      {:else if kernelRoutesError}
        <p class="error-message">{kernelRoutesError}</p>
      {:else if kernelRoutes.length === 0}
        <p class="empty-message">No kernel routes found</p>
      {:else}
        <div class="kernel-routes-table">
          <table>
            <thead>
              <tr>
                <th>Destination</th>
                <th>Gateway</th>
                <th>Interface</th>
                <th>Metric</th>
                <th>Protocol</th>
                <th>Scope</th>
              </tr>
            </thead>
            <tbody>
              {#each kernelRoutes as route}
                <tr>
                  <td
                    ><code>{route.destination || route.dst || "default"}</code
                    ></td
                  >
                  <td><code>{route.gateway || route.gw || "-"}</code></td>
                  <td
                    ><Badge variant="outline"
                      >{route.interface || route.dev || "-"}</Badge
                    ></td
                  >
                  <td>{route.metric || route.priority || "-"}</td>
                  <td
                    ><Badge variant="secondary"
                      >{route.protocol || route.proto || "-"}</Badge
                    ></td
                  >
                  <td>{route.scope || "-"}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </Card>
  {:else if activeTab === "routes"}
    <div class="sub-header">
      <h3>{$t("routing.static_routes")}</h3>
      <Button onclick={toggleAddRoute} size="sm"
        >{isAddingRoute
          ? $t("common.cancel")
          : `+ ${$t("common.add_item", {
              values: { item: $t("item.static_route") },
            })}`}</Button
      >
    </div>

    {#if isAddingRoute}
      <div class="mb-4">
        <RouteCreateCard
          {loading}
          {interfaces}
          on:save={handleCreateRoute}
          on:cancel={toggleAddRoute}
        />
      </div>
    {/if}

    <Card>
      {#if routes.length === 0}
        <p class="empty-message">
          {$t("common.no_items", {
            values: { items: $t("item.static_route") },
          })}
        </p>
      {:else}
        <div class="routes-list">
          {#each routes as route, index}
            <div class="route-row">
              <code class="route-dest">{route.destination}</code>
              <span class="route-arrow">{$t("routing.via")}</span>
              {#if route.gateway}
                <code class="route-gateway">{route.gateway}</code>
              {/if}
              {#if route.interface}
                <Badge variant="outline">{route.interface}</Badge>
              {/if}
              <span class="route-metric"
                >{$t("routing.metric_val", {
                  values: { n: route.metric || 100 },
                })}</span
              >
              <Button
                variant="ghost"
                size="sm"
                onclick={() => openEditRoute(index)}
                ><Icon name="edit" size="sm" /></Button
              >
              <Button
                variant="ghost"
                size="sm"
                onclick={() => deleteRoute(index)}
                ><Icon name="delete" size="sm" /></Button
              >
            </div>
          {/each}
        </div>
      {/if}
    </Card>
  {:else if activeTab === "marks"}
    <div class="sub-header">
      <h3>{$t("routing.mark_rules")}</h3>
      <Button onclick={openAddMarkRule} size="sm"
        >+ {$t("common.add_item", {
          values: { item: $t("item.mark_rule") },
        })}</Button
      >
    </div>
    <Card>
      {#if markRules.length === 0}
        <p class="empty-message">
          {$t("common.no_items", { values: { items: $t("item.mark_rule") } })}
        </p>
      {:else}
        <Table
          columns={[
            { key: "name", label: $t("common.name") },
            { key: "mark", label: $t("routing.mark") },
            { key: "match", label: $t("routing.match") },
            { key: "action", label: $t("routing.action") },
            { key: "actions", label: "" },
          ]}
          data={markRules.map((r: any) => ({
            ...r,
            match: `${r.src_ip || "*"} -> ${r.out_interface || "*"}`,
            action: r.save_mark ? "Save" : "-",
          }))}
        >
          {#snippet children(r: any, i: number)}
            <td>{r.name}</td>
            <td><Badge>{r.mark}</Badge></td>
            <td>
              <div class="filter-match">
                {#if r.src_ip}
                  <span
                    >{$t("routing.src_prefix", {
                      values: { ip: r.src_ip },
                    })}</span
                  >
                {/if}
                {#if r.out_interface}
                  <span
                    >{$t("routing.out_prefix", {
                      values: { iface: r.out_interface },
                    })}</span
                  >
                {/if}
              </div>
            </td>
            <td>{r.save_mark ? $t("routing.save_mark") : ""}</td>
            <td class="actions">
              <Button
                variant="ghost"
                size="sm"
                onclick={() => openEditMarkRule(i)}
                ><Icon name="edit" size="sm" /></Button
              >
              <Button
                variant="ghost"
                size="sm"
                onclick={() => deleteMarkRule(i)}
                ><Icon name="delete" size="sm" /></Button
              >
            </td>
          {/snippet}
        </Table>
      {/if}
    </Card>
  {:else if activeTab === "uid"}
    <div class="sub-header">
      <h3>{$t("routing.user_routing")}</h3>
      <Button onclick={openAddUIDRule} size="sm"
        >+ {$t("common.add_item", {
          values: { item: $t("item.uid_rule") },
        })}</Button
      >
    </div>
    <Card>
      {#if uidRouting.length === 0}
        <p class="empty-message">
          {$t("common.no_items", { values: { items: $t("item.uid_rule") } })}
        </p>
      {:else}
        <div class="routes-list">
          {#each uidRouting as route, index}
            <div class="route-row">
              <span
                >{$t("routing.uid_label", { values: { uid: route.uid } })}</span
              >
              <span class="route-arrow">{$t("routing.via")}</span>
              <Badge>{route.uplink}</Badge>
              <Button
                variant="ghost"
                size="sm"
                onclick={() => openEditUIDRule(index)}
                ><Icon name="edit" size="sm" /></Button
              >
              <Button
                variant="ghost"
                size="sm"
                onclick={() => deleteUIDRule(index)}
                ><Icon name="delete" size="sm" /></Button
              >
            </div>
          {/each}
        </div>
      {/if}
    </Card>
  {/if}
</div>

<style>
  .routing-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .tabs {
    display: flex;
    gap: var(--space-2);
    border-bottom: 1px solid var(--color-border);
    padding-bottom: var(--space-2);
    margin-bottom: var(--space-4);
  }

  .sub-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: var(--space-2);
  }

  .sub-header h3 {
    font-size: var(--text-lg);
    margin: 0;
  }

  .actions {
    display: flex;
    gap: 5px;
  }

  .filter-match span {
    display: block;
    font-size: var(--text-xs);
    color: var(--color-muted);
  }

  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .routes-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .route-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3);
    background-color: var(--color-backgroundSecondary);
    border-radius: var(--radius-md);
  }

  .route-dest {
    font-family: var(--font-mono);
    font-weight: 600;
    color: var(--color-foreground);
  }

  .route-arrow {
    color: var(--color-muted);
  }

  .route-gateway {
    font-family: var(--font-mono);
    color: var(--color-foreground);
  }

  .route-metric {
    margin-left: auto;
    color: var(--color-muted);
    font-size: var(--text-sm);
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

  /* Kernel Routes Table */
  .kernel-routes-table {
    overflow-x: auto;
  }

  .kernel-routes-table table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--text-sm);
  }

  .kernel-routes-table th,
  .kernel-routes-table td {
    padding: var(--space-2) var(--space-3);
    text-align: left;
    border-bottom: 1px solid var(--color-border);
  }

  .kernel-routes-table th {
    font-weight: 600;
    color: var(--color-muted);
    font-size: var(--text-xs);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .kernel-routes-table td code {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }

  .kernel-routes-table tr:hover {
    background: var(--color-backgroundSecondary);
  }

  .loading-state {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-3);
    padding: var(--space-8);
    color: var(--color-muted);
  }

  .error-message {
    color: var(--color-destructive);
    text-align: center;
    padding: var(--space-4);
  }
</style>

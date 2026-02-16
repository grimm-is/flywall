<script lang="ts">
  /**
   * Interfaces Page
   * Network interface configuration with full CRUD
   */

  import { onMount } from "svelte";
  import { config, api } from "$lib/stores/app";
  import {
    Card,
    Button,
    Spinner,
    Icon,
    Modal,
  } from "$lib/components";
  import InterfaceCard from "$lib/components/InterfaceCard.svelte";
  import VlanCreateCard from "$lib/components/VlanCreateCard.svelte";
  import BondCreateCard from "$lib/components/BondCreateCard.svelte";
  import { t } from "svelte-i18n";

  let loading = $state(false);
  let isAddingVlan = $state(false);
  let isAddingBond = $state(false);

  // Removed global edit modal state
  let interfaceStatus = $state<any[]>([]);

  // Load runtime status
  onMount(async () => {
    loading = true;
    try {
      const [res, available] = await Promise.all([
        api.getInterfaces(),
        api.getAvailableInterfaces(),
      ]);

      // Handle both array response and object with .interfaces property
      const rawData = Array.isArray(res) ? res : res.interfaces || [];
      const availData = Array.isArray(available) ? available : [];

      // Create a map of existing status for quick lookup
      const statusMap = new Map(rawData.map((s: any) => [s.name, s]));

      // Merge available interfaces that aren't in status
      availData.forEach((avail: any) => {
        if (!statusMap.has(avail.name)) {
          // This is an unconfigured interface
          rawData.push({
            name: avail.name,
            state: avail.link_up ? "up" : "down",
            type: "ethernet", // Assuming mostly ethernet from available list
            mac: avail.mac,
            // Add other fields as needed
            is_unconfigured: true,
          });
        }
      });

      // Normalize API response (snake_case) to Component model (PascalCase)
      interfaceStatus = rawData.map((s: any) => ({
        ...s,
        Name: s.name,
        State: s.state,
        IPv4Addrs: s.ipv4_addrs,
        IPv6Addrs: s.ipv6_addrs,
      }));
    } catch (e) {
      console.error("Failed to load interface status", e);
    } finally {
      loading = false;
    }
  });

  const zones = $derived($config?.zones || []);
  const rawInterfaces = $derived($config?.interfaces || []);

  // Merge static config with runtime status, OR use runtime status directly if no config
  // Merge static config with runtime status to show ALL interface
  const interfaces = $derived.by(() => {
    const configMap = new Map(rawInterfaces.map((i: any) => [i.Name, i]));
    const statusMap = new Map(interfaceStatus.map((s: any) => [s.Name, s]));

    // Union of all names
    const allNames = new Set([...configMap.keys(), ...statusMap.keys()]);

    return Array.from(allNames)
      .map((name) => {
        const configIface = configMap.get(name);
        const statusIface = statusMap.get(name);

        // Base object
        const iface: any = configIface ? { ...configIface } : { Name: name };

        // If only in status (unconfigured), mark it
        if (!configIface && statusIface) {
          iface.is_unconfigured = true;
          // Infer type from name or status type if available
        }

        // Merge status details
        if (statusIface) {
          iface.IPv4 = statusIface.IPv4Addrs?.length
            ? statusIface.IPv4Addrs
            : iface.IPv4;
          iface.State = statusIface.State;
          // Propagate unconfigured state from status if set there (e.g. from onMount)
          if (statusIface.is_unconfigured) {
            iface.is_unconfigured = true;
          }
        }

        return iface;
      })
      .sort((a, b) => a.Name.localeCompare(b.Name));
  });

  const memberOfMap = $derived.by(() => {
    const map = new Map<string, string>();
    interfaces.forEach((iface: any) => {
      const type = getInterfaceType(iface);
      if (type === "bond") {
        const members = iface.Bond?.members || iface.Members || [];
        members.forEach((m: string) => map.set(m, iface.Name));
      }
    });
    return map;
  });

  // All hardware interfaces for bond creation
  const hardwareInterfaces = $derived(
    interfaces
      .filter((iface: any) => {
        // Hardware interfaces: not a VLAN (no '.'), not a bond (no 'bond' prefix)
        // Also exclude other virtual types if known, but generally name check suffices for now
        return (
          !iface.Name?.includes(".") &&
          !iface.Name?.startsWith("bond") &&
          !iface.Name?.startsWith("wg") &&
          !iface.Name?.startsWith("tun")
        );
      })
      .map((iface: any) => {
        // Check if already in a bond (in CONFIG)
        // We check rawInterfaces to see if it's bound in the persisted config
        const inBond = rawInterfaces.some(
          (other: any) =>
            other.Bond?.members?.includes(iface.Name) ||
            other.Members?.includes(iface.Name),
        );
        // Check if it IS a bond (has members in CONFIG)
        const isBond =
          iface.Bond?.members?.length > 0 || iface.Members?.length > 0;

        // Check if has an IP assigned (in CONFIG)
        const hasIP =
          (iface.IPv4?.length > 0 && !iface.IPv4[0].startsWith("169.254")) ||
          iface.DHCP;

        // Check if assigned to a zone (in CONFIG)
        // Only check config for usage constraints
        const configIface = rawInterfaces.find(
          (i: any) => i.Name === iface.Name,
        );
        const hasZone = !!configIface?.Zone;

        const isAvailable = !inBond && !isBond && !hasIP && !hasZone;
        const usageReason = inBond
          ? "in bond (staged)"
          : isBond
            ? "is bond"
            : hasIP
              ? "has IP (staged)"
              : hasZone
                ? "in zone (staged)"
                : null;

        return {
          ...iface,
          isAvailable,
          usageReason,
        };
      }),
  );

  const availableInterfaces = $derived(
    hardwareInterfaces.filter((i: any) => i.isAvailable),
  );
  const hasAnyHardwareInterfaces = $derived(hardwareInterfaces.length > 0);

  // NOTE: isDegradedBond is derived from form state in BondCreateCard, not page state anymore

  function getZoneColor(zoneName: string): string {
    const zone = zones.find((z: any) => z.name === zoneName);
    return zone?.color || "gray";
  }

  function getInterfaceType(iface: any): string {
    if (iface.Name?.startsWith("bond")) return "bond";
    if (iface.Name?.includes(".")) return "vlan";
    if (iface.Name?.startsWith("wg")) return "wireguard";
    if (iface.Name?.startsWith("tun") || iface.Name?.startsWith("tap"))
      return "tunnel";
    return "ethernet";
  }

  const canCreateBond = $derived(
    interfaces.filter((i: any) => getInterfaceType(i) === "ethernet").length >=
      2,
  );

  async function handleSaveInterface(event: CustomEvent) {
    const data = event.detail;
    loading = true;
    try {
      await api.updateInterface({
        name: data.name,
        action: "update",
        description: data.description || undefined,
        ipv4: data.ipv4, // InterfaceCard already formats this to string[]
        dhcp: data.dhcp,
        mtu: data.mtu,
        disabled: !!data.disabled,
      });
      // Refresh status
      interfaceStatus = await api.getInterfaces();
    } catch (e: any) {
      alert($t("interfaces.update_failed") + `: ${e.message || e}`);
      console.error("Failed to update interface:", e);
    } finally {
      loading = false;
    }
  }

  function toggleAddVlan() {
    isAddingVlan = !isAddingVlan;
    isAddingBond = false;
  }

  function toggleAddBond() {
    isAddingBond = !isAddingBond;
    isAddingVlan = false;
  }

  async function handleCreateVlan(event: CustomEvent) {
    loading = true;
    const { parent_interface, vlan_id, zone, ipv4 } = event.detail;

    try {
      await api.createVlan({
        parent_interface,
        vlan_id,
        zone,
        ipv4,
      });
      isAddingVlan = false;
    } catch (e: any) {
      alert($t("interfaces.vlan_create_failed") + `: ${e.message || e}`);
      console.error("Failed to create VLAN:", e);
    } finally {
      loading = false;
    }
  }

  async function handleCreateBond(event: CustomEvent) {
    loading = true;
    const { name, zone, mode, interfaces } = event.detail;

    try {
      await api.createBond({
        name,
        zone,
        mode,
        interfaces,
      });
      isAddingBond = false;
    } catch (e: any) {
       // BondCreateCard displays error if it can, but here we just alert
      alert($t("interfaces.bond_create_failed") + `: ${e.message || e}`);
      console.error("Failed to create Bond:", e);
    } finally {
      loading = false;
    }
  }

  // Delete confirmation state
  let showDeleteModal = $state(false);
  let itemToDelete = $state<any>(null);

  async function requestDelete(event: CustomEvent) {
    itemToDelete = event.detail;
    showDeleteModal = true;
  }

  async function confirmDelete() {
    if (!itemToDelete) return;

    loading = true;
    const iface = itemToDelete;
    const type = getInterfaceType(iface);
    const name = iface.Name;

    try {
      if (type === "vlan") {
        await api.deleteVlan(name);
      } else if (type === "bond") {
        await api.deleteBond(name);
      } else if (type === "ethernet") {
        await api.updateInterface({ name, action: "delete" });
      } else {
        alert($t("interfaces.delete_not_supported"));
        return;
      }
      // Refresh interface status
      const res = await api.getInterfaces();
      interfaceStatus = (res.interfaces || []).map((s: any) => ({
        ...s,
        Name: s.name,
        State: s.state,
        IPv4Addrs: s.ipv4_addrs,
        IPv6Addrs: s.ipv6_addrs,
      }));
      showDeleteModal = false;
      itemToDelete = null;
    } catch (e: any) {
      const msg = e.message || e.toString();
      if (msg.includes("interface not in configuration")) {
        console.warn(
          "Interface already removed from configuration, refreshing list.",
        );
        // Treat as success - the goal was to remove it, and it's gone.
        const res = await api.getInterfaces();
        interfaceStatus = (res.interfaces || []).map((s: any) => ({
          ...s,
          Name: s.name,
          State: s.state,
          IPv4Addrs: s.ipv4_addrs,
          IPv6Addrs: s.ipv6_addrs,
        }));
        showDeleteModal = false;
        itemToDelete = null;
        return;
      }

      alert($t("interfaces.delete_failed") + `: ${msg}`);
      console.error("Failed to delete interface:", e);
    } finally {
      loading = false;
    }
  }
</script>

<div class="interfaces-page">
  <div class="page-header">
    <div class="header-actions">
      <Button variant={isAddingVlan ? "primary" : "outline"} onclick={toggleAddVlan}
        ><Icon name="add" size={16} /> {$t("common.add_item", {
          values: { item: $t("item.vlan") },
        })}</Button
      >
      {#if canCreateBond}
        <Button variant={isAddingBond ? "primary" : "outline"} onclick={toggleAddBond}
          ><Icon name="add" size={16} /> {$t("common.add_item", {
            values: { item: $t("item.bond") },
          })}</Button
        >
      {/if}
    </div>
  </div>

  {#if isAddingVlan}
      <div class="create-section mb-4">
          <VlanCreateCard
            {interfaces}
            {zones}
            {loading}
            on:save={handleCreateVlan}
            on:cancel={() => isAddingVlan = false}
          />
      </div>
  {/if}

  {#if isAddingBond}
      <div class="create-section mb-4">
          <BondCreateCard
            {zones}
            {hardwareInterfaces}
            {loading}
            on:save={handleCreateBond}
            on:cancel={() => isAddingBond = false}
          />
      </div>
  {/if}

  {#if loading && interfaces.length === 0}
    <div class="loading-state">
      <Spinner size="lg" />
      <p>{$t("common.loading")}</p>
    </div>
  {:else if interfaces.length === 0}
    <Card>
      <p class="empty-message">
        {$t("common.no_items", {
          values: { items: $t("item.interface") },
        })}
      </p>
    </Card>
  {:else}
    <div class="interfaces-grid">
      {#each interfaces as iface (iface.Name)}
        <div id="interface-{iface.Name}" style="width: 100%;">
          <InterfaceCard
            {iface}
            {zones}
            {loading}
            {hardwareInterfaces}
            memberOf={memberOfMap.get(iface.Name)}
            on:save={handleSaveInterface}
            on:delete={requestDelete}
          />
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- Delete Confirmation Modal (Only modal remaining) -->
<Modal bind:open={showDeleteModal} title={$t("common.confirm_delete")}>
  <div class="form-stack">
    <p>
      {#if itemToDelete}
        {$t("interfaces.delete_confirm", {
          values: {
            type: getInterfaceType(itemToDelete),
            name: itemToDelete.Name,
          },
        })}
      {:else}
        {$t("common.confirm_action")}
      {/if}
    </p>

    <div class="modal-actions">
      <Button variant="ghost" onclick={() => (showDeleteModal = false)}
        >{$t("common.cancel")}</Button
      >
      <Button variant="destructive" onclick={confirmDelete} disabled={loading}>
        {#if loading}<Spinner size="sm" />{/if}
        {$t("common.delete")}
      </Button>
    </div>
  </div>
</Modal>

<style>
  .interfaces-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    padding: var(--space-4);
  }

  .page-header {
    display: flex;
    align-items: center;
    justify-content: flex-end; /* Align buttons to right */
    gap: var(--space-4);
  }

  .header-actions {
    display: flex;
    gap: var(--space-2);
  }

  .interfaces-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: var(--space-4);
  }

  .empty-message {
    color: var(--color-muted);
    text-align: center;
    margin: 0;
  }

  .create-section {
      width: 100%;
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

  /* Zone color variables */
  :global(:root) {
    --zone-red: #dc2626;
    --zone-green: #16a34a;
    --zone-blue: #2563eb;
    --zone-orange: #ea580c;
    --zone-purple: #9333ea;
    --zone-cyan: #0891b2;
    --zone-gray: #6b7280;
  }

  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-4);
    padding: var(--space-12);
    color: var(--color-muted);
  }

  .mb-4 { margin-bottom: var(--space-4); }
</style>
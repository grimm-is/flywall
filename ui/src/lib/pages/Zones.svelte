<script lang="ts">
  /**
   * Zones Page
   * Network zone management
   */

  import { config, api, alertStore } from "$lib/stores/app";
  import {
    Button,
    Icon,
  } from "$lib/components";
  import ZoneCard from "$lib/components/ZoneCard.svelte";
  import ZoneCreateCard from "$lib/components/ZoneCreateCard.svelte";

  import { t } from "svelte-i18n";

  let loading = $state(false);
  let isAdding = $state(false);

  const zones = $derived($config?.zones || []);
  const interfaces = $derived($config?.interfaces || []);
  const ipsets = $derived($config?.ipsets || []);

  const availableInterfaceNames = $derived(
    interfaces.map((i: any) => i.Name).filter((n: string) => !n.includes(".")),
  );
  const availableIPSetNames = $derived(ipsets.map((s: any) => s.name));

  function getZoneInterfaces(zoneName: string): string[] {
    // Interfaces assigned via iface.Zone
    const fromIface = interfaces
      .filter((i: any) => i.Zone === zoneName)
      .map((i: any) => i.Name);

    // Interfaces assigned via zone.interfaces
    const zone = zones.find((z: any) => z.name === zoneName);
    const fromZone = zone?.interfaces || [];

    // Deduplicate
    return [...new Set([...fromIface, ...fromZone])];
  }

  function toggleAddZone() {
    isAdding = !isAdding;
  }

  async function handleAddZone(event: CustomEvent) {
    loading = true;
    try {
      const zoneData = event.detail;

      // Add new
      const updatedZones = [...zones, zoneData];

      await api.updateZones(updatedZones);
      isAdding = false;
    } catch (e: any) {
      console.error("Failed to add zone:", e);
      alertStore.error("Failed to add zone: " + e.message);
    } finally {
      loading = false;
    }
  }

  async function handleSaveZone(event: CustomEvent) {
    const updatedZone = event.detail;
    loading = true;
    try {
      const updatedZones = zones.map((z: any) =>
        z.name === updatedZone.name ? { ...z, ...updatedZone } : z,
      );
      await api.updateZones(updatedZones);
    } catch (e: any) {
      console.error("Failed to update zone:", e);
      alertStore.error("Failed to update zone: " + e.message);
    } finally {
      loading = false;
    }
  }

  async function handleDeleteZone(event: CustomEvent) {
    const zone = event.detail;
    if (getZoneInterfaces(zone.name).length > 0) {
      alertStore.error("Cannot delete zone with assigned interfaces");
      return;
    }

    if (
      !confirm($t("common.confirm_delete", { values: { item: zone.name } }))
    ) {
      return;
    }

    loading = true;
    try {
      const updatedZones = zones.filter((z: any) => z.name !== zone.name);
      await api.updateZones(updatedZones);
    } catch (e: any) {
      console.error("Failed to delete zone:", e);
      alertStore.error("Failed to delete zone: " + e.message);
    } finally {
      loading = false;
    }
  }
</script>

<div class="zones-page">
  <div class="page-header">
    <Button onclick={toggleAddZone} variant={isAdding ? "primary" : "outline"}
      ><Icon name="add" size={16} /> {$t("common.add_item", { values: { item: $t("item.zone") } })}</Button
    >
  </div>

  {#if isAdding}
    <div class="create-section mb-4">
      <ZoneCreateCard
        {availableInterfaceNames}
        {availableIPSetNames}
        {loading}
        on:save={handleAddZone}
        on:cancel={() => isAdding = false}
      />
    </div>
  {/if}

  <div class="zones-grid">
    {#each zones as zone}
      <div id="zone-{zone.name}" style="width: 100%;">
        <ZoneCard
          {zone}
          assignedInterfaces={getZoneInterfaces(zone.name)}
          {availableInterfaceNames}
          {availableIPSetNames}
          {loading}
          on:save={handleSaveZone}
          on:delete={handleDeleteZone}
        />
      </div>
    {/each}
  </div>
</div>

<style>
  .zones-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    padding: var(--space-4);
  }

  .page-header {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    margin-bottom: var(--space-4);
  }

  .zones-grid {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .mb-4 { margin-bottom: var(--space-4); }
</style>
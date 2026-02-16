<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { t } from "svelte-i18n";
  import {
    Card,
    Button,
    Input,
    Select,
    Badge,
    Icon,
    Spinner,
    Toggle,
  } from "$lib/components";
  import ZoneColorSelector from "$lib/components/ZoneColorSelector.svelte";
  import ZoneSelectorEditor from "$lib/components/ZoneSelectorEditor.svelte";
  import ServiceTileGrid from "$lib/components/ServiceTileGrid.svelte";
  import ZoneTypeSelector from "$lib/components/ZoneTypeSelector.svelte";
  import ObjectIcon from "$lib/components/ObjectIcon.svelte";

  export let zone: any;
  // List of interface names assigned to this zone
  export let assignedInterfaces: string[] = [];
  export let availableInterfaceNames: string[] = [];
  export let availableIPSetNames: string[] = [];
  export let loading = false;

  const dispatch = createEventDispatcher();

  let isEditing = false;

  // Edit form state
  let editName = "";
  let editColor = "blue";
  let editDescription = "";
  let editExternal = false;
  let editSelectors: any[] = [];

  // Management
  let management = {
    web: false,
    ssh: false,
    api: false,
    icmp: false,
  };

  // Services
  let services = {
    dhcp: false,
    dns: false,
    ntp: false,
  };

  function startEdit() {
    editName = zone.name;
    editColor = zone.color || "blue";
    editDescription = zone.description || "";
    editExternal = zone.external === true;
    editSelectors = zone.matches
      ? JSON.parse(JSON.stringify(zone.matches))
      : [];

    // Populate Management
    management = {
      web: zone.management?.web || false,
      ssh: zone.management?.ssh || false,
      api: zone.management?.api || false,
      icmp: zone.management?.icmp || false,
    };

    // Populate Services
    services = {
      dhcp: zone.services?.dhcp || false,
      dns: zone.services?.dns || false,
      ntp: zone.services?.ntp || false,
    };

    isEditing = true;
  }

  function cancelEdit() {
    isEditing = false;
  }

  function handleSave() {
    // Dispatch save event with updated zone data
    dispatch("save", {
      name: editName,
      color: editColor,
      description: editDescription,
      external: editExternal,
      matches: editSelectors,
      management,
      services,
    });
    isEditing = false;
  }
  function handleDelete() {
    dispatch("delete", zone);
  }
</script>

<Card>
  {#if isEditing}
    <div class="edit-form form-stack">
      <div class="edit-header">
        <h3>{$t("common.edit_item", { values: { item: zone.name } })}</h3>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <Input
          id={`zone-name-${zone.name}`}
          label={$t("zones.zone_name")}
          bind:value={editName}
          placeholder="e.g., Guest"
          required
          disabled={true}
        />

        <ZoneColorSelector bind:value={editColor} />
      </div>

      <Input
        id={`zone-desc-${zone.name}`}
        label={$t("common.description")}
        bind:value={editDescription}
        placeholder="e.g., Guest network for visitors"
      />

      <div class="selector-section">
        <h3 class="text-sm font-medium text-foreground mb-2">Selectors</h3>
        <ZoneSelectorEditor
          bind:matches={editSelectors}
          availableInterfaces={availableInterfaceNames}
          availableIPSets={availableIPSetNames}
        />
      </div>

      <div class="space-y-4">
        <h3 class="text-sm font-medium text-foreground">
          {$t("zones.zone_type")}
        </h3>
        <ZoneTypeSelector bind:value={editExternal} />
      </div>

      <div class="grid grid-cols-2 gap-6">
        <div class="space-y-3">
          <div class="flex flex-col gap-1">
            <h3 class="text-sm font-medium text-foreground">
              {$t("zones.management_access")}
            </h3>
            <span class="text-xs text-muted-foreground"
              >Applies only to traffic matching this zone</span
            >
          </div>
          <ServiceTileGrid
            type="management"
            on:change={(e) => (management = e.detail)}
            services={management}
          />
        </div>

        <div class="space-y-3">
          <div class="flex flex-col gap-1">
            <h3 class="text-sm font-medium text-foreground">
              {$t("zones.network_services")}
            </h3>
            <span class="text-xs text-muted-foreground"
              >Applies only to traffic matching this zone</span
            >
          </div>
          <ServiceTileGrid
            type="network"
            on:change={(e) => (services = e.detail)}
            {services}
          />
        </div>
      </div>

      <div class="edit-actions">
        <Button variant="ghost" onclick={cancelEdit} disabled={loading}
          >{$t("common.cancel")}</Button
        >
        <Button onclick={handleSave} disabled={loading}>
          {#if loading}<Spinner size="sm" />{/if}
          {$t("common.save")}
        </Button>
      </div>
    </div>
  {:else}
    <div class="zone-header">
      <div class="flex items-center gap-2">
        <div
          class="zone-badge"
          style={zone.color?.startsWith("#")
            ? `--zone-color: ${zone.color}`
            : `--zone-color: var(--zone-${zone.color}, var(--color-primary))`}
        >
          {zone.name}
        </div>
        {#if zone.external}
          <Badge variant="secondary">{$t("zones.external")}</Badge>
        {/if}
      </div>
      <div class="zone-actions">
        <Button variant="ghost" size="sm" onclick={startEdit}
          ><Icon name="edit" size="sm" /></Button
        >
        <Button variant="ghost" size="sm" onclick={handleDelete}
          ><Icon name="delete" size="sm" /></Button
        >
      </div>
    </div>

    {#if zone.description}
      <p class="zone-description">{zone.description}</p>
    {/if}

    <div class="zone-details">
      <div class="detail-section">
        <span class="detail-label">{$t("zones.selectors")}</span>
        <div class="detail-tags">
          {#if zone.matches && zone.matches.length > 0}
            {#each zone.matches as m}
              <div class="flex flex-wrap gap-1">
                {#if m.interface}<Badge variant="outline" class="gap-1 pl-1"
                    ><ObjectIcon
                      type="interface"
                      name={m.interface}
                    />{m.interface}</Badge
                  >{/if}
                {#if m.src}<Badge variant="outline" class="gap-1 pl-1"
                    ><ObjectIcon type="ip" />{m.src}</Badge
                  >{/if}
                {#if m.dst}<Badge variant="outline" class="gap-1 pl-1"
                    ><ObjectIcon type="ip" />→ {m.dst}</Badge
                  >{/if}
                {#if m.protocol}<Badge variant="outline" class="gap-1 pl-1"
                    ><ObjectIcon type="protocol" />{m.protocol}</Badge
                  >{/if}
                {#if !m.interface && !m.src && !m.dst && !m.protocol}
                  <Badge variant="outline">Match Rule</Badge>
                {/if}
              </div>
            {/each}
          {:else if assignedInterfaces.length > 0}
            {#each assignedInterfaces as iface}
              <Badge variant="outline" class="gap-1 pl-1"
                ><ObjectIcon type="interface" name={iface} />{iface}</Badge
              >
            {/each}
          {:else}
            <span class="text-sm text-muted-foreground italic"
              >{$t("zones.none_assigned")}</span
            >
          {/if}
        </div>
      </div>

      {#if zone.management && Object.values(zone.management).some(Boolean)}
        <div class="detail-section">
          <span class="detail-label">{$t("zones.allow")}</span>
          <div class="detail-tags">
            {#if zone.management.web}<Badge variant="secondary"
                >{$t("zones.svc.web")}</Badge
              >{/if}
            {#if zone.management.ssh}<Badge variant="secondary"
                >{$t("zones.svc.ssh")}</Badge
              >{/if}
            {#if zone.management.api}<Badge variant="secondary"
                >{$t("zones.svc.api")}</Badge
              >{/if}
            {#if zone.management.icmp}<Badge variant="secondary"
                >{$t("zones.svc.icmp")}</Badge
              >{/if}
          </div>
        </div>
      {/if}

      {#if zone.services && Object.values(zone.services).some(Boolean)}
        <div class="detail-section">
          <span class="detail-label">{$t("zones.services")}</span>
          <div class="detail-tags">
            {#if zone.services.dhcp}<Badge variant="secondary"
                >{$t("zones.svc.dhcp")}</Badge
              >{/if}
            {#if zone.services.dns}<Badge variant="secondary"
                >{$t("zones.svc.dns")}</Badge
              >{/if}
            {#if zone.services.ntp}<Badge variant="secondary"
                >{$t("zones.svc.ntp")}</Badge
              >{/if}
          </div>
        </div>
      {/if}
    </div>
  {/if}
</Card>

<style>
  .zone-header,
  .edit-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-3);
  }

  .edit-header h3 {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: 600;
  }

  .zone-badge {
    display: inline-flex;
    padding: var(--space-1) var(--space-3);
    /* Dynamic color handling now done partly in style attribute, fallback here */
    background-color: var(--zone-color, var(--color-primary));
    color: white;
    font-weight: 600;
    font-size: var(--text-sm);
    border-radius: var(--radius-md);
  }

  .zone-actions,
  .edit-actions {
    display: flex;
    gap: var(--space-1);
  }

  .edit-actions {
    justify-content: flex-end;
    margin-top: var(--space-4);
  }

  .zone-description {
    color: var(--color-muted);
    font-size: var(--text-sm);
    margin: 0 0 var(--space-3) 0;
  }

  .zone-details {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    margin-top: var(--space-3);
    padding-top: var(--space-3);
    border-top: 1px solid var(--color-border);
  }

  .detail-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .detail-label {
    font-size: var(--text-xs);
    font-weight: 500;
    color: var(--color-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .detail-tags {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  .form-stack {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
</style>

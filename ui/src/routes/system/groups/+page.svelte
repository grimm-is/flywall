<script lang="ts">
  import { groups, api } from "$lib/stores/app";
  import Card from "$lib/components/Card.svelte";
  import Table from "$lib/components/Table.svelte";
  import Button from "$lib/components/Button.svelte";
  import Modal from "$lib/components/Modal.svelte";
  import Input from "$lib/components/Input.svelte";
  import Icon from "$lib/components/Icon.svelte";
  import Badge from "$lib/components/Badge.svelte";

  // --- State ---
  let showEditModal = false;
  let editingGroup: any = null; // { id, name, description, tags, icon, color, schedule: { enabled, blocks: [] } }

  // --- Form State ---
  let formName = "";
  let formDescription = "";
  let formColor = "blue";
  let formScheduleEnabled = false;
  let formBlocks: any[] = []; // { start_time, end_time, days: [] }

  const colors = ["blue", "green", "red", "yellow", "purple", "gray"];
  const daysOfWeek = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

  // --- Actions ---
  function openCreate() {
    editingGroup = null;
    formName = "";
    formDescription = "";
    formColor = "blue";
    formScheduleEnabled = false;
    formBlocks = [];
    showEditModal = true;
  }

  function openEdit(group: any) {
    editingGroup = group;
    formName = group.name;
    formDescription = group.description;
    formColor = group.color || "blue";
    formScheduleEnabled = group.schedule?.enabled || false;
    // Deep copy blocks
    formBlocks = (group.schedule?.blocks || []).map((b: any) => ({
      start_time: b.start_time,
      end_time: b.end_time,
      days: [...(b.days || [])],
    }));
    showEditModal = true;
  }

  async function saveGroup() {
    const payload = {
      id: editingGroup?.id, // undefined for create, handled by backend/store? Store expects empty for create?
      name: formName,
      description: formDescription,
      color: formColor,
      schedule: {
        enabled: formScheduleEnabled,
        blocks: formBlocks,
      },
    };

    try {
      if (editingGroup) {
        await api.updateGroup(payload);
      } else {
        await api.createGroup(payload);
      }
      showEditModal = false;
    } catch (e) {
      console.error(e);
      alert("Failed to save group");
    }
  }

  async function deleteGroup(id: string) {
    if (
      !confirm(
        "Are you sure you want to delete this group? Devices will be ungrouped.",
      )
    )
      return;
    try {
      await api.deleteGroup(id);
    } catch (e) {
      console.error(e);
      alert("Failed to delete group");
    }
  }

  // --- Schedule Helpers ---
  function addBlock() {
    formBlocks = [
      ...formBlocks,
      {
        start_time: "09:00",
        end_time: "17:00",
        days: ["Mon", "Tue", "Wed", "Thu", "Fri"],
      },
    ];
  }

  function removeBlock(i: number) {
    formBlocks = formBlocks.filter((_, idx) => idx !== i);
  }

  function toggleDay(blockIndex: number, day: string) {
    const block = formBlocks[blockIndex];
    const hasDay = block.days.includes(day);
    if (hasDay) {
      block.days = block.days.filter((d: string) => d !== day);
    } else {
      block.days = [...block.days, day];
    }
    formBlocks = [...formBlocks]; // Trigger reactivity
  }
</script>

<div class="groups-page">
  <header class="page-header">
    <h1>Device Groups</h1>
    <Button variant="default" onclick={openCreate}>
      <Icon name="add" size={16} /> New Group
    </Button>
  </header>

  <Card>
    <Table
      columns={[
        { label: "Group", key: "name", width: "30%" },
        { label: "Description", key: "description", width: "40%" },
        { label: "Schedule", key: "schedule", width: "20%" },
        { label: "", key: "actions", width: "10%" },
      ]}
      data={$groups}
    >
      {#snippet children(row: any, i: number)}
        <td class="col-name">
          <div class="flex items-center gap-2">
            <div class="color-dot bg-{row.color || 'blue'}"></div>
            <span class="font-medium">{row.name}</span>
          </div>
        </td>

        <td class="col-desc text-muted">
          {row.description || "-"}
        </td>

        <td class="col-schedule">
          {#if row.schedule?.enabled}
            <Badge variant="success">Active</Badge>
            <span class="text-xs text-muted ml-2">
              {row.schedule.blocks?.length || 0} blocks
            </span>
          {:else}
            <span class="text-muted text-sm">Disabled</span>
          {/if}
        </td>

        <td class="col-actions">
          <div class="flex gap-2 justify-end">
            <Button size="sm" variant="outline" onclick={() => openEdit(row)}
              >Edit</Button
            >
            <Button
              size="sm"
              variant="destructive"
              onclick={() => deleteGroup(row.id)}
            >
              <Icon name="delete" size={14} />
            </Button>
          </div>
        </td>
      {/snippet}
    </Table>
  </Card>
</div>

<!-- Edit Modal -->
{#if showEditModal}
  <Modal
    title={editingGroup ? "Edit Group" : "New Group"}
    bind:open={showEditModal}
  >
    <div class="form-grid">
      <!-- Basic Info -->
      <div class="form-group">
        <label for="name">Group Name</label>
        <Input
          id="name"
          bind:value={formName}
          placeholder="e.g. Kids Devices"
        />
      </div>

      <div class="form-group">
        <label for="desc">Description</label>
        <Input
          id="desc"
          bind:value={formDescription}
          placeholder="Optional description"
        />
      </div>

      <div class="form-group">
        <label>Color Tag</label>
        <div class="color-picker">
          {#each colors as c}
            <button
              class="color-btn bg-{c}"
              class:selected={formColor === c}
              onclick={() => (formColor = c)}
              aria-label="Select {c}"
            ></button>
          {/each}
        </div>
      </div>

      <hr class="divider" />

      <!-- Schedule -->
      <div class="schedule-section">
        <div class="flex justify-between items-center mb-2">
          <h3 class="text-sm font-medium">Internet Access Schedule</h3>
          <label class="flex items-center gap-2 text-sm cursor-pointer">
            <input type="checkbox" bind:checked={formScheduleEnabled} />
            Enable Schedule
          </label>
        </div>

        {#if formScheduleEnabled}
          <div class="blocks-list">
            {#each formBlocks as block, i}
              <div class="schedule-block">
                <div class="block-header">
                  <span class="text-xs font-medium text-muted"
                    >Block {i + 1} (Downtime)</span
                  >
                  <button class="remove-btn" onclick={() => removeBlock(i)}
                    >Remove</button
                  >
                </div>

                <div class="time-range">
                  <div class="time-input">
                    <label>Start</label>
                    <input type="time" bind:value={block.start_time} />
                  </div>
                  <div class="time-input">
                    <label>End</label>
                    <input type="time" bind:value={block.end_time} />
                  </div>
                </div>

                <div class="days-selector">
                  {#each daysOfWeek as day}
                    <button
                      class="day-btn"
                      class:active={block.days.includes(day)}
                      onclick={() => toggleDay(i, day)}
                    >
                      {day[0]}
                    </button>
                  {/each}
                </div>
              </div>
            {/each}

            <Button
              size="sm"
              variant="outline"
              onclick={addBlock}
              class="w-full dashed"
            >
              + Add Time Block
            </Button>
            <p class="text-xs text-muted mt-2">
              During these times, internet access will be blocked for devices in
              this group.
            </p>
          </div>
        {:else}
          <div class="text-sm text-muted italic p-2 bg-surface rounded">
            Schedule is disabled. Internet access allows 24/7.
          </div>
        {/if}
      </div>

      <div class="modal-footer">
        <Button variant="ghost" onclick={() => (showEditModal = false)}
          >Cancel</Button
        >
        <Button variant="default" onclick={saveGroup}>Save Group</Button>
      </div>
    </div>
  </Modal>
{/if}

<style>
  .groups-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding: var(--space-4);
  }

  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .color-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }

  /* Colors */
  .bg-blue {
    background-color: #3b82f6;
  }
  .bg-green {
    background-color: #22c55e;
  }
  .bg-red {
    background-color: #ef4444;
  }
  .bg-yellow {
    background-color: #eab308;
  }
  .bg-purple {
    background-color: #a855f7;
  }
  .bg-gray {
    background-color: #6b7280;
  }

  /* Form */
  .form-grid {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .form-group label {
    font-size: var(--text-sm);
    font-weight: 500;
    color: var(--dashboard-text);
  }

  /* Color Picker */
  .color-picker {
    display: flex;
    gap: var(--space-2);
  }

  .color-btn {
    width: 24px;
    height: 24px;
    border-radius: 50%;
    border: 2px solid transparent;
    cursor: pointer;
    transition: transform 0.1s;
  }

  .color-btn:hover {
    transform: scale(1.1);
  }

  .color-btn.selected {
    border-color: var(--color-foreground);
    box-shadow: 0 0 0 2px var(--color-background);
  }

  .divider {
    border: 0;
    border-top: 1px solid var(--color-border);
  }

  /* Schedule */
  .schedule-block {
    background: var(--color-backgroundSecondary);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    margin-bottom: var(--space-3);
    border: 1px solid var(--color-border);
  }

  .block-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: var(--space-2);
  }

  .remove-btn {
    font-size: var(--text-xs);
    color: var(--color-destructive);
    background: none;
    border: none;
    cursor: pointer;
  }

  .time-range {
    display: flex;
    gap: var(--space-4);
    margin-bottom: var(--space-2);
  }

  .time-input {
    display: flex;
    flex-direction: column;
    flex: 1;
  }

  .time-input label {
    font-size: var(--text-xs);
    color: var(--dashboard-text-muted);
  }

  .time-input input {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: var(--space-1);
    font-family: monospace;
  }

  .days-selector {
    display: flex;
    justify-content: space-between;
    gap: 2px;
  }

  .day-btn {
    flex: 1;
    padding: 4px;
    font-size: var(--text-xs);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    cursor: pointer;
    color: var(--color-muted);
  }

  .day-btn.active {
    background: var(--color-primary);
    color: white;
    border-color: var(--color-primary);
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    margin-top: var(--space-2);
  }

  .flex {
    display: flex;
  }
  .gap-2 {
    gap: var(--space-2);
  }
  .items-center {
    align-items: center;
  }
  .justify-between {
    justify-content: space-between;
  }
  .font-medium {
    font-weight: 500;
  }
  .text-sm {
    font-size: var(--text-sm);
  }
  .text-xs {
    font-size: 11px;
  }
  .text-muted {
    color: var(--color-muted);
  }
  .ml-2 {
    margin-left: var(--space-2);
  }
  .mb-2 {
    margin-bottom: var(--space-2);
  }
  .mt-2 {
    margin-top: var(--space-2);
  }
  .cursor-pointer {
    cursor: pointer;
  }
</style>

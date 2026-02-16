<script lang="ts">
  import { groups, api } from "$lib/stores/app";
  import Card from "$lib/components/Card.svelte";
  import Button from "$lib/components/Button.svelte";
  import Input from "$lib/components/Input.svelte";
  import Icon from "$lib/components/Icon.svelte";
  import Badge from "$lib/components/Badge.svelte";
  import Select from "$lib/components/Select.svelte";

  // --- State ---
  let loading = false;
  let editingId: string | null = null; // ID of group being edited, or null
  let isAdding = false; // "Add New" mode

  // --- Form State ---
  let formName = "";
  let formDescription = "";
  let formColor = "blue";
  let formScheduleEnabled = false;
  let formBlocks: any[] = []; // { start_time, end_time, days: [] }

  const colors = ["blue", "green", "red", "yellow", "purple", "gray"];
  const daysOfWeek = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

  // --- Actions ---
  function startAdd() {
    isAdding = true;
    editingId = null;
    resetForm();
  }

  function startEdit(group: any) {
    if (isAdding) isAdding = false; // Cancel add if starting edit
    editingId = group.id;
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
  }

  function cancelEdit() {
    editingId = null;
    isAdding = false;
    resetForm();
  }

  function resetForm() {
    formName = "";
    formDescription = "";
    formColor = "blue";
    formScheduleEnabled = false;
    formBlocks = [];
  }

  async function saveGroup() {
    // Validate
    if (!formName) return;

    const payload = {
      id: editingId, // undefined for create
      name: formName,
      description: formDescription,
      color: formColor,
      schedule: {
        enabled: formScheduleEnabled,
        blocks: formBlocks,
      },
    };

    loading = true;
    try {
      if (editingId) {
        await api.updateGroup(payload);
      } else {
        await api.createGroup(payload);
      }
      cancelEdit();
    } catch (e) {
      console.error(e);
      alert("Failed to save group");
    } finally {
      loading = false;
    }
  }

  async function deleteGroup(id: string) {
    if (
      !confirm(
        "Are you sure you want to delete this group? Devices will be ungrouped.",
      )
    )
      return;

    loading = true;
    try {
      await api.deleteGroup(id);
    } catch (e) {
      console.error(e);
      alert("Failed to delete group");
    } finally {
      loading = false;
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
    {#if !isAdding}
      <Button variant="default" onclick={startAdd} disabled={!!editingId}>
        <Icon name="add" size={16} /> New Group
      </Button>
    {/if}
  </header>

  <Card class="table-card">
    <div class="table-container">
      <table class="table">
        <thead>
          <tr>
            <th style="width: 30%">Group</th>
            <th style="width: 40%">Description</th>
            <th style="width: 20%">Schedule</th>
            <th style="width: 10%"></th>
          </tr>
        </thead>
        <tbody>
          <!-- Add Form Row -->
          {#if isAdding}
            <tr class="editing-row">
              <td colspan="4">
                <div class="inline-form">
                  <div class="form-header">
                    <h3>New Group</h3>
                  </div>

                  <div class="form-grid">
                    <!-- Basic Info Row -->
                    <div class="form-row">
                      <div class="field-group flex-1">
                        <label for="new-name">Name</label>
                        <Input
                          id="new-name"
                          bind:value={formName}
                          placeholder="Name"
                        />
                      </div>
                      <div class="field-group flex-1">
                        <label for="new-desc">Description</label>
                        <Input
                          id="new-desc"
                          bind:value={formDescription}
                          placeholder="Description"
                        />
                      </div>
                      <div class="field-group">
                        <label>Color</label>
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
                    </div>

                    <!-- Schedule Toggle -->
                    <div class="form-row">
                      <label class="checkbox-label">
                        <input
                          type="checkbox"
                          bind:checked={formScheduleEnabled}
                        />
                        Enable Internet Access Schedule
                      </label>
                    </div>

                    <!-- Schedule Blocks -->
                    {#if formScheduleEnabled}
                      <div class="schedule-editor">
                        {#each formBlocks as block, i}
                          <div class="schedule-block-inline">
                            <div class="time-inputs">
                              <input
                                type="time"
                                bind:value={block.start_time}
                              />
                              <span>to</span>
                              <input type="time" bind:value={block.end_time} />
                            </div>
                            <div class="day-toggles">
                              {#each daysOfWeek as day}
                                <button
                                  class="day-btn-mini"
                                  class:active={block.days.includes(day)}
                                  onclick={() => toggleDay(i, day)}
                                  >{day[0]}</button
                                >
                              {/each}
                            </div>
                            <button
                              class="icon-btn"
                              onclick={() => removeBlock(i)}
                            >
                              <Icon name="close" size={14} />
                            </button>
                          </div>
                        {/each}
                        <Button size="sm" variant="outline" onclick={addBlock}
                          >+ Add Block</Button
                        >
                      </div>
                    {/if}

                    <div class="form-actions">
                      <Button
                        variant="ghost"
                        onclick={cancelEdit}
                        disabled={loading}>Cancel</Button
                      >
                      <Button
                        variant="default"
                        onclick={saveGroup}
                        disabled={loading || !formName}>Save</Button
                      >
                    </div>
                  </div>
                </div>
              </td>
            </tr>
          {/if}

          {#each $groups as group (group.id)}
            {#if editingId === group.id}
              <!-- Edit Mode Row -->
              <tr class="editing-row">
                <td colspan="4">
                  <div class="inline-form">
                    <div class="form-grid">
                      <!-- Basic Info Row -->
                      <div class="form-row">
                        <div class="field-group flex-1">
                          <label for="edit-name">Name</label>
                          <Input
                            id="edit-name"
                            bind:value={formName}
                            placeholder="Name"
                          />
                        </div>
                        <div class="field-group flex-1">
                          <label for="edit-desc">Description</label>
                          <Input
                            id="edit-desc"
                            bind:value={formDescription}
                            placeholder="Description"
                          />
                        </div>
                        <div class="field-group">
                          <label>Color</label>
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
                      </div>

                      <!-- Schedule Toggle -->
                      <div class="form-row">
                        <label class="checkbox-label">
                          <input
                            type="checkbox"
                            bind:checked={formScheduleEnabled}
                          />
                          Enable Internet Access Schedule
                        </label>
                      </div>

                      <!-- Schedule Blocks -->
                      {#if formScheduleEnabled}
                        <div class="schedule-editor">
                          {#each formBlocks as block, i}
                            <div class="schedule-block-inline">
                              <div class="time-inputs">
                                <input
                                  type="time"
                                  bind:value={block.start_time}
                                />
                                <span>to</span>
                                <input
                                  type="time"
                                  bind:value={block.end_time}
                                />
                              </div>
                              <div class="day-toggles">
                                {#each daysOfWeek as day}
                                  <button
                                    class="day-btn-mini"
                                    class:active={block.days.includes(day)}
                                    onclick={() => toggleDay(i, day)}
                                    >{day[0]}</button
                                  >
                                {/each}
                              </div>
                              <button
                                class="icon-btn"
                                onclick={() => removeBlock(i)}
                              >
                                <Icon name="close" size={14} />
                              </button>
                            </div>
                          {/each}
                          <Button size="sm" variant="outline" onclick={addBlock}
                            >+ Add Block</Button
                          >
                        </div>
                      {/if}

                      <div class="form-actions">
                        <Button
                          variant="ghost"
                          onclick={cancelEdit}
                          disabled={loading}>Cancel</Button
                        >
                        <Button
                          variant="default"
                          onclick={saveGroup}
                          disabled={loading || !formName}>Save</Button
                        >
                      </div>
                    </div>
                  </div>
                </td>
              </tr>
            {:else}
              <!-- View Mode Row -->
              <tr class="view-row">
                <td class="col-name">
                  <div class="flex items-center gap-2">
                    <div class="color-dot bg-{group.color || 'blue'}"></div>
                    <span class="font-medium">{group.name}</span>
                  </div>
                </td>
                <td class="col-desc text-muted">
                  {group.description || "-"}
                </td>
                <td class="col-schedule">
                  {#if group.schedule?.enabled}
                    <Badge variant="success">Active</Badge>
                    <span class="text-xs text-muted ml-2">
                      {group.schedule.blocks?.length || 0} blocks
                    </span>
                  {:else}
                    <span class="text-muted text-sm">Disabled</span>
                  {/if}
                </td>
                <td class="col-actions">
                  <div class="flex gap-2 justify-end">
                    <Button
                      size="sm"
                      variant="outline"
                      onclick={() => startEdit(group)}
                      disabled={isAdding ||
                        (editingId !== null && editingId !== group.id)}
                    >
                      Edit
                    </Button>
                    <Button
                      size="sm"
                      variant="destructive"
                      onclick={() => deleteGroup(group.id)}
                      disabled={isAdding || editingId !== null}
                    >
                      <Icon name="delete" size={14} />
                    </Button>
                  </div>
                </td>
              </tr>
            {/if}
          {/each}

          {#if $groups.length === 0 && !isAdding}
            <tr>
              <td colspan="4" class="empty-state"> No groups found. </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>
  </Card>
</div>

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

  .table-container {
    overflow-x: auto;
  }

  .table {
    width: 100%;
    border-collapse: collapse;
  }

  th {
    text-align: left;
    padding: var(--space-3);
    border-bottom: 2px solid var(--color-border);
    color: var(--color-muted);
    font-weight: 500;
    font-size: var(--text-sm);
  }

  td {
    padding: var(--space-3);
    border-bottom: 1px solid var(--color-border);
    vertical-align: middle;
  }

  .view-row:hover td {
    background: var(--color-surfaceHover);
  }

  .editing-row td {
    background: var(--color-surface);
    padding: var(--space-4);
  }

  .inline-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .form-header h3 {
    font-size: var(--text-md);
    font-weight: 600;
    margin: 0;
  }

  .form-grid {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .form-row {
    display: flex;
    gap: var(--space-4);
    align-items: center;
    flex-wrap: wrap;
  }

  .field-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .field-group label {
    font-size: var(--text-xs);
    font-weight: 500;
    color: var(--color-muted);
  }

  .flex-1 {
    flex: 1;
  }

  /* Color Picker */
  .color-picker {
    display: flex;
    gap: var(--space-2);
    align-items: center;
    height: 38px; /* Match input height roughly */
  }

  .color-btn {
    width: 20px;
    height: 20px;
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

  .color-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }

  /* Schedule */
  .checkbox-label {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    cursor: pointer;
    user-select: none;
  }

  .schedule-editor {
    background: var(--color-backgroundSecondary);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .schedule-block-inline {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    background: var(--color-surface);
    padding: var(--space-2);
    border-radius: var(--radius-sm);
    border: 1px solid var(--color-border);
  }

  .time-inputs {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
  }

  .time-inputs input {
    padding: 2px 4px;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
  }

  .day-toggles {
    display: flex;
    gap: 2px;
  }

  .day-btn-mini {
    font-size: 10px;
    padding: 2px 6px;
    border: 1px solid var(--color-border);
    background: var(--color-background);
    cursor: pointer;
  }

  .day-btn-mini.active {
    background: var(--color-primary);
    color: white;
    border-color: var(--color-primary);
  }

  .icon-btn {
    background: none;
    border: none;
    cursor: pointer;
    color: var(--color-muted);
    display: flex;
    align-items: center;
  }
  .icon-btn:hover {
    color: var(--color-destructive);
  }

  .form-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    margin-top: var(--space-2);
  }

  /* Helpers */
  .flex {
    display: flex;
  }
  .gap-2 {
    gap: var(--space-2);
  }
  .items-center {
    align-items: center;
  }
  .justify-end {
    justify-content: flex-end;
  }
  .font-medium {
    font-weight: 500;
  }
  .text-muted {
    color: var(--color-muted);
  }
  .text-sm {
    font-size: var(--text-sm);
  }
  .text-xs {
    font-size: 11px;
  }
  .ml-2 {
    margin-left: var(--space-2);
  }
  .empty-state {
    text-align: center;
    padding: var(--space-8);
    color: var(--color-muted);
  }
</style>

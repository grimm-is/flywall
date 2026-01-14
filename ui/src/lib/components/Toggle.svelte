<script lang="ts">
  /**
   * Toggle Switch Component
   * A visual toggle switch for boolean on/off settings
   */

  interface Props {
    checked?: boolean;
    label?: string;
    disabled?: boolean;
    onchange?: (checked: boolean) => void;
  }

  let {
    checked = $bindable(false),
    label = "",
    disabled = false,
    onchange,
  }: Props = $props();

  function handleToggle(e: Event) {
    if (disabled) return;
    e.preventDefault();
    const oldVal = checked;
    checked = !checked;
    console.log(`[TOGGLE] handleToggle: ${label} ${oldVal} -> ${checked}`);
    onchange?.(checked);
  }
</script>

<div class="toggle-container" class:disabled>
  {#if label}
    <span class="toggle-label">{label}</span>
  {/if}
  <button
    type="button"
    class="toggle-switch"
    role="switch"
    aria-checked={checked}
    aria-label={label}
    {disabled}
    onclick={handleToggle}
  >
    <input
      type="checkbox"
      {checked}
      {disabled}
      tabindex="-1"
      style="display: none"
    />
    <span class="toggle-slider"></span>
  </button>
</div>

<style>
  .toggle-container {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-1) 0;
  }

  .toggle-container.disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .toggle-label {
    font-size: var(--text-sm);
    color: var(--color-foreground);
  }

  .toggle-switch {
    position: relative;
    display: inline-block;
    width: 44px;
    height: 24px;
    flex-shrink: 0;
    border: none;
    background: none;
    padding: 0;
    cursor: pointer;
    outline: none;
  }

  .toggle-switch:focus-visible .toggle-slider {
    box-shadow:
      0 0 0 2px var(--color-background),
      0 0 0 4px var(--color-primary);
  }

  .toggle-slider {
    position: absolute;
    cursor: pointer;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: var(--color-muted);
    transition: 0.2s;
    border-radius: 24px;
  }

  .toggle-slider:before {
    position: absolute;
    content: "";
    height: 18px;
    width: 18px;
    left: 3px;
    bottom: 3px;
    background-color: white;
    transition: 0.2s;
    border-radius: 50%;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  }

  .toggle-switch[aria-checked="true"] .toggle-slider {
    background-color: var(--color-success, #16a34a);
  }

  .toggle-switch[aria-checked="true"] .toggle-slider:before {
    transform: translateX(20px);
  }

  .toggle-container.disabled .toggle-slider {
    cursor: not-allowed;
  }
</style>

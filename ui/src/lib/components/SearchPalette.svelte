<script lang="ts">
  import { createEventDispatcher, onMount } from "svelte";
  import Icon from "./Icon.svelte";
  import { goto } from "$app/navigation";

  let { onclose }: { onclose: () => void } = $props();

  let searchQuery = $state("");
  let selectedIndex = $state(0);
  let inputEl: HTMLInputElement;

  // Command actions
  const commands = [
    // Navigation
    {
      id: "goto-topology",
      category: "Navigate",
      label: "Go to Topology",
      icon: "hub",
      action: () => goto("/"),
    },
    {
      id: "goto-policy",
      category: "Navigate",
      label: "Go to Policy",
      icon: "shield",
      action: () => goto("/policy"),
    },
    {
      id: "goto-observatory",
      category: "Navigate",
      label: "Go to Observatory",
      icon: "monitoring",
      action: () => goto("/observatory"),
    },
    {
      id: "goto-tunnels",
      category: "Navigate",
      label: "Go to Tunnels",
      icon: "vpn_key",
      action: () => goto("/tunnels"),
    },
    {
      id: "goto-system",
      category: "Navigate",
      label: "Go to System",
      icon: "settings",
      action: () => goto("/system"),
    },
    // Actions
    {
      id: "action-reboot",
      category: "Actions",
      label: "Reboot Router",
      icon: "restart_alt",
      action: () => alert("Reboot..."),
    },
    {
      id: "action-backup",
      category: "Actions",
      label: "Create Backup",
      icon: "backup",
      action: () => alert("Backup..."),
    },
    {
      id: "action-block-ip",
      category: "Actions",
      label: "Block IP Address...",
      icon: "block",
      action: () => {
        const ip = prompt("Enter IP address to block:");
        if (ip && ip.trim()) {
          goto(`/policy?action=block&ip=${encodeURIComponent(ip.trim())}`);
        }
      },
    },
    {
      id: "action-capture",
      category: "Actions",
      label: "Start Packet Capture...",
      icon: "radio_button_checked",
      action: () => goto("/observatory?capture=true"),
    },
    {
      id: "action-simulate",
      category: "Actions",
      label: "Packet Simulator...",
      icon: "science",
      action: () => {
        if (typeof window !== "undefined") {
          window.dispatchEvent(new CustomEvent("open-packet-simulator"));
        }
      },
    },
    // Quick access
    {
      id: "quick-lan",
      category: "Zones",
      label: "Show LAN clients",
      icon: "devices",
      action: () => goto("/?zone=lan"),
    },
    {
      id: "quick-wan",
      category: "Zones",
      label: "Show WAN status",
      icon: "public",
      action: () => goto("/?zone=wan"),
    },
    {
      id: "quick-containers",
      category: "Zones",
      label: "Show Containers",
      icon: "deployed_code",
      action: () => goto("/?filter=containers"),
    },
    {
      id: "quick-flows",
      category: "Network",
      label: "Active Connections",
      icon: "swap_calls",
      action: () => goto("/observatory?tab=flows"),
    },
    {
      id: "quick-logs",
      category: "Network",
      label: "Live Logs",
      icon: "terminal",
      action: () => goto("/observatory?tab=logs"),
    },
  ];

  // Filter commands by search
  let filteredCommands = $derived(() => {
    if (!searchQuery.trim()) return commands;
    const q = searchQuery.toLowerCase();
    return commands.filter(
      (c) =>
        c.label.toLowerCase().includes(q) ||
        c.category.toLowerCase().includes(q),
    );
  });

  // Group by category
  let groupedCommands = $derived(() => {
    const groups: Record<string, typeof commands> = {};
    for (const cmd of filteredCommands()) {
      if (!groups[cmd.category]) groups[cmd.category] = [];
      groups[cmd.category].push(cmd);
    }
    return groups;
  });

  // Flat list for keyboard nav
  let flatList = $derived(() => filteredCommands());

  function handleKeydown(e: KeyboardEvent) {
    const list = flatList();
    if (e.key === "ArrowDown") {
      e.preventDefault();
      selectedIndex = Math.min(selectedIndex + 1, list.length - 1);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      selectedIndex = Math.max(selectedIndex - 1, 0);
    } else if (e.key === "Enter" && list[selectedIndex]) {
      e.preventDefault();
      executeCommand(list[selectedIndex]);
    } else if (e.key === "Escape") {
      onclose();
    }
  }

  function executeCommand(cmd: (typeof commands)[0]) {
    cmd.action();
    onclose();
  }

  onMount(() => {
    inputEl?.focus();
  });
</script>

<div
  class="palette-overlay"
  onclick={onclose}
  onkeydown={handleKeydown}
  role="presentation"
>
  <div
    class="palette-container"
    onclick={(e) => e.stopPropagation()}
    onkeydown={handleKeydown}
    role="dialog"
    aria-modal="true"
    aria-label="Command palette"
    tabindex="-1"
  >
    <div class="palette-header">
      <Icon name="search" size={18} />
      <input
        type="text"
        bind:this={inputEl}
        bind:value={searchQuery}
        placeholder="Type a command..."
        class="palette-input"
        onkeydown={handleKeydown}
      />
      <kbd class="palette-kbd">esc</kbd>
    </div>

    <div class="palette-results">
      {#each Object.entries(groupedCommands()) as [category, cmds], gi}
        <div class="palette-group">
          <div class="palette-group-label">{category}</div>
          {#each cmds as cmd, i}
            {@const globalIndex = flatList().indexOf(cmd)}
            <button
              class="palette-item"
              class:selected={globalIndex === selectedIndex}
              onclick={() => executeCommand(cmd)}
              onmouseenter={() => (selectedIndex = globalIndex)}
            >
              <Icon name={cmd.icon} size={16} />
              <span>{cmd.label}</span>
            </button>
          {/each}
        </div>
      {/each}

      {#if flatList().length === 0}
        <div class="palette-empty">No commands found</div>
      {/if}
    </div>
  </div>
</div>

<style>
  .palette-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding-top: 15vh;
    z-index: var(--z-modal);
  }

  .palette-container {
    width: 100%;
    max-width: 560px;
    background: var(--dashboard-card);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-lg);
    overflow: hidden;
  }

  .palette-header {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-4);
    border-bottom: 1px solid var(--dashboard-border);
    color: var(--dashboard-text-muted);
  }

  .palette-input {
    flex: 1;
    background: none;
    border: none;
    font-size: var(--text-base);
    color: var(--dashboard-text);
    outline: none;
  }

  .palette-input::placeholder {
    color: var(--dashboard-text-muted);
  }

  .palette-kbd {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    padding: var(--space-1) var(--space-2);
    background: var(--dashboard-input);
    border-radius: var(--radius-sm);
    color: var(--dashboard-text-muted);
  }

  .palette-results {
    max-height: 400px;
    overflow-y: auto;
  }

  .palette-group {
    padding: var(--space-2);
  }

  .palette-group-label {
    font-size: var(--text-xs);
    font-weight: 600;
    color: var(--dashboard-text-muted);
    padding: var(--space-2);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .palette-item {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    width: 100%;
    padding: var(--space-3);
    background: none;
    border: none;
    border-radius: var(--radius-md);
    color: var(--dashboard-text);
    font-size: var(--text-sm);
    cursor: pointer;
    text-align: left;
  }

  .palette-item:hover,
  .palette-item.selected {
    background: var(--dashboard-input);
  }

  .palette-empty {
    padding: var(--space-6);
    text-align: center;
    color: var(--dashboard-text-muted);
    font-size: var(--text-sm);
  }
</style>

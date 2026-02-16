<script lang="ts">
  interface Props {
    name: string;
    size?: "sm" | "md" | "lg" | number;
    filled?: boolean;
    class?: string;
  }

  let {
    name,
    size = "md",
    filled = false,
    class: className = "",
  }: Props = $props();

  const sizeMap: Record<string, string> = {
    sm: "16px",
    md: "20px",
    lg: "24px",
  };

  // Handle both string sizes and numeric pixel values
  const fontSize = $derived(
    typeof size === "number" ? `${size}px` : sizeMap[size] || "20px",
  );

  const style = $derived(`
    font-size: ${fontSize};
    font-variation-settings: 'FILL' ${filled ? 1 : 0}, 'wght' 400, 'GRAD' 0, 'opsz' 24;
  `);

  // Map our internal names to Google Material Symbols names
  const iconMap: Record<string, string> = {
    "state-up": "check_circle",
    "state-down": "pause_circle",
    "state-no_carrier": "link_off",
    "state-missing": "cancel",
    "state-disabled": "do_not_disturb_on",
    "state-degraded": "warning",
    "state-error": "error",

    check: "check_circle",
    warning: "warning",
    error: "error",
    "alert-circle": "error", // Map alert-circle to error icon

    // Common actions
    plus: "add",
    minus: "remove",
    delete: "delete",
    trash: "delete", // Alias for common trash icon
    edit: "edit",
    "arrow-left-right": "swap_horiz",
    "arrow-right": "arrow_forward",
    "arrow-left": "arrow_back",
    "chevron-up": "expand_less",
    "chevron-down": "expand_more",

    // Services
    web: "public",
    ssh: "terminal",
    api: "api",
    icmp: "network_check",
    dhcp: "settings_ethernet",
    dns: "dns",
    ntp: "schedule",

    // Zone Types
    home: "home",
    cloud: "cloud",
    domain: "domain",

    // Dashboard / System
    power: "power_settings_new",
    power_off: "power_off",
    shield: "shield",
    restart_alt: "restart_alt",
    logs: "article",
    refresh: "refresh",
    monitoring: "analytics",
    hub: "hub",
    lan: "lan",
    vpn_key: "vpn_key",
    settings: "settings",
    menu: "menu",
    logout: "logout",
    router: "router",
    search: "search",
  };

  const iconName = $derived(iconMap[name] || name);
</script>

<span class="material-symbols-rounded {className}" {style} aria-hidden="true">
  {iconName}
</span>

<style>
  .material-symbols-rounded {
    display: inline-block;
    vertical-align: middle;
    line-height: 1;
    user-select: none;
    /* Default to non-filled, weight 400 */
    font-variation-settings:
      "FILL" 0,
      "wght" 400,
      "GRAD" 0,
      "opsz" 24;
  }
</style>

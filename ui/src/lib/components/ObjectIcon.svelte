<script lang="ts">
    import Icon from "$lib/components/Icon.svelte";

    let {
        type,
        name,
        size = 14,
    } = $props<{
        type:
            | "interface"
            | "ip"
            | "zone"
            | "port"
            | "protocol"
            | "mac"
            | "set"
            | "vlan"
            | "bond"
            | "user";
        name?: string;
        size?: number;
    }>();

    function getIcon(t: string): string {
        switch (t) {
            case "interface":
                return "settings_ethernet";
            case "ip":
                return "laptop"; // or dns?
            case "zone":
                return "security";
            case "port":
                return "tag"; // placeholder
            case "protocol":
                return "swap_horiz";
            case "mac":
                return "fingerprint";
            case "set":
                return "list";
            case "vlan":
                return "hub"; // or something else
            case "bond":
                return "link";
            case "user":
                return "person";
            default:
                return "help";
        }
    }

    // Handle specific interface types if type is 'interface' and name is provided
    function getInterfaceIcon(n: string): string {
        if (n.startsWith("bond")) return "link";
        if (n.startsWith("wg")) return "vpn_key";
        if (n.includes(".")) return "hub"; // vlan
        return "settings_ethernet";
    }

    const iconName = $derived(
        type === "interface" && name ? getInterfaceIcon(name) : getIcon(type),
    );
</script>

<div class="object-icon">
    <Icon name={iconName} {size} />
</div>

<style>
    .object-icon {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        vertical-align: middle;
    }
</style>

<script lang="ts">
    import { createEventDispatcher } from "svelte";
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
    import InterfaceStateBadge from "$lib/components/InterfaceStateBadge.svelte";
    import InterfaceLink from "$lib/components/InterfaceLink.svelte";
    import ZoneLink from "$lib/components/ZoneLink.svelte";
    import { t } from "svelte-i18n";

    // Actually, better to pass save/delete handlers from parent or use dispatched events.
    // Using events keeps it dumb.

    const dispatch = createEventDispatcher();

    export let iface: any;
    export let zones: any[] = [];
    export let hardwareInterfaces: any[] = [];
    export let loading = false;
    export let memberOf: string | undefined = undefined;

    let isEditing = false;

    // Edit form state
    let editZone = "";
    let editDescription = "";
    let editIpv4 = "";
    let editDhcp = false;
    let editMtu = "";
    let editGateway = "";
    let editDisabled = false;
    let editBondMembers: string[] = [];
    let editBondMode = "";
    let validationError = "";

    function startEdit() {
        editZone = iface.Zone || "";
        editDescription = iface.Description || "";
        editIpv4 = (iface.IPv4 || []).join(", ");
        editDhcp = iface.DHCP || false;
        editMtu = iface.MTU?.toString() || "";
        editGateway = iface.Gateway || "";
        editDisabled = iface.Disabled || false;
        editBondMembers = iface.Bond?.members || iface.Members || [];
        editBondMode = iface.Bond?.mode || iface.Mode || "balance-rr";
        validationError = "";
        isEditing = true;
    }

    function toggleBondMember(name: string) {
        if (editBondMembers.includes(name)) {
            editBondMembers = editBondMembers.filter((m) => m !== name);
        } else {
            editBondMembers = [...editBondMembers, name];
        }
    }

    function cancelEdit() {
        isEditing = false;
        validationError = "";
    }

    function handleSave() {
        validationError = "";
        let mtu: number | undefined;
        if (editMtu) {
            const parsed = parseInt(editMtu);
            // Allow 0 as explicit "default"
            if (parsed === 0) {
                mtu = 0;
            } else if (isNaN(parsed) || parsed < 68 || parsed > 65535) {
                validationError =
                    $t("interfaces.invalid_mtu") ||
                    "Invalid MTU. Must be 0 (default) or between 68 and 65535.";
                return;
            } else {
                mtu = parsed;
            }
        }

        const isBond = getInterfaceType(iface) === "bond";
        if (isBond && editBondMembers.length === 0) {
            validationError = "Bond must have at least one member";
            return;
        }

        // Validate IPv4 CIDR
        if (editIpv4) {
            const ips = editIpv4
                .split(",")
                .map((s) => s.trim())
                .filter(Boolean);
            const cidrRegex =
                /^(\d{1,3}\.){3}\d{1,3}\/(1?[0-9]|2[0-9]|3[0-2])$/;
            for (const ip of ips) {
                if (!cidrRegex.test(ip)) {
                    validationError =
                        $t("interfaces.invalid_cidr") ||
                        "Invalid CIDR format (e.g. 192.168.1.1/24)";
                    return;
                }
            }
        }

        dispatch("save", {
            name: iface.Name,
            zone: editZone,
            description: editDescription,
            ipv4: editIpv4
                ? editIpv4
                      .split(",")
                      .map((s) => s.trim())
                      .filter(Boolean)
                : [],
            dhcp: editDhcp,
            mtu: mtu,
            gateway: editGateway,
            disabled: editDisabled,
            bond: isBond
                ? {
                      mode: editBondMode,
                      interfaces: editBondMembers,
                  }
                : undefined,
        });
        isEditing = false;
    }

    function handleDelete() {
        dispatch("delete", iface);
    }

    function getZoneColorStyle(zoneName: string): string {
        const zone = zones.find((z: any) => z.name === zoneName);
        if (!zone) return "--zone-color: var(--color-muted)";
        if (zone.color?.startsWith("#")) {
            return `--zone-color: ${zone.color}`;
        }
        return `--zone-color: var(--zone-${zone.color}, var(--color-muted))`;
    }

    function getInterfaceType(iface: any): string {
        if (iface.Name?.startsWith("bond")) return "bond";
        if (iface.Name?.includes(".")) return "vlan";
        if (iface.Name?.startsWith("wg")) return "wireguard";
        if (iface.Name?.startsWith("tun") || iface.Name?.startsWith("tap"))
            return "tunnel";
        return "ethernet";
    }

    function formatBytes(bytes: number): string {
        if (bytes === 0) return "0 B";
        const k = 1024;
        const sizes = ["B", "KB", "MB", "GB", "TB"];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
    }
</script>

<Card>
    {#if isEditing}
        <div class="edit-form form-stack">
            <div class="edit-header">
                <h3>Editing {iface.Name}</h3>
                <Badge variant="outline">{getInterfaceType(iface)}</Badge>
            </div>

            <Select
                id={`zone-${iface.Name}`}
                label={$t("item.zone")}
                bind:value={editZone}
                options={[
                    { value: "", label: $t("common.none") },
                    ...zones.map((z) => ({ value: z.name, label: z.name })),
                ]}
            />

            {#if getInterfaceType(iface) === "bond"}
                <Select
                    id={`bond-mode-${iface.Name}`}
                    label={$t("interfaces.bond_mode")}
                    bind:value={editBondMode}
                    options={[
                        {
                            value: "balance-rr",
                            label: "Round Robin (balance-rr)",
                        },
                        {
                            value: "active-backup",
                            label: "Active Backup (active-backup)",
                        },
                        { value: "balance-xor", label: "XOR (balance-xor)" },
                        { value: "broadcast", label: "Broadcast" },
                        { value: "802.3ad", label: "LACP (802.3ad)" },
                        {
                            value: "balance-tlb",
                            label: "Adaptive TLB (balance-tlb)",
                        },
                        {
                            value: "balance-alb",
                            label: "Adaptive ALB (balance-alb)",
                        },
                    ]}
                />

                <div class="member-selection">
                    <span class="member-label"
                        >{$t("interfaces.select_members")}</span
                    >
                    <div class="member-list">
                        {#each hardwareInterfaces as hwIface}
                            <label
                                class="member-item"
                                class:disabled={!hwIface.isAvailable &&
                                    !editBondMembers.includes(hwIface.Name)}
                            >
                                <input
                                    type="checkbox"
                                    checked={editBondMembers.includes(
                                        hwIface.Name,
                                    )}
                                    disabled={!hwIface.isAvailable &&
                                        !editBondMembers.includes(hwIface.Name)}
                                    onchange={() =>
                                        toggleBondMember(hwIface.Name)}
                                />
                                <span class="member-name">{hwIface.Name}</span>
                                {#if !hwIface.isAvailable && !editBondMembers.includes(hwIface.Name)}
                                    <span class="member-status"
                                        >({hwIface.usageReason})</span
                                    >
                                {/if}
                            </label>
                        {/each}
                    </div>
                </div>
            {/if}

            <Input
                id={`desc-${iface.Name}`}
                label={$t("common.description")}
                bind:value={editDescription}
                placeholder="e.g., Primary WAN"
            />

            <Toggle label={$t("interfaces.use_dhcp")} bind:checked={editDhcp} />

            {#if !editDhcp}
                <Input
                    id={`ip-${iface.Name}`}
                    label={$t("interfaces.ipv4_list")}
                    bind:value={editIpv4}
                    placeholder="192.168.1.1/24"
                />
                <Input
                    id={`gw-${iface.Name}`}
                    label={$t("common.gateway")}
                    bind:value={editGateway}
                    placeholder="192.168.1.254"
                />
            {/if}

            <div class="row-2">
                <Input
                    id={`mtu-${iface.Name}`}
                    label={$t("common.mtu")}
                    bind:value={editMtu}
                    type="text"
                    placeholder="1500"
                />
                <Toggle
                    label={$t("interfaces.interface_enabled")}
                    checked={!editDisabled}
                    onchange={(checked) => (editDisabled = !checked)}
                />
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
            {#if validationError}
                <div class="mt-2 text-sm text-destructive">
                    {validationError}
                </div>
            {/if}
        </div>
    {:else}
        <div class="iface-header">
            <div class="iface-name-row">
                <InterfaceLink
                    name={iface.Name}
                    className="text-lg font-semibold"
                />
                {#if iface.Alias}<span class="iface-alias">({iface.Alias})</span
                    >{/if}
                {#if memberOf}
                    <Badge
                        variant="outline"
                        class="text-primary border-primary"
                    >
                        <Icon name="link" size={12} class="mr-1" /> Member of {memberOf}
                    </Badge>
                {/if}
                <Badge
                    variant={getInterfaceType(iface) === "ethernet"
                        ? "outline"
                        : "secondary"}
                >
                    {getInterfaceType(iface)}
                </Badge>
                {#if iface.is_unconfigured}
                    <Badge variant="warning">
                        {$t("common.unconfigured", { default: "Unconfigured" })}
                    </Badge>
                {/if}
                <InterfaceStateBadge
                    state={iface.State || (iface.Disabled ? "disabled" : "up")}
                    size="sm"
                />
            </div>
            {#if iface.Speed && iface.Speed > 0}
                <div class="link-speed text-xs text-muted-foreground">
                    {iface.Speed} Mbps {iface.Duplex || ""}
                </div>
            {/if}
            <div class="iface-actions">
                <Button
                    variant="ghost"
                    size="sm"
                    onclick={startEdit}
                    title="Edit interface"
                    ><Icon name="edit" size="sm" /></Button
                >
                {#if !iface.is_unconfigured && ["vlan", "bond", "ethernet"].includes(getInterfaceType(iface))}
                    <Button
                        variant="ghost"
                        size="sm"
                        onclick={handleDelete}
                        title="Delete interface configuration"
                        ><Icon name="trash" size="sm" /></Button
                    >
                {/if}
            </div>
        </div>

        {#if iface.Description}
            <p class="iface-description">{iface.Description}</p>
        {/if}

        <div class="iface-details">
            <div class="detail-row">
                <span class="detail-label">{$t("item.zone")}:</span>
                {#if iface.Zone}
                    <ZoneLink name={iface.Zone} />
                {:else}
                    <span class="text-sm text-muted-foreground"
                        >{$t("common.none")}</span
                    >
                {/if}
            </div>

            {#if iface.Vendor}
                <div class="detail-row">
                    <span class="detail-label">{$t("common.vendor")}:</span>
                    <span class="detail-value">{iface.Vendor}</span>
                </div>
            {/if}

            <div class="detail-row">
                <span class="detail-label">{$t("interfaces.ipv4")}:</span>
                <span class="detail-value mono">
                    {#if iface.DHCP && (!iface.IPv4 || iface.IPv4.length === 0)}
                        {$t("interfaces.dhcp_acquiring")}
                    {:else if iface.IPv4?.length > 0}
                        {iface.IPv4.join(", ")}
                        {#if iface.DHCP}
                            <span class="text-xs text-muted-foreground ml-1"
                                >({$t("interfaces.dhcp")})</span
                            >
                        {/if}
                    {:else}
                        {$t("common.none")}
                    {/if}
                </span>
            </div>

            {#if iface.IPv6?.length > 0}
                <div class="detail-row">
                    <span class="detail-label">{$t("interfaces.ipv6")}:</span>
                    <span class="detail-value mono"
                        >{iface.IPv6.join(", ")}</span
                    >
                </div>
            {/if}

            {#if iface.Gateway}
                <div class="detail-row">
                    <span class="detail-label">{$t("common.gateway")}:</span>
                    <span class="detail-value mono">{iface.Gateway}</span>
                </div>
            {/if}

            {#if iface.MTU}
                <div class="detail-row">
                    <span class="detail-label">{$t("common.mtu")}:</span>
                    <span class="detail-value">{iface.MTU}</span>
                </div>
            {/if}

            {#if iface.Bond?.members?.length > 0 || iface.Members?.length > 0}
                <div class="detail-row">
                    <span class="detail-label"
                        >{$t("interfaces.bond_members")}:</span
                    >
                    <span class="detail-value">
                        {(iface.Bond?.members || iface.Members || []).join(
                            ", ",
                        )}
                    </span>
                </div>
            {/if}

            {#if iface.VLANs?.length > 0}
                <div class="detail-row">
                    <span class="detail-label">{$t("interfaces.vlans")}:</span>
                    <span class="detail-value">
                        {iface.VLANs.map((v: any) => v.ID || v.id).join(", ")}
                    </span>
                </div>
            {/if}

            {#if iface.Stats}
                <div class="detail-separator"></div>
                <div class="stats-grid">
                    <div class="stat-item">
                        <span class="stat-label">RX:</span>
                        <span class="stat-value"
                            >{formatBytes(iface.Stats.rx_bytes)}</span
                        >
                    </div>
                    <div class="stat-item">
                        <span class="stat-label">TX:</span>
                        <span class="stat-value"
                            >{formatBytes(iface.Stats.tx_bytes)}</span
                        >
                    </div>
                </div>
            {/if}
        </div>
    {/if}
</Card>

<style>
    .iface-header,
    .edit-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-bottom: var(--space-2);
    }
    .edit-header h3 {
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
    }
    .form-stack {
        display: flex;
        flex-direction: column;
        gap: var(--space-3);
    }
    .row-2 {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: var(--space-4);
        align-items: center;
    }
    .edit-actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        margin-top: var(--space-2);
    }
    .iface-name-row {
        display: flex;
        align-items: center;
        gap: var(--space-2);
    }
    .iface-name-row {
        display: flex;
        align-items: center;
        gap: var(--space-2);
    }
    .iface-actions {
        display: flex;
        gap: var(--space-1);
    }
    .iface-description {
        color: var(--color-muted);
        font-size: var(--text-sm);
        margin: 0 0 var(--space-3) 0;
    }
    .iface-details {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
        padding-top: var(--space-3);
        border-top: 1px solid var(--color-border);
    }
    .detail-row {
        display: flex;
        justify-content: space-between;
        font-size: var(--text-sm);
    }
    .detail-label {
        color: var(--color-muted);
    }
    .detail-value {
        color: var(--color-foreground);
    }
    .mono {
        font-family: var(--font-mono);
    }
    .link-speed {
        margin-left: auto;
        margin-right: var(--space-4);
        font-family: var(--font-mono);
    }
    .detail-separator {
        height: 1px;
        background: var(--color-border);
        margin: var(--space-2) 0;
    }
    .stats-grid {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: var(--space-4);
    }
    .stat-item {
        display: flex;
        justify-content: space-between;
        font-size: var(--text-xs);
    }
    .stat-label {
        color: var(--color-muted);
    }
    .stat-value {
        font-family: var(--font-mono);
        color: var(--color-foreground);
    }

    /* Utility for custom badge style */
    :global(.border-primary) {
        border-color: var(--color-primary) !important;
    }
</style>

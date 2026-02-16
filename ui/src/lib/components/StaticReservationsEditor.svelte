<script lang="ts">
    import { t } from "svelte-i18n";
    import { Button, Input, Icon } from "$lib/components";

    export let reservations: any[] = [];

    let newMac = "";
    let newIp = "";
    let newHostname = "";

    function addReservation() {
        if (!newMac || !newIp) return;
        reservations = [
            ...reservations,
            { mac: newMac, ip: newIp, hostname: newHostname },
        ];
        newMac = "";
        newIp = "";
        newHostname = "";
    }

    function removeReservation(mac: string) {
        reservations = reservations.filter((r) => r.mac !== mac);
    }
</script>

<div class="reservations-section bg-secondary/10 p-4 rounded-lg">
    <h3 class="text-sm font-medium mb-3">{$t("dhcp.static_reservations")}</h3>

    {#if reservations.length > 0}
        <div class="space-y-2 mb-4">
            {#each reservations as res}
                <div
                    class="flex items-center justify-between bg-background p-2 rounded border border-border"
                >
                    <div class="flex flex-col text-xs">
                        <span class="font-mono">{res.mac}</span>
                        <span class="text-muted-foreground">{res.ip}</span>
                    </div>
                    <div class="flex items-center gap-2">
                        {#if res.hostname}<span
                                class="text-xs text-muted-foreground"
                                >{res.hostname}</span
                            >{/if}
                        <Button
                            variant="ghost"
                            size="sm"
                            onclick={() => removeReservation(res.mac)}
                        >
                            <Icon name="delete" />
                        </Button>
                    </div>
                </div>
            {/each}
        </div>
    {/if}

    <div class="grid grid-cols-3 gap-2">
        <Input
            placeholder={$t("network.mac_address")}
            bind:value={newMac}
            class="text-xs"
        />
        <Input
            placeholder={$t("common.ip_address")}
            bind:value={newIp}
            class="text-xs"
        />
        <Input
            placeholder={$t("common.hostname")}
            bind:value={newHostname}
            class="text-xs"
        />
    </div>
    <div class="mt-2 flex justify-end">
        <Button
            variant="outline"
            size="sm"
            onclick={addReservation}
            disabled={!newMac || !newIp}>{$t("common.add")}</Button
        >
    </div>
</div>

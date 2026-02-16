<script lang="ts">
    import { onMount } from "svelte";
    import TopologyGraph from "$lib/components/TopologyGraph.svelte";
    import BaseWidget from "./BaseWidget.svelte";
    import { api } from "$lib/stores/app";
    import { wanZones, lanZones, totalDevices } from "$lib/stores/zones";
    import { containers } from "$lib/stores/runtime";
    import Icon from "$lib/components/Icon.svelte";

    let { onremove } = $props();

    // Build topology graph from derived stores (same logic as before)
    let topologyGraph = $derived(() => {
        const nodes: any[] = [
            { id: "router", type: "router", label: "Router", ip: "" },
        ];
        const links: any[] = [];

        // Connect WAN zones to internet
        if ($wanZones.length > 0) {
            nodes.push({
                id: "internet",
                type: "cloud",
                label: "Internet",
                ip: "",
            });
            for (const zone of $wanZones) {
                nodes.push({
                    id: zone.name,
                    type: "wan",
                    label: zone.name.toUpperCase(),
                    ip: zone.ips[0] || "",
                });
                links.push({ source: "router", target: zone.name });
                links.push({ source: zone.name, target: "internet" });
            }
        }

        // LAN zones connect to router
        for (const zone of $lanZones) {
            nodes.push({
                id: zone.name,
                type: "switch",
                label: zone.name.toUpperCase(),
                ip: zone.ip || "",
                deviceCount: zone.deviceCount,
            });
            links.push({ source: "router", target: zone.name });
        }

        // Containers connect to router
        for (const c of $containers) {
            const name = c.Names[0]?.replace(/^\//, "") || c.Id.slice(0, 12);
            let ip = "";
            const networks = Object.values(c.NetworkSettings.Networks);
            if (networks.length > 0) ip = (networks[0] as any).IPAddress;

            nodes.push({
                id: c.Id,
                type: "container",
                label: name,
                ip: ip,
                description: c.Image,
                icon: "container",
            });
            links.push({ source: "router", target: c.Id });
        }

        return { nodes, links };
    });
</script>

<BaseWidget title="Network Topology" icon="hub" {onremove}>
    <div class="hero-visualization">
        <TopologyGraph graph={topologyGraph()} />
    </div>
    <div class="footer-stats">
        <span>{$totalDevices} devices online</span>
    </div>
</BaseWidget>

<style>
    .hero-visualization {
        height: 100%;
        min-height: 250px;
        overflow: hidden;
    }

    .footer-stats {
        position: absolute;
        bottom: var(--space-4);
        right: var(--space-4);
        font-size: var(--text-xs);
        color: var(--dashboard-text-muted);
        background: var(--dashboard-card);
        padding: 2px 8px;
        border-radius: 12px;
        border: 1px solid var(--dashboard-border);
    }
</style>

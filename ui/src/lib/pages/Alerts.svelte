<script lang="ts">
    import { onMount } from "svelte";
    import { api } from "$lib/stores/app";
    import Table from "$lib/components/Table.svelte";
    import Badge from "$lib/components/Badge.svelte";
    import Icon from "$lib/components/Icon.svelte";
    import Button from "$lib/components/Button.svelte";
    import Select from "$lib/components/Select.svelte";

    let alerts = $state([]);
    let loading = $state(true);
    let error = $state(null);

    // Filters
    let severityFilter = $state("all");
    let limit = $state(100);

    const columns = [
        { key: "severity", label: "Severity", width: "100px" },
        { key: "timestamp", label: "Time", width: "180px" },
        { key: "rule_name", label: "Source", width: "200px" },
        { key: "message", label: "Message" },
    ];

    async function loadAlerts() {
        loading = true;
        error = null;
        try {
            const res = await api.get(`/api/alerts/history?limit=${limit}`);
            alerts = res || [];
        } catch (e) {
            error = e.message;
        } finally {
            loading = false;
        }
    }

    onMount(() => {
        loadAlerts();
    });

    // Derived state for filtering
    let filteredAlerts = $derived(
        severityFilter === "all"
            ? alerts
            : alerts.filter((a) => a.severity === severityFilter),
    );

    function getBadgeVariant(severity) {
        switch (severity) {
            case "critical":
                return "destructive"; // Red
            case "error":
                return "warning"; // Orange/Yellow (Badge might not have error)
            case "warning":
                return "warning";
            case "info":
                return "info"; // Blue
            default:
                return "default";
        }
    }

    function formatTime(ts) {
        return new Date(ts).toLocaleString();
    }
</script>

<div class="alerts-page space-y-4">
    <div
        class="flex justify-between items-center bg-card p-4 rounded-lg border border-border"
    >
        <div class="flex gap-4 items-center">
            <h2 class="text-lg font-medium">Alert History</h2>

            <div class="w-40">
                <select
                    class="border rounded px-2 py-1 bg-background text-sm w-full"
                    bind:value={severityFilter}
                >
                    <option value="all">All Severities</option>
                    <option value="critical">Critical</option>
                    <option value="error">Error</option>
                    <option value="warning">Warning</option>
                    <option value="info">Info</option>
                </select>
            </div>
        </div>

        <Button
            variant="outline"
            size="sm"
            onclick={loadAlerts}
            disabled={loading}
        >
            <Icon
                name="refresh"
                size={16}
                class={loading ? "animate-spin" : ""}
            />
            Refresh
        </Button>
    </div>

    {#if error}
        <div
            class="bg-destructive/10 text-destructive p-4 rounded-lg flex gap-2 items-center"
        >
            <Icon name="error" />
            {error}
        </div>
    {/if}

    <Table {columns} data={filteredAlerts} emptyMessage="No alerts found">
        {#snippet children(row)}
            <td>
                <Badge variant={getBadgeVariant(row.severity)}>
                    {row.severity}
                </Badge>
            </td>
            <td class="text-muted-foreground tabular-nums">
                {formatTime(row.timestamp)}
            </td>
            <td class="font-medium">
                {row.rule_name || row.source || "System"}
            </td>
            <td>
                {row.message}
            </td>
        {/snippet}
    </Table>
</div>

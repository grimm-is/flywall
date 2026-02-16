<script lang="ts">
    import { onMount } from "svelte";
    import { api } from "$lib/stores/app";
    import Table from "$lib/components/Table.svelte";
    import Icon from "$lib/components/Icon.svelte";
    import Button from "$lib/components/Button.svelte";

    let logs = $state([]);
    let loading = $state(true);
    let error = $state(null);
    let limit = $state(100);

    const columns = [
        { key: "timestamp", label: "Time", width: "180px" },
        { key: "user", label: "User", width: "120px" },
        { key: "action", label: "Action", width: "150px" },
        { key: "resource", label: "Resource", width: "150px" },
        { key: "details", label: "Details" },
    ];

    async function loadLogs() {
        loading = true;
        error = null;
        try {
            const res = await api.get(`/api/audit?limit=${limit}`);
            logs = res || [];
        } catch (e) {
            error = e.message;
        } finally {
            loading = false;
        }
    }

    onMount(() => {
        loadLogs();
    });

    function formatTime(ts) {
        try {
            return new Date(ts).toLocaleString();
        } catch {
            return ts;
        }
    }
</script>

<div class="audit-page space-y-4">
    <div
        class="flex justify-between items-center bg-card p-4 rounded-lg border border-border"
    >
        <div class="flex gap-4 items-center">
            <h2 class="text-lg font-medium">Audit Log</h2>
        </div>

        <Button
            variant="outline"
            size="sm"
            onclick={loadLogs}
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

    <Table {columns} data={logs} emptyMessage="No audit logs found">
        {#snippet children(row)}
            <td class="text-muted-foreground tabular-nums">
                {formatTime(row.timestamp)}
            </td>
            <td class="font-medium">
                {row.username || row.user || "System"}
            </td>
            <td>
                <div class="flex items-center gap-2">
                    <span class="capitalize">{row.action}</span>
                </div>
            </td>
            <td>
                <code class="text-xs bg-muted px-1 rounded">{row.resource}</code
                >
            </td>
            <td class="text-sm text-muted-foreground">
                {JSON.stringify(row.details || {})}
            </td>
        {/snippet}
    </Table>
</div>

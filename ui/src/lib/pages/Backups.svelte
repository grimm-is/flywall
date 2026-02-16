<script lang="ts">
    import { onMount } from "svelte";
    import { api } from "$lib/stores/app";
    import Table from "$lib/components/Table.svelte";
    import Icon from "$lib/components/Icon.svelte";
    import Button from "$lib/components/Button.svelte";
    import { formatBytes } from "$lib/utils/format";

    let backups = $state([]);
    let loading = $state(true);
    let error = $state(null);
    let processError = $state(null);
    let creating = $state(false);

    const columns = [
        { key: "filename", label: "Filename", width: "250px" },
        { key: "description", label: "Description" },
        { key: "size", label: "Size", width: "100px" },
        { key: "created_at", label: "Created", width: "180px" },
        { key: "actions", label: "Actions", width: "180px", align: "right" },
    ];

    async function loadBackups() {
        loading = true;
        error = null;
        try {
            const res = await api.get("/api/backups");
            backups = res || [];
        } catch (e) {
            error = e.message;
        } finally {
            loading = false;
        }
    }

    async function createBackup() {
        creating = true;
        processError = null;
        try {
            await api.post("/api/backups", { description: "Manual backup" });
            await loadBackups();
        } catch (e) {
            processError = "Failed to create backup: " + e.message;
        } finally {
            creating = false;
        }
    }

    async function restoreBackup(backup) {
        if (
            !confirm(
                `Are you sure you want to restore ${backup.filename}? This will overwrite current configuration.`,
            )
        )
            return;

        try {
            await api.post(`/api/backups/${backup.id}/restore`);
            alert("Restore initiated. System may reboot.");
        } catch (e) {
            alert("Restore failed: " + e.message);
        }
    }

    async function deleteBackup(backup) {
        if (!confirm(`Delete backup ${backup.filename}?`)) return;

        try {
            await api.delete(`/api/backups/${backup.id}`);
            backups = backups.filter((b) => b.id !== backup.id);
        } catch (e) {
            alert("Delete failed: " + e.message);
        }
    }

    onMount(() => {
        loadBackups();
    });

    function formatTime(ts) {
        try {
            return new Date(ts).toLocaleString();
        } catch {
            return ts;
        }
    }

    // Simple byte formatter fallback if utils/format doesn't exist or work
    function fmtBytes(bytes) {
        if (bytes === 0) return "0 B";
        const k = 1024;
        const sizes = ["B", "KB", "MB", "GB"];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
    }
</script>

<div class="backups-page space-y-4">
    <div
        class="flex justify-between items-center bg-card p-4 rounded-lg border border-border"
    >
        <div class="flex gap-4 items-center">
            <h2 class="text-lg font-medium">System Backups</h2>
        </div>

        <div class="flex gap-2">
            <Button
                variant="outline"
                size="sm"
                onclick={loadBackups}
                disabled={loading}
            >
                <Icon
                    name="refresh"
                    size={16}
                    class={loading ? "animate-spin" : ""}
                />
                Refresh
            </Button>
            <Button
                variant="primary"
                size="sm"
                onclick={createBackup}
                disabled={creating}
            >
                <Icon name="add" size={16} />
                Create Backup
            </Button>
        </div>
    </div>

    {#if error || processError}
        <div
            class="bg-destructive/10 text-destructive p-4 rounded-lg flex gap-2 items-center"
        >
            <Icon name="error" />
            {error || processError}
        </div>
    {/if}

    <Table {columns} data={backups} emptyMessage="No backups found">
        {#snippet children(row)}
            <td class="font-medium">
                {row.filename}
            </td>
            <td class="text-muted-foreground">
                {row.description || "-"}
            </td>
            <td class="text-sm tabular-nums">
                {fmtBytes(row.size)}
            </td>
            <td class="text-muted-foreground tabular-nums">
                {formatTime(row.created_at)}
            </td>
            <td class="text-right">
                <div class="flex justify-end gap-2">
                    <Button
                        variant="ghost"
                        size="icon"
                        onclick={() => restoreBackup(row)}
                        title="Restore"
                    >
                        <Icon name="upload" size={16} />
                    </Button>
                    <a
                        href="/api/backups/{row.id}/download"
                        class="btn btn-ghost btn-icon"
                        title="Download"
                        download
                    >
                        <Icon name="download" size={16} />
                    </a>
                    <Button
                        variant="ghost"
                        size="icon"
                        class="text-destructive"
                        onclick={() => deleteBackup(row)}
                        title="Delete"
                    >
                        <Icon name="delete" size={16} />
                    </Button>
                </div>
            </td>
        {/snippet}
    </Table>
</div>

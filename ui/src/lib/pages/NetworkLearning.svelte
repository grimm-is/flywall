<script lang="ts">
    import { onMount } from "svelte";
    import { api } from "$lib/stores/app";
    import Card from "$lib/components/Card.svelte";
    import Button from "$lib/components/Button.svelte";
    import Badge from "$lib/components/Badge.svelte";
    import Spinner from "$lib/components/Spinner.svelte";
    import Icon from "$lib/components/Icon.svelte";
    import { t } from "svelte-i18n";

    type Flow = {
        id: number;
        src_ip: string;
        src_mac: string;
        src_zone?: string;
        vendor?: string;
        dst_ip: string;
        dst_port: number;
        dst_hostname?: string;
        dst_zone?: string;
        protocol: string;
        state: string;
        first_seen?: string;
        last_seen?: string;
        packet_count?: number;
    };

    type FlowCounts = {
        pending: number;
        approved: number;
        denied: number;
        total: number;
    };

    let flows = $state<Flow[]>([]);
    let counts = $state<FlowCounts>({
        pending: 0,
        approved: 0,
        denied: 0,
        total: 0,
    });
    let loading = $state(false);
    let error = $state("");
    let filter = $state("pending"); // pending, approved, denied

    async function loadFlows() {
        loading = true;
        error = "";
        try {
            const result = await api.getFlows(filter, 100, 0);
            flows = result.flows || [];
            counts = result.counts || {
                pending: 0,
                approved: 0,
                denied: 0,
                total: 0,
            };
        } catch (e: any) {
            error = e.message || "Failed to load flows";
            flows = [];
        } finally {
            loading = false;
        }
    }

    async function approveFlow(id: number) {
        try {
            await api.approveFlow(id);
            loadFlows();
        } catch (e: any) {
            alert(
                $t("learning.failed_approve", { values: { err: e.message } }),
            );
        }
    }

    async function denyFlow(id: number) {
        try {
            await api.denyFlow(id);
            loadFlows();
        } catch (e: any) {
            alert($t("learning.failed_deny", { values: { err: e.message } }));
        }
    }

    async function deleteFlow(id: number) {
        if (
            !confirm(
                $t("common.delete_confirm_item", {
                    values: { item: $t("item.flow") },
                }),
            )
        )
            return;
        try {
            await api.deleteFlow(id);
            loadFlows();
        } catch (e: any) {
            alert($t("learning.failed_delete", { values: { err: e.message } }));
        }
    }

    function formatDate(dateStr?: string): string {
        if (!dateStr) return "-";
        const date = new Date(dateStr);
        return date.toLocaleString();
    }

    onMount(() => {
        loadFlows();
    });

    $effect(() => {
        // Reload when filter changes
        filter;
        loadFlows();
    });
</script>

<div class="page-header">
    <div class="tabs">
        <button
            class:active={filter === "pending"}
            onclick={() => (filter = "pending")}
            >Pending {#if counts.pending > 0}<Badge variant="secondary"
                    >{counts.pending}</Badge
                >{/if}</button
        >
        <button
            class:active={filter === "approved"}
            onclick={() => (filter = "approved")}>Approved</button
        >
        <button
            class:active={filter === "denied"}
            onclick={() => (filter = "denied")}>Denied</button
        >
    </div>
    <Button variant="outline" size="sm" onclick={loadFlows} disabled={loading}>
        <Icon name="refresh" size="sm" />
        Refresh
    </Button>
</div>

{#if error}
    <div class="error">{error}</div>
{/if}

<Card>
    <div class="table-container">
        {#if loading}
            <div class="loading-state">
                <Spinner size="md" />
                <span>Loading flows...</span>
            </div>
        {:else if flows.length === 0}
            <div class="empty-state">
                <Icon name="check_circle" size={48} />
                <p>No {filter} flows</p>
                <span class="text-muted"
                    >Flows will appear here as network traffic is detected</span
                >
            </div>
        {:else}
            <table>
                <thead>
                    <tr>
                        <th>Source</th>
                        <th>Destination</th>
                        <th>Protocol</th>
                        <th>First Seen</th>
                        <th class="text-right">Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {#each flows as flow}
                        <tr>
                            <td>
                                <div class="cell-stack">
                                    <span class="font-mono">{flow.src_ip}</span>
                                    <span class="text-xs text-muted">
                                        {flow.src_mac || ""}
                                        {#if flow.vendor}({flow.vendor}){/if}
                                        {#if flow.src_zone}<Badge
                                                variant="outline"
                                                >{flow.src_zone}</Badge
                                            >{/if}
                                    </span>
                                </div>
                            </td>
                            <td>
                                <div class="cell-stack">
                                    <span class="font-mono"
                                        >{flow.dst_ip}:{flow.dst_port}</span
                                    >
                                    {#if flow.dst_hostname}
                                        <span class="text-xs text-muted"
                                            >{flow.dst_hostname}</span
                                        >
                                    {/if}
                                </div>
                            </td>
                            <td>
                                <Badge variant="outline">{flow.protocol}</Badge>
                            </td>
                            <td class="text-sm"
                                >{formatDate(flow.first_seen)}</td
                            >
                            <td class="text-right">
                                <div class="actions">
                                    {#if filter === "pending"}
                                        <Button
                                            size="sm"
                                            variant="default"
                                            onclick={() => approveFlow(flow.id)}
                                            aria-label={`Allow traffic from ${flow.src_ip} to ${flow.dst_ip}`}
                                            >Allow</Button
                                        >
                                        <Button
                                            size="sm"
                                            variant="destructive"
                                            onclick={() => denyFlow(flow.id)}
                                            aria-label={`Block traffic from ${flow.src_ip} to ${flow.dst_ip}`}
                                            >Block</Button
                                        >
                                    {:else}
                                        <Button
                                            size="sm"
                                            variant="ghost"
                                            onclick={() => deleteFlow(flow.id)}
                                            aria-label={`Delete flow ${flow.id}`}
                                            >Delete</Button
                                        >
                                    {/if}
                                </div>
                            </td>
                        </tr>
                    {/each}
                </tbody>
            </table>
        {/if}
    </div>
</Card>

<style>
    .page-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: var(--space-4);
    }

    .tabs {
        display: flex;
        gap: var(--space-2);
        background: var(--color-surface);
        padding: var(--space-1);
        border-radius: var(--radius-md);
        border: 1px solid var(--color-border);
    }

    .tabs button {
        background: none;
        border: none;
        padding: var(--space-2) var(--space-4);
        border-radius: var(--radius-sm);
        color: var(--color-muted);
        cursor: pointer;
        font-size: var(--text-sm);
        font-weight: 500;
    }

    .tabs button.active {
        background: var(--color-background);
        color: var(--color-foreground);
        box-shadow: var(--shadow-sm);
    }

    .table-container {
        overflow-x: auto;
    }

    table {
        width: 100%;
        border-collapse: collapse;
    }

    th,
    td {
        padding: var(--space-3);
        text-align: left;
        border-bottom: 1px solid var(--color-border);
    }

    th {
        font-size: var(--text-xs);
        font-weight: 600;
        color: var(--color-muted);
        text-transform: uppercase;
        letter-spacing: 0.05em;
    }

    .cell-stack {
        display: flex;
        flex-direction: column;
    }

    .text-xs {
        font-size: var(--text-xs);
    }
    .text-sm {
        font-size: var(--text-sm);
    }
    .text-muted {
        color: var(--color-muted);
    }
    .font-mono {
        font-family: var(--font-mono);
    }
    .text-right {
        text-align: right;
    }

    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
    }

    .error {
        color: var(--color-destructive);
        margin-bottom: var(--space-4);
    }

    .loading-state {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: var(--space-3);
        padding: var(--space-8);
        color: var(--color-muted);
    }

    .empty-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: var(--space-2);
        padding: var(--space-8);
        color: var(--color-muted);
        text-align: center;
    }

    .empty-state p {
        font-size: var(--text-lg);
        font-weight: 500;
        margin: 0;
    }
</style>

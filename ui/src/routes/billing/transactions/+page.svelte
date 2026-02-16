<script lang="ts">
    import { onMount } from "svelte";
    import { billingStore } from "$lib/stores/billing";
    import Table from "$lib/components/Table.svelte";
    import Icon from "$lib/components/Icon.svelte";

    onMount(() => {
        billingStore.listTransactions();
    });

    const columns = [
        { key: "created_at", label: "Date" },
        { key: "type", label: "Type" },
        { key: "amount", label: "Amount" },
        { key: "status", label: "Status" },
        { key: "provider_ref", label: "Ref ID", align: "right" as const },
    ];

    let transactions = $derived($billingStore.transactions);
    let loading = $derived($billingStore.loading);
</script>

<div class="transactions-page">
    {#if loading}
        <div class="loading-state">
            <div class="spinner"></div>
            Loading transactions...
        </div>
    {:else if transactions.length === 0}
        <div class="empty-state">
            <Icon name="activity" size={48} />
            <h3>No Transactions</h3>
            <p>No payment activity recorded yet.</p>
        </div>
    {:else}
        <Table {columns} data={transactions}>
            {#snippet children(row: any)}
                <td>{new Date(row.created_at).toLocaleString()}</td>
                <td>
                    <span class="type-badge">
                        {row.type}
                    </span>
                </td>
                <td>
                    <span class="amount" class:negative={row.type === "refund"}>
                        {row.type === "refund" ? "-" : ""}
                        {(row.amount / 100).toLocaleString("en-US", {
                            style: "currency",
                            currency: "USD",
                        })}
                    </span>
                </td>
                <td>
                    <span
                        class="status-badge"
                        class:success={row.status === "succeeded"}
                        class:failed={row.status === "failed"}
                    >
                        {row.status}
                    </span>
                </td>
                <td class="ref-cell">
                    <span class="ref-id"
                        >{row.provider_ref || row.id.slice(0, 8)}</span
                    >
                </td>
            {/snippet}
        </Table>
    {/if}
</div>

<style>
    .transactions-page {
        animation: fade-in 0.3s ease;
    }

    .type-badge {
        font-size: 0.85rem;
        text-transform: capitalize;
        color: var(--color-foreground);
    }

    .amount {
        font-weight: 500;
        color: var(--color-foreground);
    }

    .amount.negative {
        color: var(--color-muted);
    }

    .status-badge {
        font-size: 0.85rem;
        text-transform: capitalize;
        color: var(--color-muted);
    }

    .status-badge.success {
        color: var(--color-success);
    }

    .status-badge.failed {
        color: var(--color-destructive);
    }

    .ref-cell {
        text-align: right;
    }

    .ref-id {
        font-family: monospace;
        font-size: 0.85rem;
        color: var(--color-muted);
    }

    .loading-state,
    .empty-state {
        padding: 4rem;
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 1rem;
        text-align: center;
        color: var(--color-muted);
    }

    .spinner {
        width: 32px;
        height: 32px;
        border: 2px solid var(--color-border);
        border-top-color: var(--color-primary);
        border-radius: 50%;
        animation: spin 1s linear infinite;
    }

    @keyframes spin {
        to {
            transform: rotate(360deg);
        }
    }
    @keyframes fade-in {
        from {
            opacity: 0;
        }
        to {
            opacity: 1;
        }
    }
</style>

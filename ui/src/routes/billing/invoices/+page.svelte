<script lang="ts">
    import { onMount } from "svelte";
    import { billingStore } from "$lib/stores/billing";
    import Table from "$lib/components/Table.svelte";
    import Icon from "$lib/components/Icon.svelte";
    import Button from "$lib/components/Button.svelte";

    onMount(() => {
        billingStore.listInvoices();
    });

    const columns = [
        { key: "created_at", label: "Date" },
        { key: "invoice_number", label: "Invoice #" },
        { key: "amount_due", label: "Amount" },
        { key: "status", label: "Status" },
        { key: "action", label: "", align: "right" as const },
    ];

    let invoices = $derived($billingStore.invoices);
    let loading = $derived($billingStore.loading);
</script>

<div class="invoices-page">
    {#if loading}
        <div class="loading-state">
            <div class="spinner"></div>
            Loading invoices...
        </div>
    {:else if invoices.length === 0}
        <div class="empty-state">
            <Icon name="file-text" size={48} />
            <h3>No Invoices Yet</h3>
            <p>You haven't been billed for anything yet.</p>
        </div>
    {:else}
        <Table {columns} data={invoices}>
            {#snippet children(row: any)}
                <td>{new Date(row.created_at).toLocaleDateString()}</td>
                <td><span class="invoice-number">{row.invoice_number}</span></td
                >
                <td>
                    <span class="amount">
                        {(row.amount_due / 100).toLocaleString("en-US", {
                            style: "currency",
                            currency: "USD",
                        })}
                    </span>
                </td>
                <td>
                    <span
                        class="status-badge"
                        class:paid={row.status === "paid"}
                    >
                        {row.status}
                    </span>
                </td>
                <td class="action-cell">
                    <Button
                        variant="ghost"
                        size="sm"
                        onclick={() =>
                            alert("Download PDF not implemented yet")}
                    >
                        <Icon name="download" size={16} />
                        PDF
                    </Button>
                </td>
            {/snippet}
        </Table>
    {/if}
</div>

<style>
    .invoices-page {
        animation: fade-in 0.3s ease;
    }

    .invoice-number {
        font-family: monospace;
        color: var(--color-foreground);
    }

    .amount {
        font-weight: 600;
        color: var(--color-foreground);
    }

    .status-badge {
        padding: 0.25rem 0.5rem;
        background: var(--color-surfaceHover);
        color: var(--color-muted);
        border-radius: var(--radius-sm);
        font-size: 0.85rem;
        text-transform: capitalize;
    }

    .status-badge.paid {
        background: rgba(72, 187, 120, 0.15);
        color: var(--color-success);
    }

    .action-cell {
        text-align: right;
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

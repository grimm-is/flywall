<script lang="ts">
    import { onMount } from "svelte";
    import { staffStore } from "$lib/stores/staff";
    import Table from "$lib/components/Table.svelte";
    import Icon from "$lib/components/Icon.svelte";
    import Button from "$lib/components/Button.svelte";
    import Input from "$lib/components/Input.svelte";

    let searchQuery = $state("");
    let timeout: any;

    const columns = [
        { key: "name", label: "Organization" },
        { key: "account_status", label: "Status" },
        { key: "created_at", label: "Joined" },
        { key: "actions", label: "", align: "right" as const },
    ];

    onMount(() => {
        staffStore.listOrganizations();
    });

    function handleSearch(e: Event) {
        const value = (e.target as HTMLInputElement).value;
        searchQuery = value;
        clearTimeout(timeout);
        timeout = setTimeout(() => {
            if (value.trim()) {
                staffStore.searchOrganizations(value);
            } else {
                staffStore.listOrganizations();
            }
        }, 300);
    }
</script>

<div class="page-header">
    <div class="header-content">
        <h1>Organizations</h1>
        <p class="subtitle">Manage customer accounts and subscriptions</p>
    </div>
    <div class="header-actions">
        <div class="search-wrapper">
            <span class="search-icon-wrapper">
                <Icon name="search" size={18} />
            </span>
            <input
                type="text"
                placeholder="Search organizations..."
                class="search-input"
                bind:value={searchQuery}
                oninput={handleSearch}
            />
        </div>
        <Button variant="default">
            <Icon name="plus" size={18} />
            <span>New Organization</span>
        </Button>
    </div>
</div>

<div class="content-card">
    {#if $staffStore.loading}
        <div class="loading-state">
            <div class="spinner"></div>
            <span>Loading organizations...</span>
        </div>
    {:else if $staffStore.error}
        <div class="error-state">
            <Icon name="alert-triangle" size={24} />
            <span>{$staffStore.error}</span>
        </div>
    {:else}
        <Table {columns} data={$staffStore.organizations}>
            {#snippet children(row: any)}
                <td>
                    <div class="org-cell">
                        <div class="org-icon">
                            <Icon name="building" size={16} />
                        </div>
                        <div class="org-info">
                            <span class="org-name">{row.name}</span>
                            <span class="org-id">{row.id}</span>
                        </div>
                    </div>
                </td>
                <td>
                    <span
                        class="status-badge"
                        class:active={row.account_status === "active"}
                    >
                        {row.account_status}
                    </span>
                </td>
                <td>
                    {new Date(row.created_at).toLocaleDateString()}
                </td>
                <td class="actions-cell">
                    <a href="/staff/organizations/{row.id}" class="action-btn">
                        View Details
                    </a>
                </td>
            {/snippet}
        </Table>
    {/if}
</div>

<style>
    .page-header {
        display: flex;
        justify-content: space-between;
        align-items: flex-end;
        margin-bottom: 2rem;
    }

    h1 {
        font-size: 2rem;
        font-weight: 700;
        margin: 0;
        background: linear-gradient(to right, #fff, #a0aec0);
        -webkit-background-clip: text;
        -webkit-text-fill-color: transparent;
    }

    .subtitle {
        color: #a0aec0;
        margin-top: 0.5rem;
    }

    .header-actions {
        display: flex;
        gap: 1rem;
    }

    .search-wrapper {
        position: relative;
        display: flex;
        align-items: center;
    }

    .search-icon-wrapper {
        position: absolute;
        left: 0.75rem;
        color: #718096;
        pointer-events: none;
        display: flex;
        align-items: center;
    }

    .search-input {
        background: #2d3748;
        border: 1px solid #4a5568;
        color: #fff;
        padding: 0.5rem 1rem 0.5rem 2.5rem;
        border-radius: 0.375rem;
        font-size: 0.9rem;
        outline: none;
        transition: all 0.2s;
        width: 250px;
    }

    .search-input:focus {
        border-color: var(--color-primary);
        box-shadow: 0 0 0 2px rgba(66, 153, 225, 0.2);
    }

    .content-card {
        background: #1a202c;
        border: 1px solid #2d3748;
        border-radius: 0.5rem;
        overflow: hidden;
    }

    .org-cell {
        display: flex;
        align-items: center;
        gap: 0.75rem;
    }

    .org-icon {
        background: #2d3748;
        padding: 0.5rem;
        border-radius: 0.375rem;
        color: #a0aec0;
    }

    .org-info {
        display: flex;
        flex-direction: column;
    }

    .org-name {
        font-weight: 500;
        color: #fff;
    }

    .org-id {
        font-size: 0.75rem;
        color: #718096;
        font-family: monospace;
    }

    .status-badge {
        display: inline-flex;
        padding: 0.25rem 0.75rem;
        border-radius: 9999px;
        font-size: 0.75rem;
        font-weight: 600;
        background: #2d3748;
        color: #a0aec0;
        text-transform: capitalize;
    }

    .status-badge.active {
        background: rgba(72, 187, 120, 0.2);
        color: #48bb78;
    }

    .actions-cell {
        text-align: right;
    }

    .action-btn {
        display: inline-block;
        padding: 0.375rem 0.75rem;
        background: #2d3748;
        color: #fff;
        border-radius: 0.375rem;
        text-decoration: none;
        font-size: 0.85rem;
        transition: background 0.2s;
    }

    .action-btn:hover {
        background: #4a5568;
    }

    .loading-state,
    .error-state {
        padding: 3rem;
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 1rem;
        color: #a0aec0;
    }

    .spinner {
        width: 24px;
        height: 24px;
        border: 2px solid #4a5568;
        border-top-color: var(--color-primary);
        border-radius: 50%;
        animation: spin 0.8s linear infinite;
    }

    @keyframes spin {
        to {
            transform: rotate(360deg);
        }
    }
</style>

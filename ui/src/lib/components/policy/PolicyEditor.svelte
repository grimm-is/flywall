<script lang="ts">
    /**
     * PolicyEditor - Main orchestrator for ClearPath Policy Editor
     * Dashboard-native styling using CSS variables
     */
    import { onMount, onDestroy } from "svelte";
    import RuleRow from "./RuleRow.svelte";
    import Icon from "../Icon.svelte";
    import { t } from "svelte-i18n";
    import {
        rulesApi,
        flatRules,
        filteredRules,
        groups,
        selectedGroup,
        isLoading,
        lastError,
        type RuleWithStats,
    } from "$lib/stores/rules";

    interface Props {
        title?: string;
        showGroupFilter?: boolean;
    }

    let { title = "Firewall Rules", showGroupFilter = true }: Props = $props();

    // Load data on mount
    onMount(() => {
        rulesApi.loadGroups();
        rulesApi.startStatsPolling();
    });

    // Cleanup on destroy
    onDestroy(() => {
        rulesApi.stopStatsPolling();
    });

    function handleGroupSelect(group: string | null) {
        rulesApi.selectGroup(group);
    }

    async function handleToggleRule(id: string, disabled: boolean) {
        try {
            await rulesApi.toggleRule(id, disabled);
        } catch (e) {
            alert(
                `Failed to toggle rule: ${e instanceof Error ? e.message : e}`,
            );
        }
    }

    async function handleDeleteRule(id: string) {
        if (confirm("Delete this rule?")) {
            try {
                await rulesApi.deleteRule(id);
            } catch (e) {
                alert(
                    `Failed to delete rule: ${e instanceof Error ? e.message : e}`,
                );
            }
        }
    }

    async function handleDuplicateRule(rule: RuleWithStats) {
        try {
            const newRule = {
                ...rule,
                name: `${rule.name || "Rule"} (Copy)`,
                description: rule.description,
            };
            delete (newRule as any).id;
            delete (newRule as any).stats;
            await rulesApi.createRule(newRule);
        } catch (e) {
            alert(
                `Failed to duplicate rule: ${e instanceof Error ? e.message : e}`,
            );
        }
    }

    function handleEditRule(rule: RuleWithStats) {
        // TODO: Open modal for editing
        alert(
            `Edit rule: ${rule.name || rule.id}\n\nRule editor modal coming soon.`,
        );
    }

    function handleCreateRule() {
        // TODO: Open modal for creating
        alert("Create Rule\n\nRule creator modal coming soon.");
    }
</script>

<div class="policy-editor">
    <!-- Header -->
    <div class="editor-header">
        <h2 class="editor-title">{title}</h2>

        <div class="header-actions">
            <!-- Group Filter -->
            {#if showGroupFilter && $groups.length > 0}
                <div class="group-filter">
                    <button
                        class="filter-btn"
                        class:active={!$selectedGroup}
                        onclick={() => handleGroupSelect(null)}
                    >
                        {$t("policy.all")}
                    </button>
                    {#each $groups as group (group.name)}
                        <button
                            class="filter-btn"
                            class:active={$selectedGroup === group.name}
                            onclick={() => handleGroupSelect(group.name)}
                        >
                            {group.name}
                            <span class="group-count">({group.count})</span>
                        </button>
                    {/each}
                </div>
            {/if}

            <!-- Add Rule Button -->
            <button class="btn-primary" onclick={handleCreateRule}>
                <Icon name="add" size="sm" />
                {$t("policy.add_rule")}
            </button>
        </div>
    </div>

    <!-- Content -->
    <div class="editor-content">
        {#if $isLoading && $flatRules.length === 0}
            <!-- Loading State -->
            <div class="state-container">
                <div class="loading-pulse">{$t("policy.loading_rules")}</div>
            </div>
        {:else if $lastError}
            <!-- Error State -->
            <div class="state-container">
                <div class="error-message">{$lastError}</div>
            </div>
        {:else if $filteredRules.length === 0}
            <!-- Empty State -->
            <div class="state-container empty-state">
                <Icon name="inbox" size="lg" />
                <p>{$t("policy.no_rules")}</p>
                <button class="btn-primary" onclick={handleCreateRule}>
                    {$t("policy.create_first_rule")}
                </button>
            </div>
        {:else}
            <!-- Rules List -->
            <div class="rules-list">
                {#each $filteredRules as rule (rule.id || `${rule.policy_from}-${rule.policy_to}-${rule.name}`)}
                    <RuleRow
                        {rule}
                        onToggle={handleToggleRule}
                        onEdit={handleEditRule}
                        onDelete={handleDeleteRule}
                        onDuplicate={handleDuplicateRule}
                    />
                {/each}
            </div>
        {/if}
    </div>

    <!-- Footer -->
    <div class="editor-footer">
        <div class="rule-count">
            {$filteredRules.length} rule{$filteredRules.length !== 1 ? "s" : ""}
            {#if $selectedGroup}
                in "{$selectedGroup}"
            {/if}
        </div>
        <div class="live-indicator">
            {#if $isLoading}
                <span class="loading-pulse">{$t("policy.updating")}</span>
            {:else}
                <span>{$t("policy.live_stats")}</span>
            {/if}
            <span class="pulse-dot"></span>
        </div>
    </div>
</div>

<style>
    .policy-editor {
        display: flex;
        flex-direction: column;
        height: 100%;
        background: var(--dashboard-card);
        border-radius: var(--radius-lg);
        overflow: hidden;
    }

    /* Header */
    .editor-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: var(--space-3) var(--space-4);
        border-bottom: 1px solid var(--dashboard-border);
        background: var(--dashboard-canvas);
    }

    .editor-title {
        font-size: var(--text-lg);
        font-weight: 600;
        color: var(--dashboard-text);
        margin: 0;
    }

    .header-actions {
        display: flex;
        align-items: center;
        gap: var(--space-3);
    }

    /* Group Filter */
    .group-filter {
        display: flex;
        align-items: center;
        gap: var(--space-2);
    }

    .filter-btn {
        padding: var(--space-1) var(--space-3);
        font-size: var(--text-xs);
        border-radius: var(--radius-md);
        border: 1px solid var(--dashboard-border);
        background: var(--dashboard-input);
        color: var(--dashboard-text-muted);
        cursor: pointer;
        transition: all var(--transition-fast);
    }

    .filter-btn:hover {
        background: var(--dashboard-border);
        color: var(--dashboard-text);
    }

    .filter-btn.active {
        background: var(--color-primary);
        color: var(--color-primaryForeground);
        border-color: var(--color-primary);
    }

    .group-count {
        opacity: 0.6;
        margin-left: var(--space-1);
    }

    /* Primary Button */
    .btn-primary {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-2) var(--space-3);
        background: var(--color-primary);
        color: var(--color-primaryForeground);
        border: none;
        border-radius: var(--radius-md);
        font-size: var(--text-sm);
        font-weight: 500;
        cursor: pointer;
        transition: all var(--transition-fast);
    }

    .btn-primary:hover {
        filter: brightness(1.1);
    }

    /* Content */
    .editor-content {
        flex: 1;
        overflow-y: auto;
    }

    .rules-list {
        display: flex;
        flex-direction: column;
    }

    /* States */
    .state-container {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        height: 100%;
        min-height: 200px;
        gap: var(--space-4);
        color: var(--dashboard-text-muted);
    }

    .empty-state {
        padding: var(--space-8);
    }

    .loading-pulse {
        animation: pulse 2s ease-in-out infinite;
    }

    @keyframes pulse {
        0%,
        100% {
            opacity: 1;
        }
        50% {
            opacity: 0.5;
        }
    }

    .error-message {
        padding: var(--space-3) var(--space-4);
        background: rgba(239, 68, 68, 0.1);
        border: 1px solid var(--color-destructive);
        border-radius: var(--radius-md);
        color: var(--color-destructive);
    }

    /* Footer */
    .editor-footer {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: var(--space-2) var(--space-4);
        border-top: 1px solid var(--dashboard-border);
        background: var(--dashboard-canvas);
        font-size: var(--text-xs);
        color: var(--dashboard-text-muted);
    }

    .live-indicator {
        display: flex;
        align-items: center;
        gap: var(--space-2);
    }

    .pulse-dot {
        width: 8px;
        height: 8px;
        border-radius: var(--radius-full);
        background: var(--color-success);
        animation: pulse 2s ease-in-out infinite;
    }
</style>

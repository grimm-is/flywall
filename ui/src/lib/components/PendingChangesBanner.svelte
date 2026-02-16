<script lang="ts">
    import { api, hasPendingChanges } from "$lib/stores/app";
    import { t } from "svelte-i18n";
    import Button from "./Button.svelte";
    import Spinner from "./Spinner.svelte";
    import Icon from "./Icon.svelte";

    let loading = $state(false);

    async function applyChanges() {
        loading = true;
        try {
            await api.applyConfig();
        } catch (e) {
            console.error("Failed to apply changes", e);
            alert("Failed to apply changes: " + e);
        } finally {
            loading = false;
        }
    }

    async function discardChanges() {
        if (!confirm($t("common.confirm_discard"))) return;
        loading = true;
        try {
            await api.discardConfig();
        } catch (e) {
            console.error("Failed to discard changes", e);
        } finally {
            loading = false;
        }
    }
</script>

{#if $hasPendingChanges}
    <div class="pending-banner">
        <div class="banner-content">
            <Icon name="warning" size={20} class="banner-icon" />
            <span class="banner-text">
                {$t("common.pending_changes_message", {
                    default: "You have pending configuration changes.",
                })}
            </span>
        </div>
        <div class="banner-actions">
            <Button
                variant="ghost"
                size="sm"
                onclick={discardChanges}
                disabled={loading}
            >
                {$t("common.discard", { default: "Discard" })}
            </Button>
            <Button
                variant="default"
                size="sm"
                onclick={applyChanges}
                disabled={loading}
            >
                {#if loading}
                    <Spinner size="sm" />
                {/if}
                {$t("common.apply", { default: "Apply Changes" })}
            </Button>
        </div>
    </div>
{/if}

<style>
    .pending-banner {
        background-color: var(--color-warning-soft, #fffbeb);
        border: 1px solid var(--color-warning-border, #fcd34d);
        color: var(--color-warning-text, #92400e);
        padding: var(--space-3) var(--space-4);
        display: flex;
        align-items: center;
        justify-content: space-between;
        border-radius: var(--radius-md);
        margin-bottom: var(--space-4);
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);
    }

    /* Dark mode adjustments if variables aren't global */
    :global(.dark) .pending-banner {
        background-color: rgba(245, 158, 11, 0.1);
        border-color: rgba(245, 158, 11, 0.2);
        color: var(--color-warning, #fbbf24);
    }

    .banner-content {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        font-weight: 500;
    }

    .banner-actions {
        display: flex;
        gap: var(--space-2);
    }

    /* Mobile handling */
    @media (max-width: 640px) {
        .pending-banner {
            flex-direction: column;
            gap: var(--space-3);
            align-items: stretch;
            text-align: center;
        }

        .banner-content {
            justify-content: center;
        }

        .banner-actions {
            justify-content: center;
        }
    }
</style>

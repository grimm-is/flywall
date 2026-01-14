<script lang="ts">
    import { t } from "svelte-i18n";
</script>

<div class="loading-view">
    <div class="loading-content">
        <div class="loading-icon">🛡️</div>
        <p class="loading-text">{$t("common.loading")}</p>
    </div>
</div>

<style>
    .loading-view {
        min-height: 100vh;
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: var(--color-background);
    }

    .loading-content {
        text-align: center;
    }

    .loading-icon {
        font-size: 3rem;
        margin-bottom: var(--space-4);
    }

    .loading-text {
        color: var(--color-muted);
    }
</style>

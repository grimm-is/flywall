<script lang="ts">
    import {
        isAnonymized,
        toggle,
        anonymizeEnabled,
    } from "$lib/utils/anonymize";
    import { t } from "svelte-i18n";

    function handleClick() {
        toggle();
    }

    function handleKeydown(e: KeyboardEvent) {
        // Ctrl/Cmd + Shift + A to toggle anonymization
        if (
            (e.ctrlKey || e.metaKey) &&
            e.shiftKey &&
            e.key.toLowerCase() === "a"
        ) {
            e.preventDefault();
            toggle();
        }
    }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if $anonymizeEnabled}
    <button
        class="anonymize-btn"
        class:active={$isAnonymized}
        onclick={handleClick}
        title={$isAnonymized
            ? "Click to restore original data (Ctrl+Shift+A)"
            : "Click to anonymize for screenshots (Ctrl+Shift+A)"}
        aria-label={$isAnonymized
            ? "Restore original data"
            : "Anonymize for screenshots"}
    >
        {#if $isAnonymized}
            <svg
                xmlns="http://www.w3.org/2000/svg"
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
            >
                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                <circle cx="12" cy="12" r="3" />
            </svg>
            <span class="btn-text">Restore</span>
        {:else}
            <svg
                xmlns="http://www.w3.org/2000/svg"
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
            >
                <path
                    d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"
                />
                <line x1="1" y1="1" x2="23" y2="23" />
            </svg>
            <span class="btn-text">Anonymize</span>
        {/if}
    </button>
{/if}

<style>
    .anonymize-btn {
        display: inline-flex;
        align-items: center;
        gap: 0.5rem;
        padding: 0.5rem 0.75rem;
        border: 1px solid var(--border-color, #ddd);
        border-radius: 0.375rem;
        background: var(--card-bg, #fff);
        color: var(--text-color, #333);
        font-size: 0.875rem;
        font-weight: 500;
        cursor: pointer;
        transition: all 0.15s ease;
    }

    .anonymize-btn:hover {
        background: var(--hover-bg, #f5f5f5);
        border-color: var(--primary-color, #3b82f6);
    }

    .anonymize-btn.active {
        background: var(--warning-bg, #fef3c7);
        border-color: var(--warning-color, #f59e0b);
        color: var(--warning-dark, #92400e);
    }

    .anonymize-btn svg {
        flex-shrink: 0;
    }

    .btn-text {
        white-space: nowrap;
    }

    /* Hide text on very small screens */
    @media (max-width: 480px) {
        .btn-text {
            display: none;
        }
        .anonymize-btn {
            padding: 0.5rem;
        }
    }
</style>

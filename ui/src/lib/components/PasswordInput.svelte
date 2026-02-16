<script lang="ts">
    import Input from "$lib/components/Input.svelte";
    import Button from "$lib/components/Button.svelte";
    import { t } from "svelte-i18n";

    interface Props {
        id?: string;
        value?: string;
        placeholder?: string;
        label?: string;
        error?: string;
        disabled?: boolean;
        required?: boolean;
        showComplexity?: boolean;
        class?: string;
        username?: string;
    }

    let {
        id = "",
        value = $bindable(""),
        placeholder = "",
        label = "",
        error = "",
        disabled = false,
        required = false,
        showComplexity = false,
        class: className = "",
        username = "",
        ...rest
    }: Props = $props();

    let visible = $state(false);

    function toggleVisibility() {
        visible = !visible;
    }

    // Complexity logic from Setup.svelte
    function calculateComplexity(
        password: string,
        user: string,
    ): {
        score: number;
        label: string;
        color: string;
    } {
        if (!password) return { score: 0, label: "", color: "bg-gray-200" };

        let score = 0;
        const length = password.length;

        // 1. Check Character Classes
        if (/[a-z]/.test(password)) score += 10;
        if (/[A-Z]/.test(password)) score += 10;
        if (/[0-9]/.test(password)) score += 10;
        if (/[^A-Za-z0-9]/.test(password)) score += 15;

        // 2. Length Base Score
        score += length * 4;

        // 3. Apply Multipliers
        if (length < 8) {
            score *= 0.5;
        } else if (length >= 16) {
            score *= 1.5;
        }

        // 4. Deductions (The "Lazy" Checks)
        const lower = password.toLowerCase();
        if (lower === "password" || password === "12345678") {
            score = 0;
        }

        // Deduction for containing username
        if (user && user.length > 0 && lower.includes(user.toLowerCase())) {
            score = 0;
        }

        // Clamp between 0-100
        score = Math.min(Math.max(score, 0), 100);

        if (score < 40) return { score, label: "Weak", color: "bg-red-500" };
        if (score < 70)
            return { score, label: "Medium", color: "bg-yellow-500" };
        return { score, label: "Strong", color: "bg-green-500" };
    }

    let complexity = $derived(calculateComplexity(value, username));
</script>

<div class="password-input-wrapper {className}">
    <Input
        {id}
        type={visible ? "text" : "password"}
        {label}
        bind:value
        {placeholder}
        {disabled}
        {required}
        {error}
        {...rest}
    >
        {#snippet suffix()}
            <button
                type="button"
                class="visibility-toggle"
                onclick={toggleVisibility}
                aria-label={visible ? "Hide password" : "Show password"}
                tabindex="-1"
            >
                {#if visible}
                    <!-- Eye Off Icon -->
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="20"
                        height="20"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                    >
                        <path
                            d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"
                        ></path>
                        <line x1="1" y1="1" x2="23" y2="23"></line>
                    </svg>
                {:else}
                    <!-- Eye Icon -->
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="20"
                        height="20"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                    >
                        <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"
                        ></path>
                        <circle cx="12" cy="12" r="3"></circle>
                    </svg>
                {/if}
            </button>
        {/snippet}
    </Input>

    {#if showComplexity && value}
        <div class="complexity-meter">
            <div class="progress-bg">
                <div
                    class="progress-fill {complexity.color}"
                    style="width: {complexity.score}%"
                ></div>
            </div>
            <span class="complexity-label">{complexity.label}</span>
            <span class="complexity-text-label">Password Strength</span>
        </div>
    {/if}
</div>

<style>
    .password-input-wrapper {
        display: flex;
        flex-direction: column;
        width: 100%;
    }

    .visibility-toggle {
        background: none;
        border: none;
        cursor: pointer;
        color: var(--color-muted);
        padding: var(--space-1);
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: var(--radius-sm);
        transition: color 0.2s;
    }

    .visibility-toggle:hover {
        color: var(--color-foreground);
        background-color: var(--color-backgroundSecondary);
    }

    /* Complexity Meter Styles */
    .complexity-meter {
        margin-top: var(--space-2);
        margin-bottom: var(--space-1);
    }

    .progress-bg {
        height: 4px;
        background-color: var(--color-border);
        border-radius: var(--radius-sm);
        overflow: hidden;
        margin-bottom: var(--space-1);
    }

    .progress-fill {
        height: 100%;
        transition:
            width 0.3s ease,
            background-color 0.3s ease;
    }

    .complexity-label {
        font-size: var(--text-xs);
        font-weight: 600;
        color: var(--color-foreground);
        margin-right: 8px;
    }

    .complexity-text-label {
        font-size: var(--text-xs);
        color: var(--color-muted);
    }

    /* Utility classes for complexity colors */
    :global(.bg-gray-200) {
        background-color: var(--color-border);
    }
    :global(.bg-red-500) {
        background-color: var(--color-destructive);
    }
    :global(.bg-yellow-500) {
        background-color: var(--color-warning);
    }
    :global(.bg-green-500) {
        background-color: var(--color-success);
    }
</style>

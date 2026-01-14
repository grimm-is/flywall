<script lang="ts">
    import { api, currentView, brand } from "$lib/stores/app";
    import Button from "$lib/components/Button.svelte";
    import Input from "$lib/components/Input.svelte";
    import Card from "$lib/components/Card.svelte";
    import { t } from "svelte-i18n";

    let setupUsername = $state("admin");
    let setupPassword = $state("");
    let setupConfirm = $state("");
    let setupError = $state("");

    async function handleSetup() {
        setupError = "";
        if (setupPassword !== setupConfirm) {
            setupError = "Passwords do not match";
            return;
        }
        if (setupPassword.length < 8) {
            setupError = "Password must be at least 8 characters";
            return;
        }
        try {
            await api.createAdmin(setupUsername, setupPassword);
            await api.loadDashboard();
            currentView.set("app");
        } catch (e: any) {
            setupError = e.message || "Setup failed";
        }
    }
</script>

<div class="auth-view">
    <div class="auth-container">
        <div class="auth-header">
            <div class="auth-icon">🛡️</div>
            <h1 class="auth-title">
                {$t("auth.setup_title", { values: { name: $brand?.name } })}
            </h1>
            <p class="auth-subtitle">{$t("auth.setup_subtitle")}</p>
        </div>

        <Card class="auth-card">
            <form
                onsubmit={(e) => {
                    e.preventDefault();
                    handleSetup();
                }}
            >
                <div class="form-stack">
                    <Input
                        id="setup-username"
                        label={$t("auth.username")}
                        bind:value={setupUsername}
                        placeholder={$t("auth.username_placeholder")}
                        required
                    />

                    <Input
                        id="setup-password"
                        type="password"
                        label={$t("auth.password")}
                        bind:value={setupPassword}
                        placeholder={$t("auth.password_min_chars")}
                        required
                    />

                    <Input
                        id="setup-confirm"
                        type="password"
                        label={$t("auth.confirm_password")}
                        bind:value={setupConfirm}
                        placeholder={$t("auth.confirm_password_placeholder")}
                        required
                    />

                    {#if setupError}
                        <div class="error-message">{setupError}</div>
                    {/if}

                    <Button type="submit">{$t("auth.create_account")}</Button>
                </div>
            </form>
        </Card>
    </div>
</div>

<style>
    .auth-view {
        min-height: 100vh;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: var(--space-4);
        background-color: var(--color-background);
    }

    .auth-container {
        width: 100%;
        max-width: 400px;
    }

    .auth-header {
        text-align: center;
        margin-bottom: var(--space-6);
    }

    .auth-icon {
        font-size: 3rem;
        margin-bottom: var(--space-4);
    }

    .auth-title {
        font-size: var(--text-2xl);
        font-weight: 700;
        color: var(--color-foreground);
        margin: 0 0 var(--space-2) 0;
    }

    .auth-subtitle {
        color: var(--color-muted);
        margin: 0;
    }

    .form-stack {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
    }

    .error-message {
        padding: var(--space-3);
        background-color: rgba(239, 68, 68, 0.1);
        border: 1px solid var(--color-destructive);
        border-radius: var(--radius-md);
        color: var(--color-destructive);
        font-size: var(--text-sm);
    }
</style>

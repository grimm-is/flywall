<script lang="ts">
    import { api, currentView, brand } from "$lib/stores/app";
    import Button from "$lib/components/Button.svelte";
    import Input from "$lib/components/Input.svelte";
    import PasswordInput from "$lib/components/PasswordInput.svelte";
    import Card from "$lib/components/Card.svelte";
    import { t } from "svelte-i18n";

    let loginUsername = $state("");
    let loginPassword = $state("");
    let loginError = $state("");

    async function handleLogin() {
        loginError = "";
        try {
            await api.login(loginUsername, loginPassword);
            await api.loadDashboard();
            currentView.set("app");
        } catch (e: any) {
            loginError = e.message || "Login failed";
        }
    }
</script>

<div class="auth-view">
    <div class="auth-container">
        <div class="auth-header">
            <div class="auth-icon">🛡️</div>
            <h1 class="auth-title">{$brand?.name}</h1>
            <p class="auth-subtitle">{$t("auth.signin_subtitle")}</p>
        </div>

        <Card class="auth-card">
            <form
                onsubmit={(e) => {
                    e.preventDefault();
                    handleLogin();
                }}
            >
                <div class="form-stack">
                    <Input
                        id="login-username"
                        label={$t("auth.username")}
                        bind:value={loginUsername}
                        placeholder={$t("auth.username")}
                        required
                    />

                    <PasswordInput
                        id="login-password"
                        label={$t("auth.password")}
                        bind:value={loginPassword}
                        placeholder={$t("auth.password")}
                        required
                    />

                    {#if loginError}
                        <div class="error-message">{loginError}</div>
                    {/if}

                    <Button type="submit">{$t("auth.login")}</Button>
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

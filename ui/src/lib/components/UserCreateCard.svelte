<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { Card, Button, Input, Select, Spinner } from "$lib/components";
    import { t } from "svelte-i18n";

    export let loading = false;

    const dispatch = createEventDispatcher();

    let username = "";
    let password = "";
    let role = "admin";

    let errors = { username: "", password: "" };

    function validate() {
        let valid = true;
        errors = { username: "", password: "" };

        if (!username) {
            errors.username =
                $t("auth.username_required") || "Username is required";
            valid = false;
        } else if (username.length < 3) {
            errors.username = "Minimum 3 characters";
            valid = false;
        }

        if (!password) {
            errors.password =
                $t("auth.password_required") || "Password is required";
            valid = false;
        } else if (password.length < 8) {
            errors.password =
                $t("auth.password_min_chars") || "Minimum 8 characters";
            valid = false;
        }

        return valid;
    }

    function handleSave() {
        if (!validate()) return;
        dispatch("save", { username, password, role });
    }

    function handleCancel() {
        dispatch("cancel");
    }
</script>

<Card>
    <div class="create-card">
        <div class="header">
            <h3>
                {$t("common.create_item", {
                    values: { item: $t("item.user") },
                })}
            </h3>
        </div>

        <div class="form-stack">
            <Input
                label={$t("auth.username")}
                bind:value={username}
                error={errors.username}
                placeholder="username"
            />
            <Input
                label={$t("settings.password")}
                type="password"
                bind:value={password}
                error={errors.password}
                placeholder="********"
            />

            <!-- Future: Role selector if multiple roles supported -->

            <div class="actions">
                <Button
                    variant="ghost"
                    onclick={handleCancel}
                    disabled={loading}
                >
                    {$t("common.cancel")}
                </Button>
                <Button onclick={handleSave} disabled={loading}>
                    {#if loading}<Spinner size="sm" />{/if}
                    {$t("common.create")}
                </Button>
            </div>
        </div>
    </div>
</Card>

<style>
    .create-card {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
        padding: var(--space-2);
    }
    .header h3 {
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
    }
    .form-stack {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
    }
    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        margin-top: var(--space-2);
        border-top: 1px solid var(--color-border);
        padding-top: var(--space-4);
    }
</style>

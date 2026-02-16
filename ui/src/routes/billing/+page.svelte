<script lang="ts">
    import { onMount } from "svelte";
    import { billingStore } from "$lib/stores/billing";
    import Button from "$lib/components/Button.svelte";
    import Icon from "$lib/components/Icon.svelte";

    let billingCycle = $state<"monthly" | "yearly">("monthly");

    import { goto } from "$app/navigation";
    import { authStatus } from "$lib/stores/app";

    onMount(() => {
        if (!$authStatus?.is_staff) {
            goto("/");
            return;
        }
        billingStore.getSubscription();
        billingStore.listPlans();
    });

    async function handleSelectPlan(planId: string) {
        try {
            await billingStore.createSubscription(planId, billingCycle);
        } catch (e: any) {
            alert("Failed to update plan: " + e.message);
        }
    }

    async function handleCancel() {
        if (
            !confirm(
                "Are you sure you want to cancel? You will lose access at the end of the billing period.",
            )
        )
            return;
        try {
            await billingStore.cancelSubscription();
        } catch (e: any) {
            alert("Failed to cancel: " + e.message);
        }
    }

    let currentSub = $derived($billingStore.subscription);
    let plans = $derived($billingStore.plans);
    let isLoading = $derived($billingStore.loading);
</script>

<div class="billing-page">
    {#if isLoading && !currentSub}
        <div class="loading-state">
            <div class="spinner"></div>
            Loading billing info...
        </div>
    {:else}
        <!-- Current Plan -->
        <section class="current-plan-section">
            <h2>Current Plan</h2>
            <div class="current-card">
                <div class="plan-details">
                    {#if currentSub && currentSub.status === "active"}
                        <h3>
                            {currentSub.plan?.display_name || "Active Plan"}
                        </h3>
                        <p class="status">
                            <span class="status-dot healthy"></span>
                            Active until {new Date(
                                currentSub.current_period_end,
                            ).toLocaleDateString()}
                        </p>
                        {#if currentSub.cancel_at_period_end}
                            <p class="warning">Cancels at end of period</p>
                        {/if}
                    {:else}
                        <h3>Free Tier</h3>
                        <p class="status">
                            <span class="status-dot"></span> Active
                        </p>
                    {/if}
                </div>
                <div class="plan-actions">
                    {#if currentSub && !currentSub.cancel_at_period_end}
                        <Button variant="destructive" onclick={handleCancel}
                            >Cancel Plan</Button
                        >
                    {/if}
                </div>
            </div>
        </section>

        <!-- Available Plans -->
        <section class="plans-section">
            <div class="plans-header">
                <h2>Available Plans</h2>
                <div class="cycle-toggle">
                    <button
                        class="toggle-btn"
                        class:active={billingCycle === "monthly"}
                        onclick={() => (billingCycle = "monthly")}
                        >Monthly</button
                    >
                    <button
                        class="toggle-btn"
                        class:active={billingCycle === "yearly"}
                        onclick={() => (billingCycle = "yearly")}
                        >Yearly <span class="discount">-20%</span></button
                    >
                </div>
            </div>

            <div class="plans-grid">
                {#each plans as plan}
                    <div
                        class="plan-card"
                        class:active={currentSub?.plan_id === plan.id}
                    >
                        <div class="plan-header">
                            <h3>{plan.display_name}</h3>
                            <div class="price">
                                <span class="amount">
                                    ${(
                                        (billingCycle === "monthly"
                                            ? plan.price_monthly
                                            : plan.price_yearly ||
                                              plan.price_monthly * 12) / 100
                                    ).toFixed(2)}
                                </span>
                                <span class="period"
                                    >/{billingCycle === "monthly"
                                        ? "mo"
                                        : "yr"}</span
                                >
                            </div>
                        </div>
                        <ul class="features">
                            <!-- Placeholder features until DB has them -->
                            <li>Audit Logs</li>
                            <li>Priority Support</li>
                            {#if plan.name === "enterprise"}
                                <li>SSO / SAML</li>
                                <li>Unlimited Devices</li>
                            {:else}
                                <li>Up to 100 Devices</li>
                            {/if}
                        </ul>
                        <div class="card-footer">
                            {#if currentSub?.plan_id === plan.id}
                                <Button disabled variant="outline"
                                    >Current Plan</Button
                                >
                            {:else}
                                <Button
                                    variant="default"
                                    onclick={() => handleSelectPlan(plan.id)}
                                >
                                    {currentSub ? "Switch Plan" : "Subscribe"}
                                </Button>
                            {/if}
                        </div>
                    </div>
                {/each}
            </div>
        </section>
    {/if}
</div>

<style>
    .billing-page {
        animation: fade-in 0.3s ease;
    }

    h2 {
        font-size: 1.25rem;
        font-weight: 600;
        margin-bottom: 1rem;
        color: var(--color-foreground);
    }

    .current-plan-section,
    .plans-section {
        margin-bottom: 3rem;
    }

    .current-card {
        background: var(--color-surface);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-lg);
        padding: 1.5rem;
        display: flex;
        justify-content: space-between;
        align-items: center;
    }

    .plan-details h3 {
        margin: 0;
        font-size: 1.5rem;
        margin-bottom: 0.5rem;
    }

    .status {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        color: var(--color-muted);
    }

    .status-dot {
        width: 8px;
        height: 8px;
        background: #ccc;
        border-radius: 50%;
    }

    .status-dot.healthy {
        background: var(--color-success);
    }

    .warning {
        color: var(--color-warning);
        font-size: 0.9rem;
        margin-top: 0.5rem;
    }

    .plans-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 1.5rem;
    }

    .cycle-toggle {
        background: var(--color-surface);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        padding: 0.25rem;
        display: flex;
        gap: 0.25rem;
    }

    .toggle-btn {
        padding: 0.5rem 1rem;
        border: none;
        background: none;
        color: var(--color-muted);
        cursor: pointer;
        border-radius: var(--radius-sm);
        font-weight: 500;
        transition: all 0.2s;
    }

    .toggle-btn.active {
        background: var(--color-surfaceHover);
        color: var(--color-foreground);
        box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
    }

    .discount {
        font-size: 0.75rem;
        background: var(--color-success);
        color: #fff;
        padding: 0.1rem 0.3rem;
        border-radius: 4px;
        margin-left: 0.25rem;
    }

    .plans-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
        gap: 1.5rem;
    }

    .plan-card {
        background: var(--color-surface);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-lg);
        padding: 2rem;
        display: flex;
        flex-direction: column;
        transition: all 0.2s;
    }

    .plan-card:hover {
        border-color: var(--color-primary);
        transform: translateY(-2px);
    }

    .plan-card.active {
        border-color: var(--color-primary);
        box-shadow: 0 0 0 2px rgba(var(--color-primary-rgb), 0.2);
    }

    .plan-header {
        text-align: center;
        margin-bottom: 2rem;
    }

    .plan-header h3 {
        color: var(--color-muted);
        font-size: 1.1rem;
        margin-bottom: 0.5rem;
    }

    .price {
        display: flex;
        justify-content: center;
        align-items: baseline;
    }

    .amount {
        font-size: 2.5rem;
        font-weight: 800;
        color: var(--color-foreground);
    }

    .period {
        color: var(--color-muted);
        margin-left: 0.25rem;
    }

    .features {
        list-style: none;
        padding: 0;
        margin: 0 0 2rem 0;
        flex: 1;
    }

    .features li {
        padding: 0.5rem 0;
        border-bottom: 1px solid var(--color-border);
        color: var(--color-foreground);
        text-align: center;
    }

    .features li:last-child {
        border-bottom: none;
    }

    .card-footer {
        display: flex;
        justify-content: center;
    }

    .loading-state {
        padding: 4rem;
        text-align: center;
        color: var(--color-muted);
    }

    .spinner {
        width: 32px;
        height: 32px;
        border: 2px solid var(--color-border);
        border-top-color: var(--color-primary);
        border-radius: 50%;
        margin: 0 auto 1rem auto;
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

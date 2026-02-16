<script>
    import Alerts from "$lib/pages/Alerts.svelte";
</script>

<svelte:head>
    <title>Alerts | Flywall</title>
</svelte:head>

<div class="page-container">
    <div class="mb-6">
        <h1 class="text-2xl font-semibold mb-2">System Alerts</h1>
        <p class="text-muted-foreground">
            View and manage system notifications and alert history.
        </p>
    </div>

    <Alerts />
</div>

<script>
    import AuditLog from "$lib/pages/AuditLog.svelte";
</script>

<svelte:head>
    <title>Audit Log | Flywall</title>
</svelte:head>

<div class="page-container">
    <div class="mb-6">
        <h1 class="text-2xl font-semibold mb-2">Audit Log</h1>
        <p class="text-muted-foreground">
            View system activity and configuration changes.
        </p>
    </div>

    <AuditLog />
</div>

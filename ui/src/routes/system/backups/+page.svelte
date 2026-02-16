<script>
    import Backups from "$lib/pages/Backups.svelte";
</script>

<svelte:head>
    <title>Backups | Flywall</title>
</svelte:head>

<div class="page-container">
    <div class="mb-6">
        <h1 class="text-2xl font-semibold mb-2">Backups</h1>
        <p class="text-muted-foreground">
            Manage system configuration backups and restores.
        </p>
    </div>

    <Backups />
</div>

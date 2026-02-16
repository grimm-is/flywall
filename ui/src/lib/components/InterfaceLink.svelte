<script lang="ts">
    import { t } from "svelte-i18n";

    let { name, className = "" } = $props<{
        name: string;
        className?: string;
    }>();
</script>

<a
    href="/network?tab=interfaces#interface-{name}"
    class="interface-link {className}"
    title={$t("common.view_details")}
>
    {name}
</a>

<style>
    .interface-link {
        color: var(--color-primary);
        text-decoration: none;
        font-family: var(--font-mono);
        cursor: pointer;
    }

    .interface-link:hover {
        text-decoration: underline;
    }
</style>

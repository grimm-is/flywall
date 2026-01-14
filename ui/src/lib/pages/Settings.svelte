<script lang="ts">
    /**
     * Settings Page
     * Global firewall settings
     */

    import { onMount } from "svelte";
    import { config, api, alertStore } from "$lib/stores/app";
    import {
        Card,
        Button,
        Toggle,
        Badge,
        Spinner,
        Table,
        Modal,
        Input,
    } from "$lib/components";
    import { t } from "svelte-i18n";

    let loading = $state(false);
    let users = $state<User[]>([]);
    let showUserModal = $state(false);
    let newUser = $state({ username: "", password: "", role: "admin" });

    interface User {
        username: string;
        role: string;
    }

    // Local state for toggles to ensure immediate feedback and prevent flickering
    let localIPForwarding = $state($config?.ip_forwarding ?? false);
    let localMSSClamping = $state($config?.mss_clamping ?? false);
    let localFlowOffload = $state($config?.enable_flow_offload ?? false);

    let updatingStates = $state<Record<string, boolean>>({});

    // Sync with config store
    $effect(() => {
        if (!$config) return;

        // Use untrack for the local state updates to prevent reactivity loops
        // although we depend on $config and updatingStates
        if (!updatingStates["ip_forwarding"]) {
            localIPForwarding = $config.ip_forwarding ?? false;
        }
        if (!updatingStates["mss_clamping"]) {
            localMSSClamping = $config.mss_clamping ?? false;
        }
        if (!updatingStates["enable_flow_offload"]) {
            localFlowOffload = $config.enable_flow_offload ?? false;
        }
    });

    async function loadUsers() {
        try {
            users = await api.getUsers();
        } catch (e) {
            console.error("Failed to load users", e);
        }
    }

    async function updateSetting(key: string, value: boolean) {
        console.log(`[SETTINGS] updateSetting: ${key} = ${value}`);
        updatingStates[key] = true;
        loading = true;
        try {
            const payload = { [key]: value };
            await api.updateSettings(payload);
        } catch (e) {
            console.error(`Failed to update ${key}:`, e);
            // Revert local state on error
            if (key === "ip_forwarding") localIPForwarding = !value;
            if (key === "mss_clamping") localMSSClamping = !value;
            if (key === "enable_flow_offload") localFlowOffload = !value;
        } finally {
            updatingStates[key] = false;
            loading = false;
        }
    }

    async function handleCreateUser() {
        try {
            await api.createUser(
                newUser.username,
                newUser.password,
                newUser.role,
            );
            showUserModal = false;
            newUser = { username: "", password: "", role: "admin" };
            alertStore.success($t("settings.user_create_success"));
        } catch (e: any) {
            alertStore.error(e.message || $t("settings.user_create_failed"));
        }
    }

    // System & Backups Logic
    let safeMode = $state(false);
    let backups = $state<any[]>([]);

    async function checkSafeMode() {
        try {
            const status = await api.getSafeModeStatus();
            safeMode = status.in_safe_mode;
        } catch (e) {
            console.error("Failed to check safe mode", e);
        }
    }

    async function handleSafeModeToggle() {
        try {
            if (safeMode) {
                await api.exitSafeMode();
                alertStore.success($t("settings.safe_mode_exited"));
            } else {
                if (!confirm($t("settings.safe_mode_confirm"))) return;
                await api.enterSafeMode();
                alertStore.success($t("settings.safe_mode_entered"));
            }
            checkSafeMode();
        } catch (e: any) {
            alertStore.error(e.message || $t("settings.safe_mode_failed"));
        }
    }

    async function handleReboot() {
        if (!confirm($t("settings.reboot_confirm"))) return;
        try {
            await api.reboot();
            alertStore.success($t("settings.reboot_initiated"));
        } catch (e: any) {
            alertStore.error(e.message || $t("settings.reboot_failed"));
        }
    }

    async function loadBackups() {
        try {
            backups = await api.listBackups();
        } catch (e) {
            console.error("Failed to load backups", e);
        }
    }

    async function handleCreateBackup() {
        const desc = prompt($t("settings.backup_create_description"));
        if (desc === null) return;
        try {
            await api.createBackup(desc);
            loadBackups();
            alertStore.success($t("settings.backup_create_success"));
        } catch (e: any) {
            alertStore.error(e.message || $t("settings.backup_create_failed"));
        }
    }

    async function handleRestoreBackup(version: number) {
        if (
            !confirm(
                $t("settings.backup_confirm_restore", { values: { version } }),
            )
        )
            return;
        try {
            await api.restoreBackup(version);
            alertStore.success($t("settings.backup_restore_success"));
        } catch (e: any) {
            alertStore.error(e.message || $t("settings.backup_restore_failed"));
        }
    }

    // Scheduler Logic
    interface SchedulerTask {
        id: string;
        name: string;
        description: string;
        enabled: boolean;
        schedule: string;
        type: string;
    }

    let schedulerTasks = $state<SchedulerTask[]>([]);
    let schedulerLoading = $state(false);
    let runningTask = $state<string | null>(null);

    async function loadSchedulerTasks() {
        schedulerLoading = true;
        try {
            schedulerTasks = await api.getSchedulerStatus();
        } catch (e) {
            console.error("Failed to load scheduler tasks", e);
        } finally {
            schedulerLoading = false;
        }
    }

    async function handleRunTask(taskId: string) {
        runningTask = taskId;
        try {
            await api.runSchedulerTask(taskId);
            alertStore.success(`Task "${taskId}" triggered successfully`);
        } catch (e: any) {
            alertStore.error(e.message || `Failed to run task "${taskId}"`);
        } finally {
            runningTask = null;
        }
    }

    onMount(() => {
        loadUsers();
        loadBackups();
        checkSafeMode();
        loadSchedulerTasks();
    });

    async function handleDeleteUser(username: string) {
        if (
            !confirm(
                $t("settings.user_confirm_delete", { values: { username } }),
            )
        )
            return;
        try {
            await api.deleteUser(username);
            loadUsers();
            alertStore.success($t("settings.user_delete_success"));
        } catch (e: any) {
            alertStore.error(e.message || $t("settings.user_delete_failed"));
        }
    }
</script>

<div class="settings-page">
    <div class="page-header"></div>

    <!-- General Settings -->
    <div class="section-title">{$t("settings.general")}</div>
    {#if $config}
        <div class="settings-grid">
            <!-- IP Forwarding -->
            <Card>
                <div class="setting-item">
                    <div class="setting-info">
                        <div class="setting-label">
                            {$t("settings.ip_forwarding")}
                        </div>
                        <div class="setting-desc">
                            {$t("settings.ip_forwarding_desc")}
                        </div>
                    </div>
                    <div class="setting-control">
                        <Toggle
                            label={$t("settings.ip_forwarding")}
                            bind:checked={localIPForwarding}
                            onchange={(val) =>
                                updateSetting("ip_forwarding", val)}
                        />
                    </div>
                </div>
            </Card>

            <!-- MSS Clamping -->
            <Card>
                <div class="setting-item">
                    <div class="setting-info">
                        <div class="setting-label">
                            {$t("settings.mss_clamping")}
                        </div>
                        <div class="setting-desc">
                            {$t("settings.mss_clamping_desc")}
                        </div>
                    </div>
                    <div class="setting-control">
                        <Toggle
                            label={$t("settings.mss_clamping")}
                            bind:checked={localMSSClamping}
                            onchange={(val) =>
                                updateSetting("mss_clamping", val)}
                        />
                    </div>
                </div>
            </Card>

            <!-- Flow Offload -->
            <Card>
                <div class="setting-item">
                    <div class="setting-info">
                        <div class="setting-label">
                            {$t("settings.flow_offload")}
                            <Badge variant="warning"
                                >{$t("settings.experimental")}</Badge
                            >
                        </div>
                        <div class="setting-desc">
                            {$t("settings.flow_offload_desc")}
                        </div>
                    </div>
                    <div class="setting-control">
                        <Toggle
                            label={$t("settings.flow_offload")}
                            bind:checked={localFlowOffload}
                            onchange={(val) =>
                                updateSetting("enable_flow_offload", val)}
                        />
                    </div>
                </div>
            </Card>
        </div>
    {:else}
        <div class="loading-state">
            <Spinner /> Loading settings...
        </div>
    {/if}

    <!-- User Management -->
    <div class="section-header">
        <div class="section-title">{$t("settings.users")}</div>
        <Button size="sm" onclick={() => (showUserModal = true)}
            >{$t("common.add_item", {
                values: { item: $t("item.user") },
            })}</Button
        >
    </div>

    <Card class="p-0">
        <div class="table-container">
            <table class="table">
                <thead>
                    <tr>
                        <th>{$t("settings.username")}</th>
                        <th>{$t("settings.role")}</th>
                        <th class="actions">{$t("settings.actions")}</th>
                    </tr>
                </thead>
                <tbody>
                    {#if users.length === 0}
                        <tr>
                            <td colspan="3" class="empty-row"
                                >{$t("settings.not_found")}</td
                            >
                        </tr>
                    {:else}
                        {#each users as user}
                            <tr>
                                <td>{user.username}</td>
                                <td><Badge>{user.role}</Badge></td>
                                <td class="actions">
                                    <Button
                                        variant="destructive"
                                        size="sm"
                                        onclick={() =>
                                            handleDeleteUser(user.username)}
                                        disabled={user.username === "admin"}
                                    >
                                        {$t("common.delete")}
                                    </Button>
                                </td>
                            </tr>
                        {/each}
                    {/if}
                </tbody>
            </table>
        </div>
    </Card>

    <!-- Create User Modal -->
    <Modal
        bind:open={showUserModal}
        title={$t("common.create_item", { values: { item: $t("item.user") } })}
    >
        <div class="space-y-4">
            <Input label={$t("auth.username")} bind:value={newUser.username} />
            <Input
                label={$t("settings.password")}
                type="password"
                bind:value={newUser.password}
            />
            <div class="modal-actions">
                <Button variant="ghost" onclick={() => (showUserModal = false)}
                    >{$t("common.cancel")}</Button
                >
                <Button onclick={handleCreateUser}>{$t("common.create")}</Button
                >
            </div>
        </div>
    </Modal>

    <!-- System -->
    <div class="section-title">{$t("settings.system")}</div>
    <div class="settings-grid">
        <Card>
            <div class="setting-item">
                <div class="setting-info">
                    <div class="setting-label">{$t("settings.system_ops")}</div>
                    <div class="setting-desc">
                        {$t("settings.system_ops_desc")}
                    </div>
                </div>
                <div class="setting-control system-actions">
                    <Button variant="outline" onclick={handleSafeModeToggle}>
                        {safeMode
                            ? $t("settings.exit_safe_mode")
                            : $t("settings.safe_mode")}
                    </Button>
                    <Button variant="destructive" onclick={handleReboot}
                        >{$t("settings.reboot")}</Button
                    >
                </div>
            </div>
        </Card>
    </div>

    <!-- Backups -->
    <div class="section-header">
        <div class="section-title">{$t("settings.backups")}</div>
        <Button size="sm" onclick={handleCreateBackup}
            >{$t("common.create_item", {
                values: { item: $t("item.backup") },
            })}</Button
        >
    </div>

    <Card class="p-0">
        <div class="table-container">
            <table class="table">
                <thead>
                    <tr>
                        <th>{$t("settings.id")}</th>
                        <th>{$t("settings.date")}</th>
                        <th>{$t("common.description")}</th>
                        <th class="actions">{$t("settings.actions")}</th>
                    </tr>
                </thead>
                <tbody>
                    {#if backups.length === 0}
                        <tr>
                            <td colspan="4" class="empty-row"
                                >{$t("settings.backups_none")}</td
                            >
                        </tr>
                    {:else}
                        {#each backups as backup}
                            <tr>
                                <td>{backup.id}</td>
                                <td
                                    >{new Date(
                                        backup.timestamp,
                                    ).toLocaleString()}</td
                                >
                                <td>{backup.description || "-"}</td>
                                <td class="actions">
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        onclick={() =>
                                            handleRestoreBackup(backup.version)}
                                    >
                                        {$t("settings.restore")}
                                    </Button>
                                </td>
                            </tr>
                        {/each}
                    {/if}
                </tbody>
            </table>
        </div>
    </Card>

    <!-- Scheduled Tasks -->
    <div class="section-header">
        <div class="section-title">Scheduled Tasks</div>
        <Button
            size="sm"
            variant="outline"
            onclick={loadSchedulerTasks}
            disabled={schedulerLoading}
        >
            Refresh
        </Button>
    </div>

    <Card class="p-0">
        <div class="table-container">
            <table class="table">
                <thead>
                    <tr>
                        <th>Task</th>
                        <th>Schedule</th>
                        <th>Type</th>
                        <th class="actions">Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {#if schedulerLoading}
                        <tr>
                            <td colspan="4" class="empty-row">
                                <Spinner size="sm" /> Loading tasks...
                            </td>
                        </tr>
                    {:else if schedulerTasks.length === 0}
                        <tr>
                            <td colspan="4" class="empty-row"
                                >No scheduled tasks</td
                            >
                        </tr>
                    {:else}
                        {#each schedulerTasks as task}
                            <tr>
                                <td>
                                    <div class="task-info">
                                        <span class="task-name"
                                            >{task.name}</span
                                        >
                                        {#if task.description}
                                            <span class="task-desc"
                                                >{task.description}</span
                                            >
                                        {/if}
                                    </div>
                                </td>
                                <td>{task.schedule}</td>
                                <td>
                                    <Badge
                                        variant={task.type === "system"
                                            ? "secondary"
                                            : "outline"}
                                    >
                                        {task.type}
                                    </Badge>
                                </td>
                                <td class="actions">
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        onclick={() => handleRunTask(task.id)}
                                        disabled={runningTask !== null}
                                        aria-label={`Run task ${task.name}`}
                                    >
                                        {#if runningTask === task.id}
                                            <Spinner size="sm" />
                                        {:else}
                                            Run Now
                                        {/if}
                                    </Button>
                                </td>
                            </tr>
                        {/each}
                    {/if}
                </tbody>
            </table>
        </div>
    </Card>
</div>

<style>
    .settings-page {
        display: flex;
        flex-direction: column;
        gap: var(--space-6);
        max-width: 800px;
        padding-bottom: var(--space-8);
    }

    .section-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-top: var(--space-4);
    }

    .section-title {
        font-size: var(--text-lg);
        font-weight: 600;
        color: var(--color-foreground);
        margin-bottom: var(--space-2);
    }

    .settings-grid {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
    }

    .setting-item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-6);
    }

    .setting-info {
        flex: 1;
    }

    .setting-label {
        font-weight: 600;
        color: var(--color-foreground);
        margin-bottom: var(--space-1);
        display: flex;
        align-items: center;
        gap: var(--space-2);
    }

    .setting-desc {
        color: var(--color-muted);
        font-size: var(--text-sm);
        line-height: 1.5;
    }

    .setting-control {
        flex-shrink: 0;
    }

    .space-y-4 > :global(*) + :global(*) {
        margin-top: var(--space-4);
    }

    .modal-actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        margin-top: var(--space-6);
    }

    .actions {
        text-align: right;
    }

    /* Table Styles */
    .table-container {
        overflow-x: auto;
    }

    .table {
        width: 100%;
        border-collapse: collapse;
        font-size: var(--text-sm);
    }

    th {
        padding: var(--space-3) var(--space-4);
        background-color: var(--color-backgroundSecondary);
        font-weight: 500;
        color: var(--color-foreground);
        border-bottom: 1px solid var(--color-border);
        text-align: left;
    }

    td {
        padding: var(--space-3) var(--space-4);
        border-bottom: 1px solid var(--color-border);
        color: var(--color-foreground);
    }

    tr:last-child td {
        border-bottom: none;
    }

    .empty-row {
        text-align: center;
        color: var(--color-muted);
        padding: var(--space-8);
    }

    /* Task info styling */
    .task-info {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
    }

    .task-name {
        font-weight: 500;
    }

    .task-desc {
        font-size: var(--text-xs);
        color: var(--color-muted);
    }

    .system-actions {
        display: flex;
        gap: var(--space-2);
    }
</style>

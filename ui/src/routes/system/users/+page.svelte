<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import { fade, slide } from "svelte/transition";
    import Icon from "$lib/components/Icon.svelte";
    import { api } from "$lib/stores/app";

    let users = [];
    let loading = true;
    let error = null;

    // State for creating a new user
    let isCreating = false;
    let newUser = { username: "", password: "", role: "operator" };
    let createError = null;

    // State for editing users (key: username, value: boolean)
    let editingState = {};
    let editForm = { password: "", role: "" };

    onMount(async () => {
        await loadUsers();
    });

    async function loadUsers() {
        loading = true;
        try {
            users = await api.get("/api/users");
            error = null;
        } catch (e) {
            error = e.message;
        } finally {
            loading = false;
        }
    }

    async function handleCreate() {
        if (!newUser.username || !newUser.password) {
            createError = "Username and password are required";
            return;
        }

        try {
            await api.post("/api/users", newUser);
            await loadUsers();
            isCreating = false;
            newUser = { username: "", password: "", role: "operator" };
            createError = null;
        } catch (e) {
            createError = e.message;
        }
    }

    function startEditing(user) {
        editingState[user.username] = true;
        editForm = { password: "", role: user.role || "operator" };
        editingState = { ...editingState }; // Trigger reactivity
    }

    function cancelEditing(username) {
        editingState[username] = false;
        editingState = { ...editingState };
    }

    async function handleUpdate(username) {
        try {
            await api.put("/api/users", {
                username,
                password: editForm.password,
                role: editForm.role,
            });
            await loadUsers();
            cancelEditing(username);
        } catch (e) {
            alert("Update failed: " + e.message);
        }
    }

    async function handleDelete(username) {
        if (!confirm(`Are you sure you want to delete user "${username}"?`))
            return;

        try {
            await api.delete(
                `/api/users?username=${encodeURIComponent(username)}`,
            );
            await loadUsers();
        } catch (e) {
            alert("Delete failed: " + e.message);
        }
    }
</script>

<div class="page-container">
    <header class="page-header">
        <div class="header-content">
            <h1>User Management</h1>
            <p class="subtitle">Manage system access and roles</p>
        </div>
        <div class="header-actions">
            {#if !isCreating}
                <button class="btn-primary" onclick={() => (isCreating = true)}>
                    <Icon name="plus" size={16} /> Add User
                </button>
            {/if}
        </div>
    </header>

    {#if error}
        <div class="alert error" transition:slide>
            <Icon name="alert-triangle" size={20} />
            <span>{error}</span>
        </div>
    {/if}

    <div class="users-grid">
        <!-- Create User Card -->
        {#if isCreating}
            <div class="user-card create-card" transition:slide>
                <div class="card-header">
                    <h3>New User</h3>
                    <div class="card-actions">
                        <button
                            class="btn-icon"
                            onclick={() => (isCreating = false)}
                        >
                            <Icon name="x" size={16} />
                        </button>
                    </div>
                </div>
                <div class="card-body form-layout">
                    <div class="form-group">
                        <label for="new-username">Username</label>
                        <input
                            type="text"
                            id="new-username"
                            bind:value={newUser.username}
                            placeholder="username"
                        />
                    </div>
                    <div class="form-group">
                        <label for="new-password">Password</label>
                        <input
                            type="password"
                            id="new-password"
                            bind:value={newUser.password}
                            placeholder="password"
                        />
                    </div>
                    <div class="form-group">
                        <label for="new-role">Role</label>
                        <select id="new-role" bind:value={newUser.role}>
                            <option value="admin">Admin</option>
                            <option value="operator">Operator</option>
                            <option value="viewer">Viewer</option>
                        </select>
                    </div>
                    {#if createError}
                        <div class="form-error">{createError}</div>
                    {/if}
                    <div class="form-actions">
                        <button
                            class="btn-secondary"
                            onclick={() => (isCreating = false)}>Cancel</button
                        >
                        <button class="btn-primary" onclick={handleCreate}
                            >Create User</button
                        >
                    </div>
                </div>
            </div>
        {/if}

        <!-- User List -->
        {#if loading}
            <div class="loading-state">
                <div class="spinner"></div>
                <span>Loading users...</span>
            </div>
        {:else if users.length === 0}
            <div class="empty-state">
                <Icon name="users" size={48} />
                <h3>No users found</h3>
                <p>Click "Add User" to create the first user.</p>
            </div>
        {:else}
            {#each users as user (user.username)}
                <div
                    class="user-card"
                    class:editing={editingState[user.username]}
                >
                    {#if editingState[user.username]}
                        <!-- Edit Mode -->
                        <div class="card-header">
                            <h3>Editing {user.username}</h3>
                        </div>
                        <div class="card-body form-layout">
                            <div class="form-group">
                                <label for="edit-password-{user.username}"
                                    >New Password</label
                                >
                                <input
                                    type="password"
                                    id="edit-password-{user.username}"
                                    bind:value={editForm.password}
                                    placeholder="Leave empty to keep current"
                                />
                            </div>
                            <div class="form-group">
                                <label for="edit-role-{user.username}"
                                    >Role</label
                                >
                                <select
                                    id="edit-role-{user.username}"
                                    bind:value={editForm.role}
                                >
                                    <option value="admin">Admin</option>
                                    <option value="operator">Operator</option>
                                    <option value="viewer">Viewer</option>
                                </select>
                            </div>
                            <div class="form-actions">
                                <button
                                    class="btn-secondary"
                                    onclick={() => cancelEditing(user.username)}
                                    >Cancel</button
                                >
                                <button
                                    class="btn-primary"
                                    onclick={() => handleUpdate(user.username)}
                                    >Save Changes</button
                                >
                            </div>
                        </div>
                    {:else}
                        <!-- View Mode -->
                        <div class="card-header">
                            <div class="user-info">
                                <div class="avatar-placeholder">
                                    {user.username
                                        .substring(0, 2)
                                        .toUpperCase()}
                                </div>
                                <div class="user-details">
                                    <h3>{user.username}</h3>
                                    <span class="badge role-{user.role}"
                                        >{user.role || "operator"}</span
                                    >
                                </div>
                            </div>
                            <div class="card-actions">
                                <button
                                    class="btn-icon"
                                    onclick={() => startEditing(user)}
                                    title="Edit"
                                >
                                    <Icon name="edit" size={16} />
                                </button>
                                {#if user.username !== "admin"}
                                    <button
                                        class="btn-icon destructive"
                                        onclick={() =>
                                            handleDelete(user.username)}
                                        title="Delete"
                                    >
                                        <Icon name="trash-2" size={16} />
                                    </button>
                                {/if}
                            </div>
                        </div>
                        <div class="card-body">
                            <div class="meta-row">
                                <span class="label">Created</span>
                                <span class="value"
                                    >{new Date(
                                        user.created_at,
                                    ).toLocaleDateString()}</span
                                >
                            </div>
                            <div class="meta-row">
                                <span class="label">Last Login</span>
                                <span class="value">Never</span>
                            </div>
                        </div>
                    {/if}
                </div>
            {/each}
        {/if}
    </div>
</div>

<style>
    .page-container {
        padding: 2rem;
        max-width: 1200px;
        margin: 0 auto;
    }

    .page-header {
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
        margin-bottom: 2rem;
    }

    .subtitle {
        color: var(--text-secondary);
        margin-top: 0.5rem;
    }

    .users-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
        gap: 1.5rem;
    }

    .user-card {
        background: var(--surface-1);
        border: 1px solid var(--border-color);
        border-radius: var(--radius-md);
        display: flex;
        flex-direction: column;
        transition: box-shadow 0.2s;
        overflow: hidden;
    }

    .user-card:hover {
        box-shadow: var(--shadow-sm);
        border-color: var(--primary-color-dim);
    }

    .user-card.create-card {
        border-color: var(--primary-color);
        background: var(--surface-2);
        grid-column: 1 / -1;
        max-width: 600px;
        margin-bottom: 1rem;
    }

    .card-header {
        padding: 1rem;
        border-bottom: 1px solid var(--border-color);
        display: flex;
        justify-content: space-between;
        align-items: center;
        background: var(--surface-2);
    }

    .user-info {
        display: flex;
        align-items: center;
        gap: 1rem;
    }

    .avatar-placeholder {
        width: 40px;
        height: 40px;
        background: var(--primary-color);
        color: white;
        border-radius: 50%;
        display: flex;
        align-items: center;
        justify-content: center;
        font-weight: bold;
        font-size: 0.9rem;
    }

    .user-details h3 {
        margin: 0;
        font-size: 1rem;
        font-weight: 500;
    }

    .btn-icon {
        background: none;
        border: none;
        color: var(--text-secondary);
        cursor: pointer;
        padding: 0.5rem;
        border-radius: var(--radius-sm);
        transition:
            color 0.2s,
            background 0.2s;
    }

    .btn-icon:hover {
        background: var(--surface-3);
        color: var(--text-primary);
    }

    .btn-icon.destructive:hover {
        color: var(--danger-color);
        background: var(--danger-color-dim);
    }

    .card-body {
        padding: 1rem;
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
    }

    .meta-row {
        display: flex;
        justify-content: space-between;
        font-size: 0.9rem;
    }

    .label {
        color: var(--text-secondary);
    }

    /* Form Styles */
    .form-layout {
        gap: 1rem;
    }

    .form-group {
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
    }

    .form-group label {
        font-size: 0.85rem;
        color: var(--text-secondary);
        font-weight: 500;
    }

    input,
    select {
        padding: 0.5rem;
        background: var(--surface-1);
        border: 1px solid var(--border-color);
        border-radius: var(--radius-sm);
        color: var(--text-primary);
    }

    input:focus,
    select:focus {
        border-color: var(--primary-color);
        outline: none;
    }

    .form-actions {
        display: flex;
        justify-content: flex-end;
        gap: 0.5rem;
        margin-top: 1rem;
    }

    .form-error {
        color: var(--danger-color);
        font-size: 0.9rem;
    }

    /* Badges */
    .badge {
        font-size: 0.75rem;
        padding: 0.1rem 0.5rem;
        border-radius: 1rem;
        background: var(--surface-3);
        color: var(--text-secondary);
    }

    .role-admin {
        background: var(--primary-color-dim);
        color: var(--primary-color);
    }
    .role-operator {
        background: var(--success-color-dim);
        color: var(--success-color);
    }

    /* Buttons */
    .btn-primary,
    .btn-secondary {
        padding: 0.5rem 1rem;
        border-radius: var(--radius-sm);
        border: none;
        cursor: pointer;
        font-weight: 500;
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .btn-primary {
        background: var(--primary-color);
        color: white;
    }

    .btn-secondary {
        background: var(--surface-3);
        color: var(--text-primary);
    }

    .btn-primary:hover {
        filter: brightness(1.1);
    }
    .btn-secondary:hover {
        background: var(--surface-4);
    }

    /* Alert */
    .alert {
        padding: 1rem;
        border-radius: var(--radius-md);
        display: flex;
        align-items: center;
        gap: 1rem;
        margin-bottom: 1.5rem;
    }
    .alert.error {
        background: var(--danger-color-dim);
        color: var(--danger-color);
        border: 1px solid var(--danger-color);
    }
</style>

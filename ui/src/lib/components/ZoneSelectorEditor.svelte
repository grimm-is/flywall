<script lang="ts">
    import { t } from "svelte-i18n";
    import Icon from "$lib/components/Icon.svelte";
    import Badge from "$lib/components/Badge.svelte";
    import Button from "$lib/components/Button.svelte";
    import Input from "$lib/components/Input.svelte";
    import Select from "$lib/components/Select.svelte";
    import { createEventDispatcher } from "svelte";
    import NetworkInput from "$lib/components/NetworkInput.svelte";
    import ObjectIcon from "$lib/components/ObjectIcon.svelte";

    // Interface for RuleMatch from backend
    interface RuleMatch {
        interface?: string;
        src?: string;
        dst?: string;
        protocol?: string;
        mac?: string;
        mark?: string;
        dscp?: string;
        tos?: number;
        out_interface?: string;
        phys_in?: string;
        phys_out?: string;
        // Add internal ID for keying
        _id?: string;
    }

    let {
        matches = $bindable([]),
        availableInterfaces = [],
        availableIPSets = [],
    } = $props<{
        matches: RuleMatch[];
        availableInterfaces: string[];
        availableIPSets: string[];
    }>();

    const dispatch = createEventDispatcher();

    // Editing state
    let isAdding = $state(false);
    let editingMatchId = $state<string | null>(null);
    let showAdvanced = $state(false);

    // Temporary match state
    let currentMatch = $state<RuleMatch>({});

    // Initialize new match
    function initMatch(): RuleMatch {
        return {
            _id: Math.random().toString(36).substr(2, 9),
            interface: "",
            src: "",
            dst: "",
            protocol: "",
            mac: "",
            mark: "",
            dscp: "",
            tos: 0,
            out_interface: "",
            phys_in: "",
            phys_out: "",
        };
    }

    function startAdd() {
        currentMatch = initMatch();
        isAdding = true;
        showAdvanced = false;
        editingMatchId = null;
    }

    function startEdit(match: RuleMatch, index: number) {
        currentMatch = { ...match };
        // Ensure ID exists
        if (!currentMatch._id)
            currentMatch._id = Math.random().toString(36).substr(2, 9);

        isAdding = true;
        editingMatchId = currentMatch._id;

        // Auto-expand advanced if advanced fields are present
        showAdvanced = !!(
            currentMatch.protocol ||
            currentMatch.mac ||
            currentMatch.mark ||
            currentMatch.dscp ||
            (currentMatch.tos !== undefined && currentMatch.tos !== 0) ||
            currentMatch.out_interface
        );
    }

    function cancelAdd() {
        isAdding = false;
        editingMatchId = null;
        currentMatch = {};
    }

    function saveMatch() {
        // Basic validation: at least one criteria must be set
        const hasCriteria = Object.entries(currentMatch).some(([k, v]) => {
            if (k === "_id") return false;
            return v !== "" && v !== undefined && v !== 0;
        });

        if (!hasCriteria) return;

        // Clean up empty fields
        const cleaned: RuleMatch = { ...currentMatch };
        if (!cleaned.interface) delete cleaned.interface;
        if (!cleaned.src) delete cleaned.src;
        if (!cleaned.dst) delete cleaned.dst;
        if (!cleaned.protocol) delete cleaned.protocol;
        if (!cleaned.mac) delete cleaned.mac;
        if (!cleaned.mark) delete cleaned.mark;
        if (!cleaned.dscp) delete cleaned.dscp;
        if (!cleaned.tos) delete cleaned.tos;
        if (!cleaned.out_interface) delete cleaned.out_interface;

        let newMatches: RuleMatch[];

        if (editingMatchId) {
            // Update existing
            newMatches = matches.map((m: RuleMatch) =>
                m._id === editingMatchId ? cleaned : m,
            );
        } else {
            // Add new
            newMatches = [...matches, cleaned];
        }

        dispatch("change", newMatches);
        cancelAdd();
    }

    function removeMatch(index: number) {
        const newMatches = matches.filter(
            (_: RuleMatch, i: number) => i !== index,
        );
        dispatch("change", newMatches);
    }
</script>

<div class="zone-selector-editor">
    {#if isAdding}
        <div
            class="match-editor p-4 border rounded-lg bg-backgroundSecondary space-y-4"
        >
            <div class="flex justify-between items-center mb-2">
                <h4 class="font-medium text-sm">
                    {editingMatchId ? "Edit Match Rule" : "New Match Rule"}
                </h4>
            </div>

            <!-- Basic Fields -->
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                {#if availableInterfaces.length > 0}
                    <Select
                        label="Interface"
                        bind:value={currentMatch.interface}
                        options={[
                            { value: "", label: "Any" },
                            ...availableInterfaces.map((i: string) => ({
                                value: i,
                                label: i,
                            })),
                        ]}
                    />
                {:else}
                    <Input
                        label="Interface"
                        bind:value={currentMatch.interface}
                        placeholder="e.g. eth0"
                    />
                {/if}

                <NetworkInput
                    label="Source (IP/CIDR/Set)"
                    bind:value={currentMatch.src}
                    suggestions={availableIPSets}
                    placeholder="e.g. 192.168.1.0/24 or @guest_set"
                />

                <NetworkInput
                    label="Destination"
                    bind:value={currentMatch.dst}
                    suggestions={availableIPSets}
                    placeholder="e.g. 10.0.0.1"
                />
            </div>

            <!-- Advanced Toggle -->
            <div class="pt-2">
                <button
                    type="button"
                    class="text-xs text-primary underline flex items-center gap-1"
                    onclick={() => (showAdvanced = !showAdvanced)}
                >
                    {showAdvanced ? "Hide Advanced" : "Show Advanced Options"}
                    <Icon
                        name={showAdvanced ? "chevron-up" : "chevron-down"}
                        size={12}
                    />
                </button>
            </div>

            {#if showAdvanced}
                <div
                    class="advanced-fields grid grid-cols-2 md:grid-cols-3 gap-4 pt-2 border-t border-border/50"
                >
                    <Input
                        id="match-protocol"
                        label="Protocol"
                        bind:value={currentMatch.protocol}
                        placeholder="tcp, udp..."
                    />
                    <Input
                        id="match-mac"
                        label="MAC Address"
                        bind:value={currentMatch.mac}
                        placeholder="00:11:22..."
                    />
                    <Input
                        id="match-mark"
                        label="Mark (Hex/Int)"
                        bind:value={currentMatch.mark}
                        placeholder="0x100"
                    />
                    <Input
                        id="match-dscp"
                        label="DSCP"
                        bind:value={currentMatch.dscp}
                        placeholder="0-63"
                    />
                    <Input
                        id="match-tos"
                        label="TOS"
                        type="number"
                        bind:value={currentMatch.tos}
                        placeholder="0-255"
                    />
                    <Input
                        id="match-out-interface"
                        label="Out Interface"
                        bind:value={currentMatch.out_interface}
                        placeholder="eth1"
                    />
                </div>
            {/if}

            <!-- Actions -->
            <div class="flex justify-end gap-2 mt-4">
                <Button variant="ghost" size="sm" onclick={cancelAdd}
                    >Cancel</Button
                >
                <Button size="sm" onclick={saveMatch}>
                    {editingMatchId ? "Update Rule" : "Add Rule"}
                </Button>
            </div>
        </div>
    {:else}
        <!-- List View -->
        <div class="match-list space-y-2">
            {#each matches as match, index}
                <div
                    class="match-item flex items-center justify-between p-3 border rounded-md bg-background"
                >
                    <div class="match-details text-sm">
                        {#if !match.interface && !match.src && !match.dst && !match.protocol && !match.mac}
                            <span class="text-muted-foreground italic"
                                >Empty Match (Matches Everything)</span
                            >
                        {:else}
                            <div class="flex flex-wrap gap-2">
                                {#if match.interface}
                                    <Badge
                                        variant="outline"
                                        class="bg-blue-500/10 text-blue-500 border-blue-500/20 gap-1 pl-1"
                                    >
                                        <ObjectIcon
                                            type="interface"
                                            name={match.interface}
                                        />
                                        {match.interface}
                                    </Badge>
                                {/if}
                                {#if match.src}
                                    <Badge
                                        variant="outline"
                                        class="bg-green-500/10 text-green-500 border-green-500/20 gap-1 pl-1"
                                    >
                                        <ObjectIcon type="ip" />
                                        {match.src}
                                    </Badge>
                                {/if}
                                {#if match.dst}
                                    <Badge
                                        variant="outline"
                                        class="bg-green-500/10 text-green-500 border-green-500/20 gap-1 pl-1"
                                    >
                                        <ObjectIcon type="ip" />
                                        → {match.dst}
                                    </Badge>
                                {/if}
                                {#if match.protocol}
                                    <Badge
                                        variant="outline"
                                        class="bg-purple-500/10 text-purple-500 border-purple-500/20 gap-1 pl-1"
                                    >
                                        <ObjectIcon type="protocol" />
                                        {match.protocol}
                                    </Badge>
                                {/if}
                                {#if match.mac}
                                    <Badge
                                        variant="outline"
                                        class="bg-yellow-500/10 text-yellow-500 border-yellow-500/20 gap-1 pl-1"
                                    >
                                        <ObjectIcon type="mac" />
                                        {match.mac}
                                    </Badge>
                                {/if}
                                {#if match.mark || match.dscp || match.tos || match.out_interface}
                                    <Badge
                                        variant="outline"
                                        class="text-muted-foreground">...</Badge
                                    >
                                {/if}
                            </div>
                        {/if}
                    </div>

                    <div class="actions flex gap-1">
                        <button
                            type="button"
                            class="p-1 hover:text-primary transition-colors"
                            onclick={() => startEdit(match, index)}
                        >
                            <Icon name="edit" size={14} />
                        </button>
                        <button
                            type="button"
                            class="p-1 hover:text-destructive transition-colors"
                            onclick={() => removeMatch(index)}
                        >
                            <Icon name="delete" size={14} />
                        </button>
                    </div>
                </div>
            {/each}

            {#if matches.length === 0}
                <div
                    class="text-xs text-muted-foreground italic p-2 text-center border border-dashed rounded-md"
                >
                    No match rules defined. (Zone may rely on assigned
                    interfaces)
                </div>
            {/if}

            <Button
                variant="outline"
                size="sm"
                class="w-full mt-2"
                onclick={startAdd}
            >
                <Icon name="plus" size={14} class="mr-2" />
                Add Match Rule
            </Button>
        </div>
    {/if}
</div>

<style>
    .zone-selector-editor {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
    }
</style>

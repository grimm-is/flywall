<script lang="ts">
  /**
   * Policy Page
   * Security policy and rule management
   *
   * Supports two views:
   * - Classic: Card-based policy groups
   * - ClearPath: Unified rule table with sparklines
   */

  import { config, api } from "$lib/stores/app";
  import {
    Card,
    Button,
    Modal,
    Input,
    Select,
    Badge,
    Table,
    Spinner,
    PillInput,
    Toggle,
  } from "$lib/components";
  import { PolicyEditor } from "$lib/components/policy";
  import RuleCreateCard from "$lib/components/policy/RuleCreateCard.svelte";
  import PolicyCreateCard from "$lib/components/policy/PolicyCreateCard.svelte";
  import { t } from "svelte-i18n";
  import {
    SERVICE_GROUPS,
    getService,
    type ServiceDefinition,
  } from "$lib/data/common_services";
  import { NetworkInput } from "$lib/components";
  import { getAddressType } from "$lib/utils/validation";

  let loading = $state(false);
  // Renamed from showRuleModal to showEditRuleModal
  let showEditRuleModal = $state(false);
  let isAddingRule = $state(false);
  let selectedPolicy = $state<any>(null);
  let editingRuleIndex = $state<number | null>(null);
  let isEditMode = $derived(editingRuleIndex !== null);

  // Rule form (used for editing)
  let ruleAction = $state("accept");
  let ruleName = $state("");
  // Protocol selection (array for PillInput)
  let protocols = $state<string[]>([]);
  let ruleDestPort = $state("");
  let ruleSrc = $state("");
  let ruleDest = $state("");
  let selectedService = $state("");

  // Advanced options state (used for editing)
  let showAdvanced = $state(false);
  let invertSrc = $state(false);
  let invertDest = $state(false);
  let tcpFlagsArray = $state<string[]>([]);
  let maxConnections = $state("");

  // Protocol options for PillInput
  const PROTOCOL_OPTIONS = [
    { value: "tcp", label: "TCP" },
    { value: "udp", label: "UDP" },
    { value: "icmp", label: "ICMP" },
  ];

  // TCP Flags options for PillInput
  const TCP_FLAG_OPTIONS = [
    { value: "syn", label: "SYN" },
    { value: "ack", label: "ACK" },
    { value: "fin", label: "FIN" },
    { value: "rst", label: "RST" },
    { value: "psh", label: "PSH" },
    { value: "urg", label: "URG" },
  ];

  // Reactive: when service changes, update protocol and port
  $effect(() => {
    if (selectedService) {
      const svc = getService(selectedService);
      if (svc) {
        // Set protocols array
        if (svc.protocol === "both") {
          protocols = ["tcp", "udp"];
        } else if (svc.protocol === "tcp") {
          protocols = ["tcp"];
        } else if (svc.protocol === "udp") {
          protocols = ["udp"];
        }
        ruleDestPort = svc.port?.toString() || "";
        // Auto-generate rule name if empty
        if (!ruleName) {
          ruleName = `Allow ${svc.label}`;
        }
      }
    }
  });

  // Add Policy modal state
  let isAddingPolicy = $state(false);

  const zones = $derived($config?.zones || []);
  const zoneNames = $derived(zones.map((z: any) => z.name));
  const policies = $derived($config?.policies || []);
  const ipsets = $derived($config?.ipsets || []);
  const availableIPSets = $derived(
    (Array.isArray(ipsets) ? ipsets : [])
      .map((s: any) => s?.name)
      .filter((n: any) => n)
      .sort(),
  );

  const policyColumns = [
    { key: "from", label: "From" },
    { key: "to", label: "To" },
    { key: "ruleCount", label: "Rules" },
  ];

  const policyData = $derived(
    policies.map((p: any) => ({
      ...p,
      ruleCount: p.rules?.length || 0,
    })),
  );

  // Status values from backend: "live", "pending_add", "pending_edit", "pending_delete"
  type ItemStatus = "live" | "pending_add" | "pending_edit" | "pending_delete";

  // Get CSS class for item status
  function getStatusClass(status: ItemStatus | undefined): string {
    switch (status) {
      case "pending_add":
        return "pending-add";
      case "pending_edit":
        return "pending-modify";
      case "pending_delete":
        return "pending-delete";
      default:
        return "";
    }
  }

  // Get badge text for item status
  function getStatusBadgeText(status: ItemStatus | undefined): string {
    switch (status) {
      case "pending_add":
        return "NEW";
      case "pending_edit":
        return "CHANGED";
      case "pending_delete":
        return "DELETED";
      default:
        return "";
    }
  }

  // Check if item has pending status
  function isPending(status: ItemStatus | undefined): boolean {
    return status !== undefined && status !== "live";
  }

  function getActionBadge(action: string) {
    switch (action) {
      case "accept":
        return "success";
      case "drop":
        return "destructive";
      case "reject":
        return "warning";
      default:
        return "secondary";
    }
  }

  function openAddRule(policy: any | null) {
    isAddingRule = true;
    selectedPolicy = policy;
  }

  function openEditRule(policy: any, ruleIndex: number) {
    selectedPolicy = policy;
    editingRuleIndex = ruleIndex;
    const rule = policy.rules[ruleIndex];
    ruleAction = rule.action || "accept";
    ruleName = rule.name || "";
    // Parse protocol - could be "tcp", "udp", "tcp,udp", etc. into array
    const proto = rule.proto || "";
    protocols = proto
      ? proto
          .split(",")
          .map((p: string) => p.trim())
          .filter((p: string) => p)
      : [];
    ruleDestPort =
      rule.dest_port?.toString() || rule.dest_ports?.join(",") || "";
    rule.dest_port?.toString() || rule.dest_ports?.join(",") || "";

    // Handle Source (IPSet or IP)
    if (rule.src_ipset) {
      ruleSrc = rule.src_ipset;
    } else if (
      rule.src_ip &&
      Array.isArray(rule.src_ip) &&
      rule.src_ip.length > 0
    ) {
      ruleSrc = rule.src_ip[0]; // TODO: Support multiple
    } else if (rule.src_ip) {
      ruleSrc = rule.src_ip as string;
    } else {
      ruleSrc = "";
    }

    // Handle Dest (IPSet or IP)
    if (rule.dest_ipset) {
      ruleDest = rule.dest_ipset;
    } else if (
      rule.dest_ip &&
      Array.isArray(rule.dest_ip) &&
      rule.dest_ip.length > 0
    ) {
      ruleDest = rule.dest_ip[0];
    } else if (rule.dest_ip) {
      ruleDest = rule.dest_ip as string;
    } else {
      ruleDest = "";
    }
    // Load advanced options
    invertSrc = rule.invert_src || false;
    invertDest = rule.invert_dest || false;
    // Parse tcp_flags into array
    const flags = rule.tcp_flags || "";
    tcpFlagsArray = flags
      ? flags
          .split(",")
          .map((f: string) => f.trim())
          .filter((f: string) => f && !f.startsWith("!"))
      : [];
    maxConnections = rule.max_connections?.toString() || "";
    showAdvanced = !!(
      invertSrc ||
      invertDest ||
      tcpFlagsArray.length > 0 ||
      maxConnections
    );
    showEditRuleModal = true;
  }

  async function handleCreateRule(event: CustomEvent) {
    const data = event.detail;
    loading = true;
    try {
      const newRule: any = {
        action: data.action,
        name: data.name,
        proto: data.protocols.length > 0 ? data.protocols.join(",") : undefined,
      };

      if (data.destPort && data.destPort.trim()) {
        const parts = data.destPort.split(",").map((p: string) => p.trim());
        const destPorts: number[] = [];
        for (const part of parts) {
          if (part.includes("-")) {
            const [start, end] = part
              .split("-")
              .map((n: string) => parseInt(n.trim()));
            if (!isNaN(start) && !isNaN(end)) {
              for (let i = start; i <= end; i++) destPorts.push(i);
            }
          } else {
            const port = parseInt(part);
            if (!isNaN(port)) destPorts.push(port);
          }
        }
        if (destPorts.length === 1) newRule.dest_port = destPorts[0];
        else if (destPorts.length > 1) newRule.dest_ports = destPorts;
      }

      const srcType = getAddressType(data.src);
      if (srcType === "name") newRule.src_ipset = data.src;
      else if (data.src) newRule.src_ip = [data.src];

      const destType = getAddressType(data.dest);
      if (destType === "name") newRule.dest_ipset = data.dest;
      else if (data.dest) newRule.dest_ip = [data.dest];

      if (data.invertSrc) newRule.invert_src = true;
      if (data.invertDest) newRule.invert_dest = true;
      if (data.tcpFlags && data.tcpFlags.length > 0)
        newRule.tcp_flags = data.tcpFlags.join(",");
      if (data.maxConnections)
        newRule.max_connections = parseInt(data.maxConnections);

      const from = data.policyFrom;
      const to = data.policyTo;

      if (!from || !to) {
        alert("Policy Zone (From/To) is required");
        loading = false;
        return;
      }

      let policyFound = false;
      const updatedPolicies = policies.map((p: any) => {
        if (p.from === from && p.to === to) {
          policyFound = true;
          return { ...p, rules: [...(p.rules || []), newRule] };
        }
        return p;
      });

      if (!policyFound) {
        updatedPolicies.push({ from, to, rules: [newRule] });
      }

      await api.updatePolicies(updatedPolicies);
      await loadRules();
      isAddingRule = false;
    } catch (e) {
      console.error("Failed to create rule", e);
    } finally {
      loading = false;
    }
  }

  async function saveRule() {
    if (!selectedPolicy || !ruleName) return;

    loading = true;
    try {
      // Protocol string from array
      const protoString =
        protocols.length > 0 ? protocols.join(",") : undefined;

      // Parse ports - support comma-separated and ranges (e.g., "80,443" or "3000-3010")
      let destPorts: number[] = [];
      if (ruleDestPort && ruleDestPort.trim()) {
        const parts = ruleDestPort.split(",").map((p: string) => p.trim());
        for (const part of parts) {
          if (part.includes("-")) {
            const [start, end] = part
              .split("-")
              .map((n: string) => parseInt(n.trim()));
            if (!isNaN(start) && !isNaN(end)) {
              for (let i = start; i <= end; i++) destPorts.push(i);
            }
          } else {
            const port = parseInt(part);
            if (!isNaN(port)) destPorts.push(port);
          }
        }
      }

      const newRule: any = {
        action: ruleAction,
        name: ruleName,
        proto: protoString,
        // Use dest_ports for multiple, dest_port for single
        dest_port: destPorts.length === 1 ? destPorts[0] : undefined,
        dest_ports: destPorts.length > 1 ? destPorts : undefined,
      };

      // Map Source/Dest based on type
      const srcType = getAddressType(ruleSrc);
      if (srcType === "name") {
        newRule.src_ipset = ruleSrc;
      } else if (ruleSrc) {
        // IP, CIDR, or Hostname
        newRule.src_ip = [ruleSrc]; // Backend expects array
      }

      const destType = getAddressType(ruleDest);
      if (destType === "name") {
        newRule.dest_ipset = ruleDest;
      } else if (ruleDest) {
        newRule.dest_ip = [ruleDest];
      }
      // Advanced options
      if (invertSrc) newRule.invert_src = true;
      if (invertDest) newRule.invert_dest = true;
      if (tcpFlagsArray.length > 0) newRule.tcp_flags = tcpFlagsArray.join(",");
      if (maxConnections) newRule.max_connections = parseInt(maxConnections);

      const updatedPolicies = policies.map((p: any) => {
        if (p.from === selectedPolicy.from && p.to === selectedPolicy.to) {
          if (isEditMode && editingRuleIndex !== null) {
            // Edit existing rule
            const newRules = [...(p.rules || [])];
            newRules[editingRuleIndex] = newRule;
            return { ...p, rules: newRules };
          } else {
            // Add new rule
            return { ...p, rules: [...(p.rules || []), newRule] };
          }
        }
        return p;
      });

      await api.updatePolicies(updatedPolicies);
      await loadRules();
      showEditRuleModal = false;
    } catch (e) {
      console.error("Failed to save rule:", e);
    } finally {
      loading = false;
    }
  }

  async function deletePolicy(policy: any) {
    if (
      !confirm(
        $t("common.delete_confirm_item", {
          values: { item: $t("item.policy") },
        }),
      )
    ) {
      return;
    }

    loading = true;
    try {
      const updatedPolicies = policies.filter(
        (p: any) => !(p.from === policy.from && p.to === policy.to),
      );
      await api.updatePolicies(updatedPolicies);
      await loadRules();
    } catch (e) {
      console.error("Failed to delete policy:", e);
    } finally {
      loading = false;
    }
  }

  async function deleteRule(policy: any, ruleIndex: number) {
    const rule = policy.rules[ruleIndex];
    if (
      !confirm(
        $t("common.delete_confirm_item", {
          values: { item: $t("item.rule") },
        }),
      )
    ) {
      return;
    }

    loading = true;
    try {
      const updatedPolicies = policies.map((p: any) => {
        if (p.from === policy.from && p.to === policy.to) {
          const newRules = [...p.rules];
          newRules.splice(ruleIndex, 1);
          return { ...p, rules: newRules };
        }
        return p;
      });
      await api.updatePolicies(updatedPolicies);
      await loadRules();
    } catch (e) {
      console.error("Failed to delete rule:", e);
    } finally {
      loading = false;
    }
  }

  // Load enriched rules from API and synthesize implicit rules
  let effectiveRules = $state<any[]>([]);

  async function loadRules() {
    loading = true;
    try {
      const data = await api.get("/rules");
      let apiRules: any[] = [];

      // Flatten the nested PolicyWithStats structure
      if (Array.isArray(data)) {
        apiRules = data.flatMap((policy: any) => policy.rules || []);
      }

      effectiveRules = apiRules;
    } catch (e) {
      console.error("Failed to load rules", e);
      effectiveRules = [];
    } finally {
      loading = false;
    }
  }

  // Reload rules when config changes
  $effect(() => {
    if ($config) {
      loadRules();
    }
  });

  function handleEditorCreate() {
    // Open add rule modal with no pre-selected policy (forcing selection)
    selectedPolicy = null;
    openAddRule(null);
  }

  function handleEditorEdit(event: CustomEvent) {
    const { rule } = event.detail;
    // Find the policy
    const policy = policies.find(
      (p: any) => p.from === rule.policy_from && p.to === rule.policy_to,
    );
    if (policy) {
      // Find rule index
      const index = policy.rules.findIndex((r: any) => r.name === rule.name); // improved matching needed?
      if (index !== -1) {
        openEditRule(policy, index);
      }
    }
  }

  function handleEditorDelete(event: CustomEvent) {
    const { id } = event.detail; // id is likely the name? In flatRules we set id = name.
    // We need to find the rule by ID.
    // But looking at flatRules, we set id = name.
    // So we search for rule where name === id
    for (const policy of policies) {
      const idx = (policy.rules || []).findIndex((r: any) => r.name === id);
      if (idx !== -1) {
        deleteRule(policy, idx);
        return;
      }
    }
  }

  function handleEditorToggle(event: CustomEvent) {
    const { id, disabled } = event.detail;
    for (const policy of policies) {
      const idx = (policy.rules || []).findIndex((r: any) => r.name === id);
      if (idx !== -1) {
        // Toggle locally and save
        // We reuse the update logic but formatted for toggle
        const rule = policy.rules[idx];
        const updatedRule = { ...rule, disabled };

        const updatedPolicies = policies.map((p: any) => {
          if (p === policy) {
            const newRules = [...p.rules];
            newRules[idx] = updatedRule;
            return { ...p, rules: newRules };
          }
          return p;
        });

        api
          .updatePolicies(updatedPolicies)
          .then(() => loadRules())
          .catch((e) => console.error("Toggle failed", e));
        return;
      }
    }
  }

  function handleEditorDuplicate(event: CustomEvent) {
    const { rule } = event.detail;
    const policy = policies.find(
      (p: any) => p.from === rule.policy_from && p.to === rule.policy_to,
    );
    if (policy) {
      // We can just open the add modal pre-filled with this rule's data
      selectedPolicy = policy;
      ruleAction = rule.action;
      ruleName = `${rule.name} (Copy)`;
      protocols = rule.proto ? rule.proto.split(",") : [];
      // ... need to fill all fields similar to openEditRule but treating as new
      // For simplicity, let's reuse openEditRule then reset editingRuleIndex to null
      const idx = policy.rules.findIndex((r: any) => r.name === rule.name);
      if (idx !== -1) {
        openEditRule(policy, idx);
        editingRuleIndex = null; // Mark as new mode
        ruleName = ruleName + " (Copy)"; // Override name
      }
    }
  }

  async function handleEditorPromote(event: CustomEvent) {
    const { rule } = event.detail;
    if (
      !confirm(
        $t("policy.promote_confirm", {
          default:
            "Promote this rule to an explicit policy rule? The Zone setting will be disabled.",
        }),
      )
    ) {
      return;
    }

    loading = true;
    try {
      // 1. Disable the implicit setting in Zone
      const zoneName = rule.policy_from;
      const service = rule.service;
      if (!zoneName || !service) {
        throw new Error("Cannot promote rule: missing zone or service");
      }

      const updatedZones = zones.map((z: any) => {
        if (z.name === zoneName) {
          // Clone zone
          const newZone = { ...z };

          // Check Management
          if (newZone.management && service in newZone.management) {
            newZone.management = { ...newZone.management, [service]: false };
          }
          // Check Services
          else if (newZone.services && service in newZone.services) {
            newZone.services = { ...newZone.services, [service]: false };
          }
          return newZone;
        }
        return z;
      });

      // 2. Create the explicit rule
      // We need to add it to the correct policy
      const policyFrom = rule.policy_from;
      const policyTo = rule.policy_to || "firewall"; // Implicit rules are usually to firewall?
      // Wait, rule.policy_to should be set.

      let policyFound = false;
      let updatedPolicies = policies.map((p: any) => {
        if (p.from === policyFrom && p.to === policyTo) {
          policyFound = true;
          return {
            ...p,
            rules: [
              ...(p.rules || []),
              {
                action: rule.action,
                name: rule.name || `Allow ${service}`,
                service: service, // Key: Use the macro!
                description: `Promoted from ${zoneName} zone config`,
                // Protocol/Ports handled by macro
                disabled: false,
                // Clear origin so it's explicit
              },
            ],
          };
        }
        return p;
      });

      if (!policyFound) {
        // Create new policy
        updatedPolicies.push({
          from: policyFrom,
          to: policyTo,
          rules: [
            {
              action: rule.action,
              name: rule.name || `Allow ${service}`,
              service: service,
              description: `Promoted from ${zoneName} zone config`,
              disabled: false,
            },
          ],
        });
      }

      // 3. Save both
      await Promise.all([
        api.updateZones(updatedZones),
        api.updatePolicies(updatedPolicies),
      ]);
      await loadRules();

      // Success notification?
    } catch (e) {
      console.error("Promote failed", e);
      alert("Failed to promote rule: " + e);
    } finally {
      loading = false;
    }
  }
</script>

<div class="policy-page">
  <div class="page-header">
    <h1 class="page-title">{$t("nav.policy")}</h1>
    <Button onclick={() => (isAddingPolicy = !isAddingPolicy)}
      >{isAddingPolicy ? "Cancel" : "+ New Policy Group"}</Button
    >
  </div>

  {#if isAddingPolicy}
    <div class="mb-4">
      <PolicyCreateCard
        {loading}
        {zoneNames}
        on:save={async (e) => {
          const { from, to } = e.detail;
          loading = true;
          try {
            const updated = [...policies, { from, to, rules: [] }];
            await api.updatePolicies(updated);
            isAddingPolicy = false;
          } catch (e) {
            console.error("Failed to create policy", e);
            alert("Failed to create policy");
          } finally {
            loading = false;
          }
        }}
        on:cancel={() => (isAddingPolicy = false)}
      />
    </div>
  {/if}

  {#if isAddingRule}
    <div class="mb-4">
      <RuleCreateCard
        {loading}
        {zones}
        {availableIPSets}
        on:save={handleCreateRule}
        on:cancel={() => (isAddingRule = false)}
      />
    </div>
  {/if}

  <div class="clearpath-container">
    <PolicyEditor
      title="Consolidated Rules"
      showGroupFilter={true}
      rules={effectiveRules}
      isLoading={loading}
      on:create={handleEditorCreate}
      on:edit={handleEditorEdit}
      on:delete={handleEditorDelete}
      on:toggle={handleEditorToggle}
      on:duplicate={handleEditorDuplicate}
      on:promote={handleEditorPromote}
    />
  </div>
</div>

<!-- Add/Edit Rule Modal (Now Edit Only) -->
<Modal
  bind:open={showEditRuleModal}
  title={isEditMode
    ? $t("common.edit_item", { values: { item: $t("item.rule") } })
    : $t("common.add_item", { values: { item: $t("item.rule") } })}
>
  <div class="form-stack">
    <Input
      id="rule-name"
      label={$t("common.name")}
      bind:value={ruleName}
      placeholder={$t("firewall.rule_name_placeholder")}
      required
    />

    <Select
      id="rule-action"
      label={$t("common.action")}
      bind:value={ruleAction}
      options={[
        { value: "accept", label: $t("firewall.accept") },
        { value: "drop", label: $t("firewall.drop") },
        { value: "reject", label: $t("firewall.reject") },
      ]}
    />

    <Select
      id="rule-service"
      label={$t("firewall.quick_service")}
      bind:value={selectedService}
      options={[
        { value: "", label: "-- Custom / Manual --" },
        ...SERVICE_GROUPS.flatMap((group) =>
          group.services.map((svc) => ({
            value: svc.name,
            label: `${group.label}: ${svc.label} (${svc.protocol === "both" ? "tcp+udp" : svc.protocol}/${svc.port})`,
          })),
        ),
      ]}
    />

    <PillInput
      id="rule-protocols"
      label={$t("common.protocol")}
      bind:value={protocols}
      options={PROTOCOL_OPTIONS}
      placeholder={$t("firewall.protocol_placeholder")}
    />

    {#if !protocols.includes("icmp") || protocols.includes("tcp") || protocols.includes("udp")}
      <Input
        id="rule-port"
        label={$t("firewall.dest_port")}
        bind:value={ruleDestPort}
        placeholder={$t("firewall.dest_port_placeholder")}
        type="text"
      />
    {/if}

    <div class="form-row">
      <NetworkInput
        id="rule-src"
        label={$t("firewall.source")}
        bind:value={ruleSrc}
        suggestions={availableIPSets}
        placeholder="IP, CIDR, Host, or IPSet"
      />
      <NetworkInput
        id="rule-dest"
        label={$t("firewall.destination")}
        bind:value={ruleDest}
        suggestions={availableIPSets}
        placeholder="IP, CIDR, Host, or IPSet"
      />
    </div>

    <!-- Advanced Options (collapsible) -->
    <details class="advanced-options" bind:open={showAdvanced}>
      <summary>{$t("firewall.advanced_options")}</summary>
      <div class="advanced-content">
        <Toggle label={$t("firewall.invert_src")} bind:checked={invertSrc} />
        <Toggle label={$t("firewall.invert_dest")} bind:checked={invertDest} />

        <PillInput
          id="rule-tcp-flags"
          label={$t("firewall.tcp_flags")}
          bind:value={tcpFlagsArray}
          options={TCP_FLAG_OPTIONS}
          placeholder={$t("firewall.tcp_flags_placeholder")}
        />

        <Input
          id="rule-max-connections"
          label={$t("firewall.max_connections")}
          bind:value={maxConnections}
          placeholder={$t("firewall.max_connections_placeholder")}
          type="number"
        />
      </div>
    </details>

    <div class="modal-actions">
      <Button variant="ghost" onclick={() => (showEditRuleModal = false)}
        >{$t("common.cancel")}</Button
      >
      <Button onclick={saveRule} disabled={loading || !ruleName}>
        {#if loading}<Spinner size="sm" />{/if}
        {$t("common.save_item", { values: { item: $t("item.rule") } })}
      </Button>
    </div>
  </div>
</Modal>

<style>
  .policy-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    height: 100%;
  }

  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .page-title {
    font-size: var(--text-2xl);
    font-weight: 700;
    color: var(--color-foreground);
    margin: 0;
  }

  .clearpath-container {
    flex: 1;
    min-height: 0;
    /* background: var(--color-backgroundSecondary); removed to let Editor handle its own background */
    border-radius: var(--radius-lg);
    overflow: hidden;
  }

  .form-stack {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    margin-top: var(--space-4);
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
  }

  .form-hint {
    font-size: var(--text-sm);
    color: var(--color-muted);
    margin: 0;
    padding: var(--space-2);
    background: var(--color-backgroundSecondary);
    border-radius: var(--radius-sm);
  }

  .form-hint strong {
    color: var(--color-foreground);
  }
</style>

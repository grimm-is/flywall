<script lang="ts">
  /**
   * NAT Page
   * Port forwarding and NAT rules
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
    Icon,
  } from "$lib/components";
  import NatCreateCard from "$lib/components/NatCreateCard.svelte";
  import { t } from "svelte-i18n";

  let loading = $state(false);
  let isAddingRule = $state(false);

  const natRules = $derived($config?.nat || []);
  const interfaces = $derived($config?.interfaces || []);

  const natColumns = [
    { key: "type", label: "Type" },
    { key: "proto", label: $t("common.protocol") },
    { key: "dest_port", label: $t("common.destination") },
    { key: "to_ip", label: "Forward To" },
    { key: "snat_ip", label: "SNAT IP" },
    { key: "description", label: $t("common.description") },
  ];

  function toggleAddRule() {
    isAddingRule = !isAddingRule;
  }

  async function handleCreateRule(event: CustomEvent) {
    const data = event.detail;
    loading = true;
    try {
      let newRule: any;

      if (data.type === "masquerade") {
        newRule = {
          type: "masquerade",
          out_interface: data.outInterface,
        };
      } else if (data.type === "snat") {
        newRule = {
          type: "snat",
          out_interface: data.outInterface,
          src_ip: data.srcIP,
          mark: data.mark ? parseInt(data.mark) : 0,
          snat_ip: data.snatIP,
          description: data.description,
        };
      } else {
        newRule = {
          type: "dnat",
          proto: data.proto,
          dest_port: String(data.destPort),
          to_ip: data.toIP,
          to_port: data.toPort ? String(data.toPort) : String(data.destPort),
          description: data.description,
        };
      }

      await api.updateNAT([...natRules, newRule]);
      isAddingRule = false;
    } catch (e: any) {
      console.error("Failed to add NAT rule:", e);
    } finally {
      loading = false;
    }
  }

  async function deleteRule(index: number) {
    loading = true;
    try {
      const updatedRules = natRules.filter((_: any, i: number) => i !== index);
      await api.updateNAT(updatedRules);
    } catch (e) {
      console.error("Failed to delete NAT rule:", e);
    } finally {
      loading = false;
    }
  }
</script>

<div class="nat-page">
  <div class="page-header">
    <Button onclick={toggleAddRule}
      >{isAddingRule
        ? $t("common.cancel")
        : `+ ${$t("common.add_item", { values: { item: $t("item.rule") } })}`}</Button
    >
  </div>

  {#if isAddingRule}
    <div class="mb-4">
      <NatCreateCard
        {loading}
        {interfaces}
        on:save={handleCreateRule}
        on:cancel={toggleAddRule}
      />
    </div>
  {/if}

  {#if natRules.length === 0}
    <Card>
      <div class="empty-state">
        <Icon name="arrow-left-right" size="lg" />
        <h3>{$t("common.no_items", { values: { items: $t("item.rule") } })}</h3>
        <p>
          {$t("nat.port_forwarding_desc")}
        </p>
        <Button onclick={toggleAddRule}>
          <Icon name="plus" size="sm" />
          {$t("common.create_item", { values: { item: $t("item.rule") } })}
        </Button>
      </div>
    </Card>
  {:else}
    <Card>
      <div class="rules-list">
        {#each natRules as rule, index}
          <div class="rule-row">
            <Badge
              variant={rule.type === "masquerade" ? "secondary" : "default"}
            >
              {$t(`nat.type_${rule.type}`)}
            </Badge>

            {#if rule.type === "masquerade"}
              <span class="rule-detail">Outbound on {rule.out_interface}</span>
            {:else if rule.type === "snat"}
              <span class="rule-detail">
                <span class="mono">{rule.src_ip || "Any"}</span>
                {#if rule.mark}
                  <Badge variant="outline">Mk:{rule.mark}</Badge>
                {/if}
                → SNAT: <span class="mono">{rule.snat_ip}</span>
                via {rule.out_interface}
              </span>
            {:else}
              <span class="rule-detail mono">
                {rule.proto?.toUpperCase() || "TCP"}:{rule.dest_port} → {rule.to_ip}:{rule.to_port ||
                  rule.dest_port}
              </span>
              {#if rule.description}
                <span class="rule-desc">{rule.description}</span>
              {/if}
            {/if}

            <Button variant="ghost" size="sm" onclick={() => deleteRule(index)}
              ><Icon name="delete" size="sm" /></Button
            >
          </div>
        {/each}
      </div>
    </Card>
  {/if}
</div>

<style>
  .nat-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .rules-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .rule-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3);
    background-color: var(--color-backgroundSecondary);
    border-radius: var(--radius-md);
  }

  .rule-detail {
    flex: 1;
    color: var(--color-foreground);
  }

  .rule-desc {
    color: var(--color-muted);
    font-size: var(--text-sm);
  }

  .mono {
    font-family: var(--font-mono);
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-8);
    text-align: center;
    color: var(--color-muted);
  }

  .empty-state h3 {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: 600;
    color: var(--color-foreground);
  }

  .empty-state p {
    margin: 0;
    max-width: 400px;
    font-size: var(--text-sm);
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

  .validation-error {
    padding: var(--space-3);
    background-color: rgba(220, 38, 38, 0.1);
    border: 1px solid var(--color-destructive);
    border-radius: var(--radius-md);
    color: var(--color-destructive);
    font-size: var(--text-sm);
  }
</style>

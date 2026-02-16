<script lang="ts">
  import { hasPendingChanges, api, alertStore } from "$lib/stores/app";
  import { t } from "svelte-i18n";
  import Modal from "./Modal.svelte";
  import Button from "./Button.svelte";
  import Icon from "./Icon.svelte";

  let applying = $state(false);
  let discarding = $state(false);
  let confirmOpen = $state(false);
  let verify = $state(true);
  let pingTargetsText = $state("8.8.8.8");

  // Countdown / verification state
  let applyPhase = $state<"idle" | "applying" | "verifying" | "success" | "failed">("idle");
  let countdown = $state(0);
  let countdownMax = $state(30);
  let countdownInterval: ReturnType<typeof setInterval> | null = null;
  let verificationMessage = $state("");

  function startCountdown(seconds: number) {
    countdown = seconds;
    countdownMax = seconds;
    if (countdownInterval) clearInterval(countdownInterval);
    countdownInterval = setInterval(() => {
      countdown -= 1;
      if (countdown <= 0 && countdownInterval) {
        clearInterval(countdownInterval);
        countdownInterval = null;
      }
    }, 1000);
  }

  function stopCountdown() {
    if (countdownInterval) {
      clearInterval(countdownInterval);
      countdownInterval = null;
    }
    countdown = 0;
  }

  async function handleApply() {
    confirmOpen = true;
  }

  async function executeApply() {
    applying = true;
    applyPhase = "applying";
    verificationMessage = "Applying configuration...";

    try {
      let targets: string[] = [];
      if (verify && pingTargetsText.trim()) {
        targets = pingTargetsText
          .split("\n")
          .map((s) => s.trim())
          .filter((s) => s.length > 0);
      }

      // Start countdown if we have verification targets
      if (targets.length > 0) {
        applyPhase = "verifying";
        verificationMessage = `Verifying connectivity to ${targets.length} target(s)...`;
        startCountdown(targets.length * 5 + 10); // ~5s per target + buffer
      }

      const res = await api.safeApplyConfig(targets);
      stopCountdown();

      if (res.success) {
        applyPhase = "success";
        verificationMessage = "";
        if (res.warning) {
          alertStore.show(
            $t("config.applied_warning", { message: res.warning } as any),
            "warning",
          );
        } else {
          alertStore.success($t("config.applied_success"));
        }
        confirmOpen = false;
      } else {
        applyPhase = "failed";
        verificationMessage = res.message || "Verification failed";
        alertStore.error(res.message || res.error || $t("config.apply_failed"));
        if (res.rolled_back) {
          verificationMessage = "Changes rolled back due to verification failure";
          // Brief delay then close
          setTimeout(() => {
            confirmOpen = false;
            applyPhase = "idle";
          }, 2000);
        }
      }
    } catch (e: any) {
      stopCountdown();
      applyPhase = "failed";
      verificationMessage = e.message || "Apply failed";
      alertStore.error(e.message || $t("config.apply_failed"));
    } finally {
      applying = false;
      // Reset phase after a delay if needed
      if (applyPhase === "success") {
        setTimeout(() => { applyPhase = "idle"; }, 1000);
      }
    }
  }

  async function handleDiscard() {
    discarding = true;
    try {
      await api.discardConfig();
      alertStore.success($t("config.discarded_success"));
    } catch (e: any) {
      alertStore.error(e.message || $t("config.discard_failed"));
    } finally {
      discarding = false;
    }
  }
</script>

{#if $hasPendingChanges}
  <div class="staged-bar">
    <div class="staged-content">
      <div class="staged-info">
        <Icon name="alert-circle" size="md" class="text-amber-400" />
        <span class="staged-text">{$t("common.unsaved_changes")}</span>
      </div>
      <div class="staged-actions">
        <Button
          variant="outline"
          size="sm"
          onclick={handleDiscard}
          loading={discarding}
          disabled={applying}
        >
          {$t("common.discard")}
        </Button>
        <Button
          variant="default"
          size="sm"
          onclick={handleApply}
          loading={applying}
          disabled={discarding}
        >
          {$t("config.apply_changes")}
        </Button>
      </div>
    </div>
  </div>

  <Modal bind:open={confirmOpen} title={$t("config.apply_changes")} size="md">
    <div class="confirm-content">
      <p class="mb-4 text-muted-foreground">
        {$t("config.apply_confirmation_text", {
          default:
            "Are you sure you want to apply pending changes? This will construct a new firewall ruleset and apply it to the system.",
        })}
      </p>

      <div class="verify-section p-4 bg-muted rounded-lg mb-6">
        <label class="flex items-center gap-2 mb-2 font-medium cursor-pointer">
          <input
            type="checkbox"
            bind:checked={verify}
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          <span
            >{$t("config.verify_connectivity", {
              default: "Verify Connectivity",
            })}</span
          >
        </label>

        {#if verify}
          <div class="mt-2">
            <label class="block text-xs font-medium text-muted-foreground mb-1">
              {$t("config.ping_targets", {
                default: "Ping Targets (one IP per line)",
              })}
            </label>
            <textarea
              bind:value={pingTargetsText}
              class="w-full h-24 p-2 text-sm bg-background border rounded focus:ring-2 focus:ring-primary-500 outline-none"
              placeholder="8.8.8.8"
            ></textarea>
            <p class="text-xs text-muted-foreground mt-1">
              {$t("config.verify_help", {
                default:
                  "If verification fails, changes will be automatically rolled back.",
              })}
            </p>
          </div>
        {/if}
      </div>

      {#if applyPhase !== "idle"}
        <div class="apply-progress" class:success={applyPhase === "success"} class:failed={applyPhase === "failed"}>
          <div class="progress-header">
            {#if applyPhase === "applying"}
              <Icon name="sync" size="sm" />
              <span>Applying configuration...</span>
            {:else if applyPhase === "verifying"}
              <Icon name="network_check" size="sm" />
              <span>{verificationMessage}</span>
            {:else if applyPhase === "success"}
              <Icon name="check_circle" size="sm" />
              <span>Configuration applied successfully!</span>
            {:else if applyPhase === "failed"}
              <Icon name="error" size="sm" />
              <span>{verificationMessage}</span>
            {/if}
          </div>
          {#if countdown > 0}
            <div class="countdown-container">
              <progress value={countdown} max={countdownMax}></progress>
              <span class="countdown-timer">{countdown}s remaining</span>
            </div>
          {/if}
        </div>
      {/if}

      <div class="flex justify-end gap-3">
        <Button
          variant="ghost"
          onclick={() => (confirmOpen = false)}
          disabled={applying}
        >
          {$t("common.cancel")}
        </Button>
        <Button variant="default" onclick={executeApply} loading={applying}>
          {$t("config.apply")}
        </Button>
      </div>
    </div>
  </Modal>
{/if}

<style>
  .staged-bar {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    background: var(--color-surface);
    border-top: 2px solid var(--color-warning);
    padding: var(--space-3) var(--space-4);
    z-index: var(--z-modal);
    box-shadow: 0 -4px 12px rgba(0, 0, 0, 0.2);
  }

  .staged-content {
    max-width: 1200px;
    margin: 0 auto;
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-4);
  }

  .staged-info {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .staged-text {
    color: var(--color-foreground);
    font-weight: 500;
  }

  .staged-actions {
    display: flex;
    gap: var(--space-2);
  }

  textarea {
    resize: vertical;
    border-color: var(--color-border);
  }

  .apply-progress {
    margin-top: var(--space-4);
    padding: var(--space-4);
    border-radius: var(--radius-lg);
    background: var(--color-backgroundSecondary);
    border: 1px solid var(--color-border);
  }

  .apply-progress.success {
    background: color-mix(in srgb, var(--color-success) 10%, transparent);
    border-color: var(--color-success);
  }

  .apply-progress.failed {
    background: color-mix(in srgb, var(--color-destructive) 10%, transparent);
    border-color: var(--color-destructive);
  }

  .progress-header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    font-weight: 500;
  }

  .apply-progress.success .progress-header {
    color: var(--color-success);
  }

  .apply-progress.failed .progress-header {
    color: var(--color-destructive);
  }

  .countdown-container {
    margin-top: var(--space-3);
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .countdown-container progress {
    flex: 1;
    height: 8px;
    border-radius: 4px;
    appearance: none;
  }

  .countdown-container progress::-webkit-progress-bar {
    background: var(--color-border);
    border-radius: 4px;
  }

  .countdown-container progress::-webkit-progress-value {
    background: var(--color-warning);
    border-radius: 4px;
    transition: width 1s linear;
  }

  .countdown-container progress::-moz-progress-bar {
    background: var(--color-warning);
    border-radius: 4px;
  }

  .countdown-timer {
    font-size: var(--text-xs);
    font-family: var(--font-mono);
    color: var(--color-warning);
    font-weight: 600;
    white-space: nowrap;
  }
</style>

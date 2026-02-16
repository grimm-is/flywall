<script lang="ts">
  import FlowTable from "$lib/components/FlowTable.svelte";
  import DVRScrubber from "$lib/components/DVRScrubber.svelte";
  import { onMount, onDestroy } from "svelte";
  import { flowActions, dvr } from "$lib/stores/flows";

  // Mock Data Generator for Prototype Visualization
  // In real implementation, this would be replaced by WebSocket connection
  let mockInterval: any;

  onMount(() => {
    // 1. Fetch History from Backend (Time Travel Prep)
    dvr.fetchHistory();

    // 2. Start Live Updates (Mock)
    // Generate some realistic-looking traffic patterns
    mockInterval = setInterval(() => {
      const count = 50 + Math.floor(Math.random() * 20); // 50-70 flows
      const mockUpdate = [];

      for (let i = 0; i < count; i++) {
        const id = `flow-${i}`;
        // Simulate stable IP pairs for most flows
        const src = `10.0.0.${10 + (i % 20)}`;
        const dest = `8.8.8.${10 + (i % 5)}`;

        // Simulate accumulating counters
        mockUpdate.push({
          id,
          src_ip: src,
          src_port: 30000 + i,
          dest_ip: dest,
          dest_port: 443,
          protocol: i % 3 === 0 ? "UDP" : "TCP",
          bytes: Date.now() * (10 + (i % 10)) + Math.random() * 10000, // Continually growing
          packets: Date.now() / 100, // Continually growing
        });
      }

      flowActions.handleUpdate(mockUpdate);
    }, 1000);
  });

  onDestroy(() => {
    if (mockInterval) clearInterval(mockInterval);
  });
</script>

<div class="discovery-page p-6 max-w-[1600px] mx-auto flex flex-col gap-6">
  <header class="flex justify-between items-end">
    <div>
      <h1 class="text-3xl font-bold tracking-tight text-glow">Discovery</h1>
      <p class="text-muted tracking-widest text-sm uppercase">
        Network Visibility & Flow Tracking
      </p>
    </div>
    <div class="w-[400px]">
      <DVRScrubber />
    </div>
  </header>

  <main class="flex-1 min-h-0">
    <FlowTable />
  </main>
</div>

<style>
  .discovery-page {
    min-height: 100vh;
  }
</style>

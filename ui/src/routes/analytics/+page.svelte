<script lang="ts">
  import { onMount } from "svelte";
  import { api } from "$lib/stores/app";
  import { formatBytes } from "$lib/stores/flows";
  import Icon from "$lib/components/Icon.svelte";
  import * as d3 from "d3";

  // State
  let timeRange = $state("24h");
  let bandwidthPoints = $state<any[]>([]);
  let topTalkers = $state<any[]>([]);
  let flows = $state<any[]>([]);
  let loading = $state(false);
  let chartContainer = $state<HTMLElement | null>(null);

  const ranges = [
    { id: "1h", label: "Last Hour", value: 3600 },
    { id: "6h", label: "Last 6 Hours", value: 6 * 3600 },
    { id: "24h", label: "Last 24 Hours", value: 24 * 3600 },
    { id: "7d", label: "Last 7 Days", value: 7 * 24 * 3600 },
  ];

  async function loadData() {
    loading = true;
    try {
      const range = ranges.find((r) => r.id === timeRange) || ranges[2];
      const now = Math.floor(Date.now() / 1000);
      const from = now - range.value;

      const [bw, tt, historical] = await Promise.all([
        api.getAnalyticsBandwidth({ from, to: now }),
        api.getAnalyticsTopTalkers({ from, to: now, limit: 10 }),
        api.getAnalyticsFlows({ from, to: now, limit: 20 }),
      ]);

      bandwidthPoints = bw.points || [];
      topTalkers = tt.summaries || [];
      flows = historical.flows || [];

      renderChart();
    } catch (e) {
      console.error("Failed to load analytics data", e);
    } finally {
      loading = false;
    }
  }

  function renderChart() {
    if (!chartContainer || bandwidthPoints.length === 0) return;

    // Clear previous chart
    d3.select(chartContainer).selectAll("*").remove();

    const margin = { top: 20, right: 30, bottom: 30, left: 60 };
    const width = chartContainer.clientWidth - margin.left - margin.right;
    const height = 250 - margin.top - margin.bottom;

    const svg = d3
      .select(chartContainer)
      .append("svg")
      .attr("width", width + margin.left + margin.right)
      .attr("height", height + margin.top + margin.bottom)
      .append("g")
      .attr("transform", `translate(${margin.left},${margin.top})`);

    // Parse dates
    const data = bandwidthPoints.map((d) => ({
      time: new Date(d.time),
      bytes: d.bytes,
    }));

    const x = d3
      .scaleTime()
      .domain(d3.extent(data, (d) => d.time) as [Date, Date])
      .range([0, width]);

    const y = d3
      .scaleLinear()
      .domain([0, d3.max(data, (d) => d.bytes) || 0])
      .range([height, 0]);

    // X Axis
    svg
      .append("g")
      .attr("transform", `translate(0,${height})`)
      .call(d3.axisBottom(x).ticks(5))
      .attr("class", "axis");

    // Y Axis
    svg
      .append("g")
      .call(d3.axisLeft(y).tickFormat((d) => formatBytes(Number(d))))
      .attr("class", "axis");

    // Area
    const area = d3
      .area<any>()
      .x((d) => x(d.time))
      .y0(height)
      .y1((d) => y(d.bytes))
      .curve(d3.curveMonotoneX);

    svg
      .append("path")
      .datum(data)
      .attr("fill", "var(--color-primary)")
      .attr("fill-opacity", 0.1)
      .attr("d", area);

    // Line
    const line = d3
      .line<any>()
      .x((d) => x(d.time))
      .y((d) => y(d.bytes))
      .curve(d3.curveMonotoneX);

    svg
      .append("path")
      .datum(data)
      .attr("fill", "none")
      .attr("stroke", "var(--color-primary)")
      .attr("stroke-width", 2)
      .attr("d", line);
  }

  onMount(() => {
    loadData();
    const handleResize = () => renderChart();
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  });

  $effect(() => {
    if (timeRange) loadData();
  });
</script>

<div class="analytics-page">
  <header class="page-header">
    <div class="page-title">
      <h1>Historical Analytics</h1>
      <span class="muted">Retrospective traffic analysis</span>
    </div>
    <div class="header-actions">
      <div class="range-selector">
        {#each ranges as range}
          <button
            class="range-btn"
            class:active={timeRange === range.id}
            onclick={() => (timeRange = range.id)}
          >
            {range.label}
          </button>
        {/each}
      </div>
      <button class="btn-refresh" onclick={loadData} disabled={loading}>
        <Icon name="refresh" size={16} />
      </button>
    </div>
  </header>

  <section class="section chart-section">
    <div class="section-header">
      <h2>Bandwidth Usage</h2>
    </div>
    <div class="chart-card">
      <div bind:this={chartContainer} class="chart-content">
        {#if bandwidthPoints.length === 0 && !loading}
          <div class="empty-chart">No bandwidth data for this period</div>
        {/if}
      </div>
    </div>
  </section>

  <div class="details-grid">
    <section class="section">
      <div class="section-header">
        <h2>Top Talkers</h2>
      </div>
      <div class="card overflow-x-auto">
        <table class="data-table">
          <thead>
            <tr>
              <th>Device</th>
              <th>MAC</th>
              <th>Total Traffic</th>
            </tr>
          </thead>
          <tbody>
            {#each topTalkers as talker}
              <tr>
                <td class="font-medium">Device {talker.src_mac.slice(-5)}</td>
                <td class="muted">{talker.src_mac}</td>
                <td class="font-mono">{formatBytes(talker.bytes)}</td>
              </tr>
            {:else}
              <tr>
                <td colspan="3" class="text-center p-8 muted">No data found</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>

    <section class="section">
      <div class="section-header">
        <h2>Recent Historical Flows</h2>
      </div>
      <div class="card overflow-x-auto">
        <table class="data-table">
          <thead>
            <tr>
              <th>Time</th>
              <th>Destination</th>
              <th>Protocol</th>
              <th>Traffic</th>
              <th>Class</th>
            </tr>
          </thead>
          <tbody>
            {#each flows as flow}
              <tr>
                <td class="text-xs"
                  >{new Date(flow.bucket_time).toLocaleString()}</td
                >
                <td>
                  <div class="flow-dest">
                    <span class="ip">{flow.dst_ip}</span>
                    <span class="port">:{flow.dst_port}</span>
                  </div>
                </td>
                <td class="text-xs uppercase">{flow.protocol}</td>
                <td class="font-mono">{formatBytes(flow.bytes)}</td>
                <td>
                  {#if flow.class}
                    <span class="badge" data-class={flow.class.toLowerCase()}
                      >{flow.class}</span
                    >
                  {:else}
                    <span class="muted">-</span>
                  {/if}
                </td>
              </tr>
            {:else}
              <tr>
                <td colspan="5" class="text-center p-8 muted">No data found</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  </div>
</div>

<style>
  .analytics-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }

  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .page-title h1 {
    font-size: var(--text-2xl);
    font-weight: 600;
  }

  .muted {
    font-size: var(--text-sm);
    color: var(--dashboard-text-muted);
  }

  .header-actions {
    display: flex;
    gap: var(--space-4);
    align-items: center;
  }

  .range-selector {
    display: flex;
    background: var(--dashboard-card);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-md);
    padding: 2px;
  }

  .range-btn {
    padding: var(--space-1) var(--space-3);
    font-size: var(--text-xs);
    border: none;
    background: transparent;
    color: var(--dashboard-text-muted);
    cursor: pointer;
    border-radius: var(--radius-sm);
    transition: all 0.2s;
  }

  .range-btn.active {
    background: var(--dashboard-input);
    color: var(--dashboard-text);
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
  }

  .btn-refresh {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    background: var(--dashboard-card);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-md);
    color: var(--dashboard-text);
    cursor: pointer;
  }

  .section-header {
    margin-bottom: var(--space-4);
  }

  .section-header h2 {
    font-size: var(--text-lg);
    font-weight: 600;
  }

  .chart-card {
    background: var(--dashboard-card);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-lg);
    padding: var(--space-6);
  }

  .chart-content {
    min-height: 250px;
    width: 100%;
  }

  .empty-chart {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 250px;
    color: var(--dashboard-text-muted);
  }

  .details-grid {
    display: grid;
    grid-template-columns: 1fr 2fr;
    gap: var(--space-6);
  }

  .card {
    background: var(--dashboard-card);
    border: 1px solid var(--dashboard-border);
    border-radius: var(--radius-lg);
    overflow: hidden;
  }

  .data-table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
    font-size: var(--text-sm);
  }

  .data-table th {
    padding: var(--space-3) var(--space-4);
    background: var(--color-backgroundSecondary);
    border-bottom: 1px solid var(--dashboard-border);
    font-weight: 600;
    color: var(--dashboard-text-muted);
    text-transform: uppercase;
    font-size: var(--text-xs);
    letter-spacing: 0.05em;
  }

  .data-table td {
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--dashboard-border);
  }

  .font-mono {
    font-family: var(--font-mono);
  }

  .flow-dest {
    display: flex;
    align-items: center;
  }

  .port {
    color: var(--dashboard-text-muted);
  }

  .badge {
    padding: var(--space-0-5) var(--space-2);
    border-radius: var(--radius-full);
    font-size: var(--text-xs);
    font-weight: 500;
    text-transform: uppercase;
  }

  .badge[data-class="streaming"] {
    background: rgba(var(--color-primary-rgb), 0.1);
    color: var(--color-primary);
  }
  .badge[data-class="web"] {
    background: rgba(16, 185, 129, 0.1);
    color: #10b981;
  }
  .badge[data-class="gaming"] {
    background: rgba(245, 158, 11, 0.1);
    color: #f59e0b;
  }
  .badge[data-class="iot"] {
    background: rgba(139, 92, 246, 0.1);
    color: #8b5cf6;
  }
  .badge[data-class="malicious"] {
    background: rgba(239, 68, 68, 0.1);
    color: #ef4444;
  }

  @media (max-width: 1024px) {
    .details-grid {
      grid-template-columns: 1fr;
    }
  }

  :global(.axis path, .axis line) {
    stroke: var(--dashboard-border);
  }

  :global(.axis text) {
    fill: var(--dashboard-text-muted);
    font-size: 10px;
  }
</style>

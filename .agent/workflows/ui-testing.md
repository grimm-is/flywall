---
description: How to run Automated UI-HCL Generation Tests
---

# Automated UI-HCL Testing Framework

This workflow allows you to verify that UI actions correctly generate the expected HCL configuration. It uses **Playwright** for browser automation and a **staged backend** for verification, allowing tests to run on macOS without a Linux kernel.

## Architecture

1.  **Backend (Simulator)**: `flywall ctl` runs in a local environment with `disable_sandbox=true`. It accepts API requests and updates its in-memory "staged" configuration.
2.  **Frontend**: The Svelte app runs via Vite, proxying API requests to the simulator.
3.  **Verification**: Playwright navigates the UI, performs actions, and then queries `GET /api/config` to verify the staged configuration matches expectations.

## Prerequisites

- Node.js (v18+)
- Go (1.21+)

## Setup

1.  Install UI dependencies:
    ```bash
    cd ui
    npm install
    npx playwright install
    ```

2.  Ensure backend builds (optional check):
    ```bash
    go build -o flywall .
    ```

## Running Tests

We have created helper scripts to automate the process:

```bash
# Run all UI E2E tests
./scripts/run-ui-tests.sh
```

This script will:
1.  Start the Flywall backend on port `:8081` (using `configs/mac_test.hcl`).
2.  Start the Vite dev server.
3.  Run Playwright tests.
4.  Clean up background processes.

## Writing New Tests

Add new test scenarios to `ui/tests/e2e/hcl_gen.spec.ts`.

Example:

```typescript
test('Enable Feature X', async ({ page, request }) => {
    await page.goto('/settings');
    await page.getByLabel('Feature X').check();

    // Verify backend state
    const response = await request.get(`${process.env.API_URL}/api/config`);
    const config = await response.json();
    expect(config.feature_x).toBe(true);
});
```

## Troubleshooting

- **Backend fails to start**: Check `local/log/flywall.log`. Ensure ports 8081 and 5173 are free.
- **Playwright timeout**: Increase timeout in `playwright.config.ts`.
- **"bind: permission denied"**: Ensure `local/run` directory exists and is writable (the script creates it).

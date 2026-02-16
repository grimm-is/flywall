import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright config for testing against the real Demo VM
 * Run with: npm run test:e2e:demo
 *
 * Requires: ./flywall.sh demo to be running first
 */
export default defineConfig({
    testDir: './',
    fullyParallel: false,
    retries: 0,
    workers: 1, // Sequential execution to avoid state conflict
    reporter: 'list',
    timeout: 60000, // Longer timeout for real API responses
    use: {
        // @ts-ignore
        baseURL: `https://localhost:${process.env.DEMO_PORT || '8443'}`,
        ignoreHTTPSErrors: true, // Demo VM uses self-signed cert
        trace: 'on-first-retry',
    },
    projects: [
        {
            name: 'chromium',
            use: { ...devices['Desktop Chrome'] },
        },
    ],
    // Note: Demo VM must be started manually with: ./flywall.sh demo
    // Not using webServer because it's a long-running VM, not a subprocess
});

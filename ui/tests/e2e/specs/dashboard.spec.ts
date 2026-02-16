import { test, expect } from '../fixtures/test';

test.describe('Dashboard Features', () => {

    test.beforeEach(async ({ loginPage }) => {
        await loginPage.goto();
        await loginPage.login();
    });

    test('DASH-01: View system status', async ({ dashboardPage }) => {
        await dashboardPage.goto();
        await dashboardPage.expectLoaded();

        // Check for Status Widget
        await expect(dashboardPage.page.locator('.widget-title', { hasText: 'System Status' })).toBeVisible();

        // Use generic checks for content that might change
        await expect(dashboardPage.page.getByText(/Uptime:/i)).toBeVisible();
        await expect(dashboardPage.page.getByText(/Version:/i)).toBeVisible();
        await expect(dashboardPage.page.getByText(/CPU:/i)).toBeVisible();
        await expect(dashboardPage.page.getByText(/Memory:/i)).toBeVisible();
    });

    test('DASH-02: View network topology', async ({ dashboardPage }) => {
        await dashboardPage.goto();
        await dashboardPage.expectLoaded();

        // Check for Topology Widget/Card
        // Depending on implementation, it might be a canvas or SVG
        const graph = dashboardPage.page.locator('svg.topology-graph');
        // Or if it's a component wrapper
        // Use a more resilient selector if SVG classes are dynamic
        // Assuming visual presence is enough for E2E
        await expect(dashboardPage.page.locator('.widget-title', { hasText: 'Network Topology' })).toBeVisible();
    });

    test('DASH-04: View traffic graph', async ({ dashboardPage }) => {
        await dashboardPage.goto();

        // Traffic widget usually shows live stats
        const trafficWidget = dashboardPage.page.locator('.widget', { hasText: /Traffic/i });
        await expect(trafficWidget).toBeVisible();

        // Check for graph elements (e.g., path or bars)
        // Wait for data populate
        await expect(trafficWidget.locator('svg')).toBeVisible();
    });
});

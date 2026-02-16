import { test, expect } from '../fixtures/test';

test.describe('Dashboard Widgets', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
    });

    test('should display all new widgets', async ({ page }) => {
        // Check for widget titles
        await expect(page.getByRole('heading', { name: 'Bandwidth' })).toBeVisible();
        await expect(page.getByRole('heading', { name: 'Active Connections' })).toBeVisible();
        await expect(page.getByRole('heading', { name: 'DNS Queries' })).toBeVisible();
        await expect(page.getByRole('heading', { name: 'DHCP Leases' })).toBeVisible();
        await expect(page.getByRole('heading', { name: 'System Health' })).toBeVisible();
        await expect(page.getByRole('heading', { name: 'Recent Alerts' })).toBeVisible();
    });

    test('should show connection stats', async ({ page }) => {
        const connsWidget = page.locator('.widget-wrapper', { hasText: 'Active Connections' });
        await expect(connsWidget.getByText('TCP')).toBeVisible();
        await expect(connsWidget.getByText('UDP')).toBeVisible();
    });

    test('should show bandwidth sparkline', async ({ page }) => {
        const bandwidthWidget = page.locator('.widget-wrapper', { hasText: 'Bandwidth' });
        await expect(bandwidthWidget.locator('svg')).toBeVisible();
        await expect(bandwidthWidget.getByText('Aggregated Traffic')).toBeVisible();
    });
});

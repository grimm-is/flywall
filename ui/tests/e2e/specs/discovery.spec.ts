import { test, expect } from '../fixtures/test';

test.describe('Discovery Features', () => {

    test.beforeEach(async ({ loginPage }) => {
        await loginPage.goto();
        await loginPage.login();
    });

    test('DISC-02: View discovered devices/networks', async ({ discoveryPage }) => {
        // NOTE: The mock-api has /api/scanner/result providing wifi results.
        // Assuming 'Discovery' page consumes this.
        await discoveryPage.goto();

        // Check for data from mock-api
        // { ssid: 'Flywall-Secure', ... }
        await discoveryPage.expectDeviceVisible('Flywall-Secure');
        await discoveryPage.expectDeviceVisible('Neighbor-Wifi');
    });

    test('DISC-01: Start scan', async ({ discoveryPage }) => {
        await discoveryPage.goto();
        // Check if scan button exists
        const scanBtn = discoveryPage.page.getByRole('button', { name: /Scan/i });
        if (await scanBtn.isVisible()) {
            await discoveryPage.startScan();
            // Verify feedback?
            // "Scan started" toast or similar
            await expect(discoveryPage.page.locator('.toast')).toBeVisible();
        }
    });
});

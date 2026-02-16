import { test, expect } from '../fixtures/test';

test.describe('System Configuration', () => {

    test.beforeEach(async ({ loginPage }) => {
        await loginPage.goto();
        await loginPage.login();
    });

    test('General Settings', async ({ systemPage }) => {
        await systemPage.gotoSettings();

        await test.step('Toggle Global Settings', async () => {
            const configStart = await systemPage.getConfig();

            // Toggle IP Forwarding
            await systemPage.toggleSetting('IP Forwarding', !configStart.ip_forwarding);
            await systemPage.toggleSetting('Flow Offload', true);
            await systemPage.toggleSetting('MSS Clamping', true);

            await systemPage.page.waitForTimeout(1000); // Wait for API

            const config = await systemPage.getConfig();
            expect(config.ip_forwarding).toBe(!configStart.ip_forwarding);
            expect(typeof config.enable_flow_offload).toBe('boolean');
            expect(typeof config.mss_clamping).toBe('boolean');
        });
    });
});

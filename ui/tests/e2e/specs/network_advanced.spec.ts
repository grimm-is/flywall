import { test, expect } from '../fixtures/test';

test.describe('Advanced Network Features', () => {

    test.beforeEach(async ({ loginPage }) => {
        await loginPage.goto();
        await loginPage.login();
    });

    test('IFACE-04: Create a VLAN', async ({ networkPage }) => {
        await networkPage.goto('interfaces');

        await test.step('Add VLAN 10 to eth1', async () => {
            await networkPage.interfaces.addVlan('eth1', '10', 'Guest', 'Guest VLAN');

            const config = await networkPage.getConfig();
            const vlan = config.interfaces.find((i: any) => i.name === 'eth1.10');
            expect(vlan).toBeDefined();
            expect(vlan.zone).toBe('Guest');
        });
    });

    test('IFACE-05: Create a Bond', async ({ networkPage }) => {
        await networkPage.goto('interfaces');

        await test.step('Create bond0', async () => {
            // Ensure members exist? Mock API says eth3/eth4 available.
            await networkPage.interfaces.addBond('bond0', ['eth3', 'eth4'], 'LAN');

            const config = await networkPage.getConfig();
            const bond = config.interfaces.find((i: any) => i.name === 'bond0');
            expect(bond).toBeDefined();
        });
    });
});

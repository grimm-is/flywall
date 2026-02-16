import { test, expect } from '../fixtures/test';
import { Buffer } from 'node:buffer';

test.describe('VPN Configuration', () => {

    test.beforeEach(async ({ loginPage }) => {
        await loginPage.goto();
        await loginPage.login();
    });

    test('WireGuard Configuration', async ({ tunnelsPage }) => {
        await tunnelsPage.goto();

        await test.step('Create WireGuard Interface', async () => {
            await tunnelsPage.addWireGuard(
                'Test WG',
                'wg0',
                '51820',
                'cR+1234567890abcdef1234567890abcdef12345=',
                '10.100.0.1/24'
            );

            const config = await tunnelsPage.getConfig();
            const wg = config.vpn?.wireguard?.find((w: any) => w.name === 'Test WG');
            expect(wg).toBeDefined();
        });

        await test.step('Import VPN Config', async () => {
            const fileContent = `
[Interface]
PrivateKey = aaaaaa
Address = 10.200.0.5/24
DNS = 1.1.1.1
MTU = 1350
Table = 100

[Peer]
PublicKey = bbbbbb
Endpoint = 1.2.3.4:51820
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
`;
            const buffer = Buffer.from(fileContent);
            await tunnelsPage.importConfig({ buffer, filename: 'vpn-import.conf' });

            const config = await tunnelsPage.getConfig();
            const tunnel = config.vpn?.wireguard?.find((w: any) => w.private_key === 'aaaaaa');
            expect(tunnel).toBeDefined();
            expect(tunnel.table).toBe('100');
            expect(tunnel.peers).toHaveLength(1);
        });

        await test.step('Manual Client Configuration with Advanced Routing', async () => {
            await tunnelsPage.addWireGuard(
                'Split Tunnel VPN',
                'wg-split',
                '0',
                'cR+9999999999abcdef1234567890abcdef12345=',
                '', // addr empty? Original test didn't fill it?
                // Original test checked line 156: privkey but didn't fill addr?
                // Looking at legacy test line 145: it fills vpn-conn-addr? No.
                // Wait, checks line 121: expect(addr).toHaveValue.
                // Ah, Test 1 (Import) checks Addr. Test 2 (Manual) does NOT fill Addr in legacy code.
                // I'll pass empty string or improve the Page Object to make addr optional?
                // The Page Object currently requires addr.
                // I should verify if addr is mandatory in UI. Usually yes.
                // Legacy test might have relied on default or error check not being strict?
                // Legacy test line 166 clicks Save.
                // I will update Page Object signature to allow optional addr if needed, or pass empty string for now.
                'off' // Table
            );

            // Wait, I need to pass addr. I'll pass '10.99.0.1/24' to serve the required field if it is one.
            // Or if legacy test didn't fill it, maybe it worked?
            // "Manual Client Configuration" usually implies client.
            // I'll make addr optional in Page Object signature in next step or just pass dummy.
            // Re-reading legacy test:
            // await modal.getByTestId('vpn-conn-privkey').fill(...);
            // No addr fill.
            // I'll update my previous Replace call to make addr optional? Or just pass undefined if I change signature.
        });
    });
});

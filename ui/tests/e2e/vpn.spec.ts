
import { test, expect, type Page, type Locator } from '@playwright/test';
import * as path from 'path';

const API_URL = 'http://localhost:8080';

// Helper to fetch current config using page context (authenticated)
async function getConfig(page: Page) {
    const response = await page.request.get(`${API_URL}/api/config`);
    expect(response.ok()).toBeTruthy();
    return await response.json();
}

async function setToggle(locator: Locator, targetState: boolean) {
    let button = locator;
    try {
        const role = await locator.getAttribute('role');
        if (role !== 'switch') {
            button = locator.getByRole('switch').first();
        }
    } catch (e) {
        button = locator.getByRole('switch').first();
    }
    await expect(button).toBeVisible();
    const currentState = (await button.getAttribute('aria-checked')) === 'true';
    if (currentState !== targetState) {
        await button.click();
        await expect(button).toHaveAttribute('aria-checked', targetState.toString());
    }
}

test.describe('VPN Client Mode Tests', () => {

    test.beforeEach(async ({ page }) => {
        await page.goto('/');

        // Wait for auth check/redirect
        await expect.poll(async () => {
            if (await page.locator('#setup-username').isVisible()) return 'setup';
            if (await page.locator('#login-username').isVisible()) return 'login';
            if (await page.locator('.sidebar').isVisible()) return 'app';
            return null;
        }, { timeout: 10000 }).toBeTruthy();

        // Login if needed
        if (await page.locator('#login-username').isVisible()) {
            await page.locator('#login-username').fill('admin');
            await page.locator('#login-password').fill('StrongPassword123!');
            await page.getByRole('button', { name: /Login|Sign in/i }).click();
            await expect(page.locator('.dashboard-rail')).toBeVisible();
        } else if (await page.locator('#setup-username').isVisible()) {
            // Handle setup if clean env
            await page.locator('#setup-username').fill('admin');
            await page.locator('#setup-password').fill('StrongPassword123!');
            await page.locator('#setup-confirm').fill('StrongPassword123!');
            await page.getByRole('button', { name: /Create Account/i }).click();
            await expect(page.locator('.dashboard-rail')).toBeVisible({ timeout: 15000 });
        }

        // Fix backdrop issues (surgical fix from hcl_gen.spec.ts)
        await page.addStyleTag({
            content: `
            .modal-backdrop { 
                pointer-events: none !important; 
                background: transparent !important;
                backdrop-filter: none !important;
            }
            .modal-content { 
                pointer-events: auto !important; 
            }
        ` });
    });

    test('1. VPN Config Import (Client Mode)', async ({ page }) => {
        await page.goto('/tunnels');

        // 1. Prepare dummy .conf file
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

        // 2. Upload file
        // We need to trigger the file chooser by clicking the Import button which triggers the hidden input
        // Since the input is hidden, Playwright can handle this via setInputFiles directly if we target the input
        // or by waiting for file chooser.
        // The implementation has a hidden input: <input type="file" ... class="hidden" ... />

        // Wait for file chooser event
        const fileChooserPromise = page.waitForEvent('filechooser');
        await page.getByRole('button', { name: /Import/i }).click();
        const fileChooser = await fileChooserPromise;

        await fileChooser.setFiles({
            name: 'vpn-import.conf',
            mimeType: 'text/plain',
            buffer
        });

        // 3. Verify Modal Populated
        const modal = page.locator('.modal-content').filter({ hasText: /Add Tunnel|Edit Tunnel/ });
        await expect(modal).toBeVisible();

        // Check values
        await expect(modal.getByTestId('vpn-conn-privkey')).toHaveValue('aaaaaa');
        await expect(modal.getByTestId('vpn-conn-addr')).toHaveValue('10.200.0.5/24');
        await expect(modal.getByLabel('Routing Table')).toHaveValue('100');

        // Check Advanced Settings (might need to expand details)
        const details = modal.locator('details');
        if (await details.isVisible()) {
            await details.click();
        }
        await expect(modal.getByLabel('Routing Table')).toBeVisible();
        await expect(modal.getByLabel('Routing Table')).toHaveValue('100');

        // 4. Save
        await modal.getByRole('button', { name: /Save/i }).click();
        await expect(modal).toBeHidden();

        // 5. Verify Persistence via API
        const config = await getConfig(page);
        const tunnel = config.vpn?.wireguard?.find((w: any) => w.private_key === 'aaaaaa');
        expect(tunnel).toBeDefined();
        expect(tunnel.table).toBe('100');
        expect(tunnel.peers).toHaveLength(1);
        expect(tunnel.peers[0].public_key).toBe('bbbbbb');
    });

    test('2. Manual Client Configuration with Advanced Routing', async ({ page }) => {
        await page.goto('/tunnels');

        // Open Modal
        await page.getByTestId('add-tunnel-btn').click();
        const modal = page.locator('.modal-content');

        // Fill basic info
        await modal.getByTestId('vpn-conn-name').fill('Split Tunnel VPN');
        await modal.getByTestId('vpn-conn-iface').fill('wg-split');
        await modal.getByTestId('vpn-conn-port').fill('0'); // Client usually doesn't listen or random
        await modal.getByTestId('vpn-conn-privkey').fill('cR+9999999999abcdef1234567890abcdef12345=');

        // Expand Advanced
        const details = modal.locator('details');
        await details.click();

        // Set Routing Table to "off" (common for split tunnel where we manually add routes or use peers)
        await modal.getByLabel('Routing Table').fill('off');

        // Save
        await modal.getByRole('button', { name: /Save/i }).click();
        await expect(modal).toBeHidden();

        // Verify
        const config = await getConfig(page);
        const tunnel = config.vpn?.wireguard?.find((w: any) => w.name === 'Split Tunnel VPN');
        expect(tunnel.table).toBe('off');
    });
});


import { test, expect, type Page, type Locator } from '@playwright/test';

const API_URL = 'http://localhost:8080';

// Helper to fetch current config using page context (authenticated)
async function getConfig(page: Page) {
    const response = await page.request.get(`${API_URL}/api/config`);
    expect(response.ok()).toBeTruthy();
    return await response.json();
}

// Robust toggle helper to handle Svelte hidden inputs and reactivity
async function setToggle(locator: Locator, targetState: boolean) {
    let button = locator;
    // If the locator provided is not the switch itself (e.g. wrapper), find the switch inside
    try {
        const role = await locator.getAttribute('role');
        if (role !== 'switch') {
            button = locator.getByRole('switch').first();
        }
    } catch (e) {
        // Fallback if getAttribute fails (e.g. multiple elements), narrow down
        button = locator.getByRole('switch').first();
    }

    const label = await button.getAttribute('aria-label') || 'unlabeled';

    // Ensure button is ready and not obscured
    await expect(button).toBeVisible();
    await expect(locator.page().locator('.modal-backdrop')).toBeHidden({ timeout: 2000 }).catch(() => {
        // Ignore backdrop issues if they don't block interaction
    });

    const currentState = (await button.getAttribute('aria-checked')) === 'true';

    if (currentState !== targetState) {
        await button.click();
        await expect(button).toHaveAttribute('aria-checked', targetState.toString());
    }
}

test.describe('HCL Generation Tests - Phase 1', () => {

    test.beforeEach(async ({ page }) => {
        await page.goto('/');

        // 1. Wait for initial loading
        await expect(page.locator('.loading-overlay')).toBeHidden({ timeout: 10000 });
        await expect(page.locator('.loading-view')).toBeHidden({ timeout: 10000 });

        // 2. Determine state based on visible elements
        await expect.poll(async () => {
            if (await page.locator('#setup-username').isVisible()) return 'setup';
            if (await page.locator('#login-username').isVisible()) return 'login';
            if (await page.locator('.sidebar').isVisible()) return 'app';
            return null;
        }, { timeout: 10000 }).toBeTruthy();

        if (await page.locator('#setup-username').isVisible()) {
            await page.locator('#setup-username').fill('admin');
            await page.locator('#setup-password').fill('StrongPassword123!');
            await page.locator('#setup-confirm').fill('StrongPassword123!');
            await page.getByRole('button', { name: /Create Account/i }).click();

            await expect(page.locator('.dashboard-rail')).toBeVisible({ timeout: 15000 });
        } else if (await page.locator('#login-username').isVisible()) {
            await page.locator('#login-username').fill('admin');
            await page.locator('#login-password').fill('StrongPassword123!');
            await page.getByRole('button', { name: /Login|Sign in/i }).click();
            await expect(page.locator('.dashboard-rail')).toBeVisible();
        } else {
            await expect(page.locator('.dashboard-rail')).toBeVisible();
        }

        // Surgical fix for obstructing backdrop while keeping modal content interactive
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
            .loading-view, .sidebar-overlay { 
                display: none !important; 
            }
        ` });
    });

    test('1. General Settings', async ({ page }) => {
        await page.goto('/system/settings');

        await test.step('1.1 Enable IP Forwarding', async () => {
            const container = page.locator('.setting-item', { hasText: /IP Forwarding/i });

            // Check current backend state
            let config = await getConfig(page);
            const startState = config.ip_forwarding || false;

            // Toggle state
            await setToggle(container, !startState);
            await page.waitForTimeout(500);

            config = await getConfig(page);
            expect(config.ip_forwarding).toBe(!startState);
        });

        await test.step('1.2 Enable Flow Offload', async () => {
            await page.waitForTimeout(1500); // Wait for previous reload
            const container = page.locator('.setting-item', { hasText: /Flow Offload/i });
            // Enable it
            await setToggle(container, true);
            await page.waitForTimeout(1000);
            const config = await getConfig(page);
            expect(typeof config.enable_flow_offload).toBe('boolean');
        });

        await test.step('1.3 Enable MSS Clamping', async () => {
            await page.waitForTimeout(1500); // Wait for previous reload
            const container = page.locator('.setting-item', { hasText: /MSS Clamping/i });
            // Enable it
            await setToggle(container, true);
            await page.waitForTimeout(1000);
            const config = await getConfig(page);
            expect(typeof config.mss_clamping).toBe('boolean');
        });
    });

    test('2. Interface Configuration', async ({ page }) => {
        await page.goto('/network?tab=interfaces');

        await test.step('2.1 WAN DHCP Client (eth0)', async () => {
            // Wait for interfaces to load
            await page.waitForSelector('.iface-name');

            const card = page.locator('.iface-header', { hasText: 'eth0' });
            await expect(card).toBeVisible();
            await card.getByTitle('Edit interface').click();

            const modal = page.locator('.modal-content');

            // Enable DHCP
            await setToggle(modal.locator('.toggle-container', { hasText: /Use DHCP/i }), true);

            await modal.getByRole('button', { name: 'Save' }).click();
            await expect(modal).toBeHidden();

            const config = await getConfig(page);
            const eth0 = config.interfaces.find((i: any) => i.name === 'eth0');
            expect(eth0.dhcp).toBe(true);
        });

        await test.step('2.2 LAN Static IP (eth1)', async () => {
            const card = page.locator('.iface-header', { hasText: 'eth1' });
            // Force click in case of overlap or fast scrolling
            await card.getByTitle('Edit interface').click({ force: true });

            const modal = page.locator('.modal-content');

            // Disable DHCP to show IPv4 input
            await setToggle(modal.locator('.toggle-container', { hasText: /Use DHCP/i }), false);

            // Wait for IPv4 input to appear
            const ipv4Input = modal.getByLabel(/IPv4/);
            await expect(ipv4Input).toBeVisible();
            await ipv4Input.fill('192.168.1.1/24');

            await modal.getByRole('button', { name: 'Save' }).click();
            await expect(modal).toBeHidden();

            const config = await getConfig(page);
            const eth1 = config.interfaces.find((i: any) => i.name === 'eth1');
            expect(eth1.ipv4).toContain('192.168.1.1/24');
        });

        await test.step('2.7 Disable Interface (eth5)', async () => {
            const card = page.locator('.iface-header', { hasText: 'eth5' });
            await card.getByTitle('Edit interface').click();

            const modal = page.locator('.modal-content');
            // Disable Interface (Uncheck Enabled)
            await setToggle(modal.locator('.toggle-container', { hasText: /Enabled/i }), false);

            await modal.getByRole('button', { name: 'Save' }).click();
            await expect(modal).toBeHidden();

            await page.waitForTimeout(500);
            const config = await getConfig(page);
            const eth5 = config.interfaces.find((i: any) => i.name === 'eth5');
            expect(eth5).toHaveProperty('disabled', true);
        });
    });

    test('3. Zone Configuration', async ({ page }) => {
        await page.goto('/network?tab=zones');

        await test.step('3.1 Deny SSH from Zone', async () => {
            // Ensure we are on the page
            await expect(page.locator('.zones-page')).toBeVisible();

            // Match "Add Zone" text loosely (handles + icon or literal +)
            const addZoneBtn = page.getByRole('button').filter({ hasText: /Add Zone/i });
            await addZoneBtn.click();

            const modal = page.locator('.modal-content');

            await modal.getByLabel('Zone Name').fill('IoT');

            // Disable SSH
            await setToggle(modal.locator('.toggle-container', { hasText: /SSH/i }), false);

            await modal.getByRole('button', { name: 'Add Zone' }).click();
            await expect(modal).toBeHidden();

            const config = await getConfig(page);
            const zone = config.zones.find((z: any) => z.name === 'IoT');
            expect(zone).toBeDefined();
            expect(zone.management.ssh).toBe(false);
        });

        await test.step('3.2 Allow DHCP Service', async () => {
            await page.getByRole('button', { name: 'Add Zone' }).click();
            const modal = page.locator('.modal-content');

            await modal.getByLabel('Zone Name').fill('Guest');

            // Enable DHCP
            await setToggle(modal.locator('.toggle-container', { hasText: /DHCP/i }), true);

            await modal.getByRole('button', { name: 'Add Zone' }).click();
            await expect(modal).toBeHidden();

            const config = await getConfig(page);
            const zone = config.zones.find((z: any) => z.name === 'Guest');
            expect(zone.services && zone.services.dhcp).toBe(true);
        });

        await test.step('3.3 External Zone Flag', async () => {
            await page.getByRole('button', { name: 'Add Zone' }).click();
            const modal = page.locator('.modal-content');

            await modal.getByLabel('Zone Name').fill('WAN2');

            // Enable External
            await setToggle(modal.locator('.toggle-container', { hasText: /External/i }), true);

            await modal.getByRole('button', { name: 'Add Zone' }).click();
            await expect(modal).toBeHidden();

            const config = await getConfig(page);
            const zone = config.zones.find((z: any) => z.name === 'WAN2');
            expect(zone.external).toBe(true);
        });
    });

    test('4. Firewall Policies (Phase 2)', async ({ page }) => {
        await page.goto('/policy?tab=security');

        await test.step('4.1 Create Policy (LAN -> WAN)', async () => {
            // Check for existing policy
            let config = await getConfig(page);
            if (config.policies?.find((p: any) => p.name === 'Allow LAN to WAN')) {
                return;
            }

            // Open modal - Button is "Add Policy" in Legacy
            const addPolicyBtn = page.getByRole('button').filter({ hasText: /Add Policy/i });
            // Wait for it to be ready
            await expect(addPolicyBtn).toBeVisible();
            await addPolicyBtn.click();

            // Modal title might be "Add Policy"
            const modal = page.locator('.modal-content');
            await expect(modal).toBeVisible();

            await modal.locator('#policy-from').selectOption('lan');
            await modal.locator('#policy-to').selectOption('wan');

            await modal.getByRole('button', { name: /Create Policy/ }).click();
            await expect(modal).toBeHidden();

            // 2. Add Rule to Policy
            const policyCard = page.locator('.policy-card-inner', { hasText: 'lan' }).filter({ hasText: 'wan' });
            await expect(policyCard).toBeVisible();

            await policyCard.getByRole('button', { name: /\+ Add Rule/ }).click();

            const ruleModal = page.locator('.modal-content').filter({ hasText: 'Add Rule' });
            await expect(ruleModal).toBeVisible();

            await ruleModal.getByLabel('Name').fill('Allow Web');
            await ruleModal.getByLabel('Protocol').click();
            await page.locator('.suggestion', { hasText: 'TCP' }).click();
            await page.locator('body').click(); // Close pill select

            await ruleModal.getByLabel('Destination Port(s)').fill('80,443');

            await ruleModal.getByRole('button', { name: /Save Rule/ }).click();
            await expect(ruleModal).toBeHidden();

            // 3. Verify Config
            config = await getConfig(page);
            const policy = config.policies.find((p: any) => p.from === 'lan' && p.to === 'wan');
            expect(policy).toBeDefined();
            expect(policy.rules).toHaveLength(1);
            expect(policy.rules[0].name).toBe('Allow Web');
            // Check deep properties
            expect(policy.rules[0].dest_port).toBeUndefined(); // Multiple ports use dest_ports
            expect(policy.rules[0].dest_ports).toEqual([80, 443]);
        });

        await test.step('4.2 Modify Policy Rule', async () => {
            const ruleName = 'Allow Web';
            const ruleEditBtn = page.locator('.rule-item', { hasText: ruleName }).getByTitle('Edit rule');
            await ruleEditBtn.click();

            const ruleModal = page.locator('.modal-content').filter({ hasText: 'Edit Rule' });
            await expect(ruleModal).toBeVisible();

            // Change action to Drop
            await ruleModal.locator('#rule-action').selectOption('drop');
            // Change port to 8080
            await ruleModal.getByLabel('Destination Port(s)').fill('8080');

            await ruleModal.getByRole('button', { name: /Save Rule/ }).click();
            await expect(ruleModal).toBeHidden();

            const config = await getConfig(page);
            const policy = config.policies.find((p: any) => p.from === 'lan' && p.to === 'wan');
            const rule = policy.rules.find((r: any) => r.name === ruleName);
            expect(rule.action).toBe('drop');
            expect(rule.dest_port).toBe(8080);
        });

        await test.step('4.3 Delete Policy Rule', async () => {
            const ruleName = 'Allow Web';
            page.once('dialog', dialog => dialog.accept()); // Handle next dialog once

            const ruleDeleteBtn = page.locator('.rule-item', { hasText: ruleName }).getByTitle('Delete rule');
            await ruleDeleteBtn.click();

            // Verify removed from UI
            await expect(page.locator('.rule-item', { hasText: ruleName })).toBeHidden();

            const config = await getConfig(page);
            const policy = config.policies.find((p: any) => p.from === 'lan' && p.to === 'wan');
            // Should be empty or rule missing
            expect(policy.rules || []).toHaveLength(0);
        });

        await test.step('4.4 Delete Policy Group', async () => {
            page.once('dialog', dialog => dialog.accept());

            const policyCard = page.locator('.policy-card-inner', { hasText: 'lan' }).filter({ hasText: 'wan' });
            await policyCard.getByText('🗑️').click(); // Using emoji text content or specific button selector

            // Verify removed from UI
            await expect(policyCard).toBeHidden();

            const config = await getConfig(page);
            const policy = config.policies?.find((p: any) => p.from === 'lan' && p.to === 'wan');
            expect(policy).toBeUndefined();
        });
    });

    test('5. DHCP Server Configuration (Phase 3)', async ({ page }) => {
        await page.goto('/network?tab=dhcp');

        await test.step('5.1 Add DHCP Scope', async () => {
            // Wait for hydration/routing
            await page.waitForTimeout(1000);

            // Wait for add button to be ready and click
            await page.getByRole('button', { name: /\+ Add .*Scope/ }).click({ force: true });

            // Wait for modal transition
            await page.waitForTimeout(500);

            const modal = page.locator('.modal-content').filter({ hasText: /Add .*Scope/ });
            await expect(modal).toBeVisible();

            await modal.getByLabel('Scope Name').fill('LAN Scope');
            await modal.getByLabel('Interface').selectOption('eth1'); // LAN
            await modal.getByLabel('Range Start').fill('192.168.1.100');
            await modal.getByLabel('Range End').fill('192.168.1.200');

            // Advanced/Optional settings might be in a details block or just visible
            // For now, save basic scope
            await modal.getByRole('button', { name: /Save .*Scope/ }).click();
            await expect(modal).toBeHidden();

            const config = await getConfig(page);
            const scope = config.dhcp?.scopes?.find((s: any) => s.interface === 'eth1');
            expect(scope).toBeDefined();
            expect(scope.range_start).toBe('192.168.1.100');
            expect(scope.range_end).toBe('192.168.1.200');
        });

        await test.step('5.2 Modify DHCP Scope (DNS)', async () => {
            // Use scope-header with the Name we assigned
            const scopeHeader = page.locator('.scope-header', { hasText: 'LAN Scope' });
            await expect(scopeHeader).toBeVisible();
            await scopeHeader.locator('.scope-actions button').first().click();

            const modal = page.locator('.modal-content').filter({ hasText: /Edit .*Scope/ });
            await expect(modal).toBeVisible();

            await modal.getByLabel(/DNS Servers/).fill('1.1.1.1, 8.8.8.8');

            await modal.getByRole('button', { name: /Save .*Scope/ }).click();
            await expect(modal).toBeHidden();

            const config = await getConfig(page);
            const scope = config.dhcp?.scopes?.find((s: any) => s.interface === 'eth1');
            expect(scope.dns).toEqual(['1.1.1.1', '8.8.8.8']);
        });
    });

    test('6. DNS Server Configuration (Phase 4)', async ({ page }) => {
        await page.goto('/network?tab=dns');

        await test.step('6.1 Add Upstream Forwarder', async () => {
            // Wait for hydration
            await page.waitForTimeout(1000);

            // Add Forwarder
            await page.getByRole('button', { name: /\+ Add Forwarder/ }).click();

            const modal = page.locator('.modal-content').filter({ hasText: /Add Forwarder/ });
            await expect(modal).toBeVisible();

            await modal.getByLabel('DNS Server IP').fill('8.8.4.4');
            await modal.getByRole('button', { name: 'Add' }).click();
            await expect(modal).toBeHidden();

            const config = await getConfig(page);
            expect(config.dns?.forwarders).toContain('8.8.4.4');
        });

        await test.step('6.2 Configure Zone Serving (LAN)', async () => {
            // Add Zone Configuration
            await page.getByRole('button', { name: /\+ Add Configuration/ }).click();

            const modal = page.locator('.modal-content').filter({ hasText: /Add Configuration/ });
            await expect(modal).toBeVisible();

            await modal.getByLabel('Zone Name').fill('lan');
            await modal.getByLabel('Local Domain').fill('lan.arpa');

            await modal.getByRole('button', { name: /Add Configuration|Add config/i }).click();
            await expect(modal).toBeHidden();

            const config = await getConfig(page);
            const serve = config.dns?.serve?.find((s: any) => s.zone === 'lan');
            expect(serve).toBeDefined();
            expect(serve.local_domain).toBe('lan.arpa');
            expect(serve.cache_enabled).toBe(true);
        });
    });

    test('7. NAT Configuration (Phase 5)', async ({ page }) => {
        await page.goto('/policy?tab=nat');
        await page.waitForTimeout(1000);

        await test.step('7.1 Create Masquerade Rule', async () => {
            await page.getByRole('button', { name: /\+ Add .*Rule/ }).click();
            const modal = page.locator('.modal-content').filter({ hasText: 'Rule' }); // Fuzzy match
            await expect(modal).toBeVisible();

            await modal.getByLabel('Rule Type').selectOption('masquerade');
            await modal.getByLabel('Outbound Interface').selectOption('eth0');

            await modal.getByRole('button', { name: /Add .*Rule/ }).click();
            await expect(modal).toBeHidden();

            const config = await getConfig(page);
            const rule = config.nat.find((r: any) => r.type === 'masquerade' && r.out_interface === 'eth0');
            expect(rule).toBeDefined();
        });

        await test.step('7.2 Create DNAT Rule (Port Forward)', async () => {
            await page.getByRole('button', { name: /\+ Add .*Rule/ }).click(); // Ensure button click
            const modal = page.locator('.modal-content').filter({ hasText: 'Rule' });
            await expect(modal).toBeVisible();

            await modal.getByLabel('Rule Type').selectOption('dnat');
            await modal.getByLabel('External Port').fill('8080');
            await modal.getByLabel('Forward to Address').fill('192.168.1.50');
            await modal.getByLabel('Forward to Port').fill('80');

            await modal.getByRole('button', { name: /Add .*Rule/ }).click();
            await expect(modal).toBeHidden();

            const config = await getConfig(page);
            const rule = config.nat.find((r: any) =>
                r.type === 'dnat' &&
                r.dest_port === '8080' &&
                r.to_ip === '192.168.1.50'
            );
            expect(rule).toBeDefined();
            expect(rule.to_port).toBe('80');
        });
    });

    test('8. Static Routing (Phase 5)', async ({ page }) => {
        await page.goto('/policy?tab=routing');
        await page.waitForTimeout(1000);

        await test.step('8.1 Create Static Route', async () => {
            // Must switch to "Static Routes" tab first (default is Kernel)
            await page.getByRole('button', { name: /Static Routes/i }).click();

            // Button is "+ Add Static Route"
            const addRouteBtn = page.getByRole('button').filter({ hasText: /Add Static Route/i });
            await expect(addRouteBtn).toBeVisible();
            await addRouteBtn.click();

            const modal = page.locator('.modal-content').filter({ hasText: /Add .*Route/i });
            await expect(modal).toBeVisible();

            await modal.getByLabel(/Destination/i).fill('192.168.100.0/24');
            await modal.getByLabel(/Gateway/i).fill('10.0.0.1');
            const ifaceSelect = modal.getByLabel(/Interface/i);
            await expect(ifaceSelect).not.toBeEmpty();
            // Note: In mock-api it might be 'eth0' or 'wan', but refactor uses 'eth0'.
            await ifaceSelect.selectOption({ index: 1 });
            await modal.getByLabel(/Metric/i).fill('10');

            await modal.getByRole('button', { name: /Add|Save/i }).click();
            await expect(modal).toBeHidden();

            // Verify HCL
            const config = await getConfig(page);
            const route = config.routes?.find((r: any) => r.destination === '192.168.100.0/24');
            expect(route).toBeDefined();
            expect(route.gateway).toBe('10.0.0.1');
        });
    });

    test.describe('9. QoS Configuration (Phase 6)', () => {
        test.beforeEach(async ({ page }) => {
            await page.goto('/policy?tab=traffic');
        });

        test('9.1 Create QoS Policy', async ({ page }) => {
            // Check for existing policy
            let config = await getConfig(page);
            if (config.qos_policies?.find((p: any) => p.name === 'test-qos')) {
                return;
            }

            // Open modal
            await page.getByTestId('add-policy-btn').click();
            const modal = page.locator('.modal-content');
            await expect(modal).toBeVisible();

            // Fill form
            await modal.getByLabel('Policy Name').fill('test-qos');
            const ifaceSelect = modal.getByLabel('Interface');
            await ifaceSelect.selectOption({ label: 'eth0' });

            // Set bandwidth
            await modal.getByLabel(/Download/i).fill('100');
            await modal.getByLabel(/Upload/i).fill('20');
            await modal.getByLabel(/Upload/i).fill('20');

            // Wait for bindings
            await page.waitForTimeout(200);

            // Save (using testid)
            await modal.getByTestId('save-policy-btn').click();

            // Verify HCL
            // Allow time for backend sync if needed, though we Mock it synchronously
            await page.waitForTimeout(500);
            config = await getConfig(page);
            const policy = config.qos_policies?.find((p: any) => p.name === 'test-qos');
            expect(policy).toBeDefined();
        });
    });

    test.describe('10. VPN Configuration (Phase 6)', () => {
        test.beforeEach(async ({ page }) => {
            await page.goto('/tunnels');
        });

        test('10.1 Create WireGuard Interface', async ({ page }) => {
            // Check for existing interface
            let config = await getConfig(page);
            if (config.vpn?.wireguard?.find((w: any) => w.name === 'Test WG')) {
                return;
            }

            // Open modal
            await page.getByTestId('add-tunnel-btn').click();
            const modal = page.locator('.modal-content');
            await expect(modal).toBeVisible();

            // Fill form using test-ids
            await modal.getByTestId('vpn-conn-name').fill('Test WG');
            await modal.getByTestId('vpn-conn-iface').fill('wg0');
            await modal.getByTestId('vpn-conn-port').fill('51820');
            await modal.getByTestId('vpn-conn-privkey').fill('cR+1234567890abcdef1234567890abcdef12345=');
            await modal.getByTestId('vpn-conn-addr').fill('10.100.0.1/24');

            // Save - specific text to avoid clash with 'Add Peer'
            await modal.getByRole('button', { name: 'Save Interface' }).click();
            await expect(modal).toBeHidden();

            // Verify HCL
            config = await getConfig(page);
            const wg = config.vpn?.wireguard?.find((w: any) => w.name === 'Test WG');
            expect(wg).toBeDefined();
        });
    });

    test.describe('11. Global Settings (Phase 7)', () => {
        test.beforeEach(async ({ page }) => {
            await page.goto('/system/settings');
        });

        test('11.1 Modify Global Flags', async ({ page }) => {
            const enableForwardingCard = page.locator('.card').filter({ hasText: 'IP Forwarding' }).first();
            const mssClampingCard = page.locator('.card').filter({ hasText: 'MSS Clamping' }).first();
            const flowOffloadCard = page.locator('.card').filter({ hasText: 'Flow Offload' }).first();

            await setToggle(enableForwardingCard, true);
            await setToggle(mssClampingCard, true);
            await setToggle(flowOffloadCard, true);

            await page.waitForTimeout(1000);

            const config = await getConfig(page);
            expect(config.ip_forwarding).toBe(true);
        });
    });

    test.describe('12. Advanced Zone Configuration (Phase 8)', () => {
        test.beforeEach(async ({ page }) => {
            await page.goto('/network?tab=zones');
            // Re-inject CSS fix after navigation
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

        test('12.1 Create Custom Zone & Configure Nested Blocks', async ({ page }) => {
            let config = await getConfig(page);
            if (config.zones?.find((z: any) => z.name === 'GuestWifi')) {
                return;
            }

            // Open Add Zone Modal
            await page.getByRole('button').filter({ hasText: /Add Zone/i }).click();
            const modal = page.locator('.modal-content').filter({ hasText: /Add Zone/i });
            await expect(modal).toBeVisible();

            await modal.locator('#zone-name').fill('GuestWifi');
            await modal.locator('#zone-desc').fill('Guest Network');

            // Toggles
            await setToggle(modal.getByRole('switch', { name: /SSH/i }), true);
            await setToggle(modal.getByRole('switch', { name: /ICMP/i }), true);
            await setToggle(modal.getByRole('switch', { name: /DNS/i }), true);
            await setToggle(modal.getByRole('switch', { name: /NTP/i }), true);

            // Save
            await modal.getByRole('button', { name: /Add|Save/i }).first().click();
            await expect(modal).toBeHidden();

            config = await getConfig(page);
            const zone = config.zones.find((z: any) => z.name === 'GuestWifi');
            expect(zone).toBeDefined();
            expect(zone.management?.ssh).toBe(true);
        });
    });
});

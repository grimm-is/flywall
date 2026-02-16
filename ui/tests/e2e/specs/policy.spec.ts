import { test, expect } from '../fixtures/test';

test.describe('Policy Configuration', () => {

    test.beforeEach(async ({ loginPage }) => {
        await loginPage.goto();
        await loginPage.login();
    });

    test('Firewall Policies', async ({ policyPage }) => {
        await policyPage.goto('security');

        await test.step('Create Policy (LAN -> WAN)', async () => {
            // Check existence? The API mock handles state, but let's assume clean or check
            // Logic port:
            // 1. Add Policy
            await policyPage.security.addPolicy('lan', 'wan');

            // 2. Add Rule
            await policyPage.security.addRule('lan', 'wan', 'Allow Web', '80,443', 'TCP');

            const config = await policyPage.getConfig();
            const policy = config.policies.find((p: any) => p.from === 'lan' && p.to === 'wan');
            expect(policy.rules[0].name).toBe('Allow Web');
            expect(policy.rules[0].dest_ports).toEqual([80, 443]);
        });

        await test.step('Modify Policy Rule', async () => {
            await policyPage.security.editRule('Allow Web');
            await policyPage.security.modal.selectOption('Action', 'drop');
            await policyPage.security.modal.fill('Destination Port(s)', '8080');
            await policyPage.security.modal.clickButton(/Save Rule/);
            await policyPage.security.modal.expectHidden();

            const config = await policyPage.getConfig();
            const policy = config.policies.find((p: any) => p.from === 'lan' && p.to === 'wan');
            const rule = policy.rules.find((r: any) => r.name === 'Allow Web');
            expect(rule.action).toBe('drop');
            expect(rule.dest_port).toBe(8080);
        });

        await test.step('Delete Policy Rule', async () => {
            await policyPage.security.deleteRule('Allow Web');
            const config = await policyPage.getConfig();
            const policy = config.policies.find((p: any) => p.from === 'lan' && p.to === 'wan');
            expect(policy.rules || []).toHaveLength(0);
        });

        await test.step('Delete Policy Group', async () => {
            await policyPage.security.deletePolicy('lan', 'wan');
            const config = await policyPage.getConfig();
            const policy = config.policies?.find((p: any) => p.from === 'lan' && p.to === 'wan');
            expect(policy).toBeUndefined();
        });
    });

    test('NAT Configuration', async ({ policyPage }) => {
        await policyPage.goto('nat');

        await test.step('Create Masquerade Rule', async () => {
            await policyPage.nat.addMasquerade('eth0');

            const config = await policyPage.getConfig();
            const rule = config.nat.find((r: any) => r.type === 'masquerade' && r.out_interface === 'eth0');
            expect(rule).toBeDefined();
        });

        await test.step('Create DNAT Rule', async () => {
            await policyPage.nat.addDNAT('8080', '192.168.1.50', '80');

            const config = await policyPage.getConfig();
            const rule = config.nat.find((r: any) =>
                r.type === 'dnat' &&
                r.dest_port === '8080' &&
                r.to_ip === '192.168.1.50'
            );
            expect(rule).toBeDefined();
        });
    });

    test('Static Routing', async ({ policyPage }) => {
        await policyPage.goto('routing');

        await test.step('Create Static Route', async () => {
            await policyPage.routing.addStaticRoute('192.168.100.0/24', '10.0.0.1', '10');

            const config = await policyPage.getConfig();
            const route = config.routes?.find((r: any) => r.destination === '192.168.100.0/24');
            expect(route).toBeDefined();
            expect(route.gateway).toBe('10.0.0.1');
        });
    });

    test('QoS Configuration', async ({ policyPage }) => {
        await policyPage.goto('traffic');

        await test.step('Create QoS Policy', async () => {
            await policyPage.qos.addPolicy('test-qos', 'eth0', '100', '20');

            await policyPage.page.waitForTimeout(500); // Sync wait
            const config = await policyPage.getConfig();
            const policy = config.qos_policies?.find((p: any) => p.name === 'test-qos');
            expect(policy).toBeDefined();
        });
    });
});

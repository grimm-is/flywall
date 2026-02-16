import { test, expect } from '../fixtures/test';

test.describe('Network Configuration', () => {

    test.beforeEach(async ({ loginPage }) => {
        await loginPage.goto();
        await loginPage.login();
    });

    test('Interface Configuration', async ({ networkPage }) => {
        await networkPage.goto('interfaces');

        await test.step('Enable DHCP on eth0', async () => {
            await networkPage.interfaces.editInterface('eth0');
            await networkPage.interfaces.enableDHCP(true);
            await networkPage.interfaces.save();

            const config = await networkPage.getConfig();
            const eth0 = config.interfaces.find((i: any) => i.name === 'eth0');
            expect(eth0.dhcp).toBe(true);
        });

        await test.step('Set Static IP on eth1', async () => {
            await networkPage.interfaces.editInterface('eth1');
            await networkPage.interfaces.enableDHCP(false);
            await networkPage.interfaces.setIPv4('192.168.1.1/24');
            await networkPage.interfaces.save();

            const config = await networkPage.getConfig();
            const eth1 = config.interfaces.find((i: any) => i.name === 'eth1');
            expect(eth1.ipv4).toContain('192.168.1.1/24');
        });

        await test.step('Disable Interface eth5', async () => {
            await networkPage.interfaces.editInterface('eth5');
            await networkPage.interfaces.setEnabled(false);
            await networkPage.interfaces.save();

            const config = await networkPage.getConfig();
            const eth5 = config.interfaces.find((i: any) => i.name === 'eth5');
            expect(eth5.disabled).toBe(true);
        });
    });

    test('Zone Configuration', async ({ networkPage }) => {
        await networkPage.goto('zones');

        await test.step('Create Zone with SSH Disabled', async () => {
            await networkPage.zones.addZone();
            await networkPage.zones.fillName('IoT');
            await networkPage.zones.setService('SSH', false);
            await networkPage.zones.save();

            const config = await networkPage.getConfig();
            const zone = config.zones.find((z: any) => z.name === 'IoT');
            expect(zone.management.ssh).toBe(false);
        });

        await test.step('Create Guest Zone with DHCP', async () => {
            await networkPage.zones.addZone();
            await networkPage.zones.fillName('Guest');
            await networkPage.zones.setService('DHCP', true);
            await networkPage.zones.save();

            const config = await networkPage.getConfig();
            const zone = config.zones.find((z: any) => z.name === 'Guest');
            expect(zone.services.dhcp).toBe(true);
        });

        await test.step('Create External Zone', async () => {
            await networkPage.zones.addZone();
            await networkPage.zones.fillName('WAN2');
            await networkPage.zones.setExternal(true);
            await networkPage.zones.save();

            const config = await networkPage.getConfig();
            const zone = config.zones.find((z: any) => z.name === 'WAN2');
            expect(zone.external).toBe(true);
        });

        await test.step('Create Custom Zone with multiple services', async () => {
            await networkPage.zones.addZone();
            await networkPage.zones.fillName('GuestWifi');
            await networkPage.zones.fillDescription('Guest Network');
            await networkPage.zones.setService('SSH', true);
            await networkPage.zones.setService('ICMP', true);
            await networkPage.zones.setService('DNS', true);
            await networkPage.zones.setService('NTP', true);
            await networkPage.zones.save();

            const config = await networkPage.getConfig();
            const zone = config.zones.find((z: any) => z.name === 'GuestWifi');
            expect(zone.management.ssh).toBe(true);
            expect(zone.services.dns).toBe(true);
        });
    });

    test('DHCP Configuration', async ({ networkPage }) => {
        await networkPage.goto('dhcp');

        await test.step('Add DHCP Scope', async () => {
            await networkPage.dhcp.addScope();
            await networkPage.dhcp.fillScopeDetails('LAN Scope', 'eth1', '192.168.1.100', '192.168.1.200');
            await networkPage.dhcp.save();

            const config = await networkPage.getConfig();
            const scope = config.dhcp?.scopes?.find((s: any) => s.interface === 'eth1');
            expect(scope.range_start).toBe('192.168.1.100');
        });

        await test.step('Edit DHCP Scope DNS', async () => {
            await networkPage.dhcp.editScope('LAN Scope');
            await networkPage.dhcp.setDNS('1.1.1.1, 8.8.8.8');
            await networkPage.dhcp.save();

            const config = await networkPage.getConfig();
            const scope = config.dhcp?.scopes?.find((s: any) => s.interface === 'eth1');
            expect(scope.dns).toEqual(['1.1.1.1', '8.8.8.8']);
        });
    });

    test('DNS Configuration', async ({ networkPage }) => {
        await networkPage.goto('dns');

        await test.step('Add Forwarder', async () => {
            await networkPage.dns.addForwarder('8.8.4.4');

            const config = await networkPage.getConfig();
            expect(config.dns?.forwarders).toContain('8.8.4.4');
        });

        await test.step('Configure Zone Serving', async () => {
            await networkPage.dns.addZoneConfig('lan', 'lan.arpa');

            const config = await networkPage.getConfig();
            const serve = config.dns?.serve?.find((s: any) => s.zone === 'lan');
            expect(serve.local_domain).toBe('lan.arpa');
            expect(serve.cache_enabled).toBe(true);
        });
    });
});

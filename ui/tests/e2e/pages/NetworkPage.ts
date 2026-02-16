import { type Page } from '@playwright/test';
import { BasePage } from './BasePage';
import { InterfacesTab } from './network/InterfacesTab';
import { ZonesTab } from './network/ZonesTab';
import { DHCPTab } from './network/DHCPTab';
import { DNSTab } from './network/DNSTab';

export class NetworkPage extends BasePage {
    readonly interfaces: InterfacesTab;
    readonly zones: ZonesTab;
    readonly dhcp: DHCPTab;
    readonly dns: DNSTab;

    constructor(page: Page) {
        super(page);
        this.interfaces = new InterfacesTab(page);
        this.zones = new ZonesTab(page);
        this.dhcp = new DHCPTab(page);
        this.dns = new DNSTab(page);
    }

    async goto(tab?: 'interfaces' | 'zones' | 'dhcp' | 'dns') {
        let url = '/network';
        if (tab) {
            url += `?tab=${tab}`;
        }
        await super.goto(url);
    }
}

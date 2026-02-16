import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from '../BasePage';
import { InlineCard } from '../../components/InlineCard';

export class InterfacesTab extends BasePage {
    readonly card: InlineCard;

    constructor(page: Page) {
        super(page);
        this.card = new InlineCard(page);
    }

    async editInterface(name: string) {
        // InterfaceCard uses inline editing - click edit button by name
        // The card shows interface name in iface-header, edit button has title="Edit interface"
        const card = this.page.locator('.iface-header').filter({ hasText: name });
        await card.getByTitle('Edit interface').click({ force: true });
        // Wait for inline edit form to appear
        await this.page.waitForSelector('.edit-form, form', { timeout: 5000 });
    }

    async enableDHCP(enabled: boolean) {
        await this.toggle.set(
            this.page.locator('.toggle-container', { hasText: /Use DHCP/i }),
            enabled
        );
    }

    async setIPv4(cidr: string) {
        await this.page.getByLabel(/IPv4/i).fill(cidr);
    }

    async setEnabled(enabled: boolean) {
        await this.toggle.set(
            this.page.locator('.toggle-container', { hasText: /Enabled/i }),
            enabled
        );
    }

    async save() {
        await this.page.getByRole('button', { name: /Save/i }).click();
        await this.page.waitForTimeout(500);
    }

    async addVlan(parent: string, vlanId: string, zone = 'LAN', desc?: string) {
        // Find parent interface card
        const card = this.page.locator('.iface-header', { hasText: parent });
        await expect(card).toBeVisible();
        await card.getByTitle('Add VLAN').click();

        // Wait for inline form
        await this.page.waitForSelector('.vlan-form, form', { timeout: 5000 });

        await this.page.getByLabel(/VLAN ID/i).fill(vlanId);
        await this.page.getByLabel(/Zone/i).selectOption(zone);
        if (desc) {
            await this.page.getByLabel(/Description/i).fill(desc);
        }

        await this.page.getByRole('button', { name: /Add VLAN/i }).click();
        await this.page.waitForTimeout(500);
    }

    async addBond(name: string, members: string[], zone = 'LAN') {
        await this.page.getByRole('button', { name: /Add Bond/i }).click();

        await this.page.waitForSelector('.bond-form, form', { timeout: 5000 });

        await this.page.getByLabel(/Name/i).fill(name);
        await this.page.getByLabel(/Zone/i).selectOption(zone);

        // Members might be a multi-select or check list
        for (const member of members) {
            await this.page.getByLabel(/Members/i).click();
            await this.page.locator('.suggestion', { hasText: member }).click();
        }
        await this.page.locator('body').click(); // close pills

        await this.page.getByRole('button', { name: /Add Bond/i }).click();
        await this.page.waitForTimeout(500);
    }
}

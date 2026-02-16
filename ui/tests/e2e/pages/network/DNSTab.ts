import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from '../BasePage';
import { InlineCard } from '../../components/InlineCard';

export class DNSTab extends BasePage {
    readonly card: InlineCard;

    constructor(page: Page) {
        super(page);
        this.card = new InlineCard(page);
    }

    async addForwarder(ip: string) {
        await this.page.getByRole('button', { name: /\+ Add Forwarder/i }).click();
        // Wait for inline form to appear
        await this.page.waitForSelector('.create-card, .forwarder-form, input[type="text"]', { timeout: 5000 });

        // Fill in the forwarder IP
        await this.page.getByLabel(/DNS Server|Server IP|Forwarder/i).fill(ip);

        // Click add button
        await this.page.getByRole('button', { name: /Add|Save/i }).click();
        await this.page.waitForTimeout(500);
    }

    async addZoneConfig(zone: string, domain: string) {
        await this.page.getByRole('button', { name: /\+ Add Configuration/i }).click();
        // Wait for inline form
        await this.page.waitForSelector('.create-card, .serve-form, input[type="text"]', { timeout: 5000 });

        await this.page.getByLabel(/Zone Name/i).fill(zone);
        await this.page.getByLabel(/Local Domain/i).fill(domain);

        await this.page.getByRole('button', { name: /Add|Save|Create/i }).click();
        await this.page.waitForTimeout(500);
    }
}

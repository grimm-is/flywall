import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from '../BasePage';
import { InlineCard } from '../../components/InlineCard';

export class ZonesTab extends BasePage {
    readonly card: InlineCard;

    constructor(page: Page) {
        super(page);
        this.card = new InlineCard(page);
    }

    async addZone() {
        // Match "Add Zone" text loosely (handles + icon or literal +)
        const btn = this.page.getByRole('button').filter({ hasText: /Add Zone/i });
        await btn.click();
        // Wait for inline card to appear
        await this.page.waitForSelector('.create-card, .card form', { timeout: 5000 });
    }

    async fillName(name: string) {
        await this.page.getByLabel(/Zone Name/i).fill(name);
    }

    async fillDescription(desc: string) {
        await this.page.getByLabel(/Description/i).fill(desc);
    }

    async setService(service: 'SSH' | 'DHCP' | 'DNS' | 'NTP' | 'ICMP', enabled: boolean) {
        // Regex to match ignoring case
        const regex = new RegExp(service, 'i');
        await this.toggle.set(
            this.page.locator('.toggle-container', { hasText: regex }),
            enabled
        );
    }

    async setExternal(enabled: boolean) {
        await this.toggle.set(
            this.page.locator('.toggle-container', { hasText: /External/i }),
            enabled
        );
    }

    async save() {
        // Click save button in the inline form
        await this.page.getByRole('button', { name: /Add Zone|Save|Create/i }).click();
        // Wait for form to disappear
        await this.page.waitForTimeout(500);
    }
}

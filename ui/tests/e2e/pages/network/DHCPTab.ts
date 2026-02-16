import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from '../BasePage';
import { InlineCard } from '../../components/InlineCard';

export class DHCPTab extends BasePage {
    readonly card: InlineCard;

    constructor(page: Page) {
        super(page);
        this.card = new InlineCard(page);
    }

    async addScope() {
        await this.page.getByRole('button', { name: /\+ Add .*Scope/ }).click({ force: true });
        // Wait for inline card to appear
        await this.page.waitForSelector('.create-card, .card form, .scope-form', { timeout: 5000 });
    }

    async editScope(name: string) {
        const header = this.page.locator('.scope-header', { hasText: name });
        await expect(header).toBeVisible();
        await header.locator('.scope-actions button').first().click();
        // Wait for inline edit form
        await this.page.waitForSelector('.edit-card, .scope-edit', { timeout: 5000 });
    }

    async fillScopeDetails(name: string, iface: string, start: string, end: string) {
        await this.page.getByLabel(/Scope Name/i).fill(name);
        await this.page.getByLabel(/Interface/i).selectOption({ label: iface });
        await this.page.getByLabel(/Range Start/i).fill(start);
        await this.page.getByLabel(/Range End/i).fill(end);
    }

    async setDNS(servers: string) {
        await this.page.getByLabel(/DNS Servers/i).fill(servers);
    }

    async save() {
        await this.page.getByRole('button', { name: /Save|Create|Add/i }).click();
        await this.page.waitForTimeout(500);
    }
}

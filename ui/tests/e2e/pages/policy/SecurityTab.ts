import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from '../BasePage';
import { InlineCard } from '../../components/InlineCard';
import { Modal } from '../../components/Modal';

export class SecurityTab extends BasePage {
    readonly card: InlineCard;
    readonly modal: Modal;

    constructor(page: Page) {
        super(page);
        this.card = new InlineCard(page);
        this.modal = new Modal(page);
    }

    async addPolicy(from: string, to: string) {
        // match "New Policy Group" or "Add Policy"
        const btn = this.page.getByRole('button').filter({ hasText: /(New Policy Group)|(Add Policy)/i });
        await btn.click();

        const card = new InlineCard(this.page, '.create-card, .edit-card'); // More specific
        await card.expectVisible();

        await card.selectOption('From Zone', from);
        await card.selectOption('To Zone', to);
        await card.clickButton(/Create|Save/i);
        await card.expectHidden();
    }

    async addRule(policyFrom: string, policyTo: string, ruleName: string, port: string, proto = 'TCP') {
        const card = this.page.locator('.policy-card-inner', { hasText: policyFrom }).filter({ hasText: policyTo });
        await card.getByRole('button', { name: /\+ Add Rule/ }).click();

        // Wait for the inline rule edit form to appear
        const ruleCard = new InlineCard(this.page, '.rule-edit-card, .create-card');
        await ruleCard.expectVisible();

        await ruleCard.fill('Name', ruleName);

        // Protocol suggestion handling
        await ruleCard.content.getByLabel('Protocol').click();
        await this.page.locator('.suggestion', { hasText: proto }).click();
        await this.page.locator('body').click(); // close pill

        await ruleCard.fill('Destination Port(s)', port);
        await ruleCard.clickButton(/Save Rule/);
        await ruleCard.expectHidden();
    }

    async editRule(ruleName: string) {
        await this.page.locator('.rule-item', { hasText: ruleName }).getByTitle('Edit rule').click();
        // Edit uses Modal, not InlineCard
        await this.modal.expectVisible();
    }

    async deleteRule(ruleName: string) {
        this.page.once('dialog', dialog => dialog.accept());
        await this.page.locator('.rule-item', { hasText: ruleName }).getByTitle('Delete rule').click();
    }

    async deletePolicy(policyFrom: string, policyTo: string) {
        this.page.once('dialog', dialog => dialog.accept());
        const card = this.page.locator('.policy-card-inner', { hasText: policyFrom }).filter({ hasText: policyTo });
        await card.getByText('🗑️').click();
    }
}

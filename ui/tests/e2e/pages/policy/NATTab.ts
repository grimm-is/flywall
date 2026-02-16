import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from '../BasePage';
import { InlineCard } from '../../components/InlineCard';

export class NATTab extends BasePage {
    readonly card: InlineCard;

    constructor(page: Page) {
        super(page);
        this.card = new InlineCard(page);
    }

    async addMasquerade(outInterface: string) {
        await this.page.getByRole('button', { name: /\+ Add .*Rule/ }).click();
        const card = new InlineCard(this.page, '.create-card');
        await card.expectVisible();

        await card.selectOption('Rule Type', 'masquerade');
        await card.selectOption('Outbound Interface', outInterface);

        await card.clickButton(/Add .*Rule/);
        await card.expectHidden();
    }

    async addDNAT(externalPort: string, toIp: string, toPort: string) {
        await this.page.getByRole('button', { name: /\+ Add .*Rule/ }).click();
        const card = new InlineCard(this.page, '.create-card');
        await card.expectVisible();

        await card.selectOption('Rule Type', 'dnat');
        await card.fill('External Port', externalPort);
        await card.fill('Forward to Address', toIp);
        await card.fill('Forward to Port', toPort);

        await card.clickButton(/Add .*Rule/);
        await card.expectHidden();
    }
}

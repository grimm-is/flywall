import { type Page } from '@playwright/test';
import { BasePage } from '../BasePage';
import { InlineCard } from '../../components/InlineCard';

export class QoSTab extends BasePage {
    readonly card: InlineCard;

    constructor(page: Page) {
        super(page);
        this.card = new InlineCard(page);
    }

    async addPolicy(name: string, iface: string, down: string, up: string) {
        await this.page.getByTestId('add-policy-btn').click();
        const card = new InlineCard(this.page, '.create-card');
        await card.expectVisible();

        await card.fill('Policy Name', name);
        await card.selectOption('Interface', { label: iface });
        await card.fill(/Download/i, down);
        await card.fill(/Upload/i, up);

        await card.content.getByTestId('save-policy-btn').click();
        await card.expectHidden();
    }
}

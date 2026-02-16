import { type Page } from '@playwright/test';
import { BasePage } from '../BasePage';
import { InlineCard } from '../../components/InlineCard';

export class RoutingTab extends BasePage {
    readonly card: InlineCard;

    constructor(page: Page) {
        super(page);
        this.card = new InlineCard(page);
    }

    async addStaticRoute(dest: string, gateway: string, metric: string, ifaceIdx = 1) {
        await this.page.getByRole('button', { name: /Static Routes/i }).click();

        await this.page.getByRole('button').filter({ hasText: /Add Static Route/i }).click();

        const card = new InlineCard(this.page, '.create-card');
        await card.expectVisible();

        await card.fill(/Destination/i, dest);
        await card.fill(/Gateway/i, gateway);
        await card.selectOption(/Interface/i, { index: ifaceIdx });
        await card.fill(/Metric/i, metric);

        await card.clickButton(/Add|Save/i);
        await card.expectHidden();
    }
}

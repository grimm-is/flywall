import { type Page, type Locator, expect } from '@playwright/test';

/**
 * InlineCard component helper for e2e tests
 * Works with the inline form pattern used throughout the app
 */
export class InlineCard {
    readonly page: Page;
    readonly content: Locator;

    constructor(page: Page, cardSelector = '.edit-card, .create-card, .card') {
        this.page = page;
        // Target the inline card form - typically inside a Card component
        this.content = page.locator(cardSelector).first();
    }

    async expectVisible(titlePattern?: RegExp | string) {
        await expect(this.content).toBeVisible({ timeout: 5000 });
        if (titlePattern) {
            await expect(this.content).toContainText(titlePattern);
        }
    }

    async expectHidden() {
        await expect(this.content).toBeHidden({ timeout: 5000 });
    }

    async fill(label: string | RegExp, value: string) {
        await this.content.getByLabel(label).fill(value);
    }

    async selectOption(label: string | RegExp, option: string | { label: string } | { index: number }) {
        await this.content.getByLabel(label).selectOption(option);
    }

    async clickButton(name: string | RegExp) {
        await this.content.getByRole('button', { name }).click();
    }
}

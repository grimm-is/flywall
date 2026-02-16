import { type Page, type Locator, expect } from '@playwright/test';

export class Modal {
    readonly page: Page;
    readonly content: Locator;

    constructor(page: Page) {
        this.page = page;
        this.content = page.locator('.modal-content');
    }

    async expectVisible(titlePattern?: RegExp | string) {
        await expect(this.content).toBeVisible();
        if (titlePattern) {
            await expect(this.content).toContainText(titlePattern);
        }
    }

    async expectHidden() {
        await expect(this.content).toBeHidden();
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

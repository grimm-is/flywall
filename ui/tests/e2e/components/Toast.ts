import { type Page, type Locator, expect } from '@playwright/test';

export class Toast {
    readonly page: Page;

    constructor(page: Page) {
        this.page = page;
    }

    async expectSuccess(timeout = 10000) {
        await expect(this.page.locator('.toast-success')).toBeVisible({ timeout });
    }

    async expectError(timeout = 5000) {
        await expect(this.page.locator('.toast-error')).toBeVisible({ timeout });
    }
}

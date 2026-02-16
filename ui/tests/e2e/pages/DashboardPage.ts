import { type Page, expect } from '@playwright/test';
import { BasePage } from './BasePage';

export class DashboardPage extends BasePage {
    constructor(page: Page) {
        super(page);
    }

    async goto() {
        await super.goto('/');
    }

    async expectLoaded() {
        await expect(this.page.locator('.dashboard-grid')).toBeVisible();
    }
}

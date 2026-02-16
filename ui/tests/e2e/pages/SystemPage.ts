import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './BasePage';

export class SystemPage extends BasePage {
    constructor(page: Page) {
        super(page);
    }

    async gotoSettings() {
        await super.goto('/system/settings');
    }

    async toggleSetting(name: string | RegExp, enabled: boolean) {
        const container = this.page.locator('.setting-item', { hasText: name });
        await this.toggle.set(container, enabled);
    }
}

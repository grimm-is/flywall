import { type Page, type Locator, expect } from '@playwright/test';
import { Toggle } from '../components/Toggle';

export abstract class BasePage {
    readonly page: Page;
    readonly toggle: Toggle;

    constructor(page: Page) {
        this.page = page;
        this.toggle = new Toggle(page);
    }

    async goto(path = '/') {
        await this.page.goto(path);
        await this.waitForLoad();
    }

    async waitForLoad() {
        await expect(this.page.locator('.loading-overlay')).toBeHidden({ timeout: 10000 });
        await expect(this.page.locator('.loading-view')).toBeHidden({ timeout: 10000 });
        // Basic check for dashboard shell
        await expect(this.page.locator('.dashboard-shell')).toBeVisible({ timeout: 10000 });
    }

    async getConfig(source: 'staged' | 'running' = 'staged') {
        const url = source === 'running'
            ? `/api/config?source=running`
            : `/api/config`;
        const response = await this.page.request.get(url);
        expect(response.ok()).toBeTruthy();
        return await response.json();
    }

    // Helper to bypass backdrop issues
    async fixBackdrop() {
        await this.page.addStyleTag({
            content: `
            .modal-backdrop {
                pointer-events: none !important;
                background: transparent !important;
                backdrop-filter: none !important;
            }
            .modal-content {
                pointer-events: auto !important;
            }
        ` });
    }
}

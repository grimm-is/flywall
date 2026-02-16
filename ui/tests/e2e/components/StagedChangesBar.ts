import { type Page, type Locator, expect } from '@playwright/test';
import { Toast } from './Toast';

export class StagedChangesBar {
    readonly page: Page;
    readonly bar: Locator;
    readonly toast: Toast;

    constructor(page: Page) {
        this.page = page;
        this.bar = page.locator('.staged-changes-bar');
        this.toast = new Toast(page);
    }

    async expectVisible() {
        await expect(this.bar).toBeVisible();
    }

    async expectHidden() {
        await expect(this.bar).toBeHidden();
    }

    async apply() {
        await this.expectVisible();
        await this.bar.getByRole('button', { name: /Apply/i }).click();

        // Handle confirmation modal if it appears
        const modal = this.page.locator('.modal-content', { hasText: /Confirm Configuration/i });
        try {
            await expect(modal).toBeVisible({ timeout: 2000 });
            await modal.getByRole('button', { name: /Apply|Confirm/i }).click();
        } catch (e) {
            // Confirmation might be skipped
        }

        await this.toast.expectSuccess();
        await this.expectHidden();
    }

    async discard() {
        await this.expectVisible();
        await this.bar.getByRole('button', { name: /Discard/i }).click();

        // Confirm discard modal
        const modal = this.page.locator('.modal-content', { hasText: /Discard Changes/i });
        await expect(modal).toBeVisible();
        await modal.getByRole('button', { name: /Discard|Confirm/i }).click();

        await this.expectHidden();
    }
}

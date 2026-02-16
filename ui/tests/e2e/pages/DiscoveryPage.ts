import { type Page, expect } from '@playwright/test';
import { BasePage } from './BasePage';

export class DiscoveryPage extends BasePage {
    constructor(page: Page) {
        super(page);
    }

    async goto() {
        await super.goto('/discovery');
    }

    async startScan() {
        // Assuming there's a scan button
        await this.page.getByRole('button', { name: /Scan Network|Start Scan/i }).click();

        // Wait for scan to indicate active or completion
        // Mock API returns immediate start, might show specific UI state
        // Let's assume a toast or status change
    }

    async expectDeviceVisible(ssidOrName: string) {
        await expect(this.page.getByText(ssidOrName)).toBeVisible();
    }
}

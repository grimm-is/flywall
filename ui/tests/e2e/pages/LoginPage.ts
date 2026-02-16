import { type Page, expect } from '@playwright/test';
import { BasePage } from './BasePage';

export class LoginPage extends BasePage {
    constructor(page: Page) {
        super(page);
    }

    async goto() {
        await this.page.goto('/');
        // Don't call waitForLoad here - login page doesn't have dashboard-shell
        // Wait for either login or setup form to be visible
        await this.page.waitForSelector('#login-username, #setup-username', { timeout: 10000 });
    }

    async login(username = 'admin', password = 'TestP@ssw0rd!') {
        // Handle setup if present
        if (await this.page.locator('#setup-username').isVisible()) {
            await this.page.locator('#setup-username').fill(username);
            await this.page.locator('#setup-password').fill(password);
            await this.page.locator('#setup-confirm').fill(password);
            await this.page.getByRole('button', { name: /Create Account/i }).click();
        } else if (await this.page.locator('#login-username').isVisible()) {
            await this.page.locator('#login-username').fill(username);
            await this.page.locator('#login-password').fill(password);
            await this.page.getByRole('button', { name: /Login|Sign in/i }).click();
        }

        await expect(this.page.locator('.dashboard-rail')).toBeVisible({ timeout: 15000 });

        // Apply backdrop fix
        await this.fixBackdrop();
    }
}

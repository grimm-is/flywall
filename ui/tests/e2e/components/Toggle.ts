import { type Page, type Locator, expect } from '@playwright/test';

export class Toggle {
    readonly page: Page;

    constructor(page: Page) {
        this.page = page;
    }

    async set(locator: Locator, targetState: boolean) {
        let button = locator;
        // If the locator provided is not the switch itself (e.g. wrapper), find the switch inside
        try {
            const role = await locator.getAttribute('role');
            if (role !== 'switch') {
                button = locator.getByRole('switch').first();
            }
        } catch (e) {
            button = locator.getByRole('switch').first();
        }

        // Ensure button is ready and not obscured
        await expect(button).toBeVisible();

        // Handle potential backdrop obstruction
        await this.page.locator('.modal-backdrop').evaluateAll((elements) => {
            elements.forEach((el) => {
                (el as HTMLElement).style.pointerEvents = 'none';
            });
        });

        const currentState = (await button.getAttribute('aria-checked')) === 'true';

        if (currentState !== targetState) {
            await button.click();
            await expect(button).toHaveAttribute('aria-checked', targetState.toString());
        }
    }
}

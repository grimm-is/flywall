import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './BasePage';
import { InlineCard } from '../components/InlineCard';

export class TunnelsPage extends BasePage {
    readonly card: InlineCard;

    constructor(page: Page) {
        super(page);
        this.card = new InlineCard(page);
    }

    async goto() {
        await super.goto('/tunnels');
    }

    async addWireGuard(name: string, iface: string, port: string, privKey: string, addr = '', table?: string) {
        await this.page.getByTestId('add-tunnel-btn').click();

        const card = new InlineCard(this.page, '.create-card');
        await card.expectVisible();

        await this.page.getByTestId('vpn-conn-name').fill(name);
        await this.page.getByTestId('vpn-conn-iface').fill(iface);
        await this.page.getByTestId('vpn-conn-port').fill(port);
        await this.page.getByTestId('vpn-conn-privkey').fill(privKey);
        await this.page.getByTestId('vpn-conn-addr').fill(addr);

        if (table) {
            const details = card.content.locator('details');
            if (await details.isVisible()) {
                await details.click();
            }
            await card.fill('Routing Table', table);
        }

        await card.clickButton('Save Interface');
        await card.expectHidden();
    }

    async importConfig(params: { buffer: Buffer, filename: string, mimeType?: string }) {
        const fileChooserPromise = this.page.waitForEvent('filechooser');
        await this.page.getByRole('button', { name: /Import/i }).click();
        const fileChooser = await fileChooserPromise;

        await fileChooser.setFiles({
            name: params.filename,
            mimeType: params.mimeType || 'text/plain',
            buffer: params.buffer
        });

        await this.card.expectVisible();
        await this.card.clickButton(/Save/i);
        await this.card.expectHidden();
    }
}

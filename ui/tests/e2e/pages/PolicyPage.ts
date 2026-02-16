import { type Page } from '@playwright/test';
import { BasePage } from './BasePage';
import { SecurityTab } from './policy/SecurityTab';
import { NATTab } from './policy/NATTab';

import { RoutingTab } from './policy/RoutingTab';
import { QoSTab } from './policy/QoSTab';

export class PolicyPage extends BasePage {
    readonly security: SecurityTab;
    readonly nat: NATTab;
    readonly routing: RoutingTab;
    readonly qos: QoSTab;

    constructor(page: Page) {
        super(page);
        this.security = new SecurityTab(page);
        this.nat = new NATTab(page);
        this.routing = new RoutingTab(page);
        this.qos = new QoSTab(page);
    }

    async goto(tab?: 'security' | 'nat' | 'routing' | 'traffic') {
        let url = '/policy';
        if (tab) {
            url += `?tab=${tab}`;
        }
        await super.goto(url);
    }
}

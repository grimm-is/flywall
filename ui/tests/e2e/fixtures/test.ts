import { test as base } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';
import { NetworkPage } from '../pages/NetworkPage';
import { PolicyPage } from '../pages/PolicyPage';
import { TunnelsPage } from '../pages/TunnelsPage';
import { SystemPage } from '../pages/SystemPage';
import { DiscoveryPage } from '../pages/DiscoveryPage';

// Declare the types of your fixtures.
type MyFixtures = {
    loginPage: LoginPage;
    dashboardPage: DashboardPage;
    networkPage: NetworkPage;
    policyPage: PolicyPage;
    tunnelsPage: TunnelsPage;
    systemPage: SystemPage;
    discoveryPage: DiscoveryPage;
};

// Extend base test to include fixtures.
export const test = base.extend<MyFixtures>({
    loginPage: async ({ page }, use) => {
        await use(new LoginPage(page));
    },
    dashboardPage: async ({ page }, use) => {
        await use(new DashboardPage(page));
    },
    networkPage: async ({ page }, use) => {
        await use(new NetworkPage(page));
    },
    policyPage: async ({ page }, use) => {
        await use(new PolicyPage(page));
    },
    tunnelsPage: async ({ page }, use) => {
        await use(new TunnelsPage(page));
    },
    systemPage: async ({ page }, use) => {
        await use(new SystemPage(page));
    },
    discoveryPage: async ({ page }, use) => {
        await use(new DiscoveryPage(page));
    },
});

export { expect } from '@playwright/test';

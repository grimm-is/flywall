import { test, expect } from '../fixtures/test';
import { StagedChangesBar } from '../components/StagedChangesBar';

test.describe.fixme('Staged Configuration Workflow', () => {

    test.beforeEach(async ({ loginPage }) => {
        await loginPage.goto();
        await loginPage.login();
    });

    test('Verify Staged vs Running state', async ({ policyPage }) => {
        const stagedBar = new StagedChangesBar(policyPage.page);
        const ruleName = 'Staged Test Rule';

        await policyPage.goto('security');

        // 1. Create a Staged Change
        // Ensure policy exists (might need helper/fixture setup, but assuming 'lan'/'wan' available or we create it)
        // For simplicity, let's create a policy or verify existence
        // In real E2E, we might want to ensure a clean slate or known state.
        // Assuming 'lan' -> 'wan' exists from previous tests or default state?
        // Let's create it if missing, handling the error if it exists?
        // The original test assumed it might exist or created it.
        // Let's just create a new one to be safe or use a safe one.
        // Using 'lan'->'wan' again.

        // This is tricky if previous tests deleted it.
        // Best approach: try to create, ignore if fails/exists.
        try { await policyPage.security.addPolicy('lan', 'wan'); } catch (e) { }

        await policyPage.security.addRule('lan', 'wan', ruleName, '5353', 'UDP');

        // 2. Verify Staged has the change
        const stagedConfig = await policyPage.getConfig('staged');
        let policy = stagedConfig.policies.find((p: any) => p.from === 'lan' && p.to === 'wan');
        expect(policy?.rules?.find((r: any) => r.name === ruleName)).toBeDefined();

        // 3. Verify Running does NOT have the change
        const runningConfig = await policyPage.getConfig('running');
        policy = runningConfig.policies?.find((p: any) => p.from === 'lan' && p.to === 'wan');
        expect(policy?.rules?.find((r: any) => r.name === ruleName)).toBeUndefined();

        // 4. Apply Configuration
        await stagedBar.apply();

        // 5. Verify Running HAS the change now
        const runningConfigAfter = await policyPage.getConfig('running');
        policy = runningConfigAfter.policies.find((p: any) => p.from === 'lan' && p.to === 'wan');
        expect(policy?.rules?.find((r: any) => r.name === ruleName)).toBeDefined();
    });

    test('Verify Discard Changes', async ({ policyPage }) => {
        const stagedBar = new StagedChangesBar(policyPage.page);
        await policyPage.goto('security');

        try { await policyPage.security.addPolicy('lan', 'wan'); } catch (e) { }

        await policyPage.security.addRule('lan', 'wan', 'To Be Discarded', '1234', 'TCP');

        // Verify in Staged
        let config = await policyPage.getConfig('staged');
        let policy = config.policies.find((p: any) => p.from === 'lan' && p.to === 'wan');
        expect(policy?.rules?.find((r: any) => r.name === 'To Be Discarded')).toBeDefined();

        // Discard
        await stagedBar.discard();

        // Verify GONE from Staged
        config = await policyPage.getConfig('staged');
        policy = config.policies.find((p: any) => p.from === 'lan' && p.to === 'wan');
        expect(policy?.rules?.find((r: any) => r.name === 'To Be Discarded')).toBeUndefined();
    });
});

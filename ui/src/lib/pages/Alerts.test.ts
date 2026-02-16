import { render, screen } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import Alerts from './Alerts.svelte';
import { api } from '$lib/stores/app';

vi.mock('$lib/stores/app', () => ({
    api: {
        get: vi.fn()
    }
}));

describe('Alerts Page', () => {
    it('renders and fetches alerts', async () => {
        vi.mocked(api.get).mockResolvedValue([
            { severity: 'critical', timestamp: new Date().toISOString(), rule_name: 'Test Rule', message: 'Test Message' }
        ]);

        render(Alerts);

        expect(api.get).toHaveBeenCalledWith(expect.stringContaining('/api/alerts/history'));
        // Wait for it to appear
        expect(await screen.findByText('Test Message')).toBeTruthy();
        expect(await screen.findByText('critical')).toBeTruthy();
    });
});

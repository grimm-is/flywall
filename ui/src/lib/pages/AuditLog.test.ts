import { render, screen } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import AuditLog from './AuditLog.svelte';
import { api } from '$lib/stores/app';

vi.mock('$lib/stores/app', () => ({
    api: {
        get: vi.fn()
    }
}));

describe('AuditLog Page', () => {
    it('renders and fetches audit logs', async () => {
        vi.mocked(api.get).mockResolvedValue([
            { timestamp: new Date().toISOString(), user: 'admin', action: 'login', resource: 'auth', details: { ip: '1.2.3.4' } }
        ]);

        render(AuditLog);

        expect(api.get).toHaveBeenCalledWith(expect.stringContaining('/api/audit'));
        // Wait for it to appear
        expect(await screen.findByText('admin')).toBeTruthy();
        expect(await screen.findByText('login')).toBeTruthy();
    });
});

import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import Backups from './Backups.svelte';
import { api } from '$lib/stores/app';

vi.mock('$lib/stores/app', () => ({
    api: {
        get: vi.fn(),
        post: vi.fn(),
        delete: vi.fn()
    }
}));

// Mock utils/format
vi.mock('$lib/utils/format', () => ({
    formatBytes: (b) => `${b} B`
}));

// Mock Button and Icon if needed (assumed they work in test env)

describe('Backups Page', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('renders and fetches backups', async () => {
        vi.mocked(api.get).mockResolvedValue([
            { id: '1', filename: 'backup-1.tar.gz', description: 'Auto backup', size: 1024, created_at: new Date().toISOString() }
        ]);

        render(Backups);

        expect(api.get).toHaveBeenCalledWith('/api/backups');
        expect(await screen.findByText('backup-1.tar.gz')).toBeTruthy();
    });

    it('creates backup when button clicked', async () => {
        vi.mocked(api.get).mockResolvedValue([]);
        vi.mocked(api.post).mockResolvedValue({});

        render(Backups);

        const createBtn = screen.getByText('Create Backup'); // Icon name logic might make text different? 
        // Our button has <Icon ... /> Create Backup
        // Testing library finds by text content usually.

        await fireEvent.click(createBtn);

        expect(api.post).toHaveBeenCalledWith('/api/backups', expect.any(Object));
        // It reloads
        expect(api.get).toHaveBeenCalledTimes(2); // Once on mount, once after create
    });
});

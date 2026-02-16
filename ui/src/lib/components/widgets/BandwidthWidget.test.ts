import { render, screen } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import BandwidthWidget from './BandwidthWidget.svelte';
import { api } from '$lib/stores/app';

// Mock the API store
vi.mock('$lib/stores/app', () => ({
    api: {
        get: vi.fn()
    }
}));

// Mock UI store to avoid localStorage issues
vi.mock('$lib/stores/ui', () => ({
    isLayoutEditing: { subscribe: (fn: any) => { fn(false); return () => { }; } },
    dashboardLayout: { subscribe: (fn: any) => { fn([]); return () => { }; } },
    sidebarExpanded: { subscribe: (fn: any) => { fn(true); return () => { }; } }
}));

// Polyfill ResizeObserver for JSDOM
global.ResizeObserver = class ResizeObserver {
    observe() { }
    unobserve() { }
    disconnect() { }
};


describe('BandwidthWidget', () => {
    it('renders the widget title', () => {
        vi.mocked(api.get).mockResolvedValue({});
        render(BandwidthWidget, { props: { onremove: vi.fn() } });
        expect(screen.getByText('Bandwidth')).toBeTruthy();
    });

    it('fetches data on mount', async () => {
        vi.mocked(api.get).mockResolvedValue({
            "eth0": { rx_bytes: 1000, tx_bytes: 2000 },
            "lo": { rx_bytes: 500, tx_bytes: 500 }
        });

        render(BandwidthWidget, { props: { onremove: vi.fn() } });

        expect(api.get).toHaveBeenCalledWith('/api/traffic');
    });
});

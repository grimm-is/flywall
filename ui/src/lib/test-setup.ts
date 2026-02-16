/**
 * Vitest test setup file
 * Configures test environment before each test run
 */

import { vi } from 'vitest';

(globalThis as any).fetch = vi.fn();

// Mock localStorage for tests
const localStorageMock = {
    getItem: vi.fn(),
    setItem: vi.fn(),
    removeItem: vi.fn(),
    clear: vi.fn(),
};

Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock });

// Mock matchMedia for theme tests
Object.defineProperty(globalThis, 'matchMedia', {
    value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
    })),
});

// Mock svelte-i18n
vi.mock('svelte-i18n', () => ({
    t: {
        subscribe: (cb: (val: any) => void) => {
            cb((key: string, options?: any) => key);
            return () => { };
        },
    },
    locale: {
        subscribe: (cb: (val: any) => void) => {
            cb('en');
            return () => { };
        },
    },
    isLoading: {
        subscribe: (cb: (val: any) => void) => {
            cb(false);
            return () => { };
        },
    },
}));

// Mock $app/navigation
vi.mock('$app/navigation', () => ({
    goto: vi.fn(),
    invalidate: vi.fn(),
    invalidateAll: vi.fn(),
    preloadData: vi.fn(),
    preloadCode: vi.fn(),
    beforeNavigate: vi.fn(),
    afterNavigate: vi.fn(),
}));

// Mock $app/stores
vi.mock('$app/stores', () => ({
    getStores: () => ({
        page: {
            subscribe: (cb: (val: any) => void) => {
                cb({ url: new URL('http://localhost') });
                return () => { };
            },
        },
        navigating: {
            subscribe: (cb: (val: any) => void) => {
                cb(null);
                return () => { };
            },
        },
        updated: {
            subscribe: (cb: (val: any) => void) => {
                cb(false);
                return () => { };
            },
        },
    }),
    page: {
        subscribe: (cb: (val: any) => void) => {
            cb({ url: new URL('http://localhost') });
            return () => { };
        },
    },
}));

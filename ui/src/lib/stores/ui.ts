import { writable } from 'svelte/store';

// ============================================================================
// UI Persistence Store
// Handles state that should persist across sessions via localStorage
// ============================================================================

// Key prefix for localStorage
const STORAGE_PREFIX = 'flywall:ui:';

function createPersistentStore<T>(key: string, initialValue: T) {
    // Check if running in browser
    const isBrowser = typeof localStorage !== 'undefined';
    const storageKey = `${STORAGE_PREFIX}${key}`;

    // Get stored value
    let startValue = initialValue;
    if (isBrowser) {
        const stored = localStorage.getItem(storageKey);
        if (stored !== null) {
            try {
                startValue = JSON.parse(stored);
            } catch (e) {
                console.error(`Failed to parse stored value for ${key}`, e);
            }
        }
    }

    const store = writable<T>(startValue);

    // Subscribe to changes and persist
    if (isBrowser) {
        store.subscribe(value => {
            try {
                localStorage.setItem(storageKey, JSON.stringify(value));
            } catch (e) {
                console.error(`Failed to persist value for ${key}`, e);
            }
        });
    }

    return store;
}

// ============================================================================
// Specific Stores
// ============================================================================

// Sidebar Navigation State (expanded/collapsed)
export const sidebarExpanded = createPersistentStore<boolean>('sidebarExpanded', false);

// Dashboard Widget Layout
export interface WidgetLayout {
    id: string;      // Unique ID for the widget instance
    type: string;    // 'system-stats', 'interfaces', 'uplinks', 'topology-hero', etc.
    x: number;       // Grid X position
    y: number;       // Grid Y position
    w: number;       // Grid Width (1-4)
    h: number;       // Grid Height (1-4)
}

// Default Dashboard Layout
const DEFAULT_LAYOUT: WidgetLayout[] = [
    { id: 'hero', type: 'topology-hero', x: 0, y: 0, w: 4, h: 2 },
    { id: 'stats-uptime', type: 'stat-uptime', x: 0, y: 2, w: 1, h: 1 },
    { id: 'stats-cpu', type: 'stat-cpu', x: 1, y: 2, w: 1, h: 1 },
    { id: 'stats-mem', type: 'stat-memory', x: 2, y: 2, w: 1, h: 1 },
    { id: 'stats-disk', type: 'stat-disk', x: 3, y: 2, w: 1, h: 1 },
    { id: 'uplinks', type: 'uplinks', x: 0, y: 3, w: 4, h: 1 },
    { id: 'zones', type: 'zones', x: 0, y: 4, w: 4, h: 2 },
];

export const dashboardLayout = createPersistentStore<WidgetLayout[]>('dashboardLayout', DEFAULT_LAYOUT);

// Edit Mode State (not persisted)
export const isLayoutEditing = writable(false);

import { writable, get } from 'svelte/store';
import { authStatus } from './app';

const STAFF_API_BASE = '/api/staff';

// Types corresponding to Go structs
export interface Organization {
    id: string;
    name: string;
    type: string;
    account_status: string;
    billing_email: string;
    created_at: string;
}

export interface OrganizationDetails {
    organization: Organization;
    subscription: Subscription | null;
    sites: Site[];
    users: User[];
    devices: Device[];
    device_count: number;
    site_count: number;
    user_count: number;
    account_status: string;
    billing_email: string;
}

export interface Subscription {
    id: string;
    org_id: string;
    plan_id: string;
    status: string;
    billing_cycle: string;
    current_period_start: string;
    current_period_end: string;
    canceled_at?: string;
    cancel_at_period_end: boolean;
    plan?: SubscriptionPlan;
}

export interface SubscriptionPlan {
    id: string;
    name: string;
    display_name: string;
    price_monthly: number;
    price_yearly?: number;
    active: boolean;
}

export interface Invoice {
    id: string;
    invoice_number: string;
    status: string;
    amount_due: number;
    amount_paid: number;
    currency: string;
    due_date?: string;
    paid_at?: string;
    created_at: string;
}

export interface Site {
    id: string;
    name: string;
    description: string;
}

export interface User {
    id: string;
    email: string;
    name: string;
    created_at: string;
}

export interface Device {
    id: string;
    name: string;
    status: string;
    last_seen?: string;
}

// API Request Helper
async function apiStaffRequest(endpoint: string, options: RequestInit = {}): Promise<any> {
    const url = `${STAFF_API_BASE}${endpoint}`;
    const auth = get(authStatus);

    const defaultOptions: RequestInit = {
        headers: {
            'Content-Type': 'application/json',
            ...(auth?.csrf_token ? { 'X-CSRF-Token': auth.csrf_token } : {}),
        },
        credentials: 'include',
    };

    const response = await fetch(url, { ...defaultOptions, ...options });

    if (!response.ok) {
        throw new Error(`API Error: ${response.statusText}`);
    }

    // Handle 204 No Content
    if (response.status === 204) return null;

    return response.json();
}

// Store Definition
function createStaffStore() {
    const { subscribe, update } = writable({
        loading: false,
        error: null as string | null,
        organizations: [] as Organization[],
        currentOrg: null as OrganizationDetails | null,
        plans: [] as SubscriptionPlan[],
    });

    return {
        subscribe,

        async listOrganizations(limit = 50, offset = 0) {
            update(s => ({ ...s, loading: true, error: null }));
            try {
                const orgs = await apiStaffRequest(`/organizations?limit=${limit}&offset=${offset}`);
                update(s => ({ ...s, organizations: orgs, loading: false }));
                return orgs;
            } catch (e: any) {
                console.error("List Orgs Failed", e);
                update(s => ({ ...s, error: e.message, loading: false }));
                throw e;
            }
        },

        async searchOrganizations(query: string) {
            update(s => ({ ...s, loading: true, error: null }));
            try {
                const orgs = await apiStaffRequest(`/organizations/search?q=${encodeURIComponent(query)}`);
                update(s => ({ ...s, organizations: orgs, loading: false }));
                return orgs;
            } catch (e: any) {
                update(s => ({ ...s, error: e.message, loading: false }));
                throw e;
            }
        },

        async getOrganizationDetails(id: string) {
            update(s => ({ ...s, loading: true, error: null }));
            try {
                const details = await apiStaffRequest(`/organizations/${id}`);
                update(s => ({ ...s, currentOrg: details, loading: false }));
                return details;
            } catch (e: any) {
                update(s => ({ ...s, error: e.message, loading: false }));
                throw e;
            }
        },

        async updateOrganizationStatus(id: string, status: string) {
            await apiStaffRequest(`/organizations/${id}/status`, {
                method: 'PUT',
                body: JSON.stringify({ status }),
            });
            // Refresh details if current
            const s = get(this);
            if (s.currentOrg?.organization.id === id) {
                await this.getOrganizationDetails(id);
            }
        },

        async updateSubscription(orgID: string, planID: string, status?: string) {
            await apiStaffRequest(`/organizations/${orgID}/subscription`, {
                method: 'PUT',
                body: JSON.stringify({ plan_id: planID, status }),
            });
            await this.getOrganizationDetails(orgID);
        },

        async createInvoice(orgID: string, amount: number, currency = 'usd') {
            await apiStaffRequest(`/organizations/${orgID}/invoices`, {
                method: 'POST',
                body: JSON.stringify({ amount_due: amount, currency, line_items: [] }), // Simplified
            });
            await this.getOrganizationDetails(orgID); // Refresh to see new invoice
        },

        async listInvoices(orgID: string) {
            // New Method
            try {
                return await apiStaffRequest(`/organizations/${orgID}/invoices`);
            } catch (e) {
                console.error("Failed to list invoices", e);
                return [];
            }
        },

        async listPlans() {
            const plans = await apiStaffRequest('/plans');
            update(s => ({ ...s, plans }));
            return plans;
        }
    };
}

export const staffStore = createStaffStore();

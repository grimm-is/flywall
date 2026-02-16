import { writable, get } from 'svelte/store';
import { authStatus } from './app';

const API_BASE = '/api/v1/billing';

export interface Plan {
    id: string;
    name: string;
    display_name: string;
    price_monthly: number; // cents
    price_yearly?: number;
    features: string[]; // JSON array in DB, handle parsing if string
}

export interface Subscription {
    id: string;
    plan_id: string;
    status: string;
    billing_cycle: string;
    current_period_end: string;
    cancel_at_period_end: boolean;
    plan?: Plan;
}

export interface Invoice {
    id: string;
    invoice_number: string;
    amount_due: number;
    amount_paid: number;
    status: string;
    created_at: string;
    pdf_url?: string; // Placeholder
}

export interface Transaction {
    id: string;
    amount: number;
    status: string;
    type: string;
    created_at: string;
    provider_ref?: string;
}

async function apiRequest(endpoint: string, options: RequestInit = {}): Promise<any> {
    const url = `${API_BASE}${endpoint}`;
    const auth = get(authStatus);

    const defaultOptions: RequestInit = {
        headers: {
            'Content-Type': 'application/json',
            ...(auth?.csrf_token ? { 'X-CSRF-Token': auth.csrf_token } : {}),
        },
        credentials: 'include',
    };

    const response = await fetch(url, { ...defaultOptions, ...options });

    // Handle 204 No Content
    if (response.status === 204) return null;

    if (!response.ok) {
        throw new Error(`API Error: ${response.statusText}`);
    }

    return response.json();
}

function createBillingStore() {
    const { subscribe, update } = writable({
        loading: false,
        error: null as string | null,
        subscription: null as Subscription | null,
        plans: [] as Plan[],
        invoices: [] as Invoice[],
        transactions: [] as Transaction[],
    });

    return {
        subscribe,

        async getSubscription() {
            update(s => ({ ...s, loading: true, error: null }));
            try {
                const sub = await apiRequest('/subscription');
                // Backend might return { status: 'none' } if no sub
                if (sub.status === 'none') {
                    update(s => ({ ...s, subscription: null, loading: false }));
                } else {
                    update(s => ({ ...s, subscription: sub, loading: false }));
                }
            } catch (e: any) {
                console.error("Get Subscription Failed", e);
                update(s => ({ ...s, error: e.message, loading: false }));
            }
        },

        async listPlans() {
            try {
                const plans = await apiRequest('/plans');
                update(s => ({ ...s, plans }));
            } catch (e) {
                console.error("List Plans Failed", e);
            }
        },

        async createSubscription(planID: string, cycle: 'monthly' | 'yearly') {
            update(s => ({ ...s, loading: true, error: null }));
            try {
                const sub = await apiRequest('/subscription', {
                    method: 'POST',
                    body: JSON.stringify({ plan_id: planID, billing_cycle: cycle })
                });
                update(s => ({ ...s, subscription: sub, loading: false }));
            } catch (e: any) {
                update(s => ({ ...s, error: e.message, loading: false }));
                throw e;
            }
        },

        async cancelSubscription() {
            try {
                await apiRequest('/subscription', { method: 'DELETE' });
                await this.getSubscription();
            } catch (e) {
                throw e;
            }
        },

        async listInvoices() {
            try {
                const invoices = await apiRequest('/invoices');
                update(s => ({ ...s, invoices }));
            } catch (e) {
                console.error("List Invoices Failed", e);
            }
        },

        async listTransactions() {
            try {
                const txs = await apiRequest('/transactions');
                update(s => ({ ...s, transactions: txs }));
            } catch (e) {
                console.error("List Transactions Failed", e);
            }
        }
    };
}

export const billingStore = createBillingStore();

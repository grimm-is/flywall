/**
 * Screenshot Anonymizer
 * 
 * Replaces sensitive network and device data in the visible DOM
 * for safe screenshot sharing. All replacements are deterministic
 * (same input -> same output) for visual consistency.
 */

import { writable, get } from 'svelte/store';

// Track anonymization state
export const isAnonymized = writable(false);
export const anonymizeEnabled = writable(true); // Feature flag

// Store original values for undo
let originalValues: Map<Element, string> = new Map();

// Deterministic hash for consistent replacements
function simpleHash(str: string): number {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
        const char = str.charCodeAt(i);
        hash = ((hash << 5) - hash) + char;
        hash = hash & hash; // Convert to 32bit integer
    }
    return Math.abs(hash);
}

// Generate fake IP from real IP (deterministic)
function anonymizeIP(ip: string): string {
    if (!ip) return ip;

    // Check if IPv6
    if (ip.includes(':')) {
        const hash = simpleHash(ip);
        return `2001:db8:${(hash % 0xffff).toString(16)}::${((hash >> 16) % 0xffff).toString(16)}`;
    }

    // IPv4
    const hash = simpleHash(ip);
    const parts = [
        10 + (hash % 245),           // 10-254
        (hash >> 8) % 256,
        (hash >> 16) % 256,
        1 + ((hash >> 24) % 254)     // 1-254
    ];
    return parts.join('.');
}

// Generate fake MAC from real MAC (deterministic)
function anonymizeMAC(mac: string): string {
    if (!mac) return mac;
    const hash = simpleHash(mac);
    const parts = [];
    for (let i = 0; i < 6; i++) {
        parts.push(((hash >> (i * 4)) & 0xff).toString(16).padStart(2, '0'));
    }
    return parts.join(':').toUpperCase();
}

// Generate fake hostname (deterministic)
function anonymizeHostname(hostname: string): string {
    if (!hostname) return hostname;
    const hash = simpleHash(hostname);
    const adjectives = ['happy', 'quick', 'clever', 'calm', 'bright', 'cool', 'swift', 'bold'];
    const nouns = ['device', 'host', 'node', 'client', 'system', 'machine', 'unit', 'station'];
    const adj = adjectives[hash % adjectives.length];
    const noun = nouns[(hash >> 8) % nouns.length];
    return `${adj}-${noun}-${(hash % 1000).toString().padStart(3, '0')}`;
}

// Generate fake domain (deterministic)
function anonymizeDomain(domain: string): string {
    if (!domain) return domain;
    const hash = simpleHash(domain);
    const prefixes = ['app', 'api', 'cdn', 'www', 'srv', 'data', 'cloud', 'net'];
    const bases = ['example', 'sample', 'demo', 'test', 'mock', 'fake', 'anon', 'redacted', 'placeholder'];
    const tlds = ['com', 'net', 'org', 'io', 'dev'];
    const prefix = prefixes[hash % prefixes.length];
    const base = bases[(hash >> 8) % bases.length];
    const tld = tlds[(hash >> 16) % tlds.length];
    return `${prefix}.${base}.${tld}`;
}

// Anonymize vendor name
function anonymizeVendor(vendor: string): string {
    if (!vendor) return vendor;
    const hash = simpleHash(vendor);
    const vendors = ['Acme Corp', 'Generic Inc', 'Network Co', 'Device Ltd', 'Tech Systems', 'IoT Devices'];
    return vendors[hash % vendors.length];
}

// Pattern matchers
const patterns = {
    // IPv4 address
    ipv4: /\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b/g,

    // IPv6 address (simplified)
    ipv6: /\b(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}\b|\b(?:[0-9a-fA-F]{1,4}:){1,7}:\b|\b::(?:[0-9a-fA-F]{1,4}:){0,6}[0-9a-fA-F]{1,4}\b/g,

    // MAC address
    mac: /\b(?:[0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}\b/g,

    // Domain-like strings (includes SNI)
    domain: /\b(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}\b/g,
};

// Anonymize text content
function anonymizeText(text: string): string {
    let result = text;

    // Replace MACs first (before they might be confused with other patterns)
    result = result.replace(patterns.mac, (match) => anonymizeMAC(match));

    // Replace IPs
    result = result.replace(patterns.ipv4, (match) => anonymizeIP(match));
    result = result.replace(patterns.ipv6, (match) => anonymizeIP(match));

    // Replace domains (be careful not to replace file paths)
    result = result.replace(patterns.domain, (match) => {
        // Skip if it looks like a file path or internal reference
        if (match.endsWith('.js') || match.endsWith('.ts') || match.endsWith('.css') ||
            match.endsWith('.svelte') || match.includes('localhost') ||
            match.startsWith('127.') || match === 'svelte.dev') {
            return match;
        }
        return anonymizeDomain(match);
    });

    return result;
}

// Walk DOM and replace text
function walkAndReplace(node: Node, restore: boolean = false): void {
    if (node.nodeType === Node.TEXT_NODE) {
        const text = node.textContent || '';
        if (text.trim()) {
            if (restore) {
                // Restore original
                const original = originalValues.get(node as Element);
                if (original !== undefined) {
                    node.textContent = original;
                }
            } else {
                // Store original and replace
                originalValues.set(node as Element, text);
                const anonymized = anonymizeText(text);
                if (anonymized !== text) {
                    node.textContent = anonymized;
                }
            }
        }
    } else if (node.nodeType === Node.ELEMENT_NODE) {
        const element = node as Element;

        // Skip script and style elements
        if (element.tagName === 'SCRIPT' || element.tagName === 'STYLE') {
            return;
        }

        // Handle input/textarea values
        if (element.tagName === 'INPUT' || element.tagName === 'TEXTAREA') {
            const input = element as HTMLInputElement;
            if (restore) {
                const original = originalValues.get(element);
                if (original !== undefined) {
                    input.value = original;
                }
            } else {
                originalValues.set(element, input.value);
                input.value = anonymizeText(input.value);
            }
        }

        // Handle title attribute
        if (element.hasAttribute('title')) {
            const title = element.getAttribute('title') || '';
            if (restore) {
                const original = originalValues.get(element);
                if (original !== undefined) {
                    element.setAttribute('title', original);
                }
            } else {
                originalValues.set(element, title);
                element.setAttribute('title', anonymizeText(title));
            }
        }

        // Recurse into children
        for (const child of Array.from(node.childNodes)) {
            walkAndReplace(child, restore);
        }
    }
}

// Public API
export function anonymize(): void {
    if (get(isAnonymized)) return;

    originalValues.clear();
    walkAndReplace(document.body);
    isAnonymized.set(true);

    // Add visual indicator
    document.body.classList.add('anonymized');
}

export function restore(): void {
    if (!get(isAnonymized)) return;

    walkAndReplace(document.body, true);
    originalValues.clear();
    isAnonymized.set(false);

    document.body.classList.remove('anonymized');
}

export function toggle(): void {
    if (get(isAnonymized)) {
        restore();
    } else {
        anonymize();
    }
}

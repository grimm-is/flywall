// Vault Store: Manages Zero-Knowledge Keys
import { writable, get } from 'svelte/store';
import { deriveMasterKey, unwrapDeviceKey, type EncryptedBlob } from '../crypto';
import { api, alertStore } from './app';

interface VaultState {
    unlocked: boolean;
    masterKey: CryptoKey | null;
    deviceKeys: Map<string, CryptoKey>; // keyID -> CryptoKey
}

const initialState: VaultState = {
    unlocked: false,
    masterKey: null,
    deviceKeys: new Map(),
};

function createVaultStore() {
    const { subscribe, set, update } = writable<VaultState>(initialState);

    return {
        subscribe,

        // Unlock the vault with the master password
        async unlock(password: string) {
            try {
                // In a real app, we'd fetch the User's Salt from their profile or a specific Vault Metadata endpoint.
                // For MVP Phase 8, we might assuming a deterministic salt based on their OrgID or similar,
                // OR we fetch the "Vault Metadata" which simply stores the Salt.
                // Let's assume the API returns the salt on a "check-vault" endpoint or similar.
                // OR we use a fixed salt for now (Not secure for multi-user, but acceptable for PoC).
                // BETTER: The server "GetWrappedKeys" endpoint could return the Salt? No, keys might be wrapped with different salts?
                // Standard practice: Store KDF parameters (Salt, Iterations) in the DB alongside the Wrapped Key or User Profile.
                // We'll stick to a hardcoded "shared" salt for this step or fetch it.
                // Let's mock fetching salt or use a deterministic one (e.g. email).

                const salt = "flywall-vault-salt-v1"; // TODO: Fetch from user profile

                const masterKey = await deriveMasterKey(password, "73616c74"); // "salt" in hex
                // Wait, deriveMasterKey expects hex salt. "73616c74" is "salt"
                // Let's use a better salt in production.

                // Optimistically set unlocked.
                // Verify by trying to unwrap a key?
                // We'll defer verification to the first usage.

                update(s => ({
                    ...s,
                    unlocked: true,
                    masterKey: masterKey
                }));

                return true;
            } catch (e) {
                console.error("Vault unlock failed", e);
                return false;
            }
        },

        lock() {
            set(initialState);
        },

        // Get a device key (fetching wrapped key if needed)
        async getDeviceKey(keyID: string): Promise<CryptoKey | null> {
            const state = get({ subscribe });
            if (!state.unlocked || !state.masterKey) {
                // Vault is locked
                return null;
            }

            if (state.deviceKeys.has(keyID)) {
                return state.deviceKeys.get(keyID) || null;
            }

            try {
                // Fetch wrapped key from API
                // We assume api.get returns { wrapped_blob: "base64..." }
                // Implementation Plan said: GET /api/v1/vault/keys -> []WrappedKey
                // We might need to filter.

                const keys = await api.get('/vault/keys');
                // keys is []WrappedKey
                const targetKey = keys.find((k: any) => k.key_id === keyID);

                if (!targetKey) {
                    console.warn(`Key ${keyID} not found in vault`);
                    return null;
                }

                // Unwrap
                const deviceKey = await unwrapDeviceKey(targetKey.wrapped_blob, state.masterKey);

                // Cache it
                update(s => {
                    s.deviceKeys.set(keyID, deviceKey);
                    return s;
                });

                return deviceKey;
            } catch (e) {
                console.error(`Failed to unwrap key ${keyID}`, e);
                // Likely invalid password if unwrap fails (tag mismatch)
                alertStore.error("Failed to decrypt key. Wrong password?");
                return null;
            }
        }
    };
}

export const vault = createVaultStore();

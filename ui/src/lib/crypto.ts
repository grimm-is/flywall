// WebCrypto Utilities for Zero-Knowledge Vault

// Constants
const PBKDF2_ITERATIONS = 500000; // High iteration count for security
const KEY_LENGTH = 256;
const SALT_LENGTH = 16;
const IV_LENGTH = 12; // 96-bit IV for AES-GCM

// Types
export interface EncryptedBlob {
    iv: string; // Base64
    ciphertext: string; // Base64
}

// Utilities for Base64 conversion
function arrayBufferToBase64(buffer: ArrayBuffer): string {
    let binary = '';
    const bytes = new Uint8Array(buffer);
    const len = bytes.byteLength;
    for (let i = 0; i < len; i++) {
        binary += String.fromCharCode(bytes[i]);
    }
    return window.btoa(binary);
}

function base64ToArrayBuffer(base64: string): ArrayBuffer {
    const binary_string = window.atob(base64);
    const len = binary_string.length;
    const bytes = new Uint8Array(len);
    for (let i = 0; i < len; i++) {
        bytes[i] = binary_string.charCodeAt(i);
    }
    return bytes.buffer;
}

// hex to buffer
function hexToBuff(hex: string): Uint8Array {
    return new Uint8Array(hex.match(/.{1,2}/g)!.map(byte => parseInt(byte, 16)));
}

// 1. Key Derivation (Master Key from Password)
// Returns the CryptoKey (AES-KW) derived from password
export async function deriveMasterKey(password: string, saltHex: string): Promise<CryptoKey> {
    const enc = new TextEncoder();
    const keyMaterial = await window.crypto.subtle.importKey(
        "raw",
        enc.encode(password),
        { name: "PBKDF2" },
        false,
        ["deriveKey"]
    );

    const salt = hexToBuff(saltHex);

    return window.crypto.subtle.deriveKey(
        {
            name: "PBKDF2",
            salt: salt as any,
            iterations: PBKDF2_ITERATIONS,
            hash: "SHA-256"
        },
        keyMaterial,
        { name: "AES-KW", length: 256 },
        true, // Extractable? Maybe not. We just use it to Unwrap.
        ["wrapKey", "unwrapKey"]
    );
}

// 2. Generate Device Key
export async function generateDeviceKey(): Promise<CryptoKey> {
    return window.crypto.subtle.generateKey(
        {
            name: "AES-GCM",
            length: 256
        },
        true,
        ["encrypt", "decrypt"]
    );
}

// 3. Wrap Device Key (Encrypt it with Master Key)
export async function wrapDeviceKey(deviceKey: CryptoKey, masterKey: CryptoKey): Promise<string> {
    const wrapped = await window.crypto.subtle.wrapKey(
        "raw",
        deviceKey,
        masterKey,
        "AES-KW"
    );
    return arrayBufferToBase64(wrapped);
}

// 4. Unwrap Device Key
export async function unwrapDeviceKey(wrappedBlobBase64: string, masterKey: CryptoKey): Promise<CryptoKey> {
    const wrapped = base64ToArrayBuffer(wrappedBlobBase64);
    return window.crypto.subtle.unwrapKey(
        "raw",
        wrapped,
        masterKey,
        "AES-KW",
        { name: "AES-GCM" },
        true,
        ["encrypt", "decrypt"]
    );
}

// 5. Decrypt Payload
export async function decryptPayload(
    ciphertextBase64: string,
    // IV is usually prepended to ciphertext or separate.
    // Agent implementation `EncryptPayload` returns `nonce + ciphertext`.
    // So we need to slice it.
    deviceKey: CryptoKey
): Promise<ArrayBuffer> {
    const raw = base64ToArrayBuffer(ciphertextBase64);
    const iv = raw.slice(0, IV_LENGTH);
    const data = raw.slice(IV_LENGTH);

    return window.crypto.subtle.decrypt(
        {
            name: "AES-GCM",
            iv: iv
        },
        deviceKey,
        data
    );
}

// Helper: Decrypt JSON
export async function decryptJSON(ciphertextBase64: string, deviceKey: CryptoKey): Promise<any> {
    const decrypted = await decryptPayload(ciphertextBase64, deviceKey);
    const dec = new TextDecoder();
    const jsonStr = dec.decode(decrypted);
    return JSON.parse(jsonStr);
}

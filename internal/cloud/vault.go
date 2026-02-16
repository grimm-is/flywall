// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package cloud

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// DeviceKeySize is the size of the symmetric key used for telemetry encryption (AES-256)
const DeviceKeySize = 32

// GenerateDeviceKey creates a new random symmetric key for this device.
func GenerateDeviceKey() ([]byte, error) {
	key := make([]byte, DeviceKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate device key: %w", err)
	}
	return key, nil
}

// WrapKey encrypts a key (the target) with another key (the wrapper) using AES-GCM.
// This is used by the agent if it needs to wrap its own key for local storage (optional),
// but primarily the browser does this. We include it here for completeness and potential
// use in "break-glass" local recovery scenarios.
func WrapKey(wrapperKey, targetKey []byte) ([]byte, error) {
	block, err := aes.NewCipher(wrapperKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, targetKey, nil), nil
}

// UnwrapKey decrypts a wrapped key.
func UnwrapKey(wrapperKey, wrappedBlob []byte) ([]byte, error) {
	block, err := aes.NewCipher(wrapperKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(wrappedBlob) < nonceSize {
		return nil, fmt.Errorf("wrapped blob too short")
	}

	nonce, ciphertext := wrappedBlob[:nonceSize], wrappedBlob[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

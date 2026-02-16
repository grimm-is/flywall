// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package state

import (
	"testing"
)

func TestGenerateNonce(t *testing.T) {
	nonce1, err := generateNonce()
	if err != nil {
		t.Fatal(err)
	}
	if len(nonce1) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("Expected nonce length 64, got %d", len(nonce1))
	}

	// Nonces should be unique
	nonce2, _ := generateNonce()
	if nonce1 == nonce2 {
		t.Error("Nonces should be unique")
	}
}

func TestComputeAndVerifyMAC(t *testing.T) {
	secretKey := []byte("test-secret-key")
	nonce := "test-nonce-12345"

	mac := computeMAC(nonce, secretKey)
	if mac == "" {
		t.Error("MAC should not be empty")
	}

	// Verify should pass with correct key
	if !verifyMAC(nonce, mac, secretKey) {
		t.Error("MAC verification should pass with correct key")
	}

	// Verify should fail with wrong key
	if verifyMAC(nonce, mac, []byte("wrong-key")) {
		t.Error("MAC verification should fail with wrong key")
	}

	// Verify should fail with wrong nonce
	if verifyMAC("wrong-nonce", mac, secretKey) {
		t.Error("MAC verification should fail with wrong nonce")
	}
}

func TestComputeAndVerifyChecksum(t *testing.T) {
	data := []byte("test data for checksum")

	checksum := computeChecksum(data)
	if checksum == "" {
		t.Error("Checksum should not be empty")
	}

	// Verify should pass with correct data
	if !verifyChecksum(data, checksum) {
		t.Error("Checksum verification should pass with correct data")
	}

	// Verify should fail with modified data
	if verifyChecksum([]byte("modified data"), checksum) {
		t.Error("Checksum verification should fail with modified data")
	}
}

func TestSecurityConfigFromReplicationConfig(t *testing.T) {
	cfg := ReplicationConfig{
		SecretKey:   "my-secret",
		TLSCertFile: "/path/to/cert.pem",
		TLSKeyFile:  "/path/to/key.pem",
		TLSCAFile:   "/path/to/ca.pem",
		TLSMutual:   true,
	}

	secCfg := cfg.securityConfig()
	if secCfg.SecretKey != "my-secret" {
		t.Error("SecretKey not copied")
	}
	if secCfg.TLSCertFile != "/path/to/cert.pem" {
		t.Error("TLSCertFile not copied")
	}
	if secCfg.TLSMutual != true {
		t.Error("TLSMutual not copied")
	}
}

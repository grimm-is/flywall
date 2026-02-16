// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package learning

import (
	"encoding/hex"
	"testing"
)

func TestIsGREASE(t *testing.T) {
	tests := []struct {
		val  uint16
		want bool
	}{
		{0x0a0a, true},
		{0x1a1a, true},
		{0x2a2a, true},
		{0x3a3a, true},
		{0x4a4a, true},
		{0x5a5a, true},
		{0x6a6a, true},
		{0x7a7a, true},
		{0x8a8a, true},
		{0x9a9a, true},
		{0xaaaa, true},
		{0xbaba, true},
		{0xcaca, true},
		{0xdada, true},
		{0xeaea, true},
		{0xfafa, true},
		{0x0000, false},
		{0x0001, false},
		{0xc02b, false}, // TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
		{0x1301, false}, // TLS_AES_128_GCM_SHA256
		{0x0a0b, false}, // Not GREASE (different low nibbles)
		{0x1a2a, false}, // Not GREASE (different high nibbles)
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := isGREASE(tt.val); got != tt.want {
				t.Errorf("isGREASE(0x%04x) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

func TestParseJA3(t *testing.T) {
	// Construct a minimal TLS 1.2 Client Hello packet for testing
	// Extension calculations:
	//   supported_groups (0x000a): type(2) + len(2) + list_len(2) + 3*2 = 12 bytes
	//   ec_point_formats (0x000b): type(2) + len(2) + list_len(1) + 1 = 6 bytes
	//   SNI (0x0000): type(2) + len(2) + list_len(2) + type(1) + name_len(2) + 4 = 13 bytes
	// Total extensions: 12 + 6 + 13 = 31 = 0x001f

	// Build the packet hex string
	packetHex := "" +
		// Record Header
		"160301" + // Content type (handshake), version
		"0076" + // Record length (118 bytes = 4 + 2 + 32 + 1 + 8 + 2 + 1 + 1 + 2 + 31 + 34 = adjusted)
		// Handshake Header
		"01" + // Client Hello
		"000072" + // Handshake length
		// Client Version
		"0303" + // TLS 1.2 (0x0303 = 771)
		// Random (32 bytes)
		"0000000000000000000000000000000000000000000000000000000000000000" +
		// Session ID Length
		"00" +
		// Cipher Suites (with one GREASE value that should be filtered)
		"0008" + // Length: 8 bytes (4 ciphers)
		"0a0a" + // GREASE (should be filtered)
		"c02b" + // TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256 (49195)
		"c02f" + // TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 (49199)
		"009e" + // TLS_DHE_RSA_WITH_AES_128_GCM_SHA256 (158)
		// Compression Methods
		"01" + // Length: 1
		"00" + // null compression
		// Extensions - corrected length
		"001f" + // Extensions length: 31 bytes (12 + 6 + 13)
		// Extension: supported_groups (0x000a) - 12 bytes total
		"000a" + // Type: supported_groups
		"0008" + // Length: 8 bytes
		"0006" + // List length: 6 bytes (3 curves)
		"001d" + // x25519 (29)
		"0017" + // secp256r1 (23)
		"0018" + // secp384r1 (24)
		// Extension: ec_point_formats (0x000b) - 6 bytes total
		"000b" + // Type: ec_point_formats
		"0002" + // Length: 2 bytes
		"01" + // List length: 1
		"00" + // uncompressed (0)
		// Extension: SNI (0x0000) - 13 bytes total
		"0000" + // Type: SNI
		"0009" + // Length: 9 bytes
		"0007" + // Server name list length: 7
		"00" + // Name type: host_name
		"0004" + // Name length: 4
		"74657374" // "test"

	packet, err := hex.DecodeString(packetHex)
	if err != nil {
		t.Fatalf("Failed to decode test packet: %v", err)
	}

	result, err := ParseJA3(packet)
	if err != nil {
		t.Fatalf("ParseJA3 returned error: %v", err)
	}
	if result == nil {
		t.Fatal("ParseJA3 returned nil result")
	}

	// Expected JA3 string:
	// Version: 771 (0x0303)
	// Ciphers: 49195-49199-158 (GREASE filtered out)
	// Extensions: 10-11-0 (supported_groups, ec_point_formats, SNI)
	// Curves: 29-23-24
	// EC Point Formats: 0
	expectedRaw := "771,49195-49199-158,10-11-0,29-23-24,0"

	t.Logf("JA3 Raw: %s", result.Raw)
	t.Logf("JA3 Hash: %s", result.Hash)

	if result.Raw != expectedRaw {
		t.Errorf("JA3 raw mismatch:\ngot:  %s\nwant: %s", result.Raw, expectedRaw)
	}

	// The hash should be MD5 of the raw string
	if len(result.Hash) != 32 {
		t.Errorf("JA3 hash length = %d, want 32", len(result.Hash))
	}
}

func TestParseJA3_NotHandshake(t *testing.T) {
	// Not a handshake packet (wrong content type)
	packet := []byte{0x17, 0x03, 0x01, 0x00, 0x10}
	result, err := ParseJA3(packet)
	if err != nil {
		t.Fatalf("ParseJA3 returned error: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result for non-handshake packet")
	}
}

func TestParseJA3_NotClientHello(t *testing.T) {
	// Server Hello (handshake type 0x02)
	packet := []byte{0x16, 0x03, 0x01, 0x00, 0x10, 0x02}
	result, err := ParseJA3(packet)
	if err != nil {
		t.Fatalf("ParseJA3 returned error: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result for Server Hello packet")
	}
}

func TestParseJA3_ShortPacket(t *testing.T) {
	// Too short to be valid
	packet := []byte{0x16, 0x03, 0x01}
	result, err := ParseJA3(packet)
	if err != nil {
		t.Fatalf("ParseJA3 returned error: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result for short packet")
	}
}

func TestParseJA3_NoExtensions(t *testing.T) {
	// Client Hello with no extensions
	packetHex := "" +
		"160301" + // Record header
		"002d" + // Record length
		"01" + // Client Hello
		"000029" + // Handshake length
		"0303" + // TLS 1.2
		"0000000000000000000000000000000000000000000000000000000000000000" + // Random
		"00" + // Session ID length
		"0004" + // Cipher suites length
		"c02b" + // Cipher 1
		"c02f" + // Cipher 2
		"01" + // Compression methods length
		"00" + // null compression
		"0000" // Extensions length: 0

	packet, err := hex.DecodeString(packetHex)
	if err != nil {
		t.Fatalf("Failed to decode test packet: %v", err)
	}

	result, err := ParseJA3(packet)
	if err != nil {
		t.Fatalf("ParseJA3 returned error: %v", err)
	}
	if result == nil {
		t.Fatal("ParseJA3 returned nil result")
	}

	// Expected: version,ciphers,extensions,curves,formats
	// No extensions means empty extensions, curves, and formats
	expectedRaw := "771,49195-49199,,,"

	if result.Raw != expectedRaw {
		t.Errorf("JA3 raw mismatch:\ngot:  %s\nwant: %s", result.Raw, expectedRaw)
	}
}

func TestBuildJA3String(t *testing.T) {
	tests := []struct {
		name   string
		fields ja3Fields
		want   string
	}{
		{
			name: "Full fields",
			fields: ja3Fields{
				version:        771,
				cipherSuites:   []uint16{49195, 49199, 158},
				extensions:     []uint16{0, 10, 11},
				ellipticCurves: []uint16{29, 23, 24},
				ecPointFormats: []uint8{0},
			},
			want: "771,49195-49199-158,0-10-11,29-23-24,0",
		},
		{
			name: "Empty lists",
			fields: ja3Fields{
				version:        771,
				cipherSuites:   nil,
				extensions:     nil,
				ellipticCurves: nil,
				ecPointFormats: nil,
			},
			want: "771,,,,",
		},
		{
			name: "Single values",
			fields: ja3Fields{
				version:        769, // TLS 1.0
				cipherSuites:   []uint16{47},
				extensions:     []uint16{0},
				ellipticCurves: []uint16{23},
				ecPointFormats: []uint8{0, 1, 2},
			},
			want: "769,47,0,23,0-1-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fields.buildJA3String()
			if got != tt.want {
				t.Errorf("buildJA3String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- JA3S Tests ---

func TestParseJA3S(t *testing.T) {
	// Construct a minimal TLS 1.2 Server Hello packet for testing
	// Structure:
	// Record Header (5 bytes): 16 03 03 <len>
	// Handshake Header (4 bytes): 02 <len>
	// Server Version (2 bytes): 03 03 (TLS 1.2)
	// Random (32 bytes): all zeros
	// Session ID Length (1 byte): 00
	// Cipher Suite (2 bytes): C0 2F (TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 = 49199)
	// Compression Method (1 byte): 00
	// Extensions Length (2 bytes): 00 05
	// Extension: renegotiation_info (0xff01): type(2) + len(2) + data(1) = 5 bytes

	packetHex := "" +
		// Record Header
		"160303" + // Content type (handshake), TLS 1.2
		"0032" + // Record length (50 bytes)
		// Handshake Header
		"02" + // Server Hello
		"00002e" + // Handshake length (46 bytes)
		// Server Version
		"0303" + // TLS 1.2 (0x0303 = 771)
		// Random (32 bytes)
		"0000000000000000000000000000000000000000000000000000000000000000" +
		// Session ID Length
		"00" +
		// Cipher Suite (single value)
		"c02f" + // TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 (49199)
		// Compression Method
		"00" + // null compression
		// Extensions
		"0005" + // Extensions length: 5 bytes
		// Extension: renegotiation_info (0xff01) - 5 bytes total
		"ff01" + // Type: renegotiation_info (65281)
		"0001" + // Length: 1 byte
		"00" // Data

	packet, err := hex.DecodeString(packetHex)
	if err != nil {
		t.Fatalf("Failed to decode test packet: %v", err)
	}

	result, err := ParseJA3S(packet)
	if err != nil {
		t.Fatalf("ParseJA3S returned error: %v", err)
	}
	if result == nil {
		t.Fatal("ParseJA3S returned nil result")
	}

	// Expected JA3S string:
	// Version: 771 (0x0303)
	// Cipher: 49199 (0xc02f)
	// Extensions: 65281 (0xff01)
	expectedRaw := "771,49199,65281"

	t.Logf("JA3S Raw: %s", result.Raw)
	t.Logf("JA3S Hash: %s", result.Hash)

	if result.Raw != expectedRaw {
		t.Errorf("JA3S raw mismatch:\ngot:  %s\nwant: %s", result.Raw, expectedRaw)
	}

	// The hash should be MD5 of the raw string
	if len(result.Hash) != 32 {
		t.Errorf("JA3S hash length = %d, want 32", len(result.Hash))
	}
}

func TestParseJA3S_NotServerHello(t *testing.T) {
	// Client Hello (handshake type 0x01) should return nil
	packet := []byte{0x16, 0x03, 0x01, 0x00, 0x30, 0x01}
	result, err := ParseJA3S(packet)
	if err != nil {
		t.Fatalf("ParseJA3S returned error: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result for Client Hello packet")
	}
}

func TestParseJA3S_NoExtensions(t *testing.T) {
	// Server Hello with no extensions (valid case)
	packetHex := "" +
		"160303" + // Record header
		"002a" + // Record length (42 bytes)
		"02" + // Server Hello
		"000026" + // Handshake length (38 bytes)
		"0303" + // TLS 1.2
		"0000000000000000000000000000000000000000000000000000000000000000" + // Random
		"00" + // Session ID length
		"c02f" + // Cipher suite
		"00" // Compression method
		// No extensions

	packet, err := hex.DecodeString(packetHex)
	if err != nil {
		t.Fatalf("Failed to decode test packet: %v", err)
	}

	result, err := ParseJA3S(packet)
	if err != nil {
		t.Fatalf("ParseJA3S returned error: %v", err)
	}
	if result == nil {
		t.Fatal("ParseJA3S returned nil result")
	}

	// No extensions means empty extension field
	expectedRaw := "771,49199,"

	if result.Raw != expectedRaw {
		t.Errorf("JA3S raw mismatch:\ngot:  %s\nwant: %s", result.Raw, expectedRaw)
	}
}

func TestBuildJA3SString(t *testing.T) {
	tests := []struct {
		name   string
		fields ja3sFields
		want   string
	}{
		{
			name: "With extensions",
			fields: ja3sFields{
				version:     771,
				cipherSuite: 49199,
				extensions:  []uint16{65281, 11, 35},
			},
			want: "771,49199,65281-11-35",
		},
		{
			name: "No extensions",
			fields: ja3sFields{
				version:     771,
				cipherSuite: 49199,
				extensions:  nil,
			},
			want: "771,49199,",
		},
		{
			name: "TLS 1.0",
			fields: ja3sFields{
				version:     769,
				cipherSuite: 47,
				extensions:  []uint16{0},
			},
			want: "769,47,0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fields.buildJA3SString()
			if got != tt.want {
				t.Errorf("buildJA3SString() = %q, want %q", got, tt.want)
			}
		})
	}
}

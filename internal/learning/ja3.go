// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package learning

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

// JA3Result contains the parsed JA3 fingerprint data
type JA3Result struct {
	Hash string // MD5 hash of the JA3 string (32 hex chars)
	Raw  string // Raw JA3 string before hashing
}

// ParseJA3 extracts JA3 fingerprint from a TLS Client Hello packet.
// The payload is expected to be the TCP payload (TLS record).
// Returns empty result if the packet is not a valid Client Hello.
func ParseJA3(payload []byte) (*JA3Result, error) {
	fields, err := parseJA3Fields(payload)
	if err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, nil
	}

	// Build JA3 string: version,ciphers,extensions,curves,formats
	raw := fields.buildJA3String()
	hash := fmt.Sprintf("%x", md5.Sum([]byte(raw)))

	return &JA3Result{
		Hash: hash,
		Raw:  raw,
	}, nil
}

// ja3Fields holds the parsed components for JA3 fingerprinting
type ja3Fields struct {
	version        uint16   // TLS version from Client Hello
	cipherSuites   []uint16 // Cipher suites (GREASE filtered)
	extensions     []uint16 // Extension types (GREASE filtered)
	ellipticCurves []uint16 // Supported groups/curves (GREASE filtered)
	ecPointFormats []uint8  // EC point format list
}

// buildJA3String constructs the JA3 string from parsed fields
func (f *ja3Fields) buildJA3String() string {
	// Convert version
	version := strconv.FormatUint(uint64(f.version), 10)

	// Convert cipher suites to hyphen-delimited string
	ciphers := uint16SliceToString(f.cipherSuites)

	// Convert extensions to hyphen-delimited string
	extensions := uint16SliceToString(f.extensions)

	// Convert elliptic curves to hyphen-delimited string
	curves := uint16SliceToString(f.ellipticCurves)

	// Convert EC point formats to hyphen-delimited string
	formats := uint8SliceToString(f.ecPointFormats)

	// Join with commas
	return strings.Join([]string{version, ciphers, extensions, curves, formats}, ",")
}

// uint16SliceToString converts a slice of uint16 to hyphen-delimited string
func uint16SliceToString(vals []uint16) string {
	if len(vals) == 0 {
		return ""
	}
	strs := make([]string, len(vals))
	for i, v := range vals {
		strs[i] = strconv.FormatUint(uint64(v), 10)
	}
	return strings.Join(strs, "-")
}

// uint8SliceToString converts a slice of uint8 to hyphen-delimited string
func uint8SliceToString(vals []uint8) string {
	if len(vals) == 0 {
		return ""
	}
	strs := make([]string, len(vals))
	for i, v := range vals {
		strs[i] = strconv.FormatUint(uint64(v), 10)
	}
	return strings.Join(strs, "-")
}

// isGREASE checks if a value is a GREASE placeholder that should be filtered.
// GREASE values follow the pattern 0x?a?a where ? is the same nibble.
// Reference: RFC 8701
func isGREASE(val uint16) bool {
	// GREASE values: 0x0a0a, 0x1a1a, 0x2a2a, 0x3a3a, 0x4a4a, 0x5a5a,
	//                0x6a6a, 0x7a7a, 0x8a8a, 0x9a9a, 0xaaaa, 0xbaba,
	//                0xcaca, 0xdada, 0xeaea, 0xfafa
	if val&0x0f0f != 0x0a0a {
		return false
	}
	hi := (val >> 8) & 0xf0
	lo := val & 0xf0
	return hi == lo
}

// parseJA3Fields extracts all fields needed for JA3 from a TLS Client Hello
func parseJA3Fields(payload []byte) (*ja3Fields, error) {
	if len(payload) < 43 { // Min size for valid Client Hello header
		return nil, nil
	}

	// TLS Record Header
	// Content Type: 0x16 (Handshake)
	if payload[0] != 0x16 {
		return nil, nil
	}

	// Skip Record Header (5 bytes) -> Handshake Header
	// Handshake Type: 0x01 (Client Hello)
	if payload[5] != 0x01 {
		return nil, nil
	}

	fields := &ja3Fields{}

	// Pointer arithmetic
	cursor := 5 + 4 // Skip Record(5) + HandshakeHeader(4)

	// Client Version (2 bytes) - this is what JA3 uses
	if cursor+2 > len(payload) {
		return nil, nil
	}
	fields.version = binary.BigEndian.Uint16(payload[cursor : cursor+2])
	cursor += 2

	// Skip Random (32 bytes)
	cursor += 32

	// Session ID Length
	if cursor >= len(payload) {
		return nil, nil
	}
	sessionIDLen := int(payload[cursor])
	cursor += 1 + sessionIDLen

	// Cipher Suites Length
	if cursor+2 > len(payload) {
		return nil, nil
	}
	cipherSuitesLen := int(binary.BigEndian.Uint16(payload[cursor : cursor+2]))
	cursor += 2

	// Parse Cipher Suites
	if cursor+cipherSuitesLen > len(payload) {
		return nil, nil
	}
	numCiphers := cipherSuitesLen / 2
	for i := 0; i < numCiphers; i++ {
		cs := binary.BigEndian.Uint16(payload[cursor+i*2 : cursor+i*2+2])
		if !isGREASE(cs) {
			fields.cipherSuites = append(fields.cipherSuites, cs)
		}
	}
	cursor += cipherSuitesLen

	// Compression Methods Length
	if cursor >= len(payload) {
		return nil, nil
	}
	compMethodsLen := int(payload[cursor])
	cursor += 1 + compMethodsLen

	// Extensions Length
	if cursor+2 > len(payload) {
		return nil, nil
	}
	extTotalLen := int(binary.BigEndian.Uint16(payload[cursor : cursor+2]))
	cursor += 2

	end := cursor + extTotalLen
	if end > len(payload) {
		return nil, nil
	}

	// Loop through Extensions
	for cursor < end {
		if cursor+4 > end {
			break
		}
		extType := binary.BigEndian.Uint16(payload[cursor : cursor+2])
		extLen := int(binary.BigEndian.Uint16(payload[cursor+2 : cursor+4]))
		cursor += 4

		if cursor+extLen > end {
			break
		}

		// Add extension type to list (filtered for GREASE)
		if !isGREASE(extType) {
			fields.extensions = append(fields.extensions, extType)
		}

		// Parse specific extensions for additional JA3 data
		extData := payload[cursor : cursor+extLen]

		switch extType {
		case 0x000a: // supported_groups (elliptic_curves)
			fields.ellipticCurves = parseEllipticCurves(extData)
		case 0x000b: // ec_point_formats
			fields.ecPointFormats = parseECPointFormats(extData)
		}

		cursor += extLen
	}

	return fields, nil
}

// parseEllipticCurves parses the supported_groups extension (0x000a)
func parseEllipticCurves(data []byte) []uint16 {
	if len(data) < 2 {
		return nil
	}
	listLen := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+listLen {
		return nil
	}

	var curves []uint16
	numCurves := listLen / 2
	for i := 0; i < numCurves; i++ {
		curve := binary.BigEndian.Uint16(data[2+i*2 : 2+i*2+2])
		if !isGREASE(curve) {
			curves = append(curves, curve)
		}
	}
	return curves
}

// parseECPointFormats parses the ec_point_formats extension (0x000b)
func parseECPointFormats(data []byte) []uint8 {
	if len(data) < 1 {
		return nil
	}
	listLen := int(data[0])
	if len(data) < 1+listLen {
		return nil
	}

	formats := make([]uint8, listLen)
	copy(formats, data[1:1+listLen])
	return formats
}

// --- JA3S (Server Fingerprint) ---

// JA3SResult contains the parsed JA3S fingerprint data
type JA3SResult struct {
	Hash string // MD5 hash of the JA3S string (32 hex chars)
	Raw  string // Raw JA3S string before hashing
}

// ParseJA3S extracts JA3S fingerprint from a TLS Server Hello packet.
// The payload is expected to be the TCP payload (TLS record).
// Returns empty result if the packet is not a valid Server Hello.
// JA3S format: version,cipher,extensions
func ParseJA3S(payload []byte) (*JA3SResult, error) {
	fields, err := parseJA3SFields(payload)
	if err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, nil
	}

	// Build JA3S string: version,cipher,extensions
	raw := fields.buildJA3SString()
	hash := fmt.Sprintf("%x", md5.Sum([]byte(raw)))

	return &JA3SResult{
		Hash: hash,
		Raw:  raw,
	}, nil
}

// ja3sFields holds the parsed components for JA3S fingerprinting
type ja3sFields struct {
	version     uint16   // TLS version from Server Hello
	cipherSuite uint16   // Selected cipher suite (single value)
	extensions  []uint16 // Extension types (GREASE filtered)
}

// buildJA3SString constructs the JA3S string from parsed fields
func (f *ja3sFields) buildJA3SString() string {
	// Convert version
	version := strconv.FormatUint(uint64(f.version), 10)

	// Convert cipher suite (single value)
	cipher := strconv.FormatUint(uint64(f.cipherSuite), 10)

	// Convert extensions to hyphen-delimited string
	extensions := uint16SliceToString(f.extensions)

	// Join with commas
	return strings.Join([]string{version, cipher, extensions}, ",")
}

// parseJA3SFields extracts all fields needed for JA3S from a TLS Server Hello
func parseJA3SFields(payload []byte) (*ja3sFields, error) {
	if len(payload) < 43 { // Min size for valid Server Hello header
		return nil, nil
	}

	// TLS Record Header
	// Content Type: 0x16 (Handshake)
	if payload[0] != 0x16 {
		return nil, nil
	}

	// Skip Record Header (5 bytes) -> Handshake Header
	// Handshake Type: 0x02 (Server Hello)
	if payload[5] != 0x02 {
		return nil, nil
	}

	fields := &ja3sFields{}

	// Pointer arithmetic
	cursor := 5 + 4 // Skip Record(5) + HandshakeHeader(4)

	// Server Version (2 bytes)
	if cursor+2 > len(payload) {
		return nil, nil
	}
	fields.version = binary.BigEndian.Uint16(payload[cursor : cursor+2])
	cursor += 2

	// Skip Random (32 bytes)
	cursor += 32

	// Session ID Length
	if cursor >= len(payload) {
		return nil, nil
	}
	sessionIDLen := int(payload[cursor])
	cursor += 1 + sessionIDLen

	// Cipher Suite (2 bytes - single value, not a list like in Client Hello)
	if cursor+2 > len(payload) {
		return nil, nil
	}
	fields.cipherSuite = binary.BigEndian.Uint16(payload[cursor : cursor+2])
	cursor += 2

	// Compression Method (1 byte)
	if cursor >= len(payload) {
		return nil, nil
	}
	cursor += 1

	// Check if there are extensions (they're optional in Server Hello)
	if cursor+2 > len(payload) {
		// No extensions - that's valid for Server Hello
		return fields, nil
	}

	// Extensions Length
	extTotalLen := int(binary.BigEndian.Uint16(payload[cursor : cursor+2]))
	cursor += 2

	end := cursor + extTotalLen
	if end > len(payload) {
		return nil, nil
	}

	// Loop through Extensions
	for cursor < end {
		if cursor+4 > end {
			break
		}
		extType := binary.BigEndian.Uint16(payload[cursor : cursor+2])
		extLen := int(binary.BigEndian.Uint16(payload[cursor+2 : cursor+4]))
		cursor += 4

		if cursor+extLen > end {
			break
		}

		// Add extension type to list (filtered for GREASE)
		if !isGREASE(extType) {
			fields.extensions = append(fields.extensions, extType)
		}

		cursor += extLen
	}

	return fields, nil
}

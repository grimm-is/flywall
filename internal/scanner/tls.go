// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package scanner

import (
	"encoding/hex"

	"github.com/dreadl0ck/ja3"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

// ExtractTLS analyzes a packet for JA3 and SNI fingerprints
func ExtractTLS(packet gopacket.Packet, record *DeviceFingerprint) {
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		return
	}

	// Check for Client Hello (Handshake Type 1)
	// TLS Record Protocol (0x16) + Version (0x0301/02/03) + Length (2 bytes) + Handshake (0x01)
	tcp, _ := tcpLayer.(*layers.TCP)
	payload := tcp.Payload
	if len(payload) < 6 {
		return
	}

	// Simple check for TLS Handshake content type
	if payload[0] != 0x16 {
		return
	}
	// Check for ClientHello handshake type
	if payload[5] != 0x01 {
		return
	}

	// JA3 Calculation
	// ja3.DigestPacket returns the raw MD5 digest
	digest := ja3.DigestPacket(packet)
	// Check for empty digest (matches empty MD5 128 bit)
	// d41d8cd98f00b204e9800998ecf8427e is md5("")

	// Convert to hex
	ja3Hash := hex.EncodeToString(digest[:])

	if ja3Hash != "d41d8cd98f00b204e9800998ecf8427e" && ja3Hash != "00000000000000000000000000000000" {
		record.AddTLS(ja3Hash, "")
	}

	// SNI Extraction
	// For this MVP, we rely on the fact that JA3 hash usually implies a unique client implementation.
	// Extracting SNI robustly requires a full TLS parser (like cryptobyte or golang.org/x/crypto/cryptobyte)
	// or `dreadl0ck/tlsx`.
	// For now, we omit SNI to keep dependencies minimal as per user preference,
	// unless we want to do a basic string extract.
}

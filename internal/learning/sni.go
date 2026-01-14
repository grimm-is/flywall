package learning

import (
	"encoding/binary"
	"errors"
)

// TLSClientHelloInfo contains extracted data from a TLS Client Hello packet
type TLSClientHelloInfo struct {
	SNI string     // Server Name Indication (hostname)
	JA3 *JA3Result // JA3 fingerprint (hash and raw string)
}

// TLSServerHelloInfo contains extracted data from a TLS Server Hello packet
type TLSServerHelloInfo struct {
	JA3S *JA3SResult // JA3S fingerprint (hash and raw string)
}

// ParseTLSClientHello extracts both SNI and JA3 fingerprint from a TLS Client Hello packet.
// This is more efficient than calling ParseSNI and ParseJA3 separately.
// The payload is expected to be the TCP payload (TLS record).
func ParseTLSClientHello(payload []byte) (*TLSClientHelloInfo, error) {
	// Parse SNI
	sni, sniErr := ParseSNI(payload)

	// Parse JA3 (independent of SNI parsing success)
	ja3, ja3Err := ParseJA3(payload)

	// If both failed, return nil
	if sniErr != nil && ja3Err != nil {
		return nil, sniErr
	}
	if sni == "" && ja3 == nil {
		return nil, nil
	}

	return &TLSClientHelloInfo{
		SNI: sni,
		JA3: ja3,
	}, nil
}

// ParseTLSServerHello extracts JA3S fingerprint from a TLS Server Hello packet.
// The payload is expected to be the TCP payload (TLS record).
func ParseTLSServerHello(payload []byte) (*TLSServerHelloInfo, error) {
	ja3s, err := ParseJA3S(payload)
	if err != nil {
		return nil, err
	}
	if ja3s == nil {
		return nil, nil
	}

	return &TLSServerHelloInfo{
		JA3S: ja3s,
	}, nil
}

// ParseSNI attempts to extract the Server Name Indication from a TLS Client Hello packet.
// It returns "" if no SNI is found or the packet is not a Client Hello.
// The payload is expected to be the TCP payload (TLS record).
func ParseSNI(payload []byte) (string, error) {
	if len(payload) < 43 { // Min size for a valid Client Hello header
		return "", nil
	}

	// TLS Record Header
	// Content Type: 0x16 (Handshake)
	if payload[0] != 0x16 {
		return "", nil
	}

	// Skip Record Header (5 bytes) -> Handshake Header
	// Handshake Type: 0x01 (Client Hello)
	if payload[5] != 0x01 {
		return "", nil
	}

	// Pointer arithmetic helpers
	cursor := 5 + 4 // Skip Record(5) + HandshakeHeader(4)

	// Skip Protocol Version (2) + Random (32)
	cursor += 34

	// Session ID Length
	if cursor >= len(payload) {
		return "", nil
	}
	sessionIDLen := int(payload[cursor])
	cursor += 1 + sessionIDLen

	// Cipher Suites Length
	if cursor+1 >= len(payload) {
		return "", nil
	}
	cipherSuitesLen := int(binary.BigEndian.Uint16(payload[cursor : cursor+2]))
	cursor += 2 + cipherSuitesLen

	// Compression Methods Length
	if cursor >= len(payload) {
		return "", nil
	}
	compMethodsLen := int(payload[cursor])
	cursor += 1 + compMethodsLen

	// Extensions Length
	if cursor+1 >= len(payload) {
		return "", nil
	}
	extTotalLen := int(binary.BigEndian.Uint16(payload[cursor : cursor+2]))
	cursor += 2

	end := cursor + extTotalLen
	if end > len(payload) {
		return "", errors.New("incomplete packet")
	}

	// Loop through Extensions
	for cursor < end {
		if cursor+4 > end {
			break
		}
		extType := binary.BigEndian.Uint16(payload[cursor : cursor+2])
		extLen := int(binary.BigEndian.Uint16(payload[cursor+2 : cursor+4]))
		cursor += 4

		if extType == 0x0000 { // Server Name Extension
			// Parse SNI
			if cursor+2 > end {
				break
			}
			sniCursor := cursor + 2

			if sniCursor+3 > end {
				break
			}
			nameType := payload[sniCursor] // Should be 0 (host_name)
			nameLen := int(binary.BigEndian.Uint16(payload[sniCursor+1 : sniCursor+3]))
			sniCursor += 3

			if nameType == 0 && sniCursor+nameLen <= end {
				return string(payload[sniCursor : sniCursor+nameLen]), nil
			}
		}
		cursor += extLen
	}

	return "", nil
}

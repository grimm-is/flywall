package scanner

import (
	"encoding/hex"
	"fmt"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

// ExtractDHCP analyzes a packet for DHCPv4/v6 fingerprints
func ExtractDHCP(packet gopacket.Packet, record *DeviceFingerprint) {
	// DHCPv4 (UDP 67/68)
	if layer := packet.Layer(layers.LayerTypeDHCPv4); layer != nil {
		if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
			udp, _ := udpLayer.(*layers.UDP)
			
			// Try parsing as DHCPv4
			msg, err := dhcpv4.FromBytes(udp.Payload)
			if err == nil && msg.MessageType() == dhcpv4.MessageTypeRequest {
				// Option 55: Parameter Request List
				if prl := msg.ParameterRequestList(); prl != nil {
					// Convert OptionCodeList ([]uint8) to []byte
					bytes := make([]byte, len(prl))
					for i, b := range prl {
						bytes[i] = b.Code()
					}
					record.DHCPv4Params = hex.EncodeToString(bytes)
				}
				
				// Option 60: Vendor Class Identifier
				if vci := msg.ClassIdentifier(); vci != "" {
					record.DHCPv4Vendor = vci
				}
			}
		}
	} else {
		// DHCPv6 (UDP 546/547)
		if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
			udp, _ := udpLayer.(*layers.UDP)
			srcPort := int(udp.SrcPort)
			dstPort := int(udp.DstPort)

			if (srcPort == 546 && dstPort == 547) || (srcPort == 547 && dstPort == 546) {
				msg, err := dhcpv6.FromBytes(udp.Payload)
				if err == nil {
					// We only care about client messages (Solicit, Request, etc.)
					if msg.Type() == dhcpv6.MessageTypeSolicit || msg.Type() == dhcpv6.MessageTypeRequest {
						// Option 6: Option Request Option (ORO)
						if oro := msg.GetOneOption(dhcpv6.OptionORO); oro != nil {
							record.DHCPv6Options = hex.EncodeToString(oro.ToBytes())
						}

						// Option 16: Vendor Class
						if vc := msg.GetOneOption(dhcpv6.OptionVendorClass); vc != nil {
							record.DHCPv6Vendor = fmt.Sprintf("%x", vc.ToBytes())
						}
					}
				}
			}
		}
	}
}

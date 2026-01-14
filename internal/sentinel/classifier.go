package sentinel

// PacketMetadata contains features needed for classification
type PacketMetadata struct {
	SrcMAC     string
	SrcIP      string
	DstIP      string
	DstPort    int
	Protocol   string // "TCP", "UDP", "ICMP"
	PayloadLen int
}

// Classifier implements a rule-based decision tree
type Classifier struct{}

// NewClassifier creates a new traffic classifier
func NewClassifier() *Classifier {
	return &Classifier{}
}

// TrafficClass constants
const (
	ClassWeb            = "Web"
	ClassStreaming      = "Streaming"
	ClassGaming         = "Gaming"
	ClassVoIP           = "VoIP"
	ClassFileTransfer   = "File Transfer"
	ClassInfrastructure = "Infrastructure"
	ClassUnknown        = "Unknown"
)

// Classify determines the traffic class based on packet metadata
func (c *Classifier) Classify(pkt PacketMetadata) string {
	// 1. Infrastructure Checks (DNS, NTP, DHCP)
	if pkt.DstPort == 53 || pkt.DstPort == 67 || pkt.DstPort == 68 || pkt.DstPort == 123 {
		return ClassInfrastructure
	}

	// 2. Web Traffic (HTTP/HTTPS)
	if pkt.Protocol == "TCP" && (pkt.DstPort == 80 || pkt.DstPort == 443 || pkt.DstPort == 8080) {
		return ClassWeb
	}

	// 3. File Transfer (SSH, FTP)
	if pkt.Protocol == "TCP" && (pkt.DstPort == 22 || pkt.DstPort == 21) {
		return ClassFileTransfer
	}

	// 4. Gaming / VoIP / Streaming Heuristics (UDP High Ports)
	if pkt.Protocol == "UDP" && pkt.DstPort > 1024 {
		// Heuristic: Small UDP packets at high frequency often Gaming or VoIP
		// Large UDP packets often Streaming (QUIC)

		if pkt.PayloadLen > 1000 {
			// Large payload -> Likely Streaming (QUIC/Media)
			return ClassStreaming
		}

		if pkt.PayloadLen < 200 {
			// Small payload -> Likely Gaming or VoIP
			// Optimistically label as Gaming for now, need flow stats for better accuracy
			return ClassGaming
		}
	}

	// 5. Default
	return ClassUnknown
}

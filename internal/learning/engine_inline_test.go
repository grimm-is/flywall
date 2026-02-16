// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package learning

import (
	"testing"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/learning/flowdb"
	"grimm.is/flywall/internal/logging"
)

func TestProcessPacketInline(t *testing.T) {
	// Create in-memory database
	db, err := flowdb.Open(":memory:", nil)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create engine with test config
	cfg := &config.RuleLearningConfig{
		LearningMode: true,
		PacketWindow: 3, // Small window for testing
		OffloadMark:  "0x200000",
	}

	engine := &Engine{
		config:          cfg,
		db:              db,
		flowCache:       NewFlowCache(100),
		dnsCache:        NewDNSSnoopCache(logging.Default(), 100),
		learningMode:    true,
		logger:          logging.Default(),
		portScanTracker: make(map[string]*portScanState),
	}

	// Test packet
	pkt := &PacketInfo{
		SrcMAC:    "aa:bb:cc:dd:ee:ff",
		SrcIP:     "192.168.1.100",
		DstIP:     "8.8.8.8",
		DstPort:   443,
		Protocol:  "TCP",
		Interface: "eth0",
	}

	// First packet should be allowed (learning mode)
	verdict, err := engine.ProcessPacketInline(pkt)
	if err != nil {
		t.Fatalf("ProcessPacketInline failed: %v", err)
	}
	if verdict != VerdictAllow {
		t.Errorf("Expected VerdictAllow for first packet, got %v", verdict)
	}

	// Send packets up to the window limit
	for i := 1; i < cfg.PacketWindow; i++ {
		verdict, err = engine.ProcessPacketInline(pkt)
		if err != nil {
			t.Fatalf("ProcessPacketInline failed on packet %d: %v", i+1, err)
		}
		if verdict != VerdictAllow {
			t.Errorf("Expected VerdictAllow for packet %d, got %v", i+1, verdict)
		}
	}

	// Next packet should trigger offload
	verdict, err = engine.ProcessPacketInline(pkt)
	if err != nil {
		t.Fatalf("ProcessPacketInline failed on offload packet: %v", err)
	}
	if verdict != VerdictOffload {
		t.Errorf("Expected VerdictOffload after packet window, got %v", verdict)
	}

	// Another packet for offloaded flow should still be offloaded
	verdict, err = engine.ProcessPacketInline(pkt)
	if err != nil {
		t.Fatalf("ProcessPacketInline failed on second offload packet: %v", err)
	}
	if verdict != VerdictOffload {
		t.Errorf("Expected VerdictOffload for second offload packet, got %v", verdict)
	}
}

func TestProcessPacketInlineDeniedFlow(t *testing.T) {
	// Create in-memory database
	db, err := flowdb.Open(":memory:", nil)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	cfg := &config.RuleLearningConfig{
		LearningMode: false, // Learning mode off
		PacketWindow: 3,
		OffloadMark:  "0x200000",
	}

	engine := &Engine{
		config:          cfg,
		db:              db,
		flowCache:       NewFlowCache(100),
		dnsCache:        NewDNSSnoopCache(logging.Default(), 100),
		learningMode:    false,
		logger:          logging.Default(),
		portScanTracker: make(map[string]*portScanState),
	}

	pkt := &PacketInfo{
		SrcMAC:    "aa:bb:cc:dd:ee:ff",
		SrcIP:     "192.168.1.100",
		DstIP:     "8.8.8.8",
		DstPort:   443,
		Protocol:  "TCP",
		Interface: "eth0",
	}

	// First packet should be inspect (pending flow)
	verdict, err := engine.ProcessPacketInline(pkt)
	if err != nil {
		t.Fatalf("ProcessPacketInline failed: %v", err)
	}
	if verdict != VerdictInspect {
		t.Errorf("Expected VerdictInspect for first packet, got %v", verdict)
	}

	// Manually deny the flow
	flows, err := db.ListFlows(flowdb.ListOptions{Limit: 1})
	if err != nil || len(flows) == 0 {
		t.Fatalf("Failed to get created flow")
	}
	flowID := flows[0].ID

	err = db.UpdateState(flowID, flowdb.StateDenied)
	if err != nil {
		t.Fatalf("Failed to deny flow: %v", err)
	}

	// Clear cache to force DB read
	engine.flowCache = NewFlowCache(100)

	// Next packet should be dropped
	verdict, err = engine.ProcessPacketInline(pkt)
	if err != nil {
		t.Fatalf("ProcessPacketInline failed after deny: %v", err)
	}
	if verdict != VerdictDrop {
		t.Errorf("Expected VerdictDrop for denied flow, got %v", verdict)
	}
}

func TestProcessPacketInlineDefaults(t *testing.T) {
	// Test with nil config to ensure defaults work
	db, err := flowdb.Open(":memory:", nil)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	engine := &Engine{
		config:          nil, // No config
		db:              db,
		flowCache:       NewFlowCache(100),
		dnsCache:        NewDNSSnoopCache(logging.Default(), 100),
		learningMode:    true,
		logger:          logging.Default(),
		portScanTracker: make(map[string]*portScanState),
	}

	pkt := &PacketInfo{
		SrcMAC:    "aa:bb:cc:dd:ee:ff",
		SrcIP:     "192.168.1.100",
		DstIP:     "8.8.8.8",
		DstPort:   443,
		Protocol:  "TCP",
		Interface: "eth0",
	}

	// Send 11 packets (default window is 10)
	for i := 0; i < 11; i++ {
		verdict, err := engine.ProcessPacketInline(pkt)
		if err != nil {
			t.Fatalf("ProcessPacketInline failed on packet %d: %v", i+1, err)
		}
		if i < 10 {
			if verdict != VerdictAllow {
				t.Errorf("Expected VerdictAllow for packet %d, got %v", i+1, verdict)
			}
		} else {
			if verdict != VerdictOffload {
				t.Errorf("Expected VerdictOffload for packet %d, got %v", i+1, verdict)
			}
		}
	}
}

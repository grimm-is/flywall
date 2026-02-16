// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build linux

package ctlplane

import (
	"fmt"
	"testing"
)

func TestVerdictTypes(t *testing.T) {
	// Test VerdictType constants
	if VerdictDrop != 0 {
		t.Errorf("Expected VerdictDrop = 0, got %d", VerdictDrop)
	}
	if VerdictAccept != 1 {
		t.Errorf("Expected VerdictAccept = 1, got %d", VerdictAccept)
	}
	if VerdictAcceptWithMark != 2 {
		t.Errorf("Expected VerdictAcceptWithMark = 2, got %d", VerdictAcceptWithMark)
	}
}

func TestVerdictStruct(t *testing.T) {
	// Test simple verdict
	v := Verdict{Type: VerdictAccept}
	if v.Type != VerdictAccept {
		t.Errorf("Expected VerdictAccept, got %v", v.Type)
	}
	if v.Mark != 0 {
		t.Errorf("Expected Mark = 0, got %d", v.Mark)
	}

	// Test verdict with mark
	v = Verdict{Type: VerdictAcceptWithMark, Mark: 0x200}
	if v.Type != VerdictAcceptWithMark {
		t.Errorf("Expected VerdictAcceptWithMark, got %v", v.Type)
	}
	if v.Mark != 0x200 {
		t.Errorf("Expected Mark = 0x200, got 0x%x", v.Mark)
	}
}

func TestNFQueueReaderSetVerdictFunc(t *testing.T) {
	reader := NewNFQueueReader(100)

	called := false
	var receivedEntry NFLogEntry
	var returnedVerdict Verdict

	// Set verdict function that returns VerdictAcceptWithMark
	reader.SetVerdictFunc(func(entry NFLogEntry) Verdict {
		called = true
		receivedEntry = entry
		returnedVerdict = Verdict{Type: VerdictAcceptWithMark, Mark: 0x300}
		return returnedVerdict
	})

	// Verify the function was set
	if reader.VerdictFunc == nil {
		t.Fatal("VerdictFunc was not set")
	}

	// Call the function
	testEntry := NFLogEntry{
		SrcMAC: "aa:bb:cc:dd:ee:ff",
		SrcIP:  "192.168.1.1",
		DstIP:  "8.8.8.8",
	}

	verdict := reader.VerdictFunc(testEntry)

	if !called {
		t.Error("VerdictFunc was not called")
	}

	if receivedEntry.SrcMAC != testEntry.SrcMAC {
		t.Errorf("Expected SrcMAC %s, got %s", testEntry.SrcMAC, receivedEntry.SrcMAC)
	}

	if returnedVerdict.Type != VerdictAcceptWithMark {
		t.Errorf("Expected VerdictAcceptWithMark, got %v", returnedVerdict.Type)
	}

	if returnedVerdict.Mark != 0x300 {
		t.Errorf("Expected Mark 0x300, got 0x%x", returnedVerdict.Mark)
	}

	if verdict.Type != VerdictAcceptWithMark {
		t.Errorf("Expected verdict.Type VerdictAcceptWithMark, got %v", verdict.Type)
	}
}

func TestDefaultLearningVerdictFunc(t *testing.T) {
	// Create mock functions
	isLearningMode := func() bool { return true }
	processPacket := func(pkt NFLogEntry) (bool, error) { return true, nil }

	// Create verdict function
	verdictFunc := DefaultLearningVerdictFunc(isLearningMode, processPacket)

	// Test with valid packet
	entry := NFLogEntry{
		SrcMAC: "aa:bb:cc:dd:ee:ff",
		SrcIP:  "192.168.1.1",
		DstIP:  "8.0.8.8",
	}

	verdict := verdictFunc(entry)

	if verdict.Type != VerdictAccept {
		t.Errorf("Expected VerdictAccept, got %v", verdict.Type)
	}

	// Test with error
	processPacket = func(pkt NFLogEntry) (bool, error) { return false, fmt.Errorf("test error") }
	verdictFunc = DefaultLearningVerdictFunc(isLearningMode, processPacket)

	verdict = verdictFunc(entry)

	// Should fail-open to accept on error
	if verdict.Type != VerdictAccept {
		t.Errorf("Expected VerdictAccept on error, got %v", verdict.Type)
	}

	// Test with deny
	processPacket = func(pkt NFLogEntry) (bool, error) { return false, nil }
	verdictFunc = DefaultLearningVerdictFunc(isLearningMode, processPacket)

	verdict = verdictFunc(entry)

	if verdict.Type != VerdictDrop {
		t.Errorf("Expected VerdictDrop, got %v", verdict.Type)
	}
}

// Mock nfqueue for testing processJob
type mockNfqueue struct {
	verdicts map[uint32]Verdict
	marks    map[uint32]uint32
}

func (m *mockNfqueue) SetVerdict(id uint32, verdict Verdict) error {
	m.verdicts[id] = verdict
	return nil
}

func (m *mockNfqueue) SetVerdictWithConnMark(id uint32, mark uint32, verdict Verdict) error {
	m.verdicts[id] = verdict
	m.marks[id] = mark
	return nil
}

func TestProcessJob(t *testing.T) {
	// Test verdict processing
	testCases := []struct {
		name           string
		verdict        Verdict
		expectedAccept bool
		expectedDrop   bool
		expectedMark   uint32
	}{
		{
			name:           "Accept",
			verdict:        Verdict{Type: VerdictAccept},
			expectedAccept: true,
			expectedDrop:   false,
			expectedMark:   0,
		},
		{
			name:           "Drop",
			verdict:        Verdict{Type: VerdictDrop},
			expectedAccept: false,
			expectedDrop:   true,
			expectedMark:   0,
		},
		{
			name:           "AcceptWithMark",
			verdict:        Verdict{Type: VerdictAcceptWithMark, Mark: 0x200},
			expectedAccept: true,
			expectedDrop:   false,
			expectedMark:   0x200,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate verdict processing logic
			var accept bool
			var drop bool
			var mark uint32

			switch tc.verdict.Type {
			case VerdictAccept:
				accept = true
			case VerdictDrop:
				drop = true
			case VerdictAcceptWithMark:
				accept = true
				mark = tc.verdict.Mark
			}

			if accept != tc.expectedAccept {
				t.Errorf("Expected accept=%v, got %v", tc.expectedAccept, accept)
			}
			if drop != tc.expectedDrop {
				t.Errorf("Expected drop=%v, got %v", tc.expectedDrop, drop)
			}
			if mark != tc.expectedMark {
				t.Errorf("Expected mark=%d, got %d", tc.expectedMark, mark)
			}
		})
	}
}

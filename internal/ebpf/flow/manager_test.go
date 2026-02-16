// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package flow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

func TestFlowManager_Logic(t *testing.T) {
	logger := logging.New(logging.DefaultConfig())
	config := DefaultConfig()
	
	// Create manager with nil map (should handle it gracefully in our tests)
	m := NewManager(nil, logger, config)

	key := types.FlowKey{
		SrcIP:   0x01020304,
		DstIP:   0x05060708,
		SrcPort: 1234,
		DstPort: 80,
		IPProto: 6,
	}

	t.Run("CreateFlow", func(t *testing.T) {
		state, err := m.CreateFlow(key, types.VerdictTrusted)
		assert.NoError(t, err)
		assert.NotNil(t, state)
		assert.Equal(t, uint8(types.VerdictTrusted), state.Verdict)
		assert.Equal(t, 1, m.GetFlowCount())
	})

	t.Run("GetFlow", func(t *testing.T) {
		state, err := m.GetFlow(key)
		assert.NoError(t, err)
		assert.Equal(t, uint8(types.VerdictTrusted), state.Verdict)
	})

	t.Run("UpdateFlow", func(t *testing.T) {
		state, _ := m.GetFlow(key)
		state.PacketCount = 10
		err := m.UpdateFlow(key, state)
		assert.NoError(t, err)
		
		updated, _ := m.GetFlow(key)
		assert.Equal(t, uint64(10), updated.PacketCount)
	})

	t.Run("DeleteFlow", func(t *testing.T) {
		err := m.DeleteFlow(key)
		assert.NoError(t, err)
		assert.Equal(t, 0, m.GetFlowCount())
	})
}

func TestFlowManager_Expiration(t *testing.T) {
	logger := logging.New(logging.DefaultConfig())
	config := DefaultConfig()
	config.FlowTimeout = 100 * time.Millisecond
	
	m := NewManager(nil, logger, config)
	
	key := types.FlowKey{SrcIP: 1}
	m.CreateFlow(key, types.VerdictTrusted)
	
	assert.Equal(t, 1, m.GetFlowCount())
	
	// Wait for expiration
	time.Sleep(200 * time.Millisecond)
	
	m.cleanupExpiredFlows()
	
	assert.Equal(t, 0, m.GetFlowCount())
}

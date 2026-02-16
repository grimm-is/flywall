// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package network

import (
	"errors"
	"testing"

	"grimm.is/flywall/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func TestApplyVRFs(t *testing.T) {
	mockNetlink := new(MockNetlinker)
	m := NewManagerWithDeps(mockNetlink, nil, nil)

	// Test Case 1: Create new VRF
	vrfName := "vrf-blue"
	tableID := 100
	vrfCfg := []config.VRF{{Name: vrfName, TableID: tableID}}

	// Expect lookup to fail (not found)
	mockNetlink.On("LinkByName", vrfName).Return(netlink.Link(nil), errors.New("not found")).Once()

	// Expect creation
	mockNetlink.On("LinkAdd", mock.MatchedBy(func(link netlink.Link) bool {
		vrf, ok := link.(*netlink.Vrf)
		return ok && vrf.Name == vrfName && vrf.Table == uint32(tableID)
	})).Return(nil).Once()

	// Expect lookup after creation
	vrfLink := &netlink.Vrf{LinkAttrs: netlink.LinkAttrs{Name: vrfName, Index: 10}, Table: uint32(tableID)}
	mockNetlink.On("LinkByName", vrfName).Return(vrfLink, nil).Once()

	// Expect LinkSetUp
	mockNetlink.On("LinkSetUp", vrfLink).Return(nil).Once()

	err := m.ApplyVRFs(vrfCfg)
	assert.NoError(t, err)
	mockNetlink.AssertExpectations(t)

	// Test Case 2: VRF already exists
	mockNetlink = new(MockNetlinker)
	m = NewManagerWithDeps(mockNetlink, nil, nil)

	mockNetlink.On("LinkByName", vrfName).Return(vrfLink, nil).Once() // Found
	mockNetlink.On("LinkSetUp", vrfLink).Return(nil).Once()

	err = m.ApplyVRFs(vrfCfg)
	assert.NoError(t, err)
	mockNetlink.AssertExpectations(t)
}

func TestApplyInterface_VRFEnslavement(t *testing.T) {
	mockNetlink := new(MockNetlinker)
	m := NewManagerWithDeps(mockNetlink, nil, nil)

	ifaceName := "eth0"
	vrfName := "vrf-red"

	eth0Link := &netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: ifaceName, Index: 2}}
	vrfLink := &netlink.Vrf{LinkAttrs: netlink.LinkAttrs{Name: vrfName, Index: 5}}

	// Expectations
	mockNetlink.On("LinkByName", ifaceName).Return(eth0Link, nil).Once()
	mockNetlink.On("LinkSetDown", eth0Link).Return(nil).Once()

	// VRF Enslavement logic
	mockNetlink.On("LinkByName", vrfName).Return(vrfLink, nil).Once()
	mockNetlink.On("LinkSetMaster", eth0Link, vrfLink).Return(nil).Once()

	// Rest of ApplyInterface logic
	mockNetlink.On("LinkSetMTU", eth0Link, 1500).Return(nil).Once()
	mockNetlink.On("LinkSetUp", eth0Link).Return(nil).Once()
	mockNetlink.On("AddrList", eth0Link, unix.AF_UNSPEC).Return([]netlink.Addr{}, nil).Once()

	addr, _ := netlink.ParseAddr("10.0.0.2/24")
	mockNetlink.On("ParseAddr", "10.0.0.2/24").Return(addr, nil).Once()
	mockNetlink.On("AddrAdd", eth0Link, addr).Return(nil).Once()

	cfg := config.Interface{
		Name: ifaceName,
		MTU:  1500,
		IPv4: []string{"10.0.0.2/24"},
		VRF:  vrfName,
	}

	err := m.ApplyInterface(cfg)
	assert.NoError(t, err)
	mockNetlink.AssertExpectations(t)
}

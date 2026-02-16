// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package socket

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

func TestResponseFilter_Regex(t *testing.T) {
	logger := logging.New(logging.DefaultConfig())
	config := DefaultResponseFilterConfig()
	config.Enabled = true
	config.RegexPatterns = []string{
		`^ads-.*\.doubleclick\.net$`,
		`.*\.malicious\.com`,
	}

	rf := NewResponseFilter(logger, config)
	rf.enabled = true
	err := rf.loadDomainLists()
	assert.NoError(t, err)

	tests := []struct {
		domain  string
		blocked bool
		reason  string
	}{
		{"ads-123.doubleclick.net", true, "blocked domain"},
		{"other.doubleclick.net", false, "allowed"},
		{"very.malicious.com", true, "blocked domain"},
		{"not-malicious.org", false, "allowed"},
	}

	for _, tt := range tests {
		event := &types.DNSResponseEvent{
			Domain: tt.domain,
		}
		allowed, reason := rf.FilterResponse(event)
		assert.Equal(t, !tt.blocked, allowed, "Domain: %s", tt.domain)
		if tt.blocked {
			assert.Equal(t, tt.reason, reason, "Domain: %s", tt.domain)
		}
	}
}

func TestDomainList_Regex(t *testing.T) {
	dl := NewDomainList()
	err := dl.AddRule(`.*\.google\.com`, true, "block", "no google")
	assert.NoError(t, err)

	assert.True(t, dl.Contains("www.google.com"))
	assert.True(t, dl.Contains("images.google.com"))
	assert.False(t, dl.Contains("google.com.bad"))
	assert.False(t, dl.Contains("google.org"))
}

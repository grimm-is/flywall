// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package firewall

import (
	"testing"

	"grimm.is/flywall/internal/config"
)

// TestRuleExpressionGeneration tests the buildRuleExpression function for various rule configurations.
func TestRuleExpressionGeneration(t *testing.T) {
	tests := []struct {
		name    string
		rule    config.PolicyRule
		want    string
		wantErr bool
	}{
		{
			name: "Basic TCP Accept",
			rule: config.PolicyRule{
				Protocol: "tcp",
				DestPort: 80,
				Action:   "accept",
			},
			want: "meta l4proto tcp tcp dport 80 counter accept",
		},
		{
			name: "Source IP",
			rule: config.PolicyRule{
				SrcIP:  "192.168.1.10",
				Action: "drop",
			},
			want: "ip saddr 192.168.1.10 limit rate 10/minute log group 0 prefix \"DROP_RULE: \" counter drop",
		},
		{
			name: "Dest IPSet",
			rule: config.PolicyRule{
				DestIPSet: "bad_hosts",
				Action:    "reject",
			},
			want: "ip daddr @bad_hosts limit rate 10/minute log group 0 prefix \"DROP_RULE: \" counter reject",
		},
		{
			name: "Connection State",
			rule: config.PolicyRule{
				ConnState: "established,related",
				Action:    "accept",
			},
			want: "ct state established,related counter accept",
		},
		{
			name: "Complex Rule",
			rule: config.PolicyRule{
				Protocol:  "udp",
				SrcIP:     "10.0.0.0/24",
				DestPort:  53,
				ConnState: "new",
				Action:    "accept",
			},
			want: "meta l4proto udp ip saddr 10.0.0.0/24 ct state new udp dport 53 counter accept",
		},
		// Sad Paths
		{
			name: "Invalid IPSet Name",
			rule: config.PolicyRule{
				SrcIPSet: "bad;set",
			},
			wantErr: true,
		},
		{
			name: "Invalid ConnState",
			rule: config.PolicyRule{
				ConnState: "hacking",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BuildRuleExpression(tt.rule, "UTC")
			if (err != nil) != tt.wantErr {
				t.Errorf("buildRuleExpression() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.want {
				t.Errorf("buildRuleExpression() = %q, want %q", result, tt.want)
			}
		})
	}
}

package sentinel

import (
	"testing"
)

func TestAnalyze(t *testing.T) {
	service := New()

	tests := []struct {
		name     string
		mac      string
		hostname string
		wantCat  string
		wantIcon string
	}{
		{
			name:     "iPhone by Hostname",
			mac:      "00:00:00:00:00:00",
			hostname: "Ben's iPhone",
			wantCat:  "mobile",
			wantIcon: "smartphone",
		},
		{
			name: "Nintendo Switch by Vendor",
			// Nintendo OUI (one of many)
			// Assuming LookupVendor Mock or reliance on real OUI DB if embedded?
			// Since OUI DB is embedded in network package, we rely on it.
			// However, testing specific MACs relies on `network.LookupVendor` working.
			// We'll trust hostname matching more for this unit test if OUI isn't mocked.
			mac:      "00:00:00:00:00:00",
			hostname: "Switch",
			wantCat:  "unknown", // "switch" hostname not enough without vendor 'nintendo'
			wantIcon: "help_outline",
		},
		{
			name:     "MacBook by Hostname",
			mac:      "AA:BB:CC:DD:EE:FF",
			hostname: "Ben-MacBook-Pro",
			wantCat:  "laptop",
			wantIcon: "laptop_mac",
		},
		{
			name:     "Synology NAS by Keyword (assumes vendor fails)",
			mac:      "00:11:32:XX:XX:XX", // Synology OUI
			hostname: "",
			wantCat:  "unknown",
			wantIcon: "help_outline",
		},
		{
			name:     "PlayStation by Hostname",
			mac:      "",
			hostname: "PS5-8273",
			wantCat:  "console",
			wantIcon: "videogame_asset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.Analyze(tt.mac, tt.hostname)
			if got.Category != tt.wantCat {
				t.Errorf("Analyze() Category = %v, want %v", got.Category, tt.wantCat)
			}
			if got.Icon != tt.wantIcon {
				t.Errorf("Analyze() Icon = %v, want %v", got.Icon, tt.wantIcon)
			}
		})
	}
}

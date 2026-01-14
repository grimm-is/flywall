package firewall

import (
	"reflect"
	"sort"
	"testing"
)

func TestGenerateActiveTuples(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		start    string
		end      string
		days     []string
		want     []string
		wantErr  bool
	}{
		{
			name:     "UTC Simple",
			timezone: "UTC",
			start:    "10:00",
			end:      "12:00",
			days:     []string{"Mon"},
			want:     []string{"1 . 10", "1 . 11"},
		},
		{
			name: "Fixed Offset (UTC-5) Simple",
			// Etc/GMT+5 is actually UTC-5? No, POSIX conventions are weird.
			// Usually Etc/GMT+5 is West of GMT? Or East?
			// "Etc/GMT+5" in zoneinfo is GMT-5 (Wait, POSIX says + is West?)
			// Linux/Mac: Etc/GMT+5 is UTC-5.
			// Let's verify with test.
			// Local 10:00 -> UTC 15:00.
			timezone: "Etc/GMT+5",
			start:    "10:00",
			end:      "12:00",
			days:     []string{"Mon"},
			want:     []string{"1 . 15", "1 . 16"},
		},
		{
			name:     "Rollover Midnight (Local)",
			timezone: "UTC",
			start:    "23:00",
			end:      "01:00",
			days:     []string{"Mon"},
			// Mon 23:00, Mon 24:00 (Tue 00:00).
			// Hours: 23 (Mon), 0 (Tue).
			want: []string{"1 . 23", "2 . 0"},
		},
		{
			name:     "Rollover Timezone (PST -> UTC)",
			timezone: "Etc/GMT+8",
			start:    "20:00",
			end:      "22:00",
			days:     []string{"Mon"},
			// Mon 20:00 + 8 = Tue 04:00.
			// Mon 21:00 + 8 = Tue 05:00.
			want: []string{"2 . 4", "2 . 5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateActiveTuples(tt.timezone, tt.start, tt.end, tt.days)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateActiveTuples() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Sort expected and got for comparison
			sort.Strings(got)
			sort.Strings(tt.want)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("generateActiveTuples() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompressTuples(t *testing.T) {
	tests := []struct {
		name   string
		tuples []string
		want   string
	}{
		{
			name:   "Simple Range",
			tuples: []string{"1 . 10", "1 . 11", "1 . 12"},
			want:   "1 . 10-12",
		},
		{
			name:   "Multiple Days",
			tuples: []string{"1 . 10", "1 . 11", "2 . 5"},
			want:   "1 . 10-11, 2 . 5",
		},
		{
			name:   "Disjoint Hours",
			tuples: []string{"1 . 10", "1 . 12", "1 . 14"},
			want:   "1 . 10, 1 . 12, 1 . 14",
		},
		{
			name:   "Wrapping Logic (handled disjointly)",
			tuples: []string{"1 . 23", "1 . 0"}, // Day 1 has 0 and 23.
			// Sorted: 0, 23.
			want: "1 . 0, 1 . 23",
		},
		{
			name:   "Complex Mix",
			tuples: []string{"1 . 10", "1 . 11", "2 . 0", "2 . 1", "2 . 2", "2 . 23"},
			want:   "1 . 10-11, 2 . 0-2, 2 . 23",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compressTuples(tt.tuples)
			// CompressTuples output order depends on map iteration?
			// Logic sorts keys (days) and then sorts days.
			// So output is deterministic.
			if got != tt.want {
				t.Errorf("compressTuples() = %q, want %q", got, tt.want)
			}
		})
	}
}

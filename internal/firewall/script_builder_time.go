// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package firewall

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "time/tzdata" // Embed timezone database for reliability in minimal environments
)

// generateActiveTuples generates a list of "Day . Hour" tuples (e.g. "1 . 23") for the
// given local schedule, converted to UTC.
// Day mapping: 1=Mon, 2=Tue, ..., 7=Sun (matches nftables day mapping? No, nftables meta day is 0-6 or 1-7 depending on version/locale. Usually 1=Mon).
// Wait, nftables `meta day` usually follows: 0=Sunday, 1=Monday, ..., 6=Saturday.
// We will use integer output compatible with `meta day`.
func generateActiveTuples(timezone, startStr, endStr string, days []string) ([]string, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("failed to load timezone %s: %w", timezone, err)
	}

	// Parse HH:MM
	parseTime := func(s string) (int, int, error) {
		t, err := time.Parse("15:04", s)
		if err != nil {
			return 0, 0, err
		}
		return t.Hour(), t.Minute(), nil
	}

	sH, _, err := parseTime(startStr)
	if err != nil {
		return nil, err
	}
	eH, _, err := parseTime(endStr)
	if err != nil {
		return nil, err
	}

	// Calculate duration in hours (ceiling)
	// If eH < sH, it wraps directly (e.g. 22 to 02 is 4 hours: 22, 23, 00, 01)
	// Active hours list (0-23)
	var activeHours []int
	if eH > sH {
		// e.g. 09 to 17 -> 9, 10, ... 16
		for h := sH; h < eH; h++ {
			activeHours = append(activeHours, h)
		}
	} else {
		// e.g. 22 to 02 -> 22, 23, 0, 1
		for h := sH; h < 24; h++ {
			activeHours = append(activeHours, h)
		}
		for h := 0; h < eH; h++ {
			activeHours = append(activeHours, h)
		}
	}

	// Map day names to integers (0=Sun, 1=Mon...6=Sat)
	dayToInt := map[string]int{
		"sunday": 0, "monday": 1, "tuesday": 2, "wednesday": 3,
		"thursday": 4, "friday": 5, "saturday": 6,
		"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
	}

	var tuples []string
	now := time.Now() // Reference for year/month

	// For each active day
	for _, dayName := range days {
		dayName = strings.ToLower(dayName)
		targetDayIdx, ok := dayToInt[dayName]
		if !ok {
			continue // Skip invalid days
		}

		// Find a date that matches this day of week (e.g. next targetDayIdx)
		// We use a reference week.
		// Simply: Construct a time for "Today" + offset to match targetDayIdx?
		// No, we just need ANY valid date with that weekday to handle timezone conversion accurately?
		// Actually, DST might change offset.
		// Ideally we project for "now" or "next occurance".
		// We'll use "next occurance of Day" from now.
		daysUntil := (targetDayIdx - int(now.Weekday()) + 7) % 7
		baseDate := now.AddDate(0, 0, daysUntil)

		for _, h := range activeHours {
			// Construct local time: baseDate (Year/Month/Day) + h
			// Note: baseDate is in Local or UTC? The conversion logic needs Local inputs.
			// We construct a specific time in the Location.
			localT := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), h, 0, 0, 0, loc)

			// Adjust if we crossed day boundary in local generation?
			// The `activeHours` loop handles wrapping (e.g. 22, 23, 0, 1).
			// If h < sH (and wrapped), it implies "next day" relative to start?
			// Yes. If schedule is 22:00-02:00 on Monday...
			// Does it mean Mon 22:00 and Tue 00:00?
			// Or Mon 22:00 and Mon 00:00?
			// Usually "Mon 22:00-02:00" implies Mon night.
			// So hours 22, 23 belong to Mon. Hours 0, 1 belong to Tue.

			// Determine day offset for this hour
			// If wrapped (eH < sH) AND h < sH (e.g. h=0,1), it's +1 day.
			currentLocalT := localT
			if eH <= sH && h < sH {
				currentLocalT = currentLocalT.AddDate(0, 0, 1)
			}

			// Convert to UTC
			utcT := currentLocalT.UTC()

			// Extract UTC Day/Hour
			utcDay := int(utcT.Weekday())
			utcHour := utcT.Hour()

			tuples = append(tuples, fmt.Sprintf("%d . %d", utcDay, utcHour))
		}
	}

	// Deduplicate and sort?
	// The set syntax handles duplicates, but nice to be clean.
	return tuples, nil
}

// compressTuples compresses a list of "Day . Hour" tuples into disjoint ranges
// e.g. "1.22", "1.23", "2.0" -> "1 . 22-23, 2 . 0"
func compressTuples(tuples []string) string {
	days := make(map[int][]int)
	for _, t := range tuples {
		parts := strings.Split(t, " . ")
		if len(parts) != 2 {
			continue
		}
		d, _ := strconv.Atoi(parts[0])
		h, _ := strconv.Atoi(parts[1])

		exists := false
		for _, existing := range days[d] {
			if existing == h {
				exists = true
				break
			}
		}
		if !exists {
			days[d] = append(days[d], h)
		}
	}

	var elements []string
	var dayKeys []int
	for k := range days {
		dayKeys = append(dayKeys, k)
	}
	sort.Ints(dayKeys)

	for _, d := range dayKeys {
		hours := days[d]
		sort.Ints(hours)

		// Compress hours into ranges
		var ranges []string
		if len(hours) > 0 {
			start := hours[0]
			prev := hours[0]
			for i := 1; i < len(hours); i++ {
				if hours[i] == prev+1 {
					prev = hours[i]
				} else {
					if start == prev {
						ranges = append(ranges, strconv.Itoa(start))
					} else {
						ranges = append(ranges, fmt.Sprintf("%d-%d", start, prev))
					}
					start = hours[i]
					prev = hours[i]
				}
			}
			if start == prev {
				ranges = append(ranges, strconv.Itoa(start))
			} else {
				ranges = append(ranges, fmt.Sprintf("%d-%d", start, prev))
			}
		}

		for _, r := range ranges {
			elements = append(elements, fmt.Sprintf("%d . %s", d, r))
		}
	}
	return strings.Join(elements, ", ")
}

// GetNextDSTTransition returns the next time the offset changes for the given timezone.
// It scans forward up to 365 days. Returns zero time if no transition found.
func GetNextDSTTransition(timezone string) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, err
	}

	now := time.Now().In(loc)
	_, initialOffset := now.Zone()

	// Scan forward by day to find offset change
	for i := 1; i <= 365; i++ {
		t := now.AddDate(0, 0, i)
		_, offset := t.Zone()
		if offset != initialOffset {
			// Found day with different offset. Return midnight of that day as hint.
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc), nil
		}
	}
	return time.Time{}, nil
}

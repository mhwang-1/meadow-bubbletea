package domain

import (
	"testing"
	"time"
)

func TestWeekForDate(t *testing.T) {
	tests := []struct {
		name      string
		date      time.Time
		wantYear  int
		wantWeek  int
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name:      "27_Mar_2026_Fri",
			date:      time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC),
			wantYear:  2026,
			wantWeek:  13,
			wantStart: time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "22_Mar_2026_Sun",
			date:      time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC),
			wantYear:  2026,
			wantWeek:  13,
			wantStart: time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "28_Mar_2026_Sat",
			date:      time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC),
			wantYear:  2026,
			wantWeek:  13,
			wantStart: time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "1_Jan_2026_Thu",
			date:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantYear:  2026,
			wantWeek:  1,
			wantStart: time.Date(2025, 12, 28, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "4_Jan_2026_Sun",
			date:      time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
			wantYear:  2026,
			wantWeek:  2,
			wantStart: time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wi := WeekForDate(tc.date)
			if wi.Year != tc.wantYear {
				t.Errorf("Year = %d, want %d", wi.Year, tc.wantYear)
			}
			if wi.Week != tc.wantWeek {
				t.Errorf("Week = %d, want %d", wi.Week, tc.wantWeek)
			}
			if !wi.StartDate.Equal(tc.wantStart) {
				t.Errorf("StartDate = %v, want %v", wi.StartDate, tc.wantStart)
			}
			if !wi.EndDate.Equal(tc.wantEnd) {
				t.Errorf("EndDate = %v, want %v", wi.EndDate, tc.wantEnd)
			}
		})
	}
}

func TestFormatDate(t *testing.T) {
	date := time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC)
	got := FormatDate(date)
	expected := "Fri, 27 Mar 2026"
	if got != expected {
		t.Errorf("FormatDate() = %q, want %q", got, expected)
	}
}

func TestDayFilename(t *testing.T) {
	tests := []struct {
		date     time.Time
		expected string
	}{
		{time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC), "2026-W13-fri"},
		{time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC), "2026-W13-sun"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			got := DayFilename(tc.date)
			if got != tc.expected {
				t.Errorf("DayFilename(%v) = %q, want %q", tc.date, got, tc.expected)
			}
		})
	}
}

func TestParseDayFilename(t *testing.T) {
	// Round-trip: generate a filename and parse it back to the original date.
	dates := []time.Time{
		time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
	}

	for _, date := range dates {
		filename := DayFilename(date)
		t.Run(filename, func(t *testing.T) {
			got, err := ParseDayFilename(filename)
			if err != nil {
				t.Fatalf("ParseDayFilename(%q) unexpected error: %v", filename, err)
			}
			if !got.Equal(date) {
				t.Errorf("ParseDayFilename(%q) = %v, want %v", filename, got, date)
			}
		})
	}
}

func TestFormatWeekRange(t *testing.T) {
	t.Run("same_month", func(t *testing.T) {
		wi := WeekInfo{
			Year:      2026,
			Week:      13,
			StartDate: time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC),
		}
		got := FormatWeekRange(wi)
		expected := "22\u201328 Mar 2026"
		if got != expected {
			t.Errorf("FormatWeekRange() = %q, want %q", got, expected)
		}
	})

	t.Run("cross_month", func(t *testing.T) {
		wi := WeekInfo{
			Year:      2026,
			Week:      14,
			StartDate: time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC),
		}
		got := FormatWeekRange(wi)
		expected := "29 Mar \u2013 4 Apr 2026"
		if got != expected {
			t.Errorf("FormatWeekRange() = %q, want %q", got, expected)
		}
	})
}

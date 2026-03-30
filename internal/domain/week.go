package domain

import (
	"fmt"
	"strings"
	"time"
)

// WeekInfo represents a Sunday-start week.
type WeekInfo struct {
	Year      int
	Week      int
	StartDate time.Time // Sunday
	EndDate   time.Time // Saturday
}

// dayAbbrevs maps time.Weekday to the lowercase abbreviation used in filenames.
var dayAbbrevs = [7]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// WeekForDate calculates the Sunday-start week number for a given date.
//
// Algorithm:
//   - Find the Sunday on or before the given date (weekSunday).
//   - Year = weekSunday.Year().
//   - Find 1 Jan of that year.
//   - If 1 Jan is a Sunday, firstFullWeekSunday = 1 Jan.
//     Otherwise, firstFullWeekSunday = the next Sunday after 1 Jan.
//   - If weekSunday < firstFullWeekSunday: week = 1 (partial first week).
//   - Otherwise: week = (weekSunday - firstFullWeekSunday).Days/7 + 2.
//
// Example: 27 Mar 2026 (Friday) → weekSunday = 22 Mar 2026, year = 2026,
// 1 Jan 2026 = Thursday, firstFullWeekSunday = 4 Jan 2026,
// (22 Mar - 4 Jan) = 77 days, 77/7 = 11, week = 11 + 2 = 13. ✓
func WeekForDate(date time.Time) WeekInfo {
	date = stripTime(date)

	// Find the Sunday on or before date.
	weekSunday := sundayOnOrBefore(date)
	weekSaturday := weekSunday.AddDate(0, 0, 6)

	// Determine the year: a week belongs to the year whose Jan 1 it contains.
	// If the week spans a year boundary and contains the next year's Jan 1,
	// the week belongs to that next year.
	year := weekSunday.Year()
	nextJan1 := time.Date(year+1, time.January, 1, 0, 0, 0, 0, date.Location())
	if !nextJan1.After(weekSaturday) {
		year = year + 1
	}

	// Find the start of week 1: the Sunday on or before Jan 1 of the owning year.
	jan1 := time.Date(year, time.January, 1, 0, 0, 0, 0, date.Location())
	w1Sunday := sundayOnOrBefore(jan1)

	// Week number = distance from W1's Sunday, in weeks, plus 1.
	days := int(weekSunday.Sub(w1Sunday).Hours() / 24)
	week := days/7 + 1

	return WeekInfo{
		Year:      year,
		Week:      week,
		StartDate: weekSunday,
		EndDate:   weekSaturday,
	}
}

// DaysOfWeek returns an array of 7 dates (Sunday through Saturday) for the given week.
func DaysOfWeek(wi WeekInfo) [7]time.Time {
	var days [7]time.Time
	for i := 0; i < 7; i++ {
		days[i] = wi.StartDate.AddDate(0, 0, i)
	}
	return days
}

// FormatDate formats a date as "Fri, 27 Mar 2026" (British format).
func FormatDate(t time.Time) string {
	return t.Format("Mon, 02 Jan 2006")
}

// FormatWeekRange formats a week range as "22–28 Mar 2026" when the week
// falls within one month, or "29 Mar – 4 Apr 2026" when it spans months.
func FormatWeekRange(wi WeekInfo) string {
	s := wi.StartDate
	e := wi.EndDate

	if s.Month() == e.Month() && s.Year() == e.Year() {
		// Same month: "22–28 Mar 2026"
		return fmt.Sprintf("%d\u2013%d %s %d",
			s.Day(), e.Day(), s.Format("Jan"), s.Year())
	}

	if s.Year() == e.Year() {
		// Different months, same year: "29 Mar – 4 Apr 2026"
		return fmt.Sprintf("%d %s \u2013 %d %s %d",
			s.Day(), s.Format("Jan"),
			e.Day(), e.Format("Jan"),
			e.Year())
	}

	// Different years: "28 Dec 2025 – 3 Jan 2026"
	return fmt.Sprintf("%d %s %d \u2013 %d %s %d",
		s.Day(), s.Format("Jan"), s.Year(),
		e.Day(), e.Format("Jan"), e.Year())
}

// DayFilename returns the timebox filename for a date, e.g. "2026-W13-fri".
// The week number is that of the Sunday-start week the date belongs to.
func DayFilename(date time.Time) string {
	date = stripTime(date)
	wi := WeekForDate(date)
	dow := dayAbbrevs[date.Weekday()]
	return fmt.Sprintf("%d-W%02d-%s", wi.Year, wi.Week, dow)
}

// ParseDayFilename parses a filename like "2026-W13-fri" back to a date.
// It reconstructs the date from the year, week number, and day of week.
func ParseDayFilename(name string) (time.Time, error) {
	// Remove any path or extension.
	name = strings.TrimSuffix(name, ".md")

	var year, week int
	var dayStr string
	n, err := fmt.Sscanf(name, "%d-W%d-%s", &year, &week, &dayStr)
	if err != nil || n != 3 {
		return time.Time{}, fmt.Errorf("invalid day filename format: %q", name)
	}

	dayStr = strings.ToLower(dayStr)
	dayIndex := -1
	for i, abbr := range dayAbbrevs {
		if abbr == dayStr {
			dayIndex = i
			break
		}
	}
	if dayIndex < 0 {
		return time.Time{}, fmt.Errorf("invalid day abbreviation in filename: %q", dayStr)
	}

	// Reconstruct the week's Sunday using the same algorithm as WeekForDate:
	// Week 1 starts on the Sunday on or before Jan 1 of the given year.
	jan1 := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	w1Sunday := sundayOnOrBefore(jan1)
	weekSunday := w1Sunday.AddDate(0, 0, (week-1)*7)

	result := weekSunday.AddDate(0, 0, dayIndex)
	return result, nil
}

// sundayOnOrBefore returns the Sunday on or before the given date.
func sundayOnOrBefore(date time.Time) time.Time {
	offset := int(date.Weekday()) // Sunday=0, Monday=1, ..., Saturday=6
	return date.AddDate(0, 0, -offset)
}

// nextSunday returns the first Sunday strictly after the given date.
func nextSunday(date time.Time) time.Time {
	daysUntilSunday := (7 - int(date.Weekday())) % 7
	if daysUntilSunday == 0 {
		daysUntilSunday = 7
	}
	return date.AddDate(0, 0, daysUntilSunday)
}

// StripTimeForDate returns the date with time components zeroed out, preserving the location.
func StripTimeForDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// stripTime is an internal alias for StripTimeForDate.
func stripTime(t time.Time) time.Time {
	return StripTimeForDate(t)
}

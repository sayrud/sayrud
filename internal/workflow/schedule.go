package workflow

import (
	"time"

	"github.com/samber/lo"
)

// ScheduleLayout is the layout of a schedule trigger's start time.
const ScheduleLayout = "2006-01-02T15:04"

// Interval ranges of schedule triggers.
const (
	minMinuteInterval = 5
	maxMinuteInterval = 24 * 60
	maxHourInterval   = 7 * 24
)

// parseSchedule returns the start time interpreted in the schedule's time zone.
func parseSchedule(s *ScheduleTrigger) (time.Time, bool) {
	if s == nil || s.Timezone == "" {
		return time.Time{}, false
	}
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.Time{}, false
	}
	start, err := time.ParseInLocation(ScheduleLayout, s.Start, loc)
	if err != nil {
		return time.Time{}, false
	}
	return start, true
}

// checkSchedule returns the issue code of the schedule configuration, or an empty string if there is none.
func checkSchedule(s *ScheduleTrigger) string {
	if s == nil {
		return "trigger.invalid_start"
	}
	if _, err := time.LoadLocation(s.Timezone); err != nil || s.Timezone == "" {
		return "trigger.invalid_timezone"
	}
	if _, ok := parseSchedule(s); !ok {
		return "trigger.invalid_start"
	}
	switch s.Repeat {
	case RepeatNone, RepeatDaily, RepeatWorkdays, RepeatMonthly:
	case RepeatMinutes:
		if s.Interval < minMinuteInterval || s.Interval > maxMinuteInterval {
			return "trigger.invalid_interval"
		}
	case RepeatHours:
		if s.Interval < 1 || s.Interval > maxHourInterval {
			return "trigger.invalid_interval"
		}
	case RepeatWeekly:
		if len(s.Weekdays) == 0 || lo.SomeBy(s.Weekdays, func(d int) bool { return d < 0 || d > 6 }) {
			return "trigger.no_weekdays"
		}
	default:
		return "trigger.invalid_repeat"
	}
	return ""
}

// NextFire returns the next fire time after after; ok is false if it will not fire again.
func NextFire(s *ScheduleTrigger, after time.Time) (time.Time, bool) {
	start, ok := parseSchedule(s)
	if !ok || checkSchedule(s) != "" {
		return time.Time{}, false
	}
	switch s.Repeat {
	case RepeatNone:
		return start, start.After(after)
	case RepeatMinutes, RepeatHours:
		step := time.Duration(s.Interval) * time.Minute
		if s.Repeat == RepeatHours {
			step = time.Duration(s.Interval) * time.Hour
		}
		if start.After(after) {
			return start, true
		}
		n := after.Sub(start)/step + 1
		return start.Add(n * step), true
	}

	loc := start.Location()
	from := after.In(loc)
	if start.After(after) {
		from = start
	}
	// A day-based repeating trigger always fires within a little over a year.
	for i := 0; i <= 400; i++ {
		day := time.Date(from.Year(), from.Month(), from.Day()+i, start.Hour(), start.Minute(), 0, 0, loc)
		if day.Before(start) || !day.After(after) || !matchesDay(s, start, day) {
			continue
		}
		return day, true
	}
	return time.Time{}, false
}

// matchesDay reports whether a day-based repeating trigger fires on day.
func matchesDay(s *ScheduleTrigger, start, day time.Time) bool {
	switch s.Repeat {
	case RepeatWorkdays:
		return day.Weekday() != time.Saturday && day.Weekday() != time.Sunday
	case RepeatWeekly:
		return lo.Contains(s.Weekdays, int(day.Weekday()))
	case RepeatMonthly:
		// If this month has no day matching the start date's day, fire on the last day of the month.
		last := time.Date(day.Year(), day.Month()+1, 0, 0, 0, 0, 0, day.Location()).Day()
		return day.Day() == min(start.Day(), last)
	}
	return true
}

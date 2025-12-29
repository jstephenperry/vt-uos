package util

import (
	"fmt"
	"time"
)

const (
	// DateFormat is the standard date format for vault records.
	DateFormat = "2006-01-02"

	// DateTimeFormat is the standard datetime format for vault records.
	DateTimeFormat = "2006-01-02 15:04:05"

	// ISO8601Format is the RFC3339 format used in configuration and APIs.
	ISO8601Format = time.RFC3339
)

// VaultClock manages the simulated vault time.
type VaultClock struct {
	// startRealTime is when the simulation started in real time.
	startRealTime time.Time

	// startVaultTime is the vault time when simulation started.
	startVaultTime time.Time

	// timeScale is the ratio of vault time to real time.
	// 1.0 = real-time, 60.0 = 1 real minute = 1 vault hour
	timeScale float64

	// paused indicates if time progression is stopped.
	paused bool

	// pausedAt is the vault time when pause occurred.
	pausedAt time.Time
}

// NewVaultClock creates a new vault clock starting at the given time.
func NewVaultClock(vaultStartTime time.Time, timeScale float64) *VaultClock {
	return &VaultClock{
		startRealTime:  time.Now(),
		startVaultTime: vaultStartTime,
		timeScale:      timeScale,
		paused:         false,
	}
}

// Now returns the current vault time.
func (vc *VaultClock) Now() time.Time {
	if vc.paused {
		return vc.pausedAt
	}

	realElapsed := time.Since(vc.startRealTime)
	vaultElapsed := time.Duration(float64(realElapsed) * vc.timeScale)
	return vc.startVaultTime.Add(vaultElapsed)
}

// Pause stops time progression.
func (vc *VaultClock) Pause() {
	if !vc.paused {
		vc.pausedAt = vc.Now()
		vc.paused = true
	}
}

// Resume continues time progression.
func (vc *VaultClock) Resume() {
	if vc.paused {
		vc.startRealTime = time.Now()
		vc.startVaultTime = vc.pausedAt
		vc.paused = false
	}
}

// IsPaused returns true if the clock is paused.
func (vc *VaultClock) IsPaused() bool {
	return vc.paused
}

// SetTimeScale changes the time scaling factor.
func (vc *VaultClock) SetTimeScale(scale float64) {
	// Save current vault time
	currentVaultTime := vc.Now()

	// Reset with new scale
	vc.startRealTime = time.Now()
	vc.startVaultTime = currentVaultTime
	vc.timeScale = scale

	if vc.paused {
		vc.pausedAt = currentVaultTime
	}
}

// TimeScale returns the current time scale.
func (vc *VaultClock) TimeScale() float64 {
	return vc.timeScale
}

// Advance manually advances vault time by the given duration.
// Only works when paused.
func (vc *VaultClock) Advance(d time.Duration) error {
	if !vc.paused {
		return fmt.Errorf("cannot advance time while running; pause first")
	}
	vc.pausedAt = vc.pausedAt.Add(d)
	return nil
}

// SetTime sets the vault time to a specific time.
// Only works when paused.
func (vc *VaultClock) SetTime(t time.Time) error {
	if !vc.paused {
		return fmt.Errorf("cannot set time while running; pause first")
	}
	vc.pausedAt = t
	return nil
}

// FormatDate formats a time as a date string.
func FormatDate(t time.Time) string {
	return t.Format(DateFormat)
}

// FormatDateTime formats a time as a datetime string.
func FormatDateTime(t time.Time) string {
	return t.Format(DateTimeFormat)
}

// FormatISO8601 formats a time as an ISO8601/RFC3339 string.
func FormatISO8601(t time.Time) string {
	return t.Format(ISO8601Format)
}

// ParseDate parses a date string.
func ParseDate(s string) (time.Time, error) {
	return time.Parse(DateFormat, s)
}

// ParseDateTime parses a datetime string.
func ParseDateTime(s string) (time.Time, error) {
	return time.Parse(DateTimeFormat, s)
}

// ParseISO8601 parses an ISO8601/RFC3339 string.
func ParseISO8601(s string) (time.Time, error) {
	return time.Parse(ISO8601Format, s)
}

// CalculateAge calculates age in years from date of birth.
func CalculateAge(dob time.Time, asOf time.Time) int {
	years := asOf.Year() - dob.Year()

	// Adjust if birthday hasn't occurred yet this year
	if asOf.YearDay() < dob.YearDay() {
		years--
	}

	return years
}

// CalculateAgeAtDate calculates age at a specific date.
func CalculateAgeAtDate(dob, date time.Time) int {
	return CalculateAge(dob, date)
}

// IsAdult returns true if the person is 18 or older.
func IsAdult(dob time.Time, asOf time.Time) bool {
	return CalculateAge(dob, asOf) >= 18
}

// IsWorkingAge returns true if the person is between 16 and 65.
func IsWorkingAge(dob time.Time, asOf time.Time) bool {
	age := CalculateAge(dob, asOf)
	return age >= 16 && age < 65
}

// IsElderly returns true if the person is 65 or older.
func IsElderly(dob time.Time, asOf time.Time) bool {
	return CalculateAge(dob, asOf) >= 65
}

// DaysSince calculates the number of days between two dates.
func DaysSince(from, to time.Time) int {
	// Normalize to midnight
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	to = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location())

	return int(to.Sub(from).Hours() / 24)
}

// DaysUntil calculates the number of days until a future date.
func DaysUntil(from, to time.Time) int {
	return DaysSince(from, to)
}

// IsSameDay checks if two times are on the same calendar day.
func IsSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// StartOfDay returns midnight of the given day.
func StartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// EndOfDay returns 23:59:59 of the given day.
func EndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
}

// RelativeTimeString returns a human-readable relative time string.
func RelativeTimeString(t time.Time, now time.Time) string {
	diff := now.Sub(t)

	if diff < 0 {
		diff = -diff
		return futureTimeString(diff)
	}

	return pastTimeString(diff)
}

func pastTimeString(diff time.Duration) string {
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(diff.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

func futureTimeString(diff time.Duration) string {
	switch {
	case diff < time.Minute:
		return "now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "in 1 minute"
		}
		return fmt.Sprintf("in %d minutes", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "in 1 hour"
		}
		return fmt.Sprintf("in %d hours", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "tomorrow"
		}
		return fmt.Sprintf("in %d days", days)
	default:
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "in 1 week"
		}
		return fmt.Sprintf("in %d weeks", weeks)
	}
}

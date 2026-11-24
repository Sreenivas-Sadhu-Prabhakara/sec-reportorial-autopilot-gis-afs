// Package secfiling holds the reportorial rules this tool applies for a
// Philippine corporation: when the General Information Sheet falls due, when
// the Audited Financial Statements fall due, how close the next deadline is,
// and what a missed filing leads to.
//
// Everything here is a pure function over plain values. No clock is read
// except through an explicit "as of" date passed in by the caller, no file is
// touched, and no network is used, so every rule can be read and tested on its
// own.
package secfiling

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// dateLayout is the only date format this tool reads or writes.
const dateLayout = "2006-01-02"

// Date is a calendar date with no time and no zone. Filing deadlines are
// counted in calendar days by the Commission, so a plain date is the right
// unit: there is no hour at which a deadline falls, and no timezone
// conversion can move a Philippine filing date by a day.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// NewDate builds a date without normalising it. Use Valid to check it.
func NewDate(year int, month time.Month, day int) Date {
	return Date{Year: year, Month: month, Day: day}
}

// DateOf takes the calendar date out of a time.Time, in that time's own zone.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// Today is the current date in the machine's own timezone. It is the only
// place in the package that reads the clock, and nothing else calls it: every
// rule takes the date it should work from as an argument.
func Today() Date { return DateOf(time.Now()) }

// ParseDate reads a YYYY-MM-DD date and rejects dates that do not exist, so
// "2026-02-30" is an error rather than quietly becoming 2 March.
func ParseDate(s string) (Date, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Date{}, fmt.Errorf("a date is needed, in the form YYYY-MM-DD")
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("%q is not a date in the form YYYY-MM-DD", s)
	}
	return DateOf(t), nil
}

// MustParseDate is ParseDate for dates written into this source file, where a
// typo is a programming error rather than bad input.
func MustParseDate(s string) Date {
	d, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

// IsZero reports whether the date was never set.
func (d Date) IsZero() bool { return d == Date{} }

// Valid reports whether the date exists in the calendar.
func (d Date) Valid() bool {
	if d.IsZero() {
		return false
	}
	if d.Month < time.January || d.Month > time.December {
		return false
	}
	return d.Day >= 1 && d.Day <= DaysInMonth(d.Year, d.Month)
}

func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day)
}

// Time is the date at midnight UTC, used only for arithmetic inside this
// package.
func (d Date) Time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// MarshalJSON writes an empty date as null and any other date as a string.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
}

// UnmarshalJSON accepts null, an empty string or YYYY-MM-DD.
func (d *Date) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*d = Date{}
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	if strings.TrimSpace(s) == "" {
		*d = Date{}
		return nil
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// Before, After and Equal compare two dates on the calendar.
func (d Date) Before(o Date) bool { return d.Time().Before(o.Time()) }
func (d Date) After(o Date) bool  { return d.Time().After(o.Time()) }
func (d Date) Equal(o Date) bool  { return d == o }

// DaysInMonth answers how long a month is, which is where February and leap
// years come in.
func DaysInMonth(year int, month time.Month) int {
	switch month {
	case time.January, time.March, time.May, time.July, time.August, time.October, time.December:
		return 31
	case time.April, time.June, time.September, time.November:
		return 30
	case time.February:
		if IsLeapYear(year) {
			return 29
		}
		return 28
	}
	return 0
}

// IsLeapYear uses the full Gregorian rule, so 1900 is not a leap year and 2000
// is.
func IsLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// AddDays counts forward (or back) in calendar days, which is the unit the
// Commission uses: "within thirty (30) calendar days" counts weekends and
// holidays alike.
func (d Date) AddDays(n int) Date {
	return DateOf(d.Time().AddDate(0, 0, n))
}

// AddMonths counts forward in months and clamps to the end of the target
// month, so 31 January plus one month is 28 February (29 February in a leap
// year) rather than spilling into March. Go's own AddDate spills; a filing
// deadline must not.
func (d Date) AddMonths(n int) Date {
	total := (d.Year * 12) + int(d.Month) - 1 + n
	year := floorDiv(total, 12)
	month := time.Month(floorMod(total, 12) + 1)
	day := d.Day
	if last := DaysInMonth(year, month); day > last {
		day = last
	}
	return Date{Year: year, Month: month, Day: day}
}

// AddYears counts whole years with the same month-end clamping, so 29 February
// plus one year is 28 February.
func (d Date) AddYears(n int) Date { return d.AddMonths(12 * n) }

// AnniversaryIn is the same day and month in another year, clamped the same
// way: a licence issued on 29 February has its anniversary on 28 February in
// an ordinary year.
func (d Date) AnniversaryIn(year int) Date {
	day := d.Day
	if last := DaysInMonth(year, d.Month); day > last {
		day = last
	}
	return Date{Year: year, Month: d.Month, Day: day}
}

// EndOfMonth is the last day of the date's own month.
func (d Date) EndOfMonth() Date {
	return Date{Year: d.Year, Month: d.Month, Day: DaysInMonth(d.Year, d.Month)}
}

// DaysUntil counts calendar days from d to o: positive when o is later,
// negative when it is earlier, zero on the same day.
func (d Date) DaysUntil(o Date) int {
	return int(o.Time().Sub(d.Time()).Hours() / 24)
}

// MonthsStarted counts how many months of delay have begun between d and o.
// The day d itself is month zero; one day later is the first month started.
// It is used only where a rule is stated per month of delay, and the counting
// is the tool's own reading, stated as such wherever it is shown.
func (d Date) MonthsStarted(o Date) int {
	if !o.After(d) {
		return 0
	}
	n := 1
	for d.AddMonths(n).Before(o) {
		n++
		if n > 1200 {
			return n
		}
	}
	return n
}

// Weekday is the day of the week the date falls on.
func (d Date) Weekday() time.Weekday { return d.Time().Weekday() }

// IsWeekend reports a Saturday or a Sunday.
func (d Date) IsWeekend() bool {
	w := d.Weekday()
	return w == time.Saturday || w == time.Sunday
}

// Holidays is a list of non-working days the user keeps themselves, keyed by
// date and holding the name shown in the interface. The tool ships with none:
// Philippine holidays are proclaimed yearly and this binary never goes online
// to fetch them, so an out-of-date built-in list would be worse than an empty
// one the user fills in from the proclamation.
type Holidays map[string]string

// Name returns the holiday falling on a date, if the user recorded one.
func (h Holidays) Name(d Date) (string, bool) {
	if h == nil {
		return "", false
	}
	name, ok := h[d.String()]
	return name, ok
}

// IsWorkingDay reports a weekday that is not one of the user's holidays.
func (h Holidays) IsWorkingDay(d Date) bool {
	if d.IsWeekend() {
		return false
	}
	_, holiday := h.Name(d)
	return !holiday
}

// PrecedingWorkingDay walks back to the first working day on or before d.
func (h Holidays) PrecedingWorkingDay(d Date) Date {
	for i := 0; i < 400; i++ {
		if h.IsWorkingDay(d) {
			return d
		}
		d = d.AddDays(-1)
	}
	return d
}

// NextWorkingDay walks forward to the first working day on or after d.
func (h Holidays) NextWorkingDay(d Date) Date {
	for i := 0; i < 400; i++ {
		if h.IsWorkingDay(d) {
			return d
		}
		d = d.AddDays(1)
	}
	return d
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func floorMod(a, b int) int { return a - floorDiv(a, b)*b }

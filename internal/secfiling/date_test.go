package secfiling

import (
	"encoding/json"
	"testing"
	"time"
)

func d(t *testing.T, s string) Date {
	t.Helper()
	parsed, err := ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return parsed
}

func TestParseDateRejectsDatesThatDoNotExist(t *testing.T) {
	cases := []struct {
		in   string
		want string // empty means an error is expected
	}{
		{"2026-06-15", "2026-06-15"},
		{"  2026-06-15  ", "2026-06-15"},
		{"2024-02-29", "2024-02-29"}, // 2024 is a leap year
		{"2026-02-29", ""},           // 2026 is not
		{"2026-02-30", ""},
		{"2026-13-01", ""},
		{"2026-00-10", ""},
		{"2026-04-31", ""}, // April has 30 days
		{"15/06/2026", ""},
		{"", ""},
	}
	for _, c := range cases {
		got, err := ParseDate(c.in)
		if c.want == "" {
			if err == nil {
				t.Errorf("ParseDate(%q) = %s, want an error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseDate(%q): unexpected error %v", c.in, err)
			continue
		}
		if got.String() != c.want {
			t.Errorf("ParseDate(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestDaysInMonthFollowsTheFullLeapRule(t *testing.T) {
	cases := []struct {
		year  int
		month time.Month
		want  int
	}{
		{2026, time.February, 28},
		{2024, time.February, 29}, // divisible by 4
		{2000, time.February, 29}, // divisible by 400
		{1900, time.February, 28}, // divisible by 100 but not 400
		{2026, time.January, 31},
		{2026, time.April, 30},
		{2026, time.December, 31},
	}
	for _, c := range cases {
		if got := DaysInMonth(c.year, c.month); got != c.want {
			t.Errorf("DaysInMonth(%d, %s) = %d, want %d", c.year, c.month, got, c.want)
		}
	}
}

func TestAddDaysCountsCalendarDaysAcrossMonthAndYearEnds(t *testing.T) {
	cases := []struct {
		name string
		from string
		add  int
		want string
	}{
		{"into a leap day", "2024-02-28", 1, "2024-02-29"},
		{"over a leap day", "2024-02-28", 2, "2024-03-01"},
		{"no leap day to land on", "2026-02-28", 1, "2026-03-01"},
		{"thirty days from a January meeting in a leap year", "2024-01-31", 30, "2024-03-01"},
		{"thirty days from the same date in an ordinary year", "2023-01-31", 30, "2023-03-02"},
		{"across the year end", "2026-12-15", 30, "2027-01-14"},
		{"year end exactly", "2026-12-31", 1, "2027-01-01"},
		{"backwards over a year end", "2027-01-01", -1, "2026-12-31"},
		{"nothing added", "2026-06-15", 0, "2026-06-15"},
		{"a hundred and twenty days from a June year end", "2026-06-30", 120, "2026-10-28"},
		{"a hundred and five days from a December year end", "2025-12-31", 105, "2026-04-15"},
		{"a hundred and twenty days from a December year end", "2025-12-31", 120, "2026-04-30"},
	}
	for _, c := range cases {
		if got := d(t, c.from).AddDays(c.add); got.String() != c.want {
			t.Errorf("%s: %s.AddDays(%d) = %s, want %s", c.name, c.from, c.add, got, c.want)
		}
	}
}

func TestAddMonthsClampsToTheEndOfTheTargetMonth(t *testing.T) {
	cases := []struct {
		from string
		add  int
		want string
	}{
		{"2026-01-31", 1, "2026-02-28"},  // no 31 February, and no 29 in 2026
		{"2024-01-31", 1, "2024-02-29"},  // leap year
		{"2026-03-31", 1, "2026-04-30"},  // April has 30 days
		{"2026-08-31", 1, "2026-09-30"},  // September has 30 days
		{"2026-01-31", 3, "2026-04-30"},  // still clamped three months out
		{"2026-01-15", 1, "2026-02-15"},  // nothing to clamp
		{"2026-12-31", 1, "2027-01-31"},  // over the year end
		{"2026-12-31", 2, "2027-02-28"},  // over the year end and clamped
		{"2026-03-31", -1, "2026-02-28"}, // backwards, clamped
		{"2026-01-15", -1, "2025-12-15"}, // backwards over the year end
		{"2026-06-15", 0, "2026-06-15"},
		{"2026-01-31", 13, "2027-02-28"},
	}
	for _, c := range cases {
		if got := d(t, c.from).AddMonths(c.add); got.String() != c.want {
			t.Errorf("%s.AddMonths(%d) = %s, want %s", c.from, c.add, got, c.want)
		}
	}
}

func TestAddYearsClampsTheLeapDay(t *testing.T) {
	cases := []struct {
		from string
		add  int
		want string
	}{
		{"2024-02-29", 1, "2025-02-28"},
		{"2024-02-29", 4, "2028-02-29"},
		{"2024-02-29", -1, "2023-02-28"},
		{"2026-06-15", 1, "2027-06-15"},
		{"2025-12-31", 1, "2026-12-31"},
	}
	for _, c := range cases {
		if got := d(t, c.from).AddYears(c.add); got.String() != c.want {
			t.Errorf("%s.AddYears(%d) = %s, want %s", c.from, c.add, got, c.want)
		}
	}
}

func TestAnniversaryInClampsTheLeapDay(t *testing.T) {
	cases := []struct {
		from string
		year int
		want string
	}{
		{"2016-02-29", 2026, "2026-02-28"},
		{"2016-02-29", 2028, "2028-02-29"},
		{"2011-07-04", 2026, "2026-07-04"},
		{"2011-12-31", 2026, "2026-12-31"},
	}
	for _, c := range cases {
		if got := d(t, c.from).AnniversaryIn(c.year); got.String() != c.want {
			t.Errorf("%s.AnniversaryIn(%d) = %s, want %s", c.from, c.year, got, c.want)
		}
	}
}

func TestDaysUntilCountsBothWaysAndOverLeapDays(t *testing.T) {
	cases := []struct {
		from, to string
		want     int
	}{
		{"2026-06-15", "2026-06-15", 0},
		{"2026-06-15", "2026-06-16", 1},
		{"2026-06-16", "2026-06-15", -1},
		{"2024-02-28", "2024-03-01", 2}, // leap year: one extra day
		{"2023-02-28", "2023-03-01", 1}, // ordinary year
		{"2026-01-01", "2027-01-01", 365},
		{"2024-01-01", "2025-01-01", 366}, // leap year
	}
	for _, c := range cases {
		if got := d(t, c.from).DaysUntil(d(t, c.to)); got != c.want {
			t.Errorf("%s.DaysUntil(%s) = %d, want %d", c.from, c.to, got, c.want)
		}
	}
}

func TestMonthsStartedCountsStartedMonthsOfDelay(t *testing.T) {
	cases := []struct {
		from, to string
		want     int
	}{
		{"2026-06-15", "2026-06-15", 0},
		{"2026-06-15", "2026-06-14", 0},
		{"2026-06-15", "2026-06-16", 1},
		{"2026-06-15", "2026-07-15", 1},
		{"2026-06-15", "2026-07-16", 2},
		{"2026-01-31", "2026-02-28", 1}, // clamped month end, still one month
		{"2026-01-31", "2026-03-01", 2},
		{"2026-06-15", "2027-06-15", 12},
		{"2026-06-15", "2027-06-16", 13},
	}
	for _, c := range cases {
		if got := d(t, c.from).MonthsStarted(d(t, c.to)); got != c.want {
			t.Errorf("%s.MonthsStarted(%s) = %d, want %d", c.from, c.to, got, c.want)
		}
	}
}

func TestWorkingDayWalksPastWeekendsAndRecordedHolidays(t *testing.T) {
	holidays := Holidays{
		"2026-06-12": "Independence Day",
		"2026-04-02": "Maundy Thursday",
		"2026-04-03": "Good Friday",
	}
	cases := []struct {
		name          string
		day           string
		wantPreceding string
		wantNext      string
	}{
		{"an ordinary Monday", "2026-06-15", "2026-06-15", "2026-06-15"},
		{"a Saturday", "2026-05-30", "2026-05-29", "2026-06-01"},
		{"a Sunday", "2026-05-31", "2026-05-29", "2026-06-01"},
		{"a Friday holiday", "2026-06-12", "2026-06-11", "2026-06-15"},
		{"Good Friday, with Thursday also out", "2026-04-03", "2026-04-01", "2026-04-06"},
		{"Easter Sunday after a long weekend", "2026-04-05", "2026-04-01", "2026-04-06"},
	}
	for _, c := range cases {
		if got := holidays.PrecedingWorkingDay(d(t, c.day)); got.String() != c.wantPreceding {
			t.Errorf("%s: PrecedingWorkingDay(%s) = %s, want %s", c.name, c.day, got, c.wantPreceding)
		}
		if got := holidays.NextWorkingDay(d(t, c.day)); got.String() != c.wantNext {
			t.Errorf("%s: NextWorkingDay(%s) = %s, want %s", c.name, c.day, got, c.wantNext)
		}
	}
}

func TestWorkingDayWithNoHolidayListStillSkipsWeekends(t *testing.T) {
	var none Holidays
	if got := none.PrecedingWorkingDay(d(t, "2026-05-30")); got.String() != "2026-05-29" {
		t.Errorf("PrecedingWorkingDay with no list = %s, want 2026-05-29", got)
	}
	if none.IsWorkingDay(d(t, "2026-05-31")) {
		t.Error("a Sunday should not be a working day even with no holiday list")
	}
}

func TestDateJSONRoundTrip(t *testing.T) {
	type holder struct {
		When Date `json:"when"`
	}
	raw, err := json.Marshal(holder{When: d(t, "2026-06-15")})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `{"when":"2026-06-15"}` {
		t.Errorf("marshal = %s, want {\"when\":\"2026-06-15\"}", raw)
	}

	var back holder
	if err := json.Unmarshal([]byte(`{"when":"2026-06-15"}`), &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.When != d(t, "2026-06-15") {
		t.Errorf("unmarshal gave %s", back.When)
	}

	var empty holder
	if err := json.Unmarshal([]byte(`{"when":null}`), &empty); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if !empty.When.IsZero() {
		t.Errorf("null should give the zero date, got %s", empty.When)
	}

	var bad holder
	if err := json.Unmarshal([]byte(`{"when":"2026-02-30"}`), &bad); err == nil {
		t.Error("a date that does not exist should not unmarshal")
	}
}

func TestValidRejectsImpossibleDates(t *testing.T) {
	cases := []struct {
		date Date
		want bool
	}{
		{NewDate(2026, time.June, 15), true},
		{NewDate(2024, time.February, 29), true},
		{NewDate(2026, time.February, 29), false},
		{NewDate(2026, time.April, 31), false},
		{NewDate(2026, time.Month(13), 1), false},
		{Date{}, false},
	}
	for _, c := range cases {
		if got := c.date.Valid(); got != c.want {
			t.Errorf("Valid(%04d-%02d-%02d) = %v, want %v", c.date.Year, c.date.Month, c.date.Day, got, c.want)
		}
	}
}

func TestEndOfMonth(t *testing.T) {
	cases := []struct{ from, want string }{
		{"2026-02-10", "2026-02-28"},
		{"2024-02-10", "2024-02-29"},
		{"2026-06-01", "2026-06-30"},
		{"2026-12-31", "2026-12-31"},
	}
	for _, c := range cases {
		if got := d(t, c.from).EndOfMonth(); got.String() != c.want {
			t.Errorf("%s.EndOfMonth() = %s, want %s", c.from, got, c.want)
		}
	}
}

package secfiling

import (
	"strings"
	"testing"
)

// company builds a plain domestic stock corporation with a 31 December fiscal
// year end, which tests then vary.
func company() *Company {
	return &Company{
		ID:                 "c1",
		Name:               "Test Corporation",
		RegistrationNumber: "CS201812345",
		EntityType:         EntityStock,
		FilerClass:         FilerOrdinary,
		FiscalYearEndMonth: 12,
		FiscalYearEndDay:   31,
		GISBasis:           GISFromMeeting,
		Meetings:           []Meeting{},
	}
}

func settings() Settings {
	return Settings{Holidays: Holidays{}, Ladder: DefaultLadder()}
}

// ───────────────────────── General Information Sheet ─────────────────────────

func TestGISDeadlineIsThirtyCalendarDaysFromTheMeeting(t *testing.T) {
	cases := []struct {
		name    string
		meeting string
		want    string
	}{
		{"an ordinary meeting", "2026-04-20", "2026-05-20"},
		{"thirty days crosses a month end", "2026-06-15", "2026-07-15"},
		{"a January meeting in a leap year", "2024-01-31", "2024-03-01"},
		{"the same date in an ordinary year lands a day later", "2023-01-31", "2023-03-02"},
		{"a December meeting runs into the next year", "2026-12-15", "2027-01-14"},
		{"a meeting on the last day of February in a leap year", "2024-02-29", "2024-03-30"},
		{"a meeting on 31 December", "2026-12-31", "2027-01-30"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			co := company()
			meeting := d(t, c.meeting)
			co.Meetings = []Meeting{{Year: meeting.Year, Date: meeting, Held: true}}

			got := GISDeadline(co, settings(), meeting.Year)
			if !got.Known {
				t.Fatalf("the deadline should be known once the meeting is held: %s", got.Reason)
			}
			if got.Due.String() != c.want {
				t.Errorf("meeting %s gives %s, want %s", c.meeting, got.Due, c.want)
			}
			if got.Provisional {
				t.Error("a meeting marked held is not provisional")
			}
			if !strings.Contains(got.Source, "Section 5(a)") {
				t.Errorf("a stock corporation's GIS cites Section 5(a), got %q", got.Source)
			}
			if !strings.Contains(got.Basis, c.meeting) {
				t.Errorf("the basis should name the meeting date, got %q", got.Basis)
			}
		})
	}
}

func TestGISDeadlineDerivesFromEachCompanysOwnMeeting(t *testing.T) {
	// Two corporations, same everything except the date they met. The whole
	// point of the GIS rule is that their deadlines differ.
	early, late := company(), company()
	early.ID, late.ID = "early", "late"
	early.Meetings = []Meeting{{Year: 2026, Date: d(t, "2026-03-10"), Held: true}}
	late.Meetings = []Meeting{{Year: 2026, Date: d(t, "2026-09-28"), Held: true}}

	a := GISDeadline(early, settings(), 2026)
	b := GISDeadline(late, settings(), 2026)
	if a.Due.String() != "2026-04-09" {
		t.Errorf("early meeting gives %s, want 2026-04-09", a.Due)
	}
	if b.Due.String() != "2026-10-28" {
		t.Errorf("late meeting gives %s, want 2026-10-28", b.Due)
	}
	if a.Due == b.Due {
		t.Error("two corporations that met six months apart cannot share a GIS deadline")
	}
}

func TestGISNonStockCorporationCitesTheMembersMeeting(t *testing.T) {
	co := company()
	co.EntityType = EntityNonStock
	co.Meetings = []Meeting{{Year: 2026, Date: d(t, "2026-05-05"), Held: true}}

	got := GISDeadline(co, settings(), 2026)
	if got.Due.String() != "2026-06-04" {
		t.Errorf("due = %s, want 2026-06-04", got.Due)
	}
	if !strings.Contains(got.Source, "Section 5(b)") {
		t.Errorf("a non-stock corporation cites Section 5(b), got %q", got.Source)
	}
	if !strings.Contains(got.Basis, "members") {
		t.Errorf("the basis should name the members' meeting, got %q", got.Basis)
	}
}

func TestGISWithoutAMeetingHasNoDeadlineAndSaysWhy(t *testing.T) {
	got := GISDeadline(company(), settings(), 2026)
	if got.Known {
		t.Fatalf("there is no deadline before the meeting, got %s", got.Due)
	}
	if !got.Due.IsZero() {
		t.Errorf("an unknown deadline must not carry a date, got %s", got.Due)
	}
	if !strings.Contains(got.Reason, "actually held") {
		t.Errorf("the reason should say the clock starts at the meeting, got %q", got.Reason)
	}
}

func TestGISFromAPlannedMeetingIsProvisional(t *testing.T) {
	co := company()
	co.Meetings = []Meeting{{Year: 2026, Date: d(t, "2026-07-01"), Held: false}}

	got := GISDeadline(co, settings(), 2026)
	if !got.Known || got.Due.String() != "2026-07-31" {
		t.Fatalf("due = %s (known %v), want 2026-07-31", got.Due, got.Known)
	}
	if !got.Provisional {
		t.Error("a meeting that has not been held gives a provisional date")
	}
	if !strings.Contains(got.Reason, "moves with the meeting") {
		t.Errorf("the reason should warn that the date moves, got %q", got.Reason)
	}
}

func TestGISForAForeignCorporationRunsFromTheLicenceAnniversary(t *testing.T) {
	cases := []struct {
		name    string
		licence string
		year    int
		want    string
	}{
		{"an ordinary anniversary", "2011-07-04", 2026, "2026-08-03"},
		{"a leap-day licence in an ordinary year", "2016-02-29", 2026, "2026-03-30"},
		{"a leap-day licence in a leap year", "2016-02-29", 2028, "2028-03-30"},
		{"a December licence runs into the next year", "2009-12-20", 2026, "2027-01-19"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			co := company()
			co.EntityType = EntityBranch
			co.GISBasis = GISFromLicenceAnniversary
			co.ReckoningDate = d(t, c.licence)

			got := GISDeadline(co, settings(), c.year)
			if !got.Known {
				t.Fatalf("a licence anniversary always gives a deadline: %s", got.Reason)
			}
			if got.Due.String() != c.want {
				t.Errorf("licence %s in %d gives %s, want %s", c.licence, c.year, got.Due, c.want)
			}
			if !strings.Contains(got.Source, "Section 5(c)") {
				t.Errorf("a foreign corporation cites Section 5(c), got %q", got.Source)
			}
		})
	}
}

func TestGISForAnOPCSaysTheCircularDoesNotNameIt(t *testing.T) {
	co := company()
	co.EntityType = EntityOPC
	co.GISBasis = GISFromIncorporationAnniversary
	co.ReckoningDate = d(t, "2021-09-09")

	got := GISDeadline(co, settings(), 2026)
	if got.Due.String() != "2026-10-09" {
		t.Errorf("due = %s, want 2026-10-09", got.Due)
	}
	if !got.Indicative {
		t.Error("an OPC's reckoning is not stated in Section 5, so it must be marked indicative")
	}
	if !strings.Contains(got.Reason, "One Person Corporation") {
		t.Errorf("the reason should say the circular does not name the OPC, got %q", got.Reason)
	}
}

// ───────────────────────── Audited Financial Statements ──────────────────────

func TestAFSDeadlineUsesThePublishedDateForADecemberYearEnd(t *testing.T) {
	cases := []struct {
		name       string
		class      FilerClass
		wantReq    Requirement
		wantDue    string
		wantSuper  string
		wantSource string
	}{
		{"an ordinary filer", FilerOrdinary, RequirementAFS, "2026-06-15", "2026-05-29", "Notice dated 14 April 2026"},
		{"a broker or dealer", FilerBrokerDealer, RequirementForm52AR, "2026-05-15", "2026-04-30", "Notice dated 14 April 2026"},
		{"a listed issuer", FilerListedOrPublic, RequirementForm17A, "2026-05-15", "2026-04-15", "Notice dated 14 April 2026"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			co := company()
			co.FilerClass = c.class

			got := AFSDeadline(co, settings(), 2026)
			if got.Requirement != c.wantReq {
				t.Errorf("requirement = %s, want %s", got.Requirement, c.wantReq)
			}
			if !got.Known || got.Due.String() != c.wantDue {
				t.Fatalf("due = %s (known %v), want %s", got.Due, got.Known, c.wantDue)
			}
			if !strings.Contains(got.Source, c.wantSource) {
				t.Errorf("source = %q, want it to name the %s", got.Source, c.wantSource)
			}
			if !strings.Contains(got.Reason, c.wantSuper) {
				t.Errorf("the reason should record the date it replaced (%s), got %q", c.wantSuper, got.Reason)
			}
			if !got.Indicative {
				t.Error("a published date is still indicative until the filer confirms it")
			}
			if got.Period != "fiscal year ended 2025-12-31" {
				t.Errorf("period = %q", got.Period)
			}
		})
	}
}

func TestTheRegistrationNumberDoesNotMoveTheAFSDeadline(t *testing.T) {
	// Filing seasons used to stagger the AFS by the last digit of the
	// registration number. Under the circular this build was written against
	// they all share one date, and the tool must not reintroduce the digit.
	var due []string
	for digit := 0; digit <= 9; digit++ {
		co := company()
		co.RegistrationNumber = strings.Repeat("9", 10) + string(rune('0'+digit))
		got := AFSDeadline(co, settings(), 2026)
		if !got.Known {
			t.Fatalf("digit %d: no deadline", digit)
		}
		due = append(due, got.Due.String())
		if co.LastDigit() != digit {
			t.Errorf("LastDigit() = %d, want %d", co.LastDigit(), digit)
		}
		note := RegistrationNumberNote(co)
		if !strings.Contains(note, "does not set the filing window") {
			t.Errorf("the note should say the digit does not set the window, got %q", note)
		}
	}
	for _, got := range due {
		if got != due[0] {
			t.Fatalf("the AFS deadlines differ by last digit: %v", due)
		}
	}
}

func TestAFSDeadlineForASeasonWithNoPublishedScheduleIsNotGuessed(t *testing.T) {
	got := AFSDeadline(company(), settings(), 2027)
	if got.Known {
		t.Fatalf("the 2027 schedule is not in this build, yet a date was given: %s", got.Due)
	}
	if !got.Due.IsZero() {
		t.Errorf("an unknown deadline must not carry a date, got %s", got.Due)
	}
	for _, want := range []string{"not carried forward", AFSDueFY2025Dec31Original, AFSDueFY2025Dec31, "Settings"} {
		if !strings.Contains(got.Reason, want) {
			t.Errorf("the reason should mention %q, got %q", want, got.Reason)
		}
	}
}

func TestAFSDeadlineForANonDecemberYearEndCountsCalendarDays(t *testing.T) {
	cases := []struct {
		name       string
		class      FilerClass
		fyeMonth   int
		fyeDay     int
		filingYear int
		wantFYE    string
		wantDue    string
	}{
		{"a June year end, ordinary filer", FilerOrdinary, 6, 30, 2026, "2026-06-30", "2026-10-28"},
		{"a March year end, ordinary filer", FilerOrdinary, 3, 31, 2026, "2026-03-31", "2026-07-29"},
		{"an October year end falls due the next year", FilerOrdinary, 10, 31, 2026, "2025-10-31", "2026-02-28"},
		{"a June year end, listed issuer at 105 days", FilerListedOrPublic, 6, 30, 2026, "2026-06-30", "2026-10-13"},
		{"a June year end, broker at 110 days", FilerBrokerDealer, 6, 30, 2026, "2026-06-30", "2026-10-18"},
		{"a leap-day year end clamps in an ordinary year", FilerOrdinary, 2, 29, 2026, "2026-02-28", "2026-06-28"},
		{"a leap-day year end stands in a leap year", FilerOrdinary, 2, 29, 2028, "2028-02-29", "2028-06-28"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			co := company()
			co.FilerClass = c.class
			co.FiscalYearEndMonth, co.FiscalYearEndDay = c.fyeMonth, c.fyeDay

			if got := FiscalYearEndForFilingYear(co, c.filingYear); got.String() != c.wantFYE {
				t.Errorf("fiscal year end = %s, want %s", got, c.wantFYE)
			}
			got := AFSDeadline(co, settings(), c.filingYear)
			if !got.Known || got.Due.String() != c.wantDue {
				t.Fatalf("due = %s (known %v), want %s", got.Due, got.Known, c.wantDue)
			}
			if !got.Indicative {
				t.Error("a day-count rule restated each season is indicative")
			}
		})
	}
}

func TestAFSDeadlineForCOAAuditedCorporationsIsNotADate(t *testing.T) {
	co := company()
	co.FilerClass = FilerCOAAudited

	got := AFSDeadline(co, settings(), 2026)
	if got.Known {
		t.Fatalf("a COA-audited corporation is outside the dated schedule, got %s", got.Due)
	}
	if !strings.Contains(got.Source, "Section 2(c)") {
		t.Errorf("source = %q, want Section 2(c)", got.Source)
	}
	if !strings.Contains(got.Basis, "Commission on Audit") {
		t.Errorf("basis = %q", got.Basis)
	}
}

func TestAnOverrideReplacesTheShippedDeadline(t *testing.T) {
	s := settings()
	s.Overrides = []DeadlineOverride{
		{Requirement: RequirementAFS, FilingYear: 2027, Due: d(t, "2027-06-14"),
			Source: "SEC Memorandum Circular No. 4, Series of 2027"},
		{Requirement: RequirementGIS, FilingYear: 2026, CompanyID: "c1", Due: d(t, "2026-08-31"),
			Source: "letter from the Commission"},
	}

	afs := AFSDeadline(company(), s, 2027)
	if !afs.Known || afs.Due.String() != "2027-06-14" {
		t.Fatalf("the override should supply the 2027 deadline, got %s (known %v)", afs.Due, afs.Known)
	}
	if !afs.Overridden || !strings.Contains(afs.Source, "No. 4, Series of 2027") {
		t.Errorf("the override should be marked and sourced, got %+v", afs)
	}

	co := company()
	co.Meetings = []Meeting{{Year: 2026, Date: d(t, "2026-04-20"), Held: true}}
	gis := GISDeadline(co, s, 2026)
	if gis.Due.String() != "2026-08-31" || !gis.Overridden {
		t.Errorf("the company-specific GIS override should win, got %s (overridden %v)", gis.Due, gis.Overridden)
	}
}

func TestAnOverrideForAnotherCompanyIsIgnored(t *testing.T) {
	s := settings()
	s.Overrides = []DeadlineOverride{
		{Requirement: RequirementAFS, FilingYear: 2026, CompanyID: "someone-else",
			Due: d(t, "2026-01-01"), Source: "not ours"},
	}
	got := AFSDeadline(company(), s, 2026)
	if got.Due.String() != "2026-06-15" {
		t.Errorf("another company's override leaked in: %s", got.Due)
	}
}

func TestLastSafeDayStepsBackOffNonWorkingDays(t *testing.T) {
	cases := []struct {
		name     string
		due      string
		holidays Holidays
		wantSafe string
		wantNote bool
	}{
		{"a Monday deadline stands", "2026-06-15", Holidays{}, "2026-06-15", false},
		{"a Saturday deadline", "2026-05-30", Holidays{}, "2026-05-29", true},
		{"a Sunday deadline", "2026-05-31", Holidays{}, "2026-05-29", true},
		{"a holiday deadline", "2026-06-12", Holidays{"2026-06-12": "Independence Day"}, "2026-06-11", true},
		{"a holiday on a Monday after a weekend", "2026-06-15",
			Holidays{"2026-06-15": "special non-working day"}, "2026-06-12", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			co := company()
			due := d(t, c.due)
			co.Meetings = []Meeting{{Year: due.Year, Date: due.AddDays(-GISDaysAfterReckoning), Held: true}}
			s := settings()
			s.Holidays = c.holidays

			got := GISDeadline(co, s, due.Year)
			if got.Due.String() != c.due {
				t.Fatalf("due = %s, want %s", got.Due, c.due)
			}
			if got.LastSafeDay.String() != c.wantSafe {
				t.Errorf("last safe day = %s, want %s", got.LastSafeDay, c.wantSafe)
			}
			if (got.SafeDayNote != "") != c.wantNote {
				t.Errorf("note = %q, want a note: %v", got.SafeDayNote, c.wantNote)
			}
		})
	}
}

func TestPublishedScheduleIsConsistentWithTheNamedConstants(t *testing.T) {
	want := map[Requirement]struct{ due, superseded string }{
		RequirementAFS:      {AFSDueFY2025Dec31, AFSDueFY2025Dec31Original},
		RequirementForm52AR: {Form52ARDueFY2025Dec31, Form52ARDueFY2025Dec31Original},
		RequirementForm17A:  {Form17ADueFY2025Dec31, Form17ADueFY2025Dec31Original},
	}
	schedule := PublishedSchedule()
	if len(schedule) != len(want) {
		t.Fatalf("the published schedule holds %d rows, want %d", len(schedule), len(want))
	}
	for _, row := range schedule {
		w, ok := want[row.Requirement]
		if !ok {
			t.Fatalf("unexpected row for %s", row.Requirement)
		}
		if row.Due.String() != w.due || row.SupersededDue.String() != w.superseded {
			t.Errorf("%s: due %s (was %s), want %s (was %s)",
				row.Requirement, row.Due, row.SupersededDue, w.due, w.superseded)
		}
		if row.FiscalYearEnd.String() != "2025-12-31" {
			t.Errorf("%s: fiscal year end %s", row.Requirement, row.FiscalYearEnd)
		}
		if row.Source == "" || row.SupersededBy == "" {
			t.Errorf("%s: every published date needs the issuance it came from", row.Requirement)
		}
	}
	// The 105-day rule and the published Form 17-A date must agree for a
	// 31 December 2025 year end, or one of them has been mistyped.
	if got := d(t, "2025-12-31").AddDays(AFSDaysListedOrPublicFYE).String(); got != Form17ADueFY2025Dec31Original {
		t.Errorf("%d days after 2025-12-31 is %s, but the circular's original Form 17-A date is %s",
			AFSDaysListedOrPublicFYE, got, Form17ADueFY2025Dec31Original)
	}
}

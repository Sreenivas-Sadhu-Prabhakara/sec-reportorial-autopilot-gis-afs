package secfiling

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPutCompanyRejectsWhatCannotBeActedOn(t *testing.T) {
	good := func() Company { return *company() }
	cases := []struct {
		name    string
		change  func(*Company)
		wantErr string
	}{
		{"a company with everything", func(*Company) {}, ""},
		{"no reference", func(c *Company) { c.ID = "  " }, "reference of its own"},
		{"no name", func(c *Company) { c.Name = "" }, "needs a name"},
		{"an unknown registrant", func(c *Company) { c.EntityType = "charity" }, "kind of registrant"},
		{"an unknown filer class", func(c *Company) { c.FilerClass = "offshore" }, "filer class"},
		{"an unknown GIS basis", func(c *Company) { c.GISBasis = "whenever" }, "reckoning the GIS"},
		{"a thirteenth month", func(c *Company) { c.FiscalYearEndMonth = 13 }, "between 1 and 12"},
		{"a 31 February year end", func(c *Company) {
			c.FiscalYearEndMonth, c.FiscalYearEndDay = 2, 31
		}, "cannot end on day 31"},
		{"a 31 April year end", func(c *Company) {
			c.FiscalYearEndMonth, c.FiscalYearEndDay = 4, 31
		}, "cannot end on day 31"},
		{"a 29 February year end is allowed", func(c *Company) {
			c.FiscalYearEndMonth, c.FiscalYearEndDay = 2, 29
		}, ""},
		{"a licence anniversary with no licence date", func(c *Company) {
			c.EntityType, c.GISBasis = EntityBranch, GISFromLicenceAnniversary
		}, "needs that date recorded"},
		{"a licence anniversary with a licence date", func(c *Company) {
			c.EntityType, c.GISBasis = EntityBranch, GISFromLicenceAnniversary
			c.ReckoningDate = NewDate(2011, time.July, 4)
		}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := New()
			subject := good()
			c.change(&subject)
			err := b.PutCompany(subject)
			switch {
			case c.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case c.wantErr == "":
				if len(b.Companies) != 1 {
					t.Fatalf("%d companies saved, want 1", len(b.Companies))
				}
			case err == nil:
				t.Fatalf("expected an error mentioning %q", c.wantErr)
			case !strings.Contains(err.Error(), c.wantErr):
				t.Fatalf("error %q, want it to mention %q", err, c.wantErr)
			}
		})
	}
}

func TestPutCompanyKeepsTheMeetingsAlreadyRecorded(t *testing.T) {
	b, _ := book(t)
	edited := *company()
	edited.Name = "Renamed Corporation"
	if err := b.PutCompany(edited); err != nil {
		t.Fatalf("PutCompany: %v", err)
	}
	c, ok := b.Company("c1")
	if !ok {
		t.Fatal("the company disappeared")
	}
	if c.Name != "Renamed Corporation" {
		t.Errorf("name = %q", c.Name)
	}
	if len(c.Meetings) != 1 || c.Meetings[0].Date.String() != "2026-04-20" {
		t.Errorf("editing a company lost its meetings: %+v", c.Meetings)
	}
}

func TestDeleteCompanyTakesItsFilingsWithIt(t *testing.T) {
	b, _ := book(t)
	if err := b.PutFiling(Filing{
		ID: "f1", CompanyID: "c1", Requirement: RequirementGIS, FilingYear: 2026,
		FiledOn: NewDate(2026, time.May, 12), Reference: "EFAST-1",
	}); err != nil {
		t.Fatalf("PutFiling: %v", err)
	}
	if err := b.DeleteCompany("c1"); err != nil {
		t.Fatalf("DeleteCompany: %v", err)
	}
	if len(b.Companies) != 0 || len(b.Filings) != 0 {
		t.Errorf("%d companies and %d filings left", len(b.Companies), len(b.Filings))
	}
	if err := b.DeleteCompany("c1"); err == nil {
		t.Error("deleting a company twice should fail")
	}
}

func TestPutMeetingChecksTheYearMatchesTheDate(t *testing.T) {
	b, _ := book(t)
	if err := b.PutMeeting("c1", Meeting{Year: 2025, Date: NewDate(2026, time.April, 20), Held: true}); err == nil {
		t.Error("a 2026 date cannot be the 2025 meeting")
	}
	if err := b.PutMeeting("nobody", Meeting{Year: 2026, Date: NewDate(2026, time.April, 20)}); err == nil {
		t.Error("a meeting for a company that does not exist should fail")
	}
	if err := b.PutMeeting("c1", Meeting{Year: 2026, Date: NewDate(2026, time.February, 30)}); err == nil {
		t.Error("a date that does not exist should fail")
	}

	// Recording the same year twice replaces it rather than doubling it.
	if err := b.PutMeeting("c1", Meeting{Year: 2026, Date: NewDate(2026, time.May, 4), Held: true}); err != nil {
		t.Fatalf("PutMeeting: %v", err)
	}
	c, _ := b.Company("c1")
	if len(c.Meetings) != 1 || c.Meetings[0].Date.String() != "2026-05-04" {
		t.Errorf("meetings = %+v, want one on 2026-05-04", c.Meetings)
	}
	if err := b.DeleteMeeting("c1", 2026); err != nil {
		t.Fatalf("DeleteMeeting: %v", err)
	}
	if err := b.DeleteMeeting("c1", 2026); err == nil {
		t.Error("deleting a meeting twice should fail")
	}
}

func TestPutFilingInsistsOnProof(t *testing.T) {
	b, _ := book(t)
	cases := []struct {
		name    string
		filing  Filing
		wantErr string
	}{
		{"a complete record", Filing{
			ID: "f1", CompanyID: "c1", Requirement: RequirementGIS, FilingYear: 2026,
			FiledOn: NewDate(2026, time.May, 12), Reference: "EFAST-2026-0001",
		}, ""},
		{"no reference to prove it", Filing{
			ID: "f2", CompanyID: "c1", Requirement: RequirementGIS, FilingYear: 2026,
			FiledOn: NewDate(2026, time.May, 12),
		}, "proof it was filed"},
		{"no company", Filing{
			ID: "f3", CompanyID: "ghost", Requirement: RequirementGIS,
			FiledOn: NewDate(2026, time.May, 12), Reference: "EFAST-1",
		}, "no company"},
		{"a filing this tool does not track", Filing{
			ID: "f4", CompanyID: "c1", Requirement: "form-x",
			FiledOn: NewDate(2026, time.May, 12), Reference: "EFAST-1",
		}, "not a filing"},
		{"a date that does not exist", Filing{
			ID: "f5", CompanyID: "c1", Requirement: RequirementGIS,
			FiledOn: NewDate(2026, time.February, 30), Reference: "EFAST-1",
		}, "not a real date"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := b.PutFiling(c.filing)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error = %v, want it to mention %q", err, c.wantErr)
			}
		})
	}

	// The channel defaults to the Commission's electronic filing tool, and a
	// second record for the same period replaces the first.
	saved := b.Filings[0]
	if saved.Channel != "eFAST" {
		t.Errorf("channel = %q, want eFAST", saved.Channel)
	}
	if err := b.PutFiling(Filing{
		ID: "f9", CompanyID: "c1", Requirement: RequirementGIS, FilingYear: 2026,
		FiledOn: NewDate(2026, time.May, 14), Reference: "EFAST-2026-0002",
	}); err != nil {
		t.Fatalf("PutFiling: %v", err)
	}
	if len(b.Filings) != 1 {
		t.Fatalf("%d filings, want the same period replaced rather than duplicated", len(b.Filings))
	}
	got, ok := b.FilingFor("c1", RequirementGIS, 2026)
	if !ok || got.Reference != "EFAST-2026-0002" {
		t.Errorf("filing = %+v", got)
	}
	if err := b.DeleteFiling(got.ID); err != nil {
		t.Fatalf("DeleteFiling: %v", err)
	}
	if err := b.DeleteFiling(got.ID); err == nil {
		t.Error("deleting a filing twice should fail")
	}
}

func TestReminderLadderMustDescend(t *testing.T) {
	cases := []struct {
		name    string
		ladder  ReminderLadder
		wantErr string
	}{
		{"the default", DefaultLadder(), ""},
		{"a tighter ladder", ReminderLadder{30, 20, 10, 5, 2}, ""},
		{"a step that does not descend", ReminderLadder{90, 60, 60, 14, 3}, "closer to the deadline"},
		{"a step the wrong way round", ReminderLadder{90, 60, 30, 40, 3}, "closer to the deadline"},
		{"a zero step", ReminderLadder{90, 60, 30, 14, 0}, "at least 1 day"},
		{"a negative step", ReminderLadder{-1, 60, 30, 14, 3}, "at least 1 day"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.ladder.Validate()
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error = %v, want it to mention %q", err, c.wantErr)
			}
		})
	}
}

func TestPutSettingsInsistsOnSources(t *testing.T) {
	base := func() Settings {
		return Settings{Holidays: Holidays{}, Ladder: DefaultLadder()}
	}
	cases := []struct {
		name    string
		change  func(*Settings)
		wantErr string
	}{
		{"the defaults", func(*Settings) {}, ""},
		{"a holiday", func(s *Settings) { s.Holidays = Holidays{"2026-06-12": "Independence Day"} }, ""},
		{"a holiday on a date that does not exist", func(s *Settings) {
			s.Holidays = Holidays{"2026-02-30": "nope"}
		}, "holiday list"},
		{"an override with its issuance", func(s *Settings) {
			s.Overrides = []DeadlineOverride{{Requirement: RequirementAFS, FilingYear: 2027,
				Due: NewDate(2027, time.June, 14), Source: "SEC MC No. 4, s. 2027"}}
		}, ""},
		{"an override with no issuance", func(s *Settings) {
			s.Overrides = []DeadlineOverride{{Requirement: RequirementAFS, FilingYear: 2027,
				Due: NewDate(2027, time.June, 14)}}
		}, "which issuance"},
		{"an override with no year", func(s *Settings) {
			s.Overrides = []DeadlineOverride{{Requirement: RequirementAFS,
				Due: NewDate(2027, time.June, 14), Source: "SEC MC No. 4, s. 2027"}}
		}, "filing year"},
		{"an override with no date", func(s *Settings) {
			s.Overrides = []DeadlineOverride{{Requirement: RequirementAFS, FilingYear: 2027,
				Source: "SEC MC No. 4, s. 2027"}}
		}, "the deadline the Commission set"},
		{"penalty figures with their issuance", func(s *Settings) {
			s.Penalties = PenaltyFigures{Source: "SEC MC No. 6, s. 2024", BasicLate: 5000}
		}, ""},
		{"penalty figures with no issuance", func(s *Settings) {
			s.Penalties = PenaltyFigures{BasicLate: 5000}
		}, "which issuance these amounts came from"},
		{"a negative figure", func(s *Settings) {
			s.Penalties = PenaltyFigures{Source: "somewhere", BasicLate: -1}
		}, "cannot be negative"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := New()
			s := base()
			c.change(&s)
			err := b.PutSettings(s)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error = %v, want it to mention %q", err, c.wantErr)
			}
		})
	}
}

func TestNormaliseRepairsAHandEditedFile(t *testing.T) {
	raw := `{"companies":[{"id":"x","name":"Zeta Inc"},{"id":"y","name":"Alpha Inc"}],
	         "settings":{"reminderLadder":{"planDays":1,"prepareDays":99}}}`
	var b Book
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b.Normalise()

	if b.Filings == nil || b.Settings.Holidays == nil || b.Settings.Overrides == nil {
		t.Error("the empty collections should be filled in rather than left nil")
	}
	if b.Settings.Ladder != DefaultLadder() {
		t.Errorf("a ladder that does not descend should fall back to the default, got %+v", b.Settings.Ladder)
	}
	if b.Companies[0].Name != "Alpha Inc" {
		t.Errorf("companies should be sorted by name, got %q first", b.Companies[0].Name)
	}
	for _, c := range b.Companies {
		if c.FiscalYearEndMonth != 12 || c.FiscalYearEndDay != 31 {
			t.Errorf("%s: fiscal year end should default to 31 December, got %d-%d", c.Name, c.FiscalYearEndMonth, c.FiscalYearEndDay)
		}
		if c.EntityType != EntityStock || c.FilerClass != FilerOrdinary || c.GISBasis != GISFromMeeting {
			t.Errorf("%s: defaults not applied: %+v", c.Name, c)
		}
	}
}

func TestLastDigitReadsPastLettersAndPunctuation(t *testing.T) {
	cases := []struct {
		number string
		want   int
	}{
		{"CS201812345", 5},
		{"2021010012340", 0},
		{"A199900007", 7},
		{"CN-2019-1234 ", 4},
		{"PENDING", -1},
		{"", -1},
	}
	for _, c := range cases {
		co := company()
		co.RegistrationNumber = c.number
		if got := co.LastDigit(); got != c.want {
			t.Errorf("LastDigit(%q) = %d, want %d", c.number, got, c.want)
		}
	}
}

func TestFiscalYearEndInClampsTheLeapDay(t *testing.T) {
	co := company()
	co.FiscalYearEndMonth, co.FiscalYearEndDay = 2, 29
	cases := []struct {
		year int
		want string
	}{
		{2024, "2024-02-29"},
		{2026, "2026-02-28"},
		{2100, "2100-02-28"}, // not a leap year
	}
	for _, c := range cases {
		if got := co.FiscalYearEndIn(c.year); got.String() != c.want {
			t.Errorf("FiscalYearEndIn(%d) = %s, want %s", c.year, got, c.want)
		}
	}
	if co.EndsInDecember() {
		t.Error("a February year end is not a December one")
	}
}

func TestDefaultGISBasisFollowsTheRegistrantType(t *testing.T) {
	cases := []struct {
		entity EntityType
		want   GISBasis
	}{
		{EntityStock, GISFromMeeting},
		{EntityNonStock, GISFromMeeting},
		{EntityPartnership, GISFromMeeting},
		{EntityOPC, GISFromIncorporationAnniversary},
		{EntityBranch, GISFromLicenceAnniversary},
		{EntityRepOffice, GISFromLicenceAnniversary},
		{EntityRegionalHQ, GISFromLicenceAnniversary},
		{EntityRegionalOHQ, GISFromLicenceAnniversary},
	}
	for _, c := range cases {
		if got := DefaultGISBasis(c.entity); got != c.want {
			t.Errorf("DefaultGISBasis(%s) = %s, want %s", c.entity, got, c.want)
		}
		if c.entity.IsForeign() != (c.want == GISFromLicenceAnniversary) {
			t.Errorf("%s: IsForeign() and the default basis disagree", c.entity)
		}
	}
}

func TestFilerClassPicksTheFinancialStatementFiling(t *testing.T) {
	cases := []struct {
		class FilerClass
		want  Requirement
	}{
		{FilerOrdinary, RequirementAFS},
		{FilerCOAAudited, RequirementAFS},
		{FilerListedOrPublic, RequirementForm17A},
		{FilerBrokerDealer, RequirementForm52AR},
	}
	for _, c := range cases {
		if got := c.class.Requirement(); got != c.want {
			t.Errorf("%s files %s, want %s", c.class, got, c.want)
		}
	}
}

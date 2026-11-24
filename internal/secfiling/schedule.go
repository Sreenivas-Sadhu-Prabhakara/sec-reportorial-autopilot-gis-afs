package secfiling

import "fmt"

// ───────────────────────────── what the rules say ─────────────────────────────
//
// Every figure the tool counts with is named here, once, with the issuance it
// came from and the date that issuance carries. Nothing in this package reads
// a number that is not in this block.
//
// The Commission re-issues the filing schedule every year, usually in the
// first quarter, and has revised it mid-season by notice. Treat everything
// below as indicative and confirm it against the Commission's own issuance
// before you file. All of it can be overridden from the Settings tab without
// rebuilding the tool.

const (
	// ScheduleAsOfDate is the day the schedule in this file was last checked
	// against the Commission's own issuances. A filing season later than this
	// is not covered, and the tool says so instead of guessing.
	ScheduleAsOfDate = "2026-11-24"

	// GISDaysAfterReckoning is the window for the General Information Sheet:
	// "within thirty (30) calendar days" from the reckoning date.
	// Source: SEC Memorandum Circular No. 9, Series of 2026, Section 5.
	GISDaysAfterReckoning = 30

	// AFSDaysOrdinaryNonDecemberFYE applies to a corporation whose fiscal year
	// ends on a date other than 31 December: the AFS is filed "within 120
	// calendar days from the end of their respective fiscal years".
	// Source: SEC Memorandum Circular No. 9, Series of 2026, Section 2(a).
	AFSDaysOrdinaryNonDecemberFYE = 120

	// AFSDaysBrokerDealerNonDecemberFYE applies to brokers and dealers whose
	// fiscal year ends on a date other than 31 December: SEC Form 52-AR is
	// filed 110 calendar days after the close of the fiscal year.
	// Source: SEC Memorandum Circular No. 9, Series of 2026, Section 2(a),
	// citing Rule 52.1.5.2 of the SRC-IRR.
	AFSDaysBrokerDealerNonDecemberFYE = 110

	// AFSDaysListedOrPublicFYE applies to issuers listed on the Philippine
	// Stock Exchange, issuers registered but not listed, public companies and
	// other entities covered by Section 17.2 of the Securities Regulation
	// Code: the AFS goes in within 105 calendar days after the end of the
	// fiscal year, attached to the Annual Report on SEC Form 17-A.
	// Source: SEC Memorandum Circular No. 9, Series of 2026, Section 2(b).
	AFSDaysListedOrPublicFYE = 105

	// DelinquencyMissCount and DelinquencyWindowYears are the statutory
	// trigger for delinquent status: failure to submit reportorial
	// requirements three times, consecutively or intermittently, within a
	// period of five years.
	// Source: Section 177, Revised Corporation Code of the Philippines
	// (Republic Act No. 11232). Guidelines: SEC Memorandum Circular No. 19,
	// Series of 2023.
	DelinquencyMissCount   = 3
	DelinquencyWindowYears = 5

	// ForeignLateThresholdDays and ForeignNonFilingThresholdDays are how the
	// fines circular reckons a foreign corporation's delay: late once the
	// filing is more than 30 calendar days after the deadline, and assessed as
	// a non-filing once it is beyond 60 calendar days.
	// Source: SEC Memorandum Circular No. 6, Series of 2024.
	ForeignLateThresholdDays       = 30
	ForeignNonFilingThresholdDays  = 60
	DomesticNonFilingThresholdYear = 1 // a domestic filing more than one year late is assessed as a non-filing

	// NonFilingMonthlyCapMonths is the cap on the per-month component of a
	// non-filing assessment.
	// Source: SEC Memorandum Circular No. 6, Series of 2024.
	NonFilingMonthlyCapMonths = 12

	// RevocationOffenceNumber is the point in the offence ladder at which the
	// Commission may revoke the certificate of registration or the licence to
	// transact business: a sixth offence after the corporation has been
	// notified of its delinquency.
	// Source: SEC Memorandum Circular No. 6, Series of 2024.
	RevocationOffenceNumber = 6

	// MonthlyPenaltySuspendedFrom and MonthlyPenaltySuspendedUntil bound the
	// suspension of the per-month component of the fines for late and
	// non-filed AFS and GIS. The basic fine is unaffected and stays payable,
	// amounts already settled are not refunded, and the monthly component
	// returns on the day after the end date.
	// Source: SEC Memorandum Circular No. 16, Series of 2026 (13 May 2026).
	MonthlyPenaltySuspendedFrom  = "2026-05-14"
	MonthlyPenaltySuspendedUntil = "2026-12-31"
)

// The dated deadlines the Commission published for the filing season that was
// current when this tool was written. Each one is a named constant so it can
// be found, quoted and replaced in one place.
const (
	// AFSDueFY2025Dec31Original is the AFS deadline first set for a fiscal
	// year ended 31 December 2025.
	// Source: SEC Memorandum Circular No. 9, Series of 2026, Section 1
	// (issued 11 February 2026).
	AFSDueFY2025Dec31Original = "2026-05-29"

	// AFSDueFY2025Dec31 is the extended AFS deadline that replaced it, granted
	// after the Bureau of Internal Revenue moved the annual income tax return
	// deadline to 15 May 2026.
	// Source: SEC Notice dated 14 April 2026, "Extension of the Deadline for
	// Filing of 2025 Annual Financial Statements (AFS)", revising the
	// schedule in SEC Memorandum Circular No. 9, Series of 2026.
	AFSDueFY2025Dec31 = "2026-06-15"

	// Form52ARDueFY2025Dec31Original and Form52ARDueFY2025Dec31 are the same
	// pair for brokers and dealers filing SEC Form 52-AR with the AFS
	// attached.
	Form52ARDueFY2025Dec31Original = "2026-04-30"
	Form52ARDueFY2025Dec31         = "2026-05-15"

	// Form17ADueFY2025Dec31Original and Form17ADueFY2025Dec31 are the same
	// pair for issuers and public companies filing the Annual Report on SEC
	// Form 17-A with the AFS attached.
	Form17ADueFY2025Dec31Original = "2026-04-15"
	Form17ADueFY2025Dec31         = "2026-05-15"
)

// TheRegistrationNumberDoesNotSetTheAFSWindow records, in one sentence, a
// finding that matters more than any constant above.
//
// Filing seasons in earlier years staggered the AFS over weeks according to
// the last digit of the SEC registration number. The circular governing the
// filing season this tool was written against does not: Section 1 of SEC
// Memorandum Circular No. 9, Series of 2026 gives every corporation with a
// 31 December fiscal year end the same date, and the Commission's notice of
// 14 April 2026 moved that one date for everybody at once.
//
// So this tool does not derive the AFS window from the registration number.
// It records the number, shows its last digit, and says plainly that the digit
// is not what sets the deadline. If a future season brings number coding back,
// it comes back as a published schedule and belongs in the override table, not
// in a digit formula carried over from a season that has ended.
const TheRegistrationNumberDoesNotSetTheAFSWindow = "Under SEC Memorandum Circular No. 9, Series of 2026, one AFS deadline applies to all corporations with a 31 December fiscal year end. The last digit of the SEC registration number does not set the filing window for this season."

// ScheduleAsOf is ScheduleAsOfDate as a date.
var ScheduleAsOf = MustParseDate(ScheduleAsOfDate)

// Requirement is one reportorial filing the tool tracks.
type Requirement string

const (
	// RequirementGIS is the General Information Sheet.
	RequirementGIS Requirement = "gis"
	// RequirementAFS is the Audited Financial Statements filed on their own.
	RequirementAFS Requirement = "afs"
	// RequirementForm17A is the Annual Report on SEC Form 17-A with the AFS
	// attached, filed by issuers and public companies.
	RequirementForm17A Requirement = "form17a"
	// RequirementForm52AR is SEC Form 52-AR with the AFS attached, filed by
	// brokers and dealers.
	RequirementForm52AR Requirement = "form52ar"
)

// Label is the requirement's name in the interface.
func (r Requirement) Label() string {
	switch r {
	case RequirementGIS:
		return "General Information Sheet"
	case RequirementAFS:
		return "Audited Financial Statements"
	case RequirementForm17A:
		return "Annual Report (SEC Form 17-A) with AFS"
	case RequirementForm52AR:
		return "SEC Form 52-AR with AFS"
	}
	return string(r)
}

// Short is the abbreviation used in tables.
func (r Requirement) Short() string {
	switch r {
	case RequirementGIS:
		return "GIS"
	case RequirementAFS:
		return "AFS"
	case RequirementForm17A:
		return "Form 17-A"
	case RequirementForm52AR:
		return "Form 52-AR"
	}
	return string(r)
}

// Valid reports whether the requirement is one the tool knows.
func (r Requirement) Valid() bool {
	switch r {
	case RequirementGIS, RequirementAFS, RequirementForm17A, RequirementForm52AR:
		return true
	}
	return false
}

// AllRequirements lists every requirement, in the order the interface shows
// them.
func AllRequirements() []Requirement {
	return []Requirement{RequirementGIS, RequirementAFS, RequirementForm17A, RequirementForm52AR}
}

// PublishedDeadline is one dated deadline taken from a Commission issuance,
// for one requirement and one fiscal year end.
type PublishedDeadline struct {
	Requirement   Requirement `json:"requirement"`
	FiscalYearEnd Date        `json:"fiscalYearEnd"`
	Due           Date        `json:"due"`
	Source        string      `json:"source"`
	SupersededDue Date        `json:"supersededDue,omitempty"`
	SupersededBy  string      `json:"supersededBy,omitempty"`
}

// PublishedSchedule is every dated deadline this tool shipped with. It covers
// one filing season only, on purpose: a date is carried here because an
// issuance set it, never because last year's date was rolled forward.
func PublishedSchedule() []PublishedDeadline {
	const (
		mc9     = "SEC Memorandum Circular No. 9, Series of 2026 (11 February 2026)"
		notice  = "SEC Notice dated 14 April 2026, extending the deadline for filing of 2025 Annual Financial Statements"
		dec2025 = "2025-12-31"
	)
	return []PublishedDeadline{
		{
			Requirement:   RequirementAFS,
			FiscalYearEnd: MustParseDate(dec2025),
			Due:           MustParseDate(AFSDueFY2025Dec31),
			Source:        notice,
			SupersededDue: MustParseDate(AFSDueFY2025Dec31Original),
			SupersededBy:  mc9 + ", Section 1",
		},
		{
			Requirement:   RequirementForm52AR,
			FiscalYearEnd: MustParseDate(dec2025),
			Due:           MustParseDate(Form52ARDueFY2025Dec31),
			Source:        notice,
			SupersededDue: MustParseDate(Form52ARDueFY2025Dec31Original),
			SupersededBy:  mc9 + ", Section 2(a)",
		},
		{
			Requirement:   RequirementForm17A,
			FiscalYearEnd: MustParseDate(dec2025),
			Due:           MustParseDate(Form17ADueFY2025Dec31),
			Source:        notice,
			SupersededDue: MustParseDate(Form17ADueFY2025Dec31Original),
			SupersededBy:  mc9 + ", Section 2(b)",
		},
	}
}

// lookupPublished finds a shipped deadline for one requirement and fiscal year
// end.
func lookupPublished(req Requirement, fiscalYearEnd Date) (PublishedDeadline, bool) {
	for _, p := range PublishedSchedule() {
		if p.Requirement == req && p.FiscalYearEnd == fiscalYearEnd {
			return p, true
		}
	}
	return PublishedDeadline{}, false
}

// Deadline is the answer to "when is this due", including the cases where the
// honest answer is that it cannot be worked out.
type Deadline struct {
	Requirement Requirement `json:"requirement"`
	Period      string      `json:"period"`
	Known       bool        `json:"known"`
	Due         Date        `json:"due,omitempty"`
	LastSafeDay Date        `json:"lastSafeDay,omitempty"`
	SafeDayNote string      `json:"safeDayNote,omitempty"`
	Basis       string      `json:"basis"`
	Source      string      `json:"source,omitempty"`
	Indicative  bool        `json:"indicative"`
	Reason      string      `json:"reason,omitempty"`
	Overridden  bool        `json:"overridden,omitempty"`
	Provisional bool        `json:"provisional,omitempty"`
}

// withSafeDay fills in the last day on which filing still counts as made
// before the deadline.
//
// The Commission's electronic filing tool is open around the clock, but a
// submission made on a Saturday, Sunday, holiday or during a work suspension
// is treated as filed on the next working day (SEC Memorandum Circular No. 9,
// Series of 2026, Section 9). A deadline that lands on a non-working day is
// therefore not a grace period: filing that day lands after it. The last safe
// day is the working day on or before the deadline. This is the tool's reading
// of that section and is shown as guidance, not as an extension granted by the
// Commission.
func (d Deadline) withSafeDay(holidays Holidays) Deadline {
	if !d.Known || d.Due.IsZero() {
		return d
	}
	safe := holidays.PrecedingWorkingDay(d.Due)
	d.LastSafeDay = safe
	if safe != d.Due {
		why := "a non-working day you recorded"
		if name, ok := holidays.Name(d.Due); ok {
			why = name
		} else if d.Due.IsWeekend() {
			why = "a " + d.Due.Weekday().String()
		}
		d.SafeDayNote = fmt.Sprintf(
			"%s falls on %s. A submission made on a Saturday, Sunday or holiday counts as filed on the next working day, which would be after the deadline, so file by %s.",
			d.Due, why, safe)
	}
	return d
}

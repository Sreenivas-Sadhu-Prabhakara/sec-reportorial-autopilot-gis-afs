package secfiling

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
)

// EntityType is the kind of registrant, which decides how the GIS clock is
// read and how the fines circular reckons a delay.
type EntityType string

const (
	EntityStock       EntityType = "stock"       // domestic stock corporation
	EntityNonStock    EntityType = "nonstock"    // domestic non-stock corporation
	EntityOPC         EntityType = "opc"         // One Person Corporation
	EntityBranch      EntityType = "branch"      // branch office of a foreign corporation
	EntityRepOffice   EntityType = "repoffice"   // representative office
	EntityRegionalHQ  EntityType = "rhq"         // regional headquarters
	EntityRegionalOHQ EntityType = "rohq"        // regional operating headquarters
	EntityPartnership EntityType = "partnership" // registered partnership
)

// Label names the entity type in the interface.
func (e EntityType) Label() string {
	switch e {
	case EntityStock:
		return "Domestic stock corporation"
	case EntityNonStock:
		return "Domestic non-stock corporation"
	case EntityOPC:
		return "One Person Corporation"
	case EntityBranch:
		return "Branch office of a foreign corporation"
	case EntityRepOffice:
		return "Representative office of a foreign corporation"
	case EntityRegionalHQ:
		return "Regional headquarters"
	case EntityRegionalOHQ:
		return "Regional operating headquarters"
	case EntityPartnership:
		return "Registered partnership"
	}
	return string(e)
}

// IsForeign reports the registrants the fines circular treats as foreign
// corporations, which have their own reckoning of when a filing becomes late.
func (e EntityType) IsForeign() bool {
	switch e {
	case EntityBranch, EntityRepOffice, EntityRegionalHQ, EntityRegionalOHQ:
		return true
	}
	return false
}

// Valid reports whether the entity type is one the tool knows.
func (e EntityType) Valid() bool { return e.Label() != string(e) }

// AllEntityTypes lists the entity types in interface order.
func AllEntityTypes() []EntityType {
	return []EntityType{EntityStock, EntityNonStock, EntityOPC, EntityPartnership,
		EntityBranch, EntityRepOffice, EntityRegionalHQ, EntityRegionalOHQ}
}

// FilerClass decides which financial-statement filing the corporation owes and
// on what schedule.
type FilerClass string

const (
	// FilerOrdinary files the AFS on the ordinary schedule.
	FilerOrdinary FilerClass = "ordinary"
	// FilerListedOrPublic is an issuer listed on the Philippine Stock
	// Exchange, an issuer registered but not listed, a public company or
	// another entity covered by Section 17.2 of the Securities Regulation
	// Code. It attaches the AFS to the Annual Report on SEC Form 17-A.
	FilerListedOrPublic FilerClass = "listed"
	// FilerBrokerDealer is a broker or dealer filing SEC Form 52-AR with the
	// AFS attached.
	FilerBrokerDealer FilerClass = "broker"
	// FilerCOAAudited has its financial statements audited by the Commission
	// on Audit and files when that audit concludes, with the affidavit and
	// COA letter the circular requires.
	FilerCOAAudited FilerClass = "coa"
)

// Label names the filer class in the interface.
func (f FilerClass) Label() string {
	switch f {
	case FilerOrdinary:
		return "Ordinary filer"
	case FilerListedOrPublic:
		return "Listed issuer or public company (SRC Sec. 17.2)"
	case FilerBrokerDealer:
		return "Broker or dealer"
	case FilerCOAAudited:
		return "Audited by the Commission on Audit"
	}
	return string(f)
}

// Valid reports whether the filer class is one the tool knows.
func (f FilerClass) Valid() bool { return f.Label() != string(f) }

// AllFilerClasses lists the filer classes in interface order.
func AllFilerClasses() []FilerClass {
	return []FilerClass{FilerOrdinary, FilerListedOrPublic, FilerBrokerDealer, FilerCOAAudited}
}

// Requirement is the financial-statement filing this class owes.
func (f FilerClass) Requirement() Requirement {
	switch f {
	case FilerListedOrPublic:
		return RequirementForm17A
	case FilerBrokerDealer:
		return RequirementForm52AR
	}
	return RequirementAFS
}

// GISBasis is the date the thirty-day GIS clock is counted from.
type GISBasis string

const (
	// GISFromMeeting counts from the date the annual stockholders' or
	// members' meeting was actually held.
	// Source: SEC Memorandum Circular No. 9, Series of 2026, Section 5(a) and 5(b).
	GISFromMeeting GISBasis = "meeting"
	// GISFromLicenceAnniversary counts from the anniversary of the issuance of
	// the SEC licence, which is how a foreign corporation's GIS is reckoned.
	// Source: SEC Memorandum Circular No. 9, Series of 2026, Section 5(c).
	GISFromLicenceAnniversary GISBasis = "licence"
	// GISFromIncorporationAnniversary counts from the anniversary of the
	// issuance of the certificate of incorporation. Section 5 of the circular
	// above names stock corporations, non-stock corporations and foreign
	// corporations; it does not state a separate reckoning date for a One
	// Person Corporation. This option exists so a corporation that files on
	// that basis can record it, and the tool marks the result as resting on
	// the user's own reading rather than on Section 5.
	GISFromIncorporationAnniversary GISBasis = "incorporation"
)

// Label names the basis in the interface.
func (g GISBasis) Label() string {
	switch g {
	case GISFromMeeting:
		return "Date of the actual annual meeting"
	case GISFromLicenceAnniversary:
		return "Anniversary of the SEC licence"
	case GISFromIncorporationAnniversary:
		return "Anniversary of incorporation"
	}
	return string(g)
}

// Valid reports whether the basis is one the tool knows.
func (g GISBasis) Valid() bool { return g.Label() != string(g) }

// AllGISBases lists the bases in interface order.
func AllGISBases() []GISBasis {
	return []GISBasis{GISFromMeeting, GISFromLicenceAnniversary, GISFromIncorporationAnniversary}
}

// DefaultGISBasis is the basis the interface offers for a new company of this
// type.
func DefaultGISBasis(e EntityType) GISBasis {
	if e.IsForeign() {
		return GISFromLicenceAnniversary
	}
	if e == EntityOPC {
		return GISFromIncorporationAnniversary
	}
	return GISFromMeeting
}

// Meeting is one annual stockholders' or members' meeting, either held or
// still only planned. A planned meeting gives a provisional GIS date; the
// deadline is not fixed until the meeting is actually held.
type Meeting struct {
	Year int  `json:"year"`
	Date Date `json:"date"`
	Held bool `json:"held"`
}

// Company is one registrant and everything about it that moves a deadline.
type Company struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	RegistrationNumber string     `json:"registrationNumber"`
	EntityType         EntityType `json:"entityType"`
	FilerClass         FilerClass `json:"filerClass"`
	FiscalYearEndMonth int        `json:"fiscalYearEndMonth"`
	FiscalYearEndDay   int        `json:"fiscalYearEndDay"`
	GISBasis           GISBasis   `json:"gisBasis"`
	ReckoningDate      Date       `json:"reckoningDate"`
	Meetings           []Meeting  `json:"meetings"`
	Notes              string     `json:"notes"`
}

// Filing is a record that something was filed, kept for the proof reference it
// carries. The tool files nothing itself: this is the user writing down what
// they filed and the reference the Commission's electronic filing tool gave
// back.
type Filing struct {
	ID          string      `json:"id"`
	CompanyID   string      `json:"companyId"`
	Requirement Requirement `json:"requirement"`
	FilingYear  int         `json:"filingYear"`
	FiledOn     Date        `json:"filedOn"`
	Reference   string      `json:"reference"`
	Channel     string      `json:"channel"`
	FiledBy     string      `json:"filedBy"`
	Note        string      `json:"note"`
}

// PenaltyFigures holds peso amounts the user has read off the current fines
// circular themselves. The tool ships with none. See penalty.go for why.
type PenaltyFigures struct {
	Source         string `json:"source"`
	EnteredOn      Date   `json:"enteredOn"`
	BasicLate      int64  `json:"basicLate"`
	BasicNonFiling int64  `json:"basicNonFiling"`
	MonthlyRate    int64  `json:"monthlyRate"`
}

// Supplied reports whether the user has given the tool enough to compute an
// exposure in pesos.
func (p PenaltyFigures) Supplied() bool {
	return strings.TrimSpace(p.Source) != "" && (p.BasicLate > 0 || p.BasicNonFiling > 0)
}

// ReminderLadder is how far ahead each escalating reminder starts, in calendar
// days before the deadline. The defaults are the tool's own, not a rule: a
// filing season takes a quarter to prepare, so the first nudge is a quarter
// out.
type ReminderLadder struct {
	PlanDays    int `json:"planDays"`
	PrepareDays int `json:"prepareDays"`
	SoonDays    int `json:"soonDays"`
	UrgentDays  int `json:"urgentDays"`
	FinalDays   int `json:"finalDays"`
}

// The default reminder ladder, in calendar days before the deadline.
const (
	DefaultPlanDays    = 90
	DefaultPrepareDays = 60
	DefaultSoonDays    = 30
	DefaultUrgentDays  = 14
	DefaultFinalDays   = 3
)

// DefaultLadder is the ladder a new data file starts with.
func DefaultLadder() ReminderLadder {
	return ReminderLadder{
		PlanDays:    DefaultPlanDays,
		PrepareDays: DefaultPrepareDays,
		SoonDays:    DefaultSoonDays,
		UrgentDays:  DefaultUrgentDays,
		FinalDays:   DefaultFinalDays,
	}
}

// Validate rejects a ladder whose steps do not descend, because a reminder
// that fires after a later one is not a ladder.
func (l ReminderLadder) Validate() error {
	steps := []struct {
		name string
		days int
	}{
		{"plan", l.PlanDays}, {"prepare", l.PrepareDays}, {"due soon", l.SoonDays},
		{"urgent", l.UrgentDays}, {"final call", l.FinalDays},
	}
	for i, s := range steps {
		if s.days < 1 {
			return fmt.Errorf("the %s reminder must be at least 1 day before the deadline", s.name)
		}
		if i > 0 && s.days >= steps[i-1].days {
			return fmt.Errorf("the %s reminder (%d days) must be closer to the deadline than the %s reminder (%d days)",
				s.name, s.days, steps[i-1].name, steps[i-1].days)
		}
	}
	return nil
}

// DeadlineOverride replaces a deadline the tool shipped with, or supplies one
// it does not have. This is how a filing schedule issued after this build gets
// into the tool without a new binary.
type DeadlineOverride struct {
	Requirement Requirement `json:"requirement"`
	FilingYear  int         `json:"filingYear"`
	CompanyID   string      `json:"companyId"`
	Due         Date        `json:"due"`
	Source      string      `json:"source"`
	Note        string      `json:"note"`
}

// Settings is everything the user can change without a new build.
type Settings struct {
	Holidays  Holidays           `json:"holidays"`
	Ladder    ReminderLadder     `json:"reminderLadder"`
	Overrides []DeadlineOverride `json:"deadlineOverrides"`
	Penalties PenaltyFigures     `json:"penaltyFigures"`
}

// Book is everything the tool keeps: the companies, the filings recorded
// against them, and the settings.
type Book struct {
	Companies []Company `json:"companies"`
	Filings   []Filing  `json:"filings"`
	Settings  Settings  `json:"settings"`
}

// New starts an empty book with the default settings.
func New() *Book {
	return &Book{
		Companies: []Company{},
		Filings:   []Filing{},
		Settings: Settings{
			Holidays:  Holidays{},
			Ladder:    DefaultLadder(),
			Overrides: []DeadlineOverride{},
		},
	}
}

// Normalise fills in anything a hand-edited or older data file is missing, so
// the rest of the package can assume the book is well formed.
func (b *Book) Normalise() {
	if b.Companies == nil {
		b.Companies = []Company{}
	}
	if b.Filings == nil {
		b.Filings = []Filing{}
	}
	if b.Settings.Holidays == nil {
		b.Settings.Holidays = Holidays{}
	}
	if b.Settings.Overrides == nil {
		b.Settings.Overrides = []DeadlineOverride{}
	}
	if b.Settings.Ladder.Validate() != nil {
		b.Settings.Ladder = DefaultLadder()
	}
	for i := range b.Companies {
		c := &b.Companies[i]
		if c.FiscalYearEndMonth == 0 || c.FiscalYearEndDay == 0 {
			c.FiscalYearEndMonth, c.FiscalYearEndDay = 12, 31
		}
		if !c.EntityType.Valid() {
			c.EntityType = EntityStock
		}
		if !c.FilerClass.Valid() {
			c.FilerClass = FilerOrdinary
		}
		if !c.GISBasis.Valid() {
			c.GISBasis = DefaultGISBasis(c.EntityType)
		}
		if c.Meetings == nil {
			c.Meetings = []Meeting{}
		}
		sort.Slice(c.Meetings, func(a, z int) bool { return c.Meetings[a].Year < c.Meetings[z].Year })
	}
	sort.Slice(b.Companies, func(a, z int) bool {
		return strings.ToLower(b.Companies[a].Name) < strings.ToLower(b.Companies[z].Name)
	})
}

// Company finds a company by id.
func (b *Book) Company(id string) (*Company, bool) {
	for i := range b.Companies {
		if b.Companies[i].ID == id {
			return &b.Companies[i], true
		}
	}
	return nil, false
}

// PutCompany adds or replaces a company after validating it.
func (b *Book) PutCompany(c Company) error {
	clean, err := c.cleaned()
	if err != nil {
		return err
	}
	for i := range b.Companies {
		if b.Companies[i].ID == clean.ID {
			clean.Meetings = b.Companies[i].Meetings
			b.Companies[i] = clean
			b.Normalise()
			return nil
		}
	}
	b.Companies = append(b.Companies, clean)
	b.Normalise()
	return nil
}

// DeleteCompany removes a company and every filing recorded against it.
func (b *Book) DeleteCompany(id string) error {
	kept := b.Companies[:0]
	found := false
	for _, c := range b.Companies {
		if c.ID == id {
			found = true
			continue
		}
		kept = append(kept, c)
	}
	if !found {
		return fmt.Errorf("there is no company with that reference")
	}
	b.Companies = kept
	keptFilings := b.Filings[:0]
	for _, f := range b.Filings {
		if f.CompanyID != id {
			keptFilings = append(keptFilings, f)
		}
	}
	b.Filings = keptFilings
	b.Normalise()
	return nil
}

// PutMeeting records an annual meeting, replacing whatever was recorded for
// that year.
func (b *Book) PutMeeting(companyID string, m Meeting) error {
	c, ok := b.Company(companyID)
	if !ok {
		return fmt.Errorf("there is no company with that reference")
	}
	if !m.Date.Valid() {
		return fmt.Errorf("the meeting date is not a real date")
	}
	if m.Year == 0 {
		m.Year = m.Date.Year
	}
	if m.Year != m.Date.Year {
		return fmt.Errorf("a meeting dated %s belongs to %d, not %d", m.Date, m.Date.Year, m.Year)
	}
	for i := range c.Meetings {
		if c.Meetings[i].Year == m.Year {
			c.Meetings[i] = m
			return nil
		}
	}
	c.Meetings = append(c.Meetings, m)
	sort.Slice(c.Meetings, func(a, z int) bool { return c.Meetings[a].Year < c.Meetings[z].Year })
	return nil
}

// DeleteMeeting removes the meeting recorded for one year.
func (b *Book) DeleteMeeting(companyID string, year int) error {
	c, ok := b.Company(companyID)
	if !ok {
		return fmt.Errorf("there is no company with that reference")
	}
	kept := c.Meetings[:0]
	found := false
	for _, m := range c.Meetings {
		if m.Year == year {
			found = true
			continue
		}
		kept = append(kept, m)
	}
	if !found {
		return fmt.Errorf("no meeting is recorded for %d", year)
	}
	c.Meetings = kept
	return nil
}

// Meeting finds the meeting recorded for one year.
func (c *Company) Meeting(year int) (Meeting, bool) {
	for _, m := range c.Meetings {
		if m.Year == year {
			return m, true
		}
	}
	return Meeting{}, false
}

// FiscalYearEndIn is the company's fiscal year end falling in one year, with
// the same month-end clamping as everything else: a 29 February year end lands
// on 28 February in an ordinary year.
func (c *Company) FiscalYearEndIn(year int) Date {
	month := time.Month(c.FiscalYearEndMonth)
	day := c.FiscalYearEndDay
	if last := DaysInMonth(year, month); day > last {
		day = last
	}
	return Date{Year: year, Month: month, Day: day}
}

// EndsInDecember reports a 31 December fiscal year end, which is the case the
// dated schedule covers.
func (c *Company) EndsInDecember() bool {
	return c.FiscalYearEndMonth == 12 && c.FiscalYearEndDay == 31
}

// LastDigit is the last digit of the SEC registration number, or -1 if the
// number holds no digit. It is shown in the interface next to a plain
// statement that it does not set the AFS window for this filing season.
func (c *Company) LastDigit() int {
	for i := len(c.RegistrationNumber) - 1; i >= 0; i-- {
		r := rune(c.RegistrationNumber[i])
		if unicode.IsDigit(r) {
			return int(r - '0')
		}
	}
	return -1
}

func (c Company) cleaned() (Company, error) {
	c.ID = strings.TrimSpace(c.ID)
	c.Name = strings.TrimSpace(c.Name)
	c.RegistrationNumber = strings.TrimSpace(c.RegistrationNumber)
	c.Notes = strings.TrimSpace(c.Notes)
	if c.ID == "" {
		return c, fmt.Errorf("the company needs a reference of its own")
	}
	if c.Name == "" {
		return c, fmt.Errorf("the company needs a name")
	}
	if !c.EntityType.Valid() {
		return c, fmt.Errorf("%q is not a kind of registrant this tool knows", c.EntityType)
	}
	if !c.FilerClass.Valid() {
		return c, fmt.Errorf("%q is not a filer class this tool knows", c.FilerClass)
	}
	if !c.GISBasis.Valid() {
		return c, fmt.Errorf("%q is not a way of reckoning the GIS this tool knows", c.GISBasis)
	}
	if c.FiscalYearEndMonth == 0 && c.FiscalYearEndDay == 0 {
		c.FiscalYearEndMonth, c.FiscalYearEndDay = 12, 31
	}
	if c.FiscalYearEndMonth < 1 || c.FiscalYearEndMonth > 12 {
		return c, fmt.Errorf("the fiscal year end month must be between 1 and 12")
	}
	maxDay := DaysInMonth(2024, time.Month(c.FiscalYearEndMonth)) // 2024 is a leap year, so 29 February is allowed
	if c.FiscalYearEndDay < 1 || c.FiscalYearEndDay > maxDay {
		return c, fmt.Errorf("%s has %d days, so a fiscal year cannot end on day %d",
			time.Month(c.FiscalYearEndMonth), maxDay, c.FiscalYearEndDay)
	}
	if c.GISBasis != GISFromMeeting {
		if !c.ReckoningDate.Valid() {
			return c, fmt.Errorf("counting the GIS from the %s needs that date recorded",
				strings.ToLower(c.GISBasis.Label()))
		}
	} else if !c.ReckoningDate.IsZero() && !c.ReckoningDate.Valid() {
		return c, fmt.Errorf("the date recorded for this company is not a real date")
	}
	if c.Meetings == nil {
		c.Meetings = []Meeting{}
	}
	return c, nil
}

// PutFiling records a filing, replacing any earlier record of the same
// requirement in the same filing year for the same company.
func (b *Book) PutFiling(f Filing) error {
	f.ID = strings.TrimSpace(f.ID)
	f.CompanyID = strings.TrimSpace(f.CompanyID)
	f.Reference = strings.TrimSpace(f.Reference)
	f.Channel = strings.TrimSpace(f.Channel)
	f.FiledBy = strings.TrimSpace(f.FiledBy)
	f.Note = strings.TrimSpace(f.Note)
	if f.ID == "" {
		return fmt.Errorf("the filing record needs a reference of its own")
	}
	if _, ok := b.Company(f.CompanyID); !ok {
		return fmt.Errorf("there is no company with that reference")
	}
	if !f.Requirement.Valid() {
		return fmt.Errorf("%q is not a filing this tool tracks", f.Requirement)
	}
	if !f.FiledOn.Valid() {
		return fmt.Errorf("the date filed is not a real date")
	}
	if f.FilingYear == 0 {
		f.FilingYear = f.FiledOn.Year
	}
	if f.Reference == "" {
		return fmt.Errorf("record the reference the filing came back with, so there is proof it was filed")
	}
	if f.Channel == "" {
		f.Channel = "eFAST"
	}
	for i := range b.Filings {
		if b.Filings[i].ID == f.ID {
			b.Filings[i] = f
			return nil
		}
		if b.Filings[i].CompanyID == f.CompanyID &&
			b.Filings[i].Requirement == f.Requirement &&
			b.Filings[i].FilingYear == f.FilingYear {
			f.ID = b.Filings[i].ID
			b.Filings[i] = f
			return nil
		}
	}
	b.Filings = append(b.Filings, f)
	return nil
}

// DeleteFiling removes a filing record.
func (b *Book) DeleteFiling(id string) error {
	kept := b.Filings[:0]
	found := false
	for _, f := range b.Filings {
		if f.ID == id {
			found = true
			continue
		}
		kept = append(kept, f)
	}
	if !found {
		return fmt.Errorf("there is no filing record with that reference")
	}
	b.Filings = kept
	return nil
}

// FilingFor finds the filing recorded for one company, requirement and filing
// year.
func (b *Book) FilingFor(companyID string, req Requirement, filingYear int) (Filing, bool) {
	for _, f := range b.Filings {
		if f.CompanyID == companyID && f.Requirement == req && f.FilingYear == filingYear {
			return f, true
		}
	}
	return Filing{}, false
}

// PutSettings replaces the settings after validating them.
func (b *Book) PutSettings(s Settings) error {
	if err := s.Ladder.Validate(); err != nil {
		return err
	}
	clean := Holidays{}
	for day, name := range s.Holidays {
		d, err := ParseDate(day)
		if err != nil {
			return fmt.Errorf("holiday list: %w", err)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = "non-working day"
		}
		clean[d.String()] = name
	}
	overrides := []DeadlineOverride{}
	for _, o := range s.Overrides {
		if !o.Requirement.Valid() {
			return fmt.Errorf("%q is not a filing this tool tracks", o.Requirement)
		}
		if o.FilingYear < 1900 || o.FilingYear > 2200 {
			return fmt.Errorf("an override needs the filing year it applies to")
		}
		if !o.Due.Valid() {
			return fmt.Errorf("an override needs the deadline the Commission set")
		}
		o.Source = strings.TrimSpace(o.Source)
		if o.Source == "" {
			return fmt.Errorf("record which issuance set %s, so the date can be checked later", o.Due)
		}
		o.Note = strings.TrimSpace(o.Note)
		o.CompanyID = strings.TrimSpace(o.CompanyID)
		overrides = append(overrides, o)
	}
	s.Penalties.Source = strings.TrimSpace(s.Penalties.Source)
	if s.Penalties.BasicLate < 0 || s.Penalties.BasicNonFiling < 0 || s.Penalties.MonthlyRate < 0 {
		return fmt.Errorf("a penalty figure cannot be negative")
	}
	if (s.Penalties.BasicLate > 0 || s.Penalties.BasicNonFiling > 0 || s.Penalties.MonthlyRate > 0) &&
		s.Penalties.Source == "" {
		return fmt.Errorf("record which issuance these amounts came from before the tool uses them")
	}
	s.Holidays = clean
	s.Overrides = overrides
	b.Settings = s
	return nil
}

// override finds a deadline the user has supplied for one company,
// requirement and filing year. A company-specific override wins over a general
// one.
func (s Settings) override(companyID string, req Requirement, filingYear int) (DeadlineOverride, bool) {
	var general DeadlineOverride
	haveGeneral := false
	for _, o := range s.Overrides {
		if o.Requirement != req || o.FilingYear != filingYear {
			continue
		}
		if o.CompanyID == companyID {
			return o, true
		}
		if o.CompanyID == "" {
			general, haveGeneral = o, true
		}
	}
	return general, haveGeneral
}

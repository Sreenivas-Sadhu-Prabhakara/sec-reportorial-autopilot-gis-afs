package secfiling

import "fmt"

// GISDeadline works out when the General Information Sheet falls due for one
// company in one year.
//
// The rule is that the GIS is filed within thirty calendar days from the date
// of the actual annual stockholders' or members' meeting, and for a foreign
// corporation from the anniversary date of the issuance of its SEC licence
// (SEC Memorandum Circular No. 9, Series of 2026, Section 5). The deadline is
// therefore derived from the company's own meeting date: two corporations that
// met a week apart owe their GIS a week apart, and no calendar date applies to
// everyone.
//
// Where the meeting has not been held yet, there is no deadline to state. The
// tool says so rather than inventing one, and will show a provisional date if
// a meeting has been scheduled.
func GISDeadline(c *Company, settings Settings, year int) Deadline {
	d := Deadline{Requirement: RequirementGIS, Period: fmt.Sprintf("%d", year)}

	switch c.GISBasis {
	case GISFromLicenceAnniversary:
		anniversary := c.ReckoningDate.AnniversaryIn(year)
		d.Known = true
		d.Due = anniversary.AddDays(GISDaysAfterReckoning)
		d.Period = fmt.Sprintf("licence anniversary %s", anniversary)
		d.Basis = fmt.Sprintf("%d calendar days from %s, the anniversary in %d of the SEC licence issued on %s",
			GISDaysAfterReckoning, anniversary, year, c.ReckoningDate)
		d.Source = "SEC Memorandum Circular No. 9, Series of 2026, Section 5(c)"

	case GISFromIncorporationAnniversary:
		anniversary := c.ReckoningDate.AnniversaryIn(year)
		d.Known = true
		d.Indicative = true
		d.Due = anniversary.AddDays(GISDaysAfterReckoning)
		d.Period = fmt.Sprintf("incorporation anniversary %s", anniversary)
		d.Basis = fmt.Sprintf("%d calendar days from %s, the anniversary in %d of incorporation on %s",
			GISDaysAfterReckoning, anniversary, year, c.ReckoningDate)
		d.Source = "SEC Memorandum Circular No. 9, Series of 2026, Section 5 (thirty-day window)"
		d.Reason = "Section 5 names stock corporations, non-stock corporations and foreign corporations. It does not state a separate reckoning date for a One Person Corporation, so the anniversary used here is the one you recorded, not one this tool read out of the circular. Confirm the reckoning date with the Commission."

	default: // GISFromMeeting
		meeting, ok := c.Meeting(year)
		section := "Section 5(a)"
		what := "annual stockholders' meeting"
		if c.EntityType == EntityNonStock {
			section, what = "Section 5(b)", "annual members' meeting"
		}
		if !ok {
			d.Known = false
			d.Basis = fmt.Sprintf("%d calendar days from the date of the actual %s", GISDaysAfterReckoning, what)
			d.Source = "SEC Memorandum Circular No. 9, Series of 2026, " + section
			d.Reason = fmt.Sprintf("The thirty days run from the date the %s is actually held, so there is no %d deadline until that date exists. Record the meeting date, or the date it is set for, and the deadline follows from it.", what, year)
			return d.withSafeDay(settings.Holidays)
		}
		d.Known = true
		d.Due = meeting.Date.AddDays(GISDaysAfterReckoning)
		d.Period = fmt.Sprintf("meeting of %s", meeting.Date)
		d.Source = "SEC Memorandum Circular No. 9, Series of 2026, " + section
		if meeting.Held {
			d.Basis = fmt.Sprintf("%d calendar days from %s, the date of the actual %s",
				GISDaysAfterReckoning, meeting.Date, what)
		} else {
			d.Provisional = true
			d.Basis = fmt.Sprintf("%d calendar days from %s, the date the %s is set for",
				GISDaysAfterReckoning, meeting.Date, what)
			d.Reason = "The meeting is recorded as planned, not held. The deadline moves with the meeting: mark it held once it has taken place."
		}
	}

	if o, ok := settings.override(c.ID, RequirementGIS, year); ok {
		d.Known = true
		d.Due = o.Due
		d.Overridden = true
		d.Indicative = false
		d.Provisional = false
		d.Basis = "set by you, replacing the date this tool worked out"
		d.Source = o.Source
		d.Reason = o.Note
	}

	return d.withSafeDay(settings.Holidays)
}

// FiscalYearEndForFilingYear is the fiscal year end whose financial statements
// fall due in a given filing year. For the ordinary case — a 31 December year
// end — that is 31 December of the year before.
func FiscalYearEndForFilingYear(c *Company, filingYear int) Date {
	if c.EndsInDecember() {
		return c.FiscalYearEndIn(filingYear - 1)
	}
	days := afsDaysFor(c.FilerClass)
	earlier := c.FiscalYearEndIn(filingYear - 1)
	if earlier.AddDays(days).Year == filingYear {
		return earlier
	}
	later := c.FiscalYearEndIn(filingYear)
	if later.AddDays(days).Year == filingYear {
		return later
	}
	return earlier
}

// afsDaysFor is the calendar-day window for a fiscal year end other than
// 31 December.
func afsDaysFor(class FilerClass) int {
	switch class {
	case FilerListedOrPublic:
		return AFSDaysListedOrPublicFYE
	case FilerBrokerDealer:
		return AFSDaysBrokerDealerNonDecemberFYE
	}
	return AFSDaysOrdinaryNonDecemberFYE
}

// AFSDeadline works out when the audited financial statements fall due for one
// company in one filing year.
//
// This is the part of the Philippine schedule that has moved. Earlier seasons
// staggered the AFS across weeks by the last digit of the SEC registration
// number. The circular for the season this tool was built against sets one
// date for every corporation with a 31 December fiscal year end, and a notice
// issued two months later moved that date for all of them together. The tool
// therefore carries dated deadlines taken from named issuances rather than a
// formula, and where it holds no issuance for a season it says so instead of
// rolling last season's date forward. A wrong AFS window is a penalty.
func AFSDeadline(c *Company, settings Settings, filingYear int) Deadline {
	req := c.FilerClass.Requirement()
	fye := FiscalYearEndForFilingYear(c, filingYear)
	d := Deadline{
		Requirement: req,
		Period:      fmt.Sprintf("fiscal year ended %s", fye),
	}

	if o, ok := settings.override(c.ID, req, filingYear); ok {
		d.Known = true
		d.Due = o.Due
		d.Overridden = true
		d.Basis = "set by you, from the issuance named alongside it"
		d.Source = o.Source
		d.Reason = o.Note
		return d.withSafeDay(settings.Holidays)
	}

	if c.FilerClass == FilerCOAAudited {
		d.Known = false
		d.Basis = "filed once the Commission on Audit has concluded its audit, with the affidavit of the President and Treasurer and the COA letter attached"
		d.Source = "SEC Memorandum Circular No. 9, Series of 2026, Section 2(c)"
		d.Reason = "A corporation whose statements are audited by the Commission on Audit is outside the dated schedule, so no fixed deadline is shown. Record the filing when it is made."
		return d.withSafeDay(settings.Holidays)
	}

	if c.EndsInDecember() {
		if p, ok := lookupPublished(req, fye); ok {
			d.Known = true
			d.Due = p.Due
			d.Basis = fmt.Sprintf("the date the Commission published for a %s fiscal year end", fye)
			d.Source = p.Source
			d.Indicative = true
			if !p.SupersededDue.IsZero() {
				d.Reason = fmt.Sprintf("First set at %s by %s, then moved to %s.", p.SupersededDue, p.SupersededBy, p.Due)
			}
			return d.withSafeDay(settings.Holidays)
		}
		d.Known = false
		d.Basis = "a date the Commission publishes for the season, in its filing-schedule memorandum circular"
		d.Reason = fmt.Sprintf(
			"This build holds no published deadline for a %s fiscal year end. The Commission issues a filing schedule each year, usually in the first quarter, and has revised it mid-season by notice; the schedule here was checked on %s. Last season's date is not carried forward, because carrying a date forward is how a wrong AFS window happens: within one season the deadline for a 31 December 2025 year end moved from %s to %s. Put the published date into Settings, with the issuance it came from, and it will be used here.",
			fye, ScheduleAsOf, AFSDueFY2025Dec31Original, AFSDueFY2025Dec31)
		return d.withSafeDay(settings.Holidays)
	}

	days := afsDaysFor(c.FilerClass)
	section := "Section 2(a)"
	switch c.FilerClass {
	case FilerListedOrPublic:
		section = "Section 2(b)"
	case FilerBrokerDealer:
		section = "Section 2(a), citing Rule 52.1.5.2 of the SRC-IRR"
	}
	d.Known = true
	d.Indicative = true
	d.Due = fye.AddDays(days)
	d.Basis = fmt.Sprintf("%d calendar days from %s, the end of the fiscal year", days, fye)
	d.Source = "SEC Memorandum Circular No. 9, Series of 2026, " + section
	d.Reason = fmt.Sprintf("The %d-day rule is restated in each season's filing-schedule memorandum. Confirm it is unchanged for the %d season before you rely on this date.", days, filingYear)
	return d.withSafeDay(settings.Holidays)
}

// RegistrationNumberNote is what the interface says next to a registration
// number, so the question "does my last digit set my AFS date" is answered
// every time it is asked.
func RegistrationNumberNote(c *Company) string {
	digit := c.LastDigit()
	if digit < 0 {
		return TheRegistrationNumberDoesNotSetTheAFSWindow
	}
	return fmt.Sprintf("Last digit %d. %s", digit, TheRegistrationNumberDoesNotSetTheAFSWindow)
}

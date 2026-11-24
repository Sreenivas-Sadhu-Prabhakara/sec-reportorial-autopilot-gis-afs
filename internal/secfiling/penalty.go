package secfiling

import "fmt"

// Stage is how far a filing has gone past its deadline, in the terms the fines
// circular uses.
type Stage string

const (
	// StageNotDue is a filing whose deadline has not arrived.
	StageNotDue Stage = "notdue"
	// StageNoDeadline is a filing whose deadline cannot be worked out yet.
	StageNoDeadline Stage = "nodeadline"
	// StageOnTime is a filing recorded on or before its deadline.
	StageOnTime Stage = "ontime"
	// StageWithinForeignWindow is a foreign corporation's filing made after
	// the deadline but within the thirty calendar days the fines circular
	// allows before a late fine attaches.
	StageWithinForeignWindow Stage = "foreignwindow"
	// StageLate is a filing made, or still owing, after the deadline.
	StageLate Stage = "late"
	// StageTreatedAsNonFiling is a delay long enough that the Commission
	// assesses it as a non-filing rather than a late filing.
	StageTreatedAsNonFiling Stage = "nonfiling"
)

// Label names the stage in the interface.
func (s Stage) Label() string {
	switch s {
	case StageNotDue:
		return "Not yet due"
	case StageNoDeadline:
		return "No deadline yet"
	case StageOnTime:
		return "Filed on time"
	case StageWithinForeignWindow:
		return "Filed after the deadline, inside the 30-day window"
	case StageLate:
		return "Late"
	case StageTreatedAsNonFiling:
		return "Assessed as a non-filing"
	}
	return string(s)
}

// Amount is what a delay costs. It is almost always not computed, and says
// why.
type Amount struct {
	Computed         bool   `json:"computed"`
	Reason           string `json:"reason,omitempty"`
	Basic            int64  `json:"basic,omitempty"`
	MonthlyRate      int64  `json:"monthlyRate,omitempty"`
	Months           int    `json:"months,omitempty"`
	Total            int64  `json:"total,omitempty"`
	Source           string `json:"source,omitempty"`
	MonthlySuspended bool   `json:"monthlySuspended,omitempty"`
	SuspensionNote   string `json:"suspensionNote,omitempty"`
}

// Exposure is what being late leads to: the stage, the consequence in words,
// and an amount only where the user has supplied the figures to compute one.
type Exposure struct {
	Stage       Stage  `json:"stage"`
	Label       string `json:"label"`
	DaysLate    int    `json:"daysLate"`
	MonthsLate  int    `json:"monthsLate"`
	Consequence string `json:"consequence"`
	Amount      Amount `json:"amount"`
}

// WhyNoAmount is the reason the tool does not put a peso figure on a late
// filing by itself. It is shown wherever an exposure is shown.
const WhyNoAmount = "This tool does not compute a peso penalty. The scale in SEC Memorandum Circular No. 6, Series of 2024 is tiered by the kind of registrant, by retained earnings or fund balance and by how many offences have been settled before, and it has been amended since: SEC Memorandum Circular No. 10, Series of 2026 gives One Person Corporations their own scale, and SEC Memorandum Circular No. 16, Series of 2026 suspended the per-month component from 14 May 2026 to 31 December 2026. A figure out of a tool that has not read your tier and your offence count would be a guess, and the Commission's own assessment governs. Read your tier off the current circular, enter the figures in Settings, and the tool will apply them and label them as yours."

// MonthlyPenaltySuspended reports whether the per-month component of the fines
// is suspended on a given date.
// Source: SEC Memorandum Circular No. 16, Series of 2026, suspending the
// monthly rate from 14 May 2026 to 31 December 2026, with the basic fine
// unaffected and the monthly rate returning on 1 January 2027.
func MonthlyPenaltySuspended(on Date) bool {
	from := MustParseDate(MonthlyPenaltySuspendedFrom)
	until := MustParseDate(MonthlyPenaltySuspendedUntil)
	return !on.Before(from) && !on.After(until)
}

// ExposureFor works out where one filing stands against its deadline.
//
// due is the deadline; filedOn is the date it was actually filed, or the zero
// date if it is still owing; asOf is the date being asked about.
func ExposureFor(entity EntityType, due Date, filedOn Date, asOf Date, figures PenaltyFigures) Exposure {
	if due.IsZero() {
		return Exposure{
			Stage:       StageNoDeadline,
			Label:       StageNoDeadline.Label(),
			Consequence: "There is no deadline to measure against yet, so nothing is overdue.",
			Amount:      Amount{Reason: WhyNoAmount},
		}
	}

	reference := filedOn
	filed := !filedOn.IsZero()
	if !filed {
		reference = asOf
	}

	if !reference.After(due) {
		e := Exposure{Stage: StageNotDue, Label: StageNotDue.Label()}
		if filed {
			e.Stage, e.Label = StageOnTime, StageOnTime.Label()
			e.Consequence = "Filed on or before the deadline. Keep the reference: it is the proof of filing."
		} else {
			e.Consequence = fmt.Sprintf("Due on %s. Nothing is owed while it is filed by then.", due)
		}
		e.Amount = Amount{Reason: WhyNoAmount}
		return e
	}

	daysLate := due.DaysUntil(reference)
	e := Exposure{
		DaysLate:   daysLate,
		MonthsLate: due.MonthsStarted(reference),
	}

	foreign := entity.IsForeign()
	switch {
	case foreign && daysLate <= ForeignLateThresholdDays:
		e.Stage = StageWithinForeignWindow
		e.Consequence = fmt.Sprintf(
			"%d calendar days past the deadline. SEC Memorandum Circular No. 6, Series of 2024 reckons a foreign corporation's filing as late once it is more than %d calendar days after the deadline, and assesses it as a non-filing beyond %d. This is how the fine is reckoned, not permission to file late.",
			daysLate, ForeignLateThresholdDays, ForeignNonFilingThresholdDays)
	case foreign && daysLate > ForeignNonFilingThresholdDays:
		e.Stage = StageTreatedAsNonFiling
		e.Consequence = fmt.Sprintf(
			"%d calendar days past the deadline. Beyond %d calendar days a foreign corporation's filing is assessed as a non-filing under SEC Memorandum Circular No. 6, Series of 2024, which carries the higher basic fine and a per-month component capped at %d months.",
			daysLate, ForeignNonFilingThresholdDays, NonFilingMonthlyCapMonths)
	case !foreign && reference.After(due.AddYears(DomesticNonFilingThresholdYear)):
		e.Stage = StageTreatedAsNonFiling
		e.Consequence = fmt.Sprintf(
			"More than one year past the deadline (%d calendar days). A domestic corporation that files more than a year late is assessed as a non-filing under SEC Memorandum Circular No. 6, Series of 2024, which carries the higher basic fine and a per-month component capped at %d months.",
			daysLate, NonFilingMonthlyCapMonths)
	default:
		e.Stage = StageLate
		e.Consequence = fmt.Sprintf(
			"%d calendar days past the deadline. A late filing draws the basic fine for the registrant's tier under SEC Memorandum Circular No. 6, Series of 2024, and past one year it is assessed as a non-filing instead.",
			daysLate)
	}
	e.Label = e.Stage.Label()

	e.Consequence += fmt.Sprintf(
		" Beyond the money: failing to submit reportorial requirements %d times, consecutively or intermittently, within %d years is a ground for the Commission to place the corporation under delinquent status (Section 177, Revised Corporation Code, Republic Act No. 11232; guidelines in SEC Memorandum Circular No. 19, Series of 2023), and a %dth offence after a corporation has been notified of its delinquency is a ground for revoking its certificate of registration or licence to transact business.",
		DelinquencyMissCount, DelinquencyWindowYears, RevocationOffenceNumber)

	e.Amount = amountFor(e, asOf, figures)
	return e
}

// amountFor computes a peso exposure only from figures the user has entered
// themselves, and labels it as theirs.
func amountFor(e Exposure, asOf Date, figures PenaltyFigures) Amount {
	if !figures.Supplied() {
		return Amount{Reason: WhyNoAmount}
	}

	a := Amount{
		Computed:    true,
		Source:      figures.Source,
		MonthlyRate: figures.MonthlyRate,
		Months:      e.MonthsLate,
	}
	switch e.Stage {
	case StageTreatedAsNonFiling:
		a.Basic = figures.BasicNonFiling
		if a.Months > NonFilingMonthlyCapMonths {
			a.Months = NonFilingMonthlyCapMonths
		}
	default:
		a.Basic = figures.BasicLate
	}

	if MonthlyPenaltySuspended(asOf) {
		a.MonthlySuspended = true
		a.SuspensionNote = fmt.Sprintf(
			"The per-month component is left out: SEC Memorandum Circular No. 16, Series of 2026 suspended it from %s to %s. The basic fine is unaffected, amounts already settled are not refunded, and the monthly rate returns the day after.",
			MonthlyPenaltySuspendedFrom, MonthlyPenaltySuspendedUntil)
		a.Months = 0
	}

	a.Total = a.Basic + figures.MonthlyRate*int64(a.Months)
	a.Reason = fmt.Sprintf(
		"Computed from figures you entered on %s, quoting %s. Months of delay are counted as started calendar months. The Commission's own assessment governs.",
		figures.EnteredOn, figures.Source)
	return a
}

// DelinquencyWatch counts how close a company is to the statutory ground for
// delinquent status.
type DelinquencyWatch struct {
	Misses     int      `json:"misses"`
	Threshold  int      `json:"threshold"`
	WindowFrom Date     `json:"windowFrom"`
	WindowTo   Date     `json:"windowTo"`
	AtRisk     bool     `json:"atRisk"`
	Reached    bool     `json:"reached"`
	Detail     []string `json:"detail"`
	Note       string   `json:"note"`
}

// Basis records how this tool counts a miss, so the number can be argued with.
const delinquencyBasis = "Counted here as a requirement whose deadline fell inside the window and which is either still unfiled or was filed late enough to be assessed as a non-filing. Section 177 speaks of a failure to submit; the Commission counts offences from its own settled notices, so treat this as a watch list rather than an offence count."

// WatchDelinquency counts the misses in the five years to asOf across the
// obligations given.
func WatchDelinquency(obligations []Obligation, asOf Date) DelinquencyWatch {
	w := DelinquencyWatch{
		Threshold:  DelinquencyMissCount,
		WindowFrom: asOf.AddYears(-DelinquencyWindowYears),
		WindowTo:   asOf,
		Note:       delinquencyBasis,
	}
	for _, o := range obligations {
		if !o.Known || o.Due.IsZero() {
			continue
		}
		if o.Due.Before(w.WindowFrom) || o.Due.After(asOf) {
			continue
		}
		missed := false
		switch {
		case !o.Filed:
			missed = true
		case o.Exposure.Stage == StageTreatedAsNonFiling:
			missed = true
		}
		if !missed {
			continue
		}
		w.Misses++
		what := "not filed"
		if o.Filed {
			what = "filed late enough to be assessed as a non-filing"
		}
		w.Detail = append(w.Detail, fmt.Sprintf("%s due %s — %s", o.Requirement.Short(), o.Due, what))
	}
	w.Reached = w.Misses >= DelinquencyMissCount
	w.AtRisk = w.Misses >= DelinquencyMissCount-1
	return w
}

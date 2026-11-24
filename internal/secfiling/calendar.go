package secfiling

import (
	"fmt"
	"sort"
)

// Tier is how loud the reminder for one obligation should be.
type Tier string

const (
	TierFiled       Tier = "filed"
	TierUnknown     Tier = "unknown"
	TierComfortable Tier = "comfortable"
	TierPlan        Tier = "plan"
	TierPrepare     Tier = "prepare"
	TierSoon        Tier = "soon"
	TierUrgent      Tier = "urgent"
	TierFinal       Tier = "final"
	TierToday       Tier = "today"
	TierOverdue     Tier = "overdue"
)

// Label names the tier in the interface.
func (t Tier) Label() string {
	switch t {
	case TierFiled:
		return "Filed"
	case TierUnknown:
		return "No deadline yet"
	case TierComfortable:
		return "Not yet"
	case TierPlan:
		return "Plan"
	case TierPrepare:
		return "Prepare"
	case TierSoon:
		return "Due soon"
	case TierUrgent:
		return "Urgent"
	case TierFinal:
		return "Final call"
	case TierToday:
		return "Due today"
	case TierOverdue:
		return "Overdue"
	}
	return string(t)
}

// Rank orders the tiers for the board: the loudest first.
func (t Tier) Rank() int {
	switch t {
	case TierOverdue:
		return 0
	case TierToday:
		return 1
	case TierFinal:
		return 2
	case TierUrgent:
		return 3
	case TierSoon:
		return 4
	case TierPrepare:
		return 5
	case TierPlan:
		return 6
	case TierComfortable:
		return 7
	case TierUnknown:
		return 8
	case TierFiled:
		return 9
	}
	return 10
}

// TierFor places a number of days until the deadline on the reminder ladder.
func TierFor(daysUntil int, l ReminderLadder) Tier {
	switch {
	case daysUntil < 0:
		return TierOverdue
	case daysUntil == 0:
		return TierToday
	case daysUntil <= l.FinalDays:
		return TierFinal
	case daysUntil <= l.UrgentDays:
		return TierUrgent
	case daysUntil <= l.SoonDays:
		return TierSoon
	case daysUntil <= l.PrepareDays:
		return TierPrepare
	case daysUntil <= l.PlanDays:
		return TierPlan
	}
	return TierComfortable
}

// ReminderStep is one rung of the ladder for one deadline: the day the
// reminder starts and whether that day has arrived.
type ReminderStep struct {
	Tier    Tier   `json:"tier"`
	Label   string `json:"label"`
	Days    int    `json:"days"`
	Starts  Date   `json:"starts"`
	Reached bool   `json:"reached"`
}

// ReminderSteps lays out the whole ladder for one deadline, so the interface
// can show when each reminder starts rather than only the one showing now.
func ReminderSteps(due Date, l ReminderLadder, asOf Date) []ReminderStep {
	if due.IsZero() {
		return nil
	}
	rungs := []struct {
		tier Tier
		days int
	}{
		{TierPlan, l.PlanDays},
		{TierPrepare, l.PrepareDays},
		{TierSoon, l.SoonDays},
		{TierUrgent, l.UrgentDays},
		{TierFinal, l.FinalDays},
		{TierToday, 0},
	}
	steps := make([]ReminderStep, 0, len(rungs))
	for _, r := range rungs {
		starts := due.AddDays(-r.days)
		steps = append(steps, ReminderStep{
			Tier:    r.tier,
			Label:   r.tier.Label(),
			Days:    r.days,
			Starts:  starts,
			Reached: !asOf.Before(starts),
		})
	}
	return steps
}

// Obligation is one filing a company owes in one filing year: when it is due,
// how it was worked out, whether it has been filed, how loud the reminder is,
// and what being late leads to.
type Obligation struct {
	CompanyID   string `json:"companyId"`
	CompanyName string `json:"companyName"`
	Deadline
	FilingYear int            `json:"filingYear"`
	Filed      bool           `json:"filed"`
	FiledOn    Date           `json:"filedOn,omitempty"`
	Reference  string         `json:"reference,omitempty"`
	FiledBy    string         `json:"filedBy,omitempty"`
	Channel    string         `json:"channel,omitempty"`
	DaysUntil  int            `json:"daysUntil"`
	Tier       Tier           `json:"tier"`
	TierLabel  string         `json:"tierLabel"`
	Action     string         `json:"action"`
	Reminders  []ReminderStep `json:"reminders,omitempty"`
	Exposure   Exposure       `json:"exposure"`
}

// obligationFrom joins a deadline to what has been recorded against it.
func obligationFrom(b *Book, c *Company, d Deadline, filingYear int, asOf Date) Obligation {
	o := Obligation{
		CompanyID:   c.ID,
		CompanyName: c.Name,
		Deadline:    d,
		FilingYear:  filingYear,
	}

	if f, ok := b.FilingFor(c.ID, d.Requirement, filingYear); ok {
		o.Filed = true
		o.FiledOn = f.FiledOn
		o.Reference = f.Reference
		o.FiledBy = f.FiledBy
		o.Channel = f.Channel
	}

	o.Exposure = ExposureFor(c.EntityType, d.Due, o.FiledOn, asOf, b.Settings.Penalties)

	switch {
	case o.Filed:
		o.Tier = TierFiled
	case !d.Known:
		o.Tier = TierUnknown
	default:
		o.DaysUntil = asOf.DaysUntil(d.Due)
		o.Tier = TierFor(o.DaysUntil, b.Settings.Ladder)
		o.Reminders = ReminderSteps(d.Due, b.Settings.Ladder, asOf)
	}
	if d.Known && !d.Due.IsZero() {
		o.DaysUntil = asOf.DaysUntil(d.Due)
	}
	o.TierLabel = o.Tier.Label()
	o.Action = actionFor(o)
	return o
}

// actionFor is the escalating line of text shown against an obligation. It
// gets shorter and sharper as the deadline comes closer, and says what to do
// rather than only how many days are left.
func actionFor(o Obligation) string {
	what := o.Requirement.Short()
	switch o.Tier {
	case TierFiled:
		if o.Exposure.Stage == StageOnTime {
			return fmt.Sprintf("%s filed on %s. Reference %s.", what, o.FiledOn, o.Reference)
		}
		return fmt.Sprintf("%s filed on %s, after the %s deadline. Reference %s.", what, o.FiledOn, o.Due, o.Reference)
	case TierUnknown:
		return fmt.Sprintf("No %s deadline can be stated yet. %s", what, o.Reason)
	case TierOverdue:
		return fmt.Sprintf("%s was due on %s, %d days ago. File it and record the reference.", what, o.Due, -o.DaysUntil)
	case TierToday:
		return fmt.Sprintf("%s is due today. Filing after today counts as late.", what)
	case TierFinal:
		return fmt.Sprintf("%s is due in %d days. Upload it now; a submission made on a non-working day counts as filed on the next working day.", what, o.DaysUntil)
	case TierUrgent:
		return fmt.Sprintf("%s is due in %d days (%s). Have the signed copies and the attachments ready.", what, o.DaysUntil, o.Due)
	case TierSoon:
		return fmt.Sprintf("%s is due in %d days (%s). Book the signatories.", what, o.DaysUntil, o.Due)
	case TierPrepare:
		return fmt.Sprintf("%s is due on %s. Start the draft and the audit sign-off.", what, o.Due)
	case TierPlan:
		return fmt.Sprintf("%s is due on %s. Put it in the plan.", what, o.Due)
	}
	return fmt.Sprintf("%s is due on %s.", what, o.Due)
}

// CompanyObligations lists what one company owes in one filing year: the
// General Information Sheet, and the financial statements in whichever form
// that company's filer class owes them.
func CompanyObligations(b *Book, c *Company, filingYear int, asOf Date) []Obligation {
	out := []Obligation{
		obligationFrom(b, c, GISDeadline(c, b.Settings, filingYear), filingYear, asOf),
		obligationFrom(b, c, AFSDeadline(c, b.Settings, filingYear), filingYear, asOf),
	}
	sortObligations(out)
	return out
}

// Calendar is every company's obligations for one filing year, loudest first.
func Calendar(b *Book, filingYear int, asOf Date) []Obligation {
	out := []Obligation{}
	for i := range b.Companies {
		out = append(out, CompanyObligations(b, &b.Companies[i], filingYear, asOf)...)
	}
	sortObligations(out)
	return out
}

// History is one company's obligations across a run of filing years, used for
// the delinquency watch and for the record of what was filed when.
func History(b *Book, c *Company, fromYear, toYear int, asOf Date) []Obligation {
	out := []Obligation{}
	for year := fromYear; year <= toYear; year++ {
		out = append(out, CompanyObligations(b, c, year, asOf)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FilingYear != out[j].FilingYear {
			return out[i].FilingYear > out[j].FilingYear
		}
		return out[i].Requirement < out[j].Requirement
	})
	return out
}

func sortObligations(list []Obligation) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.Tier.Rank() != b.Tier.Rank() {
			return a.Tier.Rank() < b.Tier.Rank()
		}
		if a.Known != b.Known {
			return a.Known
		}
		if a.Due != b.Due {
			return a.Due.Before(b.Due)
		}
		if a.CompanyName != b.CompanyName {
			return a.CompanyName < b.CompanyName
		}
		return a.Requirement < b.Requirement
	})
}

// Counts summarises a calendar for the top of the board.
type Counts struct {
	Total       int `json:"total"`
	Overdue     int `json:"overdue"`
	DueThisWeek int `json:"dueThisWeek"`
	Filed       int `json:"filed"`
	NoDeadline  int `json:"noDeadline"`
}

// Summarise counts a calendar.
func Summarise(list []Obligation) Counts {
	var c Counts
	c.Total = len(list)
	for _, o := range list {
		switch {
		case o.Filed:
			c.Filed++
		case o.Tier == TierOverdue:
			c.Overdue++
		case o.Tier == TierUnknown:
			c.NoDeadline++
		}
		if !o.Filed && o.Known && o.DaysUntil >= 0 && o.DaysUntil <= 7 {
			c.DueThisWeek++
		}
	}
	return c
}

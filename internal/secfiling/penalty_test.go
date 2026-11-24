package secfiling

import (
	"strings"
	"testing"
)

func TestExposureStagesForADomesticCorporation(t *testing.T) {
	due := "2026-06-15"
	cases := []struct {
		name      string
		filedOn   string // empty means still unfiled
		asOf      string
		wantStage Stage
		wantDays  int
	}{
		{"filed well before the deadline", "2026-05-02", "2026-06-20", StageOnTime, 0},
		{"filed on the deadline itself", "2026-06-15", "2026-06-20", StageOnTime, 0},
		{"filed the day after", "2026-06-16", "2026-06-20", StageLate, 1},
		{"filed a year to the day later", "2027-06-15", "2027-06-20", StageLate, 365},
		{"filed a year and a day later", "2027-06-16", "2027-06-20", StageTreatedAsNonFiling, 366},
		{"not filed, deadline not reached", "", "2026-06-14", StageNotDue, 0},
		{"not filed, deadline is today", "", "2026-06-15", StageNotDue, 0},
		{"not filed, one day over", "", "2026-06-16", StageLate, 1},
		{"not filed, more than a year over", "", "2027-07-01", StageTreatedAsNonFiling, 381},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var filed Date
			if c.filedOn != "" {
				filed = d(t, c.filedOn)
			}
			got := ExposureFor(EntityStock, d(t, due), filed, d(t, c.asOf), PenaltyFigures{})
			if got.Stage != c.wantStage {
				t.Errorf("stage = %s, want %s", got.Stage, c.wantStage)
			}
			if got.DaysLate != c.wantDays {
				t.Errorf("days late = %d, want %d", got.DaysLate, c.wantDays)
			}
		})
	}
}

func TestTheOneYearNonFilingLineIsClampedOverALeapDay(t *testing.T) {
	// A deadline on 29 February: one year later is 28 February, not 1 March.
	due := d(t, "2024-02-29")
	onTheLine := ExposureFor(EntityStock, due, d(t, "2025-02-28"), d(t, "2025-03-05"), PenaltyFigures{})
	if onTheLine.Stage != StageLate {
		t.Errorf("filed on 2025-02-28 is exactly one year on, stage = %s, want %s", onTheLine.Stage, StageLate)
	}
	overTheLine := ExposureFor(EntityStock, due, d(t, "2025-03-01"), d(t, "2025-03-05"), PenaltyFigures{})
	if overTheLine.Stage != StageTreatedAsNonFiling {
		t.Errorf("filed on 2025-03-01 is past one year, stage = %s, want %s", overTheLine.Stage, StageTreatedAsNonFiling)
	}
}

func TestExposureStagesForAForeignCorporation(t *testing.T) {
	due := d(t, "2026-06-15")
	cases := []struct {
		name      string
		filedOn   string
		wantStage Stage
	}{
		{"on the deadline", "2026-06-15", StageOnTime},
		{"one day over", "2026-06-16", StageWithinForeignWindow},
		{"thirty days over", "2026-07-15", StageWithinForeignWindow},
		{"thirty-one days over", "2026-07-16", StageLate},
		{"sixty days over", "2026-08-14", StageLate},
		{"sixty-one days over", "2026-08-15", StageTreatedAsNonFiling},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExposureFor(EntityBranch, due, d(t, c.filedOn), d(t, "2026-09-01"), PenaltyFigures{})
			if got.Stage != c.wantStage {
				t.Errorf("stage = %s, want %s (%d days late)", got.Stage, c.wantStage, got.DaysLate)
			}
		})
	}
}

func TestEveryLateExposureNamesDelinquencyAndRevocation(t *testing.T) {
	got := ExposureFor(EntityStock, d(t, "2026-06-15"), Date{}, d(t, "2026-08-01"), PenaltyFigures{})
	for _, want := range []string{"delinquent status", "Section 177", "Republic Act No. 11232", "revoking"} {
		if !strings.Contains(got.Consequence, want) {
			t.Errorf("the consequence should name %q, got %q", want, got.Consequence)
		}
	}
}

func TestNoAmountIsComputedWithoutFiguresFromTheUser(t *testing.T) {
	got := ExposureFor(EntityStock, d(t, "2026-06-15"), Date{}, d(t, "2026-08-01"), PenaltyFigures{})
	if got.Amount.Computed {
		t.Fatal("the tool must not put a peso figure on a delay by itself")
	}
	if got.Amount.Total != 0 {
		t.Errorf("an uncomputed amount must stay zero, got %d", got.Amount.Total)
	}
	for _, want := range []string{"No. 6, Series of 2024", "No. 16, Series of 2026", "Settings"} {
		if !strings.Contains(got.Amount.Reason, want) {
			t.Errorf("the reason should mention %q, got %q", want, got.Amount.Reason)
		}
	}
}

func TestFiguresWithoutASourceAreNotUsed(t *testing.T) {
	figures := PenaltyFigures{BasicLate: 5000, MonthlyRate: 1000}
	if figures.Supplied() {
		t.Fatal("amounts with no issuance named are not usable")
	}
	got := ExposureFor(EntityStock, d(t, "2026-06-15"), Date{}, d(t, "2026-08-01"), figures)
	if got.Amount.Computed {
		t.Error("an amount was computed from figures with no source")
	}
}

func TestAmountFromUserFiguresIsLabelledAsTheirs(t *testing.T) {
	figures := PenaltyFigures{
		Source:         "SEC MC No. 6, s. 2024, Part II, tier read off by the filer",
		EnteredOn:      d(t, "2027-02-01"),
		BasicLate:      5000,
		BasicNonFiling: 10000,
		MonthlyRate:    1000,
	}
	// 2027: the suspension of the monthly component has ended.
	late := ExposureFor(EntityStock, d(t, "2026-11-30"), Date{}, d(t, "2027-02-01"), figures)
	if late.Stage != StageLate {
		t.Fatalf("stage = %s", late.Stage)
	}
	if !late.Amount.Computed {
		t.Fatalf("figures were supplied, so an amount should be computed: %s", late.Amount.Reason)
	}
	if late.Amount.Months != 3 || late.Amount.Basic != 5000 || late.Amount.Total != 8000 {
		t.Errorf("amount = basic %d + %d x %d months = %d, want 5000 + 1000 x 3 = 8000",
			late.Amount.Basic, late.Amount.MonthlyRate, late.Amount.Months, late.Amount.Total)
	}
	if !strings.Contains(late.Amount.Reason, "figures you entered") {
		t.Errorf("the amount should be labelled as the user's, got %q", late.Amount.Reason)
	}
}

func TestTheMonthlyComponentIsCappedForANonFiling(t *testing.T) {
	figures := PenaltyFigures{
		Source:         "SEC MC No. 6, s. 2024",
		EnteredOn:      d(t, "2028-01-10"),
		BasicLate:      5000,
		BasicNonFiling: 10000,
		MonthlyRate:    1000,
	}
	// Due at the end of 2026, still unfiled in 2028: well past both the
	// one-year line and the twelve-month cap.
	got := ExposureFor(EntityStock, d(t, "2026-11-30"), Date{}, d(t, "2028-01-10"), figures)
	if got.Stage != StageTreatedAsNonFiling {
		t.Fatalf("stage = %s, want %s", got.Stage, StageTreatedAsNonFiling)
	}
	if got.MonthsLate <= NonFilingMonthlyCapMonths {
		t.Fatalf("this case needs more than %d months of delay, got %d", NonFilingMonthlyCapMonths, got.MonthsLate)
	}
	if got.Amount.Months != NonFilingMonthlyCapMonths {
		t.Errorf("months charged = %d, want the cap of %d", got.Amount.Months, NonFilingMonthlyCapMonths)
	}
	if got.Amount.Total != 10000+1000*NonFilingMonthlyCapMonths {
		t.Errorf("total = %d, want %d", got.Amount.Total, 10000+1000*NonFilingMonthlyCapMonths)
	}
}

func TestTheMonthlyComponentIsSuspendedInsideTheCircularsWindow(t *testing.T) {
	cases := []struct {
		asOf          string
		wantSuspended bool
	}{
		{"2026-05-13", false},
		{"2026-05-14", true}, // the day the suspension starts
		{"2026-08-01", true},
		{"2026-12-31", true}, // the last day of the suspension
		{"2027-01-01", false},
	}
	for _, c := range cases {
		if got := MonthlyPenaltySuspended(d(t, c.asOf)); got != c.wantSuspended {
			t.Errorf("MonthlyPenaltySuspended(%s) = %v, want %v", c.asOf, got, c.wantSuspended)
		}
	}

	figures := PenaltyFigures{
		Source: "SEC MC No. 6, s. 2024", EnteredOn: d(t, "2026-08-01"),
		BasicLate: 5000, MonthlyRate: 1000,
	}
	inside := ExposureFor(EntityStock, d(t, "2026-01-15"), Date{}, d(t, "2026-08-01"), figures)
	if !inside.Amount.MonthlySuspended {
		t.Fatal("the monthly component should be suspended on 2026-08-01")
	}
	if inside.Amount.Total != 5000 {
		t.Errorf("total = %d, want the basic fine alone (5000)", inside.Amount.Total)
	}
	if !strings.Contains(inside.Amount.SuspensionNote, "No. 16, Series of 2026") {
		t.Errorf("the note should name the suspending circular, got %q", inside.Amount.SuspensionNote)
	}
	if inside.MonthsLate == 0 {
		t.Error("the delay itself is still counted even while the monthly fine is suspended")
	}
}

func TestExposureWithNoDeadlineOwesNothing(t *testing.T) {
	got := ExposureFor(EntityStock, Date{}, Date{}, d(t, "2026-08-01"), PenaltyFigures{})
	if got.Stage != StageNoDeadline {
		t.Errorf("stage = %s, want %s", got.Stage, StageNoDeadline)
	}
	if got.Amount.Computed {
		t.Error("nothing can be computed without a deadline")
	}
}

func TestWatchDelinquencyCountsMissesInsideTheFiveYearWindow(t *testing.T) {
	asOf := d(t, "2026-11-24")
	mk := func(due string, filed bool, stage Stage) Obligation {
		return Obligation{
			Deadline: Deadline{Requirement: RequirementGIS, Known: true, Due: d(t, due)},
			Filed:    filed,
			Exposure: Exposure{Stage: stage},
		}
	}
	cases := []struct {
		name        string
		obligations []Obligation
		wantMisses  int
		wantAtRisk  bool
		wantReached bool
	}{
		{"nothing missed", []Obligation{
			mk("2024-05-30", true, StageOnTime),
			mk("2025-05-30", true, StageOnTime),
		}, 0, false, false},
		{"one unfiled", []Obligation{
			mk("2024-05-30", false, StageLate),
			mk("2025-05-30", true, StageOnTime),
		}, 1, false, false},
		{"two unfiled is one short of the ground", []Obligation{
			mk("2023-05-30", false, StageLate),
			mk("2024-05-30", false, StageLate),
		}, 2, true, false},
		{"three reaches the statutory ground", []Obligation{
			mk("2023-05-30", false, StageLate),
			mk("2024-05-30", false, StageLate),
			mk("2025-05-30", false, StageLate),
		}, 3, true, true},
		{"a filing assessed as a non-filing still counts", []Obligation{
			mk("2023-05-30", true, StageTreatedAsNonFiling),
			mk("2024-05-30", true, StageTreatedAsNonFiling),
			mk("2025-05-30", true, StageTreatedAsNonFiling),
		}, 3, true, true},
		{"older than five years falls out of the window", []Obligation{
			mk("2019-05-30", false, StageLate),
			mk("2020-05-30", false, StageLate),
			mk("2024-05-30", false, StageLate),
		}, 1, false, false},
		{"a deadline still in the future is not a miss", []Obligation{
			mk("2027-05-30", false, StageNotDue),
		}, 0, false, false},
		{"an unknown deadline cannot be a miss", []Obligation{
			{Deadline: Deadline{Requirement: RequirementGIS, Known: false}},
		}, 0, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := WatchDelinquency(c.obligations, asOf)
			if got.Misses != c.wantMisses {
				t.Errorf("misses = %d, want %d (%v)", got.Misses, c.wantMisses, got.Detail)
			}
			if got.AtRisk != c.wantAtRisk {
				t.Errorf("at risk = %v, want %v", got.AtRisk, c.wantAtRisk)
			}
			if got.Reached != c.wantReached {
				t.Errorf("reached = %v, want %v", got.Reached, c.wantReached)
			}
			if got.WindowFrom.String() != "2021-11-24" {
				t.Errorf("window starts %s, want 2021-11-24", got.WindowFrom)
			}
			if len(got.Detail) != got.Misses {
				t.Errorf("%d misses but %d lines of detail", got.Misses, len(got.Detail))
			}
		})
	}
}

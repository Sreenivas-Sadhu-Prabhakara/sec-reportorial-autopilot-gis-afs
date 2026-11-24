package secfiling

import (
	"strings"
	"testing"
)

func TestTierForWalksUpTheLadderAtEveryBoundary(t *testing.T) {
	ladder := DefaultLadder()
	cases := []struct {
		days int
		want Tier
	}{
		{365, TierComfortable},
		{91, TierComfortable},
		{90, TierPlan}, // the plan reminder starts exactly 90 days out
		{61, TierPlan},
		{60, TierPrepare},
		{31, TierPrepare},
		{30, TierSoon},
		{15, TierSoon},
		{14, TierUrgent},
		{4, TierUrgent},
		{3, TierFinal},
		{1, TierFinal},
		{0, TierToday},
		{-1, TierOverdue},
		{-400, TierOverdue},
	}
	for _, c := range cases {
		if got := TierFor(c.days, ladder); got != c.want {
			t.Errorf("TierFor(%d) = %s, want %s", c.days, got, c.want)
		}
	}
}

func TestTierForFollowsACustomLadder(t *testing.T) {
	ladder := ReminderLadder{PlanDays: 30, PrepareDays: 20, SoonDays: 10, UrgentDays: 5, FinalDays: 2}
	cases := []struct {
		days int
		want Tier
	}{
		{31, TierComfortable},
		{30, TierPlan},
		{20, TierPrepare},
		{10, TierSoon},
		{5, TierUrgent},
		{2, TierFinal},
		{0, TierToday},
	}
	for _, c := range cases {
		if got := TierFor(c.days, ladder); got != c.want {
			t.Errorf("TierFor(%d) with a custom ladder = %s, want %s", c.days, got, c.want)
		}
	}
}

func TestReminderStepsCountBackFromTheDeadline(t *testing.T) {
	steps := ReminderSteps(d(t, "2026-06-15"), DefaultLadder(), d(t, "2026-05-01"))
	want := []struct {
		tier    Tier
		starts  string
		reached bool
	}{
		{TierPlan, "2026-03-17", true},
		{TierPrepare, "2026-04-16", true},
		{TierSoon, "2026-05-16", false},
		{TierUrgent, "2026-06-01", false},
		{TierFinal, "2026-06-12", false},
		{TierToday, "2026-06-15", false},
	}
	if len(steps) != len(want) {
		t.Fatalf("%d steps, want %d", len(steps), len(want))
	}
	for i, w := range want {
		if steps[i].Tier != w.tier || steps[i].Starts.String() != w.starts || steps[i].Reached != w.reached {
			t.Errorf("step %d = %s on %s (reached %v), want %s on %s (reached %v)",
				i, steps[i].Tier, steps[i].Starts, steps[i].Reached, w.tier, w.starts, w.reached)
		}
	}
	if ReminderSteps(Date{}, DefaultLadder(), d(t, "2026-05-01")) != nil {
		t.Error("there are no reminders without a deadline")
	}
}

// book builds a book holding one company that met on 20 April 2026.
func book(t *testing.T) (*Book, *Company) {
	t.Helper()
	b := New()
	c := *company()
	if err := b.PutCompany(c); err != nil {
		t.Fatalf("PutCompany: %v", err)
	}
	if err := b.PutMeeting("c1", Meeting{Year: 2026, Date: d(t, "2026-04-20"), Held: true}); err != nil {
		t.Fatalf("PutMeeting: %v", err)
	}
	got, _ := b.Company("c1")
	return b, got
}

func TestCompanyObligationsCoverBothFilingsAndPickUpTheRecord(t *testing.T) {
	b, c := book(t)
	asOf := d(t, "2026-05-01")

	got := CompanyObligations(b, c, 2026, asOf)
	if len(got) != 2 {
		t.Fatalf("%d obligations, want 2", len(got))
	}
	byReq := map[Requirement]Obligation{}
	for _, o := range got {
		byReq[o.Requirement] = o
	}
	gis, ok := byReq[RequirementGIS]
	if !ok {
		t.Fatal("no GIS obligation")
	}
	if gis.Due.String() != "2026-05-20" || gis.DaysUntil != 19 {
		t.Errorf("GIS due %s in %d days, want 2026-05-20 in 19 days", gis.Due, gis.DaysUntil)
	}
	if gis.Tier != TierSoon {
		t.Errorf("GIS tier = %s, want %s", gis.Tier, TierSoon)
	}
	afs, ok := byReq[RequirementAFS]
	if !ok {
		t.Fatal("no AFS obligation")
	}
	if afs.Due.String() != "2026-06-15" || afs.Tier != TierPrepare {
		t.Errorf("AFS due %s tier %s, want 2026-06-15 and %s", afs.Due, afs.Tier, TierPrepare)
	}

	// Record the GIS filing and it drops to the bottom of the board, carrying
	// its proof reference.
	if err := b.PutFiling(Filing{
		ID: "f1", CompanyID: "c1", Requirement: RequirementGIS, FilingYear: 2026,
		FiledOn: d(t, "2026-05-12"), Reference: "EFAST-2026-0001", FiledBy: "R. Cruz",
	}); err != nil {
		t.Fatalf("PutFiling: %v", err)
	}
	after := CompanyObligations(b, c, 2026, asOf)
	var filed Obligation
	for _, o := range after {
		if o.Requirement == RequirementGIS {
			filed = o
		}
	}
	if !filed.Filed || filed.Reference != "EFAST-2026-0001" {
		t.Fatalf("the filing record did not reach the obligation: %+v", filed)
	}
	if filed.Tier != TierFiled {
		t.Errorf("tier = %s, want %s", filed.Tier, TierFiled)
	}
	if filed.Exposure.Stage != StageOnTime {
		t.Errorf("filed 2026-05-12 against a 2026-05-20 deadline is on time, got %s", filed.Exposure.Stage)
	}
	if !strings.Contains(filed.Action, "EFAST-2026-0001") {
		t.Errorf("the line shown should carry the proof reference, got %q", filed.Action)
	}
}

func TestTheBoardPutsTheLoudestFirst(t *testing.T) {
	b := New()
	type spec struct {
		id      string
		name    string
		meeting string
	}
	for _, s := range []spec{
		{"a", "Alpha Holdings", "2026-01-10"}, // GIS due 2026-02-09 — overdue by May
		{"b", "Bravo Logistics", "2026-04-20"},
		{"c", "Charlie Foods", "2026-06-01"}, // GIS due 2026-07-01
	} {
		c := *company()
		c.ID, c.Name = s.id, s.name
		if err := b.PutCompany(c); err != nil {
			t.Fatalf("PutCompany: %v", err)
		}
		if err := b.PutMeeting(s.id, Meeting{Year: 2026, Date: d(t, s.meeting), Held: true}); err != nil {
			t.Fatalf("PutMeeting: %v", err)
		}
	}

	got := Calendar(b, 2026, d(t, "2026-05-01"))
	if len(got) != 6 {
		t.Fatalf("%d obligations across three companies, want 6", len(got))
	}
	if got[0].CompanyID != "a" || got[0].Requirement != RequirementGIS || got[0].Tier != TierOverdue {
		t.Errorf("the overdue GIS should lead the board, got %s/%s/%s",
			got[0].CompanyID, got[0].Requirement, got[0].Tier)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Tier.Rank() > got[i].Tier.Rank() {
			t.Fatalf("the board is out of order at %d: %s then %s", i, got[i-1].Tier, got[i].Tier)
		}
	}

	counts := Summarise(got)
	if counts.Total != 6 || counts.Overdue != 1 || counts.Filed != 0 {
		t.Errorf("counts = %+v, want 6 total and 1 overdue", counts)
	}
}

func TestSummariseCountsTheWeekAheadAndTheUnknowable(t *testing.T) {
	b := New()
	withMeeting := *company()
	withMeeting.ID, withMeeting.Name = "m", "Met Already"
	if err := b.PutCompany(withMeeting); err != nil {
		t.Fatalf("PutCompany: %v", err)
	}
	if err := b.PutMeeting("m", Meeting{Year: 2026, Date: d(t, "2026-04-25"), Held: true}); err != nil {
		t.Fatalf("PutMeeting: %v", err)
	}
	noMeeting := *company()
	noMeeting.ID, noMeeting.Name = "n", "Not Met Yet"
	if err := b.PutCompany(noMeeting); err != nil {
		t.Fatalf("PutCompany: %v", err)
	}

	// 2026-05-20 is five days before the GIS deadline of 2026-05-25.
	got := Summarise(Calendar(b, 2026, d(t, "2026-05-20")))
	if got.DueThisWeek != 1 {
		t.Errorf("due this week = %d, want 1", got.DueThisWeek)
	}
	if got.NoDeadline != 1 {
		t.Errorf("no deadline = %d, want 1 (the company that has not met)", got.NoDeadline)
	}
}

func TestHistoryRunsBackwardsAcrossFilingYears(t *testing.T) {
	b, c := book(t)
	for year, date := range map[int]string{2023: "2023-04-18", 2024: "2024-04-16", 2025: "2025-04-22"} {
		if err := b.PutMeeting("c1", Meeting{Year: year, Date: d(t, date), Held: true}); err != nil {
			t.Fatalf("PutMeeting %d: %v", year, err)
		}
	}
	got := History(b, c, 2023, 2026, d(t, "2026-11-24"))
	if len(got) != 8 {
		t.Fatalf("%d rows for four years of two filings, want 8", len(got))
	}
	if got[0].FilingYear != 2026 || got[len(got)-1].FilingYear != 2023 {
		t.Errorf("history should run newest first, got %d then %d", got[0].FilingYear, got[len(got)-1].FilingYear)
	}

	// Four unfiled GIS deadlines plus the 2026 AFS, all inside the window.
	watch := WatchDelinquency(got, d(t, "2026-11-24"))
	if !watch.Reached {
		t.Errorf("four years of unfiled GIS should reach the statutory ground, misses = %d", watch.Misses)
	}
}

func TestActionTextEscalates(t *testing.T) {
	b, c := book(t)
	cases := []struct {
		asOf string
		want string
	}{
		{"2026-01-01", "GIS is due on 2026-05-20."},
		{"2026-03-01", "Put it in the plan"},
		{"2026-04-01", "Start the draft"},
		{"2026-05-01", "Book the signatories"},
		{"2026-05-10", "signed copies"},
		{"2026-05-18", "Upload it now"},
		{"2026-05-20", "due today"},
		{"2026-06-01", "days ago"},
	}
	for _, tc := range cases {
		var gis Obligation
		for _, o := range CompanyObligations(b, c, 2026, d(t, tc.asOf)) {
			if o.Requirement == RequirementGIS {
				gis = o
			}
		}
		if !strings.Contains(gis.Action, tc.want) {
			t.Errorf("on %s the GIS line is %q, want it to contain %q", tc.asOf, gis.Action, tc.want)
		}
	}
}

func TestAnObligationWithNoDeadlineExplainsItself(t *testing.T) {
	b := New()
	c := *company()
	if err := b.PutCompany(c); err != nil {
		t.Fatalf("PutCompany: %v", err)
	}
	got, _ := b.Company("c1")
	for _, o := range CompanyObligations(b, got, 2026, d(t, "2026-05-01")) {
		if o.Requirement != RequirementGIS {
			continue
		}
		if o.Tier != TierUnknown {
			t.Fatalf("tier = %s, want %s", o.Tier, TierUnknown)
		}
		if !strings.Contains(o.Action, "actually held") {
			t.Errorf("the line should explain why there is no date, got %q", o.Action)
		}
		if o.Exposure.Stage != StageNoDeadline {
			t.Errorf("exposure = %s, want %s", o.Exposure.Stage, StageNoDeadline)
		}
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "register.json"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!DOCTYPE html>")}}
	return NewServer(store, fs.FS(ui))
}

func call(t *testing.T, s *Server, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	out := map[string]any{}
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: response is not JSON: %s", method, path, rec.Body.String())
		}
	}
	return rec, out
}

func addCompany(t *testing.T, s *Server, id, name string, extra map[string]any) {
	t.Helper()
	body := map[string]any{
		"id": id, "name": name, "registrationNumber": "CS201812345",
		"entityType": "stock", "filerClass": "ordinary",
		"fiscalYearEndMonth": 12, "fiscalYearEndDay": 31, "gisBasis": "meeting",
	}
	for k, v := range extra {
		body[k] = v
	}
	rec, got := call(t, s, http.MethodPut, "/api/company", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/company: %d %v", rec.Code, got)
	}
}

func TestStateCarriesTheRulesAndTheDisclaimer(t *testing.T) {
	s := newTestServer(t)
	rec, got := call(t, s, http.MethodGet, "/api/state", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	disclaimer, _ := got["disclaimer"].(string)
	for _, want := range []string{"does not file", "eFAST"} {
		if !strings.Contains(disclaimer, want) {
			t.Errorf("the disclaimer should say %q, got %q", want, disclaimer)
		}
	}
	if rules, ok := got["rules"].([]any); !ok || len(rules) < 8 {
		t.Errorf("the state should carry every rule with its source, got %v", got["rules"])
	}
	if schedule, ok := got["schedule"].([]any); !ok || len(schedule) != 3 {
		t.Errorf("the published schedule should be carried, got %v", got["schedule"])
	}
	if asOf, _ := got["scheduleAsOf"].(string); asOf == "" {
		t.Error("the state should say when the schedule was last checked")
	}
	note, _ := got["registrationNumberNote"].(string)
	if !strings.Contains(note, "does not set the filing window") {
		t.Errorf("registration number note = %q", note)
	}
	penalty, _ := got["penaltyNote"].(string)
	if !strings.Contains(penalty, "does not compute a peso penalty") {
		t.Errorf("penalty note = %q", penalty)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("the response should be sent with nosniff")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a register should not be cached")
	}
}

func TestTheCalendarFollowsTheMeetingThatWasRecorded(t *testing.T) {
	s := newTestServer(t)
	addCompany(t, s, "c1", "Mabuhay Freight Services Inc.", nil)

	// Before the meeting there is no GIS deadline to state.
	_, got := call(t, s, http.MethodGet, "/api/calendar?year=2026&asOf=2026-05-01", nil)
	obligations, _ := got["obligations"].([]any)
	if len(obligations) != 2 {
		t.Fatalf("%d obligations, want 2", len(obligations))
	}
	var gis map[string]any
	for _, raw := range obligations {
		o := raw.(map[string]any)
		if o["requirement"] == "gis" {
			gis = o
		}
	}
	if gis["known"] != false {
		t.Fatalf("the GIS should have no deadline before the meeting: %v", gis)
	}

	// Record the meeting and the deadline appears, thirty days later.
	rec, _ := call(t, s, http.MethodPut, "/api/meeting?company=c1",
		map[string]any{"year": 2026, "date": "2026-04-20", "held": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/meeting: %d", rec.Code)
	}
	_, got = call(t, s, http.MethodGet, "/api/calendar?year=2026&asOf=2026-05-01", nil)
	obligations, _ = got["obligations"].([]any)
	for _, raw := range obligations {
		o := raw.(map[string]any)
		if o["requirement"] != "gis" {
			continue
		}
		if o["due"] != "2026-05-20" {
			t.Errorf("GIS due = %v, want 2026-05-20", o["due"])
		}
		if o["tier"] != "soon" {
			t.Errorf("tier = %v, want soon", o["tier"])
		}
	}
	counts, _ := got["counts"].(map[string]any)
	if counts["total"].(float64) != 2 {
		t.Errorf("counts = %v", counts)
	}
}

func TestRecordingAFilingNeedsItsProofReference(t *testing.T) {
	s := newTestServer(t)
	addCompany(t, s, "c1", "Mabuhay Freight Services Inc.", nil)

	rec, got := call(t, s, http.MethodPut, "/api/filing", map[string]any{
		"id": "f1", "companyId": "c1", "requirement": "gis", "filingYear": 2026,
		"filedOn": "2026-05-12",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("a filing with no reference should be refused, got %d", rec.Code)
	}
	if msg, _ := got["error"].(string); !strings.Contains(msg, "proof") {
		t.Errorf("error = %q", msg)
	}

	rec, _ = call(t, s, http.MethodPut, "/api/filing", map[string]any{
		"id": "f1", "companyId": "c1", "requirement": "gis", "filingYear": 2026,
		"filedOn": "2026-05-12", "reference": "EFAST-2026-0001", "filedBy": "R. Cruz",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/filing: %d", rec.Code)
	}

	_, state := call(t, s, http.MethodGet, "/api/state", nil)
	filings, _ := state["filings"].([]any)
	if len(filings) != 1 {
		t.Fatalf("%d filings recorded, want 1", len(filings))
	}
	f := filings[0].(map[string]any)
	if f["reference"] != "EFAST-2026-0001" || f["channel"] != "eFAST" {
		t.Errorf("filing = %v", f)
	}
}

func TestCompanyDetailCarriesTheWatchAndTheHistory(t *testing.T) {
	s := newTestServer(t)
	addCompany(t, s, "c1", "Mabuhay Freight Services Inc.", nil)
	for year, date := range map[int]string{2022: "2022-04-20", 2023: "2023-04-19", 2024: "2024-04-17"} {
		rec, _ := call(t, s, http.MethodPut, "/api/meeting?company=c1",
			map[string]any{"year": year, "date": date, "held": true})
		if rec.Code != http.StatusOK {
			t.Fatalf("PUT /api/meeting %d: %d", year, rec.Code)
		}
	}

	rec, got := call(t, s, http.MethodGet, "/api/company?id=c1&year=2026&asOf=2026-11-24", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	watch, _ := got["delinquency"].(map[string]any)
	if watch["misses"].(float64) < 3 {
		t.Errorf("three unfiled GIS filings should be counted, got %v", watch)
	}
	if watch["reached"] != true {
		t.Errorf("three misses is the statutory ground, got %v", watch)
	}
	history, _ := got["history"].([]any)
	if len(history) == 0 {
		t.Error("the five-year record should be returned")
	}
	note, _ := got["registrationNumberNote"].(string)
	if !strings.Contains(note, "Last digit 5") {
		t.Errorf("the note should name the last digit, got %q", note)
	}

	rec, _ = call(t, s, http.MethodGet, "/api/company?id=ghost", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("an unknown company should be 404, got %d", rec.Code)
	}
}

func TestAnOverrideSuppliesASeasonTheBuildDoesNotHold(t *testing.T) {
	s := newTestServer(t)
	addCompany(t, s, "c1", "Mabuhay Freight Services Inc.", nil)

	// 2027 is not in this build.
	_, got := call(t, s, http.MethodGet, "/api/calendar?year=2027&asOf=2027-01-04", nil)
	obligations, _ := got["obligations"].([]any)
	for _, raw := range obligations {
		o := raw.(map[string]any)
		if o["requirement"] == "afs" && o["known"] != false {
			t.Fatalf("the 2027 AFS deadline should not be guessed: %v", o)
		}
	}

	rec, body := call(t, s, http.MethodPut, "/api/settings", map[string]any{
		"holidays":       map[string]string{},
		"reminderLadder": map[string]int{"planDays": 90, "prepareDays": 60, "soonDays": 30, "urgentDays": 14, "finalDays": 3},
		"deadlineOverrides": []map[string]any{{
			"requirement": "afs", "filingYear": 2027, "due": "2027-06-14",
			"source": "SEC Memorandum Circular No. 4, Series of 2027",
		}},
		"penaltyFigures": map[string]any{},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/settings: %d %v", rec.Code, body)
	}

	_, got = call(t, s, http.MethodGet, "/api/calendar?year=2027&asOf=2027-01-04", nil)
	obligations, _ = got["obligations"].([]any)
	found := false
	for _, raw := range obligations {
		o := raw.(map[string]any)
		if o["requirement"] != "afs" {
			continue
		}
		found = true
		if o["due"] != "2027-06-14" || o["overridden"] != true {
			t.Errorf("the override should be used and marked: %v", o)
		}
	}
	if !found {
		t.Error("no AFS obligation came back")
	}
}

func TestSettingsRefuseAmountsWithNoIssuanceNamed(t *testing.T) {
	s := newTestServer(t)
	rec, got := call(t, s, http.MethodPut, "/api/settings", map[string]any{
		"holidays":       map[string]string{},
		"reminderLadder": map[string]int{"planDays": 90, "prepareDays": 60, "soonDays": 30, "urgentDays": 14, "finalDays": 3},
		"penaltyFigures": map[string]any{"basicLate": 5000},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
	if msg, _ := got["error"].(string); !strings.Contains(msg, "which issuance") {
		t.Errorf("error = %q", msg)
	}
}

func TestBadInputIsRefusedWithAReadableMessage(t *testing.T) {
	s := newTestServer(t)
	cases := []struct {
		name   string
		method string
		path   string
		body   any
		want   string
	}{
		{"a company with no name", http.MethodPut, "/api/company",
			map[string]any{"id": "x", "entityType": "stock", "filerClass": "ordinary", "gisBasis": "meeting"}, "needs a name"},
		{"an as-of date that does not exist", http.MethodGet, "/api/calendar?asOf=2026-02-30", nil, "not a date"},
		{"a company that was never added", http.MethodDelete, "/api/company?id=ghost", nil, "no company"},
		{"a meeting for nobody", http.MethodPut, "/api/meeting?company=ghost",
			map[string]any{"year": 2026, "date": "2026-04-20"}, "no company"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec, got := call(t, s, c.method, c.path, c.body)
			if rec.Code < 400 {
				t.Fatalf("status %d, want an error", rec.Code)
			}
			if msg, _ := got["error"].(string); !strings.Contains(msg, c.want) {
				t.Errorf("error = %q, want it to mention %q", msg, c.want)
			}
		})
	}
}

func TestExportCarriesTheDerivationNotJustTheDate(t *testing.T) {
	s := newTestServer(t)
	addCompany(t, s, "c1", "Mabuhay Freight Services Inc.", nil)
	if rec, _ := call(t, s, http.MethodPut, "/api/meeting?company=c1",
		map[string]any{"year": 2026, "date": "2026-04-20", "held": true}); rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/meeting: %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/export.csv?year=2026&asOf=2026-05-01", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"How the date was worked out",
		"Mabuhay Freight Services Inc.",
		"2026-05-20",
		"2026-06-15",
		"Section 5(a)",
		"Notice dated 14 April 2026",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the export should contain %q:\n%s", want, body)
		}
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("content type = %q", ct)
	}
}

func TestTheInterfaceIsServedFromTheBinary(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<!DOCTYPE html>") {
		t.Errorf("the embedded interface should be served, got %d %q", rec.Code, rec.Body.String())
	}
}

func TestTheShippedInterfaceHasNoInlineStylesOrScripts(t *testing.T) {
	for _, path := range []string{"web/index.html", "docs/index.html"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		page := string(raw)
		for _, forbidden := range []string{"<style", "style=\"", "onclick=", "oninput=", "onchange=", "onsubmit=", "onload="} {
			if strings.Contains(page, forbidden) {
				t.Errorf("%s contains %q", path, forbidden)
			}
		}
		if strings.Contains(page, "<script>") {
			t.Errorf("%s has an inline script", path)
		}
		if !strings.Contains(page, "Content-Security-Policy") {
			t.Errorf("%s has no content security policy", path)
		}
	}
}

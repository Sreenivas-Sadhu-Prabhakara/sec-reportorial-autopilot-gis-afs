package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/sreenivas-sadhu-prabhakara/sec-reportorial-autopilot-gis-afs/internal/secfiling"
)

// maxBodyBytes caps every request body. The interface never sends anything
// large.
const maxBodyBytes = 1 << 20 // 1 MiB

// NotAFilingPortal is the one thing this tool says on every surface. It is
// here, in the API, so it reaches anything built on top of it too.
const NotAFilingPortal = "This tool records and reminds. It does not file anything with the Securities and Exchange Commission, has no connection to eFAST or to any other portal, and sends nothing anywhere. Every filing is still made by you, in eFAST."

// Server wires the local interface and the JSON API over one Store.
type Server struct {
	store *Store
	ui    fs.FS
	mux   *http.ServeMux
}

// NewServer builds the router.
func NewServer(store *Store, ui fs.FS) *Server {
	s := &Server{store: store, ui: ui, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A local tool still sets the headers that stop a stray page in another
	// tab from reading the register.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.Handle("GET /", http.FileServerFS(s.ui))

	s.mux.HandleFunc("GET /api/state", s.handleState)
	s.mux.HandleFunc("GET /api/calendar", s.handleCalendar)
	s.mux.HandleFunc("GET /api/company", s.handleCompanyDetail)
	s.mux.HandleFunc("GET /api/export.csv", s.handleExportCSV)

	s.mux.HandleFunc("PUT /api/company", s.handlePutCompany)
	s.mux.HandleFunc("DELETE /api/company", s.handleDeleteCompany)
	s.mux.HandleFunc("PUT /api/meeting", s.handlePutMeeting)
	s.mux.HandleFunc("DELETE /api/meeting", s.handleDeleteMeeting)
	s.mux.HandleFunc("PUT /api/filing", s.handlePutFiling)
	s.mux.HandleFunc("DELETE /api/filing", s.handleDeleteFiling)
	s.mux.HandleFunc("PUT /api/settings", s.handlePutSettings)
}

// ───────────────────────────────── reads ─────────────────────────────────

type option struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Note  string `json:"note,omitempty"`
}

type ruleNote struct {
	Rule   string `json:"rule"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

type stateResponse struct {
	Today         secfiling.Date                `json:"today"`
	DataFile      string                        `json:"dataFile"`
	Disclaimer    string                        `json:"disclaimer"`
	Companies     []secfiling.Company           `json:"companies"`
	Filings       []secfiling.Filing            `json:"filings"`
	Settings      secfiling.Settings            `json:"settings"`
	EntityTypes   []option                      `json:"entityTypes"`
	FilerClasses  []option                      `json:"filerClasses"`
	GISBases      []option                      `json:"gisBases"`
	Requirements  []option                      `json:"requirements"`
	Rules         []ruleNote                    `json:"rules"`
	Schedule      []secfiling.PublishedDeadline `json:"schedule"`
	ScheduleAsOf  secfiling.Date                `json:"scheduleAsOf"`
	RegNumberNote string                        `json:"registrationNumberNote"`
	PenaltyNote   string                        `json:"penaltyNote"`
	DefaultLadder secfiling.ReminderLadder      `json:"defaultLadder"`
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	out := stateResponse{
		Today:         secfiling.Today(),
		DataFile:      s.store.Path(),
		Disclaimer:    NotAFilingPortal,
		ScheduleAsOf:  secfiling.ScheduleAsOf,
		Schedule:      secfiling.PublishedSchedule(),
		RegNumberNote: secfiling.TheRegistrationNumberDoesNotSetTheAFSWindow,
		PenaltyNote:   secfiling.WhyNoAmount,
		Rules:         rules(),
		DefaultLadder: secfiling.DefaultLadder(),
	}
	for _, e := range secfiling.AllEntityTypes() {
		out.EntityTypes = append(out.EntityTypes, option{Value: string(e), Label: e.Label()})
	}
	for _, f := range secfiling.AllFilerClasses() {
		out.FilerClasses = append(out.FilerClasses, option{Value: string(f), Label: f.Label(),
			Note: f.Requirement().Label()})
	}
	for _, g := range secfiling.AllGISBases() {
		out.GISBases = append(out.GISBases, option{Value: string(g), Label: g.Label()})
	}
	for _, req := range secfiling.AllRequirements() {
		out.Requirements = append(out.Requirements, option{Value: string(req), Label: req.Label(), Note: req.Short()})
	}

	s.store.Read(func(b *secfiling.Book) {
		out.Companies = append(out.Companies, b.Companies...)
		out.Filings = append(out.Filings, b.Filings...)
		out.Settings = b.Settings
	})
	if out.Companies == nil {
		out.Companies = []secfiling.Company{}
	}
	if out.Filings == nil {
		out.Filings = []secfiling.Filing{}
	}
	writeJSON(w, http.StatusOK, out)
}

// rules states every figure the tool counts with, next to the issuance it came
// from, so the interface can show the whole basis on one screen.
func rules() []ruleNote {
	const (
		mc9  = "SEC Memorandum Circular No. 9, Series of 2026"
		mc6  = "SEC Memorandum Circular No. 6, Series of 2024"
		mc16 = "SEC Memorandum Circular No. 16, Series of 2026"
		rcc  = "Section 177, Revised Corporation Code (Republic Act No. 11232)"
	)
	return []ruleNote{
		{"General Information Sheet", fmt.Sprintf("%d calendar days from the date of the actual annual meeting, or from the anniversary of the SEC licence for a foreign corporation", secfiling.GISDaysAfterReckoning), mc9 + ", Section 5"},
		{"Audited Financial Statements, 31 December year end", fmt.Sprintf("%s for a fiscal year ended 31 December 2025, moved from %s", secfiling.AFSDueFY2025Dec31, secfiling.AFSDueFY2025Dec31Original), mc9 + ", Section 1, as revised by the SEC Notice dated 14 April 2026"},
		{"Audited Financial Statements, other year ends", fmt.Sprintf("%d calendar days from the end of the fiscal year", secfiling.AFSDaysOrdinaryNonDecemberFYE), mc9 + ", Section 2(a)"},
		{"Annual Report on SEC Form 17-A", fmt.Sprintf("%d calendar days after the end of the fiscal year", secfiling.AFSDaysListedOrPublicFYE), mc9 + ", Section 2(b)"},
		{"SEC Form 52-AR, brokers and dealers", fmt.Sprintf("%d calendar days after the close of the fiscal year", secfiling.AFSDaysBrokerDealerNonDecemberFYE), mc9 + ", Section 2(a)"},
		{"Filing on a non-working day", "a submission made on a Saturday, Sunday, holiday or during a work suspension counts as filed on the next working day", mc9 + ", Section 9"},
		{"Late filing, domestic corporation", "assessed as a non-filing once it is more than one year past the deadline", mc6},
		{"Late filing, foreign corporation", fmt.Sprintf("late beyond %d calendar days, assessed as a non-filing beyond %d", secfiling.ForeignLateThresholdDays, secfiling.ForeignNonFilingThresholdDays), mc6},
		{"Per-month component of the fine", fmt.Sprintf("suspended from %s to %s; the basic fine still applies", secfiling.MonthlyPenaltySuspendedFrom, secfiling.MonthlyPenaltySuspendedUntil), mc16},
		{"Delinquent status", fmt.Sprintf("a ground after failing to submit reportorial requirements %d times, consecutively or intermittently, within %d years", secfiling.DelinquencyMissCount, secfiling.DelinquencyWindowYears), rcc + "; guidelines in SEC Memorandum Circular No. 19, Series of 2023"},
		{"Revocation", fmt.Sprintf("a %dth offence after notice of delinquency is a ground for revoking the certificate of registration or licence", secfiling.RevocationOffenceNumber), mc6},
	}
}

type calendarResponse struct {
	FilingYear  int                    `json:"filingYear"`
	AsOf        secfiling.Date         `json:"asOf"`
	Counts      secfiling.Counts       `json:"counts"`
	Obligations []secfiling.Obligation `json:"obligations"`
}

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	asOf, err := dateParam(r, "asOf", secfiling.Today())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	year := intParam(r, "year", asOf.Year)

	out := calendarResponse{FilingYear: year, AsOf: asOf, Obligations: []secfiling.Obligation{}}
	s.store.Read(func(b *secfiling.Book) {
		out.Obligations = secfiling.Calendar(b, year, asOf)
		out.Counts = secfiling.Summarise(out.Obligations)
	})
	writeJSON(w, http.StatusOK, out)
}

type companyResponse struct {
	Company       secfiling.Company          `json:"company"`
	FilingYear    int                        `json:"filingYear"`
	AsOf          secfiling.Date             `json:"asOf"`
	RegNumberNote string                     `json:"registrationNumberNote"`
	Obligations   []secfiling.Obligation     `json:"obligations"`
	History       []secfiling.Obligation     `json:"history"`
	Delinquency   secfiling.DelinquencyWatch `json:"delinquency"`
}

func (s *Server) handleCompanyDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("which company?"))
		return
	}
	asOf, err := dateParam(r, "asOf", secfiling.Today())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	year := intParam(r, "year", asOf.Year)

	out := companyResponse{FilingYear: year, AsOf: asOf}
	found := false
	s.store.Read(func(b *secfiling.Book) {
		c, ok := b.Company(id)
		if !ok {
			return
		}
		found = true
		out.Company = *c
		out.RegNumberNote = secfiling.RegistrationNumberNote(c)
		out.Obligations = secfiling.CompanyObligations(b, c, year, asOf)
		out.History = secfiling.History(b, c, asOf.Year-secfiling.DelinquencyWindowYears, asOf.Year, asOf)
		out.Delinquency = secfiling.WatchDelinquency(out.History, asOf)
	})
	if !found {
		writeError(w, http.StatusNotFound, fmt.Errorf("there is no company with that reference"))
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	asOf, err := dateParam(r, "asOf", secfiling.Today())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	year := intParam(r, "year", asOf.Year)

	var rows []secfiling.Obligation
	s.store.Read(func(b *secfiling.Book) {
		rows = secfiling.Calendar(b, year, asOf)
	})

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=\"sec-compliance-calendar-%d.csv\"", year))

	out := csv.NewWriter(w)
	defer out.Flush()
	_ = out.Write([]string{"Company", "Filing", "Period", "Due", "Last safe filing day",
		"Status", "Days until due", "Filed on", "Proof reference", "How the date was worked out", "Source"})
	for _, o := range rows {
		due, safe, days := "not yet known", "", ""
		if o.Known {
			due, safe = o.Due.String(), o.LastSafeDay.String()
			days = strconv.Itoa(o.DaysUntil)
		}
		_ = out.Write([]string{
			o.CompanyName, o.Requirement.Short(), o.Period, due, safe,
			o.TierLabel, days, o.FiledOn.String(), o.Reference, o.Basis, o.Source,
		})
	}
}

// ───────────────────────────────── writes ────────────────────────────────

func (s *Server) handlePutCompany(w http.ResponseWriter, r *http.Request) {
	var c secfiling.Company
	if !readJSON(w, r, &c) {
		return
	}
	if err := s.store.Write(func(b *secfiling.Book) error { return b.PutCompany(c) }); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"saved": c.ID})
}

func (s *Server) handleDeleteCompany(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if err := s.store.Write(func(b *secfiling.Book) error { return b.DeleteCompany(id) }); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) handlePutMeeting(w http.ResponseWriter, r *http.Request) {
	company := strings.TrimSpace(r.URL.Query().Get("company"))
	var m secfiling.Meeting
	if !readJSON(w, r, &m) {
		return
	}
	if err := s.store.Write(func(b *secfiling.Book) error { return b.PutMeeting(company, m) }); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"saved": m.Date.String()})
}

func (s *Server) handleDeleteMeeting(w http.ResponseWriter, r *http.Request) {
	company := strings.TrimSpace(r.URL.Query().Get("company"))
	year := intParam(r, "year", 0)
	if err := s.store.Write(func(b *secfiling.Book) error { return b.DeleteMeeting(company, year) }); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"deleted": year})
}

func (s *Server) handlePutFiling(w http.ResponseWriter, r *http.Request) {
	var f secfiling.Filing
	if !readJSON(w, r, &f) {
		return
	}
	if err := s.store.Write(func(b *secfiling.Book) error { return b.PutFiling(f) }); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"saved": f.ID})
}

func (s *Server) handleDeleteFiling(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if err := s.store.Write(func(b *secfiling.Book) error { return b.DeleteFiling(id) }); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var settings secfiling.Settings
	if !readJSON(w, r, &settings) {
		return
	}
	if err := s.store.Write(func(b *secfiling.Book) error { return b.PutSettings(settings) }); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"saved": "settings"})
}

// ───────────────────────────────── plumbing ──────────────────────────────

func dateParam(r *http.Request, name string, fallback secfiling.Date) (secfiling.Date, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback, nil
	}
	return secfiling.ParseDate(raw)
}

func intParam(r *http.Request, name string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

func readJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("could not read what was sent"))
		return false
	}
	if err := json.Unmarshal(raw, into); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

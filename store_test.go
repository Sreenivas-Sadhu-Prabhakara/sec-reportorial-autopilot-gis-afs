package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sreenivas-sadhu-prabhakara/sec-reportorial-autopilot-gis-afs/internal/secfiling"
)

func TestOpenStoreStartsEmptyAndDoesNotCreateTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the data file should not exist until something is saved")
	}
	store.Read(func(b *secfiling.Book) {
		if len(b.Companies) != 0 || b.Settings.Ladder != secfiling.DefaultLadder() {
			t.Errorf("a new register should be empty with the default ladder, got %+v", b.Settings.Ladder)
		}
	})
}

func TestStoreWritesAtomicallyAndReloads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "register.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}

	err = store.Write(func(b *secfiling.Book) error {
		return b.PutCompany(secfiling.Company{
			ID: "c1", Name: "Mabuhay Freight Services Inc.", RegistrationNumber: "CS201812345",
			EntityType: secfiling.EntityStock, FilerClass: secfiling.FilerOrdinary,
			FiscalYearEndMonth: 12, FiscalYearEndDay: 31, GISBasis: secfiling.GISFromMeeting,
		})
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the data file was not written: %v", err)
	}
	if !strings.Contains(string(raw), "Mabuhay Freight Services Inc.") {
		t.Errorf("the company is not in the file: %s", raw)
	}
	var check secfiling.Book
	if err := json.Unmarshal(raw, &check); err != nil {
		t.Fatalf("the file is not valid JSON: %v", err)
	}

	// No temporary files should be left behind.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("a temporary file was left behind: %s", e.Name())
		}
	}

	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	reopened.Read(func(b *secfiling.Book) {
		if len(b.Companies) != 1 || b.Companies[0].Name != "Mabuhay Freight Services Inc." {
			t.Errorf("the register did not come back: %+v", b.Companies)
		}
	})
}

func TestStoreDoesNotSaveWhenTheChangeIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	err = store.Write(func(b *secfiling.Book) error {
		return b.PutCompany(secfiling.Company{ID: "c1"}) // no name
	})
	if err == nil {
		t.Fatal("a company with no name should be rejected")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a rejected change should not write the file")
	}
}

func TestStoreRejectsAFileThatIsNotJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	if err := os.WriteFile(path, []byte("not json at all"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := OpenStore(path); err == nil {
		t.Fatal("a data file that is not JSON should be reported, not silently replaced")
	}
}

func TestStoreNormalisesAnOlderFileOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	older := `{"companies":[{"id":"c1","name":"Old Format Inc"}]}`
	if err := os.WriteFile(path, []byte(older), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	store.Read(func(b *secfiling.Book) {
		c := b.Companies[0]
		if c.FiscalYearEndMonth != 12 || c.FiscalYearEndDay != 31 {
			t.Errorf("fiscal year end = %d-%d, want 12-31", c.FiscalYearEndMonth, c.FiscalYearEndDay)
		}
		if c.EntityType != secfiling.EntityStock || c.GISBasis != secfiling.GISFromMeeting {
			t.Errorf("defaults not filled in: %+v", c)
		}
	})
}

func TestStoreKeepsTheFilePrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := store.Write(func(b *secfiling.Book) error {
		return b.PutMeeting("nobody", secfiling.Meeting{Year: 2026,
			Date: secfiling.NewDate(2026, time.April, 20)})
	}); err == nil {
		t.Fatal("a meeting for a company that does not exist should be rejected")
	}
	if err := store.Write(func(b *secfiling.Book) error { return nil }); err != nil {
		t.Fatalf("Write: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("the data file holds a client's compliance record; mode = %v, want 0600", mode)
	}
}

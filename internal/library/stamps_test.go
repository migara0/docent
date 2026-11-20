package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStamps(t *testing.T) {
	at := func(s string) time.Time {
		v, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	born, mod := at("2026-11-09 10:27"), at("2026-11-17 21:59")
	cases := []struct {
		name                      string
		pg                        Page
		created, createdAt, updAt string
	}{
		{"file times agree with the front matter", Page{Meta: Meta{Date: "2026-11-09", Updated: "2026-11-17"}, Born: born, Modified: mod},
			"2026-11-09", born.Format(time.RFC3339), mod.Format(time.RFC3339)},
		// a copied or restored file: its clock says another day, so only the day of the front matter is known
		{"file times disagree", Page{Meta: Meta{Date: "2026-11-08", Updated: "2026-11-16"}, Born: born, Modified: mod}, "2026-11-08", "", ""},
		{"no updated: the file's modified time", Page{Meta: Meta{Date: "2026-11-09"}, Born: born, Modified: mod}, "2026-11-09", born.Format(time.RFC3339), mod.Format(time.RFC3339)},
		{"no date: the file's creation", Page{Born: born, Modified: mod}, "2026-11-09", born.Format(time.RFC3339), mod.Format(time.RFC3339)},
		{"no date and no creation time", Page{Modified: mod}, "", "", mod.Format(time.RFC3339)},
	}
	for _, c := range cases {
		if got := c.pg.Created(); got != c.created {
			t.Errorf("%s: Created() = %q, want %q", c.name, got, c.created)
		}
		if got := c.pg.CreatedAt(); got != c.createdAt {
			t.Errorf("%s: CreatedAt() = %q, want %q", c.name, got, c.createdAt)
		}
		if got := c.pg.UpdatedAt(); got != c.updAt {
			t.Errorf("%s: UpdatedAt() = %q, want %q", c.name, got, c.updAt)
		}
	}
}

func TestBirthTime(t *testing.T) {
	f := filepath.Join(t.TempDir(), "a.md")
	before := time.Now().Add(-time.Second)
	if err := os.WriteFile(f, []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := birthTime(f)
	if b.IsZero() {
		t.Skip("this filesystem does not record creation times")
	}
	if b.Before(before) || b.After(time.Now().Add(time.Second)) {
		t.Fatalf("birth time %v is not around now", b)
	}
	if !birthTime(f + ".missing").IsZero() {
		t.Fatal("a missing file has no birth time")
	}
}

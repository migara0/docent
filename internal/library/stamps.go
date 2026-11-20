package library

import "time"

const day = "2006-01-02"

// The front matter holds dates only (date:, updated:). The file's own clock gives the time of day, but it is trusted only when it
// falls on the same day as the front matter says: a copied or restored file has new file times, and then the date alone is shown.

// Created is the day the page was created: its front matter date, or for a page without one, the day the file was created.
func (pg *Page) Created() string {
	if pg.Meta.Date != "" {
		return pg.Meta.Date
	}
	if !pg.Born.IsZero() {
		return pg.Born.Local().Format(day)
	}
	return ""
}

// CreatedAt is the moment the page was created (RFC 3339), or "" when only the day is known.
func (pg *Page) CreatedAt() string {
	return stamp(pg.Created(), pg.Born)
}

// UpdatedAt is the moment the page was last updated (RFC 3339), or "" when only the day is known.
func (pg *Page) UpdatedAt() string {
	if pg.Meta.Updated == "" {
		return stamp(pg.Modified.Local().Format(day), pg.Modified)
	}
	return stamp(pg.Meta.Updated, pg.Modified)
}

func stamp(date string, t time.Time) string {
	if date == "" || t.IsZero() || t.Local().Format(day) != date {
		return ""
	}
	return t.Format(time.RFC3339)
}

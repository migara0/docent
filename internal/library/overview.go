package library

import (
	"path"
	"sort"
	"strings"
)

// Row is one page as shown in an overview table or a "recently updated" list.
type Row struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	ID      string `json:"id,omitempty"`
	Created string `json:"created,omitempty"`
	Updated string `json:"updated"`
	// the same moments with their time of day (RFC 3339), when the file's own clock agrees with the dates above
	CreatedAt string   `json:"createdAt,omitempty"`
	UpdatedAt string   `json:"updatedAt,omitempty"`
	Status    string   `json:"status,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Folder    string   `json:"folder"`
}

func (s *Source) row(pg *Page) Row {
	updated := pg.Meta.Updated
	if updated == "" {
		updated = pg.Modified.Format("2006-01-02")
	}
	r := Row{Path: pg.Path, Title: pg.Title, URL: s.PageURL(pg.Path), ID: pg.Meta.ID, Created: pg.Meta.Date, Updated: updated, Status: pg.Meta.Status,
		Tags: pg.Meta.Tags, Folder: path.Dir(pg.Path), UpdatedAt: pg.UpdatedAt()}
	if r.Created != "" {
		r.CreatedAt = pg.CreatedAt()
	}
	return r
}

// CreatedRows lists every page with the day it was created (from its front matter, or the file for a page without a date), for the
// search filter "created on". Pages whose creation day is unknown are left out.
func (s *Source) CreatedRows() []Row {
	var out []Row
	for _, pg := range s.snap.Load().pages {
		r := s.row(pg)
		if r.Created, r.CreatedAt = pg.Created(), pg.CreatedAt(); r.Created != "" {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return naturalLess(out[i].Path, out[j].Path) })
	return out
}

// Rows returns the pages under a folder ("" for all), oldest path first. With notesOnly, pages without a date in their front matter are left out.
func (s *Source) Rows(folder string, notesOnly bool) []Row {
	prefix := ""
	if folder != "" {
		prefix = strings.TrimSuffix(folder, "/") + "/"
	}
	var out []Row
	for p, pg := range s.snap.Load().pages {
		if !strings.HasPrefix(p, prefix) || (notesOnly && pg.Meta.Date == "") {
			continue
		}
		out = append(out, s.row(pg))
	}
	sort.Slice(out, func(i, j int) bool { return naturalLess(out[i].Path, out[j].Path) })
	return out
}

// Recent returns the most recently updated pages, newest first.
func (s *Source) Recent(limit int, notesOnly bool) []Row {
	rows := s.Rows("", notesOnly)
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Updated != rows[j].Updated {
			return rows[i].Updated > rows[j].Updated
		}
		return rows[i].Created > rows[j].Created
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

// TagCount is a tag with the pages that carry it.
type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Pages []Row  `json:"pages"`
}

// Tags groups pages by the tags in their front matter, most used first.
func (s *Source) Tags() []TagCount {
	by := map[string]*TagCount{}
	for _, pg := range s.snap.Load().pages {
		for _, tag := range pg.Meta.Tags {
			tc := by[tag]
			if tc == nil {
				tc = &TagCount{Name: tag}
				by[tag] = tc
			}
			tc.Pages = append(tc.Pages, s.row(pg))
		}
	}
	out := make([]TagCount, 0, len(by))
	for _, tc := range by {
		tc.Count = len(tc.Pages)
		sort.Slice(tc.Pages, func(i, j int) bool { return tc.Pages[i].Updated > tc.Pages[j].Updated })
		out = append(out, *tc)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

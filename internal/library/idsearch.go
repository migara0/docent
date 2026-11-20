package library

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// IDMatch is a note found by (part of) its ID.
type IDMatch struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Source  string `json:"source"`
	Library string `json:"library"`
	Exact   bool   `json:"exact"`
}

var (
	idShape    = regexp.MustCompile(`^(?:([A-Za-z]{2,5})[- ]?)?(\d{1,9})?$`)
	idOfNote   = regexp.MustCompile(`^([A-Z]{2,5})-(\d{4,})$`)
	maxIDMatch = 8
)

// FindIDs finds notes by what a person types to find an ID: `DOC-0015`, `doc-15`, `doc 15`, `15`, a part such as `DOC-001`, or just a prefix (`doc`, which lists
// the newest notes of that prefix). A bare number matches that number under every prefix, the library named by prefer first. At most limit matches are returned
// (0 means the default of 8), exact matches first.
func (m *Manager) FindIDs(q, prefer string, limit int) []IDMatch {
	q = strings.TrimSpace(q)
	sh := idShape.FindStringSubmatch(q)
	if q == "" || sh == nil {
		return []IDMatch{}
	}
	prefix, digits := strings.ToUpper(sh[1]), sh[2]
	if limit <= 0 {
		limit = maxIDMatch
	}
	num, _ := strconv.Atoi(digits)

	m.mu.RLock()
	defer m.mu.RUnlock()
	var prefixes []string
	for _, s := range m.sources {
		if s.IDPrefix != "" && !slices.Contains(prefixes, s.IDPrefix) {
			prefixes = append(prefixes, s.IDPrefix)
		}
	}
	if prefix != "" && !slices.Contains(prefixes, prefix) && digits == "" {
		return []IDMatch{} // letters that are not a known prefix are ordinary search words
	}

	type hit struct {
		IDMatch
		n      int
		prefer bool
	}
	var hits []hit
	for _, s := range m.sources {
		if s.IDPrefix == "" {
			continue
		}
		for id, path := range s.IDs() {
			p := idOfNote.FindStringSubmatch(id)
			if p == nil || (prefix != "" && p[1] != prefix) {
				continue
			}
			n, _ := strconv.Atoi(p[2])
			exact := digits != "" && n == num
			switch {
			case digits == "": // a prefix alone: its newest notes
			case exact:
			case prefix != "" && strings.HasPrefix(id, fmt.Sprintf("%s-%s", prefix, digits)): // a part of an ID
			default:
				continue
			}
			pg, ok := s.Page(path)
			if !ok {
				continue
			}
			hits = append(hits, hit{IDMatch{ID: id, Title: pg.Title, URL: s.PageURL(path), Source: s.ID, Library: s.Name, Exact: exact}, n, s.ID == prefer})
		}
	}
	slices.SortFunc(hits, func(a, b hit) int {
		switch {
		case a.Exact != b.Exact:
			if a.Exact {
				return -1
			}
			return 1
		case a.prefer != b.prefer:
			if a.prefer {
				return -1
			}
			return 1
		case digits == "" && a.n != b.n: // newest first
			return b.n - a.n
		}
		return strings.Compare(a.ID, b.ID)
	})
	out := make([]IDMatch, 0, min(limit, len(hits)))
	for _, h := range hits {
		if len(out) == limit {
			break
		}
		out = append(out, h.IDMatch)
	}
	return out
}

package library

import (
	"fmt"
	"slices"
	"sync"
)

// interner shares one copy of short repeated strings (dates, statuses, kinds, tags), so ten thousand pages do not hold ten thousand copies.
type interner struct {
	mu sync.Mutex
	m  map[string]string
}

func (in *interner) get(s string) string {
	if len(s) > 64 || s == "" {
		return s
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if v, ok := in.m[s]; ok {
		return v
	}
	if in.m == nil || len(in.m) > 50_000 {
		in.m = make(map[string]string, 1024)
	}
	in.m[s] = s
	return s
}

// slimMeta keeps the well-known front matter fields. Anything else is dropped from the index (it is still there when the page is opened).
func slimMeta(m map[string]any, in *interner) Meta {
	if len(m) == 0 {
		return Meta{}
	}
	str := func(k string) string {
		switch v := m[k].(type) {
		case string:
			if len(v) > 300 {
				return ""
			}
			return in.get(v)
		case int, int64, float64, bool:
			return in.get(fmt.Sprint(v))
		}
		return ""
	}
	_, hasTitle := m["title"]
	_, hasTags := m["tags"]
	out := Meta{HasTitle: hasTitle, HasTags: hasTags, Technology: str("technology"), ID: str("id"), Date: str("date"), Updated: str("updated"), Status: str("status"), Type: str("type"), Description: str("description"),
		Author: str("author"), Release: str("release"), Versions: str("versions"), Verified: str("verified")}
	switch v := m["tags"].(type) {
	case []any:
		for _, x := range v {
			s, ok := x.(string)
			if !ok {
				continue
			}
			if slices.Contains(out.Tags, s) {
				out.DupTags++ // a tag listed twice: shown once (the UI keys chips by name), flagged by the profile
				continue
			}
			if len(out.Tags) < 12 {
				out.Tags = append(out.Tags, in.get(s))
			}
		}
	case string:
		out.Tags = []string{in.get(v)}
	}
	return out
}

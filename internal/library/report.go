package library

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/migara0/docent/internal/config"
	"github.com/migara0/docent/internal/render"
)

// Preview scans a folder (or file) once, without registering or watching it, for `docent add` and `docent check`.
func Preview(path string, idPrefix ...string) (*Source, error) {
	return PreviewWith(path, nil, config.Library{}, idPrefix...)
}

// PreviewWith is Preview with profile hooks and the library's settings, so `docent check` applies the profile's rules.
func PreviewWith(path string, hooks Hooks, lib config.Library, idPrefix ...string) (*Source, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	root, file := abs, ""
	if isMarkdown(abs) {
		root, file = filepath.Dir(abs), filepath.Base(abs)
	}
	s := newSource("preview", filepath.Base(abs), KindFolder, "plain", root, file, render.New(), nil)
	if len(idPrefix) > 0 {
		s.IDPrefix = idPrefix[0]
	}
	s.hooks, s.Lib = hooks, lib
	if _, err := s.Rescan(); err != nil {
		return nil, err
	}
	_ = s.WaitReady(context.Background())
	return s, nil
}

// Problem is something worth fixing in a page.
type Problem struct {
	Page    string `json:"page"`
	Kind    string `json:"kind"` // broken-link or warning
	Message string `json:"message"`
}

// Problems lists broken links and parse warnings, sorted by page.
func (s *Source) Problems() []Problem {
	snap := s.snap.Load()
	var out []Problem
	for p, pg := range snap.pages {
		for _, rel := range pg.rels {
			if _, ok := resolveTarget(snap.pages, p, rel); !ok && (isMarkdown(rel) || strings.HasSuffix(rel, "/")) {
				out = append(out, Problem{Page: p, Kind: "broken-link", Message: fmt.Sprintf("%q does not point to a page", rel)})
			}
		}
		for _, w := range pg.Warnings {
			out = append(out, Problem{Page: p, Kind: "warning", Message: w})
		}
		if s.hooks != nil {
			for _, m := range s.hooks.Validate(s.Lib, p, pg) {
				out = append(out, Problem{Page: p, Kind: "front-matter", Message: m})
			}
		}
		for _, id := range pg.ids {
			if _, ok := snap.ids[id]; !ok {
				out = append(out, Problem{Page: p, Kind: "unknown-id", Message: fmt.Sprintf("note ID %s does not match any note", id)})
			}
		}
	}
	for id, paths := range snap.dupIDs {
		for _, p := range paths {
			out = append(out, Problem{Page: p, Kind: "duplicate-id", Message: fmt.Sprintf("note ID %s is used by %d notes", id, len(paths))})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Page != out[j].Page {
			return naturalLess(out[i].Page, out[j].Page)
		}
		return out[i].Message < out[j].Message
	})
	return out
}

// Package devnotes is the "developer notes" profile: conventions for a team's engineering notes, made available to any folder of Markdown.
//
//	docs/<service>/<type>/<yyyy-mm-dd>-<title>.md        a note for one service (or "platform")
//	docs/features/<feature>/<type>/...                   notes of one feature that spans services
//	docs/technologies/<technology>/guide/...             the living guide for one technology
//
// Every note has front matter with a stable ID (DOC-0007), and the profile adds note scaffolding from templates, an ID registry that never reuses a number,
// validation, linking of IDs in text, overview and tag views, commit tracking for release cherry-picking, lint, stale-module detection and bundles.
// Nothing here runs unless you ask: the profile writes to your notes only from explicit commands (docent new, docent commits, docent id backfill).
package devnotes

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/migara0/docent/internal/config"
)

// Settings are a library's profile settings with defaults filled in.
type Settings struct {
	Prefix    string // note ID prefix, e.g. DOC
	Workspace string // the folder holding the service repos (git repos next to each other)
	Templates string // a folder of note templates ("" uses the built-in ones)
	Root      string // the library folder (docs/)
	Site      string // the folder above docs/ when the library follows the <x>_developer_notes layout, else ""
}

// GroupFolders are the folders whose sub-folders group notes: docs/<group>/<name>/<type>/.
var GroupFolders = []string{"features", "technologies"}

// Technologies and Features are the group folders with a fixed meaning.
const (
	Technologies = "technologies"
	Features     = "features"
	Platform     = "platform"
)

var siteDir = regexp.MustCompile(`^([a-zA-Z]{2,5})_developer_notes$`)

// Resolve fills in what the settings leave empty, from the folder layout the notes sites use:
// <workspace>/<x>_developer_notes/docs (notes) and <workspace>/<x>_developer_notes/templates.
func Resolve(lib config.Library) Settings {
	s := Settings{Prefix: lib.Settings.Prefix, Workspace: lib.Settings.Workspace, Templates: lib.Settings.Templates, Root: lib.Path}
	parent := filepath.Dir(lib.Path)
	if filepath.Base(lib.Path) == "docs" && siteDir.MatchString(filepath.Base(parent)) {
		s.Site = parent
	}
	if s.Prefix == "" {
		switch {
		case s.Site != "":
			s.Prefix = strings.ToUpper(siteDir.FindStringSubmatch(filepath.Base(s.Site))[1])
		default:
			s.Prefix = prefixFromName(lib.Name)
		}
	}
	s.Prefix = strings.ToUpper(s.Prefix)
	if s.Workspace == "" {
		if s.Site != "" {
			s.Workspace = filepath.Dir(s.Site)
		} else {
			s.Workspace = parent
		}
	}
	if s.Templates == "" && s.Site != "" {
		if info, err := os.Stat(filepath.Join(s.Site, "templates")); err == nil && info.IsDir() {
			s.Templates = filepath.Join(s.Site, "templates")
		}
	}
	return s
}

var firstWord = regexp.MustCompile(`[A-Za-z]{2,5}`)

func prefixFromName(name string) string {
	if m := firstWord.FindString(name); m != "" {
		return strings.ToUpper(m)
	}
	return "DOC"
}

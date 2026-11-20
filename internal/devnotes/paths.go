package devnotes

import (
	"path"
	"slices"
	"strings"
)

// Types are the note types the profile knows. Each has a template.
var Types = []string{"implementation", "bugfix", "upgrade", "api", "qa", "runbook", "adr", "commits", "guide", "module", "plan"}

// Place says where a note lives: docs/<service>/<type>/<file>, docs/features/<feature>/<type>/<file> or docs/technologies/<technology>/<type>/<file>.
type Place struct {
	Group   string // "", "features" or "technologies"
	Service string // the service folder, or the feature or technology name
	Type    string
	Depth   int // path segments, including the file name
}

// Locate reads a page path (relative to the library root).
func Locate(rel string) Place {
	segs := strings.Split(rel, "/")
	p := Place{Depth: len(segs)}
	if len(segs) >= 4 && slices.Contains(GroupFolders, segs[0]) {
		p.Group, p.Service, p.Type = segs[0], segs[1], segs[2]
		return p
	}
	if len(segs) >= 3 {
		p.Service, p.Type = segs[0], segs[1]
	}
	return p
}

// Exact reports whether the path follows the strict layout (docs/<service>/<type>/<file> or docs/<group>/<name>/<type>/<file>). Deeper paths, such as
// docs/<service>/<project>/<folder>/<file>, are allowed: the folder under the service is then not necessarily a note type.
func (p Place) Exact() bool {
	if p.Group != "" {
		return p.Depth == 4
	}
	return p.Depth == 3
}

// IsNotePlace reports whether a path is where notes live (as opposed to index.md or tags.md at the top).
func (p Place) IsNotePlace() bool { return p.Type != "" }

// Label is the "service" shown for a note: "billing-api", "features/badge" or "technologies/kafkajs".
func (p Place) Label() string {
	if p.Group != "" {
		return p.Group + "/" + p.Service
	}
	return p.Service
}

// Tag is the service-like tag a new note gets (the feature or technology name for grouped notes).
func (p Place) Tag() string { return p.Service }

// FileName is "2026-10-14-some-title.md" for a note path.
func FileName(rel string) string { return path.Base(rel) }

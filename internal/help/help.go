// Package help holds the user guide that ships inside the program, so the Help button works offline and always matches the version in use.
package help

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed guide.md
var guide []byte

// FileName is the page's file name, which makes its address /help/guide.
const FileName = "guide.md"

// Text returns the guide's Markdown.
func Text() []byte { return guide }

// Write puts the guide into dir (created if needed) so it can be opened like any folder of Markdown. It is rewritten at every start, so it can never be out of date.
func Write(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, FileName), guide, 0o644)
}

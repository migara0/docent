package help

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestGuideShipsInsideTheProgram(t *testing.T) {
	text := string(Text())
	if !strings.HasPrefix(text, "# Docent guide") {
		t.Errorf("the guide must start with its title, got %.40q", text)
	}
	for _, want := range []string{"## Start", "## Reading", "Drop a file", "Open a page on your phone", "docent serve --lan"} {
		if !strings.Contains(text, want) {
			t.Errorf("the guide should cover %q", want)
		}
	}
	// it is opened on its own, away from the source tree: a link to another .md file would be broken there
	if m := regexp.MustCompile(`\]\([^)#]*\.md[^)]*\)`).FindString(text); m != "" {
		t.Errorf("the guide must not link to other Markdown files (%s): it is shown on its own", m)
	}
}

func TestWriteMakesAReadableFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "help")
	for i := 0; i < 2; i++ { // twice: it is rewritten at every start
		if err := Write(dir); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil || string(got) != string(Text()) {
		t.Errorf("written guide differs: %v", err)
	}
}

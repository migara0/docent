//go:build windows

package server

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// On Windows the picker names a drive "C:" (not "\"), lists the drives to switch between, and shows "\" between folders.
func TestFolderPickerOnWindows(t *testing.T) {
	e := newEnv(t, false)
	dir := t.TempDir()
	vol := filepath.VolumeName(dir) // "C:" (or the drive the temp folder is on)
	code, body := e.do(t, "GET", "/_api/fs?dir="+url.QueryEscape(dir), "", nil)
	var out struct {
		Crumbs []struct{ Name, Path string }
		Sep    string
		Roots  []string
	}
	_ = json.Unmarshal(body, &out)
	if code != 200 || len(out.Crumbs) == 0 {
		t.Fatalf("listing %s: %d %s", dir, code, body)
	}
	if out.Sep != `\` {
		t.Errorf("separator = %q", out.Sep)
	}
	if !slices.ContainsFunc(out.Roots, func(r string) bool { return strings.EqualFold(r, vol+`\`) }) {
		t.Errorf("the drive %s is not among the roots %q", vol, out.Roots)
	}
	if code, body = e.do(t, "GET", "/_api/fs?dir="+url.QueryEscape(vol+`\`), "", nil); code != 200 {
		t.Fatalf("the drive root: %d %s", code, body)
	}
	_ = json.Unmarshal(body, &out)
	if len(out.Crumbs) != 1 || !strings.EqualFold(out.Crumbs[0].Name, vol) {
		t.Errorf("the drive root is one crumb named %s: %+v", vol, out.Crumbs)
	}
}

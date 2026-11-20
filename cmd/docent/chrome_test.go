package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestChromePaths(t *testing.T) {
	env := map[string]string{"ProgramFiles": `C:\Program Files`, "ProgramFiles(x86)": `C:\Program Files (x86)`, "LocalAppData": `C:\Users\a\AppData\Local`}
	win := chromePaths("windows", func(k string) string { return env[k] })
	if len(win) != 5 {
		t.Fatalf("Chrome in three places and Edge in two: %q", win)
	}
	if !strings.HasSuffix(win[0], filepath.Join("Google", "Chrome", "Application", "chrome.exe")) || !strings.HasSuffix(win[4], "msedge.exe") {
		t.Errorf("Chrome first, Edge last: %q", win)
	}
	if got := chromePaths("windows", func(string) string { return "" }); len(got) != 0 {
		t.Errorf("no environment, no guesses: %q", got)
	}
	if mac := chromePaths("darwin", nil); len(mac) != 3 || !strings.Contains(mac[0], "Google Chrome.app") {
		t.Errorf("macOS app bundles: %q", mac)
	}
	if chromePaths("linux", nil) != nil {
		t.Error("on Linux the PATH is enough")
	}
}

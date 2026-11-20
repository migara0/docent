//go:build windows

package main

import (
	"testing"

	"github.com/spf13/cobra"
)

// A double-click must start the app: cobra's "This is a command line tool" screen is turned off.
func TestDoubleClickIsAllowed(t *testing.T) {
	if cobra.MousetrapHelpText != "" {
		t.Fatalf("cobra will refuse a double-click with: %q", cobra.MousetrapHelpText)
	}
	if startedByExplorer {
		t.Fatal("tests are not started from Explorer")
	}
	if got := stopHint(); got != "Press Ctrl+C to stop." {
		t.Errorf("from a terminal the hint is Ctrl+C: %q", got)
	}
}

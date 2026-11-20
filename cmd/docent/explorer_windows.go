//go:build windows

package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/inconshreveable/mousetrap"
	"github.com/spf13/cobra"
)

// On Windows most people start Docent by double-clicking docent.exe. Cobra refuses that by default: it prints "This is a command line
// tool. You need to open cmd.exe and run it from there." and exits after 5 seconds. Docent with no arguments is the app, so let it run.
func init() {
	cobra.MousetrapHelpText = ""
}

// startedByExplorer is true when docent.exe was double-clicked (its console window closes as soon as it exits).
var startedByExplorer = mousetrap.StartedByExplorer()

// pauseOnError keeps a double-clicked window open after an error, so the message can be read before the window closes.
func pauseOnError() {
	if !startedByExplorer {
		return
	}
	fmt.Print("\nPress Enter to close this window.")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

// stopHint tells how to stop the server.
func stopHint() string {
	if startedByExplorer {
		return "Keep this window open while you read; close it to stop Docent."
	}
	return "Press Ctrl+C to stop."
}

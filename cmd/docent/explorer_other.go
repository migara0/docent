//go:build !windows

package main

// pauseOnError does nothing outside Windows: a terminal stays open after the program exits.
func pauseOnError() {}

// stopHint tells how to stop the server.
func stopHint() string { return "Press Ctrl+C to stop." }

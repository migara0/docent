// Command docent opens folders and files of Markdown as a fast, local documentation reader.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// version is set at build time: go build -ldflags "-X main.version=1.2.3".
var version = "0.0.0-dev"

func init() {
	// `go install github.com/migara0/docent/cmd/docent@v1.2.3` sets no -ldflags, but Go records the module version in the binary
	if bi, ok := debug.ReadBuildInfo(); ok && version == "0.0.0-dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		version = strings.TrimPrefix(bi.Main.Version, "v")
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := newRoot().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRoot() *cobra.Command {
	var sv serveOptions
	root := &cobra.Command{
		Use:           "docent",
		Short:         "Read folders of Markdown as a clean, searchable documentation site",
		Long:          "Docent opens any folder or file of Markdown and shows it in your browser. Folders you add are remembered for the next run.\nRun it with no arguments to start the app.",
		Version:       fmt.Sprintf("%s (%s %s/%s)", version, runtime.Version(), runtime.GOOS, runtime.GOARCH),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE:          func(cmd *cobra.Command, args []string) error { return runServe(sv, nil) },
	}
	sv.bind(root.Flags())
	root.AddCommand(newServe(), newAdd(), newList(), newRemove(), newOpen(), newCheck(), newRender(), newNew(), newID(), newCommits(), newPlan(), newLint(), newStale(), newBundle(), &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		Run:   func(cmd *cobra.Command, args []string) { fmt.Printf("docent %s\n", root.Version) },
	})
	return root
}

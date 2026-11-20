package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/migara0/docent/internal/config"
	"github.com/migara0/docent/internal/devnotes"
	"github.com/migara0/docent/internal/library"
	"github.com/migara0/docent/internal/render"
)

func absPath(p string) (string, error) {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, p[2:])
	}
	return filepath.Abs(p)
}

func openStore() (*config.Store, config.Paths, error) {
	paths, err := config.ResolvePaths()
	if err != nil {
		return nil, paths, err
	}
	return config.NewStore(paths), paths, nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// ───────── add ─────────

func newAdd() *cobra.Command {
	var name, profile, prefix, workspace, templates string
	c := &cobra.Command{
		Use:   "add <folder>",
		Short: "Remember a folder of Markdown as a library",
		Long:  "Registers a folder so it is there on every run. The folder is only read, never changed. Use `docent open` to look at something once without remembering it.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			abs, err := absPath(args[0])
			if err != nil {
				return err
			}
			if profile != "" && profile != "plain" && profile != "devnotes" {
				return fmt.Errorf("unknown profile %q (use plain or devnotes)", profile)
			}
			st, err := os.Stat(abs)
			if err != nil {
				return err
			}
			if !st.IsDir() {
				return fmt.Errorf("%s is a file. Add the folder that contains it, or use `docent open %s`", args[0], args[0])
			}
			store, paths, err := openStore()
			if err != nil {
				return err
			}
			lib, err := store.AddLibrary(abs, name, profile, config.Settings{Prefix: strings.ToUpper(prefix), Workspace: workspace, Templates: templates})
			if err != nil {
				return err
			}
			var src *library.Source
			if lib.Profile == "devnotes" {
				src, err = library.PreviewWith(abs, devnotes.NewHooks(), lib, devnotes.Resolve(lib).Prefix)
			} else {
				src, err = library.Preview(abs)
			}
			if err != nil {
				return err
			}
			info := src.Info()
			fmt.Printf("Added %q (%s, %s in %s)\n", lib.Name, lib.ID, plural(info.Pages, "page", "pages"), plural(info.Folders, "folder", "folders"))
			if lib.Profile == "devnotes" {
				s := devnotes.Resolve(lib)
				fmt.Printf("Developer notes: IDs %s-NNNN, repos in %s, templates %s\n", s.Prefix, s.Workspace, firstNonEmpty(s.Templates, "built in"))
			}
			if _, running := findRunning(paths); running {
				fmt.Println("Docent is running: it will appear in the app within a second.")
			} else {
				fmt.Println("Run `docent` to read it.")
			}
			if info.Pages == 0 {
				fmt.Println("Note: no Markdown files were found in this folder (yet).")
			}
			return nil
		},
	}
	c.Flags().StringVar(&name, "name", "", "name shown in the app (default: the folder name)")
	c.Flags().StringVar(&profile, "profile", "", "plain (default) or devnotes")
	c.Flags().StringVar(&prefix, "prefix", "", "devnotes: the note ID prefix (default: from the folder name, e.g. doc_developer_notes gives DOC)")
	c.Flags().StringVar(&workspace, "workspace", "", "devnotes: the folder holding the service repos (default: above the notes site)")
	c.Flags().StringVar(&templates, "templates", "", "devnotes: a folder of note templates replacing the built-in ones")
	return c
}

// ───────── list ─────────

func newList() *cobra.Command {
	var recent bool
	c := &cobra.Command{
		Use:   "list",
		Short: "Show the libraries you added",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore()
			if err != nil {
				return err
			}
			cfg, err := store.Load()
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
			if recent {
				fmt.Fprintln(tw, "OPENED\tKIND\tPATH")
				for _, r := range cfg.Recents {
					fmt.Fprintf(tw, "%s\t%s\t%s\n", r.OpenedAt.Local().Format("2006-01-02 15:04"), r.Kind, r.Path)
				}
				if len(cfg.Recents) == 0 {
					fmt.Println("Nothing opened yet.")
					return nil
				}
				return tw.Flush()
			}
			if len(cfg.Libraries) == 0 {
				fmt.Println("No libraries yet. Add one with: docent add <folder>")
				return nil
			}
			fmt.Fprintln(tw, "ID\tNAME\tPROFILE\tPATH")
			for _, l := range cfg.Libraries {
				path := l.Path
				if _, err := os.Stat(l.Path); err != nil {
					path += "  (missing)"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", l.ID, l.Name, l.Profile, path)
			}
			return tw.Flush()
		},
	}
	c.Flags().BoolVar(&recent, "recent", false, "show recently opened folders and files instead")
	return c
}

// ───────── remove ─────────

func newRemove() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <id|name|folder>",
		Aliases: []string{"rm"},
		Short:   "Forget a library (the folder is not touched)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore()
			if err != nil {
				return err
			}
			key := args[0]
			if abs, err := absPath(key); err == nil {
				if cfg, _ := store.Load(); cfg.Libraries != nil {
					for _, l := range cfg.Libraries {
						if l.Path == abs {
							key = l.ID
						}
					}
				}
			}
			l, err := store.RemoveLibrary(key)
			if err != nil {
				return err
			}
			fmt.Printf("Removed %q. Your files at %s were not touched.\n", l.Name, l.Path)
			return nil
		},
	}
}

// ───────── open ─────────

func newOpen() *cobra.Command {
	var noOpen bool
	c := &cobra.Command{
		Use:   "open <folder-or-file>...",
		Short: "Open a folder or Markdown file once, without remembering it",
		Long:  "Opens any folder or .md file in the app for this run only. If Docent is already running it opens there; otherwise it starts.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, paths, err := openStore()
			if err != nil {
				return err
			}
			if running, ok := findRunning(paths); ok {
				var first string
				for _, a := range args {
					u, err := postOpen(running.URL, a)
					if err != nil {
						return err
					}
					if first == "" {
						first = u
					}
				}
				fmt.Println("Opened in the running app:", first)
				if !noOpen {
					openBrowser(first)
				}
				return nil
			}
			return runServe(serveOptions{noOpen: noOpen}, args)
		},
	}
	c.Flags().BoolVar(&noOpen, "no-open", false, "do not open the browser")
	return c
}

// ───────── check ─────────

func newCheck() *cobra.Command {
	var asJSON, diagrams bool
	var only []string
	c := &cobra.Command{
		Use:   "check <library-or-folder-or-file>",
		Short: "Scan Markdown and report broken links and problems (exit 1 if any)",
		Long:  "Reports broken links and Markdown problems. For a developer-notes library (give its name, or its folder) it also applies the profile's rules: front matter, IDs and duplicates.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			src, err := previewTarget(args[0])
			if err != nil {
				return err
			}
			info, problems := src.Info(), src.Problems()
			if asJSON {
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"pages": info.Pages, "folders": info.Folders, "problems": problems})
			} else {
				for _, p := range problems {
					fmt.Printf("%s: %s: %s\n", p.Page, p.Kind, p.Message)
				}
				fmt.Printf("%s in %s checked in %s, %s\n", plural(info.Pages, "page", "pages"), plural(info.Folders, "folder", "folders"), time.Since(start).Round(time.Millisecond), plural(len(problems), "problem", "problems"))
			}
			failed := len(problems) > 0
			if diagrams {
				lib, err := registeredLib(args[0])
				if err != nil {
					return err
				}
				wanted := map[string]bool{}
				for _, n := range only {
					abs, _ := filepath.Abs(n)
					if rel, err := filepath.Rel(lib.Path, abs); err == nil && !strings.HasPrefix(rel, "..") {
						wanted[filepath.ToSlash(rel)] = true
					} else {
						wanted[filepath.ToSlash(n)] = true
					}
				}
				total, broken, err := checkDiagrams(lib.ID, wanted)
				if err != nil {
					return err
				}
				fmt.Printf("%s checked, %s\n", plural(total, "diagram", "diagrams"), plural(broken, "problem", "problems"))
				failed = failed || broken > 0
			}
			if failed {
				os.Exit(1)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "machine-readable output")
	c.Flags().StringSliceVar(&only, "note", nil, "with --diagrams: only the diagrams of these notes (paths, or paths inside the library)")
	c.Flags().BoolVar(&diagrams, "diagrams", false, "also draw every Mermaid diagram in headless Chrome and report the ones that fail (registered libraries only; needs Chrome)")
	return c
}

// registeredLib finds the registered library a name, ID or folder refers to.
func registeredLib(arg string) (config.Library, error) {
	store, _, err := openStore()
	if err != nil {
		return config.Library{}, err
	}
	cfg, err := store.Load()
	if err != nil {
		return config.Library{}, err
	}
	abs, _ := absPath(arg)
	for _, l := range cfg.Libraries {
		if l.ID == arg || l.Name == arg || l.Path == abs {
			return l, nil
		}
	}
	return config.Library{}, fmt.Errorf("%q is not a registered library: add it first (docent add <folder>)", arg)
}

// ───────── render ─────────

func newRender() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "render <file.md>",
		Short: "Print the HTML for one Markdown file (for debugging)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			res, err := render.New().Process(src, render.Options{HTML: true})
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"title": res.Title, "meta": res.Meta, "headings": res.Headings, "words": res.Words, "summary": res.Summary, "warnings": res.Warnings, "html": res.HTML})
			}
			fmt.Print(res.HTML)
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "print title, headings and facts as JSON")
	return c
}

// previewTarget scans what `docent check` was pointed at: a registered library (by id, name or folder), which gets its profile's rules, or any folder or file.
func previewTarget(arg string) (*library.Source, error) {
	store, _, err := openStore()
	if err != nil {
		return nil, err
	}
	cfg, _ := store.Load()
	abs, _ := absPath(arg)
	for _, l := range cfg.Libraries {
		if l.ID == arg || l.Name == arg || l.Path == abs {
			if l.Profile == "devnotes" {
				return library.PreviewWith(l.Path, devnotes.NewHooks(), l, devnotes.Resolve(l).Prefix)
			}
			return library.Preview(l.Path)
		}
	}
	return library.Preview(arg)
}

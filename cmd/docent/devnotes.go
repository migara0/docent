package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/migara0/docent/internal/config"
	"github.com/migara0/docent/internal/devnotes"
	"github.com/migara0/docent/internal/library"
)

// devLibrary picks the developer-notes library a command works on: --library, else the one the current folder is inside, else the only one.
func devLibrary(store *config.Store, flag string) (config.Library, error) {
	cfg, err := store.Load()
	if err != nil {
		return config.Library{}, err
	}
	var devs []config.Library
	for _, l := range cfg.Libraries {
		if l.Profile == "devnotes" {
			devs = append(devs, l)
		}
	}
	if flag != "" {
		for _, l := range cfg.Libraries {
			if l.ID == flag || l.Name == flag || l.Path == flag {
				if l.Profile != "devnotes" {
					return l, fmt.Errorf("%q is a plain library: add it with --profile devnotes to use these commands", flag)
				}
				return l, nil
			}
		}
		return config.Library{}, fmt.Errorf("no library %q (see `docent list`)", flag)
	}
	if cwd, err := os.Getwd(); err == nil {
		for _, l := range devs {
			s := devnotes.Resolve(l)
			for _, root := range []string{l.Path, s.Site, s.Workspace} { // the workspace (where the service repos sit) counts as inside too
				if root != "" && (cwd == root || strings.HasPrefix(cwd, root+string(filepath.Separator))) {
					return l, nil
				}
			}
		}
	}
	switch len(devs) {
	case 0:
		return config.Library{}, fmt.Errorf("no developer-notes library yet: docent add <docs folder> --profile devnotes")
	case 1:
		return devs[0], nil
	}
	names := make([]string, len(devs))
	for i, l := range devs {
		names[i] = l.ID
	}
	return config.Library{}, fmt.Errorf("several developer-notes libraries (%s): choose one with --library", strings.Join(names, ", "))
}

// openDev scans a developer-notes library once.
func openDev(store *config.Store, flag string) (config.Library, devnotes.Settings, *library.Source, error) {
	lib, err := devLibrary(store, flag)
	if err != nil {
		return lib, devnotes.Settings{}, nil, err
	}
	s := devnotes.Resolve(lib)
	src, err := library.PreviewWith(lib.Path, devnotes.NewHooks(), lib, s.Prefix)
	return lib, s, src, err
}

func registry(store *config.Store, lib config.Library, s devnotes.Settings) devnotes.Registry {
	return devnotes.Registry{Dir: filepath.Join(store.Paths().ConfigDir, "ids"), Library: lib.ID, Prefix: s.Prefix}
}

func existingIDs(src *library.Source) []string {
	ids := src.IDs()
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	return out
}

// ───────── docent new ─────────

func newNew() *cobra.Command {
	var libFlag, date, author string
	c := &cobra.Command{
		Use:   "new <service> <type> <title>",
		Short: "Create a developer note from a template, with a fresh ID",
		Long: "Creates docs/<service>/<type>/<yyyy-mm-dd>-<title>.md from the template of that type and reserves the next note ID.\n" +
			"<service> is a repo folder, `platform`, `features/<feature>` or `technologies/<technology>` (guides only).\n" +
			"Types: " + strings.Join(devnotes.Types, ", ") + ".",
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore()
			if err != nil {
				return err
			}
			lib, s, src, err := openDev(store, libFlag)
			if err != nil {
				return err
			}
			sc := devnotes.Scaffold{S: s, Registry: registry(store, lib, s), IDs: func() []string { return existingIDs(src) }}
			rel, id, err := sc.Create(devnotes.NewNote{Service: args[0], Type: args[1], Title: args[2], Date: date, Author: author})
			if err != nil {
				return err
			}
			fmt.Println(filepath.Join(s.Root, filepath.FromSlash(rel)))
			fmt.Fprintf(os.Stderr, "created %s in %s (%s)\n", id, lib.Name, rel)
			return nil
		},
	}
	c.Flags().StringVar(&libFlag, "library", "", "the developer-notes library (default: the one you are inside, or the only one)")
	c.Flags().StringVar(&date, "date", "", "YYYY-MM-DD (default: today)")
	c.Flags().StringVar(&author, "author", "", "author (default: git's user.name)")
	return c
}

// ───────── docent id ─────────

type idEntry struct {
	ID, Date, Title, Description, Library, Path, Abs string
	Tags                                             []string
	Place                                            devnotes.Place
	Updated, Versions, Verified, Technology          string
}

// allNotes lists every note with an ID in every developer-notes library (so an ID of the sibling library resolves too).
func allNotes(store *config.Store, only string) ([]idEntry, error) {
	cfg, err := store.Load()
	if err != nil {
		return nil, err
	}
	var out []idEntry
	for _, l := range cfg.Libraries {
		if l.Profile != "devnotes" || (only != "" && l.ID != only) {
			continue
		}
		s := devnotes.Resolve(l)
		src, err := library.PreviewWith(l.Path, devnotes.NewHooks(), l, s.Prefix)
		if err != nil {
			return nil, err
		}
		for _, pg := range src.Pages() {
			if pg.Meta.Date == "" {
				continue
			}
			out = append(out, idEntry{ID: pg.Meta.ID, Date: pg.Meta.Date, Title: pg.Title, Description: pg.Meta.Description, Library: l.ID, Path: pg.Path,
				Abs: filepath.Join(l.Path, filepath.FromSlash(pg.Path)), Tags: pg.Meta.Tags, Place: devnotes.Locate(pg.Path),
				Updated: pg.Meta.Updated, Versions: pg.Meta.Versions, Verified: pg.Meta.Verified, Technology: pg.Meta.Technology})
		}
	}
	sort.Slice(out, func(i, j int) bool { return idLess(out[i].ID, out[j].ID, out[i].Path, out[j].Path) })
	return out, nil
}

func idLess(a, b, pa, pb string) bool {
	_, na, _ := devnotes.ParseID(a)
	_, nb, _ := devnotes.ParseID(b)
	pfa, _, _ := devnotes.ParseID(a)
	pfb, _, _ := devnotes.ParseID(b)
	if pfa != pfb {
		return pfa < pfb
	}
	if na != nb {
		return na < nb
	}
	return pa < pb
}

func newID() *cobra.Command {
	var libFlag string
	root := &cobra.Command{Use: "id", Short: "Work with note IDs (resolve, find, list, next, check, backfill, guides)"}
	root.PersistentFlags().StringVar(&libFlag, "library", "", "limit to one developer-notes library")

	root.AddCommand(&cobra.Command{
		Use:   "open <id>",
		Short: "Open a note in the running Docent by its ID (DOC-0015, doc-15 or 15)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, paths, err := openStore()
			if err != nil {
				return err
			}
			running, ok := findRunning(paths)
			if !ok {
				return fmt.Errorf("Docent is not running: start it with `docent` first (or use `docent id resolve %s` to print the file)", args[0])
			}
			target := running.URL + "/_id/" + url.PathEscape(args[0])
			res, err := (&http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Get(target)
			if err != nil {
				return err
			}
			res.Body.Close()
			if res.StatusCode != http.StatusFound {
				return fmt.Errorf("no note has the ID %s", args[0])
			}
			fmt.Println(running.URL + res.Header.Get("Location"))
			openBrowser(target)
			return nil
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "resolve <id>...",
		Short: "Show the title and file of each note ID (also finds IDs of the other libraries)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore()
			if err != nil {
				return err
			}
			notes, err := allNotes(store, "")
			if err != nil {
				return err
			}
			status := 0
			def := ""
			if lib, err := devLibrary(store, libFlag); err == nil {
				def = devnotes.Resolve(lib).Prefix
			}
			for _, raw := range args {
				id, ok := devnotes.NormalizeID(raw, def)
				if !ok {
					fmt.Fprintf(os.Stderr, "%s: not a note ID\n", raw)
					status = 1
					continue
				}
				found := false
				for _, n := range notes {
					if n.ID == id {
						fmt.Printf("%s  %s  %s\n    %s:%s\n    %s\n", n.ID, n.Date, n.Title, n.Library, n.Path, n.Abs)
						found = true
					}
				}
				if !found {
					fmt.Fprintf(os.Stderr, "%s: no such note\n", id)
					status = 1
				}
			}
			if status != 0 {
				os.Exit(status)
			}
			return nil
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "find <text>",
		Short: "Search note IDs, titles, descriptions, tags and paths",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore()
			if err != nil {
				return err
			}
			notes, err := allNotes(store, "")
			if err != nil {
				return err
			}
			q := strings.ToLower(strings.Join(args, " "))
			n := 0
			for _, e := range notes {
				hay := strings.ToLower(strings.Join([]string{e.ID, e.Title, e.Description, strings.Join(e.Tags, " "), e.Path}, "\n"))
				if strings.Contains(hay, q) {
					fmt.Printf("%s  %s  %s\n    %s:%s\n", e.ID, e.Date, e.Title, e.Library, e.Path)
					n++
				}
			}
			if n == 0 {
				fmt.Fprintln(os.Stderr, "no notes match")
				os.Exit(1)
			}
			return nil
		},
	})

	var service, typ string
	list := &cobra.Command{
		Use:   "list",
		Short: "List notes by ID",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore()
			if err != nil {
				return err
			}
			notes, err := allNotes(store, libFlag)
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
			for _, e := range notes {
				if (service != "" && e.Place.Label() != service) || (typ != "" && e.Place.Type != typ) {
					continue
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", e.ID, e.Date, e.Place.Label(), e.Place.Type, e.Title)
			}
			return tw.Flush()
		},
	}
	list.Flags().StringVar(&service, "service", "", "only this service, feature (features/x) or technology (technologies/x)")
	list.Flags().StringVar(&typ, "type", "", "only this note type")
	root.AddCommand(list)

	root.AddCommand(&cobra.Command{
		Use:   "next",
		Short: "Show the ID the next new note will get (without reserving it)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore()
			if err != nil {
				return err
			}
			lib, s, src, err := openDev(store, libFlag)
			if err != nil {
				return err
			}
			fmt.Println(registry(store, lib, s).Peek(existingIDs(src)))
			return nil
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Report duplicate, missing or malformed IDs (exit 1 if any)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore()
			if err != nil {
				return err
			}
			_, _, src, err := openDev(store, libFlag)
			if err != nil {
				return err
			}
			n := 0
			for _, p := range src.Problems() {
				if p.Kind == "duplicate-id" || (p.Kind == "front-matter" && (strings.HasPrefix(p.Message, "no id") || strings.HasPrefix(p.Message, "malformed id") || strings.Contains(p.Message, "has the prefix"))) {
					fmt.Printf("%s: %s\n", p.Page, p.Message)
					n++
				}
			}
			fmt.Printf("%d notes, %d problem(s)\n", len(existingIDs(src)), n)
			if n > 0 {
				os.Exit(1)
			}
			return nil
		},
	})

	var apply bool
	backfill := &cobra.Command{
		Use:   "backfill",
		Short: "Give IDs to notes that have none, oldest first (a dry run unless --apply)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore()
			if err != nil {
				return err
			}
			lib, s, src, err := openDev(store, libFlag)
			if err != nil {
				return err
			}
			var todo []*library.Page
			for _, pg := range src.Pages() {
				if pg.Meta.Date != "" && pg.Meta.ID == "" && devnotes.Locate(pg.Path).IsNotePlace() {
					todo = append(todo, pg)
				}
			}
			sort.Slice(todo, func(i, j int) bool {
				if todo[i].Meta.Date != todo[j].Meta.Date {
					return todo[i].Meta.Date < todo[j].Meta.Date
				}
				return todo[i].Path < todo[j].Path
			})
			if len(todo) == 0 {
				fmt.Println("every note already has an ID")
				return nil
			}
			reg := registry(store, lib, s)
			existing := existingIDs(src)
			for _, pg := range todo {
				var id string
				if apply {
					if id, err = reg.Next(existing); err != nil {
						return err
					}
					existing = append(existing, id)
					if err := devnotes.InsertID(filepath.Join(s.Root, filepath.FromSlash(pg.Path)), id); err != nil {
						return err
					}
				} else {
					id = reg.Peek(existing)
					existing = append(existing, id)
				}
				fmt.Printf("%s  %s\n", id, pg.Path)
			}
			if !apply {
				fmt.Printf("\n%d note(s) would get an ID. Run again with --apply to write them.\n", len(todo))
			}
			return nil
		},
	}
	backfill.Flags().BoolVar(&apply, "apply", false, "write the IDs into the notes' front matter")
	root.AddCommand(backfill)

	var months int
	guides := &cobra.Command{
		Use:   "guides",
		Short: "List technology guides with their versions, and flag any not verified lately",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore()
			if err != nil {
				return err
			}
			notes, err := allNotes(store, libFlag)
			if err != nil {
				return err
			}
			cutoff := time.Now().AddDate(0, -months, 0).Format("2006-01-02")
			tw := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
			for _, e := range notes {
				if e.Place.Type != "guide" {
					continue
				}
				flag := ""
				if e.Verified == "" || e.Verified < cutoff {
					flag = fmt.Sprintf("STALE (verified %s)", firstNonEmpty(e.Verified, "never"))
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", e.ID, firstNonEmpty(e.Technology, e.Place.Service), e.Versions, e.Verified, flag)
			}
			return tw.Flush()
		},
	}
	guides.Flags().IntVar(&months, "stale-months", 6, "a guide not verified for this many months is flagged")
	root.AddCommand(guides)
	return root
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

var _ = regexp.MustCompile
var _ = slices.Contains[[]string]

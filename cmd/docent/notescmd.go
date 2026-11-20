package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/migara0/docent/internal/devnotes"
)

// ───────── docent commits ─────────

func newCommits() *cobra.Command {
	var libFlag string
	root := &cobra.Command{
		Use:   "commits",
		Short: "Record the commits of a feature or fix and turn them into cherry-pick commands",
		Long: "Works on notes of type `commits`. Read-only against git: it only runs `git log` and `git show`, it never stages, commits or cherry-picks.\n" +
			"<note> is a path, a path under docs/, or a unique fragment of the file name. Repos are folders next to the notes folder and are addressed as `git -C <repo>`.",
	}
	root.PersistentFlags().StringVar(&libFlag, "library", "", "the developer-notes library (default: the one you are inside, or the only one)")

	open := func() (devnotes.Commits, error) {
		store, _, err := openStore()
		if err != nil {
			return devnotes.Commits{}, err
		}
		lib, err := devLibrary(store, libFlag)
		if err != nil {
			return devnotes.Commits{}, err
		}
		return devnotes.Commits{S: devnotes.Resolve(lib)}, nil
	}

	var repo, rng, grep, author string
	var shas []string
	add := &cobra.Command{
		Use:   "add <note> --repo <repo> (--range develop..feat/x | --sha <sha>...)",
		Short: "Record (or re-read) the commits of one repo in a note",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if rng == "" && len(shas) == 0 {
				return fmt.Errorf("give --range or --sha")
			}
			c, err := open()
			if err != nil {
				return err
			}
			path, err := c.Find(args[0])
			if err != nil {
				return err
			}
			n, err := c.Upsert(path, devnotes.CommitSpec{Repo: repo, Range: rng, Shas: shas, Grep: grep, Author: author})
			if err != nil {
				return err
			}
			fmt.Printf("%s: recorded %d commit(s) from %s\n", relTo(c.S.Root, path), n, repo)
			return nil
		},
	}
	add.Flags().StringVar(&repo, "repo", "", "repo folder (required)")
	add.Flags().StringVar(&rng, "range", "", "git revision range, e.g. develop..feat/x")
	add.Flags().StringSliceVar(&shas, "sha", nil, "explicit commit SHAs (repeat or comma-separate)")
	add.Flags().StringVar(&grep, "grep", "", "only commits whose message matches (case-insensitive)")
	add.Flags().StringVar(&author, "author", "", "only commits by this author")
	_ = add.MarkFlagRequired("repo")

	var only string
	refresh := &cobra.Command{
		Use:   "refresh <note> [--repo <repo>]",
		Short: "Re-read every recorded repo of a note from git",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := open()
			if err != nil {
				return err
			}
			path, err := c.Find(args[0])
			if err != nil {
				return err
			}
			done, err := c.Refresh(path, only)
			for _, d := range done {
				fmt.Println(relTo(c.S.Root, path)+":", d)
			}
			return err
		},
	}
	refresh.Flags().StringVar(&only, "repo", "", "only this repo")

	pick := &cobra.Command{
		Use:   "pick <note>...",
		Short: "Print the combined cherry-pick plan for a release",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := open()
			if err != nil {
				return err
			}
			plan, warns, err := c.Plan(args)
			if err != nil {
				return err
			}
			for _, w := range warns {
				fmt.Fprintln(os.Stderr, w)
			}
			fmt.Print(plan)
			return nil
		},
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List all commits notes with status and counts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := open()
			if err != nil {
				return err
			}
			notes := c.All()
			if len(notes) == 0 {
				fmt.Fprintln(os.Stderr, "no commits notes yet")
				return nil
			}
			for _, p := range notes {
				fmt.Println(c.ListLine(p))
			}
			return nil
		},
	}
	root.AddCommand(add, refresh, pick, list)
	return root
}

func relTo(base, p string) string {
	if r, err := filepath.Rel(base, p); err == nil {
		return filepath.ToSlash(r)
	}
	return p
}

// ───────── docent lint ─────────

func newLint() *cobra.Command {
	var libFlag string
	var strict bool
	c := &cobra.Command{
		Use:   "lint [note...]",
		Short: "Find leftovers in developer notes: template variables, TODOs, placeholder IDs, empty sections",
		Long:  "Checks the prose of every note (code blocks and front matter are ignored; `commits` notes are skipped). A line with `lint: ok` keeps a deliberate occurrence.\nExit code 1 on errors, and on warnings too with --strict.",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore()
			if err != nil {
				return err
			}
			_, s, src, err := openDev(store, libFlag)
			if err != nil {
				return err
			}
			wanted := map[string]bool{}
			for _, a := range args {
				abs, _ := filepath.Abs(a)
				if rel, err := filepath.Rel(s.Root, abs); err == nil && !strings.HasPrefix(rel, "..") {
					wanted[filepath.ToSlash(rel)] = true
				} else {
					wanted[filepath.ToSlash(a)] = true
				}
			}
			checked := 0
			for _, pg := range src.Pages() {
				if pg.Meta.Date != "" && devnotes.Locate(pg.Path).Type != "commits" && (len(wanted) == 0 || wanted[pg.Path]) {
					checked++
				}
			}
			var errs, warns int
			finds := devnotes.LintLibrary(s, src)
			sort.SliceStable(finds, func(i, j int) bool {
				if finds[i].Page != finds[j].Page {
					return finds[i].Page < finds[j].Page
				}
				return finds[i].Line < finds[j].Line
			})
			for _, f := range finds {
				if len(wanted) > 0 && !wanted[f.Page] {
					continue
				}
				fmt.Printf("%s:%d: %s: %s\n", sitePath(s, f.Page), f.Line, f.Level, f.Message)
				if f.Level == "error" {
					errs++
				} else {
					warns++
				}
			}
			fmt.Printf("%d note(s) checked, %d error(s), %d warning(s)\n", checked, errs, warns)
			if errs > 0 || (strict && warns > 0) {
				os.Exit(1)
			}
			return nil
		},
	}
	c.Flags().StringVar(&libFlag, "library", "", "the developer-notes library")
	c.Flags().BoolVar(&strict, "strict", false, "exit 1 on warnings as well as errors")
	return c
}

// ───────── docent stale ─────────

func newStale() *cobra.Command {
	var libFlag string
	var all bool
	c := &cobra.Command{
		Use:   "stale",
		Short: "Find module notes whose code changed after the note was last updated",
		Long: "A module note's \"Where it lives\" section names the files and folders of the module in backticks. For each path this asks git (read-only) for the date of its last commit\n" +
			"and compares it with the note's `updated:` date (or `date:`). Exit code 1 when a note is stale.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore()
			if err != nil {
				return err
			}
			_, s, src, err := openDev(store, libFlag)
			if err != nil {
				return err
			}
			mods := devnotes.StaleModules(s, src)
			stale := 0
			for _, m := range mods {
				if len(m.Changes) > 0 {
					stale++
					fmt.Printf("STALE  %s  %s  (updated %s; %s)\n", m.ID, m.Title, m.Since, m.Page)
					for _, ch := range m.Changes {
						fmt.Printf("         %s  last changed %s\n", ch.Path, ch.Changed)
					}
				} else if all {
					fmt.Printf("ok     %s  %s  (updated %s; %d path(s) checked)\n", m.ID, m.Title, m.Since, m.Checked)
				}
			}
			fmt.Printf("%d module note(s), %d stale\n", len(mods), stale)
			if stale > 0 {
				os.Exit(1)
			}
			return nil
		},
	}
	c.Flags().StringVar(&libFlag, "library", "", "the developer-notes library")
	c.Flags().BoolVar(&all, "all", false, "also list modules that are up to date")
	return c
}

// sitePath shows a page path the way the notes site folder holds it (docs/<...>), so it can be opened from the workspace.
func sitePath(s devnotes.Settings, rel string) string {
	if s.Site == "" {
		return rel
	}
	return relTo(s.Site, filepath.Join(s.Root, filepath.FromSlash(rel)))
}

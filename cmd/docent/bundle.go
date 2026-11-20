package main

import (
	"context"
	"fmt"
	"html"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/migara0/docent/internal/config"
	"github.com/migara0/docent/internal/devnotes"
	"github.com/migara0/docent/internal/library"
	"github.com/migara0/docent/internal/server"
)

func newBundle() *cobra.Command {
	var libFlag, out, title string
	var noCommits, keepOmitted, listOnly bool
	c := &cobra.Command{
		Use:   "bundle <note>... --out FILE.md|FILE.pdf",
		Short: "Combine a feature's notes into one Markdown or PDF file, for a release hand-off",
		Long: "docent bundle <commits-note> --out release.pdf      the commits note and every note that belongs with it\n" +
			"docent bundle DOC-0007 DOC-0012 --out pack.md       exactly these notes (IDs or paths to .md files)\n\n" +
			"Which notes belong with a `commits` note: under features/<feature>/ every note of that feature; otherwise the notes it mentions (by ID or relative link)\n" +
			"and the notes that mention it. Order: module, implementation, bugfix, upgrade, api, adr, qa, runbook, guide, commits last.\n\n" +
			"Markdown: one file with a contents list; headings move down one level and links between bundled notes become in-file anchors.\n" +
			"PDF: each note is printed with headless Chrome from Docent's own pages (the same print styles and omissions as the Print button, diagrams included),\n" +
			"behind a cover page, then merged with pdfunite. Parts left out by default: sections such as \"Related notes\", `{ data-no-print }` headings and `no-print` blocks.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore()
			if err != nil {
				return err
			}
			lib, s, src, err := openDev(store, libFlag)
			if err != nil {
				return err
			}
			all := devnotes.Notes(s, src)
			chosen, err := devnotes.Members(args, all, s.Prefix, !noCommits)
			if err != nil {
				return err
			}
			if len(chosen) == 0 {
				return fmt.Errorf("nothing to bundle")
			}
			if title == "" {
				first, _ := devnotes.FindNote(args[0], all, s.Prefix)
				title = first.Title
			}
			if listOnly || out == "" {
				for _, n := range chosen {
					fmt.Printf("%s  %-15s %s  (%s)\n", n.ID, n.Type, n.Title, sitePath(s, n.Path))
				}
				note := ""
				if out == "" {
					note = "; give --out FILE.md or FILE.pdf to write the bundle"
				}
				fmt.Printf("%d note(s)%s\n", len(chosen), note)
				return nil
			}
			ext := filepath.Ext(out)
			if ext != ".md" && ext != ".pdf" {
				return fmt.Errorf("--out must end in .md or .pdf")
			}
			if _, err := os.Stat(out); err == nil {
				return fmt.Errorf("%s already exists", out)
			}
			site := filepath.Base(s.Site)
			if s.Site == "" {
				site = lib.Name
			}
			if ext == ".md" {
				text := devnotes.MarkdownBundle(title, site, chosen, devnotes.PrintOmit(s), keepOmitted, time.Now())
				if err := os.WriteFile(out, []byte(text), 0o644); err != nil {
					return err
				}
			} else if err := pdfBundle(title, site, lib.ID, chosen, out); err != nil {
				return err
			}
			fmt.Printf("wrote %s (%d notes)\n", out, len(chosen))
			return nil
		},
	}
	c.Flags().StringVar(&libFlag, "library", "", "the developer-notes library")
	c.Flags().StringVar(&out, "out", "", "output file (.md or .pdf)")
	c.Flags().StringVar(&title, "title", "", "title of the bundle (default: the title of the first note)")
	c.Flags().BoolVar(&noCommits, "no-commits", false, "leave commits notes out")
	c.Flags().BoolVar(&keepOmitted, "keep-omitted", false, "Markdown only: keep sections that printing leaves out")
	c.Flags().BoolVar(&listOnly, "list", false, "only list the notes that would be bundled")
	return c
}

// findChrome finds a Chromium-based browser for headless printing and drawing: Chrome or Chromium on the PATH, else where Windows
// and macOS install them. Microsoft Edge comes with Windows and prints the same way, so it is the last resort there.
func findChrome() (string, error) {
	for _, n := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if p, err := exec.LookPath(n); err == nil {
			return p, nil
		}
	}
	for _, p := range chromePaths(runtime.GOOS, os.Getenv) {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("no Chrome, Chromium or Edge found (install Google Chrome, or put it on the PATH)")
}

// chromePaths lists the usual install locations of Chrome, Chromium and Edge on goos, most preferred first.
func chromePaths(goos string, env func(string) string) []string {
	switch goos {
	case "windows":
		var out []string
		for _, base := range []string{env("ProgramFiles"), env("ProgramFiles(x86)"), env("LocalAppData")} {
			if base != "" {
				out = append(out, filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"))
			}
		}
		for _, base := range []string{env("ProgramFiles(x86)"), env("ProgramFiles")} {
			if base != "" {
				out = append(out, filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"))
			}
		}
		return out
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	}
	return nil
}

func printPDF(chrome, profile, url, target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, chrome, "--headless=new", "--no-sandbox", "--disable-gpu", "--no-pdf-header-footer", "--virtual-time-budget=25000",
		"--user-data-dir="+profile, "--print-to-pdf="+target, url)
	output, _ := cmd.CombinedOutput()
	if st, err := os.Stat(target); err != nil || st.Size() == 0 {
		tail := string(output)
		if len(tail) > 400 {
			tail = tail[len(tail)-400:]
		}
		return fmt.Errorf("could not print %s\n%s", url, tail)
	}
	return nil
}

// pdfBundle prints every note from a private, loopback-only Docent server, then merges the parts with pdfunite.
func pdfBundle(title, site, libID string, chosen []devnotes.BundleNote, out string) error {
	chrome, err := findChrome()
	if err != nil {
		return err
	}
	unite, err := exec.LookPath("pdfunite")
	if err != nil {
		return fmt.Errorf("pdfunite not found (install poppler-utils)")
	}
	store, _, err := openStore()
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "docent-bundle-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	ls, err := startLocal(store)
	if err != nil {
		return err
	}
	defer ls.Close()
	live, err := ls.Source(libID)
	if err != nil {
		return err
	}
	base := ls.Base

	rows := ""
	for _, n := range chosen {
		rows += fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", html.EscapeString(n.ID), html.EscapeString(n.Title), n.Type, n.Updated)
	}
	cover := filepath.Join(tmp, "cover.html")
	doc := fmt.Sprintf(`<!doctype html><meta charset="utf-8"><title>%[1]s</title>
<style>@page{margin:18mm}body{font:12pt/1.5 system-ui,sans-serif;color:#222}h1{margin-top:30mm}table{border-collapse:collapse;width:100%%;font-size:10pt}
td,th{border-bottom:1px solid #ddd;padding:4px 8px;text-align:left}p{color:#666}</style>
<h1>%[1]s</h1><p>%[2]d notes from %[3]s · generated %[4]s</p>
<table><tr><th>ID</th><th>Note</th><th>Type</th><th>Updated</th></tr>%[5]s</table>`, html.EscapeString(title), len(chosen), html.EscapeString(site), time.Now().Format("2006-01-02"), rows)
	if err := os.WriteFile(cover, []byte(doc), 0o644); err != nil {
		return err
	}
	profile := filepath.Join(tmp, "chrome-profile")
	parts := []string{filepath.Join(tmp, "000-cover.pdf")}
	if err := printPDF(chrome, profile, "file://"+cover, parts[0]); err != nil {
		return err
	}
	for i, n := range chosen {
		target := filepath.Join(tmp, fmt.Sprintf("%03d.pdf", i+1))
		if err := printPDF(chrome, profile, base+live.PageURL(n.Path)+"?print", target); err != nil {
			return err
		}
		parts = append(parts, target)
		fmt.Fprintf(os.Stderr, "  printed %s %s\n", n.ID, n.Title)
	}
	args := append(parts, out)
	if b, err := exec.Command(unite, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("pdfunite: %v\n%s", err, b)
	}
	return nil
}

// localServer is a private, loopback-only Docent server for commands that need a real browser (bundle PDFs, diagram checks).
type localServer struct {
	Base   string
	Mgr    *library.Manager
	srv    *http.Server
	cancel context.CancelFunc
}

func startLocal(store *config.Store) (*localServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	mgr := library.NewManager(ctx, store, library.WithHooks(devnotes.NewHooks()))
	if err := mgr.Sync(); err != nil {
		cancel()
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		return nil, err
	}
	srv := &http.Server{Handler: server.New(mgr, store, server.Options{Version: version}).Handler(), ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(ln)
	return &localServer{Base: "http://" + ln.Addr().String(), Mgr: mgr, srv: srv, cancel: cancel}, nil
}

func (l *localServer) Close() {
	_ = l.srv.Close()
	l.cancel()
}

// Source returns a loaded library once its first scan has finished.
func (l *localServer) Source(id string) (*library.Source, error) {
	src, ok := l.Mgr.Get(id)
	if !ok {
		return nil, fmt.Errorf("library %q is not loaded", id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return src, src.WaitReady(ctx)
}

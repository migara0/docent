package main

import (
	"context"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	figureOpen   = regexp.MustCompile(`<figure class="diagram"`)
	diagramError = regexp.MustCompile(`(?s)<div class="diagram__error">.*?<pre>(.*?)</pre>`)
)

var fenceStart = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})\\s*(\\w*)")

// fenceLines are the line numbers of the ```mermaid fences of a file that really start a diagram (a fence shown inside a longer code block is only text).
func fenceLines(path string) []int {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []int
	open := ""
	for i, line := range strings.Split(string(data), "\n") {
		m := fenceStart.FindStringSubmatch(line)
		if open != "" {
			if m != nil && m[2] == "" && m[1][0] == open[0] && len(m[1]) >= len(open) {
				open = ""
			}
			continue
		}
		if m != nil {
			open = m[1]
			if m[2] == "mermaid" {
				out = append(out, i+1)
			}
		}
	}
	return out
}

// checkDiagrams draws every Mermaid diagram of a library in headless Chrome, with the Mermaid that ships inside Docent (no internet needed), and
// reports the ones that cannot be drawn. only limits the check to those page paths (relative to the library).
func checkDiagrams(libID string, only map[string]bool) (checked, broken int, err error) {
	chrome, err := findChrome()
	if err != nil {
		return 0, 0, err
	}
	store, _, err := openStore()
	if err != nil {
		return 0, 0, err
	}
	ls, err := startLocal(store)
	if err != nil {
		return 0, 0, err
	}
	defer ls.Close()
	src, err := ls.Source(libID)
	if err != nil {
		return 0, 0, err
	}
	tmp, err := os.MkdirTemp("", "docent-diagrams-")
	if err != nil {
		return 0, 0, err
	}
	defer os.RemoveAll(tmp)

	pages := src.Pages()
	sort.Slice(pages, func(i, j int) bool { return pages[i].Path < pages[j].Path })
	for _, pg := range pages {
		if len(only) > 0 && !only[pg.Path] {
			continue
		}
		abs := filepath.Join(src.Root, filepath.FromSlash(pg.Path))
		lines := fenceLines(abs)
		if len(lines) == 0 {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		out, _ := exec.CommandContext(ctx, chrome, "--headless=new", "--no-sandbox", "--disable-gpu", "--virtual-time-budget=30000",
			"--user-data-dir="+filepath.Join(tmp, "profile"), "--dump-dom", ls.Base+src.PageURL(pg.Path)+"?print").Output()
		cancel()
		dom := string(out)
		starts := figureOpen.FindAllStringIndex(dom, -1)
		if len(starts) != len(lines) {
			fmt.Printf("%s: %d diagram(s) in the file but %d drawn: the page could not be checked\n", pg.Path, len(lines), len(starts))
			checked += len(lines)
			broken += len(lines)
			continue
		}
		for i, st := range starts {
			checked++
			end := len(dom)
			if i+1 < len(starts) {
				end = starts[i+1][0]
			}
			if m := diagramError.FindStringSubmatch(dom[st[0]:end]); m != nil {
				broken++
				fmt.Printf("%s:%d: diagram %d %s\n", pg.Path, lines[i], i+1, strings.Join(strings.Fields(html.UnescapeString(m[1])), " "))
			}
		}
	}
	return checked, broken, nil
}

package library

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/migara0/docent/internal/render"
)

// Search comes in two forms, chosen by library size (measured: a 10,000-page library gives a 50 MB index and 360 MB of memory, so one form cannot fit all):
//
//   - Small libraries: the browser downloads a compact index once and searches it instantly, offline, with typo tolerance (MiniSearch).
//   - Large libraries: the server searches the files themselves on every query. It keeps nothing in memory, never goes stale, and uses a few cores
//     only for the moment a query runs (a 64 MB library answers in about 0.1 s).

const (
	searchBodyBytes   = 6000    // per page in the client index
	clientIndexLimit  = 3 << 20 // above this the client index is not offered
	searchWorkers     = 4
	maxQueryTerms     = 6
	snippetRadius     = 90
	maxSearchHits     = 100
	defaultSearchHits = 20
)

type searchDoc struct {
	ID       int    `json:"id"`
	Path     string `json:"path"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Folder   string `json:"folder"`
	Headings string `json:"headings"`
	Text     string `json:"text"`
}

// SearchIndex returns the JSON the browser needs to search this source itself, or a small {"mode":"server"} answer when the library is too large for that.
func (s *Source) SearchIndex() ([]byte, error) {
	snap := s.snap.Load()
	snap.searchMu.Lock()
	defer snap.searchMu.Unlock()
	if snap.search != nil {
		return snap.search, nil
	}
	est := 0
	for _, pg := range snap.pages {
		est += min(int(pg.Size), searchBodyBytes) + 200
	}
	if est > clientIndexLimit {
		snap.search, _ = json.Marshal(map[string]any{"mode": "server", "source": s.ID, "pages": len(snap.pages)})
		return snap.search, nil
	}
	paths := make([]string, 0, len(snap.pages))
	for p := range snap.pages {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	docs := make([]searchDoc, len(paths))
	var wg sync.WaitGroup
	jobs := make(chan int)
	for range min(runtime.GOMAXPROCS(0), searchWorkers) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				pg := snap.pages[paths[i]]
				d := searchDoc{ID: i, Path: pg.Path, URL: pageURL(s.ID, pg.Path), Title: pg.Title, Folder: path.Dir(pg.Path)}
				if pg.Size <= maxFileBytes {
					if src, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(pg.Path))); err == nil {
						if res, err := s.rend.Process(src, render.Options{Text: true}); err == nil {
							d.Text = clip(res.Text, searchBodyBytes)
							hs := make([]string, 0, len(res.Headings))
							for _, h := range res.Headings {
								hs = append(hs, h.Text)
							}
							d.Headings = strings.Join(hs, " · ")
						}
					}
				}
				docs[i] = d
			}
		}()
	}
	for i := range paths {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	out, err := json.Marshal(map[string]any{"mode": "client", "source": s.ID, "docs": docs})
	if err != nil {
		return nil, err
	}
	snap.search = out
	return out, nil
}

// SearchHit is one result of a server-side search.
type SearchHit struct {
	Path    string `json:"path"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	Folder  string `json:"folder"`
	Score   int    `json:"score"`
	Snippet string `json:"snippet,omitempty"`
}

// queryTerms lower-cases the query and splits it into words. Every word must appear in a page for it to match.
func queryTerms(q string) []string {
	var terms []string
	for _, f := range strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return unicode.IsSpace(r) || r == ',' }) {
		if f != "" {
			terms = append(terms, f)
		}
		if len(terms) == maxQueryTerms {
			break
		}
	}
	return terms
}

// Search finds pages containing every word of the query, reading the files directly. Title matches rank first, then headings, then how often the words occur.
// It stops early when ctx ends (the browser cancels the request when the user types another letter).
func (s *Source) Search(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	terms := queryTerms(query)
	if len(terms) == 0 {
		return []SearchHit{}, nil
	}
	if limit <= 0 || limit > maxSearchHits {
		limit = defaultSearchHits
	}
	snap := s.snap.Load()
	paths := make([]string, 0, len(snap.pages))
	for p := range snap.pages {
		paths = append(paths, p)
	}
	byteTerms := make([][]byte, len(terms))
	ascii := true
	for i, t := range terms {
		byteTerms[i] = []byte(t)
		for _, c := range []byte(t) {
			ascii = ascii && c < utf8.RuneSelf
		}
	}

	var (
		mu   sync.Mutex
		hits []SearchHit
		wg   sync.WaitGroup
	)
	jobs := make(chan string)
	for range min(runtime.GOMAXPROCS(0), searchWorkers) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var buf []byte // reused for every file this worker reads: no allocation per page
			for p := range jobs {
				if ctx.Err() != nil {
					continue
				}
				pg := snap.pages[p]
				var lower []byte
				lower, buf = s.readLowered(pg, buf, ascii)
				if score, ok := scorePage(pg, lower, byteTerms); ok {
					mu.Lock()
					hits = append(hits, SearchHit{Path: p, URL: pageURL(s.ID, p), Title: pg.Title, Folder: path.Dir(p), Score: score})
					mu.Unlock()
				}
			}
		}()
	}
	for _, p := range paths {
		if ctx.Err() != nil {
			break
		}
		jobs <- p
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return naturalLess(hits[i].Title, hits[j].Title)
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	for i := range hits {
		hits[i].Snippet = s.snippet(hits[i].Path, byteTerms[0])
	}
	return hits, nil
}

// readLowered reads a page into buf (growing it when needed) and lower-cases it in place. ASCII text, which is nearly all documentation, needs no allocation.
func (s *Source) readLowered(pg *Page, buf []byte, asciiTerms bool) ([]byte, []byte) {
	if pg.Size > maxFileBytes {
		return nil, buf
	}
	f, err := os.Open(filepath.Join(s.Root, filepath.FromSlash(pg.Path)))
	if err != nil {
		return nil, buf
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, buf
	}
	if n := int(info.Size()); cap(buf) < n {
		buf = make([]byte, n, n+n/4)
	} else {
		buf = buf[:n]
	}
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil, buf
	}
	// Plain-ASCII search words can only match ASCII bytes, so folding ASCII letters in place is exact, whatever else the page contains (em dashes and
	// arrows are common in notes). Only a search word with non-ASCII letters needs the slower Unicode-aware lower-casing (which allocates).
	if !asciiTerms {
		return bytes.ToLower(buf), buf
	}
	for i, c := range buf {
		if 'A' <= c && c <= 'Z' {
			buf[i] = c + 'a' - 'A'
		}
	}
	return buf, buf
}

// scorePage scores one page from its lower-cased text; ok is false when a word is missing.
func scorePage(pg *Page, lower []byte, terms [][]byte) (int, bool) {
	title := strings.ToLower(pg.Title)
	score := 0
	for _, t := range terms {
		inTitle := strings.Contains(title, string(t))
		n := bytes.Count(lower, t)
		if !inTitle && n == 0 {
			return 0, false
		}
		if inTitle {
			score += 20
			if strings.HasPrefix(title, string(t)) {
				score += 5
			}
		}
		score += min(n, 10)
		if n > 0 && termInHeading(lower, t) {
			score += 6
		}
	}
	return score, true
}

// termInHeading reports whether the word occurs on a Markdown heading line (one starting with #). It looks only at the lines where the word occurs.
func termInHeading(lower, term []byte) bool {
	from := 0
	for tries := 0; tries < 200; tries++ {
		i := bytes.Index(lower[from:], term)
		if i < 0 {
			return false
		}
		i += from
		if start := bytes.LastIndexByte(lower[:i], '\n') + 1; start < len(lower) && lower[start] == '#' {
			return true
		}
		from = i + len(term)
	}
	return false
}

// snippet returns a short passage around the first occurrence of the term, with Markdown punctuation removed.
func (s *Source) snippet(rel string, term []byte) string {
	src, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))                 // Windows line endings: the front matter below is found by "\n---\n"
	if body := bytes.TrimPrefix(src, []byte("---\n")); len(body) < len(src) { // skip front matter
		if i := bytes.Index(body, []byte("\n---\n")); i >= 0 {
			src = body[i+5:]
		}
	}
	i := bytes.Index(bytes.ToLower(src), term)
	if i < 0 {
		return ""
	}
	start, end := max(i-snippetRadius, 0), min(i+len(term)+snippetRadius, len(src))
	for start > 0 && !utf8.RuneStart(src[start]) {
		start--
	}
	for end < len(src) && !utf8.RuneStart(src[end]) {
		end++
	}
	text := strings.NewReplacer("!!!", "", "???+", "", "???", "", "#", "", "*", "", "`", "", "|", " ", ">", "", "[", "", "]", "", "\n", " ").Replace(string(src[start:end]))
	text = strings.Join(strings.Fields(text), " ")
	if start > 0 {
		text = "…" + text
	}
	if end < len(src) {
		text += "…"
	}
	return text
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

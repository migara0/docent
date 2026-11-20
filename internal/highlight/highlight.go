// Package highlight finds the syntax-highlighting lexer for a code block's language.
//
// It replaces chroma's own `lexers` package, which registers about 280 lexers when the program starts (parsing every embedded XML file) and so costs
// about 6 MB of memory and 3 MB of binary before a single page is read. Here only a curated set of common languages is embedded, and each one is
// parsed the first time a page uses it. Other languages show as plain text. The XML lexers and the three lexers written in Go (go.go, http.go,
// markdown.go) come from chroma (MIT licence, Copyright (C) 2017 Alec Thomas); see NOTICE.md.
package highlight

import (
	"embed"
	"io/fs"
	"regexp"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
)

//go:embed xml/*.xml
var files embed.FS

var (
	once    sync.Once
	byAlias map[string]string // lower-cased name or alias -> file in files

	mu     sync.Mutex
	reg    = chroma.NewLexerRegistry() // lexers loaded so far; XML lexers that embed another one (Using "Bash") find it here
	loaded = map[string]chroma.Lexer{}
)

var (
	tag      = regexp.MustCompile(`<(name|alias|filename)>([^<]+)</`)
	usesLexe = regexp.MustCompile(`<using\s+lexer="([^"]+)"`)
)

// index reads the names and aliases of the embedded lexers (only the top of each file) the first time a language is asked for.
func index() {
	byAlias = map[string]string{}
	extra := map[string]string{}
	defer func() {
		for k, v := range extra {
			if _, taken := byAlias[k]; !taken {
				byAlias[k] = v
			}
		}
	}()
	entries, _ := fs.ReadDir(files, "xml")
	for _, e := range entries {
		data, err := files.ReadFile("xml/" + e.Name())
		if err != nil {
			continue
		}
		head := data
		if i := strings.Index(string(head), "</config>"); i > 0 {
			head = head[:i]
		}
		for _, m := range tag.FindAllSubmatch(head, -1) {
			key := strings.ToLower(string(m[2]))
			if string(m[1]) == "filename" { // "*.jsonc" makes `jsonc` a language name too, as in chroma, but never beats a real name or alias
				if !strings.HasPrefix(key, "*.") || strings.ContainsAny(key[2:], "*?[") {
					continue
				}
				key = key[2:]
				if _, taken := byAlias[key]; !taken {
					extra[key] = "xml/" + e.Name()
				}
				continue
			}
			if _, taken := byAlias[key]; !taken || string(m[1]) == "name" {
				byAlias[key] = "xml/" + e.Name()
			}
		}
	}
}

// Get returns the lexer for a language name or alias (case-insensitive), or nil when the language is not in the curated set.
func Get(name string) chroma.Lexer {
	once.Do(index)
	mu.Lock()
	defer mu.Unlock()
	return get(name)
}

func get(name string) chroma.Lexer {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return nil
	}
	switch name {
	case "go", "golang":
		return goLexer
	case "http":
		return httpLexer
	case "md", "mkd", "markdown":
		return markdownLex
	}
	file, ok := byAlias[name]
	if !ok {
		return nil
	}
	if l, ok := loaded[file]; ok {
		return l
	}
	loaded[file] = nil // also stops a lexer that (indirectly) uses itself
	data, err := files.ReadFile(file)
	if err != nil {
		return nil
	}
	for _, m := range usesLexe.FindAllSubmatch(data, -1) { // load what it embeds first
		get(string(m[1]))
	}
	l, err := chroma.NewXMLLexer(files, file)
	if err != nil {
		return nil
	}
	l.SetRegistry(reg)
	reg.Register(l)
	loaded[file] = l
	return l
}

// MatchMimeType picks the lexer for a response body of an HTTP example by its Content-Type (the few that matter for API notes).
func MatchMimeType(mime string) chroma.Lexer {
	mime = strings.ToLower(strings.TrimSpace(strings.SplitN(mime, ";", 2)[0]))
	switch {
	case mime == "application/json" || strings.HasSuffix(mime, "+json"):
		return Get("json")
	case mime == "text/html":
		return Get("html")
	case mime == "application/xml" || mime == "text/xml" || strings.HasSuffix(mime, "+xml"):
		return Get("xml")
	case mime == "text/css":
		return Get("css")
	case strings.Contains(mime, "javascript"):
		return Get("javascript")
	case strings.Contains(mime, "yaml"):
		return Get("yaml")
	}
	return nil
}

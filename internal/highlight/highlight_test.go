package highlight

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// tokens renders a lexer's output in a comparable form.
func tokens(t *testing.T, l chroma.Lexer, code string) string {
	t.Helper()
	it, err := chroma.Coalesce(l).Tokenise(nil, code)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for tok := it(); tok != chroma.EOF; tok = it() {
		b.WriteString(tok.Type.String() + "=" + tok.Value + "|")
	}
	return b.String()
}

var samples = map[string]string{
	"bash":       "#!/bin/bash\nset -e\nfor f in *.md; do echo \"$f\" | wc -l; done\n",
	"sh":         "ls -la && echo done\n",
	"json":       `{"a": [1, 2.5, true, null], "b": "x"}`,
	"jsonc":      "{\n  // comment\n  \"a\": 1\n}\n",
	"yaml":       "a: 1\nb:\n  - x\n  - y: \"z\"\n",
	"ts":         "export async function f(a: number): Promise<string> { return `x${a}`; }\n",
	"typescript": "interface A { b?: string }\nconst x: A = {};\n",
	"js":         "const f = (a) => a + 1; // c\n",
	"javascript": "class A { #p = 1; get p() { return this.#p } }\n",
	"python":     "def f(x):\n    \"\"\"doc\"\"\"\n    return [i for i in range(x)]\n",
	"html":       "<div class=\"a\">hi <b>there</b></div>\n",
	"http":       "GET /a HTTP/1.1\nHost: x\n\n",
	"markdown":   "# Title\n\n- item *emph* `code`\n",
	"go":         "package main\n\nfunc main() { fmt := 1; _ = fmt }\n",
	"sql":        "SELECT a, count(*) FROM t WHERE b = 'x' GROUP BY a;\n",
	"dockerfile": "FROM node:24\nRUN npm ci\n",
	"diff":       "--- a\n+++ b\n@@ -1 +1 @@\n-a\n+b\n",
	"toml":       "[a]\nb = 1\n",
	"ini":        "[a]\nb=1\n",
	"xml":        "<a b=\"c\"><d/></a>\n",
	"css":        "a { color: #fff; }\n",
	"rust":       "fn main() { let x: u8 = 1; }\n",
	"java":       "class A { int f() { return 1; } }\n",
	"c":          "int main(void) { return 0; }\n",
	"ruby":       "def f; puts 'x'; end\n",
	"hcl":        "resource \"a\" \"b\" { c = 1 }\n",
	"makefile":   "all: build\n\tgo build\n",
	"powershell": "Get-ChildItem | Where-Object { $_.Length -gt 1 }\n",
	"kotlin":     "fun main() { val x = 1 }\n",
	"graphql":    "query { a { b } }\n",
	"scss":       "a { &:hover { color: red; } }\n",
}

// The curated set must highlight exactly like chroma's own lexers for the languages the notes use.
func TestSameTokensAsChroma(t *testing.T) {
	for lang, code := range samples {
		mine, theirs := Get(lang), lexers.Get(lang)
		if mine == nil {
			t.Errorf("%s: not in the curated set", lang)
			continue
		}
		if theirs == nil {
			t.Errorf("%s: chroma has no lexer; fix the sample", lang)
			continue
		}
		if a, b := tokens(t, mine, code), tokens(t, theirs, code); a != b {
			t.Errorf("%s: tokens differ\n mine:   %.300s\n chroma: %.300s", lang, a, b)
		}
	}
}

func TestUnknownLanguagesAreNil(t *testing.T) {
	for _, l := range []string{"", "nonsense", "mermaid", "cobol-9000"} {
		if Get(l) != nil {
			t.Errorf("%q should have no lexer", l)
		}
	}
}

func TestLexersAreParsedOnce(t *testing.T) {
	if Get("yaml") != Get("YAML") {
		t.Error("the same language must give the same cached lexer, whatever the case")
	}
}

package render

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	hl "github.com/migara0/docent/internal/highlight"
)

func escape(s string) string { return html.EscapeString(s) }

var titleAttr = regexp.MustCompile(`title="([^"]*)"`)

// tokenClass maps chroma's many token types onto the six colours of the design (tok-c, tok-k, tok-s, tok-n, tok-f, tok-t).
func tokenClass(t chroma.TokenType) string {
	switch {
	case t.InCategory(chroma.Comment):
		return "c"
	case t == chroma.KeywordType:
		return "t"
	case t.InCategory(chroma.Keyword):
		return "k"
	case t.InSubCategory(chroma.LiteralString):
		return "s"
	case t.InSubCategory(chroma.LiteralNumber):
		return "n"
	case t == chroma.NameFunction || t == chroma.NameFunctionMagic || t == chroma.NameBuiltin || t == chroma.NameDecorator || t == chroma.NameTag:
		return "f"
	case t == chroma.NameClass || t == chroma.NameNamespace || t == chroma.NameException || t == chroma.NameBuiltinPseudo:
		return "t"
	}
	return ""
}

// highlight returns the code as HTML, with a span per coloured token. Unknown languages and lexer errors fall back to plain escaped text.
func highlight(lang, code string) string {
	lexer := hl.Get(lang)
	if lang == "" || lexer == nil {
		return escape(code)
	}
	it, err := chroma.Coalesce(lexer).Tokenise(nil, code)
	if err != nil {
		return escape(code)
	}
	var b strings.Builder
	for tok := it(); tok != chroma.EOF; tok = it() {
		if c := tokenClass(tok.Type); c != "" {
			fmt.Fprintf(&b, `<span class="tok-%s">%s</span>`, c, escape(tok.Value))
		} else {
			b.WriteString(escape(tok.Value))
		}
	}
	return b.String()
}

// writeCode emits a code block, or a Mermaid diagram placeholder (drawn in the browser, only on pages that have one).
func writeCode(w util.BufWriter, lang, info, code string) {
	code = strings.TrimRight(code, "\n")
	if lang == "mermaid" {
		fmt.Fprintf(w, `<pre class="mermaid">%s</pre>`+"\n", escape(code))
		return
	}
	title := ""
	if m := titleAttr.FindStringSubmatch(info); m != nil {
		title = m[1]
	}
	fmt.Fprintf(w, `<div class="code" data-lang="%s"><div class="code__bar"><span class="name">%s</span><span class="lang">%s</span></div><pre><code>%s</code></pre></div>`+"\n",
		escape(lang), escape(title), escape(lang), highlight(lang, code))
}

type codeRenderer struct{}

func (codeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, func(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		fc := node.(*ast.FencedCodeBlock)
		info := ""
		if fc.Info != nil {
			info = string(fc.Info.Segment.Value(src))
		}
		var sb strings.Builder
		for i := 0; i < fc.Lines().Len(); i++ {
			s := fc.Lines().At(i)
			sb.Write(s.Value(src))
		}
		writeCode(w, strings.ToLower(string(fc.Language(src))), info, sb.String())
		return ast.WalkSkipChildren, nil
	})
	reg.Register(ast.KindCodeBlock, func(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		var sb strings.Builder
		for i := 0; i < node.Lines().Len(); i++ {
			s := node.Lines().At(i)
			sb.Write(s.Value(src))
		}
		writeCode(w, "", "", sb.String())
		return ast.WalkSkipChildren, nil
	})
}

type codeExtension struct{}

func (codeExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(codeRenderer{}, 500)))
}

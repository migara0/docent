package render

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// KindAdmonition is the AST kind of a callout block: !!! note "Title" (always open) or ??? note "Title" / ???+ (collapsible).
var KindAdmonition = ast.NewNodeKind("Admonition")

// Admonition is a callout. Children are the indented content.
type Admonition struct {
	ast.BaseBlock
	Label string // as written: note, warning, tip ...
	Title string
	Fold  bool // ??? : collapsible
	Open  bool // ???+ : collapsible, open at first
}

func (n *Admonition) Kind() ast.NodeKind { return KindAdmonition }
func (n *Admonition) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"Type": n.Label, "Title": n.Title}, nil)
}

// The title is everything between the first and the last quote on the line, so it may contain quotes of its own (\" is unescaped), as in Python-Markdown.
var admonitionLine = regexp.MustCompile(`^(!!!|\?\?\?\+?)[ \t]+([A-Za-z][\w-]*)(?:[ \t]+[\w-]+)*(?:[ \t]+"(.*)")?[ \t]*$`)

type admonitionParser struct{}

func (admonitionParser) Trigger() []byte { return []byte{'!', '?'} }

func (admonitionParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()
	if width, _ := util.IndentWidth(line, reader.LineOffset()); width > 3 {
		return nil, parser.NoChildren // four spaces make it a code block
	}
	m := admonitionLine.FindSubmatch(bytes.TrimSpace(line))
	if m == nil {
		return nil, parser.NoChildren
	}
	n := &Admonition{Label: strings.ToLower(string(m[2])), Fold: m[1][0] == '?', Open: string(m[1]) == "???+"}
	if len(m[3]) > 0 || bytes.Contains(bytes.TrimSpace(line), []byte(`""`)) {
		n.Title = strings.ReplaceAll(string(m[3]), `\"`, `"`) // an explicit empty title ("") means no title at all
	} else {
		n.Title = titleCase(n.Label)
	}
	reader.Advance(segment.Len() - 1)
	return n, parser.HasChildren
}

func (admonitionParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	line, _ := reader.PeekLine()
	if util.IsBlank(line) {
		return parser.Continue | parser.HasChildren
	}
	if width, _ := util.IndentWidth(line, reader.LineOffset()); width < 4 {
		return parser.Close
	}
	pos, padding := util.IndentPosition(line, reader.LineOffset(), 4)
	reader.AdvanceAndSetPadding(pos, padding)
	return parser.Continue | parser.HasChildren
}

func (admonitionParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {}
func (admonitionParser) CanInterruptParagraph() bool                                { return true }
func (admonitionParser) CanAcceptIndentedLine() bool                                { return false }

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// calloutStyle maps the many names used in the wild onto four looks: note, tip, warn, danger.
func calloutStyle(t string) (class, icon string) {
	switch t {
	case "tip", "hint", "success", "check", "done":
		return "tip", "bulb"
	case "warning", "warn", "caution", "attention", "important":
		return "warn", "alert"
	case "danger", "error", "failure", "fail", "bug", "missing":
		return "danger", "alert"
	default: // note, info, abstract, summary, example, quote, question, faq ...
		return "note", "info"
	}
}

type admonitionRenderer struct{}

func (admonitionRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindAdmonition, func(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		n := node.(*Admonition)
		class, icon := calloutStyle(n.Label)
		if entering {
			if n.Fold {
				open := ""
				if n.Open {
					open = " open"
				}
				fmt.Fprintf(w, `<details class="fold callout--%s"%s><summary><svg class="icon icon--sm chev"><use href="#i-chev"/></svg>%s</summary>`+"\n", class, open, escape(n.Title))
			} else {
				fmt.Fprintf(w, `<div class="callout callout--%s"><svg class="icon"><use href="#i-%s"/></svg>`+"\n", class, icon)
				if n.Title != "" {
					fmt.Fprintf(w, `<p class="callout__title">%s</p>`+"\n", escape(n.Title))
				}
			}
		} else if n.Fold {
			_, _ = w.WriteString("</details>\n")
		} else {
			_, _ = w.WriteString("</div>\n")
		}
		return ast.WalkContinue, nil
	})
}

type admonitionExtension struct{}

func (admonitionExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithBlockParsers(util.Prioritized(admonitionParser{}, 90)))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(admonitionRenderer{}, 500)))
}

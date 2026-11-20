package render

import (
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// headingRenderer adds a permalink anchor to h2 to h4 (the design shows it on hover).
type headingRenderer struct{}

func (headingRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, func(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		h := node.(*ast.Heading)
		if !entering {
			fmt.Fprintf(w, "</h%d>\n", h.Level)
			return ast.WalkContinue, nil
		}
		fmt.Fprintf(w, "<h%d", h.Level)
		html.RenderAttributes(w, node, html.HeadingAttributeFilter)
		_, _ = w.WriteString(">")
		if id, ok := node.AttributeString("id"); ok && h.Level >= 2 && h.Level <= 4 {
			fmt.Fprintf(w, `<a class="anchor" href="#%s" aria-hidden="true" tabindex="-1">#</a>`, escape(string(id.([]byte))))
		}
		return ast.WalkContinue, nil
	})
}

// imageRenderer implements the decision that images are out of scope: the alt text is shown with a marker, the file is never fetched.
type imageRenderer struct{}

func (imageRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindImage, func(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			_, _ = w.WriteString(`<span class="image-omitted" title="Images are not shown">`)
		} else {
			_, _ = w.WriteString("</span>")
		}
		return ast.WalkContinue, nil
	})
}

type nodesExtension struct{}

func (nodesExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(headingRenderer{}, 500),
		util.Prioritized(imageRenderer{}, 500),
	))
}

package render

import (
	"fmt"
	"regexp"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// IDLinker finds note IDs (such as DOC-0007) in running text. It leaves code, existing links and headings alone.
type IDLinker struct {
	Pattern *regexp.Regexp                               // matches an ID of any known prefix; it must have exactly one group around the ID
	Resolve func(id string) (url, title string, ok bool) // when set (and HTML is rendered), IDs that resolve become links
	Own     func(id string) bool                         // IDs for which an unresolved mention is a problem (the library's own prefix)
	Self    string                                       // the page's own ID, never linked to itself
}

type idHit struct {
	node       *ast.Text
	start, end int // offsets in the node's segment
	id         string
}

// processIDs collects the IDs mentioned in a page and, when rendering, turns the ones that resolve into links.
func processIDs(doc ast.Node, src []byte, l *IDLinker, render bool, res *Result) {
	if l == nil || l.Pattern == nil {
		return
	}
	var hits []idHit
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n.(type) {
		case *ast.Link, *ast.AutoLink, *ast.CodeSpan, *ast.Heading, *ast.FencedCodeBlock, *ast.CodeBlock, *ast.Image:
			return ast.WalkSkipChildren, nil
		}
		if t, ok := n.(*ast.Text); ok {
			seg := t.Segment.Value(src)
			for _, m := range l.Pattern.FindAllSubmatchIndex(seg, -1) {
				hits = append(hits, idHit{node: t, start: m[2], end: m[3], id: string(seg[m[2]:m[3]])})
			}
		}
		return ast.WalkContinue, nil
	})
	seen := map[string]bool{}
	for _, h := range hits {
		if h.id == l.Self {
			continue
		}
		if !seen[h.id] {
			seen[h.id] = true
			res.IDs = append(res.IDs, h.id)
		}
	}
	if !render || l.Resolve == nil {
		return
	}
	// group the hits by text node (a node can hold several), then replace each node by text and link pieces
	byNode := map[*ast.Text][]idHit{}
	var order []*ast.Text
	for _, h := range hits {
		if h.id == l.Self {
			continue
		}
		if _, ok := byNode[h.node]; !ok {
			order = append(order, h.node)
		}
		byNode[h.node] = append(byNode[h.node], h)
	}
	warned := map[string]bool{}
	for _, node := range order {
		var resolved []idHit
		for _, h := range byNode[node] {
			if _, _, ok := l.Resolve(h.id); ok {
				resolved = append(resolved, h)
			} else if l.Own != nil && l.Own(h.id) && !warned[h.id] {
				warned[h.id] = true
				res.Warnings = append(res.Warnings, fmt.Sprintf("note ID %s does not match any note", h.id))
			}
		}
		if len(resolved) > 0 {
			splitText(node, resolved, l)
		}
	}
}

// splitText replaces a text node by alternating text pieces and links.
func splitText(node *ast.Text, hits []idHit, l *IDLinker) {
	parent := node.Parent()
	if parent == nil {
		return
	}
	seg := node.Segment
	pos := 0 // offset in the segment already emitted
	piece := func(from, to int) {
		if to > from {
			parent.InsertBefore(parent, node, ast.NewTextSegment(text.NewSegment(seg.Start+from, seg.Start+to)))
		}
	}
	for _, h := range hits {
		piece(pos, h.start)
		url, title, _ := l.Resolve(h.id)
		link := ast.NewLink()
		link.Destination = []byte(url)
		link.Title = []byte(title)
		link.SetAttributeString("class", []byte("note-ref"))
		link.AppendChild(link, ast.NewTextSegment(text.NewSegment(seg.Start+h.start, seg.Start+h.end)))
		parent.InsertBefore(parent, node, link)
		pos = h.end
	}
	tail := ast.NewTextSegment(text.NewSegment(seg.Start+pos, seg.Stop))
	tail.SetSoftLineBreak(node.SoftLineBreak())
	tail.SetHardLineBreak(node.HardLineBreak())
	tail.SetRaw(node.IsRaw())
	parent.InsertBefore(parent, node, tail)
	parent.RemoveChild(parent, node)
}

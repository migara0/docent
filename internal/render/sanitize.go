package render

import (
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

// newPolicy is the last line of defence: whatever the renderers emit, only this vocabulary reaches the browser.
// Raw HTML from Markdown is already disabled by goldmark; this guards against mistakes in our own renderers.
func newPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "hr", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "li", "dl", "dt", "dd", "blockquote", "pre", "code",
		"em", "strong", "del", "sup", "sub", "table", "thead", "tbody", "tr", "th", "td", "div", "span", "details", "summary", "section", "figure", "figcaption", "svg")
	p.AllowAttrs("class", "id", "title", "role", "aria-hidden", "aria-label", "lang").Globally()
	p.AllowAttrs("tabindex").Matching(regexp.MustCompile(`^-1$`)).Globally() // only to take decoration (heading anchors) out of the tab order
	p.AllowAttrs("data-lang", "data-no-print").Matching(regexp.MustCompile(`^[\w .:-]*$`)).Globally()
	p.AllowDataAttributes()
	p.AllowAttrs("href").OnElements("a")
	p.AllowElements("a")
	p.AllowAttrs("rel").OnElements("a")
	p.AllowAttrs("align").OnElements("td", "th")
	p.AllowAttrs("start").OnElements("ol")
	p.AllowAttrs("open").OnElements("details")
	p.AllowElements("input")
	p.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	p.AllowAttrs("checked", "disabled").OnElements("input")
	p.AllowAttrs("viewbox").OnElements("svg")
	p.AllowElements("use")
	p.AllowAttrs("href").Matching(regexp.MustCompile(`^#i-[a-z-]+$`)).OnElements("use") // icons from the page's own sprite only
	p.AllowRelativeURLs(true)
	p.AllowURLSchemes("http", "https", "mailto", "tel")
	p.RequireNoFollowOnLinks(false)
	p.RequireNoReferrerOnLinks(false)
	return p
}

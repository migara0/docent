package devnotes

import (
	"strings"
	"testing"
)

func kinds(f []Finding) string {
	var out []string
	for _, x := range f {
		out = append(out, x.Level+":"+x.Message)
	}
	return strings.Join(out, " | ")
}

func TestLintFindsLeftoverTemplateText(t *testing.T) {
	text := "---\ntitle: x\n---\n\n# {{title}}\n\nTODO write this. See DOC-xxxx.\n\n<!-- 3. Each rule linked to its decision -->\n\n...\n- ...\n| ... | ... |\n\n## Empty\n\n## Has code\n\n```\n{{not_a_var}} TODO in a fence\n```\n\n## Has text\n\nreal\n\n## Kept <!-- lint: ok -->\n\n## Last\n"
	got := LintText("n.md", text, "DOC")
	want := []string{
		"error:unreplaced template variable {{title}}", "warning:TODO left in the text", "warning:placeholder ID DOC-xxxx",
		"warning:template instruction comment left in", "warning:placeholder line (...)", "warning:placeholder line (...)", "warning:placeholder line (...)",
		"warning:empty section: ## Empty", "warning:empty section: ## Last",
	}
	if kinds(got) != strings.Join(want, " | ") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.ReplaceAll(kinds(got), " | ", "\n"), strings.Join(want, "\n"))
	}
	if got[0].Line != 5 {
		t.Errorf("line numbers refer to the file, front matter included: %d", got[0].Line)
	}
}

func TestLintIgnoresCleanNotes(t *testing.T) {
	text := "---\ntitle: x\n---\n\n# Title\n\n## A\n\nText with `{{inline}}` in code and a [link](x.md).\n\n### Sub\n\nMore.\n"
	if got := LintText("n.md", text, "DOC"); len(got) != 0 {
		t.Errorf("a clean note: %s", kinds(got))
	}
	// a heading followed directly by a deeper heading is not empty
	if got := LintText("n.md", "# T\n\n## Parent\n\n### Child\n\ntext\n", "DOC"); len(got) != 0 {
		t.Errorf("a parent section that only holds sub-sections: %s", kinds(got))
	}
}

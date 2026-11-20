package library

import (
	"path"
	"sort"
	"strings"
)

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// naturalLess orders "note2" before "note10" and ignores case.
func naturalLess(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if isDigit(a[i]) && isDigit(b[j]) {
			si, sj := i, j
			for i < len(a) && isDigit(a[i]) {
				i++
			}
			for j < len(b) && isDigit(b[j]) {
				j++
			}
			na, nb := strings.TrimLeft(a[si:i], "0"), strings.TrimLeft(b[sj:j], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			continue
		}
		if a[i] != b[j] {
			return a[i] < b[j]
		}
		i++
		j++
	}
	return len(a)-i < len(b)-j
}

func isIndexName(name string) bool {
	n := strings.ToLower(strings.TrimSuffix(name, path.Ext(name)))
	return n == "index" || n == "readme"
}

// buildTree arranges pages into folders. In each folder: index/README first, then sub-folders, then pages, each in natural order.
func buildTree(rootName string, pages map[string]*Page, srcID string) (*Node, []string, int) {
	root := &Node{Name: rootName, Kind: "folder"}
	folders := map[string]*Node{"": root}
	paths := make([]string, 0, len(pages))
	for p := range pages {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	folderOf := func(dir string) *Node {
		if n, ok := folders[dir]; ok {
			return n
		}
		var parent *Node = root
		built := ""
		for _, seg := range strings.Split(dir, "/") {
			built = path.Join(built, seg)
			n, ok := folders[built]
			if !ok {
				n = &Node{Name: seg, Path: built, Kind: "folder"}
				parent.Children = append(parent.Children, n)
				folders[built] = n
			}
			parent = n
		}
		return parent
	}
	for _, p := range paths {
		pg := pages[p]
		dir := path.Dir(p)
		if dir == "." {
			dir = ""
		}
		f := folderOf(dir)
		f.Children = append(f.Children, &Node{Name: path.Base(p), Path: p, Title: pg.Title, ID: pg.Meta.ID, Kind: "page", URL: pageURL(srcID, p)})
	}
	var order []string
	var walk func(n *Node) int
	walk = func(n *Node) int {
		sort.SliceStable(n.Children, func(i, j int) bool {
			a, b := n.Children[i], n.Children[j]
			ra, rb := rank(a), rank(b)
			if ra != rb {
				return ra < rb
			}
			return naturalLess(a.Name, b.Name)
		})
		count := 0
		for _, c := range n.Children {
			if c.Kind == "page" {
				count++
				order = append(order, c.Path)
			} else {
				count += walk(c)
			}
		}
		n.Count = count
		return count
	}
	walk(root)
	return root, order, len(folders) - 1
}

func rank(n *Node) int {
	switch {
	case n.Kind == "page" && isIndexName(n.Name):
		return 0
	case n.Kind == "folder":
		return 1
	}
	return 2
}

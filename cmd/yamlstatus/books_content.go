package main

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type bookLinks struct {
	catalog  *bookCatalog
	source   *rulebookWorkspace
	bySource map[string]*bookEntry
	byOutput map[string]*bookEntry
	byBase   map[string][]*bookEntry
	baseURL  string
}

func newBookLinks(c *bookCatalog, source *rulebookWorkspace, baseURL string) *bookLinks {
	r := &bookLinks{catalog: c, source: source, bySource: map[string]*bookEntry{}, byOutput: map[string]*bookEntry{}, byBase: map[string][]*bookEntry{}, baseURL: baseURL}
	for _, book := range c.Books {
		r.bySource[book.Source] = book
		r.byOutput[book.Output] = book
		r.byBase[path.Base(book.Source)] = append(r.byBase[path.Base(book.Source)], book)
	}
	return r
}

func (r *bookLinks) href(from, to, fragment string) string {
	var target string
	if r.baseURL != "" {
		target = r.baseURL + to
	} else {
		// Both destinations are portable slash paths from the same site root.
		parts := strings.Split(path.Dir(from), "/")
		if parts[0] == "." {
			parts = nil
		}
		target = strings.Repeat("../", len(parts)) + to
	}
	u := &url.URL{Path: target, Fragment: fragment}
	if r.baseURL != "" {
		u, _ = url.Parse(target)
		u.Fragment = fragment
	}
	return u.String()
}

func (r *bookLinks) find(book *bookEntry, name string) *bookEntry {
	if path.Ext(name) == ".html" {
		if found := r.byOutput[path.Clean(path.Join(path.Dir(book.Output), name))]; found != nil {
			return found
		}
		if found := r.byOutput[strings.TrimPrefix(strings.TrimPrefix(name, "/"), "rules/")]; found != nil {
			return found
		}
	}
	candidate := path.Clean(path.Join(path.Dir(book.Source), name))
	if found := r.bySource[candidate]; found != nil {
		return found
	}
	direct := strings.TrimPrefix(name, "/")
	if !strings.HasPrefix(direct, "rules/") {
		direct = "rules/" + direct
	}
	if found := r.bySource[direct]; found != nil {
		return found
	}
	if !strings.Contains(name, "/") && len(r.byBase[name]) == 1 {
		return r.byBase[name][0]
	}
	return nil
}

func (r *bookLinks) diagnostic(book *bookEntry, message, reference string) {
	line := 0
	if bytes.Count(book.SourceText, []byte(reference)) == 1 {
		index := bytes.Index(book.SourceText, []byte(reference))
		line = 1 + bytes.Count(book.SourceText[:index], []byte("\n"))
	}
	r.catalog.Diagnostics = append(r.catalog.Diagnostics, bookDiagnostic{File: book.Source, Line: line, Message: message})
}

// rewriteBookLinks operates on rendered document structure; examples inside pre
// blocks are never interpreted as links. Selected books are resolved before
// resources; unselected repository documents remain explicitly unavailable.
func (r *bookLinks) rewrite(book *bookEntry) error {
	root, err := bookDOM(book.Body)
	if err != nil {
		return err
	}
	var rewriteErr error
	visitBookDOM(root, func(n *html.Node) {
		if n.Type != html.ElementNode || (n.Data != "a" && n.Data != "img") {
			return
		}
		key := "href"
		if n.Data == "img" {
			key = "src"
		}
		raw := bookAttr(n, key)
		if raw == "" {
			return
		}
		u, err := url.Parse(raw)
		if err != nil {
			r.diagnostic(book, "invalid URL "+raw, raw)
			return
		}
		if u.IsAbs() || u.Host != "" {
			return
		}
		if u.Path == "" {
			if u.Fragment != "" && !book.Anchors[u.Fragment] {
				r.diagnostic(book, "missing anchor #"+u.Fragment, raw)
			}
			return
		}
		if target := r.find(book, u.Path); target != nil {
			if u.Fragment != "" && !target.Anchors[u.Fragment] {
				r.diagnostic(book, fmt.Sprintf("missing anchor %s#%s", target.Source, u.Fragment), raw)
			}
			result := r.href(book.Output, target.Output, u.Fragment)
			if u.RawQuery != "" {
				parsed, _ := url.Parse(result)
				parsed.RawQuery = u.RawQuery
				result = parsed.String()
			}
			setBookAttr(n, key, result)
			return
		}
		local := path.Clean(path.Join(path.Dir(book.Source), u.Path))
		if strings.HasPrefix(u.Path, "rules/") {
			local = path.Clean(u.Path)
		}
		if local == "rules/README.md" || u.Path == "language-rulebook-status.md" {
			setBookAttr(n, key, r.href(book.Output, "index.html", ""))
			return
		}
		ext := strings.ToLower(path.Ext(u.Path))
		if n.Data == "img" {
			if !strings.HasPrefix(local, "rules/") {
				r.diagnostic(book, "resource outside rules root: "+raw, raw)
				return
			}
			if !strings.Contains("|.png|.jpg|.jpeg|.gif|.webp|.avif|.ico|", "|"+ext+"|") {
				r.diagnostic(book, "unsupported image format: "+raw, raw)
				return
			}
			data, err := r.source.ReadSource(local)
			if err != nil {
				r.diagnostic(book, "missing/unsafe image: "+raw, raw)
				return
			}
			output := "assets/" + strings.TrimPrefix(local, "rules/")
			if old, ok := r.catalog.Artifacts[output]; ok && !bytes.Equal(old, data) {
				rewriteErr = fmt.Errorf("resource destination collision: %s", output)
				return
			}
			r.catalog.Artifacts[output] = data
			setBookAttr(n, key, r.href(book.Output, output, ""))
			return
		}
		if ext == ".md" || ext == ".txt" {
			name := strings.TrimPrefix(local, "rules/")
			_, unavailable := r.catalog.Unavailable[name]
			if _, known := r.catalog.Unavailable[u.Path]; known {
				unavailable = true
			}
			for _, prefix := range []string{"rules/corrections/", "rules/mlir/packages/", "rules/mlir/dialect-versions/", "rules/mlir/lowering-versions/", "rules/mlir/normative-sync/", "rules/mlir/semantic-ir/"} {
				if strings.HasPrefix(local, prefix) {
					unavailable = true
				}
			}
			if !unavailable {
				r.diagnostic(book, "unresolved local book link: "+raw, raw)
			}
		}
		// Repository-only documents/code are not copied into the public site.
		n.Data = "span"
		n.DataAtom = atom.Span
		n.Attr = []html.Attribute{{Key: "title", Val: "Repository reference; not included in this rulebook site: " + u.Path}}
	})
	// Turn unambiguous book references in prose/inline code into links, while
	// leaving existing links, script-free code fences and unavailable names alone.
	var auto func(*html.Node)
	auto = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "a" || n.Data == "pre") {
			return
		}
		if n.Type == html.TextNode && n.Parent != nil {
			positions := bookReference.FindAllStringIndex(n.Data, -1)
			cursor := 0
			for _, pos := range positions {
				reference := n.Data[pos[0]:pos[1]]
				target := r.find(book, reference)
				if target == nil {
					continue
				}
				if pos[0] > cursor {
					n.Parent.InsertBefore(&html.Node{Type: html.TextNode, Data: n.Data[cursor:pos[0]]}, n)
				}
				link := &html.Node{Type: html.ElementNode, Data: "a", DataAtom: atom.A, Attr: []html.Attribute{{Key: "href", Val: r.href(book.Output, target.Output, "")}}}
				link.AppendChild(&html.Node{Type: html.TextNode, Data: reference})
				n.Parent.InsertBefore(link, n)
				cursor = pos[1]
			}
			if cursor > 0 {
				if cursor < len(n.Data) {
					n.Parent.InsertBefore(&html.Node{Type: html.TextNode, Data: n.Data[cursor:]}, n)
				}
				n.Parent.RemoveChild(n)
			}
			return
		}
		for child := n.FirstChild; child != nil; {
			next := child.NextSibling
			auto(child)
			child = next
		}
	}
	auto(root)
	book.Body = bookHTML(root)
	return rewriteErr
}

var bookReference = regexp.MustCompile(`(?:[\p{L}\p{N}_.-]+/)*[\p{L}\p{N}_-]+\.(?:md|txt)\b`)

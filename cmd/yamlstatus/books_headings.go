package main

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func bookDOM(body []byte) (*html.Node, error) {
	root := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(bytes.NewReader(body), root)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		root.AppendChild(node)
	}
	return root, nil
}

func bookHTML(root *html.Node) []byte {
	var output bytes.Buffer
	for node := root.FirstChild; node != nil; node = node.NextSibling {
		_ = html.Render(&output, node)
	}
	return output.Bytes()
}

func bookText(node *html.Node) string {
	if node.Type == html.TextNode {
		return node.Data
	}
	var out strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		out.WriteString(bookText(child))
	}
	return out.String()
}

func bookAttr(node *html.Node, key string) string {
	for _, a := range node.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func setBookAttr(node *html.Node, key, value string) {
	for i := range node.Attr {
		if node.Attr[i].Key == key {
			node.Attr[i].Val = value
			return
		}
	}
	node.Attr = append(node.Attr, html.Attribute{Key: key, Val: value})
}
func visitBookDOM(node *html.Node, fn func(*html.Node)) {
	fn(node)
	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		visitBookDOM(child, fn)
		child = next
	}
}

// bookSlug keeps Unicode letters/numbers/marks and source section numbering.
// IDs depend on heading text and duplicate order, never source line numbers.
func bookSlug(title string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || r == '-' || r == '_' {
			out.WriteRune(r)
		} else if unicode.IsSpace(r) {
			out.WriteByte('-')
		}
	}
	result := strings.Trim(out.String(), "-")
	if result == "" {
		return "section"
	}
	return result
}

var bookRevision = regexp.MustCompile(`(?im)(?:document revision|revision|document version)\s*[:：]\s*\*?\*?\s*([0-9]+(?:\.[0-9]+)+)`)

func prepareBook(book *bookEntry, source []byte) error {
	book.SourceText = source
	if path.Ext(book.Source) == ".txt" {
		for _, line := range strings.Split(string(source), "\n") {
			line = strings.TrimSpace(line)
			if strings.Trim(line, "=-_ ") != "" {
				book.Title = line
				break
			}
		}
	}
	body, err := renderRulebookBody(book.Source, source)
	if err != nil {
		return err
	}
	root, err := bookDOM(body)
	if err != nil {
		return err
	}
	book.Anchors = map[string]bool{"rulebook-content": true}
	var anchorErr error
	visitBookDOM(root, func(n *html.Node) {
		if id := bookAttr(n, "id"); id != "" {
			if book.Anchors[id] {
				anchorErr = fmt.Errorf("%s: duplicate/reserved anchor %q", book.Source, id)
			}
			book.Anchors[id] = true
		}
		if name := bookAttr(n, "name"); n.Data == "a" && name != "" && name != bookAttr(n, "id") {
			if book.Anchors[name] {
				anchorErr = fmt.Errorf("%s: duplicate anchor %q", book.Source, name)
			}
			book.Anchors[name] = true
		}
	})
	if anchorErr != nil {
		return anchorErr
	}
	visitBookDOM(root, func(n *html.Node) {
		if n.Type != html.ElementNode || len(n.Data) != 2 || n.Data[0] != 'h' || n.Data[1] < '1' || n.Data[1] > '6' {
			return
		}
		title := bookText(n)
		id := bookAttr(n, "id")
		if id == "" {
			base := bookSlug(title)
			id = base
			for i := 1; book.Anchors[id]; i++ {
				id = fmt.Sprintf("%s-%d", base, i)
			}
			setBookAttr(n, "id", id)
			book.Anchors[id] = true
		}
		if n.Data == "h1" && book.Title == "" {
			book.Title = title
			book.TitleID = id
		} else {
			book.Headings = append(book.Headings, rulebookPageLink{Title: title, Href: "#" + url.PathEscape(id)})
		}
	})
	if book.Title == "" {
		book.Title = strings.ReplaceAll(strings.TrimSuffix(path.Base(book.Source), path.Ext(book.Source)), "_", " ")
	}
	if m := bookRevision.FindSubmatch(source); len(m) > 1 {
		book.Revision = string(m[1])
	}
	// The page header supplies the first H1 and its original anchor.
	visitBookDOM(root, func(n *html.Node) {
		if n.Data == "h1" && bookAttr(n, "id") == book.TitleID && n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
	})
	book.Body = bookHTML(root)
	return nil
}

package main

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestRulebookPagePresentation(t *testing.T) {
	t.Parallel()
	options := rulebookPageOptions{
		Title: "Borrowing & lifetimes", Category: "Memory",
		Revision: "2.1", DocumentationStatus: "Written", SourcePath: "rules/memory/borrowing.md",
		IndexURL:    "../../index.html",
		Breadcrumbs: []rulebookPageLink{{Title: "Memory", Href: "../index.html"}},
		Contents:    []rulebookPageLink{{Title: "A rule", Href: "#a-rule"}},
		Navigation: []rulebookPageGroup{{Title: "Memory", Links: []rulebookPageLink{
			{Title: "Ownership", Href: "ownership.html"},
			{Title: "Borrowing", Href: "borrowing.html", Current: true},
		}}},
	}
	source := []byte("## A rule\n\n```sec\nfn A(x: ref bytes) void {}\n```\n\n| A | B |\n| - | - |\n| λ | & |\n")
	got, err := renderRulebookPage(options, "borrowing.md", source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(got, []byte("<!doctype html>")) {
		t.Fatal("missing HTML document declaration")
	}
	doc, err := html.Parse(bytes.NewReader(got))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			attrs := map[string]string{}
			for _, attr := range n.Attr {
				attrs[attr.Key] = attr.Val
			}
			switch n.Data {
			case "html":
				seen["language"] = attrs["lang"] == "en"
			case "meta":
				if attrs["name"] == "viewport" {
					seen["viewport"] = strings.Contains(attrs["content"], "width=device-width")
				}
			case "title":
				seen["title"] = n.FirstChild != nil && n.FirstChild.Data == "Borrowing & lifetimes — Sec Rulebooks"
			case "main":
				seen["skip-target"] = attrs["id"] == "rulebook-content" && attrs["tabindex"] == "-1"
			case "nav":
				seen[attrs["aria-label"]] = true
			case "a":
				if attrs["href"] == "#rulebook-content" {
					seen["skip-link"] = true
				}
				if attrs["href"] == "borrowing.html" && attrs["aria-current"] == "page" {
					seen["active-book"] = true
				}
			case "article", "table", "pre", "code", "style":
				seen[n.Data] = true
			case "link", "script", "img", "iframe":
				t.Errorf("page chrome requires unexpected external/script resource: %s", n.Data)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	for _, item := range []string{"language", "viewport", "title", "skip-target", "skip-link", "active-book", "Rulebook navigation", "Breadcrumb", "On this page", "article", "table", "pre", "code", "style"} {
		if !seen[item] {
			t.Errorf("missing accessible page feature %q", item)
		}
	}
	for _, text := range []string{"Rulebook revision", "2.1", "Documentation status", "Written", "rules/memory/borrowing.md", `href="../../index.html"`, `href="#a-rule"`, "not compiler implementation progress"} {
		if !bytes.Contains(got, []byte(text)) {
			t.Errorf("missing %q", text)
		}
	}
	again, err := renderRulebookPage(options, "borrowing.md", source)
	if err != nil || !bytes.Equal(got, again) {
		t.Fatalf("page output is not deterministic: %v", err)
	}
}

func TestRulebookPageEscapesMetadataAndBody(t *testing.T) {
	t.Parallel()
	attack := `<script>alert(1)</script>`
	got, err := renderRulebookPage(rulebookPageOptions{
		Title: attack, Category: attack, Revision: attack,
		DocumentationStatus: attack, SourcePath: attack,
		Navigation:  []rulebookPageGroup{{Title: attack, Links: []rulebookPageLink{{Title: attack, Href: "book.html"}}}},
		Contents:    []rulebookPageLink{{Title: attack, Href: "#rule"}},
		Breadcrumbs: []rulebookPageLink{{Title: attack, Href: "index.html"}},
	}, "book.md", []byte("<script>alert(1)</script>\n\n```sec\n<script>example</script>\n```"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("<script>")) || !bytes.Contains(got, []byte("&lt;script&gt;example&lt;/script&gt;")) ||
		!bytes.Contains(got, []byte("&lt;script&gt;alert(1)&lt;/script&gt;")) {
		t.Fatalf("metadata/code escaping or body sanitization failed: %s", got)
	}
}

func TestRulebookPageEmptyAndInvalidInputs(t *testing.T) {
	t.Parallel()
	got, err := renderRulebookPage(rulebookPageOptions{Title: "Empty"}, "empty.md", nil)
	if err != nil || !bytes.Contains(got, []byte(`href="index.html"`)) || bytes.Contains(got, []byte("<dt>")) || bytes.Contains(got, []byte(`aria-label="On this page"`)) {
		t.Fatalf("empty page defaults or optional metadata: %v, %s", err, got)
	}
	for _, opts := range []rulebookPageOptions{
		{}, {Title: " "}, {Title: "Bad URL", IndexURL: "javascript:alert(1)"},
		{Title: "Bad breadcrumb", Breadcrumbs: []rulebookPageLink{{Href: "file:///private"}}},
		{Title: "Bad TOC", Contents: []rulebookPageLink{{Href: "%wrong"}}},
		{Title: "Bad nav", Navigation: []rulebookPageGroup{{Links: []rulebookPageLink{{}}}}},
	} {
		if got, err := renderRulebookPage(opts, "book.md", nil); err == nil || got != nil {
			t.Errorf("invalid page options produced output: %s, %v", got, err)
		}
	}
	if got, err := renderRulebookPage(rulebookPageOptions{Title: "Bad encoding"}, "bad.md", []byte{0xff}); err == nil || got != nil {
		t.Fatalf("body error produced partial page: %s, %v", got, err)
	}
	got, err = renderRulebookPage(rulebookPageOptions{Title: "MLIR"}, "mlir.txt", []byte("MLIR\n====\n\n  %x < 10"))
	if err != nil || !bytes.Contains(got, []byte("<pre>MLIR\n====\n\n  %x &lt; 10</pre>")) {
		t.Fatalf("plain-text page: %v, %s", err, got)
	}
}

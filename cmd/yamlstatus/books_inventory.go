package main

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

type bookEntry struct {
	Source, Output, Status, Category, Title, Revision, TitleID string
	Body                                                       []byte
	SourceText                                                 []byte
	Headings                                                   []rulebookPageLink
	Anchors                                                    map[string]bool
}

type bookCategory struct {
	Key, Title, Description string
	Books                   []*bookEntry
}

type bookDiagnostic struct {
	File    string
	Line    int
	Message string
}

type bookCatalog struct {
	Books       []*bookEntry
	Categories  []*bookCategory
	Unavailable map[string]string
	Diagnostics []bookDiagnostic
	Artifacts   map[string][]byte
}

func bookNodeText(node ast.Node, source []byte) string {
	var result strings.Builder
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch n := n.(type) {
			case *ast.Text:
				result.Write(n.Segment.Value(source))
				if n.SoftLineBreak() || n.HardLineBreak() {
					result.WriteByte(' ')
				}
			case *ast.String:
				result.Write(n.Value)
			}
		}
		return ast.WalkContinue, nil
	})
	return result.String()
}

// bookTableRows parses actual GFM tables rather than splitting pipe characters
// in prose/code. Both inventory and directory-map inputs remain authoritative.
func bookTableRows(source []byte, visit func([]ast.Node)) {
	parser := goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser()
	doc := parser.Parse(text.NewReader(source))
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if _, ok := n.(*extast.TableRow); entering && ok {
			var cells []ast.Node
			for cell := n.FirstChild(); cell != nil; cell = cell.NextSibling() {
				cells = append(cells, cell)
			}
			visit(cells)
		}
		return ast.WalkContinue, nil
	})
}

func loadBookCatalog(source *rulebookWorkspace) (*bookCatalog, error) {
	inventory, err := source.ReadSource("language-rulebook-status.md")
	if err != nil {
		return nil, err
	}
	readme, err := source.ReadSource("rules/README.md")
	if err != nil {
		return nil, err
	}
	catalog := &bookCatalog{Unavailable: map[string]string{}, Artifacts: map[string][]byte{}}
	categories := map[string]*bookCategory{}
	bookTableRows(readme, func(cells []ast.Node) {
		if len(cells) != 2 {
			return
		}
		key := strings.TrimSuffix(bookNodeText(cells[0], readme), "/")
		if strings.Contains(key, "/") || key == "" {
			return
		}
		category := &bookCategory{Key: key, Title: strings.ReplaceAll(key, "-", " "), Description: bookNodeText(cells[1], readme)}
		category.Title = strings.ToUpper(category.Title[:1]) + category.Title[1:]
		if categories[key] == nil {
			categories[key] = category
			catalog.Categories = append(catalog.Categories, category)
		}
	})
	seen := map[string]bool{}
	var inventoryErr error
	bookTableRows(inventory, func(cells []ast.Node) {
		if len(cells) < 2 {
			return
		}
		status := strings.TrimSpace(bookNodeText(cells[1], inventory))
		ast.Walk(cells[0], func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if _, ok := n.(*ast.CodeSpan); !entering || !ok {
				return ast.WalkContinue, nil
			}
			name := strings.TrimPrefix(strings.TrimSpace(bookNodeText(n, inventory)), "rules/")
			ext := strings.ToLower(path.Ext(name))
			if ext != ".md" && ext != ".txt" {
				return ast.WalkSkipChildren, nil
			}
			if err := rulebookRelativePath(name); err != nil {
				inventoryErr = fmt.Errorf("inventory: %w", err)
				return ast.WalkStop, nil
			}
			if seen[name] {
				return ast.WalkSkipChildren, nil
			}
			seen[name] = true
			ready := strings.HasPrefix(status, "Written") || status == "Living"
			for _, excluded := range []string{"corrections/", "mlir/packages/", "mlir/dialect-versions/", "mlir/lowering-versions/", "mlir/semantic-ir/", "mlir/normative-sync/"} {
				if strings.HasPrefix(name, excluded) {
					ready = false
				}
			}
			if !ready || strings.Contains(strings.ToLower(status), "repository sync pending") {
				catalog.Unavailable[name] = status
				return ast.WalkSkipChildren, nil
			}
			key := strings.SplitN(name, "/", 2)[0]
			category := categories[key]
			if category == nil {
				inventoryErr = fmt.Errorf("inventory: %s has no rules/README.md directory-map category", name)
				return ast.WalkStop, nil
			}
			entry := &bookEntry{Source: "rules/" + name, Output: strings.TrimSuffix(name, path.Ext(name)) + ".html", Status: status, Category: category.Title}
			catalog.Books = append(catalog.Books, entry)
			category.Books = append(category.Books, entry)
			return ast.WalkSkipChildren, nil
		})
	})
	if inventoryErr != nil {
		return nil, inventoryErr
	}
	if len(catalog.Books) == 0 {
		return nil, fmt.Errorf("inventory contains no publishable rulebooks")
	}
	sort.Slice(catalog.Books, func(i, j int) bool { return catalog.Books[i].Source < catalog.Books[j].Source })
	for _, category := range catalog.Categories {
		sort.Slice(category.Books, func(i, j int) bool { return category.Books[i].Source < category.Books[j].Source })
	}
	for _, book := range catalog.Books {
		data, err := source.ReadSource(book.Source)
		if err != nil {
			return nil, err
		}
		if err := prepareBook(book, data); err != nil {
			return nil, err
		}
	}
	return catalog, nil
}

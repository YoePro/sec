package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"sort"
	"strings"
)

//go:embed rulebook_index.html
var bookIndexHTML string
var bookIndexTemplate = template.Must(template.New("index").Parse(bookIndexHTML))

type bookIndexGroup struct {
	Title, Description string
	Links              []rulebookPageLink
}

func buildBookSite(source *rulebookWorkspace, baseURL string) (*bookCatalog, error) {
	catalog, err := loadBookCatalog(source)
	if err != nil {
		return nil, err
	}
	links := newBookLinks(catalog, source, baseURL)
	for _, book := range catalog.Books {
		if err := links.rewrite(book); err != nil {
			return nil, err
		}
	}
	put := func(name string, data []byte) error {
		if _, exists := catalog.Artifacts[name]; exists {
			return fmt.Errorf("output destination collision: %s", name)
		}
		catalog.Artifacts[name] = data
		return nil
	}
	groups := func(from string, current *bookEntry) []rulebookPageGroup {
		var groups []rulebookPageGroup
		for _, category := range catalog.Categories {
			if len(category.Books) == 0 {
				continue
			}
			group := rulebookPageGroup{Title: category.Title}
			for _, book := range category.Books {
				group.Links = append(group.Links, rulebookPageLink{Title: book.Title, Href: links.href(from, book.Output, ""), Current: book == current})
			}
			groups = append(groups, group)
		}
		return groups
	}
	for _, book := range catalog.Books {
		options := rulebookPageOptions{Title: book.Title, TitleID: book.TitleID, Category: book.Category, Revision: book.Revision, DocumentationStatus: book.Status, SourcePath: book.Source,
			IndexURL: links.href(book.Output, "index.html", ""), Navigation: groups(book.Output, book), Contents: book.Headings,
			Breadcrumbs: []rulebookPageLink{{Title: book.Category, Href: links.href(book.Output, strings.SplitN(book.Output, "/", 2)[0]+"/index.html", "")}}}
		page, err := renderRulebookHTMLPage(options, book.Source, book.Body)
		if err != nil {
			return nil, err
		}
		if err := put(book.Output, page); err != nil {
			return nil, err
		}
	}
	makeIndex := func(output, title string, categories []*bookCategory, unavailable map[string]string) error {
		var indexGroups []bookIndexGroup
		for _, category := range categories {
			if len(category.Books) == 0 {
				continue
			}
			group := bookIndexGroup{Title: category.Title, Description: category.Description}
			for _, book := range category.Books {
				group.Links = append(group.Links, rulebookPageLink{Title: book.Title, Href: links.href(output, book.Output, "")})
			}
			indexGroups = append(indexGroups, group)
		}
		data := struct {
			Groups      []bookIndexGroup
			Unavailable map[string]string
		}{indexGroups, unavailable}
		var body bytes.Buffer
		if err := bookIndexTemplate.Execute(&body, data); err != nil {
			return err
		}
		page, err := renderRulebookHTMLPage(rulebookPageOptions{Title: title, IndexURL: links.href(output, "index.html", ""), Navigation: groups(output, nil)}, "index.md", body.Bytes())
		if err != nil {
			return err
		}
		return put(output, page)
	}
	if err := makeIndex("index.html", "Sec rulebooks", catalog.Categories, catalog.Unavailable); err != nil {
		return nil, err
	}
	for _, category := range catalog.Categories {
		if len(category.Books) > 0 {
			if err := makeIndex(category.Key+"/index.html", category.Title, []*bookCategory{category}, nil); err != nil {
				return nil, err
			}
		}
	}
	sort.Slice(catalog.Diagnostics, func(i, j int) bool {
		a, b := catalog.Diagnostics[i], catalog.Diagnostics[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Message < b.Message
	})
	return catalog, nil
}

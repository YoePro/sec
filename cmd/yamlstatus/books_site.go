package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"
)

//go:embed rulebook_index.html
var bookIndexHTML string
var bookIndexTemplate = template.Must(template.New("index").Parse(bookIndexHTML))

type bookIndexGroup struct {
	Title, Description string
	Links              []rulebookPageLink
}

// buildBookSite renders the site. A non-zero published time is shown in the
// title page footer; the output is deterministic for a given time.
func buildBookSite(source *rulebookWorkspace, baseURL string, published time.Time) (*bookCatalog, error) {
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
	makeIndex := func(output, title string, categories []*bookCategory, unavailable map[string]string, titlePage bool) error {
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
		options := rulebookPageOptions{Title: title, IndexURL: links.href(output, "index.html", ""), Navigation: groups(output, nil)}
		if titlePage && !published.IsZero() {
			utc := published.UTC()
			options.PublishedISO, options.PublishedDisplay = utc.Format(time.RFC3339), utc.Format("2006-01-02 15:04 UTC")
		}
		page, err := renderRulebookHTMLPage(options, "index.md", body.Bytes())
		if err != nil {
			return err
		}
		return put(output, page)
	}
	if err := makeIndex("index.html", "Sec rulebooks", catalog.Categories, catalog.Unavailable, true); err != nil {
		return nil, err
	}
	for _, category := range catalog.Categories {
		if len(category.Books) > 0 {
			if err := makeIndex(category.Key+"/index.html", category.Title, []*bookCategory{category}, nil, false); err != nil {
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

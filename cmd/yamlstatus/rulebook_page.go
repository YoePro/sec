package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"net/url"
	"strings"
)

// These types describe presentation inputs, not inventory or compiler facts.
// The site builder supplies selected books, resolved links and headings.
type rulebookPageLink struct {
	Title, Href string
	Current     bool
}

type rulebookPageGroup struct {
	Title string
	Links []rulebookPageLink
}

type rulebookPageOptions struct {
	Title, Category, Revision, DocumentationStatus, SourcePath string
	TitleID                                                    string
	IndexURL                                                   string
	Breadcrumbs, Contents                                      []rulebookPageLink
	Navigation                                                 []rulebookPageGroup
}

//go:embed rulebook_page.html
var rulebookPageHTML string

//go:embed rulebook_page.css
var rulebookPageCSS string

var rulebookPageTemplate = template.Must(template.New("rulebook").Parse(rulebookPageHTML))

// renderRulebookPage creates a complete offline-capable HTML page. Metadata is
// escaped by html/template; only this embedded stylesheet and the sanitized
// renderRulebookBody result enter trusted template types. No caller can inject
// a pre-rendered body. This is non-normative documentation infrastructure.
func renderRulebookPage(options rulebookPageOptions, name string, source []byte) ([]byte, error) {
	body, err := renderRulebookBody(name, source)
	if err != nil {
		return nil, err
	}
	return renderRulebookHTMLPage(options, name, body)
}

// Rewritten book bodies cross the same final sanitizer as raw Markdown.
func renderRulebookHTMLPage(options rulebookPageOptions, name string, body []byte) ([]byte, error) {
	if strings.TrimSpace(options.Title) == "" {
		return nil, fmt.Errorf("%s: rulebook page requires a title", name)
	}
	if options.IndexURL == "" {
		options.IndexURL = "index.html"
	}
	links := []rulebookPageLink{{Title: "Rulebooks", Href: options.IndexURL}}
	links = append(links, options.Breadcrumbs...)
	links = append(links, options.Contents...)
	for _, group := range options.Navigation {
		links = append(links, group.Links...)
	}
	for _, link := range links {
		u, err := url.Parse(link.Href)
		if err != nil || strings.TrimSpace(link.Href) == "" ||
			(u != nil && u.Scheme != "" && u.Scheme != "https" && u.Scheme != "http") {
			return nil, fmt.Errorf("%s: invalid page navigation URL %q", name, link.Href)
		}
	}
	data := struct {
		rulebookPageOptions
		Style template.CSS
		Body  template.HTML
	}{options, template.CSS(rulebookPageCSS), template.HTML(rulebookHTMLPolicy.SanitizeBytes(body))}
	var page bytes.Buffer
	if err := rulebookPageTemplate.Execute(&page, data); err != nil {
		return nil, fmt.Errorf("%s: render rulebook page: %w", name, err)
	}
	return page.Bytes(), nil
}

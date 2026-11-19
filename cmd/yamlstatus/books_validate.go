package main

import (
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

// validateBookArtifacts checks the actual final HTML, including templates,
// generated navigation and assets. No external URL is contacted.
func validateBookArtifacts(catalog *bookCatalog, baseURL string) {
	names := make([]string, 0, len(catalog.Artifacts))
	for name := range catalog.Artifacts {
		names = append(names, name)
	}
	sort.Strings(names)
	anchors := map[string]map[string]bool{}
	documents := map[string]*html.Node{}
	for _, name := range names {
		if path.Ext(name) != ".html" {
			continue
		}
		root, err := bookDOM(catalog.Artifacts[name])
		if err != nil {
			catalog.Diagnostics = append(catalog.Diagnostics, bookDiagnostic{File: name, Message: err.Error()})
			continue
		}
		documents[name] = root
		anchors[name] = map[string]bool{}
		visitBookDOM(root, func(n *html.Node) {
			for _, key := range []string{"id", "name"} {
				id := bookAttr(n, key)
				if id == "" || key == "name" && n.Data != "a" || key == "name" && id == bookAttr(n, "id") {
					continue
				}
				if anchors[name][id] {
					catalog.Diagnostics = append(catalog.Diagnostics, bookDiagnostic{File: name, Message: "duplicate anchor: " + id})
				}
				anchors[name][id] = true
			}
		})
	}
	for _, name := range names {
		root := documents[name]
		if root == nil {
			continue
		}
		visitBookDOM(root, func(n *html.Node) {
			key := ""
			if n.Data == "a" {
				key = "href"
			} else if n.Data == "img" {
				key = "src"
			}
			if key == "" {
				return
			}
			raw := bookAttr(n, key)
			if raw == "" {
				return
			}
			u, err := url.Parse(raw)
			if err != nil {
				catalog.Diagnostics = append(catalog.Diagnostics, bookDiagnostic{File: name, Message: "invalid output URL: " + raw})
				return
			}
			var destination string
			if baseURL != "" && strings.HasPrefix(raw, baseURL) {
				relative := strings.TrimPrefix(raw, baseURL)
				parsed, _ := url.Parse(relative)
				destination = parsed.Path
			} else if u.IsAbs() || u.Host != "" {
				return
			} else if u.Path == "" {
				destination = name
			} else {
				destination = path.Clean(path.Join(path.Dir(name), u.Path))
			}
			if _, exists := catalog.Artifacts[destination]; !exists {
				catalog.Diagnostics = append(catalog.Diagnostics, bookDiagnostic{File: name, Message: "missing output target: " + raw})
				return
			}
			if u.Fragment != "" && anchors[destination] != nil && !anchors[destination][u.Fragment] {
				catalog.Diagnostics = append(catalog.Diagnostics, bookDiagnostic{File: name, Message: fmt.Sprintf("missing output anchor: %s", raw)})
			}
		})
	}
	sort.Slice(catalog.Diagnostics, func(i, j int) bool {
		a, b := catalog.Diagnostics[i], catalog.Diagnostics[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Message < b.Message
	})
	unique := catalog.Diagnostics[:0]
	for _, diagnostic := range catalog.Diagnostics {
		if len(unique) == 0 || unique[len(unique)-1] != diagnostic {
			unique = append(unique, diagnostic)
		}
	}
	catalog.Diagnostics = unique
}

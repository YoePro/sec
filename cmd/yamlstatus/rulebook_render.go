package main

import (
	"bytes"
	"fmt"
	"html"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	goldhtml "github.com/yuin/goldmark/renderer/html"
)

// renderRulebookBody converts one in-memory rulebook into a sanitized HTML
// fragment. It performs no filesystem access, network requests, link rewriting
// or publication. See RULEBOOK_PUBLICATION.md for the supported source policy.
// This is documentation infrastructure, not Sec language semantics.
func renderRulebookBody(name string, source []byte) ([]byte, error) {
	if !utf8.Valid(source) {
		return nil, fmt.Errorf("%s: rulebook is not valid UTF-8", name)
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".txt":
		// The living MLIR text books mix prose, pseudo-Markdown and literal
		// diagrams. Preserve their content instead of guessing their syntax.
		return []byte("<pre>" + html.EscapeString(string(source)) + "</pre>\n"), nil
	case ".md":
		var rendered bytes.Buffer
		markdown := goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			// Raw markup is permitted only through the sanitizer below. This
			// retains harmless <br>, <details> and explicit anchor conventions.
			goldmark.WithRendererOptions(goldhtml.WithUnsafe()),
		)
		if err := markdown.Convert(source, &rendered); err != nil {
			return nil, fmt.Errorf("%s: render rulebook: %w", name, err)
		}
		return rulebookHTMLPolicy.SanitizeBytes(rendered.Bytes()), nil
	default:
		return nil, fmt.Errorf("%s: unsupported rulebook format (expected .md or .txt)", name)
	}
}

// rulebookHTMLPolicy is built once and never mutated while rendering. Only the
// renderer's final output crosses this allowlist; callers must keep page-level
// metadata in html/template and must not append untrusted HTML to the fragment.
var rulebookHTMLPolicy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^language-[a-zA-Z0-9_+.-]+$`)).OnElements("code")
	return p
}()

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lspserver "sec/internal/lsp/server"
)

// errorPatternFixture loads actual Sec sources and checks their baseline
// validity before testing editor recovery or semantic navigation.
// Rules: rules/tooling/lsp.md — "Recovery nodes", "Error patterns, inactive arms and `Some`".
func errorPatternFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/lsp/error_patterns/" + name + ".sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, diagnostic := range analyze("", source) {
		if diagnostic.Severity == 1 {
			t.Fatalf("invalid baseline: %+v", diagnostic)
		}
	}
	return source
}

// TestErrorPatternContextualCompletion uses only the Sema-selected channel,
// including the innermost handler and a heterogeneous protected call subtree.
// Rules: rules/errors/errorhandling.md — §§11,15.3,27.2b;
// rules/tooling/lsp.md — "Error patterns, inactive arms and `Some`".
func TestErrorPatternContextualCompletion(t *testing.T) {
	source := errorPatternFixture(t, "completion")
	for _, test := range []struct {
		marker, original, prefix string
		open                     bool
		want                     []string
	}{
		{"Err(.Denied) => 11", ".Denied", "", false, []string{"Bad", "Denied", "NotFound"}},
		{"Err(.Denied) => return 12", ".Denied", "De", false, []string{"Denied"}},
		{"Err(.Gone) => 16", ".Gone", "", false, []string{"Gone"}},
		{"Err(FileError.Denied) => 13", "FileError.Denied", "", true, nil},
		{"Err(FileError.Denied) => 13", "FileError.Denied", "De", true, nil},
		{"Err(FileError.Denied) => return 14", "FileError.Denied", "", true, nil},
		{"Err(FileError.Denied) => 15", "FileError.Denied", "", true, nil},
	} {
		t.Run(test.marker, func(t *testing.T) {
			start := strings.Index(source, test.marker) + len("Err(")
			text := source[:start] + "." + test.prefix + source[start+len(test.original):]
			offset := start + 1 + len(test.prefix)
			items := completeSource("", text, offset)
			if !test.open {
				if len(items) != len(test.want) {
					t.Fatalf("items=%+v; want %v", items, test.want)
				}
				for i, name := range test.want {
					if items[i].Label != name || items[i].TextEdit == nil || items[i].TextEdit.Range != offsetsRange(text, start+1, offset) || items[i].TextEdit.NewText != name {
						t.Fatalf("wrong variant completion: %+v", items[i])
					}
				}
			} else {
				found := false
				for _, item := range items {
					if item.Kind == 20 {
						t.Fatalf("open/heterogeneous channel guessed a variant: %+v", item)
					}
					if item.Label == "FileError" {
						found = true
						if item.TextEdit == nil || item.TextEdit.Range != offsetsRange(text, start, offset) || item.TextEdit.NewText != "FileError." || item.FilterText != "."+test.prefix {
							t.Fatalf("invalid qualification repair: %+v", item)
						}
					}
				}
				if !found {
					t.Fatalf("no concrete type suggestion: %+v", items)
				}
			}
		})
	}
}

// TestErrorPatternVariantAndBindingNavigation keeps contextual and qualified
// patterns attached to their real variant/payload declarations and bindings.
// Rules: rules/tooling/lsp.md — "Error patterns, inactive arms and `Some`";
// rules/errors/errorhandling.md — §§15.3,20.
func TestErrorPatternVariantAndBindingNavigation(t *testing.T) {
	source := errorPatternFixture(t, "patterns")
	path := filepath.Join(t.TempDir(), "patterns.sec")
	uri := uriFromPath(path)
	overlay := sourceOverlay{Sources: map[string]string{normalizedSourcePath(path): source}}
	for _, test := range []struct{ marker, selected, declaration, hover string }{
		{"Err(.NotFound { Code:", "NotFound", "NotFound { Path:", "FileError.NotFound { Path: string, Code: int }"},
		{"Err(.Bad(matchPosition))", "Bad", "Bad(int)", "FileError.Bad(int)"},
		{"Err(.Bad(matchPosition))", "matchPosition", "matchPosition))", "int"},
		{"Err(FileError.Bad(tryPosition))", "Bad", "Bad(int)", "FileError.Bad(int)"},
		{"Err(FileError.Bad(tryPosition))", "tryPosition", "tryPosition))", "int"},
		{"Err(FileError.Denied)", "Denied", "Denied\n", "FileError.Denied"},
		{"Err(.NotFound) => 0", "NotFound", "NotFound { Path:", "Path: string"},
		{"return matchCode", "matchCode", "matchCode, Path:", "int"},
		{"return matchPosition", "matchPosition", "matchPosition))", "int"},
		{"=> tryPosition", "tryPosition", "tryPosition))", "int"},
		{"=> Code", "Code", "Code })", "int"},
		{"return openPosition", "openPosition", "openPosition))", "int"},
		{"discard path", "path", "path })", "ref string"},
		{"Err(.Item(typedPayload))", "Item", "Item(T)", "TypedError[string].Item(string)"},
		{"discard typedPayload", "typedPayload", "typedPayload))", "string"},
	} {
		t.Run(test.marker, func(t *testing.T) {
			offset := strings.Index(source, test.marker) + strings.Index(test.marker, test.selected)
			hover, ok := hoverForSource(uri, source, offsetPosition(source, offset+1), overlay)
			if !ok || !strings.Contains(hover.Contents.Value, test.hover) {
				t.Fatalf("hover=%+v, found=%v, want %s", hover, ok, test.hover)
			}
			locations := definitionsForSource(uri, source, offsetPosition(source, offset+1), overlay)
			want := offsetPosition(source, strings.Index(source, test.declaration))
			if len(locations) != 1 || locations[0].Range.Start != want {
				t.Fatalf("definitions=%+v; want %+v", locations, want)
			}
			if test.selected == "tryPosition" {
				if references := referencesForSource(uri, source, offsetPosition(source, offset+1), true, overlay); len(references) != 2 {
					t.Fatalf("payload references lost their declaration identity: %+v", references)
				}
			}
		})
	}
}

// TestErrorPatternCompletionProtocol preserves the standard LSP textEdit and
// filterText needed to replace the contextual dot with a concrete type name.
// Rules: rules/tooling/lsp.md — "Completion", "Error patterns, inactive arms and `Some`".
func TestErrorPatternCompletionProtocol(t *testing.T) {
	source := errorPatternFixture(t, "completion")
	start := strings.Index(source, "Err(FileError.Denied) => 13") + len("Err(")
	text := source[:start] + ".De" + source[start+len("FileError.Denied"):]
	uri := "file:///error-pattern-completion.sec"
	s := &server{documentSnapshots: lspserver.NewDocuments()}
	s.documentSnapshots.Open(uri, 1, text)
	items := lspTestDecode[[]completionItem](t, lspTestRequest(t, s, "textDocument/completion", definitionParams{
		TextDocument: textDocumentIdentifier{URI: uri}, Position: offsetPosition(text, start+len(".De")),
	}))
	for _, item := range items {
		if item.Label == "FileError" {
			if item.TextEdit == nil || item.TextEdit.Range != offsetsRange(text, start, start+len(".De")) || item.TextEdit.NewText != "FileError." || item.FilterText != ".De" {
				t.Fatalf("protocol dropped qualification edit: %+v", item)
			}
			return
		}
	}
	t.Fatalf("no concrete type repair over JSON-RPC: %+v", items)
}

// TestErrorPatternCompletionRecoveryAndGuards covers an unfinished handler,
// unresolved channels and lexical/call positions that must supply no context.
// Rules: rules/tooling/lsp.md — "Recovery nodes", "Error patterns, inactive arms and `Some`".
func TestErrorPatternCompletionRecoveryAndGuards(t *testing.T) {
	source := errorPatternFixture(t, "completion")
	marker := "Err(.Denied) => 11"
	start := strings.Index(source, marker)
	incomplete := source[:start] + "Err(." + source[start+len(marker):]
	items := completeSource("", incomplete, start+len("Err(."))
	if len(items) != 3 {
		t.Fatalf("unfinished handler lost channel: %+v", items)
	}
	unknown := strings.Replace(incomplete, "try Read()", "try Missing()", 1)
	offset := strings.Index(unknown, "Err(.") + len("Err(.")
	if items := completeSource("", unknown, offset); len(items) != 0 {
		t.Fatalf("unresolved channel guessed variants: %+v", items)
	}
	for _, marker := range []string{"\"Err(.", "// Err(."} {
		offset := strings.Index(source, marker) + len(marker)
		if _, recognized := errorPatternCompletionItems("", source, offset, sourceOverlay{}); recognized {
			t.Fatalf("lexical content mistaken for an error pattern: %s", marker)
		}
	}
	start = strings.Index(source, "Err(FileError.Denied)\n") + len("Err(")
	ordinary := source[:start] + "." + source[start+len("FileError.Denied"):]
	if items := completeSource("", ordinary, start+1); len(items) != 0 {
		t.Fatalf("ordinary Err construction supplied pattern context: %+v", items)
	}
}

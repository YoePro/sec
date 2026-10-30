package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStringEnumNavigationAndTokens keeps alias declaration identity distinct
// from equal runtime values across navigation, references and semantic tokens.
// Rules: rules/declarations/enums.md — §§7,18;
// rules/tooling/lsp.md — "Go to definition", "References", "Semantic tokens".
func TestStringEnumNavigationAndTokens(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/string_enum_surfaces/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	path := filepath.Join(t.TempDir(), "main.sec")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(path)
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	tokens := decodeSemanticTokens(semanticTokensForSource(uri, source))
	for _, test := range []struct{ marker, selected, declaration, kind string }{
		{"Defaulted() Status", "Status", "Status string", "enum"},
		{"Status.Alias", "Alias", "Alias          =", "enumMember"},
		{"Status.Active", "Active", "Active default", "enumMember"},
		{"Status.Done", "Done", "Done           =", "enumMember"},
	} {
		offset := strings.Index(source, test.marker) + strings.Index(test.marker, test.selected)
		pos := offsetPosition(source, offset)
		definitions := definitionsForSource(uri, source, pos)
		want := offsetPosition(source, strings.Index(source, test.declaration))
		if len(definitions) != 1 || definitions[0].URI != uri || definitions[0].Range.Start != want {
			t.Fatalf("%s definitions=%+v want=%+v", test.marker, definitions, want)
		}
		assertSemanticToken(t, tokens, pos.Line, pos.Character, len(test.selected), test.kind)
	}
	// An alias's equal value must not merge its references into Active.
	alias := offsetPosition(source, strings.Index(source, "Alias          ="))
	refs := referencesForSource(uri, source, alias, true)
	if len(refs) != 2 {
		t.Fatalf("alias references=%+v", refs)
	}
	for _, ref := range refs {
		at := lineCharToOffset(source, ref.Range.Start.Line, ref.Range.Start.Character)
		if ref.URI != uri || !strings.HasPrefix(source[at:], "Alias") {
			t.Fatal("alias reference changed identity", ref)
		}
	}
	if refs := referencesForSource(uri, source, alias, false); len(refs) != 1 {
		t.Fatalf("alias use references=%+v", refs)
	}
}

// TestStringEnumDocumentSymbols includes every member inside the enum's
// complete declaration range while selecting its exact physical identifier.
// Rules: rules/declarations/enums.md — §18;
// rules/tooling/lsp.md — "Document symbols".
func TestStringEnumDocumentSymbols(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/string_enum_surfaces/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	symbols := documentSymbolsForSource("", source)
	found := false
	for _, symbol := range symbols {
		if symbol.Name != "Status" {
			continue
		}
		found = true
		if symbol.Kind != 10 || symbol.SelectionRange.Start != offsetPosition(source, strings.Index(source, "Status string")) {
			t.Fatal(symbol)
		}
		assertDocumentSymbolNames(t, symbol.Children, []string{"Pending", "Active", "Alias", "Done"})
		for _, child := range symbol.Children {
			if child.Name == "Active" {
				at := strings.Index(source, "Active default")
				end := at + len("Active default = \"å🙂\"")
				if child.Range.End != offsetPosition(source, end) {
					t.Fatalf("member range lost default/UTF-16 initializer: %+v", child)
				}
			}
			if child.Kind != 22 || !positionInRange(child.SelectionRange.Start, symbol.Range) {
				t.Fatalf("enum range excludes member: %+v parent=%+v", child, symbol)
			}
		}
		wantEnd := offsetPosition(source, strings.Index(source, "}\n")+1)
		if symbol.Range.End != wantEnd {
			t.Fatalf("enum range end=%+v want=%+v", symbol.Range.End, wantEnd)
		}
	}
	if !found {
		t.Fatal("missing enum outline")
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Static and instance member presentation uses compiler declaration identity,
// including equal spellings on different owners and extension members. All four
// LSP surfaces are tested on source that passes Sema.
// Rules: rules/declarations/static.md — §§6,8–9,26;
// rules/tooling/lsp.md — "Hover", "Navigation and references", "Document symbols", "Semantic tokens".
func TestStaticMemberTooling(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/static_members/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	path := filepath.Join(t.TempDir(), "main.sec")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(path)
	for _, d := range analyze(uri, text) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	cases := []struct {
		use, name, kind, hover string
		static                 bool
	}{
		{"Counter.Limit", "Limit", "variable", "static let Counter.Limit: int", true},
		{"Counter.Total", "Total", "variable", "static let mut Counter.Total: int", true},
		{"Counter.Global", "Global", "property", "static property Global: int", true},
		{"Counter.Create", "Create", "method", "static fn Counter.Create", true},
		{"counter.Value", "Value", "property", "property Value: int", false},
		{"counter.Read", "Read", "method", "fn Counter.Read", false},
		{"Counter.Extra", "Extra", "variable", "static let Counter.Extra: int", true},
		{"counter.More", "More", "method", "fn Counter.More", false},
		{"other.Create", "Create", "method", "fn Other.Create", false},
		{"other.Global", "Global", "property", "property Global: int", false},
		{"other.Limit", "Limit", "property", "Limit: int", false},
		{"counter.Changed", "Changed", "event", "event Changed", false},
	}
	tokens := decodeSemanticTokens(semanticTokensForSource(uri, text))
	for _, offset := range []int{strings.Index(text, "Local: int"), strings.LastIndex(text, "Local")} {
		pos := offsetPosition(text, offset)
		assertSemanticTokenWithModifier(t, tokens, pos.Line, pos.Character, len("Local"), "variable", "static")
		assertSemanticTokenWithModifier(t, tokens, pos.Line, pos.Character, len("Local"), "variable", "readonly")
	}
	for _, c := range cases {
		t.Run(c.use, func(t *testing.T) {
			offset := strings.Index(text, c.use) + len(c.use) - len(c.name)
			pos := offsetPosition(text, offset)
			hover, ok := hoverForSource(uri, text, pos)
			if !ok || !strings.Contains(hover.Contents.Value, c.hover) {
				t.Fatalf("hover=%v %q want %q", ok, hover.Contents.Value, c.hover)
			}
			if c.use == "Counter.Create" && !strings.Contains(hover.Contents.Value, "Creates a counter.") {
				t.Fatal("lost method documentation", hover.Contents.Value)
			}
			if !c.static && strings.Contains(hover.Contents.Value, "static ") {
				t.Fatal(hover.Contents.Value)
			}
			if c.static {
				assertSemanticTokenWithModifier(t, tokens, pos.Line, pos.Character, len(c.name), c.kind, "static")
			} else {
				assertSemanticTokenWithoutModifier(t, tokens, pos.Line, pos.Character, len(c.name), c.kind, "static")
			}
			references := referencesForSource(uri, text, pos, true)
			if len(references) != 2 {
				t.Fatalf("references=%+v", references)
			}
			foundUse := false
			for _, ref := range references {
				if ref.URI != uri {
					t.Fatalf("wrong owner file: %+v", ref)
				}
				if ref.Range.Start == pos {
					foundUse = true
				}
				start := lineCharToOffset(text, ref.Range.Start.Line, ref.Range.Start.Character)
				end := lineCharToOffset(text, ref.Range.End.Line, ref.Range.End.Character)
				if text[start:end] != c.name {
					t.Fatalf("wrong reference: %+v", ref)
				}
				presented, ok := hoverForSource(uri, text, ref.Range.Start)
				if !ok || !strings.Contains(presented.Contents.Value, c.hover) {
					t.Fatalf("declaration/use hover: %v %q", ok, presented.Contents.Value)
				}

				if c.static {
					assertSemanticTokenWithModifier(t, tokens, ref.Range.Start.Line, ref.Range.Start.Character, len(c.name), c.kind, "static")
				} else {
					assertSemanticTokenWithoutModifier(t, tokens, ref.Range.Start.Line, ref.Range.Start.Character, len(c.name), c.kind, "static")
				}
			}
			if !foundUse {
				t.Fatal("missing use reference")
			}
		})
	}
	symbols := documentSymbolsForSource(uri, text)
	assertDocumentSymbolRangesContainSelections(t, symbols)
	for _, parent := range symbols {
		if parent.Name != "impl Counter" && parent.Name != "impl extends Counter" && parent.Name != "impl Other" {
			continue
		}
		for _, child := range parent.Children {
			static := parent.Name != "impl Other" && (child.Name == "Limit" || child.Name == "Total" || child.Name == "Global" || child.Name == "Create" || child.Name == "Extra")
			if strings.HasPrefix(child.Detail, "static ") != static {
				t.Fatalf("outline %s.%s: %q", parent.Name, child.Name, child.Detail)
			}
		}
	}
}

// Unsupported per-instance storage remains an instance declaration in tooling;
// presentation must not turn it into accepted static storage.
// Rules: rules/declarations/static.md — §6; rules/tooling/lsp.md — "Semantic tokens", "Document symbols".
func TestStaticMemberToolingUnsupportedInstance(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/static_members/instance_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	values := analyze(uri, text)
	found := false
	for _, d := range values {
		if d.Severity == 1 && strings.Contains(d.Message, "per-instance storage and initialization") {
			found = true
		}
	}
	if !found {
		t.Fatal(values)
	}
	tokens := decodeSemanticTokens(semanticTokensForSource(uri, text))
	offset := strings.Index(text, "Instance: int")
	pos := offsetPosition(text, offset)
	assertSemanticTokenWithoutModifier(t, tokens, pos.Line, pos.Character, len("Instance"), "variable", "static")
	symbols := documentSymbolsForSource(uri, text)
	for _, parent := range symbols {
		if parent.Name == "impl Incomplete" {
			if len(parent.Children) != 2 {
				t.Fatal(parent)
			}
			for _, child := range parent.Children {
				if strings.HasPrefix(child.Detail, "static ") != (child.Name == "Shared") {
					t.Fatal(child)
				}
			}
		}
	}
}

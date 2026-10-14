package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func scopedBindingFixture(t *testing.T, name string) (string, string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../../testdata/lsp/binding_type_hints", name))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return uriFromPath(path), string(data)
}

// Inferred scoped-binding types retain each declaration's concrete Sema type,
// including borrow authority, repeated names and struct-like whole payloads.
// Rules: rules/tooling/lsp.md — "Inlay hints";
// rules/control-flow/flowcontrol_for.md — §§4,9,10;
// rules/control-flow/flowcontrol_match.md — §§9–12;
// rules/control-flow/flowcontrol_if.md — §13; rules/errors/errorhandling.md — §§15,20.
func TestInlayHintsScopedBindingTypes(t *testing.T) {
	uri, text := scopedBindingFixture(t, "valid.sec")
	if reported := analyze(uri, text); len(reported) != 0 {
		t.Fatal(reported)
	}
	hints := inlayHintsForSource(uri, text, allLines(), inlayHintSettings{Types: true})
	wants := map[string]string{
		"for index": "int", "ref mut item": "ref mut int", "ref item": "ref int",
		"ref key": "ref string", "ref mut entry": "ref mut int", "for character": "rune", "for item": "int",
		"Item(payload": "int", "Meta(meta": "Packet.Meta",
		"Item(ref borrowed": "ref int", "Item(ref mut mutable": "ref mut int", "Some(found": "int",
		"Err(reason)": "ReadError",
	}
	for source, typ := range wants {
		end := strings.Index(text, source) + len(source)
		if source == "Err(reason)" {
			end--
		}
		want := offsetPosition(text, end)
		matches := 0
		for _, hint := range hints {
			if hint.Position == want {
				if hint.Kind != inlayHintKindType || hint.Label != ": "+typ {
					t.Fatalf("%s: %+v, want %s", source, hint, typ)
				}
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("%s has %d hints; all hints %+v", source, matches, hints)
		}
	}
	if len(hints) != len(wants) {
		t.Fatalf("unexpected hints: %+v", hints)
	}
	if hints := inlayHintsForSource(uri, text, allLines(), inlayHintSettings{}); len(hints) != 0 {
		t.Fatal(hints)
	}
	for _, hint := range hints {
		at := lspRange{Start: hint.Position, End: hint.Position}
		only := inlayHintsForSource(uri, text, at, inlayHintSettings{Types: true})
		if len(only) != 1 || only[0] != hint {
			t.Fatal(only, hint)
		}
	}
}

// Failed loop/presence analysis publishes no inferred binding type hint.
// Rules: rules/tooling/lsp.md — "Inlay hints", "Unknown facts".
func TestInlayHintsScopedBindingsRejectInvalidFacts(t *testing.T) {
	uri, text := scopedBindingFixture(t, "bindings_invalid.sec")
	if reported := analyze(uri, text); len(reported) == 0 {
		t.Fatal("invalid fixture accepted")
	}
	if hints := inlayHintsForSource(uri, text, allLines(), inlayHintSettings{Types: true}); len(hints) != 0 {
		t.Fatal(hints)
	}
}

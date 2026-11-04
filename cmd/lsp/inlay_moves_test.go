package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	lspserver "sec/internal/lsp/server"
)

func moveHintFixture(t *testing.T, name string) (string, string) {
	t.Helper()
	data, err := os.ReadFile("../../testdata/lsp/move_hints/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return uriFromPath(filepath.Join(t.TempDir(), "main.sec")), string(data)
}

// Move hints must reflect accepted semantic transfers, never source-name
// guesses. All construction forms, partial sources, explicit forms, ordinary
// methods and temporaries retain their distinct ownership behavior.
// Rules: copy_move.md §§9–10,18; rules/tooling/lsp.md — "Inlay hints".
func TestInlayMovesUseAcceptedTransfers(t *testing.T) {
	uri, text := moveHintFixture(t, "valid.sec")
	for _, diagnostic := range analyze(uri, text) {
		if diagnostic.Severity == 1 {
			t.Fatalf("invalid fixture: %+v", diagnostic)
		}
	}
	hints := inlayHintsForSource(uri, text, allLines(), inlayHintSettings{Ownership: true})
	want := map[string]string{
		"Direct": "return value", "Nested": "Item: value", "Optional": "Some(value",
		"Fallible": "Ok(value", "Tagged": "Choice.Item(value", "Array": "[value",
		"Partial": "return value.Item", "Success": "return value.Ok", "Failure": "return value.Err",
		"Projected": "return value.Item.Ok",
	}
	seen := map[string]bool{}
	for _, hint := range hints {
		if hint.Label != "move" || hint.Kind != 0 || !hint.PaddingRight {
			t.Fatalf("unexpected ownership hint: %+v", hint)
		}
		lines := strings.Split(text, "\n")
		line := lines[hint.Position.Line]
		name := strings.Fields(strings.TrimPrefix(line, "fn "))[0]
		name = strings.Split(name, "(")[0]
		fragment, ok := want[name]
		if !ok || seen[name] {
			t.Fatalf("extra move hint for %s: %+v", name, hint)
		}
		start := strings.Index(line, fragment) + strings.Index(fragment, "value")
		if hint.Position.Character != start {
			t.Fatalf("%s move at %+v; want source column %d", name, hint.Position, start)
		}
		seen[name] = true
	}
	if len(seen) != len(want) {
		t.Fatalf("got %v; expected %v", seen, want)
	}
	if hints := inlayHintsForSource(uri, text, allLines(), inlayHintSettings{}); len(hints) != 0 {
		t.Fatal("disabled hints returned", hints)
	}
	for _, hint := range hints {
		requested := lspRange{Start: hint.Position, End: hint.Position}
		if got := inlayHintsForSource(uri, text, requested, inlayHintSettings{Ownership: true}); len(got) != 1 || got[0].Position != hint.Position {
			t.Fatalf("range filter lost exact source: %+v", got)
		}
	}
}

// Rejected receiver consumption must never be presented as an accepted move.
// The first successful consumption remains valid even if a later use fails.
// Rules: errorhandling.md §§6.1,29; lsp.md — "Inlay hints".
func TestInlayMovesRejectInvalidReceivers(t *testing.T) {
	uri, text := moveHintFixture(t, "invalid.sec")
	if diagnostics := analyze(uri, text); len(diagnostics) == 0 {
		t.Fatal("fixture unexpectedly valid")
	}
	hints := inlayHintsForSource(uri, text, allLines(), inlayHintSettings{Ownership: true})
	start := strings.Index(text, "fn Twice")
	if len(hints) != 1 || hints[0].Label != "move" || hints[0].Position != offsetPosition(text, start+strings.Index(text[start:], "value.Ok()")) {
		t.Fatalf("invalid receiver got a move hint: %+v", hints)
	}
}

// Real framed requests preserve UTF-16 positions, listing/resolve identity,
// tooltip meaning, and stale snapshot validation for the new ownership hint.
// Rules: rules/tooling/lsp.md — "Inlay hints", "Inlay hint resolve".
func TestInlayMovesProtocolResolve(t *testing.T) {
	uri, text := moveHintFixture(t, "valid.sec")
	text = strings.Replace(text, "return value.Ok()", "/* 😀 */ return value.Ok()", 1)
	s := &server{documentSnapshots: lspserver.NewDocuments()}
	s.documentSnapshots.Open(uri, 1, text)
	s.inlayHints = &inlayHintSettings{Ownership: true}
	hints := listResolvableInlayHints(t, s, uri)
	count := 0
	wantUnicode := offsetPosition(text, strings.Index(text, "/* 😀 */ return value.Ok()")+len("/* 😀 */ return "))
	unicodeFound := false
	for _, hint := range hints {
		if hint.Label != "move" {
			continue
		}
		count++
		if hint.Position == wantUnicode {
			unicodeFound = true
		}
		resolved := lspTestDecode[inlayHint](t, lspTestRequest(t, s, "inlayHint/resolve", hint))
		if resolved.Position != hint.Position || resolved.Label != hint.Label || resolved.Tooltip == nil ||
			!strings.Contains(resolved.Tooltip.Value, "Ownership transfers") || strings.Contains(resolved.Tooltip.Value, "defaults") {
			t.Fatalf("wrong resolve result: %+v", resolved)
		}
		if again := lspTestDecode[inlayHint](t, lspTestRequest(t, s, "inlayHint/resolve", resolved)); !reflect.DeepEqual(again, resolved) {
			t.Fatal("unstable resolve", again)
		}
	}
	if count != 10 || !unicodeFound {
		t.Fatalf("got %d move hints; UTF-16 source position found=%t", count, unicodeFound)
	}
	s.documentSnapshots.Change(uri, 2, "\n"+text)
	if response := lspTestRequest(t, s, "inlayHint/resolve", hints[0]); response.Error == nil || response.Error.Code != -32801 {
		t.Fatalf("stale move hint resolved: %+v", response)
	}
}

// Sibling source facts may participate in analysis, but their move sites must
// never be returned at coincident positions in the active document.
// Rules: rules/tooling/lsp.md — "Inlay hints" active-file and snapshot filters.
func TestInlayMovesActiveDocumentAndCRLF(t *testing.T) {
	uri, text := moveHintFixture(t, "valid.sec")
	text = strings.ReplaceAll(text, "\n", "\r\n")
	sibling := filepath.Join(filepath.Dir(pathFromURI(uri)), "sibling.sec")
	_, other := moveHintFixture(t, "sibling.sec")
	overlay := sourceOverlay{Sources: map[string]string{normalizedSourcePath(sibling): other}}
	hints := inlayHintsForSource(uri, text, allLines(), inlayHintSettings{Ownership: true}, overlay)
	if len(hints) != 10 {
		t.Fatalf("unexpected active-file moves: %+v", hints)
	}
	for _, hint := range hints {
		if hint.Position.Line < 10 || hint.Label != "move" {
			t.Fatalf("sibling hint leaked into active file: %+v", hint)
		}
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	lspserver "sec/internal/lsp/server"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Move actions consume actual compiler diagnostics and validate the complete
// edited body: later use, deferred use, live borrows and custom-free replacement
// remain governed by the same Sema analysis as compilation.
// Rules: rules/memory/copy_move.md — §29(4);
// rules/memory/borrowing.md — §§21–22; rules/memory/destruction.md — §33.
func TestOwnershipCodeActionsRequireProof(t *testing.T) {
	for _, test := range []struct {
		name string
		want int
	}{
		{"declaration_invalid", 1}, {"assignment_invalid", 1},
		{"later_use_invalid", 0}, {"deferred_use_invalid", 0}, {"live_borrow_invalid", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, err := os.ReadFile("../../testdata/tooling/move_actions/" + test.name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
			current := analyze(uri, text)
			found := false
			for _, d := range current {
				found = found || d.Code == diagnostics.ImplicitMoveDisallowed
			}
			if !found {
				t.Fatal("fixture did not exercise the implicit-move repair", current)
			}
			actions := ownershipCodeActions(uri, text, current)
			if len(actions) != test.want {
				t.Fatalf("actions=%+v; diagnostics=%+v", actions, current)
			}
			for _, action := range actions {
				fixed := applyTextEdits(text, action.Edit.Changes[uri])
				errors, complete := ownershipActionErrors(uri, fixed, sourceOverlay{})
				if !complete || len(errors) != 0 {
					t.Fatal("offered unproved move", fixed, errors)
				}
				if action.Kind != "quickfix" || len(action.Diagnostics) != 1 {
					t.Fatal(action)
				}
			}
			if test.want == 0 {
				for _, d := range current {
					if d.Code == diagnostics.ImplicitMoveDisallowed {
						edit, _, ok := ownershipMoveSyntaxEdit(text, d)
						if !ok {
							continue
						}
						errors, complete := ownershipActionErrors(uri, applyTextEdits(text, []textEdit{edit}), sourceOverlay{})
						if !complete || len(errors) == 0 {
							t.Fatal("negative fixture must demonstrate a semantic move failure", errors)
						}
					}
				}
			}
		})
	}
}

// Client data is not proof: stale, forged and duplicate diagnostics cannot
// introduce an unchecked ownership change. The current source remains primary.
// Rules: rules/tooling/lsp.md — "Document snapshots", "Safe fixes";
// rules/memory/copy_move.md — §29(4).
func TestOwnershipCodeActionsValidateCurrentDiagnostic(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/move_actions/declaration_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	var report diagnostic
	for _, d := range analyze(uri, text) {
		if d.Code == diagnostics.ImplicitMoveDisallowed {
			report = d
		}
	}
	if report.Code == "" {
		t.Fatal("missing S1007")
	}
	forged := report
	forged.Range.Start.Character++
	if actions := ownershipCodeActions(uri, text, []diagnostic{forged}); len(actions) != 0 {
		t.Fatal(actions)
	}
	if actions := ownershipCodeActions(uri, "// changed snapshot\n"+text, []diagnostic{report}); len(actions) != 0 {
		t.Fatal(actions)
	}
	valid, err := os.ReadFile("../../testdata/tooling/move_actions/copy_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if actions := ownershipCodeActions(uri, string(valid), []diagnostic{report}); len(actions) != 0 {
		t.Fatal("forged S1007 on copyable value", actions)
	}
	if actions := ownershipCodeActions(uri, text, []diagnostic{report, report}); len(actions) != 1 {
		t.Fatal("duplicate repairs", actions)
	}
	malformed := strings.Replace(text, "discard destination", "let broken :=", 1)
	if actions := ownershipCodeActions(uri, malformed, []diagnostic{report}); len(actions) != 0 {
		t.Fatal("recovered source is not ownership proof", actions)
	}
}

// Validation uses unsaved sibling inputs and the replacement active source,
// without mutating the caller's snapshots or filtering away sibling errors.
// Rules: rules/tooling/lsp.md — "Document snapshots", "Target-aware analysis";
// rules/memory/copy_move.md — §29(4).
func TestOwnershipCodeActionsSourceOverlays(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile("../../testdata/tooling/move_actions/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	text := read("declaration_invalid")
	dir := t.TempDir()
	path := filepath.Join(dir, "main.sec")
	uri := uriFromPath(path)
	sibling := normalizedSourcePath(filepath.Join(dir, "sibling.sec"))
	overlay := sourceOverlay{Sources: map[string]string{normalizedSourcePath(path): text, sibling: read("sibling")}}
	reported := analyze(uri, text, overlay)
	if actions := ownershipCodeActions(uri, text, reported, overlay); len(actions) != 1 {
		t.Fatal("unsaved valid module rejected", actions, reported)
	}
	overlay.Sources[sibling] = read("sibling_invalid")
	if actions := ownershipCodeActions(uri, text, reported, overlay); len(actions) != 0 {
		t.Fatal("other-source semantic error was filtered away", actions)
	}
	if overlay.Sources[normalizedSourcePath(path)] != text {
		t.Fatal("validation mutated an open document")
	}
}

// Projected-source diagnostics whose owning operator is not recognized are
// withheld conservatively, including custom-free field moves. The syntax
// locator must not guess the declaration operator from a nearby selector.
// Rules: rules/memory/copy_move.md — §29(4); rules/tooling/lsp.md — "Safe fixes".
func TestOwnershipCodeActionsWithholdUnrecognizedProjection(t *testing.T) {
	for _, name := range []string{"partial_safe_invalid", "custom_free_field_invalid"} {
		data, err := os.ReadFile("../../testdata/tooling/move_actions/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
		text := string(data)
		reported := analyze(uri, text)
		if len(reported) == 0 {
			t.Fatal("missing invalid copy", name)
		}
		if actions := ownershipCodeActions(uri, text, reported); len(actions) != 0 {
			t.Fatal("guessed operator for a projection", actions)
		}
	}
}

// The protocol path passes the same open module snapshots to validation. An
// unsaved sibling error cannot be hidden by analyzing just the active document.
// Rules: rules/tooling/lsp.md — "Document snapshots", "Safe fixes";
// rules/memory/copy_move.md — §29(4).
func TestOwnershipCodeActionsProtocolOverlay(t *testing.T) {
	textData, err := os.ReadFile("../../testdata/tooling/move_actions/declaration_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	siblingData, err := os.ReadFile("../../testdata/tooling/move_actions/sibling_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	uri := uriFromPath(filepath.Join(dir, "main.sec"))
	siblingURI := uriFromPath(filepath.Join(dir, "sibling.sec"))
	text := string(textData)
	reported := analyze(uri, text)
	var output bytes.Buffer
	server := &server{out: &output, documentSnapshots: lspserver.NewDocuments()}
	server.documentSnapshots.Open(uri, 1, text)
	server.documentSnapshots.Open(siblingURI, 1, string(siblingData))
	params := codeActionParams{TextDocument: textDocumentIdentifier{URI: uri}}
	params.Context.Diagnostics = reported
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.handle(rpcMessage{Method: "textDocument/codeAction", ID: json.RawMessage("1"), Params: raw}); err != nil {
		t.Fatal(err)
	}
	var response struct{ Result []codeAction }
	if err := json.Unmarshal([]byte(output.String()[strings.Index(output.String(), "{"):]), &response); err != nil {
		t.Fatal(err, output.String())
	}
	for _, action := range response.Result {
		if strings.Contains(action.Title, "to move the value") {
			t.Fatal("protocol ignored unsaved sibling input", action)
		}
	}
}

// Sema acceptance is target-dependent: this source's numeric result is exact
// on the selected 64-bit plan but requires checked conversion on the 32-bit
// plan. A host-default recheck must not authorize the edit for the latter.
// Rules: rules/memory/copy_move.md — §29(4);
// rules/tooling/lsp.md — "Target-aware analysis"; rules/types/types.md — "Exact integer conversion proof".
func TestOwnershipCodeActionsSelectedTarget(t *testing.T) {
	root, uri, _ := targetSelectionFixture(t)
	data, err := os.ReadFile("../../testdata/tooling/move_actions/target_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, variant := range []string{"small", "large"} {
		overlay := sourceOverlay{Targets: map[string]lspTargetSelection{normalizedSourcePath(root): {"app", variant}}}
		reported := analyze(uri, text, overlay)
		actions := ownershipCodeActions(uri, text, reported, overlay)
		want := 0
		if variant == "large" {
			want = 1
		}
		if len(actions) != want {
			t.Fatalf("%s actions=%+v diagnostics=%+v", variant, actions, reported)
		}
	}
}

// Operator edits preserve comments, Unicode identifiers and CRLF. The emoji
// before the source places UTF-16 protocol positions and lexer columns in
// different units, so guessing an offset would corrupt the repair.
// Rules: rules/tooling/lsp.md — "Position encoding", "Safe fixes";
// rules/memory/copy_move.md — §29(3–4).
func TestOwnershipCodeActionsUnicodeAndCRLF(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/move_actions/declaration_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, ending := range []string{"\n", "\r\n"} {
		text := strings.ReplaceAll(string(data), "source", "källa")
		text = strings.Replace(text, "    let destination :=", "    /* 😀 */ let destination :=", 1)
		text = strings.ReplaceAll(text, "\n", ending)
		uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
		actions := ownershipCodeActions(uri, text, analyze(uri, text))
		if len(actions) != 1 {
			t.Fatal("missing verified Unicode repair", actions)
		}
		edits := actions[0].Edit.Changes[uri]
		if len(edits) != 1 {
			t.Fatal(edits)
		}
		start := lineCharToOffset(text, edits[0].Range.Start.Line, edits[0].Range.Start.Character)
		end := lineCharToOffset(text, edits[0].Range.End.Line, edits[0].Range.End.Character)
		if text[start:end] != ":=" || edits[0].NewText != ":<-" {
			t.Fatal("incorrect operator range", edits)
		}
		fixed := applyTextEdits(text, edits)
		if fixed != text[:start]+":<-"+text[end:] {
			t.Fatal("unrelated text changed")
		}
		if errors, complete := ownershipActionErrors(uri, fixed, sourceOverlay{}); !complete || len(errors) != 0 {
			t.Fatal(errors)
		}
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	lspserver "sec/internal/lsp/server"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

const borrowActionSource = `module main
fn Work() void {
    let mut value := 1
    let holder := ref value
    value = 2
    discard holder
}
`

// Borrow actions only reorder an existing reference-ending operation. Rechecking
// the whole replacement must reject later/deferred uses and any unrelated error.
// Rules: rules/memory/borrowing.md §§22.2(2),32(6); rules/tooling/lsp.md §§9.1,9.4.
func TestBorrowCodeActionsRequireProof(t *testing.T) {
	for _, tt := range []struct {
		name, source string
		want         int
	}{
		{"shared_write", borrowActionSource, 1},
		{"mutable_read", strings.ReplaceAll(strings.ReplaceAll(borrowActionSource, "ref value", "ref mut value"), "value = 2", "let copy := value"), 1},
		{"unicode_holder", strings.ReplaceAll(borrowActionSource, "holder", "lån"), 1},
		{"crlf", strings.ReplaceAll(borrowActionSource, "\n", "\r\n"), 1},
		{"later_use", strings.Replace(borrowActionSource, "    discard holder", "    let copy := holder\n    discard holder", 1), 0},
		{"deferred_use", strings.Replace(borrowActionSource, "    value = 2", "    defer { let copy := holder }\n    value = 2", 1), 0},
		{"no_explicit_discard", strings.Replace(borrowActionSource, "    discard holder\n", "    let copy := holder\n", 1), 0},
		{"unrelated_error", strings.Replace(borrowActionSource, "    value = 2", "    let bad := missing\n    value = 2", 1), 0},
		{"trailing_comment", strings.Replace(borrowActionSource, "discard holder", "discard holder // keep this explanation", 1), 0},
		{"leading_comment", strings.Replace(borrowActionSource, "    discard holder", "    // end the reference here\n    discard holder", 1), 0},
		{"inline_siblings", strings.Replace(borrowActionSource, "    value = 2\n    discard holder", "    value = 2 discard holder", 1), 0},
		{"different_block", strings.Replace(borrowActionSource, "    discard holder", "    if true { discard holder }", 1), 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
			reported := analyze(uri, tt.source)
			found := false
			for _, d := range reported {
				found = found || borrowActionConflict(d.Code)
			}
			if !found {
				t.Fatalf("fixture needs a canonical borrow conflict: %+v", reported)
			}
			actions := borrowCodeActions(uri, tt.source, reported, sourceOverlay{})
			if len(actions) != tt.want {
				t.Fatalf("actions=%+v, diagnostics=%+v", actions, reported)
			}
			for _, action := range actions {
				if action.Kind != "refactor.rewrite" || len(action.Diagnostics) != 1 || !strings.Contains(action.Title, "move its discard") {
					t.Fatal(action)
				}
				fixed := applyTextEdits(tt.source, action.Edit.Changes[uri])
				holder := "holder"
				if tt.name == "unicode_holder" {
					holder = "lån"
				}
				access := "value = 2"
				if tt.name == "mutable_read" {
					access = "let copy := value"
				}
				if strings.Count(fixed, "discard "+holder) != 1 || strings.Index(fixed, "discard "+holder) > strings.Index(fixed, access) {
					t.Fatal("discard was duplicated or not moved", fixed)
				}
				errors, complete := ownershipActionErrors(uri, fixed, sourceOverlay{})
				if !complete || len(errors) != 0 {
					t.Fatal("unproved edit", fixed, errors)
				}
				formatted, err := formatSourceForDocument(uri, fixed)
				if err != nil || formatted != fixed {
					t.Fatalf("noncanonical action output: %v\n%s", err, fixed)
				}
			}
		})
	}
}

// Client diagnostics are not authority; the current compiler finding, source
// position and complete overlay snapshot must match before an edit is offered.
// Rules: rules/tooling/lsp.md §§5.3,9.4; rules/memory/borrowing.md §33.
func TestBorrowCodeActionsRejectStaleAndIncompleteSnapshots(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	reported := analyze(uri, borrowActionSource)
	var conflict diagnostic
	for _, d := range reported {
		if d.Code == diagnostics.BorrowMutationConflict {
			conflict = d
		}
	}
	if conflict.Code == "" {
		t.Fatal(reported)
	}
	if actions := borrowCodeActions(uri, borrowActionSource, []diagnostic{conflict, conflict}, sourceOverlay{}); len(actions) != 1 {
		t.Fatal("duplicate diagnostics repeated an action", actions)
	}
	forged := conflict
	forged.Range.Start.Character++
	for _, tt := range []struct {
		text     string
		reported []diagnostic
		overlay  sourceOverlay
	}{
		{borrowActionSource, []diagnostic{forged}, sourceOverlay{}},
		{"// new version\n" + borrowActionSource, []diagnostic{conflict}, sourceOverlay{}},
		{strings.ReplaceAll(borrowActionSource, "value = 2", "let copy := value"), []diagnostic{conflict}, sourceOverlay{}},
		{"module main\nfn Broken(", []diagnostic{conflict}, sourceOverlay{}},
		{borrowActionSource, []diagnostic{conflict}, sourceOverlay{Sources: map[string]string{pathFromURI(uri): "stale active overlay", filepath.Join(filepath.Dir(pathFromURI(uri)), "sibling.sec"): "module main\nfn Bad() void { let x := missing }\n"}}},
	} {
		if actions := borrowCodeActions(uri, tt.text, tt.reported, tt.overlay); len(actions) != 0 {
			t.Fatal("stale or incomplete proof", actions)
		}
	}
}

// Protocol integration keeps lifetime restructuring explicit. It is not a
// quick fix or source.fixAll action and must use current unsaved module text.
// Rules: rules/tooling/lsp.md §§5.3,9.1,9.4; rules/memory/borrowing.md §22.2(2).
func TestBorrowCodeActionsProtocol(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	reports := analyze(uri, borrowActionSource)
	for _, tt := range []struct {
		kind    string
		want    int
		sibling bool
	}{
		{"refactor", 1, false}, {"refactor.rewrite", 1, false},
		{"quickfix", 0, false}, {"source.fixAll", 0, false},
		{"refactor", 0, true},
	} {
		t.Run(tt.kind, func(t *testing.T) {
			var output bytes.Buffer
			server := &server{out: &output, documentSnapshots: lspserver.NewDocuments()}
			server.documentSnapshots.Open(uri, 1, borrowActionSource)
			if tt.sibling {
				server.documentSnapshots.Open(uriFromPath(filepath.Join(filepath.Dir(pathFromURI(uri)), "sibling.sec")), 1, "module main\nfn Broken() void { let x := missing }\n")
			}
			params := codeActionParams{TextDocument: textDocumentIdentifier{URI: uri}, Context: codeActionContext{Diagnostics: reports, Only: []string{tt.kind}}}
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
			count := 0
			for _, action := range response.Result {
				if strings.HasPrefix(action.Title, "End borrow '") {
					count++
				}
			}
			if count != tt.want {
				t.Fatalf("borrow refactorings=%d want %d: %+v", count, tt.want, response.Result)
			}
		})
	}
}

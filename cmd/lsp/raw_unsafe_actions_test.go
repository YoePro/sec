package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	lspserver "sec/internal/lsp/server"
)

// Missing raw-call authorization can be repaired only when the registry admits
// the call form and full reanalysis accepts the edit. A null pointer, invalid
// call shape, stale diagnostic or property needing a block gains no wrap.
// Rules: rules/memory/unsafe.md §§47(1–4),50(5),52(3–5);
// rules/memory/raw_pointers.md §§11–14,18(5).
func TestRawUnsafeAuthorizationActions(t *testing.T) {
	for _, tc := range []struct {
		file, want string
		fix        bool
	}{
		{"missing_read_invalid.sec", "return unsafe pointer.Read()", true},
		{"missing_write_invalid.sec", "unsafe pointer.Write(value)", true},
		{"missing_index_read_invalid.sec", "return unsafe pointers[0].Read()", true},
		{"missing_volatile_read_invalid.sec", "return unsafe pointer.VolatileRead()", true},
		{"missing_volatile_write_invalid.sec", "unsafe pointer.VolatileWrite(value)", true},
		{"missing_offset_invalid.sec", "return unsafe pointer.Offset(1)", true},
		{"missing_bytes_invalid.sec", "return unsafe pointer.AddBytes(1)", true},
		{"missing_difference_invalid.sec", "return unsafe pointer.Difference(other)", true},
		{"wrong_arity_invalid.sec", "", false},
		{"wrong_bytes_invalid.sec", "", false},
		{"property_call_invalid.sec", "", false},
		{"null_read_invalid.sec", "", false},
		{"property_invalid.sec", "", false},
	} {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile("../../testdata/tooling/unsafe_actions/" + tc.file)
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
			var item *diagnostic
			reported := analyze(uri, text)
			for i := range reported {
				if reported[i].Code == diagnostics.RawPointerUnsafeRequired || !tc.fix && reported[i].Code == diagnostics.RawPointerInvalidOperation {
					item = &reported[i]
					break
				}
			}
			if item == nil {
				t.Fatalf("fixture must produce missing/invalid raw authorization diagnostic: %+v", reported)
			}
			actions := unsafeCodeActions(uri, text, []diagnostic{*item}, sourceOverlay{})
			if !tc.fix {
				if len(actions) != 0 {
					t.Fatalf("unsafe cannot repair this operation: %+v", actions)
				}
				return
			}
			if len(actions) != 1 {
				t.Fatalf("want one proven action, got %+v; diagnostics %+v", actions, reported)
			}
			edited := applyTextEdits(text, actions[0].Edit.Changes[uri])
			if !strings.Contains(edited, tc.want) {
				t.Fatalf("wrong prefix position:\n%s", edited)
			}
			for _, d := range analyze(uri, edited) {
				if d.Severity == 1 {
					t.Fatalf("offered action leaves an error: %+v", d)
				}
			}
			// The old occurrence is stale after authorization and must not create a
			// second unsafe prefix even if a client resends the original diagnostic.
			if stale := unsafeCodeActions(uri, edited, []diagnostic{*item}, sourceOverlay{}); len(stale) != 0 {
				t.Fatalf("stale diagnostic produced another action: %+v", stale)
			}
		})
	}
}

// The protocol route must use the unsaved document snapshot and publish the
// same validated edit, rather than depend on a file already saved to disk.
// Rules: rules/tooling/lsp.md — "Code actions"; rules/memory/unsafe.md §52(3–5).
func TestRawUnsafeAuthorizationProtocol(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/unsafe_actions/missing_read_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "unsaved.sec"))
	s := &server{documentSnapshots: lspserver.NewDocuments()}
	s.documentSnapshots.Open(uri, 1, source)
	var reported diagnostic
	for _, d := range analyze(uri, source, s.sourceOverlay()) {
		if d.Code == diagnostics.RawPointerUnsafeRequired {
			reported = d
			break
		}
	}
	if reported.Code == "" {
		t.Fatal("missing raw diagnostic")
	}
	actions := lspTestDecode[[]codeAction](t, lspTestRequest(t, s, "textDocument/codeAction", codeActionParams{TextDocument: textDocumentIdentifier{URI: uri}, Range: reported.Range, Context: codeActionContext{Diagnostics: []diagnostic{reported}}}))
	for _, action := range actions {
		if action.Title == "Authorize this call with unsafe" {
			edited := applyTextEdits(source, action.Edit.Changes[uri])
			if !strings.Contains(edited, "return unsafe pointer.Read()") {
				t.Fatal(edited)
			}
			return
		}
	}
	t.Fatalf("no protocol authorization action: %+v", actions)
}

// A stale Information advisory must never remove an unsafe prefix that the
// current call actually needs, even when the caller fabricates its range.
// Rules: rules/memory/unsafe.md §§50(5),52(3–5).
func TestUnsafeRemoveRejectsStaleAdvisory(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/unsafe_actions/missing_read_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.Replace(string(data), "return pointer.Read()", "return unsafe pointer.Read()", 1)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	start := strings.Index(source, "unsafe pointer")
	stale := diagnostic{Code: diagnostics.UnsafeUnnecessary, Range: lspRange{Start: offsetPosition(source, start), End: offsetPosition(source, start+6)}}
	if actions := unsafeCodeActions(uri, source, []diagnostic{stale}, sourceOverlay{}); len(actions) != 0 {
		t.Fatalf("stale advisory removed required authorization: %+v", actions)
	}
}

// Removing a redundant call prefix retains the enclosing unsafe block and
// its authorization; only the exact current Information advisory is fixed.
// Rules: rules/memory/unsafe.md §§50(5),52(3–5).
func TestUnsafeRedundantActionPreservesBlock(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/unsafe_actions/redundant_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, d := range analyze(uri, source) {
		if d.Code == diagnostics.UnsafeRedundant {
			actions := unsafeCodeActions(uri, source, []diagnostic{d}, sourceOverlay{})
			if len(actions) != 1 {
				t.Fatalf("missing redundant prefix fix: %+v", actions)
			}
			edited := applyTextEdits(source, actions[0].Edit.Changes[uri])
			if strings.Count(edited, "unsafe") != 1 || !strings.Contains(edited, "unsafe {") || !strings.Contains(edited, "return pointer.Read()") {
				t.Fatal(edited)
			}
			for _, remaining := range analyze(uri, edited) {
				if remaining.Severity == 1 {
					t.Fatalf("edit loses enclosing authorization: %+v", remaining)
				}
			}
			return
		}
	}
	t.Fatal("missing redundant advisory")
}

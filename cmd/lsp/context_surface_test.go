package main

import (
	"os"
	"strings"
	"testing"
)

// TestCancellationContextLSP consumes the shared compiler catalog for exact
// completion, hover and warm/cold navigation; it adds no editor-owned API model.
// Rules: rules/concurrency/cancellation.md — §§33,35–38,43;
// rules/tooling/lsp.md — Completion, Hover, Go to definition.
func TestCancellationContextLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/context_surface_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source, uri := string(data), "file:///context-surface/main.sec"
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	for _, tc := range []struct {
		needle       string
		want, absent []string
	}{
		{"Context.Background", []string{"Background"}, []string{"WithCancel", "Cancel", "IsCancelled"}},
		{"root.WithCancel", []string{"WithCancel", "WithTimeout", "WithDeadline", "IsCancelled", "Error", "Deadline"}, []string{"Cancel", "Background"}},
		{"source.Cancel", []string{"Context", "Cancel"}, []string{"WithCancel", "Background", "IsCancelled"}},
	} {
		cursor := strings.Index(source, tc.needle) + strings.Index(tc.needle, ".") + 1
		items := completeSource(uri, source[:cursor]+"\n}\n", cursor)
		assertCompletionLabels(t, items, tc.want)
		for _, item := range items {
			for _, absent := range tc.absent {
				if item.Label == absent {
					t.Fatal("unexpected completion", absent)
				}
			}
		}
	}
	for _, tc := range []struct{ needle, signature, id string }{
		{"Context.Background", "static fn Background() Context", "CKM-CONTEXT-BACKGROUND"},
		{"context.WithCancel", "fn WithCancel() ContextSource", "CKM-CONTEXT-WITHCANCEL"},
		{"context.WithTimeout", "fn WithTimeout(timeout: duration) ContextSource", "CKM-CONTEXT-WITHTIMEOUT"},
		{"context.WithDeadline", "fn WithDeadline(deadline: Instant) ContextSource", "CKM-CONTEXT-WITHDEADLINE"},
		{"context.IsCancelled", "property IsCancelled: bool", "CKM-CONTEXT-ISCANCELLED"},
		{"context.Error", "property Error: Option[ContextError]", "CKM-CONTEXT-ERROR"},
		{"context.Deadline", "property Deadline: Option[Instant]", "CKM-CONTEXT-DEADLINE"},
		{"source.Context", "property Context: ref Context", "CKM-CONTEXTSOURCE-CONTEXT"},
		{"source.Cancel", "fn Cancel() void", "CKM-CONTEXTSOURCE-CANCEL"},
	} {
		cursor := strings.Index(source, tc.needle) + strings.Index(tc.needle, ".") + 1
		pos := offsetPosition(source, cursor)
		hover, ok := hoverForSource(uri, source, pos)
		if !ok || !strings.Contains(hover.Contents.Value, tc.signature) {
			t.Fatal(tc.needle, hover, ok)
		}
		locations := definitionsForSource(uri, source, pos)
		if len(locations) != 1 {
			t.Fatal(tc.needle, locations)
		}
		for _, target := range []string{locations[0].URI, "sec-compiler-known:/" + tc.id + ".sec"} {
			text, ok := compilerKnownDefinitionText(target)
			if !ok || !strings.Contains(text, tc.signature) || !strings.Contains(text, "rules/concurrency/cancellation.md") {
				t.Fatal(tc.needle, text, ok)
			}
		}
	}
	invalid, err := os.ReadFile("../../testdata/sema/context_surface_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range analyze(uri, string(invalid)) {
		if d.Severity == 1 {
			count++
		}
	}
	if count != 14 {
		t.Fatal("invalid diagnostics", count)
	}
}

// TestCancellationContextTypeNavigation reaches the real opaque core owner
// declarations from both borrowed receiver types, rather than synthetic state.
// Rules: rules/concurrency/cancellation.md — §§35–38;
// rules/tooling/lsp.md — Go to type definition.
func TestCancellationContextTypeNavigation(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/context_surface_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source, uri := string(data), "file:///context-types/main.sec"
	core, err := os.ReadFile("../../sec/core/context.sec")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(core), "\n")
	for _, tc := range []struct{ needle, name string }{{"context.WithCancel", "Context"}, {"source.Cancel", "ContextSource"}} {
		pos := offsetPosition(source, strings.Index(source, tc.needle))
		hover, ok := hoverForSource(uri, source, pos)
		if !ok || !strings.Contains(hover.Contents.Value, "ref "+tc.name) {
			t.Fatal(tc.name, hover, ok)
		}
		locations := typeDefinitionsForSource(uri, source, pos)
		if len(locations) != 1 || !strings.HasSuffix(locations[0].URI, "/sec/core/context.sec") {
			t.Fatal(tc.name, locations)
		}
		line := locations[0].Range.Start.Line
		if line < 0 || line >= len(lines) || strings.TrimSpace(lines[line]) != "type "+tc.name {
			t.Fatal(tc.name, locations)
		}
	}
}

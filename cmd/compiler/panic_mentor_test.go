package main

import (
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// CLI JSON keeps registered panic/cleanup identities, repair text and root/cause
// source links; unsupported ISR attributes do not erase canonical panic evidence.
// Rules: rules/errors/panic.md — §28; rules/control-flow/defer.md — §31.
func TestPanicMentorCompilerDiagnostics(t *testing.T) {
	for _, name := range []string{"panic_mentor_invalid", "isr_panic_paths_invalid"} {
		t.Run(name, func(t *testing.T) {
			path := "../../testdata/sema/" + name + ".sec"
			_, stderr, code := runCLIForDiagnostics(t, "check", path, "--diagnostic-format=json")
			if code != 3 {
				t.Fatalf("expected source rejection: %d %s", code, stderr)
			}
			doc := decodeOccurrenceDocument(t, stderr)
			counts := map[string]int{}
			for _, d := range doc.Occurrences {
				if d.ID == nil {
					continue
				}
				id := *d.ID
				if id != diagnostics.NoPanicViolation && id != diagnostics.InvalidDeferredCleanup && id != diagnostics.TryPropagationIncompatible && id != diagnostics.TryResidualUnpropagatable {
					continue
				}
				counts[id]++
				if d.Unregistered || d.Primary == nil || d.Primary.Span.File != path || len(d.Help) != 1 {
					t.Fatalf("missing shared diagnostic metadata: %+v", d)
				}
				if id == diagnostics.NoPanicViolation {
					if len(d.Related) != 1 || d.Related[0].Span.File != path || !strings.Contains(d.Message.Text, "via") {
						t.Fatalf("missing panic path/root: %+v", d)
					}
				}
			}
			if counts[diagnostics.NoPanicViolation] != 7 {
				t.Fatal("missing panic causes", counts)
			}
			if name == "panic_mentor_invalid" && (counts[diagnostics.InvalidDeferredCleanup] != 8 || counts[diagnostics.TryPropagationIncompatible] != 2 || counts[diagnostics.TryResidualUnpropagatable] != 2 || doc.Summary.Errors != 19) {
				t.Fatal("missing cleanup causes", counts, doc.Summary)
			}
		})
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/panic_mentor_valid.sec")
	if code != 0 {
		t.Fatalf("local cleanup recovery rejected: %d %s", code, stderr)
	}
}

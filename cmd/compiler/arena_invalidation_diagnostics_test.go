package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestArenaInvalidationCLIDiagnostics verifies identity, method/dependency
// locations, proof state and help through real CLI commands in both formats.
// Rules: rules/memory/arena.md — §§119–120;
// rules/tooling/diagnostics.md — §§9, 14, 30.
func TestArenaInvalidationCLIDiagnostics(t *testing.T) {
	path := "../../testdata/sema/arena_invalidation_diagnostics_invalid.sec"
	for _, command := range []string{"sema", "emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir"} {
		t.Run(command, func(t *testing.T) {
			args := []string{command, path}
			if command != "sema" {
				args = append(args, "-o", "-")
			}
			_, output, code := runCLIForDiagnostics(t, append(args, "--diagnostic-format=json")...)
			doc := decodeOccurrenceDocument(t, output)
			if code != 3 || doc.Summary.Errors != 6 || len(doc.Occurrences) != 6 {
				t.Fatal(code, output)
			}
			count := map[string]int{}
			for _, value := range doc.Occurrences {
				if value.ID == nil {
					t.Fatal(value)
				}
				count[*value.ID]++
				if value.Unregistered || value.Source != "sema" || value.Arguments["proof_state"] != "Invalid" || value.Primary == nil || value.Primary.Span.File != path || len(value.Related) != 1 || value.Related[0].Span.File != path || value.Related[0].Message == nil || value.Related[0].Message.Text != "live Arena dependency" || len(value.Help) != 1 {
					t.Fatal(value)
				}
			}
			if count[diagnostics.ArenaResetLiveDependency] != 4 || count[diagnostics.ArenaReleaseLiveDependency] != 2 {
				t.Fatal(count)
			}
			_, human, humanCode := runCLIForDiagnostics(t, args...)
			if humanCode != code || !strings.Contains(human, diagnostics.ArenaResetLiveDependency) || !strings.Contains(human, diagnostics.ArenaReleaseLiveDependency) || !strings.Contains(human, "live Arena dependency at") || !strings.Contains(human, "help: Reset invalidates") || !strings.Contains(human, "help: Release ends") {
				t.Fatal(humanCode, human)
			}
		})
	}
}

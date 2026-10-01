package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestUseAfterDiscardHasStableMandatoryDiagnostic verifies that ordinary
// reads and borrows share the registered identity and retain the discard site.
//
// Rules:
//   - rules/control-flow/discard.md — §§5, 19, 37
//   - rules/tooling/diagnostics.md — §§7, 25
func TestUseAfterDiscardHasStableMandatoryDiagnostic(t *testing.T) {
	cases := []struct {
		name    string
		use     string
		message string
	}{
		{name: "read", use: "let again := value", message: "value value was discarded"},
		{name: "borrow", use: "let view := ref value", message: "it was discarded here"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "module main\nfn Check() void {\n    let value := 42\n    discard value\n    " + tc.use + "\n}\n"
			errors := analyzeSourceRaw(t, source)
			if len(errors) != 1 {
				t.Fatalf("diagnostics = %+v, want one", errors)
			}
			diagnostic := errors[0]
			if diagnostic.ID != diagnostics.UseAfterDiscard ||
				diagnostic.Severity != diagnostics.SeverityError ||
				diagnostic.PreviousLine != 4 ||
				diagnostic.PreviousColumn != 13 ||
				diagnostic.Help == "" ||
				!strings.Contains(diagnostic.Message, tc.message) {
				t.Fatalf("use-after-discard diagnostic = %+v", diagnostic)
			}
		})
	}
}

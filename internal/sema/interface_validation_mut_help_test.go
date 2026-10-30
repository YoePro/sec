package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// The S1106 help names the concrete method to write: `fn`, not the interface's
// `mut fn`, which an impl rejects with P2037 (rules/declarations/impl.md).
func TestMissingMutableMemberHelpNamesImplForm(t *testing.T) {
	p := parser.New(lexer.NewWithFile(`module main
interface Reset {
    mut fn Clear() void
}
type A struct { X: int }
impl A implements Reset {
}
`, "sample.sec"))
	parsed := p.Parse()
	errors := NewAnalyzer().Analyze(parsed.Program)
	for _, diagnostic := range errors {
		if diagnostic.ID != diagnostics.InterfaceMemberMissing {
			continue
		}
		if !strings.Contains(diagnostic.Help, "declare fn Clear() void in the impl of A") {
			t.Fatalf("help = %q", diagnostic.Help)
		}
		return
	}
	t.Fatalf("no %s: %#v", diagnostics.InterfaceMemberMissing, errors)
}

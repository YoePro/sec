package semantic

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Fixed-array membership over built-in integers lowers to exactly-once
// operands, constant proven-safe element extracts in index order with an
// early exit, and a negated result for `not in`; the module verifies.
// Rules: rules/foundations/operators.md — "Membership expression".
func TestFixedArrayIntegerMembershipLowering(t *testing.T) {
	source := `module main
fn Contains(value: int, values: int[3]) bool {
    return value in values
}

fn Missing(value: int32, values: int32[2]) bool {
    return value not in values
}

fn Empty(value: int, values: int[0]) bool {
    return value in values
}
`
	p := parser.New(lexer.NewWithFile(source, "array-membership.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errs := analyzer.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatalf("sema errors: %v", errs)
	}
	module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", SourceFiles: []string{"array-membership.sec"}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	if got := strings.Count(text, "proof=constant"); got != 5 {
		t.Fatalf("want 5 constant element extracts, got %d:\n%s", got, text)
	}
	if !strings.Contains(text, "bool.not") || !strings.Contains(text, "const.bool false") {
		t.Fatalf("missing negation or empty-array result:\n%s", text)
	}
}

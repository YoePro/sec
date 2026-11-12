package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Hover over a required compile-time root shows the value the compiler
// established, or the evaluation failure with its executed call chain.
// Rules: rules/compiler/compile_time_evaluation.md — § 73; rules/tooling/lsp.md — "Hover".
func TestCompileTimeValueHover(t *testing.T) {
	text := `module main

fn Limit() int {
    return 10 * 10
}

fn Divide(n: int) int {
    return 100 / n
}

let Size: int := Limit() + 1

type Percent int range 0..Limit()
type Broken int range 0..Divide(0)

fn Main() int {
    return Size
}
`
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, test := range []struct{ marker, want string }{
		{"Limit() + 1", "`Limit() + 1` = `101: int`, established by semantic compile-time evaluation"},
		{"Limit()\ntype Broken", "`Limit()` = `100: int`"},
		{"Limit()\ntype Broken", "required by: range upper bound (SemanticCompileTimeRequiredContext)"},
		{"Limit() + 1", "required by: static initializer (SemanticCompileTimeRequiredContext)"},
		{"Divide(0)", "SemanticOperationFailed: division by zero"},
	} {
		idx := strings.Index(text, test.marker)
		if idx < 0 {
			t.Fatalf("marker %q missing", test.marker)
		}
		hover, _ := hoverForSource(uri, text, offsetPosition(text, idx))
		if !strings.Contains(hover.Contents.Value, test.want) {
			t.Errorf("%q hover lacks %q:\n%s", test.marker, test.want, hover.Contents.Value)
		}
	}
}

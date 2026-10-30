package semantic

import (
	"errors"
	"strings"
	"testing"
)

// The decimal Scale getter and its fallible exact setter have no Semantic IR
// lowering before P21; both fail closed, and the setter names itself rather
// than a constrained-type contract check.
//
// Rules:
//   - rules/library/core-library.md — § 10.1 (MD-066)
//   - rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — §§ 12(1), 13(4)
func TestDecimalScaleAccessFailsClosed(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"module main\nfn Use(x: decimal) Result[decimal, PrecisionError] {\n    let mut y := x\n    try y.Scale = 2\n    return Ok(y)\n}\n", "decimal Scale setter"},
		{"module main\nfn Use(x: decimal) int32 {\n    return x.Scale\n}\n", "member expression"},
	} {
		_, err := analyzedModule(t, tc.source, 21)
		var unsupported *UnsupportedFeatureError
		if !errors.As(err, &unsupported) || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("want unsupported %q, got %v", tc.want, err)
		}
	}
}

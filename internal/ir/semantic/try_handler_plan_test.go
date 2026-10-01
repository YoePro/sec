package semantic

import (
	"errors"
	"strings"
	"testing"
)

// Guarded handlers and block-final recovery values are not yet represented in
// Semantic IR, so plans using them are rejected explicitly instead of being
// lowered as if the guard were absent or the block produced no value.
//
// Rules:
//   - rules/errors/errorhandling.md — §19 "Guards", §21.1 "Block handler result position", §35
func TestTryPlansWithGuardsOrBlockValuesAreRejected(t *testing.T) {
	sources := map[string]string{
		"guard": `module main
fn Add(left: int, right: int) int {
  return try left + right {
    Err(error) where error == ArithmeticError.Overflow => 1
    Err(_) => 0
  }
}
`,
		"block value": `module main
fn Add(left: int, right: int) int {
  return try left + right {
    Err(_) => {
      0
    }
  }
}
`,
	}
	for name, source := range sources {
		_, err := analyzedModule(t, source, 10)
		var unsupported *UnsupportedFeatureError
		if !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "handler plan") {
			t.Fatalf("%s: lowering error = %v, want an explicit unsupported handler plan", name, err)
		}
	}
}

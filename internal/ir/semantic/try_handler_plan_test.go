package semantic

import (
	"errors"
	"strings"
	"testing"
)

// Guarded handlers, block-final recovery values, and partial handler plans
// lower to verified Semantic IR: a guard is tested after its pattern with the
// binding in scope and false continues with the next handler, a value block
// yields its final expression to the merge, and unmatched failures of a
// partial plan leave through a residual Err return.
//
// Rules:
//   - rules/errors/errorhandling.md — §16 "Partial handlers", §19 "Guards", §21.1 "Block handler result position", §35
func TestTryPlansWithGuardsBlockValuesAndResidualsLower(t *testing.T) {
	sources := map[string]struct {
		source  string
		feature string
	}{
		"guard": {source: `module main
fn Add(left: int, right: int) int {
  return try left + right {
    Err(error) where left > 0 => 1
    Err(_) => 0
  }
}
`, feature: "guarded"},
		"block value": {source: `module main
fn Add(left: int, right: int) int {
  return try left + right {
    Err(_) => {
      let fallback := 7
      fallback
    }
  }
}
`},
		"partial": {source: `module main
fn Divide(left: int, right: int) Result[int, ArithmeticError] {
  let value := try left / right {
    Err(ArithmeticError.DivisionByZero) => 0
  }
  return Ok(value)
}
`, feature: "residual-propagates"},
	}
	for name, test := range sources {
		module, err := analyzedModule(t, test.source, 14)
		if err != nil {
			t.Fatalf("%s: build: %v", name, err)
		}
		if err := Verify(module); err != nil {
			t.Fatalf("%s: verify: %v\n%s", name, err, Format(module))
		}
		if test.feature != "" && !strings.Contains(Format(module), test.feature) {
			t.Fatalf("%s: missing %q provenance:\n%s", name, test.feature, Format(module))
		}
	}
	module, _ := analyzedModule(t, sources["partial"].source, 14)
	residual := false
	for _, block := range module.Functions[0].Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpReturn && op.TryHandlerKind == TryHandlerResidual {
				residual = true
			}
		}
	}
	if !residual {
		t.Fatalf("partial plan has no residual Err return:\n%s", Format(module))
	}
}

// Open error narrowing in try and match needs concrete error identity that
// Semantic IR does not represent yet, so it is rejected explicitly.
//
// Rules:
//   - rules/errors/errorhandling.md — §27.2 "Matching Result[T, error]", §35
func TestOpenErrorNarrowingIsRejected(t *testing.T) {
	sources := map[string]string{
		"match": `module main
enum IOError error { NotFound, }
fn Read() Result[int, error] { return Ok(1) }
fn F() int {
  return match Read() {
    Ok(value) => value
    Err(IOError.NotFound) => 0
    Err(_) => 1
  }
}
`,
		"try": `module main
enum IOError error { NotFound, }
fn Read() Result[int, error] { return Ok(1) }
fn F() int {
  return try Read() {
    Err(IOError.NotFound) => 0
    Err(_) => 1
  }
}
`,
	}
	for name, source := range sources {
		_, err := analyzedModule(t, source, 13)
		var unsupported *UnsupportedFeatureError
		if !errors.As(err, &unsupported) {
			t.Fatalf("%s: lowering error = %v, want an explicit unsupported feature", name, err)
		}
	}
}

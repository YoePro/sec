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

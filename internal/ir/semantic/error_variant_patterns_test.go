package semantic

import (
	"strings"
	"testing"
)

const errorVariantPatternSource = `module main
enum IOError error { InvalidValue, NotFound }
type FileError union error {
  Missing { Code: int, Depth: int }
  Denied
  Encoding(int)
}
fn Read(kind: int) Result[int, IOError] {
  if kind == 1 { return Err(IOError.InvalidValue) }
  return Ok(kind)
}
fn Open(kind: int) Result[int, FileError] {
  if kind == 1 { return Err(FileError.Denied) }
  return Ok(kind)
}
fn ClassifyRead(kind: int) int {
  match Read(kind) {
    Ok(value) => return value
    Err(IOError.InvalidValue) => return 10
    Err(.NotFound) => return 20
  }
}
fn ClassifyOpen(kind: int) int {
  return match Open(kind) {
    Ok(found) => found
    Err(FileError.Missing { Code: code }) where code > 5 => code
    Err(.Missing {}) => 100
    Err(.Denied) => 200
    Err(FileError.Encoding(position)) => 300 + position
  }
}
fn RecoverOpen(kind: int) int {
  let result := try Open(kind) {
    Err(.Encoding(_)) => 1
    Err(.Denied) => 2
    Err(.Missing { Code: code, Depth }) => code + Depth
  }
  return result
}
fn RecoverPayload(kind: int) int {
  let result := try Open(kind) {
    Err(.Encoding(position)) => position
    Err(other) => 0
  }
  return result
}`

func errorVariantFunction(t *testing.T, module *Module, name string) *Function {
	t.Helper()
	for _, function := range module.Functions {
		if function.Name == name {
			return function
		}
	}
	t.Fatalf("function %s not built", name)
	return nil
}

// MD-008/MD-029: an `Err(E.Variant...)` arm on a closed concrete error channel
// tests Err, unwraps the error only on that edge, tests the enum case or union
// variant of the unwrapped error and projects payloads and fields on the
// variant test's true edge; the arm record names the selected union variant.
// Rules: rules/compiler/semantic_ir.md — § 71(6);
// rules/errors/errorhandling.md — §§ 15.3, 27.2b.
func TestErrorVariantMatchArmsLower(t *testing.T) {
	module, err := analyzedModule(t, errorVariantPatternSource, 15)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	read := errorVariantFunction(t, module, "ClassifyRead")
	if len(read.Matches) != 1 {
		t.Fatalf("ClassifyRead matches = %#v", read.Matches)
	}
	for _, arm := range read.Matches[0].Arms[1:] {
		if arm.PatternKind != "result-err-variant" {
			t.Fatalf("ClassifyRead arm = %#v", arm)
		}
	}
	open := errorVariantFunction(t, module, "ClassifyOpen")
	arms := open.Matches[0].Arms
	wantVariants := []UnionVariantIndex{0, 0, 1, 2}
	for index, want := range wantVariants {
		if arm := arms[index+1]; arm.PatternKind != "result-err-variant" || arm.VariantIndex != want {
			t.Fatalf("ClassifyOpen arm %d = %#v", index+1, arm)
		}
	}
	if !arms[1].Guarded {
		t.Fatalf("guarded field arm = %#v", arms[1])
	}
	text := Format(module)
	for _, want := range []string{"enum.compare", "result.unwrap-err", "union.is-variant", "union.unwrap-field", "variant=#0 field=Code[copy-trivial]", "union.unwrap-payload", "variant=#2[copy-trivial]"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
}

// MD-029 try handlers on a union error select the variant with union.is-variant
// and bind payloads and fields on its true edge; the last, unconditional
// field handler of an exhaustive list keeps its test and ends the impossible
// false edge in exhaustive-try-fallthrough.
// Rules: rules/errors/errorhandling.md — §§ 15.3, 16, 20.
func TestErrorVariantTryHandlersLower(t *testing.T) {
	module, err := analyzedModule(t, errorVariantPatternSource, 15)
	if err != nil {
		t.Fatal(err)
	}
	recover := Format(&Module{Types: module.Types, Functions: []*Function{errorVariantFunction(t, module, "RecoverOpen")}})
	if strings.Count(recover, "union.is-variant") != 3 || !strings.Contains(recover, `reason="exhaustive-try-fallthrough"`) ||
		!strings.Contains(recover, "field=Code[copy-trivial]") || !strings.Contains(recover, "field=Depth[copy-trivial]") {
		t.Fatal(recover)
	}
	payload := Format(&Module{Types: module.Types, Functions: []*Function{errorVariantFunction(t, module, "RecoverPayload")}})
	if !strings.Contains(payload, "union.unwrap-payload") || strings.Contains(payload, "exhaustive-try-fallthrough") {
		t.Fatal(payload)
	}
}

// The verifier keeps an error payload projection on the true edge of a test
// of the same variant; a mismatched variant is rejected.
func TestErrorVariantProjectionNeedsMatchingTest(t *testing.T) {
	module, err := analyzedModule(t, errorVariantPatternSource, 15)
	if err != nil {
		t.Fatal(err)
	}
	function := errorVariantFunction(t, module, "ClassifyOpen")
	mutated := false
	for _, block := range function.Blocks {
		for index := range block.Operations {
			if op := &block.Operations[index]; op.Kind == OpUnionUnwrapPayload {
				op.UnionVariant = 1
				mutated = true
			}
		}
	}
	if !mutated {
		t.Fatal("no payload projection")
	}
	if err := Verify(module); err == nil {
		t.Fatal("mismatched error payload projection verified")
	}
}

// Error patterns Semantic IR cannot represent yet stay explicit unsupported
// lowerings instead of losing their ownership or narrowing semantics.
func TestErrorVariantPatternsFailClosed(t *testing.T) {
	for _, test := range []struct {
		name, source, want string
	}{
		{"borrowed payload", `module main
type FileError union error { Denied Encoding(int) }
fn Open() Result[int, FileError] { return Ok(1) }
fn Inspect() int {
  let result := Open()
  match result {
    Ok(value) => return value
    Err(FileError.Encoding(ref position)) => return 1
    Err(.Denied) => return 2
  }
}`, "borrowed concrete error variant payload"},
		{"open narrowing", `module main
enum IOError error { NotFound }
fn Open() Result[int, error] { return Ok(1) }
fn Inspect() int {
  match Open() {
    Ok(value) => return value
    Err(IOError.NotFound) => return 1
    Err(other) => return 2
  }
}`, "type error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := analyzedModule(t, test.source, 15)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Every declared ToString, the canonical replacement, each overload, and each
// interface requirement, must return Result[string, StringError]; other
// result types are reported once at the declared result with S1100.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "ToString()", "User-defined ToString()"
func TestToStringDeclarationsMustReturnAllocationResult(t *testing.T) {
	errors := analyzeSource(t, `
module main

type Plain struct { value: int, }
type Overloaded struct { value: int, }
type Fallible struct { value: int, }

enum ShapeError error {
	Invalid,
}

impl Plain {
	fn ToString() string {
		return "plain"
	}
}

impl Overloaded {
	fn ToString(format: string) Result[string, ShapeError] {
		return Ok(format)
	}
}

impl Fallible {
	fn ToString() Result[string, StringError] {
		return self.value.ToString()
	}

	fn ToString(format: string) Result[string, StringError] {
		return Ok(format)
	}
}

interface Printable {
	fn ToString() string
}
`)
	reported := 0
	for _, diagnostic := range errors {
		if diagnostic.ID != diagnostics.ToStringSignature {
			continue
		}
		reported++
		if !strings.Contains(diagnostic.Message, "ToString must return Result[string, StringError]") ||
			!strings.Contains(diagnostic.Help, "Result[string, StringError]") {
			t.Fatalf("incomplete S1100 diagnostic: %+v", diagnostic)
		}
	}
	if reported != 3 {
		t.Fatalf("errors = %v, want three S1100 diagnostics (Plain, Overloaded, Printable)", errors)
	}
}

// The canonical declaration replaces the compiler-provided fallback, and both
// the fallback and the replacement are ordinary fallible calls handled by try.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "ToString()", "User-defined ToString()"
func TestCanonicalToStringIsAFallibleCall(t *testing.T) {
	errors := analyzeSource(t, `
module main

type Packet struct { value: int, }

impl Packet {
	fn ToString() Result[string, StringError] {
		return self.value.ToString()
	}
}

fn Render(packet: Packet, count: int) Result[string, StringError] {
	let text := try packet.ToString()
	let number := try count.ToString()
	return Ok(try text + number)
}
`)
	assertSemaErrors(t, errors, nil)

	errors = analyzeSource(t, `
module main

fn Render(count: int) string {
	let text: string := count.ToString()
	return text
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "Result[string, StringError]") {
		t.Fatalf("errors = %v, want one mismatch for an unhandled ToString result", errors)
	}
}

// StringError is the one compiler-known text failure family: ToString and
// runtime interpolation both fail with it, its Allocation and Format variants
// carry the underlying AllocationError and FormatError, and propagating it
// into a narrower AllocationError channel is rejected.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Uniform ToString result"
//   - rules/foundations/operators.md — string materialization failure
func TestStringErrorIsTheTextFailureFamily(t *testing.T) {
	errors := analyzeSource(t, `
module main

fn Describe(failure: StringError) int {
	match failure {
		StringError.Allocation(cause) => {
			if cause == AllocationError.OutOfMemory {
				return 1
			}
			return 2
		}
		StringError.Format(cause) => {
			if cause == FormatError.InvalidFormat {
				return 3
			}
			return 4
		}
		StringError.InvalidUtf8(index) => {
			return int(index)
		}
		StringError.InvalidCodePoint => {
			return 5
		}
		StringError.OutOfBounds => {
			return 6
		}
	}
}

fn Render(count: int) Result[string, StringError] {
	let text := try count.ToString()
	return Ok(try $"count={text}")
}
`)
	assertSemaErrors(t, errors, nil)

	errors = analyzeSource(t, `
module main

fn Render(text: string) Result[string, AllocationError] {
	return Ok(try $"text={text}")
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "StringError") {
		t.Fatalf("errors = %v, want StringError propagation into AllocationError rejected", errors)
	}
}

// A Result[string, StringError] operand such as a ToString() call joins a `+`
// concatenation directly: the whole expression is one runtime plan whose
// single StringError channel is handled by one try, constant text still folds
// without try, a missing try is S1097, propagation into a narrower channel is
// rejected, and non-text operands keep S1022.
//
// Rules:
//   - rules/foundations/operators.md — "Direct operand matrix", "Fallible text operands"
func TestConcatenationAcceptsFallibleTextOperands(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

fn Constant() string {
    let rv := "A" + "B"
    return rv
}

fn Single(count: int) Result[string, StringError] {
    return Ok(try "A" + count.ToString() + "B")
}

fn Both(left: int, right: int) Result[string, StringError] {
    let text := try left.ToString() + ":" + right.ToString()
    return Ok(text)
}
`)
	assertSemaErrors(t, errors, nil)
	_ = analyzer

	errors = analyzeSource(t, `
module main

fn Missing(count: int) Result[string, StringError] {
    return Ok("A" + count.ToString())
}

fn WrongChannel(count: int) Result[string, AllocationError] {
    return Ok(try "A" + count.ToString())
}

fn NotText(count: int) Result[string, StringError] {
    return Ok(try "A" + count)
}

fn Append(count: int) string {
    let mut result := ""
    result += count.ToString()
    return result
}
`)
	want := []string{diagnostics.TryPropagationIncompatible, diagnostics.OperatorInvalidConcatOperand, diagnostics.OperatorInvalidConcatOperand, diagnostics.StringMaterializationRequiresTry}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, id := range want {
		if errors[index].ID != id {
			t.Fatalf("error %d = %+v, want %s", index, errors[index], id)
		}
	}
}

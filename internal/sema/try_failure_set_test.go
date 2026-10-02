package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

const failureSetPrelude = `
module main

type Percent int range 0..100

fn Wrap(value: int) Result[int, error] {
    return Ok(value)
}

fn Find(value: int) Option[int] {
    return Some(value)
}
`

func failureSetMessages(t *testing.T, body string) []string {
	t.Helper()
	messages := []string{}
	for _, err := range analyzeSourceRaw(t, failureSetPrelude+body) {
		messages = append(messages, err.Message)
	}
	return messages
}

// Every language-defined check in the protected subtree is part of the
// failure set: indexing of dynamic arrays and slices, checked arithmetic,
// constrained conversions, and checks inside the arguments of a protected
// Result call. Naked propagation requires each member to fit the channel.
//
// Rules:
//   - rules/errors/errorhandling.md — §9 "Operands accepted by try", §11 "Compiler-internal failure sets", §12.1
//   - rules/errors/runtime_checks.md — "What try converts", "Index checks", "Type contracts"
func TestTryFailureSetPropagation(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "mixed into open channel", body: `
fn Use(values: int[], index: uint, amount: int) Result[int, error] {
    let value := try values[index] + amount
    return Ok(value)
}`},
		{name: "dynamic index", body: `
fn Use(values: int[], index: uint) Result[int, IndexError] {
    return Ok(try values[index])
}`},
		{name: "slice index", body: `
fn Use(values: ref int[], index: uint) Result[int, IndexError] {
    return Ok(try values[index])
}`},
		{name: "constrained conversion", body: `
fn Use(raw: int) Result[Percent, ContractError] {
    return Ok(try Percent(raw))
}`},
		{name: "argument of protected call", body: `
fn Use(left: int, right: int) Result[int, error] {
    return Ok(try Wrap(left + right))
}`},
		{name: "member does not fit", body: `
fn Use(values: int[], index: uint, amount: int) Result[int, ArithmeticError] {
    return Ok(try values[index] + amount)
}`, want: []string{"bodyless try propagates IndexError from values[index] with return Err, but this function returns Result[int, ArithmeticError]; handle it locally with Err(_) or map IndexError to ArithmeticError"}},
		{name: "constant conversion is not fallible", body: `
fn Use() Result[Percent, ContractError] {
    return Ok(try Percent(50))
}`, want: []string{"try requires Result expression, Option expression, or a language-defined runtime check such as checked arithmetic, indexing, or a constrained conversion; Percent(50) contains none"}},
		{name: "option cannot carry errors", body: `
fn Use(values: int[], index: uint) Option[int] {
    return Some(try Find(values[index]))
}`, want: []string{"try over an Option cannot also protect the IndexError from values[index], because absence and errors use different channels; protect that operation with its own try"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := failureSetMessages(t, test.body)
			if strings.Join(got, "\n") != strings.Join(test.want, "\n") {
				t.Fatalf("errors = %q, want %q", got, test.want)
			}
		})
	}
}

// Handlers of a heterogeneous failure set: Err(_) handles every member,
// Err(name) needs one declared binding type (S1091), narrowing patterns must
// name a member, and unhandled members propagate one by one.
//
// Rules:
//   - rules/errors/errorhandling.md — §11.1 "Binding a heterogeneous failure set", §16, §17
//   - rules/errors/runtime_checks.md — "Errorhandling revision-2 integration"
func TestTryFailureSetHandlers(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "discard handles everything", body: `
fn Use(values: int[], index: uint, amount: int) int {
    return try values[index] + amount {
        Err(_) => 0
    }
}`},
		{name: "binding needs one type", body: `
fn Use(values: int[], index: uint, amount: int) int {
    return try values[index] + amount {
        Err(failure) => 0
    }
}`, want: []string{"Err(failure) cannot bind the failures of this try because they have different types (IndexError, ArithmeticError) and no common declared error channel"}},
		{name: "declared open channel allows binding", body: `
fn Use(left: int, right: int) int {
    return try Wrap(left + right) {
        Err(failure) => {
            discard failure
            0
        }
    }
}`},
		{name: "partial handler propagates the rest", body: `
fn Use(values: int[], index: uint, amount: int) Result[int, ArithmeticError] {
    let value := try values[index] + amount {
        Err(IndexError.OutOfBounds) => 0
    }
    return Ok(value)
}`},
		{name: "unhandled member must fit", body: `
fn Use(values: int[], index: uint, amount: int) Result[int, IndexError] {
    let value := try values[index] + amount {
        Err(IndexError.OutOfBounds) => 0
    }
    return Ok(value)
}`, want: []string{"try handlers leave ArithmeticError.Overflow, ArithmeticError.DivisionByZero, ArithmeticError.InvalidShift unhandled; they would propagate with return Err, but this function returns Result[int, IndexError]; add Err(_) => ... or map ArithmeticError to IndexError"}},
		{name: "pattern outside the set", body: `
fn Use(values: int[], index: uint, amount: int) int {
    return try values[index] + amount {
        Err(AllocationError.OutOfMemory) => 0
        Err(_) => 1
    }
}`, want: []string{"this try cannot produce AllocationError; its failures are IndexError, ArithmeticError"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := failureSetMessages(t, test.body)
			if len(test.want) == 0 && len(got) != 0 {
				t.Fatalf("errors = %q, want none", got)
			}
			for _, want := range test.want {
				if !strings.Contains(strings.Join(got, "\n"), want) {
					t.Fatalf("errors = %q, want %q", got, want)
				}
			}
		})
	}
}

// The resolved fact records the ordered failure set (operands before their
// operation) without inventing a public error union.
func TestTryFailureSetFactIsOrdered(t *testing.T) {
	source := failureSetPrelude + `
fn Use(values: int[], index: uint, amount: int) Result[int, error] {
    return Ok(try values[index] + amount)
}
`
	program := parser.New(lexer.New(source)).ParseProgram()
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("errors = %v", errors)
	}
	use := program.Statements[len(program.Statements)-1].(*ast.FunctionDeclaration)
	ok := use.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.OkExpression)
	resolved, found := analyzer.ResolvedTryOf(ok.Value.(*ast.TryExpression))
	if !found || resolved.Kind != ResolvedTryFailureSetPropagation || len(resolved.Failures) != 2 {
		t.Fatalf("resolved try = %+v, found=%v", resolved, found)
	}
	if resolved.Failures[0].Kind != TryFailureBounds || resolved.Failures[1].Kind != TryFailureArithmetic {
		t.Fatalf("failure order = %v, %v; want bounds then arithmetic", resolved.Failures[0].Kind, resolved.Failures[1].Kind)
	}
}

// Failures raised by a try inside a handler body or guard are outside the
// outer try's protected set: they propagate on their own, and a diagnostic
// explains that the outer handlers do not catch them. A lambda inside a
// handler is its own function and gets no such explanation.
//
// Rules:
//   - rules/errors/errorhandling.md — §22 "Handler failures are outside the protected set"
func TestHandlerFailuresAreOutsideTheProtectedSet(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

enum ConfigError error {
    NotFound,
}

enum OtherError error {
    Broken,
}

fn LoadConfig() Result[int, ConfigError] {
    return Ok(1)
}

fn LoadDefault() Result[int, OtherError] {
    return Ok(2)
}

fn Valid() Result[bool, OtherError] {
    return Ok(true)
}

fn Body() int {
    return try LoadConfig() {
        Err(_) => try LoadDefault()
    }
}

fn Guard() int {
    return try LoadConfig() {
        Err(ConfigError.NotFound) where try Valid() => 3
        Err(_) => 4
    }
}

fn Propagates() Result[int, OtherError] {
    let value := try LoadConfig() {
        Err(_) => try LoadDefault()
    }
    return Ok(value)
}

fn Outside() int {
    return try LoadDefault()
}
`)
	if len(errors) != 3 {
		t.Fatalf("errors = %v, want body, guard, and outside propagation failures", errors)
	}
	for index, err := range errors {
		insideHandler := index < 2
		hasHint := strings.Contains(err.Help, "The outer try protects only its own expression")
		if !strings.HasPrefix(err.Message, "bodyless try propagates OtherError") || hasHint != insideHandler {
			t.Fatalf("error %d = %q help %q", index, err.Message, err.Help)
		}
	}
}

// Propagation failures of try carry stable IDs with mentor help: S1093 for a
// bodyless try whose failure cannot leave the function, S1094 for failures a
// partial handler set leaves unhandled.
//
// Rules:
//   - rules/errors/errorhandling.md — §12, §16, §30
//   - rules/tooling/diagnostics.md — stable diagnostic IDs
func TestTryPropagationDiagnosticsHaveStableIDs(t *testing.T) {
	errors := analyzeSourceRaw(t, failureSetPrelude+`
enum ReadError error {
    Failed,
    Missing,
}

fn Read() Result[int, ReadError] {
    return Ok(1)
}

fn Bodyless() int {
    return try Read()
}

fn Partial() int {
    return try Read() {
        Err(ReadError.Failed) => 0
    }
}
`)
	if len(errors) != 2 {
		t.Fatalf("errors = %v", errors)
	}
	if errors[0].ID != "S1093" || !strings.Contains(errors[0].Help, "A bodyless try returns the failure from this function") {
		t.Fatalf("bodyless diagnostic = %+v", errors[0])
	}
	if errors[1].ID != "S1094" || !strings.Contains(errors[1].Help, "Handlers in a try are partial") {
		t.Fatalf("residual diagnostic = %+v", errors[1])
	}
}

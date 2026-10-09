package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

const testBoundaryTrySource = `module boundary

type ParseError enum error {
    Empty,
}

type Percent int range 0..100

fn Parse(text: string) Result[int, ParseError] {
    if text == "" {
        return Err(ParseError.Empty)
    }
    return Ok(1)
}

test "bodyless try propagates to the test invocation" {
    let value := try Parse("x")
    let values: int[3] := [1, 2, 3]
    let mut index := 0
    index = value
    let element := try values[index]
    let sum := try value + element
    let mut percent: Percent := 0
    try percent = Percent(sum)
    discard percent
}
`

func analyzeTestBoundarySource(t *testing.T, source string) (*Analyzer, []Error, *ast.Program) {
	t.Helper()
	result := parser.New(lexer.NewWithFile(source, "boundary_test.sec")).Parse()
	if result.HasErrors {
		t.Fatalf("parser diagnostics = %+v", result.Diagnostics)
	}
	analyzer := NewAnalyzer()
	errors := analyzer.Analyze(result.Program)
	return analyzer, errors, result.Program
}

// A test invocation is a compiler-known propagation boundary: a bodyless try
// of any error type is accepted in a test body and resolved as a propagation
// to the test instead of an enclosing Result return.
//
// Rules:
//   - rules/errors/errorhandling.md — §41 "Test propagation boundary"
//   - rules/tooling/testing.md — §10.1 "Test boundary supports try"
func TestBodylessTryPropagatesToTheTestBoundary(t *testing.T) {
	analyzer, errors, program := analyzeTestBoundarySource(t, testBoundaryTrySource)
	if len(errors) != 0 {
		t.Fatalf("errors = %+v", errors)
	}
	kinds := map[ResolvedTryKind]bool{}
	assignments := 0
	for _, statement := range program.Statements {
		test, ok := statement.(*ast.TestDeclaration)
		if !ok {
			continue
		}
		for _, statement := range test.Body.Statements {
			switch statement := statement.(type) {
			case *ast.LetStatement:
				try, ok := statement.Value.(*ast.TryExpression)
				if !ok {
					continue
				}
				resolved, found := analyzer.ResolvedTryOf(try)
				if !found || !resolved.TestBoundary || resolved.EnclosingResultType.Kind != "" {
					t.Errorf("%s resolved = %+v %v", try.String(), resolved, found)
				}
				kinds[resolved.Kind] = true
			case *ast.TryAssignmentStatement:
				resolved, found := analyzer.ResolvedTryAssignmentOf(statement)
				if !found || !resolved.TestBoundary || resolved.Kind != ResolvedTryAssignmentPropagation {
					t.Errorf("try assignment resolved = %+v %v", resolved, found)
				}
				assignments++
			}
		}
	}
	if !kinds[ResolvedTryResultPropagation] || assignments != 1 || len(kinds) < 3 {
		t.Fatalf("resolved kinds = %v, try assignments = %d", kinds, assignments)
	}
}

// The boundary belongs to the test invocation only: defer bodies, lambdas
// inside a test, and ordinary void functions still reject bodyless try.
func TestTestBoundaryDoesNotExtendBeyondTheTestBody(t *testing.T) {
	source := `module boundary

type ParseError enum error {
    Empty,
}

fn Parse(text: string) Result[int, ParseError] {
    return Err(ParseError.Empty)
}

fn Helper() void {
    let value := try Parse("x")
    discard value
}

test "nested contexts keep their own boundary" {
    defer {
        let deferred := try Parse("y")
        discard deferred
    }
    let callback := fn() void {
        let inner := try Parse("z")
        discard inner
    }
    discard callback
}
`
	_, errors, _ := analyzeTestBoundarySource(t, source)
	var messages []string
	for _, err := range errors {
		messages = append(messages, err.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{
		"bodyless try cannot propagate from inside defer",
		"bodyless try propagates ParseError with return Err, but this function returns void",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	if len(errors) != 3 {
		t.Fatalf("errors = %d, want helper, defer and lambda:\n%s", len(errors), joined)
	}
}

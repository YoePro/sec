package sema

import (
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// MD-042: a match over a soundly proven active variant needs only that
// variant; other valid arms stay legal with the A2008 Information, while
// invalid, duplicate and unproven cases keep their errors
// (correction md029-md042-md102 § 17: M042-01..06).
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — §§ 5, 25A
//   - rules/tooling/diagnostics.md — § 24(4e)
func TestReachableStateMatchCoverage(t *testing.T) {
	analyzer, errs := analyzeSourceWithAnalyzerRaw(t, `
module main

type Figure union {
    Dot
    Line(int)
    Circle(int)
}

fn Only() int {
    let figure := Figure.Dot
    match figure {
        Dot => return 0
    }
}

fn Extra() int {
    let figure := Figure.Dot
    match figure {
        Dot => return 0
        Line(length) => return length
        Circle(radius) where radius > 0 => return radius
    }
}
`)
	if len(errs) != 0 {
		t.Fatalf("proven-state matches rejected: %v", errs)
	}
	inactive := 0
	for _, warning := range analyzer.Warnings() {
		if warning.ID != diagnostics.InactiveMatchArm {
			continue
		}
		inactive++
		if warning.Severity != diagnostics.SeverityInformation || !strings.Contains(warning.Help, "figure is proven to contain Figure.Dot") {
			t.Fatalf("inactive-arm advisory lacks its proof: %+v", warning)
		}
	}
	if inactive != 2 {
		t.Fatalf("want two inactive-arm advisories, got %+v", analyzer.Warnings())
	}

	rejected := analyzeSourceRaw(t, `
module main

type Figure union {
    Dot
    Line(int)
}

fn Mutable() int {
    let mut figure := Figure.Dot
    match figure {
        Dot => return 0
    }
}

fn Guarded() int {
    let figure := Figure.Line(3)
    match figure {
        Line(length) where length > 2 => return length
    }
}

fn Duplicate() int {
    let figure := Figure.Dot
    match figure {
        Dot => return 0
        Dot => return 1
    }
}

fn InvalidInactive() int {
    let figure := Figure.Dot
    match figure {
        Dot => return 0
        Line(length) => return "text"
    }
}
`)
	for _, want := range []string{
		"non-exhaustive match for Figure: missing Line",
		"non-exhaustive match for Figure: missing Dot, Line",
		"duplicate match arm for Figure.Dot",
		"must return int, got string",
	} {
		found := false
		for _, err := range rejected {
			found = found || strings.Contains(err.Message, want)
		}
		if !found {
			t.Errorf("missing %q in %v", want, rejected)
		}
	}
}

// Failure provenance (M042-07): an integer division can fail only with
// DivisionByZero or Overflow, so typed handlers for the decimal and shift
// variants are legal but inactive; a Result[int, ArithmeticError] signature
// proves nothing about its implementation's variants.
// Rules: rules/control-flow/flowcontrol_match.md — § 25A.
func TestTryFailureProvenanceInactiveHandlers(t *testing.T) {
	analyzer, errs := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Calculate(a: int) Result[int, ArithmeticError] {
    return Ok(a)
}

fn Divide(a: int, b: int) int {
    return try a / b {
        Err(ArithmeticError.DivisionByZero) => 0
        Err(ArithmeticError.PrecisionExhausted) => 1
        Err(ArithmeticError.Overflow) => 2
        Err(ArithmeticError.InvalidShift) => 3
        Err(ArithmeticError.ScaleOverflow) => 4
    }
}

fn Opaque(a: int) int {
    return try Calculate(a) {
        Err(ArithmeticError.PrecisionExhausted) => 1
        Err(_) => 0
    }
}
`)
	if len(errs) != 0 {
		t.Fatalf("typed handlers rejected: %v", errs)
	}
	inactive := map[string]bool{}
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.InactiveMatchArm {
			inactive[warning.Message] = true
		}
	}
	for _, variant := range []string{"PrecisionExhausted", "InvalidShift", "ScaleOverflow"} {
		if !inactive["this try handler cannot be selected: no protected operation can produce ArithmeticError."+variant] {
			t.Errorf("missing inactive advisory for %s in %v", variant, inactive)
		}
	}
	if len(inactive) != 3 {
		t.Fatalf("want exactly three inactive handlers (none for the opaque call), got %v", inactive)
	}
}

// MD-042 provenance: the match plan keeps the proven variant with its basis
// (construction or a dominating `is` test) and source, and marks exactly the
// arms the proof excludes (semantic_ir.md § 71(6); correction § 15(2)).
func TestReachableStateProofProvenance(t *testing.T) {
	parsed := parser.New(lexer.New(`
module main

type Figure union {
    Dot
    Line(int)
}

fn Constructed() int {
    let figure := Figure.Dot
    match figure {
        Dot => return 0
        Line(length) => return length
    }
}

fn Tested(input: Figure) int {
    let figure := input
    if figure is Line {
        match figure {
            Line(length) => return length
        }
    }
    return 0
}
`))
	program := parsed.ParseProgram()
	analyzer := NewAnalyzer()
	if errs := analyzer.Analyze(program); len(parsed.Errors()) != 0 || len(errs) != 0 {
		t.Fatal(parsed.Errors(), errs)
	}
	plans := map[string]ResolvedMatchPlan{}
	for _, statement := range program.Statements {
		function, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		walkASTValue(reflect.ValueOf(function.Body), func(node any) {
			if expr, ok := node.(*ast.MatchExpression); ok {
				plan, ok := analyzer.ResolvedMatchPlanOf(expr)
				if !ok {
					t.Fatalf("%s: no plan", function.Name.Value)
				}
				plans[function.Name.Value] = plan
			}
		})
	}
	constructed := plans["Constructed"]
	if proof := constructed.ProvenActive; proof == nil || proof.Variant != "Dot" || proof.Binding != "figure" || proof.Basis != VariantProofConstruction || proof.Source.Line != 10 {
		t.Fatalf("construction proof = %+v", constructed.ProvenActive)
	}
	if constructed.Arms[0].ProvenInactive || !constructed.Arms[1].ProvenInactive {
		t.Fatalf("inactive arms = %+v", constructed.Arms)
	}
	tested := plans["Tested"]
	if proof := tested.ProvenActive; proof == nil || proof.Variant != "Line" || proof.Basis != VariantProofStateTest || proof.Source.Line != 19 {
		t.Fatalf("state-test proof = %+v", tested.ProvenActive)
	}
}

package typecompatibility_test

import (
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

type scalar struct{ name, family string }

var scalars = []scalar{
	{"int", "integer"}, {"int8", "integer"}, {"int16", "integer"}, {"int32", "integer"}, {"int64", "integer"}, {"int256", "integer"}, {"int128", "integer"}, {"uint", "integer"}, {"uint8", "integer"}, {"byte", "integer"}, {"uint16", "integer"}, {"uint32", "integer"}, {"uint64", "integer"}, {"uint128", "integer"}, {"uint256", "integer"},
	{"float", "float"}, {"float32", "float"}, {"float64", "float"}, {"decimal", "decimal"}, {"decimal128", "decimal"},
	{"bool", "bool"}, {"string", "string"}, {"char", "char"}, {"rune", "rune"},
	{"Signed", "integer"}, {"SignedOther", "integer"}, {"SignedChain", "integer"}, {"Unsigned", "integer"}, {"Binary", "float"}, {"Exact", "decimal"},
	{"Flag", "bool"}, {"FlagOther", "bool"}, {"Text", "string"}, {"TextOther", "string"}, {"Letter", "char"}, {"LetterOther", "char"}, {"Scalar", "rune"}, {"ScalarOther", "rune"},
}

// TestTypedScalarMatrix audits typed values at initialization, mutation, calls
// and returns using a source-name oracle independent of Sema representation.
// Rules: rules/types/types.md — Type identity, Assignability, Untyped literals versus typed values.
func TestTypedScalarMatrix(t *testing.T) {
	matrix(t, "scalars", func(target, source scalar) bool { return target.name == source.name }, 4)
}

// TestExplicitScalarMatrix audits conversion relations independently of their
// remaining runtime validation/lowering implementation. Typed scalar identity
// stays distinct even when an explicit conversion relation exists.
// Rules: rules/types/types.md — Explicit conversions, char, rune, numeric-to-boolean;
// Numeric-to-string constructor casts are excluded: MD-051 records the
// missing relation; ToString does not specify constructor-cast semantics.
func TestExplicitScalarMatrix(t *testing.T) {
	matrix(t, "conversions", func(target, source scalar) bool {
		numeric := func(f string) bool { return f == "integer" || f == "float" || f == "decimal" }
		if target.family == source.family {
			return true
		}
		switch target.family {
		case "integer":
			return source.family == "decimal" || source.family == "char" || source.family == "rune"
		case "char":
			return source.family == "integer" || source.family == "rune"
		case "rune":
			return source.family == "integer" || source.family == "char"
		case "bool", "string":
			return numeric(source.family)
		case "float", "decimal":
			return numeric(source.family)
		}
		return false
	}, 1)
}

// TestLegacyNumericStringConversions characterizes the pre-existing frontend
// permission without treating it as a normative constructor conversion relation.
// The missing rule is tracked in missing-decisions.yaml MD-051; rules/library/
// core-library.md — ToString defines a separate fallible member operation.
func TestLegacyNumericStringConversions(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/sema/type_compatibility/conversions.sec.in")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"string", "Text"} {
		for _, source := range []string{"int", "float", "decimal"} {
			text := strings.NewReplacer("TARGET", target, "SOURCE", source).Replace(string(data))
			p := parser.New(lexer.NewWithFile(text, "legacy-string-cast.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			if errors := sema.NewAnalyzer().Analyze(program); len(errors) != 0 {
				t.Fatalf("legacy %s(%s): %v", target, source, errors)
			}
		}
	}
}

// matrix instantiates checked-in Sec templates on both scalar plans and all
// analysis depths; every rejected boundary retains source-local diagnostics.
// Rules: rules/compiler/compiler_analysis.md — Source validity at every depth;
// rules/types/types.md — Diagnostics, int and uint.
func matrix(t *testing.T, fixture string, allowed func(scalar, scalar) bool, failures int) {
	t.Helper()
	data, err := os.ReadFile("../../../testdata/sema/type_compatibility/" + fixture + ".sec.in")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		for _, depth := range []sema.AnalysisDepth{sema.AnalysisInteractive, sema.AnalysisStandard, sema.AnalysisDeep} {
			for _, target := range scalars {
				for _, source := range scalars {
					// MD-051: keep undocumented numeric-to-string casts outside the
					// normative matrix; their legacy permission is characterized separately.
					if fixture == "conversions" && target.family == "string" && (source.family == "integer" || source.family == "float" || source.family == "decimal") {
						continue
					}
					text := strings.NewReplacer("TARGET", target.name, "SOURCE", source.name).Replace(string(data))
					p := parser.New(lexer.NewWithFile(text, fixture+".sec"))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					errors := sema.NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth).Analyze(program)
					want := failures
					if allowed(target, source) {
						want = 0
					}
					if len(errors) != want {
						t.Fatalf("%s width=%d depth=%s %s <- %s: got %d want %d: %v", fixture, width, depth, target.name, source.name, len(errors), want, errors)
					}
					for _, e := range errors {
						if e.Line < 16 || e.Column == 0 {
							t.Fatal(e)
						}
					}
				}
			}
		}
	}
}

// TestParameterizedAndAggregateMatrix audits the distinct identity components
// in generic families, fixed/dynamic arrays, references, callables, enum/union,
// registers, temporal and special types. Conversion permission is tested
// separately from assignment, especially explicit base-wrapper conversions.
// Rules: rules/types/types.md — Type categories, Type identity, Assignability,
// Explicit conversions; rules/declarations/struct.md — nominal semantics;
// rules/memory/references.md — reference mode; collections/shaped-types.md — shape identity.
func TestParameterizedAndAggregateMatrix(t *testing.T) {
	cases := []struct {
		target, source  string
		assign, convert bool
	}{
		{"Left", "Left", true, true}, {"Right", "Left", false, false}, {"Derived", "Left", false, true}, {"Left", "DerivedAgain", false, true}, {"Derived", "DerivedAgain", false, true},
		{"Box[int]", "Box[int]", true, true}, {"Box[int]", "Box[string]", false, false},
		{"Fixed", "Box[int]", false, true}, {"Fixed", "Box[string]", false, false}, {"Fixed", "FixedOther", false, true}, {"Fixed", "DifferentlyFixed", false, false}, {"GenericFixed[string]", "Box[int]", false, true}, {"GenericFixed[string]", "Box[string]", false, false}, {"GenericIdentity[Left]", "Left", false, true},
		{"ArrayA", "ArrayB", false, true}, {"ArrayA", "int[2]", false, true}, {"int[2]", "int[3]", false, false}, {"int[2]", "uint[2]", false, false}, {"int[]", "int[2]", false, false}, {"int[]", "int[]", true, true},
		{"ref int", "ref int", true, true}, {"ref int", "ref mut int", false, false}, {"ref mut int", "ref int", false, false}, {"ref int[]", "ref int[]", true, true}, {"ref int[]", "ref int[2]", false, false},
		{"CallbackA", "CallbackB", false, true}, {"fn(int) int", "fn(int) bool", false, false}, {"fn(int) int", "fn(uint) int", false, false}, {"fn(int) int", "mut fn(int) int", false, false},
		{"Word", "Word", true, true}, {"Word", "OtherWord", false, false}, {"Choice", "Choice", true, true}, {"Choice", "OtherChoice", false, false}, {"RegisterA", "RegisterB", false, false},
		{"list[int]", "list[string]", false, false}, {"list[int]", "list[int]", true, true}, {"map[int,bool]", "map[string,bool]", false, false}, {"set[int]", "set[string]", false, false},
		{"vector[int,2]", "vector[int,3]", false, false}, {"matrix[int,2,3]", "matrix[int,3,2]", false, false}, {"tensor[int,2,3,4]", "tensor[int,2,4,3]", false, false}, {"tensor_view[int,2]", "tensor_view[int,3]", false, false},
		{"Shape[2]", "Shape[3]", false, false}, {"Strides[2]", "Strides[3]", false, false}, {"TensorLayout[2]", "TensorLayout[3]", false, false}, {"MemorySpace", "MemorySpace", true, true},
		{"Option[int]", "Option[string]", false, false}, {"Result[int,Failure]", "Result[int,Failure]", true, true}, {"Result[int,Failure]", "Result[int,OtherFailure]", false, false}, {"Result[int,Failure]", "Result[string,Failure]", false, false},
		{"Task[int]", "Task[int]", true, true}, {"Task[int]", "Task[string]", false, false}, {"Thread[int]", "Thread[string]", false, false},
		{"date", "time", false, false}, {"datetime", "duration", false, false}, {"duration", "duration", true, true},
		{"RawPtr[int]", "RawPtr[int]", true, true}, {"RawPtr[int]", "RawPtr[uint]", false, true}, {"RawPtr[void]", "RawPtr[int]", false, true}, {"RawPtr[int]", "uint", false, true}, {"uint", "RawPtr[int]", false, true}, {"ref int", "RawPtr[int]", false, false},
		{"any", "Left", true, true}, {"Left", "any", false, false}, {"error", "Failure", true, false}, {"Failure", "error", false, false},
	}
	data, err := os.ReadFile("../../../testdata/sema/type_compatibility/structured.sec.in")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		for _, depth := range []sema.AnalysisDepth{sema.AnalysisInteractive, sema.AnalysisStandard, sema.AnalysisDeep} {
			for _, c := range cases {
				for _, conversion := range []bool{false, true} {
					action, allowed := "Accept(<-value)", c.assign
					if strings.HasPrefix(c.source, "ref ") {
						action = "Accept(value)"
					}
					if conversion {
						action = "unsafe { let converted := CastTarget(value) }"
						allowed = c.convert
					}
					text := strings.NewReplacer("ACTION", action, "TARGET", c.target, "SOURCE", c.source).Replace(string(data))
					p := parser.New(lexer.NewWithFile(text, "structured.sec"))
					program := p.ParseProgram()
					if len(p.Errors()) > 0 {
						t.Fatal(c, conversion, p.Errors())
					}
					errors := sema.NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth).Analyze(program)
					if (len(errors) == 0) != allowed {
						t.Fatalf("width=%d depth=%s conversion=%v %s <- %s allowed=%v: %v", width, depth, conversion, c.target, c.source, allowed, errors)
					}
					for _, e := range errors {
						if e.Line != 26 {
							t.Fatalf("declaration failed rather than boundary: %v", e)
						}
					}
				}
			}
		}
	}
}

// TestLiteralShapingAndInferredVariadics separates contextual source literals
// from typed values and retains variadic shape from a real named declaration.
// Explicit callable parameter-mode syntax remains owned by functions/lambda governance.
// Rules: rules/types/types.md — Untyped literals versus typed values, Function types;
// rules/declarations/functions.md — native typed variadics.
func TestLiteralShapingAndInferredVariadics(t *testing.T) {
	for _, name := range []string{"literals.sec", "inferred_variadic_invalid.sec"} {
		data, err := os.ReadFile("../../../testdata/sema/type_compatibility/" + name)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{32, 64} {
			for _, depth := range []sema.AnalysisDepth{sema.AnalysisInteractive, sema.AnalysisStandard, sema.AnalysisDeep} {
				p := parser.New(lexer.NewWithFile(string(data), name))
				program := p.ParseProgram()
				if len(p.Errors()) > 0 {
					t.Fatal(p.Errors())
				}
				errors := sema.NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth).Analyze(program)
				if name == "literals.sec" && len(errors) != 0 {
					t.Fatal(errors)
				}
				if name != "literals.sec" && (len(errors) != 1 || errors[0].Line != 6) {
					t.Fatal(errors)
				}
			}
		}
	}
}

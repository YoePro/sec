package sema

import (
	"math/big"
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestCTEMaterializationGraphFromSource exercises the actual CTE handoff:
// nominal types, structs, nested Result/Option, all payload shapes and void.
// Rules: rules/compiler/compile_time_evaluation.md — §§11,30–36,46.
func TestCTEMaterializationGraphFromSource(t *testing.T) {
	file := "../../testdata/sema/cte_materialization_graph_valid.sec"
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	count := 0
	for _, static := range a.ResolvedStaticDeclarations() {
		value, ok := a.CompileTimeValueOf(static.Declaration.Value)
		if !ok {
			t.Fatal(static.Name, "missing value")
		}
		_, failure := prepareCTEMaterialization(value.Value, value.Type)
		if failure != nil {
			t.Fatal(static.Name, failure)
		}
		if static.Name == "Wrapped" && (value.Value.Aggregate.Payload.Aggregate.Variant != "Some" || value.Type.TypeArgs[1].Name != "AppError") {
			t.Fatal(value)
		}
		if static.Name == "Named" && (!value.Type.Named || value.Type.Name != "Concrete") {
			t.Fatal(value)
		}
		count++
	}
	if count != 9 {
		t.Fatal(count)
	}
}

// TestCTEMaterializationGraphRejectsUnsafeGraphs verifies graph properties at
// the compiler service boundary, including states no source evaluator produces
// yet. They must fail closed when future executor representations are added.
// Rules: rules/compiler/compile_time_evaluation.md — §§28–36,48,54,56–57.
func TestCTEMaterializationGraphRejectsUnsafeGraphs(t *testing.T) {
	integer := Type{Name: "int", Kind: IntType}
	scalar := DefaultConstant{Kind: IntType, Integer: big.NewInt(1)}
	box := Type{Name: "Box", Kind: StructType, Fields: []StructField{{Name: "child", Type: integer}}}
	makeBox := func(child DefaultConstant) DefaultConstant {
		return DefaultConstant{Kind: StructType, Aggregate: &CompileTimeAggregate{Type: box, Names: []string{"child"}, Fields: map[string]DefaultConstant{"child": child}, Owner: 17}}
	}
	for _, kind := range []TypeKind{ReferenceType, RawPtrType, SliceType, StringViewType} {
		typ := box
		typ.Fields = []StructField{{Name: "child", Type: Type{Kind: kind, Name: string(kind)}}}
		value := makeBox(DefaultConstant{Kind: kind})
		value.Aggregate.Type = typ
		_, failure := prepareCTEMaterialization(value, typ)
		if failure == nil || failure.Kind != cteResultNotMaterializable || !strings.Contains(failure.Reason, "value.child") {
			t.Fatal(kind, failure)
		}
	}
	cycleType := box
	cycleType.Fields = []StructField{{Name: "child", Type: box}}
	cycle := makeBox(DefaultConstant{})
	cycle.Aggregate.Type = cycleType
	cycle.Aggregate.Fields["child"] = cycle
	_, failure := prepareCTEMaterialization(cycle, cycleType)
	if failure == nil || failure.Kind != cteResultNotMaterializable || !strings.Contains(failure.Reason, "cyclic") {
		t.Fatal(failure)
	}
	for _, value := range []DefaultConstant{
		{Kind: StructType},
		{Kind: StructType, Aggregate: &CompileTimeAggregate{Type: box}},
		{Kind: StructType, Aggregate: &CompileTimeAggregate{Type: box, Names: []string{"wrong"}, Fields: map[string]DefaultConstant{"wrong": scalar}}},
	} {
		_, failure := prepareCTEMaterialization(value, box)
		if failure == nil || failure.Kind != cteCompilerInvariantFailure {
			t.Fatal(failure)
		}
	}
	_, failure = prepareCTEMaterialization(DefaultConstant{Kind: VoidType}, Type{Name: "never", Kind: NeverType})
	if failure == nil || failure.Kind != cteCompilerInvariantFailure {
		t.Fatal(failure)
	}
	unknown := Type{Kind: ArrayType, Name: "array", Element: &integer}
	_, failure = prepareCTEMaterialization(DefaultConstant{Kind: ArrayType}, unknown)
	if failure == nil || failure.Kind != cteImplementationGap {
		t.Fatal(failure)
	}
	original := makeBox(scalar)
	clean, failure := prepareCTEMaterialization(original, box)
	if failure != nil || clean.Aggregate.Owner != 0 || original.Aggregate.Owner != 17 {
		t.Fatal(clean, failure)
	}
	clean.Aggregate.Fields["child"] = DefaultConstant{Kind: IntType, Integer: big.NewInt(9)}
	if original.Aggregate.Fields["child"].Integer.Int64() != 1 {
		t.Fatal("sanitizer modified evaluator storage")
	}
}

// TestCTEMaterializationFailureDiagnostics distinguishes completed execution
// from failed materialization without inventing an unsupported source program.
// It fault-injects an evaluator result at the required-expression boundary.
// Rules: rules/compiler/compile_time_evaluation.md — §§30,36,54,57,59,72.
func TestCTEMaterializationFailureDiagnostics(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/cte_materialization_graph_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "materialization.sec"))
	program := p.ParseProgram()
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, static := range a.ResolvedStaticDeclarations() {
		if static.Name != "Plain" {
			continue
		}
		result := finishCTEMaterialization(cteResult{Type: Type{Kind: ReferenceType, Name: "ref int"}, Value: DefaultConstant{Kind: ReferenceType}})
		if result.Failure == nil || result.Failure.Kind != cteResultNotMaterializable {
			t.Fatal(result)
		}
		a.cteResults[static.Declaration.Value] = result
		before := len(a.errors)
		if !a.reportCompileTimeExecutionFailure(static.Declaration.Value, "static initializer") || len(a.errors) != before+1 {
			t.Fatal(a.errors)
		}
		diagnostic := a.errors[before]
		if diagnostic.ID != diagnostics.CompileTimeResultNotMaterializable || diagnostic.File != "materialization.sec" || !strings.Contains(diagnostic.Message, "was evaluated") || !strings.Contains(diagnostic.Message, "value:") {
			t.Fatal(diagnostic)
		}
		if _, ok := a.CompileTimeValueOf(static.Declaration.Value); ok {
			t.Fatal("failed graph exposed as a successful value")
		}
		definition, ok := diagnostics.Lookup(diagnostic.ID)
		if !ok || !definition.Mandatory || definition.Name != "compile-time.result-not-materializable" {
			t.Fatal(definition, ok)
		}
		return
	}
	t.Fatal("no required root")
}

// TestCTEMaterializationTaggedBoundaries catches hidden stored payloads and
// preserves shared acyclic values as values rather than inventing a heap.
// Rules: rules/compiler/compile_time_evaluation.md — §§32,36–37,48.
func TestCTEMaterializationTaggedBoundaries(t *testing.T) {
	integer := Type{Name: "int", Kind: IntType}
	scalar := DefaultConstant{Kind: IntType, Integer: big.NewInt(1)}
	box := Type{Name: "Box", Kind: StructType, Fields: []StructField{{Name: "n", Type: integer}}}
	shared := DefaultConstant{Kind: StructType, Aggregate: &CompileTimeAggregate{Type: box, Names: []string{"n"}, Fields: map[string]DefaultConstant{"n": scalar}, Owner: 4}}
	pair := Type{Name: "Pair", Kind: StructType, Fields: []StructField{{Name: "a", Type: box}, {Name: "b", Type: box}}}
	source := DefaultConstant{Kind: StructType, Aggregate: &CompileTimeAggregate{Type: pair, Names: []string{"a", "b"}, Fields: map[string]DefaultConstant{"a": shared, "b": shared}, Owner: 5}}
	result, failure := prepareCTEMaterialization(source, pair)
	if failure != nil || result.Aggregate.Owner != 0 || result.Aggregate.Fields["a"].Aggregate.Owner != 0 || result.Aggregate.Fields["b"].Aggregate.Owner != 0 || shared.Aggregate.Owner != 4 {
		t.Fatal(result, failure)
	}
	appError := Type{Name: "AppError", Kind: EnumType}
	resultType := Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{Type{Kind: VoidType}, appError}}
	hidden := shared
	hidden.Kind = VoidType
	_, failure = prepareCTEMaterialization(variantValue(resultType, "Ok", &hidden), resultType)
	if failure == nil || failure.Kind != cteCompilerInvariantFailure {
		t.Fatal(failure)
	}
	_, failure = prepareCTEMaterialization(variantValue(resultType, "Unknown", nil), resultType)
	if failure == nil || failure.Kind != cteCompilerInvariantFailure {
		t.Fatal(failure)
	}
	resultType.TypeArgs[0] = integer
	_, failure = prepareCTEMaterialization(variantValue(resultType, "Ok", nil), resultType)
	if failure == nil || failure.Kind != cteCompilerInvariantFailure {
		t.Fatal(failure)
	}
	collection := Type{Name: "list", Kind: StructType, TypeArgs: []Type{integer}}
	_, failure = prepareCTEMaterialization(DefaultConstant{Kind: StructType}, collection)
	if failure == nil || failure.Kind != cteResultNotMaterializable {
		t.Fatal(failure)
	}
}

// TestCTEMaterializationGraphBudget bounds validation and sanitation of deep
// graphs independently of host stack size, without confusing a budget with
// a language prohibition on a finite aggregate.
// Rules: rules/compiler/compile_time_evaluation.md — §§36,48–49.
func TestCTEMaterializationGraphBudget(t *testing.T) {
	typ := Type{Name: "int", Kind: IntType}
	value := DefaultConstant{Kind: IntType, Integer: big.NewInt(1)}
	for i := 0; i < cteMaxCallDepth+2; i++ {
		typ = Type{Name: "Box", Kind: StructType, Fields: []StructField{{Name: "value", Type: typ}}}
		value = DefaultConstant{Kind: StructType, Aggregate: &CompileTimeAggregate{Type: typ, Names: []string{"value"}, Fields: map[string]DefaultConstant{"value": value}}}
	}
	_, failure := prepareCTEMaterialization(value, typ)
	if failure == nil || failure.Kind != cteResourceLimit || !strings.Contains(failure.Reason, "CallDepth") {
		t.Fatal(failure)
	}
}

package sema

import (
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestResolvedFixedArrayIndexPlans covers SEC-MLIR Package 14 sections 31-36:
// exact wide constants, preserved source index types, named range proofs,
// runtime failure typing, and indexed-write use/action classification.
func TestResolvedFixedArrayIndexPlans(t *testing.T) {
	source, err := os.ReadFile("../../testdata/semantic_ir/fixed_array_index_plans.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "fixed_array_index_plans.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}

	indexExpressions := make([]*ast.IndexExpression, 0, 4)
	for _, statement := range result.Program.Statements {
		function, ok := statement.(*ast.FunctionDeclaration)
		if !ok || function.Body == nil {
			continue
		}
		for _, bodyStatement := range function.Body.Statements {
			indexExpressions = append(indexExpressions, indexesInStatement(bodyStatement)...)
		}
	}
	if len(indexExpressions) != 4 {
		t.Fatalf("index expression count = %d, want 4", len(indexExpressions))
	}

	huge, found := a.ResolvedArrayIndexPlanOf(indexExpressions[0])
	if !found || huge.ArrayLength.String() != "9223372036854775809" || huge.ConstantIndex.String() != "9223372036854775808" || huge.CheckKind != ArrayIndexProvenSafe || huge.ProofKind != ArrayIndexProofConstant || huge.IndexType.Name != "uint256" || huge.IndexSigned || huge.FailureMode != ArrayIndexFailureNone || huge.ErrorType.Kind != "" {
		t.Fatalf("huge constant plan = %#v, found=%t", huge, found)
	}

	dynamic, found := a.ResolvedArrayIndexPlanOf(indexExpressions[1])
	if !found || dynamic.CheckKind != ArrayIndexRuntimeCheck || dynamic.IndexType.Name != "int128" || !dynamic.IndexSigned || dynamic.FailureMode != ArrayIndexFailureOrdinary || dynamic.ErrorType.Name != "IndexError" || !dynamic.ErrorType.ErrorAssignable || dynamic.UseKind != ArrayIndexRead || dynamic.Action != ArrayTransferCopyTrivial {
		t.Fatalf("dynamic signed plan = %#v, found=%t", dynamic, found)
	}

	ranged, found := a.ResolvedArrayIndexPlanOf(indexExpressions[2])
	if !found || ranged.CheckKind != ArrayIndexProvenSafe || ranged.ProofKind != ArrayIndexProofRange || ranged.IndexType.Name != "Slot" || ranged.FailureMode != ArrayIndexFailureNone {
		t.Fatalf("range-proven plan = %#v, found=%t", ranged, found)
	}

	write, found := a.ResolvedArrayIndexPlanOf(indexExpressions[3])
	if !found || write.CheckKind != ArrayIndexRuntimeCheck || write.IndexType.Name != "uint128" || write.IndexSigned || write.UseKind != ArrayIndexWrite || write.Action != ArrayTransferConstructDirect || write.ErrorType.Name != "IndexError" {
		t.Fatalf("write plan = %#v, found=%t", write, found)
	}

	// Query results own their mutable exact integers and unknown queries do not
	// trigger inference or mutate the analyzer.
	huge.ArrayLength.SetInt64(0)
	huge.ConstantIndex.SetInt64(0)
	again, _ := a.ResolvedArrayIndexPlanOf(indexExpressions[0])
	if again.ArrayLength.String() != "9223372036854775809" || again.ConstantIndex.String() != "9223372036854775808" {
		t.Fatalf("query exposed analyzer-owned exact integers: %#v", again)
	}
	count := len(a.resolvedArrayIndexPlans)
	if _, ok := a.ResolvedArrayIndexPlanOf(&ast.IndexExpression{}); ok || len(a.resolvedArrayIndexPlans) != count {
		t.Fatal("unknown read-only query inferred or inserted an index plan")
	}
}

// TestFixedArrayIndexPlansRejectExactWideBounds covers the invalid half of
// Package 14 section 35 without relying on int64 conversion.
func TestFixedArrayIndexPlansRejectExactWideBounds(t *testing.T) {
	source, err := os.ReadFile("../../testdata/semantic_ir/fixed_array_index_plans_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "fixed_array_index_plans_invalid.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	errors := a.Analyze(result.Program)
	joined := joinedSemaErrors(errors)
	for _, want := range []string{
		"array index -1 is out of bounds for int[4]",
		"array index 4 is out of bounds for int[4]",
		"array index 9223372036854775810 is out of bounds for int[9223372036854775809]",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in diagnostics: %v", want, errors)
		}
	}
}

// TestPackage14Section95ConstantIndexMatrix locks every valid constant-index
// class required by SEC-MLIR Package 14 section 95. Constants and array lengths
// remain exact arbitrary-precision facts; none is reclassified as a runtime
// check merely because its source type or value is wider than the host.
func TestPackage14Section95ConstantIndexMatrix(t *testing.T) {
	source := `module main
fn Zero(values: int32[1]) int32 { return values[0] }
fn Last(values: int32[4]) int32 { return values[3] }
fn Signed128(values: int32[4]) int32 { return values[int128(2)] }
fn Unsigned128(values: int32[4]) int32 { return values[uint128(2)] }
fn Unsigned256(values: int32[4]) int32 { return values[uint256(2)] }
fn AboveInt64(values: int32[9223372036854775809]) int32 {
    return values[uint256(9223372036854775808)]
}
`
	p := parser.New(lexer.NewWithFile(source, "package14-section95.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}

	wants := []struct {
		function, constant, length, indexType string
	}{
		{"Zero", "0", "1", ""},
		{"Last", "3", "4", ""},
		{"Signed128", "2", "4", "int128"},
		{"Unsigned128", "2", "4", "uint128"},
		{"Unsigned256", "2", "4", "uint256"},
		{"AboveInt64", "9223372036854775808", "9223372036854775809", "uint256"},
	}
	for index, want := range wants {
		function := result.Program.Statements[index+1].(*ast.FunctionDeclaration)
		indexes := indexesInStatement(function.Body.Statements[0])
		if function.Name.Value != want.function || len(indexes) != 1 {
			t.Fatalf("function %d = %s with %d indexes, want %s with one", index, function.Name.Value, len(indexes), want.function)
		}
		plan, found := a.ResolvedArrayIndexPlanOf(indexes[0])
		if !found || plan.CheckKind != ArrayIndexProvenSafe || plan.ProofKind != ArrayIndexProofConstant || plan.FailureMode != ArrayIndexFailureNone || plan.ConstantIndex == nil || plan.ConstantIndex.String() != want.constant || plan.ArrayLength == nil || plan.ArrayLength.String() != want.length {
			t.Fatalf("%s constant plan = %#v, found=%t", want.function, plan, found)
		}
		if want.indexType != "" && plan.IndexType.Name != want.indexType {
			t.Fatalf("%s index type = %s, want %s", want.function, plan.IndexType.Name, want.indexType)
		}
	}
}

// SEC-MLIR Package 14 sections 32, 36, and 96 require dominating branch and
// assertion refinements to become explicit proof provenance. Incomplete and
// zero-length proofs must remain runtime checked.
func TestResolvedFixedArrayIndexRefinementProofs(t *testing.T) {
	source, err := os.ReadFile("../../testdata/semantic_ir/fixed_array_refinement_proofs.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "fixed_array_refinement_proofs.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}

	wants := []struct {
		function string
		check    ArrayIndexCheckKind
		proof    ArrayIndexProofKind
	}{
		{function: "Branch", check: ArrayIndexProvenSafe, proof: ArrayIndexProofBranch},
		{function: "Asserted", check: ArrayIndexProvenSafe, proof: ArrayIndexProofOther},
		{function: "UnsignedBranch", check: ArrayIndexProvenSafe, proof: ArrayIndexProofBranch},
		{function: "MissingLower", check: ArrayIndexRuntimeCheck},
		{function: "ZeroLength", check: ArrayIndexRuntimeCheck},
		{function: "MutableIndex", check: ArrayIndexRuntimeCheck},
		{function: "OutsideBranch", check: ArrayIndexRuntimeCheck},
		{function: "Disjunction", check: ArrayIndexRuntimeCheck},
	}
	for index, want := range wants {
		function := result.Program.Statements[index+1].(*ast.FunctionDeclaration)
		if function.Name.Value != want.function {
			t.Fatalf("function %d = %s, want %s", index, function.Name.Value, want.function)
		}
		indexes := make([]*ast.IndexExpression, 0, 1)
		for _, statement := range function.Body.Statements {
			indexes = append(indexes, indexesInStatement(statement)...)
		}
		if len(indexes) != 1 {
			t.Fatalf("%s index count = %d, want 1", want.function, len(indexes))
		}
		plan, found := a.ResolvedArrayIndexPlanOf(indexes[0])
		if !found || plan.CheckKind != want.check || plan.ProofKind != want.proof {
			t.Fatalf("%s plan = %#v, found=%t", want.function, plan, found)
		}
	}

	contracted := builtinTypes()["int"]
	contracted.Contracts = []Contract{RangeContract{Min: big.NewInt(0), Max: big.NewInt(4), Exclusive: true}}
	if proof, proven := integerTypeRangeArrayIndexProof(contracted, big.NewInt(4)); !proven || proof != ArrayIndexProofContract {
		t.Fatalf("inline contract proof = %q, proven=%t", proof, proven)
	}
}

// TestResolvedArrayLiteralPlanFactCopiesCompactExactEntries verifies the
// immutable compiler-owned fact model required by SEC-MLIR Package 14 sections
// 14-16 before P14-20 makes array-literal analysis populate it.
func TestResolvedArrayLiteralPlanFactCopiesCompactExactEntries(t *testing.T) {
	analyzer := NewAnalyzer()
	literal := &ast.ArrayLiteral{}
	largeLength, ok := new(big.Int).SetString("18446744073709551616", 10)
	if !ok {
		t.Fatal("could not construct exact test length")
	}
	plan := ResolvedArrayLiteralPlan{
		ElementType: Type{Name: "int", Kind: IntType},
		Length:      new(big.Int).Add(largeLength, big.NewInt(1)),
		Entries: []ResolvedArrayLiteralEntry{
			{
				SourceIndex: 0,
				Kind:        ArrayLiteralSpread,
				Type:        NewFixedArrayType(Type{Name: "int", Kind: IntType}, largeLength),
				Length:      largeLength,
				Action:      ArrayTransferCopyTrivial,
			},
			{
				SourceIndex: 1,
				Kind:        ArrayLiteralElement,
				Type:        Type{Name: "int", Kind: IntType},
				Length:      big.NewInt(1),
				Action:      ArrayTransferConstructDirect,
			},
		},
	}

	analyzer.recordResolvedArrayLiteralPlan(literal, plan)
	largeLength.SetInt64(0)
	plan.Length.SetInt64(0)
	plan.Entries[1].Length.SetInt64(99)
	plan.Entries = nil

	recorded, found := analyzer.resolvedArrayLiteralPlans[literal]
	if !found || len(recorded.Entries) != 2 {
		t.Fatalf("recorded compact entries = %#v, found=%t", recorded, found)
	}
	if recorded.Length.String() != "18446744073709551617" || recorded.Entries[0].Length.String() != "18446744073709551616" || recorded.Entries[1].Length.String() != "1" {
		t.Fatalf("recorded exact lengths = total %v, entries %#v", recorded.Length, recorded.Entries)
	}
	if recorded.Entries[0].Kind != ArrayLiteralSpread || recorded.Entries[0].Action != ArrayTransferCopyTrivial || recorded.Entries[1].Kind != ArrayLiteralElement || recorded.Entries[1].Action != ArrayTransferConstructDirect {
		t.Fatalf("recorded source entries = %#v", recorded.Entries)
	}
}

// TestResolvedArrayLiteralPlanQueryIsReadOnlyAndCompact covers SEC-MLIR
// Package 14 section 17: querying clones mutable storage, performs no Sema
// work, and keeps one record per source entry even for enormous lengths.
func TestResolvedArrayLiteralPlanQueryIsReadOnlyAndCompact(t *testing.T) {
	analyzer := NewAnalyzer()
	literal := &ast.ArrayLiteral{}
	hugeLength, ok := new(big.Int).SetString("100000000000000000000000000000000000000000000000000", 10)
	if !ok {
		t.Fatal("could not construct exact test length")
	}
	analyzer.recordResolvedArrayLiteralPlan(literal, ResolvedArrayLiteralPlan{
		ElementType: Type{Name: "byte", Kind: UintType},
		Length:      hugeLength,
		Entries: []ResolvedArrayLiteralEntry{
			{SourceIndex: 0, Kind: ArrayLiteralSpread, Type: NewFixedArrayType(Type{Name: "byte", Kind: UintType}, hugeLength), Length: hugeLength, Action: ArrayTransferCopyTrivial},
			{SourceIndex: 1, Kind: ArrayLiteralElement, Type: Type{Name: "byte", Kind: UintType}, Length: big.NewInt(1), Action: ArrayTransferConstructDirect},
		},
	})

	planCount := len(analyzer.resolvedArrayLiteralPlans)
	expressionTypeCount := len(analyzer.expressionTypes)
	queried, found := analyzer.ResolvedArrayLiteralPlanOf(literal)
	if !found || len(queried.Entries) != 2 || queried.Length.String() != hugeLength.String() {
		t.Fatalf("queried compact plan = %#v, found=%t", queried, found)
	}
	if len(analyzer.resolvedArrayLiteralPlans) != planCount || len(analyzer.expressionTypes) != expressionTypeCount {
		t.Fatal("read-only array literal query mutated analyzer state")
	}

	queried.Length.SetInt64(0)
	queried.Entries[0].Length.SetInt64(0)
	queried.Entries[0].Kind = ArrayLiteralElement
	queried.Entries = append(queried.Entries, ResolvedArrayLiteralEntry{})
	again, found := analyzer.ResolvedArrayLiteralPlanOf(literal)
	if !found || len(again.Entries) != 2 || again.Length.String() != hugeLength.String() || again.Entries[0].Length.String() != hugeLength.String() || again.Entries[0].Kind != ArrayLiteralSpread {
		t.Fatalf("query exposed analyzer-owned plan storage: %#v", again)
	}

	unknownPlans := len(analyzer.resolvedArrayLiteralPlans)
	if _, found := analyzer.ResolvedArrayLiteralPlanOf(&ast.ArrayLiteral{}); found {
		t.Fatal("unrecorded array literal unexpectedly had a plan")
	}
	if len(analyzer.resolvedArrayLiteralPlans) != unknownPlans {
		t.Fatal("unknown query inserted analyzer state")
	}
	var nilAnalyzer *Analyzer
	if _, found := nilAnalyzer.ResolvedArrayLiteralPlanOf(literal); found {
		t.Fatal("nil analyzer unexpectedly returned an array literal plan")
	}
}

// TestArrayLiteralAnalysisPopulatesAuthoritativePlans covers SEC-MLIR Package
// 14 sections 18-20: inferred, target-shaped, spread, and target-empty literals
// all publish the exact source-order plan produced by ordinary Sema analysis.
func TestArrayLiteralAnalysisPopulatesAuthoritativePlans(t *testing.T) {
	source := `module main
fn Source() int[2] { return [2, 3] }
fn Build(source: int[2]) int[5] { return [1, source..., 4, 5] }
fn Empty() int[0] { return [] }
fn Inferred() int[2] {
  let values := [7, 8]
  return values
}`
	parser := parser.New(lexer.NewWithFile(source, "array-literal-plans.sec"))
	result := parser.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", parser.Errors())
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}

	sourceLiteral := result.Program.Statements[1].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.ArrayLiteral)
	buildLiteral := result.Program.Statements[2].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.ArrayLiteral)
	emptyLiteral := result.Program.Statements[3].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.ArrayLiteral)
	inferredLiteral := result.Program.Statements[4].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.LetStatement).Value.(*ast.ArrayLiteral)

	for name, literal := range map[string]*ast.ArrayLiteral{
		"source": sourceLiteral, "build": buildLiteral, "empty": emptyLiteral, "inferred": inferredLiteral,
	} {
		if _, found := analyzer.ResolvedArrayLiteralPlanOf(literal); !found {
			t.Fatalf("%s literal has no resolved plan", name)
		}
	}

	build, _ := analyzer.ResolvedArrayLiteralPlanOf(buildLiteral)
	if build.ElementType.Name != "int" || build.Length.String() != "5" || len(build.Entries) != 4 {
		t.Fatalf("build plan = %#v", build)
	}
	wantKinds := []ResolvedArrayLiteralEntryKind{ArrayLiteralElement, ArrayLiteralSpread, ArrayLiteralElement, ArrayLiteralElement}
	wantActions := []ResolvedArrayTransferAction{ArrayTransferConstructDirect, ArrayTransferCopyTrivial, ArrayTransferConstructDirect, ArrayTransferConstructDirect}
	wantLengths := []string{"1", "2", "1", "1"}
	for index, entry := range build.Entries {
		if entry.SourceIndex != index || entry.Kind != wantKinds[index] || entry.Action != wantActions[index] || entry.Length.String() != wantLengths[index] {
			t.Fatalf("build entry %d = %#v", index, entry)
		}
	}
	spreadLength, fixedSpread := exactFixedArrayLength(build.Entries[1].Type)
	if build.Entries[1].Type.Kind != ArrayType || !fixedSpread || spreadLength.String() != "2" {
		t.Fatalf("spread source type = %#v", build.Entries[1].Type)
	}

	empty, _ := analyzer.ResolvedArrayLiteralPlanOf(emptyLiteral)
	if empty.ElementType.Name != "int" || empty.Length.Sign() != 0 || len(empty.Entries) != 0 {
		t.Fatalf("target-empty plan = %#v", empty)
	}
	inferred, _ := analyzer.ResolvedArrayLiteralPlanOf(inferredLiteral)
	if inferred.ElementType.Name != "int" || inferred.Length.String() != "2" || len(inferred.Entries) != 2 {
		t.Fatalf("inferred plan = %#v", inferred)
	}
}

// TestArrayLiteralPlansPreserveMultipleAndHugeSpreads covers SEC-MLIR Package
// 14 sections 20-23: every spread remains one source entry, exact lengths add
// without host-width conversion, and plan size follows source size rather than N.
func TestArrayLiteralPlansPreserveMultipleAndHugeSpreads(t *testing.T) {
	const huge = "9223372036854775808"
	const hugePlusOne = "9223372036854775809"
	source := `module main
fn Merge(left: int[2], right: int[3]) int[7] {
  return [0, left..., 1, right...]
}
fn Huge(source: int[` + huge + `]) int[` + hugePlusOne + `] {
  return [source..., 9]
}`
	parser := parser.New(lexer.NewWithFile(source, "array-literal-spread-plans.sec"))
	result := parser.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", parser.Errors())
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}

	mergeLiteral := result.Program.Statements[1].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.ArrayLiteral)
	merge, found := analyzer.ResolvedArrayLiteralPlanOf(mergeLiteral)
	if !found || merge.Length.String() != "7" || len(merge.Entries) != 4 {
		t.Fatalf("multiple-spread plan = %#v, found=%t", merge, found)
	}
	wantKinds := []ResolvedArrayLiteralEntryKind{ArrayLiteralElement, ArrayLiteralSpread, ArrayLiteralElement, ArrayLiteralSpread}
	wantLengths := []string{"1", "2", "1", "3"}
	for index, entry := range merge.Entries {
		if entry.SourceIndex != index || entry.Kind != wantKinds[index] || entry.Length.String() != wantLengths[index] {
			t.Fatalf("multiple-spread entry %d = %#v", index, entry)
		}
	}

	hugeLiteral := result.Program.Statements[2].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.ArrayLiteral)
	hugePlan, found := analyzer.ResolvedArrayLiteralPlanOf(hugeLiteral)
	if !found || hugePlan.Length.String() != hugePlusOne || hugePlan.Length.IsInt64() || len(hugePlan.Entries) != 2 {
		t.Fatalf("huge compact spread plan = %#v, found=%t", hugePlan, found)
	}
	if hugePlan.Entries[0].Kind != ArrayLiteralSpread || hugePlan.Entries[0].Length.String() != huge || hugePlan.Entries[1].Kind != ArrayLiteralElement {
		t.Fatalf("huge source entries = %#v", hugePlan.Entries)
	}
}

func TestArrayLiteralPlansRejectRuntimeAndMismatchedSpreadSources(t *testing.T) {
	// SEC-MLIR Package 14 sections 18 and 21 keep runtime-length sources out
	// of fixed-array literals and require exact element identity when inferred.
	errors := analyzeSourceRaw(t, `module main
fn Runtime(values: ref int[]) void {
  let result := [values...]
}
fn Mismatch(left: int[1], right: bool[1]) void {
  let result := [left..., right...]
}`)
	joined := joinedSemaErrors(errors)
	if !strings.Contains(joined, "cannot spread ref int[] into fixed-array literal; expansion count is not known at compile time") ||
		!strings.Contains(joined, "array literal elements must have one identical type") {
		t.Fatalf("spread rejection diagnostics = %v", errors)
	}
}

// TestArraySpreadTransferActionVocabulary covers SEC-MLIR Package 14 section
// 21: Sema records the actual copy fact, while consuming and conditional cases
// are rejected instead of being mislabeled as trivial copies.
func TestArraySpreadTransferActionVocabulary(t *testing.T) {
	tests := []struct {
		classification CopyClassification
		want           ResolvedArrayTransferAction
		ok             bool
	}{
		{CopyTrivial, ArrayTransferCopyTrivial, true},
		{CopySemantic, ArrayTransferCopySemantic, true},
		{CopyMoveOnly, "", false},
		{CopyConditional, "", false},
		{CopyNonCopyable, "", false},
	}
	for _, test := range tests {
		got, ok := arraySpreadTransferAction(test.classification)
		if got != test.want || ok != test.ok {
			t.Fatalf("spread action for %q = %q, %t; want %q, %t", test.classification, got, ok, test.want, test.ok)
		}
	}
}

// analyzeStructPlanWithLegacyDefaults exercises the compatibility boundary in
// rules/mlir/packages/sec-mlir-dialect_package13.md section 26. The returned
// plan must be independent of the optional synthesized AST fields.
func analyzeStructPlanWithLegacyDefaults(t *testing.T, source string, materialize bool) (ResolvedStructLiteralPlan, int) {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "struct-default-plan.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	a.SetLegacyASTDefaultMaterialization(materialize)
	if errs := a.Analyze(result.Program); len(errs) != 0 {
		t.Fatalf("sema: %v", errs)
	}
	function := result.Program.Statements[2].(*ast.FunctionDeclaration)
	literal := function.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.StructLiteral)
	plan, ok := a.ResolvedStructLiteralPlanOf(literal)
	if !ok {
		t.Fatal("missing resolved struct literal plan")
	}
	return plan, len(literal.Fields)
}

func TestResolvedStructPlanIgnoresLegacyASTDefaultMaterialization(t *testing.T) {
	source := `module main
type Defaults struct { Count: int, Enabled: bool }
fn Build() Defaults { return Defaults {} }`
	withLegacy, materializedFields := analyzeStructPlanWithLegacyDefaults(t, source, true)
	withoutLegacy, sourceFields := analyzeStructPlanWithLegacyDefaults(t, source, false)
	if materializedFields != 2 || sourceFields != 0 {
		t.Fatalf("legacy fields = %d, source-only fields = %d", materializedFields, sourceFields)
	}
	if !reflect.DeepEqual(withLegacy, withoutLegacy) {
		t.Fatalf("semantic plan depends on legacy AST materialization:\nwith: %#v\nwithout: %#v", withLegacy, withoutLegacy)
	}
}

func TestSemanticFactsRetainBindingAndExactCallTarget(t *testing.T) {
	source := `module main
fn Value(value: int) int { return value }
fn Value(value: bool) bool { return value }
fn Main(flag: bool) bool {
  let local := flag
  return Value(local)
}`
	p := parser.New(lexer.NewWithFile(source, "facts.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(result.Program); len(errs) > 0 {
		t.Fatalf("sema: %v", errs)
	}
	mainFn := result.Program.Statements[3].(*ast.FunctionDeclaration)
	let := mainFn.Body.Statements[0].(*ast.LetStatement)
	ret := mainFn.Body.Statements[1].(*ast.ReturnStatement)
	call := ret.Value.(*ast.CallExpression)
	use := call.Arguments[0].(*ast.Identifier)
	declFact, ok := a.ResolvedBindingOf(let.Name)
	if !ok {
		t.Fatal("local declaration has no BindingID")
	}
	useFact, ok := a.ResolvedBindingOf(use)
	if !ok {
		t.Fatal("local use has no BindingID")
	}
	if declFact.ID == 0 || declFact.ID != useFact.ID || declFact.Kind != BindingLocal {
		t.Fatalf("declaration=%#v use=%#v", declFact, useFact)
	}
	resolved, ok := a.ResolvedCallTarget(call)
	if !ok {
		t.Fatal("call target was not retained")
	}
	if resolved.Kind != ResolvedDirectCall || resolved.Function.Name != "Value" || len(resolved.Function.Parameters) != 1 || resolved.Function.Parameters[0].Type.Kind != BoolType {
		t.Fatalf("resolved call = %#v", resolved)
	}
	before := len(a.expressionTypes)
	if _, ok := a.ResolvedTypeOf(use); !ok {
		t.Fatal("resolved expression type missing")
	}
	if len(a.expressionTypes) != before {
		t.Fatal("read-only query mutated analyzer")
	}
}

func TestResolvedCallableCapabilityFactsAreCompilerOwned(t *testing.T) {
	source := `module main
fn Apply(mutable: mut fn(int) int, consuming: -> fn() int) int {
  discard mutable(1)
  return consuming()
}`
	p := parser.New(lexer.NewWithFile(source, "callable-facts.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(result.Program); len(errs) > 0 {
		t.Fatalf("sema: %v", errs)
	}
	function := result.Program.Statements[1].(*ast.FunctionDeclaration)
	mutableCall := function.Body.Statements[0].(*ast.DiscardStatement).Value.(*ast.CallExpression)
	consumingCall := function.Body.Statements[1].(*ast.ReturnStatement).Value.(*ast.CallExpression)

	mutable, ok := a.ResolvedCallableCapabilityOf(mutableCall.Callee)
	if !ok || mutable.Capability != CallableMutable || mutable.Spelling != "mut fn" || mutable.ConsumesCallable {
		t.Fatalf("mutable callable fact = %#v, %t", mutable, ok)
	}
	consuming, ok := a.ResolvedCallableCapabilityOf(consumingCall.Callee)
	if !ok || consuming.Capability != CallableConsuming || consuming.Spelling != "-> fn" || !consuming.ConsumesCallable {
		t.Fatalf("consuming callable fact = %#v, %t", consuming, ok)
	}
	if consuming.InvocationRequirement != "owned callable access" {
		t.Fatalf("consuming invocation requirement = %q", consuming.InvocationRequirement)
	}
}

func TestResolvedStructPlansPreserveSourceOrderOverridesDefaultsAndMembers(t *testing.T) {
	// rules/declarations/struct.md section 4 requires read-only semantic
	// consumers to retain ordered open-key metadata and raw escape spelling.
	source := `module main
type Settings struct { Count: int ` + "`wire:\"signed\\\"little\" json:\"count value\"`" + `, Enabled: bool, Limit: uint }
fn Build(base: Settings) int {
  let merged := Settings { base..., Enabled: true }
  let defaults := Settings { Count: 4 }
  return merged.Count
}`
	p := parser.New(lexer.NewWithFile(source, "struct-plan.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(result.Program); len(errs) > 0 {
		t.Fatalf("sema: %v", errs)
	}
	function := result.Program.Statements[2].(*ast.FunctionDeclaration)
	merged := function.Body.Statements[0].(*ast.LetStatement).Value.(*ast.StructLiteral)
	defaults := function.Body.Statements[1].(*ast.LetStatement).Value.(*ast.StructLiteral)
	member := function.Body.Statements[2].(*ast.ReturnStatement).Value.(*ast.MemberExpression)

	plan, ok := a.ResolvedStructLiteralPlanOf(merged)
	if !ok || !plan.FullyInitialized || len(plan.Entries) != 2 || len(plan.FinalFields) != 3 {
		t.Fatalf("merged plan = %#v, %t", plan, ok)
	}
	if plan.Entries[0].Kind != StructEntrySpread || plan.Entries[0].SourceIndex != 0 || plan.Entries[1].Kind != StructEntryExplicit || plan.Entries[1].SourceIndex != 1 {
		t.Fatalf("source entries = %#v", plan.Entries)
	}
	if plan.FinalFields[0].SourceKind != StructFieldSourceSpread || plan.FinalFields[1].SourceKind != StructFieldSourceExplicit || plan.FinalFields[2].SourceKind != StructFieldSourceSpread {
		t.Fatalf("merged final fields = %#v", plan.FinalFields)
	}

	defaultPlan, ok := a.ResolvedStructLiteralPlanOf(defaults)
	if !ok || defaultPlan.FinalFields[0].SourceKind != StructFieldSourceExplicit || defaultPlan.FinalFields[1].SourceKind != StructFieldSourceDefault || defaultPlan.FinalFields[2].SourceKind != StructFieldSourceDefault {
		t.Fatalf("default plan = %#v, %t", defaultPlan, ok)
	}
	defaultPlan.FinalFields = nil
	again, _ := a.ResolvedStructLiteralPlanOf(defaults)
	if len(again.FinalFields) != 3 {
		t.Fatal("read-only struct plan query exposed analyzer slice storage")
	}

	memberPlan, ok := a.ResolvedStructMemberOf(member)
	if !ok || memberPlan.Kind != MemberStoredField || memberPlan.FieldID != 0 || memberPlan.FieldName != "Count" || memberPlan.Action != StructFieldCopyTrivial {
		t.Fatalf("member plan = %#v, %t", memberPlan, ok)
	}
	if len(memberPlan.Tags) != 2 || memberPlan.Tags[0].Value != `signed\"little` || memberPlan.Tags[1].Value != "count value" {
		t.Fatalf("member tags = %#v", memberPlan.Tags)
	}
	memberPlan.Tags[0].Value = "mutated"
	againMember, _ := a.ResolvedStructMemberOf(member)
	if againMember.Tags[0].Value != `signed\"little` {
		t.Fatalf("read-only member metadata exposed analyzer storage: %#v", againMember.Tags)
	}
	metadata, ok := a.ResolvedStructFieldAt(memberPlan.OwnerType.Fields[0].Token)
	if !ok || metadata.FieldID != 0 || len(metadata.Field.Tags) != 2 || metadata.Field.Tags[1].Key != "json" {
		t.Fatalf("resolved field metadata = %#v, %t", metadata, ok)
	}
	metadata.Field.Tags[0].Value = "mutated"
	againMetadata, _ := a.ResolvedStructFieldAt(memberPlan.OwnerType.Fields[0].Token)
	if againMetadata.Field.Tags[0].Value != `signed\"little` {
		t.Fatalf("read-only field metadata exposed analyzer storage: %#v", againMetadata.Field.Tags)
	}
}

// rules/types/default_values.md and Package 13 sections 27 and 75 require the
// plan to retain canonical defaults, including recursive and constrained ones,
// rather than reconstructing them in a backend.
func TestResolvedStructPlansCoverDefaultMatrix(t *testing.T) {
	source := `module main
type Port int range 1..65535 default 8080
type Positive int range 1..10
type Inner struct { Value: Port }
type Config struct { Inner: Inner, Enabled: bool, Port: Port, Positive: Positive }
fn Build() Config {
  let all := Config {}
  return Config { Enabled: true }
}`
	p := parser.New(lexer.NewWithFile(source, "struct-default-matrix.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	a.SetLegacyASTDefaultMaterialization(false)
	if errs := a.Analyze(result.Program); len(errs) != 0 {
		t.Fatalf("sema: %v", errs)
	}
	function := result.Program.Statements[5].(*ast.FunctionDeclaration)
	all := function.Body.Statements[0].(*ast.LetStatement).Value.(*ast.StructLiteral)
	partial := function.Body.Statements[1].(*ast.ReturnStatement).Value.(*ast.StructLiteral)
	allPlan, allOK := a.ResolvedStructLiteralPlanOf(all)
	partialPlan, partialOK := a.ResolvedStructLiteralPlanOf(partial)
	if !allOK || !partialOK || len(allPlan.Entries) != 0 || len(allPlan.FinalFields) != 4 {
		t.Fatalf("all=%#v (%t), partial=%#v (%t)", allPlan, allOK, partialPlan, partialOK)
	}
	wantDefaults := []DefaultKind{StructDefault, PrimitiveDefault, ExplicitTypeDefault, RangeDefault}
	for index, want := range wantDefaults {
		if field := allPlan.FinalFields[index]; field.SourceKind != StructFieldSourceDefault || field.Default.Kind != want {
			t.Fatalf("all default field %d = %#v, want %s", index, field, want)
		}
	}
	if partialPlan.FinalFields[1].SourceKind != StructFieldSourceExplicit || partialPlan.FinalFields[0].Default.Kind != StructDefault || partialPlan.FinalFields[2].Default.Kind != ExplicitTypeDefault {
		t.Fatalf("partial plan = %#v", partialPlan)
	}
}

func TestResolvedStructPlanRejectsNonDefaultableOmission(t *testing.T) {
	errs := analyzeSourceRaw(t, `module main
type Required struct { Value: ref int }
fn Build() Required { return Required {} }`)
	if len(errs) == 0 || !strings.Contains(errs[0].Message, "has no default value and must be initialized") {
		t.Fatalf("errors = %v", errs)
	}
}

// rules/declarations/spread.md and Package 13 sections 24-25 and 75 require
// source entries to stay ordered while final fields use declaration order.
func TestResolvedStructPlanUsesLaterSpreadWithoutOverwritingExplicitField(t *testing.T) {
	source := `module main
type Triple struct { First: int, Second: int, Third: int }
fn Build(first: Triple, second: Triple) Triple {
  return Triple { first..., Second: 20, second... }
}`
	p := parser.New(lexer.NewWithFile(source, "struct-multiple-spreads.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(result.Program); len(errs) != 0 {
		t.Fatalf("sema: %v", errs)
	}
	function := result.Program.Statements[2].(*ast.FunctionDeclaration)
	literal := function.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.StructLiteral)
	plan, ok := a.ResolvedStructLiteralPlanOf(literal)
	if !ok || len(plan.Entries) != 3 || len(plan.FinalFields) != 3 {
		t.Fatalf("plan = %#v, %t", plan, ok)
	}
	for index, entry := range plan.Entries {
		if entry.SourceIndex != index {
			t.Fatalf("source entries = %#v", plan.Entries)
		}
	}
	if plan.FinalFields[0].FieldName != "First" || plan.FinalFields[0].SourceEntryIndex != 2 ||
		plan.FinalFields[1].FieldName != "Second" || plan.FinalFields[1].SourceKind != StructFieldSourceExplicit || plan.FinalFields[1].SourceEntryIndex != 1 ||
		plan.FinalFields[2].FieldName != "Third" || plan.FinalFields[2].SourceEntryIndex != 2 {
		t.Fatalf("final fields = %#v", plan.FinalFields)
	}
}

// rules/declarations/properties.md and Package 13 sections 32 and 39 keep
// property syntax out of the stored-field lowering path.
func TestResolvedStructMemberDistinguishesPropertyFromStoredField(t *testing.T) {
	source := `module main
type Meter struct { Stored: int }
impl Meter {
  property Value: int { get { return self.Stored } }
}
fn Read(meter: Meter) int { return meter.Value }`
	p := parser.New(lexer.NewWithFile(source, "struct-property-member.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(result.Program); len(errs) != 0 {
		t.Fatalf("sema: %v", errs)
	}
	function := result.Program.Statements[3].(*ast.FunctionDeclaration)
	member := function.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.MemberExpression)
	plan, ok := a.ResolvedStructMemberOf(member)
	if !ok || plan.Kind != MemberProperty || plan.FieldName != "Value" || plan.Action != "" {
		t.Fatalf("property member plan = %#v, %t", plan, ok)
	}
}

func TestResolvedMatchPlanIsReadOnlyAndNumeric(t *testing.T) {
	source := `module main
enum Flag: bit[1] { Off = 0, On = 1 }
fn Choose(flag: Flag, condition: bool) int {
  return match flag {
    Flag.Off where condition => 10
    Flag.Off => 20
    Flag.On => 30
  }
}`
	p := parser.New(lexer.NewWithFile(source, "match-plan.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(result.Program); len(errs) > 0 {
		t.Fatalf("sema: %v", errs)
	}
	function := result.Program.Statements[2].(*ast.FunctionDeclaration)
	match := function.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.MatchExpression)
	before := len(a.resolvedMatchPlans)
	plan, ok := a.ResolvedMatchPlanOf(match)
	if !ok || plan.SubjectKind != MatchSubjectEnum || !plan.ValueContext || !plan.Exhaustive || len(plan.Arms) != 3 {
		t.Fatalf("plan = %#v, %t", plan, ok)
	}
	if plan.Arms[0].EnumNumericValue.String() != "0" || !plan.Arms[0].Guarded || plan.Arms[1].EnumNumericValue.String() != "0" || plan.Arms[2].EnumNumericValue.String() != "1" {
		t.Fatalf("arms = %#v", plan.Arms)
	}
	plan.Arms[0].EnumNumericValue.SetInt64(99)
	again, _ := a.ResolvedMatchPlanOf(match)
	if again.Arms[0].EnumNumericValue.String() != "0" || len(a.resolvedMatchPlans) != before {
		t.Fatal("read-only match query exposed or mutated analyzer state")
	}
}

func TestResolvedForIterationPublishesCompilerKnownIteratorPlan(t *testing.T) {
	source := `module main
type Counter struct { current: int }
impl Counter implements Iterator[int] {
  fn Next() Option[int] {
    self.current += 1
    return Some(self.current)
  }
}
fn Consume() void {
  let mut counter := Counter { current: 0 }
  for value in counter {
    let typed: int := value
  }
}`
	p := parser.New(lexer.NewWithFile(source, "iterator-plan.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(result.Program); len(errs) != 0 {
		t.Fatalf("sema: %v", errs)
	}
	function := result.Program.Statements[3].(*ast.FunctionDeclaration)
	loop := function.Body.Statements[1].(*ast.ForStatement)
	plan, ok := a.ResolvedForIterationOf(loop)
	if !ok || plan.Kind != ForIterationCompilerKnownIterator || typeDisplayName(plan.SourceType) != "Counter" || typeDisplayName(plan.ElementType) != "int" || plan.Next.Name != "Counter.Next" || plan.Next.CompilerKnownID != "CKM-ITERATOR-NEXT" || !plan.RequiresMutableReceiver {
		t.Fatalf("iterator plan = %#v, %t", plan, ok)
	}
}

func TestResolvedMatchPlanRetainsAllTerminatingExpressionFlow(t *testing.T) {
	source := `module main
fn Choose(value: Option[int]) int {
  return match value {
    Some(number) => { return number }
    None => { return 0 }
  }
}`
	p := parser.New(lexer.NewWithFile(source, "terminating-match-plan.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(result.Program); len(errs) > 0 {
		t.Fatalf("sema: %v", errs)
	}
	function := result.Program.Statements[1].(*ast.FunctionDeclaration)
	match := function.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.MatchExpression)
	resolvedType, ok := a.ResolvedTypeOf(match)
	if !ok || resolvedType.Kind != NeverType {
		t.Fatalf("match type = %#v, %t; want never", resolvedType, ok)
	}
	plan, ok := a.ResolvedMatchPlanOf(match)
	if !ok || !plan.Exhaustive || plan.ResultType.Kind != NeverType || len(plan.Arms) != 2 {
		t.Fatalf("plan = %#v, %t", plan, ok)
	}
	for index, arm := range plan.Arms {
		if arm.Flow != MatchArmReturns {
			t.Fatalf("arm %d flow = %q, want returns", index, arm.Flow)
		}
	}
}

func TestSemanticFactsRetainResolvedBuiltinIntegerOperators(t *testing.T) {
	source := `module main
fn Signed(a: int, b: int) int { return -(a + b) }
fn Unsigned(a: uint, b: uint) uint { return a >> b }
fn Compare(a: int, b: int) bool { return a <= b }
`
	p := parser.New(lexer.NewWithFile(source, "operators.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(result.Program); len(errs) > 0 {
		t.Fatalf("sema: %v", errs)
	}
	signedReturn := result.Program.Statements[1].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement)
	negate := signedReturn.Value.(*ast.PrefixExpression)
	add := negate.Right.(*ast.InfixExpression)
	unsignedReturn := result.Program.Statements[2].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement)
	shift := unsignedReturn.Value.(*ast.InfixExpression)
	compareReturn := result.Program.Statements[3].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement)
	compare := compareReturn.Value.(*ast.InfixExpression)

	for expression, want := range map[ast.Expression]ResolvedOperatorKind{
		add: ResolvedIntegerAddChecked, negate: ResolvedIntegerNegateChecked,
		shift: ResolvedIntegerShiftRightUnsignedChecked, compare: ResolvedIntegerCompareLE,
	} {
		resolved, ok := a.ResolvedOperatorOf(expression)
		if !ok || resolved.Kind != want {
			t.Errorf("resolved %T = %#v, %t; want %s", expression, resolved, ok, want)
		}
	}
	before := len(a.resolvedOperators)
	if _, ok := a.ResolvedOperatorOf(&ast.InfixExpression{}); ok || len(a.resolvedOperators) != before {
		t.Fatal("read-only operator query resolved or mutated an unknown expression")
	}
}

func TestResolvedIntegerOperatorsCoverKindsAndActiveWidths(t *testing.T) {
	source := `module main
fn F01(a: int8) int8 { return +a }
fn F02(a: int16) int16 { return -a }
fn F03(a: int32, b: int32) int32 { return a + b }
fn F04(a: int64, b: int64) int64 { return a - b }
fn F05(a: int128, b: int128) int128 { return a * b }
fn F06(a: int256, b: int256) int256 { return a / b }
fn F07(a: uint8, b: uint8) uint8 { return a % b }
fn F08(a: uint16) uint16 { return ~a }
fn F09(a: uint32, b: uint32) uint32 { return a & b }
fn F10(a: uint64, b: uint64) uint64 { return a | b }
fn F11(a: uint128, b: uint128) uint128 { return a ^ b }
fn F12(a: uint256, count: int) uint256 { return a << count }
fn F13(a: int64, count: uint8) int64 { return a >> count }
fn F14(a: int256, count: uint16) int256 { return a << count }
fn F15(a: uint64, count: int8) uint64 { return a >> count }
fn F16(a: int, b: int) bool { return a == b }
fn F17(a: int, b: int) bool { return a != b }
fn F18(a: int, b: int) bool { return a < b }
fn F19(a: int, b: int) bool { return a <= b }
fn F20(a: int, b: int) bool { return a > b }
fn F21(a: int, b: int) bool { return a >= b }
`
	p := parser.New(lexer.NewWithFile(source, "operator-matrix.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(result.Program); len(errs) > 0 {
		t.Fatalf("sema: %v", errs)
	}
	wants := []ResolvedOperatorKind{
		ResolvedIntegerUnaryPlus, ResolvedIntegerNegateChecked,
		ResolvedIntegerAddChecked, ResolvedIntegerSubtractChecked,
		ResolvedIntegerMultiplyChecked, ResolvedIntegerDivideChecked,
		ResolvedIntegerRemainderChecked, ResolvedIntegerBitNot,
		ResolvedIntegerBitAnd, ResolvedIntegerBitOr, ResolvedIntegerBitXor,
		ResolvedIntegerShiftLeftUnsignedChecked, ResolvedIntegerShiftRightSignedChecked,
		ResolvedIntegerShiftLeftSignedChecked, ResolvedIntegerShiftRightUnsignedChecked,
		ResolvedIntegerCompareEQ, ResolvedIntegerCompareNE, ResolvedIntegerCompareLT,
		ResolvedIntegerCompareLE, ResolvedIntegerCompareGT, ResolvedIntegerCompareGE,
	}
	for index, want := range wants {
		function := result.Program.Statements[index+1].(*ast.FunctionDeclaration)
		expression := function.Body.Statements[0].(*ast.ReturnStatement).Value
		resolved, ok := a.ResolvedOperatorOf(expression)
		if !ok || resolved.Kind != want {
			t.Errorf("%s: resolved = %#v, %t; want %s", function.Name.Value, resolved, ok, want)
		}
	}
}

func TestResolvedArithmeticTryRequiresExactCoreErrorAndCoversWideIntegers(t *testing.T) {
	source := `module main
fn Add(left: int, right: int) Result[int, ArithmeticError] {
  let value := try left + right
  return Ok(value)
}
fn Divide(left: int128, right: int128) Result[int128, ArithmeticError] {
  return Ok(try left / right)
}
fn Shift(left: uint256, right: int) Result[uint256, ArithmeticError] {
  return Ok(try left << right)
}
`
	p := parser.New(lexer.NewWithFile(source, "arithmetic-try.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(result.Program); len(errs) > 0 {
		t.Fatalf("sema: %v", errs)
	}
	for _, index := range []int{1, 2, 3} {
		function := result.Program.Statements[index].(*ast.FunctionDeclaration)
		var expression ast.Expression
		if let, ok := function.Body.Statements[0].(*ast.LetStatement); ok {
			expression = let.Value
		} else {
			expression = function.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.OkExpression).Value
		}
		tryExpr := expression.(*ast.TryExpression)
		fact, ok := a.ResolvedTryOf(tryExpr)
		if !ok || fact.Kind != ResolvedTryArithmeticPropagation || fact.ErrorType.Name != "ArithmeticError" || fact.EnclosingResultType.Kind != ResultType {
			t.Errorf("%s resolved try = %#v, %t", function.Name.Value, fact, ok)
		}
	}
}

func TestArithmeticTryRejectsNonResultAndDifferentError(t *testing.T) {
	source := `module main
enum OtherError error { Failed }
fn Plain(left: int, right: int) int { return try left + right }
fn Other(left: int, right: int) Result[int, OtherError] {
  return Ok(try left / right)
}
`
	p := parser.New(lexer.NewWithFile(source, "invalid-arithmetic-try.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	errors := a.Analyze(result.Program)
	if len(errors) != 2 || !strings.Contains(errors[0].Message, "bodyless arithmetic try propagates ArithmeticError with return Err") || !strings.Contains(errors[0].Message, "add a local try handler") || !strings.Contains(errors[1].Message, "map ArithmeticError to OtherError") {
		t.Fatalf("errors = %#v", errors)
	}
}

func TestResolvedTryPlanPreservesHandlerOrderPatternsAndFlow(t *testing.T) {
	source := `module main
fn Divide(left: int64, right: int64) int64 {
  let value := try left / right {
    Err(ArithmeticError.DivisionByZero) => 0
    Err(ArithmeticError.Overflow) => 1
    Err(ArithmeticError.InvalidShift) => 2
    Ok(success) => success
  }
  return value
}

fn Add(left: int128, right: int128) Result[int128, ArithmeticError] {
  let value := try left + right {
    Err(error) => return Err(error)
  }
  return Ok(value)
}
`
	p := parser.New(lexer.NewWithFile(source, "try-plan.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}

	divideTry := result.Program.Statements[1].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.LetStatement).Value.(*ast.TryExpression)
	plan, ok := a.ResolvedTryPlanOf(divideTry)
	if !ok || !plan.Exhaustive || !plan.HasExplicitOk || len(plan.Handlers) != 4 {
		t.Fatalf("divide plan = %#v, %t", plan, ok)
	}
	wants := []struct {
		kind    ResolvedTryHandlerPatternKind
		variant string
	}{
		{TryHandlerErrVariant, "DivisionByZero"},
		{TryHandlerErrVariant, "Overflow"},
		{TryHandlerErrVariant, "InvalidShift"},
		{TryHandlerOkBinding, ""},
	}
	for index, want := range wants {
		got := plan.Handlers[index]
		if got.SourceIndex != index || got.PatternKind != want.kind || got.Variant != want.variant || got.Flow != TryHandlerProducesValue {
			t.Errorf("handler %d = %#v, want %#v", index, got, want)
		}
	}

	addTry := result.Program.Statements[2].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.LetStatement).Value.(*ast.TryExpression)
	returnPlan, ok := a.ResolvedTryPlanOf(addTry)
	if !ok || len(returnPlan.Handlers) != 1 || returnPlan.Handlers[0].PatternKind != TryHandlerErrCatchAll || returnPlan.Handlers[0].BindingName != "error" || returnPlan.Handlers[0].Flow != TryHandlerReturns {
		t.Fatalf("return plan = %#v, %t", returnPlan, ok)
	}

	before := len(a.resolvedTryPlans)
	if _, ok := a.ResolvedTryPlanOf(&ast.TryExpression{}); ok || len(a.resolvedTryPlans) != before {
		t.Fatal("read-only try-plan query resolved or mutated an unknown expression")
	}
}

func TestFixedArrayBoundsTryPublishesTypedFlowAndRemovesPanicEffect(t *testing.T) {
	source := `module main
fn Read(values: int[4], index: int) Result[int, IndexError] {
  return Ok(try values[index])
}
fn Local(values: int[4], index: int) int {
  return try values[index] {
    Err(IndexError.OutOfBounds) => 0
  }
}
`
	p := parser.New(lexer.NewWithFile(source, "bounds-try.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}

	for index, wantKind := range []ResolvedTryKind{ResolvedTryBoundsPropagation, ResolvedTryHandledBounds} {
		function := result.Program.Statements[index+1].(*ast.FunctionDeclaration)
		ret := function.Body.Statements[0].(*ast.ReturnStatement)
		var tryExpr *ast.TryExpression
		switch value := ret.Value.(type) {
		case *ast.TryExpression:
			tryExpr = value
		case *ast.OkExpression:
			tryExpr = value.Value.(*ast.TryExpression)
		default:
			t.Fatalf("%s return value = %T", function.Name.Value, ret.Value)
		}
		fact, found := a.ResolvedTryOf(tryExpr)
		if !found || fact.Kind != wantKind || fact.ErrorType.Name != "IndexError" {
			t.Fatalf("%s try fact = %#v, found=%t", function.Name.Value, fact, found)
		}
		indexExpr := tryExpr.Expression.(*ast.IndexExpression)
		plan, found := a.ResolvedArrayIndexPlanOf(indexExpr)
		if !found || plan.FailureMode != ArrayIndexFailureFallible || plan.ErrorType.Name != "IndexError" {
			t.Fatalf("%s index plan = %#v, found=%t", function.Name.Value, plan, found)
		}
		id := callGraphNodeIDByName(t, a.CallGraph(), function.Name.Value)
		if summary := a.CallGraph().EffectSummary(id); summary.MayPanic || len(summary.DirectEffects) != 0 {
			t.Fatalf("%s effects = %#v, want typed bounds flow without panic", function.Name.Value, summary)
		}
	}
}

func TestTryHandlerBlockRequiresStructuralTermination(t *testing.T) {
	source := `module main
fn Divide(left: int, right: int, condition: bool) int {
  let value := try left / right {
    Err(error) => {
      if condition { return 0 }
    }
  }
  return value
}
`
	p := parser.New(lexer.NewWithFile(source, "try-flow.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	errors := a.Analyze(result.Program)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "try handler must return, propagate, terminate or produce int") {
		t.Fatalf("errors = %#v", errors)
	}
}

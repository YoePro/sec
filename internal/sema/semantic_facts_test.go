package sema

import (
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

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

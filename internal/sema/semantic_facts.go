package sema

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// BindingID is the stable identity assigned by one successful analysis to a
// parameter or local declaration. Zero is reserved for unresolved bindings.
type BindingID uint32

type BindingKind string

const (
	BindingParameter BindingKind = "parameter"
	BindingLocal     BindingKind = "local"
)

type ResolvedBinding struct {
	ID      BindingID
	Kind    BindingKind
	Name    string
	Type    Type
	Mutable bool
}

type ResolvedCallKind string

const (
	ResolvedDirectCall       ResolvedCallKind = "direct"
	ResolvedForeignCall      ResolvedCallKind = "foreign"
	ResolvedStaticMethodCall ResolvedCallKind = "static-method"
)

type ResolvedCall struct {
	Function Function
	Kind     ResolvedCallKind
}

// CallableCapabilityFact is the compiler-owned editor/lowering view of the
// invocation authority defined by rules/declarations/lambda-functions.md.
// Tooling consumes these facts instead of reconstructing capability semantics
// from source prefixes.
type CallableCapabilityFact struct {
	Capability            CallableCapability
	Spelling              string
	InvocationRequirement string
	ConsumesCallable      bool
}

// ResolvedConstruction records the init overload selected by a new
// expression. Construction remains distinct from both calls and conversions.
type ResolvedConstruction struct {
	Initializer Function
	Target      Type
	ErrorType   *Type
	Implicit    bool
}

type ResolvedOperatorKind string

const (
	ResolvedIntegerUnaryPlus                 ResolvedOperatorKind = "integer-unary-plus"
	ResolvedIntegerNegateChecked             ResolvedOperatorKind = "integer-negate-checked"
	ResolvedIntegerBitNot                    ResolvedOperatorKind = "integer-bit-not"
	ResolvedIntegerAddChecked                ResolvedOperatorKind = "integer-add-checked"
	ResolvedIntegerSubtractChecked           ResolvedOperatorKind = "integer-subtract-checked"
	ResolvedIntegerMultiplyChecked           ResolvedOperatorKind = "integer-multiply-checked"
	ResolvedIntegerDivideChecked             ResolvedOperatorKind = "integer-divide-checked"
	ResolvedIntegerRemainderChecked          ResolvedOperatorKind = "integer-remainder-checked"
	ResolvedIntegerBitAnd                    ResolvedOperatorKind = "integer-bit-and"
	ResolvedIntegerBitOr                     ResolvedOperatorKind = "integer-bit-or"
	ResolvedIntegerBitXor                    ResolvedOperatorKind = "integer-bit-xor"
	ResolvedIntegerShiftLeftUnsignedChecked  ResolvedOperatorKind = "integer-shift-left-unsigned-checked"
	ResolvedIntegerShiftLeftSignedChecked    ResolvedOperatorKind = "integer-shift-left-signed-checked"
	ResolvedIntegerShiftRightUnsignedChecked ResolvedOperatorKind = "integer-shift-right-unsigned-checked"
	ResolvedIntegerShiftRightSignedChecked   ResolvedOperatorKind = "integer-shift-right-signed-checked"
	ResolvedIntegerCompareEQ                 ResolvedOperatorKind = "integer-compare-eq"
	ResolvedIntegerCompareNE                 ResolvedOperatorKind = "integer-compare-ne"
	ResolvedIntegerCompareLT                 ResolvedOperatorKind = "integer-compare-lt"
	ResolvedIntegerCompareLE                 ResolvedOperatorKind = "integer-compare-le"
	ResolvedIntegerCompareGT                 ResolvedOperatorKind = "integer-compare-gt"
	ResolvedIntegerCompareGE                 ResolvedOperatorKind = "integer-compare-ge"
	ResolvedEnumCompareEQ                    ResolvedOperatorKind = "enum-compare-eq"
	ResolvedEnumCompareNE                    ResolvedOperatorKind = "enum-compare-ne"
)

type OperatorFailureBehavior string

const (
	OperatorDoesNotFail       OperatorFailureBehavior = "none"
	OperatorArithmeticFailure OperatorFailureBehavior = "ordinary-arithmetic-failure"
)

type ResolvedOperator struct {
	Kind            ResolvedOperatorKind
	LeftType        Type
	RightType       *Type
	ResultType      Type
	RuntimeCheck    bool
	FailureBehavior OperatorFailureBehavior
}

type ResolvedTryKind string

const (
	ResolvedTryResultPropagation     ResolvedTryKind = "result-propagation"
	ResolvedTryHandledResult         ResolvedTryKind = "handled-result"
	ResolvedTryHandledArithmetic     ResolvedTryKind = "handled-arithmetic"
	ResolvedTryArithmeticPropagation ResolvedTryKind = "arithmetic-propagation"
	ResolvedTryHandledBounds         ResolvedTryKind = "handled-bounds"
	ResolvedTryBoundsPropagation     ResolvedTryKind = "bounds-propagation"
)

// ResolvedTry records the exact success/error contract selected by Sema.
// Lowering consumers must not reconstruct this decision from source syntax.
type ResolvedTry struct {
	Kind                ResolvedTryKind
	SuccessType         Type
	ErrorType           Type
	EnclosingResultType Type
}

type ResolvedTryHandlerPatternKind string

const (
	TryHandlerOkBinding   ResolvedTryHandlerPatternKind = "ok-binding"
	TryHandlerOkDiscard   ResolvedTryHandlerPatternKind = "ok-discard"
	TryHandlerErrVariant  ResolvedTryHandlerPatternKind = "err-variant"
	TryHandlerErrCatchAll ResolvedTryHandlerPatternKind = "err-catch-all"
)

type ResolvedTryHandlerFlow string

const (
	TryHandlerInvalidFlow   ResolvedTryHandlerFlow = "invalid"
	TryHandlerProducesValue ResolvedTryHandlerFlow = "produces-value"
	TryHandlerReturns       ResolvedTryHandlerFlow = "returns"
	TryHandlerTerminates    ResolvedTryHandlerFlow = "terminates"
)

type ResolvedTryHandler struct {
	PatternKind ResolvedTryHandlerPatternKind
	Variant     string
	BindingName string
	BindingType Type
	// PayloadDiscard implements explicit Ok(_)/Err(_) handling from
	// rules/errors/errorhandling.md and correction20.md.
	PayloadDiscard bool
	Flow           ResolvedTryHandlerFlow
	ResultType     Type
	SourceIndex    int
}

type ResolvedTryPlan struct {
	SuccessType   Type
	ErrorType     Type
	HasExplicitOk bool
	Exhaustive    bool
	Handlers      []ResolvedTryHandler
}

type ResolvedMatchSubjectKind string

const (
	MatchSubjectEnum   ResolvedMatchSubjectKind = "enum"
	MatchSubjectUnion  ResolvedMatchSubjectKind = "union"
	MatchSubjectResult ResolvedMatchSubjectKind = "result"
	MatchSubjectOption ResolvedMatchSubjectKind = "option"
)

type ResolvedMatchPatternKind string

const (
	MatchPatternEnumValue    ResolvedMatchPatternKind = "enum-value"
	MatchPatternUnionVariant ResolvedMatchPatternKind = "union-variant"
	MatchPatternResultOk     ResolvedMatchPatternKind = "result-ok"
	MatchPatternResultErr    ResolvedMatchPatternKind = "result-err"
	MatchPatternOptionSome   ResolvedMatchPatternKind = "option-some"
	MatchPatternOptionNone   ResolvedMatchPatternKind = "option-none"
	MatchPatternCatchAll     ResolvedMatchPatternKind = "catch-all"
)

type ResolvedMatchBindingAction string

const (
	MatchBindingNone          ResolvedMatchBindingAction = "none"
	MatchBindingDiscard       ResolvedMatchBindingAction = "discard"
	MatchBindingCopyTrivial   ResolvedMatchBindingAction = "copy-trivial"
	MatchBindingBorrowShared  ResolvedMatchBindingAction = "borrow-shared"
	MatchBindingBorrowMutable ResolvedMatchBindingAction = "borrow-mutable"
	MatchBindingMove          ResolvedMatchBindingAction = "move"
	MatchBindingCopySemantic  ResolvedMatchBindingAction = "copy-semantic"
	MatchBindingConditional   ResolvedMatchBindingAction = "copy-conditional"
)

type ResolvedMatchArmFlow string

const (
	MatchArmProducesValue ResolvedMatchArmFlow = "produces-value"
	MatchArmContinues     ResolvedMatchArmFlow = "continues"
	MatchArmReturns       ResolvedMatchArmFlow = "returns"
	MatchArmTerminates    ResolvedMatchArmFlow = "terminates"
	MatchArmLoopControl   ResolvedMatchArmFlow = "loop-control"
)

type ResolvedMatchArm struct {
	SourceIndex           int
	PatternKind           ResolvedMatchPatternKind
	EnumNumericValue      *big.Int
	EnumCaseName          string
	UnionVariantIndex     uint32
	UnionVariantName      string
	BindingName           string
	BindingType           Type
	BindingAction         ResolvedMatchBindingAction
	Guarded               bool
	Flow                  ResolvedMatchArmFlow
	ResultType            Type
	ResidualAlwaysMatches bool
}

type ResolvedMatchPlan struct {
	SubjectKind  ResolvedMatchSubjectKind
	SubjectType  Type
	ValueContext bool
	ResultType   Type
	Exhaustive   bool
	Arms         []ResolvedMatchArm
}

type ResolvedEnumCase struct {
	EnumType Type
	Name     string
	Ordinal  uint32
	Value    *big.Int
	Token    lexer.Token
}

type ResolvedEnumConversionKind string

const (
	ResolvedIntegerToEnum ResolvedEnumConversionKind = "integer-to-enum"
	ResolvedEnumToInteger ResolvedEnumConversionKind = "enum-to-integer"
)

type ResolvedEnumConversion struct {
	Kind        ResolvedEnumConversionKind
	EnumType    Type
	IntegerType Type
	Fallible    bool
}

type ResolvedUnionVariantKind string

const (
	ResolvedUnionVariantEmpty  ResolvedUnionVariantKind = "empty"
	ResolvedUnionVariantSingle ResolvedUnionVariantKind = "single"
	ResolvedUnionVariantFields ResolvedUnionVariantKind = "fields"
)

type ResolvedUnionConstruction struct {
	UnionType        Type
	VariantName      string
	VariantIndex     uint32
	Kind             ResolvedUnionVariantKind
	CanonicalFields  []string
	SourceFieldOrder []string
}

// ResolvedArrayLiteralEntryKind identifies one source entry in the compact
// fixed-array construction plan required by SEC-MLIR Package 14 sections 14-15.
// A spread remains one entry regardless of its conceptual element count.
type ResolvedArrayLiteralEntryKind string

const (
	ArrayLiteralElement ResolvedArrayLiteralEntryKind = "element"
	ArrayLiteralSpread  ResolvedArrayLiteralEntryKind = "spread"
)

// ResolvedArrayTransferAction records the ownership decision already made by
// Sema. SEC-MLIR Package 14 sections 14 and 21 require later IR consumers to
// consume this fact rather than reconstruct transfer behavior from syntax.
type ResolvedArrayTransferAction string

const (
	ArrayTransferConstructDirect ResolvedArrayTransferAction = "construct-direct"
	ArrayTransferCopyTrivial     ResolvedArrayTransferAction = "copy-trivial"
	ArrayTransferMove            ResolvedArrayTransferAction = "move"
	ArrayTransferCopySemantic    ResolvedArrayTransferAction = "copy-semantic"
	ArrayTransferBorrowShared    ResolvedArrayTransferAction = "borrow-shared"
	ArrayTransferBorrowMutable   ResolvedArrayTransferAction = "borrow-mutable"
)

// ResolvedArrayLiteralEntry is one source-ordered fixed-array literal entry.
// Per SEC-MLIR Package 14 sections 15 and 20-23, it deliberately stores no
// AST expression: source syntax remains in the keyed ast.ArrayLiteral, while
// this fact retains only the semantic decision and exact contribution length.
type ResolvedArrayLiteralEntry struct {
	SourceIndex int
	Kind        ResolvedArrayLiteralEntryKind
	Type        Type
	Length      *big.Int
	Action      ResolvedArrayTransferAction
}

// ResolvedArrayLiteralPlan is the compiler-owned compact fixed-array literal
// plan from SEC-MLIR Package 14 sections 14-16. Length and entry lengths are
// arbitrary-precision facts; the plan has one entry per source element/spread,
// never one entry per expanded array element.
type ResolvedArrayLiteralPlan struct {
	ElementType Type
	Length      *big.Int
	Entries     []ResolvedArrayLiteralEntry
}

// ArrayIndexCheckKind records whether a fixed-array projection needs the
// runtime predicate required by SEC-MLIR Package 14 sections 31-36.
type ArrayIndexCheckKind string

const (
	ArrayIndexProvenSafe   ArrayIndexCheckKind = "proven-safe"
	ArrayIndexRuntimeCheck ArrayIndexCheckKind = "runtime-check"
)

// ArrayIndexUseKind preserves the source use selected before Semantic IR
// chooses extraction, replacement, or a later-package borrow operation.
type ArrayIndexUseKind string

const (
	ArrayIndexRead      ArrayIndexUseKind = "read"
	ArrayIndexWrite     ArrayIndexUseKind = "write"
	ArrayIndexBorrow    ArrayIndexUseKind = "borrow"
	ArrayIndexMutBorrow ArrayIndexUseKind = "mut-borrow"
)

// ArrayIndexProofKind is compiler provenance for a proven-safe projection,
// as defined by SEC-MLIR Package 14 section 32.
type ArrayIndexProofKind string

const (
	ArrayIndexProofConstant ArrayIndexProofKind = "constant"
	ArrayIndexProofRange    ArrayIndexProofKind = "range"
	ArrayIndexProofBranch   ArrayIndexProofKind = "branch"
	ArrayIndexProofContract ArrayIndexProofKind = "contract"
	ArrayIndexProofOther    ArrayIndexProofKind = "analysis"
)

// ArrayIndexFailureMode distinguishes ordinary terminating bounds failure
// from the typed fallible path introduced later in Package 14 sections 48-53.
type ArrayIndexFailureMode string

const (
	ArrayIndexFailureNone     ArrayIndexFailureMode = "none"
	ArrayIndexFailureOrdinary ArrayIndexFailureMode = "ordinary"
	ArrayIndexFailureFallible ArrayIndexFailureMode = "fallible"
)

// ResolvedArrayIndexPlan is the immutable Sema authority required by
// SEC-MLIR Package 14 sections 31-36. Exact values are never narrowed to a
// host integer, and downstream builders must not reconstruct this decision.
type ResolvedArrayIndexPlan struct {
	ArrayType     Type
	ArrayLength   *big.Int
	ElementType   Type
	IndexType     Type
	IndexSigned   bool
	ConstantIndex *big.Int
	CheckKind     ArrayIndexCheckKind
	ProofKind     ArrayIndexProofKind
	UseKind       ArrayIndexUseKind
	Action        ResolvedArrayTransferAction
	FailureMode   ArrayIndexFailureMode
	ErrorType     Type
}

// ResolvedStructEntryKind and the adjacent struct plan types implement the
// read-only facts from rules/mlir/semantic-ir/sec_semantic_ir_struct_v1.md
// sections 7-15.
type ResolvedStructEntryKind string

const (
	StructEntryExplicit ResolvedStructEntryKind = "explicit"
	StructEntrySpread   ResolvedStructEntryKind = "spread"
)

type ResolvedStructFieldSourceKind string

const (
	StructFieldSourceExplicit ResolvedStructFieldSourceKind = "explicit"
	StructFieldSourceSpread   ResolvedStructFieldSourceKind = "spread"
	StructFieldSourceDefault  ResolvedStructFieldSourceKind = "default"
)

// ResolvedStructFieldAction uses the current P13 plus P17 action vocabulary.
// Later lowering must consume this fact and may reject actions outside the
// package subset it implements; it must not reconstruct ownership from syntax.
type ResolvedStructFieldAction string

const (
	StructFieldConstructDirect        ResolvedStructFieldAction = "construct-direct"
	StructFieldCopyTrivial            ResolvedStructFieldAction = "copy-trivial"
	StructFieldMove                   ResolvedStructFieldAction = "move"
	StructFieldCopySemanticInfallible ResolvedStructFieldAction = "copy-semantic-infallible"
	StructFieldBorrowShared           ResolvedStructFieldAction = "borrow-shared"
	StructFieldBorrowMutable          ResolvedStructFieldAction = "borrow-mutable"
)

type ResolvedStructEntry struct {
	SourceIndex int
	Kind        ResolvedStructEntryKind
	FieldName   string
	FieldID     uint32
	Expression  ast.Expression
	Type        Type
}

type ResolvedStructFinalField struct {
	FieldID          uint32
	FieldName        string
	FieldType        Type
	SourceKind       ResolvedStructFieldSourceKind
	SourceEntryIndex int
	SpreadFieldID    uint32
	Action           ResolvedStructFieldAction
	Default          DefaultResolution
}

type ResolvedStructLiteralPlan struct {
	StructType       Type
	Entries          []ResolvedStructEntry
	FinalFields      []ResolvedStructFinalField
	FullyInitialized bool
}

type ResolvedMemberKind string

const (
	MemberStoredField ResolvedMemberKind = "stored-field"
	MemberProperty    ResolvedMemberKind = "property"
	MemberOther       ResolvedMemberKind = "other"
)

type ResolvedStructMemberPlan struct {
	Kind       ResolvedMemberKind
	OwnerType  Type
	MemberType Type
	FieldID    uint32
	FieldName  string
	Tags       []StructTag
	Action     ResolvedStructFieldAction
}

// ResolvedStructFieldMetadata exposes the open, ordered field metadata required
// by rules/declarations/struct.md section 4 and the P13 Semantic IR amendment,
// rules/mlir/semantic-ir/sec_semantic_ir_struct_v1.md sections 1-2.
type ResolvedStructFieldMetadata struct {
	OwnerTypeName string
	FieldID       uint32
	Field         StructField
}

// ResolvedTypeOf returns only facts recorded by the completed analysis. It
// never invokes inference and does not mutate Analyzer state.
func (a *Analyzer) ResolvedTypeOf(expr ast.Expression) (Type, bool) {
	if a == nil || expr == nil {
		return Type{}, false
	}
	typ, ok := a.expressionTypes[expr]
	return typ, ok && typ.Kind != InvalidType
}

// CallableCapabilityFactOf returns the normalized capability contract for a
// resolved FunctionType, including legacy shared-capability zero values.
func CallableCapabilityFactOf(typ Type) (CallableCapabilityFact, bool) {
	if typ.Kind != FunctionType {
		return CallableCapabilityFact{}, false
	}
	switch normalizedCallableCapability(typ.FunctionCapability) {
	case CallableMutable:
		return CallableCapabilityFact{
			Capability:            CallableMutable,
			Spelling:              "mut fn",
			InvocationRequirement: "mutable/exclusive callable access",
		}, true
	case CallableConsuming:
		return CallableCapabilityFact{
			Capability:            CallableConsuming,
			Spelling:              "-> fn",
			InvocationRequirement: "owned callable access",
			ConsumesCallable:      true,
		}, true
	default:
		return CallableCapabilityFact{
			Capability:            CallableShared,
			Spelling:              "fn",
			InvocationRequirement: "shared reusable callable access",
		}, true
	}
}

// ResolvedCallableCapabilityOf reads an immutable expression type fact from a
// completed analysis and exposes its callable capability to compiler clients.
func (a *Analyzer) ResolvedCallableCapabilityOf(expr ast.Expression) (CallableCapabilityFact, bool) {
	typ, ok := a.ResolvedTypeOf(expr)
	if !ok {
		return CallableCapabilityFact{}, false
	}
	return CallableCapabilityFactOf(typ)
}

// ResolvedTypeDeclarationLocation returns declaration provenance recorded by
// Sema without exposing its mutable symbol tables.
func (a *Analyzer) ResolvedTypeDeclarationLocation(typ Type) (lexer.Token, bool) {
	if a == nil {
		return lexer.Token{}, false
	}
	token, ok := a.typeDefinitionTokens[typ.Name]
	return token, ok
}

// ResolvedEnumCaseOf returns the exact declared case selected by a successfully
// analyzed member expression. Alias cases retain distinct ordinals.
func (a *Analyzer) ResolvedEnumCaseOf(expr *ast.MemberExpression) (ResolvedEnumCase, bool) {
	if a == nil || expr == nil || expr.Property == nil {
		return ResolvedEnumCase{}, false
	}
	typ, ok := a.ResolvedTypeOf(expr)
	if !ok || typ.Kind != EnumType {
		return ResolvedEnumCase{}, false
	}
	value, ok := typ.EnumConsts[expr.Property.Value]
	if !ok || value.Value == nil {
		return ResolvedEnumCase{}, false
	}
	for ordinal, name := range typ.EnumValues {
		if name == value.Name {
			return ResolvedEnumCase{EnumType: typ, Name: value.Name, Ordinal: uint32(ordinal), Value: new(big.Int).Set(value.Value), Token: value.Token}, true
		}
	}
	return ResolvedEnumCase{}, false
}

// ResolvedEnumConversionOf classifies only explicit conversions accepted by
// Sema. It does not infer a conversion from callee spelling alone.
func (a *Analyzer) ResolvedEnumConversionOf(call *ast.CallExpression) (ResolvedEnumConversion, bool) {
	if a == nil || call == nil || len(call.Arguments) != 1 {
		return ResolvedEnumConversion{}, false
	}
	result, resultOK := a.ResolvedTypeOf(call)
	operand, operandOK := a.ResolvedTypeOf(call.Arguments[0])
	if !resultOK || !operandOK {
		return ResolvedEnumConversion{}, false
	}
	if result.Kind == EnumType && isIntegerType(operand) {
		return ResolvedEnumConversion{Kind: ResolvedIntegerToEnum, EnumType: result, IntegerType: operand}, true
	}
	if operand.Kind == EnumType && isIntegerType(result) {
		return ResolvedEnumConversion{Kind: ResolvedEnumToInteger, EnumType: operand, IntegerType: result}, true
	}
	if operand.Kind == EnumType && result.Kind == ResultType && len(result.TypeArgs) == 2 && isIntegerType(result.TypeArgs[0]) {
		return ResolvedEnumConversion{Kind: ResolvedEnumToInteger, EnumType: operand, IntegerType: result.TypeArgs[0], Fallible: true}, true
	}
	return ResolvedEnumConversion{}, false
}

// ResolvedUnionConstructionOf exposes the concrete union and stable variant
// selected by Sema, including source and canonical field order.
func (a *Analyzer) ResolvedUnionConstructionOf(expr ast.Expression) (ResolvedUnionConstruction, bool) {
	if a == nil || expr == nil {
		return ResolvedUnionConstruction{}, false
	}
	typ, ok := a.ResolvedTypeOf(expr)
	if !ok || typ.Kind != UnionType || len(typ.GenericParameters) != 0 {
		return ResolvedUnionConstruction{}, false
	}
	name := ""
	sourceFields := []string{}
	switch value := expr.(type) {
	case *ast.MemberExpression:
		if value.Property != nil {
			name = value.Property.Value
		}
	case *ast.Identifier:
		name = value.Value
	case *ast.CallExpression:
		if member, memberOK := value.Callee.(*ast.MemberExpression); memberOK && member.Property != nil {
			name = member.Property.Value
		} else {
			name = callExpressionName(value)
		}
	case *ast.StructLiteral:
		_, name, ok = splitUnionVariantTypeName(value.Type.Name)
		if !ok {
			return ResolvedUnionConstruction{}, false
		}
		for _, field := range value.Fields {
			if field != nil && field.Name != nil && !field.Spread {
				sourceFields = append(sourceFields, field.Name.Value)
			}
		}
	default:
		return ResolvedUnionConstruction{}, false
	}
	for index, variant := range typ.UnionVariants {
		if variant.Name != name {
			continue
		}
		resolved := ResolvedUnionConstruction{UnionType: typ, VariantName: name, VariantIndex: uint32(index), SourceFieldOrder: sourceFields}
		switch {
		case variant.Payload != nil:
			resolved.Kind = ResolvedUnionVariantSingle
		case len(variant.PayloadFields) != 0:
			resolved.Kind = ResolvedUnionVariantFields
			for _, field := range variant.PayloadFields {
				resolved.CanonicalFields = append(resolved.CanonicalFields, field.Name)
			}
		default:
			resolved.Kind = ResolvedUnionVariantEmpty
		}
		return resolved, true
	}
	return ResolvedUnionConstruction{}, false
}

// ResolvedStructLiteralPlanOf returns the immutable construction decision
// recorded during Sema according to the current struct/spread/default rules.
func (a *Analyzer) ResolvedStructLiteralPlanOf(expr *ast.StructLiteral) (ResolvedStructLiteralPlan, bool) {
	if a == nil || expr == nil {
		return ResolvedStructLiteralPlan{}, false
	}
	plan, ok := a.resolvedStructLiteralPlans[expr]
	if !ok {
		return ResolvedStructLiteralPlan{}, false
	}
	plan.Entries = append([]ResolvedStructEntry(nil), plan.Entries...)
	plan.FinalFields = append([]ResolvedStructFinalField(nil), plan.FinalFields...)
	return plan, true
}

// ResolvedArrayLiteralPlanOf returns the immutable compact construction fact
// recorded during Sema. SEC-MLIR Package 14 section 17 requires this query to
// perform no inference or expansion and to expose no mutable plan storage.
func (a *Analyzer) ResolvedArrayLiteralPlanOf(expr *ast.ArrayLiteral) (ResolvedArrayLiteralPlan, bool) {
	if a == nil || expr == nil {
		return ResolvedArrayLiteralPlan{}, false
	}
	plan, ok := a.resolvedArrayLiteralPlans[expr]
	if !ok {
		return ResolvedArrayLiteralPlan{}, false
	}
	return cloneResolvedArrayLiteralPlan(plan), true
}

// recordResolvedArrayLiteralPlan retains an immutable Sema-owned copy of a
// Package 14 sections 14-16 literal-plan fact. Literal analysis will populate
// this map in the P14-20 migration; cloning here already prevents a caller's
// temporary big.Int values or entry slice from becoming mutable analyzer state.
func (a *Analyzer) recordResolvedArrayLiteralPlan(expr *ast.ArrayLiteral, plan ResolvedArrayLiteralPlan) {
	if a == nil || expr == nil {
		return
	}
	if a.resolvedArrayLiteralPlans == nil {
		a.resolvedArrayLiteralPlans = map[*ast.ArrayLiteral]ResolvedArrayLiteralPlan{}
	}
	a.resolvedArrayLiteralPlans[expr] = cloneResolvedArrayLiteralPlan(plan)
}

// cloneResolvedArrayLiteralPlan preserves the exact-length immutability
// boundary required by SEC-MLIR Package 14 sections 15-17. Type values are
// already resolved immutable Sema facts; mutable big.Int and slice storage is
// copied at the plan boundary.
func cloneResolvedArrayLiteralPlan(plan ResolvedArrayLiteralPlan) ResolvedArrayLiteralPlan {
	if plan.Length != nil {
		plan.Length = new(big.Int).Set(plan.Length)
	}
	plan.Entries = append([]ResolvedArrayLiteralEntry(nil), plan.Entries...)
	for index := range plan.Entries {
		if plan.Entries[index].Length != nil {
			plan.Entries[index].Length = new(big.Int).Set(plan.Entries[index].Length)
		}
	}
	return plan
}

// ResolvedArrayIndexPlanOf returns a defensive snapshot and performs no
// inference. This is the read-only query mandated by Package 14 section 34.
func (a *Analyzer) ResolvedArrayIndexPlanOf(expr *ast.IndexExpression) (ResolvedArrayIndexPlan, bool) {
	if a == nil || expr == nil {
		return ResolvedArrayIndexPlan{}, false
	}
	plan, ok := a.resolvedArrayIndexPlans[expr]
	if !ok {
		return ResolvedArrayIndexPlan{}, false
	}
	return cloneResolvedArrayIndexPlan(plan), true
}

// recordResolvedArrayIndexPlan retains an analyzer-owned copy of the exact
// Package 14 sections 31-36 index decision.
func (a *Analyzer) recordResolvedArrayIndexPlan(expr *ast.IndexExpression, plan ResolvedArrayIndexPlan) {
	if a == nil || expr == nil {
		return
	}
	if a.resolvedArrayIndexPlans == nil {
		a.resolvedArrayIndexPlans = map[*ast.IndexExpression]ResolvedArrayIndexPlan{}
	}
	a.resolvedArrayIndexPlans[expr] = cloneResolvedArrayIndexPlan(plan)
}

func cloneResolvedArrayIndexPlan(plan ResolvedArrayIndexPlan) ResolvedArrayIndexPlan {
	if plan.ArrayLength != nil {
		plan.ArrayLength = new(big.Int).Set(plan.ArrayLength)
	}
	if plan.ConstantIndex != nil {
		plan.ConstantIndex = new(big.Int).Set(plan.ConstantIndex)
	}
	return plan
}

// ResolvedStructMemberOf distinguishes stored fields from properties using
// compiler-owned facts. It never resolves a member again from its spelling.
func (a *Analyzer) ResolvedStructMemberOf(expr *ast.MemberExpression) (ResolvedStructMemberPlan, bool) {
	if a == nil || expr == nil {
		return ResolvedStructMemberPlan{}, false
	}
	plan, ok := a.resolvedStructMemberPlans[expr]
	plan.Tags = cloneStructTags(plan.Tags)
	return plan, ok
}

// ResolvedStructFieldAt returns an immutable snapshot for tooling and other
// metadata consumers. It follows rules/tooling/lsp.md's one-semantic-source
// requirement instead of making clients reconstruct tags from source text.
func (a *Analyzer) ResolvedStructFieldAt(definition lexer.Token) (ResolvedStructFieldMetadata, bool) {
	if a == nil {
		return ResolvedStructFieldMetadata{}, false
	}
	wanted := sourceTokenLocation(definition)
	for _, owner := range a.types {
		for fieldID, field := range owner.Fields {
			if sourceTokenLocation(field.Token) != wanted {
				continue
			}
			field.Tags = cloneStructTags(field.Tags)
			return ResolvedStructFieldMetadata{OwnerTypeName: typeDisplayName(owner), FieldID: uint32(fieldID), Field: field}, true
		}
	}
	return ResolvedStructFieldMetadata{}, false
}

// cloneStructTags preserves the source order and raw values defined by
// rules/declarations/struct.md section 4 while isolating consumer mutations.
func cloneStructTags(tags []StructTag) []StructTag {
	return append([]StructTag(nil), tags...)
}

// ResolvedFunctionForDeclaration returns the already registered declaration.
func (a *Analyzer) ResolvedFunctionForDeclaration(decl *ast.FunctionDeclaration) (Function, bool) {
	if a == nil || decl == nil || decl.Name == nil {
		return Function{}, false
	}
	return a.lookupFunctionByToken(decl.Name.Value, decl.Name.Token)
}

// ResolvedBindingOf returns the declaration identity selected by Sema for an
// identifier occurrence.
func (a *Analyzer) ResolvedBindingOf(identifier *ast.Identifier) (ResolvedBinding, bool) {
	if a == nil || identifier == nil {
		return ResolvedBinding{}, false
	}
	return a.ResolvedBindingAt(identifier.Token.File, identifier.Token.Line, identifier.Token.Column)
}

// ResolvedBindingAt returns the local or parameter binding selected for one
// source position without invoking inference or mutating analyzer state.
func (a *Analyzer) ResolvedBindingAt(file string, line int, column int) (ResolvedBinding, bool) {
	if a == nil {
		return ResolvedBinding{}, false
	}
	use := sourceTokenKey{File: file, Line: line, Column: column}
	definitions := a.definitionTokens[use]
	if len(definitions) != 1 {
		return ResolvedBinding{}, false
	}
	declaration := definitions[0]
	key := sourceTokenLocation(declaration)
	id, ok := a.bindingIDs[key]
	if !ok {
		return ResolvedBinding{}, false
	}
	fact := a.bindingFacts[key]
	fact.ID = id
	return fact, true
}

func (a *Analyzer) ResolvedCallTarget(call *ast.CallExpression) (ResolvedCall, bool) {
	if a == nil || call == nil {
		return ResolvedCall{}, false
	}
	resolved, ok := a.resolvedCalls[call]
	return resolved, ok
}

// ResolvedConstructionOf returns the exact initializer selection recorded for
// a successfully analyzed new expression. Tooling and lowering consume this
// fact instead of reconstructing lifecycle overload resolution from syntax.
func (a *Analyzer) ResolvedConstructionOf(expr *ast.NewExpression) (ResolvedConstruction, bool) {
	if a == nil || expr == nil {
		return ResolvedConstruction{}, false
	}
	resolved, ok := a.resolvedConstructions[expr]
	if !ok {
		return ResolvedConstruction{}, false
	}
	resolved.Initializer.GenericParameters = append([]string(nil), resolved.Initializer.GenericParameters...)
	resolved.Initializer.Parameters = append([]FunctionParameter(nil), resolved.Initializer.Parameters...)
	if resolved.ErrorType != nil {
		errorType := *resolved.ErrorType
		resolved.ErrorType = &errorType
	}
	return resolved, true
}

// ResolvedOperatorOf returns operator metadata recorded by successful Sema.
// It performs no inference or resolution and never mutates Analyzer state.
func (a *Analyzer) ResolvedOperatorOf(expr ast.Expression) (ResolvedOperator, bool) {
	if a == nil || expr == nil {
		return ResolvedOperator{}, false
	}
	resolved, ok := a.resolvedOperators[expr]
	if !ok {
		return ResolvedOperator{}, false
	}
	if resolved.RightType != nil {
		right := *resolved.RightType
		resolved.RightType = &right
	}
	return resolved, true
}

// ResolvedTryOf returns the completed Sema decision for a try expression.
func (a *Analyzer) ResolvedTryOf(expr *ast.TryExpression) (ResolvedTry, bool) {
	if a == nil || expr == nil {
		return ResolvedTry{}, false
	}
	resolved, ok := a.resolvedTries[expr]
	return resolved, ok
}

// ResolvedTryPlanOf returns the immutable handler plan recorded by successful
// Sema. It performs no pattern resolution, exhaustiveness checking or inference.
func (a *Analyzer) ResolvedTryPlanOf(expr *ast.TryExpression) (ResolvedTryPlan, bool) {
	if a == nil || expr == nil {
		return ResolvedTryPlan{}, false
	}
	plan, ok := a.resolvedTryPlans[expr]
	if !ok {
		return ResolvedTryPlan{}, false
	}
	plan.Handlers = append([]ResolvedTryHandler(nil), plan.Handlers...)
	return plan, true
}

// ResolvedMatchPlanOf returns the immutable source-order match decision recorded
// by successful Sema. It performs no inference or coverage analysis.
func (a *Analyzer) ResolvedMatchPlanOf(expr *ast.MatchExpression) (ResolvedMatchPlan, bool) {
	if a == nil || expr == nil {
		return ResolvedMatchPlan{}, false
	}
	plan, ok := a.resolvedMatchPlans[expr]
	if !ok {
		return ResolvedMatchPlan{}, false
	}
	plan.Arms = append([]ResolvedMatchArm(nil), plan.Arms...)
	for index := range plan.Arms {
		if plan.Arms[index].EnumNumericValue != nil {
			plan.Arms[index].EnumNumericValue = new(big.Int).Set(plan.Arms[index].EnumNumericValue)
		}
	}
	return plan, true
}

func (a *Analyzer) recordResolvedOperator(expr ast.Expression, result Type) {
	if a == nil || expr == nil || result.Kind == InvalidType {
		return
	}
	var resolved ResolvedOperator
	switch expression := expr.(type) {
	case *ast.PrefixExpression:
		operand, ok := a.expressionTypes[expression.Right]
		if !ok || !isBuiltinIntegerOperatorType(operand) {
			return
		}
		resolved = ResolvedOperator{LeftType: operand, ResultType: result, FailureBehavior: OperatorDoesNotFail}
		switch expression.Operator {
		case "+":
			resolved.Kind = ResolvedIntegerUnaryPlus
		case "-":
			if operand.Kind != IntType {
				return
			}
			resolved.Kind = ResolvedIntegerNegateChecked
			resolved.RuntimeCheck = true
			resolved.FailureBehavior = OperatorArithmeticFailure
		case "~":
			resolved.Kind = ResolvedIntegerBitNot
		default:
			return
		}
	case *ast.InfixExpression:
		left, leftOK := a.expressionTypes[expression.Left]
		right, rightOK := a.expressionTypes[expression.Right]
		if !leftOK || !rightOK {
			return
		}
		resolved = ResolvedOperator{LeftType: left, RightType: &right, ResultType: result, FailureBehavior: OperatorDoesNotFail}
		if left.Kind == EnumType && right.Kind == EnumType && sameConcreteType(left, right) {
			switch expression.Operator {
			case "==":
				resolved.Kind = ResolvedEnumCompareEQ
			case "!=":
				resolved.Kind = ResolvedEnumCompareNE
			default:
				return
			}
			break
		}
		if !isBuiltinIntegerOperatorType(left) || !isBuiltinIntegerOperatorType(right) {
			return
		}
		switch expression.Operator {
		case "+":
			resolved.Kind = ResolvedIntegerAddChecked
		case "-":
			resolved.Kind = ResolvedIntegerSubtractChecked
		case "*":
			resolved.Kind = ResolvedIntegerMultiplyChecked
		case "/":
			resolved.Kind = ResolvedIntegerDivideChecked
		case "%":
			resolved.Kind = ResolvedIntegerRemainderChecked
		case "&":
			resolved.Kind = ResolvedIntegerBitAnd
		case "|":
			resolved.Kind = ResolvedIntegerBitOr
		case "^":
			resolved.Kind = ResolvedIntegerBitXor
		case "<<":
			if left.Kind == UintType {
				resolved.Kind = ResolvedIntegerShiftLeftUnsignedChecked
			} else {
				resolved.Kind = ResolvedIntegerShiftLeftSignedChecked
			}
		case ">>":
			if left.Kind == UintType {
				resolved.Kind = ResolvedIntegerShiftRightUnsignedChecked
			} else {
				resolved.Kind = ResolvedIntegerShiftRightSignedChecked
			}
		case "==":
			resolved.Kind = ResolvedIntegerCompareEQ
		case "!=":
			resolved.Kind = ResolvedIntegerCompareNE
		case "<":
			resolved.Kind = ResolvedIntegerCompareLT
		case "<=":
			resolved.Kind = ResolvedIntegerCompareLE
		case ">":
			resolved.Kind = ResolvedIntegerCompareGT
		case ">=":
			resolved.Kind = ResolvedIntegerCompareGE
		default:
			return
		}
		switch resolved.Kind {
		case ResolvedIntegerAddChecked, ResolvedIntegerSubtractChecked,
			ResolvedIntegerMultiplyChecked, ResolvedIntegerDivideChecked,
			ResolvedIntegerRemainderChecked, ResolvedIntegerShiftLeftUnsignedChecked,
			ResolvedIntegerShiftLeftSignedChecked, ResolvedIntegerShiftRightUnsignedChecked,
			ResolvedIntegerShiftRightSignedChecked:
			resolved.RuntimeCheck = true
			resolved.FailureBehavior = OperatorArithmeticFailure
		}
	default:
		return
	}
	if resolved.Kind != "" {
		a.resolvedOperators[expr] = resolved
	}
}

func isBuiltinIntegerOperatorType(typ Type) bool {
	return isIntegerType(typ) && !typ.Named && !typ.Declared && typ.Unit == "" && typ.Dimension.IsZero() && len(typ.Contracts) == 0
}

func (a *Analyzer) recordBinding(token lexer.Token, kind BindingKind, name string, typ Type, mutable bool) {
	key := sourceTokenLocation(token)
	if !validDefinitionToken(token) {
		return
	}
	if _, exists := a.bindingIDs[key]; exists {
		return
	}
	a.bindingIDs[key] = a.nextBindingID
	a.bindingFacts[key] = ResolvedBinding{ID: a.nextBindingID, Kind: kind, Name: name, Type: typ, Mutable: mutable}
	a.nextBindingID++
}

func resolvedCallKind(dispatch CallDispatchKind) ResolvedCallKind {
	switch dispatch {
	case CallDispatchForeign:
		return ResolvedForeignCall
	case CallDispatchStaticMethod:
		return ResolvedStaticMethodCall
	default:
		return ResolvedDirectCall
	}
}

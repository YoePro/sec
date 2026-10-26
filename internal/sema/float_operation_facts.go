package sema

import "sec/internal/ast"

// ResolvedFloatOperation is a detached, value-only snapshot of a successfully
// resolved primitive float operator. CommonType preserves source identity:
// float and float64 remain distinct even on a 64-bit plan. FormatBits is zero
// for platform-sized float without a selected CompilationPlan; no host width
// is guessed. Core methods and conversions have their own resolution plans.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package20.md — §§6–9,20–27,64–65
//   - rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md — §2
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §6
type ResolvedFloatOperation struct {
	Kind                ResolvedOperatorKind
	CommonType          string
	FormatBits          uint16
	PlanWidthBits       uint16
	SourceOperator      string
	ComparisonPredicate string
	Rounding            string
	FastMath            string
	NoContraction       bool
	PreserveSubnormals  bool
	RemainderKind       string
}

// resolvedFloatOperationFacts publishes policies at successful operator
// resolution, never at a consumer's query or by inspecting source spelling.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§20–27,64–65;
// rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md — §2.
func (a *Analyzer) resolvedFloatOperationFacts(operator ResolvedOperator) ResolvedFloatOperation {
	var source, predicate string
	switch operator.Kind {
	case ResolvedFloatUnaryPlus, ResolvedFloatAdd:
		source = "+"
	case ResolvedFloatNegate, ResolvedFloatSubtract:
		source = "-"
	case ResolvedFloatMultiply:
		source = "*"
	case ResolvedFloatDivide:
		source = "/"
	case ResolvedFloatRemainder:
		source = "%"
	case ResolvedFloatCompareEQ:
		source, predicate = "==", "oeq"
	case ResolvedFloatCompareNE:
		source, predicate = "!=", "une"
	case ResolvedFloatCompareLT:
		source, predicate = "<", "olt"
	case ResolvedFloatCompareLE:
		source, predicate = "<=", "ole"
	case ResolvedFloatCompareGT:
		source, predicate = ">", "ogt"
	case ResolvedFloatCompareGE:
		source, predicate = ">=", "oge"
	default:
		return ResolvedFloatOperation{}
	}
	width := uint16(operator.LeftType.FloatBits)
	if operator.LeftType.Name == "float" {
		width = a.ScalarPlanWidthBits()
	}
	facts := ResolvedFloatOperation{
		Kind: operator.Kind, CommonType: operator.LeftType.Name,
		FormatBits: width, PlanWidthBits: a.ScalarPlanWidthBits(),
		SourceOperator: source, ComparisonPredicate: predicate,
		Rounding: "nearest-even", FastMath: "none", NoContraction: true,
		PreserveSubnormals: true,
	}
	if operator.Kind == ResolvedFloatRemainder {
		facts.RemainderKind = "truncation"
	}
	return facts
}

// ResolvedFloatOperationOf reads published primitive float operator facts.
// Its value-only result cannot mutate the Analyzer and querying never infers
// types, creates semantic identities or populates missing facts.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§20–27,64–65.
func (a *Analyzer) ResolvedFloatOperationOf(expression ast.Expression) (ResolvedFloatOperation, bool) {
	if a == nil || expression == nil {
		return ResolvedFloatOperation{}, false
	}
	resolved, ok := a.resolvedOperators[expression]
	return resolved.Float, ok && resolved.Float.CommonType != ""
}

// ResolvedFloatCompoundAssignmentOf reads the same float operator facts for
// x op= y; assignment adds no different arithmetic or failure semantics.
// Rules: rules/foundations/operators.md — "Compound assignment";
// rules/mlir/packages/sec-mlir-dialect_package20.md — §§20–27.
func (a *Analyzer) ResolvedFloatCompoundAssignmentOf(statement *ast.AssignmentStatement) (ResolvedFloatOperation, bool) {
	if a == nil || statement == nil {
		return ResolvedFloatOperation{}, false
	}
	resolved, ok := a.resolvedCompoundAssignments[statement]
	return resolved.Float, ok && resolved.Float.CommonType != ""
}

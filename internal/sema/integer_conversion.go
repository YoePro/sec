package sema

import (
	"math/big"
	"strings"

	"sec/internal/ast"
)

// ResolvedIntegerConversion is Sema's plan for an explicit conversion
// between two built-in integer types (`int(x)`, `int8(x)`, `uint(x)`, ...).
// Exact means no runtime failure branch is needed; Proof retains why. A
// non-exact conversion needs the checked form (ConversionError) and `try`,
// and is never lowered as numeric.convert-exact.
//
// Rules:
//   - rules/types/types.md — "Explicit conversions", "Exact integer conversion proof"
//   - rules/compiler/semantic_ir.md — § 124
//   - rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 44, 60–63 (NumericConvertExactOp)
type ResolvedIntegerConversion struct {
	SourceType Type
	TargetType Type
	Exact      bool
	Proof      IntegerConversionProof
}

// IntegerConversionProofBasis names what an exactness proof rests on.
type IntegerConversionProofBasis string

const (
	// IntegerConversionUnproven: the source domain may not fit the target.
	IntegerConversionUnproven IntegerConversionProofBasis = ""
	// IntegerConversionConstant: a compile-time constant validated against
	// the target domain.
	IntegerConversionConstant IntegerConversionProofBasis = "constant"
	// IntegerConversionSourceDomain: the complete source type domain fits.
	IntegerConversionSourceDomain IntegerConversionProofBasis = "source-domain"
	// IntegerConversionClosedEnumDomain: every declared value class of a
	// closed enum fits (aliases do not enlarge the domain).
	IntegerConversionClosedEnumDomain IntegerConversionProofBasis = "closed-enum-domain"
	// IntegerConversionOpenEnumDomain: the complete 0..2^N-1 domain of an
	// open bit[N] enum fits; its named cases never bound it.
	IntegerConversionOpenEnumDomain IntegerConversionProofBasis = "open-enum-domain"
)

// IntegerConversionProof is the plan-compatible exactness contract Semantic
// IR consumes instead of rediscovering domains or widths (MD-104). The
// domains are the ones the proof compared; PlanPointerWidthBits is the
// selected CompilationPlan's width when the proof depends on target-sized
// types, and zero when it holds on every supported plan.
type IntegerConversionProof struct {
	Basis                IntegerConversionProofBasis
	SourceMin, SourceMax *big.Int
	TargetMin, TargetMax *big.Int
	PlanPointerWidthBits uint16
}

// Proven reports whether the proof establishes exactness.
func (p IntegerConversionProof) Proven() bool {
	return p.Basis != IntegerConversionUnproven
}

// recordIntegerConversion publishes the plan of a built-in integer to
// built-in integer conversion call.
func (a *Analyzer) recordIntegerConversion(call *ast.CallExpression, target, source Type) {
	if call == nil || len(call.Arguments) != 1 || !isBuiltinConversionInteger(target) || !isBuiltinConversionInteger(source) {
		return
	}
	var constant *big.Int
	if value, ok := a.integerConstantValue(call.Arguments[0]); ok {
		constant = value
	}
	proof := a.integerConversionProof(target, source, constant)
	if a.resolvedIntegerConversions == nil {
		a.resolvedIntegerConversions = map[*ast.CallExpression]ResolvedIntegerConversion{}
	}
	a.resolvedIntegerConversions[call] = ResolvedIntegerConversion{SourceType: source, TargetType: target, Exact: proof.Proven(), Proof: proof}
}

// integerConversionProof proves a conversion exact for the selected
// CompilationPlan (MD-104 Option A): target-sized int and uint take the
// plan's width. Without a resolved plan the proof must hold at every
// supported width (32 and 64 bits), never at an assumed default. A constant
// argument is proved by its exact value; an enum source by its semantic
// domain (closed: declared value classes; open bit[N]: 0..2^N-1).
//
// Rules:
//   - rules/types/types.md — "Exact integer conversion proof"
//   - rules/declarations/enums.md — closed and open enum domains
func (a *Analyzer) integerConversionProof(target, source Type, constant *big.Int) IntegerConversionProof {
	widths := []uint16{32, 64}
	if a.scalarPlanWidthBits != 0 {
		widths = []uint16{a.scalarPlanWidthBits}
	}
	basis := IntegerConversionSourceDomain
	switch {
	case constant != nil:
		basis = IntegerConversionConstant
	case source.Kind == EnumType && enumIsOpenBitDomain(source):
		basis = IntegerConversionOpenEnumDomain
	case source.Kind == EnumType:
		basis = IntegerConversionClosedEnumDomain
	}
	proof := IntegerConversionProof{Basis: basis}
	for _, width := range widths {
		targetMin, targetMax, ok := scalarIntegerDomain(portableDomainType(target, width))
		if !ok {
			return IntegerConversionProof{}
		}
		sourceMin, sourceMax, ok := a.conversionSourceDomain(source, constant, width)
		if !ok || sourceMin.Cmp(targetMin) < 0 || sourceMax.Cmp(targetMax) > 0 {
			return IntegerConversionProof{}
		}
		proof.SourceMin, proof.SourceMax, proof.TargetMin, proof.TargetMax = sourceMin, sourceMax, targetMin, targetMax
	}
	if a.scalarPlanWidthBits != 0 && (isTargetSizedInteger(target) || isTargetSizedInteger(source)) {
		proof.PlanPointerWidthBits = a.scalarPlanWidthBits
	}
	return proof
}

// conversionSourceDomain is the value domain a conversion proof may assume
// for its source at one target width.
func (a *Analyzer) conversionSourceDomain(source Type, constant *big.Int, width uint16) (*big.Int, *big.Int, bool) {
	if constant != nil {
		return new(big.Int).Set(constant), new(big.Int).Set(constant), true
	}
	if source.Kind != EnumType {
		return scalarIntegerDomain(portableDomainType(source, width))
	}
	if enumIsOpenBitDomain(source) {
		if source.MinInteger == nil || source.MaxInteger == nil {
			return nil, nil, false
		}
		return new(big.Int).Set(source.MinInteger), new(big.Int).Set(source.MaxInteger), true
	}
	var minimum, maximum *big.Int
	for _, value := range source.EnumConsts {
		if value.Value == nil {
			return nil, nil, false
		}
		if minimum == nil || value.Value.Cmp(minimum) < 0 {
			minimum = value.Value
		}
		if maximum == nil || value.Value.Cmp(maximum) > 0 {
			maximum = value.Value
		}
	}
	if minimum == nil {
		return nil, nil, false
	}
	return new(big.Int).Set(minimum), new(big.Int).Set(maximum), true
}

// enumIsOpenBitDomain reports a bit[N]-backed hardware enum, whose domain is
// open over its complete bit width (enums.md § 2).
func enumIsOpenBitDomain(typ Type) bool {
	return typ.Kind == EnumType && typ.BitWidth > 0 && strings.HasPrefix(typ.Underlying, "bit")
}

// isTargetSizedInteger reports int and uint, whose width the plan selects.
// An enum source's proof domain is its declared or bit domain, which no plan
// changes.
func isTargetSizedInteger(typ Type) bool {
	return typ.Kind != EnumType && (typ.Name == "int" || typ.Name == "uint")
}

// ResolvedIntegerConversionOf returns the recorded plan without inference.
func (a *Analyzer) ResolvedIntegerConversionOf(call *ast.CallExpression) (ResolvedIntegerConversion, bool) {
	if a == nil || call == nil {
		return ResolvedIntegerConversion{}, false
	}
	plan, ok := a.resolvedIntegerConversions[call]
	return plan, ok
}

// isBuiltinConversionInteger admits the built-in signed and unsigned integer
// types; named, unit-bearing, contract, char and rune types keep their own
// conversion rules.
func isBuiltinConversionInteger(typ Type) bool {
	return (typ.Kind == IntType || typ.Kind == UintType) && !typ.Named && len(typ.Contracts) == 0 && !hasUnitSemantics(typ)
}

// recordForRangeStep publishes the compile-time value of an explicit range
// step, so lowering can choose the progression direction without
// re-evaluating it (flowcontrol_for.md §§ 23, 26).
func (a *Analyzer) recordForRangeStep(stmt *ast.ForStatement) {
	if stmt == nil || stmt.Step == nil {
		return
	}
	if value, ok := a.integerConstantValue(stmt.Step); ok {
		a.resolvedForRangeSteps[stmt] = new(big.Int).Set(value)
	} else {
		delete(a.resolvedForRangeSteps, stmt)
	}
}

// ResolvedForRangeStepOf returns the compile-time value of a for loop's
// explicit integer step, when it is a compile-time constant.
func (a *Analyzer) ResolvedForRangeStepOf(stmt *ast.ForStatement) (*big.Int, bool) {
	if a == nil || stmt == nil {
		return nil, false
	}
	value, ok := a.resolvedForRangeSteps[stmt]
	if !ok {
		return nil, false
	}
	return new(big.Int).Set(value), true
}

package sema

import (
	"math/big"

	"sec/internal/ast"
)

// NumericConversionKind identifies the primitive P20 conversion, independently
// of whether a particular argument proves it exact. FloatToFloat retains
// identity conversions and unresolved platform formats without guessing width.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§42–55,60.
type NumericConversionKind string

const (
	NumericConvertFloatToFloat    NumericConversionKind = "float-to-float"
	NumericConvertFloatWiden      NumericConversionKind = "float-widen"
	NumericConvertFloatNarrow     NumericConversionKind = "float-narrow"
	NumericConvertSignedToFloat   NumericConversionKind = "signed-to-float"
	NumericConvertUnsignedToFloat NumericConversionKind = "unsigned-to-float"
	NumericConvertFloatToSigned   NumericConversionKind = "float-to-signed"
	NumericConvertFloatToUnsigned NumericConversionKind = "float-to-unsigned"
	NumericConvertFloatToBool     NumericConversionKind = "float-to-bool"
)

// NumericConversionProof retains the argument facts used by the checked
// conversion classifier. Basis is empty when exactness is unproven; otherwise
// it is identity, source-domain, value-interval, constant or boolean-total.
// Boolean totality means no failure, not preservation of numeric information.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§44–55,60;
// rules/types/types.md — §25(5),(7).
type NumericConversionProof struct {
	Basis                string
	SourceMin, SourceMax *big.Int
	Constant             *big.Rat
	FloatCategory        string
}

// ResolvedNumericConversionPlan is a detached snapshot for typed primitive
// binary-float conversions. Width zero explicitly means unresolved, never a
// host default. Failures retain canonical ConversionError priority. Literal
// shaping, decimals, named types, quantities and C ABI conversions keep their
// own plans. This does not claim that checked conversion lowering is available.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§42–61;
// rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md — §§2,4;
// rules/mlir/normative-sync/sec_float_numeric_sync_v1.md — §§4–5,12–15.
type ResolvedNumericConversionPlan struct {
	Kind                      NumericConversionKind
	SourceType, TargetType    Type
	SourceBits, TargetBits    int
	PlanWidthBits             uint16
	ProvenExact, RuntimeCheck bool
	ErrorType                 Type
	Failures                  []ConversionFailureKind
	Proof                     NumericConversionProof
	// Checks are ordered semantic obligations; exactness may discharge them.
	Checks                                   []string
	IntegerLowerBound, IntegerUpperExclusive *big.Int
	BoolPredicate                            string
}

// numericConversionPlanType admits only the primitive types covered by P20;
// declared wrappers and ABI scalars must retain their independent obligations.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§42–43.
func numericConversionPlanType(typ Type) bool {
	if typ.Named || len(typ.Contracts) != 0 || hasUnitSemantics(typ) {
		return false
	}
	switch typ.Name {
	case "float", "float32", "float64":
		return typ.Kind == FloatType
	case "int", "int8", "int16", "int32", "int64", "int128", "int256":
		return typ.Kind == IntType
	case "byte", "uint", "uint8", "uint16", "uint32", "uint64", "uint128", "uint256":
		return typ.Kind == UintType
	case "bool":
		return typ.Kind == BoolType
	}
	return false
}

// numericConversionPlanBits resolves represented format/domain widths, leaving
// target-sized types unknown without a CompilationPlan (MD-014/MD-104).
// Rules: rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md — §2;
// rules/types/types.md — §25.1(2).
func (a *Analyzer) numericConversionPlanBits(typ Type) int {
	if typ.Name == "float" || isTargetSizedInteger(typ) {
		return int(a.ScalarPlanWidthBits())
	}
	if typ.Kind == FloatType {
		return typ.FloatBits
	}
	if typ.Kind == BoolType {
		return 1
	}
	if typ.MaxInteger != nil {
		bits := typ.MaxInteger.BitLen()
		if typ.Kind == IntType {
			bits++
		}
		return bits
	}
	return 0
}

// recordNumericConversionPlan publishes the successful typed conversion using
// the checked-conversion producer's classification, rather than introducing a
// second conversion engine. Queries never redo classification or value proofs.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§42–61;
// rules/mlir/normative-sync/sec_float_numeric_sync_v1.md — §§4–5,12–15.
func (a *Analyzer) recordNumericConversionPlan(call *ast.CallExpression, target, source Type) {
	delete(a.resolvedNumericConversionPlans, call)
	if call == nil || len(call.Arguments) != 1 || !numericConversionPlanType(target) || !numericConversionPlanType(source) ||
		(source.Kind != FloatType && target.Kind != FloatType) ||
		(source.Kind == BoolType) || isUntypedNumericExpression(call.Arguments[0]) {
		return
	}
	plan := ResolvedNumericConversionPlan{
		SourceType: snapshotNumericConversionType(source), TargetType: snapshotNumericConversionType(target),
		SourceBits: a.numericConversionPlanBits(source), TargetBits: a.numericConversionPlanBits(target),
		PlanWidthBits: a.ScalarPlanWidthBits(),
	}
	if source.Kind == FloatType {
		plan.SourceType.FloatBits = plan.SourceBits
	}
	if target.Kind == FloatType {
		plan.TargetType.FloatBits = plan.TargetBits
	}
	switch {
	case source.Kind == FloatType && target.Kind == FloatType:
		plan.Kind = NumericConvertFloatToFloat
		if plan.SourceBits != 0 && plan.TargetBits != 0 {
			if plan.SourceBits < plan.TargetBits {
				plan.Kind = NumericConvertFloatWiden
			} else if plan.SourceBits > plan.TargetBits {
				plan.Kind = NumericConvertFloatNarrow
			}
		}
	case source.Kind == IntType:
		plan.Kind = NumericConvertSignedToFloat
	case source.Kind == UintType:
		plan.Kind = NumericConvertUnsignedToFloat
	case target.Kind == IntType:
		plan.Kind = NumericConvertFloatToSigned
	case target.Kind == UintType:
		plan.Kind = NumericConvertFloatToUnsigned
	case target.Kind == BoolType:
		plan.Kind = NumericConvertFloatToBool
	default:
		return
	}
	plan.resolveNumericConversionChecks()
	if checked, ok := a.checkedConversions[call]; ok {
		plan.Failures = append([]ConversionFailureKind(nil), checked.failures...)
		plan.ErrorType = semanticSnapshotType(a.types["ConversionError"])
		plan.RuntimeCheck = true
	} else {
		plan.ProvenExact = true
		plan.Proof.Basis = "source-domain"
		if sameConcreteType(source, target) {
			plan.Proof.Basis = "identity"
		} else if target.Kind == BoolType {
			plan.Proof.Basis = "boolean-total"
		} else if value, known := a.numericFloatConstantOf(call.Arguments[0], source); source.Kind == FloatType && known {
			plan.Proof.Basis, plan.Proof.FloatCategory = "constant", value.special
			if value.value != nil {
				plan.Proof.Constant = new(big.Rat).Set(value.value)
			}
		} else if source.Kind == IntType || source.Kind == UintType {
			if low, high, known := a.scalarValueInterval(call.Arguments[0], source); known {
				plan.Proof.Basis = "value-interval"
				plan.Proof.SourceMin, plan.Proof.SourceMax = new(big.Int).Set(low), new(big.Int).Set(high)
			}
		}
	}
	if a.resolvedNumericConversionPlans == nil {
		a.resolvedNumericConversionPlans = map[*ast.CallExpression]ResolvedNumericConversionPlan{}
	}
	a.resolvedNumericConversionPlans[call] = plan
}

// ResolvedNumericConversionPlanOf returns a detached recorded plan. It never
// infers, fills caches, allocates semantic identities or guesses a missing plan.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§60–61.
func (a *Analyzer) ResolvedNumericConversionPlanOf(expr ast.Expression) (ResolvedNumericConversionPlan, bool) {
	if a == nil || expr == nil {
		return ResolvedNumericConversionPlan{}, false
	}
	call, ok := expr.(*ast.CallExpression)
	if !ok {
		return ResolvedNumericConversionPlan{}, false
	}
	plan, ok := a.resolvedNumericConversionPlans[call]
	if !ok {
		return ResolvedNumericConversionPlan{}, false
	}
	plan.SourceType, plan.TargetType = snapshotNumericConversionType(plan.SourceType), snapshotNumericConversionType(plan.TargetType)
	plan.ErrorType = semanticSnapshotType(plan.ErrorType)
	plan.Failures = append([]ConversionFailureKind(nil), plan.Failures...)
	plan.Checks = append([]string(nil), plan.Checks...)
	if plan.IntegerLowerBound != nil {
		plan.IntegerLowerBound = new(big.Int).Set(plan.IntegerLowerBound)
		plan.IntegerUpperExclusive = new(big.Int).Set(plan.IntegerUpperExclusive)
	}
	if plan.Proof.Constant != nil {
		plan.Proof.Constant = new(big.Rat).Set(plan.Proof.Constant)
	}
	if plan.Proof.SourceMin != nil {
		plan.Proof.SourceMin = new(big.Int).Set(plan.Proof.SourceMin)
		plan.Proof.SourceMax = new(big.Int).Set(plan.Proof.SourceMax)
	}
	return plan, true
}

// snapshotNumericConversionType retains scalar format and arbitrary-precision
// domain facts in addition to the structural, target-independent type snapshot.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§43–55,60–61.
func snapshotNumericConversionType(typ Type) Type {
	result := semanticSnapshotType(typ)
	result.FloatBits = typ.FloatBits
	if typ.MinInteger != nil {
		result.MinInteger = new(big.Int).Set(typ.MinInteger)
	}
	if typ.MaxInteger != nil {
		result.MaxInteger = new(big.Int).Set(typ.MaxInteger)
	}
	return result
}

// resolveNumericConversionChecks records the guard order and exact integer
// half-open domain once, before consumers read the plan. The integer cast is
// permitted only after all three checks; bool uses unordered inequality to zero.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§47–55;
// rules/mlir/normative-sync/sec_float_numeric_sync_v1.md — §§12–16.
func (plan *ResolvedNumericConversionPlan) resolveNumericConversionChecks() {
	switch plan.Kind {
	case NumericConvertFloatToSigned, NumericConvertFloatToUnsigned:
		plan.Checks = []string{"finite", "range", "integral"}
		if plan.TargetBits != 0 {
			power := plan.TargetBits
			plan.IntegerLowerBound = new(big.Int)
			if plan.Kind == NumericConvertFloatToSigned {
				power--
				plan.IntegerLowerBound.Neg(new(big.Int).Lsh(big.NewInt(1), uint(power)))
			}
			plan.IntegerUpperExclusive = new(big.Int).Lsh(big.NewInt(1), uint(power))
		}
	case NumericConvertSignedToFloat, NumericConvertUnsignedToFloat:
		plan.Checks = []string{"finite-range", "significand"}
	case NumericConvertFloatToFloat, NumericConvertFloatNarrow, NumericConvertFloatWiden:
		plan.Checks = []string{"map-nan-infinity", "finite-range", "round-trip"}
	case NumericConvertFloatToBool:
		plan.BoolPredicate = "une-zero"
	}
}

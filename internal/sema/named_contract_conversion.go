package sema

import (
	"math"
	"math/big"

	"sec/internal/ast"
)

// ResolvedContractPredicate is one runtime check of a constrained named
// type, in effective declaration order. Kind is the core ContractKind member
// a violation reports and DeclarationIndex its zero-based source-order index
// (contracts.md § 13.1).
type ResolvedContractPredicate struct {
	Kind             string
	DeclarationIndex int
	// Min and Max bound a Range check; nil is an open side.
	Min, Max  *big.Int
	Exclusive bool
	// Values are the allowed members of an In check.
	Values []*big.Int
	// Divisor is the MultipleOf divisor.
	Divisor *big.Int
	// FormatBits is the binary format (32 or 64) of a float carrier. A float
	// Range compares against MinBits with an ordered >= and against MaxBits
	// with an ordered <= (or < when MaxStrict). Each bound is a value of the
	// base type (contracts.md § 6(2)), so it is shaped like a literal of that
	// format, nearest-even; ordered comparison makes NaN fail.
	FormatBits       uint16
	MinBits, MaxBits *uint64
	MaxStrict        bool
}

// ResolvedNamedContractConversion is Sema's detached plan for a runtime
// conversion into a constrained named type: the intrinsic conversion from
// the source into the carrier first, then every declared contract in order,
// then nominal construction. Unavailable names a part this plan cannot
// represent yet, so lowering stops at a named boundary instead of guessing.
//
// Rules:
//   - rules/types/contracts.md — §§ 12, 13, 13.1
//   - rules/mlir/packages/sec-mlir-dialect_package22.md — §§ 9–13, 18–19
type ResolvedNamedContractConversion struct {
	SourceType, CarrierType, TargetType Type
	// Intrinsic is the integer conversion from the source into the carrier,
	// or nil when the source already has the carrier type.
	Intrinsic   *ResolvedIntegerConversion
	Predicates  []ResolvedContractPredicate
	ErrorType   Type
	Unavailable string
}

// recordNamedContractConversion publishes the plan of a runtime-checked
// conversion into a constrained named integer type. Contract bounds come
// from the resolved contract facts; a bound only the compile-time evaluator
// establishes, and non-integer carriers, are recorded as unavailable.
// Rules: rules/types/contracts.md — §§ 13, 13.1; rules/mlir/packages/sec-mlir-dialect_package22.md — §§ 9–15.
func (a *Analyzer) recordNamedContractConversion(call *ast.CallExpression, target, source Type) {
	if call == nil {
		return
	}
	delete(a.resolvedNamedContractConversions, call)
	runtime := a.runtimeContractConversion(call)
	if !runtime {
		// A constant argument was already proved against every contract
		// (contracts.md § 12(1)); its plan has no runtime predicate.
		if len(call.Arguments) != 1 || !hasContracts(target) || !isIntegerType(target) && target.Kind != FloatType {
			return
		}
		if _, constant := a.integerConstantValue(call.Arguments[0]); !constant {
			return
		}
	}
	carrier := target
	for carrier.Named && carrier.NamedBase != nil {
		carrier = *carrier.NamedBase
	}
	plan := ResolvedNamedContractConversion{SourceType: source, CarrierType: carrier, TargetType: target, ErrorType: a.types["ConversionError"]}
	floating := carrier.Kind == FloatType
	format := uint16(0)
	if floating {
		switch {
		case carrier.Name == "float32":
			format = 32
		case carrier.Name == "float64":
			format = 64
		default:
			format = a.ScalarPlanWidthBits()
		}
	}
	switch {
	case floating && format != 32 && format != 64:
		plan.Unavailable = "float carrier without a resolved binary format"
	case floating && !sameConcreteType(source, carrier):
		plan.Unavailable = "intrinsic conversion from " + typeDisplayName(source) + " to " + typeDisplayName(carrier)
	case !floating && !isBuiltinConversionInteger(carrier) && carrier.Name != "byte":
		plan.Unavailable = "constrained " + typeDisplayName(carrier) + " carrier"
	case sameConcreteType(source, carrier):
	case isBuiltinConversionInteger(source) || source.Name == "byte":
		var constant *big.Int
		if value, ok := a.integerConstantValue(call.Arguments[0]); ok {
			constant = value
		}
		proof := a.integerConversionProof(carrier, source, constant)
		plan.Intrinsic = &ResolvedIntegerConversion{SourceType: source, TargetType: carrier, Exact: proof.Proven(), Proof: proof}
	default:
		plan.Unavailable = "intrinsic conversion from " + typeDisplayName(source) + " to " + typeDisplayName(carrier)
	}
	for index, contract := range target.Contracts {
		if plan.Unavailable != "" || !runtime {
			break
		}
		predicate := ResolvedContractPredicate{DeclarationIndex: index, FormatBits: format}
		if floating {
			plan.Unavailable = floatContractPredicate(&predicate, contract)
			plan.Predicates = append(plan.Predicates, predicate)
			continue
		}
		switch c := contract.(type) {
		case RangeContract:
			predicate.Kind, predicate.Min, predicate.Max, predicate.Exclusive = "Range", c.Min, c.Max, c.Exclusive
			if c.Min == nil && c.MinLexeme != "" || c.Max == nil && c.MaxLexeme != "" {
				plan.Unavailable = "range bound established by compile-time evaluation"
			}
		case MembershipContract:
			predicate.Kind = "In"
			for _, value := range c.Values {
				if value.Integer == nil {
					plan.Unavailable = "non-integer membership value"
					break
				}
				predicate.Values = append(predicate.Values, new(big.Int).Set(value.Integer))
			}
		case MultipleOfContract:
			predicate.Kind, predicate.Divisor = "MultipleOf", c.Value
			if c.Value == nil || c.Value.Sign() == 0 {
				plan.Unavailable = "multipleOf without a resolved divisor"
			}
		case MarkerContract:
			switch c.Name {
			case "odd":
				predicate.Kind = "Odd"
			case "even":
				predicate.Kind = "Even"
			default:
				plan.Unavailable = c.Name + " contract on an integer carrier"
			}
		default:
			plan.Unavailable = "contract family without a runtime integer plan"
		}
		plan.Predicates = append(plan.Predicates, predicate)
	}
	a.resolvedNamedContractConversions[call] = plan
}

// ResolvedNamedContractConversionOf returns a detached copy of the recorded
// plan; it never infers one.
// Rules: rules/mlir/packages/sec-mlir-dialect_package22.md — §§ 9–13 (read-only plan query).
func (a *Analyzer) ResolvedNamedContractConversionOf(call *ast.CallExpression) (ResolvedNamedContractConversion, bool) {
	if a == nil || call == nil {
		return ResolvedNamedContractConversion{}, false
	}
	plan, ok := a.resolvedNamedContractConversions[call]
	if !ok {
		return ResolvedNamedContractConversion{}, false
	}
	predicates := make([]ResolvedContractPredicate, len(plan.Predicates))
	for index, predicate := range plan.Predicates {
		copied := predicate
		if predicate.Min != nil {
			copied.Min = new(big.Int).Set(predicate.Min)
		}
		if predicate.Max != nil {
			copied.Max = new(big.Int).Set(predicate.Max)
		}
		if predicate.Divisor != nil {
			copied.Divisor = new(big.Int).Set(predicate.Divisor)
		}
		if predicate.MinBits != nil {
			bits := *predicate.MinBits
			copied.MinBits = &bits
		}
		if predicate.MaxBits != nil {
			bits := *predicate.MaxBits
			copied.MaxBits = &bits
		}
		copied.Values = make([]*big.Int, len(predicate.Values))
		for position, value := range predicate.Values {
			copied.Values[position] = new(big.Int).Set(value)
		}
		predicates[index] = copied
	}
	plan.Predicates = predicates
	if plan.Intrinsic != nil {
		intrinsic := *plan.Intrinsic
		plan.Intrinsic = &intrinsic
	}
	return plan, true
}

// floatContractPredicate resolves one contract of a float carrier: Range
// with directed-rounded bounds and Finite. It returns the reason a contract
// family cannot be represented yet, or "".
// Rules: rules/types/contracts.md — §§ 4, 8, 13.1; rules/mlir/packages/sec-mlir-dialect_package22.md — § 14.
func floatContractPredicate(predicate *ResolvedContractPredicate, contract Contract) string {
	switch c := contract.(type) {
	case RangeContract:
		predicate.Kind, predicate.Exclusive = "Range", c.Exclusive
		if c.ExactMin == nil && c.MinLexeme != "" || c.ExactMax == nil && c.MaxLexeme != "" {
			return "range bound established by compile-time evaluation"
		}
		if c.ExactMin != nil {
			bits := baseFloatBits(c.ExactMin, predicate.FormatBits)
			predicate.MinBits = &bits
		}
		if c.ExactMax != nil {
			bits := baseFloatBits(c.ExactMax, predicate.FormatBits)
			predicate.MaxBits = &bits
			predicate.MaxStrict = c.Exclusive
		}
		return ""
	case MarkerContract:
		if c.Name == "finite" {
			predicate.Kind = "Finite"
			return ""
		}
		return c.Name + " contract on a float carrier"
	}
	return "contract family without a runtime float plan"
}

// baseFloatBits shapes an exact bound into the carrier format with
// round-to-nearest, ties-to-even, as a literal of that type
// (package20.md §§ 13–17).
func baseFloatBits(bound *big.Rat, format uint16) uint64 {
	if format == 32 {
		value, _ := bound.Float32()
		return uint64(math.Float32bits(value))
	}
	value, _ := bound.Float64()
	return math.Float64bits(value)
}

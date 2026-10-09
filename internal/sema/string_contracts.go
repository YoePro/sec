package sema

import (
	"sec/internal/ast"
	"sec/internal/sema/stringcontract"
)

// StringLengthContractPlan snapshots a complete supported string conjunction.
// Other contract families are rejected, never discarded from a runtime plan.
// Rules: rules/types/contracts.md — Composition, String and collection contracts.
func StringLengthContractPlan(typ Type) (stringcontract.Plan, bool) {
	if typ.Kind != StringType || len(typ.Contracts) == 0 {
		return stringcontract.Plan{}, false
	}
	requirements := []stringcontract.Requirement{}
	for index, contract := range typ.Contracts {
		switch c := contract.(type) {
		case LengthContract:
			if !isLengthContractName(c.Name) || c.Value == nil {
				return stringcontract.Plan{}, false
			}
			requirements = append(requirements, stringcontract.Requirement{Kind: c.Name, Bound: c.Value.String(), DeclarationIndex: uint64(index)})
		case MarkerContract:
			if c.Name != "notEmpty" {
				return stringcontract.Plan{}, false
			}
			requirements = append(requirements, stringcontract.Requirement{Kind: c.Name, DeclarationIndex: uint64(index)})
		default:
			return stringcontract.Plan{}, false
		}
	}
	plan, err := stringcontract.New(requirements)
	return plan, err == nil
}

// ResolvedStringContractConversionOf consumes exact analyzed expression facts,
// and retains whether Sema requires a runtime check or already proved the value.
// Rules: rules/types/contracts.md — Conversion failure layers;
// rules/types/types.md — Explicit conversions.
func (a *Analyzer) ResolvedStringContractConversionOf(call *ast.CallExpression) (stringcontract.Plan, bool, bool) {
	if a == nil || len(a.errors) > 0 {
		return stringcontract.Plan{}, false, false
	}
	target, resolved := a.expressionTypes[call]
	if !resolved || len(call.Arguments) != 1 || len(a.functions[callExpressionName(call)]) != 0 {
		return stringcontract.Plan{}, false, false
	}
	plan, supported := StringLengthContractPlan(target)
	return plan, a.runtimeContractConversion(call), supported
}

// ResolvedContractNode reports whether the exact AST contract was consumed by
// the latest Sema run, preventing stale declaration snapshots from erasing checks.
// Rules: rules/compiler/compiler_pipeline.md — lowering prerequisites.
func (a *Analyzer) ResolvedContractNode(contract ast.Contract) bool {
	return a != nil && len(a.errors) == 0 && a.analyzedContractNodes[contract]
}

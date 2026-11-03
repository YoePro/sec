package sema

import "sec/internal/ast"

// EnumDomainInfo describes the domain already established by Sema. Open bit
// domains include unnamed encodings; declared value classes cannot bound them.
type EnumDomainInfo struct {
	Underlying string
	Open       bool
	BitWidth   int64
}

// EnumDomainOf projects the canonical enum classifier, without resolving a
// carrier or deriving representation bounds in the tooling consumer.
// Rules: rules/declarations/enums.md — §§9–10,18.
func EnumDomainOf(typ Type) (EnumDomainInfo, bool) {
	if typ.Kind != EnumType || typ.Underlying == "" {
		return EnumDomainInfo{}, false
	}
	return EnumDomainInfo{Underlying: typ.Underlying, Open: enumIsOpenBitDomain(typ), BitWidth: typ.BitWidth}, true
}

// EnumConversionInfo is a detached presentation of an accepted explicit
// conversion. Checked means the recorded result is Result, not that tooling
// has independently proved a check necessary. Proof is present only when the
// canonical numeric conversion producer retained exactness evidence.
type EnumConversionInfo struct {
	Source, Target, Result Type
	Enum                   Type
	Checked                bool
	Proof                  IntegerConversionProof
}

// EnumConversionInfoOf presents retained result/operand types only for calls
// identified by the accepted explicit-conversion producer. User, intrinsic and
// indirect function calls do not become casts by their argument/result shape.
// Numeric exactness comes exclusively from the retained conversion contract.
// Rules: rules/declarations/enums.md — §§12,18;
// rules/types/types.md — "Exact integer conversion proof";
// rules/tooling/lsp.md — "Unknown facts", "Hover".
func (a *Analyzer) EnumConversionInfoOf(call *ast.CallExpression) (EnumConversionInfo, bool) {
	if a == nil || call == nil || len(call.Arguments) != 1 {
		return EnumConversionInfo{}, false
	}
	if !a.enumConversionCalls[call] {
		return EnumConversionInfo{}, false
	}
	if _, selected := a.ResolvedCallTarget(call); selected {
		return EnumConversionInfo{}, false
	}
	if callee, known := a.ResolvedTypeOf(call.Callee); known && callee.Kind == FunctionType {
		return EnumConversionInfo{}, false
	}
	source, sourceOK := a.ResolvedTypeOf(call.Arguments[0])
	result, resultOK := a.ResolvedTypeOf(call)
	if !sourceOK || !resultOK || source.Kind == InvalidType || result.Kind == InvalidType {
		return EnumConversionInfo{}, false
	}
	target := result
	checked := target.Kind == ResultType && len(target.TypeArgs) == 2
	if checked {
		target = target.TypeArgs[0]
	}
	enum := target
	switch {
	case target.Kind == EnumType && (isIntegerType(source) || source.Kind == StringType):
	case source.Kind == EnumType && (isIntegerType(target) || target.Kind == StringType):
		enum = source
	default:
		return EnumConversionInfo{}, false
	}
	info := EnumConversionInfo{Source: enumConversionTypeSnapshot(source), Target: enumConversionTypeSnapshot(target),
		Result: enumConversionTypeSnapshot(result), Enum: enumConversionTypeSnapshot(enum), Checked: checked}
	if fact, found := a.ResolvedEnumConversionOf(call); found {
		info.Proof = fact.Proof
	}
	return info, true
}

// recordEnumConversionCall retains identity only at the accepted explicit
// conversion branch. Matching argument/result shapes cannot prove a call is a
// cast: intrinsic operations may also take an enum and return an integer.
// Rules: rules/declarations/enums.md — §12;
// rules/tooling/lsp.md — "One semantic source of truth", "Unknown facts".
func (a *Analyzer) recordEnumConversionCall(call *ast.CallExpression) {
	if a.enumConversionCalls == nil {
		a.enumConversionCalls = map[*ast.CallExpression]bool{}
	}
	a.enumConversionCalls[call] = true
}

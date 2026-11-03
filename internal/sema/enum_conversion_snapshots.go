package sema

import (
	"maps"
	"math/big"
)

// cloneEnumConversion detaches the source/target identity and proof domains
// from both the analyzer and earlier callers' snapshots.
// Rules: rules/compiler/semantic_ir.md — §124;
// rules/types/types.md — Exact integer conversion proof (MD-104).
func cloneEnumConversion(fact ResolvedEnumConversion) ResolvedEnumConversion {
	fact.EnumType = enumConversionTypeSnapshot(fact.EnumType)
	fact.IntegerType = enumConversionTypeSnapshot(fact.IntegerType)
	fact.Proof.SourceMin = cloneConversionInteger(fact.Proof.SourceMin)
	fact.Proof.SourceMax = cloneConversionInteger(fact.Proof.SourceMax)
	fact.Proof.TargetMin = cloneConversionInteger(fact.Proof.TargetMin)
	fact.Proof.TargetMax = cloneConversionInteger(fact.Proof.TargetMax)
	return fact
}

// enumConversionTypeSnapshot projects conversion-relevant nominal identity,
// generic arguments, scalar/unit identity and enum inventory into owned data.
// Declaration bodies (methods, fields, contracts and default expressions) are
// not part of a conversion fact; consumers query their separate resolved plans.
// Rules: rules/types/types.md — Type identity, Exact integer conversion proof;
// rules/compiler/semantic_ir.md — resolved enum identities, §124.
func enumConversionTypeSnapshot(t Type) Type {
	s := Type{
		Name: t.Name, Module: t.Module, Kind: t.Kind, Named: t.Named, Declared: t.Declared, Intrinsic: t.Intrinsic,
		FixedNamedArguments: t.FixedNamedArguments, Underlying: t.Underlying,
		ConstantBaseName: t.ConstantBaseName, ConstantAncestors: append([]string(nil), t.ConstantAncestors...),
		ExplicitlyNonCopyable: t.ExplicitlyNonCopyable, NoCopyPolicyOrigin: t.NoCopyPolicyOrigin,
		ErrorAssignable: t.ErrorAssignable, CustomFree: t.CustomFree, MonotonicPoint: t.MonotonicPoint,
		Unit: t.Unit, Dimension: Dimension{Base: maps.Clone(t.Dimension.Base)}, UnitSemantics: t.UnitSemantics,
		ReferenceMutable: t.ReferenceMutable, ArrayShape: t.ArrayShape, ArrayLength: t.ArrayLength, ArrayLengthDecimal: t.ArrayLengthDecimal,
		MinInteger: cloneConversionInteger(t.MinInteger), MaxInteger: cloneConversionInteger(t.MaxInteger),
		BitWidth: t.BitWidth, FloatBits: t.FloatBits, EnumDefault: t.EnumDefault, AssociatedEnum: t.AssociatedEnum,
		EnumValues: append([]string(nil), t.EnumValues...), DeclarationToken: t.DeclarationToken,
		ConstArgs: append([]int64(nil), t.ConstArgs...), GenericParameters: append([]string(nil), t.GenericParameters...),
		StaticElementCount: cloneConversionInteger(t.StaticElementCount),
		FunctionCapability: t.FunctionCapability, FunctionVariadic: t.FunctionVariadic,
	}
	if t.MinInt != nil {
		v := *t.MinInt
		s.MinInt = &v
	}
	if t.MaxInt != nil {
		v := *t.MaxInt
		s.MaxInt = &v
	}
	if t.MinUint != nil {
		v := *t.MinUint
		s.MinUint = &v
	}
	if t.MaxUint != nil {
		v := *t.MaxUint
		s.MaxUint = &v
	}
	s.UnitSemantics.SourceFactors = append([]string(nil), t.UnitSemantics.SourceFactors...)
	s.UnitSemantics.Factors = maps.Clone(t.UnitSemantics.Factors)
	s.UnitSemantics.Categories = maps.Clone(t.UnitSemantics.Categories)
	s.UnitSemantics.Scale = cloneRat(t.UnitSemantics.Scale)
	s.UnitSemantics.Offset = cloneRat(t.UnitSemantics.Offset)
	s.UnitSemantics.LogBase = cloneRat(t.UnitSemantics.LogBase)
	s.UnitSemantics.LogFactor = cloneRat(t.UnitSemantics.LogFactor)
	if t.EnumConsts != nil {
		s.EnumConsts = make(map[string]EnumValue, len(t.EnumConsts))
		for name, value := range t.EnumConsts {
			value.Value = cloneConversionInteger(value.Value)
			if value.StringValue != nil {
				text := *value.StringValue
				value.StringValue = &text
			}
			s.EnumConsts[name] = value
		}
	}
	s.TypeArgs = make([]Type, len(t.TypeArgs))
	for i, arg := range t.TypeArgs {
		s.TypeArgs[i] = enumConversionTypeSnapshot(arg)
	}
	if t.Element != nil {
		element := enumConversionTypeSnapshot(*t.Element)
		s.Element = &element
	}
	s.FunctionParameterTypes = make([]Type, len(t.FunctionParameterTypes))
	for i, param := range t.FunctionParameterTypes {
		s.FunctionParameterTypes[i] = enumConversionTypeSnapshot(param)
	}
	if t.FunctionReturnType != nil {
		result := enumConversionTypeSnapshot(*t.FunctionReturnType)
		s.FunctionReturnType = &result
	}
	return s
}

// cloneConversionInteger preserves exact numeric evidence without sharing the
// mutable big.Int storage (rules/types/types.md — Exact integer conversion proof).
func cloneConversionInteger(value *big.Int) *big.Int {
	if value == nil {
		return nil
	}
	return new(big.Int).Set(value)
}

package semantic

import (
	"fmt"
	"math/big"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// resolvedDefaultSupported selects canonical scalar defaults and the aggregate
// defaults supported by the requested package, retaining declared scalar widths.
// Rules: rules/types/default_values.md — Named types, Floating and decimal ranges;
// rules/mlir/packages/sec-mlir-dialect_package13.md — §26;
// rules/mlir/packages/sec-mlir-dialect_package14.md — §§24–27.
func (fb *functionBuilder) resolvedDefaultSupported(typ sema.Type) bool {
	switch typ.Kind {
	case sema.IntType, sema.UintType, sema.BoolType, sema.CharType, sema.RuneType,
		sema.StringType, sema.FloatType, sema.DecimalType:
		return true
	case sema.StructType:
		return fb.owner.maxPackage >= 13
	case sema.ArrayType:
		return fb.owner.maxPackage >= 14
	}
	return false
}

// buildResolvedDefault materializes exact canonical default facts without
// re-inferring an AST spelling or expanding compact fixed-array defaults.
// Rules: rules/types/default_values.md — Backend lowering, A.7–A.9;
// rules/mlir/packages/sec-mlir-dialect_package13.md — §26;
// rules/mlir/packages/sec-mlir-dialect_package14.md — §§24–27.
func (fb *functionBuilder) buildResolvedDefault(typ sema.Type, resolution sema.DefaultResolution, loc Location) (builtValue, error) {
	typeID, err := fb.owner.internType(typ)
	if err != nil {
		return builtValue{}, err
	}
	switch resolution.Kind {
	case sema.PrimitiveDefault, sema.NamedDefault, sema.RangeDefault, sema.MembershipDefault, sema.ExplicitTypeDefault:
		switch typ.Kind {
		case sema.BoolType:
			value := resolution.Value.Bool
			return fb.result(Operation{Kind: OpConstBool, Bool: &value, Location: loc}, typeID), nil
		case sema.StringType:
			return fb.result(Operation{Kind: OpConstString, String: resolution.Value.String, Location: loc}, typeID), nil
		case sema.IntType, sema.UintType, sema.CharType, sema.RuneType:
			if resolution.Value.Integer == nil {
				return builtValue{}, fb.unsupported("non-integer scalar default", lexer.Token{})
			}
			return fb.result(Operation{Kind: OpConstInt, Integer: new(big.Int).Set(resolution.Value.Integer), Location: loc}, typeID), nil
		case sema.FloatType:
			return fb.result(Operation{Kind: OpConstFloat, FloatLexeme: resolution.Value.Lexeme, Location: loc}, typeID), nil
		case sema.DecimalType:
			decimal, parseErr := parseDecimal(resolution.Value.Lexeme)
			if parseErr != nil {
				return builtValue{}, parseErr
			}
			return fb.result(Operation{Kind: OpConstDecimal, Decimal: &decimal, Location: loc}, typeID), nil
		}
	case sema.StructDefault:
		definition, ok := fb.owner.structDefinition(typeID)
		if !ok {
			break
		}
		op := Operation{Kind: OpStructConstruct, Location: loc}
		resolvedByName := map[string]sema.DefaultResolution{}
		for _, field := range resolution.Fields {
			resolvedByName[field.Name] = field.Value
		}
		for index, field := range typ.Fields {
			value, valueErr := fb.buildResolvedDefault(field.Type, resolvedByName[field.Name], loc)
			if valueErr != nil {
				return builtValue{}, valueErr
			}
			op.Operands = append(op.Operands, value.id)
			op.StructOrigins = append(op.StructOrigins, StructOriginDefault)
			op.StructActions = append(op.StructActions, StructActionConstructDirect)
			_ = definition.Fields[index]
		}
		return fb.result(op, typeID), nil
	case sema.ArrayDefault:
		// SEC-MLIR Package 14 sections 24-27: array defaults remain one compact
		// semantic operation. Zero length never queries or constructs an element;
		// positive lengths are restricted to the infallible trivial P14 subset.
		length, fixed := sema.FixedArrayLength(typ)
		if typ.Kind != sema.ArrayType || typ.Element == nil || !fixed {
			return builtValue{}, fb.unsupported("dynamic or malformed array default", lexer.Token{})
		}
		if resolution.ArrayLengthDecimal != length.String() {
			return builtValue{}, fmt.Errorf("array default length fact mismatch for %s", typ.Name)
		}
		if length.Sign() != 0 {
			if resolution.ArrayElementDefault == nil || sema.CopyClassificationOf(*typ.Element) != sema.CopyTrivial || !sema.TriviallyDestructible(*typ.Element) {
				return builtValue{}, fb.unsupported("non-trivial fixed-array default for "+typ.Name, lexer.Token{})
			}
		}
		elementType, elementErr := fb.owner.internType(*typ.Element)
		if elementErr != nil {
			return builtValue{}, elementErr
		}
		return fb.result(Operation{
			Kind: OpArrayDefault, ArrayElementType: elementType,
			ArrayLength: length.String(), Location: loc,
		}, typeID), nil
	}
	return builtValue{}, fb.unsupported("resolved default "+string(resolution.Kind)+" for "+typ.Name, lexer.Token{})
}

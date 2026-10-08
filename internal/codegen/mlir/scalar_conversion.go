package mlir

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/codegen/readiness"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
)

// emitBuiltinNumericConversion uses canonical conversion destinations and preserves exact contextual constants.
// Rules: rules/types/types.md — "int and uint", "Context shaping";
// rules/declarations/enums.md — "Underlying type"; correction5.md — selected scalar facts.
func (g *Generator) emitBuiltinNumericConversion(expr *ast.CallExpression, name string) (value, error) {
	if len(expr.Arguments) != 1 {
		return value{}, fmt.Errorf("conversion to %s expects 1 argument", name)
	}
	targetType := g.mlirBuiltinNumericType(name)
	targetUnsigned := isUnsignedBuiltinName(name)
	source, err := g.emitExpressionForTargetUnsigned(expr.Arguments[0], targetType, targetUnsigned)
	if err != nil {
		return value{}, err
	}

	switch {
	case isMLIRDecimalType(targetType):
		return g.coerceExplicitInteger(source, targetType, targetUnsigned, expr.Arguments[0], expr.Token)
	case isMLIRIntegerType(targetType) && isMLIRDecimalType(source.typ):
		return g.coerceExplicitInteger(source, targetType, targetUnsigned, expr.Arguments[0], expr.Token)
	case source.typ == "!llvm.ptr" && isMLIRIntegerType(targetType):
		result := g.nextTemp()
		g.write("    %s = llvm.ptrtoint %s : !llvm.ptr to %s\n", result, source.ref, targetType)
		return value{typ: targetType, ref: result, unsigned: targetUnsigned}, nil
	default:
		result, err := g.coerceExplicitInteger(source, targetType, targetUnsigned, expr.Arguments[0], expr.Token)
		if err != nil {
			return value{}, err
		}
		result.enumName = ""
		return result, nil
	}
}

// coerceExplicitInteger validates integer destination bounds before coercion;
// calls and explicit AST conversions, including nominal carriers, share it.
// Rules: rules/types/types.md — Explicit conversions; MD-012 correction §4.
func (g *Generator) coerceExplicitInteger(source value, target string, unsigned bool, expression ast.Expression, token lexer.Token) (value, error) {
	if (isMLIRDecimalType(source.typ) || isMLIRFloatType(source.typ)) && isMLIRIntegerType(target) && target != "i1" {
		return value{}, &semantic.UnsupportedFeatureError{Feature: "checked scalar-to-integer conversion requires exactness and target-domain validation (MD-012)", Location: semantic.Location{File: token.File, Line: token.Line, Column: token.Column}}
	}
	if coefficient, ok := decimalCoefficientType(target); ok && isMLIRIntegerType(source.typ) {
		bits := integerBitWidth(source.typ)
		if err := readiness.CheckIntegerConversion(bits, source.unsigned || bits == 1, integerBitWidth(coefficient), false, expression, source.integer, token); err != nil {
			return value{}, err
		}
	}
	if isMLIRIntegerType(source.typ) && isMLIRIntegerType(target) && target != "i1" && source.enumName == "" {
		bits := integerBitWidth(source.typ)
		if err := readiness.CheckIntegerConversion(bits, source.unsigned || bits == 1, integerBitWidth(target), unsigned, expression, source.integer, token); err != nil {
			return value{}, err
		}
	}
	return g.coerceValue(source, target, unsigned)
}

// convertDecimalToInteger refuses unchecked precision loss even when an
// internal storage coercion reaches this path without an explicit cast node.
// Rules: rules/types/types.md — Explicit conversions; MD-012 correction §4.
func (g *Generator) convertDecimalToInteger(source value, targetType string, targetUnsigned bool) (value, error) {
	return value{}, &semantic.UnsupportedFeatureError{Feature: "checked scalar-to-integer conversion requires exactness and target-domain validation (MD-012)"}
}

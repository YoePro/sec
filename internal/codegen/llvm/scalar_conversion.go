package llvm

import (
	"fmt"
	"math/big"
	"sec/internal/ast"
	"sec/internal/codegen/readiness"
	"sec/internal/ir/semantic"
)

// emitBuiltinConversionCall preserves checked target-domain semantics before
// legacy integer conversion emission, including canonical native widths.
// Rules: rules/types/types.md — Explicit conversions; MD-012 correction §4.
func (g *Generator) emitBuiltinConversionCall(expr *ast.CallExpression, name string) (value, error) {
	if len(expr.Arguments) != 1 {
		return value{}, fmt.Errorf("conversion to %s expects 1 argument, got %d", name, len(expr.Arguments))
	}
	arg, err := g.emitExpression(expr.Arguments[0])
	if err != nil {
		return value{}, err
	}
	ref := &ast.TypeReference{Name: name}
	target := g.llvmType(ref)
	if _, integer := llvmIntegerWidth(target); integer && name != "bool" && (arg.typ == llvmDecimalType || arg.typ == "float" || arg.typ == "double") {
		return value{}, &semantic.UnsupportedFeatureError{Feature: "checked scalar-to-integer conversion requires exactness and target-domain validation (MD-012)", Location: semantic.Location{File: expr.Token.File, Line: expr.Token.Line, Column: expr.Token.Column}}
	}
	if target == llvmDecimalType {
		if bits, ok := llvmIntegerWidth(arg.typ); ok {
			known, _ := new(big.Int).SetString(arg.ref, 10)
			if err := readiness.CheckIntegerConversion(bits, arg.unsigned || bits == 1, 64, false, expr.Arguments[0], known, expr.Token); err != nil {
				return value{}, err
			}
		}
	}
	if sourceBits, ok := llvmIntegerWidth(arg.typ); ok && name != "bool" {
		if targetBits, ok := llvmIntegerWidth(target); ok {
			if _, enum := g.enums[name]; !enum {
				known, _ := new(big.Int).SetString(arg.ref, 10)
				if err := readiness.CheckIntegerConversion(sourceBits, arg.unsigned || sourceBits == 1, targetBits, g.typeReferenceIsUnsigned(ref), expr.Arguments[0], known, expr.Token); err != nil {
					return value{}, err
				}
			}
		}
	}
	result, err := g.coerceValue(arg, target)
	result.unsigned = g.typeReferenceIsUnsigned(ref)
	return result, err
}

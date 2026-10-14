package mlir

import (
	"fmt"
	"strconv"

	"sec/internal/ast"
)

// emitMemberExpression preserves native uint identity for string and fixed-array length values.
// Rules: rules/types/types.md — "int and uint", "Context shaping";
// rules/compiler/compiler_known_members.md — "Legacy lowercase member spellings";
// rules/declarations/enums.md — "Underlying type"; correction5.md — selected scalar facts.
func (g *Generator) emitMemberExpression(expr *ast.MemberExpression) (value, error) {
	if expr.Object == nil || expr.Property == nil {
		return value{}, fmt.Errorf("emit-mlir member expression is incomplete")
	}
	if typeName, ok := mlirExpressionPath(expr.Object); ok {
		if info, exists := g.enums[typeName]; exists {
			literal, found := info.values[expr.Property.Value]
			if !found {
				return value{}, fmt.Errorf("emit-mlir unknown enum value %s.%s", typeName, expr.Property.Value)
			}
			result := g.emitIntegerConstantUnsigned(literal, info.typ, info.unsigned)
			result.enumName = info.name
			return result, nil
		}
	}
	object, err := g.emitExpression(expr.Object)
	if err != nil {
		return value{}, err
	}
	if object.typ == "string" {
		switch expr.Property.Value {
		case "ptr", "Ptr":
			return value{typ: "!llvm.ptr", ref: object.ref}, nil
		case "Len", "len":
			return g.coerceValue(g.emitStringRuneLength(object), g.nativeIntegerType(), true)
		case "SizeOf":
			// MD-058: the value-form SizeOf of a string is its UTF-8 payload bytes.
			return g.coerceValue(value{typ: "i64", ref: object.len, unsigned: true}, g.nativeIntegerType(), true)
		default:
			return value{}, fmt.Errorf("emit-mlir unknown string property %s", expr.Property.Value)
		}
	}
	if length, _, ok := parseMLIRArrayType(object.typ); ok {
		if expr.Property.Value != "Len" && expr.Property.Value != "len" {
			return value{}, fmt.Errorf("emit-mlir unknown array property %s", expr.Property.Value)
		}
		return g.emitIntegerConstantUnsigned(strconv.FormatInt(length, 10), g.nativeIntegerType(), true), nil
	}
	if object.structName == "" {
		return value{}, fmt.Errorf("emit-mlir member access currently requires a struct value")
	}
	info, ok := g.structs[object.structName]
	if !ok {
		return value{}, fmt.Errorf("emit-mlir unknown struct type %s", object.structName)
	}
	index, field, ok := info.field(expr.Property.Value)
	if !ok {
		return value{}, fmt.Errorf("emit-mlir unknown field %s.%s", info.name, expr.Property.Value)
	}
	result := g.nextTemp()
	g.write("    %s = llvm.extractvalue %s[%d] : %s\n", result, object.ref, index, info.typ)
	if field.string {
		return g.unpackString(value{typ: mlirStringType, ref: result})
	}
	return value{
		typ:        field.typ,
		ref:        result,
		structName: field.structName,
		enumName:   field.enumName,
		unsigned:   field.unsigned,
	}, nil
}

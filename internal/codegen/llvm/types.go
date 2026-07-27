package llvm

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
)

const llvmDecimalType = "%sec.decimal"

func llvmReturnType(ref *ast.TypeReference) string {
	if ref == nil {
		return "void"
	}

	if ref.Name == "fn" || ref.FunctionReturnType != nil {
		return "ptr"
	}
	if ref.Name == "RawPtr" {
		return "ptr"
	}

	switch ref.Name {
	case "bool":
		return "i1"
	case "void":
		return "void"
	case "int":
		return "i32"
	case "uint":
		return "i64"
	case "int64":
		return "i64"
	case "uint64":
		return "i64"
	case "byte":
		return "i8"
	case "string":
		return "ptr"
	case "decimal":
		return llvmDecimalType
	default:
		return "void"
	}
}

func llvmParameterType(param *ast.Parameter) string {
	if param.Ref {
		return "ptr"
	}
	return llvmReturnType(param.Type)
}

func (g *Generator) llvmType(ref *ast.TypeReference) string {
	if ref == nil {
		return "void"
	}
	if ref.Name == "Result" && len(ref.TypeArgs) == 2 {
		return g.llvmType(ref.TypeArgs[0])
	}
	if ref.Name == "decimal" {
		g.needsDecimal = true
		return llvmDecimalType
	}
	if enum, ok := g.enums[ref.Name]; ok {
		return enum.typ
	}
	if alias, ok := g.typeAliases[ref.Name]; ok {
		return g.llvmType(alias)
	}
	return llvmReturnType(ref)
}

func (g *Generator) llvmParameterType(param *ast.Parameter) string {
	if param.Ref {
		return "ptr"
	}
	return g.llvmType(param.Type)
}

func (g *Generator) registerEnum(enumDecl *ast.EnumDeclaration, owner string) {
	if enumDecl == nil || enumDecl.Name == nil {
		return
	}
	name := enumDecl.Name.Value
	if owner != "" {
		name = owner + "." + name
	}
	typ := "i32"
	if enumDecl.BitUnderlying && enumDecl.UnderlyingBitWidth > 0 {
		typ = fmt.Sprintf("i%d", enumDecl.UnderlyingBitWidth)
	} else if enumDecl.UnderlyingType != nil {
		typ = llvmReturnType(enumDecl.UnderlyingType)
		if typ == "void" {
			typ = "i32"
		}
	}

	info := enumInfo{typ: typ, values: map[string]string{}}
	previous := big.NewInt(-1)
	for index, enumValue := range enumDecl.Values {
		if enumValue == nil || enumValue.Name == nil {
			continue
		}
		value := new(big.Int).Add(previous, big.NewInt(1))
		if enumValue.Initializer != nil {
			if parsed, ok := enumInitializerValue(enumValue.Initializer, big.NewInt(int64(index))); ok {
				value = parsed
			}
		}
		info.values[enumValue.Name.Value] = value.String()
		previous = new(big.Int).Set(value)
	}
	g.enums[name] = info
}

func (g *Generator) registerTypeDeclaration(typeDecl *ast.TypeDeclStatement, owner string) {
	if typeDecl == nil || typeDecl.Name == nil {
		return
	}
	name := typeDecl.Name.Value
	if owner != "" {
		name = owner + "." + name
	}

	if len(typeDecl.Variants) > 0 {
		info := enumInfo{typ: "i32", values: map[string]string{}}
		for i, variant := range typeDecl.Variants {
			if variant == nil {
				continue
			}
			info.values[variant.Value] = fmt.Sprintf("%d", i)
		}
		g.enums[name] = info
		return
	}

	switch {
	case typeDecl.BaseType != nil:
		g.typeAliases[name] = typeDecl.BaseType
	case typeDecl.AssignedType != nil:
		g.typeAliases[name] = typeDecl.AssignedType
	case typeDecl.StructType != nil:
		// TODO: Emit real LLVM struct layouts. The current first-pass codegen
		// only needs struct values to be recognizable placeholders.
		g.typeAliases[name] = &ast.TypeReference{Name: "byte"}
	}
}

func llvmZeroValue(typ string) string {
	if typ == llvmDecimalType {
		return "zeroinitializer"
	}
	return "0"
}

func enumInitializerValue(expr ast.Expression, iotaValue *big.Int) (*big.Int, bool) {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		return ast.ParseIntegerLiteralLexeme(expr.Token.Lexeme)
	case *ast.Identifier:
		if expr.Value == "iota" {
			return new(big.Int).Set(iotaValue), true
		}
		return nil, false
	case *ast.ConversionExpression:
		return enumInitializerValue(expr.Value, iotaValue)
	case *ast.CallExpression:
		if len(expr.Arguments) != 1 {
			return nil, false
		}
		return enumInitializerValue(expr.Arguments[0], iotaValue)
	case *ast.PrefixExpression:
		if expr.Operator != "-" {
			return nil, false
		}
		value, ok := enumInitializerValue(expr.Right, iotaValue)
		if !ok {
			return nil, false
		}
		return new(big.Int).Neg(value), true
	case *ast.InfixExpression:
		left, ok := enumInitializerValue(expr.Left, iotaValue)
		if !ok {
			return nil, false
		}
		right, ok := enumInitializerValue(expr.Right, iotaValue)
		if !ok {
			return nil, false
		}
		result := new(big.Int)
		switch expr.Operator {
		case "+":
			return result.Add(left, right), true
		case "-":
			return result.Sub(left, right), true
		case "*":
			return result.Mul(left, right), true
		case "<<":
			if !right.IsUint64() {
				return nil, false
			}
			return result.Lsh(left, uint(right.Uint64())), true
		case ">>":
			if !right.IsUint64() {
				return nil, false
			}
			return result.Rsh(left, uint(right.Uint64())), true
		default:
			return nil, false
		}
	default:
		return nil, false
	}
}

package llvm

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/codegen/llvm/scalar"
)

const llvmDecimalType = "%sec.decimal"

// llvmType maps supported aliases and representations, retaining an explicit
// failure when a nominal type's representation and contracts are unavailable.
// Rules: rules/types/types.md — Named types; contracts.md — Core rule.
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
		if g.resolvingAliases == nil {
			g.resolvingAliases = map[string]bool{}
		}
		if g.resolvingAliases[ref.Name] {
			g.typeFailure = unsupportedContract("cyclic unresolved type representation for "+ref.Name, ref.Token)
			return "void"
		}
		g.resolvingAliases[ref.Name] = true
		defer delete(g.resolvingAliases, ref.Name)
		return g.llvmType(alias)
	}
	typ := scalar.Type(ref, g.scalarPlan)
	if typ == "void" && ref.Name != "void" && g.typeFailure == nil {
		// An unresolved nominal name may carry contracts unavailable to this
		// raw AST backend. It must not silently become void.
		// Rules: rules/types/types.md — Named types; contracts.md — Core rule.
		g.typeFailure = unsupportedContract("unresolved type representation for "+ref.Name, ref.Token)
	}
	return typ
}

func (g *Generator) llvmParameterType(param *ast.Parameter) string {
	if param.Ref {
		return "ptr"
	}
	return g.llvmType(param.Type)
}

// typeReferenceIsUnsigned preserves the integer signedness required by
// rules/foundations/operators.md when the legacy direct LLVM backend is used.
// See correction2.md.
func (g *Generator) typeReferenceIsUnsigned(ref *ast.TypeReference) bool {
	if ref == nil {
		return false
	}
	if alias, ok := g.typeAliases[ref.Name]; ok {
		return g.typeReferenceIsUnsigned(alias)
	}
	switch ref.Name {
	case "uint", "uint8", "uint16", "uint32", "uint64", "uint128", "uint256", "byte", "bit":
		return true
	default:
		return false
	}
}

// registerEnum uses the selected native int carrier for ordinary enums and preserves explicit integer or bit carriers.
// Rules: rules/types/types.md — "int and uint", "Context shaping";
// rules/declarations/enums.md — "Underlying type"; rules/foundations/operators.md — integer operations.
func (g *Generator) registerEnum(enumDecl *ast.EnumDeclaration, owner string) {
	if enumDecl == nil || enumDecl.Name == nil {
		return
	}
	name := enumDecl.Name.Value
	if owner != "" {
		name = owner + "." + name
	}
	typ := g.nativeIntegerType()
	if enumDecl.BitUnderlying && enumDecl.UnderlyingBitWidth > 0 {
		typ = fmt.Sprintf("i%d", enumDecl.UnderlyingBitWidth)
	} else if enumDecl.UnderlyingType != nil {
		typ = g.llvmType(enumDecl.UnderlyingType)
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

// nativeIntegerType consumes the resolved platform width for inferred literals
// and uint-valued builtin members as well as explicit int/uint type references.
// Rules: rules/types/types.md — "int and uint", "Context shaping".
func (g *Generator) nativeIntegerType() string {
	return fmt.Sprintf("i%d", g.scalarPlan.PointerWidthBits)
}

// contextualIntegerConstant retains exact integer constants in an already
// established destination carrier without introducing a runtime conversion.
// Rules: rules/types/types.md — "Context shaping"; rules/foundations/operators.md
// — integer literal operands. Sema proves source signedness and representability.
func contextualIntegerConstant(arg value, targetType string) (value, bool) {
	if _, ok := llvmIntegerWidth(arg.typ); !ok {
		return value{}, false
	}
	width, ok := llvmIntegerWidth(targetType)
	if !ok {
		return value{}, false
	}
	literal, ok := new(big.Int).SetString(arg.ref, 10)
	if !ok {
		return value{}, false
	}
	limit := new(big.Int).Lsh(big.NewInt(1), uint(width))
	if literal.Sign() >= 0 && literal.Cmp(limit) < 0 || literal.Sign() < 0 && literal.Cmp(new(big.Int).Neg(new(big.Int).Rsh(limit, 1))) >= 0 {
		arg.typ = targetType
		return arg, true
	}
	return value{}, false
}

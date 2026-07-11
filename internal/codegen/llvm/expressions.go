package llvm

import (
	"fmt"
	"strings"

	"sec/internal/ast"
)

type value struct {
	typ    string
	ref    string
	lenRef string
}

func (g *Generator) emitExpression(expr ast.Expression) (value, error) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return g.emitIdentifier(expr)
	case *ast.IntegerLiteral:
		return value{typ: "i32", ref: expr.Token.Lexeme}, nil
	case *ast.BooleanLiteral:
		if expr.Value {
			return value{typ: "i1", ref: "true"}, nil
		}
		return value{typ: "i1", ref: "false"}, nil
	case *ast.StringLiteral:
		return g.emitStringLiteral(expr)
	case *ast.MemberExpression:
		return g.emitMemberExpression(expr)
	case *ast.PrefixExpression:
		return g.emitPrefixExpression(expr)
	case *ast.InfixExpression:
		return g.emitInfixExpression(expr)
	default:
		return value{}, fmt.Errorf("emit-llvm does not support expression %T yet", expr)
	}
}

func (g *Generator) emitIdentifier(expr *ast.Identifier) (value, error) {
	slot, ok := g.locals[expr.Value]
	if !ok {
		return value{}, fmt.Errorf("emit-llvm unknown identifier %s", expr.Value)
	}
	if slot.direct {
		return value{typ: slot.typ, ref: slot.ref, lenRef: slot.lenRef}, nil
	}
	if slot.typ == "string" {
		return value{typ: "string", ref: slot.ref, lenRef: slot.lenRef}, nil
	}
	temp := g.nextTemp()
	g.write("  %s = load %s, ptr %s\n", temp, slot.typ, slot.ptr)
	return value{typ: slot.typ, ref: temp}, nil
}

func (g *Generator) emitMemberExpression(expr *ast.MemberExpression) (value, error) {
	object, err := g.emitExpression(expr.Object)
	if err != nil {
		return value{}, err
	}
	if object.typ != "string" {
		return value{}, fmt.Errorf("emit-llvm only supports members on string for now")
	}
	switch expr.Property.Value {
	case "ptr":
		return value{typ: "ptr", ref: object.ref}, nil
	case "len":
		return value{typ: "i64", ref: object.lenRef}, nil
	default:
		return value{}, fmt.Errorf("unknown string member %s", expr.Property.Value)
	}
}

func (g *Generator) emitPrefixExpression(expr *ast.PrefixExpression) (value, error) {
	right, err := g.emitExpression(expr.Right)
	if err != nil {
		return value{}, err
	}
	switch expr.Operator {
	case "-":
		if right.typ != "i32" {
			return value{}, fmt.Errorf("emit-llvm unary - currently expects int")
		}
		temp := g.nextTemp()
		g.write("  %s = sub i32 0, %s\n", temp, right.ref)
		return value{typ: "i32", ref: temp}, nil
	case "!":
		if right.typ != "i1" {
			return value{}, fmt.Errorf("emit-llvm unary ! currently expects bool")
		}
		temp := g.nextTemp()
		g.write("  %s = xor i1 %s, true\n", temp, right.ref)
		return value{typ: "i1", ref: temp}, nil
	default:
		return value{}, fmt.Errorf("emit-llvm does not support prefix operator %q yet", expr.Operator)
	}
}

func (g *Generator) emitInfixExpression(expr *ast.InfixExpression) (value, error) {
	left, err := g.emitExpression(expr.Left)
	if err != nil {
		return value{}, err
	}
	right, err := g.emitExpression(expr.Right)
	if err != nil {
		return value{}, err
	}

	switch expr.Operator {
	case "+":
		return g.emitIntegerBinary("add", left, right)
	case "-":
		return g.emitIntegerBinary("sub", left, right)
	case "*":
		return g.emitIntegerBinary("mul", left, right)
	case "/":
		return g.emitIntegerBinary("sdiv", left, right)
	case "==":
		return g.emitCompare("eq", left, right), nil
	case "!=":
		return g.emitCompare("ne", left, right), nil
	case "<":
		return g.emitCompare("slt", left, right), nil
	case "<=":
		return g.emitCompare("sle", left, right), nil
	case ">":
		return g.emitCompare("sgt", left, right), nil
	case ">=":
		return g.emitCompare("sge", left, right), nil
	default:
		return value{}, fmt.Errorf("emit-llvm does not support operator %q yet", expr.Operator)
	}
}

func (g *Generator) emitIntegerBinary(op string, left value, right value) (value, error) {
	if left.typ != right.typ {
		return value{}, fmt.Errorf("emit-llvm binary operator requires matching operand types")
	}
	if left.typ != "i32" && left.typ != "i64" {
		return value{}, fmt.Errorf("emit-llvm binary operator currently expects integers")
	}
	temp := g.nextTemp()
	g.write("  %s = %s %s %s, %s\n", temp, op, left.typ, left.ref, right.ref)
	return value{typ: left.typ, ref: temp}, nil
}

func (g *Generator) emitCompare(predicate string, left value, right value) value {
	temp := g.nextTemp()
	g.write("  %s = icmp %s %s %s, %s\n", temp, predicate, left.typ, left.ref, right.ref)
	return value{typ: "i1", ref: temp}
}

func (g *Generator) emitBoolAnd(left value, right value) (value, error) {
	if left.typ != "i1" || right.typ != "i1" {
		return value{}, fmt.Errorf("emit-llvm boolean and expects bool operands")
	}
	temp := g.nextTemp()
	g.write("  %s = and i1 %s, %s\n", temp, left.ref, right.ref)
	return value{typ: "i1", ref: temp}, nil
}

func (g *Generator) emitBoolOr(left value, right value) (value, error) {
	if left.typ != "i1" || right.typ != "i1" {
		return value{}, fmt.Errorf("emit-llvm boolean or expects bool operands")
	}
	temp := g.nextTemp()
	g.write("  %s = or i1 %s, %s\n", temp, left.ref, right.ref)
	return value{typ: "i1", ref: temp}, nil
}

func (g *Generator) emitStringLiteral(expr *ast.StringLiteral) (value, error) {
	name := fmt.Sprintf("@.str.%d", g.stringID)
	g.stringID++

	bytes := append([]byte(expr.Value), 0)
	g.globals.WriteString(fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\"\n", name, len(bytes), llvmCString(bytes)))

	temp := g.nextTemp()
	g.write("  %s = getelementptr inbounds [%d x i8], ptr %s, i64 0, i64 0\n", temp, len(bytes), name)
	return value{typ: "string", ref: temp, lenRef: fmt.Sprintf("%d", len(expr.Value))}, nil
}

func llvmCString(bytes []byte) string {
	var out strings.Builder
	for _, b := range bytes {
		switch b {
		case '\\':
			out.WriteString("\\5C")
		case '"':
			out.WriteString("\\22")
		case '\n':
			out.WriteString("\\0A")
		case '\t':
			out.WriteString("\\09")
		case 0:
			out.WriteString("\\00")
		default:
			if b >= 32 && b <= 126 {
				out.WriteByte(b)
				continue
			}
			out.WriteString("\\")
			out.WriteString(fmt.Sprintf("%02X", b))
		}
	}
	return out.String()
}

func callExpressionName(expr *ast.CallExpression) string {
	if expr.Callee != nil {
		if name, ok := expressionPath(expr.Callee); ok {
			return name
		}
	}
	if expr.Function != nil {
		return expr.Function.Value
	}
	return ""
}

func expressionPath(expr ast.Expression) (string, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr.Value, true
	case *ast.MemberExpression:
		left, ok := expressionPath(expr.Object)
		if !ok {
			return "", false
		}
		return left + "." + expr.Property.Value, true
	default:
		return "", false
	}
}

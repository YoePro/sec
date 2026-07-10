package llvm

import (
	"fmt"
	"strings"

	"sec/internal/ast"
)

type value struct {
	typ string
	ref string
}

func (g *Generator) emitExpression(expr ast.Expression) (value, error) {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		return value{typ: "i32", ref: expr.Token.Lexeme}, nil
	case *ast.BooleanLiteral:
		if expr.Value {
			return value{typ: "i1", ref: "true"}, nil
		}
		return value{typ: "i1", ref: "false"}, nil
	case *ast.StringLiteral:
		return g.emitStringLiteral(expr)
	case *ast.InfixExpression:
		return g.emitInfixExpression(expr)
	default:
		return value{}, fmt.Errorf("emit-llvm does not support expression %T yet", expr)
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

func (g *Generator) emitCompare(predicate string, left value, right value) value {
	temp := g.nextTemp()
	g.write("  %s = icmp %s %s %s, %s\n", temp, predicate, left.typ, left.ref, right.ref)
	return value{typ: "i1", ref: temp}
}

func (g *Generator) emitStringLiteral(expr *ast.StringLiteral) (value, error) {
	name := fmt.Sprintf("@.str.%d", g.stringID)
	g.stringID++

	bytes := append([]byte(expr.Value), 0)
	g.globals.WriteString(fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\"\n", name, len(bytes), llvmCString(bytes)))

	temp := g.nextTemp()
	g.write("  %s = getelementptr inbounds [%d x i8], ptr %s, i64 0, i64 0\n", temp, len(bytes), name)
	return value{typ: "ptr", ref: temp}, nil
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

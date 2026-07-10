package llvm

import (
	"fmt"

	"sec/internal/ast"
)

func (g *Generator) emitBlock(block *ast.BlockStatement) error {
	if block == nil {
		return nil
	}

	for _, stmt := range block.Statements {
		if !g.blockOpen {
			return nil
		}
		if err := g.emitStatement(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) emitStatement(stmt ast.Statement) error {
	switch stmt := stmt.(type) {
	case *ast.ReturnStatement:
		return g.emitReturn(stmt)
	case *ast.IfStatement:
		return g.emitIf(stmt)
	case *ast.ExpressionStatement:
		_, err := g.emitExpressionStatement(stmt.Expression)
		return err
	default:
		return fmt.Errorf("emit-llvm does not support %T yet", stmt)
	}
}

func (g *Generator) emitExpressionStatement(expr ast.Expression) (value, error) {
	switch expr := expr.(type) {
	case *ast.CallExpression:
		return g.emitCallExpression(expr)
	case *ast.RuntimeCallExpression:
		return g.emitRuntimeCallExpression(expr)
	default:
		return g.emitExpression(expr)
	}
}

func (g *Generator) emitCallExpression(expr *ast.CallExpression) (value, error) {
	switch callExpressionName(expr) {
	case "fmt.Println":
		if len(expr.Arguments) != 1 {
			return value{}, fmt.Errorf("fmt.Println expects 1 argument")
		}
		arg, err := g.emitExpression(expr.Arguments[0])
		if err != nil {
			return value{}, err
		}
		if arg.typ != "ptr" {
			return value{}, fmt.Errorf("fmt.Println currently expects string")
		}
		g.needsPuts = true
		g.write("  call i32 @puts(ptr %s)\n", arg.ref)
		return value{typ: "void"}, nil
	default:
		return value{}, fmt.Errorf("emit-llvm does not support call %s yet", callExpressionName(expr))
	}
}

func (g *Generator) emitRuntimeCallExpression(expr *ast.RuntimeCallExpression) (value, error) {
	switch expr.Name {
	case "runtime.PrintlnString":
		if len(expr.Arguments) != 1 {
			return value{}, fmt.Errorf("@runtime.PrintlnString expects 1 argument")
		}
		arg, err := g.emitExpression(expr.Arguments[0])
		if err != nil {
			return value{}, err
		}
		if arg.typ != "ptr" {
			return value{}, fmt.Errorf("@runtime.PrintlnString currently expects string")
		}
		g.needsPuts = true
		g.write("  call i32 @puts(ptr %s)\n", arg.ref)
		return value{typ: "void"}, nil
	default:
		return value{}, fmt.Errorf("emit-llvm does not support @%s yet", expr.Name)
	}
}

func (g *Generator) emitReturn(stmt *ast.ReturnStatement) error {
	if stmt.Value == nil {
		g.write("  ret void\n")
		g.blockOpen = false
		return nil
	}

	value, err := g.emitExpression(stmt.Value)
	if err != nil {
		return err
	}
	g.write("  ret %s %s\n", value.typ, value.ref)
	g.blockOpen = false
	return nil
}

func (g *Generator) emitIf(stmt *ast.IfStatement) error {
	if stmt.Condition == nil || stmt.Consequence == nil {
		return fmt.Errorf("emit-llvm requires complete if statements")
	}

	condition, err := g.emitExpression(stmt.Condition)
	if err != nil {
		return err
	}
	if condition.typ != "i1" {
		return fmt.Errorf("emit-llvm if condition must be bool")
	}

	thenLabel := g.nextLabel("if.then")
	endLabel := g.nextLabel("if.end")

	g.write("  br i1 %s, label %%%s, label %%%s\n\n", condition.ref, thenLabel, endLabel)

	g.write("%s:\n", thenLabel)
	g.blockOpen = true
	if err := g.emitBlock(stmt.Consequence); err != nil {
		return err
	}
	if g.blockOpen {
		g.write("  br label %%%s\n", endLabel)
	}

	g.write("\n%s:\n", endLabel)
	g.blockOpen = true
	return nil
}

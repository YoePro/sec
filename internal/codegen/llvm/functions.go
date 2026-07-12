package llvm

import (
	"fmt"

	"sec/internal/ast"
)

func (g *Generator) emitFunction(fn *ast.FunctionDeclaration) error {
	returnType := g.llvmType(fn.ReturnType)
	previousReturnType := g.returnType
	previousLoops := g.loops
	g.locals = map[string]local{}
	g.loops = nil
	g.returnType = returnType
	defer func() {
		g.returnType = previousReturnType
		g.loops = previousLoops
	}()

	g.write("define %s @%s(", returnType, fn.Name.Value)
	for i, param := range fn.Parameters {
		if i > 0 {
			g.write(", ")
		}
		if param.Type != nil && param.Type.Name == "string" {
			g.write("ptr %%%s.ptr, i64 %%%s.len", param.Name.Value, param.Name.Value)
			g.locals[param.Name.Value] = local{typ: "string", ref: "%" + param.Name.Value + ".ptr", lenRef: "%" + param.Name.Value + ".len", direct: true}
			continue
		}
		paramType := g.llvmParameterType(param)
		g.write("%s %%%s", paramType, param.Name.Value)
		g.locals[param.Name.Value] = local{typ: paramType, ref: "%" + param.Name.Value, direct: true}
	}
	g.write(") {\n")
	g.write("entry:\n")
	g.blockOpen = true

	if err := g.emitBlock(fn.Body); err != nil {
		return err
	}

	if g.blockOpen {
		switch returnType {
		case "void":
			g.write("  ret void\n")
		default:
			g.write("  ret %s %s\n", returnType, llvmZeroValue(returnType))
		}
		g.blockOpen = false
	}

	g.write("}\n\n")
	return nil
}

func (g *Generator) emitLambdaExpression(expr *ast.LambdaExpression) (value, error) {
	if len(expr.Captures) > 0 {
		return value{}, fmt.Errorf("emit-llvm does not support capturing lambdas yet")
	}
	if expr.ReturnType == nil || expr.Body == nil {
		return value{}, fmt.Errorf("emit-llvm requires complete lambda expressions")
	}

	name := fmt.Sprintf("__sec_lambda_%d", g.lambdaID)
	g.lambdaID++

	fnType := lambdaFunctionType(expr)
	previousOut := g.activeOut
	previousLocals := g.locals
	previousLoops := g.loops
	previousReturnType := g.returnType
	previousBlockOpen := g.blockOpen

	g.activeOut = &g.lambdaDefs
	g.locals = map[string]local{}
	g.loops = nil
	g.returnType = g.llvmType(expr.ReturnType)
	g.blockOpen = true
	defer func() {
		g.activeOut = previousOut
		g.locals = previousLocals
		g.loops = previousLoops
		g.returnType = previousReturnType
		g.blockOpen = previousBlockOpen
	}()

	g.write("define %s @%s(", g.returnType, name)
	for i, param := range expr.Parameters {
		if i > 0 {
			g.write(", ")
		}
		if param.Type != nil && param.Type.Name == "string" {
			g.write("ptr %%%s.ptr, i64 %%%s.len", param.Name.Value, param.Name.Value)
			g.locals[param.Name.Value] = local{typ: "string", ref: "%" + param.Name.Value + ".ptr", lenRef: "%" + param.Name.Value + ".len", direct: true}
			continue
		}
		paramType := g.llvmType(param.Type)
		g.write("%s %%%s", paramType, param.Name.Value)
		g.locals[param.Name.Value] = local{typ: paramType, ref: "%" + param.Name.Value, direct: true}
	}
	g.write(") {\n")
	g.write("entry:\n")

	if err := g.emitBlock(expr.Body); err != nil {
		return value{}, err
	}

	if g.blockOpen {
		switch g.returnType {
		case "void":
			g.write("  ret void\n")
		default:
			g.write("  ret %s %s\n", g.returnType, llvmZeroValue(g.returnType))
		}
		g.blockOpen = false
	}
	g.write("}\n\n")

	return value{typ: "ptr", ref: "@" + name, fnType: fnType}, nil
}

func lambdaFunctionType(expr *ast.LambdaExpression) *ast.TypeReference {
	ref := &ast.TypeReference{
		Token:                  expr.Token,
		Name:                   "fn",
		FunctionReturnType:     expr.ReturnType,
		FunctionParameterTypes: make([]*ast.TypeReference, 0, len(expr.Parameters)),
	}
	for _, param := range expr.Parameters {
		ref.FunctionParameterTypes = append(ref.FunctionParameterTypes, param.Type)
	}
	return ref
}

func functionDeclarationType(fn *ast.FunctionDeclaration) *ast.TypeReference {
	ref := &ast.TypeReference{
		Token:                  fn.Token,
		Name:                   "fn",
		FunctionReturnType:     fn.ReturnType,
		FunctionParameterTypes: make([]*ast.TypeReference, 0, len(fn.Parameters)),
	}
	for _, param := range fn.Parameters {
		ref.FunctionParameterTypes = append(ref.FunctionParameterTypes, param.Type)
	}
	return ref
}

package llvm

import "sec/internal/ast"

func (g *Generator) emitFunction(fn *ast.FunctionDeclaration) error {
	returnType := llvmReturnType(fn.ReturnType)
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
		paramType := llvmParameterType(param)
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
			g.write("  ret %s 0\n", returnType)
		}
		g.blockOpen = false
	}

	g.write("}\n\n")
	return nil
}

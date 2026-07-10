package llvm

import (
	"fmt"

	"sec/internal/ast"
)

func (g *Generator) emitFunction(fn *ast.FunctionDeclaration) error {
	if len(fn.Parameters) != 0 {
		return fmt.Errorf("emit-llvm only supports parameterless functions for now: %s", fn.Name.Value)
	}

	returnType := llvmReturnType(fn.ReturnType)
	g.write("define %s @%s() {\n", returnType, fn.Name.Value)
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

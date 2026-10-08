package mlir

import (
	"sec/internal/ast"
	"sec/internal/codegen/readiness"
	"sec/internal/codegen/targetplan"
	"sec/internal/ir/semantic"
	"sec/internal/sema"
)

// GenerateWithTriple emits legacy LLVM-dialect MLIR using canonical scalar facts
// resolved from the exact compiler-owned target identity.
// Rules: rules/types/types.md — "int and uint"; correction5.md — backend widths.
func GenerateWithTriple(program *ast.Program, triple string) (string, error) {
	g := &Generator{targetTriple: triple}
	return g.Generate(program)
}

// GenerateAnalyzed checks Sema/target scalar agreement before legacy MLIR lowering;
// callers cannot analyze native bounds for one target and emit another width.
// Native float uses on 32-bit targets are explicitly unsupported by this backend.
// Rules: rules/types/types.md — "int and uint"; rules/compiler/compiler_pipeline.md — §61(3).
func GenerateAnalyzed(program *ast.Program, analyzer *sema.Analyzer, triple string) (string, error) {
	if analyzer == nil || program == nil {
		return "", &semantic.UnsupportedFeatureError{Feature: "missing analyzed native integer facts"}
	}
	if err := readiness.CheckWideProgram(program, analyzer, "MLIR"); err != nil {
		return "", err
	}
	plan, err := targetplan.Plan(triple)
	if err != nil {
		return "", &semantic.UnsupportedFeatureError{Feature: err.Error()}
	}
	types := analyzer.Types()
	for _, name := range []string{"int", "uint"} {
		typ := types[name]
		width := 0
		if typ.MaxInteger != nil {
			width = typ.MaxInteger.BitLen()
			if name == "int" {
				width++
			}
		}
		if width != int(plan.PointerWidthBits) {
			return "", &semantic.UnsupportedFeatureError{Feature: "analyzed " + name + " width does not match selected target scalar plan"}
		}
	}
	return GenerateWithTriple(program, triple)
}

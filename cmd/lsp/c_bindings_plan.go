package main

import (
	"os"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/cbindingplan"
	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
)

// applyLSPCBindingPlan gives the editor the CLI's C binding environment:
// the Variant matching the analysis plan's target is resolved with the
// shared cbindingplan resolver, its module sources join the program, and
// Sema receives the same frozen surface. A resolution failure leaves the
// surface empty, so binding names report S1078 exactly as without one; the
// CLI reports the failure itself.
// Rules: rules/tooling/lsp.md — "Target-aware analysis"; rules/platform/ffi.md — § 56;
// MD-021 correction §§ 8.6(5), 8.7.
func applyLSPCBindingPlan(analyzer *sema.Analyzer, program *ast.Program, sourcePath string) {
	if program == nil || sourcePath == "" {
		return
	}
	targetOS, targetArch := analyzer.PlanTarget()
	if targetOS == "" || targetArch == "" {
		return
	}
	plan, _ := cbindingplan.Resolve(findProjectRoot(sourcePath), "", platformtarget.Target{OS: targetOS, Arch: targetArch})
	if plan == nil {
		return
	}
	surface := sema.CBindingSurface{Identity: plan.Identity(), Files: map[string]sema.CBindingLayer{}}
	for _, file := range plan.Files {
		layer := sema.CBindingEnvironment
		if file.Layer == cbindingplan.Common {
			layer = sema.CBindingCommon
		}
		surface.Files[file.Path] = layer
	}
	var statements []ast.Statement
	modules := map[string]string{}
	for _, file := range plan.Files {
		data, err := os.ReadFile(file.Path)
		if err != nil {
			return
		}
		p := parser.New(lexer.NewWithFile(string(data), file.Path))
		modules[file.Path] = cbindingplan.ModuleName(p.Parse().Program)
	}
	// A module-identity collision leaves the surface empty, as the CLI rejects it.
	if len(cbindingplan.ModuleIdentityProblems(plan, modules, program)) != 0 {
		return
	}
	for _, file := range cbindingplan.MissingFiles(program, plan) {
		data, err := os.ReadFile(file.Path)
		if err != nil {
			return
		}
		p := parser.New(lexer.NewWithFile(string(data), file.Path))
		result := p.Parse()
		if result.Program == nil || len(p.Errors()) != 0 {
			return
		}
		statements = append(statements, result.Program.Statements...)
	}
	program.Statements = append(program.Statements, statements...)
	analyzer.SetCBindingSurface(surface)
}

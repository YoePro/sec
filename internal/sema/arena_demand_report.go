package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// ArenaDemandReport is the byte demand of every Arena domain one callable
// creates, as Sema computed it for the Semantic IR demand summary: the same
// constant, unknown or unbounded upper bound and symbolic derivation, with
// the fixed backing capacity when known. Tooling presents it; it never
// re-derives demand or turns an unknown bound into a number.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79-81, 130;
// rules/memory/arena.md — §§ 114-115.
type ArenaDemandReport struct {
	Callable  CallableID
	Name      string
	Source    lexer.Token
	Summaries []ArenaDemandSummary
}

// ArenaDemandReports lists the per-callable demand reports in source order.
// Rules: rules/memory/arena.md — § 114; rules/compiler/compiler_analysis.md — § 61.
func (a *Analyzer) ArenaDemandReports() []ArenaDemandReport {
	if a == nil {
		return nil
	}
	return append([]ArenaDemandReport(nil), a.arenaDemandReports...)
}

// collectArenaDemandReports publishes, after every callable body is analyzed,
// the demand summaries of each free function and method that creates an
// Arena domain.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79-81, 130.
func (a *Analyzer) collectArenaDemandReports(program *ast.Program) {
	a.arenaDemandReports = nil
	visit := func(fn *ast.FunctionDeclaration, name string) {
		if fn == nil || fn.Body == nil {
			return
		}
		summaries := a.ArenaDemandSummariesOf(fn.Body)
		if len(summaries) == 0 {
			return
		}
		function, ok := a.lookupFunctionByToken(name, fn.Name.Token)
		if !ok {
			return
		}
		a.arenaDemandReports = append(a.arenaDemandReports, ArenaDemandReport{Callable: callableID(function), Name: name, Source: fn.Name.Token, Summaries: summaries})
	}
	a.withProgramModules(program, func(statement ast.Statement) {
		switch statement := statement.(type) {
		case *ast.FunctionDeclaration:
			visit(statement, statement.Name.Value)
		case *ast.ImplStatement:
			if statement.Target == nil || !a.validImplStatements[statement] {
				return
			}
			for _, member := range statement.Members {
				if fn, ok := member.(*ast.FunctionDeclaration); ok {
					visit(fn, statement.Target.Name+"."+fn.Name.Value)
				}
			}
		}
	})
}

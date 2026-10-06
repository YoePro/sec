package sema

import "sec/internal/ast"

// runSemanticAnalysisPipeline makes producer/consumer dependencies explicit
// after foundational declaration/conformance resolution. Body analysis owns
// local ownership, borrowing, escape, capture and call facts; existing summary
// solvers retain their domains and widening rules inside coordinated passes.
// Rules: rules/compiler/compiler_analysis.md — §§9–10, §14;
// rules/analysis/parameter_usage_analysis.md — "Calls propagate demand";
// rules/analysis/pitfall_analysis.md — "Semantic recognition, not syntax matching".
func (a *Analyzer) runSemanticAnalysisPipeline(program *ast.Program) {
	once := func(run func()) func() analysisStepResult {
		return func() analysisStepResult {
			run()
			return analysisStepResult{Iterations: 1, Converged: true}
		}
	}
	passes := []analysisPass{
		{id: "reference-summaries", dependencies: []string{"declaration-facts"}, priority: 10,
			run: func() analysisStepResult { return a.inferFunctionReferenceSummaries(program) }},
		{id: "module-bodies", dependencies: []string{"reference-summaries"}, priority: 20,
			run: once(func() { a.analyzeModuleBodies(program) })},
		{id: "callable-bodies", dependencies: []string{"module-bodies", "reference-summaries"}, priority: 30,
			run: once(func() {
				a.analyzeFunctionBodies(program)
				a.analyzeImplBodies(program)
				a.analyzeTestBodies(program)
			})},
		{id: "string-materializations", dependencies: []string{"callable-bodies"}, priority: 40,
			run: once(a.reportStringMaterializations)},
		{id: "no-panic", dependencies: []string{"callable-bodies", "string-materializations"}, priority: 50,
			run: once(func() { a.validateNoPanicGuarantees(program) })},
		{id: "no-alloc", dependencies: []string{"callable-bodies", "string-materializations"}, priority: 60,
			run: once(func() { a.validateNoAllocGuarantees(program) })},
		{id: "no-block", dependencies: []string{"callable-bodies", "string-materializations"}, priority: 70,
			run: once(func() { a.validateNoBlockGuarantees(program) })},
		{id: "parameter-demand", dependencies: []string{"callable-bodies", "no-panic", "no-alloc", "no-block"}, priority: 80,
			run: func() analysisStepResult {
				a.parameterUsageAnalysis = buildParameterUsageAnalysis(program, a)
				iterations, converged := a.parameterUsageAnalysis.InterproceduralStatus()
				return analysisStepResult{Iterations: iterations, Converged: converged, Widened: !converged}
			}},
		{id: "parameter-advice", dependencies: []string{"parameter-demand"}, priority: 90,
			run: once(a.emitLargeValueParameterAdvisories)},
		{id: "pitfalls", dependencies: []string{"callable-bodies", "no-panic", "no-alloc", "no-block", "parameter-demand", "parameter-advice"}, priority: 100,
			run: once(func() { a.pitfallAnalysis = buildPitfallAnalysis(program, a) })},
	}
	records, err := runAnalysisSchedule(passes, []string{"declaration-facts"})
	if err != nil {
		// This graph is compiler configuration, not user source. An invalid
		// graph must never silently produce a partially analyzed program.
		panic(err)
	}
	a.analysisSchedule = records
}

// analyzeModuleBodies resets transient reference-inference facts before the
// final semantic traversal and checks allowed module-level statements.
// Rules: rules/compiler/compiler_analysis.md — §10(2,5);
// rules/projects/modules.md — "Module declaration".
func (a *Analyzer) analyzeModuleBodies(program *ast.Program) {
	a.expressionTypes = map[ast.Expression]Type{}
	a.expressionReferenceOrigins = map[ast.Expression]localReferenceOrigin{}
	a.withProgramModules(program, func(stmt ast.Statement) {
		switch stmt.(type) {
		case *ast.TargetDirective, *ast.TypeDeclStatement, *ast.UnitDeclStatement, *ast.EnumDeclaration, *ast.InterfaceDeclaration, *ast.ImplStatement, *ast.FunctionDeclaration, *ast.TestDeclaration:
			return
		}
		if !isAllowedModuleStatement(stmt) {
			a.addTopLevelStatementError(stmt)
			return
		}
		a.analyzeStatement(stmt)
	})
}

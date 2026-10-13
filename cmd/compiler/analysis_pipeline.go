package main

import (
	"os"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/sema"
)

// analyzeProgram validates project and target inputs, assembles modules and runs shared Sema.
// Rules: rules/projects/projects.md — §§8–11,48(1–3);
// rules/compiler/compiler_pipeline.md — §26 semantic validation boundary;
// rules/compiler/compiler_analysis.md — §62 analysis-depth modes.
func analyzeProgram(program *ast.Program, target CompilerTarget) {
	analyzeProgramWithSources(program, target, nil, diagnosticSummary{})
}

// analyzeProgramWithSources validates project and target inputs, assembles modules and runs shared Sema.
// Rules: rules/projects/projects.md — §§8–11,48(1–3);
// rules/compiler/compiler_pipeline.md — §26 semantic validation boundary;
// rules/compiler/compiler_analysis.md — §62 analysis-depth modes.
func analyzeProgramWithSources(program *ast.Program, target CompilerTarget, sourceFiles []string, summary diagnosticSummary) {
	_ = analyzeProgramWithSourcesRetained(program, target, sourceFiles, summary, true)
}

// analyzeProgramWithSourcesRetained validates project and target inputs, assembles modules and runs shared Sema.
// Rules: rules/projects/projects.md — §§8–11,48(1–3);
// rules/compiler/compiler_pipeline.md — §26 semantic validation boundary;
// rules/compiler/compiler_analysis.md — §62 analysis-depth modes.
func analyzeProgramWithSourcesRetained(program *ast.Program, target CompilerTarget, sourceFiles []string, summary diagnosticSummary, printSuccessSummary bool) *sema.Analyzer {
	return analyzeProgramWithSourcesAtDepth(program, target, sourceFiles, summary, printSuccessSummary, sema.AnalysisStandard)
}

// analyzeProgramWithSourcesAtDepth validates project and target inputs, assembles modules and runs shared Sema.
// Rules: rules/projects/projects.md — §§8–11,48(1–3);
// rules/compiler/compiler_pipeline.md — §26 semantic validation boundary;
// rules/compiler/compiler_analysis.md — §62 analysis-depth modes.
func analyzeProgramWithSourcesAtDepth(program *ast.Program, target CompilerTarget, sourceFiles []string, summary diagnosticSummary, printSuccessSummary bool, depth sema.AnalysisDepth) *sema.Analyzer {
	versions := validateDiscoveredProjectVersions(sourceFiles)
	if versions.Errors > 0 {
		summary.Errors += versions.Errors
		printDiagnosticSummary(summary)
		exitCLI(3)
	}
	siblings := assembleCLIModuleSources(program, target)
	summary.Errors += siblings.Errors
	summary.Warnings += siblings.Warnings
	if siblings.Errors > 0 {
		printDiagnosticSummary(summary)
		exitCLI(2)
	}
	if err := validateProgramTarget(program, target); err != nil {
		reportToolError("target", "%s", err)
		summary.Errors++
		printDiagnosticSummary(summary)
		exitCLI(1)
	}

	resolveCoreLibraryWithSources(program, sourceFiles)
	resolveStdlibImportsWithSources(program, target, sourceFiles)

	targetDefinition, ok := findTargetDefinition(target)
	if !ok {
		reportToolError("target", "unsupported target %s", target.String())
		summary.Errors++
		printDiagnosticSummary(summary)
		exitCLI(1)
	}
	scalarPlan, err := targetDefinition.scalarPlan()
	if err != nil {
		reportToolError("target", "%s", err)
		summary.Errors++
		printDiagnosticSummary(summary)
		exitCLI(1)
	}
	analyzer := sema.NewAnalyzerWithScalarPlanAndDepth(scalarPlan, depth)
	errors := analyzer.Analyze(program)
	printSemaWarnings(analyzer)
	for _, warning := range analyzer.Warnings() {
		if warning.Severity == diagnostics.SeverityInformation {
			summary.Information++
		} else {
			summary.Warnings++
		}
	}
	if len(errors) > 0 {
		for _, err := range errors {
			printSemaError(os.Stderr, err)
		}
		summary.Errors += len(errors)
		printDiagnosticSummary(summary)
		exitCLI(3)
	}
	if printSuccessSummary {
		printDiagnosticSummary(summary)
	}
	return analyzer
}

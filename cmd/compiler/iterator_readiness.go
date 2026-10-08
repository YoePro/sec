package main

import (
	"os"
	"sec/internal/sema"
)

// validateIteratorReadiness applies the same canonical prerequisite to legacy
// AST backends as Semantic IR. Analysis-only requests remain independent.
// Rules: rules/compiler/compiler_pipeline.md — §§23(3–4), 33(1–3), 34(2–3).
func validateIteratorReadiness(analyzed analyzedProgram, inputFile string) error {
	if analyzed.Program == nil || analyzed.Analyzer == nil {
		return analyzed.Analyzer.ValidateIteratorLoweringReadiness(analyzed.Program, "")
	}
	module, _ := entryModuleOf(analyzed.Program, inputFile)
	if module == "" {
		return &sema.IteratorLoweringReadinessError{Issues: []sema.IteratorLoweringIssue{{Classification: "Unproven", Reason: "requested input module is absent from the analyzed snapshot"}}}
	}
	return analyzed.Analyzer.ValidateIteratorLoweringReadiness(analyzed.Program, module)
}

// requireIteratorReadiness stops an output request before any backend runs;
// valid but unimplemented iteration remains a later capability diagnostic.
// Rules: rules/compiler/compiler_pipeline.md — §§32(2–5), 33(1,3).
func requireIteratorReadiness(analyzed analyzedProgram, inputFile string) {

	if err := validateIteratorReadiness(analyzed, inputFile); err != nil {
		reportPipelineError("lowering-readiness", err)
		exitCLI(4)
	}
}

// parseAndAnalyzeFileForLowering retains final analysis through the readiness
// boundary instead of passing a bare AST directly to a legacy backend.
// Rules: rules/compiler/compiler_pipeline.md — §§33(1–3), 34(2–3).
func parseAndAnalyzeFileForLowering(path string, target CompilerTarget) analyzedProgram {
	input, err := os.ReadFile(path)
	if err != nil {
		reportToolError("read", "%v", err)
		exitCLI(1)
	}
	analyzed := parseAndAnalyzeSourceForTargetWithAnalyzerMode(string(input), path, target, false)
	requireOutputLoweringReadiness(analyzed, path)
	return analyzed
}

package main

import "sec/internal/ir/semantic"

// requireOutputLoweringReadiness enforces owner-produced iterator prerequisites
// and rejects floating conversion plans that output backends cannot preserve.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/corrections/applied/semantic-ir-units-correction-20260818.md — Erasure boundary.
func requireOutputLoweringReadiness(analyzed analyzedProgram, inputFile string) {
	requireIteratorReadiness(analyzed, inputFile)
	// The legacy AST backends do not consume fixed floating conversion plans.
	// Reject before publishing output instead of dropping the unit operation.
	// Rules: semantic-ir-units-correction-20260818.md — Erasure boundary.
	if sites := analyzed.Analyzer.FloatingUnitConversionSites(); len(sites) > 0 {
		token := sites[0]
		reportPipelineError("lowering-readiness", &semantic.UnsupportedFeatureError{Feature: "exact floating unit conversion", Package: 14, Location: semantic.Location{File: token.File, Line: token.Line, Column: token.Column}})
		exitCLI(4)
	}

}

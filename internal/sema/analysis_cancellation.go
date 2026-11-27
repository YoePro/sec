package sema

// analysisCancellation unwinds abandoned compiler work through existing scope
// restoration defers. Only AnalyzeContext catches this private control value;
// it is never a Sec panic, source diagnostic or conservative semantic fact.
// Rules: rules/tooling/lsp.md §§6.7(2–3),7;
// rules/compiler/compile_time_evaluation.md §§53(6),57(1–3).
type analysisCancellation struct{ err error }

// checkAnalysisCancellation is a cooperative boundary in declaration walks,
// statement/expression recursion and between semantic producers. The guard is
// enabled only by AnalyzeContext; optional CTE query APIs retain their own
// explicit context-error transport. Cancelled analyzers must be discarded.
// Rules: rules/tooling/lsp.md §§6.7(2–3),7, Appendix A.11(2);
// rules/compiler/compiler_analysis.md §§9–10,14.
func (a *Analyzer) checkAnalysisCancellation() {
	if a.analysisCancellable {
		if err := a.compileTimeCancellation(); err != nil {
			panic(analysisCancellation{err})
		}
	}
}

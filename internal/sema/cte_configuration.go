package sema

// recordConfiguration retains a semantic configuration step so that the
// compile-time evaluation probe (cte_requests.go) can rebuild an analyzer with
// the same target plan, profile and module context. Only configuration that
// changes typed Sec semantics is recorded; tooling budgets and diagnostic
// presentation policies do not affect evaluation.
// Rules: rules/compiler/compile_time_evaluation.md — § 50(5) (the request's CompilationPlan).
func (a *Analyzer) recordConfiguration(apply func(*Analyzer)) {
	if a == nil || a.replayingConfiguration {
		return
	}
	a.configurationReplay = append(a.configurationReplay, apply)
}

// freshConfiguredAnalyzer builds a new analyzer from the same constructor and
// recorded configuration, with no analysis state.
func (a *Analyzer) freshConfiguredAnalyzer() *Analyzer {
	var fresh *Analyzer
	if a.rebuildBase != nil {
		fresh = a.rebuildBase()
	} else {
		fresh = NewAnalyzerWithDepth(a.analysisDepth)
	}
	fresh.replayingConfiguration = true
	for _, apply := range a.configurationReplay {
		apply(fresh)
	}
	fresh.replayingConfiguration = false
	fresh.configurationReplay = append(fresh.configurationReplay, a.configurationReplay...)
	return fresh
}

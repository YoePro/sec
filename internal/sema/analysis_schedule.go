package sema

import (
	"fmt"
	"sort"
	"strings"
)

// AnalysisPassRecord describes execution, not proof of source validity. A
// converged solver can still contain Unknown facts. Widened means that the
// producer published conservative facts after exhausting refinement.
type AnalysisPassRecord struct {
	ID           string
	Dependencies []string
	Iterations   int
	Converged    bool
	Widened      bool
}

type analysisStepResult struct {
	Iterations int
	Converged  bool
	Widened    bool
}

type analysisPass struct {
	id           string
	dependencies []string
	// Priority preserves a deliberate diagnostic presentation order among
	// independent ready passes; identity breaks ties, never registration order.
	priority int
	run      func() analysisStepResult
}

// AnalysisSchedule returns detached records for this completed analysis. It
// exposes dependency and convergence provenance without re-running analyses.
// Rules: rules/compiler/compiler_analysis.md — §§9–10, §14.
func (a *Analyzer) AnalysisSchedule() []AnalysisPassRecord {
	result := append([]AnalysisPassRecord(nil), a.analysisSchedule...)
	for index := range result {
		result[index].Dependencies = append([]string(nil), result[index].Dependencies...)
	}
	return result
}

// planAnalysisSchedule validates the entire dependency graph before any pass
// runs and chooses a deterministic topological order. A cyclic solver must be
// represented as one coordinated fixed-point pass, not mutually recursive
// callbacks that might consume intermediate positive facts.
// Rules: rules/compiler/compiler_analysis.md — §9(1–4), §10(4–5), §14(2).
func planAnalysisSchedule(passes []analysisPass, initialFacts []string) ([]analysisPass, error) {
	available := map[string]bool{}
	for _, fact := range initialFacts {
		if fact == "" || available[fact] {
			return nil, fmt.Errorf("invalid or duplicate initial analysis fact %q", fact)
		}
		available[fact] = true
	}
	byID := map[string]analysisPass{}
	for _, pass := range passes {
		if pass.id == "" || pass.run == nil {
			return nil, fmt.Errorf("invalid analysis pass %q", pass.id)
		}
		if _, exists := byID[pass.id]; exists || available[pass.id] {
			return nil, fmt.Errorf("duplicate analysis producer %q", pass.id)
		}
		byID[pass.id] = pass
	}
	ordered := append([]analysisPass(nil), passes...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].priority != ordered[j].priority {
			return ordered[i].priority < ordered[j].priority
		}
		return ordered[i].id < ordered[j].id
	})
	for _, pass := range ordered {
		seen := map[string]bool{}
		dependencies := append([]string(nil), pass.dependencies...)
		sort.Strings(dependencies)
		for _, dependency := range dependencies {
			if seen[dependency] {
				return nil, fmt.Errorf("analysis %q repeats dependency %q", pass.id, dependency)
			}
			seen[dependency] = true
			if _, exists := byID[dependency]; !exists && !available[dependency] {
				return nil, fmt.Errorf("analysis %q requires missing producer %q", pass.id, dependency)
			}
		}
	}
	result := make([]analysisPass, 0, len(passes))
	for len(result) < len(ordered) {
		progress := false
		for _, pass := range ordered {
			if available[pass.id] {
				continue
			}
			ready := true
			for _, dependency := range pass.dependencies {
				ready = ready && available[dependency]
			}
			if !ready {
				continue
			}
			// Reconsider priorities after each producer, including newly ready
			// consumers whose priority precedes another independent producer.
			result = append(result, pass)
			available[pass.id] = true
			progress = true
			break
		}
		if !progress {
			pending := []string{}
			for _, pass := range ordered {
				if !available[pass.id] {
					pending = append(pending, pass.id)
				}
			}
			sort.Strings(pending)
			return nil, fmt.Errorf("analysis dependency cycle requires a coordinated fixed point: %s", strings.Join(pending, ", "))
		}
	}
	return result, nil
}

// runAnalysisSchedule executes only a fully validated plan. Solvers must
// publish either converged or conservatively widened facts before consumers
// may run; stopping refinement alone never authorizes a positive proof.
// Rules: rules/compiler/compiler_analysis.md — §10(5), §14(2–3), §15(3–5).
func runAnalysisSchedule(passes []analysisPass, initialFacts []string) ([]AnalysisPassRecord, error) {
	plan, err := planAnalysisSchedule(passes, initialFacts)
	if err != nil {
		return nil, err
	}
	records := make([]AnalysisPassRecord, 0, len(plan))
	for _, pass := range plan {
		result := pass.run()
		if !result.Converged && !result.Widened {
			return records, fmt.Errorf("analysis %q stopped without converged or conservative facts", pass.id)
		}
		dependencies := append([]string(nil), pass.dependencies...)
		sort.Strings(dependencies)
		records = append(records, AnalysisPassRecord{
			ID: pass.id, Dependencies: dependencies, Iterations: result.Iterations,
			Converged: result.Converged, Widened: result.Widened,
		})
	}
	return records, nil
}

// runAnalysisFixedPoint drives an analysis-owned monotone/convergent domain
// in deterministic rounds. step returns whether facts changed; widen must
// publish conservative facts if the finite domain/budget bound is exhausted.
// Rules: rules/compiler/compiler_analysis.md — §9(4), §14(1–3), §15(3–5).
func runAnalysisFixedPoint(limit int, step func() bool, widen func()) analysisStepResult {
	if limit <= 0 || step == nil || widen == nil {
		panic("invalid analysis fixed-point configuration")
	}
	for iteration := 0; iteration < limit; iteration++ {
		if !step() {
			return analysisStepResult{Iterations: iteration + 1, Converged: true}
		}
	}
	widen()
	return analysisStepResult{Iterations: limit, Widened: true}
}

package sema

import (
	"fmt"
	"path/filepath"
	"reflect"

	"sec/internal/ast"
)

// PitfallCoverage describes deterministic syntax-search admission, not a time
// limit or a semantic proof. SkippedUnits counts whole bodies/declarations.
type PitfallCoverage struct {
	MaxNodes, MaxDepth int
	VisitedNodes       int
	SkippedUnits       int
}

// Coverage returns detached coverage metadata, including budget exhaustion.
// Rule: rules/analysis/pitfall_analysis.md — "Analysis states", "Deep".
func (p *PitfallAnalysis) Coverage() PitfallCoverage {
	if p == nil {
		return PitfallCoverage{}
	}
	return p.coverage
}

// SetPitfallBudget configures optional search before the next Analyze call.
// Zero disables search; negative limits are rejected without changing policy.
// Rules: rules/compiler/compiler_analysis.md — §15(1–5);
// rules/analysis/pitfall_analysis.md — "Interactive, Standard, and Deep analysis".
func (a *Analyzer) SetPitfallBudget(maxNodes, maxDepth int) error {
	if maxNodes < 0 || maxDepth < 0 {
		return fmt.Errorf("pitfall search limits must be nonnegative")
	}
	a.analysisBudget.MaxPitfallNodes = maxNodes
	a.analysisBudget.MaxPitfallDepth = maxDepth
	return nil
}

// SetPitfallSourcePriority gives requested documents first access to the optional
// whole-body budget. It changes scheduling only: mandatory proofs always cover
// all sources, and unvisited optional bodies retain incomplete coverage.
// Rules: rules/analysis/pitfall_analysis.md — "Interactive analysis", "Analysis states";
// rules/tooling/lsp.md — "Responsiveness model".
func (a *Analyzer) SetPitfallSourcePriority(paths []string) {
	a.pitfallPrioritySources = map[string]bool{}
	for _, path := range paths {
		a.pitfallPrioritySources[filepath.Clean(path)] = true
	}
}

// walkBudgetedStatement admits complete units so an omitted guard or intent
// witness cannot turn partial traversal into an unsound finding or fix.
// Rules: rules/analysis/pitfall_analysis.md — "Analysis states", "Guards
// participate in pitfall reasoning", "Interactive, Standard, and Deep analysis".
func (b *pitfallBuilder) walkBudgetedStatement(statement ast.Statement) {
	if parameterUsageNodeIsNil(statement) {
		return
	}
	switch statement := statement.(type) {
	case *ast.FunctionDeclaration:
		b.walkBudgetedBlock(statement.Body)
	case *ast.ImplStatement:
		for _, member := range statement.Members {
			switch member := member.(type) {
			case *ast.FunctionDeclaration:
				b.walkBudgetedBlock(member.Body)
			case *ast.PropertyDeclaration:
				b.walkBudgetedBlock(member.Getter)
				if member.Setter != nil {
					b.walkBudgetedBlock(member.Setter.Body)
				}
			}
		}
	default:
		if b.admitPitfallUnit(statement) {
			b.walkStatement(statement)
		}
	}
}

// walkBudgetedBlock runs ordinary, complete contextual reasoning only after
// the entire body fits. Nested lambdas and blocks belong to the same unit.
// Rule: rules/analysis/pitfall_analysis.md — "Evidence model", "Analysis states".
func (b *pitfallBuilder) walkBudgetedBlock(block *ast.BlockStatement) {
	if block != nil && b.admitPitfallUnit(block) {
		b.walkPitfallBody(block)
	}
}

// admitPitfallUnit bounds the syntax available to optional rule searches. It
// deliberately visits syntax fields only, never semantic types, tokens, source
// text, or compiler facts. Structural traversal includes every AST form and
// auxiliary AST record without maintaining a second language visitor.
// Rules: rules/compiler/compiler_analysis.md — §15(2–5);
// rules/analysis/pitfall_analysis.md — "Analysis states".
func (b *pitfallBuilder) admitPitfallUnit(node ast.Node) bool {
	coverage := &b.result.coverage
	if !pitfallSyntaxFits(reflect.ValueOf(node), 0, coverage) {
		coverage.SkippedUnits++
		return false
	}
	return true
}

// pitfallSyntaxFits is a bounded structural preflight. Depth counts AST
// records; slice, pointer and interface wrappers do not add nesting. Failed
// attempts consume their inspected prefix, keeping total admission work finite.
func pitfallSyntaxFits(value reflect.Value, depth int, coverage *PitfallCoverage) bool {
	if !value.IsValid() {
		return true
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return true
		}
		return pitfallSyntaxFits(value.Elem(), depth, coverage)
	case reflect.Struct:
		if value.Type().PkgPath() != reflect.TypeOf(ast.Program{}).PkgPath() {
			return true
		}
		if coverage.VisitedNodes >= coverage.MaxNodes || depth >= coverage.MaxDepth {
			return false
		}
		coverage.VisitedNodes++
		for index := 0; index < value.NumField(); index++ {
			if !pitfallSyntaxFits(value.Field(index), depth+1, coverage) {
				return false
			}
		}
	case reflect.Slice, reflect.Array:
		for index := 0; index < value.Len(); index++ {
			if !pitfallSyntaxFits(value.Index(index), depth, coverage) {
				return false
			}
		}
	}
	return true
}

// finishBudgetCoverage prevents an incomplete search being reported as proof
// of NoFinding, while keeping supported findings and suppression evidence.
// Rules: rules/analysis/pitfall_analysis.md — "Analysis states";
// rules/compiler/compiler_analysis.md — §14(3), §15(3–5).
func (b *pitfallBuilder) finishBudgetCoverage() {
	if b.result.coverage.SkippedUnits == 0 {
		return
	}
	for _, evaluation := range b.counts {
		if evaluation.State != PitfallStateNotEvaluated {
			evaluation.Incomplete = true
			evaluation.State = PitfallStateNotEvaluated
		}
	}
}

// buildPitfallAnalysis applies depth prerequisites and bounded whole-unit search,
// preserving explicit incomplete coverage in the immutable result.
// Rules: rules/analysis/pitfall_analysis.md — "Analysis states",
// "Interactive, Standard, and Deep analysis"; rules/compiler/compiler_analysis.md — §§14–15.
func buildPitfallAnalysis(program *ast.Program, analyzer *Analyzer) *PitfallAnalysis {
	builder := &pitfallBuilder{
		analyzer:                  analyzer,
		result:                    newPitfallAnalysis(),
		counts:                    map[PitfallRuleID]*PitfallRuleEvaluation{},
		handledBooleanComparisons: map[*ast.InfixExpression]bool{},
		handledIntervalChains:     map[*ast.InfixExpression]bool{},
	}
	for _, rule := range pitfallRuleRegistry {
		evaluation := &PitfallRuleEvaluation{Rule: rule.ID, State: PitfallStateNoFinding}
		if !analysisDepthAtLeast(analyzer.analysisDepth, rule.MinimumDepth) {
			evaluation.State = PitfallStateNotEvaluated
		}
		builder.counts[rule.ID] = evaluation
	}
	builder.result.coverage = PitfallCoverage{MaxNodes: analyzer.analysisBudget.MaxPitfallNodes, MaxDepth: analyzer.analysisBudget.MaxPitfallDepth}
	if program != nil {
		for _, priority := range []bool{true, false} {
			for _, statement := range program.Statements {
				if analyzer.pitfallPrioritySources[filepath.Clean(statementToken(statement).File)] == priority {
					builder.walkBudgetedStatement(statement)
				}
			}
		}
	}
	builder.finishBudgetCoverage()
	builder.finish()
	return builder.result
}

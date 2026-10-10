package sema

import (
	"fmt"
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// IteratorResolution is a detached, source-ordered view of a Sema-owned loop
// plan. A generic template has a constraint, not a concrete dispatch target.
// Rules: rules/compiler/semantic_ir.md — §§64(2–5),109(1–4);
// rules/compiler/compiler_analysis.md — §18(2,6).
type IteratorResolution struct {
	Source, Iterable, Binding, NextDeclaration                   lexer.Token
	SourceType, Conformance, ElementType, NextTarget, NextResult string
	NextCallable                                                 CallableID
	GenericTemplate                                              bool
}

// IteratorResolutions exposes validated iterator resolution from this analysis
// snapshot without rediscovering interface conformance or methods. Invalid
// loops and recovery-only Next requirements are never advertised as resolved.
// Rules: rules/compiler/semantic_ir.md — §§64(2–5),109(1–4);
// rules/compiler/compiler_analysis.md — §18(1–6).
func (a *Analyzer) IteratorResolutions() []IteratorResolution {
	if a == nil || a.iteratorLoweringProgram == nil {
		return nil
	}
	facts := []IteratorResolution{}
	for stmt, requirement := range a.iteratorLoweringRequirements {
		if !requirement.required || requirement.invalid || !requirement.conformanceValid {
			continue
		}
		if stmt.Iterable != requirement.iterable || !sameSourceToken(stmt.Token, requirement.source) {
			continue
		}
		plan, ok := a.resolvedForIterations[stmt]
		if !ok || plan.Kind != ForIterationCompilerKnownIterator {
			continue
		}
		fact := IteratorResolution{Source: requirement.source, Iterable: expressionToken(stmt.Iterable),
			SourceType: typeDisplayName(plan.SourceType), Conformance: typeDisplayName(plan.Conformance),
			ElementType: typeDisplayName(plan.ElementType), NextResult: typeDisplayName(plan.Next.ReturnType),
			GenericTemplate: requirement.genericTemplate}
		if call, ok := stmt.Iterable.(*ast.CallExpression); ok {
			fact.Iterable = callCalleeDefinitionToken(call)
		}
		if len(stmt.Bindings) == 1 {
			fact.Binding = stmt.Bindings[0].Token
		}
		if !fact.GenericTemplate {
			if plan.Next.Token.Line == 0 || plan.NextCallable == "" {
				continue
			}
			fact.NextTarget, fact.NextCallable, fact.NextDeclaration = plan.Next.Name, plan.NextCallable, plan.Next.Token
		}
		facts = append(facts, fact)
	}
	sort.Slice(facts, func(i, j int) bool {
		left, right := facts[i].Source, facts[j].Source
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Column < right.Column
	})
	return facts
}

// Display presents the same resolution in compiler reports and editor hover.
// Rules: rules/compiler/semantic_ir.md — §109(1–3);
// rules/compiler/compiler_analysis.md — §18(6),§61(2).
func (f IteratorResolution) Display() string {
	target := f.NextTarget + "() " + f.NextResult
	if f.GenericTemplate {
		target = "resolved per specialization (generic constraint)"
	}
	return fmt.Sprintf("%s: source %s; yields %s; Next target: %s", f.Conformance, f.SourceType, f.ElementType, target)
}

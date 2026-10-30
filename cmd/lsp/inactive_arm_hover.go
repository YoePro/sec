package main

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// inactiveArmHover returns the reachable-state proof of a proven-inactive
// match arm whose pattern is under the cursor, from the compiler's match
// plan; it never re-derives the proof or rewrites source.
// Rules: rules/tooling/lsp.md — "Error patterns, inactive arms and `Some`";
// rules/control-flow/flowcontrol_match.md — § 25A.
func inactiveArmHover(program *ast.Program, analyzer *sema.Analyzer, token lexer.Token) string {
	if program == nil || analyzer == nil {
		return ""
	}
	for _, node := range astNodesInProgram(program) {
		match, ok := node.(*ast.MatchExpression)
		if !ok {
			continue
		}
		plan, found := analyzer.ResolvedMatchPlanOf(match)
		if !found || plan.ProvenActive == nil {
			continue
		}
		for _, arm := range plan.Arms {
			if !arm.ProvenInactive || arm.SourceIndex < 0 || arm.SourceIndex >= len(match.Arms) {
				continue
			}
			pattern := match.Arms[arm.SourceIndex].Pattern
			if pattern == nil || !sameSourceToken(pattern.Token, token) && !sameSourceToken(pattern.NameToken, token) {
				continue
			}
			return inactiveArmProofText(plan, arm)
		}
	}
	return ""
}

func inactiveArmProofText(plan sema.ResolvedMatchPlan, arm sema.ResolvedMatchArm) string {
	proof := plan.ProvenActive
	subject := lspTypeName(plan.SubjectType)
	basis := fmt.Sprintf("its construction at line %d", proof.Source.Line)
	if proof.Basis == sema.VariantProofStateTest {
		basis = fmt.Sprintf("the `is %s` test at line %d", proof.Variant, proof.Source.Line)
	}
	return fmt.Sprintf("\n\n**Inactive arm.** `%s` is proven to contain `%s.%s` here (from %s), so this `%s.%s` arm cannot be selected. It remains a legal, type-checked arm (A2008).",
		proof.Binding, subject, proof.Variant, basis, subject, arm.UnionVariantName)
}

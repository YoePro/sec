package main

import (
	"fmt"
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// calleeHintToken is the callee name token call-fact hints attach after.
func calleeHintToken(call *ast.CallExpression) lexer.Token {
	if member, ok := call.Callee.(*ast.MemberExpression); ok && member.Property != nil {
		return member.Property.Token
	}
	return leftmostExpressionToken(call.Callee)
}

// callFactHints shows facts of one resolved call that the source does not
// spell, each under its own setting: the selected overload's parameter types
// when the name has several overloads; the runtime contract checks of a
// constrained conversion; `allocates` when Sema proves the callee may
// allocate (unknown allocation behavior gets no claim); and the @target or
// @when selection of the called declaration.
// Rules: rules/tooling/lsp.md — § 17(7b); rules/declarations/functions.md — overloads;
// rules/types/contracts.md — runtime checks; rules/memory/allocation.md — § 24;
// rules/foundations/attributes.md — target selection.
func callFactHints(program *ast.Program, analyzer *sema.Analyzer, text string, call *ast.CallExpression, function sema.Function, settings inlayHintSettings) []inlayHint {
	token := calleeHintToken(call)
	if !validDefinitionTokenForHint(token) {
		return nil
	}
	at := tokenRange(text, token).End
	var hints []inlayHint
	hint := func(label, detail string) {
		hints = append(hints, inlayHint{Position: at, Label: label, Kind: inlayHintKindType, PaddingLeft: true, detail: detail, declaration: function.Token})
	}
	if settings.Overloads && function.Name != "" && overloadCount(analyzer, function) > 1 {
		types := []string{}
		for _, parameter := range function.Parameters {
			if parameter.Name != "self" {
				types = append(types, lspTypeName(parameter.Type))
			}
		}
		hint("overload("+strings.Join(types, ", ")+")", "selected overload of "+function.Name)
	}
	if settings.Allocation && function.Token.Line > 0 {
		facts := map[sema.CallableID]sema.AllocationKnowledge{}
		for _, fact := range analyzer.AllocationFacts() {
			facts[fact.Callable] = fact.Knowledge
		}
		for _, node := range analyzer.CallGraph().NodesForDeclaration(function.Token) {
			if facts[node.ID] == sema.AllocationMayAllocate {
				hint("allocates", function.Name+" may allocate through the active allocation context")
				break
			}
		}
	}
	if settings.Targets {
		if selection := declarationSelection(program, function.Token); selection != "" {
			hint(selection, "variant selected for the active target")
		}
	}
	return hints
}

// contractCheckHint shows the runtime contract checks of a conversion into a
// constrained named type, from Sema's recorded plan; a constant argument
// already proven, an unavailable plan or a plain conversion gets none.
// Rules: rules/tooling/lsp.md — § 17(7b); rules/types/contracts.md — §§ 12-13.
func contractCheckHint(analyzer *sema.Analyzer, text string, call *ast.CallExpression) (inlayHint, bool) {
	plan, ok := analyzer.ResolvedNamedContractConversionOf(call)
	if !ok || plan.Unavailable != "" || len(plan.Predicates) == 0 {
		return inlayHint{}, false
	}
	token := calleeHintToken(call)
	if !validDefinitionTokenForHint(token) {
		return inlayHint{}, false
	}
	checks := make([]string, 0, len(plan.Predicates))
	for _, predicate := range plan.Predicates {
		checks = append(checks, contractPredicateLabel(predicate))
	}
	hint := inlayHint{Position: tokenRange(text, token).End, Label: "checks " + strings.Join(checks, ", "), Kind: inlayHintKindType, PaddingLeft: true}
	hint.detail = "runtime contract check into " + lspTypeName(plan.TargetType)
	return hint, true
}

// overloadCount counts the distinct declarations sharing the callee's name.
func overloadCount(analyzer *sema.Analyzer, function sema.Function) int {
	seen := map[string]bool{}
	for _, candidate := range analyzer.Functions()[function.Name] {
		seen[fmt.Sprintf("%s:%d:%d", candidate.Token.File, candidate.Token.Line, candidate.Token.Column)] = true
	}
	return len(seen)
}

// contractPredicateLabel spells one resolved runtime contract predicate.
func contractPredicateLabel(predicate sema.ResolvedContractPredicate) string {
	switch {
	case predicate.Min != nil || predicate.Max != nil:
		bound := func(value *big.Int) string {
			if value == nil {
				return ""
			}
			return value.String()
		}
		operator := ".."
		if predicate.Exclusive {
			operator = "..<"
		}
		return "range " + bound(predicate.Min) + operator + bound(predicate.Max)
	case len(predicate.Values) > 0:
		values := make([]string, len(predicate.Values))
		for i, value := range predicate.Values {
			values[i] = value.String()
		}
		return "in [" + strings.Join(values, ", ") + "]"
	}
	return strings.ToLower(predicate.Kind)
}

// declarationSelection returns the @target/@when attributes of the source
// declaration at token, which the active plan selected for this call.
func declarationSelection(program *ast.Program, token lexer.Token) string {
	var found string
	walkLSPNodes(program, func(node any) {
		if found != "" {
			return
		}
		fn, ok := node.(*ast.FunctionDeclaration)
		if !ok || fn == nil || fn.Name == nil || fn.Name.Token.Line != token.Line || fn.Name.Token.Column != token.Column || fn.Name.Token.File != token.File {
			return
		}
		var parts []string
		for _, attribute := range fn.Attributes {
			if attribute != nil && attribute.Name != nil && (attribute.Name.Value == "target" || attribute.Name.Value == "when") {
				parts = append(parts, attributeSpelling(attribute))
			}
		}
		found = strings.Join(parts, " ")
	})
	return found
}

// unitConversionHint shows an implicit unit conversion Sema planned on an
// expression: `m→km ×1/1000` before it, with an affine offset when present.
// Expressions without a recorded plan or identity conversions get no hint.
// Rules: rules/tooling/lsp.md — § 17(7b); rules/types/units.md — conversions.
func unitConversionHint(analyzer *sema.Analyzer, text string, expression ast.Expression, inThisFile func(lexer.Token) bool) (inlayHint, bool) {
	plan, ok := analyzer.UnitConversionPlanOf(expression)
	if !ok || plan.Factor == nil || plan.Proof == sema.UnitConversionIdentityCoordinate {
		return inlayHint{}, false
	}
	token := leftmostExpressionToken(expression)
	if !validDefinitionTokenForHint(token) || !inThisFile(token) {
		return inlayHint{}, false
	}
	name := func(unit sema.UnitSemantics) string {
		if unit.Named != "" {
			return unit.Named
		}
		return unit.Source
	}
	label := fmt.Sprintf("%s→%s ×%s", name(plan.Source), name(plan.Target), plan.Factor.RatString())
	if plan.Offset != nil && plan.Offset.Sign() != 0 {
		label += " +" + plan.Offset.RatString()
	}
	hint := inlayHint{Position: tokenRange(text, token).Start, Label: label, Kind: inlayHintKindType, PaddingRight: true}
	hint.detail = "implicit unit conversion (" + string(plan.Proof) + ")"
	return hint, true
}

// attributeSpelling renders `@name(args)` from the parsed attribute.
func attributeSpelling(attribute *ast.Attribute) string {
	args := make([]string, 0, len(attribute.Arguments))
	for _, argument := range attribute.Arguments {
		if argument == nil {
			continue
		}
		value := ""
		if argument.Value != nil {
			value = argument.Value.String()
		}
		if argument.Name != nil {
			value = argument.Name.Value + ": " + value
		}
		args = append(args, value)
	}
	if len(args) == 0 {
		return "@" + attribute.Name.Value
	}
	return "@" + attribute.Name.Value + "(" + strings.Join(args, ", ") + ")"
}

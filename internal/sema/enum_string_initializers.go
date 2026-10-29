package sema

import "sec/internal/ast"

// stringEnumInitializer evaluates a required ordinary string value using the
// shared semantic constant evaluator. It preserves decoded text and rejects
// nominal text, nonstrings and runtime inputs. Calls/getters retain the shared
// unavailable-execution diagnostic rather than inventing an enum-only CTE path.
// Rules: rules/declarations/enums.md — §5 Member initializer syntax;
// rules/compiler/compile_time_evaluation.md — SemanticCompileTimeRequiredContext;
// rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §3.
func (a *Analyzer) stringEnumInitializer(owner, member string, expr ast.Expression) (string, bool) {
	if identifier, ok := expr.(*ast.Identifier); ok && identifier.Value == "iota" {
		a.addErrorAtToken(identifier.Token, "iota is not available in string-backed enum %s", owner)
		return "", false
	}
	value, outcome := a.semanticCompileTimeConstant(expr)
	if outcome == compileTimeRequiresExecution || outcome == compileTimeForbiddenClock {
		a.reportCompileTimeRequirement(expr, outcome, "string-backed enum member "+owner+"."+member+" initializer", "", "Use a compile-time string expression.")
		return "", false
	}
	if outcome != compileTimeEvaluated || value.Kind != StringType || value.NominalText || value.Nominal.Owner != "" {
		if outcome != compileTimeAlreadyInvalid {
			a.addErrorAtToken(expressionToken(expr), "string-backed enum value %s.%s initializer must be a compile-time string constant", owner, member)
		}
		return "", false
	}
	return value.String, true
}

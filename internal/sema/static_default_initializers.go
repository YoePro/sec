package sema

import "sec/internal/ast"

// staticAggregateDefault hands the canonical default to the shared CTE value
// materializer. Synthesized syntax is only a bounded compatibility preview:
// it has no ordinary array-literal plan and may omit all large-array elements.
// Preserve the result as an initializer fact, including for static dependency
// evaluation, without runtime startup or a second default-selection algorithm.
// Rules: rules/types/default_values.md — §§31-32,40-41;
// rules/declarations/static.md — §§15-16;
// rules/compiler/compile_time_evaluation.md — §§30-37,48-49.
func (a *Analyzer) staticAggregateDefault(binding ResolvedStatic) (DefaultConstant, compileTimeOutcome) {
	executor := newCTEExecutor(a, nil)
	value, ok := executor.defaultValue(DefaultValueOf(binding.Type), binding.Type, binding.Declaration.Name.Token)
	result := cteResult{Value: value, Type: binding.Type, Failure: executor.failure}
	if ok {
		result = finishCTEMaterialization(result)
	}
	if result.Failure != nil {
		if a.cteResults == nil {
			a.cteResults = map[ast.Expression]cteResult{}
		}
		a.cteResults[binding.Declaration.Value] = result
		a.reportCompileTimeExecutionFailure(binding.Declaration.Value, "static default for "+binding.Name)
		return DefaultConstant{}, compileTimeAlreadyInvalid
	}
	if !ok {
		return DefaultConstant{}, compileTimeNotConstant
	}
	return result.Value, compileTimeEvaluated
}

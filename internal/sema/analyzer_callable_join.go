package sema

import (
	"reflect"

	"sec/internal/ast"
)

// nestedCallableWrite is one assignment to a function-value binding in a
// scope nested below its declaration; Known is false when the assigned
// value's callable identity could not be resolved.
type nestedCallableWrite struct {
	Identity ResolvedCallableIdentity
	Known    bool
}

// logNestedCallableWrite records an assignment to a function-value binding
// made in a scope nested below the binding's declaration. Such a write may or
// may not execute before a later read once the nested scope is left, so
// reads join it with the binding's current identity.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Callable value flow", "Soundness of target sets", "Closed set"
func (a *Analyzer) logNestedCallableWrite(name string) {
	symbol, ok := a.symbols[name]
	if !ok || symbol.Type.Kind != FunctionType || a.scopeDepth <= symbol.ScopeDepth {
		return
	}
	if a.nestedCallableWrites == nil {
		a.nestedCallableWrites = map[sourceTokenKey][]nestedCallableWrite{}
	}
	key := sourceTokenLocation(symbol.Token)
	a.nestedCallableWrites[key] = append(a.nestedCallableWrites[key], nestedCallableWrite{
		Identity: cloneResolvedCallableIdentity(symbol.CallableIdentity),
		Known:    symbol.HasCallableIdentity,
	})
}

// effectiveCallableIdentity is the callable identity a read of a binding may
// observe: the binding's flow-sensitive identity joined with every identity
// assigned to it in a nested scope. A binding the function assigns inside a
// loop is unknown, because a read early in the loop body may observe a write
// made later in the same body on an earlier iteration. The join is a closed
// target set over environment-free bodies; an unknown write, an open set, or
// differing closure environments make the result unknown, which is never
// positive proof.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Callable target sets", "Closed set", "Soundness of target sets"
func (a *Analyzer) effectiveCallableIdentity(name string, symbol Symbol) (ResolvedCallableIdentity, bool) {
	if !symbol.HasCallableIdentity {
		return ResolvedCallableIdentity{}, false
	}
	identity := cloneResolvedCallableIdentity(symbol.CallableIdentity)
	if symbol.Type.Kind != FunctionType || !symbol.Mutable {
		return identity, true
	}
	if a.loopWrittenNames[name] {
		return ResolvedCallableIdentity{}, false
	}
	for _, write := range a.nestedCallableWrites[sourceTokenLocation(symbol.Token)] {
		if !write.Known {
			return ResolvedCallableIdentity{}, false
		}
		joined, ok := joinCallableIdentities(identity, write.Identity)
		if !ok {
			return ResolvedCallableIdentity{}, false
		}
		identity = joined
	}
	return identity, true
}

// joinCallableIdentities forms the closed target set of two identities. Two
// identical identities stay exact; otherwise both must be closed and free of
// a closure environment, since one binding cannot name two environments.
func joinCallableIdentities(left, right ResolvedCallableIdentity) (ResolvedCallableIdentity, bool) {
	if reflect.DeepEqual(left, right) {
		return left, true
	}
	if left.HasEnvironment || right.HasEnvironment || !left.Targets.IsClosed || !right.Targets.IsClosed ||
		left.Targets.HasOpenContract || right.Targets.HasOpenContract {
		return ResolvedCallableIdentity{}, false
	}
	joined := cloneResolvedCallableIdentity(left)
	seen := map[CallableBodyID]bool{}
	targets := []CallableBodyID{}
	for _, target := range append(append([]CallableBodyID(nil), left.Targets.KnownTargets...), right.Targets.KnownTargets...) {
		if !seen[target] {
			seen[target] = true
			targets = append(targets, target)
		}
	}
	joined.Targets = CallableTargetSet{KnownTargets: targets, IsClosed: true}
	if len(targets) > 1 {
		// A joined set selects no single body or named target.
		joined.Body = ""
		joined.NamedTarget = ""
		joined.Value = ""
	}
	return joined, true
}

// loopAssignedNames returns the bindings a function assigns inside a while
// or for loop body.
func loopAssignedNames(function *ast.FunctionDeclaration) map[string]bool {
	names := map[string]bool{}
	if function == nil || function.Body == nil {
		return names
	}
	collect := func(body *ast.BlockStatement) {
		walkASTValue(reflect.ValueOf(body), func(node any) {
			if assignment, ok := node.(*ast.AssignmentStatement); ok && assignment != nil {
				if root, ok := expressionRootName(assignment.Target); ok {
					names[root] = true
				}
			}
		})
	}
	walkASTValue(reflect.ValueOf(function.Body), func(node any) {
		switch loop := node.(type) {
		case *ast.WhileStatement:
			if loop != nil {
				collect(loop.Body)
			}
		case *ast.ForStatement:
			if loop != nil {
				collect(loop.Body)
			}
		}
	})
	return names
}

// recordFunctionValueCalleeBinding publishes the lexical declaration selected
// by the optimized function-value call lookup. Parameter analysis and tooling
// consume this same identity instead of reconstructing it from a name after
// the callable scope has ended. Type/capability resolution remains unchanged.
// Rules: rules/declarations/lambda-functions.md — §§9,11–12,39 (callable capability, identity and direct-call optimization);
// rules/analysis/parameter_usage_analysis.md — "Inputs from other analyses";
// rules/analysis/closure_analysis.md — "Callable-flow analysis".
func (a *Analyzer) recordFunctionValueCalleeBinding(callee ast.Expression, symbol Symbol) {
	a.expressionTypes[callee] = symbol.Type
	if identifier, ok := callee.(*ast.Identifier); ok {
		a.bindDefinition(identifier.Token, symbol.Token)
	}
}

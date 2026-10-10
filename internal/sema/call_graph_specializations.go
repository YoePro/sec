package sema

import (
	"sec/internal/lexer"
)

// genericConstraintCall is a method call inside a generic template whose
// receiver is a template parameter, resolved only through its constraint.
// The template node records it as an unknown callee; every concrete
// specialization replaces it with a static edge to the concrete method.
type genericConstraintCall struct {
	parameter string
	method    string
	arguments int
	token     lexer.Token
}

// recordGenericConstraintCall retains a constraint-resolved call of the
// generic template currently being analyzed.
func (a *Analyzer) recordGenericConstraintCall(receiver Type, method Function, token lexer.Token) {
	receiver = dereferenceType(receiver)
	if a.summaryPass || receiver.Kind != GenericType || !validDefinitionToken(a.currentFunctionMetadata.Token) {
		return
	}
	key := sourceTokenLocation(a.currentFunctionMetadata.Token)
	for _, existing := range a.genericConstraintCalls[key] {
		if sameSourceToken(existing.token, token) {
			return
		}
	}
	a.genericConstraintCalls[key] = append(a.genericConstraintCalls[key], genericConstraintCall{
		parameter: receiver.Name, method: method.Name, arguments: len(method.Parameters), token: token,
	})
}

// linkGenericSpecializations gives every concrete specialization reached
// from a concrete call an executable call-graph node: the template's own
// effects and call sites, constraint calls replaced by static edges to the
// concrete methods, and forwarded generic calls replaced by edges to their
// concrete inner specializations. Effect, allocation and blocking
// guarantees therefore see what a specialization really executes.
//
// Rules:
//   - rules/compiler/monomorphization.md — §§12.4–12.5 specialization nodes
//   - rules/analysis/call_graph.md — "Callable node", generic specializations
//   - rules/memory/allocation.md — § 24 allocation effects
func (a *Analyzer) linkGenericSpecializations() {
	graph := a.callGraph
	if graph == nil {
		return
	}
	linked := map[CallableID]bool{}
	var link func(specialization Function, template sourceTokenKey, substitution map[string]Type)
	link = func(specialization Function, template sourceTokenKey, substitution map[string]Type) {
		templateFunction, ok := a.genericTemplateAt(template, specialization.Name)
		if !ok {
			return
		}
		specID := graph.addCallable(specialization)
		templateID := callableID(templateFunction)
		if specID == templateID || linked[specID] {
			return
		}
		linked[specID] = true
		constraintTokens := map[sourceTokenKey]bool{}
		for _, call := range a.genericConstraintCalls[template] {
			constraintTokens[sourceTokenLocation(call.token)] = true
		}
		for _, effect := range graph.effects[templateID] {
			if !constraintTokens[sourceTokenLocation(effect.Source)] {
				graph.addEffect(specID, effect)
			}
		}
		for _, effect := range graph.arenaEffects[templateID] {
			if !constraintTokens[sourceTokenLocation(effect.Source)] {
				graph.addArenaEffect(specID, effect)
			}
		}
		for _, effect := range graph.blockEffects[templateID] {
			if !constraintTokens[sourceTokenLocation(effect.Source)] {
				graph.addBlockEffect(specID, effect)
			}
		}
		for _, site := range append([]CallSite(nil), graph.sites...) {
			if site.Caller != templateID || constraintTokens[sourceTokenLocation(site.Source)] {
				continue
			}
			if inner, forwarded := a.genericInstantiations[sourceTokenLocation(site.Source)]; forwarded && inner.forwarded && inner.caller == template {
				composed := make(map[string]Type, len(inner.substitution))
				for name, typ := range inner.substitution {
					composed[name] = substituteGenericType(typ, substitution)
				}
				innerTemplate, found := a.genericTemplateAt(inner.template, inner.function.Name)
				if !found {
					continue
				}
				innerSpecialization := a.instantiateGenericFunction(innerTemplate, composed)
				graph.addCall(specID, innerSpecialization, site.Source, site.Dispatch, site.Execution)
				link(innerSpecialization, inner.template, composed)
				continue
			}
			graph.addSpecializedSite(specID, site)
		}
		for _, call := range a.genericConstraintCalls[template] {
			concrete := dereferenceType(substituteGenericType(Type{Name: call.parameter, Kind: GenericType}, substitution))
			if concrete.Kind == GenericType || concrete.Name == "" {
				continue
			}
			if method, found := a.concreteConstraintMethod(concrete, call); found {
				graph.addCall(specID, method, call.token, CallDispatchStaticMethod, CallExecutionSynchronous)
			} else {
				// No unique concrete target: keep the specialization unknown.
				graph.addEffect(specID, EffectSite{Kind: EffectMayPanicUnknownCallee, Source: call.token})
				graph.addArenaEffect(specID, ArenaEffectSite{Kind: ArenaEffectUnknownCallee, Source: call.token, UnknownAllocation: true})
				graph.addBlockEffect(specID, BlockEffectSite{Kind: BlockEffectUnknownCallee, Source: call.token})
			}
		}
	}
	instantiations := make([]genericInstantiation, 0, len(a.genericInstantiations))
	for _, instantiation := range a.genericInstantiations {
		if !instantiation.forwarded {
			instantiations = append(instantiations, instantiation)
		}
	}
	for _, instantiation := range sortedInstantiations(instantiations) {
		link(instantiation.function, instantiation.template, instantiation.substitution)
	}
}

// genericTemplateAt finds the template whose declaration token is key.
func (a *Analyzer) genericTemplateAt(key sourceTokenKey, name string) (Function, bool) {
	for _, function := range a.functions[name] {
		if len(function.GenericParameters) > 0 && sourceTokenLocation(function.Token) == key {
			return function, true
		}
	}
	return Function{}, false
}

// concreteConstraintMethod resolves the unique concrete implementation of a
// constraint method on a substituted receiver type.
func (a *Analyzer) concreteConstraintMethod(receiver Type, call genericConstraintCall) (Function, bool) {
	var found []Function
	for _, method := range a.functions[receiver.Name+"."+call.method] {
		if len(method.Parameters) == call.arguments && len(method.GenericParameters) == 0 {
			found = append(found, method)
		}
	}
	if len(found) != 1 {
		return Function{}, false
	}
	return found[0], true
}

func sortedInstantiations(instantiations []genericInstantiation) []genericInstantiation {
	for i := 1; i < len(instantiations); i++ {
		for j := i; j > 0 && tokenBefore(instantiations[j].call, instantiations[j-1].call); j-- {
			instantiations[j], instantiations[j-1] = instantiations[j-1], instantiations[j]
		}
	}
	return instantiations
}

// addSpecializedSite copies one template call site to a specialization node.
func (g *CallGraph) addSpecializedSite(caller CallableID, site CallSite) {
	id := g.semanticCallSiteID(caller, site.Source, site.Dispatch, site.Execution)
	if g.siteIDs[id] {
		return
	}
	g.siteIDs[id] = true
	g.invalidateAllocationFixedPoint()
	copied := site
	copied.ID = id
	copied.Caller = caller
	copied.Targets = append([]CallableID(nil), site.Targets...)
	copied.TargetSet = cloneCallableTargetSet(site.TargetSet)
	g.sites = append(g.sites, copied)
}

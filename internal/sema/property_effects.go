package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// propertyWithAccessors preserves separate callable contracts for each source
// accessor before any caller is analyzed. A setter's recoverable error is part
// of its return type, not a panic effect.
// Rules: rules/declarations/properties.md — §§4–6,14;
// rules/analysis/effect_analysis.md — §§9–10;
// rules/corrections/applied/md030-no-panic-attribute-placement-correction-20261017.md — §7(4).
func propertyWithAccessors(owner Type, declaration *ast.PropertyDeclaration, property Property) Property {
	makeCallable := func(kind string, body *ast.BlockStatement, result Type) *Function {
		if body == nil {
			return nil
		}
		return &Function{
			Name: property.Name + "." + kind, Module: property.DeclaringModule,
			ImplTarget: owner.Name, Static: property.Static,
			Token: body.Token, DeclarationEnd: body.EndToken, ReturnType: result,
			GenericParameters: append([]string(nil), owner.GenericParameters...),
		}
	}
	property.GetterCallable = makeCallable("get", declaration.Getter, property.Type)
	if setter := declaration.Setter; setter != nil && !setter.Invalid {
		result := Type{Name: "void", Kind: VoidType}
		kind := "set"
		if setter.Fallible {
			if property.Error == nil {
				return property
			}
			kind = "try-set"
			result = Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{result, *property.Error}}
		}
		property.SetterCallable = makeCallable(kind, setter.Body, result)
		if property.SetterCallable != nil {
			property.SetterCallable.Parameters = []FunctionParameter{{Type: property.Type}}
			property.SetterCallable.ReceiverMutable = !property.Static
		}
	}
	return property
}

// enterPropertyAccessor attributes every accessor's direct operations, calls
// and deferred cleanup to its own callable, even without a written guarantee.
// Only the annotated accessor is subsequently verified for @noPanic.
// Rules: rules/declarations/properties.md — §14;
// rules/analysis/effect_analysis.md — §10(1–2), §§15–17;
// rules/foundations/attributes.md — "Callable-body attachment (MD-030)".
func (a *Analyzer) enterPropertyAccessor(target Type, name, kind string, attributes []*ast.Attribute, body *ast.BlockStatement) func() {
	property, ok := lookupProperty(target, name)
	if !ok || body == nil || a.summaryPass {
		return func() {}
	}
	function := property.GetterCallable
	if kind != "get" {
		function = property.SetterCallable
	}
	if function == nil {
		return func() {}
	}
	id := a.callGraph.addCallable(*function)
	if _, guaranteed := EffectiveGuaranteeOf(attributes, GuaranteeNoPanic); guaranteed {
		a.noPanicAccessors = append(a.noPanicAccessors, noPanicAccessor{attributes: attributes, callable: id, name: target.Name + "." + name, kind: kind})
	}
	previous := a.currentCallable
	previousMetadata := a.currentFunctionMetadata
	previousToken := a.currentFunctionToken
	a.currentCallable = id
	a.currentFunctionMetadata = *function
	a.currentFunctionToken = function.Token
	return func() {
		a.currentCallable = previous
		a.currentFunctionMetadata = previousMetadata
		a.currentFunctionToken = previousToken
	}
}

// recordPropertyAccessorEffects links validated operations to exactly the
// accessors they execute. Compound updates execute both, with distinct sites
// at the same source expression; simple stores never invoke the getter.
// Compiler-semantic members without source bodies keep their owning intrinsic
// effect producers rather than acquiring fabricated Sec bodies.
// Rules: rules/declarations/properties.md — §§4–7,14;
// rules/analysis/call_graph.md — §§19–21,84–85.
func (a *Analyzer) recordPropertyAccessorEffects(expr ast.Expression, property Property, kind ResolvedPropertyAccessKind) {
	if a.summaryPass || !a.callGraphPathReachable || a.currentCallable == "" || a.callGraph == nil {
		return
	}
	if kind == PropertyRead || kind == PropertyCompoundUpdate {
		a.callGraph.addPropertyAccessorCall(a.currentCallable, property.GetterCallable, expressionToken(expr), "get")
	}
	if kind != PropertyRead {
		a.callGraph.addPropertyAccessorCall(a.currentCallable, property.SetterCallable, expressionToken(expr), "set")
	}
}

// addPropertyAccessorCall records one synchronous compiler-generated accessor
// invocation. The accessor role is part of generated syntax identity so a
// compound update's getter and setter do not collapse into one call site.
// Rules: rules/analysis/call_graph.md — §20(1–4), §21, §85;
// rules/declarations/properties.md — §§7,14.
func (g *CallGraph) addPropertyAccessorCall(caller CallableID, target *Function, source lexer.Token, role string) {
	if target == nil || source.Line <= 0 || source.Column <= 0 {
		return
	}
	targetID := g.addCallable(*target)
	base := g.semanticCallSiteID(caller, source, CallDispatchGenerated, CallExecutionSynchronous)
	id := CallSiteID(string(base) + "|property-" + role)
	if g.siteIDs[id] {
		return
	}
	g.siteIDs[id] = true
	g.invalidateAllocationFixedPoint()
	g.sites = append(g.sites, CallSite{
		ID: id, Caller: caller, Targets: []CallableID{targetID},
		TargetSet: exactCallableTargetSet(callableBodyID(*target)), Source: source,
		Dispatch: CallDispatchGenerated, Execution: CallExecutionSynchronous,
	})
}

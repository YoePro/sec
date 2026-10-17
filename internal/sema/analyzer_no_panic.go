package sema

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"strings"
)

// validateNoPanicGuarantees enforces the transitive verified guarantee from
// attributes.md and effect_analysis.md. Package 14 section 55 consumes this
// shared call-graph fact for fixed-array indexing.
// Rules: rules/errors/panic.md — §21; rules/analysis/effect_analysis.md — "Declared guarantees".
func (a *Analyzer) validateNoPanicGuarantees(program *ast.Program) {
	a.withProgramModules(program, func(statement ast.Statement) {
		switch statement := statement.(type) {
		case *ast.FunctionDeclaration:
			a.validateFunctionNoPanicGuarantee(statement, statement.Name.Value)
		case *ast.ImplStatement:
			if statement.Target == nil || !a.validImplStatements[statement] {
				return
			}
			for _, member := range statement.Members {
				fn, ok := member.(*ast.FunctionDeclaration)
				if ok && fn != nil {
					a.validateFunctionNoPanicGuarantee(fn, statement.Target.Name+"."+fn.Name.Value)
					continue
				}
				a.validateBodyNoPanicGuarantees(statement.Target.Name, member)
			}
		}
	})
	a.validateAccessorAndLambdaNoPanicGuarantees()
}

// validateFunctionNoPanicGuarantee selects a canonical panic cause for the verified root.
// Rules: rules/errors/panic.md — §§21,25,28(2)–(3).
func (a *Analyzer) validateFunctionNoPanicGuarantee(fn *ast.FunctionDeclaration, name string) {
	guarantee, ok := EffectiveGuaranteeOf(fn.Attributes, GuaranteeNoPanic)
	if !ok || (fn.Extern && guarantee.Explicit()) {
		return
	}
	function, ok := a.lookupFunctionByToken(name, fn.Name.Token)
	if !ok || function.ReturnType.Kind == InvalidType {
		return
	}
	a.reportNoPanicViolation(fn.Attributes, callableID(function), "function", name)
}

// reportNoPanicViolation reports the canonical panic path of one callable
// body whose attributes declare @noPanic (directly or through an implying
// attribute): a function, init, free, property accessor or lambda (MD-030).
// The diagnostic names the body kind, the reachable path and the introducing
// operation, and is a compile error, never advisory.
//
// Rules:
//   - rules/errors/panic.md — §§21,25,28(2)–(3)
//   - rules/foundations/attributes.md — "Callable-body attachment (MD-030)"
func (a *Analyzer) reportNoPanicViolation(attributes []*ast.Attribute, id CallableID, kindLabel, name string) {
	guarantee, ok := EffectiveGuaranteeOf(attributes, GuaranteeNoPanic)
	if !ok || id == "" {
		return
	}
	attribute := guarantee.Source
	summary := a.callGraph.EffectSummary(id)
	if !summary.MayPanic || len(summary.PanicPath) == 0 {
		return
	}
	chain := make([]string, 0, len(summary.PanicPath))
	for _, step := range summary.PanicPath {
		if node, exists := a.callGraph.Node(step); exists {
			chain = append(chain, node.Name)
		}
	}
	introducingID := summary.PanicPath[len(summary.PanicPath)-1]
	kind := EffectKind("may-panic")
	source := lexer.Token{}
	for _, cause := range summary.PanicCauses {
		if cause.Callable == introducingID {
			kind = cause.Effect.Kind
			source = cause.Source
			break
		}
	}
	message := fmt.Sprintf("%s %s does not satisfy @noPanic%s: reachable %s effect via %s", kindLabel, name, guarantee.impliedSuffix(), kind, strings.Join(chain, " -> "))
	if source.Line > 0 && source.Column > 0 {
		message += fmt.Sprintf("; effect introduced at %s", formatLocation(source.File, source.Line, source.Column))
	}
	help := noPanicEffectHelp(kind)
	if len(chain) > 1 {
		help = "The panic comes from " + chain[len(chain)-1] + ", which " + name + " calls. " + help
	}
	if root, context := interruptPanicRoot(attributes); root != nil {
		message = context + " " + message
		help = interruptPanicHelp(kind, root.Name.Value)
		if len(chain) > 1 {
			help = "The reachable panic path is " + strings.Join(chain, " -> ") + ". " + help
		}
		// ISR diagnostics locate the actual cause and relate it to the
		// interrupt context, including when @noPanic is also explicit.
		// Rules: rules/platform/interrupts.md — §43; rules/errors/panic.md — §22,§28(5).
		if source.Line > 0 && source.Column > 0 {
			a.addErrorAtTokenWithMetadataAndPrevious(source, root.Token, diagnostics.NoPanicViolation, help, "%s", message)
			return
		}
		a.addErrorAtTokenWithMetadata(root.Token, diagnostics.NoPanicViolation, help, "%s", message)
		return
	}
	if source.Line > 0 && source.Column > 0 {
		a.addErrorAtTokenWithMetadataAndPrevious(attribute.Token, source, diagnostics.NoPanicViolation, help, "%s", message)
		return
	}
	a.addErrorAtTokenWithMetadata(attribute.Token, diagnostics.NoPanicViolation, help, "%s", message)
}

// noPanicEffectHelp explains, in programmer terms, why an effect kind may
// panic and how the program can avoid it.
//
// Rules:
//   - rules/errors/panic.md — § 21 "@noPanic"
//   - rules/errors/errorhandling.md — §30 "Diagnostics must act as a mentor"
func noPanicEffectHelp(kind EffectKind) string {
	switch kind {
	case EffectMayPanicArithmetic:
		return "Checked arithmetic can overflow, divide by zero, or shift out of range. Prove the operand ranges, or use try to turn the failure into ArithmeticError."
	case EffectMayPanicBounds:
		return "Indexing panics when the index is out of range. Prove the index (for example with a range-constrained index type or a dominating assert), or use try to handle IndexError."
	case EffectMayPanicContract:
		return "Converting a run-time value into a constrained type panics when the value breaks the type's contract. Use try to handle ContractError."
	case EffectMayPanicExplicit:
		return "An explicit panic always panics when reached. Return an error instead, or remove @noPanic."
	case EffectMayPanicAssertion:
		return "An assert whose condition the compiler cannot prove may fail. Return an error for the failing case, or establish the condition so the assertion is provable."
	case EffectMayPanicUnreachable:
		return "checked unreachable panics if control reaches it. Make the path statically impossible, or handle the case explicitly."
	case EffectMayPanicUnknownCallee:
		return "This call's target is not known here (a function value, an interface method, or a generic constraint method), so whether it can panic cannot be proven. Call a known function or concrete method directly, or remove @noPanic."
	case EffectMayPanicInvalidReference:
		return "This use goes through an Arena-backed reference whose allocation may have been reset or released, so the compiler checks its generation at run time and panics with InvalidReferenceGeneration when it is stale. Keep the Arena live and unreset across the use, or remove @noPanic."
	case EffectMayPanicForeign:
		return "A call to an extern function has unknown foreign behavior and may abort. When the foreign documentation guarantees it cannot abort or unwind, mark the extern declaration @noPanic as a trusted foreign contract."
	}
	return "Remove the panic-capable operation from every path reachable from this function, or remove @noPanic."
}

// noPanicAccessor is a property accessor whose own body carries an
// attribute-declared guarantee (MD-030 § 3(3)).
type noPanicAccessor struct {
	attributes []*ast.Attribute
	callable   CallableID
	name       string
	kind       string
}

// enterNoPanicAccessor gives an accessor annotated with a verified guarantee
// its own call-graph callable while its body is analysed, so the effects of
// exactly that accessor are summarized and verified; the other accessor of
// the property does not acquire the guarantee. It returns the restore step.
//
// Rules:
//   - rules/foundations/attributes.md — "Callable-body attachment (MD-030)"
//   - rules/errors/panic.md — § 21
func (a *Analyzer) enterNoPanicAccessor(target Type, property, kind string, attributes []*ast.Attribute, body *ast.BlockStatement, returnType Type) func() {
	if _, ok := EffectiveGuaranteeOf(attributes, GuaranteeNoPanic); !ok || body == nil || a.summaryPass {
		return func() {}
	}
	function := Function{
		Name: property + "." + strings.ReplaceAll(kind, " ", "-"), ImplTarget: target.Name,
		Module: a.currentModule, Token: body.Token, ReturnType: returnType,
	}
	id := a.callGraph.addCallable(function)
	a.noPanicAccessors = append(a.noPanicAccessors, noPanicAccessor{attributes: attributes, callable: id, name: target.Name + "." + property, kind: kind})
	previous := a.currentCallable
	a.currentCallable = id
	return func() { a.currentCallable = previous }
}

// validateBodyNoPanicGuarantees verifies the MD-030 callable bodies: init,
// free, property accessors and directly initialized lambdas.
func (a *Analyzer) validateBodyNoPanicGuarantees(target string, member ast.ImplMember) {
	switch member := member.(type) {
	case *ast.InitDeclaration:
		if member == nil {
			return
		}
		if function, ok := a.lookupFunctionByToken(initFunctionName(target), member.Token); ok && function.ReturnType.Kind != InvalidType {
			a.reportNoPanicViolation(member.Attributes, callableID(function), "init", target+".init")
		}
	case *ast.FreeDeclaration:
		if member == nil {
			return
		}
		if function, ok := a.lookupFunctionByToken(freeFunctionName(target), member.Token); ok && function.ReturnType.Kind != InvalidType {
			a.reportNoPanicViolation(member.Attributes, callableID(function), "free", target+".free")
		}
	}
}

func (a *Analyzer) validateAccessorAndLambdaNoPanicGuarantees() {
	for _, accessor := range a.noPanicAccessors {
		a.reportNoPanicViolation(accessor.attributes, accessor.callable, "property "+accessor.kind+" accessor", accessor.name)
	}
	for _, lambda := range a.noPanicLambdas {
		identity, ok := a.resolvedCallableIdentities[lambda]
		if !ok {
			continue
		}
		if id, exists := a.callGraph.bodyNodes[identity.Body]; exists {
			a.reportNoPanicViolation(lambda.Attributes, id, "lambda", fmt.Sprintf("at %s", formatLocation(lambda.Token.File, lambda.Token.Line, lambda.Token.Column)))
		}
	}
}

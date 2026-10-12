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
				}
			}
		}
	})
}

// validateFunctionNoPanicGuarantee selects a canonical panic cause for the verified root.
// Rules: rules/errors/panic.md — §§21,25,28(2)–(3).
func (a *Analyzer) validateFunctionNoPanicGuarantee(fn *ast.FunctionDeclaration, name string) {
	guarantee, ok := EffectiveGuaranteeOf(fn.Attributes, GuaranteeNoPanic)
	if !ok || (fn.Extern && guarantee.Explicit()) {
		return
	}
	attribute := guarantee.Source
	function, ok := a.lookupFunctionByToken(name, fn.Name.Token)
	if !ok || function.ReturnType.Kind == InvalidType {
		return
	}
	summary := a.callGraph.EffectSummary(callableID(function))
	if !summary.MayPanic || len(summary.PanicPath) == 0 {
		return
	}
	chain := make([]string, 0, len(summary.PanicPath))
	for _, id := range summary.PanicPath {
		if node, exists := a.callGraph.Node(id); exists {
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
	message := fmt.Sprintf("function %s does not satisfy @noPanic%s: reachable %s effect via %s", name, guarantee.impliedSuffix(), kind, strings.Join(chain, " -> "))
	if source.Line > 0 && source.Column > 0 {
		message += fmt.Sprintf("; effect introduced at %s", formatLocation(source.File, source.Line, source.Column))
	}
	help := noPanicEffectHelp(kind)
	if len(chain) > 1 {
		help = "The panic comes from " + chain[len(chain)-1] + ", which " + name + " calls. " + help
	}
	if root, context := interruptPanicRoot(fn.Attributes); root != nil {
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

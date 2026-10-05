package sema

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// BlockEffectKind classifies why a site may physically block.
type BlockEffectKind string

const (
	// BlockEffectOperation is a represented waiting operation: await,
	// Sender.Send/SendRevocable, Receiver.Receive, Mutex.lock, or select
	// without default.
	BlockEffectOperation BlockEffectKind = "may-block"
	// BlockEffectForeign is a call to an extern function without a trusted
	// @noBlock foreign contract.
	BlockEffectForeign BlockEffectKind = "may-block-foreign"
	// BlockEffectUnknownCallee is a call whose target is not known here (a
	// function value, an interface method, or a generic constraint method).
	BlockEffectUnknownCallee BlockEffectKind = "may-block-unknown-callee"
)

// BlockEffectSite is one direct site that may block, or whose blocking
// behavior is unknown.
type BlockEffectSite struct {
	Kind      BlockEffectKind
	Operation string
	Source    lexer.Token
}

// BlockCallableSummary reports whether a callable may block through a
// synchronous path, with the shortest such path. Unknown blocking behavior
// counts as blocking (rules/foundations/attributes.md "@noBlock verification").
type BlockCallableSummary struct {
	DirectEffects []BlockEffectSite
	MayBlock      bool
	BlockPath     []CallableID
}

func (g *CallGraph) addBlockEffect(caller CallableID, effect BlockEffectSite) {
	if g == nil || caller == "" || effect.Source.Line <= 0 || effect.Source.Column <= 0 {
		return
	}
	g.blockEffects[caller] = append(g.blockEffects[caller], effect)
}

// BlockSummary returns the blocking summary of one callable. Only synchronous
// call edges are followed: starting a task or thread that blocks does not
// block the caller.
//
// Rules:
//   - rules/concurrency/blocking.md — "Blocking effects", "Call graph analysis"
func (g *CallGraph) BlockSummary(id CallableID) BlockCallableSummary {
	if g == nil {
		return BlockCallableSummary{}
	}
	summary := BlockCallableSummary{DirectEffects: append([]BlockEffectSite(nil), g.blockEffects[id]...)}
	summary.BlockPath = g.synchronousPathTo(id, func(candidate CallableID) bool {
		return len(g.blockEffects[candidate]) > 0
	})
	summary.MayBlock = len(summary.BlockPath) > 0
	return summary
}

// recordBlockingOperation records a represented waiting operation. Inside a
// select branch the operation only registers readiness; the select itself is
// recorded instead.
//
// Rules:
//   - rules/concurrency/blocking.md — "Potentially blocking operations", "Nonblocking operations", "Select"
func (a *Analyzer) recordBlockingOperation(operation string, source lexer.Token) {
	if a.summaryPass || !a.callGraphPathReachable || a.inSelectOperation {
		return
	}
	a.callGraph.addBlockEffect(a.currentCallable, BlockEffectSite{Kind: BlockEffectOperation, Operation: operation, Source: source})
}

// recordForeignBlockingEffect treats a call to an extern function as
// potentially blocking unless its declaration carries a trusted @noBlock
// foreign contract.
//
// Rules:
//   - rules/concurrency/blocking.md — "FFI"
//   - rules/foundations/attributes.md — "Sec code versus foreign declarations"
func (a *Analyzer) recordForeignBlockingEffect(callee Function, source lexer.Token) {
	if !callee.Extern || callee.TrustedNoBlock || a.summaryPass || !a.callGraphPathReachable {
		return
	}
	a.callGraph.addBlockEffect(a.currentCallable, BlockEffectSite{Kind: BlockEffectForeign, Source: source})
}

// recordUnknownBlockingCallee records a call whose blocking behavior is
// unknown because its target is not known.
//
// Rules:
//   - rules/concurrency/blocking.md — "Call graph analysis": indirect calls carry conservative effects
func (a *Analyzer) recordUnknownBlockingCallee(source lexer.Token) {
	if a.summaryPass || !a.callGraphPathReachable {
		return
	}
	a.callGraph.addBlockEffect(a.currentCallable, BlockEffectSite{Kind: BlockEffectUnknownCallee, Source: source})
}

// validateNoBlockGuarantees enforces the transitive @noBlock guarantee on
// every Sec function and method that writes it, or whose effective
// guarantees imply it: no reachable synchronous path may wait or reach code
// whose blocking behavior is unknown. On an extern declaration @noBlock is a
// trusted foreign contract instead.
//
// Rules:
//   - rules/foundations/attributes.md — "@noBlock", "Blocking operations", "@noBlock verification"
//   - rules/concurrency/blocking.md — "Blocking effects", "Call graph analysis", "FFI"
func (a *Analyzer) validateNoBlockGuarantees(program *ast.Program) {
	a.withProgramModules(program, func(statement ast.Statement) {
		switch statement := statement.(type) {
		case *ast.FunctionDeclaration:
			a.validateFunctionNoBlockGuarantee(statement, statement.Name.Value)
		case *ast.ImplStatement:
			if statement.Target == nil || !a.validImplStatements[statement] {
				return
			}
			for _, member := range statement.Members {
				fn, ok := member.(*ast.FunctionDeclaration)
				if ok && fn != nil {
					a.validateFunctionNoBlockGuarantee(fn, statement.Target.Name+"."+fn.Name.Value)
				}
			}
		}
	})
}

func (a *Analyzer) validateFunctionNoBlockGuarantee(fn *ast.FunctionDeclaration, name string) {
	if fn.Extern || fn.Name == nil {
		return
	}
	guarantee, ok := EffectiveGuaranteeOf(fn.Attributes, GuaranteeNoBlock)
	if !ok {
		return
	}
	function, ok := a.lookupFunctionByToken(name, fn.Name.Token)
	if !ok || function.ReturnType.Kind == InvalidType {
		return
	}
	summary := a.callGraph.BlockSummary(callableID(function))
	if !summary.MayBlock {
		return
	}
	chain := make([]string, 0, len(summary.BlockPath))
	for _, id := range summary.BlockPath {
		if node, exists := a.callGraph.Node(id); exists {
			chain = append(chain, node.Name)
		}
	}
	effects := a.callGraph.BlockSummary(summary.BlockPath[len(summary.BlockPath)-1]).DirectEffects
	var site BlockEffectSite
	found := false
	for _, effect := range effects {
		if effect.Source.Line > 0 {
			site, found = effect, true
			break
		}
	}
	kind := "blocking operation"
	switch site.Kind {
	case BlockEffectForeign:
		kind = "foreign call with unknown blocking behavior"
	case BlockEffectUnknownCallee:
		kind = "call with unknown blocking behavior"
	case BlockEffectOperation:
		kind = "blocking operation " + site.Operation
	}
	message := fmt.Sprintf("function %s does not satisfy @noBlock%s: reachable %s via %s", name, guarantee.impliedSuffix(), kind, strings.Join(chain, " -> "))
	if found {
		message += fmt.Sprintf("; introduced at %s", formatLocation(site.Source.File, site.Source.Line, site.Source.Column))
	}
	help := noBlockEffectHelp(site.Kind)
	if len(chain) > 1 {
		help = "It comes from " + chain[len(chain)-1] + ", which " + name + " calls. " + help
	}
	if found {
		a.addErrorAtTokenWithMetadataAndPrevious(guarantee.Source.Token, site.Source, diagnostics.NoBlockViolation, help, "%s", message)
		return
	}
	a.addErrorAtTokenWithMetadata(guarantee.Source.Token, diagnostics.NoBlockViolation, help, "%s", message)
}

// noBlockEffectHelp explains, in programmer terms, why a site breaks the
// @noBlock guarantee and how to avoid it.
//
// Rules:
//   - rules/foundations/attributes.md — "What @noBlock does not automatically forbid"
//   - rules/concurrency/blocking.md — "Nonblocking operations", "FFI"
func noBlockEffectHelp(kind BlockEffectKind) string {
	switch kind {
	case BlockEffectOperation:
		return "This operation may wait. Use a nonblocking form such as TrySend, TryReceive, or tryLock, give select a default branch, or remove @noBlock."
	case BlockEffectForeign:
		return "A call to an extern function may block unless its declaration says otherwise. When the foreign documentation guarantees it never blocks, mark the extern declaration @noBlock as a trusted foreign contract."
	case BlockEffectUnknownCallee:
		return "This call's target is not known here (a function value, an interface method, or a generic constraint method), so whether it blocks cannot be proven. Call a known function or concrete method directly, or remove @noBlock."
	}
	return "Remove the blocking operation from every path reachable from this function, or remove @noBlock."
}

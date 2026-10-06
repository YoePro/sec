package semantic

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/sema"
)

// buildExplicitPanic lowers a panic statement from Sema's immutable
// ResolvedExplicitPanic fact: the registered reason, the optional static
// message, and the source location, without reading the AST spelling.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 55 "Panic"
//   - rules/errors/panic.md — explicit panic
func (fb *functionBuilder) buildExplicitPanic(stmt *ast.PanicStatement) error {
	fact, ok := fb.owner.analyzer.ResolvedExplicitPanicOf(stmt)
	if !ok {
		return fmt.Errorf("missing resolved explicit panic at %s", formatLocation(location(stmt.Token)))
	}
	return fb.emitPanic(fact.ReasonID, fact.Message, fact.HasMessage, Location{File: fact.File, Line: fact.Line, Column: fact.Column})
}

// buildCheckedUnreachable lowers a reached checked `unreachable` to its
// defined panic with the registered CheckedUnreachableReached reason; it
// never becomes backend undefined behavior.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 57 "Checked unreachable"
//   - rules/errors/panic.md — § 16 "Checked unreachable"
func (fb *functionBuilder) buildCheckedUnreachable(stmt *ast.UnreachableStatement) error {
	return fb.emitPanic(diagnostics.PanicReasonCheckedUnreachableReached, "", false, location(stmt.Token))
}

// buildAssertion lowers an assertion from Sema's ResolvedAssertion fact. An
// unproven assertion evaluates its bool condition once and branches to a
// failure block that panics with the AssertionFailure reason and the static
// message; a proven assertion emits no check, its refinement being consumed
// from Sema's condition facts. An assertion is never an optimizer assumption.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 56 "Assertion"
//   - rules/errors/panic.md — § 15 "Assertions"
func (fb *functionBuilder) buildAssertion(stmt *ast.AssertStatement) error {
	fact, ok := fb.owner.analyzer.ResolvedAssertionOf(stmt)
	if !ok {
		return fmt.Errorf("missing resolved assertion at %s", formatLocation(location(stmt.Token)))
	}
	if fact.Proven {
		return nil
	}
	boolType, err := fb.boolType()
	if err != nil {
		return err
	}
	condition, err := fb.buildExpr(stmt.Condition, boolType)
	if err != nil {
		return err
	}
	loc := Location{File: fact.File, Line: fact.Line, Column: fact.Column}
	failure := fb.newBlock()
	success := fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{condition.id}, Successors: []BranchTarget{{Block: success.ID}, {Block: failure.ID}}, AssertCheck: true, Location: loc})
	fb.current = failure
	if err := fb.emitPanic(fact.ReasonID, fact.Message, fact.HasMessage, loc); err != nil {
		return err
	}
	fb.current = success
	return nil
}

func (fb *functionBuilder) emitPanic(reason diagnostics.PanicReasonID, message string, hasMessage bool, loc Location) error {
	definition, ok := diagnostics.PanicReasonByID(reason)
	if !ok {
		return fmt.Errorf("unregistered panic reason %d", reason)
	}
	fb.emit(Operation{Kind: OpPanic, PanicReasonID: uint16(definition.ID), PanicReason: definition.Name, PanicMessage: message, PanicHasMessage: hasMessage, Location: loc})
	fb.current = nil
	return nil
}

// functionNoPanic records the effective @noPanic guarantee Sema verified
// for a declaration, naming whether it was written, implied, or a trusted
// foreign contract.
func functionNoPanic(decl *ast.FunctionDeclaration) (bool, string) {
	guarantee, ok := sema.EffectiveGuaranteeOf(decl.Attributes, sema.GuaranteeNoPanic)
	if !ok {
		return false, ""
	}
	switch {
	case decl.Extern:
		return true, "trusted foreign contract"
	case guarantee.Explicit():
		return true, "written"
	default:
		return true, "implied by @" + strings.Join(guarantee.ImpliedBy, " via @")
	}
}

// verifyPanicOperation checks an OpPanic against the panic reason registry.
func verifyPanicOperation(op Operation) error {
	if len(op.Operands) != 0 || len(op.Results) != 0 || len(op.Successors) != 0 {
		return fmt.Errorf("invalid panic shape")
	}
	definition, ok := diagnostics.PanicReasonByID(diagnostics.PanicReasonID(op.PanicReasonID))
	if !ok || definition.Name != op.PanicReason {
		return fmt.Errorf("panic has unregistered reason %d %q", op.PanicReasonID, op.PanicReason)
	}
	if !op.PanicHasMessage && op.PanicMessage != "" {
		return fmt.Errorf("panic carries a message without a source message")
	}
	return nil
}

// verifyNoPanicContradiction rejects a function marked @noPanic that still
// reaches an explicit panic, a failing assertion, or a checked unreachable:
// such a function contradicts the guarantee Sema verified.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 55(3) noPanic effect facts
//   - rules/errors/panic.md — § 21 "@noPanic"
func verifyNoPanicContradiction(fn *Function, blocks map[BlockID]*Block) error {
	if !fn.NoPanic {
		return nil
	}
	if fn.NoPanicSource == "" {
		return fmt.Errorf("@noPanic function has no guarantee source")
	}
	seen := map[BlockID]bool{}
	work := []BlockID{fn.Entry}
	for len(work) > 0 {
		id := work[len(work)-1]
		work = work[:len(work)-1]
		block := blocks[id]
		if block == nil || seen[id] {
			continue
		}
		seen[id] = true
		for _, op := range block.Operations {
			if op.Kind == OpPanic {
				return fmt.Errorf("@noPanic function reaches panic %s at %s", op.PanicReason, formatLocation(op.Location))
			}
			for _, successor := range op.Successors {
				work = append(work, successor.Block)
			}
		}
	}
	return nil
}

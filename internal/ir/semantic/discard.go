package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// buildDiscard lowers `discard expression` and `discard binding` to one
// explicit discard.value: the expression is evaluated, and the consumed value
// (or the binding's storage) is recorded with its trivial-destruction
// classification. Only trivially destructible values are lowered yet; a value
// that needs a destruction plan stays an explicit unsupported construct.
//
// Rules:
//   - rules/control-flow/discard.md — §§ 3, 5, 9, 36
func (fb *functionBuilder) buildDiscard(stmt *ast.DiscardStatement) error {
	loc := location(stmt.Token)
	if stmt.Name != nil {
		fact, ok := fb.owner.analyzer.ResolvedBindingOf(stmt.Name)
		if !ok {
			return fb.unsupported("discard of an unresolved binding", stmt.Token)
		}
		if !sema.TriviallyDestructible(fact.Type) {
			return fb.unsupported("discard of a binding that needs destruction", stmt.Token)
		}
		bind, ok := fb.bindings[fact.ID]
		if !ok {
			return fb.unsupported("discard of a binding without semantic storage", stmt.Token)
		}
		if bind.storage == 0 {
			if handled, err := fb.discardSliceHolder(builtValue{id: bind.value, typ: bind.typ}, stmt.Token); handled {
				return err
			}
		}
		op := Operation{Kind: OpDiscardValue, DiscardProvenance: DiscardExplicit, DiscardTrivial: true, Location: loc}
		if bind.storage != 0 {
			op.Storage = bind.storage
		} else {
			op.Operands = []ValueID{bind.value}
		}
		fb.emit(op)
		return nil
	}
	if stmt.Value == nil {
		return fb.unsupported("empty discard", stmt.Token)
	}
	return fb.discardExpression(stmt.Value, DiscardExplicit, stmt.Token)
}

// discardExpression evaluates expr and consumes its value.
func (fb *functionBuilder) discardExpression(expr ast.Expression, provenance DiscardProvenance, token lexer.Token) error {
	typ, ok := fb.owner.analyzer.ResolvedTypeOf(expr)
	if !ok {
		return fb.unsupported("discard of an untyped expression", token)
	}
	if !sema.TriviallyDestructible(typ) {
		return fb.unsupported(fmt.Sprintf("discard of %s, which needs destruction", typ.Name), token)
	}
	value, err := fb.buildExpr(expr, 0)
	if err != nil {
		return err
	}
	if handled, err := fb.discardSliceHolder(value, token); handled {
		return err
	}
	fb.emit(Operation{Kind: OpDiscardValue, Operands: []ValueID{value.id}, DiscardProvenance: provenance, DiscardTrivial: true, Location: location(token)})
	return nil
}

// discardSliceHolder ends a constructed non-owning slice holder through P15's
// marker. It does not copy/destroy elements or discard the backing storage.
// Rules: rules/compiler/semantic_ir.md — §126.5,21;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§42,63.
func (fb *functionBuilder) discardSliceHolder(value builtValue, token lexer.Token) (bool, error) {
	typ, _ := fb.owner.module.Types.Lookup(value.typ)
	if typ.Kind != TypeSharedSlice && typ.Kind != TypeMutableSlice {
		return false, nil
	}
	facts := fb.references[value.id]
	if facts == nil {
		return true, fmt.Errorf("discarded slice holder has no reference facts")
	}
	fb.emit(Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{value.id}, Reference: cloneReferenceMetadata(facts), Location: location(token)})
	return true, nil
}

func verifyDiscardValue(op Operation, storages map[StorageID]Storage) error {
	if op.DiscardProvenance != DiscardExplicit && op.DiscardProvenance != DiscardImplicitCallResult {
		return fmt.Errorf("invalid discard provenance %q", op.DiscardProvenance)
	}
	if len(op.Results) != 0 || len(op.Successors) != 0 {
		return fmt.Errorf("discard.value has no results or successors")
	}
	if op.Storage != 0 {
		if len(op.Operands) != 0 || storages[op.Storage].ID == 0 {
			return fmt.Errorf("invalid discard.value of storage")
		}
		return nil
	}
	if len(op.Operands) != 1 {
		return fmt.Errorf("discard.value consumes exactly one value")
	}
	return nil
}

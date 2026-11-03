package semantic

import (
	"sec/internal/lexer"
	"sec/internal/sema"
)

// pendingTemporary is an owned temporary that no owner took: it stays valid to
// the end of its full expression and is then destroyed (destruction.md § 8).
type pendingTemporary struct {
	value builtValue
	typ   sema.Type
	token lexer.Token
	// loops is the number of enclosing loops where it was created; a break or
	// continue of an inner loop leaves it alive.
	loops int
	// block is where it was created; it must dominate where it ends.
	block BlockID
	// scopes is the depth of open cleanup scopes at its creation: deeper ones
	// hold younger owners, which a jump must clean before it (§ 8(3)).
	scopes int
	// stored is the compiler storage a materialized match subject lives in;
	// it ends in place.
	stored *ownedStorage
}

// holdTemporary keeps a fresh owned value alive to the end of its full
// expression. A statement, a return and an if/while condition each end one
// (owner decision 2026-11-03).
func (fb *functionBuilder) holdTemporary(value builtValue, typ sema.Type, token lexer.Token) {
	fb.temporaries = append(fb.temporaries, pendingTemporary{value: value, typ: typ, token: token, loops: len(fb.loops), block: fb.current.ID})
}

// endFullExpression destroys the pending temporaries in reverse creation order
// (destruction.md § 8(3)). One created on only some paths of its full
// expression, as under `&&` or a conditional expression, has no form yet.
func (fb *functionBuilder) endFullExpression() error {
	pending := fb.temporaries
	fb.temporaries = nil
	if fb.temporaryError != nil {
		err := fb.temporaryError
		fb.temporaryError = nil
		return err
	}
	if fb.current == nil {
		return nil
	}
	for _, temporary := range pending {
		if !fb.dominatesCurrent(temporary.block) {
			return fb.unsupported("owned temporary created on only some paths of its full expression (P17-40)", temporary.token)
		}
	}
	return fb.destroyTemporaries(pending)
}

// dominatesCurrent reports whether every path from the entry to the current
// block passes through block.
func (fb *functionBuilder) dominatesCurrent(block BlockID) bool {
	if block == fb.current.ID {
		return true
	}
	successors := map[BlockID][]BlockID{}
	for _, candidate := range fb.fn.Blocks {
		for _, op := range candidate.Operations {
			for _, successor := range op.Successors {
				successors[candidate.ID] = append(successors[candidate.ID], successor.Block)
			}
		}
	}
	seen := map[BlockID]bool{block: true}
	work := []BlockID{fb.fn.Entry}
	for len(work) != 0 {
		next := work[len(work)-1]
		work = work[:len(work)-1]
		if next == fb.current.ID {
			return false
		}
		if seen[next] {
			continue
		}
		seen[next] = true
		work = append(work, successors[next]...)
	}
	return true
}

// holdArmTemporary keeps a fresh match subject that the selected arm does not
// take alive through the whole match (owner decision 2026-11-03); the arm
// destroys it on every way out.
func (fb *functionBuilder) holdArmTemporary(value builtValue, typ sema.Type, token lexer.Token) {
	fb.armTemporaries = append(fb.armTemporaries, pendingTemporary{value: value, typ: typ, token: token, loops: len(fb.loops), scopes: len(fb.cleanupScopes)})
}

// endArmTemporaries destroys and drops the arm temporaries above depth when the
// arm falls out of the match.
func (fb *functionBuilder) endArmTemporaries(depth int) error {
	pending := fb.armTemporaries[depth:]
	fb.armTemporaries = fb.armTemporaries[:depth]
	if fb.current == nil {
		return nil
	}
	return fb.destroyTemporaries(pending)
}

// leaveArmTemporaries destroys, without dropping, the arm temporaries a jump
// leaves: every one for a return, those inside the innermost loop for a break or
// continue. Each is younger than the owners of the scopes that were open when it
// was created and older than those of deeper scopes, so the deeper scopes the
// jump leaves are cleaned first (destruction.md § 8(3)); the scopes cleaned here
// are recorded so the jump's own cleanup skips them.
func (fb *functionBuilder) leaveArmTemporaries(returning bool, kind sema.CleanupExitKind, exited []sema.CleanupScopeID, token lexer.Token) error {
	leaving := map[sema.CleanupScopeID]bool{}
	for _, scope := range exited {
		leaving[scope] = true
	}
	cleaned := map[sema.CleanupScopeID]bool{}
	for index := len(fb.armTemporaries) - 1; index >= 0; index-- {
		temporary := fb.armTemporaries[index]
		if !returning && temporary.loops < len(fb.loops) {
			continue
		}
		var deeper []uint32
		for _, scope := range fb.cleanupScopes[temporary.scopes:] {
			if !cleaned[scope] && (returning || leaving[scope]) {
				cleaned[scope] = true
				deeper = append([]uint32{uint32(scope)}, deeper...)
			}
		}
		if len(deeper) != 0 {
			fb.emit(Operation{Kind: OpCleanupRunScope, Cleanup: &CleanupFacts{Scopes: deeper, Exit: string(kind)}, Location: location(token)})
		}
		if err := fb.destroyTemporaries([]pendingTemporary{temporary}); err != nil {
			return err
		}
	}
	fb.scopesCleaned = cleaned
	return nil
}

func (fb *functionBuilder) destroyTemporaries(pending []pendingTemporary) error {
	for index := len(pending) - 1; index >= 0; index-- {
		temporary := pending[index]
		planID, err := fb.owner.importDestructionPlans(sema.ResolveDestructionPlans(temporary.typ), temporary.typ)
		if err != nil {
			return err
		}
		if temporary.stored != nil {
			place := fb.ownedStoragePlace(*temporary.stored, temporary.token)
			fb.emit(Operation{Kind: OpOwnDestroyPlace, Operands: []ValueID{place.id}, Ownership: &OwnershipFacts{Cause: "temporary-end", DestructionPlan: planID}, Location: location(temporary.token)})
			continue
		}
		fb.emit(Operation{Kind: OpOwnDestroyValue, Operands: []ValueID{temporary.value.id}, Ownership: &OwnershipFacts{Cause: "temporary-end", DestructionPlan: planID}, Location: location(temporary.token)})
	}
	return nil
}

// leaveOnReturn destroys, ahead of a return emitted inside an expression (a try
// propagation), every temporary still owned on that edge: those of the current
// full expression and the arm temporaries (destruction.md § 8(7)).
func (fb *functionBuilder) leaveOnReturn(op Operation) {
	if op.Kind != OpReturn || fb.temporaryExit || len(fb.temporaries) == 0 && len(fb.armTemporaries) == 0 {
		return
	}
	fb.temporaryExit = true
	defer func() { fb.temporaryExit = false }()
	if err := fb.destroyTemporaries(fb.temporaries); err != nil && fb.temporaryError == nil {
		fb.temporaryError = err
	}
	if err := fb.leaveArmTemporaries(true, sema.CleanupExitErrorPropagation, nil, lexer.Token{File: op.Location.File, Line: op.Location.Line, Column: op.Location.Column}); err != nil && fb.temporaryError == nil {
		fb.temporaryError = err
	}
	fb.scopesCleaned = nil
}

package semantic

import (
	"fmt"
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// MD-065 path-exact borrow ends. Sema ends a local reference holder's borrow
// after its last required use on each path and publishes the statement the
// end precedes. Semantic IR emits ref.end-borrow there — under the borrow's
// own BorrowID and LifetimeID — when that statement belongs to the holder's
// own statement list (its let's block, or the arm body of a borrowed match
// payload). An end Sema records inside a nested block is not placed: the
// borrow then conservatively ends at its scope exit (borrowing.md § 8.1(8)).
// An arm-exit marker of a payload borrow is emitted only on exits not already
// dominated by such an end.
//
// Rules:
//   - rules/memory/borrowing.md — § 8.1(9)–(14)
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 49–50

// borrowEndMark records where a holder's borrow was ended: in block, before
// any block allocated from next onward existed.
type borrowEndMark struct {
	block BlockID
	next  BlockID
}

func lifetimeEndKey(token lexer.Token) string {
	return fmt.Sprintf("%s:%d:%d", token.File, token.Line, token.Column)
}

// lifetimeEndsBefore indexes Sema's last-required-use ends by the statement
// they precede, once per module.
func (b *builder) lifetimeEndsBefore() map[string][]sema.ReferenceLifetimeEnd {
	if b.borrowEnds != nil {
		return b.borrowEnds
	}
	b.borrowEnds = map[string][]sema.ReferenceLifetimeEnd{}
	seen := map[string]bool{}
	for _, end := range b.analyzer.ReferenceLifetimeEnds() {
		if end.Before.Line == 0 || end.Binding == 0 {
			continue
		}
		key := lifetimeEndKey(end.Before)
		unique := fmt.Sprintf("%s#%d", key, end.Binding)
		if seen[unique] {
			continue
		}
		seen[unique] = true
		b.borrowEnds[key] = append(b.borrowEnds[key], end)
	}
	// Sema releases several holders before one statement in map order; the
	// emitted markers follow binding identity so the module is deterministic.
	for _, ends := range b.borrowEnds {
		sort.Slice(ends, func(i, j int) bool { return ends[i].Binding < ends[j].Binding })
	}
	return b.borrowEnds
}

// enterStatementList opens a holder home list and adopts the bindings that
// were declared for it (a borrowed match payload's arm body).
func (fb *functionBuilder) enterStatementList() int {
	fb.statementLists++
	list := fb.statementLists
	for _, id := range fb.pendingHomes {
		fb.setHolderHome(id, list)
	}
	fb.pendingHomes = nil
	return list
}

func (fb *functionBuilder) setHolderHome(id sema.BindingID, list int) {
	if fb.holderHomes == nil {
		fb.holderHomes = map[sema.BindingID]int{}
	}
	fb.holderHomes[id] = list
}

// endBorrowsBefore emits the Sema-resolved ends that precede statement in
// list.
func (fb *functionBuilder) endBorrowsBefore(statement ast.Statement, list int) {
	if fb.current == nil {
		return
	}
	for _, end := range fb.owner.lifetimeEndsBefore()[lifetimeEndKey(statementToken(statement))] {
		if fb.holderHomes[end.Binding] != list {
			continue
		}
		held, ok := fb.bindings[end.Binding]
		if !ok || held.storage != 0 {
			continue
		}
		metadata := fb.references[held.value]
		if metadata == nil {
			continue
		}
		if _, ended := fb.borrowEnds[held.value]; ended {
			continue
		}
		fb.emit(Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{held.value}, Reference: cloneReferenceMetadata(metadata), Location: location(statementToken(statement))})
		if fb.borrowEnds == nil {
			fb.borrowEnds = map[ValueID]borrowEndMark{}
		}
		fb.borrowEnds[held.value] = borrowEndMark{block: fb.current.ID, next: fb.nextBlock}
	}
}

// endedBeforeExit reports an arm exit block already dominated by the
// Sema-resolved end of reference.
func (fb *functionBuilder) endedBeforeExit(reference ValueID, block BlockID) bool {
	mark, ok := fb.borrowEnds[reference]
	return ok && (block == mark.block || block >= mark.next)
}

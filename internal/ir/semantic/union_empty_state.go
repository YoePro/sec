package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// The compiler-known `empty` initialization state of a mutable binding of a
// NonDefaultable union (default_values.md § 33(4)–(5); unions.md §§ 7–8;
// semantic_ir.md § 23(4)). `empty` is not a union value, variant, `null` or
// `None`; it is the storage's initialization state before its first value.
//
//   - storage.init-empty $s: the declaration's initialization to `empty`;
//   - storage.is-empty $s: tests that state without reading a value, for
//     `is empty` and the `empty` match pattern.
//
// A load of the storage is valid only where it definitely holds a value.
const (
	OpStorageInitEmpty OpKind = "storage.init-empty"
	OpStorageIsEmpty   OpKind = "storage.is-empty"
)

// MatchPatternEmptyState names the `empty` arm; on the value path it never
// matches (flowcontrol_match.md § 11).
const MatchPatternEmptyState = string(sema.MatchPatternEmptyState)

func verifyEmptyStateOperation(types *TypeTable, op Operation, storages map[StorageID]Storage) error {
	storage := storages[op.Storage]
	if storage.ID == 0 || !storage.Mutable || !typeHasKind(types, storage.Type, TypeUnion) {
		return fmt.Errorf("%s requires mutable union storage", op.Kind)
	}
	if len(op.Operands) != 0 || len(op.Successors) != 0 {
		return fmt.Errorf("%s takes no operands", op.Kind)
	}
	switch op.Kind {
	case OpStorageInitEmpty:
		if len(op.Results) != 0 {
			return fmt.Errorf("storage.init-empty has no result")
		}
	case OpStorageIsEmpty:
		if len(op.Results) != 1 || !typeHasKind(types, op.Results[0].Type, TypeBool) {
			return fmt.Errorf("storage.is-empty produces bool")
		}
	}
	return nil
}

// verifyEmptyStorageReads proves that every value read of storage that began
// `empty` sees a value on every path: storage.store, storage.init and a Place
// initialization set the value state; the false edge of a storage.is-empty
// test refines to the value state and its true edge to empty.
func verifyEmptyStorageReads(fn *Function) error {
	const (
		stateEmpty = 1 << iota
		stateValue
	)
	tracked := map[StorageID]bool{}
	tests := map[ValueID]StorageID{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpStorageInitEmpty {
				tracked[op.Storage] = true
			}
			if op.Kind == OpStorageIsEmpty && len(op.Results) == 1 {
				tests[op.Results[0].ID] = op.Storage
			}
		}
	}
	if len(tracked) == 0 {
		return nil
	}
	type state map[StorageID]uint8
	entry := map[BlockID]state{}
	if len(fn.Blocks) == 0 {
		return nil
	}
	entry[fn.Blocks[0].ID] = state{}
	if fn.Entry != 0 {
		entry = map[BlockID]state{fn.Entry: {}}
	}
	clone := func(s state) state {
		copied := state{}
		for key, value := range s {
			copied[key] = value
		}
		return copied
	}
	apply := func(block *Block, s state, check bool) (state, error) {
		for index, op := range block.Operations {
			if !tracked[op.Storage] {
				continue
			}
			switch op.Kind {
			case OpStorageInitEmpty:
				s[op.Storage] = stateEmpty
			case OpStorageInit, OpStorageStore:
				s[op.Storage] = stateValue
			case OpStorageLoad:
				if check && s[op.Storage] != stateValue {
					return nil, fmt.Errorf("storage.load of $%d that may still be empty", op.Storage)
				}
			case OpPlaceStorageRoot:
				// A Place that initializes, writes or conditionally replaces
				// (destroying only what is owned) needs no value first.
				initializes := false
				if index+1 < len(block.Operations) && len(op.Results) == 1 {
					next := block.Operations[index+1]
					conditional := next.Kind == OpOwnReplacePlace && next.Ownership != nil && next.Ownership.IfOwned
					initializes = (next.Kind == OpOwnInitializePlace || next.Kind == OpPlaceWrite || conditional) &&
						len(next.Operands) > 0 && next.Operands[0] == op.Results[0].ID
				}
				if initializes {
					s[op.Storage] = stateValue
				} else if check && s[op.Storage] != stateValue {
					return nil, fmt.Errorf("Place of $%d that may still be empty", op.Storage)
				}
			}
		}
		return s, nil
	}
	successors := func(block *Block, s state) map[BlockID]state {
		out := map[BlockID]state{}
		if len(block.Operations) == 0 {
			return out
		}
		last := block.Operations[len(block.Operations)-1]
		for index, successor := range last.Successors {
			next := clone(s)
			if last.Kind == OpCondBranch && len(last.Operands) == 1 {
				if storage, test := tests[last.Operands[0]]; test {
					if index == 0 {
						next[storage] = stateEmpty
					} else {
						next[storage] = stateValue
					}
				}
			}
			out[successor.Block] = next
		}
		return out
	}
	blocks := map[BlockID]*Block{}
	for _, block := range fn.Blocks {
		blocks[block.ID] = block
	}
	for changed := true; changed; {
		changed = false
		for _, block := range fn.Blocks {
			in, reached := entry[block.ID]
			if !reached {
				continue
			}
			out, _ := apply(block, clone(in), false)
			for id, next := range successors(block, out) {
				target, seen := entry[id]
				if !seen {
					entry[id] = next
					changed = true
					continue
				}
				for storage, bits := range next {
					if target[storage]|bits != target[storage] {
						target[storage] |= bits
						changed = true
					}
				}
			}
		}
	}
	for _, block := range fn.Blocks {
		if in, reached := entry[block.ID]; reached {
			if _, err := apply(block, clone(in), true); err != nil {
				return err
			}
		}
	}
	return nil
}

// emptyStateStorage is the storage of a union binding that may be `empty`.
func (fb *functionBuilder) emptyStateStorage(subject ast.Expression) (StorageID, bool) {
	identifier, ok := subject.(*ast.Identifier)
	if !ok {
		return 0, false
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier)
	if !ok {
		return 0, false
	}
	bound, ok := fb.bindings[fact.ID]
	if !ok || bound.storage == 0 || !fb.emptyStorages[bound.storage] {
		return 0, false
	}
	return bound.storage, true
}

// declareEmptyUnion lowers `let mut p: U` of a NonDefaultable union: a
// storage that begins `empty` (default_values.md § 33(4)–(5)).
func (fb *functionBuilder) declareEmptyUnion(stmt *ast.LetStatement) error {
	if fb.owner.maxPackage < ownershipLoweringPackage {
		return fb.unsupported("mutable local declaration without initializer (empty union state)", stmt.Token)
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(stmt.Name)
	if !ok || !stmt.Mutable || fact.Type.Kind != sema.UnionType || fact.Type.Name == "Option" {
		return fb.unsupported("mutable local declaration without initializer", stmt.Token)
	}
	typ, err := fb.owner.internType(fact.Type)
	if err != nil {
		return err
	}
	// An owned union registers no cleanup while empty: its first assignment
	// initializes the storage and registers, and supplies the storage's
	// Place facts (destruction.md § 13.3; MD-125).
	owned := !fb.storageAllowed(typ)
	var place sema.Place
	id := fb.nextStorage
	fb.nextStorage++
	fb.fn.Storages = append(fb.fn.Storages, Storage{ID: id, Name: stmt.Name.Value, Type: typ, Mutable: true, Class: StorageLocalAutomatic, AddressTaken: owned, Location: location(stmt.Token)})
	fb.bindings[fact.ID] = binding{storage: id, typ: typ, mutable: true}
	if owned {
		if fb.ownedStorages == nil {
			fb.ownedStorages = map[sema.BindingID]ownedStorage{}
		}
		fb.ownedStorages[fact.ID] = ownedStorage{storage: id, place: place, typ: typ, scope: fb.currentCleanupScope()}
	}
	if fb.emptyStorages == nil {
		fb.emptyStorages = map[StorageID]bool{}
	}
	fb.emptyStorages[id] = true
	loc := location(stmt.Token)
	fb.emit(Operation{Kind: OpStorageDeclare, Storage: id, Location: loc})
	fb.emit(Operation{Kind: OpStorageInitEmpty, Storage: id, Location: loc})
	return nil
}

// buildEmptyStateTest lowers `p is empty` and `p is Variant` on a binding
// that may still be empty: the state is tested first, without a value read.
func (fb *functionBuilder) buildEmptyStateTest(expr *ast.StateTestExpression, fact sema.ResolvedStateTest) (builtValue, error) {
	boolType, err := fb.boolType()
	if err != nil {
		return builtValue{}, err
	}
	loc := location(expr.Token)
	if fact.StaticallyKnown {
		value := fact.Value
		return fb.result(Operation{Kind: OpConstBool, Bool: &value, Location: loc}, boolType), nil
	}
	storage, ok := fb.emptyStateStorage(expr.Subject)
	if !ok {
		return builtValue{}, fb.unsupported("empty-state test of a binding without empty-state storage", expr.Token)
	}
	empty := fb.result(Operation{Kind: OpStorageIsEmpty, Storage: storage, Location: loc}, boolType)
	if fact.Empty {
		if !fact.Negated {
			return empty, nil
		}
		return fb.result(Operation{Kind: OpBoolNot, Operands: []ValueID{empty.id}, Location: loc}, boolType), nil
	}
	// A variant test is false on the empty state; otherwise it tests the value.
	emptyBlock, valueBlock, merge := fb.newBlock(), fb.newBlock(), fb.newBlock()
	result := fb.newValue(boolType, OwnershipImmediate, loc)
	merge.Parameters = []Value{result}
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{empty.id}, Successors: []BranchTarget{{Block: emptyBlock.ID}, {Block: valueBlock.ID}}, Location: loc})
	fb.current = emptyBlock
	onEmpty := fact.Negated
	constant := fb.result(Operation{Kind: OpConstBool, Bool: &onEmpty, Location: loc}, boolType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{constant.id}}}, Location: loc})
	fb.current = valueBlock
	fact.MaybeEmpty = false
	tested, err := fb.buildValueStateTest(expr, fact)
	if err != nil {
		return builtValue{}, err
	}
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{tested.id}}}, Location: loc})
	fb.current = merge
	return builtValue{id: result.ID, typ: boolType}, nil
}

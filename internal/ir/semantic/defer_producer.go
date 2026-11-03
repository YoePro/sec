package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// buildDefer lowers an executed defer statement to cleanup.defer-register with an
// isolated body region. Captures are Places, never copies: an immutable binding
// through a read-only value root, a mutable one through its storage, so the body
// observes the value when it runs. Each execution registers one instance, and
// cleanup.run-function runs them in reverse order at the function's exits.
// Rules: rules/control-flow/defer.md — §§9–11; rules/mlir/lowering-versions/sec_mlir_lowering_v13.md — §§13–14;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§41–44,74,77.
func (fb *functionBuilder) buildDefer(stmt *ast.DeferStatement) error {
	plan, ok := fb.owner.analyzer.ResolvedDeferPlanOf(stmt)
	if !ok || fb.owner.maxPackage < ownershipLoweringPackage {
		return fb.unsupported("defer without a resolved plan", stmt.Token)
	}
	if fb.inDeferBody {
		return fb.unsupported("defer inside a defer body", stmt.Token)
	}
	if body, ok := fb.cleanupPlan(); !ok || len(body.Registrations) == 0 {
		return fb.unsupported("defer without a resolved cleanup plan", stmt.Token)
	}
	scope, found := fb.deferRegistrationScope(plan)
	if !found {
		return fb.unsupported("defer without a resolved cleanup registration", stmt.Token)
	}
	loc := location(stmt.Token)
	var operands []ValueID
	var parameters []Value
	saved := map[sema.BindingID]binding{}
	for _, capture := range plan.Captures {
		if capture.Binding == 0 {
			continue
		}
		if _, seen := saved[capture.Binding]; seen {
			continue
		}
		if capture.Kind != sema.DeferCaptureValueUse {
			return fb.unsupported("defer capturing a reference holder", capture.Use)
		}
		bound, known := fb.bindings[capture.Binding]
		// An owned value is captured as a Place like any other: the defer reads
		// it when it runs, before the owner's own cleanup, which registered
		// earlier (destruction.md § 8; defer.md §§ 9–11).
		_, tracked := fb.trackedActions[capture.Binding]
		if !known || !tracked && !fb.storageAllowed(bound.typ) {
			return fb.unsupported("defer capturing a value that needs destruction", capture.Use)
		}
		// An owner of an inner scope must outlive its scope until the defer
		// runs at function exit (defer.md § 8), once per registration in a loop.
		if root, ok := fb.cleanupPlan(); tracked && ok && fb.trackedScopes[capture.Binding] != root.Root {
			return fb.unsupported("P17 defer reading an owned value of an inner scope", capture.Use)
		}
		// A Place captures the slot, so every registration would read the last
		// iteration's instance instead of its own (defer.md § 13).
		if bound.storage != 0 && fb.loopStorages[bound.storage] {
			return fb.unsupported("P17 defer reading mutable storage declared in its loop", capture.Use)
		}
		root := capture.Place
		root.Projections = nil
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: bound.typ, Mutable: bound.storage != 0})
		var place builtValue
		if bound.storage != 0 {
			fb.markStorageAddressTaken(bound.storage)
			place = fb.placeResult(Operation{Kind: OpPlaceStorageRoot, Storage: bound.storage, Place: placeMetadataOf(root), Location: loc}, placeType)
		} else {
			place = fb.placeResult(Operation{Kind: OpPlaceValueRoot, Operands: []ValueID{bound.value}, Place: placeMetadataOf(root), Location: loc}, placeType)
		}
		if tracked {
			if fb.deferCaptured == nil {
				fb.deferCaptured = map[sema.BindingID]bool{}
			}
			fb.deferCaptured[capture.Binding] = true
		}
		operands = append(operands, place.id)
		parameter := fb.newValue(placeType, OwnershipCompilerTemporary, loc)
		parameters = append(parameters, parameter)
		saved[capture.Binding] = bound
		fb.bindings[capture.Binding] = binding{typ: bound.typ, capture: parameter.ID, mutable: bound.storage != 0}
	}
	restore := func() {
		for id, bound := range saved {
			fb.bindings[id] = bound
		}
	}
	storagesBefore := map[StorageID]bool{}
	for _, storage := range fb.fn.Storages {
		storagesBefore[storage.ID] = true
	}
	outerBlocks, outerCurrent, outerLoops := fb.fn.Blocks, fb.current, fb.loops
	fb.fn.Blocks, fb.loops, fb.inDeferBody = nil, nil, true
	entry := fb.newBlock()
	entry.Parameters = parameters
	fb.current = entry
	err := fb.buildScope(stmt.Body)
	if err == nil && fb.current != nil {
		fb.emit(Operation{Kind: OpCleanupDeferYield, Cleanup: &CleanupFacts{}, Location: loc})
	}
	region := &Region{Entry: entry.ID, Blocks: fb.fn.Blocks}
	if err == nil {
		err = fb.requireIsolatedDeferBody(region, stmt, storagesBefore)
	}
	fb.fn.Blocks, fb.current, fb.loops, fb.inDeferBody = outerBlocks, outerCurrent, outerLoops, false
	restore()
	if err != nil {
		return err
	}
	fb.cleanupOrdinal++
	fb.emit(Operation{Kind: OpCleanupDeferRegister, Operands: operands, Region: region, Cleanup: &CleanupFacts{
		Action: uint32(plan.Identity.Action), Scope: scope, Ordinal: fb.cleanupOrdinal, Defer: uint32(plan.Identity.Defer),
	}, Location: loc})
	return nil
}

// requireIsolatedDeferBody names a body that reached a value or storage outside its
// captures, for example through a form Sema does not publish as a capture yet.
func (fb *functionBuilder) requireIsolatedDeferBody(region *Region, stmt *ast.DeferStatement, storagesBefore map[StorageID]bool) error {
	defined := map[ValueID]bool{}
	for _, block := range region.Blocks {
		for _, parameter := range block.Parameters {
			defined[parameter.ID] = true
		}
		for _, op := range block.Operations {
			for _, result := range op.Results {
				defined[result.ID] = true
			}
		}
	}
	for _, block := range region.Blocks {
		for _, op := range block.Operations {
			for _, operand := range op.Operands {
				if !defined[operand] {
					return fb.unsupported("defer body using an outer value that is not a resolved capture", stmt.Token)
				}
			}
			if op.Storage != 0 && storagesBefore[op.Storage] {
				return fb.unsupported("defer body using outer storage that is not a resolved capture", stmt.Token)
			}
		}
	}
	return nil
}

// deferRegistrationScope is the lexical scope Sema registered this defer site in.
func (fb *functionBuilder) deferRegistrationScope(plan sema.ResolvedDeferPlan) (uint32, bool) {
	cleanup, ok := fb.cleanupPlan()
	if !ok {
		return 0, false
	}
	for _, registration := range cleanup.Registrations {
		if registration.Action == plan.Identity.Action {
			return uint32(registration.Scope), true
		}
	}
	return 0, false
}

func (fb *functionBuilder) markStorageAddressTaken(id StorageID) {
	for index := range fb.fn.Storages {
		if fb.fn.Storages[index].ID == id {
			fb.fn.Storages[index].AddressTaken = true
		}
	}
}

// readCapturedField reads a copy-trivial stored field, possibly nested, of a
// value a defer body captured, through its Place, never copying the whole value.
func (fb *functionBuilder) readCapturedField(member *ast.MemberExpression, plan sema.ResolvedStructMemberPlan) (builtValue, bool, error) {
	root, chain := memberRoot(member)
	if root == nil {
		return builtValue{}, false, nil
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(root)
	if !ok {
		return builtValue{}, false, nil
	}
	bound, ok := fb.bindings[fact.ID]
	if !ok || bound.capture == 0 {
		return builtValue{}, false, nil
	}
	place := builtValue{id: bound.capture}
	var fieldType TypeID
	for _, step := range chain {
		field, resolved := fb.owner.analyzer.ResolvedStructMemberOf(step)
		if !resolved || field.Kind != sema.MemberStoredField {
			return builtValue{}, true, fb.unsupported("defer body reading an unresolved captured field", step.Token)
		}
		var err error
		if fieldType, err = fb.owner.internType(field.MemberType); err != nil {
			return builtValue{}, true, err
		}
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: fieldType, Mutable: bound.mutable})
		place = fb.placeResult(Operation{Kind: OpPlaceField, Operands: []ValueID{place.id}, StructField: StructFieldID(field.FieldID), Location: location(step.Token)}, placeType)
	}
	return fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: location(member.Token)}, fieldType), true, nil
}

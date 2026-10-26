package semantic

import (
	"errors"
	"fmt"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// ownershipLoweringPackage is the package that admits P17 ownership and
// cleanup producers.
const ownershipLoweringPackage = 17

// buildCustomFree builds a resolved custom `free` as the type's
// destruction implementation: a non-callable lifecycle function over a
// mutable `self` receiver, identified by destruction plans (Semantic IR
// ownership amendment § 17; destruction.md §§ 15.2–15.7).
func (b *builder) buildCustomFree(impl *ast.ImplStatement, free *ast.FreeDeclaration) error {
	if free == nil || free.Body == nil {
		return nil
	}
	var resolved sema.Function
	found := false
	for _, function := range b.analyzer.Functions()[sema.CustomFreeFunctionName(impl.Target.Name)] {
		if function.CustomFree && sameToken(function.Token, free.Token) {
			resolved, found = function, true
		}
	}
	if !found {
		return fmt.Errorf("missing resolved custom free of %s", impl.Target.Name)
	}
	declaration := &ast.FunctionDeclaration{
		Token:      free.Token,
		Name:       &ast.Identifier{Token: free.Token, Value: "free"},
		ReturnType: &ast.TypeReference{Token: free.Token, Name: "void"},
		Body:       free.Body,
	}
	if err := b.buildResolvedFunction(declaration, resolved); err != nil {
		return err
	}
	if b.customFrees == nil {
		b.customFrees = map[string]FunctionID{}
	}
	b.customFrees[impl.Target.Name] = b.module.Functions[len(b.module.Functions)-1].ID
	return nil
}

func sameToken(left, right lexer.Token) bool {
	return left.File == right.File && left.Line == right.Line && left.Column == right.Column
}

// importDestructionPlans copies Sema's destruction plan graph for a value of
// type typ into the module table and returns the root plan's identity. Types
// come from the destroyed value: a struct step destroys the field at its
// declared index, an array step the element. Variant and resource plans
// belong to later P17 slices and are rejected explicitly.
func (b *builder) importDestructionPlans(set sema.DestructionPlanSet, typ sema.Type) (string, error) {
	imported := map[string]bool{}
	for _, plan := range b.module.DestructionPlans {
		imported[plan.ID] = true
	}
	var add func(id sema.DestructionPlanID, typ sema.Type) error
	add = func(id sema.DestructionPlanID, typ sema.Type) error {
		if imported[string(id)] {
			return nil
		}
		plan, ok := set.Plan(id)
		if !ok {
			return fmt.Errorf("destruction plan %q is missing from its plan set", id)
		}
		typeID, err := b.internType(typ)
		if err != nil {
			return err
		}
		record := DestructionPlanRecord{ID: string(id), Type: typeID, Kind: string(plan.Kind)}
		switch plan.Kind {
		case sema.DestructionTrivial:
		case sema.DestructionStruct:
			record.CustomFree = plan.CustomFree
			if plan.CustomFree {
				record.FreeType = typ.Name
			}
			for _, step := range plan.Fields {
				if step.Index < 0 || step.Index >= len(typ.Fields) {
					return fmt.Errorf("destruction plan %q steps to unknown field %s", id, step.Field)
				}
				if err := add(step.Plan, typ.Fields[step.Index].Type); err != nil {
					return err
				}
				record.Fields = append(record.Fields, DestructionFieldRecord{Field: StructFieldID(step.Index), Plan: string(step.Plan)})
			}
		case sema.DestructionFixedArray:
			if typ.Element == nil {
				return fmt.Errorf("fixed-array destruction plan %q has no element type", id)
			}
			if err := add(plan.Element, *typ.Element); err != nil {
				return err
			}
			record.Element, record.ElementCount = string(plan.Element), plan.ElementCount
		default:
			return &UnsupportedFeatureError{Feature: "P17 " + string(plan.Kind) + " destruction plan (later P17 producer slice)", Package: b.maxPackage}
		}
		imported[string(id)] = true
		b.module.DestructionPlans = append(b.module.DestructionPlans, record)
		return nil
	}
	if err := add(set.Root, typ); err != nil {
		return "", err
	}
	return string(set.Root), nil
}

// bindCustomFrees gives every custom-free destruction plan the identity of
// its built lifecycle function, once all impls are built.
func (b *builder) bindCustomFrees() error {
	for index := range b.module.DestructionPlans {
		plan := &b.module.DestructionPlans[index]
		if !plan.CustomFree || plan.FreeFunction != "" {
			continue
		}
		free, ok := b.customFrees[plan.FreeType]
		if !ok {
			return fmt.Errorf("destruction plan %q has no built custom free for %s", plan.ID, plan.FreeType)
		}
		plan.FreeFunction = free
	}
	return nil
}

// trackOwnedLocal registers an owned local after its successful
// initialization (package17 §§ 34, 38; Sema ResolvedOwnedCleanupInitializationOf).
// Locals of nested and loop scopes are tracked the same way; their scope
// exits run cleanup.run-scope (P17-42).
func (fb *functionBuilder) trackOwnedLocal(stmt *ast.LetStatement, registration sema.ResolvedOwnedCleanupRegistration, value builtValue, typ sema.Type) error {
	return fb.trackOwned(stmt.Token, registration, value, typ)
}

// trackOwnedParameters registers the function's owned by-value parameters
// on entry (Sema's at-function-entry registrations), before the body runs.
func (fb *functionBuilder) trackOwnedParameters(decl *ast.FunctionDeclaration) error {
	if fb.owner.maxPackage < ownershipLoweringPackage || fb.resolved == nil {
		return nil
	}
	for index, parameter := range fb.resolved.Parameters {
		if index >= len(decl.Parameters) || decl.Parameters[index].Name == nil {
			continue
		}
		fact, ok := fb.owner.analyzer.ResolvedBindingOf(decl.Parameters[index].Name)
		if !ok {
			continue
		}
		registration, owned := fb.owner.analyzer.ResolvedOwnedCleanupParameterOf(fact.ID)
		if !owned {
			continue
		}
		bound, ok := fb.bindings[fact.ID]
		if !ok || bound.storage != 0 {
			return fb.unsupported("owned parameter without an SSA value", parameter.Token)
		}
		if err := fb.trackOwned(parameter.Token, registration, builtValue{id: bound.value, typ: bound.typ}, parameter.Type); err != nil {
			return err
		}
	}
	return nil
}

// trackOwned emits cleanup.track-owned for one owned value's registration.
func (fb *functionBuilder) trackOwned(token lexer.Token, registration sema.ResolvedOwnedCleanupRegistration, value builtValue, typ sema.Type) error {
	if _, ok := fb.cleanupPlan(); !ok {
		return fb.unsupported("owned value without a resolved cleanup plan", token)
	}
	planID, err := fb.owner.importDestructionPlans(registration.Destruction, typ)
	if err != nil {
		var unsupported *UnsupportedFeatureError
		if errors.As(err, &unsupported) && unsupported.Location.Line == 0 {
			unsupported.Location = location(token)
		}
		return err
	}
	fb.cleanupOrdinal++
	if fb.trackedActions == nil {
		fb.trackedActions = map[sema.BindingID]uint32{}
	}
	fb.trackedActions[registration.Binding] = uint32(registration.Action)
	fb.emit(Operation{Kind: OpCleanupTrackOwned, Operands: []ValueID{value.id}, Cleanup: &CleanupFacts{
		Action: uint32(registration.Action), Scope: uint32(registration.Scope), Ordinal: fb.cleanupOrdinal, DestructionPlan: planID,
	}, Location: location(token)})
	return nil
}

// cleanupPlan is Sema's cleanup plan of the function being built.
func (fb *functionBuilder) cleanupPlan() (sema.ResolvedCleanupPlan, bool) {
	if fb.resolved == nil {
		return sema.ResolvedCleanupPlan{}, false
	}
	return fb.owner.analyzer.ResolvedCleanupPlanOf(*fb.resolved)
}

// runFunctionCleanup inserts cleanup.run-function before every return of a
// function that tracked owned values, with the exit kind Sema resolved for
// that exit. Exits this slice cannot represent are rejected.
func (fb *functionBuilder) runFunctionCleanup(decl *ast.FunctionDeclaration) error {
	if fb.cleanupOrdinal == 0 {
		return nil
	}
	plan, ok := fb.cleanupPlan()
	if !ok {
		return fb.unsupported("owned locals without a resolved cleanup plan", decl.Token)
	}
	exits := map[sourceKey]string{}
	fallthroughExit := false
	for _, exit := range plan.Exits {
		if !exit.FunctionExit && exit.Kind != sema.CleanupExitPanic {
			// Scope exits run cleanup.run-scope (verifyCleanupExitsEmitted).
			continue
		}
		switch exit.Kind {
		case sema.CleanupExitReturn, sema.CleanupExitErrorPropagation:
			// The propagated error leaves before cleanup runs (§ 40).
			exits[sourceKey{exit.Source.Line, exit.Source.Column}] = string(exit.Kind)
		case sema.CleanupExitFallthrough:
			fallthroughExit = true
		case sema.CleanupExitPanic:
		default:
			return fb.unsupported("P17 "+string(exit.Kind)+" exit with owned locals (later P17 producer slice)", exit.Source)
		}
	}
	for _, block := range fb.fn.Blocks {
		for index := 0; index < len(block.Operations); index++ {
			op := block.Operations[index]
			if op.Kind != OpReturn {
				continue
			}
			key := sourceKey{op.Location.Line, op.Location.Column}
			if alias, aliased := fb.propagationReturns[key]; aliased {
				key = alias
			}
			exit, found := exits[key]
			if !found {
				if !fallthroughExit || op.Location.Line != decl.Token.Line || op.Location.Column != decl.Token.Column {
					return &UnsupportedFeatureError{Feature: "function exit without a resolved cleanup exit", Package: fb.owner.maxPackage, Location: op.Location}
				}
				exit = string(sema.CleanupExitFallthrough)
			}
			run := Operation{Kind: OpCleanupRunFunction, Cleanup: &CleanupFacts{Exit: exit}, Location: op.Location}
			block.Operations = append(block.Operations[:index], append([]Operation{run}, block.Operations[index:]...)...)
			index++
		}
	}
	return fb.verifyCleanupExitsEmitted(plan)
}

// cleanupExitKey identifies one Sema cleanup exit.
type cleanupExitKey struct {
	kind   sema.CleanupExitKind
	line   int
	column int
}

// scopeCleanupExit is Sema's exit of kind at source when it leaves a scope
// holding owned registrations; only such exits run scope cleanup, so programs
// without owned locals keep their predecessor IR.
func (fb *functionBuilder) scopeCleanupExit(kind sema.CleanupExitKind, source lexer.Token) (sema.ResolvedCleanupExit, bool) {
	plan, ok := fb.cleanupPlan()
	if !ok || fb.owner.maxPackage < ownershipLoweringPackage {
		return sema.ResolvedCleanupExit{}, false
	}
	registered := map[sema.CleanupScopeID]bool{}
	for _, registration := range plan.Registrations {
		registered[registration.Scope] = true
	}
	for _, exit := range plan.Exits {
		if exit.Kind != kind || exit.FunctionExit || !exit.RunsCleanup || exit.Source.Line != source.Line || exit.Source.Column != source.Column {
			continue
		}
		for _, scope := range exit.Exited {
			if registered[scope] {
				return exit, true
			}
		}
	}
	return sema.ResolvedCleanupExit{}, false
}

// runScopeCleanup emits cleanup.run-scope for Sema's exit of kind at source
// before control leaves the exited scopes (package17 §§ 39–40, 44).
func (fb *functionBuilder) runScopeCleanup(kind sema.CleanupExitKind, source lexer.Token) {
	exit, ok := fb.scopeCleanupExit(kind, source)
	if !ok || fb.current == nil {
		return
	}
	scopes := make([]uint32, len(exit.Exited))
	for index, scope := range exit.Exited {
		scopes[index] = uint32(scope)
	}
	fb.emit(Operation{Kind: OpCleanupRunScope, Cleanup: &CleanupFacts{Scopes: scopes, Exit: string(kind)}, Location: location(source)})
	if fb.cleanupExitsDone == nil {
		fb.cleanupExitsDone = map[cleanupExitKey]bool{}
	}
	fb.cleanupExitsDone[cleanupExitKey{kind, source.Line, source.Column}] = true
}

// buildScope builds a lexical block and, when control falls out of it, runs
// the cleanup of the scope it leaves.
func (fb *functionBuilder) buildScope(block *ast.BlockStatement) error {
	if block == nil {
		return nil
	}
	defer fb.enterBuildScope(block)()
	if err := fb.buildStatements(block.Statements); err != nil {
		return err
	}
	fb.runScopeCleanup(sema.CleanupExitFallthrough, block.Token)
	return nil
}

// verifyCleanupExitsEmitted rejects a function in which a scope exit that
// must run owned-value cleanup was not lowered through a cleanup-aware path
// (for example a value-producing handler block), instead of dropping it.
func (fb *functionBuilder) verifyCleanupExitsEmitted(plan sema.ResolvedCleanupPlan) error {
	registered := map[sema.CleanupScopeID]bool{}
	for _, registration := range plan.Registrations {
		registered[registration.Scope] = true
	}
	for _, exit := range plan.Exits {
		if exit.FunctionExit || !exit.RunsCleanup {
			continue
		}
		relevant := false
		for _, scope := range exit.Exited {
			relevant = relevant || registered[scope]
		}
		if relevant && !fb.cleanupExitsDone[cleanupExitKey{exit.Kind, exit.Source.Line, exit.Source.Column}] {
			return fb.unsupported("P17 "+string(exit.Kind)+" scope exit with owned locals outside a cleanup-aware block (later P17 producer slice)", exit.Source)
		}
	}
	return nil
}

type sourceKey struct{ line, column int }

// moveOwnedLocal lowers Sema's move of a whole tracked owned local out of
// its binding (a return or `let x :<- h`): own.move consumes the value and
// cleanup.cancel ends the binding's registration, so scope and function
// cleanup no longer destroy it (package17 §§ 38, 40, 55). It reports false
// when the identifier is not such a move.
func (fb *functionBuilder) moveOwnedLocal(identifier *ast.Identifier, binding sema.BindingID, value builtValue) (builtValue, bool, error) {
	if fb.owner.maxPackage < ownershipLoweringPackage {
		return builtValue{}, false, nil
	}
	plan, ok := fb.owner.analyzer.ResolvedTransferPlanOf(identifier)
	if !ok || !ownedLocalMoveLowerable(plan) {
		return builtValue{}, false, nil
	}
	action, tracked := fb.trackedActions[binding]
	if !tracked {
		return builtValue{}, true, fb.unsupported("P17 move of an owned value without a tracked registration", identifier.Token)
	}
	token := identifier.Token
	if len(fb.owner.analyzer.ResolvedOwnedCleanupCancellationsAt(token.File, token.Line, token.Column)) == 0 {
		return builtValue{}, true, fb.unsupported("P17 move without a resolved cleanup cancellation", token)
	}
	loc := location(token)
	moved := fb.result(Operation{Kind: OpOwnMove, Operands: []ValueID{value.id}, Ownership: &OwnershipFacts{}, Location: loc}, value.typ)
	reason := "moved"
	if plan.Site == sema.TransferSiteReturn || plan.Site == sema.TransferSiteArgument {
		reason = "transferred"
	}
	cancel := Operation{Kind: OpCleanupCancel, Cleanup: &CleanupFacts{Action: action, Reason: reason}, Location: loc}
	if plan.Site == sema.TransferSiteArgument {
		// The call emits it after preparing every argument.
		fb.pendingArgumentCancels = append(fb.pendingArgumentCancels, cancel)
		return moved, true, nil
	}
	fb.emit(cancel)
	return moved, true, nil
}

// notePropagationReturn records that a propagation return emitted at loc
// belongs to the bodyless try being built, whose Sema exit is keyed by the
// try keyword.
func (fb *functionBuilder) notePropagationReturn(loc Location) {
	if fb.propagatingTry.Line == 0 {
		return
	}
	if fb.propagationReturns == nil {
		fb.propagationReturns = map[sourceKey]sourceKey{}
	}
	fb.propagationReturns[sourceKey{loc.Line, loc.Column}] = sourceKey{fb.propagatingTry.Line, fb.propagatingTry.Column}
}

// ownedArgumentLowerable reports an argument needing destruction that the
// P17 producers pass by move: a moved whole owned local (`<-h`, own.move +
// cleanup.cancel) or a fresh temporary forwarded into the parameter.
func (fb *functionBuilder) ownedArgumentLowerable(argument ast.Expression) bool {
	if fb.owner.maxPackage < ownershipLoweringPackage {
		return false
	}
	switch argument := argument.(type) {
	case *ast.Identifier:
		plan, ok := fb.owner.analyzer.ResolvedTransferPlanOf(argument)
		return ok && ownedLocalMoveLowerable(plan) && plan.Site == sema.TransferSiteArgument
	case *ast.PrefixExpression:
		return argument.Operator == "<-" && fb.ownedArgumentLowerable(argument.Right)
	case *ast.CallExpression, *ast.StructLiteral:
		return true
	}
	return false
}

// ownedStorage is a mutable owned local held in address-taken storage and
// accessed through its storage Place (P17-28, P17-36).
type ownedStorage struct {
	storage StorageID
	place   sema.Place
	typ     TypeID
	// scope is the declaring cleanup scope: re-registration there reaches
	// every later use, so the live incarnation stays statically known.
	scope sema.CleanupScopeID
}

// declareOwnedStorage lowers `let mut x := value` of an owned value:
// storage.declare, then own.initialize-place into the storage Place and
// cleanup.track-owned of that Place (package17 §§ 28, 34, 54).
func (fb *functionBuilder) declareOwnedStorage(stmt *ast.LetStatement, bindingID sema.BindingID, registration sema.ResolvedOwnedCleanupRegistration, value builtValue, typ sema.Type) error {
	id := fb.nextStorage
	fb.nextStorage++
	fb.fn.Storages = append(fb.fn.Storages, Storage{ID: id, Name: stmt.Name.Value, Type: value.typ, Mutable: true, Class: StorageLocalAutomatic, AddressTaken: true, Location: location(stmt.Token)})
	fb.bindings[bindingID] = binding{storage: id, typ: value.typ, mutable: true}
	if fb.ownedStorages == nil {
		fb.ownedStorages = map[sema.BindingID]ownedStorage{}
	}
	fb.ownedStorages[bindingID] = ownedStorage{storage: id, place: registration.Place, typ: value.typ, scope: fb.currentCleanupScope()}
	fb.emit(Operation{Kind: OpStorageDeclare, Storage: id, Location: location(stmt.Token)})
	place := fb.ownedStoragePlace(fb.ownedStorages[bindingID], stmt.Token)
	fb.emit(Operation{Kind: OpOwnInitializePlace, Operands: []ValueID{place.id, value.id}, Ownership: &OwnershipFacts{NewCleanup: uint32(registration.Action)}, Location: location(stmt.Token)})
	return fb.trackOwned(stmt.Token, registration, place, typ)
}

// ownedStoragePlace forms the writable storage-root Place of an owned local.
func (fb *functionBuilder) ownedStoragePlace(owned ownedStorage, token lexer.Token) builtValue {
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: owned.typ, Mutable: true})
	return fb.placeResult(Operation{Kind: OpPlaceStorageRoot, Storage: owned.storage, Place: placeMetadataOf(owned.place), Location: location(token)}, placeType)
}

// readOwnedStorage reads an owned mutable local as a whole value: only a
// resolved whole-local move is legal (own.move-from-place + cancel); a copy
// of a value needing destruction does not exist.
func (fb *functionBuilder) readOwnedStorage(identifier *ast.Identifier, binding sema.BindingID, owned ownedStorage) (builtValue, error) {
	plan, ok := fb.owner.analyzer.ResolvedTransferPlanOf(identifier)
	if !ok || !ownedLocalMoveLowerable(plan) {
		return builtValue{}, fb.unsupported("P17 whole-value read of owned mutable storage", identifier.Token)
	}
	action, tracked := fb.trackedActions[binding]
	if !tracked {
		return builtValue{}, fb.unsupported("P17 move of an owned value without a tracked registration", identifier.Token)
	}
	place := fb.ownedStoragePlace(owned, identifier.Token)
	moved := fb.result(Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{place.id}, Ownership: &OwnershipFacts{}, Location: location(identifier.Token)}, owned.typ)
	reason := "moved"
	if plan.Site == sema.TransferSiteReturn || plan.Site == sema.TransferSiteArgument {
		reason = "transferred"
	}
	cancel := Operation{Kind: OpCleanupCancel, Cleanup: &CleanupFacts{Action: action, Reason: reason}, Location: location(identifier.Token)}
	if plan.Site == sema.TransferSiteArgument {
		fb.pendingArgumentCancels = append(fb.pendingArgumentCancels, cancel)
	} else {
		fb.emit(cancel)
	}
	return moved, nil
}

// readOwnedStorageField reads a copy-trivial stored field of an owned
// mutable local through its Place, never copying the whole value.
func (fb *functionBuilder) readOwnedStorageField(member *ast.MemberExpression, owned ownedStorage, plan sema.ResolvedStructMemberPlan) (builtValue, error) {
	identifier := member.Object.(*ast.Identifier)
	fieldType, err := fb.owner.internType(plan.MemberType)
	if err != nil {
		return builtValue{}, err
	}
	place := fb.ownedStoragePlace(owned, identifier.Token)
	fieldPlaceType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: fieldType, Mutable: true})
	fieldPlace := fb.placeResult(Operation{Kind: OpPlaceField, Operands: []ValueID{place.id}, StructField: StructFieldID(plan.FieldID), Place: placeMetadataOf(owned.place), Location: location(member.Token)}, fieldPlaceType)
	return fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{fieldPlace.id}, Location: location(member.Token)}, fieldType), nil
}

// assignOwnedStorage lowers `x = value` of an owned mutable local:
// replacement of an available old value destroys it (own.replace-place with
// its plan and the old -> new cleanup incarnation) and re-registers the new
// one; reinitialization after a move initializes and registers
// (package17 §§ 31–33, 55; destruction.md § 13).
func (fb *functionBuilder) assignOwnedStorage(stmt *ast.AssignmentStatement, binding sema.BindingID, owned ownedStorage, assignment sema.ResolvedOwnedCleanupAssignment, typ sema.Type) error {
	if fb.currentCleanupScope() != owned.scope {
		// The incarnation would then differ by path (P17-19 path-exact
		// registration matching).
		return fb.unsupported("P17 owned reassignment outside its declaring scope (path-dependent cleanup incarnation)", stmt.Token)
	}
	value, err := fb.buildExpr(stmt.Value, owned.typ)
	if err != nil {
		return err
	}
	place := fb.ownedStoragePlace(owned, stmt.Token)
	registration := assignment.Registration
	switch assignment.Previous {
	case sema.CleanupPreviousAvailable:
		old, tracked := fb.trackedActions[binding]
		if !tracked {
			return fb.unsupported("P17 replacement of an untracked owned value", stmt.Token)
		}
		planID, err := fb.owner.importDestructionPlans(registration.Destruction, typ)
		if err != nil {
			return err
		}
		fb.emit(Operation{Kind: OpOwnReplacePlace, Operands: []ValueID{place.id, value.id}, Ownership: &OwnershipFacts{DestructionPlan: planID, OldCleanup: old, NewCleanup: uint32(registration.Action)}, Location: location(stmt.Token)})
		fb.emit(Operation{Kind: OpCleanupCancel, Cleanup: &CleanupFacts{Action: old, Reason: "replaced"}, Location: location(stmt.Token)})
	case sema.CleanupPreviousUnavailable:
		fb.emit(Operation{Kind: OpOwnInitializePlace, Operands: []ValueID{place.id, value.id}, Ownership: &OwnershipFacts{NewCleanup: uint32(registration.Action)}, Location: location(stmt.Token)})
	default:
		return fb.unsupported("P17 "+string(assignment.Previous)+" owned assignment (path/subobject plans)", stmt.Token)
	}
	return fb.trackOwned(stmt.Token, registration, place, typ)
}

// ownedStorageOf returns the owned mutable local an expression names.
func (fb *functionBuilder) ownedStorageOf(expr ast.Expression) (ownedStorage, bool) {
	identifier, ok := expr.(*ast.Identifier)
	if !ok || len(fb.ownedStorages) == 0 {
		return ownedStorage{}, false
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier)
	if !ok {
		return ownedStorage{}, false
	}
	owned, ok := fb.ownedStorages[fact.ID]
	return owned, ok
}

// assignOwnedStorageTarget lowers an assignment whose target is an owned
// mutable local; it reports false for every other assignment.
func (fb *functionBuilder) assignOwnedStorageTarget(stmt *ast.AssignmentStatement) (bool, error) {
	if fb.owner.maxPackage < ownershipLoweringPackage {
		return false, nil
	}
	assignment, ok := fb.owner.analyzer.ResolvedOwnedCleanupAssignmentOf(stmt)
	if !ok {
		return false, nil
	}
	identifier, ok := stmt.Target.(*ast.Identifier)
	if !ok {
		return true, fb.unsupported("P17 owned assignment to a projected Place", stmt.Token)
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier)
	if !ok {
		return true, fmt.Errorf("missing resolved binding for %s", identifier.Value)
	}
	owned, ok := fb.ownedStorages[fact.ID]
	if !ok {
		return true, fb.unsupported("P17 owned assignment to a binding without owned storage", stmt.Token)
	}
	return true, fb.assignOwnedStorage(stmt, fact.ID, owned, assignment, fact.Type)
}

// currentCleanupScope is the innermost lexical cleanup scope being built, or
// the function's root scope.
func (fb *functionBuilder) currentCleanupScope() sema.CleanupScopeID {
	if len(fb.cleanupScopes) > 0 {
		return fb.cleanupScopes[len(fb.cleanupScopes)-1]
	}
	plan, ok := fb.cleanupPlan()
	if !ok {
		return 0
	}
	return plan.Root
}

// enterBuildScope pushes a block's cleanup scope while it is built; the
// returned function pops it.
func (fb *functionBuilder) enterBuildScope(block *ast.BlockStatement) func() {
	scope, ok := fb.owner.analyzer.ResolvedCleanupScopeOf(block)
	if !ok {
		return func() {}
	}
	fb.cleanupScopes = append(fb.cleanupScopes, scope)
	return func() { fb.cleanupScopes = fb.cleanupScopes[:len(fb.cleanupScopes)-1] }
}

package semantic

import (
	"errors"
	"fmt"

	"sec/internal/ast"
	"sec/internal/ast/walk"
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
	// A named-field payload's struct plan comes from its own plan set.
	sets := []sema.DestructionPlanSet{set}
	var add func(id sema.DestructionPlanID, typ sema.Type) error
	add = func(id sema.DestructionPlanID, typ sema.Type) error {
		if imported[string(id)] {
			return nil
		}
		var plan sema.ResolvedDestructionPlan
		ok := false
		for _, candidate := range sets {
			if plan, ok = candidate.Plan(id); ok {
				break
			}
		}
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
		case sema.DestructionVariant:
			// A union's custom free runs before its active payload is
			// destroyed, as a struct's runs before its fields (destruction.md § 15.5).
			record.CustomFree = plan.CustomFree
			if plan.CustomFree {
				record.FreeType = typ.Name
			}
			for _, step := range plan.Variants {
				if len(step.Payload) != 0 && step.Payload[0].Field != "" {
					// A named-field payload is destroyed as its synthetic
					// payload struct, the same object its payload Place denotes.
					if step.Index >= len(typ.UnionVariants) {
						return fmt.Errorf("destruction plan %q steps to an unknown variant", id)
					}
					payload := sema.FieldPayloadType(typ, typ.UnionVariants[step.Index])
					payloadPlans := sema.ResolveDestructionPlans(payload)
					sets = append(sets, payloadPlans)
					if err := add(payloadPlans.Root, payload); err != nil {
						return err
					}
					record.Variants = append(record.Variants, DestructionVariantRecord{Variant: UnionVariantIndex(step.Index), Plan: string(payloadPlans.Root)})
					continue
				}
				payload, ok := variantPayloadOf(typ, step)
				if !ok {
					return &UnsupportedFeatureError{Feature: "P17 named-field variant destruction plan", Package: b.maxPackage}
				}
				if err := add(step.Payload[0].Plan, payload); err != nil {
					return err
				}
				record.Variants = append(record.Variants, DestructionVariantRecord{Variant: UnionVariantIndex(step.Index), Plan: string(step.Payload[0].Plan)})
			}
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

// variantPayloadOf is the one whole payload type a variant plan step destroys.
// A named-field variant steps per field and has no whole-payload plan shape.
func variantPayloadOf(typ sema.Type, step sema.DestructionVariantStep) (sema.Type, bool) {
	if len(step.Payload) != 1 || step.Payload[0].Field != "" {
		return sema.Type{}, false
	}
	if typ.Kind == sema.ResultType {
		if step.Index >= len(typ.TypeArgs) {
			return sema.Type{}, false
		}
		return typ.TypeArgs[step.Index], true
	}
	if step.Index >= len(typ.UnionVariants) || typ.UnionVariants[step.Index].Payload == nil {
		return sema.Type{}, false
	}
	return *typ.UnionVariants[step.Index].Payload, true
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
		if fb.partialRoots[fact.ID] {
			if err := fb.declareOwnedStorageAt(parameter.Name, parameter.Token, fact.ID, registration, builtValue{id: bound.value, typ: bound.typ}, parameter.Type); err != nil {
				return err
			}
			continue
		}
		if err := fb.trackOwned(parameter.Token, registration, builtValue{id: bound.value, typ: bound.typ}, parameter.Type); err != nil {
			return err
		}
	}
	return nil
}

// trackOwned emits cleanup.track-owned for one owned value's registration.
func (fb *functionBuilder) trackOwned(token lexer.Token, registration sema.ResolvedOwnedCleanupRegistration, value builtValue, typ sema.Type) error {
	if fb.inDeferBody {
		return fb.unsupported("an owned local inside a defer body", token)
	}
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
	// An owner a defer reads outlives its inner scope until the defer runs at
	// function exit, so it registers in the function scope (defer.md § 8). In
	// a loop it would need one instance per iteration; buildDefer stops there.
	if root, ok := fb.cleanupPlan(); ok && fb.deferReadOwners[registration.Binding] && len(fb.loops) == 0 {
		registration.Scope = root.Root
	}
	fb.cleanupOrdinal++
	if fb.trackedActions == nil {
		fb.trackedActions = map[sema.BindingID]uint32{}
		fb.trackedScopes = map[sema.BindingID]sema.CleanupScopeID{}
	}
	fb.trackedActions[registration.Binding] = uint32(registration.Action)
	fb.trackedScopes[registration.Binding] = registration.Scope
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
	var scopes []uint32
	for _, scope := range exit.Exited {
		if !fb.scopesCleaned[scope] {
			scopes = append(scopes, uint32(scope))
		}
	}
	if len(scopes) != 0 {
		fb.emit(Operation{Kind: OpCleanupRunScope, Cleanup: &CleanupFacts{Scopes: scopes, Exit: string(kind)}, Location: location(source)})
	}
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
	if isBorrowValueType(fb.owner.module, value.typ) {
		// A direct reference or slice holder moves on its P15/P16 path, which
		// keeps its reference facts; it owns no referent (P17 § 27).
		return builtValue{}, false, nil
	}
	if sema.TriviallyDestructible(plan.Source.Type) {
		// E.g. a whole array of mutable borrow holders moves ownership but
		// has no referent-destruction registration to cancel (P17 §15).
		return fb.result(Operation{Kind: OpOwnMove, Operands: []ValueID{value.id}, Ownership: &OwnershipFacts{}, Location: location(identifier.Token)}, value.typ), true, nil
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
	move := Operation{Kind: OpOwnMove, Operands: []ValueID{value.id}, Ownership: &OwnershipFacts{}, Location: loc}
	reason := "moved"
	if plan.Site == sema.TransferSiteReturn || plan.Site == sema.TransferSiteArgument {
		reason = "transferred"
	}
	cancels := fb.ownedCancels(binding, action, reason, false, loc)
	if plan.Site == sema.TransferSiteArgument {
		// The argument's consumption commits with the call, after every
		// argument is prepared: a failing later argument leaves the owner
		// unmoved and registered (destruction.md § 9.2(3); P17-23).
		moved := fb.deferredResult(move, value.typ)
		fb.pendingArgumentCancels = append(fb.pendingArgumentCancels, cancels...)
		return moved, true, nil
	}
	moved := fb.result(move, value.typ)
	fb.emitAll(cancels)
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
		// `return Some(x)` moves x into the returned payload (site return).
		return ok && ownedLocalMoveLowerable(plan) &&
			(plan.Site == sema.TransferSiteArgument || plan.Site == sema.TransferSiteConstruction || plan.Site == sema.TransferSiteReturn)
	case *ast.PrefixExpression:
		return argument.Operator == "<-" && fb.ownedArgumentLowerable(argument.Right)
	case *ast.CallExpression, *ast.StructLiteral:
		return true
	case *ast.TryExpression:
		// The success value of a try over a fresh call is itself fresh.
		_, call := argument.Expression.(*ast.CallExpression)
		return call
	case *ast.MemberExpression:
		plan, ok := fb.owner.analyzer.ResolvedTransferPlanOf(argument)
		return ok && fieldMoveLowerable(plan)
	}
	return false
}

// freshArgument reports an argument expression that creates a new owned value,
// which a failing later argument must destroy (P17-16), unlike a moved owner
// whose cancellation waits for the call (destruction.md § 9.2(3)).
func freshArgument(argument ast.Expression) bool {
	switch argument := argument.(type) {
	case *ast.CallExpression, *ast.StructLiteral:
		return true
	case *ast.TryExpression:
		_, call := argument.Expression.(*ast.CallExpression)
		return call
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
	return fb.declareOwnedStorageAt(stmt.Name.Value, stmt.Token, bindingID, registration, value, typ)
}

// declareOwnedStorageAt holds an owned value in address-taken storage. Besides
// `let mut`, an owner whose field moves out lives there, since its remaining
// fields keep their own state (package17 §§ 12, 59).
func (fb *functionBuilder) declareOwnedStorageAt(name string, token lexer.Token, bindingID sema.BindingID, registration sema.ResolvedOwnedCleanupRegistration, value builtValue, typ sema.Type) error {
	id := fb.nextStorage
	fb.nextStorage++
	fb.fn.Storages = append(fb.fn.Storages, Storage{ID: id, Name: name, Type: value.typ, Mutable: true, Class: StorageLocalAutomatic, AddressTaken: true, Location: location(token)})
	fb.bindings[bindingID] = binding{storage: id, typ: value.typ, mutable: true}
	if fb.ownedStorages == nil {
		fb.ownedStorages = map[sema.BindingID]ownedStorage{}
	}
	fb.ownedStorages[bindingID] = ownedStorage{storage: id, place: registration.Place, typ: value.typ, scope: fb.currentCleanupScope()}
	fb.emit(Operation{Kind: OpStorageDeclare, Storage: id, Location: location(token)})
	place := fb.ownedStoragePlace(fb.ownedStorages[bindingID], token)
	fb.emit(Operation{Kind: OpOwnInitializePlace, Operands: []ValueID{place.id, value.id}, Ownership: &OwnershipFacts{NewCleanup: uint32(registration.Action)}, Location: location(token)})
	return fb.trackOwned(token, registration, place, typ)
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
	move := Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{place.id}, Ownership: &OwnershipFacts{}, Location: location(identifier.Token)}
	reason := "moved"
	if plan.Site == sema.TransferSiteReturn || plan.Site == sema.TransferSiteArgument {
		reason = "transferred"
	}
	cancels := fb.ownedCancels(binding, action, reason, false, location(identifier.Token))
	if plan.Site == sema.TransferSiteArgument {
		// Moved out only when the call commits (destruction.md § 9.2(3)).
		moved := fb.deferredResult(move, owned.typ)
		fb.pendingArgumentCancels = append(fb.pendingArgumentCancels, cancels...)
		return moved, nil
	}
	moved := fb.result(move, owned.typ)
	fb.emitAll(cancels)
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
	if fb.currentCleanupScope() != owned.scope && !fb.pathDependent[binding] {
		return fb.unsupported("P17 owned reassignment outside its declaring scope that the path scan missed", stmt.Token)
	}
	if fb.deferCaptured[binding] && assignment.Previous == sema.CleanupPreviousConditional {
		// On a path without a live registration the new value would register
		// after the defer and so be destroyed before the defer reads it
		// (destruction.md §§ 6.3(3), 35.6(2)). A replacement keeps the live
		// registration's position before the defer (MD-125).
		return fb.unsupported("P17 replacement of an owned value a registered defer reads", stmt.Token)
	}
	value, err := fb.buildExpr(stmt.Value, owned.typ)
	if err != nil {
		return err
	}
	place := fb.ownedStoragePlace(owned, stmt.Token)
	registration := assignment.Registration
	// MD-125: a replacement keeps the destination's live registration and its
	// position; only a path without one registers the new incarnation
	// (destruction.md §§ 6.1(4)–(5), 13.2(5)–(8), 13.3(5); semantic_ir.md § 40(7)–(10)).
	if assignment.Previous != sema.CleanupPreviousUnavailable && assignment.Cleanup.Inherit != binding {
		return fb.unsupported("P17 owned replacement without an inherited registration", stmt.Token)
	}
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
		fb.emit(Operation{Kind: OpOwnReplacePlace, Operands: []ValueID{place.id, value.id}, Ownership: &OwnershipFacts{DestructionPlan: planID, OldCleanup: old, NewCleanup: old}, Location: location(stmt.Token)})
		return nil
	case sema.CleanupPreviousConditional:
		old, tracked := fb.trackedActions[binding]
		if !tracked || assignment.Cleanup.Register == 0 {
			return fb.unsupported("P17 conditional replacement without its inherited and new registrations", stmt.Token)
		}
		planID, err := fb.owner.importDestructionPlans(registration.Destruction, typ)
		if err != nil {
			return err
		}
		fb.emit(Operation{Kind: OpOwnReplacePlace, Operands: []ValueID{place.id, value.id}, Ownership: &OwnershipFacts{DestructionPlan: planID, OldCleanup: old, NewCleanup: uint32(assignment.Cleanup.Register), IfOwned: true}, Location: location(stmt.Token)})
		// Every incarnation that may be live on some path keeps its position;
		// the new one registers only where none is (§ 13.3(5)).
		unless := fb.liveIncarnations(binding, old, uint32(assignment.Cleanup.Register))
		registration.Action = assignment.Cleanup.Register
		if err := fb.trackOwned(stmt.Token, registration, place, typ); err != nil {
			return err
		}
		fb.currentBlockLast().Cleanup.UnlessActive = unless
		if fb.pathDependent == nil {
			fb.pathDependent = map[sema.BindingID]bool{}
		}
		fb.pathDependent[binding] = true
		return nil
	case sema.CleanupPreviousUnavailable:
		if assignment.Cleanup.Register == 0 {
			return fb.unsupported("P17 owned reinitialization without a new registration", stmt.Token)
		}
		registration.Action = assignment.Cleanup.Register
		fb.emit(Operation{Kind: OpOwnInitializePlace, Operands: []ValueID{place.id, value.id}, Ownership: &OwnershipFacts{NewCleanup: uint32(registration.Action)}, Location: location(stmt.Token)})
	case sema.CleanupPreviousPartial:
		// The RHS is complete; the old value's remaining fields are destroyed,
		// skipping those moved out, before the new value initializes the Place
		// (package17 §§ 12, 31, 58; destruction.md § 13). A field moved on only
		// some paths makes the destruction act only on what is still owned.
		if len(assignment.Moved) == 0 {
			return fb.unsupported("P17 replacement of a partially moved owner without moved paths", stmt.Token)
		}
		var skipped []OwnedPath
		if !assignment.MovedConditional {
			var err error
			if skipped, err = fieldPaths(typ, assignment.Moved); err != nil {
				return fb.unsupported("P17 replacement of a partially moved owner: "+err.Error(), stmt.Token)
			}
		}
		old, tracked := fb.trackedActions[binding]
		if !tracked {
			return fb.unsupported("P17 replacement of an untracked owned value", stmt.Token)
		}
		planID, err := fb.owner.importDestructionPlans(registration.Destruction, typ)
		if err != nil {
			return err
		}
		// The remaining fields end; the owner's registration stays in place
		// and covers the new whole value (MD-125, destruction.md § 13.2(8)).
		loc := location(stmt.Token)
		fb.emit(Operation{Kind: OpOwnDestroyPlace, Operands: []ValueID{place.id}, Ownership: &OwnershipFacts{Cause: "cleanup", DestructionPlan: planID, Skipped: skipped, IfOwned: assignment.MovedConditional}, Location: loc})
		fb.emit(Operation{Kind: OpOwnInitializePlace, Operands: []ValueID{place.id, value.id}, Ownership: &OwnershipFacts{NewCleanup: old}, Location: loc})
		return nil
	default:
		return fb.unsupported("P17 "+string(assignment.Previous)+" owned assignment (path/subobject plans)", stmt.Token)
	}
	return fb.trackOwned(stmt.Token, registration, place, typ)
}

// fieldMoveLowerable reports a move of a stored field (`<-p.left`, `p.a.b`) out of
// a local or parameter owner, which keeps owning its remaining fields (package17 § 12).
func fieldMoveLowerable(plan sema.ResolvedTransferPlan) bool {
	if plan.Action != sema.TransferMove || len(plan.Source.Projections) == 0 ||
		plan.Source.RootKind != sema.PlaceRootLocal && plan.Source.RootKind != sema.PlaceRootParameter {
		return false
	}
	for _, projection := range plan.Source.Projections {
		if projection.Kind != sema.PlaceField {
			return false
		}
	}
	return plan.Site == sema.TransferSiteReturn || plan.Site == sema.TransferSiteInitialization || plan.Site == sema.TransferSiteArgument || plan.Site == sema.TransferSiteConstruction
}

// memberRoot is the identifier a chain of field accesses starts from.
func memberRoot(expr ast.Expression) (*ast.Identifier, []*ast.MemberExpression) {
	var chain []*ast.MemberExpression
	for {
		switch node := expr.(type) {
		case *ast.MemberExpression:
			chain = append([]*ast.MemberExpression{node}, chain...)
			expr = node.Object
		case *ast.Identifier:
			return node, chain
		default:
			return nil, nil
		}
	}
}

// movesFieldPayload reports a match with an arm that moves out a whole
// named-field payload.
func movesFieldPayload(analyzer *sema.Analyzer, match *ast.MatchExpression) bool {
	plan, ok := analyzer.ResolvedMatchPlanOf(match)
	if !ok {
		return false
	}
	for _, arm := range plan.Arms {
		if arm.BindingAction != sema.MatchBindingMove || arm.PatternKind != sema.MatchPatternUnionVariant || int(arm.UnionVariantIndex) >= len(plan.SubjectType.UnionVariants) {
			continue
		}
		if len(plan.SubjectType.UnionVariants[arm.UnionVariantIndex].PayloadFields) != 0 {
			return true
		}
	}
	return false
}

// markPartialRoots finds, before any code is built, the owners a field moves out
// of; they are held in storage so their remaining fields keep their own state.
func (fb *functionBuilder) markPartialRoots(body *ast.BlockStatement) {
	mark := func(expr ast.Expression) {
		if root, chain := memberRoot(expr); root != nil && len(chain) != 0 {
			if fact, ok := fb.owner.analyzer.ResolvedBindingOf(root); ok {
				if fb.partialRoots == nil {
					fb.partialRoots = map[sema.BindingID]bool{}
				}
				fb.partialRoots[fact.ID] = true
			}
		}
	}
	_ = walk.Inspect(body, func(node any) error {
		// A payload may move out of a stored field a match or presence test names.
		switch node := node.(type) {
		case *ast.MatchExpression:
			if typ, ok := fb.owner.analyzer.ResolvedTypeOf(node.Subject); ok && !sema.TriviallyDestructible(typ) {
				mark(node.Subject)
				// A whole named-field payload leaves through its payload Place, so
				// even an immutable local subject lives in storage.
				if identifier, named := node.Subject.(*ast.Identifier); named && movesFieldPayload(fb.owner.analyzer, node) {
					if fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier); ok {
						if fb.partialRoots == nil {
							fb.partialRoots = map[sema.BindingID]bool{}
						}
						fb.partialRoots[fact.ID] = true
					}
				}
			}
		case *ast.IfStatement:
			if node.OptionBinding != nil {
				if typ, ok := fb.owner.analyzer.ResolvedTypeOf(node.OptionBinding.Subject); ok && !sema.TriviallyDestructible(typ) {
					mark(node.OptionBinding.Subject)
				}
			}
		case *ast.TryExpression:
			if typ, ok := fb.owner.analyzer.ResolvedTypeOf(node.Expression); ok && !sema.TriviallyDestructible(typ) {
				mark(node.Expression)
			}
		case *ast.AssignmentStatement:
			// An owned parameter that is replaced, whole or through a stored
			// field, lives in storage from entry like a mutable owned local
			// (destruction.md § 13; P17-19).
			root, _ := memberRoot(node.Target)
			if root == nil {
				break
			}
			if fact, ok := fb.owner.analyzer.ResolvedBindingOf(root); ok && !sema.TriviallyDestructible(fact.Type) {
				if fb.partialRoots == nil {
					fb.partialRoots = map[sema.BindingID]bool{}
				}
				fb.partialRoots[fact.ID] = true
			}
		case *ast.DiscardStatement:
			// Discarding a stored field ends it through its field Place.
			if typ, ok := fb.owner.analyzer.ResolvedTypeOf(node.Value); ok && !sema.TriviallyDestructible(typ) {
				mark(node.Value)
			}
		case *ast.CallExpression:
			// Result.Ok()/Err() consume a stored field receiver the same way.
			if member, ok := node.Callee.(*ast.MemberExpression); ok && member.Property != nil {
				site := member.Property.Token
				if known, ok := fb.owner.analyzer.CompilerKnownMemberAt(site.File, site.Line, site.Column); ok && (known.ID == "CKM-RESULT-OK" || known.ID == "CKM-RESULT-ERR") {
					if typ, ok := fb.owner.analyzer.ResolvedTypeOf(member.Object); ok && !sema.TriviallyDestructible(typ) {
						mark(member.Object)
					}
				}
			}
		}
		member, ok := node.(*ast.MemberExpression)
		if !ok {
			return nil
		}
		plan, planned := fb.owner.analyzer.ResolvedTransferPlanOf(member)
		if !planned || !fieldMoveLowerable(plan) || fb.owner.concreteTriviallyCopyable(plan.Source.Type) {
			return nil
		}
		if root, _ := memberRoot(member); root != nil {
			if fact, ok := fb.owner.analyzer.ResolvedBindingOf(root); ok {
				if fb.partialRoots == nil {
					fb.partialRoots = map[sema.BindingID]bool{}
				}
				fb.partialRoots[fact.ID] = true
			}
		}
		return nil
	})
}

// moveOwnedField moves a stored field out of an owner held in storage through
// its field Place. The owner's registration stays: scope cleanup destroys only
// its remaining fields (package17 §§ 12, 58–59).
func (fb *functionBuilder) moveOwnedField(member *ast.MemberExpression) (builtValue, bool, error) {
	if fb.owner.maxPackage < ownershipLoweringPackage {
		return builtValue{}, false, nil
	}
	plan, planned := fb.owner.analyzer.ResolvedTransferPlanOf(member)
	if !planned || !fieldMoveLowerable(plan) || fb.owner.concreteTriviallyCopyable(plan.Source.Type) {
		// A template's move of a field that is trivially copyable in this
		// specialization is an ordinary read (monomorphization.md § 11.2).
		return builtValue{}, false, nil
	}
	root, chain := memberRoot(member)
	if root == nil {
		return builtValue{}, true, fb.unsupported("P17 field move out of a computed owner", member.Token)
	}
	owned, stored := fb.ownedStorageOf(root)
	if !stored {
		return builtValue{}, true, fb.unsupported("P17 field move out of an owner without storage", member.Token)
	}
	place, err := fb.storedFieldPlace(owned, root, chain)
	if err != nil {
		return builtValue{}, true, err
	}
	element, _ := fb.owner.module.Types.Lookup(place.typ)
	return fb.result(Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{place.id}, Ownership: &OwnershipFacts{}, Location: location(member.Token)}, element.Element), true, nil
}

// storedFieldPlace projects the field Place a chain of stored fields names in
// an owner held in storage.
func (fb *functionBuilder) storedFieldPlace(owned ownedStorage, root *ast.Identifier, chain []*ast.MemberExpression) (builtValue, error) {
	place := fb.ownedStoragePlace(owned, root.Token)
	for _, step := range chain {
		field, resolved := fb.owner.analyzer.ResolvedStructMemberOf(step)
		if !resolved || field.Kind != sema.MemberStoredField {
			return builtValue{}, fb.unsupported("P17 field move through an unresolved field", step.Token)
		}
		fieldType, err := fb.owner.internType(field.MemberType)
		if err != nil {
			return builtValue{}, err
		}
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: fieldType, Mutable: true})
		place = fb.placeResult(Operation{Kind: OpPlaceField, Operands: []ValueID{place.id}, StructField: StructFieldID(field.FieldID), Place: placeMetadataOf(owned.place), Location: location(step.Token)}, placeType)
	}
	return place, nil
}

// assignOwnedField lowers `p.f = v` on an owner held in storage through the field
// Place: a held old value is replaced, a moved-out one reinitialized, and a value
// owned on some paths only replaced where owned. The owner's registration keeps
// covering the field (package17 §§ 13, 64; destruction.md § 13).
func (fb *functionBuilder) assignOwnedField(stmt *ast.AssignmentStatement, target *ast.MemberExpression) (bool, error) {
	if fb.owner.maxPackage < ownershipLoweringPackage || stmt.Operator != "=" {
		return false, nil
	}
	root, chain := memberRoot(target)
	if root == nil {
		return false, nil
	}
	owned, stored := fb.ownedStorageOf(root)
	if !stored {
		return false, nil
	}
	previous, known := fb.owner.analyzer.ResolvedOwnedFieldAssignmentOf(stmt)
	if !known {
		return true, fb.unsupported("P17 owned field assignment without resolved field state", stmt.Token)
	}
	var plans []sema.ResolvedStructMemberPlan
	for _, step := range chain {
		field, resolved := fb.owner.analyzer.ResolvedStructMemberOf(step)
		if !resolved || field.Kind != sema.MemberStoredField {
			return true, fb.unsupported("P17 owned field assignment through an unresolved field", step.Token)
		}
		plans = append(plans, field)
	}
	leaf := plans[len(plans)-1].MemberType
	leafType, err := fb.owner.internType(leaf)
	if err != nil {
		return true, err
	}
	value, err := fb.buildExpr(stmt.Value, leafType)
	if err != nil {
		return true, err
	}
	place := fb.ownedStoragePlace(owned, root.Token)
	for index, field := range plans {
		fieldType, err := fb.owner.internType(field.MemberType)
		if err != nil {
			return true, err
		}
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: fieldType, Mutable: true})
		place = fb.placeResult(Operation{Kind: OpPlaceField, Operands: []ValueID{place.id}, StructField: StructFieldID(field.FieldID), Place: placeMetadataOf(owned.place), Location: location(chain[index].Token)}, placeType)
	}
	loc := location(stmt.Token)
	if sema.TriviallyDestructible(leaf) && sema.CopyClassificationOf(leaf) == sema.CopyTrivial {
		fb.emit(Operation{Kind: OpPlaceWrite, Operands: []ValueID{place.id, value.id}, Location: loc})
		return true, nil
	}
	switch previous {
	case sema.CleanupPreviousAvailable, sema.CleanupPreviousConditional:
		planID, err := fb.owner.importDestructionPlans(sema.ResolveDestructionPlans(leaf), leaf)
		if err != nil {
			return true, err
		}
		fb.emit(Operation{Kind: OpOwnReplacePlace, Operands: []ValueID{place.id, value.id}, Ownership: &OwnershipFacts{DestructionPlan: planID, IfOwned: previous == sema.CleanupPreviousConditional}, Location: loc})
	case sema.CleanupPreviousUnavailable:
		fb.emit(Operation{Kind: OpOwnInitializePlace, Operands: []ValueID{place.id, value.id}, Ownership: &OwnershipFacts{}, Location: loc})
	default:
		return true, fb.unsupported("P17 "+string(previous)+" owned field assignment", stmt.Token)
	}
	return true, nil
}

// markPathDependentBindings finds, before any code is built, the bindings assigned
// in a block other than the one declaring them. A loop can bring an earlier use
// back after such an assignment, so even that use must cancel every incarnation.
func (fb *functionBuilder) markPathDependentBindings(body *ast.BlockStatement) {
	declared := map[sema.BindingID]*ast.BlockStatement{}
	assigned := map[sema.BindingID][]*ast.BlockStatement{}
	_ = walk.Inspect(body, func(node any) error {
		block, ok := node.(*ast.BlockStatement)
		if !ok {
			return nil
		}
		for _, statement := range block.Statements {
			switch statement := statement.(type) {
			case *ast.LetStatement:
				if fact, ok := fb.owner.analyzer.ResolvedBindingOf(statement.Name); ok {
					declared[fact.ID] = block
				}
			case *ast.AssignmentStatement:
				if identifier, ok := statement.Target.(*ast.Identifier); ok {
					if fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier); ok {
						assigned[fact.ID] = append(assigned[fact.ID], block)
					}
				}
			}
		}
		return nil
	})
	for binding, blocks := range assigned {
		home := declared[binding]
		if home == nil {
			home = body
		}
		for _, block := range blocks {
			if block != home {
				if fb.pathDependent == nil {
					fb.pathDependent = map[sema.BindingID]bool{}
				}
				fb.pathDependent[binding] = true
			}
		}
	}
}

// fieldPaths maps Sema's moved field-name paths to owned subobject paths.
func fieldPaths(root sema.Type, names [][]string) ([]OwnedPath, error) {
	var paths []OwnedPath
	for _, chain := range names {
		var path OwnedPath
		typ := root
		for _, name := range chain {
			index := -1
			for position, field := range typ.Fields {
				if field.Name == name {
					index = position
					typ = field.Type
					break
				}
			}
			if index < 0 {
				return nil, fmt.Errorf("no stored field %s", name)
			}
			path = append(path, OwnedStep{Index: uint32(index)})
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// ownedCancels ends a binding's live cleanup registration. After a reassignment
// in a nested scope or loop the live incarnation depends on the path, so every
// incarnation of the binding is cancelled where it is active.
// Rules: rules/memory/destruction.md — §§ 9.1, 13; rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 38, 55.
func (fb *functionBuilder) ownedCancels(binding sema.BindingID, action uint32, reason string, ifActive bool, loc Location) []Operation {
	if !fb.pathDependent[binding] {
		return []Operation{{Kind: OpCleanupCancel, Cleanup: &CleanupFacts{Action: action, Reason: reason, IfActive: ifActive}, Location: loc}}
	}
	plan, _ := fb.cleanupPlan()
	var cancels []Operation
	for _, registration := range plan.Registrations {
		if registration.Binding == binding && registration.Kind != sema.CleanupRegisterDefer && registration.Kind != sema.CleanupRegisterMatchBinding {
			cancels = append(cancels, Operation{Kind: OpCleanupCancel, Cleanup: &CleanupFacts{Action: uint32(registration.Action), Reason: reason, IfActive: true}, Location: loc})
		}
	}
	return cancels
}

// liveIncarnations lists the registrations of binding that may be live here:
// the current one and, once the live incarnation depends on the path, every
// owned registration Sema published for it, including fresh itself when an
// earlier loop iteration registered it and it is still live.
func (fb *functionBuilder) liveIncarnations(binding sema.BindingID, current, fresh uint32) []uint32 {
	actions := []uint32{current}
	if !fb.pathDependent[binding] {
		return actions
	}
	plan, _ := fb.cleanupPlan()
	for _, registration := range plan.Registrations {
		if registration.Binding == binding && registration.Kind != sema.CleanupRegisterDefer && registration.Kind != sema.CleanupRegisterMatchBinding && uint32(registration.Action) != current {
			actions = append(actions, uint32(registration.Action))
		}
	}
	return actions
}

// currentBlockLast is the operation just emitted in the current block.
func (fb *functionBuilder) currentBlockLast() *Operation {
	return &fb.current.Operations[len(fb.current.Operations)-1]
}

func (fb *functionBuilder) emitAll(operations []Operation) {
	for _, op := range operations {
		fb.emit(op)
	}
}

// ownedStorageOf returns the owned mutable local an expression names.
func (fb *functionBuilder) ownedStorageOf(expr ast.Expression) (ownedStorage, bool) {
	identifier, ok := expr.(*ast.Identifier)
	if !ok || len(fb.ownedStorages) == 0 {
		return ownedStorage{}, false
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier)
	if !ok || fb.bindings[fact.ID].capture != 0 {
		// Inside a defer body the binding is its capture Place.
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

// readNestedOwnedField reads a copy-trivial field below a non-trivial field of
// a binding (`pair.left.id`) through its Place chain, so the intermediate owned
// subobject is observed in place and never copied out (package17 § 12;
// destruction.md § 8).
func (fb *functionBuilder) readNestedOwnedField(expr *ast.MemberExpression, typeID TypeID) (builtValue, bool, error) {
	if fb.owner.maxPackage < ownershipLoweringPackage {
		return builtValue{}, false, nil
	}
	root, chain := memberRoot(expr)
	if root == nil || len(chain) < 2 {
		return builtValue{}, false, nil
	}
	objectType, typed := fb.owner.analyzer.ResolvedTypeOf(expr.Object)
	if !typed || sema.TriviallyDestructible(objectType) {
		return builtValue{}, false, nil
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(root)
	if !ok {
		return builtValue{}, false, nil
	}
	bound, ok := fb.bindings[fact.ID]
	if !ok || bound.capture != 0 {
		return builtValue{}, false, nil
	}
	loc := location(expr.Token)
	var place builtValue
	var metadata sema.Place
	if owned, isOwned := fb.ownedStorageOf(root); isOwned {
		place, metadata = fb.ownedStoragePlace(owned, root.Token), owned.place
	} else if bound.storage == 0 {
		rootPlace, resolved := fb.owner.analyzer.ResolvedPlaceOf(root)
		if !resolved {
			return builtValue{}, false, nil
		}
		rootPlace.Projections = nil
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: bound.typ})
		place = fb.placeResult(Operation{Kind: OpPlaceValueRoot, Operands: []ValueID{bound.value}, Place: placeMetadataOf(rootPlace), Location: loc}, placeType)
		metadata = rootPlace
	} else {
		return builtValue{}, false, nil
	}
	mutable := false
	if typ, found := fb.owner.module.Types.Lookup(place.typ); found {
		mutable = typ.Mutable
	}
	for _, member := range chain {
		plan, resolved := fb.owner.analyzer.ResolvedStructMemberOf(member)
		if !resolved || plan.Kind != sema.MemberStoredField {
			return builtValue{}, true, fb.unsupported("nested owned field read through a non-stored member", member.Token)
		}
		fieldType, err := fb.owner.internType(plan.MemberType)
		if err != nil {
			return builtValue{}, true, err
		}
		fieldPlaceType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: fieldType, Mutable: mutable})
		place = fb.placeResult(Operation{Kind: OpPlaceField, Operands: []ValueID{place.id}, StructField: StructFieldID(plan.FieldID), Place: placeMetadataOf(metadata), Location: location(member.Token)}, fieldPlaceType)
	}
	return fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: loc}, typeID), true, nil
}

// deferredResult allocates op's result now and emits op when the enclosing
// call commits, together with the argument cancellations.
func (fb *functionBuilder) deferredResult(op Operation, typ TypeID) builtValue {
	v := fb.newValue(typ, OwnershipImmediate, op.Location)
	op.Results = []Value{v}
	fb.pendingArgumentCancels = append(fb.pendingArgumentCancels, op)
	return builtValue{id: v.ID, typ: typ}
}

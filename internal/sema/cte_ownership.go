package sema

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// cteCleanup is one entry of an invocation's unified cleanup stack
// (destruction.md § 6.2; defer.md §§ 3, 6): either a registered deferred
// block or the destruction responsibility of an owned binding.
type cteCleanup struct {
	deferBody *ast.BlockStatement
	// overlay holds the binding instances a deferred block reads whose
	// lexical scope ended before the defer runs, such as the bindings of
	// the loop iteration that registered it (defer.md § 13).
	overlay map[sourceTokenKey]DefaultConstant
	binding sourceTokenKey
	// depth is the lexical scope depth that owns a destruction entry;
	// depth 0 is the invocation body, whose entries end at invocation exit.
	depth int
}

// cteScope records the bindings declared in one active lexical scope and
// where its cleanup registrations start.
type cteScope struct {
	declared []sourceTokenKey
	mark     int
}

// ownership is the evaluator-local ownership state of one evaluation:
// every value of a type with a custom `free` receives a logical identity
// when it is created, which moves and copy-on-write updates preserve. An
// identity ends exactly once, by destruction; one moved out of an
// aggregate is detached so the container's destruction skips it.
type cteOwnership struct {
	next      int
	live      map[int]Type
	ended     map[int]bool
	detached  map[int]bool
	freeBody  map[sourceTokenKey]*ast.BlockStatement
	execution int
}

func (x *cteExecutor) ownershipState() *cteOwnership {
	if x.ownership == nil {
		x.ownership = &cteOwnership{live: map[int]Type{}, ended: map[int]bool{}, detached: map[int]bool{}, freeBody: map[sourceTokenKey]*ast.BlockStatement{}}
	}
	return x.ownership
}

// assignOwner gives a newly created value of a custom-free type its logical
// identity.
func (x *cteExecutor) assignOwner(value DefaultConstant) {
	aggregate := value.Aggregate
	if aggregate == nil || aggregate.Owner != 0 || !aggregate.Type.CustomFree {
		return
	}
	state := x.ownershipState()
	state.next++
	aggregate.Owner = state.next
	state.live[state.next] = aggregate.Type
}

// ownedIdentities lists the logical identities in a value tree.
func ownedIdentities(value DefaultConstant, out []int) []int {
	aggregate := value.Aggregate
	if aggregate == nil {
		return out
	}
	if aggregate.Owner != 0 {
		out = append(out, aggregate.Owner)
	}
	for _, name := range aggregate.Names {
		out = ownedIdentities(aggregate.Fields[name], out)
	}
	if aggregate.Payload != nil {
		out = ownedIdentities(*aggregate.Payload, out)
	}
	return out
}

// enterScope opens a lexical scope in the current frame. Bindings the
// enclosing statement introduced for it (loop variables) belong to it.
func (x *cteExecutor) enterScope() {
	frame := x.frame()
	frame.scopes = append(frame.scopes, &cteScope{declared: frame.pending, mark: len(frame.cleanups)})
	frame.pending = nil
}

// declare records a binding in the innermost scope.
func (x *cteExecutor) declare(binding sourceTokenKey) {
	frame := x.frame()
	if len(frame.scopes) > 0 {
		scope := frame.scopes[len(frame.scopes)-1]
		scope.declared = append(scope.declared, binding)
	}
}

// registerBinding establishes the destruction responsibility of an owned
// binding at its initialization point (destruction.md § 6.1(1)). Values of
// trivially destructible types need none.
func (x *cteExecutor) registerBinding(binding sourceTokenKey, typ Type) {
	if TriviallyDestructible(typ) {
		return
	}
	frame := x.frame()
	delete(frame.moved, binding)
	frame.cleanups = append(frame.cleanups, cteCleanup{binding: binding, depth: len(frame.scopes)})
}

// registered reports whether a binding has an active destruction entry.
func (frame *cteFrame) registered(binding sourceTokenKey) bool {
	for _, cleanup := range frame.cleanups {
		if cleanup.deferBody == nil && cleanup.binding == binding {
			return true
		}
	}
	return false
}

// registerDefer registers one deferred block; it runs when the enclosing
// invocation exits, not when its block ends (defer.md §§ 3, 5, 6).
func (x *cteExecutor) registerDefer(body *ast.BlockStatement) {
	frame := x.frame()
	frame.cleanups = append(frame.cleanups, cteCleanup{deferBody: body, depth: len(frame.scopes)})
}

// leaveScope ends a lexical scope: deferred blocks registered inside it
// keep the binding instances of this scope they may read, and the scope's
// owned bindings are destroyed in reverse registration order. Deferred
// blocks stay registered for invocation exit.
func (x *cteExecutor) leaveScope() {
	frame := x.frame()
	scope := frame.scopes[len(frame.scopes)-1]
	depth := len(frame.scopes)
	frame.scopes = frame.scopes[:len(frame.scopes)-1]
	if x.failure != nil {
		return
	}
	for index := scope.mark; index < len(frame.cleanups); index++ {
		cleanup := &frame.cleanups[index]
		if cleanup.deferBody == nil {
			continue
		}
		for _, binding := range scope.declared {
			if _, kept := cleanup.overlay[binding]; kept {
				continue
			}
			if value, ok := frame.values[binding]; ok {
				if cleanup.overlay == nil {
					cleanup.overlay = map[sourceTokenKey]DefaultConstant{}
				}
				cleanup.overlay[binding] = value
			}
		}
	}
	kept := frame.cleanups[:scope.mark]
	destroy := []cteCleanup{}
	for _, cleanup := range frame.cleanups[scope.mark:] {
		if cleanup.deferBody == nil && cleanup.depth >= depth {
			destroy = append(destroy, cleanup)
			continue
		}
		kept = append(kept, cleanup)
	}
	frame.cleanups = kept
	for index := len(destroy) - 1; index >= 0 && x.failure == nil; index-- {
		x.destroyBinding(frame, destroy[index].binding)
	}
}

// runInvocationCleanups runs the remaining unified cleanup stack of an
// invocation in reverse registration order: deferred blocks and the
// destruction of body-scope bindings and by-value parameters interleave
// as registered (destruction.md § 6.2(1)–(4)). An evaluation that already
// failed does not continue source cleanup (compile_time_evaluation.md
// § 27(4)).
func (x *cteExecutor) runInvocationCleanups() {
	frame := x.frame()
	for len(frame.cleanups) > 0 && x.failure == nil {
		cleanup := frame.cleanups[len(frame.cleanups)-1]
		frame.cleanups = frame.cleanups[:len(frame.cleanups)-1]
		if cleanup.deferBody == nil {
			x.destroyBinding(frame, cleanup.binding)
			continue
		}
		saved := map[sourceTokenKey]DefaultConstant{}
		for binding, value := range cleanup.overlay {
			if current, ok := frame.values[binding]; ok {
				saved[binding] = current
			}
			frame.values[binding] = value
		}
		pending := x.pendingReturn
		x.pendingReturn = nil
		x.block(cleanup.deferBody)
		x.pendingReturn = pending
		for binding := range cleanup.overlay {
			if value, ok := saved[binding]; ok {
				frame.values[binding] = value
			} else {
				delete(frame.values, binding)
			}
		}
	}
}

// destroyBinding destroys the value an owned binding still holds.
func (x *cteExecutor) destroyBinding(frame *cteFrame, binding sourceTokenKey) {
	if frame.moved[binding] {
		return
	}
	value, ok := frame.values[binding]
	if !ok {
		return
	}
	frame.moved[binding] = true
	x.destroyValue(value, true, lexer.Token{})
}

// destroyValue ends the ownership of a value (destruction.md §§ 7.2, 15.5):
// a custom `free` body runs first, then the remaining owned fields are
// destroyed in reverse declaration order and an active payload after them.
// direct is false while destroying the parts of a container, so parts
// moved out of it earlier are skipped.
func (x *cteExecutor) destroyValue(value DefaultConstant, direct bool, at lexer.Token) {
	aggregate := value.Aggregate
	if aggregate == nil || x.failure != nil {
		return
	}
	state := x.ownershipState()
	if aggregate.Owner != 0 {
		if state.ended[aggregate.Owner] || !direct && state.detached[aggregate.Owner] {
			return
		}
		state.ended[aggregate.Owner] = true
		delete(state.live, aggregate.Owner)
		delete(state.detached, aggregate.Owner)
	}
	if aggregate.Type.CustomFree {
		x.runFree(value, at)
	}
	for index := len(aggregate.Names) - 1; index >= 0 && x.failure == nil; index-- {
		x.destroyValue(aggregate.Fields[aggregate.Names[index]], false, at)
	}
	if aggregate.Payload != nil {
		x.destroyValue(*aggregate.Payload, false, at)
	}
}

// runFree executes a type's custom `free` body on the value, as its own
// invocation with its own local cleanup (destruction.md §§ 15.3, 15.7;
// compile_time_evaluation.md § 27(2)).
func (x *cteExecutor) runFree(value DefaultConstant, at lexer.Token) {
	typ := value.Aggregate.Type
	var body *ast.BlockStatement
	var free Function
	for _, name := range []string{freeFunctionName(typ.Name), freeFunctionName(typ.Module + "." + typ.Name)} {
		for _, candidate := range x.a.functions[name] {
			if candidate.Module == typ.Module || candidate.Module == "" || typ.Module == "" {
				if found := x.ownershipState().freeBody[sourceTokenLocation(candidate.Token)]; found != nil {
					body, free = found, candidate
				}
			}
		}
	}
	if body == nil {
		x.fail(cteImplementationGap, at, "the custom free of %s is not available to this compile-time evaluation", typeDisplayName(typ))
		return
	}
	if len(x.frames) >= cteMaxCallDepth {
		x.fail(cteResourceLimit, at, "the CallDepth budget of %d nested calls was reached", cteMaxCallDepth)
		return
	}
	receiver := value
	x.depend(CTEDependsOnDeclaration, string(callableID(free)))
	x.frames = append(x.frames, newCTEFrame(&receiver))
	x.calls = append(x.calls, cteCallSite{Function: "free of " + typeDisplayName(typ), Call: free.Token, Module: free.Module})
	x.invocation(body)
	x.calls = x.calls[:len(x.calls)-1]
	x.frames = x.frames[:len(x.frames)-1]
}

// noteMove ends the source's responsibility when an executed expression is
// one Sema accepted as an ownership transfer: a moved binding is not
// destroyed at scope exit, and a value moved out of an aggregate is
// detached from its container.
func (x *cteExecutor) noteMove(expr ast.Expression, value DefaultConstant) {
	if !x.a.moveSources[expr] {
		return
	}
	switch expr := expr.(type) {
	case *ast.Identifier:
		if declaration, ok := x.declarationOf(expr.Token); ok {
			if _, local := x.frame().values[declaration]; local {
				x.frame().moved[declaration] = true
			}
		}
	case *ast.MemberExpression:
		state := x.ownershipState()
		for _, owner := range outermostOwners(value, nil) {
			state.detached[owner] = true
		}
	}
}

// outermostOwners lists the owned values of a tree that are not parts of
// another owned value in it: transferring the tree transfers exactly these,
// and each still owns its own parts.
func outermostOwners(value DefaultConstant, out []int) []int {
	aggregate := value.Aggregate
	if aggregate == nil {
		return out
	}
	if aggregate.Owner != 0 {
		return append(out, aggregate.Owner)
	}
	for _, name := range aggregate.Names {
		out = outermostOwners(aggregate.Fields[name], out)
	}
	if aggregate.Payload != nil {
		out = outermostOwners(*aggregate.Payload, out)
	}
	return out
}

// isPlaceExpression reports whether expr names storage (a binding or a
// field path of one) rather than a temporary.
func isPlaceExpression(expr ast.Expression) bool {
	root, _ := memberPath(expr)
	_, identifier := root.(*ast.Identifier)
	return identifier
}

// unaccountedOwnership reports a created custom-free value that was never
// destroyed and is not part of the evaluated result: a temporary whose
// destruction point the executor does not model. Reporting it keeps a
// `free` from being skipped silently.
func (x *cteExecutor) unaccountedOwnership(result DefaultConstant) (Type, bool) {
	if x.ownership == nil || len(x.ownership.live) == 0 {
		return Type{}, false
	}
	retained := map[int]bool{}
	for _, owner := range ownedIdentities(result, nil) {
		retained[owner] = true
	}
	owners := []int{}
	for owner := range x.ownership.live {
		if !retained[owner] {
			owners = append(owners, owner)
		}
	}
	if len(owners) == 0 {
		return Type{}, false
	}
	sort.Ints(owners)
	return x.ownership.live[owners[0]], true
}

// invocation executes a callable body as one invocation boundary: its
// body-scope bindings and deferred blocks clean up at exit.
func (x *cteExecutor) invocation(body *ast.BlockStatement) (cteControl, DefaultConstant) {
	control, value := x.blockStatements(body)
	x.runInvocationCleanups()
	return control, value
}

func newCTEFrame(receiver *DefaultConstant) *cteFrame {
	return &cteFrame{values: map[sourceTokenKey]DefaultConstant{}, receiver: receiver, moved: map[sourceTokenKey]bool{}}
}

// placeValue reads the current value of a local Place without executing a
// move.
func (x *cteExecutor) placeValue(target ast.Expression) (DefaultConstant, bool) {
	root, path := memberPath(target)
	identifier, ok := root.(*ast.Identifier)
	if !ok {
		return DefaultConstant{}, false
	}
	var current DefaultConstant
	if receiver := x.frame().receiver; receiver != nil && identifier.Value == "self" {
		current = *receiver
	} else {
		declaration, found := x.declarationOf(identifier.Token)
		if !found {
			return DefaultConstant{}, false
		}
		if alias, aliased := x.frame().aliases[declaration]; aliased {
			aliasRoot, aliasPath := memberPath(alias)
			if _, ok := aliasRoot.(*ast.Identifier); ok {
				value, ok := x.placeValue(aliasRoot)
				if !ok {
					return DefaultConstant{}, false
				}
				return fieldAtPath(value, append(append([]string(nil), aliasPath...), path...))
			}
		}
		current, found = x.frame().values[declaration]
		if !found {
			return DefaultConstant{}, false
		}
	}
	return fieldAtPath(current, path)
}

// fieldAtPath reads a field path of an aggregate value.
func fieldAtPath(value DefaultConstant, path []string) (DefaultConstant, bool) {
	for _, name := range path {
		if value.Aggregate == nil {
			return DefaultConstant{}, false
		}
		field, ok := value.Aggregate.Fields[name]
		if !ok {
			return DefaultConstant{}, false
		}
		value = field
	}
	return value, true
}

// destroyReplaced destroys the custom-free parts of old that replacement no
// longer holds.
func (x *cteExecutor) destroyReplaced(old, replacement DefaultConstant, at lexer.Token) {
	if x.ownership == nil {
		return
	}
	kept := ownedIdentities(replacement, nil)
	state := x.ownershipState()
	marked := []int{}
	for _, owner := range kept {
		if !state.detached[owner] {
			state.detached[owner] = true
			marked = append(marked, owner)
		}
	}
	var walk func(DefaultConstant)
	walk = func(value DefaultConstant) {
		aggregate := value.Aggregate
		if aggregate == nil || x.failure != nil {
			return
		}
		if aggregate.Owner != 0 && !state.detached[aggregate.Owner] && !state.ended[aggregate.Owner] {
			x.destroyValue(value, true, at)
			return
		}
		for _, name := range aggregate.Names {
			walk(aggregate.Fields[name])
		}
		if aggregate.Payload != nil {
			walk(*aggregate.Payload)
		}
	}
	walk(old)
	for _, owner := range marked {
		delete(state.detached, owner)
	}
}

// bindPattern binds a match pattern variable and remembers it for the arm.
func (x *cteExecutor) bindPattern(binding sourceTokenKey, value DefaultConstant) {
	x.frame().values[binding] = value
	x.patternBindings = append(x.patternBindings, binding)
}

// takeOwnedPatternBindings makes the selected arm's bindings that received
// owned values responsible for them: the values are detached from the
// subject and destroyed when the arm's scope ends unless moved on.
func (x *cteExecutor) takeOwnedPatternBindings() {
	frame := x.frame()
	for _, binding := range x.patternBindings {
		x.declare(binding)
		owners := outermostOwners(frame.values[binding], nil)
		if len(owners) == 0 {
			continue
		}
		state := x.ownershipState()
		for _, owner := range owners {
			state.detached[owner] = true
		}
		delete(frame.moved, binding)
		frame.cleanups = append(frame.cleanups, cteCleanup{binding: binding, depth: len(frame.scopes)})
	}
	x.patternBindings = nil
}

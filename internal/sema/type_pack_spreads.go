package sema

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// maxTypePackForwardInstances bounds the walk over forwarded specializations. A
// template such as `F(1, values...)` grows its pack at every level; this is a
// resource limit, not a proof of non-convergence (monomorphization.md §18.7).
const maxTypePackForwardInstances = 256

// typePackSourceMode is how the enclosing template received its pack.
type typePackSourceMode int

const (
	typePackSourceOwned typePackSourceMode = iota
	typePackSourceShared
	typePackSourceMutable
)

// typePackForwardArgument is one source argument of a forwarding call.
type typePackForwardArgument struct {
	expression ast.Expression
	typ        Type
	spread     bool
}

// typePackForward is a call whose arguments expand the enclosing template's
// heterogeneous pack. Its arity and position types exist only per
// specialization, so the template records it and every selected sequence
// proves it (functions.md §38A(4)(2),(4)(6),(7)).
type typePackForward struct {
	call      *ast.CallExpression
	targets   []Function // Every candidate; each specialization selects one.
	template  sourceTokenKey
	pack      string
	mode      typePackSourceMode
	consuming bool
	arguments []typePackForwardArgument
	borrow    ResolvedReferencePlan // The shared loan an owned pack lends its elements under.
}

// ResolvedTypePackForward is a forwarding call in one specialization: the
// selected concrete callee and, for each expanded argument, the source argument
// it comes from and the pack position it reads (-1 for an ordinary argument).
type ResolvedTypePackForward struct {
	Function  Function
	Consuming bool
	Mode      string // "owned", "shared" or "mutable": how the template holds its pack.
	Borrow    ResolvedReferencePlan
	Expanded  []ResolvedTypePackForwardArgument
}

// ResolvedTypePackForwardArgument is one expanded argument of a forwarding call.
type ResolvedTypePackForwardArgument struct {
	Source    int
	Position  int
	Parameter FunctionParameter
}

// consumingPackSpread recognizes `<-source...`, whose `...` binds tighter than
// the prefix marker.
func consumingPackSpread(argument ast.Expression) (*ast.PrefixExpression, *ast.SpreadExpression, bool) {
	move, explicit := explicitMoveArgument(argument)
	if !explicit {
		return nil, nil, false
	}
	spread, ok := move.Right.(*ast.SpreadExpression)
	return move, spread, ok
}

// validateConsumingPackSpreads checks `<-values...` outside a heterogeneous
// template: the target parameter must take ownership and no other argument may
// touch the pack (functions.md §38A(7)(3),(7)).
func (a *Analyzer) validateConsumingPackSpreads(function Function, call *ast.CallExpression) bool {
	for index, argument := range call.Arguments {
		move, spread, consuming := consumingPackSpread(argument)
		if !consuming || a.callExpandsDeclaredPack(call) {
			continue
		}
		parameter, known := functionVariadicParameter(function)
		if _, borrowed := callParameterBorrowMode(parameter); known && borrowed {
			a.addErrorAtToken(move.Token, "<-%s... moves the pack, but parameter %s borrows; pass %s... to lend it", spread.Value.String(), parameter.Name, spread.Value.String())
			return false
		}
		place, ok := a.resolvePlace(spread.Value)
		if !ok {
			continue
		}
		for other, sibling := range call.Arguments {
			if other == index {
				continue
			}
			source := explicitMoveSource(sibling)
			if inner, spreadSibling := source.(*ast.SpreadExpression); spreadSibling {
				source = inner.Value
			}
			if overlapping, reusable := a.resolvePlace(source); reusable && placesMayOverlap(place, overlapping) {
				a.addErrorAtTokenWithMetadata(expressionToken(sibling), diagnostics.BorrowOwnershipEndConflict,
					"Pass the pack once; it is moved as a whole.",
					"argument %d uses %s while the same call moves the complete pack", other+1, overlapping.String())
				return false
			}
		}
		if !a.validateNamedOwnershipSource(ast.OwnershipMove, spread.Value, move.Token, false, false, TransferSiteArgument) {
			return false
		}
	}
	return true
}

// markConsumedPackSpreads commits the whole-pack transfer with the call.
func (a *Analyzer) markConsumedPackSpreads(call *ast.CallExpression) {
	for _, argument := range call.Arguments {
		if _, spread, consuming := consumingPackSpread(argument); consuming && a.markExplicitMoveSource(spread.Value) {
			if identifier, named := spread.Value.(*ast.Identifier); named {
				a.availabilityStates[identifier.Value] = unavailableState("consumed by call")
				a.endBorrowsHeldBy(identifier.Value)
			}
		}
	}
}

// declaredPackSpreadSource resolves the identifier of a spread to the enclosing
// callable's own heterogeneous pack.
func (a *Analyzer) declaredPackSpreadSource(spread *ast.SpreadExpression) (*ast.Identifier, Type, bool) {
	identifier, ok := spread.Value.(*ast.Identifier)
	if !ok {
		return nil, Type{}, false
	}
	symbol, found := a.symbols[identifier.Value]
	if !found || symbol.Type.Kind != VariadicPackType || symbol.Type.Element == nil {
		return nil, Type{}, false
	}
	element := *symbol.Type.Element
	mode := element
	if mode.Kind == ReferenceType && mode.Element != nil {
		mode = *mode.Element
	}
	return identifier, element, mode.Kind == GenericTypePackType
}

// callExpandsDeclaredPack reports a call that forwards the enclosing pack, which
// resolves through inferTypePackForward instead of ordinary overload ranking.
func (a *Analyzer) callExpandsDeclaredPack(call *ast.CallExpression) bool {
	for _, argument := range call.Arguments {
		spread, ok := argument.(*ast.SpreadExpression)
		if !ok {
			_, spread, ok = consumingPackSpread(argument)
		}
		if !ok {
			continue
		}
		if _, _, declared := a.declaredPackSpreadSource(spread); declared {
			return true
		}
	}
	return false
}

// typePackNameOf names the pack inside a pack parameter's element type.
func typePackNameOf(element Type) string {
	if element.Kind == ReferenceType && element.Element != nil {
		element = *element.Element
	}
	return element.Name
}

func typePackSourceModeOf(element Type) typePackSourceMode {
	if element.Kind != ReferenceType {
		return typePackSourceOwned
	}
	if element.ReferenceMutable {
		return typePackSourceMutable
	}
	return typePackSourceShared
}

func (mode typePackSourceMode) String() string {
	switch mode {
	case typePackSourceShared:
		return "shared"
	case typePackSourceMutable:
		return "mutable"
	}
	return "owned"
}

// typePackForwardUnavailable reports a form with no representation yet; it
// never falls back to ordinary generic semantics (diagnostics.md §24(4h)).
func (a *Analyzer) typePackForwardUnavailable(token lexer.Token, form string) {
	a.addErrorAtTokenWithMetadata(token, diagnostics.GenericTypePackExpansionUnavailable,
		"Pack forwarding currently needs a statically dispatched target with no consuming receiver, one result type, no reference held inside a returned value and no array or slice spread in the same call.",
		"compiler capability unavailable: pack forwarding %s", form)
}

// typePackForwardCandidates applies the receiver to each candidate and keeps
// those that accept it and the ordinary arguments before the first spread.
func (a *Analyzer) typePackForwardCandidates(call *ast.CallExpression, functions []Function, receiver methodReceiverInfo, isMethodCall bool, prefix int) ([]Function, methodReceiverInfo, bool) {
	token := call.Token
	candidates := []Function{}
	failure := ""
	for _, function := range functions {
		if function.Extern {
			a.addErrorAtToken(token, "a native pack cannot be spread into foreign function %s; native packs are not C varargs", function.Name)
			return nil, receiver, false
		}
		// A direct reference gets per-call origins; one held inside an aggregate
		// has no projection through an expansion yet.
		if function.ReturnType.Kind != ReferenceType && typeContainsReference(function.ReturnType, map[string]bool{}) {
			a.typePackForwardUnavailable(token, "into a callable returning a value that contains a reference")
			return nil, receiver, false
		}
		if function.ReceiverConsuming {
			a.typePackForwardUnavailable(token, "into a method consuming its receiver")
			return nil, receiver, false
		}
		if isMethodCall && dereferenceType(receiver.Type).Kind != StructType {
			if kind := dereferenceType(receiver.Type).Kind; kind == InterfaceType || kind == GenericType {
				a.typePackForwardUnavailable(token, "through interface dispatch")
				return nil, receiver, false
			}
		}
		if functionUsesReceiver(function) {
			candidateReceiver, known := receiver, isMethodCall
			if !known {
				candidateReceiver, known = a.implicitTypePackReceiver(function, call)
			}
			if !known {
				failure = "method " + function.Name + " requires an instance receiver"
				continue
			}
			if !a.canPassImplicitMethodReceiver(function, candidateReceiver) {
				failure = a.implicitMethodReceiverError(function, candidateReceiver)
				continue
			}
			receiver, isMethodCall = candidateReceiver, true
			function = a.instantiateGenericImplMethod(function, candidateReceiver)
		}
		if _, variadic := functionVariadicParameter(function); !variadic && prefix > len(function.Parameters) {
			failure = fmt.Sprintf("%s expects %d arguments, got at least %d", function.Name, len(function.Parameters), prefix)
			continue
		}
		candidates = append(candidates, function)
	}
	if len(candidates) == 0 {
		if failure == "" {
			failure = "no callable accepts this pack forwarding call"
		}
		a.addErrorAtToken(token, "%s", failure)
		return nil, receiver, false
	}
	return candidates, receiver, true
}

// typePackForwardCommonContract requires what the template relies on before a
// specialization selects among candidates: one result type, and one ownership
// mode for each ordinary argument and the receiver (functions.md §38A(4)(5)).
func (a *Analyzer) typePackForwardCommonContract(call *ast.CallExpression, candidates []Function, prefix []ast.Expression, types []Type) (Type, bool) {
	var result Type
	for index, candidate := range candidates {
		returned, known := a.typePackForwardReturn(candidate, prefix, types)
		if !known {
			a.typePackForwardUnavailable(call.Token, "whose result type depends on the pack")
			return Type{}, false
		}
		if index == 0 {
			result = returned
			continue
		}
		if !sameConcreteType(result, returned) {
			a.typePackForwardUnavailable(call.Token, "into overloads with different result types")
			return Type{}, false
		}
		if candidate.ReceiverMutable != candidates[0].ReceiverMutable {
			a.typePackForwardUnavailable(call.Token, "into overloads with different receiver modes")
			return Type{}, false
		}
		for position := range prefix {
			left, leftKnown := functionParameterForArgument(candidates[0], position)
			right, rightKnown := functionParameterForArgument(candidate, position)
			leftMutable, leftBorrowed := callParameterBorrowMode(left)
			rightMutable, rightBorrowed := callParameterBorrowMode(right)
			if !leftKnown || !rightKnown || leftBorrowed != rightBorrowed || leftMutable != rightMutable || left.Consuming != right.Consuming {
				a.typePackForwardUnavailable(expressionToken(prefix[position]), "into overloads with different modes for an ordinary argument")
				return Type{}, false
			}
		}
	}
	return result, true
}

// typePackForwardTrailing checks ordinary arguments after the first spread, whose
// parameter exists per specialization. A single variadic target fixes their mode;
// any other target accepts only a copy, since no transfer or loan can be planned.
func (a *Analyzer) typePackForwardTrailing(call *ast.CallExpression, candidates []Function, trailing []ast.Expression, types []Type) bool {
	if len(trailing) == 0 {
		return true
	}
	if len(candidates) == 1 {
		if parameter, variadic := functionVariadicParameter(candidates[0]); variadic {
			view := candidates[0]
			view.Parameters = []FunctionParameter{parameter}
			if !a.validateCallArgumentOwnership(view, trailing, nil, call) || !a.validateCallArgumentBorrows(view, trailing, nil) {
				return false
			}
			a.recordCallReferencePlans(view, trailing, nil)
			a.markMovedCallArguments(view, trailing, nil, false)
			return true
		}
	}
	for index, argument := range trailing {
		if _, moved := explicitMoveArgument(argument); moved || !implicitlyCopyable(types[index]) {
			a.typePackForwardUnavailable(expressionToken(argument), "with an ordinary argument after a pack spread that must move or borrow")
			return false
		}
	}
	return true
}

// inferTypePackForward checks a call expanding the enclosing pack once, in its
// template: the candidates, the mode of the source pack, loans and the consuming
// transfer. Arity, position types and overload selection are proved per
// specialization by typePackInstances.
func (a *Analyzer) inferTypePackForward(call *ast.CallExpression, functions []Function, receiver methodReceiverInfo, isMethodCall bool) (Type, expressionValue) {
	invalid := Type{Kind: InvalidType}
	value := expressionValue{Display: call.String()}
	token := call.Token
	forward := &typePackForward{call: call, template: sourceTokenLocation(a.currentFunctionToken)}
	var packIdentifier *ast.Identifier
	spreads := 0
	prefix, trailing := []ast.Expression{}, []ast.Expression{}
	prefixTypes, trailingTypes := []Type{}, []Type{}
	for _, argument := range call.Arguments {
		move, spread, consuming := consumingPackSpread(argument)
		if !consuming {
			spread, _ = argument.(*ast.SpreadExpression)
		}
		if spread == nil {
			typ, _ := a.inferCallArgumentExpression(argument)
			if typ.Kind == InvalidType {
				return invalid, value
			}
			forward.arguments = append(forward.arguments, typePackForwardArgument{expression: argument, typ: typ})
			if spreads == 0 {
				prefix, prefixTypes = append(prefix, argument), append(prefixTypes, typ)
			} else {
				trailing, trailingTypes = append(trailing, argument), append(trailingTypes, typ)
			}
			continue
		}
		identifier, element, declared := a.declaredPackSpreadSource(spread)
		if !declared {
			a.typePackForwardUnavailable(spread.Token, "mixed with an array or slice spread")
			return invalid, value
		}
		if typ, _ := a.inferExpression(identifier); typ.Kind == InvalidType {
			return invalid, value
		}
		mode := typePackSourceModeOf(element)
		if consuming && mode != typePackSourceOwned {
			a.addErrorAtToken(move.Token, "<-%s... needs a currently owned pack; %s is %s, so forward it with %s...", identifier.Value, identifier.Value, mode, identifier.Value)
			return invalid, value
		}
		if spreads != 0 && (mode == typePackSourceMutable || consuming || forward.consuming) {
			a.addErrorAtToken(spread.Token, "pack %s cannot be expanded twice in one call when an expansion moves it or lends it mutably", identifier.Value)
			return invalid, value
		}
		packIdentifier = identifier
		forward.pack = typePackNameOf(element)
		forward.mode = mode
		forward.consuming = forward.consuming || consuming
		forward.arguments = append(forward.arguments, typePackForwardArgument{expression: identifier, typ: element, spread: true})
		spreads++
	}
	candidates, receiver, ok := a.typePackForwardCandidates(call, functions, receiver, isMethodCall, len(prefix))
	if !ok {
		return invalid, value
	}
	forward.targets = candidates
	target := candidates[0]
	if len(candidates) == 1 {
		if problem := typePackForwardTemplateModeProblem(target, packIdentifier.Value, forward.mode, forward.consuming); problem != "" {
			a.addErrorAtToken(packIdentifier.Token, "%s", problem)
			return invalid, value
		}
	}
	for _, candidate := range candidates {
		if candidate.Unsafe && !a.callUnsafeAuthorized(call) {
			a.addErrorAtToken(token, "calling unsafe function %s requires unsafe", candidate.Name)
			return invalid, value
		}
	}
	returnType, known := a.typePackForwardCommonContract(call, candidates, prefix, prefixTypes)
	if !known {
		return invalid, value
	}
	if functionUsesReceiver(target) && !a.validateTypePackReceiverArguments(target, receiver, prefix) {
		return invalid, value
	}
	if !a.validateCallArgumentOwnership(target, prefix, nil, call) || !a.validateCallArgumentBorrows(target, prefix, nil) {
		return invalid, value
	}
	if !a.checkTypePackForwardLoans(forward, packIdentifier, prefix) {
		return invalid, value
	}
	if forward.mode == typePackSourceOwned && !forward.consuming {
		if place, ok := a.resolvePlace(packIdentifier); ok {
			forward.borrow = ResolvedReferencePlan{Facts: a.referenceFactsForPlace(place, false, packIdentifier.Token), Place: place, Source: packIdentifier.Token}
		}
	}
	a.recordCallReferencePlans(target, prefix, nil)
	if !a.typePackForwardTrailing(call, candidates, trailing, trailingTypes) {
		return invalid, value
	}
	dispatch := CallDispatchDirect
	if functionUsesReceiver(target) {
		dispatch = CallDispatchStaticMethod
		if member, isMember := call.Callee.(*ast.MemberExpression); isMember {
			a.recordReceiverReferencePlan(target, member.Object)
		}
		a.recordReceiverReborrow(call, target)
	} else if target.Static && target.ImplTarget != "" {
		dispatch = CallDispatchStaticMethod
	}
	if len(candidates) == 1 {
		a.setDefinitions(callCalleeDefinitionToken(call), target.Token)
	}
	a.recordResolvedCall(call, target, dispatch, receiver.Type)
	if execution, recordCall := a.callGraphExecutionForCall(call); !a.summaryPass && a.callGraphPathReachable && recordCall {
		for _, candidate := range candidates {
			a.callGraph.addCall(a.currentCallable, candidate, callCalleeDefinitionToken(call), dispatch, execution)
		}
	}
	a.setTypePackForwardOrigin(call, forward, candidates, prefix, trailing, packIdentifier, receiver.Expr)
	a.markMovedCallArguments(target, prefix, nil, false)
	if forward.consuming && a.markExplicitMoveSource(packIdentifier) {
		a.availabilityStates[packIdentifier.Value] = unavailableState("consumed by call")
		a.endBorrowsHeldBy(packIdentifier.Value)
	}
	if a.typePackForwards == nil {
		a.typePackForwards = map[*ast.CallExpression]*typePackForward{}
	}
	a.typePackForwards[call] = forward
	return returnType, value
}

// setTypePackForwardOrigin gives a forward's returned reference a finite origin
// set. Which position a specialization returns from is per specialization, so any
// summary root beyond the ordinary leading arguments stands for the whole pack: a
// superset that keeps every proof sound and every exclusive grant live. An owned
// pack's storage ends with the invocation, so its origin is local and cannot be
// returned, only used.
// Rules: rules/declarations/functions.md §38A(8)(1–5); rules/memory/lifetime_analysis.md §28.
func (a *Analyzer) setTypePackForwardOrigin(call *ast.CallExpression, forward *typePackForward, candidates []Function, prefix, trailing []ast.Expression, pack *ast.Identifier, receiver ast.Expression) {
	if candidates[0].ReturnType.Kind != ReferenceType {
		return
	}
	for index := range candidates {
		candidate := candidates[index]
		if a.prepareReferenceSummaryCall(call, &candidate) {
			return
		}
		candidates[index] = candidate
	}
	delete(a.expressionReferenceOrigins, call)
	origins := []localReferenceOrigin{}
	for _, candidate := range candidates {
		if !candidate.HasReturnOrigin {
			return
		}
		leading := []Place{}
		wholePack := false
		for _, place := range localOriginPlaces(candidate.ReturnOrigin) {
			if a.originRootInLeadingArguments(place.Root, len(prefix)) {
				leading = append(leading, place)
			} else {
				wholePack = true
			}
		}
		exact := localOriginWithPlaces(localReferenceOrigin{Mutable: candidate.ReturnOrigin.Mutable}, leading)
		exact.Unknown, exact.Ambiguous = candidate.ReturnOrigin.Unknown, candidate.ReturnOrigin.Ambiguous
		if len(leading) != 0 || exact.Unknown {
			origins = append(origins, a.instantiateFunctionReturnOrigin(exact, prefix, receiver, call))
		}
		if wholePack {
			origins = append(origins, a.typePackForwardPackOrigin(forward, pack))
			for _, argument := range trailing {
				if origin, known := a.directReferenceOrigin(argument); known {
					origins = append(origins, origin)
				}
			}
		}
	}
	origin := mergeLocalReferenceOrigins(origins, false)
	a.expressionReferenceOrigins[call] = origin
	a.recordCallReferenceFacts(call, candidates[0], origin)
}

func (a *Analyzer) originRootInLeadingArguments(root string, leading int) bool {
	if root == "$receiver" || strings.HasPrefix(root, "$static:") {
		return true
	}
	if !strings.HasPrefix(root, "$param:") {
		return false
	}
	index, err := strconv.Atoi(strings.TrimPrefix(root, "$param:"))
	return err == nil && index < leading
}

func (a *Analyzer) typePackForwardPackOrigin(forward *typePackForward, pack *ast.Identifier) localReferenceOrigin {
	place, ok := a.resolvePlace(pack)
	if !ok {
		return localReferenceOrigin{Unknown: true, Ambiguous: true}
	}
	return localOriginWithPlaces(localReferenceOrigin{Name: pack.Value, Token: pack.Token, Local: forward.mode == typePackSourceOwned}, placeOriginAlternatives(place))
}

// typePackForwardTemplateModeProblem rejects what no specialization can make
// valid when the target's trailing parameter fixes the receiving mode.
func typePackForwardTemplateModeProblem(target Function, name string, mode typePackSourceMode, consuming bool) string {
	parameter, variadic := functionVariadicParameter(target)
	if !variadic {
		return ""
	}
	// An owned pack target receives a held reference as its reference-typed
	// element, an ordinary borrowed parameter (generics.md §16A(3)).
	if parameter.Type.Kind == GenericTypePackType && mode != typePackSourceOwned {
		return ""
	}
	return typePackForwardModeProblem(parameter, mode, consuming, name)
}

// typePackForwardModeProblem is the §38A(7) rule for one receiving parameter.
// Ownership moves only as a whole pack, authority never grows and a `ref mut`
// is never copied.
func typePackForwardModeProblem(parameter FunctionParameter, mode typePackSourceMode, consuming bool, name string) string {
	mutable, borrowed := callParameterBorrowMode(parameter)
	switch {
	case consuming && borrowed:
		return fmt.Sprintf("<-%s... moves the pack, but parameter %s borrows; pass %s... to lend it", name, parameter.Name, name)
	case mode == typePackSourceOwned && mutable:
		return fmt.Sprintf("parameter %s needs ref mut, but the owned pack %s is read-only; a pack never grants mutable access to its own storage", parameter.Name, name)
	case mode == typePackSourceShared && mutable:
		return fmt.Sprintf("parameter %s needs ref mut, but %s holds shared references; a shared reference never grants mutation", parameter.Name, name)
	case mode != typePackSourceOwned && !borrowed:
		return fmt.Sprintf("parameter %s takes its value, but %s holds references, which are never dereferenced implicitly and a ref mut never copied; declare the parameter as a ref or ref mut", parameter.Name, name)
	case mode == typePackSourceOwned && !consuming && parameter.Consuming:
		return fmt.Sprintf("parameter %s takes ownership; write <-%s... to transfer the complete pack", parameter.Name, name)
	}
	return ""
}

// checkTypePackForwardLoans holds the pack against live loans and the other
// arguments of the call: forwarding lends or moves every position at once.
func (a *Analyzer) checkTypePackForwardLoans(forward *typePackForward, identifier *ast.Identifier, prefix []ast.Expression) bool {
	place, ok := a.resolvePlace(identifier)
	if !ok {
		return false
	}
	exclusive := forward.consuming || forward.mode == typePackSourceMutable
	if a.checkPackForwardLiveLoans(place, exclusive, identifier.Token) {
		return false
	}
	for index, argument := range prefix {
		other, reusable := a.callArgumentBorrowPlace(argument)
		if !reusable {
			other, reusable = a.resolvePlace(explicitMoveSource(argument))
		}
		if !reusable || !placesMayOverlap(place, other) {
			continue
		}
		parameter, known := functionParameterForArgument(forward.targets[0], index)
		lendsMutably := known && parameter.Type.Kind == ReferenceType && parameter.Type.ReferenceMutable
		if exclusive || lendsMutably {
			a.addErrorAtTokenWithMetadata(expressionToken(argument), diagnostics.BorrowMutableConflict,
				"Pass the pack on its own, or end this use before the call.",
				"argument %d uses %s while the same call expands the complete pack %s", index+1, other.String(), identifier.Value)
			return false
		}
	}
	return true
}

// checkPackForwardLiveLoans applies the borrow-creation conflict rules without
// the binding-mutability test: a ref mut pack's authority is in its elements,
// and an owned pack is only lent shared or moved.
func (a *Analyzer) checkPackForwardLiveLoans(place Place, exclusive bool, token lexer.Token) bool {
	place.Mutable = exclusive
	return a.checkBorrowCreationPlace(place, exclusive, token)
}

// typePackForwardReturn is the template's result type, which ordinary arguments
// before the first spread must determine: a pack never selects a result type.
func (a *Analyzer) typePackForwardReturn(target Function, prefix []ast.Expression, types []Type) (Type, bool) {
	if !typePackPatternNeedsInference(target.ReturnType) {
		return target.ReturnType, true
	}
	substitution := map[string]Type{}
	for index, typ := range types {
		if index >= functionFixedParameterCount(target) {
			break
		}
		pattern := target.Parameters[index].Type
		if pattern.Kind == ReferenceType && pattern.Element != nil && typ.Kind != ReferenceType {
			pattern = *pattern.Element
		}
		if typePackPatternNeedsInference(pattern) && !inferGenericArgumentSubstitution(pattern, a.conformingViewArgument(pattern, typ, substitution), substitution) {
			return Type{}, false
		}
	}
	result := substituteGenericType(target.ReturnType, substitution)
	return result, !typePackPatternNeedsInferenceForParameters(result, target.GenericParameters)
}

func typePackPatternNeedsInferenceForParameters(typ Type, parameters []string) bool {
	for _, name := range parameters {
		if typePackPatternNeedsInference(typ, name) {
			return true
		}
	}
	return false
}

// typePackForwardInstance selects the concrete callee of a forwarding call in
// the specialization given by outer. known is false while a position is still
// opaque; problem is a message when this specialization makes the call invalid.
func (a *Analyzer) typePackForwardInstance(forward *typePackForward, outer map[string]Type) (resolved ResolvedTypePackForward, known bool, problem string) {
	pack, ok := outer[forward.pack]
	if !ok || pack.Kind != GenericTypePackType {
		return resolved, false, ""
	}
	types := []Type{}
	expanded := []ResolvedTypePackForwardArgument{}
	for source, argument := range forward.arguments {
		if !argument.spread {
			typ := SubstituteGenericType(argument.typ, outer)
			if typePackPatternNeedsInference(typ) {
				return resolved, false, ""
			}
			types = append(types, typ)
			expanded = append(expanded, ResolvedTypePackForwardArgument{Source: source, Position: -1})
			continue
		}
		for position, element := range pack.TypeArgs {
			if typePackPatternNeedsInference(element) {
				return resolved, false, ""
			}
			typ := element
			if forward.mode != typePackSourceOwned {
				referent := element
				mutable := forward.mode == typePackSourceMutable
				typ = Type{Name: referenceTypeName(referent, mutable), Kind: ReferenceType, Element: &referent, ReferenceMutable: mutable}
			}
			types = append(types, typ)
			expanded = append(expanded, ResolvedTypePackForwardArgument{Source: source, Position: position})
		}
	}
	if len(forward.targets) == 1 {
		resolved, _, problem = a.typePackForwardCandidate(forward, forward.targets[0], pack, types, expanded)
		return resolved, true, problem
	}
	// Overload selection is unchanged (§38A(4)(5)): ordinary ranking over the
	// concrete expansion, a fixed signature winning a tie, and ambiguity an error.
	matches := []overloadMatch{}
	found := map[sourceTokenKey]ResolvedTypePackForward{}
	for _, candidate := range forward.targets {
		selected, rank, rejected := a.typePackForwardCandidate(forward, candidate, pack, types, append([]ResolvedTypePackForwardArgument(nil), expanded...))
		if rejected != "" {
			continue
		}
		matches = append(matches, overloadMatch{Function: selected.Function, Rank: rank})
		found[sourceTokenLocation(candidate.Token)] = selected
	}
	name := forward.call.Callee.String()
	best := bestOverloadMatches(matches)
	switch len(best) {
	case 0:
		return resolved, true, fmt.Sprintf("no overload of %s accepts this expansion of %d arguments", name, len(types))
	case 1:
		return found[sourceTokenLocation(best[0].Function.Token)], true, ""
	}
	return resolved, true, fmt.Sprintf("ambiguous call to %s for this expansion of %d arguments", name, len(types))
}

// typePackForwardCandidate proves one candidate for the concrete expansion and
// ranks it; a non-empty problem rejects it.
func (a *Analyzer) typePackForwardCandidate(forward *typePackForward, target Function, pack Type, types []Type, expanded []ResolvedTypePackForwardArgument) (ResolvedTypePackForward, int, string) {
	var resolved ResolvedTypePackForward
	function, problem := a.typePackForwardCallee(target, types)
	if problem != "" {
		return resolved, 0, problem
	}
	name := forward.call.Callee.String()
	rank := 0
	for index := range expanded {
		parameter, found := functionParameterForArgument(function, index)
		if !found {
			return resolved, 0, fmt.Sprintf("%s has no parameter for expanded argument %d", name, index+1)
		}
		rank += overloadArgumentRank(parameter.Type, types[index])
		expanded[index].Parameter = parameter
		argument := forward.arguments[expanded[index].Source]
		if !argument.spread {
			// MD-057: a conforming argument passes as a borrowed interface view,
			// exactly as in a call without a spread.
			if !a.canPassCallArgumentUnrecorded(parameter.Type, types[index], argument.expression) && !a.borrowedInterfaceView(parameter.Type, types[index]) {
				return resolved, 0, fmt.Sprintf("argument %d of type %s cannot be passed to parameter %s of type %s", index+1, typeDisplayName(types[index]), parameter.Name, typeDisplayName(parameter.Type))
			}
			continue
		}
		position := expanded[index].Position
		element := pack.TypeArgs[position]
		sourceName := argument.expression.String()
		// A reference element of an owned pack passes like any held reference;
		// moving the complete pack moves the holder (§38A(7)(6)).
		mode, consuming := forward.mode, forward.consuming
		if mode == typePackSourceOwned && element.Kind == ReferenceType {
			mode, consuming = typePackSourceModeOf(element), false
		}
		if detail := typePackForwardModeProblem(parameter, mode, consuming, sourceName); detail != "" {
			return resolved, 0, fmt.Sprintf("pack position %d of type %s: %s", position, typeDisplayName(element), detail)
		}
		if !a.canPassCallArgumentUnrecorded(dereferenceType(parameter.Type), dereferenceType(element), nil) && !a.borrowedInterfaceView(parameter.Type, element) {
			return resolved, 0, fmt.Sprintf("pack position %d of type %s cannot be passed to parameter %s of type %s", position, typeDisplayName(element), parameter.Name, typeDisplayName(parameter.Type))
		}
		if _, borrowed := callParameterBorrowMode(parameter); mode == typePackSourceOwned && !borrowed && !consuming {
			if classification := CopyClassificationOf(element); classification == CopyMoveOnly || classification == CopyNonCopyable {
				return resolved, 0, fmt.Sprintf("%s... copies its elements, but pack position %d of type %s is not implicitly copyable; write <-%s... to transfer the complete pack", sourceName, position, typeDisplayName(element), sourceName)
			}
		}
	}
	return ResolvedTypePackForward{Function: function, Consuming: forward.consuming, Mode: forward.mode.String(), Borrow: forward.borrow, Expanded: expanded}, rank, ""
}

// typePackForwardCallee instantiates the target for the expanded argument types.
func (a *Analyzer) typePackForwardCallee(target Function, types []Type) (Function, string) {
	if len(target.GenericParameters) == 0 {
		if !functionAcceptsArgumentCount(target, len(types)) {
			return Function{}, fmt.Sprintf("%s expects %d arguments, but this specialization expands to %d", target.Name, functionFixedParameterCount(target), len(types))
		}
		return target, ""
	}
	instance, ok := a.inferGenericFunctionInstance(target, types)
	if !ok {
		return Function{}, fmt.Sprintf("%s cannot be instantiated for the expanded argument types", target.Name)
	}
	if failure := instance.GenericConstraintFailure; failure != nil {
		return Function{}, fmt.Sprintf("type %s does not satisfy constraint %s for %s", typeDisplayName(failure.Argument), typeDisplayName(failure.Interface), failure.Parameter)
	}
	return instance, ""
}

// typePackInstance is one specialization a source call demands: directly, or
// through the forwarding calls of the templates it reaches.
type typePackInstance struct {
	origin   *ast.CallExpression
	function Function
}

// typePackInstances walks forwarding calls from each direct call of a pack
// template, proving every forward for its concrete sequence. A sequence still
// holding generic elements is proved by Semantic IR once they are concrete.
// Each error is reported at the source call and names the forwarding call.
func (a *Analyzer) typePackInstances() []typePackInstance {
	calls := a.callsInSourceOrder(func(function Function) bool {
		_, known := inferredTypePack(function)
		return known
	})
	forwards := a.typePackForwardsByTemplate()
	instances := make([]typePackInstance, 0, len(calls))
	limitReported := false
	for _, call := range calls {
		// One specialization demanded by two source calls is reported at both.
		seen := map[string]bool{}
		pending := []Function{a.resolvedCalls[call].Function}
		for len(pending) > 0 {
			function := pending[0]
			pending = pending[1:]
			key := typePackInstanceKey(function)
			if seen[key] {
				continue
			}
			seen[key] = true
			instances = append(instances, typePackInstance{origin: call, function: function})
			sequence, _ := inferredTypePack(function)
			if typePackPatternNeedsInference(Type{TypeArgs: sequence}) {
				continue
			}
			if len(seen) > maxTypePackForwardInstances {
				if !limitReported {
					a.reportTypePackForwardProblem(call, call, fmt.Sprintf("pack forwarding from %s exceeds the limit of %d specializations", function.Name, maxTypePackForwardInstances))
				}
				limitReported = true
				break
			}
			for _, forward := range forwards[sourceTokenLocation(function.Token)] {
				resolved, known, problem := a.typePackForwardInstance(forward, typePackIndexedSubstitution(forward.pack, function.GenericSubstitution))
				if problem != "" {
					a.reportTypePackForwardProblem(call, forward.call, problem)
					continue
				}
				if !known {
					continue
				}
				if _, pack := inferredTypePack(resolved.Function); pack {
					pending = append(pending, resolved.Function)
				}
			}
		}
	}
	return instances
}

// typePackIndexedSubstitution binds the `Types[k]` of constant indexes, which
// an ordinary argument of a forward may name.
func typePackIndexedSubstitution(pack string, substitution map[string]Type) map[string]Type {
	result := make(map[string]Type, len(substitution))
	for name, typ := range substitution {
		result[name] = typ
	}
	for position, element := range substitution[pack].TypeArgs {
		result[TypePackElementName(pack, strconv.Itoa(position))] = element
	}
	return result
}

// typePackInstanceKey identifies one specialization of a pack template.
func typePackInstanceKey(function Function) string {
	names := make([]string, 0, len(function.GenericSubstitution))
	for name := range function.GenericSubstitution {
		names = append(names, name)
	}
	sort.Strings(names)
	arguments := make([]Type, len(names))
	for index, name := range names {
		arguments[index] = function.GenericSubstitution[name]
	}
	return functionDeclarationIdentity(function) + "|" + canonicalTypeArgumentsKey(arguments)
}

func (a *Analyzer) typePackForwardsByTemplate() map[sourceTokenKey][]*typePackForward {
	grouped := map[sourceTokenKey][]*typePackForward{}
	for _, forward := range a.typePackForwards {
		grouped[forward.template] = append(grouped[forward.template], forward)
	}
	for key := range grouped {
		list := grouped[key]
		sort.Slice(list, func(i, j int) bool {
			if list[i].call.Token.File != list[j].call.Token.File {
				return list[i].call.Token.File < list[j].call.Token.File
			}
			return list[i].call.Token.ByteStart < list[j].call.Token.ByteStart
		})
	}
	return grouped
}

func (a *Analyzer) reportTypePackForwardProblem(origin, forward *ast.CallExpression, problem string) {
	endLine, endColumn := origin.Token.EndPosition()
	a.appendError(Error{Severity: diagnostics.SeverityError, Message: problem,
		File: origin.Token.File, Line: origin.Token.Line, Column: origin.Token.Column, EndLine: endLine, EndColumn: endColumn,
		PreviousFile: forward.Token.File, PreviousLine: forward.Token.Line, PreviousColumn: forward.Token.Column, RelatedLabel: "pack forwarding call"})
}

// IsTypePackForward reports a call that expands the enclosing template's pack.
func (a *Analyzer) IsTypePackForward(call *ast.CallExpression) bool {
	if a == nil || call == nil {
		return false
	}
	_, ok := a.typePackForwards[call]
	return ok
}

// ResolvedTypePackForwardOf resolves a forwarding call for the specialization
// whose substitution is outer. Facts are detached like every post-Sema call fact.
func (a *Analyzer) ResolvedTypePackForwardOf(call *ast.CallExpression, outer map[string]Type) (ResolvedTypePackForward, string) {
	forward, ok := a.typePackForwards[call]
	if !ok {
		return ResolvedTypePackForward{}, "pack forwarding call without resolved template facts"
	}
	resolved, known, problem := a.typePackForwardInstance(forward, outer)
	if problem != "" {
		return ResolvedTypePackForward{}, problem
	}
	if !known {
		return ResolvedTypePackForward{}, "pack forwarding call with an unresolved element"
	}
	for index := range resolved.Expanded {
		resolved.Expanded[index].Parameter.Type = semanticSnapshotType(resolved.Expanded[index].Parameter.Type)
	}
	return resolved, ""
}

// TypePackForwardBorrowIdentity gives one expanded reference argument of a
// forwarding call its own borrow and lifetime identity.
func (a *Analyzer) TypePackForwardBorrowIdentity(call *ast.CallExpression, index int) (BorrowID, LifetimeID) {
	key := fmt.Sprintf("type-pack-forward:%s:%d:%d:%d", call.Token.File, call.Token.Line, call.Token.Column, index)
	return a.placeIdentities.borrow(key), a.placeIdentities.lifetime(key)
}

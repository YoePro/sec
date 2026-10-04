package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// For statement analysis: iterable classification, binding types, range and
// step validation, and loop-body flow.

// analyzeForStatement validates one for statement: the once-evaluated
// iterable, binding count and types, the loop-body scope, loop-binding
// immutability, and the zero-iteration and break continuation paths.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §2 "Core forms", §4 "Loop bindings", §10 "Loop-binding immutability", §11 "Scope and shadowing"
//   - rules/control-flow/flowcontrol_for.md — §30 "`break`", §31 "`continue`", §33 "Definite assignment after finite loops"
func (a *Analyzer) analyzeForStatement(stmt *ast.ForStatement) {
	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	previousLoopDepth := a.loopDepth
	frame := a.pushLoopBreakFrame()

	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = copyMoved(previousMoved)
	a.moveReasons = copyMoveReasons(previousMoveReasons)
	a.closedResources = copyMoved(previousClosedResources)
	a.borrows = copyBorrows(previousBorrows)
	a.localRefContainers = copyLocalRefContainers(previousLocalRefContainers)
	a.arenaGenerations = copyArenaGenerations(previousArenaGenerations)
	a.loopDepth++

	if len(stmt.Bindings) > 0 || stmt.Iterable != nil {
		a.analyzeForIterable(stmt)
	}
	iterationEntry := a.captureLoopIterationAnalysisState()

	activeIterations := len(a.activeCollectionIterations)
	if active, ok := a.activeCollectionIterationFor(stmt); ok {
		a.activeCollectionIterations = append(a.activeCollectionIterations, active)
	}
	if stmt.Body != nil {
		a.enterLoopBodyFacts()
		a.analyzeBlockStatements(stmt.Body)
	}
	a.activeCollectionIterations = a.activeCollectionIterations[:activeIterations]

	loopMoved := a.moved
	loopMoveReasons := a.moveReasons
	loopClosedResources := a.closedResources
	loopBorrows := a.borrows
	loopLocalRefContainers := a.localRefContainers
	loopArenaGenerations := a.arenaGenerations
	frameState := a.loopBreakFrames[frame]
	for _, binding := range stmt.Bindings {
		if binding.Discard {
			continue
		}
		clearRootPlaceStateMaps(loopMoved, loopMoveReasons, binding.Name)
		for index := range frameState.moved {
			clearRootPlaceStateMaps(frameState.moved[index], frameState.moveReasons[index], binding.Name)
		}
		for index := range frameState.continueMoved {
			clearRootPlaceStateMaps(frameState.continueMoved[index], frameState.continueReasons[index], binding.Name)
		}
		delete(loopClosedResources, binding.Name)
		for index := range frameState.closedResources {
			delete(frameState.closedResources[index], binding.Name)
		}
		for index := range frameState.continueClosedResources {
			delete(frameState.continueClosedResources[index], binding.Name)
		}
		clearLoopBindingReferenceState(loopBorrows, loopLocalRefContainers, binding.Name)
		for index := range frameState.borrows {
			clearLoopBindingReferenceState(frameState.borrows[index], frameState.localRefContainers[index], binding.Name)
		}
		for index := range frameState.continueBorrows {
			clearLoopBindingReferenceState(frameState.continueBorrows[index], frameState.continueLocalRefContainers[index], binding.Name)
		}
	}
	a.loopBreakFrames[frame] = frameState
	bodyFallsThrough := a.blockCanFallThrough(stmt.Body)
	headerMoved, headerReasons := loopBackedgeMoveState(iterationEntry.moved, iterationEntry.moveReasons, loopMoved, loopMoveReasons, frameState, bodyFallsThrough)
	headerClosedResources := loopBackedgeClosedResourceState(iterationEntry.closedResources, loopClosedResources, frameState, bodyFallsThrough)
	headerBorrows := loopBackedgeBorrowState(iterationEntry.borrows, loopBorrows, frameState, bodyFallsThrough)
	headerLocalRefContainers := loopBackedgeReferenceState(iterationEntry.localRefContainers, loopLocalRefContainers, frameState, bodyFallsThrough)
	headerArenaGenerations := loopBackedgeArenaGenerationState(iterationEntry.arenaGenerations, loopArenaGenerations, frameState, bodyFallsThrough)
	a.checkLoopBackedgeFixedPoint(nil, stmt.Body, iterationEntry, headerMoved, headerReasons, headerClosedResources, headerBorrows, headerLocalRefContainers, headerArenaGenerations)
	breakFrame := a.popLoopBreakFrame(frame)
	a.symbols = previousSymbols
	a.constInts = previousConstInts
	a.assigned = previousAssigned
	a.moved, a.moveReasons = mergeLoopMoveState(previousMoved, previousMoveReasons, loopMoved, loopMoveReasons, breakFrame)
	a.closedResources = mergeLoopClosedResourceState(previousClosedResources, loopClosedResources, breakFrame, bodyFallsThrough, false)
	a.borrows = mergeLoopBorrowState(previousBorrows, loopBorrows, breakFrame)
	a.localRefContainers = mergeLoopReferenceState(previousLocalRefContainers, loopLocalRefContainers, breakFrame)
	a.arenaGenerations = mergeLoopArenaGenerations(previousArenaGenerations, loopArenaGenerations, breakFrame.arenaGenerations)
	a.loopDepth = previousLoopDepth
}

// analyzeForIterable resolves the iterable expression exactly once and
// classifies it as a range or a collection/iterator source.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §12 "Iterable expression evaluation", §13 "Sec 0.1 iterable categories"
func (a *Analyzer) analyzeForIterable(stmt *ast.ForStatement) {
	if stmt.Iterable == nil {
		a.addErrorAtToken(stmt.Token, "for loop requires an iterable expression")
		return
	}

	bindingTypes, ok := a.inferForIterableBindingTypes(stmt)
	if !ok {
		return
	}

	if len(stmt.Bindings) != len(bindingTypes) {
		if len(stmt.Bindings) > 0 {
			a.addErrorAtToken(stmt.Bindings[0].Token, "iteration over %s requires %d loop binding(s), got %d", forIterableKind(stmt.Iterable), len(bindingTypes), len(stmt.Bindings))
		}
		return
	}

	bindingTypes = a.applyForBindingModes(stmt, bindingTypes)

	for i, binding := range stmt.Bindings {
		if binding.Discard {
			continue
		}
		if a.defineSymbol(binding.Name, bindingTypes[i], false, binding.Token) {
			a.assigned[binding.Name] = true
		}
	}
}

// forIterationCategory classifies the iterable for binding-mode validation.
type forIterationCategory int

const (
	forCategoryRange forIterationCategory = iota
	forCategoryString
	forCategoryIterator
	forCategorySet
	forCategoryMap
	forCategorySequential
)

// forIterationSource returns the iterable category and whether the iterable is
// itself a reference, and if so whether that reference is mutable.
func (a *Analyzer) forIterationSource(stmt *ast.ForStatement) (forIterationCategory, bool, bool) {
	if _, ok := stmt.Iterable.(*ast.RangeExpression); ok {
		return forCategoryRange, false, false
	}
	if fact, ok := a.resolvedForIterations[stmt]; ok && fact.Kind == ForIterationCompilerKnownIterator {
		return forCategoryIterator, false, false
	}
	iterableType := a.expressionTypes[stmt.Iterable]
	isReference := iterableType.Kind == ReferenceType
	mutableReference := isReference && iterableType.ReferenceMutable
	base := dereferenceType(iterableType)
	switch {
	case base.Kind == StringType:
		return forCategoryString, isReference, mutableReference
	case base.Name == "Set" || base.Name == "set":
		return forCategorySet, isReference, mutableReference
	case base.Name == "Map" || base.Name == "map":
		return forCategoryMap, isReference, mutableReference
	}
	return forCategorySequential, isReference, mutableReference
}

// applyForBindingModes validates each binding position's mode against the
// iterable category and source authority and returns the symbol type of every
// position: the yielded type for plain bindings and `ref T` or `ref mut T`
// carrying the iterable's reference origin for reference bindings. A rejected
// mode still yields its written reference type so the body does not cascade
// into undefined names.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §4 "Loop bindings", §5 "Plain by-value iteration"
//   - rules/control-flow/flowcontrol_for.md — §6 "Shared element iteration", §7 "Mutable element iteration"
//   - rules/control-flow/flowcontrol_for.md — §14 "Sequential collections", §17 "Slices", §19 "Strings", §20 "Sets", §21 "Maps"
//   - rules/control-flow/flowcontrol_for.md — §37 "Compiler-known `Iterator[T]`", §40 "Unsupported forms"
func (a *Analyzer) applyForBindingModes(stmt *ast.ForStatement, bindingTypes []Type) []Type {
	category, isReference, mutableReference := a.forIterationSource(stmt)
	result := append([]Type{}, bindingTypes...)
	for i, binding := range stmt.Bindings {
		indexPosition := len(stmt.Bindings) == 2 && i == 0 && (category == forCategorySequential || category == forCategoryString)
		elementType := bindingTypes[i]
		if binding.Mode == ast.ForBindingValue {
			a.checkForPlainBindingCopy(binding, elementType, category, indexPosition)
			continue
		}
		mutable := binding.Mode == ast.ForBindingRefMut
		element := elementType
		result[i] = Type{Name: referenceTypeName(element, mutable), Kind: ReferenceType, Element: &element, ReferenceMutable: mutable}
		if !a.checkForBindingModeCategory(binding, elementType, category, indexPosition, i) {
			continue
		}
		if !a.checkForReferenceSource(stmt.Iterable, binding, isReference, mutableReference) {
			continue
		}
		originName, originToken, originLocal, originStorage, generation := a.referenceOriginForExpression(stmt.Iterable)
		result[i].ReferenceOriginName = originName
		result[i].ReferenceOriginToken = originToken
		result[i].ReferenceOriginLocal = originLocal
		result[i].ReferenceOriginStorage = originStorage
		result[i].ReferenceOriginGeneration = generation
		result[i].ReferenceOriginMatchScoped = a.referenceOriginMatchScopedForExpression(stmt.Iterable)
	}
	return result
}

// checkForPlainBindingCopy rejects a plain element binding whose yielded type
// is not implicitly copyable, since ordinary iteration never moves an element
// out of its collection. Index, range, rune, and iterator positions yield
// fresh values and are exempt.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §5 "Plain by-value iteration", §15 "Fixed arrays", §16 "Dynamic arrays and lists"
func (a *Analyzer) checkForPlainBindingCopy(binding ast.ForBinding, elementType Type, category forIterationCategory, indexPosition bool) {
	if binding.Discard || indexPosition || category == forCategoryRange || category == forCategoryString || category == forCategoryIterator {
		return
	}
	if elementType.Kind == GenericType || elementType.Kind == InvalidType {
		return
	}
	if classification := CopyClassificationOf(elementType); classification == CopyMoveOnly || classification == CopyNonCopyable {
		a.addErrorAtToken(binding.Token, "plain loop binding %s would copy each %s element, but %s is not implicitly copyable; use ref %s to inspect it", binding.Name, typeDisplayName(elementType), typeDisplayName(elementType), binding.Name)
	}
}

// checkForBindingModeCategory rejects reference modes the iterable category
// does not provide.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §14 "Sequential collections", §19 "Strings", §20 "Sets", §21 "Maps"
//   - rules/control-flow/flowcontrol_for.md — §37 "Compiler-known `Iterator[T]`", §40 "Unsupported forms"
func (a *Analyzer) checkForBindingModeCategory(binding ast.ForBinding, elementType Type, category forIterationCategory, indexPosition bool, position int) bool {
	mutable := binding.Mode == ast.ForBindingRefMut
	switch {
	case category == forCategoryRange:
		a.addErrorAtToken(binding.ModeToken, "%s loop bindings are not provided for synthesized range values; use a plain binding", binding.Mode)
	case indexPosition:
		a.addErrorAtToken(binding.ModeToken, "the sequential index binding is an int, not an element reference; remove %s", binding.Mode)
	case category == forCategoryString:
		a.addErrorAtToken(binding.ModeToken, "string iteration yields decoded rune values; %s rune bindings are not provided", binding.Mode)
	case category == forCategoryIterator:
		a.addErrorAtToken(binding.ModeToken, "Iterator[%s] loops yield owned values; %s bindings are not defined", typeDisplayName(elementType), binding.Mode)
	case mutable && category == forCategorySet:
		a.addErrorAtToken(binding.ModeToken, "set elements cannot be mutably borrowed during iteration; use explicit set operations to replace or remove values")
	case mutable && category == forCategoryMap && position == 0:
		a.addErrorAtToken(binding.ModeToken, "map keys cannot be mutably borrowed during iteration; use ref for the key")
	default:
		return true
	}
	return false
}

// checkForReferenceSource requires a reference loop binding to borrow from a
// stable addressable iterable, and a ref mut binding to have mutable authority:
// a ref mut reference source, or a mutable reusable place compatible with the
// active borrows.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §6 "Shared element iteration", §7 "Mutable element iteration", §17 "Slices"
//   - rules/memory/borrowing.md — §7 "Mutability authority"
func (a *Analyzer) checkForReferenceSource(iterable ast.Expression, binding ast.ForBinding, isReference bool, mutableReference bool) bool {
	mutable := binding.Mode == ast.ForBindingRefMut
	if isReference {
		if mutable && !mutableReference {
			a.addErrorAtToken(binding.ModeToken, "ref mut loop binding requires mutable element authority, but the iterable is a shared reference")
			return false
		}
		return true
	}
	place, ok := a.resolvePlace(iterable)
	if !ok || !place.Addressable {
		a.addErrorAtToken(binding.ModeToken, "%s loop binding requires a stable addressable iterable; bind the collection to a local first", binding.Mode)
		return false
	}
	if mutable && !place.Mutable {
		a.addErrorAtToken(binding.ModeToken, "ref mut loop binding requires a mutable iterable source; declare it with let mut or pass it as ref mut")
		return false
	}
	return !a.checkBorrowCreationPlace(place, mutable, binding.ModeToken)
}

// inferForIterableBindingTypes resolves the yielded binding types for each
// iterable category and validates the number of bindings.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §13 "Sec 0.1 iterable categories", §14–§21 per-category iteration
//   - rules/control-flow/flowcontrol_for.md — §37 "Compiler-known `Iterator[T]`"
func (a *Analyzer) inferForIterableBindingTypes(stmt *ast.ForStatement) ([]Type, bool) {
	switch iterable := stmt.Iterable.(type) {
	case *ast.RangeExpression:
		bindingType, ok := a.inferForRangeBindingType(iterable, stmt.Step)
		if !ok {
			return nil, false
		}
		return []Type{bindingType}, true
	default:
		if stmt.Step != nil {
			a.addErrorAtToken(expressionToken(stmt.Step), "for step is only valid for range iteration")
			return nil, false
		}
		iterableType, _ := a.inferExpression(iterable)
		if iterableType.Kind == InvalidType {
			return nil, false
		}
		if elementType, next, conformance, ok := a.compilerKnownIterator(iterableType); ok {
			if len(stmt.Bindings) != 1 {
				token := expressionToken(iterable)
				if len(stmt.Bindings) > 0 {
					token = stmt.Bindings[0].Token
				}
				a.addErrorAtToken(token, "Iterator[%s] iteration requires exactly one loop binding, got %d", typeDisplayName(elementType), len(stmt.Bindings))
				return nil, false
			}
			// Iterator.Next advances compiler-visible state. A fresh owned
			// temporary may become the loop's hidden local; reusable storage must
			// already carry mutable authority. No runtime borrow flag is created.
			plan := ResolvedForIteration{
				Kind:                    ForIterationCompilerKnownIterator,
				SourceType:              iterableType,
				ElementType:             elementType,
				Next:                    next,
				RequiresMutableReceiver: true,
				Conformance:             conformance,
				Source:                  ForIteratorFreshTemporary,
				Binding:                 ForIteratorOwnedValueBinding,
			}
			if place, reusable := a.resolvePlace(iterable); reusable && place.Addressable {
				if !place.Mutable {
					a.addErrorAtToken(expressionToken(iterable), "Iterator[%s] iteration requires a mutable iterator source", typeDisplayName(elementType))
					return nil, false
				}
				if a.checkBorrowedMutationPlace(place, expressionToken(iterable)) {
					return nil, false
				}
				plan.Source = ForIteratorReusableStorage
				plan.SourcePlace = place
			} else {
				plan.DestroysTemporary = !TriviallyDestructible(dereferenceType(iterableType))
			}
			if stmt.Bindings[0].Discard {
				plan.Binding = ForIteratorDiscardBinding
			}
			if next.Token.Line > 0 {
				plan.NextCallable = callableID(next)
				// Each iteration statically calls the concrete Next target, so
				// its effects, allocation, and reachability belong to the loop's
				// callable exactly like an explicit method call.
				if !a.summaryPass && a.callGraphPathReachable {
					a.callGraph.addCall(a.currentCallable, next, stmt.Token, CallDispatchStaticMethod, CallExecutionSynchronous)
				}
			}
			a.resolvedForIterations[stmt] = plan
			return []Type{elementType}, true
		}
		if iterableType.Kind == ReferenceType && iterableType.Element != nil &&
			(iterableType.Element.Kind == ArrayType || iterableType.Element.Kind == SliceType || isForCollectionFamily(*iterableType.Element)) {
			iterableType = *iterableType.Element
		}
		indexType := Type{Name: "int", Kind: IntType}
		if iterableType.Kind == StringType {
			// rules/library/core-library.md: string iteration yields the canonical
			// compiler-known rune type. Reusing the registered type is essential:
			// same-module impl properties such as Utf8Length and IsWhitespace are
			// attached there and must remain visible in the loop body.
			runeType := a.types["rune"]
			return a.inferSequentialForBindingTypes(stmt, runeType, indexType)
		}
		if (iterableType.Kind == ArrayType || iterableType.Kind == SliceType || iterableType.Kind == VariadicPackType) && iterableType.Element != nil {
			// rules/declarations/functions.md section 30 permits read-only
			// iteration over native variadic packs with their element type.
			return a.inferSequentialForBindingTypes(stmt, *iterableType.Element, indexType)
		}
		if (iterableType.Name == "Vec" || iterableType.Name == "list") && len(iterableType.TypeArgs) == 1 {
			return a.inferSequentialForBindingTypes(stmt, iterableType.TypeArgs[0], indexType)
		}
		if iterableType.Name == "vector" && len(iterableType.TypeArgs) == 1 && len(iterableType.ConstArgs) == 1 {
			return a.inferSequentialForBindingTypes(stmt, iterableType.TypeArgs[0], indexType)
		}
		if (iterableType.Name == "Set" || iterableType.Name == "set") && len(iterableType.TypeArgs) == 1 {
			if len(stmt.Bindings) > 1 {
				a.addErrorAtToken(stmt.Bindings[0].Token, "set iteration supports one loop binding, got %d", len(stmt.Bindings))
				return nil, false
			}
			return []Type{iterableType.TypeArgs[0]}, true
		}
		if (iterableType.Name == "Map" || iterableType.Name == "map") && len(iterableType.TypeArgs) == 2 {
			if len(stmt.Bindings) != 2 {
				a.addErrorAtToken(stmt.Bindings[0].Token, "map iteration requires key and value bindings, got %d", len(stmt.Bindings))
				return nil, false
			}
			return []Type{iterableType.TypeArgs[0], iterableType.TypeArgs[1]}, true
		}
		a.addErrorAtToken(expressionToken(iterable), "type %s is not iterable", typeDisplayName(iterableType))
		return nil, false
	}
}

// compilerKnownIterator resolves only explicit Iterator[T] conformance. The
// method name Next alone is deliberately insufficient: flowcontrol_for.md
// section 37 forbids naming-convention discovery, and no interface value or
// dynamic-dispatch runtime is introduced here.
func (a *Analyzer) compilerKnownIterator(source Type) (Type, Function, Type, bool) {
	concrete := dereferenceType(source)
	for _, iface := range concrete.Implements {
		if iface.Name != "Iterator" || iface.Kind != InterfaceType || len(iface.TypeArgs) != 1 {
			continue
		}
		element := iface.TypeArgs[0]
		for _, method := range a.functions[concrete.Name+".Next"] {
			if method.Static || len(explicitInterfaceComparableParameters(method.Parameters)) != 0 {
				continue
			}
			if method.ReturnType.Name != "Option" || len(method.ReturnType.TypeArgs) != 1 || !sameConcreteType(method.ReturnType.TypeArgs[0], element) {
				continue
			}
			method.CompilerKnownID = "CKM-ITERATOR-NEXT"
			return element, method, iface, true
		}
		// Preserve useful loop binding inference while ordinary interface
		// conformance emits the canonical missing/signature diagnostic.
		required := Function{Name: "Next", ImplTarget: concrete.Name, CompilerKnownID: "CKM-ITERATOR-NEXT", ReceiverMutable: true, ReturnType: Type{Name: "Option", Kind: UnionType, TypeArgs: []Type{element}}}
		return element, required, iface, true
	}
	return Type{}, Function{}, Type{}, false
}

// inferSequentialForBindingTypes types the one-binding (element) and
// two-binding (index, element) forms of sequential iteration.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §14 "Sequential collections"
func (a *Analyzer) inferSequentialForBindingTypes(stmt *ast.ForStatement, valueType Type, indexType Type) ([]Type, bool) {
	if len(stmt.Bindings) > 2 {
		a.addErrorAtToken(stmt.Bindings[0].Token, "sequential iteration supports one or two loop bindings, got %d", len(stmt.Bindings))
		return nil, false
	}
	if len(stmt.Bindings) == 2 {
		return []Type{indexType, valueType}, true
	}
	return []Type{valueType}, true
}

// isForCollectionFamily reports compiler-known sequential collection types.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §13 "Sec 0.1 iterable categories"
func isForCollectionFamily(typ Type) bool {
	switch typ.Name {
	case "Vec", "Set", "Map", "list", "set", "map", "vector":
		return true
	default:
		return false
	}
}

// forIterableKind names the iterable category for diagnostics.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §13 "Sec 0.1 iterable categories"
func forIterableKind(expr ast.Expression) string {
	if _, ok := expr.(*ast.RangeExpression); ok {
		return "range"
	}
	return "iterable"
}

// inferForRangeBindingType validates a finite range iterable and its optional
// step: compatible ordered numeric bounds that keep named-type identity, a
// non-zero step whose compile-time sign progresses toward the end, no negative
// step for unsigned ranges, and a mandatory step for float and decimal ranges.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §22 "Range forms", §24 "Range type compatibility"
//   - rules/control-flow/flowcontrol_for.md — §25 "Implicit range step", §26 "Explicit range step"
//   - rules/control-flow/flowcontrol_for.md — §27 "Unsigned descending ranges", §28 "Float and decimal ranges"
func (a *Analyzer) inferForRangeBindingType(expr *ast.RangeExpression, step ast.Expression) (Type, bool) {
	if expr.Start == nil || expr.End == nil {
		a.addErrorAtToken(expr.Token, "range used in for loop must be finite")
		return Type{Kind: InvalidType}, false
	}

	startType, _ := a.inferExpression(expr.Start)
	endType, _ := a.inferExpression(expr.End)
	if startType.Kind == InvalidType || endType.Kind == InvalidType {
		return Type{Kind: InvalidType}, false
	}

	if !sameConcreteType(startType, endType) {
		a.addErrorAtToken(expr.Token, "cannot create range with bounds %s and %s", typeDisplayName(startType), typeDisplayName(endType))
		return Type{Kind: InvalidType}, false
	}

	if step != nil {
		stepType, _ := a.inferExpression(step)
		if stepType.Kind == InvalidType {
			return Type{Kind: InvalidType}, false
		}
		if !canInitialize(startType, stepType, step) {
			a.addErrorAtToken(expressionToken(step), "for range step must be %s, got %s", typeDisplayName(startType), typeDisplayName(stepType))
			return Type{Kind: InvalidType}, false
		}
		if value, ok := a.integerConstantValue(step); ok && value.Sign() == 0 {
			a.addErrorAtToken(expressionToken(step), "for range step must not be zero")
			return Type{Kind: InvalidType}, false
		}
		if startType.Kind == UintType {
			if value, ok := a.integerConstantValue(step); ok && value.Sign() < 0 {
				a.addErrorAtToken(expressionToken(step), "unsigned range %s cannot use negative step %s; use a signed range type or while", typeDisplayName(startType), value.String())
				return Type{Kind: InvalidType}, false
			}
		}
		if startValue, startOK := a.integerConstantValue(expr.Start); startOK {
			if endValue, endOK := a.integerConstantValue(expr.End); endOK {
				if stepValue, stepOK := a.integerConstantValue(step); stepOK {
					if startValue.Cmp(endValue) < 0 && stepValue.Sign() < 0 {
						a.addErrorAtToken(expressionToken(step), "for ascending range step must be positive")
						return Type{Kind: InvalidType}, false
					}
					if startValue.Cmp(endValue) > 0 && stepValue.Sign() > 0 {
						a.addErrorAtToken(expressionToken(step), "for descending range step must be negative")
						return Type{Kind: InvalidType}, false
					}
				}
			}
		}
		if value, ok := decimalLiteralValue(step); ok && value.Int64 == 0 {
			a.addErrorAtToken(expressionToken(step), "for range step must not be zero")
			return Type{Kind: InvalidType}, false
		}
		if startValue, startOK := decimalLiteralValue(expr.Start); startOK {
			if endValue, endOK := decimalLiteralValue(expr.End); endOK {
				if stepValue, stepOK := decimalLiteralValue(step); stepOK {
					if startValue.Int64 < endValue.Int64 && stepValue.Int64 < 0 {
						a.addErrorAtToken(expressionToken(step), "for ascending range step must be positive")
						return Type{Kind: InvalidType}, false
					}
					if startValue.Int64 > endValue.Int64 && stepValue.Int64 > 0 {
						a.addErrorAtToken(expressionToken(step), "for descending range step must be negative")
						return Type{Kind: InvalidType}, false
					}
				}
			}
		}
	}

	if !isNumericType(startType) || !isNumericType(endType) {
		a.addErrorAtToken(expr.Token, "type %s is not iterable", typeDisplayName(startType))
		return Type{Kind: InvalidType}, false
	}
	if step == nil && (startType.Kind == FloatType || startType.Kind == DecimalType) {
		a.addErrorAtToken(expr.Token, "%s range iteration requires an explicit step", typeDisplayName(startType))
		return Type{Kind: InvalidType}, false
	}

	return startType, true
}

// activeCollectionIteration is one collection whose structure an enclosing
// for loop depends on while its body executes.
type activeCollectionIteration struct {
	place Place
	token lexer.Token
}

// activeCollectionIterationFor records the reusable collection storage a loop
// iterates. Ranges evaluate their bounds once and Iterator[T] loops advance
// their own iterator state, so neither depends on collection structure here.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §8 "Structural stability during iteration"; §16
func (a *Analyzer) activeCollectionIterationFor(stmt *ast.ForStatement) (activeCollectionIteration, bool) {
	if stmt.Iterable == nil {
		return activeCollectionIteration{}, false
	}
	switch category, _, _ := a.forIterationSource(stmt); category {
	case forCategorySequential, forCategorySet, forCategoryMap:
	default:
		return activeCollectionIteration{}, false
	}
	place, ok := a.resolvePlace(stmt.Iterable)
	if !ok || place.Root == "" {
		return activeCollectionIteration{}, false
	}
	return activeCollectionIteration{place: place, token: stmt.Token}, true
}

// invalidatedCollectionIteration returns the innermost active iteration whose
// collection structure a mutation of place may change. Mutating the iterated
// collection itself or any storage enclosing it can change length, backing
// storage, or element identity; mutating storage inside one element cannot.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §7 and §8; §16
func (a *Analyzer) invalidatedCollectionIteration(place Place) (activeCollectionIteration, bool) {
	for index := len(a.activeCollectionIterations) - 1; index >= 0; index-- {
		active := a.activeCollectionIterations[index]
		if len(place.Projections) <= len(active.place.Projections) && Relationship(place, active.place).EnclosesOrMayEnclose() {
			return active, true
		}
	}
	return activeCollectionIteration{}, false
}

// structurallyMutatesIteratedCollection rejects a compiler-known structural
// collection operation, identified by its registry operation contract rather
// than by name, on a collection that an enclosing loop is iterating.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §7 "Mutable element access does not grant structural mutation"
//   - rules/control-flow/flowcontrol_for.md — §8 "Structural stability during iteration"; §41 diagnostics
//   - rules/compiler/compiler_analysis.md — § 18(2) structural mutation dependencies
func (a *Analyzer) structurallyMutatesIteratedCollection(receiver ast.Expression, operation string, token lexer.Token) bool {
	place, ok := a.resolvePlace(receiver)
	if !ok {
		return false
	}
	active, invalidated := a.invalidatedCollectionIteration(place)
	if !invalidated {
		return false
	}
	a.addErrorAtTokenWithMetadataAndPrevious(token, active.token, diagnostics.StructuralMutationDuringIteration,
		"Record the change during the loop and apply it after the loop finishes.",
		"cannot %s %s while the enclosing for loop iterates %s; structural mutation can invalidate the active iteration",
		operation, place.String(), active.place.String())
	return true
}

// assignmentReplacesIteratedCollection rejects replacing the iterated
// collection, or storage enclosing it, inside the loop body. Assigning one
// element is element mutation and remains governed by ordinary rules;
// rebinding a reference holder does not mutate its referent.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §8 "move or replace backing storage"
func (a *Analyzer) assignmentReplacesIteratedCollection(stmt *ast.AssignmentStatement) bool {
	if len(a.activeCollectionIterations) == 0 {
		return false
	}
	if identifier, ok := stmt.Target.(*ast.Identifier); ok {
		if symbol, exists := a.symbols[identifier.Value]; exists && symbol.Type.Kind == ReferenceType {
			return false
		}
	}
	place, ok := a.resolvePlace(stmt.Target)
	if !ok {
		return false
	}
	active, invalidated := a.invalidatedCollectionIteration(place)
	if !invalidated {
		return false
	}
	a.addErrorAtTokenWithMetadataAndPrevious(expressionToken(stmt.Target), active.token, diagnostics.StructuralMutationDuringIteration,
		"Record the change during the loop and apply it after the loop finishes.",
		"cannot assign %s while the enclosing for loop iterates %s; replacing the collection can invalidate the active iteration",
		place.String(), active.place.String())
	return true
}

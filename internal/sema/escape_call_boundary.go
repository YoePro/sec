package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// EscapeCallRelation is the resolved semantic relation between one argument
// and its callee parameter. Escape analysis records the relation from the
// resolved call contract, never from source-token presence, so the fact
// stays valid after source syntax is lowered.
type EscapeCallRelation string

const (
	EscapeCallBorrow            EscapeCallRelation = "borrow"
	EscapeCallMutableBorrow     EscapeCallRelation = "mutable-borrow"
	EscapeCallCopy              EscapeCallRelation = "copy"
	EscapeCallOwnershipTransfer EscapeCallRelation = "ownership-transfer"
)

// EscapeCallArgument is the call-boundary fact for one argument of a call to
// a resolved callee. ParameterIndex is the callee parameter the argument
// binds. Sources carry the argument's canonical Place provenance in the
// caller, or a Temporary source for a fresh value.
//
// Retention is deliberately not a relation here: a borrowed parameter of a
// Sec callable has the NoRetention contract, because Sec 0.1 has no
// retaining parameter contract, and retention is not implied merely by
// crossing a call boundary. Explicit retention contracts and foreign
// retention are separate work.
//
// A call through a function value has no single Callee: Targets lists the
// proven closed target set, and UnknownTargets reports an open or
// unresolved target set, which is conservative UnknownRetention for
// reference-like, address-like and callable arguments.
type EscapeCallArgument struct {
	Caller         CallableID
	Callee         CallableID
	Targets        []CallableID
	UnknownTargets bool
	// Foreign reports a call to an extern declaration, whose retention
	// follows the FFI parameter contract.
	Foreign        bool
	CalleeName     string
	ParameterIndex int
	Relation       EscapeCallRelation
	Sources        []EscapeSource
	Sink           lexer.Token
}

func (e *EscapeAnalysis) CallArguments() []EscapeCallArgument {
	if e == nil {
		return nil
	}
	return cloneEscapeCallArguments(e.callArguments)
}

// addCallArgument records one call-boundary relation. A consuming transfer
// is also a ValueTransfer escape into the callee, which the TransfersOwnership
// call contract permits to keep the value (RetainingCall destination), and a
// transferred caller parameter or receiver gets the OwnershipTransferred
// disposition in the caller summary.
func (e *EscapeAnalysis) addCallArgument(function Function, argument EscapeCallArgument, typ Type) {
	if e == nil || argument.Caller == "" {
		return
	}
	key := escapeCallArgumentKey{argument.Caller, argument.Sink.File, argument.Sink.Line, argument.Sink.Column, argument.ParameterIndex}
	if e.callArgumentKeys == nil {
		e.callArgumentKeys = map[escapeCallArgumentKey]bool{}
	}
	if e.callArgumentKeys[key] {
		return
	}
	e.callArgumentKeys[key] = true
	e.callArguments = append(e.callArguments, cloneEscapeCallArgument(argument))
	if argument.UnknownTargets && escapeArgumentMayBeRetained(argument.Relation, typ) {
		e.addUnknownRetention(function, argument, typ)
	}
	if argument.Foreign && foreignArgumentMayBeRetained(argument.Relation, typ) {
		e.addForeignRetention(function, argument, typ)
	}
	if argument.Relation != EscapeCallOwnershipTransfer {
		return
	}
	fact := EscapeFact{
		Callable:    argument.Caller,
		Subject:     escapeSubjectForType(typ),
		Mode:        EscapeModeValueTransfer,
		Destination: EscapeDestinationRetainingCall,
		Sources:     cloneEscapeSources(argument.Sources),
		Sink:        argument.Sink,
	}
	e.facts = append(e.facts, cloneEscapeFact(fact))
	summary := e.ensureSummary(function, argument.Caller)
	for _, source := range argument.Sources {
		switch source.Kind {
		case EscapeSourceParameter:
			e.addParameterDisposition(summary, function, source.ParameterIndex, EscapeParameterOwnershipTransferred)
		case EscapeSourceReceiver:
			summary.Receiver = appendEscapeDisposition(summary.Receiver, EscapeParameterOwnershipTransferred)
		}
	}
}

// escapeArgumentMayBeRetained reports the reference-like, address-like and
// callable arguments for which unknown retention is conservative: borrowed
// dependencies and values whose type carries a reference, view, address,
// callable environment, handle or storage dependency.
func escapeArgumentMayBeRetained(relation EscapeCallRelation, typ Type) bool {
	if relation == EscapeCallBorrow || relation == EscapeCallMutableBorrow {
		return true
	}
	return escapeSubjectForType(typ) != EscapeSubjectOwnedValue
}

// addUnknownRetention records that an argument may be kept by an unresolved
// callee beyond the call: a Retention fact to the Unknown destination marked
// Unknown, and the UnknownRetention disposition for a caller parameter or
// receiver source.
func (e *EscapeAnalysis) addUnknownRetention(function Function, argument EscapeCallArgument, typ Type) {
	fact := EscapeFact{
		Callable:    argument.Caller,
		Subject:     escapeSubjectForType(typ),
		Mode:        EscapeModeRetention,
		Destination: EscapeDestinationUnknown,
		Sources:     cloneEscapeSources(argument.Sources),
		Sink:        argument.Sink,
		Unknown:     true,
	}
	if argument.Relation == EscapeCallBorrow || argument.Relation == EscapeCallMutableBorrow {
		fact.Subject = EscapeSubjectSafeReference
	}
	e.facts = append(e.facts, cloneEscapeFact(fact))
	summary := e.ensureSummary(function, argument.Caller)
	for _, source := range argument.Sources {
		switch source.Kind {
		case EscapeSourceParameter:
			e.addParameterDisposition(summary, function, source.ParameterIndex, EscapeParameterUnknownRetention)
		case EscapeSourceReceiver:
			summary.Receiver = appendEscapeDisposition(summary.Receiver, EscapeParameterUnknownRetention)
		}
	}
}

// foreignArgumentMayBeRetained applies the FFI parameter contracts: a ref T
// or ref mut T extern parameter is a call-bounded borrow that foreign code
// must not retain, while a raw pointer, callable, view or any value carrying
// one establishes no retention behavior, so foreign retention is unknown.
// Plain scalar and address-free aggregate values are copied and retain
// nothing of the caller.
func foreignArgumentMayBeRetained(relation EscapeCallRelation, typ Type) bool {
	if relation == EscapeCallBorrow || relation == EscapeCallMutableBorrow {
		return false
	}
	return escapeSubjectForType(typ) != EscapeSubjectOwnedValue || typeContainsAddress(typ, map[string]bool{})
}

// addForeignRetention records an argument whose address or dependency enters
// foreign code under no retention contract: a ForeignRetention fact to the
// ForeignCode destination marked Unknown, and the PassedToForeign and
// UnknownRetention dispositions for a caller parameter or receiver source.
func (e *EscapeAnalysis) addForeignRetention(function Function, argument EscapeCallArgument, typ Type) {
	subject := escapeSubjectForType(typ)
	if subject == EscapeSubjectOwnedValue {
		subject = EscapeSubjectStorageDependency
	}
	fact := EscapeFact{
		Callable:    argument.Caller,
		Subject:     subject,
		Mode:        EscapeModeForeignRetention,
		Destination: EscapeDestinationForeignCode,
		Sources:     cloneEscapeSources(argument.Sources),
		Sink:        argument.Sink,
		Unknown:     true,
	}
	e.facts = append(e.facts, cloneEscapeFact(fact))
	summary := e.ensureSummary(function, argument.Caller)
	for _, source := range argument.Sources {
		switch source.Kind {
		case EscapeSourceParameter:
			e.addParameterDisposition(summary, function, source.ParameterIndex, EscapeParameterPassedToForeign)
			e.addParameterDisposition(summary, function, source.ParameterIndex, EscapeParameterUnknownRetention)
		case EscapeSourceReceiver:
			summary.Receiver = appendEscapeDisposition(summary.Receiver, EscapeParameterPassedToForeign)
			summary.Receiver = appendEscapeDisposition(summary.Receiver, EscapeParameterUnknownRetention)
		}
	}
}

// typeContainsAddress reports raw pointers and callables stored anywhere in
// a value, including C-compatible struct fields (ffi.md § 14).
func typeContainsAddress(typ Type, visiting map[string]bool) bool {
	if typ.Kind == RawPtrType || typ.Kind == FunctionType {
		return true
	}
	identity := typeDeclarationIdentity(typ)
	if identity != "" && visiting[identity] {
		return false
	}
	if identity != "" {
		visiting[identity] = true
		defer delete(visiting, identity)
	}
	if typ.Element != nil && typeContainsAddress(*typ.Element, visiting) {
		return true
	}
	for _, field := range typ.Fields {
		if typeContainsAddress(field.Type, visiting) {
			return true
		}
	}
	for _, arg := range typ.TypeArgs {
		if typeContainsAddress(arg, visiting) {
			return true
		}
	}
	for _, variant := range typ.UnionVariants {
		if variant.Payload != nil && typeContainsAddress(*variant.Payload, visiting) {
			return true
		}
	}
	return false
}

type escapeCallArgumentKey struct {
	caller       CallableID
	file         string
	line, column int
	parameter    int
}

// recordCallBoundaryEscapeFacts classifies every argument of a validated call
// to callee by its resolved parameter contract: a shared or mutable borrowed
// parameter is a borrow, a consuming parameter or an owning parameter that
// receives a move-only value is an ownership transfer, and an owning
// parameter receiving a copyable value is a copy. Prepared spread values are
// covered by their pack element facts and are not separate arguments.
//
// Rules:
//   - rules/analysis/escape_analysis.md — "Call retention contracts", "Call-boundary semantic classification", "Calls", "Parameter summary facts", "ValueTransfer", "RetainingCall"
//   - rules/memory/copy_move.md — §8.2 "Consuming parameters"
//   - rules/memory/borrowing.md — §15.2 "Call-site borrow creation"
//   - rules/platform/ffi.md — § 10 "Raw pointers", § 12 "Safe references in foreign parameters", § 14 "Stored foreign pointer fields"
func (a *Analyzer) recordCallBoundaryEscapeFacts(callee Function, sourceArgs []ast.Expression, preparedSpreadValues []bool) {
	// Lambda callable identity is owned by closure analysis and is not yet
	// part of the canonical call graph (as for return facts).
	// Calls Sema proves unreachable transfer and retain nothing.
	if a == nil || a.summaryPass || a.inLambda || a.escapeAnalysis == nil || a.currentCallable == "" || !a.callGraphPathReachable {
		return
	}
	calleeID := callableID(callee)
	for sourceIndex, arg := range sourceArgs {
		if sourceIndex < len(preparedSpreadValues) && preparedSpreadValues[sourceIndex] {
			continue
		}
		parameter, ok := functionParameterForArgument(callee, sourceIndex)
		if !ok {
			return
		}
		parameterIndex := sourceIndex
		if parameterIndex >= len(callee.Parameters) {
			parameterIndex = len(callee.Parameters) - 1
		}
		source := explicitMoveSource(arg)
		_, explicit := explicitMoveArgument(arg)
		var sources []EscapeSource
		typ, typed := a.expressionTypes[source]
		if place, reusable := a.resolvePlace(source); reusable {
			sources = []EscapeSource{escapeSourceFromPlace(place, a.currentFunctionMetadata, "", expressionToken(source))}
			typ, typed = place.Type, true
		} else {
			sources = []EscapeSource{{Kind: EscapeSourceTemporary, ParameterIndex: -1, Token: expressionToken(source)}}
		}
		relation := EscapeCallCopy
		if mutable, borrowed := callParameterBorrowMode(parameter); borrowed {
			relation = EscapeCallBorrow
			if mutable {
				relation = EscapeCallMutableBorrow
			}
		} else if parameter.Consuming || explicit || !typed || requiresOwnershipTransfer(typ) {
			relation = EscapeCallOwnershipTransfer
		}
		a.escapeAnalysis.addCallArgument(a.currentFunctionMetadata, EscapeCallArgument{
			Caller:         a.currentCallable,
			Callee:         calleeID,
			Foreign:        callee.Extern,
			CalleeName:     callee.Name,
			ParameterIndex: parameterIndex,
			Relation:       relation,
			Sources:        sources,
			Sink:           expressionToken(arg),
		}, typ)
	}
}

// recordFunctionValueCallEscapeFacts composes call-boundary facts for a
// validated call through a function value. Argument validation used the
// function type, so each relation follows its parameter type: a reference
// parameter is a borrow, a move-only value or explicit <- an ownership
// transfer, and other values copies. Retention follows the target set: a
// closed set of known Sec bodies (named functions and lambdas) carries the
// NoRetention contract of Sec callables, while an open set, an unresolved
// callee or a foreign target is UnknownRetention.
//
// Rules:
//   - rules/analysis/escape_analysis.md — "Call retention contracts" (UnknownRetention), "Unknown", "Calls", "Call-boundary semantic classification"
//   - rules/analysis/closure_analysis.md — "Exact", closed and open target sets
//   - rules/analysis/call_graph.md — "Unknown concrete target"
func (a *Analyzer) recordFunctionValueCallEscapeFacts(call *ast.CallExpression, calleeType Type) {
	if a == nil || call == nil || a.summaryPass || a.inLambda || a.escapeAnalysis == nil || a.currentCallable == "" || !a.callGraphPathReachable {
		return
	}
	targets, known := a.knownSecCallTargets(call.Callee)
	fixed := len(calleeType.FunctionParameterTypes)
	if calleeType.FunctionVariadic {
		fixed--
	}
	for index, arg := range call.Arguments {
		parameterType, ok := functionTypeParameterForArgument(calleeType, index)
		if !ok {
			return
		}
		parameterIndex := index
		if calleeType.FunctionVariadic && parameterIndex > fixed {
			parameterIndex = fixed
		}
		source := explicitMoveSource(arg)
		_, explicit := explicitMoveArgument(arg)
		typ, typed := a.expressionTypes[source]
		var sources []EscapeSource
		if place, reusable := a.resolvePlace(source); reusable {
			sources = []EscapeSource{escapeSourceFromPlace(place, a.currentFunctionMetadata, "", expressionToken(source))}
			typ, typed = place.Type, true
		} else {
			sources = []EscapeSource{{Kind: EscapeSourceTemporary, ParameterIndex: -1, Token: expressionToken(source)}}
		}
		relation := EscapeCallCopy
		switch {
		case parameterType.Kind == ReferenceType && parameterType.ReferenceMutable:
			relation = EscapeCallMutableBorrow
		case parameterType.Kind == ReferenceType:
			relation = EscapeCallBorrow
		case explicit || !typed || requiresOwnershipTransfer(typ):
			relation = EscapeCallOwnershipTransfer
		}
		a.escapeAnalysis.addCallArgument(a.currentFunctionMetadata, EscapeCallArgument{
			Caller:         a.currentCallable,
			Targets:        targets,
			UnknownTargets: !known,
			CalleeName:     call.Callee.String(),
			ParameterIndex: parameterIndex,
			Relation:       relation,
			Sources:        sources,
			Sink:           expressionToken(arg),
		}, typ)
	}
}

// knownSecCallTargets returns the callable nodes of a closed target set whose
// every member is a known Sec body. An open set, an unmapped body or a
// foreign target leaves the set unknown.
func (a *Analyzer) knownSecCallTargets(callee ast.Expression) ([]CallableID, bool) {
	identity, ok := a.callableIdentityForExpression(callee)
	if !ok || !identity.Targets.IsClosed || identity.Targets.HasOpenContract || len(identity.Targets.KnownTargets) == 0 || a.callGraph == nil {
		return nil, false
	}
	targets := make([]CallableID, 0, len(identity.Targets.KnownTargets))
	for _, body := range identity.Targets.KnownTargets {
		id, mapped := a.callGraph.bodyNodes[body]
		if !mapped {
			return nil, false
		}
		node, found := a.callGraph.Node(id)
		if !found || node.Extern {
			return nil, false
		}
		targets = append(targets, id)
	}
	return targets, true
}

func cloneEscapeSources(sources []EscapeSource) []EscapeSource {
	if sources == nil {
		return nil
	}
	out := make([]EscapeSource, len(sources))
	for index, source := range sources {
		out[index] = cloneEscapeSource(source)
	}
	return out
}

func cloneEscapeCallArgument(argument EscapeCallArgument) EscapeCallArgument {
	argument.Sources = cloneEscapeSources(argument.Sources)
	argument.Targets = append([]CallableID(nil), argument.Targets...)
	return argument
}

func cloneEscapeCallArguments(arguments []EscapeCallArgument) []EscapeCallArgument {
	if arguments == nil {
		return nil
	}
	out := make([]EscapeCallArgument, len(arguments))
	for index, argument := range arguments {
		out[index] = cloneEscapeCallArgument(argument)
	}
	return out
}

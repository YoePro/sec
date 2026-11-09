package sema

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// lambdaEnvironmentState tracks, while one lambda body is analyzed, how the
// body uses its closure environment: owned captures are environment fields
// the body may mutate or move out of, and a `ref mut` capture holds exclusive
// authority. The facts select the inferred callable capability.
//
// Rules:
//   - rules/declarations/lambda-functions.md — §§9, 18, 20, 26, 28
type lambdaEnvironmentState struct {
	owned          map[string]bool
	mutableBorrow  bool
	mutated        bool
	mutatedToken   lexer.Token
	consumed       bool
	consumedToken  lexer.Token
	moveOnlyOwned  bool
	borrowCaptures []lambdaBorrowCapture
}

// lambdaBorrowCapture is one validated `ref`/`ref mut` capture: the outer
// Place it borrows at closure construction.
type lambdaBorrowCapture struct {
	Name    string
	Place   Place
	Mutable bool
	Token   lexer.Token
}

// noteLambdaEnvironmentWrite records that the current lambda body writes an
// owned environment field (assignment, mutable borrow, mutating receiver or
// mutable callable invocation), which needs `mut fn` authority (§18).
func (a *Analyzer) noteLambdaEnvironmentWrite(place Place, token lexer.Token) {
	state := a.lambdaEnvironment
	if state == nil || place.Root == "" || !state.owned[place.Root] {
		return
	}
	if !state.mutated {
		state.mutated, state.mutatedToken = true, token
	}
}

// noteLambdaEnvironmentConsume records that the current lambda body moves
// state out of an owned environment field, which makes the callable
// consuming (§28).
func (a *Analyzer) noteLambdaEnvironmentConsume(place Place, token lexer.Token) {
	state := a.lambdaEnvironment
	if state == nil || place.Root == "" || !state.owned[place.Root] {
		return
	}
	if !state.consumed {
		state.consumed, state.consumedToken = true, token
	}
}

// inferredCapability selects the least callable authority the body needs:
// consuming when it moves environment state out, mutable when it writes an
// owned field or holds a `ref mut` capture, otherwise shared (§§9, 20, 28).
func (state *lambdaEnvironmentState) inferredCapability() CallableCapability {
	switch {
	case state == nil:
		return CallableShared
	case state.consumed:
		return CallableConsuming
	case state.mutated || state.mutableBorrow:
		return CallableMutable
	default:
		return CallableShared
	}
}

// callableCapabilityRank orders callable authority: a callable needing less
// authority may stand where more is permitted (§10).
func callableCapabilityRank(capability CallableCapability) int {
	switch normalizedCallableCapability(capability) {
	case CallableMutable:
		return 1
	case CallableConsuming:
		return 2
	default:
		return 0
	}
}

// callableCapabilityStrengthens reports a value function type that may
// initialize a target function type: the same signature, and the value's
// capability no stronger than the target's. Nominal callable wrappers keep
// exact identity.
//
// Rules:
//   - rules/declarations/lambda-functions.md — §§10–11
func callableCapabilityStrengthens(target, value Type) bool {
	if target.Kind != FunctionType || value.Kind != FunctionType || target.Named || value.Named {
		return false
	}
	if callableCapabilityRank(value.FunctionCapability) > callableCapabilityRank(target.FunctionCapability) {
		return false
	}
	relaxed := value
	relaxed.FunctionCapability = target.FunctionCapability
	return sameFunctionType(target, relaxed)
}

// callableCapabilityMismatch explains a rejected function-value
// initialization whose only difference is a weaker target capability.
func callableCapabilityMismatch(target, value Type) (string, bool) {
	if target.Kind != FunctionType || value.Kind != FunctionType || target.Named || value.Named {
		return "", false
	}
	relaxed := value
	relaxed.FunctionCapability = target.FunctionCapability
	if !sameFunctionType(target, relaxed) {
		return "", false
	}
	return fmt.Sprintf("a %s callable cannot be used as %s: it needs more invocation authority than the target grants",
		callableCapabilityKeyword(value.FunctionCapability), callableCapabilityKeyword(target.FunctionCapability)), true
}

func callableCapabilityKeyword(capability CallableCapability) string {
	switch normalizedCallableCapability(capability) {
	case CallableMutable:
		return "`mut fn`"
	case CallableConsuming:
		return "`-> fn`"
	default:
		return "`fn`"
	}
}

// applyCallableInvocationAuthority checks the authority a function-value call
// needs over its callee after the arguments are prepared: a `mut fn` call
// needs mutable access to the callable, and a `-> fn` call needs an owned,
// available callable and consumes it when the call is made.
//
// Rules:
//   - rules/declarations/lambda-functions.md — §§8, 26, 28
func (a *Analyzer) applyCallableInvocationAuthority(expr *ast.CallExpression, calleeType Type) bool {
	capability := normalizedCallableCapability(calleeType.FunctionCapability)
	if capability == CallableShared {
		return true
	}
	place, ok := a.resolvePlace(expr.Callee)
	if !ok {
		// A temporary callable is owned by the call expression.
		return true
	}
	token := expressionToken(expr.Callee)
	if a.checkPlaceAvailableForRead(place, token) {
		return false
	}
	switch capability {
	case CallableMutable:
		if !place.Mutable {
			a.addErrorAtTokenWithMetadata(token, diagnostics.BorrowMutableAuthority,
				fmt.Sprintf("A `mut fn` call may change the callable's environment, so it needs mutable access. Declare the binding with `let mut %s` or call it through `ref mut`.", place.Root),
				"calling `mut fn` value %s requires mutable access to it", place.String())
			return false
		}
		a.noteLambdaEnvironmentWrite(place, token)
	case CallableConsuming:
		if a.placeIsThroughReference(place) {
			a.addErrorAtTokenWithMetadata(token, diagnostics.ImplicitMoveDisallowed,
				"A `-> fn` call consumes the callable value, which a reference does not own. Call an owned callable binding.",
				"calling `-> fn` value %s requires owning it; a reference cannot be consumed", place.String())
			return false
		}
		if !a.checkBorrowedMovePlace(place, token) {
			a.markPlaceUnavailable(place, token, "moved")
		}
	}
	return true
}

// placeIsThroughReference reports a Place reached through a reference holder.
func (a *Analyzer) placeIsThroughReference(place Place) bool {
	if symbol, ok := a.symbols.lookup(place.Root); ok && symbol.Type.Kind == ReferenceType {
		return true
	}
	for _, projection := range place.Projections {
		if projection.Kind == PlaceDereference {
			return true
		}
	}
	return false
}

// validateLambdaBorrowCapture checks a `ref`/`ref mut` capture at closure
// construction like an ordinary borrow of the outer Place: the Place must be
// borrowable, a mutable capture needs mutable authority, and no conflicting
// borrow may be live. It returns the Place on success.
//
// Rules:
//   - rules/declarations/lambda-functions.md — §§19–20, 23
//   - rules/memory/borrowing.md — §§20.1–20.2
func (a *Analyzer) validateLambdaBorrowCapture(capture ast.LambdaCapture, symbol Symbol) (Place, bool) {
	if symbol.Type.Kind == ReferenceType {
		a.addErrorAtTokenWithMetadata(capture.Name.Token, diagnostics.BorrowMutableAuthority,
			fmt.Sprintf("%s is already a reference; capture the reference value with `capture(%s)` instead of borrowing the holder.", capture.Name.Value, capture.Name.Value),
			"cannot borrow-capture reference holder %s", capture.Name.Value)
		return Place{}, false
	}
	place, ok := a.resolvePlace(capture.Name)
	if !ok {
		return Place{}, false
	}
	mutable := capture.Mode == ast.LambdaCaptureMutableBorrow
	if a.checkBorrowCreationPlace(place, mutable, capture.Token) {
		return Place{}, false
	}
	if mutable {
		a.noteLambdaEnvironmentWrite(place, capture.Token)
	}
	return place, true
}

// lambdaBorrowCaptureType is the binding type of a borrow capture inside the
// lambda body: a reference holder `ref T` or `ref mut T` with the outer Place
// as origin, used through `*name` like a `ref` pattern binding (owner
// decision 2026-11-07; flowcontrol_match.md "ref" payload bindings).
func (a *Analyzer) lambdaBorrowCaptureType(capture ast.LambdaCapture, symbol Symbol, place Place) Type {
	valueType := symbol.Type
	mutable := capture.Mode == ast.LambdaCaptureMutableBorrow
	return Type{
		Name:                   referenceTypeName(valueType, mutable),
		Kind:                   ReferenceType,
		Element:                &valueType,
		ReferenceMutable:       mutable,
		ReferenceOriginName:    capture.Name.Value,
		ReferenceOriginToken:   symbol.Token,
		ReferenceOriginLocal:   true,
		ReferenceOriginStorage: place.StorageOrigin,
	}
}

// registerLambdaBorrowCaptures makes the closure binding the holder of every
// borrow its `ref`/`ref mut` captures took, so outer conflicting access is
// rejected while the closure lives and the closure cannot escape the
// borrowed storage (§§20, 23–24).
func (a *Analyzer) registerLambdaBorrowCaptures(holder string, lambda *ast.LambdaExpression) {
	captures := a.lambdaBorrowCaptures[lambda]
	if holder == "" || len(captures) == 0 {
		return
	}
	origin := localReferenceOrigin{Contained: map[string]localReferenceOrigin{}}
	for _, capture := range captures {
		kind := sharedBorrow
		if capture.Mutable {
			kind = mutableBorrow
		}
		for _, alternative := range placeOriginAlternatives(capture.Place) {
			a.borrows[alternative.Root] = append(a.borrows[alternative.Root], borrowRecord{
				Root: alternative.Root, Place: alternative, Holder: holder, Kind: kind, Token: capture.Token,
			})
		}
		origin.Contained["."+capture.Name] = localReferenceOrigin{
			Name: capture.Name, Token: capture.Token, Local: true, Mutable: capture.Mutable,
			Place: clonePlace(capture.Place), HasPlace: true,
		}
	}
	a.localRefContainers[holder] = origin
}

// lambdaEnvironmentMoveOnly reports a closure the rules make move-only from
// its environment: a move-only owned capture or a `ref mut` capture (§27).
// A consuming callable is move-only through its type.
func (state *lambdaEnvironmentState) environmentMoveOnly() bool {
	return state != nil && (state.moveOnlyOwned || state.mutableBorrow)
}

// reportLambdaSelfReference explains a lambda body naming the binding its own
// let initializes: that binding does not exist until the lambda value is
// created, and Sec 0.1 has no recursive-closure binding form.
//
// Rules:
//   - rules/declarations/lambda-functions.md — § 33 "Direct recursion"
func (a *Analyzer) reportLambdaSelfReference(token lexer.Token, name string) bool {
	if !a.inLambda || !a.lambdaInitializerNames[name] {
		return false
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.UnknownCallTarget,
		"A lambda cannot call itself through the binding it initializes; use a named function for direct recursion.",
		"lambda cannot refer to %s, the binding its let is initializing; %s does not exist until the lambda value is created", name, name)
	return true
}

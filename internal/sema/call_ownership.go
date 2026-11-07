package sema

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// validateCallArgumentOwnership requires visible transfer from every reusable
// Place passed to a consuming parameter, including copyable Places, while
// preserving marker-free forwarding of fresh temporaries.
//
// Rules:
//   - rules/memory/copy_move.md — §§7,8.1–8.3,28.1
//   - rules/declarations/functions.md — §9 "Explicit consuming parameter"
func (a *Analyzer) validateCallArgumentOwnership(function Function, sourceArgs []ast.Expression, preparedSpreadValues []bool, call *ast.CallExpression) bool {
	if !a.validateCallTransferSources(function, sourceArgs, preparedSpreadValues) {
		return false
	}
	valid := true
	for sourceIndex, arg := range sourceArgs {
		param, ok := functionParameterForArgument(function, sourceIndex)
		if !ok {
			return false
		}
		preparedSpread := sourceIndex < len(preparedSpreadValues) && preparedSpreadValues[sourceIndex]
		move, explicit := explicitMoveArgument(arg)
		source := explicitMoveSource(arg)
		borrowParameter := param.Ref || param.MutableRef || param.Type.Kind == ReferenceType
		if explicit {
			if borrowParameter {
				a.reportMoveIntoBorrowedParameter(move, param, function, call)
				valid = false
				continue
			}
			if preparedSpread {
				a.addErrorAtToken(move.Token, "explicit <- argument requires an owning value parameter")
				valid = false
				continue
			}
			if !a.validateNamedOwnershipSource(ast.OwnershipMove, source, move.Token, false, false, TransferSiteArgument) {
				valid = false
			} else {
				a.bindTransferArgumentDestination(source, function, sourceIndex, param)
			}
			continue
		}
		if preparedSpread || borrowParameter {
			continue
		}
		place, reusable := a.resolvePlace(source)
		if !reusable {
			continue
		}
		// An owned pack element copied into an ordinary value parameter is
		// proved copyable per selected call (checkTypePackByValueLoops); a
		// consuming parameter would extract it and stays rejected.
		needsTransfer := param.Consuming || requiresOwnershipTransfer(place.Type) && !(isOwnedTypePackElement(place.Type) && a.variadicPackElementExpression(source))
		if !needsTransfer {
			// Publish the already selected ordinary copy for canonical
			// consumers, including final-use advice; do not perform a move.
			// Rules: rules/memory/copy_move.md — §§5,8.1;
			// rules/analysis/pitfall_analysis.md — "Large by-value argument at its final use".
			action := TransferCopyTrivial
			if CopyClassificationOf(place.Type) == CopySemantic {
				action = TransferCopySemantic
			}
			a.recordTransferPlan(source, place, TransferSiteArgument, action)
			a.bindTransferArgumentDestination(source, function, sourceIndex, param)
			continue
		}
		if param.Consuming {
			a.reportMissingConsumingArgument(function, param, source, place)
		} else {
			a.reportOrdinaryArgumentCopy(function, param, source, place)
		}
		valid = false
	}
	return valid
}

// validateCallTransferSources reserves explicit owning sources without ending
// their lifetime. A source cannot be transferred twice or handed to the same
// callee as both owned storage and a live reference into that storage. Argument
// evaluation is already complete; failed validation commits no source moves.
// Rules: rules/declarations/functions.md §§17–18,38A(6)(1–3);
// rules/memory/ownership.md — single ownership; rules/memory/borrowing.md §15.4.
func (a *Analyzer) validateCallTransferSources(function Function, args []ast.Expression, prepared []bool) bool {
	type transfer struct {
		place Place
		index int
	}
	transfers := []transfer{}
	for i, argument := range args {
		if i < len(prepared) && prepared[i] {
			continue
		}
		parameter, known := functionParameterForArgument(function, i)
		if !known {
			continue
		}
		if _, borrowed := callParameterBorrowMode(parameter); borrowed {
			continue
		}
		move, explicit := explicitMoveArgument(argument)
		if !explicit {
			continue
		}
		place, reusable := a.resolvePlace(move.Right)
		if !reusable {
			continue
		}
		for _, previous := range transfers {
			if placesMayOverlap(place, previous.place) {
				a.addErrorAtToken(move.Token, "argument %d moves %s while overlapping storage is already selected for transfer by argument %d", i+1, place.String(), previous.index+1)
				return false
			}
		}
		transfers = append(transfers, transfer{place, i})
	}
	for _, source := range transfers {
		for i, argument := range args {
			if i < len(prepared) && prepared[i] {
				continue
			}
			parameter, known := functionParameterForArgument(function, i)
			if !known {
				continue
			}
			if _, borrowed := callParameterBorrowMode(parameter); !borrowed {
				continue
			}
			place, reusable := a.callArgumentBorrowPlace(argument)
			if reusable && placesMayOverlap(source.place, place) {
				a.addErrorAtTokenWithMetadata(expressionToken(args[source.index]), diagnostics.BorrowOwnershipEndConflict,
					"Pass separate owned storage, or end the borrowed use before transferring this source.",
					"argument %d moves %s while argument %d borrows overlapping storage", source.index+1, source.place.String(), i+1)
				return false
			}
		}
	}
	return true
}

// validateOwningConstructionSource preserves ordinary copy semantics outside
// terminal return construction. A reusable move-only Place must carry <-;
// temporaries and copyable Places remain marker-free.
// Rules: rules/memory/copy_move.md — §§9.2,10.1–10.4,28.1–28.2.
func (a *Analyzer) validateOwningConstructionSource(expr ast.Expression, site TransferSiteKind) bool {
	if a.terminalReturnDepth > 0 {
		return true
	}
	if source, explicit := explicitMoveArgument(expr); explicit {
		// inferPrefixExpression already validated the source Place as a
		// construction operand; an arm result hands it to the branch.
		if site != TransferSiteConstruction {
			a.relabelTransferSite(source.Right, site)
		}
		return true
	}
	place, reusable := a.resolvePlace(expr)
	if !reusable {
		return true
	}
	if !requiresOwnershipTransfer(place.Type) {
		// An accepted copy into the constructed value or arm result (P17-13).
		a.recordAcceptedCopy(expr, place, site)
		return true
	}
	a.reportMissingPayloadMove(expr, place)
	return false
}

// ownershipArgumentDiagnostic keeps a selected API declaration and the source
// owner navigable without committing the invalid transfer.
// Rules: rules/memory/copy_move.md — §§7,8.1–8.3,28.1;
// rules/tooling/diagnostics.md — §§8–9.
func ownershipArgumentDiagnostic(source ast.Expression, place Place, parameter FunctionParameter, function Function, message, help string) Error {
	token := expressionToken(source)
	declaration := parameter.Token
	if declaration.Line <= 0 {
		declaration = function.Token
	}
	endLine, endColumn := token.EndPosition()
	return Error{
		ID: diagnostics.ImplicitMoveDisallowed, Message: message, Help: help,
		File: token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn,
		PreviousFile: declaration.File, PreviousLine: declaration.Line, PreviousColumn: declaration.Column,
		RelatedLabel: "parameter ownership contract",
		Related:      []RelatedLocation{ownershipSourceLocation(place)},
	}
}

// ownershipSourceLocation exposes the canonical owner declaration when known.
// Rules: rules/memory/copy_move.md — §28.1; rules/tooling/diagnostics.md — §9.
func ownershipSourceLocation(place Place) RelatedLocation {
	token := place.RootToken
	endLine, endColumn := token.EndPosition()
	return RelatedLocation{File: token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn, Label: "source ownership established here"}
}

// reportMissingConsumingArgument distinguishes an explicit consuming API from
// ordinary copying and offers a local marker only as a source-level suggestion.
// Rules: rules/memory/copy_move.md — §§8.2(1–5),28.1(3),28.2(1);
// rules/corrections/applied/diagnostics-ownership-v2-correction-20260826.md — "Missing explicit move".
func (a *Analyzer) reportMissingConsumingArgument(function Function, parameter FunctionParameter, source ast.Expression, place Place) {
	help := fmt.Sprintf("Parameter %s takes ownership. Write `<-%s` in the call to %s to make that transfer explicit; the source becomes unavailable after call entry. If you still need it afterwards, use it before the call or redesign the API to borrow it.", parameter.Name, place.String(), function.Name)
	message := fmt.Sprintf("reusable source %s passed to %s must use explicit <- ownership transfer", place.String(), function.Name)
	a.appendError(ownershipArgumentDiagnostic(source, place, parameter, function, message, help))
}

// reportOrdinaryArgumentCopy explains why an ordinary value parameter cannot
// accept an implicitly copied non-copyable source. An explicit <- may instead
// transfer it without changing the default by-value API contract.
// Rules: rules/memory/copy_move.md — §§8.1(1–5),22(2–3),28.1.
func (a *Analyzer) reportOrdinaryArgumentCopy(function Function, parameter FunctionParameter, source ast.Expression, place Place) {
	cause := nonCopyableCause(place.Type)
	if CopyClassificationOf(place.Type) == CopyConditional {
		cause = "generic copyability has not been proven"
	}
	if cause == "" {
		cause = "the type is move-only"
	}
	message := fmt.Sprintf("cannot copy reusable source %s to ordinary by-value parameter %s of %s because %s; this parameter copies its argument", place.String(), parameter.Name, function.Name, cause)
	help := fmt.Sprintf("If this source is no longer needed and has no conflicting borrow, pass `<-%[2]s` to explicitly move it into this value parameter. To require transfer at every call, declare `->%[1]s`. If the API needs only borrowed access, use `ref`, `ref mut` or a suitable slice/view. Changing the parameter contract requires reviewing its callers and body.", parameter.Name, place.String())
	a.appendError(ownershipArgumentDiagnostic(source, place, parameter, function, message, help))
}

// reportMissingPayloadMove explains non-terminal owning construction while
// preserving the existing copy-classification rejection and diagnostic ID.
// Rules: rules/memory/copy_move.md — §§10.1–10.4,28.1(3),28.2(1).
func (a *Analyzer) reportMissingPayloadMove(source ast.Expression, place Place) {
	classification := CopyClassificationOf(place.Type)
	var message string
	switch classification {
	case CopyNonCopyable:
		message = fmt.Sprintf("%s value %s cannot be copied because %s; use explicit move syntax <-", typeDisplayName(place.Type), place.String(), nonCopyableCause(place.Type))
	case CopyConditional:
		message = fmt.Sprintf("cannot copy value %s because generic copyability has not been proven; use explicit move syntax <-", place.String())
	default:
		message = fmt.Sprintf("cannot copy move-only value %s%s; use explicit move syntax <-", place.String(), customFreeMoveOnlyCause(place.Type))
	}
	token := expressionToken(source)
	endLine, endColumn := token.EndPosition()
	a.appendError(Error{ID: diagnostics.ImplicitMoveDisallowed, Message: message,
		Help: fmt.Sprintf("This owning field or payload needs an explicit transfer from %s. Write `<-%s` at this payload position; the source becomes unavailable when construction commits. A copyable source or fresh temporary needs no marker, and direct terminal return construction permits marker-free forwarding.", place.String(), place.String()),
		File: token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn,
		Related: []RelatedLocation{ownershipSourceLocation(place)},
	})
}

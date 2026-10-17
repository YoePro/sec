package sema

import "sec/internal/ast"

// analyzeMatchArmBody establishes payload bindings, borrow/move authority and
// isolated branch state before checking a guard and arm body. Binding names
// and borrow keywords retain distinct source provenance for navigation.
// Rules: rules/control-flow/flowcontrol_match.md — Payload bindings, Guards;
// rules/memory/borrowing.md — §§8,33(3); rules/concurrency/atomics.md — §44(4–5).
func (a *Analyzer) analyzeMatchArmBody(arm *ast.MatchArm, info matchPatternInfo) (Type, branchAnalysis) {
	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = copyMoved(previousMoved)
	a.moveReasons = copyMoveReasons(previousMoveReasons)
	a.closedResources = copyMoved(previousClosedResources)
	a.borrows = copyBorrows(previousBorrows)
	a.localRefContainers = copyLocalRefContainers(previousLocalRefContainers)
	a.arenaGenerations = copyArenaGenerations(previousArenaGenerations)
	if info.InitializesSubject != "" {
		a.assigned[info.InitializesSubject] = true
	}
	if info.PayloadMoves && info.PayloadPlace.Root != "" {
		switch {
		case a.rejectCustomFreePartialMove(info.PayloadPlace, info.PayloadToken):
		case !info.PayloadPlace.PartialMoveSafe:
			a.reportUnionPayloadMoveStorage(info)
		default:
			if _, _, _, unavailable := a.unavailablePlace(info.PayloadPlace); !unavailable && !a.checkBorrowedMovePlace(info.PayloadPlace, info.PayloadToken) {
				a.markPlaceUnavailable(info.PayloadPlace, info.PayloadToken, "moved")
			}
		}
	}
	if info.BindingName != "" {
		// Bind the payload at its own name token. Using the arm/variant token
		// would overwrite the variant's core declaration with a local symbol.
		// Rules: rules/control-flow/flowcontrol_match.md — Payload bindings;
		// rules/concurrency/atomics.md — §44(4–5).
		bindingToken := info.PayloadToken
		if bindingToken.Line == 0 {
			bindingToken = arm.Token
		}
		if a.defineSymbol(info.BindingName, info.BindingType, false, bindingToken) {
			a.bindMutexPayloadAcquisition(info.BindingName, info.PayloadPlace.Root)
			if info.PayloadToken.Line > 0 && info.PayloadToken.Column > 0 {
				a.bindDefinition(info.PayloadToken, bindingToken)
			}
			a.assigned[info.BindingName] = true
			a.clearRootPlaceState(info.BindingName)
			delete(a.constInts, info.BindingName)
			if info.PayloadBorrow && info.PayloadPlace.Root != "" && info.BindingType.Kind != InvalidType {
				borrowToken := info.PayloadBorrowToken
				if borrowToken.Line == 0 {
					borrowToken = info.PayloadToken
				}
				if !a.checkBorrowCreationPlace(info.PayloadPlace, info.PayloadBorrowMutable, borrowToken) {
					kind := sharedBorrow
					if info.PayloadBorrowMutable {
						kind = mutableBorrow
					}
					a.borrows[info.PayloadPlace.Root] = append(a.borrows[info.PayloadPlace.Root], borrowRecord{
						Root: info.PayloadPlace.Root, Place: info.PayloadPlace, Holder: info.BindingName,
						Kind: kind, Token: borrowToken,
					})
					a.localRefContainers[info.BindingName] = localReferenceOrigin{
						Name: info.BindingType.ReferenceOriginName, Token: info.BindingType.ReferenceOriginToken,
						Local: info.BindingType.ReferenceOriginLocal, MatchScoped: true,
						Place: clonePlace(info.PayloadPlace), HasPlace: true,
					}
					// package 15 § 99: publish the payload borrow's plan for
					// Semantic IR, keyed by the ref pattern node.
					a.recordReferencePlan(info.PayloadNode, info.PayloadPlace, info.PayloadBorrowMutable, false, info.PayloadToken)
				}
			}
		}
	}
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.moveReasons = previousMoveReasons
		a.closedResources = previousClosedResources
		a.borrows = previousBorrows
		a.localRefContainers = previousLocalRefContainers
		a.arenaGenerations = previousArenaGenerations
	}()

	if arm.Guard != nil {
		guardType, _ := a.inferExpression(arm.Guard)
		if guardType.Kind != InvalidType && guardType.Kind != BoolType {
			a.addErrorAtToken(expressionToken(arm.Guard), "match guard must be bool, got %s", typeDisplayName(guardType))
		}
	}

	if arm.ReturnBody != nil {
		a.analyzeReturnStatement(a.currentFunctionName, a.currentFunctionReturn, arm.ReturnBody)
		return Type{Kind: InvalidType}, a.finalizeMatchBranch(info, branchAnalysis{assigned: copyAssigned(a.assigned), moved: copyMoved(a.moved), moveReasons: copyMoveReasons(a.moveReasons), closedResources: copyMoved(a.closedResources), borrows: copyBorrows(a.borrows), localRefContainers: copyLocalRefContainers(a.localRefContainers), arenaGenerations: copyArenaGenerations(a.arenaGenerations), referenceOrigins: captureReferenceOrigins(a.symbols), continues: false})
	}
	if arm.BlockBody != nil {
		a.analyzeBlockStatements(arm.BlockBody)
		return Type{Kind: InvalidType}, a.finalizeMatchBranch(info, branchAnalysis{
			assigned:           copyAssigned(a.assigned),
			moved:              copyMoved(a.moved),
			moveReasons:        copyMoveReasons(a.moveReasons),
			closedResources:    copyMoved(a.closedResources),
			borrows:            copyBorrows(a.borrows),
			localRefContainers: copyLocalRefContainers(a.localRefContainers),
			arenaGenerations:   copyArenaGenerations(a.arenaGenerations),
			referenceOrigins:   captureReferenceOrigins(a.symbols),
			continues:          a.blockCanFallThrough(arm.BlockBody),
		})
	}
	if arm.Body == nil {
		return Type{Kind: InvalidType}, a.finalizeMatchBranch(info, branchAnalysis{assigned: copyAssigned(a.assigned), moved: copyMoved(a.moved), moveReasons: copyMoveReasons(a.moveReasons), closedResources: copyMoved(a.closedResources), borrows: copyBorrows(a.borrows), localRefContainers: copyLocalRefContainers(a.localRefContainers), arenaGenerations: copyArenaGenerations(a.arenaGenerations), referenceOrigins: captureReferenceOrigins(a.symbols), continues: true})
	}
	a.constructionOwnershipDepth++
	bodyType, _ := a.inferExpression(arm.Body)
	a.constructionOwnershipDepth--
	ownershipValid := true
	if a.terminalReturnDepth > 0 {
		ownershipValid = !a.validateTerminalReturnConstruction(arm.Body)
	} else {
		ownershipValid = a.validateOwningConstructionSource(arm.Body)
	}
	if ownershipValid {
		a.markMoveSource(arm.Body)
	}
	return bodyType, a.finalizeMatchBranch(info, branchAnalysis{assigned: copyAssigned(a.assigned), moved: copyMoved(a.moved), moveReasons: copyMoveReasons(a.moveReasons), closedResources: copyMoved(a.closedResources), borrows: copyBorrows(a.borrows), localRefContainers: copyLocalRefContainers(a.localRefContainers), arenaGenerations: copyArenaGenerations(a.arenaGenerations), referenceOrigins: captureReferenceOrigins(a.symbols), continues: true})
}

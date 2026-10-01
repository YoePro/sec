package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// Switch statement analysis: clause structure, case-item validation, per-case
// scopes, fallthrough, state merging, and termination.

// analyzeSwitchStatement validates clause structure, the once-evaluated
// subject, every case item, fallthrough placement, and per-case scopes, then
// merges the continuing ownership, assignment, borrow, reference, and Arena
// states of all reachable clauses plus the unmatched path.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §4 "Subject evaluation", §5 "Case evaluation order", §14 "`default`"
//   - rules/control-flow/flowcontrol_switch.md — §18 "Case scopes", §32 "Switch termination", §33 "Definite assignment"
//   - rules/control-flow/flowcontrol_switch.md — §36 "Sema requirements for subject switch", §37 "Sema requirements for subjectless switch"
//   - rules/corrections/applied/correction24-20260823.md — "Architectural correction"
func (a *Analyzer) analyzeSwitchStatement(stmt *ast.SwitchStatement) {
	before := copyAssigned(a.assigned)
	beforeMoved := copyMoved(a.moved)
	beforeMoveReasons := copyMoveReasons(a.moveReasons)
	beforeClosedResources := copyMoved(a.closedResources)
	beforeBorrows := copyBorrows(a.borrows)
	beforeLocalRefContainers := copyLocalRefContainers(a.localRefContainers)
	beforeArenaGenerations := copyArenaGenerations(a.arenaGenerations)
	if stmt.DefaultNotFinalToken.Type != "" {
		a.addErrorAtToken(stmt.DefaultNotFinalToken, "default must be the final switch clause")
	}
	for _, token := range stmt.DuplicateDefaultTokens {
		a.addErrorAtToken(token, "switch may contain only one default clause")
	}

	var subjectType Type
	hasSubject := stmt.Subject != nil
	if hasSubject {
		subjectType, _ = a.inferExpression(stmt.Subject)
		if subjectType.Kind == VoidType {
			a.addErrorAtToken(expressionToken(stmt.Subject), "switch subject cannot be void")
		}
	}

	tracker := newSwitchCoverageTracker()
	tracker.subjectType = subjectType
	clauses := append([]*ast.SwitchCase{}, stmt.Cases...)
	if stmt.Default != nil {
		clauses = append(clauses, stmt.Default)
	}

	branches := make([]branchAnalysis, 0, len(clauses)+1)
	clauseFlows := make([]ResolvedSwitchClauseFlow, 0, len(clauses))
	var fallthroughEntry *branchAnalysis
	for i, clause := range clauses {
		if clause == nil {
			continue
		}
		a.analyzeSwitchCaseItems(clause, hasSubject, subjectType, tracker)
		a.analyzeSwitchFallthrough(clause, i == len(clauses)-1)
		// rules/control-flow/flowcontrol_switch.md; correction24.md: the
		// direct case-test path and a preceding fallthrough edge meet at the
		// destination body. Fallthrough bypasses this clause's test expressions.
		directEntry := a.currentBranchAnalysisState()
		if fallthroughEntry != nil {
			a.applySwitchEntryMerge(directEntry, *fallthroughEntry)
		}
		branch := a.analyzeSwitchCaseBody(clause.Body)
		branches = append(branches, branch)
		clauseFlows = append(clauseFlows, ResolvedSwitchClauseFlow{
			SourceIndex:  i,
			Default:      clause.Default,
			ItemCount:    len(clause.Items),
			FallsThrough: branch.fallsThrough,
			Continues:    branch.continues,
		})
		a.applyBranchAnalysisState(directEntry)
		if branch.fallsThrough {
			copy := branch
			copy.continues = true
			fallthroughEntry = &copy
		} else {
			fallthroughEntry = nil
		}
	}
	if stmt.Default == nil {
		a.warnIncompleteEnumSwitch(stmt, tracker)
	}

	exhaustive := stmt.Default != nil || tracker.isExhaustive()
	a.recordResolvedSwitchFlow(stmt, subjectType, hasSubject, exhaustive, clauseFlows)
	if !exhaustive {
		branches = append(branches, branchAnalysis{assigned: before, moved: beforeMoved, moveReasons: beforeMoveReasons, closedResources: beforeClosedResources, borrows: beforeBorrows, localRefContainers: beforeLocalRefContainers, arenaGenerations: beforeArenaGenerations, continues: true})
	}
	a.assigned = mergeContinuingAssigned(before, branches...)
	a.moved, a.moveReasons = mergeContinuingMoveState(beforeMoved, beforeMoveReasons, branches...)
	a.closedResources = mergeContinuingClosedResources(beforeClosedResources, branches...)
	a.borrows = mergeContinuingBorrows(beforeBorrows, branches...)
	a.localRefContainers = mergeContinuingLocalRefContainers(beforeLocalRefContainers, branches...)
	a.arenaGenerations = mergeContinuingArenaGenerations(beforeArenaGenerations, branches...)
}

// applySwitchEntryMerge joins the direct case-test entry with the preceding
// clause's fallthrough edge at the destination case body.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §16 "Explicit `fallthrough`", §19 "Fallthrough and scope"
//   - rules/corrections/applied/correction24-20260823.md — "Architectural correction"
func (a *Analyzer) applySwitchEntryMerge(direct, fallthroughEntry branchAnalysis) {
	direct.continues = true
	fallthroughEntry.continues = true
	a.assigned = mergeContinuingAssigned(direct.assigned, direct, fallthroughEntry)
	a.moved, a.moveReasons = mergeContinuingMoveState(direct.moved, direct.moveReasons, direct, fallthroughEntry)
	a.closedResources = mergeContinuingClosedResources(direct.closedResources, direct, fallthroughEntry)
	a.borrows = mergeContinuingBorrows(direct.borrows, direct, fallthroughEntry)
	a.localRefContainers = mergeContinuingLocalRefContainers(direct.localRefContainers, direct, fallthroughEntry)
	a.arenaGenerations = mergeContinuingArenaGenerations(direct.arenaGenerations, direct, fallthroughEntry)
}

// analyzeSwitchCaseItems validates every comma-separated case alternative
// independently against the subject (or as a bool condition for subjectless
// switches) and feeds compile-time coverage tracking. Variant payload and
// wildcard patterns are rejected before inference so they do not cascade into
// undefined-name diagnostics.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §6 "Value cases", §8 "Range cases", §10 "Relational cases"
//   - rules/control-flow/flowcontrol_switch.md — §12 "Subjectless switch", §28 "No pattern matching"
//   - rules/control-flow/flowcontrol_switch.md — §36 "Sema requirements for subject switch", §37 "Sema requirements for subjectless switch", §40 "Required diagnostics"
func (a *Analyzer) analyzeSwitchCaseItems(clause *ast.SwitchCase, hasSubject bool, subjectType Type, tracker *switchCoverageTracker) {
	if clause.Default {
		return
	}

	for _, item := range clause.Items {
		switch item := item.(type) {
		case *ast.SwitchValueCase:
			if a.rejectSwitchPattern(item.Value, subjectType) {
				continue
			}
			valueType, _ := a.inferExpressionWithExpected(item.Value, subjectType)
			if valueType.Kind == InvalidType {
				continue
			}
			if hasSubject {
				if !canCompareEquality(subjectType, valueType) {
					a.addErrorAtToken(expressionToken(item.Value), "switch case must be compatible with subject type %s, got %s", typeDisplayName(subjectType), typeDisplayName(valueType))
				}
				a.checkSwitchValueCoverage(item.Value, tracker)
			} else if valueType.Kind != BoolType {
				a.addErrorAtToken(expressionToken(item.Value), "subjectless switch case must be bool, got %s", typeDisplayName(valueType))
			}
		case *ast.SwitchRangeCase:
			if !hasSubject {
				a.addErrorAtToken(item.Token, "subjectless switch case must be bool, got range")
				continue
			}
			a.analyzeSwitchRangeCase(item, subjectType)
			a.checkSwitchRangeCoverage(item.Range, tracker)
		case *ast.SwitchRelationalCase:
			if !hasSubject {
				a.addErrorAtToken(item.Token, "subjectless switch case must be bool, got relational case")
				continue
			}
			if !isOrderedSwitchType(subjectType) {
				a.addErrorAtToken(item.Token, "relational switch case requires ordered subject type")
				continue
			}
			valueType, _ := a.inferExpressionWithExpected(item.Value, subjectType)
			if valueType.Kind != InvalidType && !canCompareEquality(subjectType, valueType) {
				a.addErrorAtToken(expressionToken(item.Value), "switch case must be compatible with subject type %s, got %s", typeDisplayName(subjectType), typeDisplayName(valueType))
			}
			a.checkSwitchRelationalCoverage(item, tracker)
		}
	}
}

// analyzeSwitchFallthrough validates that fallthrough is the final statement
// of a non-final, non-default case body.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §16 "Explicit `fallthrough`", §38 "`fallthrough` validation"
func (a *Analyzer) analyzeSwitchFallthrough(clause *ast.SwitchCase, isFinal bool) {
	if clause == nil || clause.Body == nil {
		return
	}
	fallthroughIndex := -1
	for i, stmt := range clause.Body.Statements {
		if _, ok := stmt.(*ast.FallthroughStatement); ok {
			fallthroughIndex = i
		}
	}
	if fallthroughIndex == -1 {
		return
	}
	if clause.Default || isFinal {
		a.addErrorAtToken(clause.Body.Statements[fallthroughIndex].(*ast.FallthroughStatement).Token, "fallthrough is not allowed in the final switch case")
	}
	for i := fallthroughIndex + 1; i < len(clause.Body.Statements); i++ {
		if _, ok := clause.Body.Statements[i].(*ast.CommentStatement); ok {
			continue
		}
		a.addErrorAtToken(clause.Body.Statements[fallthroughIndex].(*ast.FallthroughStatement).Token, "fallthrough must be the final statement in a switch case")
		return
	}
}

// analyzeSwitchCaseBody analyzes one case body in its own lexical scope and
// returns its exit state, distinguishing ordinary continuation from an explicit
// fallthrough edge.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §15 "No implicit fallthrough", §18 "Case scopes", §19 "Fallthrough and scope"
func (a *Analyzer) analyzeSwitchCaseBody(block *ast.BlockStatement) branchAnalysis {
	if block == nil {
		return branchAnalysis{assigned: copyAssigned(a.assigned), moved: copyMoved(a.moved), moveReasons: copyMoveReasons(a.moveReasons), closedResources: copyMoved(a.closedResources), borrows: copyBorrows(a.borrows), localRefContainers: copyLocalRefContainers(a.localRefContainers), arenaGenerations: copyArenaGenerations(a.arenaGenerations), continues: true}
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	previousInSwitchCaseBody := a.inSwitchCaseBody
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = copyMoved(previousMoved)
	a.moveReasons = copyMoveReasons(previousMoveReasons)
	a.closedResources = copyMoved(previousClosedResources)
	a.borrows = copyBorrows(previousBorrows)
	a.localRefContainers = copyLocalRefContainers(previousLocalRefContainers)
	a.arenaGenerations = copyArenaGenerations(previousArenaGenerations)
	a.inSwitchCaseBody = true
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
		a.inSwitchCaseBody = previousInSwitchCaseBody
	}()

	hasFallthrough := blockEndsWithFallthrough(block)
	a.analyzeBlockStatements(block)
	return branchAnalysis{
		assigned:           copyAssigned(a.assigned),
		moved:              copyMoved(a.moved),
		moveReasons:        copyMoveReasons(a.moveReasons),
		closedResources:    copyMoved(a.closedResources),
		borrows:            copyBorrows(a.borrows),
		localRefContainers: copyLocalRefContainers(a.localRefContainers),
		arenaGenerations:   copyArenaGenerations(a.arenaGenerations),
		continues:          a.blockCanFallThrough(block) && !hasFallthrough,
		fallsThrough:       hasFallthrough,
	}
}

// switchStatementDefinitelyReturns reports whether every reachable path through
// an exhaustive switch terminates, following fallthrough chains to the next
// body.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §32 "Switch termination"
func (a *Analyzer) switchStatementDefinitelyReturns(stmt *ast.SwitchStatement) bool {
	if stmt == nil {
		return false
	}
	flow, resolved := a.resolvedSwitchFlows[stmt]
	exhaustive := resolved && flow.Exhaustive
	if !resolved {
		exhaustive = stmt.Default != nil || switchCoversBoolLiterals(stmt)
	}
	if !exhaustive {
		return false
	}

	nextTerminates := stmt.Default == nil
	if stmt.Default != nil {
		nextTerminates = a.blockDefinitelyReturns(stmt.Default.Body)
		if !nextTerminates {
			return false
		}
	}
	for index := len(stmt.Cases) - 1; index >= 0; index-- {
		clause := stmt.Cases[index]
		if clause == nil {
			return false
		}
		terminates := a.blockDefinitelyReturns(clause.Body) || blockEndsWithFallthrough(clause.Body) && nextTerminates
		if !terminates {
			return false
		}
		nextTerminates = true
	}
	return true
}

// rejectSwitchPattern reports a case item written as a match-style pattern: a
// wildcard `_`, or a variant constructor whose payload position names an
// unbound identifier. Such items attempt binding or destructuring, which only
// match supports. Ordinary constructed values with resolvable arguments remain
// value cases and are validated by equality compatibility.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §28 "No pattern matching"
//   - rules/control-flow/flowcontrol_switch.md — §40 "Required diagnostics": "switch does not support pattern binding; use match"
//   - rules/tooling/diagnostics.md — § 5(8) central registry allocation
func (a *Analyzer) rejectSwitchPattern(expr ast.Expression, subjectType Type) bool {
	switch expr := expr.(type) {
	case *ast.Identifier:
		if expr.Value != "_" {
			return false
		}
		a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.SwitchPatternBinding,
			"Use default for the remaining values, or match for pattern matching.",
			"switch does not support wildcard patterns; use default or match")
		return true
	case *ast.OkExpression:
		return a.rejectSwitchPayloadPattern(expr.Token, "Ok", expr.Arguments)
	case *ast.ErrExpression:
		return a.rejectSwitchPayloadPattern(expr.Token, "Err", expr.Arguments)
	case *ast.CallExpression:
		variant, ok := a.switchVariantCallee(expr, subjectType)
		if !ok {
			return false
		}
		return a.rejectSwitchPayloadPattern(expressionToken(expr), variant, expr.Arguments)
	}
	return false
}

func (a *Analyzer) rejectSwitchPayloadPattern(token lexer.Token, variant string, arguments []ast.Expression) bool {
	for _, argument := range arguments {
		if a.isUnboundSwitchPatternName(argument) {
			a.addErrorAtTokenWithMetadata(token, diagnostics.SwitchPatternBinding,
				"Use match to bind or destructure the "+variant+" payload.",
				"switch does not support pattern binding; use match")
			return true
		}
	}
	return false
}

// switchVariantCallee recognizes a call whose callee names a Result, Option,
// or union variant rather than a function, returning the variant name.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §28 "No pattern matching"
func (a *Analyzer) switchVariantCallee(call *ast.CallExpression, subjectType Type) (string, bool) {
	callee := call.Callee
	if callee == nil && call.Function != nil {
		callee = call.Function
	}
	switch callee := callee.(type) {
	case *ast.Identifier:
		name := callee.Value
		if _, shadowed := a.symbols[name]; shadowed {
			return "", false
		}
		if len(a.accessibleFunctions(a.functions[name])) > 0 {
			return "", false
		}
		if name == "Some" {
			return name, true
		}
		if unionHasVariant(subjectType, name) {
			return name, true
		}
	case *ast.MemberExpression:
		owner, ok := callee.Object.(*ast.Identifier)
		if !ok || callee.Property == nil {
			return "", false
		}
		if _, shadowed := a.symbols[owner.Value]; shadowed {
			return "", false
		}
		if typ, ok := a.types[owner.Value]; ok && unionHasVariant(typ, callee.Property.Value) {
			return owner.Value + "." + callee.Property.Value, true
		}
	}
	return "", false
}

func unionHasVariant(typ Type, name string) bool {
	if typ.Kind != UnionType {
		return false
	}
	for _, variant := range typ.UnionVariants {
		if variant.Name == name {
			return true
		}
	}
	return false
}

// isUnboundSwitchPatternName reports a payload argument that can only be a
// binding attempt: `_` or a plain name that resolves to no value, function,
// compiler-known value, or type in the current scope.
func (a *Analyzer) isUnboundSwitchPatternName(expr ast.Expression) bool {
	identifier, ok := expr.(*ast.Identifier)
	if !ok {
		return false
	}
	name := identifier.Value
	if name == "_" {
		return true
	}
	if _, ok := a.symbols[name]; ok {
		return false
	}
	if len(a.accessibleFunctions(a.functions[name])) > 0 {
		return false
	}
	if _, ok := compilerKnownValue(name); ok {
		return false
	}
	if _, ok := a.types[name]; ok {
		return false
	}
	return true
}

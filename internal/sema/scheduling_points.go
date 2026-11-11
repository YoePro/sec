package sema

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// ResolvedSchedulingPoint describes independent properties of a resolved yield
// operation. Task scheduling/cancellation are distinct from a physical scheduler
// hint; none of these facts establishes fairness, migration or synchronization.
// These are source operation requirements, not proof of a valid execution
// context/profile or implementation of the runtime cancellation commit race.
// Wait operations retain the semantics of their own rulebooks.
// Rules: rules/concurrency/scheduling.md — §§7–9,11–15,16;
// rules/concurrency/cancellation.md — current-execution cancellation points.
type ResolvedSchedulingPoint struct {
	Source                   lexer.Token
	MemberID                 string
	TaskSchedulingPoint      bool
	CurrentCancellationPoint bool
	PhysicalSchedulerHint    bool
}

// recordSchedulingPoint consumes the compiler-selected member identity only
// after successful signature validation, never classifying a source spelling
// or a similarly named user function as a concurrency operation.
// Rules: rules/concurrency/scheduling.md — §§7,11,15.
func (a *Analyzer) recordSchedulingPoint(call *ast.CallExpression, member CompilerKnownMember, source lexer.Token) {
	fact := ResolvedSchedulingPoint{Source: source, MemberID: member.ID}
	switch member.ID {
	case "CKM-TASK-YIELD":
		fact.TaskSchedulingPoint = true
		fact.CurrentCancellationPoint = true
	case "CKM-THREAD-YIELD":
		fact.PhysicalSchedulerHint = true
	default:
		return
	}
	a.schedulingPoints[call] = fact
}

// SchedulingPointOf returns the compiler's signature-resolved yield facts for
// this AST occurrence. The value contains no mutable analyzer-owned storage.
// Rules: rules/concurrency/scheduling.md — §§15,50,55–56.
func (a *Analyzer) SchedulingPointOf(call *ast.CallExpression) (ResolvedSchedulingPoint, bool) {
	if a == nil || call == nil {
		return ResolvedSchedulingPoint{}, false
	}
	fact, ok := a.schedulingPoints[call]
	return fact, ok
}

// SchedulingPoints returns a detached, source-ordered snapshot for compiler and
// tooling consumers; repeated inference of one call never duplicates its fact.
// Rules: rules/concurrency/scheduling.md — §§15,50,55–56;
// rules/compiler/semantic_ir.md — resolved facts rather than syntax inference.
func (a *Analyzer) SchedulingPoints() []ResolvedSchedulingPoint {
	if a == nil {
		return nil
	}
	facts := make([]ResolvedSchedulingPoint, 0, len(a.schedulingPoints))
	for _, fact := range a.schedulingPoints {
		facts = append(facts, fact)
	}
	sort.Slice(facts, func(i, j int) bool {
		left, right := facts[i], facts[j]
		if left.Source.File != right.Source.File {
			return left.Source.File < right.Source.File
		}
		if left.Source.Line != right.Source.Line {
			return left.Source.Line < right.Source.Line
		}
		if left.Source.Column != right.Source.Column {
			return left.Source.Column < right.Source.Column
		}
		return left.MemberID < right.MemberID
	})
	return facts
}

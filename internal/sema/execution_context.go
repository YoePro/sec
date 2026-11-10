package sema

import "sec/internal/ast"

// inferExecutionContextCall resolves the canonical static current-execution
// surface: Task.Current() TaskContext, Thread.Current() ThreadContext, and the
// Task.Yield()/Thread.Yield() operations. Task.Current/Task.Yield record the
// logical-task requirement checked against every execution root
// (task_context_requirement.go); the physical-thread capability of
// Thread.Current is a target-profile check (cancellation.md § 24).
//
// Rules:
//   - rules/concurrency/cancellation.md — §§ 8–11
//   - rules/concurrency/scheduling.md — Task.Yield and Thread.Yield
func (a *Analyzer) inferExecutionContextCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return Type{}, expressionValue{}, false
	}
	owner, ok := member.Object.(*ast.Identifier)
	if !ok || (owner.Value != "Task" && owner.Value != "Thread") || !a.expressionNamesType(owner) {
		return Type{}, expressionValue{}, false
	}
	name := owner.Value + "." + member.Property.Value
	var result Type
	switch member.Property.Value {
	case "Current":
		result = a.types[owner.Value+"Context"]
	case "Yield":
		result = Type{Name: "void", Kind: VoidType}
	default:
		return Type{}, expressionValue{}, false
	}
	if !a.checkCompilerKnownCallArity(expr, name, 0, 0) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if member.Property.Value == "Yield" {
		known, found := compilerKnownMember(a.types[owner.Value], "Yield", true)
		if found {
			a.compilerKnownMemberFacts[sourceTokenLocation(member.Property.Token)] = known
			result = known.Result
		}
	}
	if owner.Value == "Task" {
		a.recordExecutionRequirement(requiresLogicalTask, name+"()", member.Property.Token)
	}
	return result, expressionValue{Display: expr.String()}, true
}

package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
	"strings"
)

// forbiddenOperation classifies an executed operation whose effect semantic
// CTE never performs (compile_time_evaluation.md §§ 21, 47(3)): concurrency
// (spawn, await, select, detach, join and operations on tasks, threads,
// channels, mutexes, events and subscriptions), atomic operations and
// barriers, target-memory and volatile access, ambient nondeterministic
// input, and runtime I/O. The classification reads Sema facts only: the
// resolved receiver type, the compiler-known member bound at the call and
// Sema's fence recognition. Filesystem, network and environment access
// reach Sec code only through foreign or runtime calls, which are
// classified at the call itself.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 21, 47(3).
func (x *cteExecutor) forbiddenOperation(expr ast.Expression) (string, lexer.Token, bool) {
	switch expr := expr.(type) {
	case *ast.SpawnExpression:
		return "spawn starts concurrent execution", expr.Token, true
	case *ast.AwaitExpression:
		return "await waits on concurrent execution", expr.Token, true
	case *ast.RuntimeCallExpression:
		return "@" + expr.Name + " is a runtime operation with an observable I/O effect", expr.Token, true
	case *ast.CallExpression:
		if x.a.isAtomicFenceCall(expr) {
			return expr.String() + " is a memory barrier", expr.Token, true
		}
		member, ok := expr.Callee.(*ast.MemberExpression)
		if !ok || member.Property == nil {
			return "", lexer.Token{}, false
		}
		if known, ok := x.a.compilerKnownMemberFacts[sourceTokenLocation(member.Property.Token)]; ok {
			if strings.HasPrefix(known.ID, "CKM-ARENA-") {
				return expr.String() + " interacts with an explicit runtime allocator resource", member.Property.Token, true
			}
			for _, effect := range known.Effects {
				switch effect {
				case EffectVolatileRead, EffectVolatileWrite:
					return expr.String() + " is a volatile access", member.Property.Token, true
				case EffectMemoryRead, EffectMemoryWrite:
					return expr.String() + " accesses target memory", member.Property.Token, true
				case EffectMayUseNondeterministicInput:
					return expr.String() + " reads ambient nondeterministic input", member.Property.Token, true
				}
			}
		}
		receiver, ok := x.a.ResolvedTypeOf(member.Object)
		if !ok {
			return "", lexer.Token{}, false
		}
		switch {
		case isAtomicType(receiver):
			return expr.String() + " is an atomic operation", member.Property.Token, true
		case receiver.Kind == RawPtrType:
			return expr.String() + " accesses target memory", member.Property.Token, true
		case isTaskType(receiver), isThreadType(receiver), isThreadObserverType(receiver), isChannelType(receiver),
			isSenderType(receiver), isReceiverType(receiver), isMutexType(receiver), isMutexGuardType(receiver),
			isEventType(receiver), isSubscriptionType(receiver):
			return expr.String() + " is a concurrency operation", member.Property.Token, true
		}
	}
	return "", lexer.Token{}, false
}

// forbiddenStatement classifies concurrency statements the same way.
func forbiddenStatement(statement ast.Statement) (string, lexer.Token, bool) {
	switch statement := statement.(type) {
	case *ast.SelectStatement:
		return "select waits on concurrent communication", statement.Token, true
	case *ast.DetachStatement:
		return "detach transfers concurrent execution to the program", statement.Token, true
	case *ast.JoinStatement:
		return "join waits on a thread", statement.Token, true
	}
	return "", lexer.Token{}, false
}

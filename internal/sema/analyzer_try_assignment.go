package sema

import "sec/internal/ast"

// analyzeTryAssignmentStatement validates a fallible assignment and resolves
// either its local handler plan or its enclosing Result propagation edge.
// Keeping this statement-specific flow outside analyzer.go also prevents later
// lowering from having to infer propagation from an empty handler list.
//
// Rules:
//   - rules/errors/errorhandling.md — §8 "try and error compatibility"
//   - rules/errors/errorhandling.md — §23 "Fallible assignment"
//   - rules/foundations/grammar.md — TryAssignmentStatement
func (a *Analyzer) analyzeTryAssignmentStatement(stmt *ast.TryAssignmentStatement) {
	if stmt == nil || stmt.Assignment == nil {
		return
	}

	errorsBefore := len(a.errors)
	a.analyzeAssignmentStatement(stmt.Assignment, true)
	if len(a.errors) != errorsBefore {
		return
	}

	errorType, ok := a.tryAssignmentErrorType(stmt.Assignment)
	if !ok {
		a.addErrorAtToken(stmt.Token, "try assignment requires a fallible assignment target with a known error type")
		return
	}
	if len(stmt.Handlers) > 0 {
		a.analyzeTryAssignmentHandlers(stmt, errorType)
		return
	}

	if a.inDeferBlock {
		a.addErrorAtToken(stmt.Token, "naked try assignment cannot propagate from inside defer; add a local try handler")
		return
	}
	if !a.inFunctionBody {
		a.addErrorAtToken(stmt.Token, "naked try assignment cannot propagate outside a function; add a local try handler")
		return
	}
	if a.currentFunctionReturn.Kind != ResultType || len(a.currentFunctionReturn.TypeArgs) != 2 {
		a.addErrorAtToken(stmt.Token, "naked try assignment propagates %s with return Err, but this function returns %s; add a local try handler or change the function return type to Result[void, %s]", typeDisplayName(errorType), typeDisplayName(a.currentFunctionReturn), typeDisplayName(errorType))
		return
	}
	functionErrorType := a.currentFunctionReturn.TypeArgs[1]
	if !canInitialize(functionErrorType, errorType, stmt.Assignment.Value) {
		a.addErrorAtToken(stmt.Token, "naked try assignment propagates %s with return Err, but this function returns %s; add a local try handler or map %s to %s", typeDisplayName(errorType), typeDisplayName(a.currentFunctionReturn), typeDisplayName(errorType), typeDisplayName(functionErrorType))
		return
	}
	a.resolvedTryAssignments[stmt] = ResolvedTryAssignment{
		Kind:                ResolvedTryAssignmentPropagation,
		ErrorType:           errorType,
		EnclosingResultType: a.currentFunctionReturn,
	}
}

func (a *Analyzer) analyzeTryAssignmentHandlers(stmt *ast.TryAssignmentStatement, errorType Type) {
	resultType := Type{
		Name:     "Result",
		Kind:     ResultType,
		TypeArgs: []Type{{Name: "void", Kind: VoidType}, errorType},
	}
	expr := &ast.TryExpression{
		Token:      stmt.Token,
		Expression: &ast.Identifier{Token: stmt.Token, Value: "__try_assignment"},
		Handlers:   stmt.Handlers,
	}
	plan, valid := a.analyzeTryHandlers(expr, resultType)
	if valid {
		a.resolvedTryAssignments[stmt] = ResolvedTryAssignment{
			Kind:        ResolvedTryAssignmentHandled,
			ErrorType:   errorType,
			HandlerPlan: plan,
		}
	}
}

func (a *Analyzer) tryAssignmentErrorType(stmt *ast.AssignmentStatement) (Type, bool) {
	switch target := stmt.Target.(type) {
	case *ast.MemberExpression:
		property, ok := a.lookupPropertyOnMember(target)
		if !ok || property.Error == nil {
			return Type{}, false
		}
		return *property.Error, true
	case *ast.Identifier:
		property, ok := a.lookupCurrentImplProperty(target.Value)
		if ok {
			if property.Error == nil {
				return Type{}, false
			}
			return *property.Error, true
		}
		symbol, ok := a.symbols[target.Value]
		if ok && hasContracts(symbol.Type) {
			return a.types["ContractError"], true
		}
	}
	return Type{}, false
}

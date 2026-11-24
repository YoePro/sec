package sema

import "sec/internal/ast"

// spawnBlockHasValueReturn excludes nested callable definitions and spawn
// bodies from the enclosing child's normal return candidates. Defer returns
// remain invalid under ordinary defer validation; they cannot infer a result.
// Rules: rules/concurrency/spawn.md §§3A(3–6); rules/control-flow/defer.md §3.
func spawnBlockHasValueReturn(body *ast.BlockStatement) bool {
	found := false
	ast.Visit(body, func(node any) bool {
		switch n := node.(type) {
		case *ast.LambdaExpression, *ast.FunctionDeclaration, *ast.SpawnExpression, *ast.DeferStatement:
			return false
		case *ast.ReturnStatement:
			found = found || n.Value != nil
		}
		return true
	})
	return found
}

// voidSpawnCallable supplies the established void return contract to the
// ordinary callable analyzer. This is an analysis-only view: the source keeps
// its SpawnExpression/Body and every expression statement remains a discard.
// With no capture clause, enclosing locals are hidden by lambda validation.
// Rules: rules/concurrency/spawn.md §§3A(2–7),16; rules/declarations/lambda-functions.md §§13–21.
func voidSpawnCallable(expr *ast.SpawnExpression) *ast.LambdaExpression {
	return &ast.LambdaExpression{Token: expr.Token,
		ReturnType: &ast.TypeReference{Token: expr.Token, Name: "void"}, Body: expr.Body}
}

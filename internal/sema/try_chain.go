package sema

import "sec/internal/ast"

// try protects its complete following expression, including every
// successive member and call of a chain: the successful value of a fallible
// intermediate Result is the receiver of the next member, and its failure is
// one more point of the compiler-internal failure set, evaluated before the
// member that consumes it. A member that Result itself provides (such as Ok
// or OkRef) keeps addressing the Result.
//
// Rules:
//   - rules/errors/errorhandling.md — §13 "try expression boundaries", §11 "Compiler-internal failure sets"
//   - rules/errors/runtime_checks.md — "try boundaries"
//   - rules/corrections/applied/md028-md032-md055-try-switch-import-comments-correction-20261012.md — §§ 2.1-2.3

// markTryChainReceivers marks every member receiver of the protected
// expression, stopping at nested try, lambda and match boundaries, and
// returns the marked receivers so the caller can clear them afterwards.
func (a *Analyzer) markTryChainReceivers(expr ast.Expression) []ast.Expression {
	if a.tryChainReceivers == nil {
		a.tryChainReceivers = map[ast.Expression]string{}
	}
	var marked []ast.Expression
	mark := func(receiver ast.Expression, member string) {
		if receiver == nil {
			return
		}
		if _, exists := a.tryChainReceivers[receiver]; !exists {
			a.tryChainReceivers[receiver] = member
			marked = append(marked, receiver)
		}
	}
	var visit func(ast.Expression)
	visit = func(expr ast.Expression) {
		switch node := expr.(type) {
		case nil, *ast.TryExpression, *ast.LambdaExpression, *ast.MatchExpression:
			return
		case *ast.MemberExpression:
			if node.Property != nil {
				mark(node.Object, node.Property.Value)
			}
			visit(node.Object)
		case *ast.CallExpression:
			if member, ok := node.Callee.(*ast.MemberExpression); ok {
				if member.Property != nil {
					mark(member.Object, member.Property.Value)
				}
				visit(member.Object)
			}
			for _, argument := range node.Arguments {
				visit(argument)
			}
		case *ast.InfixExpression:
			visit(node.Left)
			visit(node.Right)
		case *ast.PrefixExpression:
			visit(node.Right)
		case *ast.IndexExpression:
			visit(node.Left)
			visit(node.Index)
		case *ast.SliceExpression:
			visit(node.Left)
			visit(node.Start)
			visit(node.End)
		case *ast.BitExtractionExpression:
			visit(node.Value)
		case *ast.ConversionExpression:
			visit(node.Value)
		case *ast.NewExpression:
			for _, argument := range node.Arguments {
				visit(argument)
			}
		case *ast.RefExpression:
			visit(node.Value)
		case *ast.ArrayLiteral:
			for _, element := range node.Elements {
				visit(element)
			}
		case *ast.StructLiteral:
			for _, field := range node.Fields {
				if field != nil {
					visit(field.Value)
				}
			}
		case *ast.OkExpression:
			visit(node.Value)
		case *ast.ErrExpression:
			visit(node.Value)
		case *ast.SpreadExpression:
			visit(node.Value)
		case *ast.InterpolatedStringLiteral:
			for _, part := range node.Parts {
				visit(part.Expression)
			}
		}
	}
	visit(expr)
	return marked
}

// unwrapTryChainReceiver gives a marked receiver of type Result[T, E] the
// success type T when the member it is accessed through is not a Result
// member, and records the unwrap; the receiver's own expression type stays
// the Result.
func (a *Analyzer) unwrapTryChainReceiver(expr ast.Expression, typ Type) (Type, bool) {
	member, marked := a.tryChainReceivers[expr]
	if !marked || typ.Kind != ResultType || len(typ.TypeArgs) != 2 {
		return Type{}, false
	}
	if _, resultMember := compilerKnownMember(typ, member, false); resultMember {
		return Type{}, false
	}
	if a.tryUnwrappedReceivers == nil {
		a.tryUnwrappedReceivers = map[ast.Expression]Type{}
	}
	a.tryUnwrappedReceivers[expr] = typ
	return typ.TypeArgs[0], true
}

// appendTryChainCarrierPoint adds the failure of an unwrapped intermediate
// Result receiver after the points of the receiver's own subtree.
func (a *Analyzer) appendTryChainCarrierPoint(points []TryFailurePoint, receiver ast.Expression) []TryFailurePoint {
	result, unwrapped := a.tryUnwrappedReceivers[receiver]
	if !unwrapped {
		return points
	}
	return append(points, TryFailurePoint{Kind: TryFailureCarrier, ErrorType: result.TypeArgs[1], Expression: receiver, Token: expressionToken(receiver)})
}

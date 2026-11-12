package sema

import "sec/internal/ast"

type implReceiverBody struct {
	Target   Type
	Name     string
	Body     *ast.BlockStatement
	Shadowed map[string]bool
	Accessor *Function
}

// resolveImplMethodReceiverMutability computes receiver mutability after every
// impl method has been registered. Method declarations may be in any source
// order, and a mutating receiver requirement can propagate through calls or
// property reads to a fixed point shared by methods and getters.
// Rules: rules/declarations/impl.md — §8;
// rules/declarations/properties.md — §8; rules/memory/borrowing.md — §7.
func (a *Analyzer) resolveImplMethodReceiverMutability(program *ast.Program) {
	if program == nil {
		return
	}

	methods := []implReceiverBody{}
	for _, stmt := range program.Statements {
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok || impl == nil || !a.validImplStatements[impl] || impl.Target == nil {
			continue
		}
		target, ok := a.types[impl.Target.Name]
		if !ok {
			continue
		}
		for _, member := range impl.Members {
			if declaration, ok := member.(*ast.PropertyDeclaration); ok && declaration != nil && declaration.Name != nil && !declaration.Static {
				if property, found := lookupPropertyByToken(target, declaration.Name.Value, declaration.Name.Token); found && property.GetterCallable != nil {
					methods = append(methods, implReceiverBody{Target: target, Body: declaration.Getter, Shadowed: map[string]bool{}, Accessor: property.GetterCallable})
				}
				continue
			}
			fn, ok := member.(*ast.FunctionDeclaration)
			if !ok || fn == nil || fn.Name == nil {
				continue
			}
			name := target.Name + "." + fn.Name.Value
			if len(a.functions[name]) == 0 {
				continue
			}
			methods = append(methods, implReceiverBody{Target: target, Name: name, Body: fn.Body, Shadowed: functionParameterNames(fn)})
		}
	}

	for _, method := range methods {
		if method.Accessor != nil {
			method.Accessor.ReceiverMutable = false
		} else {
			a.setImplMethodReceiverMutable(method.Name, method.Target.Name, false)
		}
	}
	for changed := true; changed; {
		changed = false
		for _, method := range methods {
			if !a.functionBodyWritesTargetMember(method.Body, method.Target, method.Shadowed) {
				continue
			}
			if method.Accessor != nil {
				if !method.Accessor.ReceiverMutable {
					method.Accessor.ReceiverMutable = true
					changed = true
				}
				continue
			}
			if a.implMethodReceiverMutable(method.Name, method.Target.Name) {
				continue
			}
			a.setImplMethodReceiverMutable(method.Name, method.Target.Name, true)
			changed = true
		}
	}
}

// implMethodReceiverMutable reads a registered method receiver requirement.
// Rules: rules/declarations/impl.md — §8; rules/declarations/properties.md — §8;
// rules/memory/borrowing.md — §7.
func (a *Analyzer) implMethodReceiverMutable(name string, targetName string) bool {
	for _, function := range a.functions[name] {
		if function.ImplTarget == targetName && function.ReceiverMutable {
			return true
		}
	}
	return false
}

// setImplMethodReceiverMutable updates method receiver requirements during fixed-point inference.
// Rules: rules/declarations/impl.md — §8; rules/declarations/properties.md — §8;
// rules/memory/borrowing.md — §7.
func (a *Analyzer) setImplMethodReceiverMutable(name string, targetName string, mutable bool) {
	functions := a.functions[name]
	for index := range functions {
		if functions[index].ImplTarget == targetName {
			functions[index].ReceiverMutable = mutable
		}
	}
	a.functions[name] = functions
}

// functionBodyWritesTargetMember derives receiver mutation from body operations.
// Rules: rules/declarations/impl.md — §8; rules/declarations/properties.md — §8;
// rules/memory/borrowing.md — §7.
func (a *Analyzer) functionBodyWritesTargetMember(block *ast.BlockStatement, target Type, shadowed map[string]bool) bool {
	if block == nil {
		return false
	}
	return a.blockWritesTargetMember(block, target, targetAssignableMemberNames(target), shadowed)
}

// targetAssignableMemberNames collects implicit writable receiver member names.
// Rules: rules/declarations/impl.md — §8; rules/declarations/properties.md — §8;
// rules/memory/borrowing.md — §7.
func targetAssignableMemberNames(target Type) map[string]bool {
	names := map[string]bool{}
	for _, field := range target.Fields {
		names[field.Name] = true
	}
	for _, field := range target.RegisterFields {
		if field.Name != "_" {
			names[field.Name] = true
		}
	}
	for _, property := range target.Properties {
		if property.HasSetter {
			names[property.Name] = true
		}
	}
	return names
}

// statementUsesSelf recognizes receiver uses in a statement.
// Rules: rules/declarations/impl.md — §8; rules/declarations/properties.md — §8;
// rules/memory/borrowing.md — §7.
func statementUsesSelf(stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case *ast.LetStatement:
		return expressionUsesSelf(stmt.Value) || expressionUsesSelf(stmt.Address)
	case *ast.LetGroupStatement:
		for _, let := range stmt.Lets {
			if statementUsesSelf(let) {
				return true
			}
		}
	case *ast.AssignmentStatement:
		return expressionUsesSelf(stmt.Target) || expressionUsesSelf(stmt.Value)
	case *ast.TryAssignmentStatement:
		return stmt.Assignment != nil && statementUsesSelf(stmt.Assignment)
	case *ast.ExpressionStatement:
		return expressionUsesSelf(stmt.Expression)
	case *ast.AssertStatement:
		return expressionUsesSelf(stmt.Condition)
	case *ast.DetachStatement:
		return expressionUsesSelf(stmt.Value)
	case *ast.JoinStatement:
		return expressionUsesSelf(stmt.Value)
	case *ast.ReturnStatement:
		return expressionUsesSelf(stmt.Value)
	case *ast.IfStatement:
		return expressionUsesSelf(stmt.Condition) || functionBodyUsesSelf(stmt.Consequence) || functionBodyUsesSelf(stmt.Alternative)
	case *ast.ForStatement:
		return expressionUsesSelf(stmt.Iterable) || expressionUsesSelf(stmt.Step) || functionBodyUsesSelf(stmt.Body)
	case *ast.WhileStatement:
		return expressionUsesSelf(stmt.Condition) || functionBodyUsesSelf(stmt.Body)
	case *ast.DeferStatement:
		return functionBodyUsesSelf(stmt.Body)
	case *ast.UnsafeStatement:
		return functionBodyUsesSelf(stmt.Body)
	case *ast.MatchStatement:
		return expressionUsesSelf(stmt.Match)
	}
	return false
}

// statementWritesSelf recognizes direct receiver writes in a statement.
// Rules: rules/declarations/impl.md — §8; rules/declarations/properties.md — §8;
// rules/memory/borrowing.md — §7.
func statementWritesSelf(stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case *ast.AssignmentStatement:
		return assignmentTargetUsesSelf(stmt.Target)
	case *ast.TryAssignmentStatement:
		return stmt.Assignment != nil && statementWritesSelf(stmt.Assignment)
	case *ast.LetGroupStatement:
		return false
	case *ast.IfStatement:
		return functionBodyWritesSelf(stmt.Consequence) || functionBodyWritesSelf(stmt.Alternative)
	case *ast.ForStatement:
		return functionBodyWritesSelf(stmt.Body)
	case *ast.WhileStatement:
		return functionBodyWritesSelf(stmt.Body)
	case *ast.DeferStatement:
		return functionBodyWritesSelf(stmt.Body)
	case *ast.UnsafeStatement:
		return functionBodyWritesSelf(stmt.Body)
	case *ast.MatchStatement:
		if stmt.Match == nil {
			return false
		}
		for _, arm := range stmt.Match.Arms {
			if arm != nil && functionBodyWritesSelf(arm.BlockBody) {
				return true
			}
		}
	}
	return false
}

// blockWritesTargetMember tracks lexical member shadowing across a receiver body.
// Rules: rules/declarations/impl.md — §8; rules/declarations/properties.md — §8;
// rules/memory/borrowing.md — §7.
func (a *Analyzer) blockWritesTargetMember(block *ast.BlockStatement, target Type, memberNames map[string]bool, inheritedShadowed map[string]bool) bool {
	if block == nil {
		return false
	}
	shadowed := copyBoolMap(inheritedShadowed)
	for _, stmt := range block.Statements {
		if a.statementWritesTargetMember(stmt, target, memberNames, shadowed) {
			return true
		}
		addStatementDeclarationsToShadowed(stmt, shadowed)
	}
	return false
}

// statementWritesTargetMember combines receiver mutation requirements of statement operations.
// Rules: rules/declarations/impl.md — §8; rules/declarations/properties.md — §8;
// rules/memory/borrowing.md — §7.
func (a *Analyzer) statementWritesTargetMember(stmt ast.Statement, target Type, memberNames map[string]bool, shadowed map[string]bool) bool {
	// A `ref mut` borrow of a receiver member, or a `ref mut` payload
	// binding over a receiver-rooted match subject, needs a mutable receiver
	// as much as an assignment does (borrowing.md § 7).
	if statementMutablyBorrowsReceiver(stmt, memberNames, shadowed) {
		return true
	}
	switch stmt := stmt.(type) {
	case *ast.MatchStatement:
		if stmt.Match == nil {
			return false
		}
		if a.expressionCallsMutableTargetMethod(stmt.Match.Subject, target, shadowed) {
			return true
		}
		for _, arm := range stmt.Match.Arms {
			if arm == nil {
				continue
			}
			armShadowed := shadowed
			if arm.Pattern != nil && arm.Pattern.Binding != nil && arm.Pattern.Binding.Name != nil {
				armShadowed = copyBoolMap(shadowed)
				armShadowed[arm.Pattern.Binding.Name.Value] = true
			}
			if a.expressionCallsMutableTargetMethod(arm.Guard, target, armShadowed) || a.expressionCallsMutableTargetMethod(arm.Body, target, armShadowed) ||
				arm.ReturnBody != nil && a.expressionCallsMutableTargetMethod(arm.ReturnBody.Value, target, armShadowed) ||
				a.blockWritesTargetMember(arm.BlockBody, target, memberNames, armShadowed) {
				return true
			}
		}
		return false
	case *ast.LetStatement:
		return stmt.Ownership == ast.OwnershipMove && isSelfMemberProjection(stmt.Value) || a.expressionCallsMutableTargetMethod(stmt.Value, target, shadowed)
	case *ast.LetGroupStatement:
		for _, let := range stmt.Lets {
			if let != nil && (let.Ownership == ast.OwnershipMove && isSelfMemberProjection(let.Value) || a.expressionCallsMutableTargetMethod(let.Value, target, shadowed)) {
				return true
			}
		}
		return false
	case *ast.AssignmentStatement:
		return assignmentTargetUsesSelf(stmt.Target) || assignmentTargetUsesUnshadowedMember(stmt.Target, memberNames, shadowed) ||
			stmt.Ownership == ast.OwnershipMove && isSelfMemberProjection(stmt.Value) || a.expressionCallsMutableTargetMethod(stmt.Value, target, shadowed)
	case *ast.TryAssignmentStatement:
		return stmt.Assignment != nil && a.statementWritesTargetMember(stmt.Assignment, target, memberNames, shadowed)
	case *ast.ExpressionStatement:
		return a.expressionCallsMutableTargetMethod(stmt.Expression, target, shadowed)
	case *ast.DiscardStatement:
		return isSelfMemberProjection(stmt.Value) || a.expressionCallsMutableTargetMethod(stmt.Value, target, shadowed)
	case *ast.AssertStatement:
		return a.expressionCallsMutableTargetMethod(stmt.Condition, target, shadowed)
	case *ast.ReturnStatement:
		return a.expressionCallsMutableTargetMethod(stmt.Value, target, shadowed)
	case *ast.IfStatement:
		return a.expressionCallsMutableTargetMethod(stmt.Condition, target, shadowed) || a.blockWritesTargetMember(stmt.Consequence, target, memberNames, shadowed) || a.blockWritesTargetMember(stmt.Alternative, target, memberNames, shadowed)
	case *ast.ForStatement:
		loopShadowed := copyBoolMap(shadowed)
		for _, binding := range stmt.Bindings {
			if !binding.Discard {
				loopShadowed[binding.Name] = true
			}
		}
		return a.expressionCallsMutableTargetMethod(stmt.Iterable, target, shadowed) || a.expressionCallsMutableTargetMethod(stmt.Step, target, shadowed) || a.blockWritesTargetMember(stmt.Body, target, memberNames, loopShadowed)
	case *ast.WhileStatement:
		return a.expressionCallsMutableTargetMethod(stmt.Condition, target, shadowed) || a.blockWritesTargetMember(stmt.Body, target, memberNames, shadowed)
	case *ast.DeferStatement:
		return a.blockWritesTargetMember(stmt.Body, target, memberNames, shadowed)
	case *ast.UnsafeStatement:
		return a.blockWritesTargetMember(stmt.Body, target, memberNames, shadowed)
	case *ast.SwitchStatement:
		// Every case body is part of the method body: a write or a mutating
		// self call in one case makes the receiver mutable (impl.md §8).
		if a.expressionCallsMutableTargetMethod(stmt.Subject, target, shadowed) {
			return true
		}
		cases := stmt.Cases
		if stmt.Default != nil {
			cases = append(append([]*ast.SwitchCase(nil), cases...), stmt.Default)
		}
		for _, clause := range cases {
			if clause != nil && a.blockWritesTargetMember(clause.Body, target, memberNames, shadowed) {
				return true
			}
		}
		return false
	case *ast.SelectStatement:
		for _, branch := range stmt.Branches {
			if branch == nil {
				continue
			}
			if a.expressionCallsMutableTargetMethod(branch.Value, target, shadowed) {
				return true
			}
			branchShadowed := copyBoolMap(shadowed)
			if branch.Binding != nil {
				branchShadowed[branch.Binding.Value] = true
			}
			if a.blockWritesTargetMember(branch.Body, target, memberNames, branchShadowed) {
				return true
			}
		}
	}
	return false
}

// expressionCallsMutableTargetMethod reports calls that require this impl
// method's implicit self receiver to be writable. Every evaluated expression
// retains that requirement, including returned and discarded call results.
// Rules: rules/declarations/impl.md — §8; rules/declarations/properties.md — §8.
func (a *Analyzer) expressionCallsMutableTargetMethod(expr ast.Expression, target Type, shadowed map[string]bool) bool {
	switch expr := expr.(type) {
	case *ast.Identifier:
		if shadowed[expr.Value] {
			return false
		}
		property, ok := a.lookupResolvedProperty(target, expr.Value)
		return ok && !property.Static && property.GetterCallable != nil && property.GetterCallable.ReceiverMutable
	case nil, *ast.IntegerLiteral, *ast.FloatLiteral, *ast.BooleanLiteral, *ast.CharLiteral, *ast.StringLiteral:
		return false
	case *ast.PrefixExpression:
		if expr.Operator == "<-" && isSelfMemberProjection(expr.Right) {
			return true
		}
		return a.expressionCallsMutableTargetMethod(expr.Right, target, shadowed)
	case *ast.InfixExpression:
		return a.expressionCallsMutableTargetMethod(expr.Left, target, shadowed) || a.expressionCallsMutableTargetMethod(expr.Right, target, shadowed)
	case *ast.RangeExpression:
		return a.expressionCallsMutableTargetMethod(expr.Start, target, shadowed) || a.expressionCallsMutableTargetMethod(expr.End, target, shadowed)
	case *ast.BitExtractionExpression:
		return a.expressionCallsMutableTargetMethod(expr.Value, target, shadowed)
	case *ast.ConversionExpression:
		return a.expressionCallsMutableTargetMethod(expr.Value, target, shadowed)
	case *ast.OkExpression:
		return a.expressionCallsMutableTargetMethod(expr.Value, target, shadowed)
	case *ast.ErrExpression:
		return a.expressionCallsMutableTargetMethod(expr.Value, target, shadowed)
	case *ast.TryExpression:
		return a.expressionCallsMutableTargetMethod(expr.Expression, target, shadowed)
	case *ast.MemberExpression:
		if receiver, ok := expr.Object.(*ast.Identifier); ok && receiver.Value == "self" && expr.Property != nil {
			if property, found := a.lookupResolvedProperty(target, expr.Property.Value); found && !property.Static && property.GetterCallable != nil && property.GetterCallable.ReceiverMutable {
				return true
			}
		}
		return a.expressionCallsMutableTargetMethod(expr.Object, target, shadowed)
	case *ast.IndexExpression:
		return a.expressionCallsMutableTargetMethod(expr.Left, target, shadowed) || a.expressionCallsMutableTargetMethod(expr.Index, target, shadowed)
	case *ast.SliceExpression:
		return a.expressionCallsMutableTargetMethod(expr.Left, target, shadowed) || a.expressionCallsMutableTargetMethod(expr.Start, target, shadowed) || a.expressionCallsMutableTargetMethod(expr.End, target, shadowed)
	case *ast.RefExpression:
		return a.expressionCallsMutableTargetMethod(expr.Value, target, shadowed)
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if a.expressionCallsMutableTargetMethod(element, target, shadowed) {
				return true
			}
		}
	case *ast.SpreadExpression:
		return a.expressionCallsMutableTargetMethod(expr.Value, target, shadowed)
	case *ast.StructLiteral:
		for _, field := range expr.Fields {
			if field != nil && a.expressionCallsMutableTargetMethod(field.Value, target, shadowed) {
				return true
			}
		}
	case *ast.CallExpression:
		if a.callRequiresMutableTargetReceiver(expr, target) {
			return true
		}
		if a.expressionCallsMutableTargetMethod(expr.Callee, target, shadowed) {
			return true
		}
		for _, argument := range expr.Arguments {
			if a.expressionCallsMutableTargetMethod(argument, target, shadowed) {
				return true
			}
		}
	}
	return false
}

// callRequiresMutableTargetReceiver recognizes a call to a mutable method on self.
// Rules: rules/declarations/impl.md — §8; rules/declarations/properties.md — §8;
// rules/memory/borrowing.md — §7.
func (a *Analyzer) callRequiresMutableTargetReceiver(call *ast.CallExpression, target Type) bool {
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return false
	}
	receiver, ok := member.Object.(*ast.Identifier)
	if !ok || receiver.Value != "self" {
		return false
	}
	for _, function := range a.functions[target.Name+"."+member.Property.Value] {
		if function.ImplTarget == target.Name && function.ReceiverMutable {
			return true
		}
	}
	return false
}

// addStatementDeclarationsToShadowed tracks local declarations that hide implicit member names.
// Rules: rules/declarations/impl.md — §8; rules/declarations/properties.md — §8;
// rules/memory/borrowing.md — §7.
func addStatementDeclarationsToShadowed(stmt ast.Statement, shadowed map[string]bool) {
	switch stmt := stmt.(type) {
	case *ast.LetStatement:
		if stmt != nil && stmt.Name != nil {
			shadowed[stmt.Name.Value] = true
		}
	case *ast.LetGroupStatement:
		if stmt == nil {
			return
		}
		for _, let := range stmt.Lets {
			if let != nil && let.Name != nil {
				shadowed[let.Name.Value] = true
			}
		}
	}
}

// assignmentTargetUsesSelf recognizes receiver-rooted assignment targets.
// Rules: rules/declarations/impl.md — §8; rules/declarations/properties.md — §8;
// rules/memory/borrowing.md — §7.
func assignmentTargetUsesSelf(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case *ast.MemberExpression:
		return expressionUsesSelf(expr.Object)
	case *ast.IndexExpression:
		return assignmentTargetUsesSelf(expr.Left)
	}
	return false
}

// assignmentTargetUsesUnshadowedMember recognizes unshadowed implicit member assignment targets.
// Rules: rules/declarations/impl.md — §8; rules/declarations/properties.md — §8;
// rules/memory/borrowing.md — §7.
func assignmentTargetUsesUnshadowedMember(expr ast.Expression, memberNames map[string]bool, shadowed map[string]bool) bool {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return memberNames[expr.Value] && !shadowed[expr.Value]
	case *ast.MemberExpression:
		return expressionUsesSelf(expr.Object)
	case *ast.IndexExpression:
		return assignmentTargetUsesUnshadowedMember(expr.Left, memberNames, shadowed)
	}
	return false
}

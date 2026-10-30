package sema

import (
	"reflect"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// staticDeclaration is the frontend dependency node required by
// rules/declarations/static.md, sections 15-16.
type staticDeclaration struct {
	name  string
	owner string
	token lexer.Token
	value ast.Expression
}

// predeclareModuleStaticStorage makes explicitly typed module storage visible
// independently of source-file discovery order (static.md, sections 3 and 16).
func (a *Analyzer) predeclareModuleStaticStorage(program *ast.Program) {
	a.withProgramModules(program, func(statement ast.Statement) {
		var declarations []*ast.LetStatement
		switch statement := statement.(type) {
		case *ast.LetStatement:
			declarations = []*ast.LetStatement{statement}
		case *ast.LetGroupStatement:
			declarations = statement.Lets
		default:
			return
		}
		for _, declaration := range declarations {
			if declaration == nil || declaration.Name == nil || declaration.Type == nil {
				continue
			}
			if _, known := a.types[a.resolveTypeName(declaration.Type.Name)]; !known {
				continue
			}
			typ, ok := a.resolveType(declaration.Type)
			if !ok || !a.defineSymbol(declaration.Name.Value, typ, declaration.Mutable, declaration.Name.Token) {
				continue
			}
			a.predeclaredStatic[sourceTokenLocation(declaration.Name.Token)] = true
		}
	})
}

func (a *Analyzer) validateStaticInitialization(program *ast.Program) {
	if program == nil {
		return
	}
	declarations := map[string]staticDeclaration{}
	a.collectFunctionStaticHolders(program)
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *ast.LetStatement:
			if statement == nil {
				continue
			}
			a.addStaticDeclaration(declarations, "", statement, true)
		case *ast.LetGroupStatement:
			if statement == nil {
				continue
			}
			for _, declaration := range statement.Lets {
				a.addStaticDeclaration(declarations, "", declaration, true)
			}
		case *ast.ImplStatement:
			if statement == nil || statement.Target == nil {
				continue
			}
			for _, member := range statement.Members {
				if declaration, ok := member.(*ast.LetStatement); ok {
					a.addStaticDeclaration(declarations, statement.Target.Name, declaration, false)
				}
			}
		}
	}

	names := make([]string, 0, len(declarations))
	for name := range declarations {
		names = append(names, name)
	}
	sort.SliceStable(names, func(i, j int) bool {
		left, right := declarations[names[i]].token, declarations[names[j]].token
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		return names[i] < names[j]
	})
	staticRoots := map[string]bool{}
	for _, name := range names {
		staticRoots[name] = true
	}
	for _, name := range names {
		declaration := declarations[name]
		a.validateStaticInitializerExpression(declaration.name, declaration.value, declaration.token, staticRoots)
	}
	a.validateStaticDependencyCycles(declarations)
}

// addStaticDeclaration adds module storage and explicitly static impl storage
// to the shared compile-time dependency graph. Instance initializers are excluded.
//
// Rules:
//   - rules/declarations/static.md — "Static initialization"
//   - rules/declarations/static.md — "Static initialization dependency order"
func (a *Analyzer) addStaticDeclaration(declarations map[string]staticDeclaration, owner string, declaration *ast.LetStatement, module bool) {
	if declaration == nil || declaration.Name == nil || (!module && !declaration.Static) {
		return
	}
	name := declaration.Name.Value
	if owner != "" {
		name = owner + "." + name
	}
	declarations[name] = staticDeclaration{name: name, owner: owner, token: declaration.Name.Token, value: declaration.Value}
}

// validateStaticInitializerExpression rejects operations that require hidden
// runtime startup execution (static.md, section 15). staticRoots names the
// module and type-owned static declarations of the module pass, whose symbols
// may not be defined yet when it runs.
func (a *Analyzer) validateStaticInitializerExpression(name string, expression ast.Expression, token lexer.Token, staticRoots map[string]bool) {
	if expression == nil {
		return
	}
	invalid, reported := false, false
	settled := map[ast.Expression]bool{}
	visitStatementExpressions(&ast.LetStatement{Value: expression}, func(candidate ast.Expression) {
		if invalid || settled[candidate] {
			return
		}
		switch candidate := candidate.(type) {
		case *ast.RuntimeCallExpression, *ast.NewExpression, *ast.TryExpression,
			*ast.SpawnExpression, *ast.AwaitExpression, *ast.LambdaExpression:
			invalid = true
		case *ast.RefExpression:
			// MD-081: `ref S` / `ref mut S` of a static Place, or of a field or
			// constant-index element of one, is a materializable static
			// reference. Authority, overlap, bounds and lifetime are checked by
			// the ordinary borrow analysis of the initializer.
			if !a.validateStaticReferenceInitializer(name, candidate, token, staticRoots, settled) {
				invalid, reported = true, true
				return
			}
		case *ast.CallExpression:
			callee, ok := typePathFromExpression(candidate.Callee)
			if !ok && candidate.Function != nil {
				callee = candidate.Function.Value
				ok = true
			}
			if ok && (len(a.functions[callee]) > 0 || a.externSymbols[callee].Name != "") {
				invalid = true
			}
		case *ast.Identifier:
			// A clock read is runtime nondeterministic input, even though it is
			// spelled as a value rather than a call.
			//
			// Rules:
			//   - rules/types/temporal.md — §3 "UTC wall-clock access"
			//   - rules/compiler/compiler_known_members.md — "Private core UTC wall-clock intrinsic"
			if _, known := compilerKnownValue(candidate.Value); known {
				invalid = true
				return
			}
			// A function-local static initializer cannot capture an invocation's
			// parameters or automatic locals (static.md, sections 4 and 15).
			if symbol, ok := a.symbols[candidate.Value]; ok && symbol.Local {
				invalid = true
			}
		case *ast.MemberExpression:
			if path, ok := typePathFromExpression(candidate.Object); ok {
				if typ, exists := a.types[a.resolveTypeName(path)]; exists {
					if property, found := lookupProperty(typ, candidate.Property.Value); found && property.Static {
						invalid = true
					}
				}
			}
		}
	})
	if invalid && !reported {
		a.addErrorAtToken(token, "static initializer for %s must be compile-time evaluable; runtime execution or invocation-local state is not allowed", name)
	}
}

// validateStaticReferenceInitializer checks the origin of a reference in a
// static initializer: a named static root (module, type-owned or
// function-local static storage) reached through fields and compile-time
// indexes only. It reports a reference-specific diagnostic, marks the Place
// path as settled so its parts are not re-reported as runtime state, and
// returns whether the origin is valid; a rejected origin replaces the generic
// runtime-initializer error.
//
// Rules:
//   - rules/declarations/static.md — §§ 5, 15 (Amendment MD-081)
//   - rules/corrections/applied/MD-081-static-reference-initializers-approved-20261030.md — §§ 2-5, 10(1)
func (a *Analyzer) validateStaticReferenceInitializer(name string, reference *ast.RefExpression, token lexer.Token, staticRoots map[string]bool, settled map[ast.Expression]bool) bool {
	settled[reference.Value] = true
	place := reference.Value
	for {
		switch node := place.(type) {
		case *ast.MemberExpression:
			// A type-owned static is reached as `Owner.Name`.
			if path, ok := typePathFromExpression(node); ok && staticRoots[path] {
				markExpressionTree(node, settled)
				return true
			}
			place = node.Object
			settled[place] = true
			continue
		case *ast.IndexExpression:
			if _, known := a.integerConstantValue(node.Index); !known {
				a.addErrorAtToken(expressionToken(node.Index), "static reference initializer for %s needs a compile-time index; a runtime index cannot select static storage", name)
				return a.settleRejectedStaticReference(reference, settled)
			}
			markExpressionTree(node.Index, settled)
			place = node.Left
			settled[place] = true
			continue
		case *ast.Identifier:
			symbol, exists := a.symbols[node.Value]
			if staticRoots[node.Value] && (!exists || !symbol.Local) || exists && symbol.Storage == StorageOriginStatic {
				return true
			}
			if exists {
				a.addErrorAtTokenWithMetadata(node.Token, "",
					"Borrow a static instead, or initialize the static with a value rather than a reference.",
					"static reference initializer for %s borrows %s, which is not static storage; a reference held by a static must have a static root", name, node.Value)
				return a.settleRejectedStaticReference(reference, settled)
			}
			return true
		}
		// A computed value is not addressable; the ordinary borrow check
		// reports it with its stable identity (S1168, MD-081 § 10(3)).
		markExpressionTree(reference.Value, settled)
		return true
	}
}

// settleRejectedStaticReference marks the rejected reference as settled so the
// reported origin error stays its only diagnostic (MD-081 § 10(1)).
func (a *Analyzer) settleRejectedStaticReference(reference *ast.RefExpression, settled map[ast.Expression]bool) bool {
	markExpressionTree(reference.Value, settled)
	settled[reference] = true
	return false
}

// markExpressionTree marks every expression of a subtree as settled.
func markExpressionTree(expression ast.Expression, settled map[ast.Expression]bool) {
	visitStatementExpressions(&ast.LetStatement{Value: expression}, func(candidate ast.Expression) {
		settled[candidate] = true
	})
}

func (a *Analyzer) validateStaticDependencyCycles(declarations map[string]staticDeclaration) {
	dependencies := map[string][]string{}
	for name, declaration := range declarations {
		seen := map[string]bool{}
		visitStatementExpressions(&ast.LetStatement{Value: declaration.value}, func(expression ast.Expression) {
			candidate := ""
			switch expression := expression.(type) {
			case *ast.Identifier:
				candidate = expression.Value
				if declaration.owner != "" {
					if _, exists := declarations[declaration.owner+"."+candidate]; exists {
						candidate = declaration.owner + "." + candidate
					}
				}
			case *ast.MemberExpression:
				if path, ok := typePathFromExpression(expression); ok {
					candidate = path
				}
			}
			if declarations[candidate].name != "" && !seen[candidate] {
				seen[candidate] = true
				dependencies[name] = append(dependencies[name], candidate)
			}
		})
		sort.Strings(dependencies[name])
	}

	state := map[string]uint8{}
	stack := []string{}
	reported := map[string]bool{}
	var visit func(string)
	visit = func(name string) {
		state[name] = 1
		stack = append(stack, name)
		for _, dependency := range dependencies[name] {
			if state[dependency] == 0 {
				visit(dependency)
				continue
			}
			if state[dependency] != 1 {
				continue
			}
			start := 0
			for stack[start] != dependency {
				start++
			}
			cycle := append(append([]string{}, stack[start:]...), dependency)
			key := strings.Join(cycle, " -> ")
			if !reported[key] {
				reported[key] = true
				a.addErrorAtToken(declarations[name].token, "cyclic static initialization: %s", key)
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = 2
	}
	names := make([]string, 0, len(declarations))
	for name := range declarations {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if state[name] == 0 {
			visit(name)
		}
	}
}

// staticHolderBorrows keeps the borrows held by module and type-owned static
// reference holders when a callable body starts: such a holder can be used
// at any time, so its shared or exclusive borrow stays live in every body and
// the ordinary conflict checks reject overlapping direct access.
//
// Rules:
//   - rules/declarations/static.md — § 5 (Amendment MD-081)
//   - rules/memory/lifetime_analysis.md — § 19.2(3)
//   - rules/corrections/applied/MD-081-static-reference-initializers-approved-20261030.md — § 6(3)
func (a *Analyzer) staticHolderBorrows() map[string][]borrowRecord {
	held := map[string][]borrowRecord{}
	for root, records := range a.borrows {
		for _, record := range records {
			holder, exists := a.symbols[record.Holder]
			if exists && !holder.Local && holder.Storage == StorageOriginStatic {
				held[root] = append(held[root], record)
			}
		}
	}
	for _, holder := range a.functionStaticHolders {
		place, ok := a.resolvePlace(holder.reference.Value)
		if !ok {
			continue
		}
		kind := sharedBorrow
		if holder.reference.Mutable {
			kind = mutableBorrow
		}
		for _, alternative := range placeOriginAlternatives(place) {
			held[alternative.Root] = append(held[alternative.Root], borrowRecord{
				Root: alternative.Root, Place: alternative, Holder: holder.name, Kind: kind, Token: holder.reference.Token,
			})
		}
	}
	return held
}

// functionStaticHolder is a function-local `static let` initialized with a
// reference: its borrow persists across invocations, so it is live in every
// callable that can reach the same static storage.
type functionStaticHolder struct {
	name      string
	reference *ast.RefExpression
}

// collectFunctionStaticHolders records every function-local static reference
// holder before any callable body is analyzed, so the conflict check does not
// depend on the order in which bodies are analyzed (MD-081 § 6(3)).
func (a *Analyzer) collectFunctionStaticHolders(program *ast.Program) {
	a.functionStaticHolders = nil
	var bodies []struct {
		owner string
		body  *ast.BlockStatement
	}
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *ast.FunctionDeclaration:
			if statement != nil && statement.Name != nil && statement.Body != nil {
				bodies = append(bodies, struct {
					owner string
					body  *ast.BlockStatement
				}{statement.Name.Value, statement.Body})
			}
		case *ast.ImplStatement:
			if statement == nil || statement.Target == nil {
				continue
			}
			for _, member := range statement.Members {
				if fn, ok := member.(*ast.FunctionDeclaration); ok && fn != nil && fn.Name != nil && fn.Body != nil {
					bodies = append(bodies, struct {
						owner string
						body  *ast.BlockStatement
					}{statement.Target.Name + "." + fn.Name.Value, fn.Body})
				}
			}
		}
	}
	for _, body := range bodies {
		walkASTValue(reflect.ValueOf(body.body), func(node any) {
			let, ok := node.(*ast.LetStatement)
			if !ok || let == nil || !let.Static || let.Name == nil {
				return
			}
			if reference, ok := let.Value.(*ast.RefExpression); ok && reference != nil {
				a.functionStaticHolders = append(a.functionStaticHolders, functionStaticHolder{name: body.owner + "." + let.Name.Value, reference: reference})
			}
		})
	}
}

// dropStaticHolderBorrow removes the borrow a function-local static holder was
// seeded with, before its own initializer registers that borrow again.
func (a *Analyzer) dropStaticHolderBorrow(reference *ast.RefExpression) {
	for root, records := range a.borrows {
		kept := records[:0]
		for _, record := range records {
			if !sameSourceToken(record.Token, reference.Token) {
				kept = append(kept, record)
			}
		}
		if len(kept) == 0 {
			delete(a.borrows, root)
		} else {
			a.borrows[root] = kept
		}
	}
}

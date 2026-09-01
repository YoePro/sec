package sema

import (
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
	for _, name := range names {
		declaration := declarations[name]
		a.validateStaticInitializerExpression(declaration.name, declaration.value, declaration.token)
	}
	a.validateStaticDependencyCycles(declarations)
}

// addStaticDeclaration adds module storage and impl-associated immutable or
// explicit static storage to the shared compile-time dependency graph.
//
// Rules:
//   - rules/declarations/static.md — "Static initialization"
//   - rules/declarations/static.md — "Static initialization dependency order"
func (a *Analyzer) addStaticDeclaration(declarations map[string]staticDeclaration, owner string, declaration *ast.LetStatement, module bool) {
	if declaration == nil || declaration.Name == nil || (!module && !declaration.Static && declaration.Mutable) {
		return
	}
	name := declaration.Name.Value
	if owner != "" {
		name = owner + "." + name
	}
	declarations[name] = staticDeclaration{name: name, owner: owner, token: declaration.Name.Token, value: declaration.Value}
}

// validateStaticInitializerExpression rejects operations that require hidden
// runtime startup execution (static.md, section 15).
func (a *Analyzer) validateStaticInitializerExpression(name string, expression ast.Expression, token lexer.Token) {
	if expression == nil {
		return
	}
	invalid := false
	visitStatementExpressions(&ast.LetStatement{Value: expression}, func(candidate ast.Expression) {
		if invalid {
			return
		}
		switch candidate := candidate.(type) {
		case *ast.RuntimeCallExpression, *ast.NewExpression, *ast.TryExpression,
			*ast.SpawnExpression, *ast.AwaitExpression, *ast.LambdaExpression,
			*ast.RefExpression:
			invalid = true
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
	if invalid {
		a.addErrorAtToken(token, "static initializer for %s must be compile-time evaluable; runtime execution or invocation-local state is not allowed", name)
	}
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

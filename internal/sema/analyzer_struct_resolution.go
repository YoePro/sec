package sema

import "sec/internal/ast"

type pendingTypeDeclaration struct {
	declaration *ast.TypeDeclStatement
	module      string
}

// analyzeNonGenericTypeDependencies resolves ordinary module-level structs and
// their named by-value wrappers in dependency order rather than source order.
// This gives every acyclic forward field reference its complete semantic field
// shape before later declarations and function bodies consume it. Cyclic
// declarations are left finite during construction and are rejected separately
// by validateStructLayoutCycles.
//
// Rules:
//   - rules/declarations/struct.md — §3 "Struct fields", rules 1 and 3
//   - rules/declarations/struct.md — §20 "Semantic analysis requirements", items 1-2
//   - rules/types/types.md — "Type identity", "Named types"
//   - rules/memory/layout.md — §13(5) "Nested structs"
//   - rules/memory/layout.md — §25(1)-(4) "Recursive layout"
func (a *Analyzer) analyzeNonGenericTypeDependencies(program *ast.Program) map[*ast.TypeDeclStatement]bool {
	resolved := map[*ast.TypeDeclStatement]bool{}
	if program == nil {
		return resolved
	}

	declarations := map[string]pendingTypeDeclaration{}
	order := []string{}
	a.withProgramModules(program, func(statement ast.Statement) {
		declaration, ok := statement.(*ast.TypeDeclStatement)
		if !ok || !isNonGenericLayoutDeclaration(declaration) || a.invalidTypeDeclaration(declaration.Name.Token) {
			return
		}
		name := declaration.Name.Value
		declarations[name] = pendingTypeDeclaration{declaration: declaration, module: a.currentModule}
		order = append(order, name)
	})

	state := map[string]uint8{}
	var resolve func(string)
	resolve = func(name string) {
		candidate, exists := declarations[name]
		if !exists || state[name] == 2 {
			return
		}
		if state[name] == 1 {
			// The canonical recursive-layout pass owns the diagnostic. Returning
			// here also prevents construction of an infinitely nested host Type.
			return
		}
		state[name] = 1
		if candidate.declaration.StructType != nil {
			for _, field := range candidate.declaration.StructType.Fields {
				if field == nil {
					continue
				}
				for _, dependency := range typeDeclarationDependencies(field.Type, declarations) {
					resolve(dependency)
				}
			}
		} else {
			for _, dependency := range typeDeclarationDependencies(layoutDeclarationUnderlying(candidate.declaration), declarations) {
				resolve(dependency)
			}
		}

		previousModule := a.currentModule
		a.currentModule = candidate.module
		a.analyzeTypeDeclaration(candidate.declaration)
		a.currentModule = previousModule
		state[name] = 2
		resolved[candidate.declaration] = true
	}

	for _, name := range order {
		resolve(name)
	}
	return resolved
}

// typeDeclarationDependencies returns source-declared struct or named-wrapper
// identities nested anywhere in a field/underlying type. Unlike the by-value
// layout graph, this ordering graph also follows indirection and dynamic-
// storage element types: they break layout recursion but their element/member
// shape must still be available to semantic consumers.
//
// Rules:
//   - rules/declarations/struct.md — §20, items 1-2
//   - rules/types/types.md — "Type identity", "Named types"
//   - rules/memory/layout.md — §13(5)
func typeDeclarationDependencies(reference *ast.TypeReference, declarations map[string]pendingTypeDeclaration) []string {
	if reference == nil || reference.Invalid {
		return nil
	}
	seen := map[string]bool{}
	result := []string{}
	var collect func(*ast.TypeReference)
	collect = func(candidate *ast.TypeReference) {
		if candidate == nil || candidate.Invalid {
			return
		}
		if _, exists := declarations[candidate.Name]; exists && !seen[candidate.Name] {
			seen[candidate.Name] = true
			result = append(result, candidate.Name)
		}
		collect(candidate.ElementType)
		for _, argument := range candidate.TypeArgs {
			collect(argument)
		}
		for _, parameter := range candidate.FunctionParameterTypes {
			collect(parameter)
		}
		collect(candidate.FunctionReturnType)
	}
	collect(reference)
	return result
}

func isNonGenericLayoutDeclaration(declaration *ast.TypeDeclStatement) bool {
	if declaration == nil || declaration.Name == nil || len(declaration.GenericParameters) != 0 {
		return false
	}
	if declaration.StructType != nil {
		return true
	}
	return !declaration.Union && declaration.RegisterType == nil && len(declaration.Variants) == 0 && (declaration.BaseType != nil || declaration.AssignedType != nil)
}

func layoutDeclarationUnderlying(declaration *ast.TypeDeclStatement) *ast.TypeReference {
	if declaration == nil {
		return nil
	}
	if declaration.AssignedType != nil {
		return declaration.AssignedType
	}
	return declaration.BaseType
}

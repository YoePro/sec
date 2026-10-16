package main

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// mutexTypeHoverContents presents compiler-owned opaque synchronization and
// Context declarations with parser-attached source documentation, without
// inventing public storage fields.
// Rules: rules/concurrency/mutex.md — §§4–8,61.
func mutexTypeHoverContents(typ sema.Type, program *ast.Program) (string, bool) {
	if !typ.Intrinsic || (typ.Name != "Mutex" && typ.Name != "MutexGuard" && typ.Name != "Context") {
		return "", false
	}
	contents := "```sec\n"
	if typ.ExplicitlyNonCopyable {
		contents += "@noCopy\n"
	}
	contents += "type " + typ.Name + genericHeaderDisplay(typ.GenericParameters, typ.GenericConstraints) + "\n```"
	for _, attachment := range program.Documentation {
		declaration, ok := attachment.Declaration.(*ast.TypeDeclStatement)
		if ok && declaration != nil && declaration.Name != nil && sameSourceToken(declaration.Name.Token, typ.DeclarationToken) {
			contents += "\n\n" + documentationCommentText(attachment.Comments)
		}
	}
	return contents, true
}

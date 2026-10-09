package sema

import (
	"path/filepath"
	"strings"

	"sec/internal/lexer"
)

// FunctionVisibleFromSource applies the source-file owner boundary to ordinary
// module-scope private functions. Methods retain their type-owner rules and
// compiler-internal functions retain their separately validated authority.
// Anonymous single-source ASTs have the same empty file identity; a missing
// identity never grants access to a function declared in a named source file.
// Rules: rules/foundations/names_scopes_visibility.md — §12.3 Private names;
// rules/library/core-library.md — §1.2 private core helpers.
func FunctionVisibleFromSource(function Function, sourceFile string) bool {
	if function.ImplTarget != "" || !strings.HasPrefix(visibilityBaseName(function.Name), "__") || function.CompilerInternal {
		return true
	}
	if sourceFile == "" || function.Token.File == "" {
		return sourceFile == function.Token.File
	}
	return filepath.Clean(sourceFile) == filepath.Clean(function.Token.File)
}

// accessibleFunctions filters both module visibility and the exact source-file
// owner before direct calls, contextual generic calls or first-class binding.
// Internal function references also retain the registry's core/owner authority;
// an ordinary __ prefix alone never grants access to that registry entry.
// Rules: rules/foundations/names_scopes_visibility.md — §§12.2–12.3, 17;
// rules/compiler/compiler_known_members.md — Internal core string-slice helper.
func (a *Analyzer) accessibleFunctions(functions []Function) []Function {
	out := make([]Function, 0, len(functions))
	for _, function := range functions {
		if function.CompilerInternal {
			known, ok := compilerKnownFunction(function.Name)
			if !ok || known.ID != function.CompilerKnownID || !a.isTrustedCoreSourceToken(lexer.Token{File: a.currentSourceFile}) || (known.OwnerFile != "" && !sourceFileMatchesOwner(a.currentSourceFile, known.OwnerFile)) {
				continue
			}
		}
		if a.canAccessDeclaredName(function.Name, function.Module) && a.FunctionVisibleFromSource(function, a.currentSourceFile) {
			out = append(out, function)
		}
	}
	return out
}

// FunctionVisibleFromSource combines ordinary source privacy with the trusted
// core boundary for core-declared underscore helpers. A matching module spelling
// cannot make an untrusted caller part of core; user module helpers stay ordinary.
// Rules: rules/library/core-library.md — §1.2; names_scopes_visibility.md — §§12.2–12.3;
// rules/compiler/compiler_known_members.md — Private core UTC wall-clock intrinsic.
func (a *Analyzer) FunctionVisibleFromSource(function Function, sourceFile string) bool {
	if strings.HasPrefix(visibilityBaseName(function.Name), "_") && a.isTrustedCoreSourceToken(function.Token) &&
		!a.isTrustedCoreSourceToken(lexer.Token{File: sourceFile}) {
		return false
	}
	return FunctionVisibleFromSource(function, sourceFile)
}

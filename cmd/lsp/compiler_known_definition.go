package main

import (
	"strings"
	"sync"

	"sec/internal/sema"
)

// compilerKnownScheme is the URI scheme of synthetic read-only definitions of
// compiler-known members. Clients show them through a content provider that
// calls sec/compilerKnownDefinition; they never become workspace sources.
const compilerKnownScheme = "sec-compiler-known"

type compilerKnownDefinitionParams struct {
	URI string `json:"uri"`
}

type compilerKnownDefinitionResult struct {
	Text string `json:"text"`
}

// syntheticDefinitionMembers remembers the resolved registry entry per member
// ID, so the content request renders the same receiver-resolved metadata the
// navigation request saw.
var syntheticDefinitionMembers sync.Map

func compilerKnownDefinitionURI(member sema.CompilerKnownMember) string {
	return compilerKnownScheme + ":/" + member.ID + ".sec"
}

// compilerKnownDefinitionLocation navigates a compiler-known member without an
// ordinary core declaration to its synthetic read-only definition, placing
// the cursor on the signature line.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "LSP" (definition or synthetic definition), "Synthetic definitions"
//   - rules/library/core-library.md — 18.3 Synthetic source positions
func compilerKnownDefinitionLocation(member sema.CompilerKnownMember) location {
	syntheticDefinitionMembers.Store(member.ID, member)
	text := sema.CompilerKnownSyntheticDefinition(member)
	line := sema.CompilerKnownSyntheticSignatureLine(text)
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	end := 0
	if line < len(lines) {
		end = utf16TextLength(lines[line])
	}
	return location{
		URI:   compilerKnownDefinitionURI(member),
		Range: lspRange{Start: position{Line: line}, End: position{Line: line, Character: end}},
	}
}

// compilerKnownDefinitionText answers sec/compilerKnownDefinition for a
// synthetic definition URI. A member not yet navigated in this server process
// is rendered from its registry ID alone.
func compilerKnownDefinitionText(uri string) (string, bool) {
	prefix := compilerKnownScheme + ":"
	if !strings.HasPrefix(uri, prefix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimLeft(strings.TrimPrefix(uri, prefix), "/"), ".sec")
	if id == "" {
		return "", false
	}
	if member, ok := syntheticDefinitionMembers.Load(id); ok {
		return sema.CompilerKnownSyntheticDefinition(member.(sema.CompilerKnownMember)), true
	}
	if symbol, ok := sema.CompilerKnownIntrinsicSymbolByID(id); ok {
		// Core-only value definitions require navigation from an authorized
		// Sema use; an ID-only content request supplies no source authority.
		// Rules: compiler_known_members.md — Private core UTC wall-clock intrinsic.
		if symbol.Kind == sema.CompilerKnownValueExpression {
			return "", false
		}
		return sema.CompilerKnownSyntheticDefinition(symbol), true
	}
	return sema.CompilerKnownSyntheticDefinition(sema.CompilerKnownMember{ID: id, Name: id}), true
}

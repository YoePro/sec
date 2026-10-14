package main

import (
	"crypto/sha256"
	"fmt"
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

// syntheticDefinitionMembers remembers the resolved registry entry per URI,
// so the content request renders the same receiver-resolved metadata the
// navigation request saw.
var syntheticDefinitionMembers = struct {
	sync.Mutex
	entries map[string]sema.CompilerKnownMember
	order   []string
}{entries: map[string]sema.CompilerKnownMember{}}

// rememberSyntheticDefinition bounds receiver-specific presentation state.
// An evicted instance requires renewed navigation; it must not fall back to
// another receiver's text. All access is serialized for concurrent requests.
// Rules: rules/tooling/lsp.md — Snapshots;
// rules/compiler/compiler_known_members.md — "Synthetic definitions".
func rememberSyntheticDefinition(uri string, member sema.CompilerKnownMember) {
	syntheticDefinitionMembers.Lock()
	defer syntheticDefinitionMembers.Unlock()
	if _, exists := syntheticDefinitionMembers.entries[uri]; !exists {
		const limit = 512
		if len(syntheticDefinitionMembers.order) == limit {
			delete(syntheticDefinitionMembers.entries, syntheticDefinitionMembers.order[0])
			syntheticDefinitionMembers.order = syntheticDefinitionMembers.order[1:]
		}
		syntheticDefinitionMembers.order = append(syntheticDefinitionMembers.order, uri)
	}
	syntheticDefinitionMembers.entries[uri] = member
}

// compilerKnownDefinitionURI keeps receiver instances separate while leaving
// the stable operation ID visible in the path. The instance digest contains
// no workspace paths and does not change compiler semantic identity.
// Rules: rules/compiler/compiler_known_members.md — "Synthetic definitions",
// "Stable member identity".
func compilerKnownDefinitionURI(member sema.CompilerKnownMember) string {
	uri := compilerKnownScheme + ":/" + member.ID + ".sec"
	if member.BoundReceiver != "" {
		instance := sha256.Sum256([]byte(member.BoundReceiver + "\x00" + member.Signature))
		uri += fmt.Sprintf("?instance=%x", instance)
	}
	return uri
}

// compilerKnownDefinitionLocation navigates a compiler-known member without an
// ordinary core declaration to its synthetic read-only definition, placing
// the cursor on the signature line.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "LSP" (definition or synthetic definition), "Synthetic definitions"
//   - rules/library/core-library.md — 18.3 Synthetic source positions
func compilerKnownDefinitionLocation(member sema.CompilerKnownMember) location {
	rememberSyntheticDefinition(compilerKnownDefinitionURI(member), member)
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
	syntheticDefinitionMembers.Lock()
	member, cached := syntheticDefinitionMembers.entries[uri]
	syntheticDefinitionMembers.Unlock()
	if cached {
		return sema.CompilerKnownSyntheticDefinition(member), true
	}
	// Unknown instance URIs have no receiver fact; never return an unrelated
	// cached instance or pretend that generic text is its resolved definition.
	if strings.Contains(uri, "?") {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimLeft(strings.TrimPrefix(uri, prefix), "/"), ".sec")
	if id == "" {
		return "", false
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

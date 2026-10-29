package main

import (
	"crypto/sha256"
	"fmt"
	"net/url"
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

// compilerKnownDefinitionURI gives the editor a friendly filename while stable
// registry identity remains a transport query. Receiver instances retain their
// digest; neither filenames nor digests contain workspace paths.
// Rules: rules/compiler/compiler_known_members.md — "Synthetic definitions",
// "User-facing symbol names", "Stable member identity".
func compilerKnownDefinitionURI(member sema.CompilerKnownMember) string {
	query := url.Values{"id": {member.ID}}
	if member.BoundReceiver != "" {
		instance := sha256.Sum256([]byte(member.BoundReceiver + "\x00" + member.Signature + "\x00" + member.Documentation))
		query.Set("instance", fmt.Sprintf("%x", instance))
	}
	// A virtual document is not a disk file, but keep its public label in a
	// single basename even when a receiver contains module path separators.
	label := strings.NewReplacer("/", "∕", "\\", "∖").Replace(member.DisplayFacts().Label)
	return (&url.URL{Scheme: compilerKnownScheme, Path: "/" + label + ".sec", RawQuery: query.Encode()}).String()
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
// Rules: rules/compiler/compiler_known_members.md — "Synthetic definitions",
// "Private core UTC wall-clock intrinsic", "User-facing symbol names".
func compilerKnownDefinitionText(uri string) (string, bool) {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != compilerKnownScheme || parsed.Host != "" || parsed.Opaque != "" || parsed.Fragment != "" {
		return "", false
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil || len(query["id"]) > 1 || len(query["instance"]) > 1 {
		return "", false
	}
	for key := range query {
		if key != "id" && key != "instance" {
			return "", false
		}
	}
	// Canonicalize URI escaping/query order after editor transport. Cache keys
	// still include the resolved label, operation identity and instance digest.
	canonical := (&url.URL{Scheme: compilerKnownScheme, Path: parsed.Path, RawQuery: query.Encode()}).String()
	syntheticDefinitionMembers.Lock()
	member, cached := syntheticDefinitionMembers.entries[canonical]
	syntheticDefinitionMembers.Unlock()
	if cached {
		return sema.CompilerKnownSyntheticDefinition(member), true
	}
	// Missing instances never fall back to a generic or different receiver.
	if _, instance := query["instance"]; instance {
		return "", false
	}
	id := query.Get("id")
	if id == "" {
		// Previously issued ID-only URIs remain readable through the registry;
		// new navigation never emits these implementation-named basenames.
		if parsed.RawQuery != "" || !strings.HasPrefix(parsed.Path, "/") || !strings.HasSuffix(parsed.Path, ".sec") {
			return "", false
		}
		id = strings.TrimSuffix(strings.TrimPrefix(parsed.Path, "/"), ".sec")
		if strings.Contains(id, "/") {
			return "", false
		}
	}
	if id == "" {
		return "", false
	}
	// Internal function definitions also require an authorized Sema navigation;
	// knowing their transport ID does not supply privileged-core authority.
	for _, function := range sema.CompilerKnownFunctions() {
		if function.ID == id && function.Internal {
			return "", false
		}
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

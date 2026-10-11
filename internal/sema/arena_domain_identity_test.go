package sema

import (
	"strings"
	"testing"
)

// An Arena reached through a local reference operates on the referenced
// Arena's stable domain, not on the alias's binding name: a Reset through the
// alias makes storage allocated from the owner stale, and storage allocated
// through the alias stays local to the owner's invocation (arena.md §§4.2,
// 44).
func TestArenaAliasUsesReferencedDomain(t *testing.T) {
	for name, test := range map[string]struct {
		body, want string
	}{
		"reset through alias": {`    let mut mem: Arena := Arena {}
    let values := try mem.Alloc[byte](16u)
    let alias := ref mut mem
    alias.Reset()
    discard values
    return Ok()`, "cannot use values after arena mem was reset"},
		"allocate and reset through alias": {`    let mut mem: Arena := Arena {}
    let first := try mem.Alloc[byte](16u)
    discard first
    let alias := ref mut mem
    let values := try alias.Alloc[byte](16u)
    alias.Reset()
    discard values
    return Ok()`, "cannot use values after arena mem was reset"},
	} {
		source := "module main\n\nfn Use() Result[void, AllocationError] {\n" + test.body + "\n}\n"
		errors := analyzeSourceRaw(t, source)
		if len(errors) != 1 || !strings.Contains(errors[0].Message, test.want) {
			t.Errorf("%s: errors = %v, want %q", name, errors, test.want)
		}
	}
	escaping := "module main\n\nfn Leak() Result[ref mut byte[], AllocationError] {\n    let mut mem: Arena := Arena {}\n    let alias := ref mut mem\n    let values := try alias.Alloc[byte](16u)\n    return Ok(values)\n}\n"
	if errors := analyzeSourceRaw(t, escaping); len(errors) == 0 {
		t.Error("storage allocated through an alias of a local Arena escaped the invocation")
	}
}

// Arena effect sites carry the stable compiler-owned domain they select or
// create; the receiver spelling is display-only, an alias selects its
// owner's domain, and a parameter receives a symbolic domain rather than its
// name.
func TestArenaEffectSitesCarryStableDomains(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

fn Fill(arena: ref mut Arena) Result[void, AllocationError] {
    let values := try arena.Alloc[byte](4u)
    discard values
    arena.Reset()
    return Ok()
}

fn Owner() Result[void, AllocationError] {
    let mut mem: Arena := Arena {}
    let first := try mem.Alloc[byte](4u)
    discard first
    let alias := ref mut mem
    let second := try alias.Alloc[byte](4u)
    discard second
    return Ok()
}
`)
	assertSemaErrors(t, errors, nil)
	domains := map[string][]string{}
	for _, node := range analyzer.callGraph.Nodes() {
		for _, site := range analyzer.callGraph.arenaEffects[node.ID] {
			if site.Domain == "" || site.Domain == site.Arena || !strings.HasPrefix(site.Domain, "$arena-domain") {
				t.Fatalf("%s site %s has no stable domain: %#v", node.Name, site.Kind, site)
			}
			domains[node.Name] = append(domains[node.Name], site.Domain)
		}
	}
	if fill := domains["Fill"]; len(fill) != 2 || fill[0] != fill[1] {
		t.Fatalf("Fill parameter domains = %v, want one symbolic domain for both operations", fill)
	}
	if owner := domains["Owner"]; len(owner) != 2 || owner[0] != owner[1] {
		t.Fatalf("Owner domains = %v, want the alias to select the owner's domain", owner)
	}
}

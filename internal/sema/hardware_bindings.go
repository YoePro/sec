package sema

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/hardware"
	"sec/internal/lexer"
)

// ResolvedHardwareBinding is the immutable fact of one `@address` register
// binding resolved to a canonical endpoint: the binding's own token and
// mutability, the nominal register type T of its HardwareInstance[T], the
// endpoint fact with its region/resource/domain/address/extent, and whether
// the source acknowledged an alias. It is a resource/authority root, never an
// ordinary memory Place (MD-077 §§ 3(4), 12(2)).
// Rules: rules/corrections/applied/md077-register-semantic-ir-final-decision-20261022.md — §§ 3(4), 12, 19(5);
// rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§ 2, 10(1).
type ResolvedHardwareBinding struct {
	Name     string
	Token    lexer.Token
	Mutable  bool
	Alias    bool
	Register Type
	Endpoint hardware.Endpoint
}

// recordHardwareBinding retains the resolved binding once per declaration.
func (a *Analyzer) recordHardwareBinding(stmt *ast.LetStatement, register Type, endpoint hardware.Endpoint) {
	if a.summaryPass || stmt == nil || stmt.Name == nil {
		return
	}
	if a.hardwareBindings == nil {
		a.hardwareBindings = map[sourceTokenKey]ResolvedHardwareBinding{}
	}
	a.hardwareBindings[sourceTokenLocation(stmt.Name.Token)] = ResolvedHardwareBinding{Name: stmt.Name.Value, Token: stmt.Name.Token, Mutable: stmt.Mutable, Alias: stmt.AddressAlias, Register: semanticSnapshotType(register), Endpoint: (hardware.Facts{Endpoints: []hardware.Endpoint{endpoint}}).Clone().Endpoints[0]}
}

// ResolvedHardwareBindings returns the resolved hardware register bindings in
// source order. Bindings whose endpoint did not resolve are absent; their
// diagnostics already stopped compilation.
func (a *Analyzer) ResolvedHardwareBindings() []ResolvedHardwareBinding {
	if a == nil {
		return nil
	}
	result := make([]ResolvedHardwareBinding, 0, len(a.hardwareBindings))
	for _, binding := range a.hardwareBindings {
		binding.Endpoint = (hardware.Facts{Endpoints: []hardware.Endpoint{binding.Endpoint}}).Clone().Endpoints[0]
		result = append(result, binding)
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i].Token, result[j].Token
		if left.File != right.File {
			return left.File < right.File
		}
		return left.ByteStart < right.ByteStart
	})
	return result
}

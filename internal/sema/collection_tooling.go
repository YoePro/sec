package sema

import (
	"fmt"
	"sort"
	"strings"
)

// CollectionMemberFact is a detached presentation of an existing registry
// entry. StructuralMutation describes the operation, not a live borrow state.
type CollectionMemberFact struct {
	ID, Name, Result           string
	Kind                       CompilerKnownMemberKind
	Static, StructuralMutation bool
}

// CollectionFacts exposes logical collection semantics without guessing runtime
// length, physical layout, allocation context or backing storage identity.
type CollectionFacts struct {
	Family, ElementType, KeyType, ValueType string
	CountUnit, IterationOrder               string
	Dynamic                                 bool
	MaximumCapacity                         string
	Members                                 []CollectionMemberFact
}

// CollectionFactsOf publishes collection type facts and the canonical member
// registry. References use the same receiver normalization as member resolution;
// unrelated types and invalid collection shapes publish no facts.
// Rules: rules/collections/collections.md — §§13.1–13.4,14.1–14.3,15.1–15.2;
// rules/compiler/compiler_known_members.md — "Collection capacity members";
// rules/tooling/lsp.md — "One semantic source of truth", "Expose all compiler knowledge".
func CollectionFactsOf(typ Type) (CollectionFacts, bool) {
	typ = dereferenceType(typ)
	if typ.Kind == InvalidType {
		return CollectionFacts{}, false
	}
	f := CollectionFacts{Family: typ.Name, Dynamic: dynamicCollection(typ), IterationOrder: "unspecified"}
	switch {
	case (typ.Name == "list" || typ.Name == "set") && len(typ.TypeArgs) == 1:
		f.ElementType = typeDisplayName(typ.TypeArgs[0])
		f.CountUnit = "distinct elements"
		if typ.Name == "list" {
			f.CountUnit, f.IterationOrder = "elements", "index order"
		}
	case typ.Name == "map" && len(typ.TypeArgs) == 2:
		f.KeyType, f.ValueType = typeDisplayName(typ.TypeArgs[0]), typeDisplayName(typ.TypeArgs[1])
		f.CountUnit = "entries"
	default:
		return CollectionFacts{}, false
	}
	if len(typ.ConstArgs) > 1 || len(typ.ConstArgs) == 1 && typ.ConstArgs[0] <= 0 {
		return CollectionFacts{}, false
	}
	if !f.Dynamic {
		f.MaximumCapacity = fmt.Sprint(typ.ConstArgs[0])
	}
	for _, static := range []bool{false, true} {
		for _, member := range CompilerKnownMembersForType(typ, static) {
			f.Members = append(f.Members, CollectionMemberFact{ID: member.ID, Name: member.Name,
				Kind: member.Kind, Result: typeDisplayName(member.Result), Static: static,
				StructuralMutation: member.StructuralMutation})
		}
	}
	sort.Slice(f.Members, func(i, j int) bool {
		if f.Members[i].Static != f.Members[j].Static {
			return !f.Members[i].Static
		}
		return f.Members[i].ID < f.Members[j].ID
	})
	return f, true
}

// Display provides a shared tooling description. Capacity counts logical values,
// never bytes/buckets; bounded capacity is a maximum, not the current Len.
// Rules: rules/collections/collections.md — §§13.2,13.4,14.3,15.2;
// rules/corrections/applied/md058-collection-capacity-reservation-correction-20261012.md — §7.
func (f CollectionFacts) Display() string {
	types := "element " + f.ElementType
	if f.Family == "map" {
		types = "key " + f.KeyType + "; value " + f.ValueType
	}
	capacity := "dynamic; Capacity is known at runtime"
	if !f.Dynamic {
		capacity = "bounded; maximum Capacity " + f.MaximumCapacity
	}
	lines := []string{fmt.Sprintf("Collection %s: %s", f.Family, types),
		fmt.Sprintf("%s (%s); Len is runtime state", capacity, f.CountUnit),
		"Iteration: " + f.IterationOrder}
	for _, member := range f.Members {
		if member.Kind != CompilerKnownMethod && member.Kind != CompilerKnownAssociatedFunction {
			continue
		}
		prefix, suffix := "", ""
		if member.Static {
			prefix = "static "
		}
		if member.StructuralMutation {
			suffix = " (structural mutation)"
		}
		lines = append(lines, fmt.Sprintf("%s%s: %s%s", prefix, member.Name, member.Result, suffix))
	}
	return strings.Join(lines, "\n")
}

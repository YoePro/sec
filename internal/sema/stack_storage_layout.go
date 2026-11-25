package sema

import (
	"fmt"

	"sec/internal/layout"
)

// StackStorageLayout is detached layout evidence for one materialized object,
// qualified by the canonical CompilationPlan layout key. Bytes describes only
// this object's representation, not its referents, backing allocation, frame
// padding, lifetime or total callable frame. Representation and its reachable
// Layouts retain aggregate fields, array elements and carrier identities.
// Rules: rules/analysis/stack_analysis.md §§8–9,41,44,48–56.
type StackStorageLayout struct {
	PlanKey        string
	Representation layout.ResolvedLayout
	Layouts        []layout.ResolvedLayout
	Bytes          StackBound
}

// ResolveStackStorageLayout consumes the canonical resolver rather than
// estimating scalar, aggregate, array, union, pointer or descriptor sizes.
// Incomplete/invalid/unsupported representations retain the resolver's state
// and missing evidence and yield Unknown bytes, never zero or a host fallback.
// The returned graph contains only the object's reachable representations;
// LayoutIDs are resolver-local and must always be interpreted with that graph.
// Callers must establish materialization and live ranges separately before
// these object facts can contribute to a semantic own-frame summary. They are
// never machine-frame or physical-domain budget proofs.
// Rules: rules/analysis/stack_analysis.md §§5,8–9,36–41,44,48–58;
// rules/memory/layout.md §§3,5,7–9,18,25,39.
func ResolveStackStorageLayout(resolver *layout.Resolver, typ Type) (StackStorageLayout, error) {
	if resolver == nil {
		return StackStorageLayout{}, fmt.Errorf("stack storage layout requires a canonical layout resolver")
	}
	root := resolver.Resolve(LayoutRequirementOf(typ))
	result := StackStorageLayout{PlanKey: resolver.Key().String(), Representation: root}
	// Resolve publishes detached copies; retain only the reachable graph so an
	// unrelated resolution cannot change the storage evidence for this object.
	seen := map[layout.LayoutID]bool{}
	var visit func(layout.LayoutID) error
	visit = func(id layout.LayoutID) error {
		if id == 0 || seen[id] {
			return nil
		}
		seen[id] = true
		item, ok := resolver.Layout(id)
		if !ok {
			return fmt.Errorf("stack storage layout references missing layout %d", id)
		}
		result.Layouts = append(result.Layouts, item)
		children := []layout.LayoutID{item.Carrier, item.Element}
		for _, field := range item.Fields {
			children = append(children, field.Layout)
		}
		for _, variant := range item.Variants {
			children = append(children, variant.Payload)
		}
		for _, child := range children {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(root.ID); err != nil {
		return StackStorageLayout{}, err
	}
	if root.Complete() {
		if root.SizeBytes == nil || root.AlignBytes == nil || root.AlignBytes.Sign() <= 0 {
			return StackStorageLayout{}, fmt.Errorf("complete stack storage layout lacks size or alignment")
		}
		bound, err := NewExactStackBound(root.SizeBytes)
		if err != nil {
			return StackStorageLayout{}, err
		}
		result.Bytes = bound
	}
	return result, nil
}

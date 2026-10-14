package sema

import (
	"fmt"
	"strings"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// layoutCarrierWitness records the stored field or variant through which a
// generic declaration embeds a parameter by value. It is a source proof edge,
// not specialization demand, a runtime instance, or a physical ABI identity.
// Rules: rules/memory/layout.md — §25(1–5); rules/declarations/generics.md — §24.
type layoutCarrierWitness struct {
	label string
	token lexer.Token
}

// layoutParameterWitnesses retains a finite shortest witness for every
// by-value parameter fact. Fixed-point construction follows nested generic
// carriers without expanding concrete instances or conflating layout cycles
// with non-converging specialization. Stable ties use source identities.
// Rules: rules/memory/layout.md — §25(3), §42(4);
// rules/compiler/monomorphization.md — §18.4–18.6.
func layoutParameterWitnesses(nodes map[string]*structLayoutNode, order []string, byValue map[string]map[int]bool) map[string]map[int][]layoutCarrierWitness {
	paths := map[string]map[int][]layoutCarrierWitness{}
	for _, name := range order {
		paths[name] = map[int][]layoutCarrierWitness{}
	}
	changed := true
	for changed {
		changed = false
		for _, name := range order {
			node := nodes[name]
			for _, member := range layoutMembers(node) {
				for _, target := range layoutTargets(member.reference, node.parameters, nodes, byValue, "", paths) {
					if target.name != "" || target.incompleteCarrier {
						continue
					}
					label := layoutNodeDisplay(node)
					if member.label != "" {
						label += "." + member.label
					}
					candidate := append([]layoutCarrierWitness{{label: label, token: member.token}}, target.carriers...)
					current, exists := paths[name][target.parameter]
					if !exists || len(candidate) < len(current) || len(candidate) == len(current) && layoutCarrierPathKey(candidate) < layoutCarrierPathKey(current) {
						paths[name][target.parameter] = candidate
						changed = true
					}
				}
			}
		}
	}
	return paths
}

// layoutCarrierPathKey provides deterministic witness ties by canonical
// declaration labels and source sites, never map or request iteration order.
// Rules: rules/tooling/diagnostics.md — §28 Deterministic ordering.
func layoutCarrierPathKey(path []layoutCarrierWitness) string {
	var key strings.Builder
	for _, edge := range path {
		key.WriteString(edge.label)
		key.WriteByte(0)
		key.WriteString(fmt.Sprintf("%s:%d:%d", edge.token.File, edge.token.Line, edge.token.Column))
		key.WriteByte(0)
	}
	return key.String()
}

// layoutNodeDisplay names a source layout template with its declared ordered
// parameters. It is diagnostic provenance, not an InstantiationKey.
// Rules: rules/memory/layout.md — §25(3); rules/declarations/generics.md — §24.
func layoutNodeDisplay(node *structLayoutNode) string {
	if node == nil {
		return ""
	}
	parameters := genericParameterNameValues(node.declaration.GenericParameters)
	if len(parameters) == 0 {
		return node.name
	}
	return node.name + "[" + strings.Join(parameters, ", ") + "]"
}

// reportStructLayoutCycle emits the stable mandatory diagnostic with the
// source field, variant, and nested generic-carrier path that closes the
// infinite by-value layout. It does not claim a canonical demand-key cycle.
//
// Rules:
//   - rules/declarations/struct.md — §21 "Diagnostics"
//   - rules/declarations/unions.md — §26 "Required diagnostics"
//   - rules/memory/layout.md — §25(3) and §42(4)
//   - rules/tooling/diagnostics.md — §5, §8, and §12
func (a *Analyzer) reportStructLayoutCycle(id string, cycle []structLayoutEdge) {
	if len(cycle) == 0 {
		return
	}
	parts := make([]string, 0, len(cycle)+1)
	var related []RelatedLocation
	primary := cycle[len(cycle)-1].token
	for _, edge := range cycle {
		part := edge.owner
		if edge.ownerDisplay != "" {
			part = edge.ownerDisplay
		}
		if edge.field != "" {
			part += "." + edge.field
		}
		if edge.via != "" {
			part += " (via " + edge.via + ")"
		}
		if (edge.ownerDisplay != edge.owner || edge.targetDisplay != edge.target) && edge.storageType != "" {
			part += " (stores " + edge.storageType + ")"
		}
		parts = append(parts, part)
		for _, witness := range edge.carriers {
			endLine, endColumn := witness.token.EndPosition()
			related = append(related, RelatedLocation{File: witness.token.File, Line: witness.token.Line, Column: witness.token.Column, EndLine: endLine, EndColumn: endColumn, Label: "by-value generic carrier " + witness.label})
		}
		declaration := edge.ownerDeclaration
		endLine, endColumn := declaration.EndPosition()
		related = append(related, RelatedLocation{File: declaration.File, Line: declaration.Line, Column: declaration.Column, EndLine: endLine, EndColumn: endColumn, Label: "layout declaration " + edge.ownerDisplay})
		if edge.token != primary {
			endLine, endColumn = edge.token.EndPosition()
			related = append(related, RelatedLocation{File: edge.token.File, Line: edge.token.Line, Column: edge.token.Column, EndLine: endLine, EndColumn: endColumn, Label: "by-value dependency " + part})
		}
	}
	target := cycle[len(cycle)-1].targetDisplay
	if target == "" {
		target = cycle[len(cycle)-1].target
	}
	parts = append(parts, target)
	kind := "struct"
	if id == diagnostics.RecursiveUnionLayout {
		kind = "union"
	}
	endLine, endColumn := primary.EndPosition()
	a.appendError(Error{ID: id, Severity: diagnostics.SeverityError, ProofState: diagnostics.ProofInvalid,
		Help:    "Break the cycle with a reference, raw pointer, slice, or owning dynamic-storage boundary. This is a by-value layout cycle, not a proof of non-converging specialization demand.",
		Message: fmt.Sprintf("recursive by-value %s layout has infinite size: %s", kind, strings.Join(parts, " -> ")),
		File:    primary.File, Line: primary.Line, Column: primary.Column, EndLine: endLine, EndColumn: endColumn, Related: related,
	})
}

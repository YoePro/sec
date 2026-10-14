package sema

import (
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

type structLayoutEdge struct {
	owner            string
	field            string
	via              string
	target           string
	token            lexer.Token
	ownerDeclaration lexer.Token
	ownerDisplay     string
	targetDisplay    string
	storageType      string
	carriers         []layoutCarrierWitness
}

type layoutDeclarationKind uint8

const (
	layoutStructDeclaration layoutDeclarationKind = iota
	layoutUnionDeclaration
	layoutWrapperDeclaration
)

type structLayoutNode struct {
	name        string
	token       lexer.Token
	kind        layoutDeclarationKind
	declaration *ast.TypeDeclStatement
	parameters  map[string]int
	edges       []structLayoutEdge
}

// layoutMember is one by-value storage position of a declaration: a struct
// field, a union variant payload or payload field, or a named wrapper's
// underlying type.
type layoutMember struct {
	label     string
	reference *ast.TypeReference
	token     lexer.Token
}

// layoutTarget is one by-value containment reached from a type reference:
// either a source-declared type or a generic parameter of the owning
// declaration. via names the instantiation that carried the containment.
type layoutTarget struct {
	name              string
	parameter         int
	via               string
	carriers          []layoutCarrierWitness
	incompleteCarrier bool
}

// validateStructLayoutCycles builds the module-level by-value layout
// dependency graph and rejects every cycle that has no indirection or storage
// boundary. The graph covers structs, tagged unions, named by-value wrappers,
// fixed arrays, compiler-known tagged Option/Result payloads, and generic
// declarations; generic instantiation contributes the by-value containment
// of each type argument whose parameter the generic stores by value, so a
// cycle introduced only by instantiation is detected as well. The graph is
// built from declarations rather than partially resolved Type values so
// forward and mutually recursive declarations receive the same deterministic
// validation.
// References, slices, dynamic arrays, owning collections, and raw pointers are
// representation boundaries and therefore do not contribute by-value edges.
//
// Rules:
//   - rules/declarations/struct.md — §7 "Struct defaults", paragraphs 6-8
//   - rules/declarations/struct.md — §20 "Semantic analysis requirements"
//   - rules/declarations/unions.md — §18 "Recursive unions"; §26 "Required diagnostics"
//   - rules/types/types.md — "Type identity", "Named types"
//   - rules/memory/layout.md — §13(6)-(7), §15(4), §15(7), §16(1)
//   - rules/memory/layout.md — §25(1)-(5) "Recursive layout"
//   - rules/memory/layout.md — §42(4) "Recursive-layout diagnostics"
func (a *Analyzer) validateStructLayoutCycles(program *ast.Program) {
	if program == nil {
		return
	}

	nodes := map[string]*structLayoutNode{}
	order := []string{}
	a.withProgramModules(program, func(statement ast.Statement) {
		declaration, ok := statement.(*ast.TypeDeclStatement)
		if !ok || declaration.Name == nil || a.invalidTypeDeclaration(declaration.Name.Token) {
			return
		}
		kind, ok := layoutDeclarationKindOf(declaration)
		if !ok {
			return
		}
		name := declaration.Name.Value
		if nodes[name] != nil {
			return
		}
		parameters := map[string]int{}
		for index, parameter := range genericParameterNameValues(declaration.GenericParameters) {
			parameters[parameter] = index
		}
		nodes[name] = &structLayoutNode{name: name, token: declaration.Name.Token, kind: kind, declaration: declaration, parameters: parameters}
		order = append(order, name)
	})

	byValueParameters := layoutByValueParameters(nodes, order)
	parameterPaths := layoutParameterWitnesses(nodes, order, byValueParameters)
	for _, name := range order {
		node := nodes[name]
		for _, member := range layoutMembers(node) {
			for _, target := range layoutTargets(member.reference, node.parameters, nodes, byValueParameters, "", parameterPaths) {
				if target.name == "" {
					continue
				}
				node.edges = append(node.edges, structLayoutEdge{
					owner: node.name, field: member.label, via: target.via, target: target.name, token: member.token,
					ownerDeclaration: node.token,
					ownerDisplay:     layoutNodeDisplay(node), targetDisplay: layoutNodeDisplay(nodes[target.name]),
					storageType: typeReferenceDisplayName(member.reference),
					carriers:    target.carriers,
				})
			}
		}
	}

	state := map[string]uint8{}
	stack := []string{}
	path := []structLayoutEdge{}
	reported := map[string]bool{}
	var visit func(string)
	visit = func(name string) {
		state[name] = 1
		stack = append(stack, name)
		for _, edge := range nodes[name].edges {
			switch state[edge.target] {
			case 0:
				path = append(path, edge)
				visit(edge.target)
				path = path[:len(path)-1]
			case 1:
				start := structLayoutStackIndex(stack, edge.target)
				if start < 0 {
					continue
				}
				members := append([]string(nil), stack[start:]...)
				sort.Strings(members)
				key := strings.Join(members, "\x00")
				if reported[key] {
					continue
				}
				cycle := append([]structLayoutEdge(nil), path[start:]...)
				cycle = append(cycle, edge)
				id, ok := layoutCycleDiagnostic(stack[start:], nodes)
				if !ok {
					continue
				}
				reported[key] = true
				if a.layoutCycleAlreadyDiagnosed(cycle) {
					continue
				}
				a.reportStructLayoutCycle(id, cycle)
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = 2
	}

	for _, name := range order {
		if state[name] == 0 {
			visit(name)
		}
	}
}

func layoutDeclarationKindOf(declaration *ast.TypeDeclStatement) (layoutDeclarationKind, bool) {
	switch {
	case declaration.StructType != nil:
		return layoutStructDeclaration, true
	case declaration.Union:
		return layoutUnionDeclaration, true
	case declaration.RegisterType == nil && len(declaration.Variants) == 0 && layoutDeclarationUnderlying(declaration) != nil:
		return layoutWrapperDeclaration, true
	}
	return 0, false
}

// layoutMembers lists every by-value storage position of one declaration.
// Every union payload is a storage position because tagged-union payload
// storage must accommodate each variant.
// Rule: rules/memory/layout.md — §15(4).
func layoutMembers(node *structLayoutNode) []layoutMember {
	declaration := node.declaration
	members := []layoutMember{}
	switch node.kind {
	case layoutStructDeclaration:
		for _, field := range declaration.StructType.Fields {
			if field != nil && field.Name != nil {
				members = append(members, layoutMember{label: field.Name.Value, reference: field.Type, token: field.Name.Token})
			}
		}
	case layoutUnionDeclaration:
		for _, variant := range declaration.UnionVariants {
			if variant == nil || variant.Name == nil {
				continue
			}
			if variant.Payload != nil {
				members = append(members, layoutMember{label: variant.Name.Value, reference: variant.Payload, token: variant.Name.Token})
			}
			for _, field := range variant.PayloadFields {
				if field != nil && field.Name != nil {
					members = append(members, layoutMember{label: variant.Name.Value + "." + field.Name.Value, reference: field.Type, token: field.Name.Token})
				}
			}
		}
	case layoutWrapperDeclaration:
		reference := layoutDeclarationUnderlying(declaration)
		members = append(members, layoutMember{reference: reference, token: reference.Token})
	}
	return members
}

// layoutByValueParameters computes, as a deterministic monotone fixed point,
// which generic parameters each generic declaration stores by value, directly
// or through another generic instantiation.
// Rule: rules/memory/layout.md — §25(5) generic instantiation may introduce a cycle.
func layoutByValueParameters(nodes map[string]*structLayoutNode, order []string) map[string]map[int]bool {
	result := map[string]map[int]bool{}
	for _, name := range order {
		if len(nodes[name].parameters) > 0 {
			result[name] = map[int]bool{}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, name := range order {
			node := nodes[name]
			if len(node.parameters) == 0 {
				continue
			}
			for _, member := range layoutMembers(node) {
				for _, target := range layoutTargets(member.reference, node.parameters, nodes, result, "", nil) {
					if target.name == "" && !result[name][target.parameter] {
						result[name][target.parameter] = true
						changed = true
					}
				}
			}
		}
	}
	return result
}

// layoutTargets returns the by-value containments of one type reference.
// Fixed arrays preserve containment; Option and Result embed their payloads
// as tagged variants; a source-declared generic contributes itself plus the
// type arguments it stores by value. Every explicit indirection, slice,
// owning collection, raw pointer, function value, and type whose layout is
// not source-declared terminates the dependency.
//
// Rules:
//   - rules/memory/layout.md — §13(7), §16(1), §25(1), §25(2), §25(4), and §25(5)
func layoutTargets(reference *ast.TypeReference, parameters map[string]int, nodes map[string]*structLayoutNode, byValueParameters map[string]map[int]bool, via string, parameterPaths map[string]map[int][]layoutCarrierWitness) []layoutTarget {
	if reference == nil || reference.Invalid || reference.Ref {
		return nil
	}
	if reference.ElementType != nil {
		if reference.Slice {
			return nil
		}
		return layoutTargets(reference.ElementType, parameters, nodes, byValueParameters, via, parameterPaths)
	}
	if reference.Name == "" || len(reference.ConstArgs) != 0 {
		return nil
	}
	if index, ok := parameters[reference.Name]; ok && len(reference.TypeArgs) == 0 {
		return []layoutTarget{{parameter: index, via: via}}
	}
	node := nodes[reference.Name]
	if len(reference.TypeArgs) == 0 {
		if node == nil {
			return nil
		}
		return []layoutTarget{{name: reference.Name, via: via}}
	}
	carrier := via
	if carrier == "" {
		carrier = typeReferenceDisplayName(reference)
	}
	targets := []layoutTarget{}
	switch {
	case node != nil:
		targets = append(targets, layoutTarget{name: reference.Name, via: via})
		for index, argument := range reference.TypeArgs {
			if byValueParameters[reference.Name][index] {
				nested := layoutTargets(argument, parameters, nodes, byValueParameters, carrier, parameterPaths)
				path, complete := parameterPaths[reference.Name][index]
				for i := range nested {
					nested[i].carriers = append(append([]layoutCarrierWitness(nil), path...), nested[i].carriers...)
					nested[i].incompleteCarrier = nested[i].incompleteCarrier || parameterPaths != nil && !complete
				}
				targets = append(targets, nested...)
			}
		}
	case reference.Name == "Option" || reference.Name == "Result":
		for _, argument := range reference.TypeArgs {
			targets = append(targets, layoutTargets(argument, parameters, nodes, byValueParameters, carrier, parameterPaths)...)
		}
	}
	return targets
}

// layoutCycleDiagnostic selects the owning diagnostic. A cycle through a
// struct is a struct layout error; a cycle through unions only is a union
// storage error. Wrapper-only cycles belong to named-type resolution.
func layoutCycleDiagnostic(names []string, nodes map[string]*structLayoutNode) (string, bool) {
	union := false
	for _, name := range names {
		switch nodes[name].kind {
		case layoutStructDeclaration:
			return diagnostics.RecursiveStructLayout, true
		case layoutUnionDeclaration:
			union = true
		}
	}
	return diagnostics.RecursiveUnionLayout, union
}

// layoutCycleAlreadyDiagnosed keeps one diagnostic per root cause: the
// an earlier owning recursive-layout check may already have rejected a
// storage position of this cycle. Generic template containment guards defer
// their errors to this graph instead of supplying a competing classification.
// Rule: rules/tooling/diagnostics.md — one root cause, one diagnostic.
func (a *Analyzer) layoutCycleAlreadyDiagnosed(cycle []structLayoutEdge) bool {
	for _, edge := range cycle {
		for _, existing := range a.errors {
			if existing.File == edge.token.File && existing.Line == edge.token.Line && existing.Column == edge.token.Column {
				return true
			}
		}
	}
	return false
}

func structLayoutStackIndex(stack []string, target string) int {
	for index := len(stack) - 1; index >= 0; index-- {
		if stack[index] == target {
			return index
		}
	}
	return -1
}

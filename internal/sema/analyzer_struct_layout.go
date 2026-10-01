package sema

import (
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

type structLayoutEdge struct {
	owner  string
	field  string
	target string
	token  lexer.Token
}

type structLayoutNode struct {
	name     string
	token    lexer.Token
	isStruct bool
	edges    []structLayoutEdge
}

// validateStructLayoutCycles rejects module-level non-generic struct graphs,
// including paths through named by-value wrappers, whose storage dependencies
// contain a cycle. The graph is built from declarations rather than partially
// resolved Type values so forward and mutually recursive declarations receive
// the same deterministic validation.
// References, slices, dynamic arrays, and raw pointers are representation
// boundaries and therefore do not contribute by-value edges.
//
// Rules:
//   - rules/declarations/struct.md — §7 "Struct defaults", paragraphs 6-8
//   - rules/declarations/struct.md — §20 "Semantic analysis requirements"
//   - rules/types/types.md — "Type identity", "Named types"
//   - rules/memory/layout.md — §25(1)-(4) "Recursive layout"
//   - rules/memory/layout.md — §42(4) "Recursive-layout diagnostics"
func (a *Analyzer) validateStructLayoutCycles(program *ast.Program) {
	if program == nil {
		return
	}

	nodes := map[string]*structLayoutNode{}
	declarations := map[string]*ast.TypeDeclStatement{}
	order := []string{}
	a.withProgramModules(program, func(statement ast.Statement) {
		declaration, ok := statement.(*ast.TypeDeclStatement)
		if !ok || !isNonGenericLayoutDeclaration(declaration) || a.invalidTypeDeclaration(declaration.Name.Token) {
			return
		}
		name := declaration.Name.Value
		node := &structLayoutNode{name: name, token: declaration.Name.Token, isStruct: declaration.StructType != nil}
		nodes[name] = node
		declarations[name] = declaration
		order = append(order, name)
	})

	for name, declaration := range declarations {
		node := nodes[name]
		if declaration.StructType != nil {
			for _, field := range declaration.StructType.Fields {
				if field == nil || field.Name == nil {
					continue
				}
				target, byValue := structLayoutDependency(field.Type)
				if !byValue || nodes[target] == nil {
					continue
				}
				node.edges = append(node.edges, structLayoutEdge{
					owner: node.name, field: field.Name.Value, target: target, token: field.Name.Token,
				})
			}
			continue
		}
		reference := layoutDeclarationUnderlying(declaration)
		target, byValue := structLayoutDependency(reference)
		if byValue && nodes[target] != nil {
			node.edges = append(node.edges, structLayoutEdge{
				owner: node.name, target: target, token: reference.Token,
			})
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
				if !structLayoutCycleContainsStruct(stack[start:], nodes) {
					continue
				}
				reported[key] = true
				a.reportStructLayoutCycle(cycle)
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

func structLayoutCycleContainsStruct(names []string, nodes map[string]*structLayoutNode) bool {
	for _, name := range names {
		if node := nodes[name]; node != nil && node.isStruct {
			return true
		}
	}
	return false
}

// structLayoutDependency returns the nominal type embedded by one field.
// Fixed arrays preserve by-value containment; every explicit indirection or
// dynamic-storage boundary terminates the dependency.
//
// Rules:
//   - rules/memory/layout.md — §25(1), §25(2), and §25(4)
func structLayoutDependency(reference *ast.TypeReference) (string, bool) {
	if reference == nil || reference.Invalid || reference.Ref {
		return "", false
	}
	if reference.ElementType != nil {
		if reference.Slice {
			return "", false
		}
		return structLayoutDependency(reference.ElementType)
	}
	switch reference.Name {
	case "RawPtr", "list", "map", "set":
		return "", false
	}
	if len(reference.TypeArgs) != 0 || len(reference.ConstArgs) != 0 || reference.Name == "" {
		return "", false
	}
	return reference.Name, true
}

func structLayoutStackIndex(stack []string, target string) int {
	for index := len(stack) - 1; index >= 0; index-- {
		if stack[index] == target {
			return index
		}
	}
	return -1
}

// reportStructLayoutCycle emits the stable mandatory struct diagnostic with
// the concrete field path that closes the infinite by-value layout.
//
// Rules:
//   - rules/declarations/struct.md — §21 "Diagnostics"
//   - rules/memory/layout.md — §25(3) and §42(4)
//   - rules/tooling/diagnostics.md — §5, §8, and §12
func (a *Analyzer) reportStructLayoutCycle(cycle []structLayoutEdge) {
	if len(cycle) == 0 {
		return
	}
	parts := make([]string, 0, len(cycle)+1)
	for _, edge := range cycle {
		part := edge.owner
		if edge.field != "" {
			part += "." + edge.field
		}
		parts = append(parts, part)
	}
	parts = append(parts, cycle[len(cycle)-1].target)
	a.addErrorAtTokenWithMetadata(
		cycle[len(cycle)-1].token,
		diagnostics.RecursiveStructLayout,
		"Break the cycle with a reference, raw pointer, slice, or owning dynamic-storage boundary.",
		"recursive by-value struct layout has infinite size: %s",
		strings.Join(parts, " -> "),
	)
}

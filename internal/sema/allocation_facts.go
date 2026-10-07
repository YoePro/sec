package sema

import (
	"reflect"
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// AllocationKnowledge is presentation of canonical synchronous allocation
// evidence, distinct from source validity, allocation context and panic.
// Rules: rules/memory/allocation.md — §§6,24,29(1)-(2).
type AllocationKnowledge string

const (
	AllocationFree        AllocationKnowledge = "allocation-free"
	AllocationMayAllocate AllocationKnowledge = "may-allocate"
	AllocationUnknown     AllocationKnowledge = "unknown"
)

// AllocationEvidence explains incomplete coverage or a canonical unknown site.
// Rules: rules/memory/allocation.md — §§24(6),28,29.
type AllocationEvidence struct {
	Source lexer.Token
	Reason string
}

// AllocationCallableFact retains independent positive and unresolved evidence:
// a known allocating path wins the classification even if another path is
// unresolved. Free requires completed semantic analysis and coverage, not
// merely an empty list of recorded allocation sites.
// Rules: rules/memory/allocation.md — §§24(2),(4),(6),29(1)-(2).
type AllocationCallableFact struct {
	Callable                    CallableID
	Declaration                 lexer.Token
	Knowledge                   AllocationKnowledge
	HasUnknown                  bool
	AllocationPath, UnknownPath []CallableID
	UnknownEvidence             []AllocationEvidence
}

// AllocationFacts returns deterministic detached facts for every represented
// callable, consuming canonical graph effects/relations, resolved types and
// foreign contracts. No additional source diagnostics are produced.
// Rules: rules/memory/allocation.md — §§24,29; rules/analysis/call_graph.md — "One canonical graph, multiple analysis views".
func (a *Analyzer) AllocationFacts() []AllocationCallableFact {
	unknown := a.allocationCoverageUnknowns()
	result := []AllocationCallableFact{}
	for _, node := range a.callGraph.Nodes() {
		result = append(result, a.allocationCallableFact(node.ID, unknown))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Callable < result[j].Callable })
	return result
}

// AllocationFact returns unknown for a missing callable rather than treating
// absent semantic evidence as a proof of allocation freedom.
// Rules: rules/memory/allocation.md — §§6(4),24(6),29(2).
func (a *Analyzer) AllocationFact(id CallableID) AllocationCallableFact {
	return a.allocationCallableFact(id, a.allocationCoverageUnknowns())
}

// allocationCallableFact correlates independent positive and unknown shortest
// paths, retaining coverage uncertainty instead of strengthening the existing
// allocation model at a tooling boundary.
// Rules: rules/memory/allocation.md — §§24(2),(6),29(1)-(2).
func (a *Analyzer) allocationCallableFact(id CallableID, unknown map[CallableID][]AllocationEvidence) AllocationCallableFact {
	fact := AllocationCallableFact{Callable: id, Knowledge: AllocationUnknown, HasUnknown: true}
	node, known := a.callGraph.Node(id)
	if !known {
		fact.UnknownEvidence = []AllocationEvidence{{Reason: "no canonical callable body is available"}}
		return fact
	}
	fact.Declaration = node.Declaration
	summary := a.callGraph.ArenaSummary(id)
	fact.AllocationPath = append([]CallableID(nil), summary.AllocationPath...)
	fact.UnknownPath = a.callGraph.synchronousPathTo(id, func(candidate CallableID) bool { return len(unknown[candidate]) > 0 })
	fact.HasUnknown = len(fact.UnknownPath) > 0
	if fact.HasUnknown {
		fact.UnknownEvidence = append([]AllocationEvidence(nil), unknown[fact.UnknownPath[len(fact.UnknownPath)-1]]...)
	}
	switch {
	case summary.MayAllocate:
		fact.Knowledge = AllocationMayAllocate
	case fact.HasUnknown:
		fact.Knowledge = AllocationUnknown
	default:
		fact.Knowledge = AllocationFree
	}
	return fact
}

// allocationCoverageUnknowns supplies conservative canonical coverage guards.
// Bodyless foreign declarations need trusted @noAlloc; unfinished Sema,
// unmodeled storage-producing operations and nontrivial cleanup cannot acquire
// positive freedom solely because the represented graph has no allocation site.
// This does not classify those operations as allocating or change @noAlloc
// validation; their complete effect models remain owned by frontend allocation.
// Rules: rules/memory/allocation.md — §§3(1),6(4),13,18,19,24(6);
// rules/compiler/compiler_analysis.md — unknown versus proven facts.
func (a *Analyzer) allocationCoverageUnknowns() map[CallableID][]AllocationEvidence {
	unknown := map[CallableID][]AllocationEvidence{}
	bodies := map[CallableID]*ast.BlockStatement{}
	parameters := map[CallableID][]*ast.Parameter{}
	functions := map[CallableID]Function{}
	for _, overloads := range a.functions {
		for _, fn := range overloads {
			functions[callableID(fn)] = fn
		}
	}
	if a.iteratorLoweringProgram != nil {
		walkASTValue(reflect.ValueOf(a.iteratorLoweringProgram), func(value any) {
			var token lexer.Token
			var body *ast.BlockStatement
			var params []*ast.Parameter
			switch value := value.(type) {
			case *ast.FunctionDeclaration:
				if value != nil && value.Name != nil {
					token = value.Name.Token
					body = value.Body
					params = value.Parameters
				}
			case *ast.LambdaExpression:
				if value != nil {
					token = value.Token
					body = value.Body
					params = value.Parameters
				}
			case *ast.DeferStatement:
				if value != nil {
					token = value.Token
					body = value.Body
				}
			}
			if body != nil {
				for _, node := range a.callGraph.NodesForDeclaration(token) {
					bodies[node.ID] = body
					parameters[node.ID] = params
				}
			}
		})
	}
	for _, node := range a.callGraph.Nodes() {
		add := func(source lexer.Token, reason string) {
			unknown[node.ID] = append(unknown[node.ID], AllocationEvidence{source, reason})
		}
		for _, site := range a.callGraph.arenaEffects[node.ID] {
			if site.UnknownAllocation {
				add(site.Source, "canonical allocation effect is unresolved: "+string(site.Kind))
			}
		}
		if len(a.errors) > 0 {
			add(node.Declaration, "semantic analysis is incomplete or invalid; allocation freedom is not proven")
		}
		if node.Extern {
			if fn, known := functions[node.ID]; !known || !fn.TrustedNoAlloc {
				add(node.Declaration, "foreign body has no trusted allocation-free contract")
			}
			continue
		}
		body, known := bodies[node.ID]
		if !known {
			add(node.Declaration, "completed allocation coverage is unavailable for this callable body")
			continue
		}
		if fn, known := functions[node.ID]; known {
			if len(fn.GenericParameters) > 0 {
				add(node.Declaration, "generic instantiation has no complete allocation-effect coverage")
			}
			for _, parameter := range fn.Parameters {
				if !allocationCleanupCovered(parameter.Type) {
					add(node.Declaration, "nontrivial parameter cleanup has no complete allocation-effect coverage")
				}
			}
		}
		for _, parameter := range parameters[node.ID] {
			if binding, known := a.ResolvedBindingOf(parameter.Name); !known || !allocationCleanupCovered(binding.Type) {
				add(parameter.Token, "parameter cleanup has no complete allocation-effect coverage")
			}
		}
		walkASTValue(reflect.ValueOf(body), func(value any) {
			if expression, ok := value.(ast.Expression); ok {
				if typ, known := a.expressionTypes[expression]; known && !allocationCleanupCovered(typ) {
					add(expressionToken(expression), "nontrivial value cleanup has no complete allocation-effect coverage")
				}
			}
			switch value := value.(type) {
			case *ast.LetStatement:
				if binding, known := a.ResolvedBindingOf(value.Name); !known || !allocationCleanupCovered(binding.Type) {
					add(value.Token, "local cleanup has no complete allocation-effect coverage")
				}
			case *ast.SpawnExpression:
				add(value.Token, "spawn control storage has no complete allocation-effect coverage")
			case *ast.RuntimeCallExpression:
				add(value.Token, "runtime call has no complete allocation-effect coverage")
			case *ast.AsmStatement:
				add(value.Token, "assembly has no complete allocation-effect coverage")
			case *ast.NewExpression:
				add(value.Token, "construction has no complete allocation-effect coverage")
			case *ast.AwaitExpression:
				add(value.Token, "await runtime storage has no complete allocation-effect coverage")
			case *ast.LambdaExpression:
				add(value.Token, "closure environment storage has no complete allocation-effect coverage")
			case *ast.CallExpression:
				if _, known := a.ResolvedCallTarget(value); known {
					return
				}
				if member, ok := value.Callee.(*ast.MemberExpression); ok && member.Property != nil {
					contract, known := a.CompilerKnownMemberAt(member.Property.Token.File, member.Property.Token.Line, member.Property.Token.Column)
					if known && contract.AllocationBehavior == AllocationFree {
						return
					}
				}
				// Known allocation operations already have canonical direct
				// effects. Other compiler-known calls cannot prove freedom
				// without an allocation-effect contract.
				for _, site := range a.callGraph.arenaEffects[node.ID] {
					if sourceTokenLocation(site.Source) == sourceTokenLocation(callCalleeDefinitionToken(value)) {
						return
					}
				}
				add(expressionToken(value), "call has no complete resolved allocation-effect contract")
			case *ast.CollectionLiteral:
				typ, known := a.expressionTypes[value]
				if !known || typ.Kind != ArrayType || arrayShapeOf(typ) != ArrayShapeFixed {
					add(value.Token, "owning collection storage has no complete allocation-effect coverage")
				}
			}
		})
	}
	for id, evidence := range unknown {
		sort.Slice(evidence, func(i, j int) bool {
			left, right := evidence[i], evidence[j]
			if left.Source.File != right.Source.File {
				return left.Source.File < right.Source.File
			}
			if left.Source.Line != right.Source.Line {
				return left.Source.Line < right.Source.Line
			}
			if left.Source.Column != right.Source.Column {
				return left.Source.Column < right.Source.Column
			}
			return left.Reason < right.Reason
		})
		unique := evidence[:0]
		for _, item := range evidence {
			if len(unique) == 0 || item != unique[len(unique)-1] {
				unique = append(unique, item)
			}
		}
		unknown[id] = unique
	}
	return unknown
}

// allocationCleanupCovered excludes unresolved templates from the otherwise
// canonical trivial-destruction proof. Raw pointers never destroy their pointee,
// including pointers whose resolved type retains generic declaration metadata.
// Runtime cleanup coverage is still partial.
// Rules: rules/memory/allocation.md — §§6(4),13,24(6); rules/memory/destruction.md — §3.3;
// rules/memory/raw_pointers.md — §§2(9),4(6).
func allocationCleanupCovered(typ Type) bool {
	if typ.Kind == RawPtrType {
		return TriviallyDestructible(typ)
	}
	return typ.Kind != GenericType && typ.Kind != InterfaceType && len(typ.GenericParameters) == 0 && TriviallyDestructible(typ)
}

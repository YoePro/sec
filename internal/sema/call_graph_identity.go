package sema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// callableID separates declaration identity from navigable source positions.
// Overload signatures, implementation receivers and foreign linkage remain
// distinct. Bodies and guarantee changes invalidate facts, not declaration IDs.
// Rules: rules/analysis/call_graph.md — "Callable node identity", "Incremental tests".
func callableID(function Function) CallableID {
	parameters := make([]string, len(function.Parameters))
	for i, parameter := range function.Parameters {
		parameters[i] = fmt.Sprintf("%s:%t:%t:%t:%t", graphTypeIdentity(parameter.Type), parameter.Ref, parameter.MutableRef,
			parameter.Consuming, parameter.Variadic)
	}
	key := []any{function.Module, function.Token.File, function.Name, function.ImplTarget,
		parameters, graphTypeIdentity(function.ReturnType), function.GenericParameters,
		function.Static, function.ReceiverConsuming,
		function.Extern, function.ABI, function.LinkName}
	encoded, _ := json.Marshal(key)
	return CallableID("declaration|" + graphIdentityDigest(encoded))
}

// graphTypeIdentity preserves nominal module identity and nested reference,
// callable, extent and unit distinctions in a resolved declaration signature.
// Body-derived layout, storage and retention annotations do not define a type
// declaration's identity. Parameter display names do not define callable types.
// Rules: rules/analysis/call_graph.md — "Callable node identity";
// rules/corrections/applied/types-callable-model-correction-20260816.md — "Required correction";
// rules/types/types.md — type identity; rules/types/units.md — quantity identity.
func graphTypeIdentity(typ Type) string {
	arguments := make([]string, len(typ.TypeArgs))
	for i, argument := range typ.TypeArgs {
		arguments[i] = graphTypeIdentity(argument)
	}
	parameters := make([]string, len(typ.FunctionParameterTypes))
	for i, parameter := range typ.FunctionParameterTypes {
		parameters[i] = graphTypeIdentity(parameter)
	}
	element, result := "", ""
	if typ.Element != nil && (typ.Kind == ReferenceType || typ.Kind == ArrayType || typ.Kind == SliceType || typ.Kind == VariadicPackType) {
		element = graphTypeIdentity(*typ.Element)
	}
	if typ.FunctionReturnType != nil {
		result = graphTypeIdentity(*typ.FunctionReturnType)
	}
	key, _ := json.Marshal([]any{typ.Module, typ.Kind, typ.Name, typ.Unit,
		canonicalTypeIdentity(typ), typeDisplayName(typ), arguments, typ.ConstArgs,
		element, typ.ReferenceMutable, parameters, result,
		normalizedCallableCapability(typ.FunctionCapability), typ.FunctionVariadic})
	return graphIdentityDigest(key)
}

// prepareGraphSyntaxOrigins assigns lexical semantic origins before Sema or
// lowering can mutate the AST. Each declaration is its own namespace, and each
// node kind has a source-ordered occurrence path within its lexical owner.
// Lambda/defer boundaries nest their own namespaces. Counters are local syntax
// occurrence paths, never parser indices or analysis registration counters.
// Pointer equality is only a traversal guard, never part of an emitted key.
// Source coordinates locate current AST records but do not define identities.
// Rules: rules/analysis/call_graph.md — "Callable node identity", "Call-site identity",
// "Incremental tests"; rules/analysis/closure_analysis.md — "Abstract closure identity".
func prepareGraphSyntaxOrigins(program *ast.Program) map[sourceTokenKey]string {
	origins := map[sourceTokenKey]string{}
	if program == nil {
		return origins
	}
	seen := map[uintptr]bool{}
	var walk func(reflect.Value, string, map[string]int)
	walk = func(value reflect.Value, owner string, counts map[string]int) {
		if !value.IsValid() {
			return
		}
		if value.Kind() == reflect.Interface {
			if !value.IsNil() {
				walk(value.Elem(), owner, counts)
			}
			return
		}
		if value.Kind() == reflect.Pointer {
			if value.IsNil() || seen[value.Pointer()] {
				return
			}
			seen[value.Pointer()] = true
			if value.CanInterface() {
				node := value.Interface()
				kind := value.Type().Elem().Name()
				if record := value.Elem(); record.Kind() == reflect.Struct {
					field := record.FieldByName("Token")
					if field.IsValid() && field.Type() == reflect.TypeOf(lexer.Token{}) {
						token := field.Interface().(lexer.Token)
						origin := fmt.Sprintf("%s/%s[%d]", owner, kind, counts[kind])
						counts[kind]++
						switch declaration := node.(type) {
						case *ast.FunctionDeclaration:
							header := []string{}
							if declaration.Name != nil {
								header = append(header, declaration.Name.Value)
							}
							for _, parameter := range declaration.Parameters {
								if parameter != nil && parameter.Type != nil {
									header = append(header, fmt.Sprintf("%s:%t:%t:%t:%t", typeReferenceDisplayName(parameter.Type), parameter.Ref, parameter.MutableRef, parameter.Consuming, parameter.Variadic))
								}
							}
							if declaration.ReturnType != nil {
								header = append(header, typeReferenceDisplayName(declaration.ReturnType))
							}
							encodedHeader, _ := json.Marshal(header)
							// Header spelling identifies syntax ownership only; semantic
							// declaration IDs use the resolved overload signature above.
							origin = owner + "/function:" + string(encodedHeader)
							owner, counts = origin, map[string]int{}
						case *ast.TestDeclaration:
							if declaration.Name != nil {
								origin = owner + "/test:" + declaration.Name.Value
							}
							owner, counts = origin, map[string]int{}
						case *ast.LambdaExpression, *ast.DeferStatement:
							owner, counts = origin, map[string]int{}
						}
						if token.Line > 0 {
							key := sourceTokenLocation(token)
							if _, exists := origins[key]; !exists {
								origins[key] = origin
							}
						}
					}
				}
			}
			walk(value.Elem(), owner, counts)
			return
		}
		switch value.Kind() {
		case reflect.Struct:
			if field := value.FieldByName("Token"); field.IsValid() && field.CanInterface() && field.Type() == reflect.TypeOf(lexer.Token{}) {
				token := field.Interface().(lexer.Token)
				key := sourceTokenLocation(token)
				if _, exists := origins[key]; !exists && token.Line > 0 {
					kind := value.Type().Name()
					origins[key] = fmt.Sprintf("%s/%s[%d]", owner, kind, counts[kind])
					counts[kind]++
				}
			}
			if value.Type() == reflect.TypeOf(lexer.Token{}) {
				return
			}
			for i := 0; i < value.NumField(); i++ {
				if value.Type().Field(i).IsExported() {
					walk(value.Field(i), owner, counts)
				}
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < value.Len(); i++ {
				walk(value.Index(i), owner, counts)
			}
		}
	}
	for _, statement := range program.Statements {
		// Module statements and declaration headers provide top-level ownership;
		// source filenames distinguish files without tying IDs to their positions.
		token := statementToken(statement)
		owner := token.File
		value := reflect.ValueOf(statement)
		if value.IsValid() && value.Kind() == reflect.Pointer && !value.IsNil() {
			record := value.Elem()
			for _, fieldName := range []string{"Name", "Target"} {
				field := record.FieldByName(fieldName)
				// Parser recovery keeps a declaration whose name was not
				// written yet (such as `test {` while typing) with a nil node.
				if field.IsValid() && (field.Kind() == reflect.Pointer || field.Kind() == reflect.Interface) && field.IsNil() {
					continue
				}
				if field.IsValid() && field.CanInterface() {
					if expression, ok := field.Interface().(interface{ String() string }); ok {
						owner += "/" + expression.String()
					}
				}
			}
		}
		walk(value, owner, map[string]int{})
	}
	return origins
}

// syntaxOrigin returns the prepared lexical origin. Graphs constructed by
// explicit runtime/contract producers may use their supplied token as a fallback
// origin until that producer provides a persistent generated operation identity.
// Rules: rules/analysis/call_graph.md — "Call-site identity", "Generated callables".
func (g *CallGraph) syntaxOrigin(source lexer.Token) string {
	if origin := g.syntaxOrigins[sourceTokenLocation(source)]; origin != "" {
		return origin
	}
	return fmt.Sprintf("%s:%d:%d", source.File, source.Line, source.Column)
}

// semanticCallSiteID preserves the containing callable, syntax origin, dispatch
// and execution operation while source locations remain in CallSite.Source.
// Rules: rules/analysis/call_graph.md — "Call-site identity".
func (g *CallGraph) semanticCallSiteID(caller CallableID, source lexer.Token, dispatch CallDispatchKind, execution CallExecutionRelation) CallSiteID {
	key, _ := json.Marshal([]string{string(caller), g.syntaxOrigin(source), string(dispatch), string(execution)})
	return CallSiteID("call-site|" + graphIdentityDigest(key))
}

// graphIdentityDigest keeps structured semantic keys compact without exposing
// map order, parser numbering or physical source coordinates as identity.
// Rules: rules/analysis/call_graph.md — "Callable node identity", "Call-site identity".
func graphIdentityDigest(key []byte) string {
	digest := sha256.Sum256(key)
	return hex.EncodeToString(digest[:])
}

// scopedGraphIdentity namespaces one semantic record by concrete plan/model.
// Rules: rules/analysis/call_graph.md — "One graph per `CompilationPlan`",
// "Callable node identity", "Call-site identity".
func scopedGraphIdentity(scope CallGraphScope, kind, identity string) string {
	key, _ := json.Marshal([]string{scope.Module, scope.CompilationPlan, scope.CompilerModel, kind, identity})
	return "plan-" + kind + "|" + graphIdentityDigest(key)
}

// CompilationScope returns the explicit producer-supplied scope. Zero denotes
// an unbound Sema snapshot, never a host-derived concrete compilation plan.
// Rules: rules/analysis/call_graph.md — "One graph per `CompilationPlan`".
func (g *CallGraph) CompilationScope() CallGraphScope {
	if g == nil {
		return CallGraphScope{}
	}
	return g.scope
}

// CallableInScope resolves an unbound declaration/body node ID into this graph.
// Already-bound IDs are accepted without applying the namespace twice.
// Rules: rules/analysis/call_graph.md — "Callable node identity".
func (g *CallGraph) CallableInScope(id CallableID) (CallableID, bool) {
	if g == nil {
		return "", false
	}
	if _, exists := g.nodes[id]; exists {
		return id, true
	}
	if g.scope == (CallGraphScope{}) {
		return "", false
	}
	qualified := CallableID(scopedGraphIdentity(g.scope, "callable", string(id)))
	_, exists := g.nodes[qualified]
	return qualified, exists
}

// ForCompilationPlan binds a detached graph to one concrete scope atomically.
// All node/body/site/root references and introducing effect facts move together;
// target coverage and reachability are unchanged. Binding does not perform
// source selection: the frontend must have analyzed the active source universe
// for this plan. A graph already bound to another plan cannot be relabelled or
// merged. This keeps cross-plan index queries separate from runtime graphs.
// Rules: rules/analysis/call_graph.md — "One graph per `CompilationPlan`",
// "Cross-plan comparison", "Inactive declarations", "Graph data model".
func (g *CallGraph) ForCompilationPlan(scope CallGraphScope) (*CallGraph, error) {
	if g == nil || scope.Module == "" || scope.CompilationPlan == "" || scope.CompilerModel == "" {
		return nil, fmt.Errorf("graph requires complete compilation scope")
	}
	if g.scope != (CallGraphScope{}) {
		if g.scope != scope {
			return nil, fmt.Errorf("graph belongs to another compilation plan")
		}
		return g.clone(), nil
	}
	if !validTestRootScope(g, scope) {
		return nil, fmt.Errorf("test roots belong to another compilation plan")
	}
	result := newCallGraph()
	result.scope = scope
	callable := func(id CallableID) CallableID { return CallableID(scopedGraphIdentity(scope, "callable", string(id))) }
	body := func(id CallableBodyID) CallableBodyID {
		return CallableBodyID(scopedGraphIdentity(scope, "body", string(id)))
	}
	siteID := func(id CallSiteID) CallSiteID {
		if id == "" {
			return ""
		}
		return CallSiteID(scopedGraphIdentity(scope, "site", string(id)))
	}
	for _, id := range g.nodeOrder {
		node := g.nodes[id]
		node.ID = callable(id)
		result.nodes[node.ID] = node
		result.nodeOrder = append(result.nodeOrder, node.ID)
	}
	for id, target := range g.bodyNodes {
		result.bodyNodes[body(id)] = callable(target)
	}
	for _, original := range g.sites {
		site := cloneCallSite(original)
		site.ID, site.Caller = siteID(site.ID), callable(site.Caller)
		for i, target := range site.Targets {
			site.Targets[i] = callable(target)
		}
		for i, target := range site.TargetSet.KnownTargets {
			site.TargetSet.KnownTargets[i] = body(target)
		}
		if site.TargetSet.OpenContract != "" {
			site.TargetSet.OpenContract = CallableContractID(scopedGraphIdentity(scope, "contract", string(site.TargetSet.OpenContract)))
			if site.TargetSet.Contract != nil {
				site.TargetSet.Contract.ID = site.TargetSet.OpenContract
			}
		}
		result.sites = append(result.sites, site)
		result.siteIDs[site.ID] = true
	}
	for _, id := range g.rootOrder {
		root := g.roots[id]
		root.ID = CallRootID(scopedGraphIdentity(scope, "root", string(root.ID)))
		root.Node, root.ParentSite, root.Scope = callable(root.Node), siteID(root.ParentSite), scope
		result.roots[root.ID] = root
		result.rootOrder = append(result.rootOrder, root.ID)
	}
	for id, facts := range g.effects {
		result.effects[callable(id)] = cloneEffectSites(facts)
	}
	for id, facts := range g.arenaEffects {
		result.arenaEffects[callable(id)] = append([]ArenaEffectSite(nil), facts...)
	}
	for id, facts := range g.blockEffects {
		result.blockEffects[callable(id)] = append([]BlockEffectSite(nil), facts...)
	}
	return result, nil
}

// GeneratedIdentity is compiler metadata, never a Sec declaration. Source is
// navigable provenance; Kind, Owner, Origin and Target define the stable key.
// Rules: rules/compiler/compiler_pipeline.md — §§76(2),103(1–4);
// rules/compiler/compiler.md — §71.
type GeneratedIdentity struct {
	ID     string
	Kind   string
	Owner  string
	Origin string
	Target string
	Source lexer.Token
}

// GeneratedIdentityIndex uses lexical declaration/occurrence origins rather
// than registration order, source coordinates or process-local pointers.
// Rules: rules/compiler/compiler.md — §71; compiler_pipeline.md — §§76,103.
type GeneratedIdentityIndex struct {
	origins map[sourceTokenKey]string
	modules map[string]string
	records map[string]GeneratedIdentity
}

// NewGeneratedIdentityIndex freezes syntax origins before analysis/emission.
// Rules: rules/compiler/compiler_pipeline.md — §§76,103; compiler.md — §71.
func NewGeneratedIdentityIndex(program *ast.Program) *GeneratedIdentityIndex {
	index := &GeneratedIdentityIndex{origins: prepareGraphSyntaxOrigins(program), modules: map[string]string{}, records: map[string]GeneratedIdentity{}}
	if program != nil {
		for _, statement := range program.Statements {
			if module, ok := statement.(*ast.ModuleStatement); ok && module != nil {
				index.modules[module.Token.File] = module.Path
			}
		}
	}
	return index
}

// Resolve returns one compiler-owned identity and its provenance. Coordinates
// are a lookup into current syntax, never part of a prepared lexical identity.
// Missing syntax origins are diagnosed rather than inventing a shared identity.
// Target is supplied by the resolved plan when representation affects the helper.
// Rules: rules/compiler/compiler.md — §71; compiler_pipeline.md — §103.
func (i *GeneratedIdentityIndex) Resolve(kind, owner string, source lexer.Token, target string) (GeneratedIdentity, bool) {
	if i == nil {
		return GeneratedIdentity{}, false
	}
	origin, ok := i.origins[sourceTokenLocation(source)]
	if !ok {
		return GeneratedIdentity{}, false
	}
	owner = i.modules[source.File] + "/" + owner
	key, _ := json.Marshal([]string{kind, owner, origin, target})
	record := GeneratedIdentity{ID: kind + "-" + graphIdentityDigest(key), Kind: kind, Owner: owner, Origin: origin, Target: target, Source: source}
	i.records[record.ID] = record
	return record, true
}

// Records returns detached provenance in canonical identity order.
// Rules: rules/compiler/compiler_pipeline.md — §§76(2),103(3); compiler.md — §71.
func (i *GeneratedIdentityIndex) Records() []GeneratedIdentity {
	if i == nil {
		return nil
	}
	records := make([]GeneratedIdentity, 0, len(i.records))
	for _, record := range i.records {
		records = append(records, record)
	}
	sort.Slice(records, func(a, b int) bool { return records[a].ID < records[b].ID })
	return records
}

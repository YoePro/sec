package sema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"

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

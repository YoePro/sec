package sema

import (
	"fmt"
	"sort"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// EscapeCauseStep is source-mapped explanation, not a new validity decision.
// Rules: rules/analysis/escape_analysis.md — "Cause paths", "Diagnostic quality".
type EscapeCauseStep struct {
	Kind    string
	Message string
	Source  lexer.Token
}

// EscapeCausePath preserves the owning escape mode, destination and one
// representative origin-to-sink witness. Incomplete describes explanation
// coverage only; it cannot weaken an owning error or manufacture a proof.
// Rules: rules/analysis/escape_analysis.md — "Cause paths", "Diagnostic architecture",
// "Diagnostic quality", "Precision exhaustion".
type EscapeCausePath struct {
	Mode        EscapeMode
	Destination EscapeDestination
	Steps       []EscapeCauseStep
	Incomplete  bool
}

// cloneEscapeCausePaths detaches explanation slices from analysis and callers.
// Rules: rules/compiler/compiler_analysis.md — §8(1–4), immutable analysis facts;
// rules/analysis/escape_analysis.md — "Public compiler-facing result".
func cloneEscapeCausePaths(paths []EscapeCausePath) []EscapeCausePath {
	if paths == nil {
		return nil
	}
	result := append([]EscapeCausePath(nil), paths...)
	for index := range result {
		result[index].Steps = append([]EscapeCauseStep(nil), paths[index].Steps...)
	}
	return result
}

// CausePaths reconstructs deterministic explanations from canonical historical
// facts without tracing source names, rerunning analysis or changing precision.
// Each source's Place projections and carrier path remain separate relationships.
// Rules: rules/analysis/escape_analysis.md — "Cause paths", "Place provenance",
// "Aggregates as dependency carriers", "Public compiler-facing result".
func (fact EscapeFact) CausePaths() []EscapeCausePath {
	sources := append([]EscapeSource(nil), fact.Sources...)
	sort.SliceStable(sources, func(i, j int) bool {
		left, right := sources[i], sources[j]
		if left.Token.File != right.Token.File {
			return left.Token.File < right.Token.File
		}
		if left.Token.Line != right.Token.Line {
			return left.Token.Line < right.Token.Line
		}
		if left.Token.Column != right.Token.Column {
			return left.Token.Column < right.Token.Column
		}
		if left.Place.String() != right.Place.String() {
			return left.Place.String() < right.Place.String()
		}
		return left.CarrierPath < right.CarrierPath
	})
	var result []EscapeCausePath
	for _, source := range sources {
		path := EscapeCausePath{Mode: fact.Mode, Destination: fact.Destination, Incomplete: fact.Unknown || source.Kind == EscapeSourceUnknown}
		kind, message := "origin", fmt.Sprintf("%s origin %q", source.Kind, source.Name)
		if source.Kind == EscapeSourceUnknown {
			kind, message = "unknown", "origin provenance is unknown"
		}
		path.Steps = append(path.Steps, EscapeCauseStep{kind, message, source.Token})
		if len(source.Place.Projections) > 0 {
			path.Steps = append(path.Steps, EscapeCauseStep{"projection", "dependency comes from Place " + source.Place.String(), source.Place.Projections[len(source.Place.Projections)-1].Token})
		}
		if source.CarrierPath != "" {
			path.Steps = append(path.Steps, EscapeCauseStep{"carrier", "dependency is carried at " + source.CarrierPath, fact.Sink})
		}
		path.Steps = append(path.Steps, EscapeCauseStep{"boundary", fmt.Sprintf("%s dependency reaches %s", fact.Mode, fact.Destination), fact.Sink})
		result = append(result, path)
	}
	if len(result) == 0 {
		result = []EscapeCausePath{{Mode: fact.Mode, Destination: fact.Destination, Incomplete: true, Steps: []EscapeCauseStep{{"unknown", "origin provenance is unavailable", lexer.Token{}}, {"boundary", fmt.Sprintf("%s dependency reaches %s", fact.Mode, fact.Destination), fact.Sink}}}}
	}
	return result
}

// escapeDiagnosticCause reconstructs one diagnostic witness at the rejecting
// operation while flow-sensitive provenance is still live. It selects existing
// facts by callable, sink and origin source identity; missing facts get only
// the origin and boundary already established by the owning check.
// Rules: rules/analysis/escape_analysis.md — "Cause paths", "Current provenance versus historical escape",
// "Diagnostic quality", "Unknown escape".
func (a *Analyzer) escapeDiagnosticCause(token, origin lexer.Token, id string, expr ast.Expression) []EscapeCausePath {
	mode, destination := EscapeModeBorrowEscape, EscapeDestinationReturnedValue
	boundary := "returned dependency requires storage that remains valid after this invocation"
	switch id {
	case diagnostics.EscapeOuterPlace:
		destination = EscapeDestinationOuterPlace
		boundary = "stored dependency requires storage valid for the destination's complete lifetime"
	case diagnostics.EscapeMatchPayload:
		boundary = "payload dependency crosses the active match arm's lifetime boundary"
	case diagnostics.EscapeClosureCapture:
		mode = EscapeModeCapture
		boundary = "returned closure retains a captured dependency beyond this invocation"
	case diagnostics.EscapeVariadicPack:
		boundary = "variadic pack storage is available only during this invocation"
	case diagnostics.EscapeProvenanceUnknown:
		mode = EscapeModeUnknown
		boundary = "returned dependency lacks a proven caller-owned origin"
	}
	path := EscapeCausePath{Mode: mode, Destination: destination}
	if origin.Line > 0 {
		message := "origin storage is local to this invocation"
		if id == diagnostics.EscapeMatchPayload {
			message = "payload storage is scoped to the active match arm"
		}
		path.Steps = append(path.Steps, EscapeCauseStep{"origin", message, origin})
	} else if id == diagnostics.EscapeProvenanceUnknown {
		path.Incomplete = true
		path.Steps = append(path.Steps, EscapeCauseStep{"unknown", "current control-flow provenance is unknown", lexer.Token{}})
	}
	if a.escapeAnalysis != nil && id != diagnostics.EscapeProvenanceUnknown {
	factLoop:
		for _, fact := range a.escapeAnalysis.facts {
			if fact.Callable != a.currentCallable || !sameSourceToken(fact.Sink, token) {
				continue
			}
			for _, candidate := range fact.CausePaths() {
				if len(candidate.Steps) == 0 || !sameSourceToken(candidate.Steps[0].Source, origin) {
					continue
				}
				for _, step := range candidate.Steps[1 : len(candidate.Steps)-1] {
					path.Steps = append(path.Steps, step)
				}
				path.Incomplete = path.Incomplete || candidate.Incomplete
				break factLoop
			}
		}
	}
	if expr != nil && id != diagnostics.EscapeProvenanceUnknown {
		a.appendEscapeExpressionSteps(&path, expr, origin, 0)
	}
	if id == diagnostics.EscapeClosureCapture && len(path.Steps) < 63 {
		path.Steps = append(path.Steps, EscapeCauseStep{"capture", "closure environment retains this captured reference", token})
	}
	if len(path.Steps) == 0 {
		path.Incomplete = true
		path.Steps = append(path.Steps, EscapeCauseStep{"unknown", "origin provenance is unavailable", lexer.Token{}})
	}
	path.Steps = append(path.Steps, EscapeCauseStep{"boundary", boundary, token})
	return []EscapeCausePath{path}
}

// appendEscapeExpressionSteps adds source operations only along the chosen
// canonical origin. It reads recorded expression types, origin facts and call
// resolutions; it never invokes inference, getters or callee analysis. Callable
// bodies are separate execution domains, and bounded explanations mark omissions.
// Rules: rules/analysis/escape_analysis.md — "Cause paths", "Canonical sources",
// "Calls", "Closures", "Bounded precision", "Diagnostic quality".
func (a *Analyzer) appendEscapeExpressionSteps(path *EscapeCausePath, expr ast.Expression, origin lexer.Token, depth int) {
	if expr == nil {
		return
	}
	if depth >= 64 || len(path.Steps) >= 64 {
		path.Incomplete = true
		return
	}
	add := func(kind, message string, source lexer.Token) {
		if len(path.Steps) >= 63 {
			path.Incomplete = true
			return
		}
		path.Steps = append(path.Steps, EscapeCauseStep{kind, message, source})
	}
	walk := func(child ast.Expression) { a.appendEscapeExpressionSteps(path, child, origin, depth+1) }
	switch expr := expr.(type) {
	case *ast.RefExpression:
		walk(expr.Value)
		add("borrow", "reference is formed here", expr.Token)
	case *ast.SliceExpression:
		walk(expr.Left)
		add("view", "view retains its backing-storage dependency", expr.Token)
	case *ast.Identifier:
		symbol, known := a.symbols[expr.Value]
		if known && !variadicPackValue(symbol.Type) && !sameSourceToken(symbol.Token, origin) {
			add("carrier", fmt.Sprintf("current carrier binding %q is declared here", expr.Value), symbol.Token)
		}
		if known && variadicPackValue(symbol.Type) {
			add("origin", fmt.Sprintf("variadic pack %q is backed by this invocation", expr.Value), symbol.Token)
		}
	case *ast.MemberExpression:
		walk(expr.Object)
		add("projection", "reference dependency is accessed through "+expr.String(), expr.Token)
	case *ast.IndexExpression:
		walk(expr.Left)
		add("projection", "reference dependency is accessed through "+expr.String(), expr.Token)
	case *ast.CallExpression:
		resolved, known := a.resolvedCalls[expr]
		if !known || !resolved.Function.HasReturnOrigin {
			path.Incomplete = true
			add("unknown", "intermediate callable return provenance is unavailable", expr.Token)
			return
		}
		path.Incomplete = path.Incomplete || resolved.Function.ReturnOrigin.Unknown
		if member, ok := expr.Callee.(*ast.MemberExpression); ok && escapeSummaryUsesRoot(resolved.Function.ReturnOrigin, "$receiver", 0) && a.escapeExpressionHasOrigin(member.Object, origin) {
			walk(member.Object)
		}
		for index, argument := range expr.Arguments {
			if escapeSummaryUsesRoot(resolved.Function.ReturnOrigin, fmt.Sprintf("$param:%d", index), 0) && a.escapeExpressionHasOrigin(argument, origin) {
				walk(argument)
			}
		}
		add("call-summary", fmt.Sprintf("resolved return summary of %s propagates the dependency", resolved.Function.Name), resolved.Function.Token)
		add("call-result", "call result carries the reference dependency", expr.Token)
	case *ast.StructLiteral:
		for _, field := range expr.Fields {
			if field != nil && a.escapeExpressionHasOrigin(field.Value, origin) {
				walk(field.Value)
				add("carrier", "reference dependency is stored in the aggregate", expressionToken(field.Value))
				break
			}
		}
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if a.escapeExpressionHasOrigin(element, origin) {
				walk(element)
				add("carrier", "reference dependency is stored in the array", expressionToken(element))
				break
			}
		}
	case *ast.OkExpression:
		walk(expr.Value)
		add("carrier", "Result Ok payload carries the dependency", expr.Token)
	case *ast.ErrExpression:
		walk(expr.Value)
		add("carrier", "Result Err payload carries the dependency", expr.Token)
	case *ast.ConversionExpression:
		walk(expr.Value)
	case *ast.PrefixExpression:
		walk(expr.Right)
	}
}

// escapeExpressionHasOrigin matches recorded dependency metadata to the
// diagnostic's chosen origin, excluding unrelated call arguments/carrier fields.
// Rules: rules/analysis/escape_analysis.md — "Place provenance", "Cause paths",
// "Aggregates as dependency carriers", "Current provenance versus historical escape".
func (a *Analyzer) escapeExpressionHasOrigin(expr ast.Expression, token lexer.Token) bool {
	return a.escapeExpressionHasOriginAtDepth(expr, token, 0)
}

// escapeExpressionHasOriginAtDepth bounds expression matching as well as path construction.
// Rules: rules/analysis/escape_analysis.md — "Bounded precision", "Cause paths".
func (a *Analyzer) escapeExpressionHasOriginAtDepth(expr ast.Expression, token lexer.Token, depth int) bool {
	if expr == nil || token.Line <= 0 || depth >= 64 {
		return false
	}
	matches := func(origin localReferenceOrigin) bool { return escapeOriginContainsToken(origin, token) }
	if origin, ok := a.expressionReferenceOrigins[expr]; ok && matches(origin) {
		return true
	}
	if typ, ok := a.expressionTypes[expr]; ok && sameSourceToken(typ.ReferenceOriginToken, token) {
		return true
	}
	switch expr := expr.(type) {
	case *ast.Identifier:
		origin, ok := a.localRefContainers[expr.Value]
		return ok && matches(origin)
	case *ast.RefExpression:
		return a.escapeExpressionHasOriginAtDepth(expr.Value, token, depth+1) || sameSourceToken(expressionToken(expr.Value), token)
	case *ast.MemberExpression, *ast.IndexExpression:
		origin, ok := a.containedOriginForAccess(expr)
		return ok && matches(origin)
	case *ast.SliceExpression:
		return a.escapeExpressionHasOriginAtDepth(expr.Left, token, depth+1)
	}
	return false
}

// escapeOriginContainsToken searches canonical contained origins in stable
// key order without relying on display names or addresses as storage identity.
// Rules: rules/analysis/escape_analysis.md — "Place provenance", "Cause paths".
func escapeOriginContainsToken(origin localReferenceOrigin, token lexer.Token) bool {
	return escapeOriginContainsTokenAtDepth(origin, token, 0)
}

// escapeOriginContainsTokenAtDepth bounds traversal of canonical aggregate origins.
// Rules: rules/analysis/escape_analysis.md — "Bounded precision", "Cause paths".
func escapeOriginContainsTokenAtDepth(origin localReferenceOrigin, token lexer.Token, depth int) bool {
	if depth >= 64 {
		return false
	}
	if sameSourceToken(origin.Token, token) {
		return true
	}
	keys := make([]string, 0, len(origin.Contained))
	for key := range origin.Contained {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if escapeOriginContainsTokenAtDepth(origin.Contained[key], token, depth+1) {
			return true
		}
	}
	return false
}

// reportEscapeExpressionDiagnostic attaches a source operation path to the
// existing owning diagnostic; it changes neither acceptance nor diagnostic ID.
// Rules: rules/analysis/escape_analysis.md — "Diagnostic architecture", "Cause paths".
func (a *Analyzer) reportEscapeExpressionDiagnostic(expr ast.Expression, origin lexer.Token, id, format string, args ...any) {
	before := len(a.errors)
	a.reportEscapeDiagnostic(expressionToken(expr), origin, id, format, args...)
	if len(a.errors) > before {
		a.errors[len(a.errors)-1].EscapeCauses = a.escapeDiagnosticCause(expressionToken(expr), origin, id, expr)
	}
}

// cloneSemanticErrors keeps published diagnostic explanations detached.
// Rules: rules/compiler/compiler_analysis.md — §8(1–4), immutable results;
// rules/analysis/escape_analysis.md — "Public compiler-facing result".
func cloneSemanticErrors(errors []Error) []Error {
	if errors == nil {
		return nil
	}
	result := append([]Error(nil), errors...)
	for index := range result {
		result[index].EscapeCauses = cloneEscapeCausePaths(result[index].EscapeCauses)
	}
	return result
}

// reportEscapeAssignmentDiagnostic records the actual outer destination for a
// rejected store, including match payload stores which share a return category.
// Rules: rules/analysis/escape_analysis.md — "Cause paths", "Diagnostic quality",
// "Current provenance versus historical escape".
func (a *Analyzer) reportEscapeAssignmentDiagnostic(target, value ast.Expression, origin lexer.Token, id, format string, args ...any) {
	before := len(a.errors)
	a.reportEscapeExpressionDiagnostic(value, origin, id, format, args...)
	if len(a.errors) == before {
		return
	}
	path := &a.errors[len(a.errors)-1].EscapeCauses[0]
	path.Destination = EscapeDestinationOuterPlace
	last := len(path.Steps) - 1
	boundary := path.Steps[last]
	if id == diagnostics.EscapeMatchPayload {
		boundary.Message = "stored payload dependency crosses the active match arm's lifetime boundary"
	}
	if last > 62 {
		path.Incomplete = true
		path.Steps = path.Steps[:63]
		last = 62
	}
	path.Steps[last] = EscapeCauseStep{"destination", "dependency is stored into " + target.String(), expressionToken(target)}
	path.Steps = append(path.Steps, boundary)
}

// escapeSummaryUsesRoot selects only arguments or receivers named by the
// canonical symbolic return summary, including aggregate-contained returns.
// Equal actual origins do not make an unused argument a causal edge.
// Rules: rules/analysis/escape_analysis.md — "Calls", "Cause paths", "Bounded precision".
func escapeSummaryUsesRoot(summary localReferenceOrigin, root string, depth int) bool {
	if depth >= 64 {
		return false
	}
	if summary.HasPlace && summary.Place.Root == root {
		return true
	}
	for _, place := range summary.Places {
		if place.Root == root {
			return true
		}
	}
	for _, child := range summary.Contained {
		if escapeSummaryUsesRoot(child, root, depth+1) {
			return true
		}
	}
	return false
}

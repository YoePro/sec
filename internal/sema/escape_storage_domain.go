package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// storageDomainDependency is one borrowed dependency carried by a value that
// is stored into static or thread-local storage: the root binding the
// reference points into and that root's storage origin. Unknown marks a
// dependency whose referent the compiler cannot identify.
type storageDomainDependency struct {
	name    string
	token   lexer.Token
	storage StorageOrigin
	place   Place
	placed  bool
	unknown bool
}

// checkStorageDomainStore handles an assignment whose target is rooted in
// Static or ThreadLocal storage. Every borrowed dependency carried by the
// stored value is recorded as an escape to that storage domain; the store is
// valid only when every referent is itself static storage, because the
// destination outlives every invocation. Owned values that carry no
// reference are ordinary value transfers except execution-bound guards, which
// cannot be retained in shared storage even when they own no references.
//
// Rules:
//   - rules/analysis/escape_analysis.md — "Static and thread-local destinations", "StaticStorage / ThreadLocalStorage", "Canonical sinks" (StaticStore, ThreadLocalStore), "Diagnostic categories"
//   - rules/memory/lifetime_analysis.md — § 19 "Static, global, and thread-local storage"
//   - rules/memory/references.md — § 7 "Lifetime relationship"
//   - rules/concurrency/mutex.md — §39(5)
func (a *Analyzer) checkStorageDomainStore(target ast.Expression, targetType Type, value ast.Expression, valueType Type) bool {
	root, ok := borrowRootName(target)
	if !ok {
		return false
	}
	symbol, ok := a.symbols[root]
	if !ok || (symbol.Storage != StorageOriginStatic && symbol.Storage != StorageOriginThreadLocal) {
		return false
	}
	destination, id, domain := EscapeDestinationStaticStorage, diagnostics.StaticStoreOfShortLivedReference, "static"
	if symbol.Storage == StorageOriginThreadLocal {
		destination, id, domain = EscapeDestinationThreadLocalStorage, diagnostics.ThreadLocalStoreOfShortLivedReference, "thread-local"
	}
	if a.checkMutexGuardEscape(value, valueType, expressionToken(target), domain+" storage", destination) {
		return true
	}
	if !typeContainsReference(targetType, map[string]bool{}) {
		return false
	}
	dependencies := a.storageDomainDependencies(explicitMoveSource(value), valueType, targetType)
	a.recordStorageDomainEscapeFacts(dependencies, value, valueType, destination)
	for _, dependency := range dependencies {
		if !dependency.unknown && dependency.storage == StorageOriginStatic {
			continue
		}
		if dependency.unknown {
			a.reportEscapeAssignmentDiagnostic(target, value, dependency.token, id,
				"cannot store a reference of unknown origin into %s storage %s", domain, root)
			return true
		}
		before := len(a.errors)
		a.reportEscapeAssignmentDiagnostic(target, value, dependency.token, id,
			"cannot store reference to %s into %s storage %s", dependency.name, domain, root)
		if a.isCurrentParameter(dependency.name) && len(a.errors) > before {
			if causes := a.errors[len(a.errors)-1].EscapeCauses; len(causes) > 0 && len(causes[0].Steps) > 0 && causes[0].Steps[0].Kind == "origin" {
				causes[0].Steps[0].Message = "origin is a borrowed parameter, valid only for this call"
			}
		}
		return true
	}
	return false
}

// storageDomainDependencies lists the borrowed dependencies of value: the
// borrowed Place of ref and slice expressions, the referent of reference
// values (and of an owned Place that initializes a reference target), the
// fields of struct literals, and tracked call-result origins.
func (a *Analyzer) storageDomainDependencies(value ast.Expression, valueType Type, targetType Type) []storageDomainDependency {
	switch expr := value.(type) {
	case *ast.RefExpression:
		return []storageDomainDependency{a.storageDomainReferent(expr.Value)}
	case *ast.SliceExpression:
		return []storageDomainDependency{a.storageDomainReferent(expr)}
	case *ast.StructLiteral:
		var dependencies []storageDomainDependency
		for _, field := range expr.Fields {
			if field == nil || field.Value == nil {
				continue
			}
			fieldType, typed := a.expressionTypes[field.Value]
			if !typed || !typeContainsReference(fieldType, map[string]bool{}) {
				continue
			}
			dependencies = append(dependencies, a.storageDomainDependencies(field.Value, fieldType, fieldType)...)
		}
		return dependencies
	case *ast.CallExpression:
		if origin, tracked := a.expressionReferenceOrigins[expr]; tracked && !origin.Unknown {
			return a.storageDomainOriginDependencies(origin, expressionToken(expr))
		}
		if !typeContainsReference(valueType, map[string]bool{}) {
			return nil
		}
		return []storageDomainDependency{{name: "call result", token: expressionToken(expr), unknown: true}}
	case *ast.Identifier, *ast.MemberExpression, *ast.IndexExpression:
		if typeContainsReference(valueType, map[string]bool{}) || typeCarriesReferenceOrigin(targetType) {
			return []storageDomainDependency{a.storageDomainReferent(expr)}
		}
		return nil
	}
	if !typeContainsReference(valueType, map[string]bool{}) {
		return nil
	}
	return []storageDomainDependency{{name: "value", token: expressionToken(value), unknown: true}}
}

// storageDomainReferent resolves the root storage a borrow or reference
// value points into, following reference holders to their canonical origin.
func (a *Analyzer) storageDomainReferent(expr ast.Expression) storageDomainDependency {
	name, token, _, storage, _ := a.referenceOriginForExpression(expr)
	if name == "" {
		return storageDomainDependency{name: "value", token: expressionToken(expr), unknown: true}
	}
	if token.Line == 0 {
		token = expressionToken(expr)
	}
	dependency := storageDomainDependency{name: name, token: token, storage: storage, unknown: storage == StorageOriginUnknown}
	if place, ok := a.resolvePlace(expr); ok {
		dependency.place, dependency.placed = place, true
	}
	return dependency
}

func (a *Analyzer) isCurrentParameter(name string) bool {
	for _, parameter := range a.currentFunctionMetadata.Parameters {
		if parameter.Name == name {
			return true
		}
	}
	return false
}

func (a *Analyzer) storageDomainOriginDependencies(origin localReferenceOrigin, fallback lexer.Token) []storageDomainDependency {
	places := localOriginPlaces(origin)
	if len(places) == 0 {
		return []storageDomainDependency{{name: origin.Name, token: fallback, unknown: true}}
	}
	dependencies := make([]storageDomainDependency, 0, len(places))
	for _, place := range places {
		dependency := storageDomainDependency{name: place.Root, token: place.RootToken, place: place, placed: true}
		if dependency.token.Line == 0 {
			dependency.token = fallback
		}
		if symbol, ok := a.symbols[place.Root]; ok {
			dependency.storage = symbol.Storage
		} else {
			dependency.unknown = true
		}
		dependencies = append(dependencies, dependency)
	}
	return dependencies
}

// recordStorageDomainEscapeFacts publishes one BorrowEscape fact per stored
// dependency to the storage-domain destination. A borrowed caller parameter
// or receiver stored there is Retained beyond the call.
func (a *Analyzer) recordStorageDomainEscapeFacts(dependencies []storageDomainDependency, value ast.Expression, valueType Type, destination EscapeDestination) {
	if a.summaryPass || a.inLambda || a.escapeAnalysis == nil || a.currentCallable == "" || len(dependencies) == 0 {
		return
	}
	function := a.currentFunctionMetadata
	sources := make([]EscapeSource, 0, len(dependencies))
	unknown := false
	for _, dependency := range dependencies {
		switch {
		case dependency.unknown:
			unknown = true
			sources = append(sources, EscapeSource{Kind: EscapeSourceUnknown, Name: dependency.name, ParameterIndex: -1, Token: dependency.token})
		case dependency.placed:
			sources = append(sources, escapeSourceFromPlace(dependency.place, function, "", dependency.token))
		default:
			source := EscapeSource{Kind: EscapeSourceLocal, Name: dependency.name, ParameterIndex: -1, Token: dependency.token}
			if dependency.storage == StorageOriginStatic {
				source.Kind = EscapeSourceStatic
			}
			sources = append(sources, source)
		}
	}
	for index, dependency := range dependencies {
		if dependency.storage == StorageOriginStatic && sources[index].Kind == EscapeSourceLocal {
			sources[index].Kind = EscapeSourceStatic
		}
	}
	subject := escapeSubjectForType(valueType)
	if subject == EscapeSubjectOwnedValue {
		subject = EscapeSubjectSafeReference
	}
	fact := EscapeFact{
		Callable:    a.currentCallable,
		Subject:     subject,
		Mode:        EscapeModeBorrowEscape,
		Destination: destination,
		Sources:     uniqueEscapeSources(sources),
		Sink:        expressionToken(value),
		Unknown:     unknown,
	}
	e := a.escapeAnalysis
	e.facts = append(e.facts, cloneEscapeFact(fact))
	summary := e.ensureSummary(function, a.currentCallable)
	summary.Unknown = summary.Unknown || unknown
	for _, source := range fact.Sources {
		switch source.Kind {
		case EscapeSourceParameter:
			e.addParameterDisposition(summary, function, source.ParameterIndex, EscapeParameterRetained)
		case EscapeSourceReceiver:
			summary.Receiver = appendEscapeDisposition(summary.Receiver, EscapeParameterRetained)
		}
	}
}

package sema

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// CaptureMode is the source-declared mechanism used to populate a closure
// environment. Reference modes are retained here for the complete canonical
// model even while their source grammar remains a separate integration step.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "CaptureMode"
//   - rules/declarations/lambda-functions.md — §§15–20 "Capture forms"
type CaptureMode string

const (
	CaptureCopy          CaptureMode = "copy-capture"
	CaptureMove          CaptureMode = "move-capture"
	CaptureSharedBorrow  CaptureMode = "shared-borrow"
	CaptureMutableBorrow CaptureMode = "mutable-borrow"
)

// CaptureTransfer records the already-validated transfer into the environment.
// It deliberately does not infer transfer semantics from type copyability.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Transfer"
type CaptureTransfer string

const (
	CaptureTransferCopy    CaptureTransfer = "copy"
	CaptureTransferMove    CaptureTransfer = "move"
	CaptureTransferUnknown CaptureTransfer = "unknown"
)

// CaptureDependencies preserves semantic requirements carried by the captured
// value. Empty fields mean that the corresponding dependency was not present,
// not that later analysis has proved a broader lifetime or storage contract.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Dependencies"
type CaptureDependencies struct {
	ReferentPlaces            []Place
	ReferentUnknown           bool
	Referents                 []CaptureReferentDependency
	Storage                   StorageOrigin
	Generation                int
	ArenaDomain               string
	OwnsResource              bool
	CallableCapability        CallableCapability
	CallableTargets           CallableTargetSet
	CallableEnvironment       AbstractClosureEnvironmentID
	HasCallableEnvironment    bool
	CallableDependenciesKnown bool
}

// CaptureReferentDependency preserves one direct or aggregate-contained
// referent relationship carried into a closure environment. CarrierPath is
// empty for a direct reference value and uses canonical relative paths such as
// `.view` or `[2].item` for nested aggregate storage.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Reference-typed values versus reference capture"
//   - rules/analysis/closure_analysis.md — "Closure environment as dependency carrier"
type CaptureReferentDependency struct {
	CarrierPath    string
	ReferentPlaces []Place
	Unknown        bool
	Mutable        bool
}

// CaptureRecord is the canonical frontend fact for one successfully resolved
// explicit capture. It is keyed by the lambda creation expression rather than
// by the captured spelling, so distinct closure creation sites stay distinct.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Capture record"
//   - rules/declarations/lambda-functions.md — §41 "AST requirements" and §42 "Sema requirements"
//   - rules/corrections/applied/closure-analysis-ownership-v2-correction-20260826.md — "Canonical capture classification" and "Transfer point"
type CaptureRecord struct {
	Name         string
	Source       lexer.Token
	SourcePlace  Place
	CapturedType Type
	Mode         CaptureMode
	Transfer     CaptureTransfer
	// SourceBecomesUnavailable is the source-level availability consequence of
	// a successfully committed explicit move capture.
	SourceBecomesUnavailable bool
	Dependencies             CaptureDependencies
}

// recordLambdaCaptureFacts publishes facts only for captures whose resolution
// and ownership transfer have already succeeded. Invalid and duplicate source
// captures never become compiler-owned semantic facts.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Captures are explicit", "Capture record"
//   - rules/declarations/lambda-functions.md — §42 "Sema requirements"
//   - rules/corrections/applied/closure-analysis-ownership-v2-correction-20260826.md — "Plain capture never silently becomes move capture"
func (a *Analyzer) recordLambdaCaptureFacts(lambda *ast.LambdaExpression) {
	if a == nil || lambda == nil || a.summaryPass {
		return
	}
	delete(a.resolvedLambdaCaptures, lambda)
	records := make([]CaptureRecord, 0, len(lambda.Captures))
	seen := map[string]bool{}
	for _, capture := range lambda.Captures {
		if capture.Name == nil || seen[capture.Name.Value] {
			continue
		}
		seen[capture.Name.Value] = true
		name := capture.Name.Value
		symbol, ok := a.symbols[name]
		if !ok || !symbol.Local || !a.assigned[name] {
			continue
		}

		mode, transfer := CaptureCopy, CaptureTransferCopy
		switch capture.Mode {
		case ast.LambdaCaptureMove:
			mode, transfer = CaptureMove, CaptureTransferMove
			movedAt, moved := a.moved[name]
			if !moved || movedAt.Line != capture.Name.Token.Line || movedAt.Column != capture.Name.Token.Column {
				continue
			}
		default:
			classification := CopyClassificationOf(symbol.Type)
			if classification != CopyTrivial && classification != CopySemantic {
				continue
			}
		}

		place, ok := a.rootPlace(name)
		if !ok {
			continue
		}
		records = append(records, CaptureRecord{
			Name:                     name,
			Source:                   capture.Name.Token,
			SourcePlace:              clonePlace(place),
			CapturedType:             semanticSnapshotType(symbol.Type),
			Mode:                     mode,
			Transfer:                 transfer,
			SourceBecomesUnavailable: transfer == CaptureTransferMove,
			Dependencies:             a.captureDependencies(name, symbol),
		})
	}
	if len(records) != 0 {
		a.resolvedLambdaCaptures[lambda] = records
	}
}

// captureDependencies projects existing canonical provenance, storage, arena,
// destruction, and callable facts into the closure capture record.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Dependencies"
//   - rules/memory/lifetime_analysis.md — §12 "References carried by aggregates"
func (a *Analyzer) captureDependencies(name string, symbol Symbol) CaptureDependencies {
	dependencies := CaptureDependencies{
		Storage:      symbol.Type.ReferenceOriginStorage,
		Generation:   symbol.Type.ReferenceOriginGeneration,
		ArenaDomain:  symbol.Type.ArenaDomainID,
		OwnsResource: !TriviallyDestructible(symbol.Type),
	}
	if symbol.Type.Kind == FunctionType {
		dependencies.CallableCapability = normalizedCallableCapability(symbol.Type.FunctionCapability)
		if symbol.HasCallableIdentity {
			identity := cloneResolvedCallableIdentity(symbol.CallableIdentity)
			dependencies.CallableTargets = identity.Targets
			dependencies.CallableEnvironment = identity.Environment
			dependencies.HasCallableEnvironment = identity.HasEnvironment
			dependencies.CallableDependenciesKnown = true
		}
	}
	if origin, ok := a.localRefContainers[name]; ok {
		dependencies.ReferentPlaces = clonePlaces(localOriginPlaces(origin))
		dependencies.ReferentUnknown = origin.Unknown
		dependencies.Referents = captureReferentDependencies(origin)
	} else if typeCarriesReferenceOrigin(symbol.Type) {
		dependencies.ReferentUnknown = true
		dependencies.Referents = []CaptureReferentDependency{{Unknown: true, Mutable: symbol.Type.ReferenceMutable}}
	}
	return dependencies
}

// recordBoundCallableIdentity transfers an already-resolved callable value
// identity to its local binding. Failure to resolve leaves the binding
// explicitly unknown and never manufactures a NoEnvironment proof.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Captured function values"
//   - rules/analysis/closure_analysis.md — "Soundness of target sets"
func (a *Analyzer) recordBoundCallableIdentity(name string, expression ast.Expression) {
	symbol, ok := a.symbols[name]
	if !ok {
		return
	}
	symbol.CallableIdentity = ResolvedCallableIdentity{}
	symbol.HasCallableIdentity = false
	if symbol.Type.Kind == FunctionType {
		if identity, known := a.callableIdentityForExpression(expression); known {
			symbol.CallableIdentity = cloneResolvedCallableIdentity(identity)
			symbol.HasCallableIdentity = true
		}
	}
	a.symbols[name] = symbol
	if completion, exists := a.completionSymbols[name]; exists && completion.Token == symbol.Token {
		completion.CallableIdentity = cloneResolvedCallableIdentity(symbol.CallableIdentity)
		completion.HasCallableIdentity = symbol.HasCallableIdentity
		a.completionSymbols[name] = completion
	}
}

// callableIdentityForExpression reads only identities established by normal
// expression resolution or by a previously proven local callable binding.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Callable value flow"
//   - rules/analysis/closure_analysis.md — "Exact"
func (a *Analyzer) callableIdentityForExpression(expression ast.Expression) (ResolvedCallableIdentity, bool) {
	if identity, ok := a.resolvedCallableIdentities[expression]; ok {
		return cloneResolvedCallableIdentity(identity), true
	}
	identifier, ok := expression.(*ast.Identifier)
	if !ok {
		return ResolvedCallableIdentity{}, false
	}
	symbol, ok := a.symbols[identifier.Value]
	if !ok || !symbol.HasCallableIdentity {
		return ResolvedCallableIdentity{}, false
	}
	return cloneResolvedCallableIdentity(symbol.CallableIdentity), true
}

// captureReferentDependencies flattens canonical aggregate provenance into a
// deterministic carrier-path list without discarding Place alternatives.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Views and slices"
//   - rules/analysis/closure_analysis.md — "Closure environment as dependency carrier"
//   - rules/memory/lifetime_analysis.md — §12 "References carried by aggregates"
func captureReferentDependencies(origin localReferenceOrigin) []CaptureReferentDependency {
	var dependencies []CaptureReferentDependency
	appendCaptureReferentDependencies(&dependencies, origin, "")
	return dependencies
}

// appendCaptureReferentDependencies recursively preserves nested carrier paths
// in lexical path order for deterministic analysis snapshots.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Closure environment as dependency carrier"
func appendCaptureReferentDependencies(out *[]CaptureReferentDependency, origin localReferenceOrigin, carrierPath string) {
	paths := make([]string, 0, len(origin.Contained))
	for path := range origin.Contained {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		appendCaptureReferentDependencies(out, origin.Contained[path], carrierPath+path)
	}
	places := localOriginPlaces(origin)
	if len(places) != 0 || origin.Unknown && len(origin.Contained) == 0 {
		*out = append(*out, CaptureReferentDependency{
			CarrierPath: carrierPath, ReferentPlaces: clonePlaces(places), Unknown: origin.Unknown, Mutable: origin.Mutable,
		})
	}
}

// defineLambdaCaptures installs validated environment bindings for lambda body
// analysis. Capture facts are recorded before this isolated symbol scope is
// entered, while their source Places still denote the enclosing function.
//
// Rules:
//   - rules/declarations/lambda-functions.md — §§13–21 explicit captures
//   - rules/analysis/closure_analysis.md — "Captures are explicit"
func (a *Analyzer) defineLambdaCaptures(expr *ast.LambdaExpression, outerSymbols map[string]Symbol, outerAssigned map[string]bool) {
	seen := map[string]lexer.Token{}
	for _, capture := range expr.Captures {
		if capture.Name == nil {
			continue
		}
		name := capture.Name.Value
		if _, exists := seen[name]; exists {
			a.addErrorAtToken(capture.Name.Token, "duplicate capture %s", name)
			continue
		}
		seen[name] = capture.Name.Token
		symbol, ok := outerSymbols[name]
		if !ok {
			if symbol, exists := a.symbols[name]; exists && !symbol.Local {
				a.addErrorAtToken(capture.Name.Token, "cannot capture non-local declaration %s; reference it directly", name)
			} else {
				a.addErrorAtToken(capture.Name.Token, "undefined capture %s", name)
			}
			continue
		}
		if assigned, exists := outerAssigned[name]; exists && !assigned {
			a.addErrorAtToken(capture.Name.Token, "cannot capture unassigned variable %s", name)
			continue
		}
		symbol.Mutable = false
		a.symbols[name] = symbol
		a.assigned[name] = true
		delete(a.constInts, name)
		a.recordCaptureEscapeFact(name, capture.Name.Token, symbol)
	}
}

// ResolvedLambdaCapturesOf returns a defensive snapshot of the capture facts
// recorded during completed semantic analysis. It performs no inference.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Capture record"
//   - rules/compiler/compiler_analysis.md — immutable analysis results
func (a *Analyzer) ResolvedLambdaCapturesOf(lambda *ast.LambdaExpression) ([]CaptureRecord, bool) {
	if a == nil || lambda == nil {
		return nil, false
	}
	records, ok := a.resolvedLambdaCaptures[lambda]
	if !ok {
		return nil, false
	}
	return cloneCaptureRecords(records), true
}

// cloneCaptureRecords recursively detaches Place, type, and dependency data
// before capture facts cross the Analyzer snapshot boundary.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Capture record"
//   - rules/compiler/compiler_analysis.md — immutable analysis results
func cloneCaptureRecords(records []CaptureRecord) []CaptureRecord {
	cloned := make([]CaptureRecord, len(records))
	for index, record := range records {
		cloned[index] = record
		cloned[index].SourcePlace = clonePlace(record.SourcePlace)
		cloned[index].CapturedType = semanticSnapshotType(record.CapturedType)
		cloned[index].Dependencies.ReferentPlaces = clonePlaces(record.Dependencies.ReferentPlaces)
		cloned[index].Dependencies.Referents = cloneCaptureReferentDependencies(record.Dependencies.Referents)
		cloned[index].Dependencies.CallableTargets = cloneCallableTargetSet(record.Dependencies.CallableTargets)
	}
	return cloned
}

// cloneCaptureReferentDependencies detaches nested Place alternatives before
// capture dependencies cross the Analyzer snapshot boundary.
//
// Rules:
//   - rules/compiler/compiler_analysis.md — immutable analysis results
func cloneCaptureReferentDependencies(dependencies []CaptureReferentDependency) []CaptureReferentDependency {
	cloned := make([]CaptureReferentDependency, len(dependencies))
	for index, dependency := range dependencies {
		cloned[index] = dependency
		cloned[index].ReferentPlaces = clonePlaces(dependency.ReferentPlaces)
	}
	return cloned
}

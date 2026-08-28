package sema

import (
	"sort"
	"strconv"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// EscapeSubjectKind classifies the semantic entity whose availability crosses
// a boundary. It deliberately says nothing about physical storage placement.
type EscapeSubjectKind string

const (
	EscapeSubjectOwnedValue          EscapeSubjectKind = "owned-value"
	EscapeSubjectSafeReference       EscapeSubjectKind = "safe-reference"
	EscapeSubjectView                EscapeSubjectKind = "view"
	EscapeSubjectRawPointer          EscapeSubjectKind = "raw-pointer"
	EscapeSubjectCallableEnvironment EscapeSubjectKind = "callable-environment"
	EscapeSubjectHandle              EscapeSubjectKind = "handle"
	EscapeSubjectStorageDependency   EscapeSubjectKind = "storage-dependency"
	EscapeSubjectUnknown             EscapeSubjectKind = "unknown"
)

type EscapeMode string

const (
	EscapeModeValueTransfer       EscapeMode = "value-transfer"
	EscapeModeBorrowEscape        EscapeMode = "borrow-escape"
	EscapeModeAddressEscape       EscapeMode = "address-escape"
	EscapeModeRetention           EscapeMode = "retention"
	EscapeModeCapture             EscapeMode = "capture"
	EscapeModeConcurrencyTransfer EscapeMode = "concurrency-transfer"
	EscapeModeForeignRetention    EscapeMode = "foreign-retention"
	EscapeModeUnknown             EscapeMode = "unknown"
)

type EscapeDestination string

const (
	EscapeDestinationCaller             EscapeDestination = "caller"
	EscapeDestinationOuterPlace         EscapeDestination = "outer-place"
	EscapeDestinationAggregate          EscapeDestination = "aggregate"
	EscapeDestinationRetainingCall      EscapeDestination = "retaining-call"
	EscapeDestinationClosureEnvironment EscapeDestination = "closure-environment"
	EscapeDestinationTask               EscapeDestination = "task"
	EscapeDestinationThread             EscapeDestination = "thread"
	EscapeDestinationStaticStorage      EscapeDestination = "static-storage"
	EscapeDestinationThreadLocalStorage EscapeDestination = "thread-local-storage"
	EscapeDestinationForeignCode        EscapeDestination = "foreign-code"
	EscapeDestinationReturnedValue      EscapeDestination = "returned-value"
	EscapeDestinationUnknown            EscapeDestination = "unknown"
)

type EscapeSourceKind string

const (
	EscapeSourceParameter              EscapeSourceKind = "parameter"
	EscapeSourceReceiver               EscapeSourceKind = "receiver"
	EscapeSourceLocal                  EscapeSourceKind = "local"
	EscapeSourceStatic                 EscapeSourceKind = "static"
	EscapeSourceThreadLocal            EscapeSourceKind = "thread-local"
	EscapeSourceArenaStorage           EscapeSourceKind = "arena-storage"
	EscapeSourceAllocatorBackedStorage EscapeSourceKind = "allocator-backed-storage"
	EscapeSourceForeignStorage         EscapeSourceKind = "foreign-storage"
	EscapeSourceCapturedValue          EscapeSourceKind = "captured-value"
	EscapeSourceReturnedCallResult     EscapeSourceKind = "returned-call-result"
	EscapeSourceTemporary              EscapeSourceKind = "temporary"
	EscapeSourceUnknown                EscapeSourceKind = "unknown"
)

// EscapeSource retains canonical Place provenance. ParameterIndex is -1 for
// every source that is not a function parameter.
type EscapeSource struct {
	Kind           EscapeSourceKind
	Name           string
	ParameterIndex int
	Place          Place
	CarrierPath    string
	Token          lexer.Token
}

// EscapeFact is historical analysis evidence. Facts accumulate even when a
// later lifetime or ownership check rejects the corresponding operation.
type EscapeFact struct {
	Callable    CallableID
	Subject     EscapeSubjectKind
	Mode        EscapeMode
	Destination EscapeDestination
	Sources     []EscapeSource
	Sink        lexer.Token
	Unknown     bool
}

type EscapeParameterDisposition string

const (
	EscapeParameterNoEscape                EscapeParameterDisposition = "no-escape"
	EscapeParameterReturned                EscapeParameterDisposition = "returned"
	EscapeParameterRetained                EscapeParameterDisposition = "retained"
	EscapeParameterStoredInEscapingCarrier EscapeParameterDisposition = "stored-in-escaping-carrier"
	EscapeParameterOwnershipTransferred    EscapeParameterDisposition = "ownership-transferred"
	EscapeParameterCaptured                EscapeParameterDisposition = "captured"
	EscapeParameterTransferredToTask       EscapeParameterDisposition = "transferred-to-task"
	EscapeParameterTransferredToThread     EscapeParameterDisposition = "transferred-to-thread"
	EscapeParameterPassedToForeign         EscapeParameterDisposition = "passed-to-foreign"
	EscapeParameterUnknownRetention        EscapeParameterDisposition = "unknown-retention"
)

type EscapeParameterSummary struct {
	Index        int
	Name         string
	Dispositions []EscapeParameterDisposition
}

type EscapeCallableSummary struct {
	Callable     CallableID
	Name         string
	Declaration  lexer.Token
	Parameters   []EscapeParameterSummary
	Receiver     []EscapeParameterDisposition
	ReturnFacts  []EscapeFact
	CaptureFacts []EscapeFact
	Unknown      bool
}

// EscapeAnalysis is an immutable snapshot when returned by Analyzer. Mutable
// construction methods remain package-private and are used only during Sema.
type EscapeAnalysis struct {
	facts        []EscapeFact
	summaries    map[CallableID]*EscapeCallableSummary
	summaryOrder []CallableID
}

func newEscapeAnalysis() *EscapeAnalysis {
	return &EscapeAnalysis{summaries: map[CallableID]*EscapeCallableSummary{}}
}

func (e *EscapeAnalysis) clone() *EscapeAnalysis {
	copyAnalysis := newEscapeAnalysis()
	if e == nil {
		return copyAnalysis
	}
	copyAnalysis.facts = cloneEscapeFacts(e.facts)
	copyAnalysis.summaryOrder = append([]CallableID(nil), e.summaryOrder...)
	for id, summary := range e.summaries {
		cloned := cloneEscapeCallableSummary(*summary)
		copyAnalysis.summaries[id] = &cloned
	}
	return copyAnalysis
}

func (e *EscapeAnalysis) Facts() []EscapeFact {
	if e == nil {
		return nil
	}
	return cloneEscapeFacts(e.facts)
}

func (e *EscapeAnalysis) Summaries() []EscapeCallableSummary {
	if e == nil {
		return nil
	}
	result := make([]EscapeCallableSummary, 0, len(e.summaryOrder))
	for _, id := range e.summaryOrder {
		if summary := e.summaries[id]; summary != nil {
			result = append(result, cloneEscapeCallableSummary(*summary))
		}
	}
	return result
}

func (e *EscapeAnalysis) Summary(id CallableID) (EscapeCallableSummary, bool) {
	if e == nil {
		return EscapeCallableSummary{}, false
	}
	summary, ok := e.summaries[id]
	if !ok || summary == nil {
		return EscapeCallableSummary{}, false
	}
	return cloneEscapeCallableSummary(*summary), true
}

func (e *EscapeAnalysis) SummariesForDeclaration(token lexer.Token) []EscapeCallableSummary {
	if e == nil {
		return nil
	}
	result := []EscapeCallableSummary{}
	for _, summary := range e.Summaries() {
		if sameSourceToken(summary.Declaration, token) {
			result = append(result, summary)
		}
	}
	return result
}

func (e *EscapeAnalysis) addReturn(function Function, callable CallableID, returnType Type, sink lexer.Token, raw localReferenceOrigin, symbolic localReferenceOrigin) {
	if e == nil || callable == "" {
		return
	}
	fact := EscapeFact{
		Callable:    callable,
		Subject:     escapeSubjectForType(returnType),
		Mode:        escapeReturnMode(returnType),
		Destination: EscapeDestinationCaller,
		Sources:     escapeSourcesFromOrigin(raw, function),
		Sink:        sink,
		Unknown:     raw.Unknown,
	}
	if len(fact.Sources) == 0 {
		fact.Sources = []EscapeSource{{Kind: EscapeSourceTemporary, ParameterIndex: -1, Token: sink}}
	}
	e.facts = append(e.facts, cloneEscapeFact(fact))
	summary := e.ensureSummary(function, callable)
	summary.ReturnFacts = append(summary.ReturnFacts, cloneEscapeFact(fact))
	summary.Unknown = summary.Unknown || symbolic.Unknown
	for _, source := range escapeSourcesFromOrigin(symbolic, function) {
		switch source.Kind {
		case EscapeSourceParameter:
			e.addParameterDisposition(summary, function, source.ParameterIndex, EscapeParameterReturned)
		case EscapeSourceReceiver:
			summary.Receiver = appendEscapeDisposition(summary.Receiver, EscapeParameterReturned)
		case EscapeSourceUnknown:
			summary.Unknown = true
		}
	}
}

func (e *EscapeAnalysis) addCapture(function Function, callable CallableID, source EscapeSource, sink lexer.Token, typ Type) {
	if e == nil || callable == "" {
		return
	}
	fact := EscapeFact{
		Callable:    callable,
		Subject:     escapeSubjectForType(typ),
		Mode:        EscapeModeCapture,
		Destination: EscapeDestinationClosureEnvironment,
		Sources:     []EscapeSource{cloneEscapeSource(source)},
		Sink:        sink,
		Unknown:     source.Kind == EscapeSourceUnknown,
	}
	e.facts = append(e.facts, cloneEscapeFact(fact))
	summary := e.ensureSummary(function, callable)
	summary.CaptureFacts = append(summary.CaptureFacts, cloneEscapeFact(fact))
	summary.Unknown = summary.Unknown || fact.Unknown
	if source.Kind == EscapeSourceParameter {
		e.addParameterDisposition(summary, function, source.ParameterIndex, EscapeParameterCaptured)
	} else if source.Kind == EscapeSourceReceiver {
		summary.Receiver = appendEscapeDisposition(summary.Receiver, EscapeParameterCaptured)
	}
}

func (e *EscapeAnalysis) addOuterPlaceEscape(function Function, callable CallableID, source EscapeSource, sink lexer.Token, typ Type) {
	if e == nil || callable == "" {
		return
	}
	fact := EscapeFact{
		Callable:    callable,
		Subject:     escapeSubjectForType(typ),
		Mode:        EscapeModeBorrowEscape,
		Destination: EscapeDestinationOuterPlace,
		Sources:     []EscapeSource{cloneEscapeSource(source)},
		Sink:        sink,
		Unknown:     source.Kind == EscapeSourceUnknown,
	}
	e.facts = append(e.facts, cloneEscapeFact(fact))
	summary := e.ensureSummary(function, callable)
	summary.Unknown = summary.Unknown || fact.Unknown
}

func (e *EscapeAnalysis) ensureSummary(function Function, callable CallableID) *EscapeCallableSummary {
	if summary := e.summaries[callable]; summary != nil {
		return summary
	}
	summary := &EscapeCallableSummary{Callable: callable, Name: function.Name, Declaration: function.Token}
	e.summaries[callable] = summary
	e.summaryOrder = append(e.summaryOrder, callable)
	return summary
}

func (e *EscapeAnalysis) addParameterDisposition(summary *EscapeCallableSummary, function Function, index int, disposition EscapeParameterDisposition) {
	if summary == nil {
		return
	}
	if index < 0 || index >= len(function.Parameters) {
		summary.Unknown = true
		return
	}
	for i := range summary.Parameters {
		if summary.Parameters[i].Index == index {
			summary.Parameters[i].Dispositions = appendEscapeDisposition(summary.Parameters[i].Dispositions, disposition)
			return
		}
	}
	summary.Parameters = append(summary.Parameters, EscapeParameterSummary{
		Index:        index,
		Name:         function.Parameters[index].Name,
		Dispositions: []EscapeParameterDisposition{disposition},
	})
	sort.Slice(summary.Parameters, func(i, j int) bool { return summary.Parameters[i].Index < summary.Parameters[j].Index })
}

func escapeSubjectForType(typ Type) EscapeSubjectKind {
	switch typ.Kind {
	case ReferenceType:
		return EscapeSubjectSafeReference
	case SliceType:
		return EscapeSubjectView
	case RawPtrType:
		return EscapeSubjectRawPointer
	case FunctionType:
		return EscapeSubjectCallableEnvironment
	case InvalidType:
		return EscapeSubjectUnknown
	default:
		if typeContainsReference(typ, map[string]bool{}) {
			return EscapeSubjectStorageDependency
		}
		return EscapeSubjectOwnedValue
	}
}

func escapeReturnMode(typ Type) EscapeMode {
	switch escapeSubjectForType(typ) {
	case EscapeSubjectSafeReference, EscapeSubjectView, EscapeSubjectStorageDependency:
		return EscapeModeBorrowEscape
	case EscapeSubjectRawPointer:
		return EscapeModeAddressEscape
	case EscapeSubjectUnknown:
		return EscapeModeUnknown
	default:
		return EscapeModeValueTransfer
	}
}

func escapeSourcesFromOrigin(origin localReferenceOrigin, function Function) []EscapeSource {
	sources := []EscapeSource{}
	appendOriginEscapeSources(&sources, origin, function, "")
	return uniqueEscapeSources(sources)
}

func appendOriginEscapeSources(out *[]EscapeSource, origin localReferenceOrigin, function Function, carrierPath string) {
	for _, place := range localOriginPlaces(origin) {
		*out = append(*out, escapeSourceFromPlace(place, function, carrierPath, origin.Token))
	}
	paths := make([]string, 0, len(origin.Contained))
	for path := range origin.Contained {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		child := origin.Contained[path]
		appendOriginEscapeSources(out, child, function, carrierPath+path)
	}
	if origin.Unknown && len(localOriginPlaces(origin)) == 0 && len(origin.Contained) == 0 {
		*out = append(*out, EscapeSource{Kind: EscapeSourceUnknown, ParameterIndex: -1, CarrierPath: carrierPath, Token: origin.Token})
	}
}

func escapeSourceFromPlace(place Place, function Function, carrierPath string, fallback lexer.Token) EscapeSource {
	source := EscapeSource{Kind: EscapeSourceLocal, Name: place.Root, ParameterIndex: -1, Place: cloneEscapePlace(place), CarrierPath: carrierPath, Token: place.RootToken}
	if source.Token.Line == 0 {
		source.Token = fallback
	}
	if strings.HasPrefix(place.Root, "$param:") {
		index, err := strconv.Atoi(strings.TrimPrefix(place.Root, "$param:"))
		if err != nil {
			source.Kind = EscapeSourceUnknown
			return source
		}
		source.Kind = EscapeSourceParameter
		source.ParameterIndex = index
		if index >= 0 && index < len(function.Parameters) {
			source.Name = function.Parameters[index].Name
			source.Token = function.Parameters[index].Token
		}
		return source
	}
	if place.Root == "$receiver" || place.Root == "self" {
		source.Kind = EscapeSourceReceiver
		source.Name = "self"
		return source
	}
	if strings.HasPrefix(place.Root, "$static:") {
		source.Kind = EscapeSourceStatic
		source.Name = strings.TrimPrefix(place.Root, "$static:")
		return source
	}
	for index, parameter := range function.Parameters {
		if place.Root == parameter.Name {
			source.Kind = EscapeSourceParameter
			source.ParameterIndex = index
			source.Token = parameter.Token
			return source
		}
	}
	return source
}

func uniqueEscapeSources(sources []EscapeSource) []EscapeSource {
	result := []EscapeSource{}
	seen := map[string]bool{}
	for _, source := range sources {
		key := string(source.Kind) + "|" + strconv.Itoa(source.ParameterIndex) + "|" + source.Name + "|" + source.Place.String() + "|" + source.CarrierPath
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, source)
	}
	return result
}

func appendEscapeDisposition(in []EscapeParameterDisposition, disposition EscapeParameterDisposition) []EscapeParameterDisposition {
	for _, existing := range in {
		if existing == disposition {
			return in
		}
	}
	return append(in, disposition)
}

func cloneEscapeSource(source EscapeSource) EscapeSource {
	source.Place = cloneEscapePlace(source.Place)
	return source
}

// Escape snapshots retain Place identity and the stable nominal type surface,
// but do not expose mutable maps or pointer-rich compiler type internals.
func cloneEscapePlace(place Place) Place {
	place = clonePlace(place)
	place.Type = escapeSnapshotType(place.Type)
	for index := range place.AlternativeOrigins {
		place.AlternativeOrigins[index].Type = escapeSnapshotType(place.AlternativeOrigins[index].Type)
	}
	return place
}

func escapeSnapshotType(typ Type) Type {
	return Type{
		Name:                  typ.Name,
		Module:                typ.Module,
		Kind:                  typ.Kind,
		Named:                 typ.Named,
		Declared:              typ.Declared,
		Intrinsic:             typ.Intrinsic,
		Underlying:            typ.Underlying,
		Unit:                  typ.Unit,
		ReferenceMutable:      typ.ReferenceMutable,
		ArrayShape:            typ.ArrayShape,
		ArrayLengthDecimal:    typ.ArrayLengthDecimal,
		ArrayLength:           typ.ArrayLength,
		EventCapacity:         typ.EventCapacity,
		EventCapacitySet:      typ.EventCapacitySet,
		RegisterWidth:         typ.RegisterWidth,
		ExplicitlyNonCopyable: typ.ExplicitlyNonCopyable,
		NoCopyPolicyOrigin:    typ.NoCopyPolicyOrigin,
	}
}

func cloneEscapeFact(fact EscapeFact) EscapeFact {
	fact.Sources = append([]EscapeSource(nil), fact.Sources...)
	for index := range fact.Sources {
		fact.Sources[index] = cloneEscapeSource(fact.Sources[index])
	}
	return fact
}

func cloneEscapeFacts(facts []EscapeFact) []EscapeFact {
	result := make([]EscapeFact, len(facts))
	for index, fact := range facts {
		result[index] = cloneEscapeFact(fact)
	}
	return result
}

func cloneEscapeCallableSummary(summary EscapeCallableSummary) EscapeCallableSummary {
	summary.Parameters = append([]EscapeParameterSummary(nil), summary.Parameters...)
	for index := range summary.Parameters {
		summary.Parameters[index].Dispositions = append([]EscapeParameterDisposition(nil), summary.Parameters[index].Dispositions...)
	}
	summary.Receiver = append([]EscapeParameterDisposition(nil), summary.Receiver...)
	summary.ReturnFacts = cloneEscapeFacts(summary.ReturnFacts)
	summary.CaptureFacts = cloneEscapeFacts(summary.CaptureFacts)
	return summary
}

func sameSourceToken(left, right lexer.Token) bool {
	return left.File == right.File && left.Line == right.Line && left.Column == right.Column
}

func (a *Analyzer) ownedReturnEscapeOrigins(expr ast.Expression) (localReferenceOrigin, localReferenceOrigin) {
	place, ok := a.resolvePlace(expr)
	if !ok {
		return localReferenceOrigin{}, localReferenceOrigin{}
	}
	raw := localOriginWithPlaces(localReferenceOrigin{Name: place.Root, Token: place.RootToken}, []Place{place})
	symbolic := cloneLocalReferenceOrigin(raw)
	if root, ok := a.symbolicFunctionOriginRoot(place.Root); ok {
		for index := range symbolic.Places {
			symbolic.Places[index].Root = root
		}
		if symbolic.HasPlace {
			symbolic.Place.Root = root
		}
		return raw, symbolic
	}
	// Returning a local owned value is a fresh value transfer, not an unknown
	// retained dependency on the callee's automatic storage.
	return raw, localReferenceOrigin{}
}

func (a *Analyzer) recordReturnEscapeFact(returnType Type, expr ast.Expression, raw localReferenceOrigin, symbolic localReferenceOrigin) {
	// Lambda callable identity is owned by closure analysis and is not yet part
	// of the canonical call graph. Do not attribute lambda returns to the
	// lexically enclosing function.
	if a == nil || a.summaryPass || a.inLambda || a.escapeAnalysis == nil || a.currentCallable == "" {
		return
	}
	a.escapeAnalysis.addReturn(a.currentFunctionMetadata, a.currentCallable, returnType, expressionToken(expr), raw, symbolic)
}

func (a *Analyzer) recordCaptureEscapeFact(name string, sink lexer.Token, symbol Symbol) {
	if a == nil || a.summaryPass || a.escapeAnalysis == nil || a.currentCallable == "" {
		return
	}
	place, ok := a.rootPlace(name)
	if !ok {
		a.escapeAnalysis.addCapture(a.currentFunctionMetadata, a.currentCallable, EscapeSource{
			Kind: EscapeSourceUnknown, Name: name, ParameterIndex: -1, Token: symbol.Token,
		}, sink, symbol.Type)
		return
	}
	source := escapeSourceFromPlace(place, a.currentFunctionMetadata, "", symbol.Token)
	a.escapeAnalysis.addCapture(a.currentFunctionMetadata, a.currentCallable, source, sink, symbol.Type)
}

func (a *Analyzer) recordOuterPlaceEscapeFact(value ast.Expression, originName string, originToken lexer.Token) {
	if a == nil || a.summaryPass || a.escapeAnalysis == nil || a.currentCallable == "" {
		return
	}
	typ, ok := a.expressionTypes[value]
	if !ok {
		typ = Type{Kind: InvalidType}
	}
	source := EscapeSource{Kind: EscapeSourceLocal, Name: originName, ParameterIndex: -1, Token: originToken}
	if place, ok := a.rootPlace(originName); ok {
		source = escapeSourceFromPlace(place, a.currentFunctionMetadata, "", originToken)
	}
	a.escapeAnalysis.addOuterPlaceEscape(a.currentFunctionMetadata, a.currentCallable, source, expressionToken(value), typ)
}

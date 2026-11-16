package semantic

import (
	"strings"
	"testing"

	"sec/internal/sema"
)

// dynamicHolderDefer builds, through the builder's own helpers, the shape a
// defer reading `*r` of a dynamic-epoch reference parameter lowers to: the
// defer captures the holder's own Place; the body reads the holder out on
// entry (place.read holder), validates it with ref.is-valid and only then
// dereferences it. Arena references are the only dynamic-epoch source and
// reach Semantic IR with P19 (codex-p19-arena-references-task.md); until
// then this is the white-box form of P17-41's last capture kind.
func dynamicHolderDefer(t *testing.T, validateAtRegistration bool) (*Module, *Function) {
	t.Helper()
	module, err := analyzedModule(t, referenceUseFixture, 17)
	if err != nil {
		t.Fatal(err)
	}
	var referenceType, pairType TypeID
	for id := TypeID(1); ; id++ {
		typ, ok := module.Types.Lookup(id)
		if !ok {
			break
		}
		if typ.Kind == TypeReference && !typ.Mutable {
			referenceType, pairType = id, typ.Element
		}
	}
	definition, found := structDefinition(module, pairType)
	if !found || len(definition.Fields) == 0 {
		t.Fatal("no Pair definition")
	}
	intType := definition.Fields[0].Type
	holderPlace := module.Types.Intern(Type{Kind: TypePlace, Element: referenceType})
	pairPlace := module.Types.Intern(Type{Kind: TypePlace, Element: pairType})
	intPlace := module.Types.Intern(Type{Kind: TypePlace, Element: intType})
	metadata := &ReferenceMetadata{Borrow: 1, Lifetime: 1, Validity: ReferenceDynamicEpoch, InvalidationDomain: 1, EpochDependency: 1, AddressSpace: AddressSpaceOrdinary, AddressStability: "Stable", Provenance: "arena"}
	function := &Function{ID: "main::deferred(ref Pair)", Name: "deferred", ReturnType: module.Types.Intern(Type{Kind: TypeVoid, Name: "void"})}
	parameter := Value{ID: 1, Type: referenceType, Ownership: OwnershipSharedReference}
	function.Parameters = []Parameter{{Name: "r", Value: parameter, Reference: metadata}}
	module.Functions = append(module.Functions, function)
	fb := &functionBuilder{owner: &builder{module: module, maxPackage: 17}, fn: function, nextValue: 2, bindings: map[sema.BindingID]binding{}}
	fb.current = fb.newBlock()
	function.Entry = fb.current.ID
	holder := builtValue{id: parameter.ID, typ: referenceType}
	place := PlaceMetadata{Root: 1, RootKind: PlaceRootKind(sema.PlaceRootParameter), Storage: 1, AddressSpace: AddressSpaceOrdinary}

	var capture builtValue
	var captureType TypeID
	if validateAtRegistration {
		guard, err := fb.validateReference(holder, metadata, Location{})
		if err != nil {
			t.Fatal(err)
		}
		capture = fb.placeResult(Operation{Kind: OpReferenceDerefPlace, Operands: []ValueID{holder.id}, Reference: metadata, ReferenceGuard: guard}, pairPlace)
		captureType = pairPlace
	} else {
		capture = fb.placeResult(Operation{Kind: OpPlaceValueRoot, Operands: []ValueID{holder.id}, Place: &place}, holderPlace)
		captureType = holderPlace
	}

	outerBlocks, outerCurrent := function.Blocks, fb.current
	function.Blocks = nil
	entry := fb.newBlock()
	parameterPlace := fb.newValue(captureType, OwnershipCompilerTemporary, Location{})
	entry.Parameters = []Value{parameterPlace}
	fb.current = entry
	referent := builtValue{id: parameterPlace.ID, typ: pairPlace}
	if !validateAtRegistration {
		read := fb.newValue(referenceType, OwnershipSharedReference, Location{})
		facts := cloneReferenceMetadata(metadata)
		fb.emit(Operation{Kind: OpPlaceRead, Operands: []ValueID{parameterPlace.ID}, Results: []Value{read}, PlaceReadMode: PlaceReadHolder, Reference: facts})
		fb.recordReference(read.ID, facts)
		guard, err := fb.validateReference(builtValue{id: read.ID, typ: referenceType}, facts, Location{})
		if err != nil || guard == 0 {
			t.Fatalf("validate in body = %d, %v", guard, err)
		}
		referent = fb.placeResult(Operation{Kind: OpReferenceDerefPlace, Operands: []ValueID{read.ID}, Reference: facts, ReferenceGuard: guard}, pairPlace)
	}
	field := fb.placeResult(Operation{Kind: OpPlaceField, Operands: []ValueID{referent.id}, StructField: 0, Place: &place}, intPlace)
	fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{field.id}}, intType)
	fb.emit(Operation{Kind: OpCleanupDeferYield, Cleanup: &CleanupFacts{}})
	region := &Region{Entry: entry.ID, Blocks: function.Blocks}
	function.Blocks, fb.current = outerBlocks, outerCurrent
	fb.emit(Operation{Kind: OpCleanupDeferRegister, Operands: []ValueID{capture.id}, Region: region, Cleanup: &CleanupFacts{Action: 1, Scope: 1, Ordinal: 1, Defer: 1}})
	fb.emit(Operation{Kind: OpCleanupRunFunction, Cleanup: &CleanupFacts{Exit: "fallthrough"}})
	fb.emit(Operation{Kind: OpReturn})
	return module, function
}

// P17-41: a defer capturing a dynamic-epoch reference holder validates the
// holder when its body runs. The verifier accepts the holder-Place capture
// with an in-body ref.is-valid and rejects validating at registration, a
// holder read escaping as a value and a holder read of a proven reference.
// Rules: rules/control-flow/defer.md — §§ 9–11; rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 25–31.
func TestDeferDynamicEpochHolderValidatesWhenTheBodyRuns(t *testing.T) {
	module, _ := dynamicHolderDefer(t, false)
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	register := strings.Index(text, "cleanup.defer-register")
	valid := strings.Index(text, "ref.is-valid")
	if register < 0 || valid < register || !strings.Contains(text, "place.read %") || !strings.Contains(text, " holder") {
		t.Errorf("the holder must be read and validated inside the body:\n%s", text)
	}

	early, _ := dynamicHolderDefer(t, true)
	if err := Verify(early); err == nil || !strings.Contains(err.Error(), "validated at registration") {
		t.Errorf("validating a dynamic holder at registration verified: %v", err)
	}

	escaped, function := dynamicHolderDefer(t, false)
	for _, block := range function.Blocks {
		for index, op := range block.Operations {
			if op.Kind == OpCleanupDeferRegister {
				region := block.Operations[index].Region
				for _, inner := range region.Blocks {
					for position, candidate := range inner.Operations {
						if candidate.Kind == OpPlaceRead && candidate.PlaceReadMode == PlaceReadHolder {
							escape := Operation{Kind: OpDiscardValue, Operands: []ValueID{candidate.Results[0].ID}}
							inner.Operations = append(append(append([]Operation{}, inner.Operations[:position+1]...), escape), inner.Operations[position+1:]...)
							break
						}
					}
				}
			}
		}
	}
	if err := Verify(escaped); err == nil || !strings.Contains(err.Error(), "escapes through") {
		t.Errorf("a captured holder read escaping as a value verified: %v", err)
	}

	proven, function := dynamicHolderDefer(t, false)
	for _, block := range function.Blocks {
		for index := range block.Operations {
			if region := block.Operations[index].Region; region != nil {
				for _, inner := range region.Blocks {
					for position := range inner.Operations {
						if inner.Operations[position].PlaceReadMode == PlaceReadHolder {
							facts := cloneReferenceMetadata(inner.Operations[position].Reference)
							facts.Validity, facts.InvalidationDomain, facts.EpochDependency, facts.ProofProvenance = ReferenceProven, 0, 0, "caller-owned-borrow"
							inner.Operations[position].Reference = facts
						}
					}
				}
			}
		}
	}
	if err := Verify(proven); err == nil || !strings.Contains(err.Error(), "dynamic-epoch reference holder") {
		t.Errorf("a holder read of a proven reference verified: %v", err)
	}
}

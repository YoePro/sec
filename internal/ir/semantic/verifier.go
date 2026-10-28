package semantic

import (
	"fmt"
	"math/big"
)

func Verify(module *Module) error {
	if module == nil {
		return fmt.Errorf("module is nil")
	}
	if module.Version != Version {
		return fmt.Errorf("unsupported semantic IR version %d", module.Version)
	}
	if module.Identity == "" {
		return fmt.Errorf("module identity is empty")
	}
	if module.Types == nil {
		return fmt.Errorf("type table is nil")
	}
	seenSources := map[string]bool{}
	for _, source := range module.SourceFiles {
		if seenSources[source] {
			return fmt.Errorf("duplicate source file %q", source)
		}
		seenSources[source] = true
	}
	if err := verifyTypes(module.Types); err != nil {
		return err
	}
	if err := verifyEnumUnionDefinitions(module); err != nil {
		return err
	}
	if err := verifyStructDefinitions(module); err != nil {
		return err
	}
	if err := verifyRegisterDefinitions(module); err != nil {
		return err
	}
	if err := verifyDestructionPlans(module); err != nil {
		return err
	}
	if _, err := verifyStatics(module); err != nil {
		return err
	}
	functions := map[FunctionID]*Function{}
	for _, fn := range module.Functions {
		if fn == nil {
			return fmt.Errorf("nil function")
		}
		if fn.ID == "" || fn.Name == "" {
			return fmt.Errorf("function identity/name is empty")
		}
		if functions[fn.ID] != nil {
			return fmt.Errorf("duplicate function ID %q", fn.ID)
		}
		functions[fn.ID] = fn
	}
	if err := verifyStaticPlaceIdentities(module); err != nil {
		return err
	}
	for _, fn := range module.Functions {
		if err := verifyFunction(module, fn, functions); err != nil {
			return fmt.Errorf("function %s: %w", fn.ID, err)
		}
		if err := verifyCleanupFunction(module, fn, functions); err != nil {
			return fmt.Errorf("function %s: %w", fn.ID, err)
		}
	}
	return nil
}

func verifyTypes(table *TypeTable) error {
	for i, t := range table.types {
		id := TypeID(i + 1)
		// rules/types/types.md — concrete generic arguments participate in identity;
		// Semantic IR type references must all resolve before any lowering.
		for _, argument := range t.TypeArgs {
			if _, ok := table.Lookup(argument); !ok {
				return fmt.Errorf("type !%d has invalid type argument !%d", id, argument)
			}
		}
		if t.Kind == TypeNamed {
			if t.Identity == "" || t.Base == 0 {
				return fmt.Errorf("invalid named type !%d", id)
			}
			seen := map[TypeID]bool{id: true}
			base := t.Base
			for base != 0 {
				if seen[base] {
					return fmt.Errorf("cyclic named type !%d", id)
				}
				seen[base] = true
				next, ok := table.Lookup(base)
				if !ok {
					return fmt.Errorf("named type !%d has missing base !%d", id, base)
				}
				if next.Kind != TypeNamed {
					break
				}
				base = next.Base
			}
		}
		if t.TargetSize && !(t.Kind == TypeInt || t.Kind == TypeUint || t.Kind == TypeFloat) {
			return fmt.Errorf("invalid target-sized type !%d", id)
		}
		if err := verifyPlaceReferenceTypes(table, id, t); err != nil {
			return err
		}
		if err := verifyRawPointerType(table, id, t); err != nil {
			return err
		}
		if err := verifySliceType(table, id, t); err != nil {
			return err
		}
		switch t.Kind {
		case TypeArithmeticFailureReason:
			if t.Base != 0 || t.Success != 0 || t.Error != 0 {
				return fmt.Errorf("invalid arithmetic failure reason type !%d", id)
			}
		case TypeCoreError:
			if t.Identity == "" {
				return fmt.Errorf("core error !%d has no identity", id)
			}
		case TypeResult:
			if _, ok := table.Lookup(t.Success); !ok {
				return fmt.Errorf("result !%d has invalid success type", id)
			}
			if _, ok := table.Lookup(t.Error); !ok {
				return fmt.Errorf("result !%d has invalid error type", id)
			}
		case TypeArray:
			if _, ok := table.Lookup(t.Element); !ok {
				return fmt.Errorf("array !%d has invalid element type !%d", id, t.Element)
			}
			if !canonicalArrayLength(t.Length) {
				return fmt.Errorf("array !%d has non-canonical length %q", id, t.Length)
			}
			if typeHasKind(table, t.Element, TypeVoid, TypeNever) {
				return fmt.Errorf("array !%d has unsupported element type", id)
			}
		}
	}
	return nil
}

// canonicalArrayLength enforces the exact decimal fixed length spelling used by
// SEC-MLIR Package 14 sections 27-28. The string form avoids host-width
// conversion and remains stable across 32- and 64-bit compilation plans.
func canonicalArrayLength(length string) bool {
	if length == "" {
		return false
	}
	if length == "0" {
		return true
	}
	if length[0] == '-' || length[0] == '+' || length[0] == '0' {
		return false
	}
	for _, ch := range length {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func verifyEnumUnionDefinitions(module *Module) error {
	seenEnums := map[TypeID]bool{}
	for _, definition := range module.Enums {
		typ, ok := module.Types.Lookup(definition.TypeID)
		if !ok || typ.Kind != TypeEnum || typ.Identity == "" || definition.SymbolID != SymbolID(typ.Identity) ||
			definition.Name == "" || definition.Name != typ.Name || definition.Underlying != typ.Underlying || seenEnums[definition.TypeID] {
			return fmt.Errorf("invalid enum definition !%d", definition.TypeID)
		}
		seenEnums[definition.TypeID] = true
		if !isBuiltinIntegerType(module.Types, definition.Underlying) {
			return fmt.Errorf("enum !%d has non-integer underlying type", definition.TypeID)
		}
		if definition.RepresentationKind != EnumRepresentationInteger && definition.RepresentationKind != EnumRepresentationBitBacked {
			return fmt.Errorf("enum !%d has invalid representation %q", definition.TypeID, definition.RepresentationKind)
		}
		if definition.RepresentationKind == EnumRepresentationInteger && definition.BitWidth != 0 {
			return fmt.Errorf("integer enum !%d has bit width", definition.TypeID)
		}
		if definition.RepresentationKind == EnumRepresentationBitBacked && (definition.BitWidth == 0 || definition.BitWidth > 256) {
			return fmt.Errorf("bit-backed enum !%d has invalid width", definition.TypeID)
		}
		if definition.RepresentationKind == EnumRepresentationBitBacked && !isExactUnsignedIntegerType(module.Types, definition.Underlying, definition.BitWidth) {
			return fmt.Errorf("bit-backed enum !%d requires an unsigned underlying type of width %d", definition.TypeID, definition.BitWidth)
		}
		caseNames := map[string]bool{}
		caseIDs := map[EnumCaseID]bool{}
		for ordinal, enumCase := range definition.Cases {
			if enumCase.ID != EnumCaseID(ordinal) || caseIDs[enumCase.ID] || enumCase.Name == "" || caseNames[enumCase.Name] || enumCase.Value == nil {
				return fmt.Errorf("invalid enum case in !%d", definition.TypeID)
			}
			caseIDs[enumCase.ID], caseNames[enumCase.Name] = true, true
			if !enumValueFits(module.Types, definition, enumCase.Value) {
				return fmt.Errorf("enum case %s is not representable by its underlying type", enumCase.Name)
			}
		}
	}
	for id, typ := range module.Types.types {
		if typ.Kind == TypeEnum && !seenEnums[TypeID(id+1)] {
			return fmt.Errorf("enum type !%d has no definition", id+1)
		}
	}

	seenUnions := map[TypeID]bool{}
	for _, definition := range module.Unions {
		typ, ok := module.Types.Lookup(definition.TypeID)
		if !ok || typ.Kind != TypeUnion || typ.Identity == "" || definition.SymbolID != SymbolID(typ.Identity) ||
			definition.Name == "" || definition.Name != typ.Name || seenUnions[definition.TypeID] || len(definition.Variants) == 0 {
			return fmt.Errorf("invalid union definition !%d", definition.TypeID)
		}
		seenUnions[definition.TypeID] = true
		if !sameTypeIDs(definition.TypeArguments, typ.TypeArgs) {
			return fmt.Errorf("union !%d type arguments disagree", definition.TypeID)
		}
		for _, argument := range definition.TypeArguments {
			if _, ok := module.Types.Lookup(argument); !ok {
				return fmt.Errorf("union !%d has invalid type argument !%d", definition.TypeID, argument)
			}
		}
		if !validCopyClassification(definition.CopyClassification) {
			return fmt.Errorf("union !%d has invalid copy classification %q", definition.TypeID, definition.CopyClassification)
		}
		variantNames := map[string]bool{}
		for index, variant := range definition.Variants {
			if variant.Index != UnionVariantIndex(index) || variant.Name == "" || variantNames[variant.Name] {
				return fmt.Errorf("invalid union variant in !%d", definition.TypeID)
			}
			variantNames[variant.Name] = true
			switch variant.Kind {
			case UnionVariantEmpty:
				if variant.Payload != 0 || len(variant.PayloadFields) != 0 {
					return fmt.Errorf("empty union variant %s has payload", variant.Name)
				}
			case UnionVariantSingle:
				if _, ok := module.Types.Lookup(variant.Payload); !ok || len(variant.PayloadFields) != 0 {
					return fmt.Errorf("single union variant %s has invalid payload", variant.Name)
				}
			case UnionVariantFields:
				if variant.Payload != 0 || len(variant.PayloadFields) == 0 {
					return fmt.Errorf("field union variant %s has invalid shape", variant.Name)
				}
				var synthetic StructDefinition
				if variant.SyntheticPayloadStruct != 0 {
					var ok bool
					synthetic, ok = structDefinition(module, variant.SyntheticPayloadStruct)
					if !ok || synthetic.SyntheticOrigin != StructSyntheticUnionPayload || len(synthetic.Fields) != len(variant.PayloadFields) {
						return fmt.Errorf("field union variant %s has invalid synthetic struct", variant.Name)
					}
				}
				fieldNames := map[string]bool{}
				for fieldIndex, field := range variant.PayloadFields {
					if field.Name == "" || fieldNames[field.Name] {
						return fmt.Errorf("invalid field in union variant %s", variant.Name)
					}
					if _, ok := module.Types.Lookup(field.Type); !ok {
						return fmt.Errorf("union field %s has invalid type", field.Name)
					}
					if variant.SyntheticPayloadStruct != 0 && (synthetic.Fields[fieldIndex].Name != field.Name || synthetic.Fields[fieldIndex].Type != field.Type) {
						return fmt.Errorf("synthetic union payload field %s mismatch", field.Name)
					}
					fieldNames[field.Name] = true
				}
			default:
				return fmt.Errorf("union variant %s has invalid kind", variant.Name)
			}
		}
	}
	for id, typ := range module.Types.types {
		if typ.Kind == TypeUnion && !seenUnions[TypeID(id+1)] {
			return fmt.Errorf("union type !%d has no definition", id+1)
		}
	}
	return nil
}

func sameTypeIDs(left []TypeID, right []TypeID) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validCopyClassification(classification string) bool {
	switch classification {
	case "trivial", "semantic", "move-only", "conditional", "non-copyable":
		return true
	default:
		return false
	}
}

func fitsUnsignedWidth(value *big.Int, width uint16) bool {
	return value != nil && value.Sign() >= 0 && value.BitLen() <= int(width)
}

func isExactUnsignedIntegerType(types *TypeTable, id TypeID, width uint16) bool {
	underlying, ok := types.Lookup(id)
	return ok && underlying.Kind == TypeUint && !underlying.Signed && !underlying.TargetSize && underlying.BitWidth == width
}

func enumValueFits(types *TypeTable, definition EnumDefinition, value *big.Int) bool {
	if value == nil {
		return false
	}
	if definition.RepresentationKind == EnumRepresentationBitBacked {
		return fitsUnsignedWidth(value, definition.BitWidth)
	}
	underlying, ok := types.Lookup(definition.Underlying)
	if !ok {
		return false
	}
	if underlying.TargetSize || underlying.BitWidth == 0 {
		return true
	}
	width := int(underlying.BitWidth)
	if !underlying.Signed {
		return value.Sign() >= 0 && value.BitLen() <= width
	}
	if value.Sign() >= 0 {
		return value.BitLen() < width
	}
	minimum := new(big.Int).Lsh(big.NewInt(1), uint(width-1))
	minimum.Neg(minimum)
	return value.Cmp(minimum) >= 0
}

func enumDefinition(module *Module, typeID TypeID) (EnumDefinition, bool) {
	for _, definition := range module.Enums {
		if definition.TypeID == typeID {
			return definition, true
		}
	}
	return EnumDefinition{}, false
}

func unionVariant(module *Module, typeID TypeID, index UnionVariantIndex) (UnionDefinition, UnionVariantDefinition, bool) {
	for _, definition := range module.Unions {
		if definition.TypeID != typeID || int(index) >= len(definition.Variants) {
			continue
		}
		return definition, definition.Variants[index], true
	}
	return UnionDefinition{}, UnionVariantDefinition{}, false
}

func verifyFunction(module *Module, fn *Function, functions map[FunctionID]*Function) error {
	if _, ok := module.Types.Lookup(fn.ReturnType); !ok {
		return fmt.Errorf("invalid return type !%d", fn.ReturnType)
	}
	if fn.Extern {
		values := map[ValueID]Value{}
		for _, p := range fn.Parameters {
			if err := addValue(module.Types, values, p.Value); err != nil {
				return err
			}
			if isRawPointerType(module.Types, p.Value.Type) && p.Reference != nil {
				return fmt.Errorf("raw-pointer parameter %s carries safe-reference facts", p.Name)
			}
		}
		if len(fn.Blocks) != 0 {
			return fmt.Errorf("extern has body")
		}
		return nil
	}
	if len(fn.Blocks) == 0 {
		return fmt.Errorf("non-extern has no blocks")
	}
	blocks := map[BlockID]*Block{}
	for _, b := range fn.Blocks {
		if b == nil {
			return fmt.Errorf("nil block")
		}
		if blocks[b.ID] != nil {
			return fmt.Errorf("duplicate block ^%d", b.ID)
		}
		blocks[b.ID] = b
	}
	if blocks[fn.Entry] == nil {
		return fmt.Errorf("entry block ^%d is missing", fn.Entry)
	}
	values := map[ValueID]Value{}
	defs := map[ValueID]BlockID{}
	paramDefs := map[ValueID]bool{}
	for _, p := range fn.Parameters {
		if isRawPointerType(module.Types, p.Value.Type) && p.Reference != nil {
			return fmt.Errorf("raw-pointer parameter %s carries safe-reference facts", p.Name)
		}
		if err := addValue(module.Types, values, p.Value); err != nil {
			return err
		}
		paramDefs[p.Value.ID] = true
		if directSafeReferenceType(module.Types, p.Value.Type) {
			if p.Reference == nil {
				return fmt.Errorf("reference parameter %s has no reference facts", p.Name)
			}
			if err := verifyReferenceMetadata(Operation{Kind: "parameter", Reference: p.Reference}); err != nil {
				return err
			}
		}
	}
	for _, b := range fn.Blocks {
		for _, v := range b.Parameters {
			if err := addValue(module.Types, values, v); err != nil {
				return err
			}
			defs[v.ID] = b.ID
		}
		for _, op := range b.Operations {
			for _, v := range op.Results {
				if err := addValue(module.Types, values, v); err != nil {
					return err
				}
				defs[v.ID] = b.ID
			}
		}
	}
	storages := map[StorageID]Storage{}
	for _, s := range fn.Storages {
		if s.ID == 0 {
			return fmt.Errorf("storage ID zero")
		}
		if _, ok := storages[s.ID]; ok {
			return fmt.Errorf("duplicate storage $%d", s.ID)
		}
		if _, ok := module.Types.Lookup(s.Type); !ok {
			return fmt.Errorf("storage $%d has invalid type", s.ID)
		}
		storages[s.ID] = s
	}
	preds := map[BlockID][]BlockID{}
	for _, b := range fn.Blocks {
		if len(b.Operations) == 0 {
			return fmt.Errorf("block ^%d has no terminator", b.ID)
		}
		terminated := false
		definedHere := map[ValueID]bool{}
		for _, v := range b.Parameters {
			definedHere[v.ID] = true
		}
		for index, op := range b.Operations {
			if terminated {
				return fmt.Errorf("operation after terminator in ^%d", b.ID)
			}
			for _, operand := range operationOperands(op) {
				if _, ok := values[operand]; !ok {
					return fmt.Errorf("undefined operand %%%d", operand)
				}
				if defBlock, hasDefinition := defs[operand]; hasDefinition && defBlock == b.ID && !definedHere[operand] {
					return fmt.Errorf("use before definition %%%d", operand)
				}
			}
			if err := verifyOperation(module, fn, op, values, storages, functions, blocks); err != nil {
				return err
			}
			for _, v := range op.Results {
				definedHere[v.ID] = true
			}
			if op.IsTerminator() {
				terminated = true
				if index != len(b.Operations)-1 {
					return fmt.Errorf("operation after terminator in ^%d", b.ID)
				}
				for _, s := range op.Successors {
					preds[s.Block] = append(preds[s.Block], b.ID)
				}
			}
		}
		if !terminated {
			return fmt.Errorf("block ^%d has no terminator", b.ID)
		}
	}
	dom := dominators(fn.Entry, blocks, preds)
	for _, b := range fn.Blocks {
		for _, op := range b.Operations {
			for _, operand := range operationOperands(op) {
				if paramDefs[operand] {
					continue
				}
				defBlock := defs[operand]
				if defBlock != b.ID && !dom[b.ID][defBlock] {
					return fmt.Errorf("value %%%d does not dominate use in ^%d", operand, b.ID)
				}
			}
		}
	}
	if err := verifyStorageDominance(fn, blocks, dom); err != nil {
		return err
	}
	if err := verifyCheckedIntegerGuards(fn, blocks, values); err != nil {
		return err
	}
	if err := verifyRegisterConversionProofs(module, fn, dom); err != nil {
		return err
	}
	if err := verifyCheckedConversionCandidates(fn, dom); err != nil {
		return err
	}
	sliceFn, sliceBlocks, _ := sliceVerificationView(module.Types, fn, blocks)
	if err := verifySliceRangeGuards(sliceFn, sliceBlocks, dom); err != nil {
		return err
	}
	if err := verifyResultGuards(module.Types, fn, blocks, values); err != nil {
		return err
	}
	if err := verifyTryHandlers(fn); err != nil {
		return err
	}
	if err := verifyUnionGuards(fn, blocks); err != nil {
		return err
	}
	if err := verifyMatchRecords(module.Types, fn, blocks, values); err != nil {
		return err
	}
	if err := verifyLoopRecords(fn, blocks); err != nil {
		return err
	}
	if err := verifyNoPanicContradiction(fn, blocks); err != nil {
		return err
	}
	if err := verifyUnionPayloadPlaces(fn, blocks, values, dom); err != nil {
		return err
	}
	if err := verifySliceIndexGuards(module, sliceFn, sliceBlocks, values, dom); err != nil {
		return err
	}
	if err := verifyArrayIndexGuards(module, fn, blocks, values, dom); err != nil {
		return err
	}
	if err := verifyPlaceReferenceFunction(module, fn, blocks, values, dom); err != nil {
		return err
	}
	if err := verifyOwnershipFlow(module, fn); err != nil {
		return err
	}
	if err := verifySubobjectState(module, fn); err != nil {
		return err
	}
	return verifyReturnOrigins(module, fn)
}

func verifyMatchRecords(types *TypeTable, fn *Function, blocks map[BlockID]*Block, values map[ValueID]Value) error {
	seen := map[MatchID]bool{}
	for _, match := range fn.Matches {
		if match.ID == 0 || seen[match.ID] || !match.Exhaustive || len(match.Arms) == 0 {
			return fmt.Errorf("invalid match record %d", match.ID)
		}
		seen[match.ID] = true
		subject, ok := values[match.Subject]
		if !ok || subject.Type != match.SubjectType {
			return fmt.Errorf("invalid match %d subject", match.ID)
		}
		if match.ValueContext {
			merge := blocks[match.MergeBlock]
			if merge == nil || len(merge.Parameters) != 1 || merge.Parameters[0].Type != match.ResultType {
				return fmt.Errorf("invalid value match %d merge", match.ID)
			}
		} else {
			if !typeHasKind(types, match.ResultType, TypeVoid) {
				return fmt.Errorf("statement match %d result is not void", match.ID)
			}
			if match.MergeBlock != 0 {
				merge := blocks[match.MergeBlock]
				if merge == nil || len(merge.Parameters) != 0 {
					return fmt.Errorf("invalid statement match %d continuation", match.ID)
				}
			}
		}
		previous := -1
		for _, arm := range match.Arms {
			if arm.SourceIndex <= previous || blocks[arm.PatternBlock] == nil || blocks[arm.BodyBlock] == nil || arm.PatternKind == "" {
				return fmt.Errorf("invalid match %d arm provenance", match.ID)
			}
			if arm.Guarded && blocks[arm.GuardBlock] == nil {
				return fmt.Errorf("invalid match %d guard provenance", match.ID)
			}
			previous = arm.SourceIndex
		}
	}
	return nil
}

func operationOperands(op Operation) []ValueID {
	operands := append([]ValueID(nil), op.Operands...)
	for _, successor := range op.Successors {
		operands = append(operands, successor.Arguments...)
	}
	return operands
}

func addValue(types *TypeTable, values map[ValueID]Value, v Value) error {
	if isRawPointerType(types, v.Type) != (v.Ownership == OwnershipRawPointer) {
		return fmt.Errorf("value %%%d raw-pointer type and ownership classification disagree", v.ID)
	}
	if _, exists := values[v.ID]; exists {
		return fmt.Errorf("duplicate value %%%d", v.ID)
	}
	if _, ok := types.Lookup(v.Type); !ok {
		return fmt.Errorf("value %%%d has invalid type !%d", v.ID, v.Type)
	}
	switch v.Ownership {
	case OwnershipOwned, OwnershipSharedReference, OwnershipMutableReference, OwnershipRawPointer, OwnershipImmediate, OwnershipCompilerTemporary:
	default:
		return fmt.Errorf("value %%%d has invalid ownership", v.ID)
	}
	values[v.ID] = v
	return nil
}

// verifyOperation enforces typed operation contracts and rejects contradictory
// slice value provenance before downstream representation consumes it.
// Rules: rules/compiler/semantic_ir.md — §§20,126.2–4,126.6–20.
func verifyOperation(module *Module, fn *Function, op Operation, values map[ValueID]Value, storages map[StorageID]Storage, functions map[FunctionID]*Function, blocks map[BlockID]*Block) error {
	if err := verifySliceValueFacts(module.Types, op); err != nil {
		return err
	}
	if err := verifyUnitMetadataOperation(op); err != nil {
		return err
	}
	for _, result := range op.Results {
		if isRawPointerType(module.Types, result.Type) && op.Reference != nil {
			return fmt.Errorf("raw-pointer result %%%d carries safe-reference facts", result.ID)
		}
	}
	resultType := func() TypeID {
		if len(op.Results) == 1 {
			return op.Results[0].Type
		}
		return 0
	}()
	// A Place is never a language value (semantic_ir.md § 96(5)).
	if !placeConsumer(op.Kind) {
		for _, operand := range op.Operands {
			if _, place := placeType(module.Types, values[operand].Type); place {
				return fmt.Errorf("Place %%%d escapes through %s", operand, op.Kind)
			}
		}
	}
	switch op.Kind {
	case OpSliceBorrowShared, OpSliceBorrowMutable, OpSliceReborrowShared, OpSliceReborrowMutable:
		if err := verifySliceBorrowOperation(module.Types, op, values); err != nil {
			return err
		}
	case OpSliceIndexInBounds, OpSliceElementPlace:
		if err := verifySliceIndexOperation(module.Types, op, values); err != nil {
			return err
		}
	case OpSliceCopyShared, OpSliceMoveMutable:
		if err := verifySliceTransferOperation(module.Types, op, values); err != nil {
			return err
		}
	case OpSliceLength:
		if err := verifySliceLengthOperation(module.Types, op, values); err != nil {
			return err
		}
	case OpSliceRangeNormalize, OpSliceRangeCheck:
		if err := verifySliceRangeOperation(module, op, values); err != nil {
			return err
		}
	case OpConstInt:
		if len(op.Results) != 1 || op.Integer == nil {
			return fmt.Errorf("invalid const.int")
		}
		if !typeHasKind(module.Types, resultType, TypeInt, TypeUint, TypeByte, TypeChar, TypeRune) {
			return fmt.Errorf("const.int result type mismatch")
		}
	case OpConstBool:
		if len(op.Results) != 1 || op.Bool == nil || !typeHasKind(module.Types, resultType, TypeBool) {
			return fmt.Errorf("invalid const.bool")
		}
	case OpConstString:
		if len(op.Results) != 1 || !typeHasKind(module.Types, resultType, TypeString) {
			return fmt.Errorf("invalid const.string")
		}
	case OpConstDecimal:
		if len(op.Results) != 1 || op.Decimal == nil || op.Decimal.Coefficient == nil ||
			(!typeHasKind(module.Types, resultType, TypeDecimal) && !typeHasKind(module.Types, resultType, TypeDecimal128)) {
			return fmt.Errorf("invalid const.decimal")
		}
	case OpConstFloat:
		if len(op.Results) != 1 || op.FloatLexeme == "" || !typeHasKind(module.Types, resultType, TypeFloat) {
			return fmt.Errorf("invalid const.float")
		}
		if op.Float != nil && (op.Float.Format != 32 && op.Float.Format != 64 || op.Float.Format == 32 && op.Float.Bits > 0xFFFFFFFF) {
			return fmt.Errorf("invalid const.float payload")
		}
	case OpFloatNeg, OpFloatBinary, OpFloatCompare:
		if err := verifyFloatOperation(module, op, values); err != nil {
			return err
		}
	case OpFloatAbs, OpFloatMinimum, OpFloatMaximum, OpFloatClassify, OpFloatRound, OpFloatProperty:
		if err := verifyFloatCoreOperation(module, op, values); err != nil {
			return err
		}
	case OpFloatClampChecked:
		if err := verifyFloatClampOperation(module, op, values); err != nil {
			return err
		}
	case OpNumericConvertChecked:
		if err := verifyCheckedConversionOperation(module, op, values); err != nil {
			return err
		}
	case OpReturn:
		void := typeHasKind(module.Types, fn.ReturnType, TypeVoid)
		if void && len(op.Operands) != 0 {
			return fmt.Errorf("void return has operand")
		}
		if !void && len(op.Operands) != 1 {
			return fmt.Errorf("non-void return arity")
		}
		if !void && values[op.Operands[0]].Type != fn.ReturnType {
			return fmt.Errorf("return type mismatch")
		}
	case OpDiscardValue:
		if err := verifyDiscardValue(op, storages); err != nil {
			return err
		}
		// Slice destruction ends the holder through the canonical P15 marker;
		// generic value discard cannot omit its borrow/lifetime facts.
		// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §63.
		if len(op.Operands) == 1 {
			if typ, ok := module.Types.Lookup(values[op.Operands[0]].Type); ok && (typ.Kind == TypeSharedSlice || typ.Kind == TypeMutableSlice) {
				return fmt.Errorf("slice discard requires ref.end-borrow with canonical holder facts")
			}
		}
	case OpUnreachable:
		if len(op.Operands) != 0 || len(op.Results) != 0 || len(op.Successors) != 0 || !op.Synthesized || op.Reason == "" {
			return fmt.Errorf("invalid synthesized unreachable")
		}
	case OpStorageDeclare:
		if op.Storage == 0 || storages[op.Storage].ID == 0 {
			return fmt.Errorf("invalid storage.declare")
		}
	case OpStorageInit, OpStorageStore:
		if len(op.Operands) != 1 || storages[op.Storage].ID == 0 {
			return fmt.Errorf("invalid %s", op.Kind)
		}
		if values[op.Operands[0]].Type != storages[op.Storage].Type {
			return fmt.Errorf("storage type mismatch")
		}
		if op.Kind == OpStorageStore && !storages[op.Storage].Mutable {
			return fmt.Errorf("store to immutable storage")
		}
	case OpStorageLoad:
		if len(op.Results) != 1 || storages[op.Storage].ID == 0 || resultType != storages[op.Storage].Type {
			return fmt.Errorf("invalid storage.load")
		}
	case OpDirectCall, OpForeignCall:
		callee := functions[op.Callee]
		if callee == nil {
			return fmt.Errorf("unknown callee %q", op.Callee)
		}
		if (op.Kind == OpDirectCall && callee.Extern) || (op.Kind == OpForeignCall && !callee.Extern) {
			return fmt.Errorf("call kind disagrees with extern status")
		}
		if len(op.Results) == 1 {
			// A returned direct reference carries the caller-side facts
			// instantiated from the callee's origin summary (package15.md § 62).
			if _, isReference := referenceType(module.Types, op.Results[0].Type); isReference || typeHasKind(module.Types, op.Results[0].Type, TypeSharedSlice, TypeMutableSlice) {
				if err := verifyReferenceMetadata(op); err != nil {
					return err
				}
			} else if op.Reference != nil {
				return fmt.Errorf("call without a reference result carries reference facts")
			}
		}
		if len(op.Operands) != len(callee.Parameters) || len(op.ArgumentActions) != len(op.Operands) {
			return fmt.Errorf("call argument arity mismatch")
		}
		for i, id := range op.Operands {
			if values[id].Type != callee.Parameters[i].Value.Type {
				return fmt.Errorf("call argument type mismatch")
			}
			want := ArgumentCopyTrivial
			if reference, ok := referenceType(module.Types, callee.Parameters[i].Value.Type); ok {
				want = ArgumentSharedReference
				if reference.Mutable {
					want = ArgumentMutableReference
				}
			} else if parameter, ok := module.Types.Lookup(callee.Parameters[i].Value.Type); ok && parameter.Kind == TypeSharedSlice {
				want = ArgumentSharedReference
			} else if ok && parameter.Kind == TypeMutableSlice {
				want = ArgumentMutableReference
			} else if !trivialDestruction(module, callee.Parameters[i].Value.Type) {
				want = ArgumentMove
			}
			if op.ArgumentActions[i] != want {
				return fmt.Errorf("call argument %d action %q, want %q for its parameter type", i, op.ArgumentActions[i], want)
			}
		}
		void := typeHasKind(module.Types, callee.ReturnType, TypeVoid)
		if void && len(op.Results) != 0 {
			return fmt.Errorf("void call has result")
		}
		if !void && (len(op.Results) != 1 || resultType != callee.ReturnType) {
			return fmt.Errorf("call result mismatch")
		}
	case OpBranch:
		if len(op.Successors) != 1 {
			return fmt.Errorf("branch successor count")
		}
		if err := verifyTarget(op.Successors[0], blocks, values); err != nil {
			return err
		}
	case OpCondBranch:
		if len(op.Operands) != 1 || !typeHasKind(module.Types, values[op.Operands[0]].Type, TypeBool) {
			return fmt.Errorf("conditional branch condition is not bool")
		}
		if len(op.Successors) != 2 {
			return fmt.Errorf("conditional branch successor count")
		}
		for _, target := range op.Successors {
			if err := verifyTarget(target, blocks, values); err != nil {
				return err
			}
		}
	case OpNumericConvertExact:
		if len(op.Operands) != 1 || len(op.Results) != 1 || !isBuiltinIntegerType(module.Types, values[op.Operands[0]].Type) || !isBuiltinIntegerType(module.Types, op.Results[0].Type) {
			return fmt.Errorf("invalid numeric.convert-exact")
		}
		if err := validConversionProof(module, op.ConversionProof); err != nil {
			return fmt.Errorf("invalid numeric.convert-exact: %w", err)
		}
	case OpIntUnaryPlus, OpIntBitNot:
		if len(op.Operands) != 1 || len(op.Results) != 1 || !isBuiltinIntegerType(module.Types, values[op.Operands[0]].Type) || op.Results[0].Type != values[op.Operands[0]].Type {
			return fmt.Errorf("invalid %s", op.Kind)
		}
	case OpIntNegChecked:
		if !validCheckedResults(module.Types, op, values, 1) || !isSignedBuiltinIntegerType(module.Types, values[op.Operands[0]].Type) {
			return fmt.Errorf("invalid int.neg-checked")
		}
	case OpIntBinaryChecked:
		if !validIntegerBinaryKind(op.IntegerBinary) || !validCheckedResults(module.Types, op, values, 2) || values[op.Operands[0]].Type != values[op.Operands[1]].Type {
			return fmt.Errorf("invalid int.binary-checked")
		}
	case OpIntBitwise:
		if !validIntegerBitwiseKind(op.IntegerBitwise) || len(op.Operands) != 2 || len(op.Results) != 1 || !isBuiltinIntegerType(module.Types, values[op.Operands[0]].Type) || values[op.Operands[0]].Type != values[op.Operands[1]].Type || op.Results[0].Type != values[op.Operands[0]].Type {
			return fmt.Errorf("invalid int.bitwise")
		}
	case OpIntBitExtract:
		if err := verifyBitExtract(module.Types, op, values); err != nil {
			return err
		}
	case OpIntShiftChecked:
		if !validIntegerShiftKind(op.IntegerShift) || !validCheckedResults(module.Types, op, values, 2) || !isBuiltinIntegerType(module.Types, values[op.Operands[1]].Type) || shiftKindSigned(op.IntegerShift) != isSignedBuiltinIntegerType(module.Types, values[op.Operands[0]].Type) {
			return fmt.Errorf("invalid int.shift-checked")
		}
	case OpIntCompare:
		if !validIntegerComparePredicate(op.IntegerCompare) || len(op.Operands) != 2 || len(op.Results) != 1 || !isComparableScalarType(module.Types, values[op.Operands[0]].Type) || values[op.Operands[0]].Type != values[op.Operands[1]].Type || !typeHasKind(module.Types, op.Results[0].Type, TypeBool) {
			return fmt.Errorf("invalid int.compare")
		}
	case OpArithmeticFailure:
		if len(op.Operands) != 1 || len(op.Results) != 0 || len(op.Successors) != 0 || !isFailureReasonType(module.Types, values[op.Operands[0]].Type) || !validArithmeticFailureCategory(op.FailureCategory) || op.Operator == "" {
			return fmt.Errorf("invalid fail.arithmetic")
		}
	case OpArithmeticFailureReasonConstant:
		if len(op.Operands) != 0 || len(op.Results) != 1 || !isFailureReasonType(module.Types, resultType) || !validArithmeticFailureReason(op.FailureReason) {
			return fmt.Errorf("invalid arithmetic failure reason constant")
		}
	case OpArithmeticErrorFromReason:
		if len(op.Operands) != 1 || len(op.Results) != 1 || !isFailureReasonType(module.Types, values[op.Operands[0]].Type) || !isArithmeticErrorType(module.Types, resultType) {
			return fmt.Errorf("invalid arithmetic-error.from-reason")
		}
	case OpResultOk, OpResultErr:
		if len(op.Results) != 1 {
			return fmt.Errorf("invalid %s", op.Kind)
		}
		result, ok := module.Types.Lookup(resultType)
		if !ok || result.Kind != TypeResult {
			return fmt.Errorf("%s result is not Result", op.Kind)
		}
		expected := result.Error
		if op.Kind == OpResultOk {
			expected = result.Success
		}
		if typeHasKind(module.Types, expected, TypeVoid) {
			if len(op.Operands) != 0 {
				return fmt.Errorf("void %s has payload", op.Kind)
			}
		} else if len(op.Operands) != 1 || values[op.Operands[0]].Type != expected {
			return fmt.Errorf("%s payload type mismatch", op.Kind)
		}
	case OpResultIsErr:
		if len(op.Operands) != 1 || len(op.Results) != 1 || !typeHasKind(module.Types, values[op.Operands[0]].Type, TypeResult) || !typeHasKind(module.Types, resultType, TypeBool) {
			return fmt.Errorf("invalid result.is-err")
		}
	case OpResultUnwrapOk, OpResultUnwrapErr:
		if len(op.Operands) != 1 || len(op.Results) != 1 {
			return fmt.Errorf("invalid %s", op.Kind)
		}
		container, ok := module.Types.Lookup(values[op.Operands[0]].Type)
		if !ok || container.Kind != TypeResult {
			return fmt.Errorf("%s operand is not Result", op.Kind)
		}
		expected := container.Error
		if op.Kind == OpResultUnwrapOk {
			expected = container.Success
		}
		if resultType != expected {
			return fmt.Errorf("%s result type mismatch", op.Kind)
		}
	case OpCoreErrorIsVariant:
		if len(op.Operands) != 1 || len(op.Results) != 1 || !isArithmeticErrorType(module.Types, values[op.Operands[0]].Type) || !typeHasKind(module.Types, resultType, TypeBool) || !validArithmeticErrorVariant(op.Variant) {
			return fmt.Errorf("invalid core-error.is-variant")
		}
	case OpEnumConstant:
		definition, ok := enumDefinition(module, resultType)
		if len(op.Operands) != 0 || len(op.Results) != 1 || !ok || int(op.EnumCase) >= len(definition.Cases) {
			return fmt.Errorf("invalid enum.constant")
		}
	case OpEnumFromInteger:
		if len(op.Operands) != 1 || len(op.Results) != 1 || !typeHasKind(module.Types, resultType, TypeEnum) || !isBuiltinIntegerType(module.Types, values[op.Operands[0]].Type) {
			return fmt.Errorf("invalid enum.from-integer")
		}
	case OpEnumToInteger:
		if len(op.Operands) != 1 || len(op.Results) != 1 || !typeHasKind(module.Types, values[op.Operands[0]].Type, TypeEnum) || !isBuiltinIntegerType(module.Types, resultType) {
			return fmt.Errorf("invalid enum.to-integer")
		}
		if err := validConversionProof(module, op.ConversionProof); err != nil {
			return fmt.Errorf("invalid enum.to-integer: %w", err)
		}
	case OpEnumCompare:
		if len(op.Operands) != 2 || len(op.Results) != 1 || (op.IntegerCompare != IntegerCompareEQ && op.IntegerCompare != IntegerCompareNE) || values[op.Operands[0]].Type != values[op.Operands[1]].Type || !typeHasKind(module.Types, values[op.Operands[0]].Type, TypeEnum) || !typeHasKind(module.Types, resultType, TypeBool) {
			return fmt.Errorf("invalid enum.compare")
		}
	case OpUnionConstruct:
		definition, variant, ok := unionVariant(module, resultType, op.UnionVariant)
		if len(op.Results) != 1 || !ok || len(op.PayloadActions) != len(op.Operands) {
			return fmt.Errorf("invalid union.construct")
		}
		for _, action := range op.PayloadActions {
			if action != UnionPayloadCopyTrivial {
				return fmt.Errorf("union.construct requires copy-trivial payload actions")
			}
		}
		_ = definition
		switch variant.Kind {
		case UnionVariantEmpty:
			if len(op.Operands) != 0 || len(op.UnionFields) != 0 {
				return fmt.Errorf("empty union variant has payload")
			}
		case UnionVariantSingle:
			if len(op.Operands) != 1 || values[op.Operands[0]].Type != variant.Payload || len(op.UnionFields) != 0 {
				return fmt.Errorf("single union payload mismatch")
			}
		case UnionVariantFields:
			if len(op.Operands) != len(variant.PayloadFields) || len(op.UnionFields) != len(variant.PayloadFields) {
				return fmt.Errorf("union field payload arity mismatch")
			}
			for index, field := range variant.PayloadFields {
				if op.UnionFields[index] != field.Name || values[op.Operands[index]].Type != field.Type {
					return fmt.Errorf("union fields are not in canonical declaration order")
				}
			}
		}
	case OpUnionIsVariant:
		if len(op.Operands) != 1 || len(op.Results) != 1 || !typeHasKind(module.Types, resultType, TypeBool) {
			return fmt.Errorf("invalid union.is-variant")
		}
		if _, _, ok := unionVariant(module, values[op.Operands[0]].Type, op.UnionVariant); !ok {
			return fmt.Errorf("union.is-variant uses invalid variant")
		}
	case OpUnionUnwrapPayload:
		if len(op.Operands) != 1 || len(op.Results) != 1 || len(op.PayloadActions) != 1 || op.PayloadActions[0] != UnionPayloadCopyTrivial {
			return fmt.Errorf("invalid union.unwrap-payload")
		}
		_, variant, ok := unionVariant(module, values[op.Operands[0]].Type, op.UnionVariant)
		if !ok || variant.Kind != UnionVariantSingle || resultType != variant.Payload {
			return fmt.Errorf("union.unwrap-payload type mismatch")
		}
	case OpUnionUnwrapField:
		if len(op.Operands) != 1 || len(op.Results) != 1 || len(op.PayloadActions) != 1 || op.PayloadActions[0] != UnionPayloadCopyTrivial || op.UnionField == "" {
			return fmt.Errorf("invalid union.unwrap-field")
		}
		_, variant, ok := unionVariant(module, values[op.Operands[0]].Type, op.UnionVariant)
		if !ok || variant.Kind != UnionVariantFields {
			return fmt.Errorf("union.unwrap-field variant mismatch")
		}
		found := false
		for _, field := range variant.PayloadFields {
			if field.Name == op.UnionField && field.Type == resultType {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("union.unwrap-field field mismatch")
		}
	case OpStructConstruct:
		if containsOwnedMutableSlice(module, resultType) {
			return fmt.Errorf("struct.construct containing a mutable slice requires move-aware aggregate lowering (P17)")
		}
		definition, ok := structDefinition(module, resultType)
		if !ok || len(op.Results) != 1 || len(op.Operands) != len(definition.Fields) || len(op.StructOrigins) != len(op.Operands) || len(op.StructActions) != len(op.Operands) {
			return fmt.Errorf("invalid struct.construct")
		}
		for index, field := range definition.Fields {
			if values[op.Operands[index]].Type != field.Type || !validP13StructAction(op.StructActions[index]) || !validStructOrigin(op.StructOrigins[index]) {
				return fmt.Errorf("struct.construct field %d mismatch", index)
			}
		}
	case OpStructSpreadFields:
		if len(op.Operands) == 1 && containsOwnedMutableSlice(module, values[op.Operands[0]].Type) {
			return fmt.Errorf("struct.spread-fields cannot copy a mutable slice")
		}
		if len(op.Operands) != 1 {
			return fmt.Errorf("invalid struct.spread-fields")
		}
		definition, ok := structDefinition(module, values[op.Operands[0]].Type)
		if !ok || len(op.Results) != len(definition.Fields) || len(op.StructActions) != len(definition.Fields) {
			return fmt.Errorf("invalid struct.spread-fields results")
		}
		for index, field := range definition.Fields {
			if op.Results[index].Type != field.Type || op.StructActions[index] != StructActionCopyTrivial {
				return fmt.Errorf("struct spread field %d mismatch", index)
			}
		}
	case OpStructExtractField:
		if containsOwnedMutableSlice(module, resultType) {
			return fmt.Errorf("struct.extract-field cannot copy a mutable slice")
		}
		if len(op.Operands) != 1 || len(op.Results) != 1 || len(op.StructActions) != 1 || op.StructActions[0] != StructActionCopyTrivial {
			return fmt.Errorf("invalid struct.extract-field")
		}
		definition, ok := structDefinition(module, values[op.Operands[0]].Type)
		if !ok || int(op.StructField) >= len(definition.Fields) || resultType != definition.Fields[op.StructField].Type {
			return fmt.Errorf("struct extract field mismatch")
		}
	case OpStructReplaceField:
		if containsOwnedMutableSlice(module, resultType) {
			return fmt.Errorf("struct.replace-field cannot copy a mutable slice")
		}
		if len(op.Operands) != 2 || len(op.Results) != 1 || resultType != values[op.Operands[0]].Type {
			return fmt.Errorf("invalid struct.replace-field")
		}
		definition, ok := structDefinition(module, resultType)
		if !ok || definition.CopyClassification != "trivial" || !definition.TriviallyDestructible || int(op.StructField) >= len(definition.Fields) || values[op.Operands[1]].Type != definition.Fields[op.StructField].Type {
			return fmt.Errorf("unsafe struct.replace-field")
		}
	case OpArrayConstruct:
		if err := verifyArrayConstruct(module, op, values); err != nil {
			return err
		}
	case OpArrayDefault:
		if err := verifyArrayDefault(module, op); err != nil {
			return err
		}
	case OpArrayLength:
		if err := verifyArrayLength(module.Types, op, values); err != nil {
			return err
		}
	case OpArrayIndexInBounds:
		if err := verifyArrayIndexInBounds(module.Types, op, values); err != nil {
			return err
		}
	case OpArrayExtract:
		if err := verifyArrayExtract(module, op, values); err != nil {
			return err
		}
	case OpArrayReplace:
		if err := verifyArrayReplace(module, op, values); err != nil {
			return err
		}
	case OpPanic:
		if err := verifyPanicOperation(op); err != nil {
			return err
		}
	case OpBoolNot:
		if len(op.Operands) != 1 || len(op.Results) != 1 || !typeHasKind(module.Types, values[op.Operands[0]].Type, TypeBool) || !typeHasKind(module.Types, op.Results[0].Type, TypeBool) {
			return fmt.Errorf("invalid bool.not")
		}
	case OpBoundsFailure:
		if len(op.Operands) != 0 || len(op.Results) != 0 || len(op.Successors) != 0 || (op.ArrayOperation != "fixed-array-index" && op.ArrayOperation != "slice-index" && op.ArrayOperation != "slice-range") {
			return fmt.Errorf("invalid fail.bounds")
		}
	case OpAtomicFence:
		if err := verifyAtomicFence(op); err != nil {
			return err
		}
	case OpPlaceValueRoot, OpPlaceStorageRoot, OpPlaceField, OpPlaceArrayIndexInBounds, OpPlaceArrayElement,
		OpPlaceUnionPayload, OpReferenceDerefPlace, OpPlaceRead, OpPlaceWrite, OpReferenceBorrowShared,
		OpReferenceBorrowMutable, OpReferenceCopyShared, OpReferenceMove, OpReferenceReborrowShared,
		OpReferenceReborrowMutable, OpReferenceIsValid, OpReferenceGenerationFailure, OpReferenceEndBorrow,
		OpReferenceCompare:
		if err := verifyPlaceReferenceOperation(module, op, values, storages); err != nil {
			return err
		}
	case OpRegisterValueExtract, OpRegisterValueUpdate:
		if err := verifyRegisterValueOperation(module, op, values); err != nil {
			return err
		}
	case OpRegisterValueFitsBits, OpRegisterValueFromBits, OpRegisterValueToBits:
		if err := verifyRegisterConversionOperation(module, op, values); err != nil {
			return err
		}
	case OpOwnCopy, OpOwnSemanticCopy, OpOwnMove, OpOwnCopyFromPlace, OpOwnMoveFromPlace,
		OpOwnInitializePlace, OpOwnReplacePlace, OpOwnDiscard, OpOwnDestroyValue, OpOwnDestroyPlace:
		if err := verifyOwnershipOperation(module, op, values); err != nil {
			return err
		}
	case OpCleanupTrackOwned, OpCleanupCancel, OpCleanupDeferRegister, OpCleanupDeferYield, OpCleanupRunScope, OpCleanupRunFunction:
		if err := verifyCleanupOperation(module, op, values); err != nil {
			return err
		}
	case OpStaticLoad, OpStaticStore, OpPlaceStaticRoot:
		if err := verifyStaticOperation(module, op, values); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown operation %q", op.Kind)
	}
	return nil
}

func verifyArrayConstruct(module *Module, op Operation, values map[ValueID]Value) error {
	types := module.Types
	if len(op.Results) != 1 ||
		len(op.ArraySegmentKinds) != len(op.Operands) ||
		len(op.ArraySegmentLengths) != len(op.Operands) ||
		len(op.ArrayActions) != len(op.Operands) {
		return fmt.Errorf("invalid array.construct")
	}
	result, ok := types.Lookup(op.Results[0].Type)
	if !ok || result.Kind != TypeArray || result.Element != op.ArrayElementType || result.Length != op.ArrayLength {
		return fmt.Errorf("array.construct result type mismatch")
	}
	if len(op.Operands) == 0 {
		if op.ArrayLength != "0" {
			return fmt.Errorf("array.construct length mismatch")
		}
		return nil
	}
	total := new(big.Int)
	for index, operand := range op.Operands {
		length, ok := parseCanonicalArrayLength(op.ArraySegmentLengths[index])
		if !ok || length.Sign() == 0 {
			return fmt.Errorf("array.construct segment %d has invalid length", index)
		}
		total.Add(total, length)
		operandType := values[operand].Type
		switch op.ArraySegmentKinds[index] {
		case ArraySegmentElement:
			if (op.ArrayActions[index] != ArrayActionConstructDirect && op.ArrayActions[index] != ArrayActionCopyTrivial && op.ArrayActions[index] != ArrayActionMove) || op.ArraySegmentLengths[index] != "1" || operandType != op.ArrayElementType {
				return fmt.Errorf("array.construct element segment %d mismatch", index)
			}
			if op.ArrayActions[index] == ArrayActionCopyTrivial && (ownedCopyClass(module, operandType) != "trivial" || !trivialDestruction(module, operandType)) {
				return fmt.Errorf("array.construct cannot copy non-trivial element %d", index)
			}
		case ArraySegmentRange:
			if op.ArrayActions[index] != ArrayActionConstructDirect || operandType != op.ArrayElementType {
				return fmt.Errorf("array.construct range segment %d mismatch", index)
			}
			if !package14TrivialValue(module, op.ArrayElementType) {
				return fmt.Errorf("array.construct range requires copy-trivial numeric elements")
			}
		case ArraySegmentSpread:
			spread, ok := types.Lookup(operandType)
			if !ok || spread.Kind != TypeArray || spread.Element != op.ArrayElementType || spread.Length != op.ArraySegmentLengths[index] || op.ArrayActions[index] != ArrayActionCopyTrivial {
				return fmt.Errorf("array.construct spread segment %d mismatch", index)
			}
			if ownedCopyClass(module, op.ArrayElementType) != "trivial" || !trivialDestruction(module, op.ArrayElementType) {
				return fmt.Errorf("array.construct spread requires copy-trivial elements")
			}
		default:
			return fmt.Errorf("array.construct segment %d has invalid kind", index)
		}
	}
	if total.String() != op.ArrayLength {
		return fmt.Errorf("array.construct length mismatch")
	}
	return nil
}

func verifyArrayDefault(module *Module, op Operation) error {
	if len(op.Results) != 1 || len(op.Operands) != 0 {
		return fmt.Errorf("invalid array.default")
	}
	result, ok := module.Types.Lookup(op.Results[0].Type)
	if !ok || result.Kind != TypeArray || result.Element != op.ArrayElementType || result.Length != op.ArrayLength {
		return fmt.Errorf("array.default result type mismatch")
	}
	length, ok := parseCanonicalArrayLength(op.ArrayLength)
	if !ok {
		return fmt.Errorf("array.default has invalid length")
	}
	if length.Sign() == 0 {
		return nil
	}
	if !arrayElementDefaultable(module, op.ArrayElementType) {
		return fmt.Errorf("array.default element has unsupported default")
	}
	return nil
}

func verifyArrayLength(types *TypeTable, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 1 || len(op.Results) != 1 {
		return fmt.Errorf("invalid array.len")
	}
	array, ok := types.Lookup(values[op.Operands[0]].Type)
	if !ok || array.Kind != TypeArray || array.Length != op.ArrayLength {
		return fmt.Errorf("array.len operand mismatch")
	}
	if !typeHasKind(types, op.Results[0].Type, TypeUint) {
		return fmt.Errorf("array.len result must be uint")
	}
	return nil
}

// verifyArrayIndexInBounds implements the total semantic predicate from
// SEC-MLIR Package 14 sections 37 and 66. The source index type and signedness
// remain intact; target-width normalization belongs to later scalar lowering.
func verifyArrayIndexInBounds(types *TypeTable, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 2 || len(op.Results) != 1 || !typeHasKind(types, op.Results[0].Type, TypeBool) {
		return fmt.Errorf("invalid array.index-in-bounds")
	}
	array, ok := types.Lookup(values[op.Operands[0]].Type)
	if !ok || array.Kind != TypeArray {
		return fmt.Errorf("array.index-in-bounds operand is not a fixed array")
	}
	signed, ok := semanticIntegerSigned(types, values[op.Operands[1]].Type)
	if !ok {
		return fmt.Errorf("array.index-in-bounds index is not an integer")
	}
	if op.ArrayIndexSigned != signed {
		return fmt.Errorf("array.index-in-bounds signedness mismatch")
	}
	return nil
}

func verifyArrayExtract(module *Module, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 2 || len(op.Results) != 1 || len(op.ArrayActions) != 1 || op.ArrayActions[0] != ArrayActionCopyTrivial {
		return fmt.Errorf("invalid array.extract")
	}
	array, ok := module.Types.Lookup(values[op.Operands[0]].Type)
	if !ok || array.Kind != TypeArray || op.Results[0].Type != array.Element {
		return fmt.Errorf("array.extract type mismatch")
	}
	if _, ok := semanticIntegerSigned(module.Types, values[op.Operands[1]].Type); !ok {
		return fmt.Errorf("array.extract index is not an integer")
	}
	if !package14TrivialValue(module, array.Element) {
		return fmt.Errorf("array.extract requires copy-trivial element")
	}
	return verifyArrayBoundsMetadata(op)
}

func verifyArrayReplace(module *Module, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 3 || len(op.Results) != 1 {
		return fmt.Errorf("invalid array.replace")
	}
	arrayType := values[op.Operands[0]].Type
	array, ok := module.Types.Lookup(arrayType)
	if !ok || array.Kind != TypeArray || op.Results[0].Type != arrayType || values[op.Operands[2]].Type != array.Element {
		return fmt.Errorf("array.replace type mismatch")
	}
	if _, ok := semanticIntegerSigned(module.Types, values[op.Operands[1]].Type); !ok {
		return fmt.Errorf("array.replace index is not an integer")
	}
	if !package14TrivialValue(module, array.Element) {
		return fmt.Errorf("array.replace requires trivial array and element")
	}
	return verifyArrayBoundsMetadata(op)
}

func verifyArrayBoundsMetadata(op Operation) error {
	switch op.ArrayCheckKind {
	case ArrayIndexProvenSafe:
		if !validArrayProof(op.ArrayProofKind) || op.ArrayGuard != 0 {
			return fmt.Errorf("%s has invalid proven-safe bounds provenance", op.Kind)
		}
	case ArrayIndexRuntimeCheck:
		if op.ArrayProofKind != ArrayIndexProofGuarded || op.ArrayGuard == 0 {
			return fmt.Errorf("%s has invalid runtime bounds provenance", op.Kind)
		}
	default:
		return fmt.Errorf("%s has invalid bounds kind", op.Kind)
	}
	return nil
}

func validArrayProof(proof ArrayIndexProofKind) bool {
	switch proof {
	case ArrayIndexProofConstant, ArrayIndexProofRange, ArrayIndexProofBranch, ArrayIndexProofContract, ArrayIndexProofAnalysis:
		return true
	default:
		return false
	}
}

// semanticIntegerSigned retains signedness through named integer types;
// byte is an unsigned integer carrier, including in range endpoints.
// Rules: rules/types/types.md — "Integer types";
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§15,35,78–79.
func semanticIntegerSigned(types *TypeTable, id TypeID) (bool, bool) {
	typ, ok := types.Lookup(id)
	if !ok {
		return false, false
	}
	if typ.Kind == TypeNamed {
		return semanticIntegerSigned(types, typ.Base)
	}
	switch typ.Kind {
	case TypeInt:
		return true, true
	case TypeUint, TypeByte:
		return false, true
	default:
		return false, false
	}
}

func package14TrivialValue(module *Module, id TypeID) bool {
	typ, ok := module.Types.Lookup(id)
	if !ok {
		return false
	}
	if typ.Kind == TypeNamed {
		return package14TrivialValue(module, typ.Base)
	}
	switch typ.Kind {
	case TypeBool, TypeByte, TypeChar, TypeRune, TypeDecimal128, TypeInt, TypeUint, TypeFloat, TypeEnum, TypeRawPointer:
		return true
	case TypeArray:
		return package14TrivialValue(module, typ.Element)
	case TypeStruct:
		definition, ok := structDefinition(module, id)
		return ok && definition.CopyClassification == "trivial" && definition.TriviallyDestructible
	case TypeUnion:
		for _, definition := range module.Unions {
			if definition.TypeID == id {
				return definition.CopyClassification == "trivial" && definition.TriviallyDestructible
			}
		}
		return false
	default:
		return false
	}
}

func parseCanonicalArrayLength(length string) (*big.Int, bool) {
	if !canonicalArrayLength(length) {
		return nil, false
	}
	value, ok := new(big.Int).SetString(length, 10)
	if !ok {
		return nil, false
	}
	return value, true
}

func arrayElementDefaultable(module *Module, id TypeID) bool {
	typ, ok := module.Types.Lookup(id)
	if !ok {
		return false
	}
	if typ.Kind == TypeNamed {
		return arrayElementDefaultable(module, typ.Base)
	}
	switch typ.Kind {
	case TypeBool, TypeByte, TypeChar, TypeRune, TypeString, TypeDecimal, TypeDecimal128, TypeInt, TypeUint, TypeFloat, TypeEnum:
		return true
	case TypeArray:
		length, ok := parseCanonicalArrayLength(typ.Length)
		return ok && (length.Sign() == 0 || arrayElementDefaultable(module, typ.Element))
	case TypeStruct:
		definition, ok := structDefinition(module, id)
		return ok && definition.Defaultable
	default:
		return false
	}
}

func structDefinition(module *Module, typeID TypeID) (StructDefinition, bool) {
	for _, definition := range module.Structs {
		if definition.TypeID == typeID {
			return definition, true
		}
	}
	return StructDefinition{}, false
}

func validP13StructAction(action StructFieldAction) bool {
	return action == StructActionConstructDirect || action == StructActionCopyTrivial
}
func validStructOrigin(origin StructFieldOrigin) bool {
	return origin == StructOriginExplicit || origin == StructOriginSpread || origin == StructOriginDefault
}

func validArithmeticErrorVariant(variant string) bool {
	// MD-067: the five canonical variants of core::ArithmeticError.
	return variant == "Overflow" || variant == "DivisionByZero" || variant == "InvalidShift" ||
		variant == "ScaleOverflow" || variant == "PrecisionExhausted"
}

func validCheckedResults(types *TypeTable, op Operation, values map[ValueID]Value, operands int) bool {
	return len(op.Operands) == operands && len(op.Results) == 3 &&
		isBuiltinIntegerType(types, values[op.Operands[0]].Type) &&
		op.Results[0].Type == values[op.Operands[0]].Type &&
		typeHasKind(types, op.Results[1].Type, TypeBool) && isFailureReasonType(types, op.Results[2].Type)
}

func isFailureReasonType(types *TypeTable, id TypeID) bool {
	return typeHasKind(types, id, TypeArithmeticFailureReason)
}

func isArithmeticErrorType(types *TypeTable, id TypeID) bool {
	t, ok := types.Lookup(id)
	return ok && (t.Kind == TypeCoreError || t.Kind == TypeEnum) && t.Identity == "core::ArithmeticError"
}

func validArithmeticFailureReason(reason ArithmeticFailureReason) bool {
	switch reason {
	case ArithmeticFailureNone, ArithmeticFailureReasonOverflow, ArithmeticFailureDivisionByZero, ArithmeticFailureInvalidShift:
		return true
	default:
		return false
	}
}

func isBuiltinIntegerType(types *TypeTable, id TypeID) bool {
	typ, ok := types.Lookup(id)
	return ok && (typ.Kind == TypeInt || typ.Kind == TypeUint || typ.Kind == TypeByte)
}

// isComparableScalarType admits the operand types of int.compare: built-in
// integers and the character types char and rune, which compare unsigned
// (operators.md "Character ordering", "Rune ordering"; MD-105).
func isComparableScalarType(types *TypeTable, id TypeID) bool {
	typ, ok := types.Lookup(id)
	return ok && (typ.Kind == TypeInt || typ.Kind == TypeUint || typ.Kind == TypeByte || typ.Kind == TypeChar || typ.Kind == TypeRune)
}

func isSignedBuiltinIntegerType(types *TypeTable, id TypeID) bool {
	typ, ok := types.Lookup(id)
	return ok && typ.Kind == TypeInt && typ.Signed
}

func validIntegerBinaryKind(kind IntegerCheckedBinaryKind) bool {
	switch kind {
	case IntegerCheckedAdd, IntegerCheckedSubtract, IntegerCheckedMultiply, IntegerCheckedDivide, IntegerCheckedRemainder:
		return true
	default:
		return false
	}
}

func validIntegerBitwiseKind(kind IntegerBitwiseKind) bool {
	return kind == IntegerBitwiseAnd || kind == IntegerBitwiseOr || kind == IntegerBitwiseXor
}

func validIntegerShiftKind(kind IntegerShiftKind) bool {
	switch kind {
	case IntegerShiftLeftUnsigned, IntegerShiftLeftSigned, IntegerShiftRightUnsigned, IntegerShiftRightSigned:
		return true
	default:
		return false
	}
}

func shiftKindSigned(kind IntegerShiftKind) bool {
	return kind == IntegerShiftLeftSigned || kind == IntegerShiftRightSigned
}

func validIntegerComparePredicate(predicate IntegerComparePredicate) bool {
	switch predicate {
	case IntegerCompareEQ, IntegerCompareNE, IntegerCompareLT, IntegerCompareLE, IntegerCompareGT, IntegerCompareGE:
		return true
	default:
		return false
	}
}

func validArithmeticFailureCategory(category ArithmeticFailureCategory) bool {
	switch category {
	case ArithmeticFailureOverflow, ArithmeticFailureDivision, ArithmeticFailureRemainder, ArithmeticFailureShift:
		return true
	default:
		return false
	}
}

func isCheckedIntegerOperation(kind OpKind) bool {
	return kind == OpIntNegChecked || kind == OpIntBinaryChecked || kind == OpIntShiftChecked
}

func verifyCheckedIntegerGuards(fn *Function, blocks map[BlockID]*Block, values map[ValueID]Value) error {
	uses := map[ValueID]int{}
	incoming := map[BlockID]int{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, operand := range operationOperands(op) {
				uses[operand]++
			}
			for _, successor := range op.Successors {
				incoming[successor.Block]++
			}
		}
	}
	for _, block := range fn.Blocks {
		for index, op := range block.Operations {
			if !isCheckedIntegerOperation(op.Kind) {
				continue
			}
			if len(op.Results) != 3 || index+1 != len(block.Operations)-1 {
				return fmt.Errorf("checked integer operation in ^%d must be immediately guarded", block.ID)
			}
			failed := op.Results[1].ID
			reason := op.Results[2].ID
			if uses[failed] != 1 {
				return fmt.Errorf("checked integer failure %%%d must have exactly one use", failed)
			}
			branch := block.Operations[index+1]
			if branch.Kind != OpCondBranch || len(branch.Operands) != 1 || branch.Operands[0] != failed || len(branch.Successors) != 2 {
				return fmt.Errorf("checked integer failure %%%d must guard with conditional branch", failed)
			}
			failureBlock := blocks[branch.Successors[0].Block]
			if failureBlock == nil || len(failureBlock.Parameters) != 1 || incoming[failureBlock.ID] != 1 || len(branch.Successors[0].Arguments) != 1 || branch.Successors[0].Arguments[0] != reason {
				return fmt.Errorf("checked integer failure block must be dedicated")
			}
			if failureBlock.Parameters[0].Type != op.Results[2].Type {
				return fmt.Errorf("checked integer failure reason block parameter type mismatch")
			}
			failureReason := failureBlock.Parameters[0].ID
			if len(failureBlock.Operations) == 1 {
				failure := failureBlock.Operations[0]
				if failure.Kind != OpArithmeticFailure || len(failure.Operands) != 1 || failure.Operands[0] != failureReason || failure.FailureCategory != expectedFailureCategory(op) || failure.Operator != op.Operator {
					return fmt.Errorf("checked integer failure endpoint does not match %s", op.Kind)
				}
			} else if !validFallibleArithmeticFailureBlock(failureBlock, failureReason) && !validHandledArithmeticFailureBlock(failureBlock, failureReason) {
				return fmt.Errorf("checked integer fallible failure endpoint does not return Result.err")
			}
		}
	}
	return nil
}

func validHandledArithmeticFailureBlock(block *Block, reason ValueID) bool {
	if len(block.Operations) < 2 {
		return false
	}
	convert := block.Operations[0]
	if convert.Kind != OpArithmeticErrorFromReason || len(convert.Operands) != 1 || convert.Operands[0] != reason || len(convert.Results) != 1 {
		return false
	}
	if next := block.Operations[1]; next.Kind == OpCoreErrorIsVariant {
		return len(next.Operands) == 1 && next.Operands[0] == convert.Results[0].ID
	} else if next.Kind == OpEnumConstant && len(block.Operations) >= 3 && len(next.Results) == 1 {
		compare := block.Operations[2]
		return compare.Kind == OpEnumCompare && len(compare.Operands) == 2 && compare.Operands[0] == convert.Results[0].ID && compare.Operands[1] == next.Results[0].ID
	}
	return block.Operations[len(block.Operations)-1].IsTerminator()
}

func verifyResultGuards(types *TypeTable, fn *Function, blocks map[BlockID]*Block, values map[ValueID]Value) error {
	uses := map[ValueID]int{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, operand := range operationOperands(op) {
				uses[operand]++
			}
		}
	}
	for _, block := range fn.Blocks {
		for index, op := range block.Operations {
			if op.Kind != OpResultIsErr {
				continue
			}
			if len(op.Operands) != 1 || len(op.Results) != 1 || index+1 != len(block.Operations)-1 {
				return fmt.Errorf("result.is-err in ^%d must be immediately guarded", block.ID)
			}
			predicate := op.Results[0].ID
			if uses[predicate] != 1 {
				return fmt.Errorf("result guard %%%d must have exactly one use", predicate)
			}
			branch := block.Operations[index+1]
			if branch.Kind != OpCondBranch || len(branch.Operands) != 1 || branch.Operands[0] != predicate || len(branch.Successors) != 2 {
				return fmt.Errorf("result guard %%%d must branch to Err then Ok", predicate)
			}
			if op.MatchID != 0 {
				if op.MatchStage != "pattern" || branch.MatchID != op.MatchID || branch.MatchArmIndex != op.MatchArmIndex || branch.MatchStage != "pattern" {
					return fmt.Errorf("match Result guard %%%d has inconsistent provenance", predicate)
				}
				// Match may discard a payload, and an Ok arm reverses the ordinary
				// Err/Ok successor roles. MatchRecord verifies the selected body;
				// payload operations are independently type- and CFG-verified.
				continue
			}
			container := op.Operands[0]
			if !blockStartsWithUnwrap(blocks[branch.Successors[0].Block], OpResultUnwrapErr, container) {
				return fmt.Errorf("result guard %%%d Err successor must unwrap Err", predicate)
			}
			resultType, _ := types.Lookup(values[container].Type)
			if !typeHasKind(types, resultType.Success, TypeVoid) && !blockStartsWithUnwrap(blocks[branch.Successors[1].Block], OpResultUnwrapOk, container) {
				return fmt.Errorf("result guard %%%d Ok successor must unwrap Ok", predicate)
			}
		}
	}
	return nil
}

func blockStartsWithUnwrap(block *Block, kind OpKind, container ValueID) bool {
	return block != nil && len(block.Operations) != 0 && block.Operations[0].Kind == kind &&
		len(block.Operations[0].Operands) == 1 && block.Operations[0].Operands[0] == container
}

func verifyUnionGuards(fn *Function, blocks map[BlockID]*Block) error {
	type guard struct {
		container ValueID
		variant   UnionVariantIndex
	}
	trueGuards := map[BlockID][]guard{}
	incoming := map[BlockID]int{}
	for _, block := range fn.Blocks {
		for _, operation := range block.Operations {
			for _, successor := range operation.Successors {
				incoming[successor.Block]++
			}
		}
	}
	for _, block := range fn.Blocks {
		for index, operation := range block.Operations {
			if operation.Kind != OpUnionIsVariant {
				continue
			}
			if index+1 >= len(block.Operations) || len(operation.Results) != 1 || len(operation.Operands) != 1 {
				return fmt.Errorf("union.is-variant in ^%d must be immediately guarded", block.ID)
			}
			branch := block.Operations[index+1]
			if branch.Kind != OpCondBranch || len(branch.Operands) != 1 || branch.Operands[0] != operation.Results[0].ID || len(branch.Successors) != 2 {
				return fmt.Errorf("union variant test in ^%d must feed a conditional branch", block.ID)
			}
			trueBlock := branch.Successors[0].Block
			trueGuards[trueBlock] = append(trueGuards[trueBlock], guard{container: operation.Operands[0], variant: operation.UnionVariant})
		}
	}
	for _, block := range fn.Blocks {
		for _, operation := range block.Operations {
			if operation.Kind != OpUnionUnwrapPayload && operation.Kind != OpUnionUnwrapField {
				continue
			}
			matched := false
			if incoming[block.ID] != 1 {
				return fmt.Errorf("union projection in ^%d requires one dedicated guarded predecessor", block.ID)
			}
			for _, guard := range trueGuards[block.ID] {
				if len(operation.Operands) == 1 && operation.Operands[0] == guard.container && operation.UnionVariant == guard.variant {
					matched = true
					break
				}
			}
			if !matched {
				return fmt.Errorf("union projection in ^%d is not on its matching guarded path", block.ID)
			}
		}
	}
	return nil
}

// verifyUnionPayloadPlaces enforces the active-variant proof of every
// place.union-payload: its guard must be a union.is-variant predicate for the
// same variant whose tested union value is the very value of the Place's
// root (the SSA value of a value root, or a load of the storage of a storage
// root), and the payload Place must be confined to that predicate's true
// edge. This deliberately does not redo Sema analysis.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 19, 44, 71, 121
func verifyUnionPayloadPlaces(fn *Function, blocks map[BlockID]*Block, values map[ValueID]Value, dom map[BlockID]map[BlockID]bool) error {
	type test struct {
		container  ValueID
		variant    UnionVariantIndex
		block      BlockID
		trueBlock  BlockID
		falseBlock BlockID
		isErr      bool
	}
	tests := map[ValueID]test{}
	definitions := map[ValueID]Operation{}
	graph := map[BlockID][]BlockID{}
	for _, block := range fn.Blocks {
		for index, op := range block.Operations {
			for _, result := range op.Results {
				definitions[result.ID] = op
			}
			for _, successor := range op.Successors {
				graph[block.ID] = append(graph[block.ID], successor.Block)
			}
			if (op.Kind != OpUnionIsVariant && op.Kind != OpResultIsErr) || len(op.Results) != 1 || len(op.Operands) != 1 || index+1 >= len(block.Operations) {
				continue
			}
			branch := block.Operations[index+1]
			if branch.Kind == OpCondBranch && len(branch.Successors) == 2 && len(branch.Operands) == 1 && branch.Operands[0] == op.Results[0].ID {
				variant := op.UnionVariant
				if op.Kind == OpResultIsErr {
					// result.is-err proves Err (variant 1) on its true edge.
					variant = 1
				}
				tests[op.Results[0].ID] = test{container: op.Operands[0], variant: variant, block: block.ID, trueBlock: branch.Successors[0].Block, falseBlock: branch.Successors[1].Block, isErr: op.Kind == OpResultIsErr}
			}
		}
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind != OpPlaceUnionPayload {
				continue
			}
			guard, ok := tests[op.VariantGuard]
			if ok && guard.isErr && op.UnionVariant == 0 {
				// The Ok payload is proven on the false edge of result.is-err.
				guard.variant = 0
				guard.trueBlock, guard.falseBlock = guard.falseBlock, guard.trueBlock
			}
			if !ok || guard.variant != op.UnionVariant {
				return fmt.Errorf("place.union-payload in ^%d does not name a matching union.is-variant guard", block.ID)
			}
			if len(op.Operands) != 1 {
				return fmt.Errorf("invalid place.union-payload")
			}
			// The payload Place and the tested union value must denote the
			// same storage: the same root (an SSA value, a storage, or the
			// dereference of one reference value) reached through the same
			// stored fields.
			read, readOK := definitions[guard.container]
			readFromPayloadBase := readOK && read.Kind == OpPlaceRead && len(read.Operands) == 1 && read.Operands[0] == op.Operands[0]
			if !readFromPayloadBase && !samePlaceChain(placeChainOf(op.Operands[0], definitions), valueChainOf(guard.container, definitions)) {
				return fmt.Errorf("place.union-payload in ^%d is not a payload of the union its guard tested", block.ID)
			}
			// A loop may lead the false edge back to the test itself, which
			// re-establishes the proof, so only paths avoiding the test count.
			if !dom[block.ID][guard.trueBlock] || reachableAvoiding(graph, guard.falseBlock, block.ID, guard.block) {
				return fmt.Errorf("place.union-payload in ^%d is not confined to its variant guard's true edge", block.ID)
			}
		}
	}
	return nil
}

// placeChain is a canonical Place path: a root kind and key plus the stored
// field path from the root.
type placeChain struct {
	kind   string
	key    uint64
	fields []StructFieldID
	ok     bool
}

func placeChainOf(id ValueID, definitions map[ValueID]Operation) placeChain {
	var reversed []StructFieldID
	for {
		op, ok := definitions[id]
		if !ok {
			return placeChain{}
		}
		switch op.Kind {
		case OpPlaceField:
			if len(op.Operands) != 1 {
				return placeChain{}
			}
			reversed = append(reversed, op.StructField)
			id = op.Operands[0]
			continue
		case OpPlaceValueRoot:
			if len(op.Operands) != 1 {
				return placeChain{}
			}
			return placeChain{kind: "value", key: uint64(op.Operands[0]), fields: reverseFields(reversed), ok: true}
		case OpPlaceStorageRoot:
			return placeChain{kind: "storage", key: uint64(op.Storage), fields: reverseFields(reversed), ok: true}
		case OpReferenceDerefPlace:
			if len(op.Operands) != 1 {
				return placeChain{}
			}
			return placeChain{kind: "deref", key: uint64(op.Operands[0]), fields: reverseFields(reversed), ok: true}
		}
		return placeChain{}
	}
}

// valueChainOf follows a value back through struct.extract-field to the root
// it was read from: an SSA value, a storage load, or a place.read of a Place.
func valueChainOf(id ValueID, definitions map[ValueID]Operation) placeChain {
	var reversed []StructFieldID
	for {
		op, ok := definitions[id]
		if !ok {
			return placeChain{kind: "value", key: uint64(id), fields: reverseFields(reversed), ok: true}
		}
		switch op.Kind {
		case OpStructExtractField:
			if len(op.Operands) != 1 {
				return placeChain{}
			}
			reversed = append(reversed, op.StructField)
			id = op.Operands[0]
			continue
		case OpStorageLoad:
			return placeChain{kind: "storage", key: uint64(op.Storage), fields: reverseFields(reversed), ok: true}
		case OpPlaceRead:
			if len(op.Operands) != 1 {
				return placeChain{}
			}
			base := placeChainOf(op.Operands[0], definitions)
			base.fields = append(base.fields, reverseFields(reversed)...)
			return base
		}
		return placeChain{kind: "value", key: uint64(id), fields: reverseFields(reversed), ok: true}
	}
}

func reverseFields(fields []StructFieldID) []StructFieldID {
	out := make([]StructFieldID, len(fields))
	for index, field := range fields {
		out[len(fields)-1-index] = field
	}
	return out
}

func samePlaceChain(left, right placeChain) bool {
	if !left.ok || !right.ok || left.kind != right.kind || left.key != right.key || len(left.fields) != len(right.fields) {
		return false
	}
	for index := range left.fields {
		if left.fields[index] != right.fields[index] {
			return false
		}
	}
	return true
}

// verifyArrayIndexGuards enforces the explicit P14 guard provenance contract.
// A runtime extraction/replacement must name the predicate that guarded the
// same array and index SSA values, and its block must be reachable only from
// that predicate's true edge. This deliberately does not redo Sema analysis.
func verifyArrayIndexGuards(module *Module, fn *Function, blocks map[BlockID]*Block, values map[ValueID]Value, dom map[BlockID]map[BlockID]bool) error {
	type guard struct {
		array      ValueID
		index      ValueID
		block      BlockID
		trueBlock  BlockID
		falseBlock BlockID
	}
	guards := map[ValueID]guard{}
	uses := map[ValueID]int{}
	graph := map[BlockID][]BlockID{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, operand := range operationOperands(op) {
				uses[operand]++
			}
			for _, successor := range op.Successors {
				graph[block.ID] = append(graph[block.ID], successor.Block)
			}
		}
	}
	for _, block := range fn.Blocks {
		for index, op := range block.Operations {
			if op.Kind != OpArrayIndexInBounds {
				continue
			}
			if len(op.Operands) != 2 || len(op.Results) != 1 || index+1 != len(block.Operations)-1 {
				return fmt.Errorf("array.index-in-bounds in ^%d must be immediately guarded", block.ID)
			}
			predicate := op.Results[0].ID
			branch := block.Operations[index+1]
			if uses[predicate] != 1 || branch.Kind != OpCondBranch || len(branch.Operands) != 1 || branch.Operands[0] != predicate || len(branch.Successors) != 2 {
				return fmt.Errorf("array bounds predicate %%%d must feed one conditional branch", predicate)
			}
			guards[predicate] = guard{
				array:      op.Operands[0],
				index:      op.Operands[1],
				block:      block.ID,
				trueBlock:  branch.Successors[0].Block,
				falseBlock: branch.Successors[1].Block,
			}
			if failure := blocks[branch.Successors[1].Block]; !validSequenceBoundsFailureBlock(module, failure, values, "fixed-array-index") {
				return fmt.Errorf("array bounds predicate %%%d has invalid failure endpoint", predicate)
			}
		}
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if (op.Kind != OpArrayExtract && op.Kind != OpArrayReplace) || op.ArrayCheckKind != ArrayIndexRuntimeCheck {
				continue
			}
			guard, ok := guards[op.ArrayGuard]
			if !ok || len(op.Operands) < 2 || op.Operands[0] != guard.array || op.Operands[1] != guard.index {
				return fmt.Errorf("%s in ^%d does not match its array bounds guard", op.Kind, block.ID)
			}
			if !dom[block.ID][guard.trueBlock] || reachableBlock(graph, guard.falseBlock, block.ID) {
				return fmt.Errorf("%s in ^%d is not confined to its bounds guard true edge", op.Kind, block.ID)
			}
		}
	}
	return nil
}

// reachableAvoiding reports whether target is reachable from from without
// passing through avoid.
func reachableAvoiding(graph map[BlockID][]BlockID, from, target, avoid BlockID) bool {
	work := []BlockID{from}
	seen := map[BlockID]bool{avoid: true}
	for len(work) != 0 {
		block := work[len(work)-1]
		work = work[:len(work)-1]
		if block == target {
			return true
		}
		if seen[block] {
			continue
		}
		seen[block] = true
		work = append(work, graph[block]...)
	}
	return false
}

func reachableBlock(graph map[BlockID][]BlockID, from, target BlockID) bool {
	work := []BlockID{from}
	seen := map[BlockID]bool{}
	for len(work) != 0 {
		block := work[len(work)-1]
		work = work[:len(work)-1]
		if block == target {
			return true
		}
		if seen[block] {
			continue
		}
		seen[block] = true
		work = append(work, graph[block]...)
	}
	return false
}

func verifyTryHandlers(fn *Function) error {
	type handlerGroup struct {
		okCount             int
		catchAll            bool
		catchAllIndex       int
		none                bool
		errHandlers         bool
		highestVariantIndex int
		variants            map[string]bool
		exhaustive          bool
		residual            bool
	}
	groups := map[BlockID]*handlerGroup{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.TryHandlerKind == "" {
				continue
			}
			if op.TryHandlerIndex < -1 {
				return fmt.Errorf("try handler index must be >= -1")
			}
			switch op.TryHandlerKind {
			case TryHandlerOK, TryHandlerErrVariant, TryHandlerErrCatchAll, TryHandlerMerge, TryHandlerResidual, TryHandlerNone:
			default:
				return fmt.Errorf("invalid try handler kind %q", op.TryHandlerKind)
			}
			if op.Kind == OpCoreErrorIsVariant && (op.TryHandlerKind != TryHandlerErrVariant || op.Variant == "") {
				return fmt.Errorf("core error variant test lacks matching try handler provenance")
			}
			if op.Kind != OpBranch || len(op.Successors) != 1 {
				continue
			}
			group := groups[op.Successors[0].Block]
			if group == nil {
				group = &handlerGroup{highestVariantIndex: -1, catchAllIndex: -1, variants: map[string]bool{}}
				groups[op.Successors[0].Block] = group
			}
			group.residual = group.residual || op.TryResidualPropagates
			switch op.TryHandlerKind {
			case TryHandlerNone:
				group.none = true
			case TryHandlerErrVariant, TryHandlerErrCatchAll:
				group.errHandlers = true
			}
			if group.none && group.errHandlers {
				return fmt.Errorf("try merge ^%d mixes Option None and Result Err handlers", op.Successors[0].Block)
			}
			if op.TryHandlerGuarded {
				// A guarded handler never covers its pattern, so it neither
				// counts as the catch-all nor claims its variant.
				group.exhaustive = group.exhaustive || op.TryHandlerExhaustive
				continue
			}
			switch op.TryHandlerKind {
			case TryHandlerOK:
				group.okCount++
			case TryHandlerNone:
				if group.catchAll {
					return fmt.Errorf("duplicate unguarded None handler for try merge ^%d", op.Successors[0].Block)
				}
				group.catchAll = true
				group.catchAllIndex = op.TryHandlerIndex
				group.exhaustive = group.exhaustive || op.TryHandlerExhaustive
			case TryHandlerErrCatchAll:
				if group.catchAll {
					return fmt.Errorf("duplicate Err catch-all for try merge ^%d", op.Successors[0].Block)
				}
				group.catchAll = true
				group.catchAllIndex = op.TryHandlerIndex
				group.exhaustive = group.exhaustive || op.TryHandlerExhaustive
			case TryHandlerErrVariant:
				if op.Variant == "" || group.variants[op.Variant] {
					return fmt.Errorf("duplicate or missing Err variant for try merge ^%d", op.Successors[0].Block)
				}
				group.variants[op.Variant] = true
				group.exhaustive = group.exhaustive || op.TryHandlerExhaustive
				if op.TryHandlerIndex > group.highestVariantIndex {
					group.highestVariantIndex = op.TryHandlerIndex
				}
			}
		}
	}
	for merge, group := range groups {
		if len(group.variants) == 0 && !group.catchAll && !group.residual && !group.exhaustive {
			continue
		}
		if group.okCount != 1 {
			return fmt.Errorf("try merge ^%d must have exactly one implicit or explicit Ok edge", merge)
		}
		if !group.exhaustive && !group.residual {
			return fmt.Errorf("try merge ^%d is missing exhaustive handler provenance", merge)
		}
		if group.catchAll {
			if group.catchAllIndex <= group.highestVariantIndex {
				return fmt.Errorf("Err catch-all must follow specific handlers for try merge ^%d", merge)
			}
			continue
		}
	}
	return nil
}

func validFallibleArithmeticFailureBlock(block *Block, reason ValueID) bool {
	if len(block.Operations) != 3 {
		return false
	}
	convert, construct, ret := block.Operations[0], block.Operations[1], block.Operations[2]
	return convert.Kind == OpArithmeticErrorFromReason && len(convert.Operands) == 1 && convert.Operands[0] == reason && len(convert.Results) == 1 &&
		construct.Kind == OpResultErr && len(construct.Operands) == 1 && construct.Operands[0] == convert.Results[0].ID && len(construct.Results) == 1 &&
		ret.Kind == OpReturn && len(ret.Operands) == 1 && ret.Operands[0] == construct.Results[0].ID
}

func expectedFailureCategory(op Operation) ArithmeticFailureCategory {
	if op.Kind == OpIntShiftChecked {
		return ArithmeticFailureShift
	}
	if op.Kind == OpIntBinaryChecked {
		switch op.IntegerBinary {
		case IntegerCheckedDivide:
			return ArithmeticFailureDivision
		case IntegerCheckedRemainder:
			return ArithmeticFailureRemainder
		}
	}
	return ArithmeticFailureOverflow
}
func verifyTarget(target BranchTarget, blocks map[BlockID]*Block, values map[ValueID]Value) error {
	block := blocks[target.Block]
	if block == nil {
		return fmt.Errorf("missing successor ^%d", target.Block)
	}
	if len(target.Arguments) != len(block.Parameters) {
		return fmt.Errorf("branch argument arity mismatch")
	}
	for i, id := range target.Arguments {
		v, ok := values[id]
		if !ok || v.Type != block.Parameters[i].Type {
			return fmt.Errorf("branch argument type mismatch")
		}
	}
	return nil
}
func typeHasKind(table *TypeTable, id TypeID, kinds ...TypeKind) bool {
	t, ok := table.Lookup(id)
	if !ok {
		return false
	}
	if t.Kind == TypeNamed {
		return typeHasKind(table, t.Base, kinds...)
	}
	for _, kind := range kinds {
		if t.Kind == kind {
			return true
		}
	}
	return false
}
func dominators(entry BlockID, blocks map[BlockID]*Block, preds map[BlockID][]BlockID) map[BlockID]map[BlockID]bool {
	all := map[BlockID]bool{}
	for id := range blocks {
		all[id] = true
	}
	dom := map[BlockID]map[BlockID]bool{}
	for id := range blocks {
		dom[id] = map[BlockID]bool{}
		if id == entry {
			dom[id][id] = true
		} else {
			for x := range all {
				dom[id][x] = true
			}
		}
	}
	changed := true
	for changed {
		changed = false
		for id := range blocks {
			if id == entry {
				continue
			}
			next := map[BlockID]bool{id: true}
			if ps := preds[id]; len(ps) > 0 {
				for x := range dom[ps[0]] {
					keep := true
					for _, p := range ps[1:] {
						if !dom[p][x] {
							keep = false
							break
						}
					}
					if keep {
						next[x] = true
					}
				}
			}
			if !sameSet(dom[id], next) {
				dom[id] = next
				changed = true
			}
		}
	}
	return dom
}
func sameSet(a, b map[BlockID]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for x := range a {
		if !b[x] {
			return false
		}
	}
	return true
}
func verifyStorageDominance(fn *Function, blocks map[BlockID]*Block, dom map[BlockID]map[BlockID]bool) error {
	type position struct {
		block BlockID
		index int
	}
	decl := map[StorageID]position{}
	init := map[StorageID]position{}
	for _, b := range fn.Blocks {
		for index, op := range b.Operations {
			switch op.Kind {
			case OpStorageDeclare:
				if _, ok := decl[op.Storage]; ok {
					return fmt.Errorf("duplicate storage.declare $%d", op.Storage)
				}
				decl[op.Storage] = position{b.ID, index}
			case OpStorageInit:
				if _, ok := init[op.Storage]; ok {
					return fmt.Errorf("duplicate storage.init $%d", op.Storage)
				}
				if d, ok := decl[op.Storage]; !ok || !dominatesPosition(d, position{b.ID, index}, dom) {
					return fmt.Errorf("storage.init before declaration")
				}
				init[op.Storage] = position{b.ID, index}
			case OpStorageLoad, OpStorageStore, OpPlaceStorageRoot:
				d, dok := decl[op.Storage]
				i, iok := init[op.Storage]
				use := position{b.ID, index}
				// An owned value initializes its storage through the storage
				// Place: a root immediately consumed by own.initialize-place
				// is the initialization (P17-28).
				if op.Kind == OpPlaceStorageRoot && !iok && index+1 < len(b.Operations) && len(op.Results) == 1 {
					next := b.Operations[index+1]
					if next.Kind == OpOwnInitializePlace && len(next.Operands) == 2 && next.Operands[0] == op.Results[0].ID {
						if !dok || !dominatesPosition(d, use, dom) {
							return fmt.Errorf("storage declaration does not dominate use")
						}
						init[op.Storage] = use
						continue
					}
				}
				if !dok || !dominatesPosition(d, use, dom) {
					return fmt.Errorf("storage declaration does not dominate use")
				}
				if !iok || !dominatesPosition(i, use, dom) {
					return fmt.Errorf("storage initialization does not dominate use")
				}
			}
		}
	}
	return nil
}

func dominatesPosition(def, use struct {
	block BlockID
	index int
}, dom map[BlockID]map[BlockID]bool) bool {
	if def.block == use.block {
		return def.index < use.index
	}
	return dom[use.block][def.block]
}

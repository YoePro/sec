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
	for _, fn := range module.Functions {
		if err := verifyFunction(module, fn, functions); err != nil {
			return fmt.Errorf("function %s: %w", fn.ID, err)
		}
	}
	return nil
}

func verifyStructDefinitions(module *Module) error {
	seen := map[TypeID]bool{}
	for _, definition := range module.Structs {
		typ, ok := module.Types.Lookup(definition.TypeID)
		if !ok || typ.Kind != TypeStruct || typ.Identity == "" || definition.SymbolID != SymbolID(typ.Identity) || definition.Name != typ.Name || seen[definition.TypeID] {
			return fmt.Errorf("invalid struct definition !%d", definition.TypeID)
		}
		seen[definition.TypeID] = true
		if !sameTypeIDs(definition.TypeArguments, typ.TypeArgs) || !validCopyClassification(definition.CopyClassification) {
			return fmt.Errorf("invalid struct metadata !%d", definition.TypeID)
		}
		names := map[string]bool{}
		for index, field := range definition.Fields {
			if field.ID != StructFieldID(index) || field.Name == "" || names[field.Name] {
				return fmt.Errorf("invalid struct field in !%d", definition.TypeID)
			}
			if _, ok := module.Types.Lookup(field.Type); !ok {
				return fmt.Errorf("struct field %s has invalid type", field.Name)
			}
			names[field.Name] = true
		}
	}
	for index, typ := range module.Types.types {
		if typ.Kind == TypeStruct && !seen[TypeID(index+1)] {
			return fmt.Errorf("struct type !%d has no definition", index+1)
		}
	}
	return nil
}

func verifyTypes(table *TypeTable) error {
	for i, t := range table.types {
		id := TypeID(i + 1)
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
		if err := addValue(module.Types, values, p.Value); err != nil {
			return err
		}
		paramDefs[p.Value.ID] = true
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
	if err := verifyArrayIndexGuards(module, fn, blocks, values, dom); err != nil {
		return err
	}
	return nil
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

func verifyOperation(module *Module, fn *Function, op Operation, values map[ValueID]Value, storages map[StorageID]Storage, functions map[FunctionID]*Function, blocks map[BlockID]*Block) error {
	resultType := func() TypeID {
		if len(op.Results) == 1 {
			return op.Results[0].Type
		}
		return 0
	}()
	switch op.Kind {
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
		if len(op.Operands) != len(callee.Parameters) || len(op.ArgumentActions) != len(op.Operands) {
			return fmt.Errorf("call argument arity mismatch")
		}
		for i, id := range op.Operands {
			if values[id].Type != callee.Parameters[i].Value.Type {
				return fmt.Errorf("call argument type mismatch")
			}
			if op.ArgumentActions[i] != ArgumentCopyTrivial {
				return fmt.Errorf("unsupported call argument action")
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
	case OpIntShiftChecked:
		if !validIntegerShiftKind(op.IntegerShift) || !validCheckedResults(module.Types, op, values, 2) || !isBuiltinIntegerType(module.Types, values[op.Operands[1]].Type) || shiftKindSigned(op.IntegerShift) != isSignedBuiltinIntegerType(module.Types, values[op.Operands[0]].Type) {
			return fmt.Errorf("invalid int.shift-checked")
		}
	case OpIntCompare:
		if !validIntegerComparePredicate(op.IntegerCompare) || len(op.Operands) != 2 || len(op.Results) != 1 || !isBuiltinIntegerType(module.Types, values[op.Operands[0]].Type) || values[op.Operands[0]].Type != values[op.Operands[1]].Type || !typeHasKind(module.Types, op.Results[0].Type, TypeBool) {
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
		if len(op.Operands) != 1 || len(op.Results) != 1 || len(op.StructActions) != 1 || op.StructActions[0] != StructActionCopyTrivial {
			return fmt.Errorf("invalid struct.extract-field")
		}
		definition, ok := structDefinition(module, values[op.Operands[0]].Type)
		if !ok || int(op.StructField) >= len(definition.Fields) || resultType != definition.Fields[op.StructField].Type {
			return fmt.Errorf("struct extract field mismatch")
		}
	case OpStructReplaceField:
		if len(op.Operands) != 2 || len(op.Results) != 1 || resultType != values[op.Operands[0]].Type {
			return fmt.Errorf("invalid struct.replace-field")
		}
		definition, ok := structDefinition(module, resultType)
		if !ok || definition.CopyClassification != "trivial" || !definition.TriviallyDestructible || int(op.StructField) >= len(definition.Fields) || values[op.Operands[1]].Type != definition.Fields[op.StructField].Type {
			return fmt.Errorf("unsafe struct.replace-field")
		}
	case OpArrayConstruct:
		if err := verifyArrayConstruct(module.Types, op, values); err != nil {
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
	case OpBoundsFailure:
		if len(op.Operands) != 0 || len(op.Results) != 0 || len(op.Successors) != 0 || op.ArrayOperation != "fixed-array-index" {
			return fmt.Errorf("invalid fail.bounds")
		}
	default:
		return fmt.Errorf("unknown operation %q", op.Kind)
	}
	return nil
}

func verifyArrayConstruct(types *TypeTable, op Operation, values map[ValueID]Value) error {
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
			if op.ArrayActions[index] != ArrayActionConstructDirect || op.ArraySegmentLengths[index] != "1" || operandType != op.ArrayElementType {
				return fmt.Errorf("array.construct element segment %d mismatch", index)
			}
		case ArraySegmentSpread:
			spread, ok := types.Lookup(operandType)
			if !ok || spread.Kind != TypeArray || spread.Element != op.ArrayElementType || spread.Length != op.ArraySegmentLengths[index] || op.ArrayActions[index] != ArrayActionCopyTrivial {
				return fmt.Errorf("array.construct spread segment %d mismatch", index)
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
	case TypeUint:
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
	case TypeBool, TypeByte, TypeChar, TypeRune, TypeDecimal128, TypeInt, TypeUint, TypeFloat, TypeEnum:
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
		return ok && definition.Defaultable && definition.CopyClassification == "trivial" && definition.TriviallyDestructible
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
	return variant == "Overflow" || variant == "DivisionByZero" || variant == "InvalidShift"
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
			if failure := blocks[branch.Successors[1].Block]; !validArrayBoundsFailureBlock(module, failure, values) {
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

func validArrayBoundsFailureBlock(module *Module, block *Block, values map[ValueID]Value) bool {
	if block == nil || len(block.Operations) == 0 {
		return false
	}
	if len(block.Operations) == 1 {
		failure := block.Operations[0]
		return failure.Kind == OpBoundsFailure && failure.ArrayOperation == "fixed-array-index"
	}
	constant := block.Operations[0]
	if constant.Kind != OpEnumConstant || len(constant.Results) != 1 {
		return false
	}
	definition, ok := enumDefinition(module, constant.Results[0].Type)
	if !ok || definition.SymbolID != "core::IndexError" || int(constant.EnumCase) >= len(definition.Cases) || definition.Cases[constant.EnumCase].Name != "OutOfBounds" {
		return false
	}
	errorValue := constant.Results[0].ID
	construct := block.Operations[1]
	if construct.Kind != OpResultErr || len(construct.Operands) != 1 || construct.Operands[0] != errorValue || len(construct.Results) != 1 {
		return false
	}
	result, ok := module.Types.Lookup(construct.Results[0].Type)
	if !ok || result.Kind != TypeResult || result.Error != values[errorValue].Type {
		return false
	}
	last := block.Operations[len(block.Operations)-1]
	return len(block.Operations) == 3 && last.Kind == OpReturn && len(last.Operands) == 1 && last.Operands[0] == construct.Results[0].ID
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
		highestVariantIndex int
		variants            map[string]bool
		exhaustive          bool
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
			case TryHandlerOK, TryHandlerErrVariant, TryHandlerErrCatchAll, TryHandlerMerge:
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
			switch op.TryHandlerKind {
			case TryHandlerOK:
				group.okCount++
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
		if len(group.variants) == 0 && !group.catchAll {
			continue
		}
		if group.okCount != 1 {
			return fmt.Errorf("try merge ^%d must have exactly one implicit or explicit Ok edge", merge)
		}
		if !group.exhaustive {
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
			case OpStorageLoad, OpStorageStore:
				d, dok := decl[op.Storage]
				i, iok := init[op.Storage]
				use := position{b.ID, index}
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

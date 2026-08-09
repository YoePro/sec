package semantic

import "fmt"

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
	}
	return nil
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
		if len(op.Operands) != 0 || len(op.Results) != 0 || len(op.Successors) != 0 || !validArithmeticFailureCategory(op.FailureCategory) || op.Operator == "" {
			return fmt.Errorf("invalid fail.arithmetic")
		}
	default:
		return fmt.Errorf("unknown operation %q", op.Kind)
	}
	return nil
}

func validCheckedResults(types *TypeTable, op Operation, values map[ValueID]Value, operands int) bool {
	return len(op.Operands) == operands && len(op.Results) == 2 &&
		isBuiltinIntegerType(types, values[op.Operands[0]].Type) &&
		op.Results[0].Type == values[op.Operands[0]].Type &&
		typeHasKind(types, op.Results[1].Type, TypeBool)
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
			if len(op.Results) != 2 || index+1 != len(block.Operations)-1 {
				return fmt.Errorf("checked integer operation in ^%d must be immediately guarded", block.ID)
			}
			failed := op.Results[1].ID
			if uses[failed] != 1 {
				return fmt.Errorf("checked integer failure %%%d must have exactly one use", failed)
			}
			branch := block.Operations[index+1]
			if branch.Kind != OpCondBranch || len(branch.Operands) != 1 || branch.Operands[0] != failed || len(branch.Successors) != 2 {
				return fmt.Errorf("checked integer failure %%%d must guard with conditional branch", failed)
			}
			failureBlock := blocks[branch.Successors[0].Block]
			if failureBlock == nil || len(failureBlock.Parameters) != 0 || len(failureBlock.Operations) != 1 || incoming[failureBlock.ID] != 1 {
				return fmt.Errorf("checked integer failure block must be dedicated")
			}
			failure := failureBlock.Operations[0]
			if failure.Kind != OpArithmeticFailure || failure.FailureCategory != expectedFailureCategory(op) || failure.Operator != op.Operator {
				return fmt.Errorf("checked integer failure endpoint does not match %s", op.Kind)
			}
		}
	}
	return nil
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

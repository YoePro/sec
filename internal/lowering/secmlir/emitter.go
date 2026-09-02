// Package secmlir lowers verified Semantic IR to textual high-level Sec MLIR.
// This package deliberately has no access to AST, parser, lexer, or Sema data.
package secmlir

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
)

const dialectSchemaVersion = 10

type emitter struct {
	module    *semantic.Module
	types     map[semantic.TypeID]string
	visiting  map[semantic.TypeID]bool
	functions map[semantic.FunctionID]string
	plan      layout.ResolvedScalarPlan
	out       strings.Builder
}

// Emit verifies Semantic IR version 1 and emits deterministic Sec MLIR schema
// version 10. It preserves high-level fixed-array values and performs no
// external tool invocation.
//
// Rules:
//   - rules/mlir/lowering-versions/sec_mlir_lowering_v10.md — sections 1-23
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 58-85
func Emit(module *semantic.Module, plan layout.ResolvedScalarPlan) ([]byte, error) {
	if err := semantic.Verify(module); err != nil {
		return nil, fmt.Errorf("verify Semantic IR: %w", err)
	}
	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("validate scalar plan: %w", err)
	}
	if module.Version != semantic.Version {
		return nil, fmt.Errorf("Semantic IR version %d is not supported", module.Version)
	}
	e := &emitter{
		module: module, types: map[semantic.TypeID]string{},
		visiting: map[semantic.TypeID]bool{}, functions: map[semantic.FunctionID]string{}, plan: plan,
	}
	if err := e.validatePackage14ArrayBoundaries(); err != nil {
		return nil, err
	}
	for index, function := range module.Functions {
		e.functions[function.ID] = fmt.Sprintf("sec_fn_%d", index)
	}
	if err := e.emitModule(); err != nil {
		return nil, err
	}
	return []byte(e.out.String()), nil
}

// validatePackage14ArrayBoundaries rejects representation requests that schema
// 10 deliberately leaves to later aggregate ABI and physical-layout packages.
// Verified Semantic IR may retain foreign functions and optional aggregate
// LayoutRef metadata, but neither is permission to select an array ABI/layout.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 81, 85, 103-104
func (e *emitter) validatePackage14ArrayBoundaries() error {
	for _, function := range e.module.Functions {
		if function == nil || !function.Extern {
			continue
		}
		if e.typeContainsFixedArray(function.ReturnType, map[semantic.TypeID]bool{}) {
			return &UnsupportedLoweringError{Feature: "foreign fixed-array ABI return", Function: function.ID}
		}
		for _, parameter := range function.Parameters {
			if e.typeContainsFixedArray(parameter.Value.Type, map[semantic.TypeID]bool{}) {
				return &UnsupportedLoweringError{Feature: "foreign fixed-array ABI parameter", Function: function.ID}
			}
		}
	}
	for _, definition := range e.module.Structs {
		if definition.LayoutRef != "" && e.typeContainsFixedArray(definition.TypeID, map[semantic.TypeID]bool{}) {
			return &UnsupportedLoweringError{Feature: "physical fixed-array layout request " + definition.LayoutRef}
		}
	}
	for _, definition := range e.module.Unions {
		if definition.LayoutRef != "" && e.typeContainsFixedArray(definition.TypeID, map[semantic.TypeID]bool{}) {
			return &UnsupportedLoweringError{Feature: "physical fixed-array layout request " + definition.LayoutRef}
		}
	}
	return nil
}

// typeContainsFixedArray follows semantic type identity and declaration-order
// aggregate metadata without computing size, alignment, stride, or ABI shape.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 6, 27, 85
func (e *emitter) typeContainsFixedArray(id semantic.TypeID, visiting map[semantic.TypeID]bool) bool {
	if id == 0 || visiting[id] {
		return false
	}
	visiting[id] = true
	defer delete(visiting, id)
	typ, ok := e.module.Types.Lookup(id)
	if !ok {
		return false
	}
	switch typ.Kind {
	case semantic.TypeArray:
		return true
	case semantic.TypeNamed:
		return e.typeContainsFixedArray(typ.Base, visiting)
	case semantic.TypeResult:
		return e.typeContainsFixedArray(typ.Success, visiting) || e.typeContainsFixedArray(typ.Error, visiting)
	case semantic.TypeStruct:
		if definition, found := e.structDefinition(id); found {
			for _, field := range definition.Fields {
				if e.typeContainsFixedArray(field.Type, visiting) {
					return true
				}
			}
		}
	case semantic.TypeUnion:
		if definition, found := e.unionDefinition(id); found {
			for _, variant := range definition.Variants {
				if e.typeContainsFixedArray(variant.Payload, visiting) {
					return true
				}
				for _, field := range variant.PayloadFields {
					if e.typeContainsFixedArray(field.Type, visiting) {
						return true
					}
				}
			}
		}
	}
	for _, argument := range typ.TypeArgs {
		if e.typeContainsFixedArray(argument, visiting) {
			return true
		}
	}
	return false
}

func (e *emitter) emitModule() error {
	e.out.WriteString("module attributes {")
	fmt.Fprintf(&e.out, "sec.dialect_version = %d : i32, ", dialectSchemaVersion)
	fmt.Fprintf(&e.out, "sec.semantic_ir_version = %d : i32, ", semantic.Version)
	fmt.Fprintf(&e.out, "sec.module_id = %s, ", mlirString(e.module.Identity))
	fmt.Fprintf(&e.out, "sec.target_os = %s, sec.target_arch = %s, ", mlirString(e.plan.TargetOS), mlirString(e.plan.TargetArch))
	fmt.Fprintf(&e.out, "sec.target_triple = %s, sec.target_abi = %s, ", mlirString(e.plan.LLVMTriple), mlirString(e.plan.ABI))
	fmt.Fprintf(&e.out, "sec.target_profile = %s, sec.target_endianness = %s, ", mlirString(e.plan.Profile), mlirString(string(e.plan.Endianness)))
	fmt.Fprintf(&e.out, "dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, %d>>, sec.source_files = [", e.plan.PointerWidthBits)
	for index, source := range e.module.SourceFiles {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(mlirString(source))
	}
	e.out.WriteString("]} {\n")
	for _, function := range e.module.Functions {
		if err := e.emitFunction(function); err != nil {
			return err
		}
	}
	e.out.WriteString("}\n")
	return nil
}

func (e *emitter) emitFunction(function *semantic.Function) error {
	symbol := e.functions[function.ID]
	e.out.WriteString("  func.func ")
	if function.Extern {
		e.out.WriteString("private ")
	}
	fmt.Fprintf(&e.out, "@%s(", symbol)
	for index, parameter := range function.Parameters {
		if index > 0 {
			e.out.WriteString(", ")
		}
		typeText, err := e.typeText(parameter.Value.Type)
		if err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "%s: %s", valueName(parameter.Value.ID), typeText)
		if scalarKind, ok := e.scalarKind(parameter.Value.Type); ok {
			fmt.Fprintf(&e.out, " {sec.scalar_kind = %s}", mlirString(scalarKind))
		}
	}
	e.out.WriteByte(')')
	if !e.isVoid(function.ReturnType) {
		returnType, err := e.typeText(function.ReturnType)
		if err != nil {
			return err
		}
		e.out.WriteString(" -> (")
		e.out.WriteString(returnType)
		if scalarKind, ok := e.scalarKind(function.ReturnType); ok {
			fmt.Fprintf(&e.out, " {sec.scalar_kind = %s}", mlirString(scalarKind))
		}
		e.out.WriteByte(')')
	}
	e.out.WriteString(" attributes {")
	fmt.Fprintf(&e.out, "sec.function_id = %s, sec.source_name = %s, sec.extern = %t, sec.unsafe = %t",
		mlirString(string(function.ID)), mlirString(function.Name), function.Extern, function.Unsafe)
	if function.LinkName != "" {
		fmt.Fprintf(&e.out, ", sec.link_name = %s", mlirString(function.LinkName))
	}
	if function.ABI != "" {
		fmt.Fprintf(&e.out, ", sec.abi = %s", mlirString(function.ABI))
	}
	if len(function.Parameters) > 0 {
		e.out.WriteString(", sec.parameter_names = [")
		for index, parameter := range function.Parameters {
			if index > 0 {
				e.out.WriteString(", ")
			}
			e.out.WriteString(mlirString(parameter.Name))
		}
		e.out.WriteByte(']')
	}
	e.out.WriteByte('}')
	if function.Extern {
		e.emitLocation(function.Location)
		e.out.WriteByte('\n')
		return nil
	}
	e.out.WriteString(" {\n")
	values := collectValues(function)
	storages := map[semantic.StorageID]semantic.Storage{}
	for _, storage := range function.Storages {
		storages[storage.ID] = storage
	}
	entry, ordered, err := orderedBlocks(function)
	if err != nil {
		return err
	}
	if len(entry.Parameters) != 0 {
		return &UnsupportedLoweringError{Feature: "entry block parameters", Function: function.ID}
	}
	for blockIndex, block := range ordered {
		if blockIndex > 0 {
			fmt.Fprintf(&e.out, "  ^bb%d", block.ID)
			if len(block.Parameters) > 0 {
				e.out.WriteByte('(')
				for index, parameter := range block.Parameters {
					if index > 0 {
						e.out.WriteString(", ")
					}
					typeText, typeErr := e.typeText(parameter.Type)
					if typeErr != nil {
						return typeErr
					}
					fmt.Fprintf(&e.out, "%s: %s", valueName(parameter.ID), typeText)
				}
				e.out.WriteByte(')')
			}
			e.out.WriteString(":\n")
		}
		for _, operation := range block.Operations {
			if err := e.emitOperation(function, operation, values, storages); err != nil {
				return err
			}
		}
	}
	e.out.WriteString("  }")
	e.emitLocation(function.Location)
	e.out.WriteByte('\n')
	return nil
}

func (e *emitter) emitOperation(function *semantic.Function, operation semantic.Operation, values map[semantic.ValueID]semantic.Value, storages map[semantic.StorageID]semantic.Storage) error {
	e.out.WriteString("    ")
	switch operation.Kind {
	case semantic.OpConstInt:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.const.int\"() <{value = %s}> : () -> ", mlirString(operation.Integer.String()))
		return e.finishResultOperation(operation)
	case semantic.OpConstBool:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.const.bool\"() <{value = %t}> : () -> ", *operation.Bool)
		return e.finishResultOperation(operation)
	case semantic.OpConstFloat:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.const.float\"() <{lexeme = %s}> : () -> ", mlirString(operation.FloatLexeme))
		return e.finishResultOperation(operation)
	case semantic.OpConstDecimal:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.const.decimal\"() <{coefficient = %s, lexeme = %s, scale = %d : i32}> : () -> ",
			mlirString(operation.Decimal.Coefficient.String()), mlirString(operation.Decimal.Lexeme), operation.Decimal.Scale)
		return e.finishResultOperation(operation)
	case semantic.OpConstString:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.const.string\"() <{value = %s}> : () -> ", mlirString(operation.String))
		return e.finishResultOperation(operation)
	case semantic.OpStorageDeclare:
		storage, ok := storages[operation.Storage]
		if !ok {
			return fmt.Errorf("missing storage %d", operation.Storage)
		}
		elementType, err := e.typeText(storage.Type)
		if err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "%s = \"sec.storage.declare\"() {sec.mutable = %t, sec.source_name = %s, sec.storage_class = %s, sec.storage_id = %d : i32",
			storageName(storage.ID), storage.Mutable, mlirString(storage.Name), mlirString(string(storage.Class)), storage.ID)
		if scalarKind, ok := e.scalarKind(storage.Type); ok {
			fmt.Fprintf(&e.out, ", sec.scalar_kind = %s", mlirString(scalarKind))
		}
		fmt.Fprintf(&e.out, "} : () -> !sec.storage<%s>", elementType)
	case semantic.OpStorageInit, semantic.OpStorageStore:
		storage, ok := storages[operation.Storage]
		if !ok || len(operation.Operands) != 1 {
			return fmt.Errorf("invalid %s storage operands", operation.Kind)
		}
		elementType, err := e.typeText(storage.Type)
		if err != nil {
			return err
		}
		name := "sec.storage.init"
		if operation.Kind == semantic.OpStorageStore {
			name = "sec.storage.store"
		}
		fmt.Fprintf(&e.out, "\"%s\"(%s, %s) : (!sec.storage<%s>, %s) -> ()", name, storageName(storage.ID), valueName(operation.Operands[0]), elementType, elementType)
	case semantic.OpStorageLoad:
		storage, ok := storages[operation.Storage]
		if !ok {
			return fmt.Errorf("missing storage %d", operation.Storage)
		}
		if err := e.emitResult(operation); err != nil {
			return err
		}
		elementType, err := e.typeText(storage.Type)
		if err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.storage.load\"(%s) : (!sec.storage<%s>) -> %s", storageName(storage.ID), elementType, elementType)
	case semantic.OpDirectCall, semantic.OpForeignCall:
		if err := e.emitOptionalResult(operation); err != nil {
			return err
		}
		callName := "sec.call.direct"
		if operation.Kind == semantic.OpForeignCall {
			callName = "sec.call.foreign"
		}
		target, ok := e.functions[operation.Callee]
		if !ok {
			return fmt.Errorf("missing symbol for callee %s", operation.Callee)
		}
		fmt.Fprintf(&e.out, "\"%s\"(", callName)
		for index, operand := range operation.Operands {
			if index > 0 {
				e.out.WriteString(", ")
			}
			e.out.WriteString(valueName(operand))
		}
		fmt.Fprintf(&e.out, ") <{callee = @%s}> {sec.argument_actions = [", target)
		for index, action := range operation.ArgumentActions {
			if index > 0 {
				e.out.WriteString(", ")
			}
			e.out.WriteString(mlirString(string(action)))
		}
		e.out.WriteString("]} : (")
		if err := e.emitOperandTypes(operation.Operands, values); err != nil {
			return err
		}
		e.out.WriteString(") -> ")
		if err := e.emitResultTypes(operation.Results); err != nil {
			return err
		}
	case semantic.OpUnreachable:
		fmt.Fprintf(&e.out, "\"sec.unreachable\"() <{reason = %s}> ", mlirString(operation.Reason))
		e.emitOperationAttributes(operation, true)
		e.out.WriteString(": () -> ()")
	case semantic.OpReturn:
		if operation.MatchID != 0 {
			e.out.WriteString("\"func.return\"(")
			if len(operation.Operands) == 1 {
				e.out.WriteString(valueName(operation.Operands[0]))
			}
			e.out.WriteString(") ")
			e.emitOperationAttributes(operation, false)
			e.out.WriteString(": (")
			if err := e.emitOperandTypes(operation.Operands, values); err != nil {
				return err
			}
			e.out.WriteString(") -> ()")
			break
		}
		if operation.TryHandlerKind != "" {
			e.out.WriteString("\"func.return\"(")
			if len(operation.Operands) == 1 {
				e.out.WriteString(valueName(operation.Operands[0]))
			}
			e.out.WriteString(") ")
			e.emitTryHandlerAttributes(operation)
			e.out.WriteString(": (")
			if err := e.emitOperandTypes(operation.Operands, values); err != nil {
				return err
			}
			e.out.WriteString(") -> ()")
			break
		}
		e.out.WriteString("return")
		if len(operation.Operands) == 1 {
			typeText, err := e.valueTypeText(operation.Operands[0], values)
			if err != nil {
				return err
			}
			fmt.Fprintf(&e.out, " %s : %s", valueName(operation.Operands[0]), typeText)
		}
	case semantic.OpBranch:
		if len(operation.Successors) != 1 {
			return fmt.Errorf("invalid branch successor count")
		}
		e.out.WriteString("cf.br ")
		if err := e.emitTarget(operation.Successors[0], values); err != nil {
			return err
		}
		e.out.WriteByte(' ')
		e.emitOperationAttributes(operation, false)
	case semantic.OpCondBranch:
		if len(operation.Operands) != 1 || len(operation.Successors) != 2 {
			return fmt.Errorf("invalid conditional branch")
		}
		fmt.Fprintf(&e.out, "cf.cond_br %s, ", valueName(operation.Operands[0]))
		if err := e.emitTarget(operation.Successors[0], values); err != nil {
			return err
		}
		e.out.WriteString(", ")
		if err := e.emitTarget(operation.Successors[1], values); err != nil {
			return err
		}
		e.out.WriteByte(' ')
		e.emitOperationAttributes(operation, false)
	case semantic.OpIntUnaryPlus:
		return e.emitSemanticIntegerOperation(operation, "sec.int.unary_plus", "", "", values)
	case semantic.OpIntNegChecked:
		return e.emitSemanticIntegerOperation(operation, "sec.int.neg_checked", "", "", values)
	case semantic.OpIntBitNot:
		return e.emitSemanticIntegerOperation(operation, "sec.int.bit_not", "", "", values)
	case semantic.OpIntBinaryChecked:
		return e.emitSemanticIntegerOperation(operation, "sec.int.binary_checked", "kind", string(operation.IntegerBinary), values)
	case semantic.OpIntBitwise:
		return e.emitSemanticIntegerOperation(operation, "sec.int.bitwise", "kind", string(operation.IntegerBitwise), values)
	case semantic.OpIntShiftChecked:
		return e.emitSemanticIntegerOperation(operation, "sec.int.shift_checked", "kind", string(operation.IntegerShift), values)
	case semantic.OpIntCompare:
		return e.emitSemanticIntegerOperation(operation, "sec.int.cmp", "predicate", string(operation.IntegerCompare), values)
	case semantic.OpArithmeticFailure:
		if len(operation.Operands) != 1 {
			return fmt.Errorf("invalid arithmetic failure operand")
		}
		typeText, err := e.valueTypeText(operation.Operands[0], values)
		if err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.fail.arithmetic\"(%s) {sec.operator = %s} : (%s) -> ()", valueName(operation.Operands[0]), mlirString(operation.Operator), typeText)
	case semantic.OpArithmeticFailureReasonConstant:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.arithmetic_failure_reason.constant\"() <{value = %s}> : () -> ", mlirString(string(operation.FailureReason)))
		return e.finishResultOperation(operation)
	case semantic.OpArithmeticErrorFromReason:
		return e.emitUnarySemanticOperation(operation, "sec.arithmetic_error.from_reason", values)
	case semantic.OpResultOk:
		return e.emitResultConstructor(operation, "sec.result.ok", values)
	case semantic.OpResultErr:
		return e.emitResultConstructor(operation, "sec.result.err", values)
	case semantic.OpResultIsErr:
		return e.emitUnarySemanticOperation(operation, "sec.result.is_err", values)
	case semantic.OpResultUnwrapOk:
		return e.emitUnarySemanticOperation(operation, "sec.result.unwrap_ok", values)
	case semantic.OpResultUnwrapErr:
		return e.emitUnarySemanticOperation(operation, "sec.result.unwrap_err", values)
	case semantic.OpCoreErrorIsVariant:
		return e.emitCoreErrorIsVariant(operation, values)
	case semantic.OpEnumConstant:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.enum.constant\"() <{caseOrdinal = %d : i32}> ", operation.EnumCase)
		e.emitOperationAttributes(operation, false)
		e.out.WriteString(": () -> ")
		return e.finishResultOperation(operation)
	case semantic.OpEnumFromInteger:
		return e.emitUnarySemanticOperation(operation, "sec.enum.from_integer", values)
	case semantic.OpEnumToInteger:
		return e.emitUnarySemanticOperation(operation, "sec.enum.to_integer", values)
	case semantic.OpEnumCompare:
		return e.emitSemanticIntegerOperation(operation, "sec.enum.cmp", "predicate", string(operation.IntegerCompare), values)
	case semantic.OpUnionConstruct:
		return e.emitUnionConstruct(operation, values)
	case semantic.OpUnionIsVariant:
		return e.emitUnionUnary(operation, "sec.union.is_variant", values)
	case semantic.OpUnionUnwrapPayload:
		return e.emitUnionUnwrap(operation, "sec.union.unwrap_payload", values)
	case semantic.OpUnionUnwrapField:
		return e.emitUnionUnwrap(operation, "sec.union.unwrap_field", values)
	case semantic.OpStructConstruct:
		return e.emitStructConstruct(operation, values)
	case semantic.OpStructSpreadFields:
		return e.emitStructSpreadFields(operation, values)
	case semantic.OpStructExtractField:
		return e.emitStructExtract(operation, values)
	case semantic.OpStructReplaceField:
		return e.emitStructReplaceField(operation, values)
	case semantic.OpArrayConstruct:
		return e.emitArrayConstruct(operation, values)
	case semantic.OpArrayDefault:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		e.out.WriteString("\"sec.array.default\"() : () -> ")
		return e.finishResultOperation(operation)
	case semantic.OpArrayLength:
		return e.emitUnarySemanticOperation(operation, "sec.array.len", values)
	case semantic.OpArrayIndexInBounds:
		return e.emitArrayIndexInBounds(operation, values)
	case semantic.OpArrayExtract:
		return e.emitArrayExtract(operation, values)
	case semantic.OpArrayReplace:
		return e.emitArrayReplace(operation, values)
	case semantic.OpBoundsFailure:
		fmt.Fprintf(&e.out, "\"sec.fail.bounds\"() {operation = %s} : () -> ()", mlirString(operation.ArrayOperation))
	default:
		return &UnsupportedLoweringError{Feature: string(operation.Kind), Function: function.ID}
	}
	e.emitLocation(operation.Location)
	e.out.WriteByte('\n')
	return nil
}

// The fixed-array helpers below implement the schema-v10 operation mapping
// without expanding aggregate values or selecting physical array layout.
//
// Rules:
//   - rules/mlir/lowering-versions/sec_mlir_lowering_v10.md — sections 2-13
//   - rules/mlir/dialect-versions/sec_mlir_dialect_v10.md — sections 7-21
func (e *emitter) emitArrayConstruct(operation semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if err := e.emitResult(operation); err != nil {
		return err
	}
	e.out.WriteString("\"sec.array.construct\"(")
	for index, operand := range operation.Operands {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(valueName(operand))
	}
	e.out.WriteString(") <{segment_actions = [")
	for index, action := range operation.ArrayActions {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(mlirString(string(action)))
	}
	e.out.WriteString("], segment_kinds = [")
	for index, kind := range operation.ArraySegmentKinds {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(mlirString(string(kind)))
	}
	e.out.WriteString("], segment_lengths = [")
	for index, length := range operation.ArraySegmentLengths {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(mlirString(length))
	}
	e.out.WriteString("]}> : (")
	if err := e.emitOperandTypes(operation.Operands, values); err != nil {
		return err
	}
	e.out.WriteString(") -> ")
	return e.finishResultOperation(operation)
}

func (e *emitter) emitArrayIndexInBounds(operation semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if err := e.emitResult(operation); err != nil {
		return err
	}
	fmt.Fprintf(&e.out, "\"sec.array.index_in_bounds\"(%s, %s) <{index_signed = %t}> : (", valueName(operation.Operands[0]), valueName(operation.Operands[1]), operation.ArrayIndexSigned)
	if err := e.emitOperandTypes(operation.Operands, values); err != nil {
		return err
	}
	e.out.WriteString(") -> ")
	return e.finishResultOperation(operation)
}

func (e *emitter) emitArrayExtract(operation semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if err := e.emitResult(operation); err != nil {
		return err
	}
	fmt.Fprintf(&e.out, "\"sec.array.extract\"(%s, %s) <{action = %s, bounds_kind = %s, bounds_proof = %s}> : (",
		valueName(operation.Operands[0]), valueName(operation.Operands[1]), mlirString(string(operation.ArrayActions[0])),
		mlirString(string(operation.ArrayCheckKind)), mlirString(string(operation.ArrayProofKind)))
	if err := e.emitOperandTypes(operation.Operands, values); err != nil {
		return err
	}
	e.out.WriteString(") -> ")
	return e.finishResultOperation(operation)
}

func (e *emitter) emitArrayReplace(operation semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if err := e.emitResult(operation); err != nil {
		return err
	}
	fmt.Fprintf(&e.out, "\"sec.array.replace\"(%s, %s, %s) <{bounds_kind = %s, bounds_proof = %s}> : (",
		valueName(operation.Operands[0]), valueName(operation.Operands[1]), valueName(operation.Operands[2]),
		mlirString(string(operation.ArrayCheckKind)), mlirString(string(operation.ArrayProofKind)))
	if err := e.emitOperandTypes(operation.Operands, values); err != nil {
		return err
	}
	e.out.WriteString(") -> ")
	return e.finishResultOperation(operation)
}

// The four helpers below implement the schema-v9 struct operations specified
// by rules/mlir/dialect-versions/sec_mlir_dialect_v9.md sections 7-13.
func (e *emitter) emitStructConstruct(operation semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if len(operation.Results) != 1 || len(operation.StructOrigins) != len(operation.Operands) || len(operation.StructActions) != len(operation.Operands) {
		return fmt.Errorf("invalid %s", operation.Kind)
	}
	if err := e.emitResult(operation); err != nil {
		return err
	}
	e.out.WriteString("\"sec.struct.construct\"(")
	for index, operand := range operation.Operands {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(valueName(operand))
	}
	e.out.WriteString(") <{field_actions = [")
	for index, action := range operation.StructActions {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(mlirString(string(action)))
	}
	e.out.WriteString("], field_origins = [")
	for index, origin := range operation.StructOrigins {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(mlirString(string(origin)))
	}
	e.out.WriteString("]}> : (")
	if err := e.emitOperandTypes(operation.Operands, values); err != nil {
		return err
	}
	e.out.WriteString(") -> ")
	return e.finishResultOperation(operation)
}

func (e *emitter) emitStructSpreadFields(operation semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if len(operation.Operands) != 1 || len(operation.Results) != len(operation.StructActions) {
		return fmt.Errorf("invalid %s", operation.Kind)
	}
	if err := e.emitResults(operation); err != nil {
		return err
	}
	fmt.Fprintf(&e.out, "\"sec.struct.spread_fields\"(%s) <{actions = [", valueName(operation.Operands[0]))
	for index, action := range operation.StructActions {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(mlirString(string(action)))
	}
	e.out.WriteString("]}> : (")
	if err := e.emitOperandTypes(operation.Operands, values); err != nil {
		return err
	}
	e.out.WriteString(") -> ")
	if err := e.emitResultTypes(operation.Results); err != nil {
		return err
	}
	e.emitLocation(operation.Location)
	e.out.WriteByte('\n')
	return nil
}

func (e *emitter) emitStructExtract(operation semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if len(operation.Operands) != 1 || len(operation.Results) != 1 || len(operation.StructActions) != 1 {
		return fmt.Errorf("invalid %s", operation.Kind)
	}
	if err := e.emitResult(operation); err != nil {
		return err
	}
	fmt.Fprintf(&e.out, "\"sec.struct.extract\"(%s) <{action = %s, field = %d : i32}> : (", valueName(operation.Operands[0]), mlirString(string(operation.StructActions[0])), operation.StructField)
	if err := e.emitOperandTypes(operation.Operands, values); err != nil {
		return err
	}
	e.out.WriteString(") -> ")
	return e.finishResultOperation(operation)
}

func (e *emitter) emitStructReplaceField(operation semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if len(operation.Operands) != 2 || len(operation.Results) != 1 {
		return fmt.Errorf("invalid %s", operation.Kind)
	}
	if err := e.emitResult(operation); err != nil {
		return err
	}
	fmt.Fprintf(&e.out, "\"sec.struct.replace_field\"(%s, %s) <{field = %d : i32}> : (", valueName(operation.Operands[0]), valueName(operation.Operands[1]), operation.StructField)
	if err := e.emitOperandTypes(operation.Operands, values); err != nil {
		return err
	}
	e.out.WriteString(") -> ")
	return e.finishResultOperation(operation)
}

func (e *emitter) emitUnionConstruct(operation semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if len(operation.Results) != 1 || len(operation.PayloadActions) != len(operation.Operands) {
		return fmt.Errorf("invalid %s", operation.Kind)
	}
	if err := e.emitResult(operation); err != nil {
		return err
	}
	e.out.WriteString("\"sec.union.construct\"(")
	for index, operand := range operation.Operands {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(valueName(operand))
	}
	fmt.Fprintf(&e.out, ") <{fieldNames = [")
	for index, field := range operation.UnionFields {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(mlirString(field))
	}
	e.out.WriteString("], payloadActions = [")
	for index, action := range operation.PayloadActions {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(mlirString(string(action)))
	}
	fmt.Fprintf(&e.out, "], variant = %d : i32}> : (", operation.UnionVariant)
	if err := e.emitOperandTypes(operation.Operands, values); err != nil {
		return err
	}
	e.out.WriteString(") -> ")
	return e.finishResultOperation(operation)
}

func (e *emitter) emitUnionUnary(operation semantic.Operation, name string, values map[semantic.ValueID]semantic.Value) error {
	if len(operation.Operands) != 1 || len(operation.Results) != 1 {
		return fmt.Errorf("invalid %s", operation.Kind)
	}
	if err := e.emitResult(operation); err != nil {
		return err
	}
	operandType, err := e.valueTypeText(operation.Operands[0], values)
	if err != nil {
		return err
	}
	resultType, err := e.typeText(operation.Results[0].Type)
	if err != nil {
		return err
	}
	fmt.Fprintf(&e.out, "\"%s\"(%s) <{variant = %d : i32}> ", name, valueName(operation.Operands[0]), operation.UnionVariant)
	e.emitOperationAttributes(operation, false)
	fmt.Fprintf(&e.out, ": (%s) -> %s", operandType, resultType)
	e.emitLocation(operation.Location)
	e.out.WriteByte('\n')
	return nil
}

func (e *emitter) emitUnionUnwrap(operation semantic.Operation, name string, values map[semantic.ValueID]semantic.Value) error {
	if len(operation.Operands) != 1 || len(operation.Results) != 1 || len(operation.PayloadActions) != 1 {
		return fmt.Errorf("invalid %s", operation.Kind)
	}
	if err := e.emitResult(operation); err != nil {
		return err
	}
	operandType, err := e.valueTypeText(operation.Operands[0], values)
	if err != nil {
		return err
	}
	resultType, err := e.typeText(operation.Results[0].Type)
	if err != nil {
		return err
	}
	fmt.Fprintf(&e.out, "\"%s\"(%s) <{", name, valueName(operation.Operands[0]))
	if operation.Kind == semantic.OpUnionUnwrapField {
		fmt.Fprintf(&e.out, "field = %s, ", mlirString(operation.UnionField))
	}
	fmt.Fprintf(&e.out, "payloadAction = %s, variant = %d : i32}> ", mlirString(string(operation.PayloadActions[0])), operation.UnionVariant)
	e.emitOperationAttributes(operation, false)
	fmt.Fprintf(&e.out, ": (%s) -> %s", operandType, resultType)
	e.emitLocation(operation.Location)
	e.out.WriteByte('\n')
	return nil
}

func (e *emitter) emitCoreErrorIsVariant(operation semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if len(operation.Operands) != 1 || len(operation.Results) != 1 || operation.Variant == "" {
		return fmt.Errorf("invalid %s", operation.Kind)
	}
	if err := e.emitResult(operation); err != nil {
		return err
	}
	operandType, err := e.valueTypeText(operation.Operands[0], values)
	if err != nil {
		return err
	}
	resultType, err := e.typeText(operation.Results[0].Type)
	if err != nil {
		return err
	}
	fmt.Fprintf(&e.out, "\"sec.core_error.is_variant\"(%s) <{variant = %s}> ", valueName(operation.Operands[0]), mlirString(operation.Variant))
	e.emitTryHandlerAttributes(operation)
	fmt.Fprintf(&e.out, ": (%s) -> %s", operandType, resultType)
	e.emitLocation(operation.Location)
	e.out.WriteByte('\n')
	return nil
}

func (e *emitter) emitTryHandlerAttributes(operation semantic.Operation) {
	if operation.TryHandlerKind == "" {
		return
	}
	fmt.Fprintf(&e.out, "{sec.try_handler_kind = %s, sec.try_handler_index = %d : i32", mlirString(string(operation.TryHandlerKind)), operation.TryHandlerIndex)
	if operation.TryHandlerExhaustive {
		e.out.WriteString(", sec.try_handler_exhaustive = true")
	}
	if operation.Variant != "" {
		fmt.Fprintf(&e.out, ", sec.try_handler_variant = %s", mlirString(operation.Variant))
	}
	e.out.WriteString("} ")
}

func (e *emitter) emitOperationAttributes(operation semantic.Operation, synthesized bool) {
	if operation.TryHandlerKind == "" && operation.MatchID == 0 && !synthesized {
		return
	}
	e.out.WriteByte('{')
	separator := ""
	attribute := func(name, value string) {
		e.out.WriteString(separator)
		fmt.Fprintf(&e.out, "%s = %s", name, value)
		separator = ", "
	}
	if synthesized {
		attribute("sec.synthesized", "true")
	}
	if operation.TryHandlerKind != "" {
		attribute("sec.try_handler_kind", mlirString(string(operation.TryHandlerKind)))
		attribute("sec.try_handler_index", fmt.Sprintf("%d : i32", operation.TryHandlerIndex))
		if operation.TryHandlerExhaustive {
			attribute("sec.try_handler_exhaustive", "true")
		}
		if operation.Variant != "" {
			attribute("sec.try_handler_variant", mlirString(operation.Variant))
		}
	}
	if operation.MatchID != 0 {
		attribute("sec.match_id", fmt.Sprintf("%d : i32", operation.MatchID))
		attribute("sec.match_arm_index", fmt.Sprintf("%d : i32", operation.MatchArmIndex))
		attribute("sec.match_stage", mlirString(operation.MatchStage))
		attribute("sec.match_pattern_kind", mlirString(operation.MatchPatternKind))
	}
	e.out.WriteString("} ")
}

func (e *emitter) emitUnarySemanticOperation(operation semantic.Operation, name string, values map[semantic.ValueID]semantic.Value) error {
	if len(operation.Operands) != 1 || len(operation.Results) != 1 {
		return fmt.Errorf("invalid %s", operation.Kind)
	}
	if err := e.emitResult(operation); err != nil {
		return err
	}
	operandType, err := e.valueTypeText(operation.Operands[0], values)
	if err != nil {
		return err
	}
	resultType, err := e.typeText(operation.Results[0].Type)
	if err != nil {
		return err
	}
	fmt.Fprintf(&e.out, "\"%s\"(%s) ", name, valueName(operation.Operands[0]))
	e.emitOperationAttributes(operation, false)
	fmt.Fprintf(&e.out, ": (%s) -> %s", operandType, resultType)
	e.emitLocation(operation.Location)
	e.out.WriteByte('\n')
	return nil
}

func (e *emitter) emitResultConstructor(operation semantic.Operation, name string, values map[semantic.ValueID]semantic.Value) error {
	if len(operation.Results) != 1 || len(operation.Operands) > 1 {
		return fmt.Errorf("invalid %s", operation.Kind)
	}
	if err := e.emitResult(operation); err != nil {
		return err
	}
	fmt.Fprintf(&e.out, "\"%s\"(", name)
	if len(operation.Operands) == 1 {
		e.out.WriteString(valueName(operation.Operands[0]))
	}
	e.out.WriteString(") : (")
	if err := e.emitOperandTypes(operation.Operands, values); err != nil {
		return err
	}
	e.out.WriteString(") -> ")
	resultType, err := e.typeText(operation.Results[0].Type)
	if err != nil {
		return err
	}
	e.out.WriteString(resultType)
	e.emitLocation(operation.Location)
	e.out.WriteByte('\n')
	return nil
}

func (e *emitter) emitSemanticIntegerOperation(operation semantic.Operation, name, attributeName, attributeValue string, values map[semantic.ValueID]semantic.Value) error {
	if err := e.emitResults(operation); err != nil {
		return err
	}
	fmt.Fprintf(&e.out, "\"%s\"(", name)
	for index, operand := range operation.Operands {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(valueName(operand))
	}
	e.out.WriteByte(')')
	if attributeName != "" {
		fmt.Fprintf(&e.out, " <{%s = %s}>", attributeName, mlirString(attributeValue))
	}
	if operation.TryHandlerKind != "" || operation.MatchID != 0 {
		e.out.WriteByte(' ')
		e.emitOperationAttributes(operation, false)
	}
	e.out.WriteString(" : (")
	if err := e.emitOperandTypes(operation.Operands, values); err != nil {
		return err
	}
	e.out.WriteString(") -> ")
	if err := e.emitResultTypes(operation.Results); err != nil {
		return err
	}
	e.emitLocation(operation.Location)
	e.out.WriteByte('\n')
	return nil
}

func (e *emitter) emitResults(operation semantic.Operation) error {
	if len(operation.Results) == 0 {
		return fmt.Errorf("%s requires results", operation.Kind)
	}
	for index, result := range operation.Results {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(valueName(result.ID))
	}
	e.out.WriteString(" = ")
	return nil
}

func (e *emitter) emitResult(operation semantic.Operation) error {
	if len(operation.Results) != 1 {
		return fmt.Errorf("%s requires one result", operation.Kind)
	}
	e.out.WriteString(valueName(operation.Results[0].ID))
	e.out.WriteString(" = ")
	return nil
}

func (e *emitter) emitOptionalResult(operation semantic.Operation) error {
	if len(operation.Results) > 1 {
		return &UnsupportedLoweringError{Feature: "multi-result call"}
	}
	if len(operation.Results) == 1 {
		return e.emitResult(operation)
	}
	return nil
}

func (e *emitter) finishResultOperation(operation semantic.Operation) error {
	typeText, err := e.typeText(operation.Results[0].Type)
	if err != nil {
		return err
	}
	e.out.WriteString(typeText)
	e.emitLocation(operation.Location)
	e.out.WriteByte('\n')
	return nil
}

func (e *emitter) emitOperandTypes(operands []semantic.ValueID, values map[semantic.ValueID]semantic.Value) error {
	for index, operand := range operands {
		if index > 0 {
			e.out.WriteString(", ")
		}
		typeText, err := e.valueTypeText(operand, values)
		if err != nil {
			return err
		}
		e.out.WriteString(typeText)
	}
	return nil
}

func (e *emitter) emitResultTypes(results []semantic.Value) error {
	if len(results) == 0 {
		e.out.WriteString("()")
		return nil
	}
	if len(results) == 1 {
		typeText, err := e.typeText(results[0].Type)
		if err != nil {
			return err
		}
		e.out.WriteString(typeText)
		return nil
	}
	e.out.WriteByte('(')
	for index, result := range results {
		if index > 0 {
			e.out.WriteString(", ")
		}
		typeText, err := e.typeText(result.Type)
		if err != nil {
			return err
		}
		e.out.WriteString(typeText)
	}
	e.out.WriteByte(')')
	return nil
}

func (e *emitter) emitTarget(target semantic.BranchTarget, values map[semantic.ValueID]semantic.Value) error {
	fmt.Fprintf(&e.out, "^bb%d", target.Block)
	if len(target.Arguments) == 0 {
		return nil
	}
	e.out.WriteByte('(')
	for index, argument := range target.Arguments {
		if index > 0 {
			e.out.WriteString(", ")
		}
		typeText, err := e.valueTypeText(argument, values)
		if err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "%s : %s", valueName(argument), typeText)
	}
	e.out.WriteByte(')')
	return nil
}

func (e *emitter) valueTypeText(id semantic.ValueID, values map[semantic.ValueID]semantic.Value) (string, error) {
	value, ok := values[id]
	if !ok {
		return "", fmt.Errorf("missing value %d", id)
	}
	return e.typeText(value.Type)
}

func (e *emitter) typeText(id semantic.TypeID) (string, error) {
	if text, ok := e.types[id]; ok {
		return text, nil
	}
	if e.visiting[id] {
		return "", fmt.Errorf("cyclic Semantic IR type %d", id)
	}
	typeValue, ok := e.module.Types.Lookup(id)
	if !ok {
		return "", fmt.Errorf("missing Semantic IR type %d", id)
	}
	e.visiting[id] = true
	defer delete(e.visiting, id)
	var text string
	switch typeValue.Kind {
	case semantic.TypeNever:
		text = "!sec.never"
	case semantic.TypeBool:
		text = "i1"
	case semantic.TypeByte:
		text = "ui8"
	case semantic.TypeChar:
		text = "!sec.char"
	case semantic.TypeRune:
		text = "!sec.rune"
	case semantic.TypeString:
		text = "!sec.string"
	case semantic.TypeDecimal:
		text = "!sec.decimal"
	case semantic.TypeDecimal128:
		text = "!sec.decimal128"
	case semantic.TypeInt:
		if typeValue.TargetSize || typeValue.Name == "int" {
			text = "!sec.int"
		} else if validFixedIntegerWidth(typeValue.BitWidth) && typeValue.Signed {
			text = fmt.Sprintf("si%d", typeValue.BitWidth)
		}
	case semantic.TypeUint:
		if typeValue.TargetSize || typeValue.Name == "uint" {
			text = "!sec.uint"
		} else if validFixedIntegerWidth(typeValue.BitWidth) {
			text = fmt.Sprintf("ui%d", typeValue.BitWidth)
		}
	case semantic.TypeFloat:
		if typeValue.TargetSize || typeValue.Name == "float" {
			text = "!sec.float"
		} else if typeValue.BitWidth == 32 || typeValue.BitWidth == 64 {
			text = fmt.Sprintf("f%d", typeValue.BitWidth)
		}
	case semantic.TypeNamed:
		base, err := e.typeText(typeValue.Base)
		if err != nil {
			return "", err
		}
		text = fmt.Sprintf("!sec.named<%s, %s>", mlirString(typeValue.Identity), base)
	case semantic.TypeArithmeticFailureReason:
		text = "!sec.arithmetic_failure_reason"
	case semantic.TypeCoreError:
		text = fmt.Sprintf("!sec.core_error<%s>", mlirString(typeValue.Identity))
	case semantic.TypeResult:
		success, err := e.typeText(typeValue.Success)
		if err != nil {
			return "", err
		}
		failure, err := e.typeText(typeValue.Error)
		if err != nil {
			return "", err
		}
		text = fmt.Sprintf("!sec.result<%s, %s>", success, failure)
	case semantic.TypeEnum:
		definition, ok := e.enumDefinition(id)
		if !ok {
			return "", fmt.Errorf("missing enum definition for semantic type %d", id)
		}
		underlying, err := e.enumUnderlyingText(definition.Underlying)
		if err != nil {
			return "", err
		}
		var cases strings.Builder
		cases.WriteByte('[')
		for index, enumCase := range definition.Cases {
			if index > 0 {
				cases.WriteString(", ")
			}
			fmt.Fprintf(&cases, "#sec.enum_case<ordinal = %d, name = %s, value = %s>", enumCase.ID, mlirString(enumCase.Name), mlirString(enumCase.Value.String()))
		}
		cases.WriteByte(']')
		text = fmt.Sprintf("!sec.enum<identity = %s, underlying = %s, representation = %s, bitWidth = %d, cases = %s>", mlirString(typeValue.Identity), underlying, mlirString(string(definition.RepresentationKind)), definition.BitWidth, cases.String())
	case semantic.TypeUnion:
		definition, ok := e.unionDefinition(id)
		if !ok {
			return "", fmt.Errorf("missing union definition for semantic type %d", id)
		}
		var arguments strings.Builder
		arguments.WriteByte('[')
		for index, argument := range definition.TypeArguments {
			if index > 0 {
				arguments.WriteString(", ")
			}
			argumentText, err := e.typeText(argument)
			if err != nil {
				return "", err
			}
			arguments.WriteString(argumentText)
		}
		arguments.WriteByte(']')
		variants, err := e.unionVariantsText(definition)
		if err != nil {
			return "", err
		}
		text = fmt.Sprintf("!sec.union<identity = %s, typeArguments = %s, variants = %s>", mlirString(typeValue.Identity), arguments.String(), variants)
	case semantic.TypeStruct:
		definition, ok := e.structDefinition(id)
		if !ok {
			return "", fmt.Errorf("missing struct definition for semantic type %d", id)
		}
		arguments, err := e.typeArgumentsText(definition.TypeArguments)
		if err != nil {
			return "", err
		}
		fields, err := e.structFieldsText(definition)
		if err != nil {
			return "", err
		}
		text = fmt.Sprintf("!sec.struct<identity = %s, typeArguments = %s, fields = %s>", mlirString(typeValue.Identity), arguments, fields)
	case semantic.TypeArray:
		if err := e.validateArrayLengthForPlan(typeValue.Length); err != nil {
			return "", err
		}
		element, err := e.typeText(typeValue.Element)
		if err != nil {
			return "", err
		}
		text = fmt.Sprintf("!sec.array<%s, %s>", element, mlirString(typeValue.Length))
	case semantic.TypeVoid:
		return "", fmt.Errorf("void has no MLIR value type")
	}
	if text == "" {
		return "", &UnsupportedLoweringError{Feature: "semantic type " + string(typeValue.Kind)}
	}
	e.types[id] = text
	return text, nil
}

// validateArrayLengthForPlan applies the schema-v10 CompilationPlan boundary
// without converting the canonical arbitrary-precision length to a host-sized
// integer.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 10 and 60
//   - rules/mlir/lowering-versions/sec_mlir_lowering_v10.md — sections 1 and 15
func (e *emitter) validateArrayLengthForPlan(length string) error {
	value, ok := new(big.Int).SetString(length, 10)
	if !ok || value.Sign() < 0 {
		return fmt.Errorf("invalid fixed-array length %q", length)
	}
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(e.plan.PointerWidthBits)), big.NewInt(1))
	if value.Cmp(maximum) > 0 {
		return fmt.Errorf("fixed-array length %s overflows target uint%d", length, e.plan.PointerWidthBits)
	}
	return nil
}

func (e *emitter) enumDefinition(id semantic.TypeID) (semantic.EnumDefinition, bool) {
	for _, definition := range e.module.Enums {
		if definition.TypeID == id {
			return definition, true
		}
	}
	return semantic.EnumDefinition{}, false
}

func (e *emitter) unionDefinition(id semantic.TypeID) (semantic.UnionDefinition, bool) {
	for _, definition := range e.module.Unions {
		if definition.TypeID == id {
			return definition, true
		}
	}
	return semantic.UnionDefinition{}, false
}

func (e *emitter) structDefinition(id semantic.TypeID) (semantic.StructDefinition, bool) {
	for _, definition := range e.module.Structs {
		if definition.TypeID == id {
			return definition, true
		}
	}
	return semantic.StructDefinition{}, false
}

func (e *emitter) typeArgumentsText(arguments []semantic.TypeID) (string, error) {
	var out strings.Builder
	out.WriteByte('[')
	for index, argument := range arguments {
		if index > 0 {
			out.WriteString(", ")
		}
		text, err := e.typeText(argument)
		if err != nil {
			return "", err
		}
		out.WriteString(text)
	}
	out.WriteByte(']')
	return out.String(), nil
}

// structFieldsText preserves declaration ordinals and open tag metadata per
// rules/mlir/dialect-versions/sec_mlir_dialect_v9.md sections 3, 4, and 25.
func (e *emitter) structFieldsText(definition semantic.StructDefinition) (string, error) {
	var out strings.Builder
	out.WriteByte('[')
	for index, field := range definition.Fields {
		if index > 0 {
			out.WriteString(", ")
		}
		typeText, err := e.typeText(field.Type)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&out, "#sec.struct_field<ordinal = %d, name = %s, type = %s, tags = [", field.ID, mlirString(field.Name), typeText)
		for tagIndex, tag := range field.Tags {
			if tagIndex > 0 {
				out.WriteString(", ")
			}
			fmt.Fprintf(&out, "#sec.struct_tag<key = %s, value = %s>", mlirString(tag.Key), mlirString(tag.Value))
		}
		out.WriteString("]>")
	}
	out.WriteByte(']')
	return out.String(), nil
}

func (e *emitter) enumUnderlyingText(id semantic.TypeID) (string, error) {
	typ, ok := e.module.Types.Lookup(id)
	if !ok {
		return "", fmt.Errorf("missing enum underlying type %d", id)
	}
	if typ.Kind == semantic.TypeUint && typ.BitWidth >= 1 && typ.BitWidth <= 256 && !typ.TargetSize {
		return fmt.Sprintf("ui%d", typ.BitWidth), nil
	}
	return e.typeText(id)
}

func (e *emitter) unionVariantsText(definition semantic.UnionDefinition) (string, error) {
	var out strings.Builder
	out.WriteByte('[')
	for index, variant := range definition.Variants {
		if index > 0 {
			out.WriteString(", ")
		}
		fmt.Fprintf(&out, "#sec.union_variant<index = %d, name = %s, kind = %s", variant.Index, mlirString(variant.Name), mlirString(string(variant.Kind)))
		if variant.Payload != 0 {
			payload, err := e.typeText(variant.Payload)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&out, ", payload = %s", payload)
		}
		out.WriteString(", fields = [")
		for fieldIndex, field := range variant.PayloadFields {
			if fieldIndex > 0 {
				out.WriteString(", ")
			}
			fieldType, err := e.typeText(field.Type)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&out, "#sec.union_field<name = %s, type = %s>", mlirString(field.Name), fieldType)
		}
		out.WriteString("]>")
	}
	out.WriteByte(']')
	return out.String(), nil
}

func validFixedIntegerWidth(width uint16) bool {
	switch width {
	case 8, 16, 32, 64, 128, 256:
		return true
	default:
		return false
	}
}

func (e *emitter) scalarKind(id semantic.TypeID) (string, bool) {
	typeValue, ok := e.module.Types.Lookup(id)
	if !ok || typeValue.Kind == semantic.TypeNamed {
		return "", false
	}
	switch typeValue.Kind {
	case semantic.TypeBool:
		return "bool", true
	case semantic.TypeByte:
		return "byte", true
	case semantic.TypeChar:
		return "char", true
	case semantic.TypeRune:
		return "rune", true
	case semantic.TypeString:
		return "string", true
	case semantic.TypeDecimal:
		return "decimal", true
	case semantic.TypeDecimal128:
		return "decimal128", true
	case semantic.TypeFloat:
		return typeValue.Name, typeValue.Name == "float" || typeValue.Name == "float32" || typeValue.Name == "float64"
	case semantic.TypeInt, semantic.TypeUint:
		return typeValue.Name, typeValue.Name != ""
	default:
		return "", false
	}
}

func (e *emitter) isVoid(id semantic.TypeID) bool {
	typeValue, ok := e.module.Types.Lookup(id)
	return ok && typeValue.Kind == semantic.TypeVoid
}

func (e *emitter) emitLocation(location semantic.Location) {
	if location.Line <= 0 || location.Column <= 0 {
		e.out.WriteString(" loc(unknown)")
		return
	}
	fmt.Fprintf(&e.out, " loc(%s:%d:%d)", mlirString(location.File), location.Line, location.Column)
}

func collectValues(function *semantic.Function) map[semantic.ValueID]semantic.Value {
	values := map[semantic.ValueID]semantic.Value{}
	for _, parameter := range function.Parameters {
		values[parameter.Value.ID] = parameter.Value
	}
	for _, block := range function.Blocks {
		for _, parameter := range block.Parameters {
			values[parameter.ID] = parameter
		}
		for _, operation := range block.Operations {
			for _, result := range operation.Results {
				values[result.ID] = result
			}
		}
	}
	return values
}

func orderedBlocks(function *semantic.Function) (*semantic.Block, []*semantic.Block, error) {
	var entry *semantic.Block
	for _, block := range function.Blocks {
		if block.ID == function.Entry {
			entry = block
			break
		}
	}
	if entry == nil {
		return nil, nil, fmt.Errorf("missing entry block %d", function.Entry)
	}
	ordered := []*semantic.Block{entry}
	for _, block := range function.Blocks {
		if block != entry {
			ordered = append(ordered, block)
		}
	}
	return entry, ordered, nil
}

func valueName(id semantic.ValueID) string     { return "%v" + strconv.FormatUint(uint64(id), 10) }
func storageName(id semantic.StorageID) string { return "%s" + strconv.FormatUint(uint64(id), 10) }

func mlirString(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for index := 0; index < len(value); index++ {
		current := value[index]
		if current >= 0x20 && current <= 0x7e && current != '"' && current != '\\' {
			out.WriteByte(current)
			continue
		}
		fmt.Fprintf(&out, "\\%02X", current)
	}
	out.WriteByte('"')
	return out.String()
}

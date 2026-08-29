package semantic

import (
	"fmt"
	"strconv"
	"strings"
)

func Format(module *Module) string {
	if module == nil {
		return "<nil semantic-ir>\n"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "semantic-ir %d\n\nmodule %q {\n", module.Version, module.Identity)
	out.WriteString("  sources {\n")
	for _, source := range module.SourceFiles {
		fmt.Fprintf(&out, "    %q\n", source)
	}
	out.WriteString("  }\n\n  types {\n")
	for i, t := range module.Types.All() {
		fmt.Fprintf(&out, "    !%d = %s\n", i+1, formatType(t))
	}
	out.WriteString("  }\n")
	if len(module.Enums) > 0 {
		out.WriteString("\n  enums {\n")
		for _, definition := range module.Enums {
			fmt.Fprintf(&out, "    !%d %q underlying=!%d representation=%s width=%d {\n", definition.TypeID, definition.SymbolID, definition.Underlying, definition.RepresentationKind, definition.BitWidth)
			for _, enumCase := range definition.Cases {
				fmt.Fprintf(&out, "      #%d %s = %s %s\n", enumCase.ID, enumCase.Name, enumCase.Value.String(), formatLocation(enumCase.Location))
			}
			out.WriteString("    }\n")
		}
		out.WriteString("  }\n")
	}
	if len(module.Unions) > 0 {
		out.WriteString("\n  unions {\n")
		for _, definition := range module.Unions {
			fmt.Fprintf(&out, "    !%d %q copy=%s trivial-destroy=%t {\n", definition.TypeID, definition.SymbolID, definition.CopyClassification, definition.TriviallyDestructible)
			for _, variant := range definition.Variants {
				fmt.Fprintf(&out, "      #%d %s %s", variant.Index, variant.Name, variant.Kind)
				if variant.Payload != 0 {
					fmt.Fprintf(&out, " !%d", variant.Payload)
				}
				for _, field := range variant.PayloadFields {
					fmt.Fprintf(&out, " %s:!%d", field.Name, field.Type)
				}
				fmt.Fprintf(&out, " %s\n", formatLocation(variant.Location))
			}
			out.WriteString("    }\n")
		}
		out.WriteString("  }\n")
	}
	if len(module.Structs) > 0 {
		out.WriteString("\n  structs {\n")
		for _, definition := range module.Structs {
			fmt.Fprintf(&out, "    !%d %q copy=%s trivial-destroy=%t defaultable=%t {\n", definition.TypeID, definition.SymbolID, definition.CopyClassification, definition.TriviallyDestructible, definition.Defaultable)
			for _, field := range definition.Fields {
				fmt.Fprintf(&out, "      #%d %s:!%d %s\n", field.ID, field.Name, field.Type, formatLocation(field.Location))
			}
			out.WriteString("    }\n")
		}
		out.WriteString("  }\n")
	}
	for _, fn := range module.Functions {
		formatFunction(&out, fn)
	}
	out.WriteString("}\n")
	return out.String()
}
func formatType(t Type) string {
	if t.Kind == TypeNamed {
		return fmt.Sprintf("named %q base !%d", t.Identity, t.Base)
	}
	if t.Kind == TypeCoreError {
		return fmt.Sprintf("core-error %q", t.Identity)
	}
	if t.Kind == TypeResult {
		return fmt.Sprintf("Result<!%d, !%d>", t.Success, t.Error)
	}
	if t.Kind == TypeEnum {
		return fmt.Sprintf("enum %q underlying !%d", t.Identity, t.Underlying)
	}
	if t.Kind == TypeUnion {
		return fmt.Sprintf("union %q", t.Identity)
	}
	if t.Kind == TypeStruct {
		return fmt.Sprintf("struct %q", t.Identity)
	}
	if t.Kind == TypeArray {
		return fmt.Sprintf("array<!%d, %q>", t.Element, t.Length)
	}
	s := t.Name
	if s == "" {
		s = string(t.Kind)
	}
	if t.TargetSize {
		s += " target-sized"
	}
	if t.BitWidth > 0 {
		s += fmt.Sprintf(" width=%d", t.BitWidth)
	}
	return s
}
func formatFunction(out *strings.Builder, fn *Function) {
	fmt.Fprintf(out, "\n  func %q %s(", fn.ID, fn.Name)
	for i, p := range fn.Parameters {
		if i > 0 {
			out.WriteString(", ")
		}
		fmt.Fprintf(out, "%%%d %s: !%d [%s]", p.Value.ID, p.Name, p.Value.Type, p.Value.Ownership)
	}
	fmt.Fprintf(out, ") -> !%d", fn.ReturnType)
	if fn.Extern {
		fmt.Fprintf(out, " extern abi=%q link=%q %s\n", fn.ABI, fn.LinkName, formatLocation(fn.Location))
		return
	}
	fmt.Fprintf(out, " %s {\n", formatLocation(fn.Location))
	if len(fn.Storages) > 0 {
		out.WriteString("    storage {\n")
		for _, s := range fn.Storages {
			fmt.Fprintf(out, "      $%d %s: !%d mutable=%t class=%s %s\n", s.ID, s.Name, s.Type, s.Mutable, s.Class, formatLocation(s.Location))
		}
		out.WriteString("    }\n")
	}
	for _, b := range fn.Blocks {
		fmt.Fprintf(out, "    ^%d", b.ID)
		if len(b.Parameters) > 0 {
			out.WriteByte('(')
			for i, p := range b.Parameters {
				if i > 0 {
					out.WriteString(", ")
				}
				fmt.Fprintf(out, "%%%d: !%d [%s]", p.ID, p.Type, p.Ownership)
			}
			out.WriteByte(')')
		}
		out.WriteString(":\n")
		for _, op := range b.Operations {
			out.WriteString("      ")
			formatOperation(out, op)
			out.WriteByte('\n')
		}
	}
	out.WriteString("  }\n")
}
func formatOperation(out *strings.Builder, op Operation) {
	if len(op.Results) > 0 {
		for index, result := range op.Results {
			if index > 0 {
				out.WriteString(", ")
			}
			fmt.Fprintf(out, "%%%d", result.ID)
		}
		out.WriteString(" = ")
	}
	out.WriteString(string(op.Kind))
	switch op.Kind {
	case OpConstInt:
		fmt.Fprintf(out, " %s", op.Integer.String())
	case OpConstBool:
		fmt.Fprintf(out, " %t", *op.Bool)
	case OpConstString:
		fmt.Fprintf(out, " %q", op.String)
	case OpConstDecimal:
		fmt.Fprintf(out, " %s coefficient=%s scale=%d", strconv.Quote(op.Decimal.Lexeme), op.Decimal.Coefficient.String(), op.Decimal.Scale)
	case OpConstFloat:
		fmt.Fprintf(out, " %s", op.FloatLexeme)
	case OpStorageDeclare:
		fmt.Fprintf(out, " $%d", op.Storage)
	case OpStorageInit, OpStorageStore:
		fmt.Fprintf(out, " $%d, %%%d", op.Storage, op.Operands[0])
	case OpStorageLoad:
		fmt.Fprintf(out, " $%d", op.Storage)
	case OpReturn:
		if len(op.Operands) > 0 {
			fmt.Fprintf(out, " %%%d", op.Operands[0])
		}
	case OpUnreachable:
		fmt.Fprintf(out, " synthesized=%t reason=%q", op.Synthesized, op.Reason)
	case OpDirectCall, OpForeignCall:
		fmt.Fprintf(out, " %q(", op.Callee)
		for i, id := range op.Operands {
			if i > 0 {
				out.WriteString(", ")
			}
			fmt.Fprintf(out, "%%%d[%s]", id, op.ArgumentActions[i])
		}
		out.WriteByte(')')
	case OpBranch:
		formatTarget(out, op.Successors[0])
	case OpCondBranch:
		fmt.Fprintf(out, " %%%d,", op.Operands[0])
		formatTarget(out, op.Successors[0])
		out.WriteString(",")
		formatTarget(out, op.Successors[1])
	case OpIntUnaryPlus, OpIntNegChecked, OpIntBitNot:
		fmt.Fprintf(out, " %%%d", op.Operands[0])
	case OpIntBinaryChecked:
		fmt.Fprintf(out, " %s %%%d, %%%d", op.IntegerBinary, op.Operands[0], op.Operands[1])
	case OpIntBitwise:
		fmt.Fprintf(out, " %s %%%d, %%%d", op.IntegerBitwise, op.Operands[0], op.Operands[1])
	case OpIntShiftChecked:
		fmt.Fprintf(out, " %s %%%d, %%%d", op.IntegerShift, op.Operands[0], op.Operands[1])
	case OpIntCompare:
		fmt.Fprintf(out, " %s %%%d, %%%d", op.IntegerCompare, op.Operands[0], op.Operands[1])
	case OpArithmeticFailure:
		fmt.Fprintf(out, " %%%d %s %q", op.Operands[0], op.FailureCategory, op.Operator)
	case OpArithmeticFailureReasonConstant:
		fmt.Fprintf(out, " %s", op.FailureReason)
	case OpArithmeticErrorFromReason:
		fmt.Fprintf(out, " %%%d", op.Operands[0])
	case OpResultOk, OpResultErr:
		if len(op.Operands) == 1 {
			fmt.Fprintf(out, " %%%d", op.Operands[0])
		}
	case OpResultIsErr, OpResultUnwrapOk, OpResultUnwrapErr:
		fmt.Fprintf(out, " %%%d", op.Operands[0])
	case OpCoreErrorIsVariant:
		fmt.Fprintf(out, " %s %%%d", op.Variant, op.Operands[0])
	case OpEnumConstant:
		fmt.Fprintf(out, " case=#%d", op.EnumCase)
	case OpEnumFromInteger, OpEnumToInteger:
		fmt.Fprintf(out, " %%%d", op.Operands[0])
	case OpEnumCompare:
		fmt.Fprintf(out, " %s %%%d, %%%d", op.IntegerCompare, op.Operands[0], op.Operands[1])
	case OpUnionConstruct:
		fmt.Fprintf(out, " variant=#%d", op.UnionVariant)
		for index, operand := range op.Operands {
			field := ""
			if index < len(op.UnionFields) {
				field = op.UnionFields[index] + "="
			}
			fmt.Fprintf(out, " %s%%%d[%s]", field, operand, op.PayloadActions[index])
		}
	case OpUnionIsVariant:
		fmt.Fprintf(out, " %%%d variant=#%d", op.Operands[0], op.UnionVariant)
	case OpUnionUnwrapPayload:
		fmt.Fprintf(out, " %%%d variant=#%d[%s]", op.Operands[0], op.UnionVariant, op.PayloadActions[0])
	case OpUnionUnwrapField:
		fmt.Fprintf(out, " %%%d variant=#%d field=%s[%s]", op.Operands[0], op.UnionVariant, op.UnionField, op.PayloadActions[0])
	case OpStructConstruct:
		for index, operand := range op.Operands {
			fmt.Fprintf(out, " #%d=%%%d[%s,%s]", index, operand, op.StructOrigins[index], op.StructActions[index])
		}
	case OpStructSpreadFields:
		fmt.Fprintf(out, " %%%d", op.Operands[0])
		for index, action := range op.StructActions {
			fmt.Fprintf(out, " #%d[%s]", index, action)
		}
	case OpStructExtractField:
		fmt.Fprintf(out, " %%%d field=#%d[%s]", op.Operands[0], op.StructField, op.StructActions[0])
	case OpStructReplaceField:
		fmt.Fprintf(out, " %%%d field=#%d value=%%%d", op.Operands[0], op.StructField, op.Operands[1])
	case OpArrayConstruct:
		fmt.Fprintf(out, " element=!%d length=%q", op.ArrayElementType, op.ArrayLength)
		for index, operand := range op.Operands {
			fmt.Fprintf(out, " #%d=%%%d[%s,%s,%s]", index, operand, op.ArraySegmentKinds[index], op.ArraySegmentLengths[index], op.ArrayActions[index])
		}
	case OpArrayDefault:
		fmt.Fprintf(out, " element=!%d length=%q", op.ArrayElementType, op.ArrayLength)
	case OpArrayLength:
		fmt.Fprintf(out, " %%%d exact=%q", op.Operands[0], op.ArrayLength)
	case OpArrayIndexInBounds:
		fmt.Fprintf(out, " %%%d, %%%d signed=%t", op.Operands[0], op.Operands[1], op.ArrayIndexSigned)
	case OpArrayExtract:
		fmt.Fprintf(out, " %%%d, %%%d bounds=%s proof=%s action=%s", op.Operands[0], op.Operands[1], op.ArrayCheckKind, op.ArrayProofKind, op.ArrayActions[0])
		if op.ArrayGuard != 0 {
			fmt.Fprintf(out, " guard=%%%d", op.ArrayGuard)
		}
	case OpArrayReplace:
		fmt.Fprintf(out, " %%%d, %%%d value=%%%d bounds=%s proof=%s", op.Operands[0], op.Operands[1], op.Operands[2], op.ArrayCheckKind, op.ArrayProofKind)
		if op.ArrayGuard != 0 {
			fmt.Fprintf(out, " guard=%%%d", op.ArrayGuard)
		}
	case OpBoundsFailure:
		fmt.Fprintf(out, " operation=%q", op.ArrayOperation)
	}
	if op.TryHandlerKind != "" {
		fmt.Fprintf(out, " [handler=%s index=%d", op.TryHandlerKind, op.TryHandlerIndex)
		if op.Variant != "" {
			fmt.Fprintf(out, " variant=%s", op.Variant)
		}
		out.WriteByte(']')
	}
	if len(op.Results) > 0 {
		out.WriteString(" : ")
		for index, result := range op.Results {
			if index > 0 {
				out.WriteString(", ")
			}
			fmt.Fprintf(out, "!%d [%s]", result.Type, result.Ownership)
		}
	}
	out.WriteByte(' ')
	out.WriteString(formatLocation(op.Location))
}
func formatTarget(out *strings.Builder, t BranchTarget) {
	fmt.Fprintf(out, " ^%d", t.Block)
	if len(t.Arguments) > 0 {
		out.WriteByte('(')
		for i, id := range t.Arguments {
			if i > 0 {
				out.WriteString(", ")
			}
			fmt.Fprintf(out, "%%%d", id)
		}
		out.WriteByte(')')
	}
}
func formatLocation(l Location) string {
	if l.Line == 0 {
		return "loc(unknown)"
	}
	return fmt.Sprintf("loc(%q:%d:%d)", l.File, l.Line, l.Column)
}

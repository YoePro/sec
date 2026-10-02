package sema

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// foreignPosition is one source position of a C or system extern signature.
// Legality is decided per position: a type may be FFI-representable and still
// be illegal where it is written.
type foreignPosition uint8

const (
	foreignParameterByValue foreignPosition = iota
	foreignParameterReferent
	foreignReturn
)

// foreignTypeVerdict explains why a type cannot cross a C or system ABI in one
// position, with the corrective help shown to the user.
type foreignTypeVerdict struct {
	legal  bool
	reason string
	help   string
}

func legalForeignType() foreignTypeVerdict { return foreignTypeVerdict{legal: true} }

func illegalForeignType(reason, help string) foreignTypeVerdict {
	return foreignTypeVerdict{reason: reason, help: help}
}

// isRawForeignABI reports the calling conventions whose boundary uses the
// raw C or system representation rules of ffi.md §45-47. The Sec ABI keeps
// ordinary Sec value semantics.
func isRawForeignABI(abi string) bool {
	return abi == "C" || abi == "system"
}

// validateExternFunction checks every parameter and the return type of an
// extern declaration in its exact position. C and system declarations use the
// per-position foreign legality rules; the Sec ABI keeps the general
// ABI-compatibility gate.
//
// Rules:
//   - rules/platform/ffi.md — §9, §12, §13, §45 "FFI-legal types", §46 "Types forbidden from direct raw FFI use"
//   - rules/platform/ffi.md — §47 "Incomplete and position-restricted types", §52, §53
func (a *Analyzer) validateExternFunction(function Function, declaration *ast.FunctionDeclaration) {
	if !isRawForeignABI(function.ABI) {
		for i, param := range function.Parameters {
			if !isFFICompatibleParameterType(param.Type) {
				a.addErrorAtToken(param.Token, "extern %s parameter %d %s has non-ABI-compatible type %s", function.ABI, i+1, param.Name, typeDisplayName(param.Type))
			}
		}
		if function.ReturnType.Kind != VoidType && !isFFICompatibleType(function.ReturnType) {
			a.addErrorAtToken(function.Token, "extern %s function %s has non-ABI-compatible return type %s", function.ABI, function.Name, typeDisplayName(function.ReturnType))
		}
		return
	}
	for index, param := range function.Parameters {
		verdict := foreignTypeLegality(param.Type, foreignParameterByValue)
		if verdict.legal {
			continue
		}
		a.reportIllegalForeignType(param.Token, function,
			fmt.Sprintf("parameter %d %s", index+1, param.Name), param.Type, verdict)
	}
	verdict := foreignTypeLegality(function.ReturnType, foreignReturn)
	if !verdict.legal {
		token := function.Token
		if declaration != nil && declaration.ReturnType != nil && declaration.ReturnType.Token.Line > 0 {
			token = declaration.ReturnType.Token
		}
		a.reportIllegalForeignType(token, function, "return type", function.ReturnType, verdict)
	}
}

func (a *Analyzer) reportIllegalForeignType(token lexer.Token, function Function, position string, typ Type, verdict foreignTypeVerdict) {
	a.addErrorAtTokenWithMetadata(token, diagnostics.IllegalForeignType, verdict.help,
		"extern %q fn %s: %s has type %s, which cannot cross the %s ABI boundary: %s",
		function.ABI, function.Name, position, typeDisplayName(typ), function.ABI, verdict.reason)
}

// foreignTypeLegality classifies one type in one C/system extern position.
// It decides legality from resolved type identity and kind, never from the
// spelling written in the declaration.
//
// Rules:
//   - rules/platform/ffi.md — §5 and §9 legal scalars; §10 RawPtr; §12 call-bounded references;
//     §13 pointer returns; §21 fixed arrays; §45; §46; §47
func foreignTypeLegality(typ Type, position foreignPosition) foreignTypeVerdict {
	if isForeignCScalar(typ) {
		return legalForeignType()
	}
	switch typ.Kind {
	case InvalidType, GenericType:
		// Resolution and unresolved-generic diagnostics own these.
		return legalForeignType()
	case VoidType:
		if position == foreignReturn {
			return legalForeignType()
		}
		return illegalForeignType("void has no value representation", "Remove the parameter or use RawPtr[void] for an untyped address.")
	case IntType, UintType, RawPtrType:
		return legalForeignType()
	case FloatType:
		if typ.Name == "float32" || typ.Name == "float64" {
			return legalForeignType()
		}
		return illegalForeignType("only the IEEE float32 and float64 Sec scalars have a fixed C representation", "Use float32, float64, or the matching C:: floating type.")
	case EnumType:
		if typ.Underlying != "" && typ.Underlying != "enum" && typ.Underlying != "string" {
			return legalForeignType()
		}
		return illegalForeignType("an enum without an integer underlying representation has no C representation", "Give the enum an integer underlying type or pass its integer value.")
	case BoolType:
		return illegalForeignType("Sec bool does not represent C _Bool", "Use C::bool and convert explicitly at the boundary.")
	case CharType, RuneType:
		return illegalForeignType(typeDisplayName(typ)+" does not represent a C character type", "Use C::char, C::schar, C::uchar, or the binding's wide-character type.")
	case StringType:
		return illegalForeignType("Sec string has no implicit C string representation", "Pass RawPtr[byte] or RawPtr[C::char] together with an explicit length or termination contract, and convert in a wrapper.")
	case DecimalType:
		return illegalForeignType("decimal has no C ABI representation", "Convert to a C floating or integer representation in a wrapper.")
	case ReferenceType:
		switch position {
		case foreignParameterByValue:
			if typ.Element == nil {
				return legalForeignType()
			}
			if typ.Element.Kind == SliceType {
				return illegalForeignType("a Sec slice is a pointer-and-length descriptor with no C representation", "Pass ref or ref mut to the first element, or RawPtr[T], together with an explicit length.")
			}
			referent := foreignTypeLegality(*typ.Element, foreignParameterReferent)
			if !referent.legal {
				referent.reason = "its referent " + typeDisplayName(*typ.Element) + " cannot cross the boundary: " + referent.reason
			}
			return referent
		case foreignReturn:
			return illegalForeignType("a raw extern declaration returns foreign pointers as RawPtr[T], never as a call-bounded reference", "Return RawPtr[T] and validate it in a wrapper before exposing a reference.")
		default:
			return illegalForeignType("a reference to a reference has no call-bounded C representation", "Pass RawPtr[RawPtr[T]] for pointer-to-pointer data.")
		}
	case ArrayType:
		if arrayShapeOf(typ) == ArrayShapeDynamic {
			return illegalForeignType("an owning dynamic array is a native Sec collection", "Pass RawPtr[T] or a reference to the first element together with an explicit length.")
		}
		if position != foreignParameterReferent {
			return illegalForeignType("fixed arrays describe inline C data but are not passed or returned by value", "Pass ref or ref mut to the array, or RawPtr[T] to its first element.")
		}
		if typ.Element == nil {
			return legalForeignType()
		}
		element := foreignTypeLegality(*typ.Element, foreignParameterReferent)
		if !element.legal {
			element.reason = "its element type " + typeDisplayName(*typ.Element) + " cannot cross the boundary: " + element.reason
		}
		return element
	case SliceType:
		return illegalForeignType("a Sec slice is a pointer-and-length descriptor with no C representation", "Pass RawPtr[T] together with an explicit length.")
	case FunctionType:
		return illegalForeignType("native Sec callable values and closures do not cross a C ABI", "Use a C::fn function-pointer type with an environment-free callback once foreign function pointers are available.")
	case InterfaceType:
		return illegalForeignType("interface values have no C representation", "Pass a concrete C-compatible representation instead.")
	case UnionType:
		switch typ.Name {
		case "Option":
			return illegalForeignType("Option has no C representation; foreign code uses its own null or status protocol", "Use the raw foreign protocol (for example a RawPtr that may be null) and normalize it to Option in a wrapper.")
		case "Result":
			return illegalForeignType("Result has no C representation; foreign code uses its own status protocol", "Use the raw foreign status representation and normalize it to Result in a wrapper.")
		}
		return illegalForeignType("an ordinary Sec tagged union has no C representation", "Declare the foreign data with its C representation, or pass its fields individually.")
	case ResultType:
		return illegalForeignType("Result has no C representation; foreign code uses its own status protocol", "Use the raw foreign status representation and normalize it to Result in a wrapper.")
	case StructType:
		if isForCollectionFamily(typ) {
			return illegalForeignType("a native Sec collection has no C representation", "Pass RawPtr[T] to its elements together with an explicit length.")
		}
		return illegalForeignType("an ordinary Sec struct has no explicit foreign representation", "Describe the data with an extern \"C\" struct once foreign data declarations are available, or pass its fields individually.")
	case AnyType:
		return illegalForeignType("any has no C representation", "Pass a concrete C-compatible type.")
	}
	return illegalForeignType("the type has no declared foreign ABI representation", "Use a C:: scalar, a fixed-width Sec scalar, RawPtr[T], or a call-bounded reference.")
}

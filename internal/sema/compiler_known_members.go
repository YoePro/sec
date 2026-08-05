package sema

import "strings"

type CompilerKnownMemberKind string

const (
	CompilerKnownProperty           CompilerKnownMemberKind = "property"
	CompilerKnownMethod             CompilerKnownMemberKind = "method"
	CompilerKnownAssociatedFunction CompilerKnownMemberKind = "associated-function"
)

type CompilerKnownMember struct {
	ID          string
	Name        string
	LegacyNames []string
	Kind        CompilerKnownMemberKind
	Result      Type
	Unsafe      bool
}

func CompilerKnownMembersForType(typ Type, static bool) []CompilerKnownMember {
	if static {
		return compilerKnownStaticMembers(typ)
	}
	return compilerKnownValueMembers(typ)
}

func compilerKnownValueMembers(typ Type) []CompilerKnownMember {
	members := []CompilerKnownMember{}
	uintType := builtinTypes()["uint"]
	stringType := builtinTypes()["string"]

	if compilerKnownSizedType(typ) {
		members = append(members,
			CompilerKnownMember{ID: "CKM-PTR-VALUE", Name: "Ptr", LegacyNames: []string{"ptr"}, Kind: CompilerKnownProperty, Result: compilerKnownRawPointerResult(typ), Unsafe: true},
			CompilerKnownMember{ID: "CKM-SIZEOF-VALUE", Name: "SizeOf", Kind: CompilerKnownMethod, Result: uintType},
		)
	}
	if compilerKnownSequenceType(typ) {
		members = append(members, CompilerKnownMember{ID: compilerKnownLenID(typ), Name: "Len", LegacyNames: []string{"len"}, Kind: CompilerKnownProperty, Result: uintType})
	}
	if compilerKnownPrintableType(typ) {
		members = append(members, CompilerKnownMember{ID: compilerKnownToStringID(typ), Name: "ToString", Kind: CompilerKnownMethod, Result: stringType})
	}
	if typ.Kind == StringType {
		members = append(members,
			CompilerKnownMember{ID: "CKM-STRING-TOBYTEARRAY", Name: "ToByteArray", Kind: CompilerKnownMethod, Result: compilerKnownDynamicArray(builtinTypes()["byte"])},
			CompilerKnownMember{ID: "CKM-STRING-TOCHARARRAY", Name: "ToCharArray", Kind: CompilerKnownMethod, Result: compilerKnownDynamicArray(builtinTypes()["char"])},
			CompilerKnownMember{ID: "CKM-STRING-TORUNEARRAY", Name: "ToRuneArray", Kind: CompilerKnownMethod, Result: compilerKnownDynamicArray(builtinTypes()["rune"])},
		)
	}
	if typ.Kind == RawPtrType {
		members = append(members,
			CompilerKnownMember{ID: "CKM-RAWPTR-READ", Name: "Read", Kind: CompilerKnownMethod, Result: compilerKnownRawPointerElement(typ), Unsafe: true},
			CompilerKnownMember{ID: "CKM-RAWPTR-WRITE", Name: "Write", Kind: CompilerKnownMethod, Result: builtinTypes()["void"], Unsafe: true},
			CompilerKnownMember{ID: "CKM-RAWPTR-OFFSET", Name: "Offset", Kind: CompilerKnownMethod, Result: typ, Unsafe: true},
			CompilerKnownMember{ID: "CKM-RAWPTR-ADDBYTES", Name: "AddBytes", Kind: CompilerKnownMethod, Result: typ, Unsafe: true},
			CompilerKnownMember{ID: "CKM-RAWPTR-DIFFERENCE", Name: "Difference", Kind: CompilerKnownMethod, Result: builtinTypes()["int"], Unsafe: true},
		)
	}
	if typ.Name == "Arena" {
		members = append(members,
			CompilerKnownMember{ID: "CKM-ARENA-NEW", Name: "New", Kind: CompilerKnownMethod},
			CompilerKnownMember{ID: "CKM-ARENA-ALLOC", Name: "Alloc", Kind: CompilerKnownMethod},
			CompilerKnownMember{ID: "CKM-ARENA-RESET", Name: "Reset", Kind: CompilerKnownMethod, Result: builtinTypes()["void"]},
			CompilerKnownMember{ID: "CKM-ARENA-RELEASE", Name: "Release", Kind: CompilerKnownMethod, Result: builtinTypes()["void"]},
		)
	}
	return members
}

func compilerKnownStaticMembers(typ Type) []CompilerKnownMember {
	members := []CompilerKnownMember{}
	if compilerKnownSizedType(typ) {
		members = append(members, CompilerKnownMember{ID: "CKM-SIZEOF-TYPE", Name: "SizeOf", Kind: CompilerKnownMethod, Result: builtinTypes()["uint"]})
	}
	if isIntegerType(typ) {
		members = append(members,
			CompilerKnownMember{ID: "CKM-NUMERIC-MIN", Name: "Min", Kind: CompilerKnownProperty, Result: typ},
			CompilerKnownMember{ID: "CKM-NUMERIC-MAX", Name: "Max", Kind: CompilerKnownProperty, Result: typ},
			CompilerKnownMember{ID: "CKM-NUMERIC-BITS", Name: "Bits", Kind: CompilerKnownProperty, Result: builtinTypes()["uint"]},
		)
	}
	if typ.Kind == FloatType {
		for _, name := range []string{"Min", "Max", "Epsilon", "Infinity", "NegativeInfinity", "NaN"} {
			members = append(members, CompilerKnownMember{ID: "CKM-FLOAT-" + strings.ToUpper(name), Name: name, Kind: CompilerKnownProperty, Result: typ})
		}
	}
	if typ.Kind == StringType {
		members = append(members,
			CompilerKnownMember{ID: "CKM-STRING-FROMBYTEARRAY", Name: "FromByteArray", Kind: CompilerKnownAssociatedFunction, Result: typ},
			CompilerKnownMember{ID: "CKM-STRING-FROMRUNEARRAY", Name: "FromRuneArray", Kind: CompilerKnownAssociatedFunction, Result: typ},
		)
	}
	if typ.Name == "Arena" {
		members = append(members,
			CompilerKnownMember{ID: "CKM-ARENA-FROMBUFFER", Name: "FromBuffer", Kind: CompilerKnownAssociatedFunction, Result: typ},
			CompilerKnownMember{ID: "CKM-ARENA-WITHCAPACITY", Name: "WithCapacity", Kind: CompilerKnownAssociatedFunction},
			CompilerKnownMember{ID: "CKM-ARENA-GROWABLE", Name: "Growable", Kind: CompilerKnownAssociatedFunction},
		)
	}
	return members
}

func compilerKnownMember(typ Type, name string, static bool) (CompilerKnownMember, bool) {
	for _, member := range CompilerKnownMembersForType(typ, static) {
		if member.Name == name {
			return member, true
		}
		for _, legacy := range member.LegacyNames {
			if legacy == name {
				return member, true
			}
		}
	}
	return CompilerKnownMember{}, false
}

func compilerKnownSequenceType(typ Type) bool {
	if typ.Kind == StringType || typ.Kind == ArrayType || typ.Kind == SliceType {
		return true
	}
	return typ.Kind == ReferenceType && typ.Element != nil && (typ.Element.Kind == ArrayType || typ.Element.Kind == SliceType)
}

func compilerKnownPrintableType(typ Type) bool {
	if typ.Kind == BoolType || typ.Kind == StringType || typ.Kind == CharType || typ.Kind == RuneType || isNumericType(typ) {
		return true
	}
	sequence := dereferenceType(typ)
	return (sequence.Kind == ArrayType || sequence.Kind == SliceType) && sequence.Element != nil && (sequence.Element.Kind == CharType || sequence.Element.Kind == RuneType)
}

func compilerKnownSizedType(typ Type) bool {
	switch typ.Kind {
	case InvalidType, VoidType, NeverType, GenericType, InterfaceType:
		return false
	default:
		return true
	}
}

func compilerKnownRawPointerResult(typ Type) Type {
	element := typ
	if typ.Kind == StringType {
		element = builtinTypes()["byte"]
	} else {
		sequence := dereferenceType(typ)
		if (sequence.Kind == ArrayType || sequence.Kind == SliceType) && sequence.Element != nil {
			element = *sequence.Element
		}
	}
	return rawPointerType(element)
}

func compilerKnownRawPointerElement(typ Type) Type {
	if len(typ.TypeArgs) == 1 {
		return typ.TypeArgs[0]
	}
	return Type{Kind: InvalidType}
}

func compilerKnownDynamicArray(element Type) Type {
	return Type{Name: typeDisplayName(element) + "[]", Kind: ArrayType, Element: &element, ArrayLength: dynamicArrayLength}
}

func compilerKnownLenID(typ Type) string {
	if typ.Kind == StringType {
		return "CKM-LEN-STRING"
	}
	if typ.Kind == ArrayType {
		return "CKM-LEN-ARRAY"
	}
	return "CKM-LEN-SLICE"
}

func compilerKnownToStringID(typ Type) string {
	sequence := dereferenceType(typ)
	if (sequence.Kind == ArrayType || sequence.Kind == SliceType) && sequence.Element != nil {
		if sequence.Element.Kind == CharType {
			return "CKM-TOSTRING-CHAR-SEQUENCE"
		}
		return "CKM-TOSTRING-RUNE-SEQUENCE"
	}
	switch typ.Kind {
	case StringType:
		return "CKM-TOSTRING-STRING"
	case BoolType:
		return "CKM-TOSTRING-BOOL"
	case IntType:
		return "CKM-TOSTRING-SIGNED-INTEGER"
	case UintType:
		return "CKM-TOSTRING-UNSIGNED-INTEGER"
	case FloatType:
		return "CKM-TOSTRING-FLOAT"
	case DecimalType:
		return "CKM-TOSTRING-DECIMAL"
	case CharType:
		return "CKM-TOSTRING-CHAR"
	case RuneType:
		return "CKM-TOSTRING-RUNE"
	default:
		return "CKM-TOSTRING-VALUE"
	}
}

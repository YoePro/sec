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
	Effects     []EffectKind
}

type CompilerKnownFunction struct {
	ID         string
	Name       string
	Parameters []FunctionParameter
	Result     Type
	Internal   bool
}

// CompilerKnownFunctions is the canonical catalog used to reserve and expose
// compiler-owned global functions. Their detailed contextual validation stays
// in Sema, while LSP observes the same registered Function values.
func CompilerKnownFunctions() []CompilerKnownFunction {
	types := builtinTypes()
	return []CompilerKnownFunction{
		{ID: "CKF-LEN", Name: "len", Result: types["int"]},
		{ID: "CKF-SIZEOF-TYPE", Name: "SizeOf", Result: types["uint"]},
		{ID: "CKF-FILL", Name: "fill", Result: Type{Kind: InvalidType}},
		{
			ID:   "CKF-STRING-SLICE-UNCHECKED",
			Name: "__StringSliceUnchecked",
			Parameters: []FunctionParameter{
				{Name: "value", Type: types["string"]},
				{Name: "start", Type: types["uint"]},
				{Name: "end", Type: types["uint"]},
			},
			Result:   types["string"],
			Internal: true,
		},
	}
}

func compilerKnownFunction(name string) (CompilerKnownFunction, bool) {
	for _, function := range CompilerKnownFunctions() {
		if function.Name == name {
			return function, true
		}
	}
	return CompilerKnownFunction{}, false
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
	boolType := builtinTypes()["bool"]
	stringType := builtinTypes()["string"]

	if compilerKnownPointerReceiver(typ) {
		members = append(members, CompilerKnownMember{ID: "CKM-PTR-VALUE", Name: "Ptr", LegacyNames: []string{"ptr"}, Kind: CompilerKnownProperty, Result: compilerKnownRawPointerResult(typ), Unsafe: true})
	}
	if compilerKnownValueSizeReceiver(typ) {
		members = append(members, CompilerKnownMember{ID: "CKM-SIZEOF-VALUE", Name: "SizeOf", Kind: CompilerKnownProperty, Result: uintType})
	}
	if dereferenceType(typ).Kind == VariadicPackType {
		// rules/declarations/functions.md section 30 exposes Len on a native
		// pack without implying array/slice methods, contiguity, or a pointer.
		members = append(members, CompilerKnownMember{ID: "CKM-LEN-VARIADIC-PACK", Name: "Len", LegacyNames: []string{"len"}, Kind: CompilerKnownProperty, Result: uintType})
	} else if compilerKnownSequenceType(typ) {
		members = append(members, CompilerKnownMember{ID: compilerKnownLenID(typ), Name: "Len", LegacyNames: []string{"len"}, Kind: CompilerKnownProperty, Result: uintType})
		if dereferenceType(typ).Kind != StringType {
			members = append(members, CompilerKnownMember{ID: compilerKnownIsEmptyID(typ), Name: "IsEmpty", Kind: CompilerKnownProperty, Result: boolType})
		}
	}
	if compilerKnownPrintableType(typ) {
		members = append(members, CompilerKnownMember{ID: compilerKnownToStringID(typ), Name: "ToString", Kind: CompilerKnownMethod, Result: stringType})
	}
	sequence := dereferenceType(typ)
	if sequence.Kind == ArrayType && arrayShapeOf(sequence) == ArrayShapeDynamic && sequence.Element != nil {
		members = append(members,
			CompilerKnownMember{ID: "CKM-DYNAMIC-ARRAY-APPEND", Name: "Append", Kind: CompilerKnownMethod, Result: compilerKnownResult(builtinTypes()["void"], builtinTypes()["CollectionError"])},
			CompilerKnownMember{ID: "CKM-DYNAMIC-ARRAY-CLEAR", Name: "Clear", Kind: CompilerKnownMethod, Result: builtinTypes()["void"]},
			CompilerKnownMember{ID: "CKM-DYNAMIC-ARRAY-REMOVEAT", Name: "RemoveAt", Kind: CompilerKnownMethod, Result: compilerKnownOption(*sequence.Element)},
		)
	}
	if compilerKnownMutableSlice(typ) {
		members = append(members,
			CompilerKnownMember{ID: "CKM-MUTABLE-SLICE-REVERSE", Name: "Reverse", Kind: CompilerKnownMethod, Result: builtinTypes()["void"]},
			CompilerKnownMember{ID: "CKM-MUTABLE-SLICE-FILL", Name: "Fill", Kind: CompilerKnownMethod, Result: builtinTypes()["void"]},
		)
	}
	if sequence.Name == "list" && len(sequence.TypeArgs) == 1 {
		element := sequence.TypeArgs[0]
		members = append(members,
			CompilerKnownMember{ID: "CKM-LIST-CAPACITY", Name: "Capacity", Kind: CompilerKnownProperty, Result: uintType},
			CompilerKnownMember{ID: "CKM-LIST-APPEND", Name: "Append", Kind: CompilerKnownMethod, Result: compilerKnownResult(builtinTypes()["void"], builtinTypes()["CollectionError"])},
			CompilerKnownMember{ID: "CKM-LIST-INSERT", Name: "Insert", Kind: CompilerKnownMethod, Result: compilerKnownResult(boolType, builtinTypes()["CollectionError"])},
			CompilerKnownMember{ID: "CKM-LIST-REMOVEAT", Name: "RemoveAt", Kind: CompilerKnownMethod, Result: compilerKnownOption(element)},
			CompilerKnownMember{ID: "CKM-LIST-REMOVE", Name: "Remove", Kind: CompilerKnownMethod, Result: boolType},
			CompilerKnownMember{ID: "CKM-LIST-CLEAR", Name: "Clear", Kind: CompilerKnownMethod, Result: builtinTypes()["void"]},
			CompilerKnownMember{ID: "CKM-LIST-CONTAINS", Name: "Contains", Kind: CompilerKnownMethod, Result: boolType},
			CompilerKnownMember{ID: "CKM-LIST-INDEXOF", Name: "IndexOf", Kind: CompilerKnownMethod, Result: compilerKnownOption(uintType)},
			CompilerKnownMember{ID: "CKM-LIST-REVERSE", Name: "Reverse", Kind: CompilerKnownMethod, Result: builtinTypes()["void"]},
			CompilerKnownMember{ID: "CKM-LIST-SORT", Name: "Sort", Kind: CompilerKnownMethod, Result: builtinTypes()["void"]},
			CompilerKnownMember{ID: "CKM-LIST-SORTBY", Name: "SortBy", Kind: CompilerKnownMethod, Result: builtinTypes()["void"]},
		)
	}
	if sequence.Name == "map" && len(sequence.TypeArgs) == 2 {
		members = append(members,
			CompilerKnownMember{ID: "CKM-MAP-REMOVE", Name: "Remove", Kind: CompilerKnownMethod, Result: compilerKnownOption(sequence.TypeArgs[1])},
			CompilerKnownMember{ID: "CKM-MAP-CONTAINSKEY", Name: "ContainsKey", Kind: CompilerKnownMethod, Result: boolType},
			CompilerKnownMember{ID: "CKM-MAP-CLEAR", Name: "Clear", Kind: CompilerKnownMethod, Result: builtinTypes()["void"]},
		)
	}
	if sequence.Name == "set" && len(sequence.TypeArgs) == 1 {
		members = append(members,
			CompilerKnownMember{ID: "CKM-SET-ADD", Name: "Add", Kind: CompilerKnownMethod, Result: compilerKnownResult(boolType, builtinTypes()["CollectionError"])},
			CompilerKnownMember{ID: "CKM-SET-REMOVE", Name: "Remove", Kind: CompilerKnownMethod, Result: boolType},
			CompilerKnownMember{ID: "CKM-SET-CONTAINS", Name: "Contains", Kind: CompilerKnownMethod, Result: boolType},
			CompilerKnownMember{ID: "CKM-SET-CLEAR", Name: "Clear", Kind: CompilerKnownMethod, Result: builtinTypes()["void"]},
			CompilerKnownMember{ID: "CKM-SET-UNION", Name: "Union", Kind: CompilerKnownMethod, Result: compilerKnownResult(sequence, builtinTypes()["CollectionError"])},
			CompilerKnownMember{ID: "CKM-SET-INTERSECTION", Name: "Intersection", Kind: CompilerKnownMethod, Result: compilerKnownResult(sequence, builtinTypes()["CollectionError"])},
			CompilerKnownMember{ID: "CKM-SET-DIFFERENCE", Name: "Difference", Kind: CompilerKnownMethod, Result: compilerKnownResult(sequence, builtinTypes()["CollectionError"])},
			CompilerKnownMember{ID: "CKM-SET-SYMMETRIC-DIFFERENCE", Name: "SymmetricDifference", Kind: CompilerKnownMethod, Result: compilerKnownResult(sequence, builtinTypes()["CollectionError"])},
		)
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
			// rules/platform/volatile.md sections 9-11: volatile access is a
			// distinct unsafe, effectful operation, not an alias for Read/Write.
			CompilerKnownMember{ID: "CKM-RAWPTR-VOLATILE-READ", Name: "VolatileRead", Kind: CompilerKnownMethod, Result: compilerKnownRawPointerElement(typ), Unsafe: true, Effects: []EffectKind{EffectVolatileRead}},
			CompilerKnownMember{ID: "CKM-RAWPTR-VOLATILE-WRITE", Name: "VolatileWrite", Kind: CompilerKnownMethod, Result: builtinTypes()["void"], Unsafe: true, Effects: []EffectKind{EffectVolatileWrite}},
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
		members = append(members, CompilerKnownMember{ID: "CKM-SIZEOF-TYPE", Name: "SizeOf", Kind: CompilerKnownProperty, Result: builtinTypes()["uint"]})
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
	if typ.Kind == DecimalType {
		members = append(members, CompilerKnownMember{ID: "CKM-DECIMAL-SCALE", Name: "Scale", Kind: CompilerKnownProperty, Result: builtinTypes()["int"]})
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
	sequence := dereferenceType(typ)
	if sequence.Kind == StringType || sequence.Kind == ArrayType || sequence.Kind == SliceType {
		return true
	}
	return compilerKnownCollectionName(sequence.Name)
}

func compilerKnownPrintableType(typ Type) bool {
	if typ.Kind == BoolType || typ.Kind == StringType || typ.Kind == CharType || typ.Kind == RuneType || isNumericType(typ) {
		return true
	}
	sequence := dereferenceType(typ)
	return sequence.Kind == ArrayType || sequence.Kind == SliceType || compilerKnownCollectionName(sequence.Name)
}

func compilerKnownCollectionName(name string) bool {
	switch name {
	case "list", "map", "set":
		return true
	default:
		return false
	}
}

func compilerKnownPointerReceiver(typ Type) bool {
	sequence := dereferenceType(typ)
	return sequence.Name != "map" && sequence.Name != "set" && compilerKnownSizedType(typ)
}

func compilerKnownValueSizeReceiver(typ Type) bool {
	sequence := dereferenceType(typ)
	return sequence.Name != "map" && sequence.Name != "set" && compilerKnownSizedType(typ)
}

func compilerKnownMutableSlice(typ Type) bool {
	return typ.Kind == ReferenceType && typ.ReferenceMutable && typ.Element != nil && typ.Element.Kind == SliceType
}

func compilerKnownSizedType(typ Type) bool {
	switch typ.Kind {
	case InvalidType, VoidType, NeverType, GenericType, InterfaceType, VariadicPackType:
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
		} else if sequence.Name == "list" && len(sequence.TypeArgs) == 1 {
			element = sequence.TypeArgs[0]
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
	return NewDynamicArrayType(element)
}

func compilerKnownLenID(typ Type) string {
	typ = dereferenceType(typ)
	if typ.Kind == StringType {
		return "CKM-LEN-STRING"
	}
	if typ.Kind == ArrayType {
		return "CKM-LEN-ARRAY"
	}
	if compilerKnownCollectionName(typ.Name) {
		return "CKM-LEN-" + strings.ToUpper(typ.Name)
	}
	return "CKM-LEN-SLICE"
}

func compilerKnownIsEmptyID(typ Type) string {
	typ = dereferenceType(typ)
	if typ.Kind == ArrayType {
		return "CKM-ISEMPTY-ARRAY"
	}
	if typ.Kind == SliceType {
		return "CKM-ISEMPTY-SLICE"
	}
	return "CKM-ISEMPTY-" + strings.ToUpper(typ.Name)
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

func compilerKnownOption(value Type) Type {
	typ := builtinTypes()["Option"]
	typ.TypeArgs = []Type{value}
	return typ
}

func compilerKnownResult(value Type, err Type) Type {
	return Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{value, err}}
}

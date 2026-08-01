package sema

import (
	"math/big"
	"unicode"

	"sec/internal/lexer"
)

type TypeKind string

const (
	InvalidType   TypeKind = "invalid"
	BoolType      TypeKind = "bool"
	IntType       TypeKind = "int"
	UintType      TypeKind = "uint"
	FloatType     TypeKind = "float"
	DecimalType   TypeKind = "decimal"
	EnumType      TypeKind = "enum"
	UnionType     TypeKind = "union"
	StringType    TypeKind = "string"
	CharType      TypeKind = "char"
	RuneType      TypeKind = "rune"
	RawPtrType    TypeKind = "rawptr"
	ReferenceType TypeKind = "reference"
	ResultType    TypeKind = "result"
	RegisterType  TypeKind = "register"
	StructType    TypeKind = "struct"
	SliceType     TypeKind = "slice"
	ArrayType     TypeKind = "array"
	FunctionType  TypeKind = "function"
	GenericType   TypeKind = "generic"
	InterfaceType TypeKind = "interface"
	NeverType     TypeKind = "never"
	VoidType      TypeKind = "void"
)

type Type struct {
	Name                      string
	Module                    string
	Kind                      TypeKind
	Named                     bool
	Declared                  bool
	Intrinsic                 bool
	Underlying                string
	Unit                      string
	Dimension                 Dimension
	ReferenceMutable          bool
	ReferenceOriginName       string
	ReferenceOriginToken      lexer.Token
	ReferenceOriginLocal      bool
	ReferenceOriginStorage    StorageOrigin
	ReferenceOriginGeneration int
	MinInt                    *int64
	MaxInt                    *int64
	MinUint                   *uint64
	MaxUint                   *uint64
	MinInteger                *big.Int
	MaxInteger                *big.Int
	Contracts                 []Contract
	ExplicitDefault           *DefaultConstant
	InvalidExplicitDefault    bool
	EnumValues                []string
	EnumConsts                map[string]EnumValue
	BitWidth                  int64
	UnionVariants             []UnionVariant
	TypeArgs                  []Type
	ConstArgs                 []int64
	Element                   *Type
	ArrayLength               int64
	EventCapacity             int64
	EventCapacitySet          bool
	FunctionParameterTypes    []Type
	FunctionReturnType        *Type
	GenericParameters         []string
	Fields                    []StructField
	RegisterWidth             int64
	RegisterFields            []RegisterField
	Properties                []Property
	Events                    []Event
	Implements                []Type
	InterfaceMethods          []Function
	InterfaceProperties       []InterfaceProperty
	InterfaceEvents           []InterfaceEvent
}

type EnumValue struct {
	Name  string
	Value *big.Int
	Token lexer.Token
}

type UnionVariant struct {
	Name          string
	Payload       *Type
	PayloadFields []StructField
	Token         lexer.Token
}

type StructField struct {
	Name  string
	Type  Type
	Token lexer.Token
	Tags  []StructTag
}

type RegisterField struct {
	Name  string
	Width int64
	Unit  string
	Type  Type
	Token lexer.Token
}

type StructTag struct {
	Key   string
	Value string
}

type Property struct {
	Name      string
	Type      Type
	Token     lexer.Token
	Fallible  bool
	Error     *Type
	HasGetter bool
	HasSetter bool
}

type Event struct {
	Name          string
	Type          Type
	Payload       Type
	Capacity      int64
	Token         lexer.Token
	Owner         string
	Storage       string
	StorageBacked bool
}

type InterfaceEvent struct {
	Name    string
	Payload Type
	Token   lexer.Token
}

type StorageOrigin string

const (
	StorageOriginInline       StorageOrigin = "Inline"
	StorageOriginStatic       StorageOrigin = "Static"
	StorageOriginArena        StorageOrigin = "Arena"
	StorageOriginExternal     StorageOrigin = "External"
	StorageOriginForeign      StorageOrigin = "Foreign"
	StorageOriginFixedAddress StorageOrigin = "FixedAddress"
	StorageOriginUnknown      StorageOrigin = "Unknown"
)

type AllocationEffect string

const (
	AllocationEffectNone    AllocationEffect = "none"
	AllocationEffectArena   AllocationEffect = "arena"
	AllocationEffectForeign AllocationEffect = "foreign"
	AllocationEffectUnknown AllocationEffect = "unknown"
)

type AllocationContext struct {
	Available bool
	Origin    StorageOrigin
	Profile   string
}

type CopyClassification string

const (
	CopyTrivial     CopyClassification = "trivial"
	CopySemantic    CopyClassification = "semantic"
	CopyMoveOnly    CopyClassification = "move-only"
	CopyConditional CopyClassification = "conditional"
	CopyNonCopyable CopyClassification = "non-copyable"
)

type InterfaceProperty struct {
	Name        string
	Type        Type
	Token       lexer.Token
	RequiresGet bool
	RequiresSet bool
}

type UnitCategory string

const (
	PhysicalUnit UnitCategory = "physical"
	CurrencyUnit UnitCategory = "currency"
	OtherUnit    UnitCategory = "other"
)

type UnitStatus string

const (
	StatusActive     UnitStatus = "active"
	StatusDeprecated UnitStatus = "deprecated"
	StatusObsolete   UnitStatus = "obsolete"
)

type UnitDefinition struct {
	Name           string
	LongName       string
	Symbol         string
	Category       UnitCategory
	Dimension      Dimension
	Scale          string
	DefaultNumeric string
	IsBaseUnit     bool
	Status         UnitStatus
	System         string
	Token          lexer.Token
}

type Function struct {
	Name              string
	Module            string
	GenericParameters []string
	Parameters        []FunctionParameter
	ReturnType        Type
	Token             lexer.Token
	Extern            bool
	ABI               string
	AllocationEffect  AllocationEffect
	ImplTarget        string
	ReceiverMutable   bool
}

type FunctionParameter struct {
	Name       string
	Type       Type
	Token      lexer.Token
	Ref        bool
	MutableRef bool
}

type Dimension struct {
	Base map[string]int
}

func (d Dimension) IsZero() bool {
	return len(d.Base) == 0
}

func (d Dimension) Equal(other Dimension) bool {
	if len(d.Base) != len(other.Base) {
		return false
	}

	for name, exp := range d.Base {
		if other.Base[name] != exp {
			return false
		}
	}

	return true
}

func (d Dimension) Mul(other Dimension) Dimension {
	return combineDimensions(d, other, 1)
}

func (d Dimension) Div(other Dimension) Dimension {
	return combineDimensions(d, other, -1)
}

func (d Dimension) HasCurrencyBase() bool {
	for name := range d.Base {
		runes := []rune(name)
		if len(runes) > 0 && unicode.IsUpper(runes[0]) {
			return true
		}
	}

	return false
}

func combineDimensions(left Dimension, right Dimension, sign int) Dimension {
	out := Dimension{Base: map[string]int{}}

	for name, exp := range left.Base {
		if exp != 0 {
			out.Base[name] = exp
		}
	}

	for name, exp := range right.Base {
		next := out.Base[name] + sign*exp
		if next == 0 {
			delete(out.Base, name)
			continue
		}
		out.Base[name] = next
	}

	return out
}

type Contract interface {
	contractNode()
}

type RangeContract struct {
	Min       *big.Int
	Max       *big.Int
	ExactMin  *big.Rat
	ExactMax  *big.Rat
	MinLexeme string
	MaxLexeme string
	Exclusive bool
}

func (RangeContract) contractNode() {}

type MembershipContract struct {
	Values []DefaultConstant
}

func (MembershipContract) contractNode() {}

type DefaultConstant struct {
	Kind    TypeKind
	Lexeme  string
	Integer *big.Int
	Exact   *big.Rat
	String  string
	Bool    bool
}

type DefaultKind string

const (
	NoDefault           DefaultKind = "none"
	PrimitiveDefault    DefaultKind = "primitive"
	NamedDefault        DefaultKind = "named"
	RangeDefault        DefaultKind = "range"
	MembershipDefault   DefaultKind = "membership"
	ExplicitTypeDefault DefaultKind = "explicit"
	StructDefault       DefaultKind = "struct"
	ArrayDefault        DefaultKind = "array"
)

type DefaultResolution struct {
	Kind     DefaultKind
	Value    DefaultConstant
	Fields   []DefaultField
	Elements []DefaultResolution
}

type DefaultField struct {
	Name  string
	Value DefaultResolution
}

type MultipleOfContract struct {
	Value *big.Int
}

func (MultipleOfContract) contractNode() {}

type MarkerContract struct {
	Name string
}

func (MarkerContract) contractNode() {}

type DecimalValue struct {
	Int64 int64
	Scale uint8
}

type Symbol struct {
	Name    string
	Type    Type
	Mutable bool
	// ImplicitMember marks the short-name alias injected for an impl member.
	// A lexical declaration may shadow such an alias without redeclaring the
	// underlying field, property or event.
	ImplicitMember bool
	Token          lexer.Token
	Addressed      bool
	Address        string
	Volatile       bool
	Storage        StorageOrigin
	Local          bool
	ScopeDepth     int
}

func builtinTypes() map[string]Type {
	types := map[string]Type{
		"bool":   {Name: "bool", Kind: BoolType},
		"byte":   unsignedType("byte", 255),
		"char":   {Name: "char", Kind: CharType},
		"rune":   {Name: "rune", Kind: RuneType},
		"RawPtr": {Name: "RawPtr", Kind: RawPtrType, GenericParameters: []string{"T"}},
		"never":  {Name: "never", Kind: NeverType},
		"Arena":  {Name: "Arena", Kind: StructType},
		"AllocationError": {
			Name:       "AllocationError",
			Kind:       EnumType,
			Underlying: "uint",
			EnumValues: []string{"OutOfMemory", "Unsupported", "InvalidSize", "InvalidAlignment"},
			EnumConsts: builtinEnumConsts([]string{"OutOfMemory", "Unsupported", "InvalidSize", "InvalidAlignment"}),
		},
		"ContractError": {Name: "ContractError", Kind: StructType},
		"Option": {
			Name:              "Option",
			Kind:              UnionType,
			GenericParameters: []string{"T"},
			UnionVariants: []UnionVariant{
				{Name: "Some", Payload: &Type{Name: "T", Kind: GenericType}},
				{Name: "None"},
			},
		},
		"Vec":                   {Name: "Vec", Kind: StructType, GenericParameters: []string{"T"}},
		"Set":                   {Name: "Set", Kind: StructType, GenericParameters: []string{"T"}},
		"Map":                   {Name: "Map", Kind: StructType, GenericParameters: []string{"K", "V"}},
		"list":                  {Name: "list", Kind: StructType, GenericParameters: []string{"T"}},
		"map":                   {Name: "map", Kind: StructType, GenericParameters: []string{"K", "V"}},
		"set":                   {Name: "set", Kind: StructType, GenericParameters: []string{"T"}},
		"vector":                {Name: "vector", Kind: StructType, GenericParameters: []string{"T"}},
		"matrix":                {Name: "matrix", Kind: StructType, GenericParameters: []string{"T"}},
		"tensor":                {Name: "tensor", Kind: StructType, GenericParameters: []string{"T"}},
		"tensor_view":           {Name: "tensor_view", Kind: StructType, GenericParameters: []string{"T"}},
		"Shape":                 {Name: "Shape", Kind: StructType},
		"Strides":               {Name: "Strides", Kind: StructType},
		"TensorLayout":          {Name: "TensorLayout", Kind: StructType},
		"MemorySpace":           {Name: "MemorySpace", Kind: StructType},
		"Task":                  {Name: "Task", Kind: StructType, GenericParameters: []string{"T"}},
		"Thread":                {Name: "Thread", Kind: StructType, GenericParameters: []string{"T"}},
		"ThreadObserver":        {Name: "ThreadObserver", Kind: StructType, GenericParameters: []string{"T"}},
		"ThreadLocal":           {Name: "ThreadLocal", Kind: StructType, GenericParameters: []string{"T"}},
		"ThreadConfig":          {Name: "ThreadConfig", Kind: StructType},
		"ThreadContext":         {Name: "ThreadContext", Kind: StructType},
		"ThreadID":              {Name: "ThreadID", Kind: StructType},
		"ThreadPriority":        {Name: "ThreadPriority", Kind: EnumType, Underlying: "uint", EnumValues: []string{"Low", "Normal", "High"}, EnumConsts: builtinEnumConsts([]string{"Low", "Normal", "High"})},
		"ThreadStatus":          {Name: "ThreadStatus", Kind: EnumType, Underlying: "uint", EnumValues: []string{"Created", "Running", "Completed", "Cancelled", "Panicked", "Terminated"}, EnumConsts: builtinEnumConsts([]string{"Created", "Running", "Completed", "Cancelled", "Panicked", "Terminated"})},
		"ThreadSpawnError":      {Name: "ThreadSpawnError", Kind: EnumType, Underlying: "uint", EnumValues: []string{"Unsupported", "ResourceUnavailable", "PermissionDenied", "InvalidConfiguration", "ThreadLocalInitializationFailed", "NativeFailure"}, EnumConsts: builtinEnumConsts([]string{"Unsupported", "ResourceUnavailable", "PermissionDenied", "InvalidConfiguration", "ThreadLocalInitializationFailed", "NativeFailure"})},
		"ThreadStartError":      {Name: "ThreadStartError", Kind: EnumType, Underlying: "uint", EnumValues: []string{"InvalidState", "Unsupported", "NativeFailure"}, EnumConsts: builtinEnumConsts([]string{"InvalidState", "Unsupported", "NativeFailure"})},
		"ThreadSchedulingError": {Name: "ThreadSchedulingError", Kind: EnumType, Underlying: "uint", EnumValues: []string{"Unsupported", "PermissionDenied", "InvalidState", "NativeFailure"}, EnumConsts: builtinEnumConsts([]string{"Unsupported", "PermissionDenied", "InvalidState", "NativeFailure"})},
		"ThreadTerminationError": {
			Name:       "ThreadTerminationError",
			Kind:       EnumType,
			Underlying: "uint",
			EnumValues: []string{"Unsupported", "PermissionDenied", "InvalidState", "NativeFailure"},
			EnumConsts: builtinEnumConsts([]string{"Unsupported", "PermissionDenied", "InvalidState", "NativeFailure"}),
		},
		"ThreadContextError":    {Name: "ThreadContextError", Kind: EnumType, Underlying: "uint", EnumValues: []string{"NotAttached", "AlreadyAttached", "ResourceUnavailable", "NativeFailure"}, EnumConsts: builtinEnumConsts([]string{"NotAttached", "AlreadyAttached", "ResourceUnavailable", "NativeFailure"})},
		"Mutex":                 {Name: "Mutex", Kind: StructType, GenericParameters: []string{"T"}},
		"MutexGuard":            {Name: "MutexGuard", Kind: StructType, GenericParameters: []string{"T"}},
		"Atomic":                {Name: "Atomic", Kind: StructType, GenericParameters: []string{"T"}},
		"CompareExchangeResult": {Name: "CompareExchangeResult", Kind: StructType, GenericParameters: []string{"T"}},
		"Event":                 {Name: "Event", Kind: StructType, GenericParameters: []string{"T"}},
		"EventStorage":          {Name: "EventStorage", Kind: StructType, GenericParameters: []string{"T"}},
		"Subscription":          {Name: "Subscription", Kind: StructType},
		"EventSubscribeResult": {
			Name:       "EventSubscribeResult",
			Kind:       UnionType,
			EnumValues: []string{"Subscribed", "Full"},
			UnionVariants: []UnionVariant{
				{Name: "Subscribed", Payload: &Type{Name: "Subscription", Kind: StructType}},
				{Name: "Full"},
			},
		},
		"Channel":        {Name: "Channel", Kind: StructType, GenericParameters: []string{"T"}},
		"Sender":         {Name: "Sender", Kind: StructType, GenericParameters: []string{"T"}},
		"Receiver":       {Name: "Receiver", Kind: StructType, GenericParameters: []string{"T"}},
		"MessageTicket":  {Name: "MessageTicket", Kind: StructType, GenericParameters: []string{"T"}},
		"ChannelOptions": {Name: "ChannelOptions", Kind: StructType},
		"SenderID":       {Name: "SenderID", Kind: StructType},
		"ChannelSendResult": {
			Name:              "ChannelSendResult",
			Kind:              UnionType,
			GenericParameters: []string{"T"},
			UnionVariants: []UnionVariant{
				{Name: "Sent"},
				{Name: "Closed", Payload: &Type{Name: "T", Kind: GenericType}},
			},
		},
		"ChannelTryReceiveResult": {
			Name:              "ChannelTryReceiveResult",
			Kind:              UnionType,
			GenericParameters: []string{"T"},
			UnionVariants: []UnionVariant{
				{Name: "Received", Payload: &Type{Name: "T", Kind: GenericType}},
				{Name: "Empty"},
				{Name: "Closed"},
			},
		},
		"ChannelRevokeResult": {
			Name:              "ChannelRevokeResult",
			Kind:              UnionType,
			GenericParameters: []string{"T"},
			UnionVariants: []UnionVariant{
				{Name: "Revoked", Payload: &Type{Name: "T", Kind: GenericType}},
				{Name: "Unavailable", Payload: &Type{Name: "MessageDisposition", Kind: EnumType}},
			},
		},
		"MessageDisposition": {
			Name:       "MessageDisposition",
			Kind:       EnumType,
			Underlying: "uint",
			EnumValues: []string{"Received", "Expired", "Discarded"},
			EnumConsts: builtinEnumConsts([]string{"Received", "Expired", "Discarded"}),
		},
		"Result":  {Name: "Result", Kind: ResultType, GenericParameters: []string{"T", "E"}},
		"decimal": {Name: "decimal", Kind: DecimalType},
		"decimal128": {
			Name: "decimal128",
			Kind: DecimalType,
		},
		"float":   {Name: "float", Kind: FloatType},
		"float32": {Name: "float32", Kind: FloatType},
		"float64": {Name: "float64", Kind: FloatType},
		"int":     signedType("int", -1<<63, 1<<63-1),
		"int8":    signedType("int8", -1<<7, 1<<7-1),
		"int16":   signedType("int16", -1<<15, 1<<15-1),
		"int32":   signedType("int32", -1<<31, 1<<31-1),
		"int64":   signedType("int64", -1<<63, 1<<63-1),
		"int128":  signedBigType("int128", "-170141183460469231731687303715884105728", "170141183460469231731687303715884105727"),
		"int256":  signedBigType("int256", "-57896044618658097711785492504343953926634992332820282019728792003956564819968", "57896044618658097711785492504343953926634992332820282019728792003956564819967"),
		"string":  {Name: "string", Kind: StringType},
		"uint":    unsignedType("uint", ^uint64(0)),
		"uint8":   unsignedType("uint8", 1<<8-1),
		"uint16":  unsignedType("uint16", 1<<16-1),
		"uint32":  unsignedType("uint32", 1<<32-1),
		"uint64":  unsignedType("uint64", ^uint64(0)),
		"uint128": unsignedBigType("uint128", "340282366920938463463374607431768211455"),
		"uint256": unsignedBigType("uint256", "115792089237316195423570985008687907853269984665640564039457584007913129639935"),
		"void":    {Name: "void", Kind: VoidType},
	}

	for name, typ := range types {
		typ.Intrinsic = true
		types[name] = typ
	}

	return types
}

func builtinEnumConsts(values []string) map[string]EnumValue {
	consts := make(map[string]EnumValue, len(values))
	for i, name := range values {
		consts[name] = EnumValue{Name: name, Value: big.NewInt(int64(i))}
	}
	return consts
}

func signedType(name string, min, max int64) Type {
	return Type{
		Name:       name,
		Kind:       IntType,
		MinInt:     &min,
		MaxInt:     &max,
		MinInteger: big.NewInt(min),
		MaxInteger: big.NewInt(max),
	}
}

func unsignedType(name string, max uint64) Type {
	var min uint64
	return Type{
		Name:       name,
		Kind:       UintType,
		MinUint:    &min,
		MaxUint:    &max,
		MinInteger: new(big.Int).SetUint64(min),
		MaxInteger: new(big.Int).SetUint64(max),
	}
}

func signedBigType(name string, min string, max string) Type {
	return Type{
		Name:       name,
		Kind:       IntType,
		MinInteger: mustBigInt(min),
		MaxInteger: mustBigInt(max),
	}
}

func unsignedBigType(name string, max string) Type {
	return Type{
		Name:       name,
		Kind:       UintType,
		MinInteger: big.NewInt(0),
		MaxInteger: mustBigInt(max),
	}
}

func mustBigInt(value string) *big.Int {
	out, ok := new(big.Int).SetString(value, 10)
	if !ok {
		panic("invalid builtin integer bound: " + value)
	}
	return out
}

func TriviallyDestructible(typ Type) bool {
	return triviallyDestructible(typ, map[string]bool{})
}

func triviallyDestructible(typ Type, visiting map[string]bool) bool {
	switch typ.Kind {
	case InvalidType,
		VoidType,
		NeverType,
		BoolType,
		IntType,
		UintType,
		FloatType,
		DecimalType,
		CharType,
		RuneType,
		RawPtrType,
		ReferenceType,
		FunctionType,
		InterfaceType,
		RegisterType:
		return true
	case StringType:
		// Current Sec strings are lowered as string views or static literals.
		// Owned string storage will need an explicit non-trivial string type.
		return true
	case GenericType:
		// Generic declarations are only templates. Concrete instantiations must
		// substitute type arguments before destruction analysis relies on this.
		return true
	case ArrayType:
		if typ.Element == nil {
			return true
		}
		return triviallyDestructible(*typ.Element, visiting)
	case SliceType:
		// Slices are non-owning views in the current language model.
		return true
	case EnumType:
		return true
	case ResultType:
		for _, arg := range typ.TypeArgs {
			if !triviallyDestructible(arg, visiting) {
				return false
			}
		}
		return true
	case StructType:
		key := typeDestructionKey(typ)
		if key != "" {
			if visiting[key] {
				return true
			}
			visiting[key] = true
			defer delete(visiting, key)
		}
		for _, field := range typ.Fields {
			if !triviallyDestructible(field.Type, visiting) {
				return false
			}
		}
		return true
	case UnionType:
		key := typeDestructionKey(typ)
		if key != "" {
			if visiting[key] {
				return true
			}
			visiting[key] = true
			defer delete(visiting, key)
		}
		for _, variant := range typ.UnionVariants {
			if variant.Payload != nil && !triviallyDestructible(*variant.Payload, visiting) {
				return false
			}
			for _, field := range variant.PayloadFields {
				if !triviallyDestructible(field.Type, visiting) {
					return false
				}
			}
		}
		return true
	default:
		return false
	}
}

func typeDestructionKey(typ Type) string {
	if typ.Module != "" || typ.Name != "" {
		return typ.Module + "." + typ.Name
	}
	return ""
}

func CopyClassificationOf(typ Type) CopyClassification {
	return copyClassificationOf(typ, map[string]bool{})
}

func TriviallyCopyable(typ Type) bool {
	return CopyClassificationOf(typ) == CopyTrivial
}

func MoveOnly(typ Type) bool {
	return CopyClassificationOf(typ) == CopyMoveOnly
}

func copyClassificationOf(typ Type, visiting map[string]bool) CopyClassification {
	switch typ.Kind {
	case InvalidType, VoidType, NeverType:
		return CopyNonCopyable
	case BoolType,
		IntType,
		UintType,
		FloatType,
		DecimalType,
		CharType,
		RuneType,
		RawPtrType,
		FunctionType,
		InterfaceType,
		RegisterType,
		EnumType,
		StringType,
		SliceType:
		return CopyTrivial
	case ReferenceType:
		if typ.ReferenceMutable {
			return CopyMoveOnly
		}
		return CopyTrivial
	case StructType:
		switch typ.Name {
		case "Task",
			"Thread",
			"MutexGuard",
			"Subscription",
			"Channel",
			"Sender",
			"Receiver",
			"MessageTicket":
			return CopyMoveOnly
		case "Mutex",
			"Atomic",
			"ThreadLocal",
			"Event",
			"EventStorage",
			"ChannelOptions",
			"SenderID":
			return CopyNonCopyable
		}
		key := typeDestructionKey(typ)
		if key != "" {
			if visiting[key] {
				return CopyConditional
			}
			visiting[key] = true
			defer delete(visiting, key)
		}
		fields := make([]Type, 0, len(typ.Fields))
		for _, field := range typ.Fields {
			fields = append(fields, field.Type)
		}
		return aggregateCopyClassification(fields, visiting)
	case GenericType:
		return CopyConditional
	case ArrayType:
		if typ.Element == nil {
			return CopyTrivial
		}
		return aggregateCopyClassification([]Type{*typ.Element}, visiting)
	case ResultType:
		return aggregateCopyClassification(typ.TypeArgs, visiting)
	case UnionType:
		key := typeDestructionKey(typ)
		if key != "" {
			if visiting[key] {
				return CopyConditional
			}
			visiting[key] = true
			defer delete(visiting, key)
		}
		parts := []Type{}
		for _, variant := range typ.UnionVariants {
			if variant.Payload != nil {
				parts = append(parts, *variant.Payload)
			}
			for _, field := range variant.PayloadFields {
				parts = append(parts, field.Type)
			}
		}
		return aggregateCopyClassification(parts, visiting)
	default:
		return CopyNonCopyable
	}
}

func aggregateCopyClassification(parts []Type, visiting map[string]bool) CopyClassification {
	result := CopyTrivial
	for _, part := range parts {
		classification := copyClassificationOf(part, visiting)
		switch classification {
		case CopyMoveOnly:
			return CopyMoveOnly
		case CopyNonCopyable:
			return CopyNonCopyable
		case CopyConditional:
			if result == CopyTrivial {
				result = CopyConditional
			}
		case CopySemantic:
			if result == CopyTrivial {
				result = CopySemantic
			}
		}
	}
	return result
}

package sema

import (
	"fmt"
	"math/big"
	"unicode"

	"sec/internal/layout"
	"sec/internal/lexer"
)

type TypeKind string

// ArrayShapeKind is the canonical fixed-versus-dynamic distinction required by
// rules/mlir/packages/sec-mlir-dialect_package14.md sections 9-12. The empty
// value belongs to non-array types.
type ArrayShapeKind string

const (
	ArrayShapeFixed   ArrayShapeKind = "fixed"
	ArrayShapeDynamic ArrayShapeKind = "dynamic"
)

const (
	InvalidType   TypeKind = "invalid"
	AnyType       TypeKind = "any"
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
	// VariadicPackType is the compiler-known invocation-lifetime pack defined
	// by rules/declarations/functions.md sections 29-34. It is deliberately
	// distinct from arrays and slices because it has no source-visible layout.
	VariadicPackType TypeKind = "variadic-pack"
	FunctionType     TypeKind = "function"
	GenericType      TypeKind = "generic"
	InterfaceType    TypeKind = "interface"
	ErrorRootType    TypeKind = "error-root"
	NeverType        TypeKind = "never"
	VoidType         TypeKind = "void"
)

type Type struct {
	Name                  string
	Module                string
	Kind                  TypeKind
	Named                 bool
	Declared              bool
	Intrinsic             bool
	ExplicitlyNonCopyable bool
	// ErrorAssignable marks concrete error enums/unions and compiler-known
	// error families assignable to the open lowercase error root defined by
	// rules/errors/errorhandling.md.
	ErrorAssignable    bool
	NoCopyPolicyOrigin string
	Underlying         string
	Unit               string
	Dimension          Dimension
	// UnitSemantics retains the complete resolved quantity identity from
	// rules/types/units.md through frontend analysis and tooling.
	UnitSemantics              UnitSemantics
	ReferenceMutable           bool
	ReferenceOriginName        string
	ReferenceOriginToken       lexer.Token
	ReferenceOriginLocal       bool
	ReferenceOriginMatchScoped bool
	ReferenceOriginStorage     StorageOrigin
	ReferenceOriginGeneration  int
	ReferenceOriginDisplayName string
	ArenaDomainID              string
	MinInt                     *int64
	MaxInt                     *int64
	MinUint                    *uint64
	MaxUint                    *uint64
	MinInteger                 *big.Int
	MaxInteger                 *big.Int
	Contracts                  []Contract
	ExplicitDefault            *DefaultConstant
	InvalidExplicitDefault     bool
	EnumValues                 []string
	EnumConsts                 map[string]EnumValue
	EnumDefault                string
	BitWidth                   int64
	UnionVariants              []UnionVariant
	UnionDefault               string
	TypeArgs                   []Type
	ConstArgs                  []int64
	StaticElementCount         *big.Int
	Element                    *Type
	ArrayShape                 ArrayShapeKind
	// ArrayLengthDecimal is the immutable, exact semantic length of a fixed
	// array. Canonical values contain unsigned base-10 digits with no leading
	// zeroes (except "0"). Dynamic arrays leave it empty.
	ArrayLengthDecimal string
	// ArrayLength is a deprecated compatibility cache for legacy consumers and
	// old tests. It is populated only when the exact fixed length fits int64;
	// -1 is reserved for translation into the old dynamic-array model and is
	// never canonical semantic authority.
	ArrayLength            int64
	EventCapacity          int64
	EventCapacitySet       bool
	FunctionParameterTypes []Type
	FunctionReturnType     *Type
	// FunctionVariadic preserves native variadic shape for function values.
	// The final FunctionParameterTypes item is the element type when true.
	FunctionVariadic bool
	// FunctionCapability is the callable environment authority from
	// rules/declarations/lambda-functions.md. The zero value is normalized to
	// CallableShared for compatibility with pre-capability semantic facts.
	FunctionCapability      CallableCapability
	GenericParameters       []string
	Fields                  []StructField
	RegisterWidth           int64
	RegisterAllocationOrder string
	RegisterByteOrder       string
	RegisterFields          []RegisterField
	Properties              []Property
	Events                  []Event
	Implements              []Type
	InterfaceMethods        []Function
	InterfaceProperties     []InterfaceProperty
	InterfaceEvents         []InterfaceEvent
}

// NewVariadicPackType constructs the ephemeral compiler-known parameter pack
// mandated by rules/declarations/functions.md sections 29-31. It represents
// neither storage nor a source-level collection type.
func NewVariadicPackType(element Type) Type {
	return Type{
		Name:    "..." + typeDisplayName(element),
		Kind:    VariadicPackType,
		Element: &element,
	}
}

// NewFixedArrayType constructs the Package 14 canonical fixed-array shape.
// The length is copied into immutable decimal form and never host-truncated.
func NewFixedArrayType(element Type, length *big.Int) Type {
	if length == nil || length.Sign() < 0 {
		return Type{Kind: InvalidType}
	}
	exact := length.String()
	legacy := int64(0)
	if length.IsInt64() {
		legacy = length.Int64()
	}
	return Type{
		Name:               typeDisplayName(element) + "[" + exact + "]",
		Kind:               ArrayType,
		Element:            &element,
		ArrayShape:         ArrayShapeFixed,
		ArrayLengthDecimal: exact,
		ArrayLength:        legacy,
	}
}

// NewDynamicArrayType constructs an owning dynamic array without a semantic
// length. The sentinel exists only in the legacy compatibility cache.
func NewDynamicArrayType(element Type) Type {
	return Type{
		Name:        typeDisplayName(element) + "[]",
		Kind:        ArrayType,
		Element:     &element,
		ArrayShape:  ArrayShapeDynamic,
		ArrayLength: dynamicArrayLength,
	}
}

// arrayShapeOf reads the explicit Package 14 shape. The legacy int64 cache is
// intentionally not consulted by canonical semantic consumers.
func arrayShapeOf(typ Type) ArrayShapeKind {
	if typ.Kind != ArrayType {
		return ""
	}
	return typ.ArrayShape
}

// exactFixedArrayLength returns a defensive arbitrary-precision copy.
func exactFixedArrayLength(typ Type) (*big.Int, bool) {
	if arrayShapeOf(typ) != ArrayShapeFixed {
		return nil, false
	}
	if typ.ArrayLengthDecimal == "" {
		return nil, false
	}
	length, ok := new(big.Int).SetString(typ.ArrayLengthDecimal, 10)
	if !ok || length.Sign() < 0 || length.String() != typ.ArrayLengthDecimal {
		return nil, false
	}
	return length, true
}

// FixedArrayLength returns a defensive copy of the exact Package 14 length.
func FixedArrayLength(typ Type) (*big.Int, bool) {
	return exactFixedArrayLength(typ)
}

// ValidateArrayTypeForScalarPlan independently applies one CompilationPlan's
// target-uint bound to an already resolved type. Callers may validate the same
// immutable Sema fact for multiple outputs without reparsing or truncation.
func ValidateArrayTypeForScalarPlan(typ Type, plan layout.ResolvedScalarPlan) error {
	if plan.PointerWidthBits != 32 && plan.PointerWidthBits != 64 {
		return fmt.Errorf("array length validation requires a 32- or 64-bit scalar plan")
	}
	if typ.Kind == ArrayType && arrayShapeOf(typ) == ArrayShapeFixed {
		length, ok := exactFixedArrayLength(typ)
		if !ok {
			return fmt.Errorf("fixed array has no canonical exact length")
		}
		if !arrayLengthFitsUint(length, plan.PointerWidthBits) {
			return fmt.Errorf("fixed-array length %s overflows target uint%d", length.String(), plan.PointerWidthBits)
		}
	}
	if typ.Element != nil {
		if err := ValidateArrayTypeForScalarPlan(*typ.Element, plan); err != nil {
			return err
		}
	}
	for _, argument := range typ.TypeArgs {
		if err := ValidateArrayTypeForScalarPlan(argument, plan); err != nil {
			return err
		}
	}
	return nil
}

// legacyArrayLength is the sole checked bridge from Package 14 shape facts to
// the old int64/sentinel model. Canonical consumers must use ArrayShape and the
// exact decimal length instead.
func legacyArrayLength(typ Type) (int64, bool) {
	if arrayShapeOf(typ) == ArrayShapeDynamic {
		return dynamicArrayLength, true
	}
	length, ok := exactFixedArrayLength(typ)
	if !ok || !length.IsInt64() {
		return 0, false
	}
	return length.Int64(), true
}

func sameArrayShape(left, right Type) bool {
	leftShape, rightShape := arrayShapeOf(left), arrayShapeOf(right)
	if leftShape != rightShape {
		return false
	}
	if leftShape == ArrayShapeDynamic {
		return true
	}
	leftLength, leftOK := exactFixedArrayLength(left)
	rightLength, rightOK := exactFixedArrayLength(right)
	return leftOK && rightOK && leftLength.Cmp(rightLength) == 0
}

// CallableCapability is the Sema-owned invocation authority of a function
// value under rules/declarations/lambda-functions.md. It is independent of
// parameter ownership modes and named-method receiver syntax.
type CallableCapability string

const (
	CallableShared    CallableCapability = "shared"
	CallableMutable   CallableCapability = "mutable"
	CallableConsuming CallableCapability = "consuming"
)

type EnumValue struct {
	Name        string
	Value       *big.Int
	StringValue *string
	Token       lexer.Token
}

// enumValueClassKey returns the canonical closed-enum coverage identity for
// integer- and string-backed members. Declared aliases intentionally share a
// key while retaining separate EnumValue names.
//
// Rules:
//   - rules/declarations/enums.md — "Value aliases"
//   - rules/declarations/enums.md — "match and exhaustiveness"
func enumValueClassKey(value EnumValue) (string, bool) {
	if value.Value != nil {
		return "integer:" + value.Value.String(), true
	}
	if value.StringValue != nil {
		return "string:" + *value.StringValue, true
	}
	return "", false
}

type UnionVariant struct {
	Name          string
	Payload       *Type
	PayloadFields []StructField
	Token         lexer.Token
	Default       bool
	DefaultToken  lexer.Token
}

type StructField struct {
	Name  string
	Type  Type
	Token lexer.Token
	Tags  []StructTag
}

type RegisterField struct {
	Name      string
	Width     int64
	BitOffset int64
	Unit      string
	Type      Type
	Token     lexer.Token
	Access    RegisterFieldAccess
}

type RegisterFieldAccess string

const (
	RegisterReadWrite      RegisterFieldAccess = "read-write"
	RegisterReadOnly       RegisterFieldAccess = "read-only"
	RegisterWriteOnly      RegisterFieldAccess = "write-only"
	RegisterWriteOneClear  RegisterFieldAccess = "write-one-clear"
	RegisterWriteZeroClear RegisterFieldAccess = "write-zero-clear"
	RegisterClearOnRead    RegisterFieldAccess = "clear-on-read"
)

type StructTag struct {
	Key   string
	Value string
}

type Property struct {
	Name      string
	Type      Type
	Token     lexer.Token
	Static    bool
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
	StorageOriginInline   StorageOrigin = "Inline"
	StorageOriginStatic   StorageOrigin = "Static"
	StorageOriginArena    StorageOrigin = "Arena"
	StorageOriginExternal StorageOrigin = "External"
	StorageOriginForeign  StorageOrigin = "Foreign"
	StorageOriginUnknown  StorageOrigin = "Unknown"
)

type AddressStability string

const (
	AddressStabilityMovable AddressStability = "Movable"
	AddressStabilityStable  AddressStability = "Stable"
	AddressStabilityFixed   AddressStability = "Fixed"
	AddressStabilityUnknown AddressStability = "Unknown"
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
	Name           string
	Type           Type
	Token          lexer.Token
	Static         bool
	RequiresGet    bool
	RequiresSet    bool
	SetterFallible bool
}

type UnitCategory string

const (
	PhysicalUnit    UnitCategory = "physical"
	CurrencyUnit    UnitCategory = "currency"
	InformationUnit UnitCategory = "information"
	RatioUnit       UnitCategory = "ratio"
	OtherUnit       UnitCategory = "other"
)

type UnitTransform string

const (
	LinearUnitTransform      UnitTransform = "linear"
	AffineUnitTransform      UnitTransform = "affine"
	LogarithmicUnitTransform UnitTransform = "logarithmic"
)

type UnitStatus string

const (
	StatusActive     UnitStatus = "active"
	StatusDeprecated UnitStatus = "deprecated"
	StatusObsolete   UnitStatus = "obsolete"
)

// UnitIdentityKind preserves the named-versus-structural distinction required
// by rules/types/units.md. Dimension equality alone must never manufacture a
// named unit result.
type UnitIdentityKind string

const (
	NamedUnitIdentity      UnitIdentityKind = "named"
	StructuralUnitIdentity UnitIdentityKind = "structural"
)

type UnitPointRole string

const (
	UnitVectorRole     UnitPointRole = "vector"
	UnitPointRolePoint UnitPointRole = "point"
	UnitDifferenceRole UnitPointRole = "difference"
)

// UnitSemantics is the compiler-owned resolved quantity descriptor shared by
// Sema and tooling. SourceFactors retains source order while Factors and
// Dimension are normalized for algebra.
type UnitSemantics struct {
	Identity           UnitIdentityKind
	Named              string
	Source             string
	SourceFactors      []string
	Factors            map[string]int
	Categories         map[UnitCategory]bool
	Kind               string
	Transform          UnitTransform
	Scale              *big.Rat
	Offset             *big.Rat
	Origin             string
	LogBase            *big.Rat
	LogFactor          *big.Rat
	Reference          string
	Role               UnitPointRole
	ConversionRequired bool
	ConversionExact    bool
}

type UnitDefinition struct {
	Name                 string
	LongName             string
	Symbol               string
	Category             UnitCategory
	Dimension            Dimension
	DimensionEstablished bool
	Kind                 string
	Scale                string
	ScaleValue           *big.Rat
	DefaultNumeric       string
	IsBaseUnit           bool
	Status               UnitStatus
	System               string
	Transform            UnitTransform
	Offset               string
	OffsetValue          *big.Rat
	Origin               string
	LogBase              string
	LogBaseValue         *big.Rat
	LogFactor            string
	LogFactorValue       *big.Rat
	Reference            string
	Token                lexer.Token
}

type Function struct {
	Name              string
	Module            string
	CompilerKnownID   string
	CompilerInternal  bool
	GenericParameters []string
	Parameters        []FunctionParameter
	ReturnType        Type
	Token             lexer.Token
	Extern            bool
	Unsafe            bool
	Static            bool
	ABI               string
	LinkName          string
	AllocationEffect  AllocationEffect
	ImplTarget        string
	ReceiverMutable   bool
	ReceiverConsuming bool
	Initializer       bool
	ConstructionType  *Type
	ConstructionError *Type
	ReturnOrigin      localReferenceOrigin
	HasReturnOrigin   bool
}

type FunctionParameter struct {
	Name       string
	Type       Type
	Token      lexer.Token
	Ref        bool
	MutableRef bool
	Consuming  bool
	// Variadic marks the final `name: ...T` parameter defined by
	// rules/declarations/functions.md sections 28 and 36. Type remains T.
	Variadic bool
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
	EnumDefault         DefaultKind = "enum"
	UnionDefault        DefaultKind = "union"
)

type DefaultResolution struct {
	Kind   DefaultKind
	Value  DefaultConstant
	Fields []DefaultField
	// Elements is a bounded legacy materialization only. Package 14 consumers
	// use ArrayLengthDecimal and ArrayElementDefault as the compact authority.
	Elements            []DefaultResolution
	ArrayLengthDecimal  string
	ArrayElementDefault *DefaultResolution
	Variant             string
	Payload             *DefaultResolution
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
	ImplicitMember   bool
	Token            lexer.Token
	Addressed        bool
	Address          string
	Volatile         bool
	Storage          StorageOrigin
	AddressStability AddressStability
	Local            bool
	ScopeDepth       int
	RegisterAccess   RegisterFieldAccess
}

func builtinTypes() map[string]Type {
	types := map[string]Type{
		"any":    {Name: "any", Kind: AnyType},
		"bool":   {Name: "bool", Kind: BoolType},
		"byte":   unsignedType("byte", 255),
		"char":   {Name: "char", Kind: CharType},
		"rune":   {Name: "rune", Kind: RuneType},
		"error":  {Name: "error", Kind: ErrorRootType},
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
		"ArithmeticError": {
			Name:       "ArithmeticError",
			Kind:       EnumType,
			Underlying: "uint",
			EnumValues: []string{"Overflow", "DivisionByZero", "InvalidShift"},
			EnumConsts: builtinEnumConsts([]string{"Overflow", "DivisionByZero", "InvalidShift"}),
		},
		"IndexError": {
			Name:       "IndexError",
			Kind:       EnumType,
			Underlying: "uint",
			EnumValues: []string{"OutOfBounds"},
			EnumConsts: builtinEnumConsts([]string{"OutOfBounds"}),
		},
		"EnumValueError": {
			Name:        "EnumValueError",
			Kind:        EnumType,
			Underlying:  "uint",
			EnumValues:  []string{"UndeclaredValue", "OutOfRange"},
			EnumConsts:  builtinEnumConsts([]string{"UndeclaredValue", "OutOfRange"}),
			EnumDefault: "UndeclaredValue",
		},
		"ContractError": {Name: "ContractError", Kind: StructType},
		"CollectionError": {
			Name:       "CollectionError",
			Kind:       EnumType,
			Underlying: "uint",
			EnumValues: []string{"AllocationFailed", "CapacityExceeded", "SizeOverflow"},
			EnumConsts: builtinEnumConsts([]string{"AllocationFailed", "CapacityExceeded", "SizeOverflow"}),
		},
		"Option": {
			Name:              "Option",
			Kind:              UnionType,
			GenericParameters: []string{"T"},
			UnionVariants: []UnionVariant{
				{Name: "Some", Payload: &Type{Name: "T", Kind: GenericType}},
				{Name: "None"},
			},
		},
		// Iterator is the compiler-known, statically dispatched iteration
		// contract from rules/control-flow/flowcontrol_for.md section 37. It has
		// no runtime representation or dynamic-dispatch requirement: a concrete
		// type participates only through explicit implements Iterator[T].
		"Iterator": {
			Name:              "Iterator",
			Kind:              InterfaceType,
			GenericParameters: []string{"T"},
			InterfaceMethods: []Function{
				{
					Name:            "Next",
					CompilerKnownID: "CKM-ITERATOR-NEXT",
					ReceiverMutable: true,
					ReturnType: Type{
						Name: "Option",
						Kind: UnionType,
						TypeArgs: []Type{
							{Name: "T", Kind: GenericType},
						},
					},
				},
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
		// Temporal types are compiler-known opaque identities until the
		// dedicated temporal rulebook defines representation and operations.
		"date":     {Name: "date", Kind: StructType},
		"time":     {Name: "time", Kind: StructType},
		"datetime": {Name: "datetime", Kind: StructType},
		"duration": {Name: "duration", Kind: StructType},
		"float":    {Name: "float", Kind: FloatType},
		"float32":  {Name: "float32", Kind: FloatType},
		"float64":  {Name: "float64", Kind: FloatType},
		"int":      signedType("int", -1<<63, 1<<63-1),
		"int8":     signedType("int8", -1<<7, 1<<7-1),
		"int16":    signedType("int16", -1<<15, 1<<15-1),
		"int32":    signedType("int32", -1<<31, 1<<31-1),
		"int64":    signedType("int64", -1<<63, 1<<63-1),
		"int128":   signedBigType("int128", "-170141183460469231731687303715884105728", "170141183460469231731687303715884105727"),
		"int256":   signedBigType("int256", "-57896044618658097711785492504343953926634992332820282019728792003956564819968", "57896044618658097711785492504343953926634992332820282019728792003956564819967"),
		"string":   {Name: "string", Kind: StringType},
		"uint":     unsignedType("uint", ^uint64(0)),
		"uint8":    unsignedType("uint8", 1<<8-1),
		"uint16":   unsignedType("uint16", 1<<16-1),
		"uint32":   unsignedType("uint32", 1<<32-1),
		"uint64":   unsignedType("uint64", ^uint64(0)),
		"uint128":  unsignedBigType("uint128", "340282366920938463463374607431768211455"),
		"uint256":  unsignedBigType("uint256", "115792089237316195423570985008687907853269984665640564039457584007913129639935"),
		"void":     {Name: "void", Kind: VoidType},
	}

	// rules/errors/errorhandling.md defines these compiler-known failure
	// families as concrete inhabitants of the open error root.
	for _, name := range []string{
		"AllocationError", "ArithmeticError", "IndexError", "EnumValueError", "ContractError", "CollectionError",
		"ThreadSpawnError", "ThreadStartError", "ThreadSchedulingError", "ThreadTerminationError", "ThreadContextError",
	} {
		typ := types[name]
		typ.ErrorAssignable = true
		types[name] = typ
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

func targetSignedIntegerType(name string, width uint16) Type {
	limit := new(big.Int).Lsh(big.NewInt(1), uint(width-1))
	min := new(big.Int).Neg(new(big.Int).Set(limit))
	max := new(big.Int).Sub(new(big.Int).Set(limit), big.NewInt(1))
	typ := Type{Name: name, Kind: IntType, MinInteger: min, MaxInteger: max, BitWidth: int64(width), Intrinsic: true}
	if width <= 64 {
		min64, max64 := min.Int64(), max.Int64()
		typ.MinInt, typ.MaxInt = &min64, &max64
	}
	return typ
}

func targetUnsignedIntegerType(name string, width uint16) Type {
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(width)), big.NewInt(1))
	typ := Type{Name: name, Kind: UintType, MinInteger: big.NewInt(0), MaxInteger: max, BitWidth: int64(width), Intrinsic: true}
	if width <= 64 {
		min64, max64 := uint64(0), max.Uint64()
		typ.MinUint, typ.MaxUint = &min64, &max64
	}
	return typ
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
		// rules/collections/collections.md; correction26.md: T[] owns
		// dynamic storage and therefore always requires destruction. T[N]
		// remains an inline aggregate whose trait follows its element type.
		if arrayShapeOf(typ) == ArrayShapeDynamic {
			return false
		}
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
		// rules/memory/arena.md; correction29.md: destroying an Arena ends
		// its domain and releases or returns its backing storage.
		if typ.Name == "Arena" {
			return false
		}
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

func EqualityComparable(typ Type) bool {
	return equalityComparable(typ, map[string]bool{})
}

func equalityComparable(typ Type, visiting map[string]bool) bool {
	switch typ.Kind {
	case AnyType:
		// An any value may contain a non-trivial or move-only concrete value.
		return false
	case BoolType,
		IntType,
		UintType,
		FloatType,
		DecimalType,
		CharType,
		RuneType,
		StringType,
		EnumType,
		RawPtrType:
		return true
	case ReferenceType:
		return typ.Element != nil && typ.Element.Kind != SliceType
	case ArrayType:
		// rules/collections/collections.md; correction26.md does not define
		// ordinary identity or content equality for owning dynamic arrays.
		return arrayShapeOf(typ) == ArrayShapeFixed && typ.Element != nil && equalityComparable(*typ.Element, visiting)
	case StructType:
		// Compiler-known struct types without ordinary stored-field semantics
		// represent resources, collections, or opaque runtime descriptors.
		if typ.Intrinsic {
			return false
		}
		key := typeDestructionKey(typ)
		if key != "" {
			if visiting[key] {
				return false
			}
			visiting[key] = true
			defer delete(visiting, key)
		}
		for _, field := range typ.Fields {
			if !equalityComparable(field.Type, visiting) {
				return false
			}
		}
		return true
	case UnionType:
		key := typeDestructionKey(typ)
		if key != "" {
			if visiting[key] {
				return false
			}
			visiting[key] = true
			defer delete(visiting, key)
		}
		for _, variant := range typ.UnionVariants {
			if variant.Payload != nil && !equalityComparable(*variant.Payload, visiting) {
				return false
			}
			for _, field := range variant.PayloadFields {
				if !equalityComparable(field.Type, visiting) {
					return false
				}
			}
		}
		return true
	default:
		return false
	}
}

func TriviallyCopyable(typ Type) bool {
	return CopyClassificationOf(typ) == CopyTrivial
}

func MoveOnly(typ Type) bool {
	return CopyClassificationOf(typ) == CopyMoveOnly
}

func requiresOwnershipTransfer(typ Type) bool {
	switch CopyClassificationOf(typ) {
	case CopyMoveOnly, CopyNonCopyable, CopyConditional:
		return true
	default:
		return false
	}
}

func copyClassificationOf(typ Type, visiting map[string]bool) CopyClassification {
	if typ.ExplicitlyNonCopyable {
		return CopyNonCopyable
	}
	switch typ.Kind {
	case InvalidType, VoidType, NeverType:
		return CopyNonCopyable
	case AnyType:
		return CopyConditional
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
			"Arena",
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
		// rules/collections/collections.md; correction26.md: an owning
		// dynamic array transfers its allocation regardless of element traits.
		if arrayShapeOf(typ) == ArrayShapeDynamic {
			return CopyMoveOnly
		}
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

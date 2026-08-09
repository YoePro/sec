// Package semantic defines Sec's frontend-neutral Semantic IR.
package semantic

import (
	"fmt"
	"math/big"
)

const Version uint32 = 1

type TypeID uint32
type FunctionID string
type BlockID uint32
type ValueID uint32
type StorageID uint32

type Location struct {
	File   string
	Line   int
	Column int
}

type OwnershipClass string

const (
	OwnershipOwned             OwnershipClass = "owned"
	OwnershipSharedReference   OwnershipClass = "shared-reference"
	OwnershipMutableReference  OwnershipClass = "mutable-reference"
	OwnershipRawPointer        OwnershipClass = "raw-pointer"
	OwnershipImmediate         OwnershipClass = "immediate"
	OwnershipCompilerTemporary OwnershipClass = "compiler-temporary"
)

type TypeKind string

const (
	TypeVoid       TypeKind = "void"
	TypeNever      TypeKind = "never"
	TypeBool       TypeKind = "bool"
	TypeByte       TypeKind = "byte"
	TypeChar       TypeKind = "char"
	TypeRune       TypeKind = "rune"
	TypeString     TypeKind = "string"
	TypeDecimal    TypeKind = "decimal"
	TypeDecimal128 TypeKind = "decimal128"
	TypeInt        TypeKind = "int"
	TypeUint       TypeKind = "uint"
	TypeFloat      TypeKind = "float"
	TypeNamed      TypeKind = "named"
)

type Type struct {
	Kind       TypeKind
	Name       string
	Module     string
	Identity   string
	Base       TypeID
	Signed     bool
	BitWidth   uint16
	TargetSize bool
}

type TypeTable struct {
	types []Type
	byKey map[string]TypeID
}

func NewTypeTable() *TypeTable { return &TypeTable{byKey: map[string]TypeID{}} }

func (t *TypeTable) Intern(typ Type) TypeID {
	if t.byKey == nil {
		t.byKey = map[string]TypeID{}
	}
	key := typeKey(typ)
	if id, ok := t.byKey[key]; ok {
		return id
	}
	id := TypeID(len(t.types) + 1)
	t.types = append(t.types, typ)
	t.byKey[key] = id
	return id
}

func (t *TypeTable) Lookup(id TypeID) (Type, bool) {
	if t == nil || id == 0 || int(id) > len(t.types) {
		return Type{}, false
	}
	return t.types[id-1], true
}

// Get is the diagnostic form of Lookup for clients which require a valid ID.
func (t *TypeTable) Get(id TypeID) (Type, error) {
	typ, ok := t.Lookup(id)
	if !ok {
		return Type{}, fmt.Errorf("invalid TypeID %d", id)
	}
	return typ, nil
}

func (t *TypeTable) All() []Type {
	if t == nil {
		return nil
	}
	return append([]Type(nil), t.types...)
}

func typeKey(t Type) string {
	return string(t.Kind) + "\x00" + t.Module + "\x00" + t.Name + "\x00" + t.Identity + "\x00" +
		fmtUint(uint64(t.Base)) + "\x00" + fmtUint(uint64(t.BitWidth)) + "\x00" + boolKey(t.Signed) + boolKey(t.TargetSize)
}

func fmtUint(value uint64) string {
	if value == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for value > 0 {
		i--
		b[i] = byte('0' + value%10)
		value /= 10
	}
	return string(b[i:])
}
func boolKey(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

type Module struct {
	Version     uint32
	Identity    string
	SourceFiles []string
	Types       *TypeTable
	Functions   []*Function
}

type Function struct {
	ID         FunctionID
	Name       string
	LinkName   string
	Parameters []Parameter
	ReturnType TypeID
	Unsafe     bool
	Extern     bool
	ABI        string
	Entry      BlockID
	Blocks     []*Block
	Storages   []Storage
	Location   Location
}

type Parameter struct {
	Name     string
	Value    Value
	Location Location
}
type Value struct {
	ID        ValueID
	Type      TypeID
	Ownership OwnershipClass
	Location  Location
}
type Block struct {
	ID         BlockID
	Parameters []Value
	Operations []Operation
}

type StorageClass string

const StorageLocalAutomatic StorageClass = "local-automatic"

type Storage struct {
	ID       StorageID
	Name     string
	Type     TypeID
	Mutable  bool
	Class    StorageClass
	Location Location
}

type OpKind string

const (
	OpConstInt          OpKind = "const.int"
	OpConstBool         OpKind = "const.bool"
	OpConstString       OpKind = "const.string"
	OpConstDecimal      OpKind = "const.decimal"
	OpConstFloat        OpKind = "const.float"
	OpReturn            OpKind = "return"
	OpStorageDeclare    OpKind = "storage.declare"
	OpStorageInit       OpKind = "storage.init"
	OpStorageLoad       OpKind = "storage.load"
	OpStorageStore      OpKind = "storage.store"
	OpDirectCall        OpKind = "call.direct"
	OpForeignCall       OpKind = "call.foreign"
	OpBranch            OpKind = "branch"
	OpCondBranch        OpKind = "conditional-branch"
	OpIntUnaryPlus      OpKind = "int.unary-plus"
	OpIntNegChecked     OpKind = "int.neg-checked"
	OpIntBitNot         OpKind = "int.bit-not"
	OpIntBinaryChecked  OpKind = "int.binary-checked"
	OpIntBitwise        OpKind = "int.bitwise"
	OpIntShiftChecked   OpKind = "int.shift-checked"
	OpIntCompare        OpKind = "int.compare"
	OpArithmeticFailure OpKind = "fail.arithmetic"
)

type IntegerCheckedBinaryKind string

const (
	IntegerCheckedAdd       IntegerCheckedBinaryKind = "add"
	IntegerCheckedSubtract  IntegerCheckedBinaryKind = "subtract"
	IntegerCheckedMultiply  IntegerCheckedBinaryKind = "multiply"
	IntegerCheckedDivide    IntegerCheckedBinaryKind = "divide"
	IntegerCheckedRemainder IntegerCheckedBinaryKind = "remainder"
)

type IntegerBitwiseKind string

const (
	IntegerBitwiseAnd IntegerBitwiseKind = "and"
	IntegerBitwiseOr  IntegerBitwiseKind = "or"
	IntegerBitwiseXor IntegerBitwiseKind = "xor"
)

type IntegerShiftKind string

const (
	IntegerShiftLeftUnsigned  IntegerShiftKind = "left_unsigned"
	IntegerShiftLeftSigned    IntegerShiftKind = "left_signed"
	IntegerShiftRightUnsigned IntegerShiftKind = "right_unsigned"
	IntegerShiftRightSigned   IntegerShiftKind = "right_signed"
)

type IntegerComparePredicate string

const (
	IntegerCompareEQ IntegerComparePredicate = "eq"
	IntegerCompareNE IntegerComparePredicate = "ne"
	IntegerCompareLT IntegerComparePredicate = "lt"
	IntegerCompareLE IntegerComparePredicate = "le"
	IntegerCompareGT IntegerComparePredicate = "gt"
	IntegerCompareGE IntegerComparePredicate = "ge"
)

type ArithmeticFailureCategory string

const (
	ArithmeticFailureOverflow  ArithmeticFailureCategory = "overflow"
	ArithmeticFailureDivision  ArithmeticFailureCategory = "division"
	ArithmeticFailureRemainder ArithmeticFailureCategory = "remainder"
	ArithmeticFailureShift     ArithmeticFailureCategory = "shift"
)

type ArgumentAction string

const ArgumentCopyTrivial ArgumentAction = "copy-trivial"

type DecimalConstant struct {
	Coefficient *big.Int
	Scale       uint32
	Lexeme      string
}
type BranchTarget struct {
	Block     BlockID
	Arguments []ValueID
}
type Operation struct {
	Kind            OpKind
	Location        Location
	Results         []Value
	Operands        []ValueID
	Integer         *big.Int
	Bool            *bool
	String          string
	Decimal         *DecimalConstant
	FloatLexeme     string
	Storage         StorageID
	Callee          FunctionID
	ArgumentActions []ArgumentAction
	Successors      []BranchTarget
	IntegerBinary   IntegerCheckedBinaryKind
	IntegerBitwise  IntegerBitwiseKind
	IntegerShift    IntegerShiftKind
	IntegerCompare  IntegerComparePredicate
	FailureCategory ArithmeticFailureCategory
	Operator        string
}

func (o Operation) IsTerminator() bool {
	return o.Kind == OpReturn || o.Kind == OpBranch || o.Kind == OpCondBranch || o.Kind == OpArithmeticFailure
}

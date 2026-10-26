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
type SymbolID string
type EnumCaseID uint32
type UnionVariantIndex uint32

// StructFieldID is the declaration-order, non-layout identity defined by
// rules/mlir/semantic-ir/sec_semantic_ir_struct_v1.md section 2.
type StructFieldID uint32

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
	TypeVoid                    TypeKind = "void"
	TypeNever                   TypeKind = "never"
	TypeBool                    TypeKind = "bool"
	TypeByte                    TypeKind = "byte"
	TypeChar                    TypeKind = "char"
	TypeRune                    TypeKind = "rune"
	TypeString                  TypeKind = "string"
	TypeDecimal                 TypeKind = "decimal"
	TypeDecimal128              TypeKind = "decimal128"
	TypeInt                     TypeKind = "int"
	TypeUint                    TypeKind = "uint"
	TypeFloat                   TypeKind = "float"
	TypeNamed                   TypeKind = "named"
	TypeArithmeticFailureReason TypeKind = "arithmetic-failure-reason"
	TypeCoreError               TypeKind = "core-error"
	TypeResult                  TypeKind = "result"
	TypeEnum                    TypeKind = "enum"
	TypeUnion                   TypeKind = "union"
	TypeStruct                  TypeKind = "struct"
	TypeArray                   TypeKind = "array"
	// TypeRegister is a detached nominal register[N] value (MD-077 § 5).
	TypeRegister TypeKind = "register"
)

type Type struct {
	Kind       TypeKind
	Name       string
	Module     string
	Identity   string
	Base       TypeID
	Success    TypeID
	Error      TypeID
	Underlying TypeID
	Element    TypeID
	Length     string
	TypeArgs   []TypeID
	Signed     bool
	BitWidth   uint16
	TargetSize bool
	// Mutable is the authority of a place (rw) or reference (ref mut) type.
	Mutable bool
}

type EnumRepresentationKind string

const (
	EnumRepresentationInteger   EnumRepresentationKind = "integer"
	EnumRepresentationBitBacked EnumRepresentationKind = "bit-backed"
)

type EnumCase struct {
	ID       EnumCaseID
	Name     string
	Value    *big.Int
	Location Location
}

type EnumDefinition struct {
	TypeID             TypeID
	SymbolID           SymbolID
	Name               string
	Underlying         TypeID
	RepresentationKind EnumRepresentationKind
	BitWidth           uint16
	Cases              []EnumCase
	Location           Location
}

type UnionVariantKind string

const (
	UnionVariantEmpty  UnionVariantKind = "empty"
	UnionVariantSingle UnionVariantKind = "single"
	UnionVariantFields UnionVariantKind = "fields"
)

type UnionPayloadField struct {
	Name     string
	Type     TypeID
	Location Location
}

type UnionVariantDefinition struct {
	Index         UnionVariantIndex
	Name          string
	Kind          UnionVariantKind
	Payload       TypeID
	PayloadFields []UnionPayloadField
	// SyntheticPayloadStruct is the canonical Package 13 whole-value view from
	// rules/mlir/packages/sec-mlir-dialect_package13.md sections 18 and 66-68.
	SyntheticPayloadStruct TypeID
	Location               Location
}

type UnionDefinition struct {
	TypeID                TypeID
	SymbolID              SymbolID
	Name                  string
	TypeArguments         []TypeID
	Variants              []UnionVariantDefinition
	CopyClassification    string
	TriviallyDestructible bool
	LayoutRef             string
	Location              Location
}

// StructTag through StructDefinition implement the canonical value metadata in
// rules/mlir/semantic-ir/sec_semantic_ir_struct_v1.md sections 2-6.
type StructTag struct{ Key, Value string }

type StructFieldDefinition struct {
	ID       StructFieldID
	Name     string
	Type     TypeID
	Tags     []StructTag
	Location Location
}

type StructSyntheticOrigin string

const (
	StructSyntheticNone         StructSyntheticOrigin = ""
	StructSyntheticUnionPayload StructSyntheticOrigin = "union-payload"
)

type StructDefinition struct {
	TypeID                TypeID
	SymbolID              SymbolID
	Name                  string
	TypeArguments         []TypeID
	Fields                []StructFieldDefinition
	CopyClassification    string
	TriviallyDestructible bool
	Defaultable           bool
	LayoutRef             string
	SyntheticOrigin       StructSyntheticOrigin
	Location              Location
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
	key := string(t.Kind) + "\x00" + t.Module + "\x00" + t.Name + "\x00" + t.Identity + "\x00" +
		fmtUint(uint64(t.Base)) + "\x00" + fmtUint(uint64(t.Success)) + "\x00" + fmtUint(uint64(t.Error)) + "\x00" +
		fmtUint(uint64(t.Underlying)) + "\x00" + fmtUint(uint64(t.Element)) + "\x00" + t.Length + "\x00" +
		fmtUint(uint64(t.BitWidth)) + "\x00" + boolKey(t.Signed) + boolKey(t.TargetSize) + boolKey(t.Mutable)
	for _, argument := range t.TypeArgs {
		key += "\x00" + fmtUint(uint64(argument))
	}
	return key
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
	Enums       []EnumDefinition
	Unions      []UnionDefinition
	Structs     []StructDefinition
	// Registers is the module's register-definition table (MD-077 § 10(1)).
	Registers []RegisterDefinition
	// DestructionPlans are the type-level destruction plans own.* operations
	// reference (P17 §§ 20–21, 57; ownership_ops.go).
	DestructionPlans []DestructionPlanRecord
	// Statics are the module's static-duration storage locations
	// (semantic_ir.md § 28), in identity order.
	Statics []Static
	// PointerWidthBits is the selected CompilationPlan's pointer width the
	// module was analyzed for, or zero without a plan; plan-specific
	// conversion proofs must match it (MD-104).
	PointerWidthBits uint16
}

// ConversionProof is the Sema-established exactness contract carried by
// numeric.convert-exact and enum.to-integer (semantic_ir.md § 124, MD-104):
// its basis, and the plan width when the proof depends on target-sized types
// (zero: valid on every supported plan).
type ConversionProof struct {
	Basis                string
	PlanPointerWidthBits uint16
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
	Matches    []MatchRecord
	Loops      []LoopRecord
	// NoPanic records Sema's verified @noPanic guarantee (written or implied)
	// so later stages need not re-derive it; NoPanicSource names its origin.
	NoPanic       bool
	NoPanicSource string
	// ReturnOrigins is the returned-reference origin summary of a function
	// whose result is a direct reference: the finite set of external roots
	// and projection paths the result may refer to (package15.md § 62).
	ReturnOrigins []ReturnOrigin
	Location      Location
}

// ReturnOriginKind names the external root of a returned reference.
type ReturnOriginKind string

const (
	ReturnOriginParameter ReturnOriginKind = "parameter"
	ReturnOriginReceiver  ReturnOriginKind = "receiver"
	ReturnOriginStatic    ReturnOriginKind = "static"
)

// ReturnOriginStepKind is one projection below a returned reference's root.
type ReturnOriginStepKind string

const (
	ReturnOriginField   ReturnOriginStepKind = "field"
	ReturnOriginElement ReturnOriginStepKind = "element"
)

// ReturnOriginStep is a stored-field projection (by P13 field identity) or a
// fixed-array element projection (constant Index, or nil for a dynamic one).
type ReturnOriginStep struct {
	Kind  ReturnOriginStepKind
	Field StructFieldID
	Index *big.Int
}

// ReturnOrigin is one possible origin of a returned reference: a parameter
// by position or the receiver, and the projection path below its referent.
// Source names are never the identity.
type ReturnOrigin struct {
	Kind      ReturnOriginKind
	Parameter int
	// Static is the module static of a static origin.
	Static StaticID
	Path   []ReturnOriginStep
}

type MatchID uint32

// LoopID identifies one source loop inside its function.
type LoopID uint32

// LoopKind distinguishes loop forms only as far as their semantics differ.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 65(2), § 67(4)
type LoopKind string

const (
	LoopWhile LoopKind = "while"
	// LoopForInfinite is `for { }` (semantic_ir.md § 65(2)).
	LoopForInfinite LoopKind = "for-infinite"
	// LoopForRange is numeric range iteration (semantic_ir.md § 65(2)).
	LoopForRange LoopKind = "for-range"
)

// LoopEdge classifies a control transfer that belongs to a loop's CFG.
type LoopEdge string

const (
	// LoopEdgeEnter transfers from the code before the loop to its condition.
	LoopEdgeEnter LoopEdge = "enter"
	// LoopEdgeCondition is the condition test that selects body or exit.
	LoopEdgeCondition LoopEdge = "condition"
	// LoopEdgeBack is the normal end of the body returning to the condition.
	LoopEdgeBack LoopEdge = "back"
	// LoopEdgeContinue is a source `continue` returning to the condition.
	LoopEdgeContinue LoopEdge = "continue"
	// LoopEdgeBreak is a source `break` leaving the loop.
	LoopEdgeBreak LoopEdge = "break"
)

// LoopRecord makes the regions of one loop explicit: its condition block
// (which is also the continue target), body entry, and exit (the break
// target). Exit is zero when Sema proved the loop non-continuing. Loop-carried
// state lives in mutable local storage, so no loop block takes parameters.
// Cleanup edges are empty while only trivially destructible storage is
// admitted.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 66(1)–(2) "Loop CFG", § 67(4)
//   - rules/control-flow/flowcontrol_while.md — §§ 19–20, § 29 "CFG requirements"
type LoopRecord struct {
	ID             LoopID
	Kind           LoopKind
	ConditionBlock BlockID
	BodyBlock      BlockID
	ExitBlock      BlockID
	ConditionKnown bool
	ConditionValue bool
	ContinuesAfter bool
	Location       Location
}

type MatchRecord struct {
	ID           MatchID
	Subject      ValueID
	SubjectType  TypeID
	ResultType   TypeID
	ValueContext bool
	Exhaustive   bool
	Arms         []MatchArmRecord
	MergeBlock   BlockID
	Location     Location
}

type MatchArmRecord struct {
	SourceIndex  int
	PatternKind  string
	PatternBlock BlockID
	GuardBlock   BlockID
	BodyBlock    BlockID
	VariantIndex UnionVariantIndex
	EnumValue    *big.Int
	Guarded      bool
	Flow         string
	Location     Location
}

type Parameter struct {
	Name     string
	Value    Value
	Location Location
	// Reference is the callee-side safe-reference facts of a direct ref or
	// slice parameter (package15.md §59; package16.md §§31,44,64).
	// It is nil for other parameters.
	Reference *ReferenceMetadata
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
	// AddressTaken marks reference-dependent storage that a P15 Place root
	// addresses; P5 trivial-core lowering must not erase it before the P15
	// representation boundary is discharged.
	AddressTaken bool
}

type OpKind string

const (
	OpConstInt       OpKind = "const.int"
	OpConstBool      OpKind = "const.bool"
	OpConstString    OpKind = "const.string"
	OpConstDecimal   OpKind = "const.decimal"
	OpConstFloat     OpKind = "const.float"
	OpReturn         OpKind = "return"
	OpStorageDeclare OpKind = "storage.declare"
	OpStorageInit    OpKind = "storage.init"
	OpStorageLoad    OpKind = "storage.load"
	OpStorageStore   OpKind = "storage.store"
	OpDirectCall     OpKind = "call.direct"
	OpForeignCall    OpKind = "call.foreign"
	OpBranch         OpKind = "branch"
	OpCondBranch     OpKind = "conditional-branch"
	OpUnreachable    OpKind = "unreachable"
	OpIntUnaryPlus   OpKind = "int.unary-plus"
	OpIntNegChecked  OpKind = "int.neg-checked"
	OpIntBitNot      OpKind = "int.bit-not"
	// OpNumericConvertExact is P20's NumericConvertExactOp, admitted early for
	// built-in integer conversions Sema proved exact (MD-104).
	OpNumericConvertExact OpKind = "numeric.convert-exact"
	OpIntBinaryChecked    OpKind = "int.binary-checked"
	OpIntBitwise          OpKind = "int.bitwise"
	// OpIntBitExtract is `bit[N](value)`: the unsigned value of the N
	// (Operation.Integer) low bits of an integer, byte or char operand.
	OpIntBitExtract                   OpKind = "int.bit-extract"
	OpIntShiftChecked                 OpKind = "int.shift-checked"
	OpIntCompare                      OpKind = "int.compare"
	OpArithmeticFailure               OpKind = "fail.arithmetic"
	OpArithmeticFailureReasonConstant OpKind = "arithmetic-failure-reason.constant"
	OpArithmeticErrorFromReason       OpKind = "arithmetic-error.from-reason"
	OpResultOk                        OpKind = "result.ok"
	OpResultErr                       OpKind = "result.err"
	OpResultIsErr                     OpKind = "result.is-err"
	OpResultUnwrapOk                  OpKind = "result.unwrap-ok"
	OpResultUnwrapErr                 OpKind = "result.unwrap-err"
	OpCoreErrorIsVariant              OpKind = "core-error.is-variant"
	OpEnumConstant                    OpKind = "enum.constant"
	OpEnumFromInteger                 OpKind = "enum.from-integer"
	OpEnumToInteger                   OpKind = "enum.to-integer"
	OpEnumCompare                     OpKind = "enum.compare"
	OpUnionConstruct                  OpKind = "union.construct"
	OpUnionIsVariant                  OpKind = "union.is-variant"
	OpUnionUnwrapPayload              OpKind = "union.unwrap-payload"
	OpUnionUnwrapField                OpKind = "union.unwrap-field"
	OpStructConstruct                 OpKind = "struct.construct"
	OpStructSpreadFields              OpKind = "struct.spread-fields"
	OpStructExtractField              OpKind = "struct.extract-field"
	OpStructReplaceField              OpKind = "struct.replace-field"
	OpRegisterValueExtract            OpKind = "register.value.extract"
	OpRegisterValueUpdate             OpKind = "register.value.update"
	OpArrayConstruct                  OpKind = "array.construct"
	OpArrayDefault                    OpKind = "array.default"
	OpArrayLength                     OpKind = "array.len"
	OpArrayIndexInBounds              OpKind = "array.index-in-bounds"
	OpArrayExtract                    OpKind = "array.extract"
	OpArrayReplace                    OpKind = "array.replace"
	OpBoundsFailure                   OpKind = "fail.bounds"
	OpBoolNot                         OpKind = "bool.not"
	// OpPanic is the explicit, non-returning panic of an explicit panic
	// statement, a failed assertion, or a reached checked unreachable. It
	// carries the registered panic reason, the static message when the
	// source wrote one, and its source location as provenance
	// (rules/compiler/semantic_ir.md §§ 55–57).
	OpPanic OpKind = "panic"
	// OpDiscardValue is the explicit terminal ownership action of a
	// validated discard: an explicit `discard` of an expression or binding,
	// or the implicit discard of an ordinary call result used as a statement
	// (rules/control-flow/discard.md §§ 3, 5, 9–10, 36).
	OpDiscardValue OpKind = "discard.value"
)

// DiscardProvenance distinguishes the source of a discard
// (discard.md § 36).
type DiscardProvenance string

const (
	DiscardExplicit           DiscardProvenance = "explicit"
	DiscardImplicitCallResult DiscardProvenance = "implicit-call-result"
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

type ArithmeticFailureReason string

const (
	ArithmeticFailureNone           ArithmeticFailureReason = "none"
	ArithmeticFailureReasonOverflow ArithmeticFailureReason = "overflow"
	ArithmeticFailureDivisionByZero ArithmeticFailureReason = "division-by-zero"
	ArithmeticFailureInvalidShift   ArithmeticFailureReason = "invalid-shift"
)

type ArgumentAction string

const (
	ArgumentCopyTrivial ArgumentAction = "copy-trivial"
	// ArgumentSharedReference and ArgumentMutableReference pass a reference
	// value produced explicitly at the call site to a ref or ref mut
	// parameter (package 15 §§ 59-60).
	ArgumentSharedReference  ArgumentAction = "shared-reference"
	ArgumentMutableReference ArgumentAction = "mutable-reference"
	// ArgumentMove transfers an owned value needing destruction into an
	// owned by-value parameter: a moved owned local or a fresh temporary
	// (package17 § 67).
	ArgumentMove ArgumentAction = "move"
)

type UnionPayloadAction string

const UnionPayloadCopyTrivial UnionPayloadAction = "copy-trivial"

type StructFieldOrigin string

const (
	StructOriginExplicit StructFieldOrigin = "explicit"
	StructOriginSpread   StructFieldOrigin = "spread"
	StructOriginDefault  StructFieldOrigin = "default"
)

type StructFieldAction string

const (
	StructActionConstructDirect        StructFieldAction = "construct-direct"
	StructActionCopyTrivial            StructFieldAction = "copy-trivial"
	StructActionCopySemanticInfallible StructFieldAction = "copy-semantic-infallible"
	StructActionMove                   StructFieldAction = "move"
)

type ArrayConstructSegmentKind string

const (
	ArraySegmentElement ArrayConstructSegmentKind = "element"
	ArraySegmentSpread  ArrayConstructSegmentKind = "spread"
	// ArraySegmentRange contributes its segment length of consecutive scalar
	// values starting at its operand, the constant lower bound
	// (rules/collections/collections.md § 5.6a). Sema guarantees the values
	// fit the element type and, for rune, avoid surrogate code points.
	ArraySegmentRange ArrayConstructSegmentKind = "range"
)

type ArrayTransferAction string

const (
	ArrayActionConstructDirect        ArrayTransferAction = "construct-direct"
	ArrayActionCopyTrivial            ArrayTransferAction = "copy-trivial"
	ArrayActionCopySemanticInfallible ArrayTransferAction = "copy-semantic-infallible"
	ArrayActionMove                   ArrayTransferAction = "move"
	ArrayActionBorrowShared           ArrayTransferAction = "borrow-shared"
	ArrayActionBorrowMutable          ArrayTransferAction = "borrow-mutable"
)

// ArrayIndexCheckKind and ArrayIndexProofKind preserve the compiler-owned
// safety decision required by SEC-MLIR Package 14 sections 31-41. Semantic IR
// consumers must not reconstruct range proofs from source syntax or CFG shape.
type ArrayIndexCheckKind string

const (
	ArrayIndexProvenSafe   ArrayIndexCheckKind = "proven-safe"
	ArrayIndexRuntimeCheck ArrayIndexCheckKind = "runtime-check"
)

type ArrayIndexProofKind string

const (
	ArrayIndexProofConstant ArrayIndexProofKind = "constant"
	ArrayIndexProofRange    ArrayIndexProofKind = "range"
	ArrayIndexProofBranch   ArrayIndexProofKind = "branch"
	ArrayIndexProofContract ArrayIndexProofKind = "contract"
	ArrayIndexProofAnalysis ArrayIndexProofKind = "analysis"
	ArrayIndexProofGuarded  ArrayIndexProofKind = "guarded"
)

type TryHandlerKind string

const (
	TryHandlerOK          TryHandlerKind = "ok"
	TryHandlerErrVariant  TryHandlerKind = "err-variant"
	TryHandlerErrCatchAll TryHandlerKind = "err-catch-all"
	TryHandlerMerge       TryHandlerKind = "merge"
	// TryHandlerResidual marks the return that propagates failures a
	// partial handler set leaves unmatched (rules/errors/errorhandling.md §16).
	TryHandlerResidual TryHandlerKind = "residual"
	// TryHandlerNone marks a None handler of a try over Option[T]; absence is
	// the only alternate state, so it is not an Err handler and keeps the
	// Option carrier kind distinct (rules/errors/errorhandling.md §§10.2, 35).
	TryHandlerNone TryHandlerKind = "none"
)

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
	Kind        OpKind
	Location    Location
	Results     []Value
	Operands    []ValueID
	Integer     *big.Int
	Bool        *bool
	String      string
	Decimal     *DecimalConstant
	FloatLexeme string
	// Float is const.float's canonical binary payload (package20.md § 18).
	Float *FloatConstant
	// FloatBinary is the kind of float.binary.
	FloatBinary FloatBinaryKind
	// FloatCore is the kind of float.classify, float.round or float.property
	// (package20.md §§ 66–68).
	FloatCore string
	Storage   StorageID
	Static    StaticID
	Callee    FunctionID
	// ConversionProof is required on numeric.convert-exact and
	// enum.to-integer (MD-104).
	ConversionProof *ConversionProof
	// ConversionFailure is the ConversionError variant of a
	// numeric.convert-checked failure edge (package20.md § 72).
	ConversionFailure string
	// SliceRange retains original endpoint types and range proof provenance.
	// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§35,78–79.
	SliceRange *SliceRangeMetadata
	// Slice retains resolved value provenance without selecting a descriptor.
	// Rules: rules/compiler/semantic_ir.md — §126.2–4,26.
	Slice *SliceFacts
	// Ownership carries the resolved facts of an own.* operation (P17
	// §§ 48–58; ownership_ops.go).
	Ownership *OwnershipFacts
	// Cleanup carries the identities of a cleanup.* operation, and Region the
	// body of cleanup.defer-register (P17-33; cleanup_ops.go).
	Cleanup *CleanupFacts
	Region  *Region
	// UnsafeAuthorization is the source position of a call's direct
	// `unsafe` prefix (MD-079); nil when the call carries no prefix.
	UnsafeAuthorization *Location
	ArgumentActions     []ArgumentAction
	Successors          []BranchTarget
	IntegerBinary       IntegerCheckedBinaryKind
	IntegerBitwise      IntegerBitwiseKind
	IntegerShift        IntegerShiftKind
	IntegerCompare      IntegerComparePredicate
	FailureCategory     ArithmeticFailureCategory
	FailureReason       ArithmeticFailureReason
	Variant             string
	EnumCase            EnumCaseID
	UnionVariant        UnionVariantIndex
	UnionField          string
	UnionFields         []string
	PayloadActions      []UnionPayloadAction
	StructField         StructFieldID
	StructOrigins       []StructFieldOrigin
	StructActions       []StructFieldAction
	// RegisterField and RegisterWrite carry a register.value.* field step
	// and, for an update, the Sema proof that the value fits (MD-077 § 10(1)).
	RegisterField RegisterFieldID
	RegisterWrite RegisterWriteProof
	// RegisterConversion carries a register.value.fits-bits / from-bits /
	// to-bits destination, proof and interpretation (MD-110 § 7).
	RegisterConversion  *RegisterConversionFacts
	ArrayElementType    TypeID
	ArrayLength         string
	ArraySegmentKinds   []ArrayConstructSegmentKind
	ArraySegmentLengths []string
	ArrayActions        []ArrayTransferAction
	ArrayIndexSigned    bool
	ArrayCheckKind      ArrayIndexCheckKind
	ArrayProofKind      ArrayIndexProofKind
	ArrayGuard          ValueID
	// VariantGuard names the union.is-variant predicate whose true edge
	// proves the active variant of a place.union-payload (P15-45).
	VariantGuard   ValueID
	ArrayOperation string
	// AtomicOrder is the established MemoryOrder of an atomic.fence.
	AtomicOrder          string
	TryHandlerKind       TryHandlerKind
	TryHandlerIndex      int
	TryHandlerExhaustive bool
	// TryHandlerGuarded marks a handler edge selected only after its where
	// guard held; it never covers its pattern (errorhandling.md §19).
	TryHandlerGuarded bool
	// TryResidualPropagates marks handler edges of a partial plan whose
	// unmatched failures leave through a TryHandlerResidual return.
	TryResidualPropagates bool
	Operator              string
	Synthesized           bool
	Reason                string
	MatchID               MatchID
	MatchArmIndex         int
	MatchStage            string
	MatchPatternKind      string
	LoopID                LoopID
	LoopEdge              LoopEdge
	// PanicReasonID and PanicReason identify the registered panic reason of
	// an OpPanic; PanicMessage is its static literal message when
	// PanicHasMessage is set.
	// DiscardProvenance and DiscardTrivial describe an OpDiscardValue: its
	// source and whether the consumed value is trivially destructible.
	DiscardProvenance DiscardProvenance
	DiscardTrivial    bool
	PanicReasonID     uint16
	PanicReason       string
	PanicMessage      string
	PanicHasMessage   bool
	// AssertCheck marks the conditional branch of an unproven assertion.
	AssertCheck bool
	// Place and Reference carry the P15 Place/direct-reference facts;
	// ReferenceGuard is the ref.is-valid result guarding a dynamic-epoch use
	// (place_reference.go).
	Place          *PlaceMetadata
	Reference      *ReferenceMetadata
	ReferenceGuard ValueID
}

func (o Operation) IsTerminator() bool {
	return o.Kind == OpReturn || o.Kind == OpBranch || o.Kind == OpCondBranch || o.Kind == OpArithmeticFailure || o.Kind == OpBoundsFailure || o.Kind == OpUnreachable || o.Kind == OpPanic || o.Kind == OpReferenceGenerationFailure ||
		o.Kind == OpCleanupDeferYield
}

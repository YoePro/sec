package ast

import (
	"fmt"
	"math/big"
	"sec/internal/lexer"
	"strconv"
	"strings"
)

// Node is the base interface for all AST nodes.
type Node interface {
	TokenLiteral() string
}

// Statement represents a top-level or block-level instruction.
type Statement interface {
	Node
	statementNode()
}

// RecoveryInfo preserves the malformed source region represented by an
// invalid or incomplete AST node. Recovery metadata is syntax-only and must
// never be interpreted as valid program semantics.
type RecoveryInfo struct {
	DiagnosticID string
	Message      string
	Start        lexer.Token
	End          lexer.Token
	Skipped      int
}

type InvalidStatement struct {
	Token    lexer.Token
	Message  string
	Recovery *RecoveryInfo
}

// InvalidDeclaration preserves a declaration-shaped source region that could
// not be completed. Keeping it distinct from InvalidStatement lets editor
// consumers retain the user's declaration intent without treating it as
// semantically valid.
type InvalidDeclaration struct {
	Token    lexer.Token
	Message  string
	Recovery *RecoveryInfo
}

func (id *InvalidDeclaration) statementNode() {}

func (id *InvalidDeclaration) TokenLiteral() string { return id.Token.Lexeme }

func (is *InvalidStatement) statementNode() {}

func (is *InvalidStatement) implMemberNode() {}

func (is *InvalidStatement) TokenLiteral() string {
	return is.Token.Lexeme
}

type DiscardStatement struct {
	Token lexer.Token
	Value Expression
	Name  *Identifier
}

func (ds *DiscardStatement) statementNode() {}

func (ds *DiscardStatement) TokenLiteral() string {
	return ds.Token.Lexeme
}

// AssertStatement represents an always-active invariant check with optional
// static diagnostic metadata.
//
// Rules:
//   - rules/errors/panic.md — § 15.1 "Canonical syntax"
//   - rules/errors/panic.md — § 15.4 "Messages"
type AssertStatement struct {
	Token     lexer.Token
	Condition Expression
	Message   *StringLiteral
}

func (as *AssertStatement) statementNode() {}

func (as *AssertStatement) TokenLiteral() string {
	return as.Token.Lexeme
}

type DetachStatement struct {
	Token         lexer.Token
	Value         Expression
	DiscardResult bool
}

func (ds *DetachStatement) statementNode() {}

func (ds *DetachStatement) TokenLiteral() string {
	return ds.Token.Lexeme
}

type CancelStatement struct {
	Token lexer.Token
}

func (cs *CancelStatement) statementNode() {}

func (cs *CancelStatement) TokenLiteral() string {
	return cs.Token.Lexeme
}

// Expression represents a value-producing AST node.
type Expression interface {
	Node
	expressionNode()
	String() string
}

type InvalidExpression struct {
	Token    lexer.Token
	Message  string
	Recovery *RecoveryInfo
}

// InvalidPattern is an expression-shaped placeholder used only in grammar
// positions that require a pattern, such as match arms and try handlers.
type InvalidPattern struct {
	Token    lexer.Token
	Message  string
	Recovery *RecoveryInfo
}

func (ip *InvalidPattern) expressionNode() {}

func (ip *InvalidPattern) TokenLiteral() string { return ip.Token.Lexeme }

func (ip *InvalidPattern) String() string { return "<invalid-pattern>" }

func (ie *InvalidExpression) expressionNode() {}

func (ie *InvalidExpression) TokenLiteral() string {
	return ie.Token.Lexeme
}

func (ie *InvalidExpression) String() string {
	return "<invalid-expression>"
}

// Program is the root node for a parsed Sec source file.
type Program struct {
	Statements       []Statement
	SourceProvenance map[string]SourceProvenance
}

// SourceProvenance is immutable trust metadata assigned by a source loader.
type SourceProvenance string

const (
	SourceUser     SourceProvenance = "user"
	SourceCore     SourceProvenance = "core"
	SourceStdlib   SourceProvenance = "stdlib"
	SourcePlatform SourceProvenance = "platform"
)

func (p *Program) TokenLiteral() string {
	if len(p.Statements) == 0 {
		return ""
	}

	return p.Statements[0].TokenLiteral()
}

// --------------------------------------------------------------------
// Type declarations
// --------------------------------------------------------------------

type TargetDirective struct {
	Token lexer.Token
	OS    string
	Arch  string
}

func (td *TargetDirective) statementNode() {}

func (td *TargetDirective) TokenLiteral() string {
	return td.Token.Lexeme
}

type Attribute struct {
	Token     lexer.Token
	Name      *Identifier
	Arguments []*AttributeArgument
}

func (a *Attribute) TokenLiteral() string {
	return a.Token.Lexeme
}

type AttributeArgument struct {
	Token lexer.Token
	Name  *Identifier
	Value Expression
}

func (a *AttributeArgument) TokenLiteral() string {
	return a.Token.Lexeme
}

// TypeDeclStatement represents:
//
//	type Percent int range 0..100
//	type Meter decimal<m>
//	type Email string
//	type IOError = FileNotFound AccessDenied InvalidValue
type TypeDeclStatement struct {
	Token lexer.Token // TYPE token

	Attributes        []*Attribute
	Name              *Identifier
	GenericParameters []*GenericParameter
	BaseType          *TypeReference
	AssignedType      *TypeReference
	Variants          []*Identifier
	StructType        *StructType
	RegisterType      *RegisterType
	Union             bool
	// ErrorType preserves the canonical `type Name union error` marker from
	// rules/declarations/unions.md and rules/errors/errorhandling.md.
	ErrorType     bool
	ErrorToken    lexer.Token
	UnionVariants []*UnionVariant
	Implements    []*TypeReference
	Contract      Contract
	DefaultToken  lexer.Token
	Default       Expression
}

func (tds *TypeDeclStatement) statementNode() {}

func (tds *TypeDeclStatement) implMemberNode() {}

func (tds *TypeDeclStatement) TokenLiteral() string {
	return tds.Token.Lexeme
}

type UnitDeclStatement struct {
	Token    lexer.Token
	Name     *Identifier
	BaseType *TypeReference
	Category string
}

func (uds *UnitDeclStatement) statementNode() {}

func (uds *UnitDeclStatement) implMemberNode() {}

func (uds *UnitDeclStatement) TokenLiteral() string {
	return uds.Token.Lexeme
}

type GenericParameter struct {
	Token      lexer.Token
	Name       *Identifier
	Constraint *TypeReference
}

type UnionVariant struct {
	Token         lexer.Token
	Name          *Identifier
	Payload       *TypeReference
	PayloadFields []*StructField
	Default       bool
	DefaultToken  lexer.Token
}

func (gp *GenericParameter) TokenLiteral() string {
	return gp.Token.Lexeme
}

type EnumDeclaration struct {
	Token              lexer.Token
	Attributes         []*Attribute
	Name               *Identifier
	GenericParameters  []*GenericParameter
	UnderlyingType     *TypeReference
	BitUnderlying      bool
	UnderlyingBitWidth int64
	// ErrorType preserves the canonical enum Name error marker from
	// rules/declarations/enums.md and rules/errors/errorhandling.md.
	ErrorType  bool
	ErrorToken lexer.Token
	Values     []*EnumValue
}

func (ed *EnumDeclaration) statementNode() {}

func (ed *EnumDeclaration) implMemberNode() {}

func (ed *EnumDeclaration) TokenLiteral() string {
	return ed.Token.Lexeme
}

type EnumValue struct {
	Token        lexer.Token
	Name         *Identifier
	Default      bool
	DefaultToken lexer.Token
	Initializer  Expression
}

func (ev *EnumValue) TokenLiteral() string {
	return ev.Token.Lexeme
}

type InterfaceDeclaration struct {
	Token             lexer.Token
	Name              *Identifier
	GenericParameters []*GenericParameter
	Implements        []*TypeReference
	Methods           []*FunctionDeclaration
	Properties        []*InterfaceProperty
	Events            []*InterfaceEvent
}

func (id *InterfaceDeclaration) statementNode() {}

func (id *InterfaceDeclaration) TokenLiteral() string {
	return id.Token.Lexeme
}

type InterfaceProperty struct {
	Token           lexer.Token
	Name            *Identifier
	Type            *TypeReference
	Static          bool
	RequiresGet     bool
	RequiresSet     bool
	SetterParameter *Identifier
	SetterFallible  bool
	SetToken        lexer.Token
}

func (ip *InterfaceProperty) TokenLiteral() string {
	return ip.Token.Lexeme
}

type InterfaceEvent struct {
	Token   lexer.Token
	Name    *Identifier
	Payload *TypeReference
}

func (ie *InterfaceEvent) TokenLiteral() string {
	return ie.Token.Lexeme
}

// Identifier represents a named symbol.
type Identifier struct {
	Token lexer.Token
	Value string
}

func (i *Identifier) expressionNode() {}

func (i *Identifier) TokenLiteral() string {
	return i.Token.Lexeme
}

func (i *Identifier) String() string {
	return i.Value
}

// TypeReference represents a type usage:
//
//	int
//	decimal<m>
//	Vec[T]
//	byte[]
//	ref byte[]
type TypeReference struct {
	Token lexer.Token
	// Invalid marks a recovered type reference that must resolve to ErrorType
	// without producing dependent semantic diagnostics.
	Invalid  bool
	Recovery *RecoveryInfo

	Name string

	Ref        bool
	MutableRef bool

	// ElementType is used for slice and array types such as byte[] and int[3].
	ElementType *TypeReference
	Slice       bool
	// ArrayLength is a parser compatibility cache only. The source expression
	// below is authoritative until Sema creates the exact Package 14 shape.
	ArrayLength           int64
	ArrayLengthExpression Expression

	// Unit is used for unit types such as decimal<m> or decimal<SEK>.
	Unit string
	// UnitExpression is the first-class structural syntax tree required by
	// rules/types/units.md, "Structural unit expressions". Unit retains the
	// exact compact source spelling for compatibility and diagnostics; semantic
	// consumers use this tree rather than reparsing that string.
	UnitExpression *UnitExpression
	// UnitOnly is used for shorthand unit types such as <m>. Semantic analysis
	// expands it to the unit's default numeric representation.
	UnitOnly bool

	// TypeArgs is used for generic types such as Vec[T], Map[K,V], Result[T,E].
	TypeArgs []*TypeReference

	// ConstArgs is used for compiler-known mixed type/value constructors such
	// as list[T, 32], vector[T, 3], Shape[2], and tensor[T, 3, 224, 224].
	ConstArgs []Expression

	// EventCapacity is the optional fixed capacity in Event[T, N] and
	// EventStorage[T, N].
	EventCapacity    int64
	EventCapacitySet bool

	// FunctionParameterTypes and FunctionReturnType are used for function
	// value types such as fn(int, string) bool. FunctionCapability preserves
	// the callable authority prefix from rules/declarations/lambda-functions.md.
	FunctionCapability     CallableCapability
	FunctionParameterTypes []*TypeReference
	FunctionReturnType     *TypeReference
}

// UnitExpressionKind classifies the source structure of a unit annotation.
// rules/types/units.md distinguishes named identity, structural algebra,
// grouping, exponents, and the dimensionless identity without rewriting the
// programmer's factor order.
type UnitExpressionKind string

const (
	UnitExpressionName     UnitExpressionKind = "name"
	UnitExpressionIdentity UnitExpressionKind = "identity"
	UnitExpressionMultiply UnitExpressionKind = "multiply"
	UnitExpressionDivide   UnitExpressionKind = "divide"
	UnitExpressionPower    UnitExpressionKind = "power"
	UnitExpressionGroup    UnitExpressionKind = "group"
)

type UnitExpression struct {
	Token    lexer.Token
	Kind     UnitExpressionKind
	Name     string
	Left     *UnitExpression
	Right    *UnitExpression
	Exponent int
	Source   string
}

func (u *UnitExpression) String() string {
	if u == nil {
		return ""
	}
	if u.Source != "" {
		return u.Source
	}
	switch u.Kind {
	case UnitExpressionName:
		return u.Name
	case UnitExpressionIdentity:
		return "1"
	case UnitExpressionMultiply:
		return u.Left.String() + "*" + u.Right.String()
	case UnitExpressionDivide:
		return u.Left.String() + "/" + u.Right.String()
	case UnitExpressionPower:
		return fmt.Sprintf("%s^%d", u.Left.String(), u.Exponent)
	case UnitExpressionGroup:
		return "(" + u.Left.String() + ")"
	default:
		return ""
	}
}

// CallableCapability is the source-level authority required to invoke a
// callable value under rules/declarations/lambda-functions.md. It is separate
// from parameter ownership and method receiver capability.
type CallableCapability string

const (
	CallableShared    CallableCapability = "shared"
	CallableMutable   CallableCapability = "mutable"
	CallableConsuming CallableCapability = "consuming"
)

func (tr *TypeReference) TokenLiteral() string {
	return tr.Token.Lexeme
}

// --------------------------------------------------------------------
// Contracts
// --------------------------------------------------------------------

type Contract interface {
	Node
	contractNode()
}

type ContractList struct {
	Token     lexer.Token
	Contracts []Contract
}

func (cl *ContractList) contractNode() {}

func (cl *ContractList) TokenLiteral() string {
	return cl.Token.Lexeme
}

// RangeContract represents:
//
//	range 0..100
//	range 1..65535
type RangeContract struct {
	Token lexer.Token // "range" identifier token for now

	Min       Expression
	Max       Expression
	Exclusive bool
}

func (rc *RangeContract) contractNode() {}

func (rc *RangeContract) TokenLiteral() string {
	return rc.Token.Lexeme
}

type MembershipContract struct {
	Token  lexer.Token
	Values []Expression
}

func (mc *MembershipContract) contractNode() {}

func (mc *MembershipContract) TokenLiteral() string {
	return mc.Token.Lexeme
}

type MarkerContract struct {
	Token lexer.Token
	Name  string
	Value Expression
}

func (mc *MarkerContract) contractNode() {}

func (mc *MarkerContract) TokenLiteral() string {
	return mc.Token.Lexeme
}

// --------------------------------------------------------------------
// Literals
// --------------------------------------------------------------------

type IntegerLiteral struct {
	Token    lexer.Token
	Value    int64
	BigValue *big.Int
}

func (il *IntegerLiteral) expressionNode() {}

func (il *IntegerLiteral) TokenLiteral() string {
	return il.Token.Lexeme
}

func (il *IntegerLiteral) String() string {
	return il.Token.Lexeme
}

func (il *IntegerLiteral) Suffix() string {
	_, suffix := SplitNumericLiteralSuffix(il.Token.Lexeme)
	return suffix
}

type FloatLiteral struct {
	Token lexer.Token
	Value float64
}

func (fl *FloatLiteral) expressionNode() {}

func (fl *FloatLiteral) TokenLiteral() string {
	return fl.Token.Lexeme
}

func (fl *FloatLiteral) String() string {
	return fl.Token.Lexeme
}

func (fl *FloatLiteral) Suffix() string {
	_, suffix := SplitNumericLiteralSuffix(fl.Token.Lexeme)
	return suffix
}

func SplitNumericLiteralSuffix(lexeme string) (string, string) {
	if lexeme == "" {
		return lexeme, ""
	}
	last := lexeme[len(lexeme)-1]
	switch last {
	case 'i', 'u', 'g', 'm', 't', 'r':
		return lexeme[:len(lexeme)-1], string(last)
	default:
		return lexeme, ""
	}
}

func NormalizeNumericLiteralLexeme(lexeme string) string {
	return strings.ReplaceAll(lexeme, "_", "")
}

func ParseIntegerLiteralLexeme(lexeme string) (*big.Int, bool) {
	_, suffix := SplitNumericLiteralSuffix(lexeme)
	if suffix == "g" || suffix == "m" {
		return nil, false
	}
	return ParseIntegerFormNumericLiteralLexeme(lexeme)
}

// ParseIntegerFormNumericLiteralLexeme preserves the exact integer value even
// when a family suffix shapes it as float or decimal later in the frontend.
func ParseIntegerFormNumericLiteralLexeme(lexeme string) (*big.Int, bool) {
	digits, _ := SplitNumericLiteralSuffix(lexeme)
	if digits == "" || strings.ContainsAny(digits, ".") {
		return nil, false
	}

	base := 10
	if len(digits) > 2 && digits[0] == '0' {
		switch digits[1] {
		case 'b', 'B':
			base = 2
			digits = digits[2:]
		case 'o', 'O':
			base = 8
			digits = digits[2:]
		case 'x', 'X':
			base = 16
			digits = digits[2:]
		}
	}
	if digits == "" {
		return nil, false
	}
	digits = NormalizeNumericLiteralLexeme(digits)

	value, ok := new(big.Int).SetString(digits, base)
	return value, ok
}

func ParseIntegerLiteralInt64(lexeme string) (int64, bool) {
	value, ok := ParseIntegerLiteralLexeme(lexeme)
	if !ok || !value.IsInt64() {
		return 0, false
	}
	return value.Int64(), true
}

func ParseFloatLiteralFloat64(lexeme string) (float64, bool) {
	digits, suffix := SplitNumericLiteralSuffix(lexeme)
	if suffix == "i" || suffix == "u" || suffix == "t" || suffix == "r" || digits == "" {
		return 0, false
	}
	digits = NormalizeNumericLiteralLexeme(digits)
	if len(digits) > 2 && digits[0] == '0' {
		switch digits[1] {
		case 'b', 'B', 'o', 'O', 'x', 'X':
			integer, ok := ParseIntegerFormNumericLiteralLexeme(digits)
			if !ok {
				return 0, false
			}
			value, _ := new(big.Float).SetInt(integer).Float64()
			return value, true
		}
	}
	value, err := strconv.ParseFloat(digits, 64)
	return value, err == nil
}

type StringLiteral struct {
	Token lexer.Token
	Value string
}

func (sl *StringLiteral) expressionNode() {}

func (sl *StringLiteral) TokenLiteral() string {
	return sl.Token.Lexeme
}

func (sl *StringLiteral) String() string {
	return sl.Token.Lexeme
}

type CharLiteral struct {
	Token lexer.Token
	Value string
}

func (cl *CharLiteral) expressionNode() {}

func (cl *CharLiteral) TokenLiteral() string {
	return cl.Token.Lexeme
}

func (cl *CharLiteral) String() string {
	return cl.Token.Lexeme
}

type ModuleStatement struct {
	Token lexer.Token
	Path  string
}

func (ms *ModuleStatement) statementNode() {}

func (ms *ModuleStatement) TokenLiteral() string {
	return ms.Token.Lexeme
}

type ImportStatement struct {
	Token lexer.Token
	Alias string
	Path  string
}

func (is *ImportStatement) statementNode() {}

func (is *ImportStatement) TokenLiteral() string {
	return is.Token.Lexeme
}

type CommentStatement struct {
	Token lexer.Token
	Text  string
}

func (cs *CommentStatement) statementNode() {}

func (cs *CommentStatement) TokenLiteral() string {
	return cs.Token.Lexeme
}

func (p *Program) String() string {
	var out strings.Builder

	for _, stmt := range p.Statements {
		out.WriteString(stmt.TokenLiteral())
		out.WriteString("\n")
	}

	return out.String()
}

type OwnershipMode string

const (
	OwnershipCopy OwnershipMode = "copy"
	OwnershipMove OwnershipMode = "move"
)

type LetStatement struct {
	Token     lexer.Token
	Static    bool
	Mutable   bool
	Ownership OwnershipMode
	Name      *Identifier
	Type      *TypeReference
	Contract  Contract
	Value     Expression
	// SynthesizedDefault records that Sema supplied Value for a declaration
	// which had no source initializer. Lowering stages must not mistake it for
	// an explicit initialization when their package boundary forbids defaults.
	SynthesizedDefault bool
	Address            Expression
	AddressToken       lexer.Token
}

func (ls *LetStatement) statementNode() {}

func (ls *LetStatement) implMemberNode() {}

func (ls *LetStatement) TokenLiteral() string {
	return ls.Token.Lexeme
}

type LetGroupStatement struct {
	Token lexer.Token
	Lets  []*LetStatement
}

func (lgs *LetGroupStatement) statementNode() {}

func (lgs *LetGroupStatement) TokenLiteral() string {
	return lgs.Token.Lexeme
}

type AssignmentStatement struct {
	Token     lexer.Token
	Target    Expression
	Operator  string
	Ownership OwnershipMode
	Value     Expression
}

func (as *AssignmentStatement) statementNode() {}

func (as *AssignmentStatement) TokenLiteral() string {
	return as.Token.Lexeme
}

type TryAssignmentStatement struct {
	Token      lexer.Token
	Assignment *AssignmentStatement
	Handlers   []*TryHandler
}

func (tas *TryAssignmentStatement) statementNode() {}

func (tas *TryAssignmentStatement) TokenLiteral() string {
	return tas.Token.Lexeme
}

type DeferStatement struct {
	Token lexer.Token
	Body  *BlockStatement
}

func (ds *DeferStatement) statementNode() {}

func (ds *DeferStatement) TokenLiteral() string {
	return ds.Token.Lexeme
}

type ExpressionStatement struct {
	Token      lexer.Token
	Expression Expression
}

func (es *ExpressionStatement) statementNode() {}

func (es *ExpressionStatement) TokenLiteral() string {
	return es.Token.Lexeme
}

type FunctionDeclaration struct {
	Token              lexer.Token
	Attributes         []*Attribute
	Name               *Identifier
	GenericParameters  []*GenericParameter
	Parameters         []*Parameter
	ReturnType         *TypeReference
	Body               *BlockStatement
	Unsafe             bool
	Extern             bool
	ABI                string
	LinkName           string
	Static             bool
	ReceiverCapability ReceiverCapability
}

type ReceiverCapability string

const (
	ReceiverShared    ReceiverCapability = "shared"
	ReceiverMutable   ReceiverCapability = "mutable"
	ReceiverConsuming ReceiverCapability = "consuming"
)

func (fd *FunctionDeclaration) statementNode() {}

func (fd *FunctionDeclaration) implMemberNode() {}

func (fd *FunctionDeclaration) TokenLiteral() string {
	return fd.Token.Lexeme
}

type Parameter struct {
	Token      lexer.Token
	Name       *Identifier
	Type       *TypeReference
	Ref        bool
	MutableRef bool
	Consuming  bool
	// Variadic marks the final native Sec `name: ...T` parameter from
	// rules/declarations/functions.md sections 28 and 41. The element type
	// remains Type; `...` is parameter shape, not a standalone type spelling.
	Variadic bool
}

func (p *Parameter) TokenLiteral() string {
	return p.Token.Lexeme
}

type LambdaExpression struct {
	Token      lexer.Token
	Captures   []LambdaCapture
	Parameters []*Parameter
	ReturnType *TypeReference
	Body       *BlockStatement
}

func (le *LambdaExpression) expressionNode() {}

func (le *LambdaExpression) TokenLiteral() string {
	return le.Token.Lexeme
}

func (le *LambdaExpression) String() string {
	out := ""
	if len(le.Captures) > 0 {
		out += "capture("
		for i, capture := range le.Captures {
			if i > 0 {
				out += ", "
			}
			if capture.Name != nil {
				out += capture.Name.Value
			}
		}
		out += ") "
	}

	out += "fn("
	for i, param := range le.Parameters {
		if i > 0 {
			out += ", "
		}
		if param.Name != nil {
			out += param.Name.Value
		}
		out += ": "
		if param.Type != nil {
			out += param.Type.Name
		} else {
			out += "<nil>"
		}
	}
	out += ")"
	if le.ReturnType != nil {
		out += " " + le.ReturnType.Name
	}
	out += " { ... }"
	return out
}

type LambdaCapture struct {
	Name *Identifier
}

type ReturnStatement struct {
	Token lexer.Token
	Value Expression
}

func (rs *ReturnStatement) statementNode() {}

func (rs *ReturnStatement) TokenLiteral() string {
	return rs.Token.Lexeme
}

type IfStatement struct {
	Token       lexer.Token
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement
}

func (is *IfStatement) statementNode() {}

func (is *IfStatement) TokenLiteral() string {
	return is.Token.Lexeme
}

type SwitchStatement struct {
	Token                  lexer.Token
	Subject                Expression
	Cases                  []*SwitchCase
	Default                *SwitchCase
	DefaultNotFinalToken   lexer.Token
	DuplicateDefaultTokens []lexer.Token
}

func (ss *SwitchStatement) statementNode() {}

func (ss *SwitchStatement) TokenLiteral() string {
	return ss.Token.Lexeme
}

type SwitchCase struct {
	Token   lexer.Token
	Default bool
	Items   []SwitchCaseItem
	Body    *BlockStatement
}

type SelectStatement struct {
	Token                   lexer.Token
	Branches                []*SelectBranch
	DefaultNotFinalToken    lexer.Token
	DuplicateDefaultTokens  []lexer.Token
	UnreachableTimeoutToken lexer.Token
}

func (ss *SelectStatement) statementNode() {}

func (ss *SelectStatement) TokenLiteral() string {
	return ss.Token.Lexeme
}

type SelectBranchKind string

const (
	SelectOperationBranch SelectBranchKind = "operation"
	SelectTimeoutBranch   SelectBranchKind = "timeout"
	SelectDefaultBranch   SelectBranchKind = "default"
)

type SelectBranch struct {
	Token   lexer.Token
	Kind    SelectBranchKind
	Binding *Identifier
	Value   Expression
	Body    *BlockStatement
}

type SwitchCaseItem interface {
	Node
	switchCaseItemNode()
}

type SwitchValueCase struct {
	Token lexer.Token
	Value Expression
}

func (svc *SwitchValueCase) switchCaseItemNode() {}

func (svc *SwitchValueCase) TokenLiteral() string {
	return svc.Token.Lexeme
}

type SwitchRangeCase struct {
	Token lexer.Token
	Range *RangeExpression
}

func (src *SwitchRangeCase) switchCaseItemNode() {}

func (src *SwitchRangeCase) TokenLiteral() string {
	return src.Token.Lexeme
}

type SwitchRelationalCase struct {
	Token    lexer.Token
	Operator string
	Value    Expression
}

func (src *SwitchRelationalCase) switchCaseItemNode() {}

func (src *SwitchRelationalCase) TokenLiteral() string {
	return src.Token.Lexeme
}

type FallthroughStatement struct {
	Token lexer.Token
}

func (fs *FallthroughStatement) statementNode() {}

func (fs *FallthroughStatement) TokenLiteral() string {
	return fs.Token.Lexeme
}

type ForStatement struct {
	Token    lexer.Token
	Bindings []ForBinding
	Iterable Expression
	Step     Expression
	Body     *BlockStatement
}

func (fs *ForStatement) statementNode() {}

func (fs *ForStatement) TokenLiteral() string {
	return fs.Token.Lexeme
}

type ForBinding struct {
	Token   lexer.Token
	Name    string
	Discard bool
}

type WhileStatement struct {
	Token     lexer.Token
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode() {}

func (ws *WhileStatement) TokenLiteral() string {
	return ws.Token.Lexeme
}

type BreakStatement struct {
	Token lexer.Token
}

func (bs *BreakStatement) statementNode() {}

func (bs *BreakStatement) TokenLiteral() string {
	return bs.Token.Lexeme
}

type ContinueStatement struct {
	Token lexer.Token
}

func (cs *ContinueStatement) statementNode() {}

func (cs *ContinueStatement) TokenLiteral() string {
	return cs.Token.Lexeme
}

type UnsafeStatement struct {
	Token lexer.Token
	Body  *BlockStatement
}

func (us *UnsafeStatement) statementNode() {}

func (us *UnsafeStatement) TokenLiteral() string {
	return us.Token.Lexeme
}

type AsmStatement struct {
	Token    lexer.Token
	Block    *AsmBlock
	Template *StringLiteral
}

func (as *AsmStatement) statementNode() {}

func (as *AsmStatement) TokenLiteral() string {
	return as.Token.Lexeme
}

type AsmBlock struct {
	Token    lexer.Token
	Template *StringLiteral
	Inputs   []AsmOperand
	Outputs  []AsmOutput
	Clobbers []string
}

type AsmOperand struct {
	Register string
	Value    Expression
}

type AsmOutput struct {
	Register string
	Name     string
}

type StructStatement struct {
	Token  lexer.Token
	Name   *Identifier
	Fields []*StructField
}

func (ss *StructStatement) statementNode() {}

func (ss *StructStatement) TokenLiteral() string {
	return ss.Token.Lexeme
}

type StructField struct {
	Token    lexer.Token
	Name     *Identifier
	Type     *TypeReference
	Contract Contract
	Tags     []StructTag
}

func (sf *StructField) TokenLiteral() string {
	return sf.Token.Lexeme
}

type StructTag struct {
	Key   string
	Value string
}

type StructType struct {
	Token  lexer.Token
	Fields []*StructField
}

func (st *StructType) TokenLiteral() string {
	return st.Token.Lexeme
}

type RegisterType struct {
	Token lexer.Token
	Width int64
	// WidthExpression preserves the compile-time expression from register[N].
	// Sema, rather than the parser, owns constant evaluation as required by
	// rules/declarations/registers.md, section 3.
	WidthExpression Expression
	AllocationOrder string
	ByteOrder       string
	Fields          []*RegisterField
}

func (rt *RegisterType) TokenLiteral() string {
	return rt.Token.Lexeme
}

type RegisterField struct {
	Token lexer.Token
	Name  *Identifier
	Type  *TypeReference
	Width int64
	// WidthExpression is the compiler-owned syntax fact for bit[N]. Width keeps
	// the literal fast path for existing AST consumers; Sema resolves this
	// expression to the canonical semantic width.
	WidthExpression Expression
	Unit            string
	// UnitExpression mirrors TypeReference.UnitExpression for bit-field unit
	// annotations so register semantics do not retain a string-only unit model.
	UnitExpression *UnitExpression
	// Access preserves the compiler-known field modifier defined by
	// rules/declarations/registers.md. Empty input is normalized by the parser
	// to RegisterReadWrite.
	Access RegisterFieldAccess
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

func (rf *RegisterField) TokenLiteral() string {
	return rf.Token.Lexeme
}

type BooleanLiteral struct {
	Token lexer.Token
	Value bool
}

func (bl *BooleanLiteral) expressionNode() {}

func (bl *BooleanLiteral) TokenLiteral() string {
	return bl.Token.Lexeme
}

func (bl *BooleanLiteral) String() string {
	return bl.Token.Lexeme
}

type InterpolatedStringLiteral struct {
	Token lexer.Token
	Value string
	Parts []InterpolatedStringPart
}

// InterpolatedStringPart preserves a text segment or a parsed expression with
// an exclusive source end. Text collapses doubled braces but retains escape
// spelling for later materialization; Expression is nil for text parts.
// Rules: rules/foundations/lexical_structure.md — "14.3 Interpolated strings", "15. Escapes".
type InterpolatedStringPart struct {
	Token      lexer.Token
	End        lexer.Token
	Text       string
	Expression Expression
}

func (isl *InterpolatedStringLiteral) expressionNode() {}

func (isl *InterpolatedStringLiteral) TokenLiteral() string {
	return isl.Token.Lexeme
}

func (isl *InterpolatedStringLiteral) String() string {
	return isl.Token.Lexeme
}

type PrefixExpression struct {
	Token    lexer.Token
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode() {}

func (pe *PrefixExpression) TokenLiteral() string {
	return pe.Token.Lexeme
}

func (pe *PrefixExpression) String() string {
	if pe.Right == nil {
		return "(" + pe.Operator + "<nil>)"
	}

	return "(" + pe.Operator + pe.Right.String() + ")"
}

type InfixExpression struct {
	Token    lexer.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode() {}

func (ie *InfixExpression) TokenLiteral() string {
	return ie.Token.Lexeme
}

func (ie *InfixExpression) String() string {
	left := "<nil>"
	if ie.Left != nil {
		left = ie.Left.String()
	}

	right := "<nil>"
	if ie.Right != nil {
		right = ie.Right.String()
	}

	return "(" + left + " " + ie.Operator + " " + right + ")"
}

type RangeExpression struct {
	Token     lexer.Token
	Start     Expression
	End       Expression
	Exclusive bool
}

func (re *RangeExpression) expressionNode() {}

func (re *RangeExpression) TokenLiteral() string {
	return re.Token.Lexeme
}

func (re *RangeExpression) String() string {
	start := ""
	if re.Start != nil {
		start = re.Start.String()
	}
	end := ""
	if re.End != nil {
		end = re.End.String()
	}
	operator := ".."
	if re.Exclusive {
		operator = "..<"
	}
	return start + operator + end
}

type ConversionExpression struct {
	Token lexer.Token
	Type  *TypeReference
	Value Expression
}

func (ce *ConversionExpression) expressionNode() {}

func (ce *ConversionExpression) TokenLiteral() string {
	return ce.Token.Lexeme
}

func (ce *ConversionExpression) String() string {
	value := "<nil>"
	if ce.Value != nil {
		value = ce.Value.String()
	}

	target := ce.Type.Name
	if ce.Type.Unit != "" {
		target += "<" + ce.Type.Unit + ">"
	}
	return target + "(" + value + ")"
}

type CallExpression struct {
	Token            lexer.Token
	Callee           Expression
	Function         *Identifier
	GenericArguments []*TypeReference
	Arguments        []Expression
}

// NewExpression selects lifecycle construction for Type. It is deliberately
// distinct from CallExpression so semantic analysis cannot confuse `new T()`
// with conversion syntax such as `T(value)`.
type NewExpression struct {
	Token     lexer.Token
	Type      *TypeReference
	Arguments []Expression
}

func (ne *NewExpression) expressionNode() {}

func (ne *NewExpression) TokenLiteral() string { return ne.Token.Lexeme }

func (ne *NewExpression) String() string {
	target := "<nil>"
	if ne.Type != nil {
		target = constructionTypeReferenceString(ne.Type)
	}
	parts := make([]string, 0, len(ne.Arguments))
	for _, argument := range ne.Arguments {
		parts = append(parts, argument.String())
	}
	return "new " + target + "(" + strings.Join(parts, ", ") + ")"
}

func constructionTypeReferenceString(ref *TypeReference) string {
	if ref == nil {
		return "<nil>"
	}
	name := ref.Name
	if ref.UnitOnly {
		name = "<" + ref.Unit + ">"
	} else if ref.Unit != "" {
		name += "<" + ref.Unit + ">"
	}
	arguments := make([]string, 0, len(ref.TypeArgs)+len(ref.ConstArgs))
	for _, argument := range ref.TypeArgs {
		arguments = append(arguments, constructionTypeReferenceString(argument))
	}
	for _, argument := range ref.ConstArgs {
		arguments = append(arguments, argument.String())
	}
	if len(arguments) > 0 {
		name += "[" + strings.Join(arguments, ", ") + "]"
	}
	if ref.Ref {
		prefix := "ref "
		if ref.MutableRef {
			prefix = "ref mut "
		}
		name = prefix + name
	}
	return name
}

func (ce *CallExpression) expressionNode() {}

func (ce *CallExpression) TokenLiteral() string {
	return ce.Token.Lexeme
}

func (ce *CallExpression) String() string {
	name := "<nil>"
	if ce.Callee != nil {
		name = ce.Callee.String()
	} else if ce.Function != nil {
		name = ce.Function.Value
	}

	out := name
	if len(ce.GenericArguments) > 0 {
		out += "["
		for i, arg := range ce.GenericArguments {
			if i > 0 {
				out += ", "
			}
			out += arg.Name
		}
		out += "]"
	}
	out += "("
	for i, arg := range ce.Arguments {
		if i > 0 {
			out += ", "
		}
		out += arg.String()
	}
	out += ")"
	return out
}

type RuntimeCallExpression struct {
	Token     lexer.Token
	Name      string
	Arguments []Expression
}

func (rce *RuntimeCallExpression) expressionNode() {}

func (rce *RuntimeCallExpression) TokenLiteral() string {
	return rce.Token.Lexeme
}

func (rce *RuntimeCallExpression) String() string {
	out := "@" + rce.Name + "("
	for i, arg := range rce.Arguments {
		if i > 0 {
			out += ", "
		}
		out += arg.String()
	}
	out += ")"
	return out
}

type OkExpression struct {
	Token     lexer.Token
	Value     Expression
	Arguments []Expression
}

func (oe *OkExpression) expressionNode() {}

func (oe *OkExpression) TokenLiteral() string {
	return oe.Token.Lexeme
}

func (oe *OkExpression) String() string {
	value := "<nil>"
	if oe.Value != nil {
		value = oe.Value.String()
	}
	return "Ok(" + value + ")"
}

type ErrExpression struct {
	Token     lexer.Token
	Value     Expression
	Arguments []Expression
}

func (ee *ErrExpression) expressionNode() {}

func (ee *ErrExpression) TokenLiteral() string {
	return ee.Token.Lexeme
}

func (ee *ErrExpression) String() string {
	value := "<nil>"
	if ee.Value != nil {
		value = ee.Value.String()
	}
	return "Err(" + value + ")"
}

type TryExpression struct {
	Token      lexer.Token
	Expression Expression
	Handlers   []*TryHandler
}

func (te *TryExpression) expressionNode() {}

func (te *TryExpression) TokenLiteral() string {
	return te.Token.Lexeme
}

func (te *TryExpression) String() string {
	if te.Expression == nil {
		return "try <nil>"
	}
	return "try " + te.Expression.String()
}

type TryHandler struct {
	Token      lexer.Token
	Pattern    Expression
	Body       Expression
	ReturnBody *ReturnStatement
	BlockBody  *BlockStatement
	Invalid    bool
	Recovery   *RecoveryInfo
}

type MatchStatement struct {
	Token lexer.Token
	Match *MatchExpression
}

func (ms *MatchStatement) statementNode() {}

func (ms *MatchStatement) TokenLiteral() string {
	return ms.Token.Lexeme
}

type MatchExpression struct {
	Token   lexer.Token
	Subject Expression
	Arms    []*MatchArm
}

type MatchPatternKind string

const (
	MatchPatternInvalid  MatchPatternKind = "invalid"
	MatchPatternCatchAll MatchPatternKind = "catch-all"
	MatchPatternEmpty    MatchPatternKind = "empty"
	MatchPatternVariant  MatchPatternKind = "variant"
	MatchPatternFields   MatchPatternKind = "fields"
)

type MatchBindingMode string

const (
	MatchBindingValue      MatchBindingMode = "value"
	MatchBindingSharedRef  MatchBindingMode = "ref"
	MatchBindingMutableRef MatchBindingMode = "ref-mut"
)

// MatchPattern is deliberately not an Expression. Sec 0.1 match syntax is a
// closed structural grammar, so arbitrary value expressions cannot enter the
// AST through a pattern position.
type MatchPattern struct {
	Token     lexer.Token
	Kind      MatchPatternKind
	Name      string
	NameToken lexer.Token
	Binding   *MatchPatternBinding
	Fields    []*MatchFieldPattern
	Invalid   bool
	Recovery  *RecoveryInfo
}

type MatchPatternBinding struct {
	Token lexer.Token
	Name  *Identifier
	Mode  MatchBindingMode
}

type MatchFieldPattern struct {
	Token   lexer.Token
	Field   *Identifier
	Binding *MatchPatternBinding
}

func (mp *MatchPattern) TokenLiteral() string {
	if mp == nil {
		return ""
	}
	return mp.Token.Lexeme
}

func (mp *MatchPattern) String() string {
	if mp == nil {
		return "<nil-pattern>"
	}
	switch mp.Kind {
	case MatchPatternCatchAll:
		return "_"
	case MatchPatternEmpty:
		return "empty"
	case MatchPatternVariant:
		if mp.Binding == nil {
			return mp.Name
		}
		return mp.Name + "(" + mp.Binding.String() + ")"
	case MatchPatternFields:
		fields := make([]string, 0, len(mp.Fields))
		for _, field := range mp.Fields {
			if field == nil || field.Field == nil {
				continue
			}
			text := field.Field.Value
			if field.Binding != nil && (field.Binding.Name == nil || field.Binding.Name.Value != field.Field.Value || field.Binding.Mode != MatchBindingValue) {
				text += ": " + field.Binding.String()
			}
			fields = append(fields, text)
		}
		return mp.Name + " { " + strings.Join(fields, ", ") + " }"
	default:
		return "<invalid-pattern>"
	}
}

func (binding *MatchPatternBinding) String() string {
	if binding == nil || binding.Name == nil {
		return "<invalid-binding>"
	}
	switch binding.Mode {
	case MatchBindingSharedRef:
		return "ref " + binding.Name.Value
	case MatchBindingMutableRef:
		return "ref mut " + binding.Name.Value
	default:
		return binding.Name.Value
	}
}

// Expression preserves compatibility for analysis and legacy generators while
// they consume the new closed pattern AST. It can only produce canonical
// shapes represented by MatchPattern.
func (mp *MatchPattern) Expression() Expression {
	if mp == nil || mp.Invalid {
		return nil
	}
	switch mp.Kind {
	case MatchPatternCatchAll, MatchPatternEmpty:
		value := "_"
		if mp.Kind == MatchPatternEmpty {
			value = "empty"
		}
		return &Identifier{Token: mp.Token, Value: value}
	case MatchPatternVariant:
		base := matchPatternNameExpression(mp.Token, mp.NameToken, mp.Name)
		if mp.Binding == nil {
			return base
		}
		binding := mp.Binding.Expression()
		if mp.Name == "Ok" {
			return &OkExpression{Token: mp.Token, Value: binding, Arguments: []Expression{binding}}
		}
		if mp.Name == "Err" {
			return &ErrExpression{Token: mp.Token, Value: binding, Arguments: []Expression{binding}}
		}
		return &CallExpression{Token: mp.Token, Callee: base, Arguments: []Expression{binding}}
	case MatchPatternFields:
		literal := &StructLiteral{Token: mp.Token, Type: &TypeReference{Token: mp.Token, Name: mp.Name}}
		for _, field := range mp.Fields {
			if field == nil || field.Field == nil || field.Binding == nil {
				continue
			}
			literal.Fields = append(literal.Fields, &StructLiteralField{Token: field.Token, Name: field.Field, Value: field.Binding.Expression()})
		}
		return literal
	default:
		return nil
	}
}

func (binding *MatchPatternBinding) Expression() Expression {
	if binding == nil || binding.Name == nil {
		return nil
	}
	if binding.Mode == MatchBindingValue {
		return binding.Name
	}
	return &RefExpression{Token: binding.Token, Mutable: binding.Mode == MatchBindingMutableRef, Value: binding.Name}
}

func matchPatternNameExpression(token, nameToken lexer.Token, name string) Expression {
	separator := strings.LastIndex(name, ".")
	if separator < 0 {
		return &Identifier{Token: token, Value: name}
	}
	object := &Identifier{Token: token, Value: name[:separator]}
	propertyToken := nameToken
	if propertyToken.Lexeme == "" {
		propertyToken = token
		propertyToken.Lexeme = name[separator+1:]
	}
	memberToken := propertyToken
	memberToken.Lexeme = "."
	if memberToken.Column > token.Column {
		memberToken.Column--
	}
	return &MemberExpression{Token: memberToken, Object: object, Property: &Identifier{Token: propertyToken, Value: propertyToken.Lexeme}}
}

func (me *MatchExpression) expressionNode() {}

func (me *MatchExpression) TokenLiteral() string {
	return me.Token.Lexeme
}

func (me *MatchExpression) String() string {
	if me.Subject == nil {
		return "match <nil>"
	}
	return "match " + me.Subject.String()
}

type MatchArm struct {
	Token      lexer.Token
	Pattern    *MatchPattern
	Guard      Expression
	Body       Expression
	ReturnBody *ReturnStatement
	BlockBody  *BlockStatement
	Invalid    bool
	Recovery   *RecoveryInfo
}

type ImplStatement struct {
	Token      lexer.Token
	Extends    bool
	Target     *TypeReference
	Implements []*TypeReference
	Members    []ImplMember
}

func (is *ImplStatement) statementNode() {}

func (is *ImplStatement) TokenLiteral() string {
	return is.Token.Lexeme
}

type ImplMember interface {
	Node
	implMemberNode()
}

// InitDeclaration is a lifecycle member. ErrorType describes construction
// failure only; successful completion produces the enclosing impl target.
type InitDeclaration struct {
	Token      lexer.Token
	Parameters []*Parameter
	ErrorType  *TypeReference
	Body       *BlockStatement
}

func (id *InitDeclaration) implMemberNode() {}

func (id *InitDeclaration) TokenLiteral() string { return id.Token.Lexeme }

// InvalidMember retains a malformed or disallowed impl member and the exact
// region skipped while finding the next member boundary.
type InvalidMember struct {
	Token    lexer.Token
	Message  string
	Recovery *RecoveryInfo
}

func (im *InvalidMember) implMemberNode() {}

func (im *InvalidMember) TokenLiteral() string { return im.Token.Lexeme }

type UnitMetadataDeclaration struct {
	Token lexer.Token
	Name  string
	Value []lexer.Token
}

func (umd *UnitMetadataDeclaration) implMemberNode() {}

func (umd *UnitMetadataDeclaration) TokenLiteral() string {
	return umd.Token.Lexeme
}

type PropertyDeclaration struct {
	Token  lexer.Token
	Name   *Identifier
	Type   *TypeReference
	Static bool
	Getter *BlockStatement
	Setter *PropertySetter
}

func (pd *PropertyDeclaration) implMemberNode() {}

func (pd *PropertyDeclaration) TokenLiteral() string {
	return pd.Token.Lexeme
}

type EventDeclaration struct {
	Token   lexer.Token
	Name    *Identifier
	Storage *Identifier
}

func (ed *EventDeclaration) implMemberNode() {}

func (ed *EventDeclaration) TokenLiteral() string {
	return ed.Token.Lexeme
}

type PropertySetter struct {
	Token     lexer.Token
	Fallible  bool
	Parameter *Identifier
	Body      *BlockStatement
}

type BlockStatement struct {
	Token      lexer.Token
	Tokens     []lexer.Token
	Statements []Statement
}

func (bs *BlockStatement) TokenLiteral() string {
	return bs.Token.Lexeme
}

type MemberExpression struct {
	Token    lexer.Token
	Object   Expression
	Property *Identifier
	// OwnerGenericArguments preserves the generic-owner interpretation of the
	// syntactically ambiguous E[T].Member form. Object retains the ordinary
	// indexed-expression interpretation so Sema can choose from resolved types.
	OwnerGenericArguments []*TypeReference
}

func (me *MemberExpression) expressionNode() {}

func (me *MemberExpression) TokenLiteral() string {
	return me.Token.Lexeme
}

func (me *MemberExpression) String() string {
	owner := me.Object.String()
	if _, indexed := me.Object.(*IndexExpression); !indexed && len(me.OwnerGenericArguments) > 0 {
		arguments := make([]string, 0, len(me.OwnerGenericArguments))
		for _, argument := range me.OwnerGenericArguments {
			arguments = append(arguments, constructionTypeReferenceString(argument))
		}
		owner += "[" + strings.Join(arguments, ", ") + "]"
	}
	return owner + "." + me.Property.Value
}

type ArrayLiteral struct {
	Token    lexer.Token
	Elements []Expression
}

func (al *ArrayLiteral) expressionNode() {}

func (al *ArrayLiteral) TokenLiteral() string {
	return al.Token.Lexeme
}

func (al *ArrayLiteral) String() string {
	out := "["
	for i, element := range al.Elements {
		if i > 0 {
			out += ", "
		}
		out += element.String()
	}
	out += "]"
	return out
}

type SpreadExpression struct {
	Token lexer.Token
	Value Expression
}

func (se *SpreadExpression) expressionNode() {}

func (se *SpreadExpression) TokenLiteral() string {
	return se.Token.Lexeme
}

func (se *SpreadExpression) String() string {
	if se.Value == nil {
		return "<nil>..."
	}
	return se.Value.String() + "..."
}

type IndexExpression struct {
	Token lexer.Token
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode() {}

func (ie *IndexExpression) TokenLiteral() string {
	return ie.Token.Lexeme
}

func (ie *IndexExpression) String() string {
	return ie.Left.String() + "[" + ie.Index.String() + "]"
}

type SliceExpression struct {
	Token     lexer.Token
	Left      Expression
	Start     Expression
	End       Expression
	Exclusive bool
}

func (se *SliceExpression) expressionNode() {}

func (se *SliceExpression) TokenLiteral() string {
	return se.Token.Lexeme
}

func (se *SliceExpression) String() string {
	start := ""
	if se.Start != nil {
		start = se.Start.String()
	}
	end := ""
	if se.End != nil {
		end = se.End.String()
	}
	operator := ".."
	if se.Exclusive {
		operator = "..<"
	}
	return se.Left.String() + "[" + start + operator + end + "]"
}

type RefExpression struct {
	Token   lexer.Token
	Mutable bool
	Value   Expression
}

func (re *RefExpression) expressionNode() {}

func (re *RefExpression) TokenLiteral() string {
	return re.Token.Lexeme
}

func (re *RefExpression) String() string {
	if re.Mutable {
		return "ref mut " + re.Value.String()
	}
	return "ref " + re.Value.String()
}

type StructLiteral struct {
	Token  lexer.Token
	Type   *TypeReference
	Fields []*StructLiteralField
}

func (sl *StructLiteral) expressionNode() {}

func (sl *StructLiteral) TokenLiteral() string {
	return sl.Token.Lexeme
}

func (sl *StructLiteral) String() string {
	return sl.Type.Name + "{...}"
}

type StructLiteralField struct {
	Token  lexer.Token
	Name   *Identifier
	Value  Expression
	Spread bool
}

type SpawnExpression struct {
	Token lexer.Token
	Kind  string
	Value Expression
	Body  *BlockStatement
}

func (se *SpawnExpression) expressionNode() {}

func (se *SpawnExpression) TokenLiteral() string {
	return se.Token.Lexeme
}

func (se *SpawnExpression) String() string {
	prefix := "spawn"
	if se.Kind != "" && se.Kind != "task" {
		prefix += " " + se.Kind
	}
	if se.Value != nil {
		return prefix + " " + se.Value.String()
	}
	return prefix + " {...}"
}

type AwaitExpression struct {
	Token lexer.Token
	Value Expression
}

func (ae *AwaitExpression) expressionNode() {}

func (ae *AwaitExpression) TokenLiteral() string {
	return ae.Token.Lexeme
}

func (ae *AwaitExpression) String() string {
	if ae.Value == nil {
		return "await <nil>"
	}
	return "await " + ae.Value.String()
}

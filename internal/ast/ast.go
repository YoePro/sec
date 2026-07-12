package ast

import (
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

type InvalidStatement struct {
	Token lexer.Token
}

func (is *InvalidStatement) statementNode() {}

func (is *InvalidStatement) TokenLiteral() string {
	return is.Token.Lexeme
}

// Expression represents a value-producing AST node.
type Expression interface {
	Node
	expressionNode()
	String() string
}

// Program is the root node for a parsed Sec source file.
type Program struct {
	Statements []Statement
}

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

// TypeDeclStatement represents:
//
//	type Percent int range 0..100
//	type Meter decimal<m>
//	type Email string
//	type IOError = FileNotFound AccessDenied InvalidValue
type TypeDeclStatement struct {
	Token lexer.Token // TYPE token

	Name         *Identifier
	BaseType     *TypeReference
	AssignedType *TypeReference
	Variants     []*Identifier
	StructType   *StructType
	Contract     Contract
}

func (tds *TypeDeclStatement) statementNode() {}

func (tds *TypeDeclStatement) implMemberNode() {}

func (tds *TypeDeclStatement) TokenLiteral() string {
	return tds.Token.Lexeme
}

type EnumDeclaration struct {
	Token          lexer.Token
	Name           *Identifier
	UnderlyingType *TypeReference
	Values         []*EnumValue
}

func (ed *EnumDeclaration) statementNode() {}

func (ed *EnumDeclaration) implMemberNode() {}

func (ed *EnumDeclaration) TokenLiteral() string {
	return ed.Token.Lexeme
}

type EnumValue struct {
	Token       lexer.Token
	Name        *Identifier
	Initializer Expression
}

func (ev *EnumValue) TokenLiteral() string {
	return ev.Token.Lexeme
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
//	[]byte
type TypeReference struct {
	Token lexer.Token

	Name string

	// ElementType is used for slice and array types such as []byte and [3]int.
	ElementType *TypeReference
	ArrayLength int64

	// Unit is used for unit types such as decimal<m> or decimal<SEK>.
	Unit string

	// TypeArgs is used for generic types such as Vec[T], Map[K,V], Result[T,E].
	TypeArgs []*TypeReference

	// FunctionParameterTypes and FunctionReturnType are used for function
	// value types such as fn(int, string) bool.
	FunctionParameterTypes []*TypeReference
	FunctionReturnType     *TypeReference
}

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

// --------------------------------------------------------------------
// Literals
// --------------------------------------------------------------------

type IntegerLiteral struct {
	Token lexer.Token
	Value int64
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
	if len(lexeme) > 2 && lexeme[0] == '0' && (lexeme[1] == 'x' || lexeme[1] == 'X') {
		last := lexeme[len(lexeme)-1]
		switch last {
		case 'i', 'u':
			return lexeme[:len(lexeme)-1], string(last)
		default:
			return lexeme, ""
		}
	}
	last := lexeme[len(lexeme)-1]
	switch last {
	case 'i', 'u', 'f', 'd':
		return lexeme[:len(lexeme)-1], string(last)
	default:
		return lexeme, ""
	}
}

func ParseIntegerLiteralLexeme(lexeme string) (*big.Int, bool) {
	digits, suffix := SplitNumericLiteralSuffix(lexeme)
	if suffix == "f" || suffix == "d" || digits == "" {
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
	if suffix == "i" || suffix == "u" || digits == "" {
		return 0, false
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

type LetStatement struct {
	Token   lexer.Token
	Mutable bool
	Name    *Identifier
	Type    *TypeReference
	Value   Expression
}

func (ls *LetStatement) statementNode() {}

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
	Token    lexer.Token
	Target   Expression
	Operator string
	Value    Expression
}

func (as *AssignmentStatement) statementNode() {}

func (as *AssignmentStatement) TokenLiteral() string {
	return as.Token.Lexeme
}

type TryAssignmentStatement struct {
	Token      lexer.Token
	Assignment *AssignmentStatement
}

func (tas *TryAssignmentStatement) statementNode() {}

func (tas *TryAssignmentStatement) TokenLiteral() string {
	return tas.Token.Lexeme
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
	Token      lexer.Token
	Name       *Identifier
	Parameters []*Parameter
	ReturnType *TypeReference
	Body       *BlockStatement
	Unsafe     bool
}

func (fd *FunctionDeclaration) statementNode() {}

func (fd *FunctionDeclaration) implMemberNode() {}

func (fd *FunctionDeclaration) TokenLiteral() string {
	return fd.Token.Lexeme
}

type Parameter struct {
	Token lexer.Token
	Name  *Identifier
	Type  *TypeReference
	Ref   bool
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

	return ce.Type.Name + "(" + value + ")"
}

type CallExpression struct {
	Token     lexer.Token
	Callee    Expression
	Function  *Identifier
	Arguments []Expression
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

	out := name + "("
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
	Token lexer.Token
	Value Expression
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
	Token lexer.Token
	Value Expression
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
	Pattern    Expression
	Guard      Expression
	Body       Expression
	ReturnBody *ReturnStatement
	BlockBody  *BlockStatement
}

type ImplStatement struct {
	Token   lexer.Token
	Target  *TypeReference
	Members []ImplMember
}

func (is *ImplStatement) statementNode() {}

func (is *ImplStatement) TokenLiteral() string {
	return is.Token.Lexeme
}

type ImplMember interface {
	Node
	implMemberNode()
}

type PropertyDeclaration struct {
	Token  lexer.Token
	Name   *Identifier
	Type   *TypeReference
	Getter *BlockStatement
	Setter *PropertySetter
}

func (pd *PropertyDeclaration) implMemberNode() {}

func (pd *PropertyDeclaration) TokenLiteral() string {
	return pd.Token.Lexeme
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
}

func (me *MemberExpression) expressionNode() {}

func (me *MemberExpression) TokenLiteral() string {
	return me.Token.Lexeme
}

func (me *MemberExpression) String() string {
	return me.Object.String() + "." + me.Property.Value
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
	Token lexer.Token
	Name  *Identifier
	Value Expression
}

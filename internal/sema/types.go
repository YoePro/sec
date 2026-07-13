package sema

import (
	"math/big"
	"unicode"

	"sec/internal/lexer"
)

type TypeKind string

const (
	InvalidType  TypeKind = "invalid"
	BoolType     TypeKind = "bool"
	IntType      TypeKind = "int"
	UintType     TypeKind = "uint"
	FloatType    TypeKind = "float"
	DecimalType  TypeKind = "decimal"
	EnumType     TypeKind = "enum"
	StringType   TypeKind = "string"
	CharType     TypeKind = "char"
	RuneType     TypeKind = "rune"
	ResultType   TypeKind = "result"
	StructType   TypeKind = "struct"
	SliceType    TypeKind = "slice"
	ArrayType    TypeKind = "array"
	FunctionType TypeKind = "function"
	VoidType     TypeKind = "void"
)

type Type struct {
	Name                   string
	Module                 string
	Kind                   TypeKind
	Named                  bool
	Declared               bool
	Underlying             string
	Unit                   string
	Dimension              Dimension
	MinInt                 *int64
	MaxInt                 *int64
	MinUint                *uint64
	MaxUint                *uint64
	Contracts              []Contract
	EnumValues             []string
	EnumConsts             map[string]EnumValue
	TypeArgs               []Type
	Element                *Type
	ArrayLength            int64
	FunctionParameterTypes []Type
	FunctionReturnType     *Type
	Fields                 []StructField
	Properties             []Property
}

type EnumValue struct {
	Name  string
	Value *big.Int
	Token lexer.Token
}

type StructField struct {
	Name  string
	Type  Type
	Token lexer.Token
	Tags  []StructTag
}

type StructTag struct {
	Key   string
	Value string
}

type Property struct {
	Name     string
	Type     Type
	Token    lexer.Token
	Fallible bool
	Error    *Type
}

type Function struct {
	Name       string
	Module     string
	Parameters []FunctionParameter
	ReturnType Type
	Token      lexer.Token
}

type FunctionParameter struct {
	Name  string
	Type  Type
	Token lexer.Token
	Ref   bool
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
	Exclusive bool
}

func (RangeContract) contractNode() {}

type DecimalValue struct {
	Int64 int64
	Scale uint8
}

type Symbol struct {
	Name    string
	Type    Type
	Mutable bool
	Token   lexer.Token
}

func builtinTypes() map[string]Type {
	types := map[string]Type{
		"bool":    {Name: "bool", Kind: BoolType},
		"byte":    unsignedType("byte", 255),
		"char":    {Name: "char", Kind: CharType},
		"rune":    {Name: "rune", Kind: RuneType},
		"Result":  {Name: "Result", Kind: ResultType},
		"decimal": {Name: "decimal", Kind: DecimalType},
		"float":   {Name: "float", Kind: FloatType},
		"float32": {Name: "float32", Kind: FloatType},
		"float64": {Name: "float64", Kind: FloatType},
		"int":     signedType("int", -1<<63, 1<<63-1),
		"int8":    signedType("int8", -1<<7, 1<<7-1),
		"int16":   signedType("int16", -1<<15, 1<<15-1),
		"int32":   signedType("int32", -1<<31, 1<<31-1),
		"int64":   signedType("int64", -1<<63, 1<<63-1),
		"string":  {Name: "string", Kind: StringType},
		"uint":    unsignedType("uint", ^uint64(0)),
		"uint8":   unsignedType("uint8", 1<<8-1),
		"uint16":  unsignedType("uint16", 1<<16-1),
		"uint32":  unsignedType("uint32", 1<<32-1),
		"uint64":  unsignedType("uint64", ^uint64(0)),
		"void":    {Name: "void", Kind: VoidType},
	}

	return types
}

func signedType(name string, min, max int64) Type {
	return Type{Name: name, Kind: IntType, MinInt: &min, MaxInt: &max}
}

func unsignedType(name string, max uint64) Type {
	var min uint64
	return Type{Name: name, Kind: UintType, MinUint: &min, MaxUint: &max}
}

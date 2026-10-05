# Grammar

## Status

This document is the canonical consolidated grammar for Sec 0.1.

It defines:

- compilation-unit structure;
- declarations;
- type references;
- statements;
- expressions;
- patterns;
- contextual syntax;
- canonical syntax versus accepted recovery syntax;
- the boundary between parsing and semantic analysis.

This document does not redefine:

- lexical tokenization from `lexical_structure.md`;
- operator precedence or operator semantics from `operators.md`;
- formatting from `formatter.md`;
- type semantics from `types.md`;
- ownership semantics from `ownership.md` and `copy_move.md`;
- detailed feature semantics from specialized rulebooks.

Implementation status is not part of this rulebook. It is tracked by
`frontend.grammar-conformance` in `governance/parser.yaml` and by the
feature-specific governance entries.

---

# Canonical syntax and legacy forms

Sec 0.1 accepts only the canonical syntax defined by the active rulebooks.
Recognizable legacy Sec syntax is invalid and receives a focused migration
diagnostic when the intended migration can be determined reliably. Default
formatting never converts invalid legacy syntax into canonical syntax; an
explicitly enabled Language Correction (`rules/tooling/formatter.md` §§ 26–27)
may rewrite a recognized legacy form only when the rewrite is unambiguous and
semantics-preserving, and otherwise reports a focused migration diagnostic
instead of guessing (MD-003; `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 4).

The recognized invalid forms are listed in "Legacy, future, and recovery
syntax" below.

---

# Forms outside Sec 0.1

The following are not part of Sec 0.1:

- the conditional expression `condition ? whenTrue : whenFalse`; `?` is
  reserved with no Sec 0.1 meaning;
- first-class range values such as `let range := 0..<10`; ranges remain
  contextual to `for`, membership, slicing, switch cases, and other explicitly
  defined range contexts;
- field-level defaults in struct declarations such as `port: Port = Port(8080)`;
  omitted fields use their type defaults;
- multiple return values such as `fn Read() (Value, Error)`; a function returns
  one value, and a named struct, union, `Result`, or another explicit type
  carries several;
- separate interface implementation syntax `impl Interface for Type`;
  interfaces are listed on the primary implementation, as in
  `impl Car implements Vehicle`;
- classes, object inheritance, and virtual class methods; Sec uses `struct`,
  `impl`, `interface`, and `union`;
- C-style `for` loops and condition-only `for`; use ranges or `while`;
- `do while`, `goto`, and loop labels;
- `switch` and `select` expressions; both are statements, and `match` is the
  value-producing exhaustive branch construct;
- arbitrary user-defined operators and operator methods selected by spelling;
  a token such as `+` never resolves to an arbitrary user method;
- general compile-time execution beyond the approved constant contexts:
  arbitrary user compile-time functions, general compile-time blocks,
  compile-time I/O, and macro or token-macro syntax
  (`rules/compiler/compile_time_evaluation.md`);
- field-level match destructuring and general collection patterns;
- an implicit user-level module initializer; executable startup is defined by
  `rules/compiler/initialization.md`.

Map and set literal syntax is not finalized by this grammar. Array literals and
empty list literals are defined below.

The module model, including resolution, import cycles, re-exports, selective
and wildcard imports, export lists, conditional imports, and target-conditioned
declarations, belongs to `rules/projects/modules.md`.

---

# Normative grammar notation

The grammar uses an EBNF-like notation.

```text
"token"
    literal source spelling

RuleName
    reference to another grammar rule

[ Rule ]
    optional

{ Rule }
    zero or more repetitions

Rule { "," Rule }
    comma-separated repetition

( A | B )
    alternatives

Contextual("word")
    spelling lexed as an identifier and interpreted by parser context

TokenClass
    lexical category defined by lexical_structure.md
```

Grammar notation is not Sec source syntax.

---

# Lexical boundary

The lexer produces tokens.

The parser consumes tokens.

The grammar assumes:

- comments and whitespace are recognized lexically;
- source positions include file, line, and column;
- multi-character operators use longest match;
- string and character escaping is already validated lexically where possible;
- identifiers are distinct from reserved keywords;
- contextual spellings may remain identifier tokens.

The grammar must not duplicate the complete lexical specification.

---

# Source file

Canonical high-level grammar:

```text
CompilationUnit
    ::= [ TargetDirective ]
        ModuleDeclaration
        { ImportDeclaration }
        { TopLevelDeclaration }
```

A source file must declare a module.

`#target`, when present, must precede every declaration and import.

The parser may recover from misplaced declarations, but canonical source follows
the order above.

---

# Target directive

```text
TargetDirective
    ::= "#" "target" "(" TargetArgument "," TargetArgument [ "," ] ")"

TargetArgument
    ::= "os" ":" StringLiteral
      | "arch" ":" StringLiteral
```

Both `os` and `arch` are required.

Their order is not semantically significant.

Unknown or duplicate arguments are invalid.

Example:

```sec
#target(os: "linux", arch: "amd64")
```

`target` is the only directive after `#` in Sec 0.1.

---

# Module declaration

```text
ModuleDeclaration
    ::= "module" Identifier
```

Example:

```sec
module parser
```

Dotted source module declarations are invalid. Directory hierarchy,
`internal`, and compiler-reserved roots belong to canonical import paths and
never become part of the source `ModuleName`.

The complete relationship between source path, `internal` directories, module
identity, and imports belongs to `rules/projects/modules.md` and
`rules/projects/projects.md`.

---

# Import declarations

```text
ImportDeclaration
    ::= SingleImport
      | ImportGroup

SingleImport
    ::= "import" [ Identifier ] StringLiteral

ImportGroup
    ::= "import" "(" { ImportGroupItem } ")"

ImportGroupItem
    ::= [ Identifier ] StringLiteral
```

Examples:

```sec
import "fmt"
import sys "platform/linux"
```

```sec
import (
    "fmt"
    "platform/linux/amd64"
    sys "platform/linux"
)
```

The string is a canonical logical import path. It uses `/`, is case-sensitive,
is not an absolute host path, and contains neither `.` nor `..` traversal
components.

An optional identifier is a source-file-local import alias. Imports do not
inject unqualified names and are not implicitly re-exported.

Selective and wildcard imports are not part of Sec 0.1.

---

# Top-level declarations

```text
TopLevelDeclaration
    ::= TypeDeclaration
      | UnitDeclaration
      | EnumDeclaration
      | InterfaceDeclaration
      | FunctionDeclaration
      | ExternFunctionDeclaration
      | UnsafeFunctionDeclaration
      | ImplDeclaration
      | StaticDeclaration
      | LetDeclaration
      | TypedDeclaration
      | TypedDeclarationGroup
      | AddressedLetDeclaration
      | TestDeclaration
      | Comment
```

Ordinary executable statements are invalid at module scope.

Top-level mutable storage remains subject to static and initialization rules.

Test source adds the contextual top-level declaration:

```text
TestDeclaration
    ::= "test" StringLiteral Block
```

It is legal only in `*_test.sec` selected by a `TestCompilationPlan`. It has no
parameters or source return type, may not be nested, and is governed by
`rules/tooling/testing.md`. Lexer, parser, formatter, editor grammar, and Sema
must share one contextual treatment of `test` rather than reserving or inferring
it independently.

---

# Identifier namespaces

The grammar permits declarations in contexts controlled by name and scope rules.

The parser does not decide:

- whether a name is already declared;
- whether a name is visible;
- whether a name is reserved by a compiler-known namespace;
- whether a nested name conflicts with a module declaration.

Those checks belong to Sema.

---

# Generic parameter lists

```text
GenericParameterList
    ::= "[" GenericParameter { "," GenericParameter } [ "," ] "]"

GenericParameter
    ::= Identifier [ GenericConstraintClause ]

GenericConstraintClause
    ::= ":" TypeReference { "&" TypeReference }
```

Examples:

```sec
type Pair[A, B] struct {
    first: A,
    second: B,
}
```

```sec
fn Save[T: Serializable](value: T) Result[void, IOError] {
    return Ok()
}
```

`&` combines multiple interface constraints. It is a compile-time conjunction,
not a runtime intersection type.

A general `where` clause for generic declarations is not part of the current
grammar.

---

# Type declarations

Canonical forms:

```text
TypeDeclaration
    ::= NamedTypeDeclaration
      | StructTypeDeclaration
      | UnionTypeDeclaration
      | TypeEnumDeclaration
      | RegisterTypeDeclaration
```

---

# Named type declaration

```text
NamedTypeDeclaration
    ::= "type" Identifier [ GenericParameterList ]
        TypeReference
        [ ImplementsClause ]
        { TypeContract }
        [ DefaultClause ]
```

Example:

```sec
type Percent int range 0..100
```

Example:

```sec
type Port int range 1..65535 default 8080
```

Example:

```sec
type Speed decimal<m/s>
```

This creates a nominal type.

It is not a transparent alias.

A named type declaration has exactly one underlying type. The named type is
distinct from its underlying type, and chained named types remain distinct
identities:

```sec
type B int
type A B
```

declares three distinct types: `int`, `B`, and `A`.

A declaration naming more than one underlying type is invalid:

```sec
type A int string
type A B int
```

A declaration names exactly one type. Several names in one declaration are
invalid; each type is declared separately:

```sec
type A, B int        // invalid
type A int
type B int
```

`type A B` is valid only when `B` resolves to a type; otherwise `B` is an
unknown type.

(MD-002; `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 3.)

---

# Legacy assigned and compact variant forms

The following legacy forms are not Sec 0.1 grammar. The parser recognizes them
only to issue a focused migration diagnostic:

```text
LegacyAssignedNamedType
    ::= "type" Identifier [ GenericParameterList ] "=" TypeReference ...

LegacyCompactVariantDeclaration
    ::= "type" Identifier "=" Identifier Identifier { Identifier }
```

Examples:

```sec
type UserID = uint64
type IOError = FileNotFound AccessDenied InvalidValue
```

The canonical replacement for the first is `type UserID uint64`. Closed
alternatives use `enum` or `union`.

---

# Implements clause

```text
InterfaceImplementsClause
    ::= "implements" TypeReference { "," TypeReference }

ImplImplementsClause
    ::= "implements" TypeReference { "," TypeReference }
```

Examples:

```sec
impl Car implements Vehicle {
}
```

```sec
interface Vehicle implements Startable, Stoppable {
}
```

Separate `impl Interface for Type` syntax is invalid.

`ImplImplementsClause` is valid only on the primary `impl Type`. An
`impl extends Type` fragment cannot redeclare conformance.

---

# Type contracts

```text
TypeContract
    ::= RangeContract
      | MembershipContract
      | MultipleOfContract
      | LengthContract
      | RegexContract
      | MarkerContract

RangeContract
    ::= "range" [ Expression ] RangeOperator [ Expression ]

RangeOperator
    ::= ".."
      | "..<"

MembershipContract
    ::= "in" "[" Expression { "," Expression } [ "," ] "]"

MultipleOfContract
    ::= Contextual("multipleOf") Expression

LengthContract
    ::= Contextual("minLen") Expression
      | Contextual("maxLen") Expression
      | Contextual("exactLen") Expression

RegexContract
    ::= Contextual("regex") Expression

MarkerContract
    ::= Contextual("notEmpty")
      | Contextual("unique")
      | Contextual("finite")
      | Contextual("odd")
      | Contextual("even")
```

Contracts are written sequentially.

Sequential contracts are logical conjunction.

Every contract argument is ordinary `Expression` syntax. Contract arguments
form no separate restricted constant sublanguage: the owning semantic rule
(`rules/types/contracts.md`) classifies each position as a
`SemanticCompileTimeRequiredContext` (`rules/compiler/compile_time_evaluation.md`)
and supplies its required result type and domain. A range bound is therefore
not limited to a signed numeric literal; `range MinimumPort..MaximumPort` and
`range 1..MaxPort()` are grammatical, and they are valid when semantic CTE
establishes the values. Expression parsing of a bound stops at the range
operator, which is not an infix operator. An upper bound starts on the line of
the range operator and is never `default` or a following contract word, which
keeps an open-ended range unambiguous (MD-011; `rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` § 3).

`regex` is a reserved contract spelling (`rules/foundations/lexical_structure.md`
§ 7.3) that is lexed identifier-like and resolved in contract position; it has
no dedicated hard-keyword token. Its `Expression` must produce the
compile-time string pattern required by `rules/types/contracts.md`. A contract,
including `regex`, has no physical same-line requirement; newline handling
follows ordinary whitespace and expression-continuation rules, so both of the
following are grammatical before canonical formatting:

```sec
type Email string regex "..."

type Email string
    regex "..."
```

The regular-expression language belongs to a dedicated future regex rulebook
(MD-010), and the regex pattern is an ordinary `Expression` evaluated in a
`SemanticCompileTimeRequiredContext` (MD-011). The earlier
`RegexContract ::= Contextual("regex") ConstantExpression` shape is superseded
(MD-009; `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 10;
`rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` §§ 2, 3.39–3.44).

Example:

```sec
type PageSize int range 10..100 multipleOf 10
```

Example:

```sec
type Tags string[] notEmpty unique
```

Contracts belong to named types.

They are not canonical on individual variables or struct fields.

---

# Default clause

```text
DefaultClause
    ::= "default" Expression
```

The default clause follows every contract. Its expression is ordinary
`Expression` syntax evaluated in a `SemanticCompileTimeRequiredContext`
(MD-011; `rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` §§ 3.4, 3.35–3.38).

Example:

```sec
type Port int range 1..65535 default 8080
```

Example:

```sec
type User string in ["Admin", "User", "Other"] default "User"
```

The expression must be compile-time evaluable, allocation-free, representable,
and valid for every contract.

---

# Struct type declaration

```text
StructTypeDeclaration
    ::= "type" Identifier [ GenericParameterList ]
        "struct"
        [ ImplementsClause ]
        StructBody

StructBody
    ::= "{" [ StructField { "," StructField } [ "," ] ] "}"

StructField
    ::= Identifier ":" TypeReference [ StructTag ]

StructTag
    ::= RawStringLiteral
```

Canonical examples:

```sec
type Point struct {
    x: int,
    y: int,
}
```

```sec
type User struct {
    ID: int       `json:"id"`,
    Name: string  `json:"name"`,
}
```

Commas are required between declared struct fields.

A trailing comma is allowed.

Contracts are expressed through named field types.

---

# Standalone struct declaration (Sec 0.2)

`struct Identifier StructBody` is planned Sec 0.2 syntax and is not part of the
Sec 0.1 grammar. Sec 0.1 source uses `type Name struct`.

---

# Struct literal

```text
StructLiteral
    ::= TypeReference "{" [ StructLiteralItems ] "}"

StructLiteralItems
    ::= StructLiteralItem
        { StructLiteralSeparator StructLiteralItem }
        [ "," ]

StructLiteralSeparator
    ::= ","
      | LineBreak

StructLiteralItem
    ::= Identifier ":" Expression
      | Expression "..."
```

Examples:

```sec
Point {
    x: 10,
    y: 20,
}
```

```sec
Lexer {
    input: runes
    file: file
}
```

```sec
Settings {
    base...
    enabled: true,
}
```

A newline may separate struct literal items.

Declared struct fields require commas.

An empty literal is valid when every omitted field is defaultable:

```sec
Token {}
```

Sema inserts omitted field defaults.

---

# Enum declaration

```text
EnumDeclaration
    ::= "enum" Identifier [ GenericParameterList ] [ EnumUnderlying ] [ "error" ] EnumBody

TypeEnumDeclaration
    ::= "type" Identifier [ GenericParameterList ] "enum" [ EnumUnderlying ] [ "error" ] EnumBody

EnumUnderlying
    ::= [ ":" ] IntegerTypeReference
      | [ ":" ] StringTypeReference
      | [ ":" ] BitUnderlying

StringTypeReference
    ::= Contextual("string")

BitUnderlying
    ::= Contextual("bit") [ "[" IntegerConstant "]" ]

EnumBody
    ::= "{" EnumValue { EnumValueSeparator EnumValue } [ "," ] "}"

EnumValueSeparator
    ::= ","
      | LineBreak

EnumValue
    ::= Identifier [ "default" ] [ "=" ConstantExpression ]
```

The marker order is normative: `MEMBER default [= expression]`.
`default MEMBER` is not enum-member syntax.

Examples:

```sec
enum Color {
    red,
    green,
    blue,
}
```

```sec
enum Status int {
    unknown = 0,
    active = 10,
    paused,
}
```

```sec
enum ClockSource: bit[2] {
    Internal = 0b00,
    External = 0b01,
}
```

Canonical enum value initialization uses `=`. The legacy `Value: expression`
initializer is invalid Sec 0.1 syntax (see "Legacy, future, and recovery
syntax").

The first omitted initializer is implicit `iota`. Every later omitted
initializer repeats the preceding initializer expression and evaluates it with
the current member's `iota`; it is not automatic previous-value-plus-one
continuation.

An explicit ordinary underlying type may be an integer type or `string`. A
string-backed enum requires an explicit compile-time string initializer for
every member; omitted initializers and `iota` are not part of string-backed
enums:

```sec
enum Program string {
    OneCare = "Zebra OneCare",
    VIQ = "Z1C+VIQ",
}
```

Generic parameters use the ordinary `GenericParameterList` grammar. They do not
give enum members payloads; payload-bearing alternatives are union declarations
(`rules/declarations/enums.md` § 16.1):

```sec
enum State[T] {
    Ready
    Busy
}
```

An enum must declare at least one value.

When present, `error` is the final semantic marker before the enum body. It is
not an underlying type and does not introduce general inheritance.

---

# Union declaration

```text
UnionTypeDeclaration
    ::= "type" Identifier [ GenericParameterList ]
        "union"
        [ "error" ]
        [ ImplementsClause ]
        UnionBody

UnionBody
    ::= "{" UnionVariant { [ "," ] UnionVariant } [ "," ] "}"

UnionVariant
    ::= Identifier [ "default" ]
      | Identifier "(" TypeReference ")" [ "default" ]
      | Identifier StructPayload [ "default" ]

StructPayload
    ::= StructBody
```

Examples:

```sec
type State union {
    idle default
    running
    stopped
}
```

```sec
type Number union {
    Integer(int)
    Decimal(decimal)
}
```

```sec
type Shape union {
    Circle {
        radius: decimal,
    }

    Rectangle {
        width: decimal,
        height: decimal,
    }
}
```

A comma after a union variant is optional.

Struct-like payload fields follow struct declaration field grammar and require
commas.

At most one variant may carry the post-variant `default` marker. Its payload
must be default-constructible under `rules/declarations/unions.md` and
`rules/types/default_values.md`.

An empty union is invalid.

---

# Register declaration

```text
RegisterTypeDeclaration
    ::= "type" Identifier
        Contextual("register")
        "[" IntegerConstant "]"
        [ RegisterAllocationOrder ]
        [ RegisterByteOrder ]
        RegisterBody
        [ ImplementsClause ]

RegisterAllocationOrder
    ::= Contextual("lsb-first")
      | Contextual("msb-first")

RegisterByteOrder
    ::= Contextual("little-endian")
      | Contextual("big-endian")

RegisterBody
    ::= "{" [ RegisterField
        { RegisterFieldSeparator RegisterField }
        [ "," ] ] "}"

RegisterFieldSeparator
    ::= ","
      | LineBreak

RegisterField
    ::= RegisterFieldName ":" RegisterFieldType

RegisterFieldName
    ::= Identifier
      | "_"

RegisterFieldType
    ::= BitFieldType
      | TypeReference

BitFieldType
    ::= Contextual("bit") [ "[" IntegerConstant "]" ] [ UnitAnnotation ]
```

Example:

```sec
type MotorProtocol register[8] {
    Speed: bit[4]<rpm>,
    Enabled: bit,
    _: bit[3],
}
```

The allocation and byte-order modifiers are independent and may both be
present:

```sec
type PacketWord register[16] msb-first big-endian {
    Kind: bit[4],
    Value: bit[12],
}
```

The total field width must match the register width.

`_` denotes reserved unnamed bits in register field position.

---

# Unit declaration

```text
UnitDeclaration
    ::= "unit" Identifier [ UnitDefaultNumericType ] [ UnitCategory ]

UnitCategory
    ::= Contextual("physical")
      | Contextual("currency")
      | Contextual("information")
      | Contextual("ratio")
      | Contextual("other")
```

`UnitDefaultNumericType` must resolve to a plain compiler-known numeric scalar
carrier and must not itself be unit-bearing. It defaults to `decimal`.
`UnitCategory` defaults to `other`. Unit identifiers remain subject to semantic
reserved-name validation; in particular, `bit` and `byte` cannot be declared as
units.

Examples:

```sec
unit Hertz physical
unit Packet uint other
unit Metre physical
unit Count
unit Euro currency
```

Complete unit semantics belong to `rules/types/units.md`.

---

# Unit metadata inside impl

The parser recognizes these contextual unit metadata names inside `impl`:

```text
LongName
Symbol
BaseUnit
Status
Dimension
Kind
Scale
System
Transform
Offset
Origin
LogBase
LogFactor
Reference
```

Grammar shape:

```text
UnitMetadataDeclaration
    ::= ContextualUnitMetadataName ":" TokensUntilLineEnd
```

PascalCase is canonical. Case and underscore tolerant parsing may remain as a
Sec 0.1 migration aid.

---

# Interface declaration

```text
InterfaceDeclaration
    ::= "interface" Identifier [ GenericParameterList ]
        [ ImplementsClause ]
        InterfaceBody

InterfaceBody
    ::= "{" { InterfaceMember } "}"

InterfaceMember
    ::= InterfaceMethod
      | InterfaceProperty
      | InterfaceEvent

InterfaceMethod
    ::= "fn" MethodSignature
      | "mut" "fn" MethodSignature
      | "->" "fn" MethodSignature
      | "static" "fn" MethodSignature

InterfaceProperty
    ::= "property" Identifier ":" TypeReference
        "{"
        InterfacePropertyAccessor { InterfacePropertyAccessor }
        "}"

InterfacePropertyAccessor
    ::= "get"
      | Contextual("set") Identifier
      | "try" Contextual("set") Identifier

InterfaceEvent
    ::= Contextual("event") Identifier "[" TypeReference "]"
```

Example:

```sec
interface Vehicle {
    fn Start() void
    fn Stop() void

    property IsRunning: bool {
        get
    }
}
```

Example:

```sec
interface PressSource {
    event ButtonPressed[ButtonPressData]
}
```

Interface methods have no body.

Interface property accessors have no body.

The setter identifier is mandatory in implementation, static, and interface
property forms. Sec does not provide an implicit setter-value binding.

---

# Impl declaration

```text
ImplDeclaration
    ::= "impl" [ "extends" ] TypeReference ImplBody

ImplBody
    ::= "{" { ImplMember } "}"

ImplMember
    ::= FunctionDeclaration
      | StaticFunctionDeclaration
      | InstanceLetDeclaration
      | StaticLetDeclaration
      | StaticPropertyDeclaration
      | PropertyDeclaration
      | EventDeclaration
      | NestedTypeDeclaration
      | NestedUnitDeclaration
      | NestedEnumDeclaration
      | InitDeclaration
      | FreeDeclaration
      | NestedImplDeclaration
      | UnitMetadataDeclaration

InstanceLetDeclaration
    ::= "let" [ "mut" ] Identifier [ ":" TypeReference ] ":=" Expression
```

`InstanceLetDeclaration` requires an instance; `mut` controls mutability.
`StaticLetDeclaration` requires explicit `static` and is type-owned. The two
forms are not equivalent; the AST and formatting retain the `static` modifier.

Lifecycle and construction syntax:

```text
InitDeclaration
    ::= "init" "(" [ ParameterList ] ")" [ TypeReference ] Block

FreeDeclaration
    ::= "free" Block

NestedImplDeclaration
    ::= "impl" TypeReference ImplBody

NewExpression
    ::= "new" TypeReference "(" [ ArgumentList ] ")"
```

The optional type after `init(...)` is a construction error type, not a return
type. `new Type(args...)` selects lifecycle construction; `Type(value)` remains
conversion syntax. Ordinary impl methods have compiler-provided implicit
`self` and do not declare receiver parameters.

Example:

```sec
impl Vehicle {
    fn Start() void {
    }

    property TopSpeed: Speed {
        get {
            return _speed
        }

        try set value {
            _speed = value
        }
    }
}
```

An additional same-module block is explicit:

```sec
impl extends Vehicle {
    fn Stop() void {
    }
}
```

Sema requires one primary `impl Vehicle` block and rejects extensions from a
different module. File order does not affect extension validity.

Invalid:

```sec
impl Interface for Type {
}
```

Invalid directly inside `impl`:

```text
stored fields
ordinary let declarations
executable statements
standalone struct declarations
```

Methods have implicit `self`.

---

# Nested type declarations

Nested declarations use the same canonical type syntax:

```sec
impl Vehicle {
    type Engine struct {
        power: Kilowatt,
    }

    enum FuelType {
        petrol,
        diesel,
        electric,
    }

    type Fuel union {
        Petrol
        Diesel
        Electric
    }
}
```

Outside the impl, nested names are qualified through the owner type.

---

# Property declaration

```text
PropertyDeclaration
    ::= "property" Identifier ":" TypeReference
        "{"
        PropertyAccessor { PropertyAccessor }
        "}"

PropertyAccessor
    ::= Getter
      | Setter
      | FallibleSetter

StaticPropertyDeclaration
    ::= "static" PropertyDeclaration

Getter
    ::= "get" Block

Setter
    ::= Contextual("set") Identifier Block

FallibleSetter
    ::= "try" Contextual("set") Identifier TypeReference Block
```

Example:

```sec
property TopSpeed: Speed {
    get {
        return _speed
    }

    try set value SpeedError {
        _speed = value
    }
}
```

A property must declare at least one accessor.

Duplicate getters or setters are invalid.

The setter parameter name is required.

`set` should ultimately be contextual.

---

# Event declaration

Inside `impl`:

```text
EventDeclaration
    ::= Contextual("event") Identifier
        Contextual("using") Identifier
```

Example:

```sec
event Pressed using buttonPressedStorage
```

An event declaration has no body; a block after it is invalid.

---

# Function declaration

```text
FunctionDeclaration
    ::= "fn" Identifier [ GenericParameterList ]
        ParameterList
        TypeReference
        FunctionBody

FunctionSignature
    ::= "fn" Identifier [ GenericParameterList ]
        ParameterList
        TypeReference

FunctionBody
    ::= Block
```

The return type and body are mandatory for an ordinary function declaration.
`FunctionSignature` is used only by constructs that explicitly permit a
bodyless callable contract, such as an interface requirement or foreign
`extern` declaration. It is not an ordinary prototype form.

Examples:

```sec
fn Add(left: int, right: int) int {
    return left + right
}
```

```sec
fn Noop() void {
    return
}
```

A return type is required.

Use `void` for no return value.

---

# Function parameters

```text
ParameterList
    ::= "(" [ Parameter { "," Parameter } [ "," ] ] ")"

Parameter
    ::= Identifier ":" TypeReference
      | "->" Identifier ":" TypeReference
      | Identifier ":" "..." TypeReference
```

Examples:

```sec
fn Inspect(value: Token) void {
}
```

```sec
fn Read(value: ref Token) void {
}
```

```sec
fn Update(value: ref mut Token) void {
}
```

`-> name: Type` is a forced-consuming by-value parameter. `name: ...Type` is
the final native typed variadic parameter. `->` cannot combine with `ref`,
`ref mut`, or `...`; a function has at most one variadic parameter and it must
be last.

`self` is implicit in methods.

Do not write it in canonical parameter lists.

---

# Function return values

```text
ReturnType
    ::= TypeReference
```

Sec 0.1 returns zero or one value:

- `void` means no value;
- every other return type means one value.

A function cannot declare a tuple-like multiple return list.

Use a named type when several related fields must be returned.

---

# Extern function declaration

```text
ExternFunctionDeclaration
    ::= [ LinkNameAnnotation ] [ "unsafe" ] "extern" StringLiteral ForeignFunctionSignature

ForeignFunctionSignature
    ::= "fn" Identifier "(" [ ForeignParameterList ] ")" TypeReference

ForeignParameterList
    ::= ParameterList
      | ParameterList "," "..."

LinkNameAnnotation
    ::= "@" "link_name" "(" StringLiteral ")"
```

Example:

```sec
extern "C" fn write(
    fd: int32,
    buffer: RawPtr[byte],
    length: uint,
) int64
```

An extern body is invalid. A bare final `...` is valid only for `unsafe extern
"C"` and is distinct from a native typed variadic parameter.

Foreign data declarations and types additionally use:

```text
ForeignTypeDeclaration
    ::= "extern" "C" "type" Identifier "struct" [ ForeignStructBody ]
      | "extern" "C" "type" Identifier "union" ForeignUnionBody
      | "extern" "C" "type" Identifier "enum" ForeignEnumBody

ForeignTypeReference
    ::= "C" "::" Identifier
      | "c" "::" Identifier { "::" Identifier }
      | "C" "::" "fn" "(" [ ForeignParameterTypeList ] ")" TypeReference
      | "C" "::" "flex" "[" TypeReference "]"
```

`bit[N]` after a field's C base type is a C bitfield declarator only inside an
`extern "C"` data declaration. `C::callback(expression)` is the explicit
environment-free callback adapter expression.

---

# Unsafe function declaration

```text
UnsafeFunctionDeclaration
    ::= "unsafe" FunctionDeclaration
      | "unsafe" ExternFunctionDeclaration
```

Example:

```sec
unsafe fn RawOperation(value: RawPtr[byte]) int {
    // ...
}
```

Unsafe syntax does not disable ordinary grammar, type, ownership, or scope
rules.

---

# Static declarations

```text
StaticDeclaration
    ::= StaticLetDeclaration
      | StaticFunctionDeclaration

StaticLetDeclaration
    ::= "static" LetDeclaration

StaticFunctionDeclaration
    ::= "static" FunctionDeclaration
```

Inside `impl`, `static` may modify `fn`, `let`, or `property`. Ordinary `fn`
members are instance-bound with compiler-provided implicit `self`; only
`static fn` is type-level and has no receiver. Static property setters use the
`StaticPropertyDeclaration` production defined with properties above and the
same mandatory explicit identifier as instance setters:

```text
"set" Identifier Block
"try" "set" Identifier Block
```

There is no implicit setter-value binding.

Inside `impl`, `StaticLetDeclaration` and `InstanceLetDeclaration` are
distinct. Canonical formatting must preserve the `static` modifier.

Compile-time static dependency order belongs to `rules/declarations/static.md`.
Executable startup and shutdown planning belong to
`rules/compiler/initialization.md`.

---

# Addressed declaration

```text
AddressedLetDeclaration
    ::= "@" Contextual("address") "(" Expression ")" LetDeclaration
```

Example:

```sec
@address(0x40021000)
let mut motorProtocol: MotorProtocol
```

The annotation may not apply to a grouped `let` declaration.

The exact address must be validated semantically.

This is a special grammar form until general attributes exist.

---

# Let declarations

```text
LetDeclaration
    ::= "let" [ "mut" ]
        LetDeclarator
        { "," LetDeclarator }

LetDeclarator
    ::= Identifier
        [ ":" TypeReference ]
        [ LetInitializer ]

LetInitializer
    ::= ":=" Expression
      | ":<-" Expression
      | "<-" Expression
```

The permitted initializer spelling depends on whether the type is explicit.

Canonical combinations:

```text
inferred copy/direct construction
    let name := expression

inferred explicit move
    let name :<- expression

typed copy/direct construction
    let name: Type := expression

typed explicit move
    let name: Type <- expression
```

Invalid combinations:

```sec
let name: Type :<- source
let name <- source
let name = source
```

A mutable typed declaration may omit the initializer when the type is
defaultable.

An immutable declaration must provide an initializer.

A grouped declaration declares independent bindings. It is not multiple-result
destructuring: `let literal, tokenType := self.readNumber()` is invalid.

---

# Type-first declarations

```text
TypedDeclaration
    ::= TypeReference [ "mut" ] ":"
        TypedDeclarator
        { "," TypedDeclarator }

TypedDeclarator
    ::= Identifier [ TypedInitializer ]

TypedInitializer
    ::= ":=" Expression
      | "<-" Expression
```

Examples:

```sec
int mut: a, b, c
```

```sec
float: a := 5.4, pi := 3.14
```

Immutable declarators require initializers.

Mutable declarators may omit them only when the declared type is defaultable.

Contracts do not appear in the canonical variable grammar.

---

# Parenthesized type-first group

```text
TypedDeclarationGroup
    ::= TypeReference
        "("
        TypedGroupDeclarator
        { "," TypedGroupDeclarator }
        [ "," ]
        ")"

TypedGroupDeclarator
    ::= Identifier ":=" Expression
```

Example:

```sec
TokenType (
    ILLEGAL := "ILLEGAL",
    EOF := "EOF",
    IDENT := "IDENT",
)
```

This form is immutable.

Every entry requires an initializer.

---

# Assignment statements

```text
AssignmentStatement
    ::= PlaceExpression AssignmentOperator Expression

AssignmentOperator
    ::= "="
      | "<-"
      | "+="
      | "-="
      | "*="
      | "/="
      | "%="
      | "&="
      | "|="
      | "^="
      | "<<="
      | ">>="
```

Assignment is a statement.

It does not produce a value.

Chained assignment is invalid.

The target is evaluated exactly once.

---

# Try assignment

```text
TryAssignmentStatement
    ::= "try" PlaceExpression AssignmentOperator Expression [TryHandlerBlock]
```

Example:

```sec
try percent += Percent(value) {
    Err(error) => {
        discard error
    }
}
```

Without a handler block, a failed setter result is propagated as an immediate
`return Err(error)` from the enclosing function. The enclosing return type must
be `Result[_, E]` with the exact setter error type `E`; this form does not
propagate `Option` and performs no implicit error conversion. With a handler
block, the ordinary local-handler rules apply.

---

# Statement grammar

```text
Statement
    ::= LetDeclaration
      | TypedDeclaration
      | TypedDeclarationGroup
      | AssignmentStatement
      | TryAssignmentStatement
      | ExpressionStatement
      | ReturnStatement
      | IfStatement
      | ForStatement
      | WhileStatement
      | SwitchStatement
      | MatchStatement
      | SelectStatement
      | BreakStatement
      | ContinueStatement
      | FallthroughStatement
      | DeferStatement
      | DiscardStatement
      | DetachStatement
      | CancelStatement
      | UnsafeStatement
      | AsmStatement
      | PanicStatement
      | AssertStatement
      | UnreachableStatement
      | IncrementDecrementStatement
      | StaticDeclaration
      | Comment

PanicStatement
    ::= "panic" [ StringLiteral ]

AssertStatement
    ::= "assert" Expression [ "," StringLiteral ]

UnreachableStatement
    ::= Contextual("unreachable")

IncrementDecrementStatement
    ::= PlaceExpression ( "++" | "--" )
```

Context restricts which statements are valid.

`panic` is a statement, not an ordinary callable, so `panic("message")` is
invalid; the optional message is an ordinary string literal. Assertions and
checked `unreachable` are defined by `rules/errors/panic.md` §§ 15–16.

`value++` and `value--` are statement-only aliases for `value += 1` and
`value -= 1` (`rules/foundations/operators.md`); they never produce a value.

---

# Block

```text
Block
    ::= "{" { Statement } "}"
```

Braces are required.

Sec does not use indentation as block syntax.

Comments may occur between statements.

---

# Statement termination

Canonical Sec source does not require semicolons after ordinary statements.

A statement normally ends through grammar completion and token position.

Line layout is relevant in selected forms, including:

- return without a value;
- comma-free struct literal fields;
- comma-free enum values;
- comma-free register fields;
- unit declarations;
- typed declaration-group recognition.

General semicolon-separated statement syntax is not part of Sec 0.1.

A semicolon in a `for` header is diagnosed as attempted C-style syntax.

---

# Expression statement

```text
ExpressionStatement
    ::= Expression
```

An expression statement is valid only when its result and effects may be
discarded according to Sema.

Must-use values require an appropriate consuming context or explicit discard.

---

# Return statement

```text
ReturnStatement
    ::= "return" [ [ "<-" ] Expression ]
```

Examples:

```sec
return
```

```sec
return value
```

```sec
return <- value
```

Both forms are grammatical. Returning an owned value transfers it according to
ownership semantics; the marker is optional at this terminal boundary.

---

# If statement

```text
IfStatement
    ::= "if" Expression Block
        { "else" "if" Expression Block }
        [ "else" Block ]
```

Examples:

```sec
if ready {
    Start()
}
```

```sec
if value in 0..<10 {
} else if value < 0 {
} else {
}
```

The condition must be `bool`.

Braces are required.

---

# Infinite for loop

```text
InfiniteForStatement
    ::= "for" Block
```

Example:

```sec
for {
    break
}
```

---

# Iterable for loop

```text
IterableForStatement
    ::= "for"
        ForBinding { "," ForBinding }
        "in"
        IterableExpression
        [ Contextual("step") Expression ]
        Block

ForBinding
    ::= Identifier
      | "_"
      | "ref" Identifier
      | "ref" "mut" Identifier
```

Examples:

```sec
for item in values {
}
```

```sec
for index, item in values {
}
```

```sec
for i in 0..<10 step 2 {
}
```

The number and meaning of bindings depend on the iterable.

The grammar accepts plain, shared-reference, mutable-reference, and discard
bindings. Semantic analysis determines whether a binding mode is valid for the
iterable category and position. Sequential indices, range values, and decoded
string runes do not accept reference binding modes. Map keys and set elements
do not accept `ref mut`.

Sec 0.1 does not add `for -> item in source`, `for item in move source`, or
another consuming-loop grammar form.

`step` is valid for approved range iteration.

Condition-only `for` is invalid.

C-style `for` is invalid.

---

# While statement

```text
WhileStatement
    ::= "while" Expression Block
```

Example:

```sec
while ready {
}
```

The condition must be `bool`.

Assignment in the condition is diagnosed.

---

# Break and continue

```text
BreakStatement
    ::= "break"

ContinueStatement
    ::= "continue"
```

They are valid inside loops.

Labeled break and continue are not part of Sec 0.1.

---

# Switch statement

```text
SwitchStatement
    ::= "switch" [ Expression ]
        "{"
        { SwitchCase }
        [ SwitchDefault ]
        "}"

SwitchCase
    ::= "case" SwitchCaseItem { "," SwitchCaseItem } ":" { Statement }

SwitchDefault
    ::= "default" ":" { Statement }

SwitchCaseItem
    ::= Expression
      | RelationalSwitchCase
      | RangeExpression

RelationalSwitchCase
    ::= ( "<" | "<=" | ">" | ">=" ) Expression
```

Examples:

```sec
switch value {
case < 0:
    return
case 0, 1, 2..<10:
    fallthrough
default:
    return
}
```

Subjectless example:

```sec
switch {
case value < 0:
    return
default:
    return
}
```

Use commas between several case values.

`||` creates one boolean expression and is not the separator.

`default` must be unique and final.

---

# Fallthrough

```text
FallthroughStatement
    ::= "fallthrough"
```

It is valid only directly inside a switch case body.

It is not a general jump statement.

---

# Match expression

```text
MatchExpression
    ::= "match" Expression
        "{"
        MatchArm { MatchArm }
        "}"

MatchArm
    ::= MatchPattern
        [ "where" Expression ]
        "=>"
        MatchArmBody

MatchArmBody
    ::= Expression
      | ReturnStatement
      | Block
```

Examples:

```sec
let value := match result {
    Ok(value) => value
    Err(error) => return Err(error)
}
```

In expression-match result position, a block arm contextually produces its
value from the final expression on every continuing path. Terminating paths do
not require an arm value. This is not general block-expression syntax.

---

# Match statement

```text
MatchStatement
    ::= MatchExpression
```

A match may be used for effects when arm values are not required.

---

# Match patterns

Canonical initial pattern grammar:

```text
MatchPattern
    ::= "_"
      | "empty"
      | Identifier
      | QualifiedIdentifier
      | CallPattern
      | StructLikeUnionPattern

CallPattern
    ::= QualifiedIdentifier "(" [ PatternArgument ] ")"

PatternArgument
    ::= Identifier
      | "_"
      | "ref" Identifier
      | "ref" "mut" Identifier

StructLikeUnionPattern
    ::= QualifiedIdentifier
        "{"
        [ FieldPattern { "," FieldPattern } [ "," ] ]
        "}"

FieldPattern
    ::= Identifier
      | Identifier ":" Identifier
      | Identifier ":" "ref" Identifier
      | Identifier ":" "ref" "mut" Identifier
```

The parser may read a pattern through the ordinary expression grammar.

Sema restricts valid patterns.

General literal, range, and direct `true` / `false` patterns are not part of
Sec 0.1 `match`; use `switch` or `if`. Enum-member patterns remain valid
resolved finite-domain patterns. Struct-like destructuring is shallow, allows
partial binding without `..`, and does not introduce recursive patterns.

Examples:

```sec
Ok(value)
Err(error)
Option.Some(value)
Option.None
Color.red
empty
Rectangle { width, height: h }
_
```

---

# Defer statement

```text
DeferStatement
    ::= "defer" Block
```

Examples:

```sec
defer {
    Close()
}
```

Control transfer from inside a defer block is restricted.

---

# Discard statement

```text
DiscardStatement
    ::= "discard" Expression
```

Example:

```sec
discard result
```

Discard is explicit consumption and early deterministic destruction according
to `discard.md`.

---

# Detach statement

```text
DetachStatement
    ::= Contextual("detach") Expression [ "discard" ]
```

Examples:

```sec
detach task
```

```sec
detach task discard
```

`detach` is a contextual spelling.

---

# Cancel statement

```text
CancelStatement
    ::= "cancel"
```

The current AST form has no explicit operand.

Its meaning depends on the current cancellable context.

Complete cancellation semantics belong to `cancellation.md`.

---

# Select statement

```text
SelectStatement
    ::= "select" "{"
        { SelectBranch }
        "}"

SelectBranch
    ::= SelectOperationBranch
      | SelectBindingBranch
      | SelectTimeoutBranch
      | SelectDefaultBranch

SelectOperationBranch
    ::= Expression "=>" Block

SelectBindingBranch
    ::= Identifier ":=" Expression "=>" Block

SelectTimeoutBranch
    ::= "after" Expression "=>" Block

SelectDefaultBranch
    ::= "default" "=>" Block
```

Example:

```sec
select {
    value := rx.Receive() => {
        discard value
    }

    tx.Send(1) => {
    }

    result := await task => {
        discard result
    }

    after 10 => {
    }

    default => {
    }
}
```

`default` must be unique and final.

A timeout after an unconditional default is unreachable.

---

# Unsafe block

```text
UnsafeStatement
    ::= "unsafe" Block
```

Example:

```sec
unsafe {
    asm "nop"
}
```

The block does not change the grammar of contained statements.

---

# Assembly statement

Simple forms:

```text
AsmStatement
    ::= "asm" StringLiteral
      | "asm" "(" StringLiteral ")"
      | StructuredAsm
```

Structured form:

```text
StructuredAsm
    ::= "asm" "{"
        StringLiteral
        { AsmSection }
        "}"

AsmSection
    ::= AsmInputs
      | AsmOutputs
      | AsmClobbers

AsmInputs
    ::= Contextual("inputs") ":"
        { AsmInput [ "," ] }

AsmInput
    ::= Identifier "(" Expression ")"

AsmOutputs
    ::= Contextual("outputs") ":"
        { AsmOutput [ "," ] }

AsmOutput
    ::= Identifier
      | Identifier "(" Identifier ")"

AsmClobbers
    ::= Contextual("clobbers") ":"
        { Identifier [ "," ] }
```

Example:

```sec
asm {
    "syscall"

    inputs:
        rax(number)
        rdi(arg1)

    outputs:
        rax(result)

    clobbers:
        rcx
        r11
        memory
}
```

The complete operand and constraint language belongs to
`rules/platform/inline_assembly.md`.

---

# Expression grammar

The canonical expression grammar is precedence-driven.

This document defines expression forms.

`operators.md` defines precedence, associativity, evaluation order, and operator
semantics.

Conceptual grammar:

```text
Expression
    ::= PrimaryExpression
        { PostfixContinuation | InfixContinuation }
```

The parser may implement this with Pratt parsing.

---

# Primary expressions

```text
PrimaryExpression
    ::= IdentifierExpression
      | Literal
      | GroupedExpression
      | ArrayLiteral
      | LambdaExpression
      | CaptureLambdaExpression
      | MatchExpression
      | TryExpression
      | SpawnExpression
      | AwaitExpression
      | RefExpression
      | MoveExpression
      | RuntimeCallExpression
      | PrefixExpression
```

---

# Identifiers and self

```text
IdentifierExpression
    ::= Identifier
      | "self"
```

`self` is valid in an instance implementation context.

The parser accepts it as an expression token.

Sema determines whether the context provides an instance.

---

# Literals

```text
Literal
    ::= IntegerLiteral
      | FloatingLiteral
      | StringLiteral
      | InterpolatedStringLiteral
      | CharacterLiteral
      | BooleanLiteral
```

Numeric suffixes, bases, escapes, and literal token boundaries belong to
`lexical_structure.md` and `types.md`.

---

# Grouped expression

```text
GroupedExpression
    ::= "(" Expression ")"
```

Parentheses override operator precedence.

---

# Array literal

```text
ArrayLiteral
    ::= "[" [ ArrayElement { "," ArrayElement } [ "," ] ] "]"

ArrayElement
    ::= Expression
      | Expression "..."
      | RangeSegment

RangeSegment
    ::= Expression RangeOperator Expression
```

Examples:

```sec
[1, 2, 3]
```

```sec
[first..., second...]
```

```sec
[224r..246r, 248r..255r, 0x41r]
```

The empty literal `[]` requires contextual type information.

Array and collection ownership rules apply to spread elements.

A `RangeSegment` contributes every value from its lower to its upper bound and
requires both bounds; its semantics are defined by
`rules/collections/collections.md` § 5.6a.

---

# Empty list literal

Canonical:

```text
EmptyListLiteral
    ::= ListTypeReference "{" "}"
```

Examples:

```sec
list[string] {}
list[Packet, 32] {}
```

It is not a struct literal.

---

# Lambda expression

```text
LambdaExpression
    ::= CaptureClause? "fn" ParameterList TypeReference Block
```

Example:

```sec
let double := fn(value: int) int {
    return value * 2
}
```

The return type is required.

---

# Capture clause

```text
CaptureClause
    ::= "capture" "(" [ CaptureEntry { "," CaptureEntry } [ "," ] ] ")"

CaptureEntry
    ::= Identifier
      | "<-" Identifier
      | "ref" Identifier
      | "ref" "mut" Identifier
```

`capture(<-value)` is the consuming capture form. The legacy
`capture(-> value)` spelling is not grammatical.

Example:

```sec
let closure := capture(value) fn(input: int) int {
    return value + input
}
```

The lambda introducer itself is always plain `fn`. Generic parameter lists,
`mut fn`, and `-> fn` are not lambda-expression introducers. `mut fn` and
`-> fn` are callable-type capability syntax.

---

# Call expression

```text
CallExpression
    ::= Expression "(" [ CallArgument { "," CallArgument } [ "," ] ] ")"

CallArgument
    ::= Expression
      | "_"
      | Expression "..."
```

Examples:

```sec
Add(1, 2)
```

```sec
object.Method(value)
```

```sec
Call(values...)
```

Sema resolves whether the callee is:

```text
function
method
conversion
constructor
union variant constructor
compiler-known callable
```

---

# Explicit generic call

```text
ExplicitGenericCall
    ::= CallableExpression TypeArgumentList
        "(" [ CallArgument { "," CallArgument } [ "," ] ] ")"
```

Examples:

```sec
Identity[int](10)
pkg.Make[Box[string]]("hello")
```

The bracket must be attached and followed by a call context.

Otherwise the syntax is indexing.

For generic function and method calls, the explicit type arguments may be a
positional prefix of the declaration's parameters. Remaining parameters are
inferred from arguments, receiver, and permitted expected-result context.
Generic argument holes such as `Foo[_, B](...)` or `Foo[, B](...)` are invalid.
Generic type references require their complete type argument list.

---

# Type conversion expression

Ordinary conversion surface:

```text
ConversionExpression
    ::= TypeReference "(" Expression ")"
```

Examples:

```sec
Percent(value)
decimal(integer)
bool(number)
```

The parser initially creates call-shaped syntax.

Sema resolves a type conversion.

---

# Unit conversion expression

```text
UnitConversionExpression
    ::= NumericTypeReference UnitAnnotation
        "(" Expression ")"
```

Example:

```sec
decimal<C>(decimal(Amp) * decimal(Second))
```

The parser must distinguish:

```sec
left < right
```

from:

```sec
decimal<m>(value)
```

through type and token context.

---

# Member access

```text
MemberExpression
    ::= Expression "." Identifier
```

Examples:

```sec
vehicle.TopSpeed
Color.red
module.Function
self.data
```

Member access may continue into calls, generic calls, indexing, or other member
access.

---

# Index expression

```text
IndexExpression
    ::= Expression "[" Expression "]"
```

Example:

```sec
values[index]
```

The indexed type determines whether the result is:

```text
value
place
reference-like access
map lookup result
compiler-known indexed value
```

---

# Slice expression

```text
SliceExpression
    ::= Expression
        "["
        [ Expression ]
        RangeOperator
        [ Expression ]
        "]"
```

Examples:

```sec
values[2..<8]
values[..<8]
values[2..]
```

Range inclusivity follows the range operator.

---

# Struct or union typed construction

```text
TypedConstruction
    ::= TypeReference StructLiteralBody
```

Sema resolves:

```text
struct literal
struct-like union variant construction
compiler-known typed collection literal
other approved typed construction
```

A non-constructible type is a semantic error.

---

# Prefix expressions

```text
PrefixExpression
    ::= PrefixOperator Expression

PrefixOperator
    ::= "+"
      | "-"
      | "!"
      | "~"
```

Unary `+` is parsed and validated for numeric operands. Constant integer
expressions fold it without changing the operand value.

---

# Reference expression

```text
RefExpression
    ::= "ref" [ "mut" ] PlaceExpression
```

---

# Move expression

```text
MoveExpression
    ::= "<-" PlaceExpression
```

A move expression is accepted in owning expression positions such as call
arguments and constructor, aggregate, `Option`, `Result`, and union payloads.
Grammar acceptance does not decide whether the source is movable or the target
consuming; Sema applies the ownership rules.

Examples:

```sec
ref value
ref mut value
ref mut self.data[0]
```

A reference requires a valid addressable place.

`ref Type[]` in type position is a slice type.

`ref expression` in expression position creates a borrow.

---

# Try expression

```text
TryExpression
    ::= "try" Expression [ TryHandlerBlock ]

TryHandlerBlock
    ::= "{" TryHandler { TryHandler } "}"

TryHandler
    ::= TryPattern [ "where" Expression ] "=>" TryHandlerBody

TryPattern
    ::= ErrPattern
      | "None"

TryHandlerBody
    ::= Expression
      | ReturnStatement
      | Block
```

Examples:

```sec
let value := try Calculate()
```

```sec
let value := try Calculate() {
    Err(IOError.InvalidValue) => 0
    Err(error) => return Err(error)
}
```

The protected expression determines the legal handler family. Result/error
`try` accepts `Err(...)`; Option `try` accepts `None`. Explicit `Ok(...)` and
`Some(...)` success handlers and the obsolete nested `match { ... }` wrapper
are invalid. Handler lists are semantically partial; unmatched states propagate
when compatible.

---

# Spawn expression

```text
SpawnExpression
    ::= "spawn" [ SpawnKind ] Expression
      | "spawn" [ SpawnKind ] Block

SpawnKind
    ::= Contextual("task")
      | Contextual("thread")
      | Contextual("process")
```

Examples:

```sec
spawn Work()
spawn task Work()
spawn thread Work()
spawn {
    Work()
}
```

Default kind is `task`.

`process` syntax is parsed but the feature is deferred.

---

# Await expression

```text
AwaitExpression
    ::= "await" Expression
```

Example:

```sec
let result := await task
```

---

# Runtime or compiler call expression

```text
RuntimeCallExpression
    ::= "@" DottedIdentifier
        "(" [ CallArgument { "," CallArgument } [ "," ] ] ")"
```

Example shape:

```sec
@compiler.operation(value)
```

This form must not be confused with general attributes.

The set of valid names is compiler-controlled.

---

# Spread expression

```text
SpreadExpression
    ::= Expression "..."
```

Spread is valid only in approved contexts.

Sec 0.1 approved contexts are governed by `declarations/spread.md`, including:

```text
call arguments
array literals
struct literals
```

Fixed-arity calls and fixed-array literals require every spread contribution to
have a compile-time-known expansion count. A runtime-length sequence may be
spread only when the destination's canonical rule defines runtime arity.

`expression...` is non-consuming syntax. It does not imply a move or partial
move.

For struct literals, explicit entries and spreads are resolved from left to
right before semantic defaults are supplied for still-omitted Defaultable
fields. A still-omitted NonDefaultable field is invalid. Canonical construction
rules are defined by `declarations/struct.md`.

Spread is not a general standalone value operator.

---

# Range expression in contextual positions

```text
RangeExpression
    ::= [ Expression ] RangeOperator [ Expression ]
```

Range expressions are accepted only where the surrounding grammar explicitly
permits them: `for` iteration, membership, slicing, switch cases, array
literal range segments, and the argument of `Append` on an owning dynamic
array (`rules/collections/collections.md` §§ 5.6a and 6.7).

Examples:

```sec
0..<10
..100
0..
```

A standalone variable initializer does not create a first-class `Range` value in
Sec 0.1.

---

# Infix expressions

```text
InfixExpression
    ::= Expression InfixOperator Expression
```

The complete operator inventory and precedence are defined by `operators.md`.

The grammar includes:

```text
arithmetic
bitwise
shift
comparison
equality
membership
logical
contextual matrix multiplication
```

The parser recognizes contextual `x` at multiplicative precedence. Sema
resolves fixed matrix/matrix and matrix/vector result shapes and rejects
incompatible inner dimensions or element types.

---

# Membership expression

```text
MembershipExpression
    ::= Expression "in" MembershipSource

MembershipSource
    ::= RangeExpression
      | FixedArrayExpression
      | SliceExpression
```

The parser accepts a range-or-expression right operand.

`for value in source` is iteration grammar, not this boolean operator.

---

# Assignment is not expression syntax

The following are statements:

```sec
destination = source
destination <- source
destination += source
```

They cannot occur where a value expression is required.

Invalid:

```sec
let result := destination = source
```

---

# Type-reference grammar

Canonical overview:

```text
TypeReference
    ::= ReferenceType
      | FunctionType
      | UnitOnlyType
      | ParenthesizedType
      | NamedTypeReference TypeSuffixes

NamedTypeReference
    ::= QualifiedIdentifier [ UnitAnnotation ]

QualifiedIdentifier
    ::= Identifier { "." Identifier }

TypeSuffixes
    ::= { TypeSuffix }

TypeSuffix
    ::= FixedArraySuffix
      | DynamicArraySuffix
      | GenericOrCollectionArguments
```

---

# Reference type

```text
ReferenceType
    ::= "ref" [ "mut" ] TypeReference
```

Examples:

```sec
ref int
ref mut Token
ref byte[]
ref mut rune[]
```

A bare safe slice is represented through reference syntax.

---

# Function type

```text
FunctionType
    ::= [ CallableCapability ] "fn"
        "(" [ CallableParameterType { "," CallableParameterType } [ "," ] ] ")"
        TypeReference

CallableCapability
    ::= "mut"
      | "->"

CallableParameterType
    ::= TypeReference
      | "->" TypeReference
      | "..." TypeReference
```

Example:

```sec
fn(int) bool
mut fn() int
-> fn(-> Resource) Handle
```

Parameter names do not appear in function type references.

---

# Unit annotation

```text
UnitAnnotation
    ::= "<" UnitExpression ">"

UnitExpression
    ::= UnitProduct

UnitProduct
    ::= UnitFactor { ( "*" | "/" ) UnitFactor }

UnitFactor
    ::= UnitAtom [ "^" SignedIntegerConstant ]

UnitAtom
    ::= QualifiedIdentifier
      | "1"
      | "(" UnitExpression ")"
```

Examples:

```sec
decimal<m>
decimal<m/s>
Speed<km/h>
```

The parser preserves source spelling. Semantic normalization determines
dimensional equivalence and does not make `<mps>` a syntactic alias for
`<m/s>`.

---

# Unit-only type reference

```text
UnitOnlyType
    ::= UnitAnnotation
```

`<NamedUnit>` uses the named unit's declared default numeric carrier, or
`decimal` when none is declared. A compound structural unit-only type such as
`<m/s>` always defaults to `decimal`; an explicit carrier may be written as
`float64<m/s>` or `decimal<m/s>`.

---

# Fixed array type

Canonical:

```text
FixedArrayType
    ::= TypeReference "[" ConstantExpression "]"
```

Examples:

```sec
byte[512]
Token[16]
matrix[float32, 4, 4][2]
```

The length must be a valid compile-time nonnegative integer.

---

# Owning dynamic array type

Canonical:

```text
OwningDynamicArrayType
    ::= TypeReference "[" "]"
```

Example:

```sec
rune[]
```

This is an owning dynamic sequence type in the current type model.

A slice is:

```sec
ref rune[]
```

or:

```sec
ref mut rune[]
```

---

# Prefix sequence types are invalid

The prefix spellings `[N]Type` and `[]Type` were never normative Sec syntax.
They are not Sec 0.1 grammar and not legacy forms; the canonical spellings are
`Type[N]` and `Type[]`.

---

# Generic type arguments

```text
TypeArgumentList
    ::= "[" TypeReference { "," TypeReference } [ "," ] "]"
```

Examples:

```sec
Result[int, IOError]
Option[string]
RawPtr[byte]
Pair[int, string]
```

Not every square-bracket list contains only types.

Compiler-known collection and shaped constructors have mixed type and constant
arguments.

---

# Collection and shaped types

Canonical families:

```text
ListType
    ::= Contextual("list")
        "[" TypeReference [ "," ConstantExpression ] "]"

MapType
    ::= Contextual("map")
        "[" TypeReference "," TypeReference
        [ "," ConstantExpression ] "]"

SetType
    ::= Contextual("set")
        "[" TypeReference [ "," ConstantExpression ] "]"

VectorType
    ::= Contextual("vector")
        "[" TypeReference "," ConstantExpression "]"

MatrixType
    ::= Contextual("matrix")
        "[" TypeReference "," ConstantExpression "," ConstantExpression "]"

TensorType
    ::= Contextual("tensor")
        "[" TypeReference { "," ConstantExpression } "]"

TensorViewType
    ::= Contextual("tensor_view")
        "[" TypeReference "," ConstantExpression "]"

ShapeType
    ::= "Shape" "[" ConstantExpression "]"

StridesType
    ::= "Strides" "[" ConstantExpression "]"

TensorLayoutType
    ::= "TensorLayout" "[" ConstantExpression "]"
```

The parser recognizes these names specially to separate type arguments from
constant arguments.

Complete dimension and capacity validation belongs to Sema.

---

# Event types

```text
EventType
    ::= "Event" "[" TypeReference [ "," IntegerConstant ] "]"

EventStorageType
    ::= "EventStorage" "[" TypeReference [ "," IntegerConstant ] "]"
```

Examples:

```sec
Event[ButtonPressData, 8]
EventStorage[ButtonPressData, 8]
```

Capacity is compile-time known and greater than zero
(`rules/concurrency/events.md`).

---

# Parenthesized type

```text
ParenthesizedType
    ::= "(" TypeReference ")"
```

This is grouping.

It does not create a tuple type.

---

# Type versus expression ambiguity

The parser must distinguish:

```sec
Type(value)
```

from a function call.

Sema resolves the callee.

The parser must distinguish:

```sec
Name[Type](value)
```

from indexing.

The parser must distinguish:

```sec
Type { ... }
```

from a following block.

The parser must distinguish:

```sec
decimal<m>(value)
```

from comparison.

The parser may use lookahead, but the semantic result must match this grammar.

---

# Contextual spellings

The following spellings are contextual or intended to become contextual:

```text
x
set
event
using
detach
step
register
bit
physical
currency
other
inputs
outputs
clobbers
address
```

A contextual spelling remains an ordinary identifier outside its context.

`task`, `thread`, and `process` (`lexical_structure.md` § 7.2) and the contract
words (§ 7.3) are also interpreted by context, but they are reserved and are
never valid declaration names.

The grammar context determines its role.

---

# Contextual `set`

`set` is contextual (`lexical_structure.md` § 7.2):

- collection type constructor in type position;
- property setter introducer in property position;
- otherwise an ordinary identifier where unambiguous.

---

# Comments

The lexer recognizes comments.

Documentation comments and attachment rules must remain synchronized with
`lexical_structure.md`, formatter rules, and future documentation tooling.

Comments do not terminate or combine tokens except through normal lexical
separation.

---

# Reserved syntax

`?` is reserved and has no Sec 0.1 meaning.

`free` is valid only as the `FreeDeclaration` impl member.

`panic` and `assert` are statement keywords whose forms are defined above and by
`rules/errors/panic.md`; neither is callable.

`require` is a general keyword (`lexical_structure.md` § 7.1) and is not a
valid identifier.

The parser issues focused diagnostics for these spellings rather than generic
token failures.

---

# Invalid forms

The following are explicitly invalid in canonical Sec 0.1:

```sec
let immutable: Type
```

```sec
let typed: Type :<- source
```

```sec
let inferred <- source
```

```sec
condition ? left : right
```

```sec
for condition {
}
```

```sec
for i := 0; i < 10; i += 1 {
}
```

```sec
impl Interface for Type {
}
```

```sec
let range := 0..<10
```

```sec
return first, second
```

```sec
type User struct {
    age: int range 0..130,
}
```

```sec
value = other = third
```

---

# Legacy, future, and recovery syntax

The parser may recognize the following invalid Sec 0.1 forms only to issue a
focused diagnostic or to recover; none of them is accepted Sec 0.1 source:

```text
type Name = ExistingType          legacy; migration diagnostic
type Error = First Second Third   legacy; migration diagnostic
inline field/variable contracts   legacy; migration diagnostic
enum value initializer Value: 1   legacy; migration diagnostic
explicit ref self parameter       legacy; migration diagnostic
explicit nested try match wrapper legacy; migration diagnostic
struct Name { ... }               Sec 0.2 syntax; not valid in Sec 0.1
prefix array syntax [N]Type       never normative; invalid
prefix sequence syntax []Type     never normative; invalid
body-bearing extern declaration   focused rejection and recovery
```

Recognized forms must be marked in AST or diagnostics where needed. Default
formatting does not convert any of them; an enabled Language Correction may
rewrite a legacy form only under the policy in "Canonical syntax and legacy
forms" above (MD-003).

---

# Parser responsibilities

The parser must:

- construct a stable AST;
- preserve source ranges and tokens;
- distinguish declarations from expressions;
- distinguish types from indexes and generic calls;
- preserve explicit copy versus move spelling;
- preserve contextual ranges;
- preserve omitted struct fields as omissions before Sema completion;
- preserve comments required by formatter and documentation tooling;
- report missing required delimiters;
- recover at stable declaration and statement boundaries;
- avoid performing type resolution;
- avoid performing ownership resolution;
- avoid performing contract evaluation;
- avoid choosing a function overload;
- avoid target lowering decisions.

---

# AST responsibilities

The AST must represent:

- source spelling;
- declaration kind;
- type references;
- generic type and constant arguments;
- contracts;
- explicit defaults;
- copy versus move ownership mode;
- statements;
- expressions;
- patterns;
- source comments and tags;
- invalid or recovery nodes;
- tokens needed for diagnostics;
- contextual syntax before semantic resolution.

The AST may contain parser-level ambiguous nodes where Sema decides the meaning.

The current recovery AST retains invalid statements, failed top-level
declarations, disallowed impl members, malformed match/handler patterns,
selected invalid expressions, and selected invalid type references with
syntax-only recovery metadata. These nodes never confer valid semantics and
batch lowering remains blocked by parser errors.

Example:

```text
CallExpression
    may later resolve to function call, method call, conversion, or constructor
```

---

# Sema responsibilities

Sema must determine:

- name resolution;
- module and scope validity;
- type resolution;
- defaultability;
- contract validity;
- explicit default validity;
- declaration initialization;
- ownership and move semantics;
- reference validity;
- function overload selection;
- call versus conversion versus constructor meaning;
- expression types;
- assignment validity;
- iterable behavior;
- switch compatibility;
- match pattern validity and exhaustiveness;
- interface conformance;
- struct field completion;
- union variant construction;
- operator validity;
- contextual `x`;
- collection literal category;
- addressability;
- unsafe requirements;
- target restrictions where semantically necessary.

Sema must not depend on backend accidents such as undefined aggregate fields.

---

# Formatter responsibilities

The formatter must print canonical syntax.

Default formatting does not convert legacy, future, or otherwise invalid syntax
into canonical syntax. Such rewrites belong to explicitly enabled Language
Corrections (`rules/tooling/formatter.md` §§ 26–27), which apply only when the
rewrite is unambiguous and semantics-preserving, for example:

```text
func -> fn
Value: 1 -> Value = 1 inside enum declaration
type Name = ExistingType -> type Name ExistingType
prefix array type -> postfix array type
```

It must preserve:

```text
:= versus :<-
= versus <-
explicit default clauses
in-list order
omitted struct fields
contextual identifier use
```

The formatter must not infer language semantics.

---

# Required parser tests

The canonical parser test suite must cover:

```text
target directive
module declaration
single and grouped imports
every top-level declaration
nested impl declarations
every type-reference form
every variable declaration form
copy and move initialization
copy and move assignment
compound assignment
default clauses
contracts
struct omission
struct spread
enum recovery syntax
union variants
register fields
interface members
properties
events
function signatures
extern and unsafe functions
all statement forms
all expression prefix forms
all postfix forms
all operator precedence pairs
match patterns
try handlers
select branches
asm sections
contextual spellings
invalid reserved syntax
recovery boundaries
```

---

# Required grammar fixtures

Create:

```text
grammar_valid.sec
grammar_invalid.sec
grammar_declarations_valid.sec
grammar_declarations_invalid.sec
grammar_types_valid.sec
grammar_types_invalid.sec
grammar_statements_valid.sec
grammar_statements_invalid.sec
grammar_expressions_valid.sec
grammar_expressions_invalid.sec
grammar_contextual_valid.sec
grammar_contextual_invalid.sec
grammar_recovery.sec
```

Every invalid fixture must include:

```sec
/* Expected error: ...
 * Reason: ...
 */
```

---

# Required synchronization

This document must remain synchronized with:

```text
lexical_structure.md
operators.md
formatter.md
tooling/testing.md
default_values.md
types.md
contracts.md
types/units.md
struct.md
enums.md
unions.md
declarations/registers.md
functions.md
declarations/lambda-functions.md
declarations/generics.md
declarations/interfaces.md
impl.md
properties.md
events.md
collections.md
shaped-types.md
declarations/spread.md
flowcontrol_if.md
flowcontrol_for.md
flowcontrol_while.md
flowcontrol_switch.md
flowcontrol_match.md
errorhandling.md
defer.md
discard.md
ownership.md
copy_move.md
references.md
raw_pointers.md
unsafe.md
inline_assembly.md
spawn.md
await.md
select.md
cancellation.md
diagnostics.md
parser_recovery.md
semantic_ir.md
language-rulebook-status.md
rules_implementations.txt
```

Planned files remain references to future canonical closure work.

---

# Design summary

Sec source files declare a module and contain declarations.

Declarations use explicit typed syntax.

Named types carry contracts and optional explicit defaults.

Structs declare data.

Impl blocks declare behavior.

Interfaces declare requirements.

Functions return one value.

Variables are immutable by default.

Mutable typed declarations may use semantic defaults.

Copy and move syntax are distinct.

Assignments are statements.

Blocks require braces.

Ranges are contextual in Sec 0.1.

`match` is the value-producing branch construct.

`switch` and `select` are statements.

`for` iterates or loops infinitely.

`while` handles condition loops.

`try`, `defer`, `discard`, `spawn`, `await`, and unsafe syntax have explicit
grammar forms.

Operators use the canonical precedence and semantics from `operators.md`.

Canonical arrays and owning dynamic sequences use postfix type syntax.

Safe slices use `ref T[]` or `ref mut T[]`.

First-class ranges, multiple returns, and arbitrary user-defined operators are
not part of Sec 0.1.

The parser constructs syntax.

Sema assigns meaning.

No implementation shortcut may redefine the canonical grammar.

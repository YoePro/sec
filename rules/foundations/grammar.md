# Grammar

- **Status:** Normative
- **Created:** 2026-08-02
- **Last updated:** 2026-11-09 (§ 61A join statement, § 63 SelectJoinBranch); 2026-11-07 (stable chapter, subsection, and paragraph numbering)
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/foundations/grammar.md`
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`
- **Implementation governance:** `governance/parser.yaml` (`frontend.grammar-conformance`)
- **Related rulebooks:** `rules/foundations/lexical_structure.md`, `rules/foundations/operators.md`, `rules/foundations/attributes.md`, `rules/tooling/formatter.md`, `rules/types/types.md`, `rules/memory/ownership.md`, `rules/memory/copy_move.md`, `rules/compiler/parser_recovery.md`, `rules/compiler/compile_time_evaluation.md`, `rules/compiler/semantic_ir.md`, `rules/projects/modules.md`

---

## § 1 Status

**§ 1(1)** This document is the canonical consolidated grammar for Sec 0.1.

**§ 1(2)** It defines:

- compilation-unit structure;
- declarations;
- type references;
- statements;
- expressions;
- patterns;
- contextual syntax;
- canonical syntax versus accepted recovery syntax;
- the boundary between parsing and semantic analysis.

**§ 1(3)** This document does not redefine:

- lexical tokenization from `lexical_structure.md`;
- operator precedence or operator semantics from `operators.md`;
- formatting from `formatter.md`;
- type semantics from `types.md`;
- ownership semantics from `ownership.md` and `copy_move.md`;
- detailed feature semantics from specialized rulebooks.

**§ 1(4)** Implementation status is not part of this rulebook. It is tracked by
`frontend.grammar-conformance` in `governance/parser.yaml` and by the
feature-specific governance entries.

---

## § 2 Canonical syntax and legacy forms

**§ 2(1)** Sec 0.1 accepts only the canonical syntax defined by the active rulebooks.
Recognizable legacy Sec syntax is invalid and receives a focused migration
diagnostic when the intended migration can be determined reliably. Default
formatting never converts invalid legacy syntax into canonical syntax; an
explicitly enabled Language Correction (`rules/tooling/formatter.md` §§ 26–27)
may rewrite a recognized legacy form only when the rewrite is unambiguous and
semantics-preserving, and otherwise reports a focused migration diagnostic
instead of guessing (MD-003; `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 4).

**§ 2(2)** The recognized invalid forms are listed in "Legacy, future, and recovery
syntax" below.

---

## § 3 Forms outside Sec 0.1

**§ 3(1)** The following are not part of Sec 0.1:

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

**§ 3(2)** Map and set literal syntax is not finalized by this grammar. Array literals and
empty list literals are defined below.

**§ 3(3)** The module model, including resolution, import cycles, re-exports, selective
and wildcard imports, export lists, conditional imports, and target-conditioned
declarations, belongs to `rules/projects/modules.md`.

---

## § 4 Normative grammar notation

**§ 4(1)** The grammar uses an EBNF-like notation.

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

**§ 4(2)** Grammar notation is not Sec source syntax.

---

## § 5 Lexical boundary

**§ 5(1)** The lexer produces tokens.

**§ 5(2)** The parser consumes tokens.

**§ 5(3)** The grammar assumes:

- comments and whitespace are recognized lexically;
- source positions include file, line, and column;
- multi-character operators use longest match;
- string and character escaping is already validated lexically where possible;
- identifiers are distinct from reserved keywords;
- contextual spellings may remain identifier tokens.

**§ 5(4)** The grammar must not duplicate the complete lexical specification.

---

## § 6 Source file

**§ 6(1)** Canonical high-level grammar:

```text
CompilationUnit
    ::= [ TargetDirective ]
        ModuleDeclaration
        { ImportDeclaration }
        { TopLevelDeclaration }
```

**§ 6(2)** A source file must declare a module.

**§ 6(3)** `#target`, when present, must precede every declaration and import.

**§ 6(4)** The parser may recover from misplaced declarations, but canonical source follows
the order above.

---

## § 7 Target directive

```text
TargetDirective
    ::= "#" "target" "(" TargetArgument "," TargetArgument [ "," ] ")"

TargetArgument
    ::= "os" ":" StringLiteral
      | "arch" ":" StringLiteral
```

**§ 7(1)** Both `os` and `arch` are required.

**§ 7(2)** Their order is not semantically significant.

**§ 7(3)** Unknown or duplicate arguments are invalid.

**§ 7(4)** Example:

```sec
#target(os: "linux", arch: "amd64")
```

**§ 7(5)** `target` is the only directive after `#` in Sec 0.1.

**§ 7(6)** **Synchronization (2026-11-07; `rules/compiler/compiler.md` § 16,
`rules/foundations/attributes.md`).** `#target(...)` above is the legacy
compatibility form. The canonical Sec 0.1 target selection is the attribute
`@target(os: ..., arch: ...)` defined in `attributes.md`; new rulebooks,
examples, formatter output and generated source use `@target`. A compiler may
temporarily accept `#target` during migration (`compiler.md` § 15(3)); that
acceptance is implementation governance, not a second language model.

---

## § 8 Module declaration

```text
ModuleDeclaration
    ::= "module" Identifier
```

**§ 8(1)** Example:

```sec
module parser
```

**§ 8(2)** Dotted source module declarations are invalid. Directory hierarchy,
`internal`, and compiler-reserved roots belong to canonical import paths and
never become part of the source `ModuleName`.

**§ 8(3)** The complete relationship between source path, `internal` directories, module
identity, and imports belongs to `rules/projects/modules.md` and
`rules/projects/projects.md`.

---

## § 9 Import declarations

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

**§ 9(1)** Examples:

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

**§ 9(2)** The string is a canonical logical import path. It uses `/`, is case-sensitive,
is not an absolute host path, and contains neither `.` nor `..` traversal
components.

**§ 9(3)** An optional identifier is a source-file-local import alias. Imports do not
inject unqualified names and are not implicitly re-exported.

**§ 9(4)** Selective and wildcard imports are not part of Sec 0.1.

---

## § 10 Top-level declarations

```text
TopLevelDeclaration
    ::= TypeDeclaration
      | ForeignTypeDeclaration
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

**§ 10(1)** Ordinary executable statements are invalid at module scope.

**§ 10(2)** Top-level mutable storage remains subject to static and initialization rules.

**§ 10(3)** Test source adds the contextual top-level declaration:

```text
TestDeclaration
    ::= "test" StringLiteral Block
```

**§ 10(4)** It is legal only in `*_test.sec` selected by a `TestCompilationPlan`. It has no
parameters or source return type, may not be nested, and is governed by
`rules/tooling/testing.md`. Lexer, parser, formatter, editor grammar, and Sema
must share one contextual treatment of `test` rather than reserving or inferring
it independently.

**§ 10(5)** **Amendment MD-111 (2026-10-30, `rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md` §§ 4(5), 8(1)).** Module scope
also admits the contextual hardware fact declarations:

```text
TopLevelDeclaration
    ::= ... | HardwareDeclaration

HardwareDeclaration
    ::= "hardware" ("region" | "resource" | "endpoint") Identifier "{" HardwareMemberList "}"

HardwareMemberList
    ::= HardwareMember { "," HardwareMember } [ "," ]

HardwareMember
    ::= Identifier ":" HardwareMetadataValue

HardwareMetadataValue
    ::= IntegerLiteral | StringLiteral | BooleanLiteral
      | QualifiedHardwareEnumCase | HardwareSymbolPath
      | "[" [ IntegerLiteral { "," IntegerLiteral } [ "," ] ] "]"

QualifiedHardwareEnumCase
    ::= HardwareEnumTypeName "." EnumCaseName

HardwareSymbolPath
    ::= Identifier | ImportedModuleQualifier "." Identifier
```

**§ 10(6)** This is restricted metadata syntax, not a struct initializer or expression:
values are exact literals, qualified metadata enum cases, symbolic hardware
selectors or homogeneous integer lists, never runtime expressions or ordinary
bindings. The attribute form `@address(EndpointPath [, alias])` is specified in
`rules/foundations/attributes.md`.

---

## § 11 Identifier namespaces

**§ 11(1)** The grammar permits declarations in contexts controlled by name and scope rules.

**§ 11(2)** The parser does not decide:

- whether a name is already declared;
- whether a name is visible;
- whether a name is reserved by a compiler-known namespace;
- whether a nested name conflicts with a module declaration.

**§ 11(3)** Those checks belong to Sema.

---

## § 12 Generic parameter lists

```text
GenericParameterList
    ::= "[" GenericParameter { "," GenericParameter } [ "," ] "]"

GenericParameter
    ::= [ "..." ] Identifier [ GenericConstraintClause ]

GenericConstraintClause
    ::= ":" TypeReference { "&" TypeReference }
```

**§ 12(1)** MD-117 amendment (2026-10-30; `rules/declarations/generics.md` § 17A and
`rules/corrections/md117-heterogeneous-variadic-type-packs-correction.md` § 2):
only a function or method generic declaration may use the `...` prefix, at most
once and on its final generic parameter. It declares one compile-time ordered
sequence of zero or more concrete types, distinct from an ordinary single type.
The spelling of its identifier remains ordinary; `Types` is not a keyword.
Concrete inference and constraint validity belong to the owning generic and
function rules. This amendment introduces no explicit pack literal, array form
or general runtime spread rule.

**§ 12(2)** Examples:

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

**§ 12(3)** `&` combines multiple interface constraints. It is a compile-time conjunction,
not a runtime intersection type.

**§ 12(4)** A general `where` clause for generic declarations is not part of the current
grammar.

---

## § 13 Type declarations

**§ 13(1)** Canonical forms:

```text
TypeDeclaration
    ::= NamedTypeDeclaration
      | StructTypeDeclaration
      | UnionTypeDeclaration
      | TypeEnumDeclaration
      | RegisterTypeDeclaration
```

---

## § 14 Named type declaration

```text
NamedTypeDeclaration
    ::= "type" Identifier [ GenericParameterList ]
        TypeReference
        [ ImplementsClause ]
        { TypeContract }
        [ DefaultClause ]
```

**§ 14(1)** Example:

```sec
type Percent int range 0..100
```

**§ 14(2)** Example:

```sec
type Port int range 1..65535 default 8080
```

**§ 14(3)** Example:

```sec
type Speed decimal<m/s>
```

**§ 14(4)** This creates a nominal type.

**§ 14(5)** It is not a transparent alias.

**§ 14(6)** A named type declaration has exactly one underlying type. The named type is
distinct from its underlying type, and chained named types remain distinct
identities:

```sec
type B int
type A B
```

**§ 14(7)** declares three distinct types: `int`, `B`, and `A`.

**§ 14(8)** A declaration naming more than one underlying type is invalid:

```sec
type A int string
type A B int
```

**§ 14(9)** A declaration names exactly one type. Several names in one declaration are
invalid; each type is declared separately:

```sec
type A, B int        // invalid
type A int
type B int
```

**§ 14(10)** `type A B` is valid only when `B` resolves to a type; otherwise `B` is an
unknown type.

**§ 14(11)** (MD-002; `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 4.)

---

## § 15 Legacy assigned and compact variant forms

**§ 15(1)** The following legacy forms are not Sec 0.1 grammar. The parser recognizes them
only to issue a focused migration diagnostic:

```text
LegacyAssignedNamedType
    ::= "type" Identifier [ GenericParameterList ] "=" TypeReference ...

LegacyCompactVariantDeclaration
    ::= "type" Identifier "=" Identifier Identifier { Identifier }
```

**§ 15(2)** Examples:

```sec
type UserID = uint64
type IOError = FileNotFound AccessDenied InvalidValue
```

**§ 15(3)** The canonical replacement for the first is `type UserID uint64`. Closed
alternatives use `enum` or `union`.

---

## § 16 Implements clause

```text
InterfaceImplementsClause
    ::= "implements" TypeReference { "," TypeReference }

ImplImplementsClause
    ::= "implements" TypeReference { "," TypeReference }
```

**§ 16(1)** Examples:

```sec
impl Car implements Vehicle {
}
```

```sec
interface Vehicle implements Startable, Stoppable {
}
```

**§ 16(2)** Separate `impl Interface for Type` syntax is invalid.

**§ 16(3)** `ImplImplementsClause` is valid only on the primary `impl Type`. An
`impl extends Type` fragment cannot redeclare conformance.

---

## § 17 Type contracts

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
      | Contextual("minByteLen") Expression
      | Contextual("maxByteLen") Expression
      | Contextual("exactByteLen") Expression

RegexContract
    ::= Contextual("regex") Expression

MarkerContract
    ::= Contextual("notEmpty")
      | Contextual("unique")
      | Contextual("finite")
      | Contextual("odd")
      | Contextual("even")
```

**§ 17(1)** Contracts are written sequentially.

**§ 17(2)** Sequential contracts are logical conjunction.

**§ 17(3)** Every contract argument is ordinary `Expression` syntax. Contract arguments
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

**§ 17(4)** `regex` is a reserved contract spelling (`rules/foundations/lexical_structure.md`
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

**§ 17(5)** The regular-expression language belongs to a dedicated future regex rulebook
(MD-010), and the regex pattern is an ordinary `Expression` evaluated in a
`SemanticCompileTimeRequiredContext` (MD-011). The earlier
`RegexContract ::= Contextual("regex") ConstantExpression` shape is superseded
(MD-009; `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 10;
`rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` §§ 2, 3.39–3.44).

**§ 17(6)** Example:

```sec
type PageSize int range 10..100 multipleOf 10
```

**§ 17(7)** Example:

```sec
type Tags string[] notEmpty unique
```

**§ 17(8)** Contracts belong to named types.

**§ 17(9)** They are not canonical on individual variables or struct fields.

---

## § 18 Default clause

```text
DefaultClause
    ::= "default" Expression
```

**§ 18(1)** The default clause follows every contract. Its expression is ordinary
`Expression` syntax evaluated in a `SemanticCompileTimeRequiredContext`
(MD-011; `rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` §§ 3.4, 3.35–3.38).

**§ 18(2)** Example:

```sec
type Port int range 1..65535 default 8080
```

**§ 18(3)** Example:

```sec
type User string in ["Admin", "User", "Other"] default "User"
```

**§ 18(4)** The expression must be compile-time evaluable, allocation-free, representable,
and valid for every contract.

---

## § 19 Struct type declaration

```text
StructTypeDeclaration
    ::= "type" Identifier [ GenericParameterList ]
        "struct" { StructLayoutModifier }
        [ ImplementsClause ]
        StructBody

StructBody
    ::= "{" [ StructField { "," StructField } [ "," ] ] "}"

StructField
    ::= Identifier ":" TypeReference
        { StructFieldLayoutModifier } [ StructTag ]

StructTag
    ::= RawStringLiteral

StructLayoutModifier
    ::= "pack" "(" Expression ")"
      | "minAlign" "(" Expression ")"

StructFieldLayoutModifier
    ::= "minAlign" "(" Expression ")"
      | "byteOffset" "(" Expression ")"
```

**§ 19(1)** Layout modifiers (MD-019, `rules/corrections/applied/MD-019-explicit-layout-modifiers-applied-20261105.md`):

- `pack`, `minAlign` and `byteOffset` are compiler-known **contextual
  declaration modifiers** in these positions only. They are not global
  keywords, functions, executable expressions or `@` attributes; the closed
  `@` attribute set is unchanged and `@pack(1)`, `@minAlign(8)` and
  `@byteOffset(8)` are invalid.
- Each modifier `Expression` is a `SemanticCompileTimeRequiredContext` with an
  integer result (`compile_time_evaluation.md` §§ 4–8); a generic-dependent
  value is evaluated for the concrete specialization and `CompilationPlan`.
- A modifier kind occurs at most once on one declaration; the order of
  different kinds does not matter. Repetition, conflicts, unknown names in a
  modifier position, malformed arguments and invalid attachment are
  diagnosed.
- Field modifiers precede an optional field tag. `implements` placement, raw
  tags, comma rules and register declaration modifiers are unchanged.
- The formatter writes no space between a modifier name and `(`, one space
  between successive modifiers and before `{`.

```sec
extern "C" type Header struct pack(1) minAlign(8) {
    kind: c::uchar,
    value: c::uint,
}

type Message struct pack(1) {
    kind: byte,
    value: uint32 minAlign(8) `json:"value"`,
    checksum: uint16 byteOffset(16),
}
```

**§ 19(2)** Their semantics are owned by `rules/declarations/struct.md` § 16A and
`rules/memory/layout.md` §§ 27–29.

**§ 19(3)** Canonical examples:

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

**§ 19(4)** Commas are required between declared struct fields.

**§ 19(5)** A trailing comma is allowed.

**§ 19(6)** Contracts are expressed through named field types.

---

## § 20 Standalone struct declaration (Sec 0.2)

**§ 20(1)** `struct Identifier StructBody` is planned Sec 0.2 syntax and is not part of the
Sec 0.1 grammar. Sec 0.1 source uses `type Name struct`.

---

## § 21 Struct literal

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

**§ 21(1)** Examples:

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

**§ 21(2)** A newline may separate struct literal items.

**§ 21(3)** Declared struct fields require commas.

**§ 21(4)** An empty literal is valid when every omitted field is defaultable:

```sec
Token {}
```

**§ 21(5)** Sema inserts omitted field defaults.

---

## § 22 Enum declaration

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

**§ 22(1)** The marker order is normative: `MEMBER default [= expression]`.
`default MEMBER` is not enum-member syntax.

**§ 22(2)** Examples:

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

**§ 22(3)** Canonical enum value initialization uses `=`. The legacy `Value: expression`
initializer is invalid Sec 0.1 syntax (see "Legacy, future, and recovery
syntax").

**§ 22(4)** The first omitted initializer is implicit `iota`. Every later omitted
initializer repeats the preceding initializer expression and evaluates it with
the current member's `iota`; it is not automatic previous-value-plus-one
continuation.

**§ 22(5)** An explicit ordinary underlying type may be an integer type or `string`. A
string-backed enum requires an explicit compile-time string initializer for
every member; omitted initializers and `iota` are not part of string-backed
enums:

```sec
enum Program string {
    OneCare = "Zebra OneCare",
    VIQ = "Z1C+VIQ",
}
```

**§ 22(6)** Generic parameters use the ordinary `GenericParameterList` grammar. They do not
give enum members payloads; payload-bearing alternatives are union declarations
(`rules/declarations/enums.md` § 16.1):

```sec
enum State[T] {
    Ready
    Busy
}
```

**§ 22(7)** An enum must declare at least one value.

**§ 22(8)** When present, `error` is the final semantic marker before the enum body. It is
not an underlying type and does not introduce general inheritance.

---

## § 23 Union declaration

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

**§ 23(1)** Examples:

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

**§ 23(2)** A comma after a union variant is optional.

**§ 23(3)** Struct-like payload fields follow struct declaration field grammar and require
commas.

**§ 23(4)** At most one variant may carry the post-variant `default` marker. Its payload
must be default-constructible under `rules/declarations/unions.md` and
`rules/types/default_values.md`.

**§ 23(5)** An empty union is invalid.

---

## § 24 Register declaration

```text
RegisterTypeDeclaration
    ::= "type" Identifier
        Contextual("register")
        "[" IntegerConstant "]"
        { RegisterTypeModifier }
        RegisterBody
        [ ImplementsClause ]

RegisterTypeModifier
    ::= RegisterAllocationOrder
      | RegisterByteOrder
      | RegisterShadowPolicy

RegisterAllocationOrder
    ::= Contextual("lsb") "-" Contextual("first")
      | Contextual("msb") "-" Contextual("first")

RegisterByteOrder
    ::= Contextual("little") "-" Contextual("endian")
      | Contextual("big") "-" Contextual("endian")

RegisterShadowPolicy
    ::= Contextual("shadow")
      | Contextual("noshadow")

RegisterBody
    ::= "{" [ RegisterField
        { RegisterFieldSeparator RegisterField }
        [ "," ] ] "}"

RegisterFieldSeparator
    ::= ","
      | LineBreak

RegisterField
    ::= RegisterFieldName ":" RegisterFieldType
        [ RegisterFieldAccess ]
        [ RegisterShadowPolicy ]

RegisterFieldAccess
    ::= Contextual("read") "-" Contextual("write")
      | Contextual("read") "-" Contextual("only")
      | Contextual("write") "-" Contextual("only")
      | Contextual("write") "-" Contextual("one") "-" Contextual("clear")
      | Contextual("write") "-" Contextual("one") "-" Contextual("set")
      | Contextual("write") "-" Contextual("one") "-" Contextual("toggle")
      | Contextual("write") "-" Contextual("zero") "-" Contextual("clear")
      | Contextual("write") "-" Contextual("zero") "-" Contextual("set")
      | Contextual("write") "-" Contextual("zero") "-" Contextual("toggle")
      | Contextual("read") "-" Contextual("clear")
      | Contextual("read") "-" Contextual("set")

RegisterFieldName
    ::= Identifier
      | "_"

RegisterFieldType
    ::= BitFieldType
      | TypeReference

BitFieldType
    ::= Contextual("bit") [ "[" IntegerConstant "]" ] [ UnitAnnotation ]
```

**§ 24(1)** Example:

```sec
type MotorProtocol register[8] {
    Speed: bit[4]<rpm>,
    Enabled: bit,
    _: bit[3],
}
```

**§ 24(2)** The allocation order, byte order, and shadow policy modifiers are independent:
each modifier class may occur at most once, their source order is irrelevant,
and duplicate or conflicting modifiers are errors. `noshadow` is the default.
At field level, an optional access modifier precedes the optional shadow
policy; each may occur at most once. The default field access is `read-write`.
The semantic meaning of these modifiers belongs to
`rules/declarations/registers.md`.

**§ 24(3)** The allocation and byte-order modifiers may both be present:

```sec
type PacketWord register[16] msb-first big-endian {
    Kind: bit[4],
    Value: bit[12],
}
```

**§ 24(4)** The total field width must match the register width.

**§ 24(5)** `_` denotes reserved unnamed bits in register field position.

---

## § 25 Unit declaration

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

**§ 25(1)** `UnitDefaultNumericType` must resolve to a plain compiler-known numeric scalar
carrier and must not itself be unit-bearing. It defaults to `decimal`.
`UnitCategory` defaults to `other`. Unit identifiers remain subject to semantic
reserved-name validation; in particular, `bit` and `byte` cannot be declared as
units.

**§ 25(2)** Examples:

```sec
unit Hertz physical
unit Packet uint other
unit Metre physical
unit Count
unit Euro currency
```

**§ 25(3)** Complete unit semantics belong to `rules/types/units.md`.

---

## § 26 Unit metadata inside impl

**§ 26(1)** The parser recognizes these contextual unit metadata names inside `impl`:

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

**§ 26(2)** Grammar shape:

```text
UnitMetadataDeclaration
    ::= ContextualUnitMetadataName ":" TokensUntilLineEnd
```

**§ 26(3)** PascalCase is canonical. Case and underscore tolerant parsing may remain as a
Sec 0.1 migration aid.

**§ 26(4)** `Name` and `Category` are not contextual metadata names: they are implicit
read-only members of a named unit, and `Name:` or `Category:` inside
`impl <U>` is rejected by semantic analysis. `IsBaseunit` and similar
spellings are not aliases of `BaseUnit`. Enum-backed values are canonically
qualified members (`Kind: UnitKind.Length`); legacy bare names stay accepted
in this context only (MD-062, `rules/types/units.md`, "Unit metadata value
types" (5)). Metadata reads use the existing qualified unit member access
`<U>.Member`; the grammar is otherwise unchanged.

---

## § 27 Interface declaration

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

**§ 27(1)** Example:

```sec
interface Vehicle {
    fn Start() void
    fn Stop() void

    property IsRunning: bool {
        get
    }
}
```

**§ 27(2)** Example:

```sec
interface PressSource {
    event ButtonPressed[ButtonPressData]
}
```

**§ 27(3)** Interface methods have no body.

**§ 27(4)** Interface property accessors have no body.

**§ 27(5)** The setter identifier is mandatory in implementation, static, and interface
property forms. Sec does not provide an implicit setter-value binding.

---

## § 28 Impl declaration

```text
ImplDeclaration
    ::= "impl" [ "extends" ] TypeReference ImplBody
      | "impl" UnitImplTarget ImplBody

UnitImplTarget
    ::= "<" UnitExpression ">"

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

**§ 28(1)** `impl <Unit>` (`UnitImplTarget`, MD-040, 2026-10-13, `rules/corrections/applied/md040-md046-md047-md056-units-register-bits-rounding-correction-20261013.md` § 3) targets only the unit named by a single `UnitExpression` name, in the unit's own module, and holds `UnitMetadataDeclaration` and `FunctionDeclaration` members only. `impl Name` targets only an ordinary type; a unit is never named bare.

**§ 28(2)** `InstanceLetDeclaration` requires an instance; `mut` controls mutability.
`StaticLetDeclaration` requires explicit `static` and is type-owned. The two
forms are not equivalent; the AST and formatting retain the `static` modifier.

**§ 28(3)** Lifecycle and construction syntax:

```text
InitDeclaration
    ::= [ CallableBodyAttributes ] "init" "(" [ ParameterList ] ")" [ TypeReference ] Block

FreeDeclaration
    ::= [ CallableBodyAttributes ] "free" Block

CallableBodyAttributes
    ::= AttributeSet
```

**§ 28(4)** `AttributeSet` is the consecutive attribute sequence of
`rules/foundations/attributes.md`, "General syntax". `CallableBodyAttributes`
attach to the immediately following callable body only — `init`, `free`, one
property accessor, or the lambda of a `LambdaLetDeclaration` — never to the
enclosing `impl` or `property` (MD-030, attributes.md "Callable-body
attachment (MD-030)").

```text
NestedImplDeclaration
    ::= "impl" TypeReference ImplBody

NewExpression
    ::= "new" TypeReference "(" [ ArgumentList ] ")"
```

**§ 28(5)** The optional type after `init(...)` is a construction error type, not a return
type. `new Type(args...)` selects lifecycle construction; `Type(value)` remains
conversion syntax. Ordinary impl methods have compiler-provided implicit
`self` and do not declare receiver parameters.

**§ 28(6)** Example:

```sec
impl Vehicle {
    fn Start() void {
    }

    property TopSpeed: Speed {
        get {
            return _speed
        }

        try set value SpeedError {
            _speed = value
        }
    }
}
```

**§ 28(7)** An additional same-module block is explicit:

```sec
impl extends Vehicle {
    fn Stop() void {
    }
}
```

**§ 28(8)** Sema requires one primary `impl Vehicle` block and rejects extensions from a
different module. File order does not affect extension validity.

**§ 28(9)** Invalid:

```sec
impl Interface for Type {
}
```

**§ 28(10)** Invalid directly inside `impl`:

```text
stored fields
ordinary let declarations
executable statements
standalone struct declarations
```

**§ 28(11)** Methods have implicit `self`.

---

## § 29 Nested type declarations

**§ 29(1)** Nested declarations use the same canonical type syntax:

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

**§ 29(2)** Outside the impl, nested names are qualified through the owner type.

---

## § 30 Property declaration

```text
PropertyDeclaration
    ::= "property" Identifier ":" TypeReference
        "{"
        PropertyAccessor { PropertyAccessor }
        "}"

PropertyAccessor
    ::= [ CallableBodyAttributes ] Getter
      | [ CallableBodyAttributes ] Setter
      | [ CallableBodyAttributes ] FallibleSetter

StaticPropertyDeclaration
    ::= "static" PropertyDeclaration

Getter
    ::= "get" Block

Setter
    ::= Contextual("set") Identifier Block

FallibleSetter
    ::= "try" Contextual("set") Identifier TypeReference Block
```

**§ 30(1)** Example:

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

**§ 30(2)** A property must declare at least one accessor.

**§ 30(3)** Duplicate getters or setters are invalid.

**§ 30(4)** The setter parameter name is required.

**§ 30(5)** `set` should ultimately be contextual.

---

## § 31 Event declaration

**§ 31(1)** Inside `impl`:

```text
EventDeclaration
    ::= Contextual("event") Identifier
        Contextual("using") Identifier
```

**§ 31(2)** Example:

```sec
event Pressed using buttonPressedStorage
```

**§ 31(3)** An event declaration has no body; a block after it is invalid.

---

## § 32 Function declaration

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

**§ 32(1)** The return type and body are mandatory for an ordinary function declaration.
`FunctionSignature` is used only by constructs that explicitly permit a
bodyless callable contract, such as an interface requirement or foreign
`extern` declaration. It is not an ordinary prototype form.

**§ 32(2)** Examples:

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

**§ 32(3)** A return type is required.

**§ 32(4)** Use `void` for no return value.

---

## § 33 Function parameters

```text
ParameterList
    ::= "(" [ Parameter { "," Parameter } [ "," ] ] ")"

Parameter
    ::= Identifier ":" TypeReference
      | "->" Identifier ":" TypeReference
      | Identifier ":" "..." TypeReference
```

**§ 33(1)** Examples:

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

**§ 33(2)** `-> name: Type` is a forced-consuming by-value parameter. `name: ...Type` is
the final native typed variadic parameter. `->` cannot combine with `ref`,
`ref mut`, or `...`; a function has at most one variadic parameter and it must
be last.

**§ 33(3)** `self` is implicit in methods.

**§ 33(4)** Do not write it in canonical parameter lists.

---

## § 34 Function return values

```text
ReturnType
    ::= TypeReference
```

**§ 34(1)** Sec 0.1 returns zero or one value:

- `void` means no value;
- every other return type means one value.

**§ 34(2)** A function cannot declare a tuple-like multiple return list.

**§ 34(3)** Use a named type when several related fields must be returned.

---

## § 35 Extern function declaration

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

**§ 35(1)** Example:

```sec
extern "C" fn write(
    fd: int32,
    buffer: RawPtr[byte],
    length: uint,
) int64
```

**§ 35(2)** An extern body is invalid. A bare final `...` is valid only for `unsafe extern
"C"` and is distinct from a native typed variadic parameter.

**§ 35(3)** Foreign data declarations and types additionally use:

```text
ForeignTypeDeclaration
    ::= "extern" ForeignABI "type" Identifier "struct" { StructLayoutModifier } [ ForeignStructBody ]
      | "extern" ForeignABI "type" Identifier "union" ForeignUnionBody
      | "extern" ForeignABI "type" Identifier "enum" ForeignEnumBody

ForeignABI
    ::= StringLiteral

ForeignTypeReference
    ::= "c" "::" Identifier { "::" Identifier }
      | "c" "::" "fn" "(" [ ForeignParameterTypeList ] ")" TypeReference
      | "c" "::" "flex" "[" TypeReference "]"

ForeignCallbackExpression
    ::= "c" "::" "callback" "(" Expression ")"
```

**§ 35(4)** `c::` is the only C qualification prefix; the capitalized `C::` form is not Sec
0.1 syntax. `::` is a single token with no whitespace inside or around it.
Sec 0.1 foreign data declarations currently require the ABI string `"C"`; the
grammar preserves the source-level string-literal spelling and Sema enforces
that current ABI restriction. Foreign forms remain subject to the support
boundary in `rules/platform/ffi.md`.
Fundamental names and target-defined binding names are distinguished during
semantic resolution; the production grants no permission to use an arbitrarily
spelled C type without a real binding (MD-020, `rules/corrections/applied/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md` § 2.8).

**§ 35(5)** `bit[N]` after a field's C base type is a C bitfield declarator only inside an
`extern "C"` data declaration. A complete foreign struct's fields take the same
`StructFieldLayoutModifier` forms as ordinary struct fields (MD-019); a bodyless
incomplete foreign struct has no stored layout, so a layout modifier on it is
invalid. `c::callback(expression)` is the explicit
environment-free callback adapter expression.

---

## § 36 Unsafe function declaration

```text
UnsafeFunctionDeclaration
    ::= "unsafe" FunctionDeclaration
      | "unsafe" ExternFunctionDeclaration
```

**§ 36(1)** Example:

```sec
unsafe fn RawOperation(value: RawPtr[byte]) int {
    // ...
}
```

**§ 36(2)** Unsafe syntax does not disable ordinary grammar, type, ownership, or scope
rules.

---

## § 37 Static declarations

```text
StaticDeclaration
    ::= StaticLetDeclaration
      | StaticFunctionDeclaration

StaticLetDeclaration
    ::= "static" LetDeclaration

StaticFunctionDeclaration
    ::= "static" FunctionDeclaration
```

**§ 37(1)** Inside `impl`, `static` may modify `fn`, `let`, or `property`. Ordinary `fn`
members are instance-bound with compiler-provided implicit `self`; only
`static fn` is type-level and has no receiver. Static property setters use the
`StaticPropertyDeclaration` production defined with properties above and the
same mandatory explicit identifier as instance setters:

```text
"set" Identifier Block
"try" "set" Identifier Block
```

**§ 37(2)** There is no implicit setter-value binding.

**§ 37(3)** Inside `impl`, `StaticLetDeclaration` and `InstanceLetDeclaration` are
distinct. Canonical formatting must preserve the `static` modifier.

**§ 37(4)** Compile-time static dependency order belongs to `rules/declarations/static.md`.
Executable startup and shutdown planning belong to
`rules/compiler/initialization.md`.

---

## § 38 Addressed declaration

```text
AddressedLetDeclaration
    ::= "@" Contextual("address") "(" EndpointPath [ "," Contextual("alias") ] ")" LetDeclaration

EndpointPath
    ::= Identifier [ "." Identifier ]
```

**§ 38(1)** Example:

```sec
@address(MotorProtocolEndpoint)
let mut motorProtocol: MotorProtocol
```

**§ 38(2)** The annotation may not apply to a grouped `let` declaration.

**§ 38(3)** The exact address must be validated semantically.

**§ 38(4)** This is a special grammar form until general attributes exist.

---

## § 39 Let declarations

```text
LetDeclaration
    ::= "let" [ "mut" ]
        LetDeclarator
        { "," LetDeclarator }

LambdaLetDeclaration
    ::= CallableBodyAttributes "let" [ "mut" ] Identifier [ ":" TypeReference ]
        ":=" LambdaExpression
```

**§ 39(1)** A `LambdaLetDeclaration` is valid at top level and in local scope; its
initializer is syntactically a `LambdaExpression` (with an optional capture
clause), and its attributes annotate the lambda body, not the binding (MD-030).

```text
LetDeclarator
    ::= Identifier
        [ ":" TypeReference ]
        [ LetInitializer ]

LetInitializer
    ::= ":=" Expression
      | ":<-" Expression
      | "<-" Expression
```

**§ 39(2)** The permitted initializer spelling depends on whether the type is explicit.

**§ 39(3)** Canonical combinations:

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

**§ 39(4)** Invalid combinations:

```sec
let name: Type :<- source
let name <- source
let name = source
```

**§ 39(5)** A mutable typed declaration may omit the initializer when the type is
defaultable.

**§ 39(6)** An immutable declaration must provide an initializer.

**§ 39(7)** A grouped declaration declares independent bindings. It is not multiple-result
destructuring: `let literal, tokenType := self.readNumber()` is invalid.

---

## § 40 Type-first declarations

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

**§ 40(1)** Examples:

```sec
int mut: a, b, c
```

```sec
float: a := 5.4, pi := 3.14
```

**§ 40(2)** Immutable declarators require initializers.

**§ 40(3)** Mutable declarators may omit them only when the declared type is defaultable.

**§ 40(4)** Contracts do not appear in the canonical variable grammar.

---

## § 41 Parenthesized type-first group

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

**§ 41(1)** Example:

```sec
TokenType (
    ILLEGAL := "ILLEGAL",
    EOF := "EOF",
    IDENT := "IDENT",
)
```

**§ 41(2)** This form is immutable.

**§ 41(3)** Every entry requires an initializer.

---

## § 42 Assignment statements

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

**§ 42(1)** Assignment is a statement.

**§ 42(2)** It does not produce a value.

**§ 42(3)** Chained assignment is invalid.

**§ 42(4)** A `PlaceExpression` is a writable Place: an identifier, a member, index or
slice projection, or the explicit dereference `*r` of a `ref mut T` (MD-063).
`*r = value` replaces the referent; `r = value` targets the holder.

**§ 42(5)** The target is evaluated exactly once.

---

## § 43 Try assignment

```text
TryAssignmentStatement
    ::= "try" PlaceExpression AssignmentOperator Expression [TryHandlerBlock]
```

**§ 43(1)** Example:

```sec
try percent += Percent(value) {
    Err(error) => {
        discard error
    }
}
```

**§ 43(2)** Without a handler block, a failed setter result is propagated as an immediate
`return Err(error)` from the enclosing function. The enclosing return type must
be `Result[_, E]` with the exact setter error type `E`; this form does not
propagate `Option` and performs no implicit error conversion. With a handler
block, the ordinary local-handler rules apply.

**§ 43(3)** `try` on a compound string assignment uses this production with error type
`StringError`, and the whole statement is the protected operation, including a
`Result[string, StringError]` right operand (MD-039). This does not make `try` a
panic catcher or change its precedence.

---

## § 44 Statement grammar

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
      | JoinStatement
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

**§ 44(1)** Context restricts which statements are valid.

**§ 44(2)** `panic` is a statement, not an ordinary callable, so `panic("message")` is
invalid; the optional message is an ordinary string literal. Assertions and
checked `unreachable` are defined by `rules/errors/panic.md` §§ 15–16.

**§ 44(3)** `value++` and `value--` are statement-only aliases for `value += 1` and
`value -= 1` (`rules/foundations/operators.md`); they never produce a value.

---

## § 45 Block

```text
Block
    ::= "{" { Statement } "}"
```

**§ 45(1)** Braces are required.

**§ 45(2)** Sec does not use indentation as block syntax.

**§ 45(3)** Comments may occur between statements.

---

## § 46 Statement termination

**§ 46(1)** Canonical Sec source does not require semicolons after ordinary statements.

**§ 46(2)** A statement normally ends through grammar completion and token position.

**§ 46(3)** Line layout is relevant in selected forms, including:

- return without a value;
- comma-free struct literal fields;
- comma-free enum values;
- comma-free register fields;
- unit declarations;
- typed declaration-group recognition.

**§ 46(4)** General semicolon-separated statement syntax is not part of Sec 0.1.

**§ 46(5)** A semicolon in a `for` header is diagnosed as attempted C-style syntax.

---

## § 47 Expression statement

```text
ExpressionStatement
    ::= Expression
```

**§ 47(1)** An expression statement is valid only when its result and effects may be
discarded according to Sema.

**§ 47(2)** Must-use values require an appropriate consuming context or explicit discard.

---

## § 48 Return statement

```text
ReturnStatement
    ::= "return" [ [ "<-" ] Expression ]
```

**§ 48(1)** Examples:

```sec
return
```

```sec
return value
```

```sec
return <- value
```

**§ 48(2)** Both forms are grammatical. Returning an owned value transfers it according to
ownership semantics; the marker is optional at this terminal boundary.

---

## § 49 If statement

```text
IfStatement
    ::= "if" Expression Block
        { "else" "if" Expression Block }
        [ "else" Block ]
```

**§ 49(1)** Examples:

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

**§ 49(2)** The condition must be `bool`.

**§ 49(3)** Braces are required.

---

## § 50 Infinite for loop

```text
InfiniteForStatement
    ::= "for" Block
```

**§ 50(1)** Example:

```sec
for {
    break
}
```

---

## § 51 Iterable for loop

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

**§ 51(1)** Examples:

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

**§ 51(2)** The number and meaning of bindings depend on the iterable.

**§ 51(3)** The grammar accepts plain, shared-reference, mutable-reference, and discard
bindings. Semantic analysis determines whether a binding mode is valid for the
iterable category and position. Sequential indices, range values, and decoded
string runes do not accept reference binding modes. Map keys and set elements
do not accept `ref mut`.

**§ 51(4)** Sec 0.1 does not add `for -> item in source`, `for item in move source`, or
another consuming-loop grammar form.

**§ 51(5)** `step` is valid for approved range iteration.

**§ 51(6)** Condition-only `for` is invalid.

**§ 51(7)** C-style `for` is invalid.

---

## § 52 While statement

```text
WhileStatement
    ::= "while" Expression Block
```

**§ 52(1)** Example:

```sec
while ready {
}
```

**§ 52(2)** The condition must be `bool`.

**§ 52(3)** Assignment in the condition is diagnosed.

---

## § 53 Break and continue

```text
BreakStatement
    ::= "break"

ContinueStatement
    ::= "continue"
```

**§ 53(1)** They are valid inside loops.

**§ 53(2)** Labeled break and continue are not part of Sec 0.1.

---

## § 54 Switch statement

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

**§ 54(1)** Examples:

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

**§ 54(2)** Subjectless example:

```sec
switch {
    case value < 0:
        return
    default:
        return
}
```

**§ 54(3)** Use commas between several case values.

**§ 54(4)** `||` creates one boolean expression and is not the separator.

**§ 54(5)** `default` must be unique and final.

---

## § 55 Fallthrough

```text
FallthroughStatement
    ::= "fallthrough"
```

**§ 55(1)** It is valid only directly inside a switch case body.

**§ 55(2)** It is not a general jump statement.

---

## § 56 Match expression

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

**§ 56(1)** Examples:

```sec
let value := match result {
    Ok(value) => value
    Err(error) => return Err(error)
}
```

**§ 56(2)** In expression-match result position, a block arm contextually produces its
value from the final expression on every continuing path. Terminating paths do
not require an arm value. This is not general block-expression syntax.

---

## § 57 Match statement

```text
MatchStatement
    ::= MatchExpression
```

**§ 57(1)** A match may be used for effects when arm values are not required.

---

## § 58 Match patterns

**§ 58(1)** Canonical initial pattern grammar:

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

**§ 58(2)** The parser may read a pattern through the ordinary expression grammar.

**§ 58(3)** Sema restricts valid patterns.

**§ 58(4)** General literal, range, and direct `true` / `false` patterns are not part of
Sec 0.1 `match`; use `switch` or `if`. Enum-member patterns remain valid
resolved finite-domain patterns. Struct-like destructuring is shallow, allows
partial binding without `..`, and does not introduce recursive patterns.

**§ 58(5)** A bare `QualifiedIdentifier` naming a payload-bearing union variant is a valid
variant-only pattern (MD-029): it selects the variant without extracting the
payload. A `Result` subject's `Err(...)` arm uses `ErrPattern` from "Try
expression".

**§ 58(6)** Examples:

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

## § 59 Defer statement

```text
DeferStatement
    ::= "defer" Block
```

**§ 59(1)** Examples:

```sec
defer {
    Close()
}
```

**§ 59(2)** Control transfer from inside a defer block is restricted.

---

## § 60 Discard statement

```text
DiscardStatement
    ::= "discard" Expression
```

**§ 60(1)** Example:

```sec
discard result
```

**§ 60(2)** Discard is explicit consumption and early deterministic destruction according
to `discard.md`.

---

## § 61 Detach statement

```text
DetachStatement
    ::= Contextual("detach") Expression [ "discard" ]
```

**§ 61(1)** Examples:

```sec
detach task
```

```sec
detach task discard
```

**§ 61(2)** `detach` is a contextual spelling.

---

## § 61A Join statement

```text
JoinStatement
    ::= Contextual("join") Identifier
```

**§ 61A(1)** Example:

```sec
join worker
```

**§ 61A(2)** `join` is a contextual spelling. It forms a join statement only when an
identifier follows it on the same line; `join(value)` and other uses of `join`
remain ordinary expressions.

**§ 61A(3)** The semantics, including the one-shot join capability and the preserved
handle, are owned by `rules/concurrency/tasks.md` § 17 for `Task[T]` and
`rules/concurrency/threads.md` § 34 for `Thread[T]`.

---

## § 62 Cancel statement

```text
CancelStatement
    ::= "cancel"
```

**§ 62(1)** The current AST form has no explicit operand.

**§ 62(2)** Its meaning depends on the current cancellable context.

**§ 62(3)** Complete cancellation semantics belong to `cancellation.md`.

---

## § 63 Select statement

```text
SelectStatement
    ::= "select" "{"
        { SelectBranch }
        "}"

SelectBranch
    ::= SelectOperationBranch
      | SelectBindingBranch
      | SelectJoinBranch
      | SelectTimeoutBranch
      | SelectDefaultBranch

SelectOperationBranch
    ::= Expression "=>" Block

SelectJoinBranch
    ::= Contextual("join") Identifier "=>" Block

SelectBindingBranch
    ::= Identifier ":=" Expression "=>" Block

SelectTimeoutBranch
    ::= "after" Expression "=>" Block

SelectDefaultBranch
    ::= "default" "=>" Block
```

**§ 63(1)** Example:

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

    after 10<ms> => {
    }

    default => {
    }
}
```

**§ 63(2)** `default` must be unique and final.

**§ 63(3)** A timeout after an unconditional default is unreachable.

---

## § 64 Unsafe block

```text
UnsafeStatement
    ::= "unsafe" Block
```

**§ 64(1)** Example:

```sec
unsafe {
    asm "nop"
}
```

**§ 64(2)** The block does not change the grammar of contained statements.

**§ 64(3)** A statement that starts with `unsafe` followed by `{` is an `UnsafeStatement`;
`unsafe` followed by `fn` or `extern` is a declaration modifier; any other
`unsafe` at statement start begins an expression statement with an
`UnsafeCallExpression`.

---

## § 65 Assembly statement

**§ 65(1)** Simple forms:

```text
AsmStatement
    ::= "asm" StringLiteral
      | "asm" "(" StringLiteral ")"
      | StructuredAsm
```

**§ 65(2)** Structured form:

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

**§ 65(3)** Example:

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

**§ 65(4)** The complete operand and constraint language belongs to
`rules/platform/inline_assembly.md`.

---

## § 66 Expression grammar

**§ 66(1)** The canonical expression grammar is precedence-driven.

**§ 66(2)** This document defines expression forms.

**§ 66(3)** `operators.md` defines precedence, associativity, evaluation order, and operator
semantics.

**§ 66(4)** Conceptual grammar:

```text
Expression
    ::= PrimaryExpression
        { PostfixContinuation | InfixContinuation }
```

**§ 66(5)** The parser may implement this with Pratt parsing.

---

## § 67 Primary expressions

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
      | UnsafeCallExpression
      | RefExpression
      | MoveExpression
      | RuntimeCallExpression
      | ForeignCallbackExpression
      | PrefixExpression
```

---

## § 68 Identifiers and self

```text
IdentifierExpression
    ::= Identifier
      | "self"
```

**§ 68(1)** `self` is valid in an instance implementation context.

**§ 68(2)** The parser accepts it as an expression token.

**§ 68(3)** Sema determines whether the context provides an instance.

---

## § 69 Literals

```text
Literal
    ::= IntegerLiteral
      | FloatingLiteral
      | QuantityLiteral
      | StringLiteral
      | InterpolatedStringLiteral
      | CharacterLiteral
      | BooleanLiteral
```

**§ 69(1)** Numeric suffixes, bases, escapes, and literal token boundaries belong to
`lexical_structure.md` and `types.md`.

### § 69.1 Quantity literals

```text
QuantityLiteral
    ::= (IntegerLiteral | FloatingLiteral) UnitAnnotation
```

**§ 69.1(1)** The `<` of the `UnitAnnotation` (see "Unit annotation") follows the numeric
literal with no intervening whitespace, and the annotation closes with `>` on
the same line. When either condition fails the parser does not form a
quantity literal and `<` is the comparison operator, so `5 < s` and
`5<count && ...` are comparisons. A leading sign is a `PrefixExpression`:
`-5<s>` negates the quantity `5<s>`. Typing, carriers and the conversion of
time quantities to `duration` belong to `rules/types/units.md` ("Quantity
literals") and `rules/types/temporal.md` § 5 (MD-015, 2026-10-13).

---

## § 70 Grouped expression

```text
GroupedExpression
    ::= "(" Expression ")"
```

**§ 70(1)** Parentheses override operator precedence.

---

## § 71 Array literal

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

**§ 71(1)** Examples:

```sec
[1, 2, 3]
```

```sec
[first..., second...]
```

```sec
[224r..246r, 248r..255r, 0x41r]
```

**§ 71(2)** The empty literal `[]` requires contextual type information.

**§ 71(3)** Array and collection ownership rules apply to spread elements.

**§ 71(4)** A `RangeSegment` contributes every value from its lower to its upper bound and
requires both bounds; its semantics are defined by
`rules/collections/collections.md` § 5.6a.

---

## § 72 Empty list literal

**§ 72(1)** Canonical:

```text
EmptyListLiteral
    ::= ListTypeReference "{" "}"
```

**§ 72(2)** Examples:

```sec
list[string] {}
list[Packet, 32] {}
```

**§ 72(3)** It is not a struct literal.

---

## § 73 Lambda expression

```text
LambdaExpression
    ::= CaptureClause? "fn" ParameterList TypeReference Block
```

**§ 73(1)** Example:

```sec
let double := fn(value: int) int {
    return value * 2
}
```

**§ 73(2)** The return type is required.

---

## § 74 Capture clause

```text
CaptureClause
    ::= "capture" "(" [ CaptureEntry { "," CaptureEntry } [ "," ] ] ")"

CaptureEntry
    ::= Identifier
      | "<-" Identifier
      | "ref" Identifier
      | "ref" "mut" Identifier
```

**§ 74(1)** `capture(<-value)` is the consuming capture form. The legacy
`capture(-> value)` spelling is not grammatical.

**§ 74(2)** Example:

```sec
let closure := capture(value) fn(input: int) int {
    return value + input
}
```

**§ 74(3)** The lambda introducer itself is always plain `fn`. Generic parameter lists,
`mut fn`, and `-> fn` are not lambda-expression introducers. `mut fn` and
`-> fn` are callable-type capability syntax.

---

## § 75 Call expression

```text
CallExpression
    ::= Expression "(" [ CallArgument { "," CallArgument } [ "," ] ] ")"

CallArgument
    ::= Expression
      | "_"
      | Expression "..."
```

**§ 75(1)** Examples:

```sec
Add(1, 2)
```

```sec
object.Method(value)
```

```sec
Call(values...)
```

**§ 75(2)** Sema resolves whether the callee is:

```text
function
method
conversion
constructor
union variant constructor
compiler-known callable
```

---

## § 76 Explicit generic call

```text
ExplicitGenericCall
    ::= CallableExpression TypeArgumentList
        "(" [ CallArgument { "," CallArgument } [ "," ] ] ")"
```

**§ 76(1)** Examples:

```sec
Identity[int](10)
pkg.Make[Box[string]]("hello")
```

**§ 76(2)** The bracket must be attached and followed by a call context.

**§ 76(3)** Otherwise the syntax is indexing.

**§ 76(4)** For generic function and method calls, the explicit type arguments may be a
positional prefix of the declaration's parameters. Remaining parameters are
inferred from arguments, receiver, and permitted expected-result context.
Generic argument holes such as `Foo[_, B](...)` or `Foo[, B](...)` are invalid.
Generic type references require their complete type argument list.

---

## § 77 Type conversion expression

**§ 77(1)** Ordinary conversion surface:

```text
ConversionExpression
    ::= TypeReference "(" Expression ")"
```

**§ 77(2)** Examples:

```sec
Percent(value)
decimal(integer)
bool(number)
```

**§ 77(3)** The parser initially creates call-shaped syntax.

**§ 77(4)** Sema resolves a type conversion.

---

## § 78 Bit extraction expression

```text
BitExtractionExpression
    ::= Contextual("bit") "[" IntegerConstant "]" "(" Expression ")"
        [ UnitAnnotation ]
```

**§ 78(1)** The optional `UnitAnnotation` suffix (MD-046, 2026-10-13, `rules/corrections/applied/md040-md046-md047-md056-units-register-bits-rounding-correction-20261013.md` § 5) is admitted only directly after `bit[N](expression)`; it does not authorize a general `Call()<U>` form. Its `<` follows the closing parenthesis with no whitespace and the annotation closes on the same line; otherwise `<` is the comparison operator (`bit[8](x) < limit`). The formatter writes `bit[N](expression)<U>` with no space before the suffix.

**§ 78(2)** A contextual call-shaped expression use of the existing `bit[N]` spelling (MD-046/MD-056, 2026-10-12, `rules/corrections/applied/md046-md056-bit-extraction-correction-20261012.md`). It is not a declaration, a general type conversion, an ordinary call or a user-definable generic, and parsing it never makes `bit[N]` a variable type. When `bit[...]` is not followed by a parenthesized operand, the source is ordinary indexing of a value named `bit`. `IntegerConstant` is evaluated as a compile-time integer constant; its domain and the semantics are owned by `types.md` "Explicit low-bit extraction". The formatter writes `bit[N](expression)` without spaces inside the extraction syntax.

---

## § 79 Unit conversion expression

```text
UnitConversionExpression
    ::= NumericTypeReference UnitAnnotation
        "(" Expression [ "," RoundingModeExpression ] ")"

RoundedNumericConversion
    ::= NumericTypeReference "(" Expression "," RoundingModeExpression ")"
```

**§ 79(1)** `RoundingModeExpression` is an ordinary expression that must resolve to a case of the core `RoundingMode` enum; it is not a special token (MD-047, 2026-10-13, `rules/corrections/applied/md040-md046-md047-md056-units-register-bits-rounding-correction-20261013.md` § 6). `RoundedNumericConversion` is the ordinary call-shaped numeric conversion `T(value)` with the rounding argument; semantics are owned by `units.md` "Explicit rounding".

## § 80 Qualified unit references

```text
QualifiedUnitCall
    ::= UnitAnnotation "(" [ ArgumentList ] ")"

QualifiedUnitMemberAccess
    ::= UnitAnnotation "." Identifier
```

**§ 80(1)** A `UnitAnnotation` in expression position (MD-040, 2026-10-13, `rules/corrections/applied/md040-md046-md047-md056-units-register-bits-rounding-correction-20261013.md` § 3) is valid only as the callee of a unit conversion or unit function call (`<Volt>(value)`, `<rpm>.FromHz(x)`) or the object of a unit member access (`<Volt>.Symbol`). Because no other expression begins with `<`, the form is unambiguous. `QualifiedUnitMemberAccess` followed by an argument list is a unit function call.

**§ 80(2)** Example:

```sec
decimal<C>(decimal(Amp) * decimal(Second))
```

**§ 80(3)** The parser must distinguish:

```sec
left < right
```

**§ 80(4)** from:

```sec
decimal<m>(value)
```

**§ 80(5)** through type and token context.

---

## § 81 Member access

```text
MemberExpression
    ::= Expression "." Identifier
```

**§ 81(1)** Examples:

```sec
vehicle.TopSpeed
Color.red
module.Function
self.data
```

**§ 81(2)** Member access may continue into calls, generic calls, indexing, or other member
access.

---

## § 82 Index expression

```text
IndexExpression
    ::= Expression "[" Expression "]"
```

**§ 82(1)** Example:

```sec
values[index]
```

**§ 82(2)** The indexed type determines whether the result is:

```text
value
place
reference-like access
map lookup result
compiler-known indexed value
```

---

## § 83 Slice expression

```text
SliceExpression
    ::= Expression
        "["
        [ Expression ]
        RangeOperator
        [ Expression ]
        "]"
```

**§ 83(1)** Examples:

```sec
values[2..<8]
values[..<8]
values[2..]
```

**§ 83(2)** Range inclusivity follows the range operator.

**§ 83(3)** The grammar is unchanged for strings; meaning is type-directed. Over a
`string` or `StringView` operand the expression creates a borrowed
`StringView` without `ref` (MD-126, `rules/corrections/applied/MD-126-borrowed-string-ranges-rune-iteration-applied-20261105.md`; `collections.md` § 7.10). Other
operands keep `ref values[...]` / `ref mut values[...]`. No `RuneSlice` token
or keyword exists.

```sec
text[1..<3]
try text[start..<end]
for ch in text[1..<3] { Process(ch) }
```

---

## § 84 Struct or union typed construction

```text
TypedConstruction
    ::= TypeReference StructLiteralBody
```

**§ 84(1)** Sema resolves:

```text
struct literal
struct-like union variant construction
compiler-known typed collection literal
other approved typed construction
```

**§ 84(2)** A non-constructible type is a semantic error.

---

## § 85 Prefix expressions

```text
PrefixExpression
    ::= PrefixOperator Expression

PrefixOperator
    ::= "+"
      | "-"
      | "!"
      | "~"
      | "*"
```

**§ 85(1)** Unary `+` is parsed and validated for numeric operands. Constant integer
expressions fold it without changing the operand value.

**§ 85(2)** Prefix `*` is the explicit dereference of a safe reference (MD-063;
`rules/foundations/operators.md` "Dereference"). Postfix forms bind tighter,
so `*r.Field` is `*(r.Field)`. A dereference may begin a statement as the
`PlaceExpression` of an assignment or compound assignment ("Assignment
statements").

**§ 85(3)** Because newlines do not terminate statements, a `*` at the start of a later
line continues the previous expression as multiplication unless it introduces
a syntactically complete assignment target directly followed by an assignment
operator (or `++` / `--`), in which case it starts a new dereference
assignment statement. The target's brackets may span lines; the decision is
syntactic (`operators.md` "Line-start dereference statements").

---

## § 86 Reference expression

```text
RefExpression
    ::= "ref" [ "mut" ] PlaceExpression
```

**§ 86(1)** When the `PlaceExpression` is a holder of a safe reference, `ref r` / `ref mut r` is an explicit reborrow of the referent with result `ref T` / `ref mut T`, never a reference to the holder; `ref mut` over a shared holder is an error (MD-061, 2026-10-15, `rules/memory/borrowing.md` § 12.4).

---

## § 87 Move expression

```text
MoveExpression
    ::= "<-" PlaceExpression
```

**§ 87(1)** A move expression is accepted in owning expression positions such as call
arguments and constructor, aggregate, `Option`, `Result`, and union payloads.
Grammar acceptance does not decide whether the source is movable or the target
consuming; Sema applies the ownership rules.

**§ 87(2)** Examples:

```sec
ref value
ref mut value
ref mut self.data[0]
```

**§ 87(3)** A reference requires a valid addressable place.

**§ 87(4)** `ref Type[]` in type position is a slice type.

**§ 87(5)** `ref expression` in expression position creates a borrow.

---

## § 88 Try expression

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

ErrPattern
    ::= "Err" "(" ErrorSelection ")"

ErrorSelection
    ::= "_"
      | Identifier
      | QualifiedErrorVariant [ ErrorVariantPayloadPattern ]
      | ContextualErrorVariant [ ErrorVariantPayloadPattern ]

QualifiedErrorVariant
    ::= QualifiedIdentifier "." Identifier

ContextualErrorVariant
    ::= "." Identifier

ErrorVariantPayloadPattern
    ::= "(" PatternArgument ")"
      | "{" [ FieldPattern { "," FieldPattern } [ "," ] ] "}"
```

**§ 88(1)** `ErrPattern` (MD-029) is shared by `try` handlers and by `Err(...)` arms of a
`Result` `match`; both use the same selection and binding semantics
(`rules/errors/errorhandling.md`). An unadorned `Identifier` binds the whole
error and is never an inferred variant. A variant-only shorthand always has a
leading dot. `ContextualErrorVariant` is valid only when the protected failure
channel or `Result[T, E]` subject already supplies one unambiguous closed
concrete error type `E`; it is unavailable for the open root `error` and for
heterogeneous contexts, and it is an additive form activated in Sec 0.1.2 (owner
choice 2026-10-28).
`ErrorVariantPayloadPattern` admissibility follows the resolved variant: `(...)`
for a single payload, `{...}` for shallow struct-like fields, and no suffix for
a variant-only selection, which is valid even when the variant carries a
payload. Destructuring is one level deep; `PatternArgument` and `FieldPattern`
are the productions of "Match patterns" and do not become recursive. The braces
inside `Err(.NotFound { Path: path })` are a field pattern, distinct from the
braces of a handler list, `match` body or arm body; parser recovery and the
formatter preserve that distinction.

**§ 88(2)** Examples:

```sec
let value := try Calculate()
```

```sec
let value := try Calculate() {
    Err(IOError.InvalidValue) => 0
    Err(error) => return Err(error)
}
```

**§ 88(3)** The protected expression determines the legal handler family. Result/error
`try` accepts `Err(...)`; Option `try` accepts `None`. Explicit `Ok(...)` and
`Some(...)` success handlers and the obsolete nested `match { ... }` wrapper
are invalid. Handler lists are semantically partial; unmatched states propagate
when compatible.

---

## § 89 Spawn expression

```text
SpawnExpression
    ::= "spawn" [ SpawnKind ] Expression
      | "spawn" [ SpawnKind ] Block

SpawnKind
    ::= Contextual("task")
      | Contextual("thread")
      | Contextual("process")
```

**§ 89(1)** Examples:

```sec
spawn Work()
spawn task Work()
spawn thread Work()
spawn {
    Work()
}
```

**§ 89(2)** Default kind is `task`.

**§ 89(3)** `process` syntax is parsed but the feature is deferred.

---

## § 90 Await expression

```text
AwaitExpression
    ::= "await" Expression
```

**§ 90(1)** Example:

```sec
let result := await task
```

---

## § 91 Unsafe call expression

```text
UnsafeCallExpression
    ::= "unsafe" CallExpression
```

**§ 91(1)** The direct single-call authorization (MD-079, 2026-10-18). The operand is
parsed at prefix precedence and must be one complete call (function, method,
associated or indirect callable); the prefix binds to that call, not to a
following binary expression:

```sec
let value := unsafe pointer.Read()
unsafe pointer.Write(value)
let buffer := unsafe Buffer.FromRawParts(pointer, length)
let adjusted := unsafe pointer.Read() + SafeAdjustment() // only Read is authorized
```

**§ 91(2)** By the ordinary expression-statement rule the same form is a statement. It is
distinct from the declaration modifier (`unsafe fn`, `unsafe extern`) and from
the block `UnsafeStatement` (`unsafe` followed by `{`). A block-less `unsafe`
followed by anything other than a call (for example `unsafe value + 1`) or by
nothing on the same line is a syntax error. The prefix authorizes only that
call's own unsafe obligations; its receiver, arguments and neighboring
operands need their own authorization (`rules/memory/unsafe.md` § 4(9)–(13)).

---

## § 92 Runtime or compiler call expression

```text
RuntimeCallExpression
    ::= "@" DottedIdentifier
        "(" [ CallArgument { "," CallArgument } [ "," ] ] ")"
```

**§ 92(1)** Example shape:

```sec
@compiler.operation(value)
```

**§ 92(2)** This form must not be confused with general attributes.

**§ 92(3)** The set of valid names is compiler-controlled.

---

## § 93 Spread expression

```text
SpreadExpression
    ::= Expression "..."
```

**§ 93(1)** Spread is valid only in approved contexts.

**§ 93(2)** Sec 0.1 approved contexts are governed by `declarations/spread.md`, including:

```text
call arguments
array literals
struct literals
```

**§ 93(3)** Fixed-arity calls and fixed-array literals require every spread contribution to
have a compile-time-known expansion count. A runtime-length sequence may be
spread only when the destination's canonical rule defines runtime arity.

**§ 93(4)** `expression...` is non-consuming syntax. It does not imply a move or partial
move.

**§ 93(5)** For struct literals, explicit entries and spreads are resolved from left to
right before semantic defaults are supplied for still-omitted Defaultable
fields. A still-omitted NonDefaultable field is invalid. Canonical construction
rules are defined by `declarations/struct.md`.

**§ 93(6)** Spread is not a general standalone value operator.

---

## § 94 Range expression in contextual positions

```text
RangeExpression
    ::= [ Expression ] RangeOperator [ Expression ]
```

**§ 94(1)** Range expressions are accepted only where the surrounding grammar explicitly
permits them: `for` iteration, membership, slicing, switch cases, array
literal range segments, and the argument of `Append` on an owning dynamic
array (`rules/collections/collections.md` §§ 5.6a and 6.7).

**§ 94(2)** Examples:

```sec
0..<10
..100
0..
```

**§ 94(3)** A standalone variable initializer does not create a first-class `Range` value in
Sec 0.1.

---

## § 95 Infix expressions

```text
InfixExpression
    ::= Expression InfixOperator Expression
```

**§ 95(1)** The complete operator inventory and precedence are defined by `operators.md`.

**§ 95(2)** The grammar includes:

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

**§ 95(3)** The parser recognizes contextual `x` at multiplicative precedence. Sema
resolves fixed matrix/matrix and matrix/vector result shapes and rejects
incompatible inner dimensions or element types.

---

## § 96 Membership expression

```text
MembershipExpression
    ::= Expression "in" MembershipSource

MembershipSource
    ::= RangeExpression
      | FixedArrayExpression
      | SliceExpression
```

**§ 96(1)** The parser accepts a range-or-expression right operand.

**§ 96(2)** `for value in source` is iteration grammar, not this boolean operator.

---

## § 97 Foreign null sentinel and null tests

```text
ForeignNullSentinel
    ::= "null"

ForeignNullTest
    ::= Expression "is" "null"
      | Expression "is" "not" "null"
```

**§ 97(1)** `ForeignNullTest` is a boolean expression at the equality/state-test precedence
of `is` and `is not`. Its left operand is bound and evaluated once. `null` is a
reserved spelling, and Sema permits the sentinel and both tests only inside
`unsafe` with a raw-pointer operand or an explicit raw-pointer target type. The
production grants no general-purpose null operand or null expression type, and
`== null`/`!= null` remain invalid (MD-024, `rules/corrections/applied/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md` § 5.8;
`rules/platform/ffi.md` § 11).

---

## § 98 Assignment is not expression syntax

**§ 98(1)** The following are statements:

```sec
destination = source
destination <- source
destination += source
```

**§ 98(2)** They cannot occur where a value expression is required.

**§ 98(3)** Invalid:

```sec
let result := destination = source
```

---

## § 99 Type-reference grammar

**§ 99(1)** Canonical overview:

```text
TypeReference
    ::= ReferenceType
      | FunctionType
      | ForeignTypeReference
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

## § 100 Reference type

```text
ReferenceType
    ::= "ref" [ "mut" ] TypeReference
```

**§ 100(1)** Examples:

```sec
ref int
ref mut Token
ref byte[]
ref mut rune[]
```

**§ 100(2)** A bare safe slice is represented through reference syntax.

---

## § 101 Function type

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

**§ 101(1)** Example:

```sec
fn(int) bool
mut fn() int
-> fn(-> Resource) Handle
```

**§ 101(2)** Parameter names do not appear in function type references.

**§ 101(3)** `FunctionType` deliberately has no `unsafe` prefix: Sec 0.1 has no general unsafe function type, and no spelling, ordering rule, formatter form or implicit coercion exists for one, including through generic type parameters. `unsafe` marks function declarations and unsafe blocks only; a foreign C function pointer is the separate type `c::fn(...)` (`rules/memory/unsafe.md` § 40(6); MD-059, 2026-10-16).

---

## § 102 Unit annotation

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

**§ 102(1)** Examples:

```sec
decimal<m>
decimal<m/s>
Speed<km/h>
```

**§ 102(2)** The parser preserves source spelling. Semantic normalization determines
dimensional equivalence and does not make `<mps>` a syntactic alias for
`<m/s>`.

---

## § 103 Unit-only type reference

```text
UnitOnlyType
    ::= UnitAnnotation
```

**§ 103(1)** `<NamedUnit>` uses the named unit's declared default numeric carrier, or
`decimal` when none is declared. A compound structural unit-only type such as
`<m/s>` always defaults to `decimal`; an explicit carrier may be written as
`float64<m/s>` or `decimal<m/s>`.

---

## § 104 Fixed array type

**§ 104(1)** Canonical:

```text
FixedArrayType
    ::= TypeReference "[" ConstantExpression "]"
```

**§ 104(2)** Examples:

```sec
byte[512]
Token[16]
matrix[float32, 4, 4][2]
```

**§ 104(3)** The length must be a valid compile-time nonnegative integer.

---

## § 105 Owning dynamic array type

**§ 105(1)** Canonical:

```text
OwningDynamicArrayType
    ::= TypeReference "[" "]"
```

**§ 105(2)** Example:

```sec
rune[]
```

**§ 105(3)** This is an owning dynamic sequence type in the current type model.

**§ 105(4)** A slice is:

```sec
ref rune[]
```

**§ 105(5)** or:

```sec
ref mut rune[]
```

---

## § 106 Prefix sequence types are invalid

**§ 106(1)** The prefix spellings `[N]Type` and `[]Type` were never normative Sec syntax.
They are not Sec 0.1 grammar and not legacy forms; the canonical spellings are
`Type[N]` and `Type[]`.

---

## § 107 Generic type arguments

```text
TypeArgumentList
    ::= "[" TypeReference { "," TypeReference } [ "," ] "]"
```

**§ 107(1)** Examples:

```sec
Result[int, IOError]
Option[string]
RawPtr[byte]
Pair[int, string]
```

**§ 107(2)** Not every square-bracket list contains only types.

**§ 107(3)** Compiler-known collection and shaped constructors have mixed type and constant
arguments.

---

## § 108 Collection and shaped types

**§ 108(1)** Canonical families:

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

**§ 108(2)** The parser recognizes these names specially to separate type arguments from
constant arguments.

**§ 108(3)** Complete dimension and capacity validation belongs to Sema.

---

## § 109 Event types

```text
EventType
    ::= "Event" "[" TypeReference [ "," ConstantExpression ] "]"

EventStorageType
    ::= "EventStorage" "[" TypeReference [ "," ConstantExpression ] "]"
```

**§ 109(1)** Examples:

```sec
Event[ButtonPressData, 8]
EventStorage[ButtonPressData, 8]
```

**§ 109(2)** Capacity is compile-time known and greater than zero
(`rules/concurrency/events.md`).

---

## § 110 Parenthesized type

```text
ParenthesizedType
    ::= "(" TypeReference ")"
```

**§ 110(1)** This is grouping.

**§ 110(2)** It does not create a tuple type.

---

## § 111 Type versus expression ambiguity

**§ 111(1)** The parser must distinguish:

```sec
Type(value)
```

**§ 111(2)** from a function call.

**§ 111(3)** Sema resolves the callee.

**§ 111(4)** The parser must distinguish:

```sec
Name[Type](value)
```

**§ 111(5)** from indexing.

**§ 111(6)** The parser must distinguish:

```sec
Type { ... }
```

**§ 111(7)** from a following block.

**§ 111(8)** The parser must distinguish:

```sec
decimal<m>(value)
```

**§ 111(9)** from comparison.

**§ 111(10)** The parser may use lookahead, but the semantic result must match this grammar.

---

## § 112 Contextual spellings

**§ 112(1)** The following spellings are contextual or intended to become contextual:

```text
x
set
event
using
detach
join
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

**§ 112(2)** A contextual spelling remains an ordinary identifier outside its context.

**§ 112(3)** `task`, `thread`, and `process` (`lexical_structure.md` § 8.2) and the contract
words (§ 7.3) are also interpreted by context, but they are reserved and are
never valid declaration names.

**§ 112(4)** The grammar context determines its role.

---

## § 113 Contextual `set`

**§ 113(1)** `set` is contextual (`lexical_structure.md` § 8.2):

- collection type constructor in type position;
- property setter introducer in property position;
- otherwise an ordinary identifier where unambiguous.

---

## § 114 Comments

**§ 114(1)** The lexer recognizes comments.

**§ 114(2)** Documentation comments and attachment rules must remain synchronized with
`lexical_structure.md`, formatter rules, and future documentation tooling.

**§ 114(3)** Comments do not terminate or combine tokens except through normal lexical
separation.

---

## § 115 Reserved syntax

**§ 115(1)** `?` is reserved and has no Sec 0.1 meaning.

**§ 115(2)** `free` is valid only as the `FreeDeclaration` impl member: a lifecycle
declaration inside a valid `impl`, not an expression, an ordinary `fn` or a
statement at arbitrary locations. Its block may use bare `return` but not
`return expression`; eligible targets and semantics are owned by `impl.md`
§ 19 and `destruction.md` § 15 (MD-025, 2026-10-15).

**§ 115(3)** `panic` and `assert` are statement keywords whose forms are defined above and by
`rules/errors/panic.md`; neither is callable.

**§ 115(4)** `require` is a general keyword (`lexical_structure.md` § 8.1) and is not a
valid identifier.

**§ 115(5)** The parser issues focused diagnostics for these spellings rather than generic
token failures.

---

## § 116 Invalid forms

**§ 116(1)** The following are explicitly invalid in canonical Sec 0.1:

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

## § 117 Legacy, future, and recovery syntax

**§ 117(1)** The parser may recognize the following invalid Sec 0.1 forms only to issue a
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

**§ 117(2)** Recognized forms must be marked in AST or diagnostics where needed. Default
formatting does not convert any of them; an enabled Language Correction may
rewrite a legacy form only under the policy in "Canonical syntax and legacy
forms" above (MD-003).

---

## § 118 Parser responsibilities

**§ 118(1)** The parser must:

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

## § 119 AST responsibilities

**§ 119(1)** The AST must represent:

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

**§ 119(2)** The AST may contain parser-level ambiguous nodes where Sema decides the meaning.

**§ 119(3)** The current recovery AST retains invalid statements, failed top-level
declarations, disallowed impl members, malformed match/handler patterns,
selected invalid expressions, and selected invalid type references with
syntax-only recovery metadata. These nodes never confer valid semantics and
batch lowering remains blocked by parser errors.

**§ 119(4)** Example:

```text
CallExpression
    may later resolve to function call, method call, conversion, or constructor
```

---

## § 120 Sema responsibilities

**§ 120(1)** Sema must determine:

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

**§ 120(2)** Sema must not depend on backend accidents such as undefined aggregate fields.

---

## § 121 Formatter responsibilities

**§ 121(1)** The formatter must print canonical syntax.

**§ 121(2)** Default formatting does not convert legacy, future, or otherwise invalid syntax
into canonical syntax. Such rewrites belong to explicitly enabled Language
Corrections (`rules/tooling/formatter.md` §§ 26–27), which apply only when the
rewrite is unambiguous and semantics-preserving, for example:

```text
func -> fn
Value: 1 -> Value = 1 inside enum declaration
type Name = ExistingType -> type Name ExistingType
prefix array type -> postfix array type
```

**§ 121(3)** It must preserve:

```text
:= versus :<-
= versus <-
explicit default clauses
in-list order
omitted struct fields
contextual identifier use
```

**§ 121(4)** The formatter must not infer language semantics.

---

## § 122 Required parser tests

**§ 122(1)** The canonical parser test suite must cover:

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

## § 123 Required grammar fixtures

**§ 123(1)** Create:

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

**§ 123(2)** Every invalid fixture must include:

```sec
/* Expected error: ...
 * Reason: ...
 */
```

---

## § 124 Required synchronization

**§ 124(1)** This document must remain synchronized with:

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
```

**§ 124(2)** Planned files remain references to future canonical closure work.

---

## § 125 Design summary

**§ 125(1)** Sec source files declare a module and contain declarations.

**§ 125(2)** Declarations use explicit typed syntax.

**§ 125(3)** Named types carry contracts and optional explicit defaults.

**§ 125(4)** Structs declare data.

**§ 125(5)** Impl blocks declare behavior.

**§ 125(6)** Interfaces declare requirements.

**§ 125(7)** Functions return one value.

**§ 125(8)** Variables are immutable by default.

**§ 125(9)** Mutable typed declarations may use semantic defaults.

**§ 125(10)** Copy and move syntax are distinct.

**§ 125(11)** Assignments are statements.

**§ 125(12)** Blocks require braces.

**§ 125(13)** Ranges are contextual in Sec 0.1.

**§ 125(14)** `match` is the value-producing branch construct.

**§ 125(15)** `switch` and `select` are statements.

**§ 125(16)** `for` iterates or loops infinitely.

**§ 125(17)** `while` handles condition loops.

**§ 125(18)** `try`, `defer`, `discard`, `spawn`, `await`, and unsafe syntax have explicit
grammar forms.

**§ 125(19)** Operators use the canonical precedence and semantics from `operators.md`.

**§ 125(20)** Canonical arrays and owning dynamic sequences use postfix type syntax.

**§ 125(21)** Safe slices use `ref T[]` or `ref mut T[]`.

**§ 125(22)** First-class ranges, multiple returns, and arbitrary user-defined operators are
not part of Sec 0.1.

**§ 125(23)** The parser constructs syntax.

**§ 125(24)** Sema assigns meaning.

**§ 125(25)** No implementation shortcut may redefine the canonical grammar.

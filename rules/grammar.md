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
- the boundary between parsing and semantic analysis;
- the current lexer, parser, AST, and Sema implementation status.

This document does not redefine:

- lexical tokenization from `lexical_structure.md`;
- operator precedence or operator semantics from `operators.md`;
- formatting from `formatter.md`;
- type semantics from `types.txt`;
- ownership semantics from `ownership.md` and `copy_move.md`;
- detailed feature semantics from specialized rulebooks.

Repository baseline reviewed:

```text
branch: main
review date: 2026-08-01
```

The implementation-status sections describe the repository state reviewed on
that date.

---

# Current implementation status

## Status meaning

In this document:

```text
Implemented
    Lexer, parser, AST, and the principal Sema path exist for the syntax.

Partly implemented
    The syntax is recognized, but one or more of parsing, AST representation,
    Sema validation, analysis, formatting, Semantic IR, or lowering is
    incomplete or inconsistent with the canonical rule.

Not implemented
    Canonical syntax has no complete parser and semantic path, or the spelling
    is only reserved.

Compatibility or recovery syntax
    The parser accepts the form to improve migration or diagnostics, but the
    form is not canonical Sec source.
```

Grammar implementation status does not imply complete target lowering.

Backend status belongs primarily to the specialized rulebooks.

---

# Implemented

## Lexical and compilation-unit foundation

Implemented:

- identifiers;
- keywords;
- integer, float, decimal-family, character, rune, string, raw-string, and
  interpolated-string tokens;
- comments;
- symbolic operators;
- longest-match handling for multi-character operators;
- `#target(os: "...", arch: "...")`;
- `module` declarations;
- dotted module names;
- single imports;
- aliased imports;
- grouped imports;
- parser enforcement that `#target` appears before code and declarations;
- Sema requirement that a program contains a module declaration;
- module-scope rejection of ordinary executable statements.

## Type and declaration syntax

Implemented:

- named type declarations;
- explicit type defaults;
- sequential type contracts;
- `range` contracts;
- `in [...]` contracts;
- `multipleOf`;
- `notEmpty`;
- `unique`;
- `finite`;
- `odd`;
- `even`;
- generic type parameter lists;
- one interface constraint per generic parameter;
- named struct declarations;
- enum declarations;
- type-declaration enum form;
- tagged union declarations;
- register declarations;
- unit declarations;
- interface declarations;
- `implements` lists on types and interfaces;
- nested type declarations inside `impl`;
- nested unit declarations inside `impl`;
- nested enum declarations inside `impl`;
- static declarations;
- addressed `let` declarations through `@address(...)`;
- function declarations;
- generic function declarations;
- extern function declarations;
- unsafe function declarations;
- static functions;
- lambda expressions;
- function types.

## Default syntax and default construction

Implemented in parser and Sema:

- `default` after named-type contracts;
- primitive default resolution;
- named-type default inheritance;
- exact integer and inclusive decimal range-derived defaults;
- first-member defaults for `in [...]`;
- explicit primitive-literal and integer constant-expression default validation;
- ambiguity rejection for equally near-to-zero numeric defaults;
- semantic default initialization of mutable typed declarations;
- semantic completion of omitted struct fields;
- recursive struct defaults;
- fixed-array defaults;
- rejection of omitted non-defaultable struct fields;
- rejection of default initialization for non-defaultable references.

The backend must still be audited wherever aggregate construction previously
started from undefined storage.

## Variable declarations

Implemented:

```sec
let value := expression
let mut value := expression
let value: Type := expression
let mut value: Type
```

Implemented grouped `let` declarations:

```sec
let a := 1, b := 2, c := 3
```

Grouped declarations are not multiple-result destructuring. This remains
unsupported:

```sec
let literal, tokenType := self.readNumber()
```

Each declarator in a grouped declaration currently requires its own
initializer.

Implemented type-first declarations:

```sec
int mut: a, b, c
float: a := 5.4, pi := 3.14
```

Implemented parenthesized immutable type-first groups:

```sec
TokenType (
    ILLEGAL := "ILLEGAL",
    EOF := "EOF",
    IDENT := "IDENT",
)
```

Implemented explicit move initialization:

```sec
let destination :<- source
let destination: Type <- source
```

Implemented copy and move assignment syntax:

```sec
destination = source
destination <- source
```

Implemented compound assignment parsing:

```text
+=
-=
*=
/=
%=
&=
|=
^=
<<=
>>=
```

Implemented `try` assignment with handler block.

## Struct syntax

Implemented:

- canonical declaration form `type Name struct`;
- empty structs;
- comma-separated fields;
- trailing commas;
- Go-style raw struct tags;
- struct literals;
- empty struct literals;
- omitted-field default completion;
- multiline struct literals with newline-separated fields;
- comma-separated struct literals;
- struct spread syntax;
- member access;
- direct field assignment;
- nested struct types in `impl`.

## Enum syntax

Implemented:

- default `int` underlying type;
- explicit integer underlying type;
- `bit`;
- `bit[N]`;
- optional colon before `bit[N]`;
- explicit `=` initializer;
- automatic numeric continuation;
- `iota`;
- aliases;
- nested enums;
- enum namespace access;
- enum conversions;
- same-enum equality;
- enum use in `switch`;
- enum patterns and exhaustiveness checks in `match`.

The parser also accepts `Value: expression` as formatter-recovery syntax and
warns that `=` is canonical.

## Union syntax

Implemented:

- payload-less variants;
- one unnamed payload;
- struct-like payloads;
- generic unions;
- nested unions;
- payload-less construction;
- single-payload construction;
- struct-like construction;
- union patterns;
- payload binding;
- match exhaustiveness checks.

## Register syntax

Implemented:

- `register[Width]`;
- `bit`;
- `bit[Width]`;
- reserved `_` fields;
- unit annotations on bit fields;
- bit-backed enum fields;
- `@address(...) let mut ...`;
- register-width and total-field-width checks.

## Interface and impl syntax

Implemented:

- interface method requirements;
- interface property requirements;
- interface event requirements;
- interface inheritance through `implements`;
- type conformance declarations through `implements`;
- `impl Type { ... }`;
- methods inside `impl`;
- static methods inside `impl`;
- static values inside `impl`;
- nested type, unit, and enum declarations;
- properties;
- contextual event declarations;
- rejection of stored fields inside `impl`;
- rejection of executable statements directly inside `impl`;
- rejection of separate `impl Interface for Type` syntax.

Methods receive implicit `self`.

Canonical method declarations do not write `self` in the parameter list.

## Function syntax

Implemented:

- required return type;
- `void` for no result;
- immutable by-value parameters;
- `ref` parameters;
- `ref mut` parameters;
- generic parameters;
- generic constraints;
- function overloading by parameter signature;
- calls;
- method calls;
- explicit generic calls;
- function values;
- lambda expressions;
- explicit capture-list syntax;
- `Ok(...)`;
- `Err(...)`;
- `try`;
- local try handlers;
- `return`;
- zero-value `Ok()` for `Result[void, E]`.

Sec 0.1 supports one return value.

## Control flow

Implemented:

- `if`;
- `else if`;
- `else`;
- range and membership conditions;
- infinite `for`;
- `for ... in ...`;
- multiple `for` bindings;
- `step`;
- `while`;
- subject `switch`;
- subjectless `switch`;
- value cases;
- relational switch cases;
- range switch cases;
- comma-separated switch case items;
- `default`;
- `fallthrough`;
- `break`;
- `continue`;
- `match` as statement or expression;
- wildcard `_` match pattern;
- match guards through `where`;
- expression, return, and block match arms;
- `select`;
- operation select branches;
- binding select branches;
- `after` timeout branches;
- `default` select branches.

## Resource, cleanup, and explicit consumption statements

Implemented syntax and Sema paths exist for:

- `defer { ... }`;
- `defer return`;
- `discard expression`;
- `detach handle`;
- `detach handle discard`;
- `cancel`;
- destruction-related ownership checks for discard;
- control-flow restrictions inside defer.

## Unsafe and assembly syntax

Implemented parsing:

- `unsafe { ... }`;
- `unsafe fn`;
- `unsafe extern`;
- `asm "template"`;
- `asm("template")`;
- structured asm blocks;
- asm input sections;
- asm output sections;
- named asm outputs;
- clobber sections.

Basic Sema checks exist.

Complete inline-assembly semantics remain a separate planned rulebook.

## Concurrency expression syntax

Implemented parsing and principal Sema paths:

```sec
spawn Work()
spawn task Work()
spawn thread Work()
spawn process Work()
spawn {
    // ...
}

await handle
detach handle
cancel
select {
    // ...
}
```

Process spawning remains a deferred feature even though its syntax is parsed.

## Expressions

Implemented expression forms:

- identifiers;
- `self`;
- integer literals;
- floating and decimal-family literals;
- character literals;
- rune numeric suffixes;
- string literals;
- interpolated-string tokens;
- booleans;
- grouped expressions;
- prefix `-`;
- prefix `!`;
- prefix `~`;
- arithmetic binary expressions;
- shifts;
- bitwise expressions;
- equality;
- ordered comparison;
- logical expressions;
- calls;
- explicit generic calls;
- member access;
- indexing;
- slicing;
- array literals;
- spread;
- struct literals;
- conversions;
- unit conversions;
- `ref`;
- `ref mut`;
- `try`;
- `match`;
- lambdas;
- capture lambdas;
- `spawn`;
- `await`;
- compiler/runtime calls beginning with `@`.

---

# Partly implemented

## Canonical operator surface versus parser

`operators.md` is canonical.

The current implementation still differs in several places:

- contextual matrix multiplication `x` is parsed at multiplicative precedence
  and Sema validates fixed matrix/matrix and matrix/vector shapes, while
  lowering and parser-aware formatter/LSP classification remain incomplete;
- `++` and `--` are accepted by the language rulebook as formatter-normalized
  statement aliases, but the current lexer and parser do not yet tokenize or
  parse them;
- `in` currently has complete Sema behavior for ranges, while fixed-array and
  slice membership require completion;
- runtime string `+` is more broadly accepted by current Sema than the Sec 0.1
  compile-time-only rule permits;
- complete checked overflow, shift validation, and remainder lowering remain
  incomplete;
- float `!=` lowering requires the canonical unordered-NaN behavior.

The grammar records canonical syntax and marks the implementation differences.

## Expression-start classification

The parser now uses `isExpressionStart` as its shared classification for
ordinary expression lookahead, including every current prefix form. Specialized
contexts such as range bounds may deliberately accept a narrower subset and
must keep that restriction explicit.

## Generics

Parsing and substantial Sema support exist for:

- generic type declarations;
- generic functions;
- generic calls;
- generic structs;
- generic unions;
- generic interfaces;
- generic type substitution.

Still incomplete:

- complete generic lowering;
- monomorphization rulebook;
- compile-time value-parameter syntax as a general generic facility;
- method-level generic coverage;
- generic enums;
- complete ambiguity diagnostics between indexing and explicit generic calls;
- full generic symbol identity across modules.

## Type syntax ambiguity

Square brackets serve several roles:

```text
generic arguments
collection type arguments
shaped type arguments
fixed-array suffixes
owning dynamic-array suffixes
indexing
slicing
array literals
explicit generic calls
```

The parser uses lookahead and speculative parsing.

The canonical grammar defines the alternatives, but complete grammar-driven
ambiguity resolution and diagnostics remain partial.

## Prefix sequence types

The parser accepts:

```sec
[N]Type
[]Type
```

as prefix sequence type references.

Canonical Sec source uses postfix forms:

```sec
Type[N]
Type[]
```

Prefix sequence syntax is an implemented compatibility form, not canonical
Sec 0.1 syntax.

The formatter should normalize it or diagnostics should direct users to the
postfix form.

## Standalone `struct Name`

The parser accepts:

```sec
struct Name {
}
```

Canonical Sec syntax is:

```sec
type Name struct {
}
```

The standalone form is compatibility syntax.

It must not be the primary grammar form.

## Assigned type syntax

The parser and Sema accept:

```sec
type Name = ExistingType
```

and the old compact variant form:

```sec
type IOError = FileNotFound AccessDenied InvalidValue
```

The ordinary canonical named-type syntax is:

```sec
type Name ExistingType
```

Tagged alternatives should normally use `enum` or `union`.

The exact continuing role of the compact `=` variant declaration is not defined
by a dedicated modern rulebook.

It is therefore documented as implemented compatibility syntax rather than a
preferred Sec 0.1 form.

## Field contracts

The parser still accepts contracts after struct field types and type-first
variable types.

Canonical Sec contracts belong to named types.

Examples such as these are not canonical:

```sec
type User struct {
    age: int range 0..130,
}

let mut value: int range 0..100
```

Use:

```sec
type Age int range 0..130
type Percent int range 0..100

type User struct {
    age: Age,
}

let mut value: Percent
```

Parser support for inline field or variable contracts is legacy and must be
removed or converted to focused diagnostics.

## Struct and list literal ambiguity

The parser can parse a generic-looking type followed by braces as a typed
literal.

Sema currently resolves that path as a struct or union construction.

Canonical empty list construction is:

```sec
list[T] {}
list[T, Capacity] {}
```

Dedicated list-literal semantic resolution is not complete.

The parser must preserve enough syntax to distinguish a compiler-known
collection literal from a struct literal.

## Properties

Property syntax is implemented, including:

```sec
get { ... }
set value { ... }
try set value { ... }
```

Still partial:

- complete getter and setter body analysis;
- `try` property assignment;
- complete lowering;
- full effect and ownership integration;
- contextual treatment of `set` rather than globally reserving it.

## Events

Interface event requirements and impl event declarations are parsed.

Still partial:

- complete event-type grammar;
- storage-field validation;
- event declaration body policy;
- event lowering;
- full interface conformance for event semantics;
- final contextual-keyword treatment.

`event` and `using` are currently contextual identifier spellings.

## Interfaces

Interface declarations and conformance checks exist.

Still partial:

- complete generic interface lowering;
- equality and hashing contracts;
- effect requirements;
- ownership requirements;
- default method bodies if ever permitted;
- complete erased representation;
- complete backend lowering.

## Impl blocks

Methods, properties, events, static members, and nested declarations are parsed.

Still partial:

- complete method lowering;
- complete property lowering;
- complete event lowering;
- privileged core and stdlib impl rules;
- generic method coverage;
- complete interface integration.

## Extern declarations

The parser accepts:

```sec
extern "C" fn name(...) ReturnType
```

It also accepts an optional body after an extern signature.

The canonical distinction between:

```text
pure foreign declaration
foreign-exported Sec function
foreign shim with Sec body
```

must be finalized by `ffi.txt`, `abi.md`, and the consolidated grammar.

Until that distinction is explicit, body-bearing extern syntax is only partly
defined.

## Explicit `self` parameter recovery

The parser accepts a special `ref self` parameter form.

Canonical Sec methods have implicit `self`, and `self` is not written in the
parameter list.

The parser form exists for compatibility and should not appear in canonical
examples.

## Match patterns

Parser and Sema support:

- wildcard;
- enum variants;
- union variants;
- Result and Option-style variants;
- payload binding;
- guards;
- exhaustiveness.

Still incomplete:

- field-level destructuring;
- array and collection patterns;
- or-patterns;
- richer literal-pattern grammar;
- full backend lowering for every subject family;
- explicit separation of pattern grammar from general expression parsing.

The parser currently parses many patterns through the ordinary expression
parser.

## Try syntax

Parser and Sema support:

```sec
try expression
try expression {
    Err(pattern) => ...
}

try assignment {
    Err(pattern) => ...
}
```

Still partial:

- complete propagation behavior;
- runtime contract failure integration;
- effect typing;
- complete lowering;
- distinction between locally handled and propagated errors in every context.

## Select and concurrency

Parser and Sema cover much of the syntax.

Still partial:

- complete task, thread, channel, and select lowering;
- process spawning;
- all failure paths;
- suspension and effect analysis;
- structured-concurrency enforcement;
- timeout type policy;
- exact readiness semantics.

## Unsafe

The keyword and block/function forms are parsed.

Still partial:

- canonical unsafe operation inventory;
- unsafe expression boundaries;
- unsafe promises versus verified attributes;
- pointer provenance rules in syntax;
- complete diagnostics;
- target restrictions.

The planned `unsafe.md` remains authoritative for final semantics.

## Inline assembly

Structured syntax is parsed.

Still partial:

- constraint grammar;
- register-class grammar;
- immediate operands;
- volatility;
- memory effects;
- target selection;
- operand typing;
- clobber validation;
- backend portability;
- exact result binding.

The planned `inline_assembly.md` remains authoritative.

## Parser recovery

The parser contains substantial local recovery logic for:

- missing struct field colons;
- malformed struct fields;
- malformed enum items;
- malformed impl members;
- malformed loop headers;
- malformed switch cases;
- malformed try handlers;
- unexpected `else`;
- invalid property members;
- unterminated blocks.

Canonical recovery behavior is specified by `parser_recovery.md`, including:

- stable recovery boundaries;
- missing-token nodes;
- malformed-node preservation;
- diagnostic IDs;
- LSP behavior;
- formatter behavior on incomplete source.

## Newline-sensitive recovery

The parser uses source line changes in several places:

- enum value separation;
- register field separation;
- struct literal field separation;
- return termination;
- typed declaration-group recognition;
- unit declaration termination.

This is implemented but not yet centralized.

The grammar defines where line layout is semantically accepted.

The parser must not rely on unrelated heuristic line tests.

---

# Not implemented

## General attributes

There is no general canonical parser for:

```sec
@attribute
@attribute(...)
```

The parser has special cases for:

```sec
@address(...)
@runtime.call(...)
```

These are not a complete attribute system.

`attributes.md` remains planned.

## Increment and decrement aliases

Canonical formatter aliases:

```sec
value++
value--
```

are not yet tokenized or parsed.

## General conditional expression

Not implemented:

```sec
condition ? whenTrue : whenFalse
```

`?` remains reserved with no Sec 0.1 meaning.

## First-class range values

Not implemented in Sec 0.1:

```sec
let range := 0..<10
```

Ranges remain contextual to:

```text
for
membership
slicing
switch cases
other explicitly defined range contexts
```

## General field-level defaults

Not implemented:

```sec
type Config struct {
    port: Port = Port(8080),
}
```

Omitted fields use their type defaults.

## Multiple return values

Not implemented:

```sec
fn Read() (Value, Error)
```

A function returns one value.

Use a named struct, union, `Result`, or another explicit type.

## Separate interface impl syntax

Not implemented and explicitly rejected:

```sec
impl Interface for Type {
}
```

Interfaces are listed on the type declaration:

```sec
type Car struct implements Vehicle {
}
```

## `free` operation syntax

`free` is reserved for destruction semantics.

The parser creates an invalid statement and reports that it is not implemented.

Custom destruction remains governed by `destruction.txt` and future compiler
work.

## Panic and assertion statements

The lexer reserves spellings including:

```text
panic
assert
require
```

Complete parser and semantic syntax is not implemented.

`panic.md` and `runtime_checks.md` remain planned.

## General compile-time execution

Not implemented:

- arbitrary user compile-time functions;
- general compile-time blocks;
- compile-time I/O;
- compile-time allocation policy;
- macro syntax;
- token macros.

Constant expressions exist only in approved contexts.

## Classes and inheritance

Not implemented and not part of Sec:

```text
class
extends
object inheritance
virtual class methods
```

Sec uses:

```text
struct
impl
interface
union
```

## C-style loops

Explicitly rejected:

```sec
for i := 0; i < 10; i += 1 {
}
```

Use ranges or `while`.

## Condition-only `for`

Explicitly rejected:

```sec
for condition {
}
```

Use:

```sec
while condition {
}
```

## `do while`, `goto`, and labels

Not implemented.

## Switch expressions

Not implemented.

`switch` is a statement.

Use `match` when a value-producing exhaustive branch expression is required.

## Select expressions

Not implemented.

`select` is a statement.

## Arbitrary user-defined operators

Not implemented and deferred.

## Operator methods selected by spelling

Not implemented.

A token such as `+` does not dynamically resolve to an arbitrary user method.

## Generic enums

Not implemented.

## Field-level match destructuring

Not implemented.

## General collection patterns

Not implemented.

## Map and set literal syntax

Not finalized by this grammar.

Array literals and empty list literal syntax are defined separately.

## Process execution

`spawn process` is parsed but process spawning is intentionally deferred.

## Complete module grammar

Basic module and import syntax is implemented.

Still not implemented as a closed language area:

- import cycles;
- re-export syntax;
- selective imports;
- wildcard imports;
- module aliases beyond import aliases;
- explicit export lists;
- module initialization syntax;
- conditional imports;
- target-conditioned declaration blocks.

`modules.md` and `initialization.md` remain planned.

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

Only `target` is currently recognized after `#`.

---

# Module declaration

```text
ModuleDeclaration
    ::= "module" ModulePath

ModulePath
    ::= Identifier { "." Identifier }
```

Example:

```sec
module parser
```

Example:

```sec
module platform.linux.amd64
```

A module name is a semantic namespace.

The complete relationship between source path, `internal` directories, module
identity, and imports belongs to `modules.md` and `projects.txt`.

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

The string is an import path.

An optional identifier is a local import alias.

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
      | CompatibilityStructDeclaration
      | Comment
```

Ordinary executable statements are invalid at module scope.

Top-level mutable storage remains subject to static and initialization rules.

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
    ::= Identifier [ ":" TypeReference ]
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

The current syntax permits one constraint type after `:`.

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

---

# Assigned named type compatibility form

Implemented compatibility syntax:

```text
AssignedNamedTypeDeclaration
    ::= "type" Identifier [ GenericParameterList ] "=" TypeReference
        { TypeContract }
        [ DefaultClause ]
```

Example:

```sec
type UserID = uint64
```

Canonical new source should prefer:

```sec
type UserID uint64
```

unless a future alias rule explicitly gives `=` a distinct meaning.

---

# Compact variant compatibility form

Implemented compatibility syntax:

```text
CompactVariantDeclaration
    ::= "type" Identifier "=" Identifier Identifier { Identifier }
```

Example:

```sec
type IOError = FileNotFound AccessDenied InvalidValue
```

New closed alternatives should normally use:

```text
enum
union
```

This compact form must not be expanded without a dedicated modern rule.

---

# Implements clause

```text
ImplementsClause
    ::= "implements" TypeReference { "," TypeReference }
```

Examples:

```sec
type Car struct implements Vehicle {
    running: bool,
}
```

```sec
interface Vehicle implements Startable, Stoppable {
}
```

Separate `impl Interface for Type` syntax is invalid.

---

# Type contracts

```text
TypeContract
    ::= RangeContract
      | MembershipContract
      | MultipleOfContract
      | MarkerContract

RangeContract
    ::= "range" [ SignedNumericConstant ] RangeOperator [ SignedNumericConstant ]

RangeOperator
    ::= ".."
      | "..<"

MembershipContract
    ::= "in" "[" ConstantExpression { "," ConstantExpression } [ "," ] "]"

MultipleOfContract
    ::= Contextual("multipleOf") ConstantExpression

MarkerContract
    ::= Contextual("notEmpty")
      | Contextual("unique")
      | Contextual("finite")
      | Contextual("odd")
      | Contextual("even")
```

Contracts are written sequentially.

Sequential contracts are logical conjunction.

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
    ::= "default" ConstantExpression
```

The default clause follows every contract.

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

# Compatibility struct declaration

Implemented compatibility syntax:

```text
CompatibilityStructDeclaration
    ::= "struct" Identifier StructBody
```

Canonical source uses `type Name struct`.

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

Declared struct fields still require commas.

An empty literal is valid when every omitted field is defaultable:

```sec
Token {}
```

Sema inserts omitted field defaults.

---

# Enum declaration

```text
EnumDeclaration
    ::= "enum" Identifier [ EnumUnderlying ] EnumBody

TypeEnumDeclaration
    ::= "type" Identifier "enum" [ EnumUnderlying ] EnumBody

EnumUnderlying
    ::= [ ":" ] IntegerTypeReference
      | [ ":" ] BitUnderlying

BitUnderlying
    ::= Contextual("bit") [ "[" IntegerConstant "]" ]

EnumBody
    ::= "{" EnumValue { EnumValueSeparator EnumValue } [ "," ] "}"

EnumValueSeparator
    ::= ","
      | LineBreak

EnumValue
    ::= Identifier [ "=" ConstantExpression ]
```

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

Canonical enum value initialization uses `=`.

Parser recovery may accept `:` and emit a formatter warning.

An enum must declare at least one value.

---

# Union declaration

```text
UnionTypeDeclaration
    ::= "type" Identifier [ GenericParameterList ]
        "union"
        [ ImplementsClause ]
        UnionBody

UnionBody
    ::= "{" UnionVariant { [ "," ] UnionVariant } [ "," ] "}"

UnionVariant
    ::= Identifier
      | Identifier "(" TypeReference ")"
      | Identifier StructPayload

StructPayload
    ::= StructBody
```

Examples:

```sec
type State union {
    idle
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

Struct-like payload fields currently follow struct declaration field grammar and
require commas.

An empty union is invalid.

---

# Register declaration

```text
RegisterTypeDeclaration
    ::= "type" Identifier
        Contextual("register")
        "[" IntegerConstant "]"
        RegisterBody
        [ ImplementsClause ]

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

The total field width must match the register width.

`_` denotes reserved unnamed bits in register field position.

---

# Unit declaration

```text
UnitDeclaration
    ::= "unit" Identifier [ TypeReference ] [ UnitCategory ]

UnitCategory
    ::= Contextual("physical")
      | Contextual("currency")
      | Contextual("other")
```

Examples:

```sec
unit Hertz decimal<Hz>
unit Packet uint other
unit Metre decimal physical
unit Count
unit Euro currency
```

When no base type is written, the current parser uses `decimal`.

Complete unit metadata belongs to `units.txt`.

---

# Unit metadata inside impl

The parser recognizes contextual unit metadata names inside `impl`:

```text
dimension
scale
system
longName
symbol
baseUnit
status
```

Grammar shape:

```text
UnitMetadataDeclaration
    ::= ContextualUnitMetadataName ":" TokensUntilLineEnd
```

This syntax remains specialized and must be synchronized with `units.txt`.

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
    ::= FunctionSignature

InterfaceProperty
    ::= "property" Identifier ":" TypeReference
        "{"
        [ "get" ]
        [ Contextual("set") ]
        "}"

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

---

# Impl declaration

```text
ImplDeclaration
    ::= "impl" TypeReference ImplBody

ImplBody
    ::= "{" { ImplMember } "}"

ImplMember
    ::= FunctionDeclaration
      | StaticFunctionDeclaration
      | StaticLetDeclaration
      | PropertyDeclaration
      | EventDeclaration
      | NestedTypeDeclaration
      | NestedUnitDeclaration
      | NestedEnumDeclaration
      | UnitMetadataDeclaration
```

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

Getter
    ::= "get" Block

Setter
    ::= Contextual("set") Identifier Block

FallibleSetter
    ::= "try" Contextual("set") Identifier Block
```

Example:

```sec
property TopSpeed: Speed {
    get {
        return _speed
    }

    try set value {
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

The current parser skips an optional following block, but no canonical event
body syntax is defined here.

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
      | "ref" Identifier ":" TypeReference
      | "ref" "mut" Identifier ":" TypeReference
```

Examples:

```sec
fn Inspect(value: Token) void {
}
```

```sec
fn Read(ref value: Token) void {
}
```

```sec
fn Update(ref mut value: Token) void {
}
```

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
    ::= "extern" StringLiteral FunctionSignature [ FunctionBody ]
```

Example:

```sec
extern "C" fn write(
    fd: int32,
    buffer: RawPtr[byte],
    length: uint,
) int64
```

The optional-body form is parsed but remains semantically partial pending the
ABI and FFI rules.

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

Inside `impl`, `static` may modify `fn` or `let`.

Complete initialization order belongs to `static.md` and
`initialization.md`.

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
    ::= "try" PlaceExpression AssignmentOperator Expression TryHandlerBlock
```

Example:

```sec
try percent += Percent(value) {
    Err(error) => {
        discard error
    }
}
```

The handler block is mandatory for try assignment.

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
      | StaticDeclaration
      | Comment
```

Context restricts which statements are valid.

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
    ::= "return" [ Expression ]
```

Examples:

```sec
return
```

```sec
return value
```

There is no:

```sec
return <- value
```

Returning a move-only value transfers it according to ownership semantics.

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
        { MatchArm }
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

```sec
let number := match condition {
    _ where condition => 1
    _ => 0
}
```

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
      | Identifier
      | QualifiedIdentifier
      | CallPattern
      | LiteralPattern

CallPattern
    ::= QualifiedIdentifier "(" [ PatternArgument ] ")"

PatternArgument
    ::= Identifier
      | "_"
      | MatchPattern
```

The parser currently accepts a broader ordinary expression grammar as a
pattern.

Sema restricts valid patterns.

Examples:

```sec
Ok(value)
Err(error)
Option.Some(value)
Option.None
Color.red
_
```

Field-level payload destructuring is not part of Sec 0.1.

---

# Defer statement

```text
DeferStatement
    ::= "defer" Block
      | "defer" "return"
```

Examples:

```sec
defer {
    Close()
}
```

```sec
defer return
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

`detach` is currently a contextual identifier spelling in the parser.

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

The complete operand and constraint language is not yet closed.

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
`lexical_structure.md` and `types.txt`.

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
```

Examples:

```sec
[1, 2, 3]
```

```sec
[first..., second...]
```

The empty literal `[]` requires contextual type information.

Array and collection ownership rules apply to spread elements.

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

This is canonical but semantic resolution is not yet complete in the current
compiler.

It is not a struct literal.

---

# Lambda expression

```text
LambdaExpression
    ::= "fn" ParameterList TypeReference Block
```

Example:

```sec
let double := fn(value: int) int {
    return value * 2
}
```

The return type is required.

---

# Capture lambda expression

```text
CaptureLambdaExpression
    ::= "capture" "(" Identifier { "," Identifier } [ "," ] ")"
        LambdaExpression
```

Example:

```sec
let closure := capture(value) fn(input: int) int {
    return value + input
}
```

The parser currently records names.

Complete capture mode and escape semantics remain partial.

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
    ::= "{"
        [ "match" "{" { TryHandler } "}" | { TryHandler } ]
        "}"

TryHandler
    ::= MatchPattern "=>" TryHandlerBody

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

The explicit nested `match` wrapper is accepted for compatibility.

The direct handler form is canonical.

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

This current parser form must not be confused with general attributes.

The set of valid names is compiler-controlled.

---

# Spread expression

```text
SpreadExpression
    ::= Expression "..."
```

Spread is valid only in approved contexts.

Sec 0.1 approved contexts are governed by `spread.txt`, including:

```text
call arguments
array literals
struct literals
```

Spread is not a general standalone value operator.

---

# Range expression in contextual positions

```text
RangeExpression
    ::= [ Expression ] RangeOperator [ Expression ]
```

Range expressions are accepted only where the surrounding grammar explicitly
permits them.

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

Sema currently needs completion for fixed arrays and slices.

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
    ::= "fn"
        "(" [ TypeReference { "," TypeReference } [ "," ] ] ")"
        TypeReference
```

Example:

```sec
fn(int) bool
```

Parameter names do not appear in function type references.

---

# Unit annotation

```text
UnitAnnotation
    ::= "<" UnitTokens ">"
```

Examples:

```sec
decimal<m>
decimal<m/s>
Speed<km/h>
```

The unit parser currently preserves the inner token spelling as one unit
expression.

Unit algebra is semantic.

---

# Unit-only type reference

```text
UnitOnlyType
    ::= UnitAnnotation
```

This syntax is parsed for specialized unit contexts.

Its general use must remain synchronized with `units.txt`.

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

# Prefix sequence compatibility types

Accepted compatibility forms:

```text
PrefixFixedArrayType
    ::= "[" IntegerConstant "]" TypeReference

PrefixDynamicSequenceType
    ::= "[" "]" TypeReference
```

Canonical formatter output uses postfix syntax.

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

Capacity must currently be an integer literal.

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
task
thread
process
register
bit
physical
currency
other
inputs
outputs
clobbers
multipleOf
notEmpty
unique
finite
odd
even
address
```

A contextual spelling is not globally unavailable as an identifier unless the
lexer currently reserves it.

The grammar context determines its role.

---

# Currently globally reserved but intended contextual spelling

The current lexer reserves `set`.

The language design requires `set` to be contextual:

- collection type constructor in type position;
- property setter introducer in property position;
- otherwise available as an identifier where unambiguous.

This mismatch is partly implemented and must be corrected across lexer, parser,
formatter, highlighting, and LSP.

---

# Comments

The lexer recognizes comments.

Top-level comments may be preserved as AST comment statements.

Comments inside many statement blocks are currently skipped rather than retained
as ordinary AST statements.

Documentation comments and attachment rules must remain synchronized with
`lexical_structure.md`, formatter rules, and future documentation tooling.

Comments do not terminate or combine tokens except through normal lexical
separation.

---

# Reserved syntax

Reserved or recognized spellings without complete Sec 0.1 meaning include:

```text
?
free
panic
assert
require
```

The parser should issue focused diagnostics rather than generic token failures.

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

# Compatibility and recovery syntax

The parser may accept the following noncanonical forms only for migration,
formatting, or diagnostics:

```text
struct Name { ... }
type Name = ExistingType
type Error = First Second Third
prefix array syntax [N]Type
prefix sequence syntax []Type
enum value initializer Value: 1
explicit ref self parameter
explicit nested try match wrapper
body-bearing extern declaration until FFI grammar is finalized
```

Compatibility syntax must be marked in AST or diagnostics where needed.

The formatter must not silently assign new semantics.

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

It may normalize parser-confirmed compatibility forms where the transformation
is semantics-preserving.

Examples:

```text
func -> fn
Value: 1 -> Value = 1 inside enum declaration
prefix array type -> postfix array type
x++ -> x += 1 when parser support exists
x-- -> x -= 1 when parser support exists
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

# Implementation-status matrix

| Area | Lexer | Parser/AST | Sema | Current grammar status |
|---|---|---|---|---|
| `#target` | Implemented | Implemented | Basic validation | Implemented |
| module/import | Implemented | Implemented | Basic namespace validation | Implemented; full modules partial |
| named types | Implemented | Implemented | Implemented | Implemented |
| explicit defaults | Implemented | Implemented | Implemented | Implemented |
| contracts on named types | Implemented | Implemented | Implemented | Implemented |
| contracts on fields/variables | Accepted | Accepted | Legacy paths | Noncanonical; remove |
| structs | Implemented | Implemented | Implemented | Implemented |
| omitted struct fields | N/A | Omission represented | Defaults materialized | Implemented; backend audit |
| enums | Implemented | Implemented | Implemented | Implemented |
| unions | Implemented | Implemented | Implemented | Implemented; lowering partial |
| registers | Implemented | Implemented | Implemented | Implemented |
| units | Implemented | Implemented | Implemented | Implemented; metadata partial |
| interfaces | Implemented | Implemented | Conformance exists | Partly implemented |
| impl methods | Implemented | Implemented | Implemented | Implemented; lowering partial |
| properties | Implemented | Implemented | Shallow checks | Partly implemented |
| events | Contextual identifier | Implemented | Partial | Partly implemented |
| functions | Implemented | Implemented | Implemented | Implemented |
| generics | Implemented | Implemented | Substantial | Partly implemented |
| lambdas | Implemented | Implemented | Implemented paths | Partly implemented |
| copy/move declarations | Implemented | Implemented | Implemented | Implemented |
| compound assignment | Implemented | Implemented | Implemented paths | Partly implemented |
| if/for/while | Implemented | Implemented | Implemented | Implemented |
| switch | Implemented | Implemented | Implemented | Implemented |
| match | Implemented | Implemented | Exhaustiveness exists | Partly implemented |
| try/Result | Implemented | Implemented | Substantial | Partly implemented |
| defer/discard | Implemented | Implemented | Implemented | Implemented |
| spawn/await/select | Implemented | Implemented | Substantial | Partly implemented |
| unsafe | Implemented | Implemented | Basic context checks | Partly implemented |
| asm | Implemented | Implemented | Basic checks | Partly implemented |
| postfix arrays/slices | Implemented | Implemented | Implemented | Implemented |
| prefix arrays/slices | Implemented | Implemented | Implemented | Compatibility only |
| array literals | Implemented | Implemented | Implemented | Implemented |
| empty list literal | Tokens available | Parsed as typed braces | Not resolved as list literal | Not implemented |
| contextual `x` | Identifier | Implemented as contextual infix | Fixed matrix/matrix and matrix/vector validation | Partly implemented; lowering and tooling pending |
| unary `+` | Implemented | Implemented | Implemented for numeric operands and constants | Implemented; backend paths covered |
| `++`/`--` | Not implemented | Not implemented | Not implemented | Not implemented |
| general attributes | `@` available | Special cases only | Special cases only | Not implemented |
| `?` | Reserved | No canonical form | None | Reserved |
| `free` | Reserved | Explicit invalid node | Explicit error | Not implemented |
| panic/assert/require | Reserved | No complete forms | No complete forms | Not implemented |
| multiple returns | Delimiters available | Not implemented | Not implemented | Not implemented |
| first-class ranges | Operators available | Contextual only | Contextual only | Not implemented in 0.1 |

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
default_values.md
types.txt
contracts.md
units.txt
struct.txt
enums.txt
unions.txt
registers.txt
functions.txt
functions_lambda.txt
generics.txt
interfaces.txt
impl.txt
properties.txt
events.md
arrays-slices.txt
collections-shaped-types.md
spread.txt
flowcontrol_if.txt
flowcontrol_for.txt
flowcontrol_while.txt
flowcontrol_switch.txt
flowcontrol_match.txt
errorhandling.txt
defer.txt
discard.md
ownership.md
copy_move.md
references.txt
raw_pointers.txt
unsafe.md
inline_assembly.md
spawn.md
await.md
select.md
cancellation.md
diagnostics.txt
parser_recovery.md
semantic_ir.txt
language-rulebook-status.md
rules_implementations.txt
```

Planned files remain references to future canonical closure work.

---

# Appendix A — Codex synchronization plan

## A.1 Add the rulebook

Add:

```text
rules/grammar.md
```

Update:

```text
language-rulebook-status.md
rules/rules_implementations.txt
```

Mark `grammar.md` as Written.

Do not mark every grammar production as implemented merely because the document
exists.

---

## A.2 Generate or centralize parser grammar metadata

Do not maintain unrelated inventories for:

```text
statement starts
expression starts
type starts
precedence
contextual words
assignment operators
contract starts
impl member starts
```

Create shared parser metadata or generated tables where practical.

At minimum, add consistency tests.

---

## A.3 Unify expression-start handling

Implemented for the current prefix-expression inventory.

`isExpressionStart` contains every current prefix form and is used by ordinary
expression lookahead. Keep it synchronized when new prefix forms are added.

Use it for:

```text
ordinary expressions
defaults
contract values
arguments
range bounds where allowed
try expressions
match arms
parser recovery
```

Broader cross-context tests remain desirable for recovery paths and deliberately
restricted contexts.

---

## A.4 Add contextual `x`

Parser and fixed-shape Sema support are implemented.

Recognize:

```sec
left x right
```

only in infix position.

Preserve:

```sec
let x := 10
```

The AST retains operator `x` and uses the precedence from `operators.md`.
Semantic IR, backend lowering, formatter context and LSP semantic-token context
remain.

---

## A.5 Add unary plus

Implemented in parser, Sema, constant folding, MLIR, and LLVM expression
lowering.

Binary `+` remains unchanged. Parser, constant-folding and MLIR coverage has
been added.

---

## A.6 Add increment/decrement recovery

Add lexer and parser support for statement-only:

```sec
value++
value--
```

Represent them as parser-confirmed aliases or dedicated recovery nodes.

Formatter canonicalizes to:

```sec
value += 1
value -= 1
```

They must never produce expression values.

---

## A.7 Remove noncanonical field and variable contracts

Keep named-type contracts.

For inline field or variable contract syntax:

- emit focused diagnostics;
- suggest a named type;
- do not silently create a storage contract;
- update parser tests that currently expect acceptance.

---

## A.8 Normalize array type syntax

Keep postfix forms canonical:

```sec
T[N]
T[]
ref T[]
ref mut T[]
```

Either:

- retain prefix forms as recovery and normalize them; or
- reject them with a focused fix.

Do not document both as equal canonical forms.

---

## A.9 Resolve empty list literals

Recognize:

```sec
list[T] {}
list[T, Capacity] {}
```

as collection literals.

Do not route them exclusively through struct-type validation.

Create explicit AST or resolved literal category.

Implement empty, allocation-free Sema defaults.

---

## A.10 Mark compatibility declarations

Preserve source information for:

```text
standalone struct declaration
assigned type declaration
compact variant declaration
enum colon initializer
explicit ref self
```

Allow formatter or diagnostics to distinguish recovery syntax from canonical
syntax.

---

## A.11 Extern declaration distinction

Synchronize grammar with `ffi.txt` and future `abi.md`.

Define separate AST semantics for:

```text
foreign declaration without body
foreign-exported Sec definition with body
foreign shim
```

Do not let optional parser body presence remain the only distinction.

---

## A.12 Properties and contextual set

Make `set` contextual.

Update:

```text
lexer
parser
formatter
VS Code grammar
LSP semantic tokens
property diagnostics
collection type parsing
```

Add tests for an ordinary identifier named `set`.

---

## A.13 Event contextual syntax

Formalize `event` and `using` as contextual grammar spellings.

Do not reserve them globally unless a later decision requires it.

Add validation for event storage fields.

Remove parser behavior that silently skips an unrecognized event body unless a
body becomes canonical.

---

## A.14 Pattern AST

Introduce a dedicated pattern hierarchy rather than using unrestricted
expressions for all match and try patterns.

Initial pattern nodes should cover:

```text
wildcard
literal
qualified variant
variant with binding
identifier binding
```

Preserve current Sema behavior.

---

## A.15 Parser recovery

`parser_recovery.md` is written. The first structured diagnostic path and the
focused parser diagnostic registry exist; broader recovery metadata and invalid
nodes remain.

Replace ad hoc recovery with documented synchronization sets.

Add stable invalid nodes for LSP and formatter use.

Do not discard a complete following declaration after one malformed member.

---

## A.16 Reserved syntax diagnostics

Add focused parser diagnostics for:

```text
?
free
panic
assert
require
C-style for
condition-only for
assignment expression
multiple return syntax
separate impl Interface for Type
first-class range value
```

---

## A.17 Status tracking

Update implementation tracker with separate columns or entries for:

```text
lexer
parser
AST
Sema
analysis
Semantic IR
MLIR
direct LLVM
formatter
LSP
```

A grammar production may be implemented in the parser while incomplete in Sema
or lowering.

---

## A.18 Recommended implementation order

```text
1. Extend grammar consistency and fixture coverage.
2. Unify the remaining token-start and statement-start tables.
3. Add contextual x lowering and tooling context.
4. Add ++/-- recovery and formatter normalization.
5. Remove inline variable and field contracts.
6. Resolve empty list literals.
7. Normalize prefix array compatibility syntax.
8. Add dedicated pattern AST.
9. Make set and event contextual.
10. Close extern declaration distinctions.
11. Add focused reserved-syntax diagnostics.
12. Extend structured parser recovery and invalid nodes.
13. Synchronize formatter, LSP, and status trackers.
```

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

General attributes, first-class ranges, multiple returns, panic syntax, arbitrary
operators, and process implementation are not complete Sec 0.1 features.

The parser constructs syntax.

Sema assigns meaning.

No implementation shortcut may redefine the canonical grammar.

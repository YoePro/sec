# Rulebook Synchronization Corrections

## Purpose

This file is a one-time synchronization instruction for Codex.

It is not a new language rulebook.

Codex must use it to update the existing Sec rulebooks so they agree with the
new canonical documents and the language decisions recorded below.

The goal is to:

- remove contradictions;
- merge default-value semantics into the natural rulebooks;
- replace obsolete `.txt` rulebooks with their canonical `.md` replacements;
- update the rulebook status and implementation trackers;
- document implementation gaps without pretending they are language rules;
- avoid reopening decisions already made.

---

# Authoritative new rulebooks

The following files are authoritative and must be added to `rules/` if they are
not already present:

```text
ownership.md
lsp.md
formatter.md
copy_move.md
memory_model.md
operators.md
default_values.md
```

The following replacements are mandatory:

```text
formatter.txt
    -> formatter.md

copy_move.txt
    -> copy_move.md

memory_model.txt
    -> memory_model.md
```

Do not keep both the old and new filenames as canonical rulebooks.

Old files may be removed after every repository reference has been updated.

---

# Non-negotiable synchronization rules

Codex must not:

- invent alternative language syntax;
- restore implicit moves from ordinary `:=` or `=`;
- restore variable-level contracts;
- leave omitted struct fields undefined;
- treat backend `undef` as a default value;
- make immutable declarations default-initialized;
- add arbitrary operator overloading;
- assign a Sec 0.1 meaning to `?`;
- make ranges first-class values in Sec 0.1;
- make runtime string `+` valid in Sec 0.1;
- duplicate canonical rules in several conflicting forms;
- mark an implementation complete when only the rulebook exists.

When an existing document conflicts with an authoritative rulebook, the
authoritative rulebook wins.

---

# Canonical default syntax

The exact explicit-default syntax is now canonical:

```sec
type Percentage int range 0..100 default 0
type Port int range 1..65535 default 8080
type User string in ["Admin", "User", "Other"] default "User"
```

Grammar shape:

```text
NamedTypeDeclaration
    := "type" Identifier TypeDefinition { Contract } [ DefaultClause ]

DefaultClause
    := "default" ConstantExpression
```

The exact placement is:

```text
after all type contracts
```

Examples:

```sec
type Temperature int
    range -50..100
    default 20
```

```sec
type Role string
    in ["Admin", "User", "Other"]
    default "User"
```

`default` is already a language keyword in other grammar contexts.

The parser must resolve it from the named-type declaration context.

---

# Canonical primitive defaults

Add the following canonical table wherever primitive defaults are summarized:

| Type family | Default |
|---|---|
| signed integer | numeric zero |
| unsigned integer | numeric zero |
| binary float | numeric zero |
| decimal | numeric zero |
| `byte` | numeric zero |
| `bool` | `false` |
| `string` | `""` |
| `char` | zero character, written `0c` |
| `rune` | Unicode scalar zero, written `0r` |

The source spellings `0c` and `0r` are canonical explicit zero literals.

The semantic values must not be described as empty source literals such as
`''`.

---

# Canonical named-type defaults

A named type resolves its default in this order:

```text
1. Explicit `default` clause
2. Type-specific canonical rule
3. Constraint-derived default
4. Underlying-type default when valid
5. No default
```

An explicit default:

- is part of the type definition;
- must be a compile-time constant expression;
- must be representable;
- must satisfy every type contract;
- must not allocate;
- must not perform I/O;
- must not depend on runtime state;
- overrides every implicit default rule.

Invalid:

```sec
type Port int range 1..65535 default 0
```

Invalid:

```sec
type User string in ["Admin", "User", "Other"] default "Guest"
```

The compiler must reject the type declaration.

It must not silently choose another default.

---

# Canonical range defaults

For a constrained numeric type with no explicit default:

```text
1. Use zero when zero satisfies every type contract.
2. Otherwise use the unique valid representable value nearest zero.
3. If two or more valid values are equally near zero, require an explicit
   default.
4. If no valid representable value exists, reject the type.
```

Examples:

```sec
type Percent int range 0..100
```

Default:

```sec
Percent(0)
```

```sec
type Port int range 1..65535
```

Default:

```sec
Port(1)
```

```sec
type NegativeLevel int range -100..-5
```

Default:

```sec
NegativeLevel(-5)
```

```sec
type PositiveEven int range 1..100 even
```

Default:

```sec
PositiveEven(2)
```

Ambiguous:

```sec
type NonZeroOdd int range -9..9 odd
```

Both `-1` and `1` are equally near zero.

This declaration requires an explicit default:

```sec
type NonZeroOdd int range -9..9 odd default 1
```

Do not invent a positive or negative tie bias.

The same rule applies to integer, unsigned, float, decimal, and compatible
unit-bearing numeric types, using exact representability for the concrete type.

---

# Canonical `in [...]` defaults

For a type constrained by `in [...]` with no explicit default:

```text
the first listed value is the default
```

Example:

```sec
type User string in ["Admin", "User", "Other"]
```

Default:

```sec
User("Admin")
```

Example:

```sec
type RetryCount int in [1, 3, 5]
```

Default:

```sec
RetryCount(1)
```

An explicit default overrides the first-item rule:

```sec
type User string in ["Admin", "User", "Other"] default "Other"
```

Default:

```sec
User("Other")
```

The list order is semantically significant.

The formatter must preserve it.

The compiler must not sort `in [...]` values.

---

# `in [...]` and other contracts

Every value listed in `in [...]` must satisfy every other contract declared on
the type.

Invalid:

```sec
type SmallEven int in [1, 2, 3, 4] even
```

Reason:

```text
1 and 3 violate the `even` contract
```

Valid:

```sec
type SmallEven int in [2, 4, 6, 8] even
```

Default:

```sec
SmallEven(2)
```

Do not implement a model where invalid list entries are silently filtered.

The declared list is the type's allowed set and must itself be valid.

An empty `in []` set is invalid.

Duplicate values should be rejected because they add no semantic value and make
the declaration misleading.

---

# Contracts belong to types

This correction is mandatory.

Contracts belong to named types.

They do not attach directly to:

```text
variables
immutable bindings
mutable bindings
ordinary struct fields
individual storage locations
```

Remove or rewrite every rule and example that permits:

```sec
let mut percentage: int range 0..100 := 50
let mut role: string in ["admin", "user", "guest"] := "user"
let mut temperature: float finite := 20.0
```

These forms are invalid.

Use named types:

```sec
type Percentage int range 0..100
type Role string in ["admin", "user", "guest"]
type FiniteTemperature float finite

let mut percentage: Percentage := 50
let mut role: Role := "user"
let mut temperature: FiniteTemperature := 20.0
```

A struct field uses a named constrained type:

```sec
type Age int range 0..130

type User struct {
    age: Age,
}
```

Do not document inline field contracts such as:

```sec
type User struct {
    age: int range 0..130,
}
```

unless a later separate language decision explicitly restores that syntax.

The current canonical rule is named-type contracts only.

---

# Constrained assignment

Assignment to a constrained named type remains fallible and requires `try`.

Example:

```sec
type Percent int range 0..100

let mut percent: Percent := 10

try percent = Percent(value) {
    Err(error) => {
        discard error
    }
}
```

Compound assignment follows the same rule:

```sec
try percent += Percent(i) {
    Err(error) => {
        discard error
    }
}
```

Declaration initialization by a compile-time literal may be checked during
compilation:

```sec
let mut percent: Percent := 50
```

Runtime conversion into the constrained named type remains fallible.

Do not describe a hidden variable-level contract setter.

The advanced hidden-property model applies conceptually to mutation of a
constrained named-type value, not to a separate variable contract feature.

---

# Mutable and immutable declarations

A mutable typed declaration may omit its initializer only when the type is
defaultable.

Valid:

```sec
let mut count: int
let mut enabled: bool
let mut token: lexer.Token
let mut port: Port
```

These are semantically initialized with their type defaults.

Examples:

```sec
let mut count: int
```

is equivalent to:

```sec
let mut count: int := 0
```

```sec
let mut port: Port
```

uses `Port`'s resolved type default.

An immutable binding still requires an explicit initializer.

Invalid:

```sec
let count: int
```

Invalid:

```sec
int: a, b, c
```

The sentence:

```text
Sec does not create implicit zero constants.
```

may remain only in the explanation of immutable declarations.

It must not be used to deny default initialization of mutable storage or
omitted struct fields.

Type-first mutable declarations:

```sec
int mut: a, b, c
```

default-initialize all three variables.

---

# Defaultability

Every type is:

```text
Defaultable
NonDefaultable
```

Defaultability is independent of:

```text
copyability
move-only status
ownership
allocation
destruction
```

A type is defaultable only when the compiler can construct one deterministic,
valid value without hidden dynamic allocation or resource acquisition.

A non-defaultable mutable declaration requires an explicit initializer.

Example diagnostic:

```text
type File has no default value; initialize `file` explicitly
```

---

# Struct literal defaults

A struct is defaultable when every omitted stored field has a default.

All omitted stored fields are initialized from their declared field type.

Example:

```sec
type Position struct {
    line: int,
    column: int,
    valid: bool,
    name: string,
}
```

This is valid:

```sec
Position {}
```

It is semantically equivalent to:

```sec
Position {
    line: 0,
    column: 0,
    valid: false,
    name: "",
}
```

This:

```sec
Position {
    line: 10,
}
```

is semantically equivalent to:

```sec
Position {
    line: 10,
    column: 0,
    valid: false,
    name: "",
}
```

Omitted fields are not:

```text
undefined
uninitialized
zeroed blindly
ignored
optional
```

They receive semantic type defaults.

---

# Non-defaultable struct fields

A field may be omitted only when its type is defaultable.

Example:

```sec
type ResourceHolder struct {
    file: File,
    active: bool,
}
```

When `File` has no default, this is invalid:

```sec
ResourceHolder {
    active: true,
}
```

Required diagnostic:

```text
field `file` has no default value and must be initialized
```

The struct itself is non-defaultable when any required field has no default.

---

# Recursive struct defaults

Default initialization is recursive.

Example:

```sec
type Point struct {
    x: int,
    y: int,
}

type Window struct {
    origin: Point,
    visible: bool,
    title: string,
}
```

`Window {}` is valid and constructs:

```sec
Window {
    origin: Point {
        x: 0,
        y: 0,
    },
    visible: false,
    title: "",
}
```

The compiler must detect invalid by-value default-construction cycles.

---

# Struct spread and defaults

Resolve struct literals in this order:

```text
1. Explicit fields
2. Spread-provided fields
3. Duplicate/conflict validation
4. Default initialization of every remaining field
```

Defaults must not overwrite explicit or spread-provided fields.

---

# Struct formatter behavior

Ordinary formatting must preserve omitted fields.

It must not expand:

```sec
Position {
    line: 10,
}
```

into all defaulted fields.

Expansion is an optional LSP/refactoring action.

Struct field alignment remains governed by `formatter.md`.

---

# Backend correction for structs

The current backend behavior of starting a struct literal with undefined
aggregate data is not valid for omitted source fields.

Codex must update lowering so every field is initialized semantically.

Acceptable lowering strategies include:

```text
explicit field default construction
constant aggregate materialization
proven semantic zero initialization
field-by-field initialization
```

Backend `undef`, poison, or uninitialized bytes may not implement a source-level
default.

Zero filling may be used only after proving it represents the exact semantic
default for every affected field.

---

# Token example

Given:

```sec
type TokenType string

type Token struct {
    Type: TokenType,
    Lexeme: string,
    File: string,
    Line: int,
    Column: int,
}
```

this is valid:

```sec
Token {}
```

It constructs:

```sec
Token {
    Type: TokenType(""),
    Lexeme: "",
    File: "",
    Line: 0,
    Column: 0,
}
```

unless `TokenType` later declares another default.

---

# Empty `list[T]`

`list[T]` is defaultable.

Its default is an initialized empty list:

```text
length
    0

capacity
    0

element storage
    none

dynamic allocation
    none

initialized elements
    none
```

The default is independent of whether `T` itself is defaultable because the
empty list contains no elements.

Creating the empty default must not allocate.

Growth remains fallible and requires a permitted allocation strategy.

---

# Empty dynamic-list literal

The canonical explicit empty-list literal is:

```sec
list[T] {}
```

Example:

```sec
list[string] {}
```

It has the same semantic value as the default empty `list[string]`.

This is a collection literal, not a struct literal.

Parser, AST, Sema, formatter, and lowering must preserve that distinction.

---

# Empty bounded list

`list[T, Capacity]` is defaultable.

Its default has:

```text
length
    0

maximum capacity
    Capacity

initialized elements
    none

hidden growth allocation
    none
```

Canonical explicit form:

```sec
list[Packet, 32] {}
```

The exact backing representation remains a compiler and target decision.

No element is default-constructed merely because storage capacity exists.

---

# List growth

Default or empty construction does not imply that later growth is infallible.

For dynamic `list[T]`:

- growth requires an approved allocation context;
- allocation failure remains explicit;
- no global hidden heap is assumed.

For bounded `list[T, Capacity]`:

- no growth beyond `Capacity`;
- capacity exhaustion remains explicit;
- no hidden dynamic growth allocation.

---

# Map and set defaults

Do not generalize the new list-literal syntax mechanically.

`map` and `set` should eventually use empty default values consistent with their
collection rules, but Codex must not invent final literal syntax in this
correction unless those rulebooks already define it.

Record them as synchronization work if necessary.

This correction only locks:

```text
list[T]
list[T, Capacity]
```

because the bootstrap and current examples depend on them.

---

# Fixed-array defaults

A fixed array is defaultable when its element type is defaultable.

Every element receives the element default.

Example:

```sec
let mut values: int[4]
```

is semantically equivalent to:

```sec
let mut values: int[4] := [0, 0, 0, 0]
```

Example:

```sec
let mut tokens: lexer.Token[2]
```

default-constructs two `lexer.Token` values.

A fixed array with a non-defaultable element type is non-defaultable unless
fully initialized explicitly.

No partial undefined element storage may be exposed as a safe array value.

---

# Slice defaults

Retain the existing slice rule:

```text
a safe slice has no implicit default
```

A safe slice must have a valid origin.

A compiler-defined empty-storage sentinel may support explicit empty-slice
construction only when it obeys reference and lifetime rules.

Do not fabricate:

```text
null slice
dangling slice
originless ref T[]
```

This is intentionally different from owning `list[T]`.

---

# Other type families

Do not invent defaults from representation alone.

The following remain governed by their own rulebooks:

```text
enums
unions
Option[T]
Result[T, E]
references
RawPtr[T]
function values
closures
interfaces
resource handles
atomics
mutexes
tasks
threads
processes
```

A representation containing zero bits does not prove a valid semantic default.

Where a canonical rule already defines a default, add a reference from
`default_values.md`.

Where no rule exists, mark the type non-defaultable until designed.

---

# Explicit field defaults

Do not add field-level default syntax in this synchronization.

This is not yet a separate Sec 0.1 feature.

Omitted fields use their type defaults.

Example not canonical:

```sec
type Config struct {
    timeout: Duration = Duration(30),
}
```

An explicit type default remains canonical:

```sec
type Timeout Duration range Duration(1)..Duration(300) default Duration(30)
```

Exact range syntax for unit-bearing values remains governed by units and type
rules.

---

# Required changes to `rules/default_values.md`

Update the file so it no longer says the explicit `default` syntax is merely
illustrative.

Make this canonical:

```sec
type Port int range 1..65535 default 8080
```

Change every phrase equivalent to:

```text
first listed valid value
```

to:

```text
first listed value
```

Then add:

```text
every listed value must satisfy all other contracts
```

Retain:

```text
equally near-to-zero values require an explicit default
```

Add the canonical list defaults from this correction.

Retain slices as non-defaultable.

Do not add field-level default syntax.

---

# Required changes to `rules/types.txt`

Add `default_values.md` to the document's canonical scope and related documents.

Add a new section covering:

```text
primitive defaults
named-type default inheritance
explicit `default`
range-derived defaults
in-list defaults
defaultability
```

Update variable-declaration rules:

```text
mutable typed declaration without initializer
    valid only when type is defaultable

immutable declaration without initializer
    invalid
```

Update grammar sketches to include the explicit type default clause.

Clarify that:

```text
Sec does not create implicit zero constants
```

applies to immutable declarations and does not deny semantic default
initialization.

Add examples:

```sec
type Port int range 1..65535 default 8080
type User string in ["Admin", "User", "Other"]

let mut port: Port
let mut user: User
```

---

# Required changes to `rules/variables_contracts.txt`

This file currently contains major obsolete semantics.

Correct its purpose from variable contracts to type contracts.

Recommended rename:

```text
variables_contracts.txt
    -> contracts.md
```

If the rename is too disruptive in one patch, update its content first and
record the rename as pending.

Mandatory content changes:

- remove contracts on variables;
- remove contracts on ordinary struct fields;
- remove storage-contract identity;
- remove combining named-type and storage contracts;
- remove examples such as:
  - `let mut role: string in [...]`;
  - `let mut percentage: int range ...`;
  - `Age: int range ...`;
- retain contracts on named types;
- retain sequential composition;
- retain that every contract must pass;
- retain contract applicability;
- retain `try` for fallible assignment to constrained named types;
- add explicit defaults;
- add range-derived defaults;
- add first-item `in [...]` defaults;
- require every `in [...]` item to satisfy all other contracts;
- reject empty and duplicate `in [...]` lists.

The updated title should be:

```text
Type Contracts
```

Do not introduce:

```text
and
or
not
```

as contract-composition keywords.

Sequential contracts remain implicit conjunction.

---

# Required changes to `rules/struct.txt`

Add a canonical section:

```text
Struct default initialization
```

It must define:

- omitted fields use field-type defaults;
- `Type {}` creates the complete struct default when possible;
- explicit fields override defaults;
- spread fields override defaults;
- non-defaultable omitted fields are errors;
- recursive defaults;
- construction cleanup;
- no backend undefined fields;
- formatter preserves omission.

Update implementation status honestly:

```text
frontend accepts omitted fields
backend default materialization is incomplete
```

Do not describe current backend `undef` behavior as source semantics.

Add tests for:

```text
empty struct literal
partial explicit literal
nested defaults
constrained field defaults
non-defaultable omitted field
spread plus defaults
```

---

# Required changes to `rules/arrays-slices.txt`

Add:

```text
fixed arrays are defaultable when their element type is defaultable
```

Define elementwise default construction.

Retain:

```text
safe slices have no implicit default
```

Add cross-reference to `default_values.md`.

Add tests for:

```text
default numeric array
default struct array
non-defaultable element array
empty slice origin rules
```

Do not change slice ownership or origin semantics.

---

# Required changes to `rules/collections-shaped-types.md`

Add a canonical `list` default section:

```text
list[T]
    default empty, length 0, capacity 0, no allocation

list[T, Capacity]
    default empty, length 0, maximum Capacity, no initialized elements
```

Add canonical explicit literals:

```sec
list[T] {}
list[T, Capacity] {}
```

Clarify that this is collection-literal syntax.

It is not a struct literal.

Retain:

- no hidden global heap;
- dynamic growth is fallible;
- bounded capacity is enforced;
- empty construction does not allocate;
- no elements are initialized for an empty list.

Remove `list` literal syntax from the list of unresolved questions.

Do not decide map/set literal syntax in the same patch unless separately
documented.

---

# Required changes to `rules/memory_model.md`

Add or strengthen cross-references stating:

- a source default is a valid initialized value;
- default initialization begins a value lifetime;
- semantic default is not all-bits-zero;
- semantic default is not uninitialized storage;
- omitted fields must not remain undefined;
- mutable declarations without initializer perform initialization;
- list empty default owns no allocated element storage;
- slice remains non-defaultable because it requires a valid origin.

Do not duplicate the full default-value rulebook.

---

# Required changes to `rules/ownership.md`

Add a short cross-reference:

```text
default initialization establishes an available owned value when the type is
owning
```

Clarify:

- defaultability is independent of copyability;
- defaultability is independent of move-only status;
- default construction creates normal destruction responsibility;
- no default may silently acquire a unique external resource.

Do not rewrite ownership rules around defaults.

---

# Required changes to `rules/copy_move.md`

Add a short cross-reference:

- defaultability and copyability are independent;
- reinitialization may use a default-constructed value;
- omitted struct fields are constructed, not copied from hidden global values;
- empty list default construction is construction, not copy.

Do not change explicit move syntax.

---

# Required changes to `rules/formatter.md`

Add or retain:

- formatter preserves explicit type `default` clauses;
- formatter preserves `in [...]` order;
- formatter does not expand omitted struct fields;
- formatter formats `list[T] {}` as an empty collection literal;
- formatter must distinguish collection literal from struct literal;
- formatter must not insert explicit defaults during ordinary formatting.

Add type-default alignment only if the general declaration formatter already
supports it.

Do not create a special configurable style.

---

# Required changes to `rules/operators.md`

No semantic redesign is required.

Add a cross-reference where relevant:

- `default` is a declaration clause, not an operator;
- compile-time default expressions use constant-expression operator semantics;
- an invalid constant operator result makes the explicit default invalid;
- no runtime allocation is permitted in an explicit default expression.

Do not assign meaning to `?`.

---

# Required changes to `rules/lexical_structure.md`

Ensure `default` remains tokenized as a keyword.

Document its grammar contexts:

```text
switch/select default branch
named-type default clause
other already canonical default contexts
```

Do not make its meaning lexical.

The parser resolves context.

Ensure `0c` and `0r` remain canonical zero literals.

---

# Required future changes to `rules/grammar.md`

When `grammar.md` is created, include:

```text
NamedTypeDeclaration
DefaultClause
mutable omitted initializer
immutable required initializer
struct omitted fields
empty list literal
collection literal versus struct literal
```

Do not delay current document corrections until `grammar.md` exists.

---

# Required changes to `language-rulebook-status.md`

Update the inventory.

Minimum changes:

```text
operators.md
    Written — repository sync pending
    or Written when committed

default_values.md
    Written — repository sync pending
    or Written when committed

formatter.md
    Written — repository sync pending
    replaces formatter.txt

copy_move.md
    Written — repository sync pending
    replaces copy_move.txt

memory_model.md
    Written — repository sync pending
    replaces memory_model.txt

lsp.md
    Written — repository sync pending
    or Living when committed
```

Update existing rows:

```text
types.txt
    Written — sync required
    include defaults and mutable initialization

variables_contracts.txt
    Written — major correction required
    contracts belong to named types only
    recommend rename to contracts.md

struct.txt
    Written — sync required
    omitted fields and semantic defaults

arrays-slices.txt
    Written — sync required
    array defaults and slice non-defaultability

collections-shaped-types.md
    Written — sync required
    list defaults and empty literals
```

Remove `operators.md` from the planned list after adding it.

Add `default_values.md` to the canonical written set.

Replace old filenames in all canonical lists:

```text
formatter.txt -> formatter.md
copy_move.txt -> copy_move.md
memory_model.txt -> memory_model.md
```

Move `lsp.md` out of Candidate after adding it.

Keep:

```text
layout.md
```

as Planned.

Remove duplicate or contradictory entries for:

```text
names_scopes_visibility.md
```

if it appears as both written and planned.

Update contextual `x` references from `formatter.txt` to `formatter.md`.

---

# Required changes to `rules_implementations.txt`

Inspect the file's existing structure and preserve its format.

Add or update entries for:

```text
default values
explicit type defaults
range-derived defaults
in-list defaults
mutable default initialization
omitted struct fields
empty list literals
array defaults
slice non-defaultability
```

Accurate implementation states should initially be:

```text
primitive default semantics
    rule written; implementation audit required

explicit type default grammar
    not implemented or partial

range-derived default resolution
    not implemented or partial

in-list default resolution
    not implemented or partial

mutable typed declaration without initializer
    parser/Sema support exists in part;
    semantic default lowering audit required

omitted struct field defaults
    frontend accepts omission;
    backend semantic default fill incomplete

list[T] {}
    not implemented or partial

fixed-array defaults
    implementation audit required

safe slice default
    intentionally unsupported
```

Update renamed rulebook references.

Do not mark features implemented merely because current code happens to produce
zero for a subset of types.

---

# Required compiler changes

This correction primarily updates documentation, but Codex should record or
implement the direct compiler gaps where requested.

## Default resolver

Add one compiler-owned default-resolution API.

Conceptual form:

```go
type DefaultResolutionKind int

const (
    DefaultUnavailable DefaultResolutionKind = iota
    DefaultPrimitive
    DefaultUnderlying
    DefaultRangeDerived
    DefaultInList
    DefaultExplicit
    DefaultAggregate
    DefaultCollection
)
```

Conceptual query:

```go
func DefaultValueOf(t Type) DefaultResolution
```

The exact Go API may differ.

It must be shared by:

```text
Sema
constant evaluation
struct literals
mutable declarations
array initialization
collection initialization
Semantic IR
LSP
```

Do not duplicate default selection logic.

---

## Explicit default parser support

Parse:

```sec
type Port int range 1..65535 default 8080
```

Preserve:

```text
default token
expression
source range
constant value
```

Reject `default` clauses on declaration forms where they are not defined.

---

## Constant validation

Explicit defaults must be compile-time constants.

Use Sec operator semantics.

Reject:

```text
runtime function call
allocation
I/O
volatile access
FFI
fallible runtime conversion
invalid arithmetic
contract violation
```

---

## Contract declaration validation

For `in [...]`:

- reject empty lists;
- reject duplicates;
- validate every listed value against base type;
- validate every listed value against every additional contract;
- preserve source order;
- select first value as implicit default.

For ranges:

- resolve exact representable nearest-to-zero value;
- detect ties;
- require explicit default for ties;
- validate explicit default.

---

## Mutable declaration initialization

When a mutable typed declaration omits an initializer:

1. resolve type;
2. request type default;
3. reject non-defaultable type;
4. emit semantic initialization;
5. begin value lifetime;
6. mark binding Available.

Do not leave storage uninitialized.

---

## Struct literal completion

For every struct literal:

1. resolve explicit fields;
2. resolve spreads;
3. reject duplicates and unknown fields;
4. identify omitted stored fields;
5. request every omitted field default;
6. reject non-defaultable omissions;
7. build one complete semantic aggregate;
8. record cleanup for constructed nontrivial fields.

---

## Empty list parsing

Recognize:

```sec
list[T] {}
list[T, Capacity] {}
```

as collection literals.

Do not send them through named-struct literal validation.

Add an AST representation or a resolved compiler-known literal category.

---

## Array default initialization

For a defaulted fixed array:

- require defaultable element type;
- construct every element;
- track partial construction cleanup;
- permit constant aggregate or zero-fill optimization only after proof.

---

## Semantic IR

Add explicit origins for:

```text
ExplicitInitialization
ImplicitMutableDefault
OmittedFieldDefault
AggregateDefault
ArrayElementDefault
EmptyListDefault
ExplicitTypeDefault
RangeDerivedDefault
InListDefault
```

Do not lower source defaults through `undef`.

---

## Backend verification

Add an internal verifier:

```text
no readable source value contains undefined fields or elements
```

Verify:

- all struct fields initialized;
- all defaulted array elements initialized;
- named constrained defaults valid;
- empty list descriptor fully initialized;
- no safe slice fabricated without origin.

---

# Required diagnostics

Add stable diagnostics for:

```text
invalid explicit default
default violates contract
default not representable
ambiguous nearest-to-zero default
type has no default
empty in-list
duplicate in-list value
in-list value violates another contract
missing non-defaultable struct field
mutable non-defaultable declaration requires initializer
immutable declaration requires initializer
empty list literal unsupported by current backend
```

Diagnostics should include:

```text
type declaration
contract location
default clause
omitted field
conflicting nearest values
```

---

# Required tests

Create or update:

```text
default_values_valid.sec
default_values_invalid.sec
default_ranges_valid.sec
default_ranges_invalid.sec
default_in_contract_valid.sec
default_in_contract_invalid.sec
default_structs_valid.sec
default_structs_invalid.sec
default_lists_valid.sec
default_lists_invalid.sec
default_arrays_valid.sec
default_arrays_invalid.sec
```

Minimum valid cases:

```sec
type Port int range 1..65535
type HttpPort int range 1..65535 default 8080
type User string in ["Admin", "User", "Other"]
type PreferredUser string in ["Admin", "User", "Other"] default "User"

let mut port: Port
let mut httpPort: HttpPort
let mut user: User
let mut preferred: PreferredUser
let mut count: int
let mut text: string
let mut runeValue: rune
let mut charValue: char
let mut values: int[4]
let mut errors: list[string]

let empty := list[string] {}
```

Minimum invalid cases:

```sec
type BadPort int range 1..65535 default 0
type EmptyRole string in []
type DuplicateRole string in ["Admin", "Admin"]
type InvalidEven int in [1, 2, 3] even
type Ambiguous int range -9..9 odd

let immutable: int
let mut file: File
```

Struct tests must include:

```sec
type TokenType string

type Token struct {
    Type: TokenType,
    Lexeme: string,
    Line: int,
}

let empty := Token {}
let partial := Token {
    Lexeme: "value",
}
```

---

# Parser bootstrap result

After these corrections, the intended parser constructor is valid:

```sec
/**
 * Creates a parser that owns the supplied lexer.
 *
 * The parser initializes its diagnostic collections and reads the first two
 * tokens so that both the current and lookahead token are available.
 */
fn New(l: lexer.Lexer) Parser {
    let mut p := Parser {
        l: l,
        errors: list[string] {},
        warnings: list[string] {},
    }

    p.nextToken()
    p.nextToken()

    return p
}
```

This requires:

```text
list[string] {}
    initialized empty list

omitted lexer.Token fields
    recursive semantic defaults

omitted bool fields
    false
```

Ownership of `l` remains governed by `copy_move.md`.

If `lexer.Lexer` is move-only, the by-value call consumes it.

If it is copyable, the call copies it.

This correction does not declare `lexer.Lexer` move-only.

That is a type-specific implementation decision.

---

# Acceptance criteria

The synchronization is complete only when:

- the old replacement filenames are no longer canonical;
- contracts are documented only on named types;
- explicit `default` syntax is documented consistently;
- primitive defaults are consistent;
- range defaults are consistent;
- `in [...]` defaults are consistent;
- omitted struct fields are never described as undefined;
- mutable declarations use semantic defaults;
- immutable declarations still require initializers;
- list empty literal semantics are documented;
- array defaults and slice non-defaultability are documented;
- status files reflect the new rulebooks;
- implementation trackers distinguish rules from code;
- repository links and filenames are valid;
- tests cover every corrected rule;
- no conflicting example remains.

---

# Recommended Codex execution order

```text
1. Add the new canonical rulebooks.
2. Rename/remove replaced rulebooks.
3. Correct default_values.md.
4. Correct variables_contracts.txt semantics.
5. Update types.txt.
6. Update struct.txt.
7. Update arrays-slices.txt.
8. Update collections-shaped-types.md.
9. Add short cross-references to memory/ownership/copy/formatter/operators.
10. Update lexical_structure.md.
11. Update language-rulebook-status.md.
12. Update rules_implementations.txt.
13. Implement or record compiler gaps.
14. Add tests.
15. Search the repository for obsolete statements and filenames.
```

Final repository searches should include:

```text
formatter.txt
copy_move.txt
memory_model.txt
operators.md | Planned
contracts may be attached to variables
variable-level contracts
struct fields may declare contracts
implicit zero constants
omitted field
llvm.mlir.undef
list[T] {}
default_values.md
```

Every surviving occurrence must be intentional and consistent.

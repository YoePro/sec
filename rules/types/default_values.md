# Default Values

- **Status:** Normative
- **Created:** 2026-08-13
- **Last updated:** 2026-08-13
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/types/default_values.md`

## Status

This document is the canonical default-value and default-initialization rulebook
for Sec.

The canonical filename is:

```text
default_values.md
```

This rulebook records the language decisions for:

- primitive defaults;
- named-type defaults;
- constrained defaults;
- `range` defaults;
- `in [...]` defaults;
- explicit type defaults;
- omitted struct fields;
- recursive aggregate defaults;
- compiler validation;
- lowering and backend behavior.

This rulebook defines semantics.

The explicit default clause has the canonical spelling:

```sec
type Port int range 1..65535 default 8080
```

Its grammar is `"default" ConstantExpression`, after every type contract in a
named-type declaration.

Implementation progress is tracked exclusively by `frontend.default-values` in
`implementation-status.yaml`.

---

# Purpose

A default value is the value the language uses when a value is intentionally
default-initialized.

Default initialization is not:

```text
uninitialized storage
undefined backend data
zeroing arbitrary bytes
a failed constructor
an implicit allocation
an implicit resource acquisition
```

A default value is a valid semantic value of its type.

The compiler must be able to construct it deterministically.

---

# Core rule

Every type is either:

```text
Defaultable
NonDefaultable
```

A defaultable type has exactly one compiler-resolved default value.

A non-defaultable type cannot be omitted from a context that requires default
initialization.

The compiler must never treat undefined storage as a default value.

---

# Default precedence

The compiler resolves the default value of a type in this order:

```text
1. Explicit default declared by the type
2. Type-specific canonical default rule
3. Constraint-derived default
4. Underlying-type default when inheritance is permitted
5. No default
```

An explicit type default overrides every implicit default rule.

An explicit default must satisfy every rule and contract of the type.

---

# Primitive defaults

| Type family | Default |
|---|---|
| signed integer | numeric zero |
| unsigned integer | numeric zero |
| binary float | numeric zero |
| decimal | numeric zero |
| `byte` | numeric zero |
| `bool` | `false` |
| `string` | `""` |
| `char` | zero character, written `0t` |
| `rune` | Unicode scalar zero, written `0r` |

## Numeric types

All ordinary numeric primitive types default to zero.

This includes:

```text
int
int8
int16
int32
int64
int128
int256

uint
uint8
uint16
uint32
uint64
uint128
uint256

float
float32
float64

decimal
decimal128
```

where supported by the compiler and target.

Examples:

```sec
let mut count: int
let mut ratio: float64
let mut amount: decimal
```

Their initial values are semantically:

```sec
0
0.0
0.0
```

according to the destination type.

The exact physical bit representation remains target- and type-defined.

The temporal types `date`, `time`, `datetime`, and `duration` are not included
in this primitive-default table. Their defaultability is intentionally
undecided until the temporal and default-value rulebooks define it explicitly;
an implementation must not infer a zero, epoch, empty, or backend-derived
default for them.

---

## Boolean

`bool` defaults to:

```sec
false
```

Example:

```sec
let mut enabled: bool
```

is equivalent to:

```sec
let mut enabled: bool := false
```

---

## String

`string` defaults to the empty string:

```sec
""
```

Example:

```sec
let mut name: string
```

is equivalent to:

```sec
let mut name: string := ""
```

The empty string default must not require dynamic allocation.

---

## Character

`char` defaults to the zero character value.

The canonical explicit zero literal is:

```sec
0t
```

Example:

```sec
let mut character: char
```

is equivalent to:

```sec
let mut character: char := 0t
```

---

## Rune

`rune` defaults to Unicode scalar value zero.

The canonical explicit zero literal is:

```sec
0r
```

Example:

```sec
let mut value: rune
```

is equivalent to:

```sec
let mut value: rune := 0r
```

---

# Named primitive types

A named type based on a defaultable primitive type inherits the primitive
default when:

- the named type declares no explicit default;
- the inherited value satisfies every contract;
- no type-specific rule disables default inheritance.

Example:

```sec
type UserName string
```

Default:

```sec
UserName("")
```

Example:

```sec
type Counter uint64
```

Default:

```sec
Counter(0)
```

Nominal identity is preserved.

The result is a value of the named type, not the underlying primitive type.

---

# Explicit type defaults

A named type may declare an explicit default value using the canonical syntax:

```sec
type Port int range 1..65535 default 8080
```

The clause appears after every type contract:

```text
NamedTypeDeclaration
    := "type" Identifier TypeDefinition { Contract } [ DefaultClause ]

DefaultClause
    := "default" ConstantExpression
```

The declared default:

- must be a compile-time constant expression;
- must be representable by the type;
- must satisfy every contract;
- must satisfy every unit rule;
- must not allocate;
- must not perform I/O;
- must not depend on runtime state;
- must not call a fallible runtime constructor;
- becomes the type's canonical default value.

Example:

```sec
type Port int range 1..65535 default 8080
```

Default:

```sec
Port(8080)
```

Example:

```sec
type User string in ["Admin", "User", "Other"] default "User"
```

Default:

```sec
User("User")
```

---

# Explicit default overrides implicit rules

Example:

```sec
type Temperature int range -50..100 default 20
```

Although zero satisfies the range, the default is:

```sec
Temperature(20)
```

Example:

```sec
type User string in ["Admin", "User", "Other"] default "Other"
```

Although `"Admin"` is the first listed value, the default is:

```sec
User("Other")
```

---

# Invalid explicit defaults

Invalid:

```sec
type Port int range 1..65535 default 0
```

Reason:

```text
0 does not satisfy range 1..65535
```

Invalid:

```sec
type User string in ["Admin", "User", "Other"] default "Guest"
```

Reason:

```text
"Guest" is not a valid member of the type
```

Invalid:

```sec
type PositiveEven int range 1..100 even default 3
```

Reason:

```text
3 violates the even contract
```

The declaration itself is a compile-time error.

The compiler must not silently replace an invalid explicit default with an
implicit value.

---

# Range-constrained numeric types

A range-constrained numeric type without an explicit default uses the valid value
nearest to zero.

The complete type contracts participate in determining validity.

---

## Zero inside the valid range

Example:

```sec
type Percent int range 0..100
```

Default:

```sec
Percent(0)
```

Example:

```sec
type Offset int range -100..100
```

Default:

```sec
Offset(0)
```

---

## Positive-only range

Example:

```sec
type Port int range 1..65535
```

Default:

```sec
Port(1)
```

`1` is the valid value nearest to zero.

---

## Negative-only range

Example:

```sec
type Temperature int range -100..-5
```

Default:

```sec
Temperature(-5)
```

`-5` is the valid value nearest to zero.

---

## Exclusive range bounds

Exclusive bounds participate in default calculation.

Example:

```sec
type Positive int range 0..<100
```

When zero is valid under the exact range syntax, default is zero.

Example conceptually excluding zero:

```sec
type Positive int range 0<..100
```

If such lower-exclusive syntax is supported, default is the smallest valid
positive value.

Exact range syntax remains governed by the range rulebook.

---

# Additional numeric contracts

The default must satisfy all contracts, not only `range`.

Example:

```sec
type PositiveEven int range 1..100 even
```

Valid values nearest to zero begin with:

```text
2
4
6
...
```

Default:

```sec
PositiveEven(2)
```

Example:

```sec
type NegativeEven int range -100..-1 even
```

Default:

```sec
NegativeEven(-2)
```

---

# Ambiguous nearest-to-zero values

When two different valid values are equally near zero and no other canonical
rule selects one, the type has no implicit default.

The type must declare an explicit default.

Example conceptually:

```sec
type NonZeroOdd int range -9..9 odd
```

The nearest valid values may be:

```text
-1
1
```

Neither is closer to zero.

The compiler must require an explicit declaration such as:

```sec
type NonZeroOdd int range -9..9 odd default 1
```

This avoids an arbitrary positive or negative bias.

---

# Floating and decimal ranges

The same nearest-to-zero rule applies to constrained floating and decimal types.

Example:

```sec
type PositiveAmount decimal range 0.01..1000000.00
```

Default:

```sec
PositiveAmount(0.01)
```

The selected value must be exactly representable by the declared type and scale.

If there is no unique nearest representable valid value, an explicit default is
required.

---

# `in [...]` constrained types

A type constrained by an ordered `in [...]` list uses the first listed value as
its implicit default.

Example:

```sec
type User string in ["Admin", "User", "Other"]
```

Default:

```sec
User("Admin")
```

The list order is semantically significant for default selection.

The compiler and formatter must preserve the declared order.

---

## Numeric `in [...]`

Example:

```sec
type RetryCount int in [1, 3, 5]
```

Default:

```sec
RetryCount(1)
```

The nearest-to-zero rule does not replace the first-item rule for an explicit
`in [...]` list.

---

## Boolean `in [...]`

Example:

```sec
type RequiredBool bool in [true]
```

Default:

```sec
RequiredBool(true)
```

The underlying primitive default `false` is invalid and is not used.

The first listed value is used.

---

## Explicit default for `in [...]`

Example:

```sec
type User string in ["Admin", "User", "Other"] default "User"
```

Default:

```sec
User("User")
```

The explicit declaration overrides the first-item rule.

---

## Empty `in [...]` list

A type with an empty `in [...]` constraint is invalid.

Invalid:

```sec
type Impossible int in []
```

Such a type has no valid values and cannot have a valid default.

---

## Duplicate values

Duplicate entries in `in [...]` are invalid and must be rejected.

---

# Multiple constraints including `in [...]`

Every value in `in [...]` must satisfy every other contract on the type. The
first listed value is the implicit default.

Example:

```sec
type SmallEven int in [2, 4, 6, 8] even
```

Default:

```sec
SmallEven(2)
```

This is invalid because some listed values violate `even`:

```sec
type InvalidEven int in [1, 2, 3, 4] even
```

Invalid entries are declaration errors; they are never silently filtered.

---

# Defaults and contracts

A default is part of the type's semantic definition.

The compiler must prove at declaration time that the default satisfies:

```text
range
in [...]
odd
even
multipleOf
finite
regex
length
unit
unique where applicable
other canonical contracts
```

A default is not revalidated at every ordinary use after the declaration has
been proven.

---

# Defaults and units

A unit-bearing numeric type normally defaults to numeric zero when zero is valid.

Example:

```sec
type Distance decimal<m>
```

Default:

```sec
Distance(0)
```

A constrained unit-bearing type follows its constraints.

Example:

```sec
type PositiveDistance decimal<m> range 0.1..1000.0
```

Default:

```sec
PositiveDistance(0.1)
```

An explicit unit-bearing default must have a compatible unit and exact
conversion.

---

# Struct defaults

A struct type has a default value when every stored field has a default value.

The struct default is constructed field by field in declaration order.

Example:

```sec
type Position struct {
    line: int,
    column: int,
    valid: bool,
    name: string,
}
```

Default:

```sec
Position {
    line: 0,
    column: 0,
    valid: false,
    name: "",
}
```

---

# Empty struct literal

A defaultable struct may be written:

```sec
Position {}
```

This constructs the struct default.

It does not create uninitialized fields.

---

# Omitted struct fields

Stored fields omitted from a struct literal are initialized with their declared
type's default value.

Example:

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

The compiler may lower the two forms identically.

---

# Nested struct defaults

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

Default:

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

Therefore:

```sec
Window {}
```

is valid.

---

# Explicit field values override defaults

Example:

```sec
Window {
    visible: true,
}
```

uses:

```text
origin
    Point default

visible
    true

title
    string default
```

No field may be initialized twice.

---

# Non-defaultable struct fields

A struct literal may omit a field only when that field's type is defaultable.

Example conceptually:

```sec
type ResourceHolder struct {
    file: File,
    active: bool,
}
```

If `File` has no default, this is invalid:

```sec
ResourceHolder {
    active: true,
}
```

Diagnostic:

```text
field `file` has no default value and must be initialized
```

The complete struct type may itself be non-defaultable.

---

# Field-level defaults

Field-level default syntax is not defined by this rulebook unless already
defined elsewhere.

A future rule may permit:

```sec
type Config struct {
    timeout: Duration = Duration(30),
}
```

or another canonical spelling.

Until that syntax is explicitly approved:

- defaults belong to field types;
- struct literals may provide explicit field values;
- omitted fields use their type defaults.

---

# Struct spread and defaults

A struct spread provides fields from the spread source according to `spread`
rules.

Conceptual resolution order:

```text
1. explicit source fields and spreads
2. duplicate and conflict validation
3. omitted-field default initialization
```

Defaults do not overwrite fields supplied by spread.

---

# Parser example

Given:

```sec
type Parser struct {
    l: lexer.Lexer,
    errors: list[string],
    warnings: list[string],
    curToken: lexer.Token,
    peekToken: lexer.Token,
    stopBeforeBrace: bool,
    inRefExpression: bool,
}
```

This literal:

```sec
Parser {
    l: l,
    errors: list[string] {},
    warnings: list[string] {},
}
```

is valid only when:

- `lexer.Lexer` can be moved or copied into `l`;
- `list[string] {}` is a valid explicit empty-list construction;
- `lexer.Token` is defaultable;
- `bool` is defaultable.

The omitted fields become:

```sec
curToken: lexer.Token {}
peekToken: lexer.Token {}
stopBeforeBrace: false
inRefExpression: false
```

---

# Token example

Given:

```sec
type Token struct {
    Type: TokenType,
    Lexeme: string,
    File: string,
    Line: int,
    Column: int,
}
```

and:

```sec
type TokenType string
```

then:

```sec
Token {}
```

uses:

```sec
Token {
    Type: TokenType(""),
    Lexeme: "",
    File: "",
    Line: 0,
    Column: 0,
}
```

unless `TokenType` declares another explicit or constrained default.

---

# Arrays

The canonical fixed-array rule is:

```text
an array is defaultable when its element type is defaultable;
every element receives the element default
```

Example:

```sec
let mut values: int[4]
```

is semantically equivalent to:

```sec
[0, 0, 0, 0]
```

A zero-length fixed array is defaultable without constructing an element.

---

# Owning dynamic arrays

`T[]` is defaultable independently of whether `T` is defaultable. Its default
is an initialized, non-allocating empty owner with length and capacity zero and
no initialized elements. Default construction does not construct a `T`.

This rule applies only to the owning dynamic array. It does not create a null
or origin-free default for a safe slice.

---

# Slices

A slice is a non-owning view.

A universal implicit slice default must not be invented without defining:

```text
origin
bounds
lifetime
mutability
empty-view representation
```

An explicit empty slice literal may be defined separately.

A safe slice has no implicit default and is non-defaultable.

---

# List defaults

`list[T]` is defaultable independently of whether `T` is defaultable. Its
default is an initialized empty list with length and capacity zero, no element
storage, no initialized elements and no allocation.

`list[T, Capacity]` is likewise defaultable. Its default has length zero,
maximum capacity `Capacity`, no initialized elements and no hidden growth
allocation.

The canonical explicit forms are:

```sec
list[string] {}
list[Packet, 32] {}
```

These are collection literals, not struct literals. Empty construction does not
allocate or default-construct elements. Later dynamic growth remains fallible
and requires an approved allocation context. Map and set literal syntax remains
governed by their own collection rules.

---

# Enums

Every valid enum is non-empty and defaultable. Its default is the single member
marked `default`, when present, and otherwise its first declared member. Member
identity selects the default; underlying numeric zero has no special priority.

```sec
enum Status int {
    UNKNOWN = 10,
    ACTIVE = 20,
}

enum ConnectionState {
    CONNECTING,
    CONNECTED,
    DISCONNECTED default,
}
```

The defaults are `Status.UNKNOWN` and `ConnectionState.DISCONNECTED`.
The marker does not alter the initializer, `iota`, aliases, or initializer
repetition. For an open bit-backed enum, the default is still a declared member;
an unnamed raw pattern is never selected merely because it is zero.

A mutable enum declaration without an initializer is initialized with the
resolved enum default. This is initialized storage and does not require a later
definite assignment before first read. An enum used in an omitted struct field
uses the same resolved default.

---

# Unions

A union has no implicit first-variant default.

A union is `Defaultable` only when exactly one declared variant is explicitly
marked `default` and that variant can be default-constructed.

- A payload-less default variant is constructible.
- A single-payload default variant requires a Defaultable payload type.
- A struct-like default variant uses the normal omitted-field/default rules and
  is valid only when all required payload state can be constructed.

```sec
type State union {
    Idle default
    Running
}

let mut state: State
```

The resolved default is `State.Idle`.

A mutable binding of a NonDefaultable union has one explicit exception to the
general omitted-initializer rule:

```sec
type PendingState union {
    Idle
    Running
}

let mut pending: PendingState
```

The binding begins in the compiler-known `empty` initialization state. `empty`
is not a default value, a valid union value, `null`, `None`, `Nothing`, or a
hidden union variant. The binding must contain a real union value before it may
escape or be used in a context requiring a value. `is empty` and the `empty`
match pattern inspect this initialization state under the union and match
rules.

Immutable union bindings still require an explicit initializer.

A struct-like union payload construction may omit a field whose type is
Defaultable; that field receives its normal resolved default. A NonDefaultable
payload field must be supplied explicitly.

---

# Option and Result

`Option[T]` and `Result[T, E]` do not receive an implicit default merely from
their representation.

Possible defaults such as:

```text
None
Ok(default(T))
Err(default(E))
```

must be explicitly defined by their canonical core rules.

Representation zero is not enough.

---

# References

Safe references have no universal default.

The language must not fabricate:

```text
null ref T
dangling ref T
reference to temporary default storage
```

Therefore:

```text
ref T
ref mut T
```

are non-defaultable unless a specific wrapper type defines optional reference
semantics.

---

# Raw pointers

`RawPtr[T]` default semantics must be defined by `raw_pointers.txt`.

If a null raw pointer value is supported, it may be a valid explicit raw-pointer
default.

This must not imply a valid reference or owned allocation.

No reference default may be derived from raw-pointer zero.

---

# Resources

Unique resource types normally have no implicit default.

Examples:

```text
File
Socket
DeviceHandle
MutexGuard
Task
Thread
Process
Allocation owner
```

A closed, detached, invalid, or null representation is not automatically a valid
semantic default.

A resource type may define an explicit valid default only when its contract
makes that state meaningful.

---

# Function values and closures

Function-value defaults require a canonical callable or empty-callable model.

The language must not fabricate an invalid call target.

Until explicitly defined, function and capturing closure types are
non-defaultable.

---

# Interfaces

General interface values require a defined empty or default dynamic state.

Until owned and borrowed erased representations define such a state, general
interface types are non-defaultable.

A specific interface wrapper may define an explicit default.

---

# Static values

Static storage may use default initialization only when:

- the type is defaultable;
- initialization is compile-time representable;
- no hidden allocation occurs;
- no runtime constructor is required;
- destruction policy is valid.

This does not introduce hidden global constructors.

---

# Mutable declarations without initializer

A mutable declaration may omit an initializer only when its type is defaultable.

Example:

```sec
let mut count: int
```

Equivalent:

```sec
let mut count: int := 0
```

Example:

```sec
let mut user: User
```

uses the resolved default of `User`.

---

# Immutable declarations without initializer

An immutable binding must not omit its initializer merely because the type has a
default.

Invalid:

```sec
let count: int
```

Reason:

```text
an immutable binding must be initialized explicitly
```

Default initialization is available to omitted struct fields and approved
mutable-declaration contexts.

This preserves the existing Sec rule for immutable empty bindings.

---

# Reinitialization

Default assignment may be expressed explicitly using the type's default
construction when syntax permits.

Examples:

```sec
value = Type {}
```

or a future compiler-known operation such as:

```sec
value = default(Type)
```

No general default-expression syntax is introduced by this rulebook.

Assignment remains subject to:

```text
mutability
contracts
ownership
borrowing
replacement
reinitialization
destruction
```

---

# Default is not zeroed bytes

The compiler must construct the semantic default.

It must not assume that all-bits-zero is valid for every type.

All-bits-zero may differ from the semantic default for:

```text
range-constrained numbers
in-list constrained types
explicit-default types
enums
unions
references
resource wrappers
nontrivial aggregates
target-specific representations
```

Example:

```sec
type Port int range 1..65535
```

Default is:

```sec
Port(1)
```

not zeroed bytes interpreted as `Port`.

---

# Default is not uninitialized storage

Backend values such as:

```text
LLVM undef
LLVM poison
uninitialized stack slot
uninitialized memref
```

must never implement a source-level default.

Omitted fields must receive explicit semantic default construction before use.

---

# Default and ownership

A default owning value establishes normal ownership.

If the default owns no resource or allocation, destruction may be trivial.

If a type declares an owning explicit default, the compiler must verify that
constructing it does not require hidden allocation or resource acquisition.

A default must not create shared unique ownership accidentally.

---

# Default and copy classification

Defaultability is distinct from copyability.

A type may be:

```text
defaultable and copyable
defaultable and move-only
non-defaultable and copyable
non-defaultable and move-only
```

Examples are type-specific.

The compiler must not infer one property from the other.

---

# Default and destruction

A default value follows ordinary destruction rules.

A default containing only trivial fields may have trivial destruction.

A default with nontrivial owned fields must be validly constructed and destroyed
exactly once.

Omitted-field default construction participates in aggregate cleanup if later
field construction fails.

---

# Default and allocation

Implicit default initialization must not allocate dynamically.

If constructing a type's meaningful empty value requires allocation, the type is
not implicitly defaultable unless the language explicitly approves a
non-allocating representation.

An explicit constructor may allocate and return `Result`.

---

# Compile-time resolution

The compiler resolves defaults during type checking.

Conceptual query:

```text
DefaultValueOf(Type)
```

The result is one of:

```text
Resolved constant/default construction
No default
Invalid type declaration
Ambiguous implicit default
```

The resolution must be deterministic.

---

# Default dependency graph

Aggregate defaults may depend on other type defaults.

The compiler must detect cycles.

Example conceptually:

```text
A default requires B
B default requires A
```

A cycle is valid only when no runtime recursive value construction is required,
such as through a non-owning indirection explicitly defined by another rule.

Ordinary by-value recursive default construction is invalid.

---

# Diagnostics

The compiler must provide stable diagnostics for:

```text
types.invalid-explicit-default
types.default-violates-contract
types.default-not-representable
types.ambiguous-implicit-default
types.no-default-value
types.empty-in-contract
types.duplicate-in-contract-value
types.in-list-value-violates-contract
types.default-cycle

struct.missing-nondefaultable-field
struct.invalid-defaulted-field

variables.immutable-requires-initializer
variables.nondefaultable-requires-initializer

backend.default-left-undefined
```

---

# Diagnostic examples

## Invalid explicit default

```text
error[S....]: default value 0 is invalid for Port
note: Port requires range 1..65535
```

## Ambiguous nearest value

```text
error[S....]: NonZeroOdd has no unique implicit default
note: both -1 and 1 are equally near zero
help: declare an explicit default
```

## Missing non-defaultable field

```text
error[S....]: field `file` must be initialized
note: File has no default value
```

## Immutable declaration

```text
error[S....]: immutable binding `count` requires an initializer
```

---

# Formatter

The formatter preserves explicit default declarations.

It does not insert explicit values for omitted fields.

Example input:

```sec
Position {
    line: 10,
}
```

remains structurally partial in source.

The compiler applies defaults semantically.

The formatter may align a canonical `default` clause once its grammar is fixed.

It must not rewrite:

```sec
Position {}
```

into every expanded field unless a separate refactoring is requested.

---

# LSP

The LSP should expose default information in hover and completion.

Example hover:

```text
Type: Port
Default: 8080
Source: explicit type default
Contracts: range 1..65535
```

Example:

```text
Type: User
Default: "Admin"
Source: first value in `in [...]`
```

Example omitted field hint:

```text
column: int = 0
```

The LSP may offer:

```text
expand defaulted fields
insert explicit default
declare explicit type default
```

These are code actions, not ordinary formatting.

---

# Semantic IR

Semantic IR must distinguish:

```text
explicit source initialization
implicit default initialization
omitted-field default initialization
aggregate default construction
```

Conceptual operations:

```text
ConstructPrimitiveDefault
ConstructNamedDefault
ConstructRangeDefault
ConstructInListDefault
ConstructExplicitTypeDefault
ConstructStructDefault
InitializeOmittedField
```

Exact operation names may differ.

The semantic distinction must remain visible for diagnostics and verification.

---

# Backend lowering

The backend must lower semantic defaults to defined values.

It must not leave omitted fields as:

```text
undef
poison
uninitialized bytes
```

For a struct literal:

1. resolve every stored field;
2. use explicit source value when supplied;
3. use the field type's default when omitted;
4. construct fields in declaration order or another proven equivalent order;
5. track cleanup for successfully constructed nontrivial fields;
6. produce one fully initialized struct value.

---

# Optimization

The compiler may optimize default construction.

Examples:

```text
constant aggregate
zero initialization when semantically identical
memset zero when every field permits it
elided writes to dead fields
shared immutable empty string representation
```

Optimization must preserve the semantic default.

It may use zero filling only after proving that zero bytes represent the exact
default for every affected part.

---

# Required tests

Create or update:

```text
default_values_valid.sec
default_values_invalid.sec
default_structs_valid.sec
default_structs_invalid.sec
default_ranges_valid.sec
default_ranges_invalid.sec
default_in_contract_valid.sec
default_in_contract_invalid.sec
default_lists_valid.sec
default_lists_invalid.sec
default_arrays_valid.sec
default_arrays_invalid.sec
```

---

# Primitive tests

Test:

```text
every numeric type -> 0
bool -> false
string -> ""
char -> 0t
rune -> 0r
```

---

# Range tests

Test:

```text
zero valid
positive-only range
negative-only range
additional even contract
additional odd contract
floating range
decimal range
ambiguous nearest values
explicit default override
invalid explicit default
```

---

# `in [...]` tests

Test:

```text
string list
numeric list
boolean list
first value default
explicit override
empty list rejection
duplicate list rejection
invalid explicit default
additional contract interaction
```

---

# Struct tests

Test:

```text
empty defaultable struct literal
one explicit field
multiple omitted fields
nested struct
named constrained field
explicit type default field
non-defaultable field
spread plus defaults
construction failure cleanup
```

---

# Collection and array tests

Test:

```text
default numeric fixed array
default struct fixed array
non-defaultable fixed-array element
zero-length fixed array
empty owning dynamic array without allocation
empty dynamic list literal
empty bounded list literal
empty list with non-defaultable element type
safe slice rejected without a valid origin
list and struct literal categories remain distinct
```

---

# Backend tests

Verify:

```text
no omitted field remains undef
no omitted field remains poison
every defaulted fixed-array element is initialized
range default is not blindly zeroed
in-list default uses first value
explicit default overrides implicit default
nested defaults lower completely
empty dynamic-array and list descriptors are fully initialized
no safe slice is fabricated without a valid origin
```

---

# Required synchronization

This rulebook must remain synchronized with:

```text
types.md
contracts.md
struct.md
collections.md
shaped-types.md
enums.md
unions.md
memory_model.md
ownership.md
copy_move.md
destruction.txt
allocation.md
formatter.md
operators.md
grammar.md
parser_recovery.md
semantic_ir.txt
compiler_pipeline.txt
lsp.md
diagnostics.txt
language-rulebook-status.md
rules_implementations.txt
```

---

# Appendix A — Compiler conformance requirements

## A.1 Defaultability interface

Add compiler-owned type queries such as:

```go
func DefaultValueOf(t Type) DefaultResolution
func IsDefaultable(t Type) bool
```

Conceptual result:

```go
type DefaultResolution struct {
    Kind   DefaultKind
    Value  ConstantValue
    Fields []DefaultField
}
```

Exact implementation types may differ.

---

## A.2 Primitive defaults

Implement canonical primitive defaults:

```text
numeric -> zero
bool -> false
string -> empty
char -> zero char
rune -> zero rune
```

Add constant and target-representation tests.

---

## A.3 Named types

Resolve named-type defaults after:

```text
base type
contracts
units
explicit default
```

Preserve nominal result type.

Do not return only the underlying primitive type.

---

## A.4 Explicit default clause

Synchronize exact grammar.

Parse and retain:

```text
default expression
source range
token
constant value
```

Require compile-time evaluation.

Validate against the complete type.

---

## A.5 Range defaults

For a constrained numeric type:

1. test zero;
2. if zero is valid, use zero;
3. otherwise find the unique valid representable value nearest zero;
4. if no value exists, reject the type;
5. if the nearest result is ambiguous, require explicit default.

Use exact arithmetic appropriate to the type.

Do not convert decimal search through binary float.

---

## A.6 `in [...]` defaults

Validate every listed value against the base type and every other contract.
Reject the declaration when any listed value is invalid; do not filter the
list. Preserve source order and use the first listed value as the implicit
default.

Reject empty and duplicate lists.

An explicit default overrides this result.

---

## A.7 Struct omitted fields

Update struct-literal Sema:

1. resolve explicit fields and spreads;
2. reject unknown or duplicate fields;
3. inspect every stored field not supplied;
4. request `DefaultValueOf(field.Type)`;
5. reject omission when no default exists;
6. add semantic default initialization for omitted fields.

---

## A.8 Backend correction

Replace current aggregate construction from undefined base values unless every
field is filled before any read.

For omitted fields, emit resolved defaults explicitly.

Verify no source-level default depends on backend `undef`.

---

## A.9 Mutable declarations

Permit omitted initializer only for mutable bindings with defaultable type.

Create explicit semantic initialization.

Reject non-defaultable types.

Keep immutable omitted initialization invalid.

---

## A.10 Partial construction cleanup

If a later field construction fails:

- destroy previously constructed nontrivial explicit fields;
- destroy previously constructed nontrivial defaulted fields;
- do not destroy fields not yet constructed;
- do not run complete-aggregate destruction before complete initialization.

---

## A.11 LSP

Expose:

```text
default source
default value
defaultability
omitted field values
invalid or ambiguous default
```

Add code actions for explicit expansion.

---

## A.12 Diagnostics

Register stable diagnostics.

Use related locations for:

```text
type declaration
contract declaration
explicit default
omitted field
```

---

## A.13 Documentation consistency

Remove legacy statements that require every field, leave omissions without a
semantic value, force every range default to zero, or assume underlying zero is
always valid.

Do not leave conflicting canonical rules.

---

# Design summary

Sec has semantic default values.

Primitive defaults are:

```text
numeric -> 0
bool -> false
string -> ""
char -> 0t
rune -> 0r
```

A constrained numeric type defaults to zero when zero is valid.

Otherwise it defaults to the unique valid representable value nearest zero.

A type constrained by `in [...]` defaults to the first listed value; every
listed value must already satisfy all other contracts.

A type may declare an explicit default.

The explicit default overrides every implicit rule and must satisfy every type
contract.

A struct is defaultable when all stored fields are defaultable.

Omitted struct fields receive their field type's default.

Default initialization never means uninitialized storage or backend `undef`.

Mutable bindings may omit initialization only when their type is defaultable.

Immutable bindings still require explicit initialization.

Defaultability is distinct from copyability, ownership, and allocation.

The compiler resolves defaults during type checking.

Semantic IR records default construction.

The backend must materialize fully defined values.

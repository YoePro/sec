# Default Values

- **Status:** Normative
- **Created:** 2026-08-13
- **Last updated:** 2026-11-08 (stable chapter, subsection, and paragraph numbering); 2026-09-28
- **Document revision:** 1.2
- **Sec language version:** 0.1
- **Canonical path:** `rules/types/default_values.md`
- **Repository baseline reviewed:** `50655d3bc5d2e2596935dc5609cb1db1387e0cec`
- **Implementation governance:** `governance/types.yaml`

---

## § 1 Status

**§ 1(1)** This document is the canonical default-value and default-initialization rulebook
for Sec.

**§ 1(2)** The canonical filename is:

```text
default_values.md
```

**§ 1(3)** This rulebook records the language decisions for:

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

**§ 1(4)** This rulebook defines semantics.

**§ 1(5)** The explicit default clause has the canonical spelling:

```sec
type Port int range 1..65535 default 8080
```

**§ 1(6)** Its grammar is `"default" ConstantExpression`, after every type contract in a
named-type declaration.

**§ 1(7)** Implementation progress is tracked by `frontend.default-values` and
`frontend.default-values-inherited-membership` in `governance/types.yaml`.

---

## § 2 Purpose

**§ 2(1)** A default value is the value the language uses when a value is intentionally
default-initialized.

**§ 2(2)** Default initialization is not:

```text
uninitialized storage
undefined backend data
zeroing arbitrary bytes
a failed constructor
an implicit allocation
an implicit resource acquisition
```

**§ 2(3)** A default value is a valid semantic value of its type.

**§ 2(4)** The compiler must be able to construct it deterministically.

---

## § 3 Core rule

**§ 3(1)** Every type is either:

```text
Defaultable
NonDefaultable
```

**§ 3(2)** A defaultable type has exactly one compiler-resolved default value.

**§ 3(3)** A non-defaultable type cannot be omitted from a context that requires default
initialization.

**§ 3(4)** The compiler must never treat undefined storage as a default value.

---

## § 4 Default precedence

**§ 4(1)** The compiler resolves the default value of a type in this order:

```text
1. Explicit default declared by the type
2. Type-specific canonical default rule
3. Constraint-derived default
4. Underlying-type default when inheritance is permitted
5. No default
```

**§ 4(2)** An explicit type default overrides every implicit default rule.

**§ 4(3)** An explicit default must satisfy every rule and contract of the type.

---

## § 5 Primitive defaults

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

### § 5.1 Numeric types

**§ 5.1(1)** All ordinary numeric primitive types default to zero.

**§ 5.1(2)** This includes:

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

**§ 5.1(3)** where supported by the compiler and target.

**§ 5.1(4)** Examples:

```sec
let mut count: int
let mut ratio: float64
let mut amount: decimal
```

**§ 5.1(5)** Their initial values are semantically:

```sec
0
0.0
0.0
```

**§ 5.1(6)** according to the destination type.

**§ 5.1(7)** The exact physical bit representation remains target- and type-defined.

**§ 5.1(8)** The temporal types `date`, `time`, `datetime`, and `duration` are
`NonDefaultable`. Their backing representations do not create implicit
defaults, and an implementation must not infer a zero, epoch, empty, or
backend-derived value for them.

**§ 5.1(9)** `datetime.Now`, `date.Today`, and `time.Now` are operations, not defaults.
Default initialization must never read a clock.

---

### § 5.2 Boolean

**§ 5.2(1)** `bool` defaults to:

```sec
false
```

**§ 5.2(2)** Example:

```sec
let mut enabled: bool
```

**§ 5.2(3)** is equivalent to:

```sec
let mut enabled: bool := false
```

---

### § 5.3 String

**§ 5.3(1)** `string` defaults to the empty string:

```sec
""
```

**§ 5.3(2)** Example:

```sec
let mut name: string
```

**§ 5.3(3)** is equivalent to:

```sec
let mut name: string := ""
```

**§ 5.3(4)** The empty string default must not require dynamic allocation.

---

### § 5.4 Character

**§ 5.4(1)** `char` defaults to the zero character value.

**§ 5.4(2)** The canonical explicit zero literal is:

```sec
0t
```

**§ 5.4(3)** Example:

```sec
let mut character: char
```

**§ 5.4(4)** is equivalent to:

```sec
let mut character: char := 0t
```

---

### § 5.5 Rune

**§ 5.5(1)** `rune` defaults to Unicode scalar value zero.

**§ 5.5(2)** The canonical explicit zero literal is:

```sec
0r
```

**§ 5.5(3)** Example:

```sec
let mut value: rune
```

**§ 5.5(4)** is equivalent to:

```sec
let mut value: rune := 0r
```

---

## § 6 Named primitive types

**§ 6(1)** A named type based on a defaultable primitive type inherits the primitive
default when:

- the named type declares no explicit default;
- the inherited value satisfies every contract;
- no type-specific rule disables default inheritance.

**§ 6(2)** Example:

```sec
type UserName string
```

**§ 6(3)** Default:

```sec
UserName("")
```

**§ 6(4)** Example:

```sec
type Counter uint64
```

**§ 6(5)** Default:

```sec
Counter(0)
```

**§ 6(6)** Nominal identity is preserved.

**§ 6(7)** The result is a value of the named type, not the underlying primitive type.

---

## § 7 Explicit type defaults

**§ 7(1)** A named type may declare an explicit default value using the canonical syntax:

```sec
type Port int range 1..65535 default 8080
```

**§ 7(2)** The clause appears after every type contract:

```text
NamedTypeDeclaration
    := "type" Identifier TypeDefinition { Contract } [ DefaultClause ]

DefaultClause
    := "default" Expression
```

**§ 7(3)** The default expression is ordinary `Expression` syntax evaluated in a
`SemanticCompileTimeRequiredContext` (`rules/compiler/compile_time_evaluation.md`
§ 8(5)). It may read immutable values already established by semantic CTE and
call ordinary functions or read property getters whose concrete execution is
CTE-legal:

```sec
fn DefaultPort() int {
    return 8080
}

type Port int range 1..65535 default DefaultPort()
```

**§ 7(4)** The declared default:

- must be established by semantic CTE;
- must be representable by the type;
- must satisfy every contract, including argumentless contracts such as `odd`,
  `even`, `finite`, `notEmpty`, and `unique`;
- must satisfy every unit rule;
- must be statically materializable without hidden runtime allocation; its
  evaluation may use transient evaluator-local allocation where semantic CTE
  permits it, because CTE evaluability and static materializability are
  separate properties;
- must not perform I/O;
- must not depend on runtime state;
- must not call a fallible runtime constructor;
- becomes the type's canonical default value.

**§ 7(5)** (MD-011; `rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` §§ 3.6–3.10, 3.35–3.38.)

**§ 7(6)** Example:

```sec
type Port int range 1..65535 default 8080
```

**§ 7(7)** Default:

```sec
Port(8080)
```

**§ 7(8)** Example:

```sec
type User string in ["Admin", "User", "Other"] default "User"
```

**§ 7(9)** Default:

```sec
User("User")
```

---

## § 8 Explicit default overrides implicit rules

**§ 8(1)** Example:

```sec
type Temperature int range -50..100 default 20
```

**§ 8(2)** Although zero satisfies the range, the default is:

```sec
Temperature(20)
```

**§ 8(3)** Example:

```sec
type User string in ["Admin", "User", "Other"] default "Other"
```

**§ 8(4)** Although `"Admin"` is the first listed value, the default is:

```sec
User("Other")
```

---

## § 9 Invalid explicit defaults

**§ 9(1)** Invalid:

```sec
type Port int range 1..65535 default 0
```

**§ 9(2)** Reason:

```text
0 does not satisfy range 1..65535
```

**§ 9(3)** Invalid:

```sec
type User string in ["Admin", "User", "Other"] default "Guest"
```

**§ 9(4)** Reason:

```text
"Guest" is not a valid member of the type
```

**§ 9(5)** Invalid:

```sec
type PositiveEven int range 1..100 even default 3
```

**§ 9(6)** Reason:

```text
3 violates the even contract
```

**§ 9(7)** The declaration itself is a compile-time error.

**§ 9(8)** The compiler must not silently replace an invalid explicit default with an
implicit value.

---

## § 10 Range-constrained numeric types

**§ 10(1)** A range-constrained numeric type without an explicit default uses the valid value
nearest to zero.

**§ 10(2)** The complete type contracts participate in determining validity.

---

### § 10.1 Zero inside the valid range

**§ 10.1(1)** Example:

```sec
type Percent int range 0..100
```

**§ 10.1(2)** Default:

```sec
Percent(0)
```

**§ 10.1(3)** Example:

```sec
type Offset int range -100..100
```

**§ 10.1(4)** Default:

```sec
Offset(0)
```

---

### § 10.2 Positive-only range

**§ 10.2(1)** Example:

```sec
type Port int range 1..65535
```

**§ 10.2(2)** Default:

```sec
Port(1)
```

**§ 10.2(3)** `1` is the valid value nearest to zero.

---

### § 10.3 Negative-only range

**§ 10.3(1)** Example:

```sec
type Temperature int range -100..-5
```

**§ 10.3(2)** Default:

```sec
Temperature(-5)
```

**§ 10.3(3)** `-5` is the valid value nearest to zero.

---

### § 10.4 Exclusive range bounds

**§ 10.4(1)** Exclusive bounds participate in default calculation.

**§ 10.4(2)** Example:

```sec
type Positive int range 0..<100
```

**§ 10.4(3)** When zero is valid under the exact range syntax, default is zero.

**§ 10.4(4)** Example conceptually excluding zero:

```sec
type Positive int range 0<..100
```

**§ 10.4(5)** If such lower-exclusive syntax is supported, default is the smallest valid
positive value.

**§ 10.4(6)** Exact range syntax remains governed by the range rulebook.

---

## § 11 Additional numeric contracts

**§ 11(1)** The default must satisfy all contracts, not only `range`.

**§ 11(2)** Example:

```sec
type PositiveEven int range 1..100 even
```

**§ 11(3)** Valid values nearest to zero begin with:

```text
2
4
6
...
```

**§ 11(4)** Default:

```sec
PositiveEven(2)
```

**§ 11(5)** Example:

```sec
type NegativeEven int range -100..-1 even
```

**§ 11(6)** Default:

```sec
NegativeEven(-2)
```

---

## § 12 Ambiguous nearest-to-zero values

**§ 12(1)** When two different valid values are equally near zero and no other canonical
rule selects one, the type has no implicit default.

**§ 12(2)** The type must declare an explicit default.

**§ 12(3)** Example conceptually:

```sec
type NonZeroOdd int range -9..9 odd
```

**§ 12(4)** The nearest valid values may be:

```text
-1
1
```

**§ 12(5)** Neither is closer to zero.

**§ 12(6)** The compiler must require an explicit declaration such as:

```sec
type NonZeroOdd int range -9..9 odd default 1
```

**§ 12(7)** This avoids an arbitrary positive or negative bias.

---

## § 13 Floating and decimal ranges

**§ 13(1)** The same nearest-to-zero rule applies to constrained floating and decimal types.

**§ 13(2)** Example:

```sec
type PositiveAmount decimal range 0.01..1000000.00
```

**§ 13(3)** Default:

```sec
PositiveAmount(0.01)
```

**§ 13(4)** The selected value must be exactly representable by the declared type and scale.

**§ 13(5)** For plain `float`, representability uses the platform-selected width
(`rules/types/types.md`, "Binary floating-point types"): float32 on a 32-bit
platform and float64 on a 64-bit platform. The nearest-valid default is
therefore deterministic once the target platform is resolved, and a
range-constrained plain `float` receives an implicit default like `float32` and
`float64` do (MD-014; `rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` §§ 6.14–6.17).

**§ 13(6)** If there is no unique nearest representable valid value, an explicit default is
required.

---

## § 14 `in [...]` constrained types

**§ 14(1)** A type constrained by an ordered `in [...]` list uses the first listed value as
its implicit default.

**§ 14(2)** Example:

```sec
type User string in ["Admin", "User", "Other"]
```

**§ 14(3)** Default:

```sec
User("Admin")
```

**§ 14(4)** The list order is semantically significant for default selection.

**§ 14(5)** The compiler and formatter must preserve the declared order.

---

### § 14.1 Numeric `in [...]`

**§ 14.1(1)** Example:

```sec
type RetryCount int in [1, 3, 5]
```

**§ 14.1(2)** Default:

```sec
RetryCount(1)
```

**§ 14.1(3)** The nearest-to-zero rule does not replace the first-item rule for an explicit
`in [...]` list.

---

### § 14.2 Boolean `in [...]`

**§ 14.2(1)** Example:

```sec
type RequiredBool bool in [true]
```

**§ 14.2(2)** Default:

```sec
RequiredBool(true)
```

**§ 14.2(3)** The underlying primitive default `false` is invalid and is not used.

**§ 14.2(4)** The first listed value is used.

---

### § 14.3 Explicit default for `in [...]`

**§ 14.3(1)** Example:

```sec
type User string in ["Admin", "User", "Other"] default "User"
```

**§ 14.3(2)** Default:

```sec
User("User")
```

**§ 14.3(3)** The explicit declaration overrides the first-item rule.

---

### § 14.4 Empty `in [...]` list

**§ 14.4(1)** A type with an empty `in [...]` constraint is invalid.

**§ 14.4(2)** Invalid:

```sec
type Impossible int in []
```

**§ 14.4(3)** Such a type has no valid values and cannot have a valid default.

---

### § 14.5 Duplicate values

**§ 14.5(1)** Duplicate entries in `in [...]` are invalid and must be rejected.

---

## § 15 Multiple constraints including `in [...]`

**§ 15(1)** Every value in `in [...]` must satisfy every other contract on the type. The
first listed value is the implicit default.

**§ 15(2)** Example:

```sec
type SmallEven int in [2, 4, 6, 8] even
```

**§ 15(3)** Default:

```sec
SmallEven(2)
```

**§ 15(4)** This is invalid because some listed values violate `even`:

```sec
type InvalidEven int in [1, 2, 3, 4] even
```

**§ 15(5)** Invalid entries are declaration errors; they are never silently filtered.

---

## § 16 Defaults and contracts

**§ 16(1)** A default is part of the type's semantic definition.

**§ 16(2)** The compiler must prove at declaration time that the default satisfies:

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

**§ 16(3)** A default is not revalidated at every ordinary use after the declaration has
been proven.

---

## § 17 Defaults and units

**§ 17(1)** A unit-bearing numeric type normally defaults to numeric zero when zero is valid.

**§ 17(2)** Example:

```sec
type Distance decimal<m>
```

**§ 17(3)** Default:

```sec
Distance(0)
```

**§ 17(4)** A constrained unit-bearing type follows its constraints.

**§ 17(5)** Example:

```sec
type PositiveDistance decimal<m> range 0.1..1000.0
```

**§ 17(6)** Default:

```sec
PositiveDistance(0.1)
```

**§ 17(7)** An explicit unit-bearing default must have a compatible unit and exact
conversion.

---

## § 18 Struct defaults

**§ 18(1)** A struct type has a default value when every stored field has a default value.

**§ 18(2)** The struct default is constructed field by field in declaration order.

**§ 18(3)** Example:

```sec
type Position struct {
    line: int,
    column: int,
    valid: bool,
    name: string,
}
```

**§ 18(4)** Default:

```sec
Position {
    line: 0,
    column: 0,
    valid: false,
    name: "",
}
```

---

## § 19 Empty struct literal

**§ 19(1)** A defaultable struct may be written:

```sec
Position {}
```

**§ 19(2)** This constructs the struct default.

**§ 19(3)** It does not create uninitialized fields.

---

## § 20 Omitted struct fields

**§ 20(1)** Stored fields omitted from a struct literal are initialized with their declared
type's default value.

**§ 20(2)** Example:

```sec
Position {
    line: 10,
}
```

**§ 20(3)** is semantically equivalent to:

```sec
Position {
    line: 10,
    column: 0,
    valid: false,
    name: "",
}
```

**§ 20(4)** The compiler may lower the two forms identically.

---

## § 21 Nested struct defaults

**§ 21(1)** Default initialization is recursive.

**§ 21(2)** Example:

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

**§ 21(3)** Default:

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

**§ 21(4)** Therefore:

```sec
Window {}
```

**§ 21(5)** is valid.

---

## § 22 Explicit field values override defaults

**§ 22(1)** Example:

```sec
Window {
    visible: true,
}
```

**§ 22(2)** uses:

```text
origin
    Point default

visible
    true

title
    string default
```

**§ 22(3)** No field may be initialized twice.

---

## § 23 Non-defaultable struct fields

**§ 23(1)** A struct literal may omit a field only when that field's type is defaultable.

**§ 23(2)** Example conceptually:

```sec
type ResourceHolder struct {
    file: File,
    active: bool,
}
```

**§ 23(3)** If `File` has no default, this is invalid:

```sec
ResourceHolder {
    active: true,
}
```

**§ 23(4)** Diagnostic:

```text
field `file` has no default value and must be initialized
```

**§ 23(5)** The complete struct type may itself be non-defaultable.

---

## § 24 Field-level defaults

**§ 24(1)** Field-level default syntax is not defined by this rulebook unless already
defined elsewhere.

**§ 24(2)** A future rule may permit:

```sec
type Config struct {
    timeout: Duration = Duration(30),
}
```

**§ 24(3)** or another canonical spelling.

**§ 24(4)** Until that syntax is explicitly approved:

- defaults belong to field types;
- struct literals may provide explicit field values;
- omitted fields use their type defaults.

---

## § 25 Struct spread and defaults

**§ 25(1)** A struct spread provides fields from the spread source according to `spread`
rules.

**§ 25(2)** Conceptual resolution order:

```text
1. explicit source fields and spreads
2. duplicate and conflict validation
3. omitted-field default initialization
```

**§ 25(3)** Defaults do not overwrite fields supplied by spread.

---

## § 26 Parser example

**§ 26(1)** Given:

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

**§ 26(2)** This literal:

```sec
Parser {
    l: l,
    errors: list[string] {},
    warnings: list[string] {},
}
```

**§ 26(3)** is valid only when:

- `lexer.Lexer` can be moved or copied into `l`;
- `list[string] {}` is a valid explicit empty-list construction;
- `lexer.Token` is defaultable;
- `bool` is defaultable.

**§ 26(4)** The omitted fields become:

```sec
curToken: lexer.Token {}
peekToken: lexer.Token {}
stopBeforeBrace: false
inRefExpression: false
```

---

## § 27 Token example

**§ 27(1)** Given:

```sec
type Token struct {
    Type: TokenType,
    Lexeme: string,
    File: string,
    Line: int,
    Column: int,
}
```

**§ 27(2)** and:

```sec
type TokenType string
```

**§ 27(3)** then:

```sec
Token {}
```

**§ 27(4)** uses:

```sec
Token {
    Type: TokenType(""),
    Lexeme: "",
    File: "",
    Line: 0,
    Column: 0,
}
```

**§ 27(5)** unless `TokenType` declares another explicit or constrained default.

---

## § 28 Arrays

**§ 28(1)** The canonical fixed-array rule is:

```text
an array is defaultable when its element type is defaultable;
every element receives the element default
```

**§ 28(2)** Example:

```sec
let mut values: int[4]
```

**§ 28(3)** is semantically equivalent to:

```sec
[0, 0, 0, 0]
```

**§ 28(4)** A zero-length fixed array is defaultable without constructing an element.

---

## § 29 Owning dynamic arrays

**§ 29(1)** `T[]` is defaultable independently of whether `T` is defaultable. Its default
is an initialized, non-allocating empty owner with length and capacity zero and
no initialized elements. Default construction does not construct a `T`.

**§ 29(2)** This rule applies only to the owning dynamic array. It does not create a null
or origin-free default for a safe slice.

---

## § 30 Slices

**§ 30(1)** A slice is a non-owning view.

**§ 30(2)** A universal implicit slice default must not be invented without defining:

```text
origin
bounds
lifetime
mutability
empty-view representation
```

**§ 30(3)** An explicit empty slice literal may be defined separately.

**§ 30(4)** A safe slice has no implicit default and is non-defaultable.

---

## § 31 List defaults

**§ 31(1)** `list[T]` is defaultable independently of whether `T` is defaultable. Its
default is an initialized empty list with length and capacity zero, no element
storage, no initialized elements and no allocation.

**§ 31(2)** `list[T, Capacity]` is likewise defaultable. Its default has length zero,
maximum capacity `Capacity`, no initialized elements and no hidden growth
allocation.

**§ 31(3)** The canonical explicit forms are:

```sec
list[string] {}
list[Packet, 32] {}
```

**§ 31(4)** These are collection literals, not struct literals. Empty construction does not
allocate or default-construct elements. Later dynamic growth remains fallible
and requires an approved allocation context. Map and set literal syntax remains
governed by their own collection rules.

---

## § 32 Enums

**§ 32(1)** Every valid enum is non-empty and defaultable. Its default is the single member
marked `default`, when present, and otherwise its first declared member. Member
identity selects the default; underlying numeric zero or an empty string has no
special priority.

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

**§ 32(2)** The defaults are `Status.UNKNOWN` and `ConnectionState.DISCONNECTED`.
The marker does not alter the initializer, `iota`, aliases, or initializer
repetition. For an open bit-backed enum, the default is still a declared member;
an unnamed raw pattern is never selected merely because it is zero.

**§ 32(3)** The same member-selection rule applies to string-backed enums:

```sec
enum Program string {
    OneCare = "Zebra OneCare",
    VIQ default = "Z1C+VIQ",
}
```

**§ 32(4)** The default is `Program.VIQ`. The empty string has no special default priority,
and every string-backed member must still provide an explicit initializer.

**§ 32(5)** A mutable enum declaration without an initializer is initialized with the
resolved enum default. This is initialized storage and does not require a later
definite assignment before first read. An enum used in an omitted struct field
uses the same resolved default.

---

## § 33 Unions

**§ 33(1)** A union has no implicit first-variant default.

**§ 33(2)** A union is `Defaultable` only when exactly one declared variant is explicitly
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

**§ 33(3)** The resolved default is `State.Idle`.

**§ 33(4)** A mutable binding of a NonDefaultable union has one explicit exception to the
general omitted-initializer rule:

```sec
type PendingState union {
    Idle
    Running
}

let mut pending: PendingState
```

**§ 33(5)** The binding begins in the compiler-known `empty` initialization state. `empty`
is not a default value, a valid union value, `null`, `None`, `Nothing`, or a
hidden union variant. The binding must contain a real union value before it may
escape or be used in a context requiring a value. `is empty` and the `empty`
match pattern inspect this initialization state under the union and match
rules.

**§ 33(6)** Immutable union bindings still require an explicit initializer.

**§ 33(7)** A struct-like union payload construction may omit a field whose type is
Defaultable; that field receives its normal resolved default. A NonDefaultable
payload field must be supplied explicitly.

---

## § 34 Option and Result

**§ 34(1)** `Option[T]` and `Result[T, E]` do not receive an implicit default merely from
their representation.

**§ 34(2)** Possible defaults such as:

```text
None
Ok(default(T))
Err(default(E))
```

**§ 34(3)** must be explicitly defined by their canonical core rules.

**§ 34(4)** Representation zero is not enough.

---

## § 35 References

**§ 35(1)** Safe references have no universal default.

**§ 35(2)** The language must not fabricate:

```text
null ref T
dangling ref T
reference to temporary default storage
```

**§ 35(3)** Therefore:

```text
ref T
ref mut T
```

**§ 35(4)** are non-defaultable unless a specific wrapper type defines optional reference
semantics.

---

## § 36 Raw pointers

**§ 36(1)** `RawPtr[T]` default semantics must be defined by `raw_pointers.md`.

**§ 36(2)** If a null raw pointer value is supported, it may be a valid explicit raw-pointer
default.

**§ 36(3)** This must not imply a valid reference or owned allocation.

**§ 36(4)** No reference default may be derived from raw-pointer zero.

---

## § 37 Resources

**§ 37(1)** Unique resource types normally have no implicit default.

**§ 37(2)** Examples:

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

**§ 37(3)** A closed, detached, invalid, or null representation is not automatically a valid
semantic default.

**§ 37(4)** A resource type may define an explicit valid default only when its contract
makes that state meaningful.

---

## § 38 Function values and closures

**§ 38(1)** Function-value defaults require a canonical callable or empty-callable model.

**§ 38(2)** The language must not fabricate an invalid call target.

**§ 38(3)** Until explicitly defined, function and capturing closure types are
non-defaultable.

---

## § 39 Interfaces

**§ 39(1)** General interface values require a defined empty or default dynamic state.

**§ 39(2)** Until owned and borrowed erased representations define such a state, general
interface types are non-defaultable.

**§ 39(3)** A specific interface wrapper may define an explicit default.

---

## § 40 Static values

**§ 40(1)** Static storage may use default initialization only when:

- the type is defaultable;
- initialization is compile-time representable;
- no hidden allocation occurs;
- no runtime constructor is required;
- destruction policy is valid.

**§ 40(2)** This does not introduce hidden global constructors.

---

## § 41 Mutable declarations without initializer

**§ 41(1)** A mutable declaration may omit an initializer only when its type is defaultable.

**§ 41(2)** Example:

```sec
let mut count: int
```

**§ 41(3)** Equivalent:

```sec
let mut count: int := 0
```

**§ 41(4)** Example:

```sec
let mut user: User
```

**§ 41(5)** uses the resolved default of `User`.

---

## § 42 Immutable declarations without initializer

**§ 42(1)** An immutable binding must not omit its initializer merely because the type has a
default.

**§ 42(2)** Invalid:

```sec
let count: int
```

**§ 42(3)** Reason:

```text
an immutable binding must be initialized explicitly
```

**§ 42(4)** Default initialization is available to omitted struct fields and approved
mutable-declaration contexts.

**§ 42(5)** This preserves the existing Sec rule for immutable empty bindings.

---

## § 43 Reinitialization

**§ 43(1)** Default assignment may be expressed explicitly using the type's default
construction when syntax permits.

**§ 43(2)** Examples:

```sec
value = Type {}
```

**§ 43(3)** or a future compiler-known operation such as:

```sec
value = default(Type)
```

**§ 43(4)** No general default-expression syntax is introduced by this rulebook.

**§ 43(5)** Assignment remains subject to:

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

## § 44 Default is not zeroed bytes

**§ 44(1)** The compiler must construct the semantic default.

**§ 44(2)** It must not assume that all-bits-zero is valid for every type.

**§ 44(3)** All-bits-zero may differ from the semantic default for:

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

**§ 44(4)** Example:

```sec
type Port int range 1..65535
```

**§ 44(5)** Default is:

```sec
Port(1)
```

**§ 44(6)** not zeroed bytes interpreted as `Port`.

---

## § 45 Default is not uninitialized storage

**§ 45(1)** Backend values such as:

```text
LLVM undef
LLVM poison
uninitialized stack slot
uninitialized memref
```

**§ 45(2)** must never implement a source-level default.

**§ 45(3)** Omitted fields must receive explicit semantic default construction before use.

---

## § 46 Default and ownership

**§ 46(1)** A default owning value establishes normal ownership.

**§ 46(2)** If the default owns no resource or allocation, destruction may be trivial.

**§ 46(3)** If a type declares an owning explicit default, the compiler must verify that
constructing it does not require hidden allocation or resource acquisition.

**§ 46(4)** A default must not create shared unique ownership accidentally.

---

## § 47 Default and copy classification

**§ 47(1)** Defaultability is distinct from copyability.

**§ 47(2)** A type may be:

```text
defaultable and copyable
defaultable and move-only
non-defaultable and copyable
non-defaultable and move-only
```

**§ 47(3)** Examples are type-specific.

**§ 47(4)** The compiler must not infer one property from the other.

---

## § 48 Default and destruction

**§ 48(1)** A default value follows ordinary destruction rules.

**§ 48(2)** A default containing only trivial fields may have trivial destruction.

**§ 48(3)** A default with nontrivial owned fields must be validly constructed and destroyed
exactly once.

**§ 48(4)** Omitted-field default construction participates in aggregate cleanup if later
field construction fails.

---

## § 49 Default and allocation

**§ 49(1)** Implicit default initialization must not allocate dynamically.

**§ 49(2)** If constructing a type's meaningful empty value requires allocation, the type is
not implicitly defaultable unless the language explicitly approves a
non-allocating representation.

**§ 49(3)** An explicit constructor may allocate and return `Result`.

---

## § 50 Compile-time resolution

**§ 50(1)** The compiler resolves defaults during type checking.

**§ 50(2)** Conceptual query:

```text
DefaultValueOf(Type)
```

**§ 50(3)** The result is one of:

```text
Resolved constant/default construction
No default
Invalid type declaration
Ambiguous implicit default
```

**§ 50(4)** The resolution must be deterministic.

---

## § 51 Default dependency graph

**§ 51(1)** Aggregate defaults may depend on other type defaults.

**§ 51(2)** The compiler must detect cycles.

**§ 51(3)** Example conceptually:

```text
A default requires B
B default requires A
```

**§ 51(4)** A cycle is valid only when no runtime recursive value construction is required,
such as through a non-owning indirection explicitly defined by another rule.

**§ 51(5)** Ordinary by-value recursive default construction is invalid.

---

## § 52 Diagnostics

**§ 52(1)** The compiler must provide stable diagnostics for:

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

## § 53 Diagnostic examples

### § 53.1 Invalid explicit default

```text
error[S....]: default value 0 is invalid for Port
note: Port requires range 1..65535
```

### § 53.2 Ambiguous nearest value

```text
error[S....]: NonZeroOdd has no unique implicit default
note: both -1 and 1 are equally near zero
help: declare an explicit default
```

### § 53.3 Missing non-defaultable field

```text
error[S....]: field `file` must be initialized
note: File has no default value
```

### § 53.4 Immutable declaration

```text
error[S....]: immutable binding `count` requires an initializer
```

---

## § 54 Formatter

**§ 54(1)** The formatter preserves explicit default declarations.

**§ 54(2)** It does not insert explicit values for omitted fields.

**§ 54(3)** Example input:

```sec
Position {
    line: 10,
}
```

**§ 54(4)** remains structurally partial in source.

**§ 54(5)** The compiler applies defaults semantically.

**§ 54(6)** The formatter may align a canonical `default` clause once its grammar is fixed.

**§ 54(7)** It must not rewrite:

```sec
Position {}
```

**§ 54(8)** into every expanded field unless a separate refactoring is requested.

---

## § 55 LSP

**§ 55(1)** The LSP should expose default information in hover and completion.

**§ 55(2)** Example hover:

```text
Type: Port
Default: 8080
Source: explicit type default
Contracts: range 1..65535
```

**§ 55(3)** Example:

```text
Type: User
Default: "Admin"
Source: first value in `in [...]`
```

**§ 55(4)** Example omitted field hint:

```text
column: int = 0
```

**§ 55(5)** The LSP may offer:

```text
expand defaulted fields
insert explicit default
declare explicit type default
```

**§ 55(6)** These are code actions, not ordinary formatting.

---

## § 56 Semantic IR

**§ 56(1)** Semantic IR must distinguish:

```text
explicit source initialization
implicit default initialization
omitted-field default initialization
aggregate default construction
```

**§ 56(2)** Conceptual operations:

```text
ConstructPrimitiveDefault
ConstructNamedDefault
ConstructRangeDefault
ConstructInListDefault
ConstructExplicitTypeDefault
ConstructStructDefault
InitializeOmittedField
```

**§ 56(3)** Exact operation names may differ.

**§ 56(4)** The semantic distinction must remain visible for diagnostics and verification.

---

## § 57 Backend lowering

**§ 57(1)** The backend must lower semantic defaults to defined values.

**§ 57(2)** It must not leave omitted fields as:

```text
undef
poison
uninitialized bytes
```

**§ 57(3)** For a struct literal:

1. resolve every stored field;
2. use explicit source value when supplied;
3. use the field type's default when omitted;
4. construct fields in declaration order or another proven equivalent order;
5. track cleanup for successfully constructed nontrivial fields;
6. produce one fully initialized struct value.

---

## § 58 Optimization

**§ 58(1)** The compiler may optimize default construction.

**§ 58(2)** Examples:

```text
constant aggregate
zero initialization when semantically identical
memset zero when every field permits it
elided writes to dead fields
shared immutable empty string representation
```

**§ 58(3)** Optimization must preserve the semantic default.

**§ 58(4)** It may use zero filling only after proving that zero bytes represent the exact
default for every affected part.

---

## § 59 Required tests

**§ 59(1)** Create or update:

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

## § 60 Primitive tests

**§ 60(1)** Test:

```text
every numeric type -> 0
bool -> false
string -> ""
char -> 0t
rune -> 0r
```

---

## § 61 Range tests

**§ 61(1)** Test:

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

## § 62 `in [...]` tests

**§ 62(1)** Test:

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

## § 63 Struct tests

**§ 63(1)** Test:

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

## § 64 Collection and array tests

**§ 64(1)** Test:

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

## § 65 Backend tests

**§ 65(1)** Verify:

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

## § 66 Required synchronization

**§ 66(1)** This rulebook must remain synchronized with:

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
destruction.md
allocation.md
formatter.md
operators.md
grammar.md
parser_recovery.md
semantic_ir.md
compiler_pipeline.md
lsp.md
diagnostics.md
language-rulebook-status.md
```

---

## Appendix A — Compiler conformance requirements

### Appendix A.1 Defaultability interface

**Appendix A.1(1)** Add compiler-owned type queries such as:

```go
func DefaultValueOf(t Type) DefaultResolution
func IsDefaultable(t Type) bool
```

**Appendix A.1(2)** Conceptual result:

```go
type DefaultResolution struct {
    Kind   DefaultKind
    Value  ConstantValue
    Fields []DefaultField
}
```

**Appendix A.1(3)** Exact implementation types may differ.

---

### Appendix A.2 Primitive defaults

**Appendix A.2(1)** Implement canonical primitive defaults:

```text
numeric -> zero
bool -> false
string -> empty
char -> zero char
rune -> zero rune
```

**Appendix A.2(2)** Add constant and target-representation tests.

---

### Appendix A.3 Named types

**Appendix A.3(1)** Resolve named-type defaults after:

```text
base type
contracts
units
explicit default
```

**Appendix A.3(2)** Preserve nominal result type.

**Appendix A.3(3)** Do not return only the underlying primitive type.

---

### Appendix A.4 Explicit default clause

**Appendix A.4(1)** Synchronize exact grammar.

**Appendix A.4(2)** Parse and retain:

```text
default expression
source range
token
constant value
```

**Appendix A.4(3)** Require compile-time evaluation.

**Appendix A.4(4)** Validate against the complete type.

---

### Appendix A.5 Range defaults

**Appendix A.5(1)** For a constrained numeric type:

1. test zero;
2. if zero is valid, use zero;
3. otherwise find the unique valid representable value nearest zero;
4. if no value exists, reject the type;
5. if the nearest result is ambiguous, require explicit default.

**Appendix A.5(2)** Use exact arithmetic appropriate to the type.

**Appendix A.5(3)** Do not convert decimal search through binary float.

---

### Appendix A.6 `in [...]` defaults

**Appendix A.6(1)** Validate every listed value against the base type and every other contract.
Reject the declaration when any listed value is invalid; do not filter the
list. Preserve source order and use the first listed value as the implicit
default.

**Appendix A.6(2)** Reject empty and duplicate lists.

**Appendix A.6(3)** An explicit default overrides this result.

---

### Appendix A.7 Struct omitted fields

**Appendix A.7(1)** Update struct-literal Sema:

1. resolve explicit fields and spreads;
2. reject unknown or duplicate fields;
3. inspect every stored field not supplied;
4. request `DefaultValueOf(field.Type)`;
5. reject omission when no default exists;
6. add semantic default initialization for omitted fields.

---

### Appendix A.8 Backend correction

**Appendix A.8(1)** Replace current aggregate construction from undefined base values unless every
field is filled before any read.

**Appendix A.8(2)** For omitted fields, emit resolved defaults explicitly.

**Appendix A.8(3)** Verify no source-level default depends on backend `undef`.

---

### Appendix A.9 Mutable declarations

**Appendix A.9(1)** Permit omitted initializer only for mutable bindings with defaultable type.

**Appendix A.9(2)** Create explicit semantic initialization.

**Appendix A.9(3)** Reject non-defaultable types.

**Appendix A.9(4)** Keep immutable omitted initialization invalid.

---

### Appendix A.10 Partial construction cleanup

**Appendix A.10(1)** If a later field construction fails:

- destroy previously constructed nontrivial explicit fields;
- destroy previously constructed nontrivial defaulted fields;
- do not destroy fields not yet constructed;
- do not run complete-aggregate destruction before complete initialization.

---

### Appendix A.11 LSP

**Appendix A.11(1)** Expose:

```text
default source
default value
defaultability
omitted field values
invalid or ambiguous default
```

**Appendix A.11(2)** Add code actions for explicit expansion.

---

### Appendix A.12 Diagnostics

**Appendix A.12(1)** Register stable diagnostics.

**Appendix A.12(2)** Use related locations for:

```text
type declaration
contract declaration
explicit default
omitted field
```

---

### Appendix A.13 Documentation consistency

**Appendix A.13(1)** Remove legacy statements that require every field, leave omissions without a
semantic value, force every range default to zero, or assume underlying zero is
always valid.

**Appendix A.13(2)** Do not leave conflicting canonical rules.

---

## § 67 Design summary

**§ 67(1)** Sec has semantic default values.

**§ 67(2)** Primitive defaults are:

```text
numeric -> 0
bool -> false
string -> ""
char -> 0t
rune -> 0r
```

**§ 67(3)** A constrained numeric type defaults to zero when zero is valid.

**§ 67(4)** Otherwise it defaults to the unique valid representable value nearest zero.

**§ 67(5)** A type constrained by `in [...]` defaults to the first listed value; every
listed value must already satisfy all other contracts.

**§ 67(6)** A type may declare an explicit default.

**§ 67(7)** The explicit default overrides every implicit rule and must satisfy every type
contract.

**§ 67(8)** A struct is defaultable when all stored fields are defaultable.

**§ 67(9)** Omitted struct fields receive their field type's default.

**§ 67(10)** Default initialization never means uninitialized storage or backend `undef`.

**§ 67(11)** Mutable bindings may omit initialization only when their type is defaultable.

**§ 67(12)** Immutable bindings still require explicit initialization.

**§ 67(13)** Defaultability is distinct from copyability, ownership, and allocation.

**§ 67(14)** The compiler resolves defaults during type checking.

**§ 67(15)** Semantic IR records default construction.

**§ 67(16)** The backend must materialize fully defined values.

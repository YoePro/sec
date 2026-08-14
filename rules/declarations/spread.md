# Spread

## Document metadata

- **Status:** Canonical
- **Language version:** Sec 0.1
- **Document revision:** 2.0
- **Created:** 2026-08-13
- **Last updated:** 2026-08-14
- **Replaces:** `rules/declarations/spread.txt`

---

## 1. Purpose

The postfix spread operator expands one source value into the surrounding
argument, element, or field list.

Canonical syntax:

```sec
expression...
```

Examples:

```sec
Call(arguments...)

let combined := [prefix..., 10, 20]

let updated := User {
    original...
    Name: "Anna"
}
```

Spread is contextual syntax. It does not produce a standalone value.

Spread does not itself mean:

- copy;
- move;
- clone;
- allocation;
- iteration;
- concatenation;
- conversion;
- borrowing.

The destination context and the ordinary ownership rules determine the
operations performed on the expanded values.

The prefix form is invalid:

```sec
...value
```

---

## 2. Approved contexts

Spread is valid only in a spread-aware surrounding construct.

Sec defines these canonical contexts:

- function and method argument lists;
- fixed-array literals;
- struct literals.

A specialized destination may define an additional spread-aware context in its
own canonical rulebook. Such a context must define expansion count, ownership,
evaluation order, and allocation semantics explicitly.

Spread is not an ordinary expression.

Invalid:

```sec
let expanded := values...
return values...
values... = other
```

---

## 3. Evaluation order

A spread source expression is evaluated exactly once.

Expanded values are then observed in their natural order:

- arrays: increasing index order;
- struct fields: declaration order.

The complete surrounding construct is evaluated left to right.

Example:

```sec
Call(
    First(),
    values...,
    Last(),
)
```

Semantic order:

```text
1. First()
2. evaluate values exactly once
3. expand values in index order
4. Last()
5. perform the call
```

Lowering must preserve this observable order.

---

## 4. Function and method argument spread

A fixed-size array may be expanded into individual call arguments.

```sec
fn Add(a: int, b: int, c: int) int {
    return a + b + c
}

let values: int[3] := [10, 20, 30]
let result := Add(values...)
```

For a fixed-arity destination, this is semantically equivalent to using each
array element as a separate argument while evaluating the source array
expression only once:

```sec
let result := Add(values[0], values[1], values[2])
```

### 4.1 Arity

For a fixed-arity call, every spread contribution must have a compile-time-known
expansion count.

A runtime-length source therefore cannot be spread into a fixed-arity call:

```sec
let values: ref int[] := ref storage[..]
Use(values...) // invalid for a fixed-arity Use
```

If a function rulebook defines a variadic or otherwise runtime-arity parameter,
a runtime-length source may be accepted only when that destination explicitly
defines how such arguments are received. Spread itself does not define variadic
parameters.

Native `...T` parameters are such an explicit runtime-arity destination.
Ordinary and spread arguments may mix, multiple spread sources are permitted,
and every expanded element must satisfy `T`. Each source is evaluated exactly
once in the call's left-to-right argument order.

### 4.2 Call resolution

Expansion occurs before final argument matching.

After expansion, normal call rules apply, including:

- total argument count;
- argument order;
- parameter type compatibility;
- ownership and borrowing requirements;
- mutability requirements;
- unsafe requirements;
- generic inference;
- overload resolution.

Return type does not participate in overload selection.

### 4.3 Ownership

Fixed-array argument spread does not provide a consuming-spread operation.

The expanded indexed reads must therefore be legal under ordinary array read
rules. In Sec 0.1 this means that a fixed-array spread source must have elements
that can be implicitly copied for the selected parameters.

Invalid:

```sec
let resources: Resource[2] := [...]
Consume(resources...)
```

Spread must not hide per-element moves, partial array state, clones, or
allocation.

The same non-consuming rule applies to native variadic destinations. A spread
from move-only collection elements is invalid when satisfying the call would
require moving individual elements out of the source.

Passing references remains explicit according to ordinary call and borrowing
syntax; spread does not invent implicit borrows.

---

## 5. Fixed-array literal spread

A fixed-size array may be expanded into another fixed-array literal.

```sec
let first: int[2] := [1, 2]
let combined := [first..., 3, 4]
```

The inferred result is:

```sec
int[4]
```

Multiple spreads are valid:

```sec
let all := [first..., second..., 10]
```

### 5.1 Result length

The resulting fixed-array length is computed at compile time as the sum of:

- ordinary literal elements;
- the compile-time lengths of every spread source.

Length arithmetic is checked. Overflow is a compile-time error.

A runtime-length source cannot be spread into a fixed-array literal:

```sec
let values: ref int[] := ref storage[..]
let combined := [values..., 10] // invalid
```

Spread must not silently convert a fixed-array literal into a dynamically sized
container or allocate hidden backing storage.

### 5.2 Element type

All ordinary and expanded elements must satisfy the target element type.

Target context may shape untyped literals:

```sec
let prefix: byte[2] := [1, 2]
let values: byte[4] := [prefix..., 3, 4]
```

Spread does not introduce structural or common-type conversion rules of its own.

### 5.3 Ownership

Fixed-array literal spread copies expanded elements according to ordinary
implicit-copy rules.

It must not:

- clone move-only elements;
- move individual elements out of the source fixed array;
- leave the source array partially initialized;
- invalidate the source array;
- allocate hidden storage merely to implement expansion.

A consuming fixed-array spread form is not part of Sec spread syntax.

---

## 6. Struct literal spread

A struct value may provide fields to a literal of the same exact nominal struct
type.

```sec
type User struct {
    Name: string
    Age: int
    Active: bool
}

let updated := User {
    original...
    Name: "Anna"
}
```

Struct spread is shallow. It expands only direct fields and never recursively
merges nested values.

### 6.1 Source type

The spread source must have exactly the target struct type.

No structural conversion is performed.

Invalid:

```sec
let user := User {
    otherType...
}
```

Nominal type identity is preserved.

### 6.2 Entry order and overriding

Struct literal entries are processed from left to right.

A later spread or explicit field entry overrides a field value supplied by an
earlier spread.

```sec
let merged := User {
    defaults...
    stored...
    Name: "Anna"
}
```

Resolution order:

```text
defaults
stored overrides defaults
Name overrides both
```

Two explicit field declarations with the same name remain invalid:

```sec
let invalid := User {
    Name: "Anna"
    Name: "Maria"
}
```

Spread does not make duplicate explicit fields valid.

### 6.3 Omitted fields and defaults

After all spreads and explicit entries are resolved:

1. each field has at most one final selected supplied value;
2. every still-omitted Defaultable field is initialized from its declared type's
   semantic default;
3. every still-omitted NonDefaultable field is a compile-time error.

Example:

```sec
type Settings struct {
    Retries: int
    Enabled: bool
    File: File
}

let settings := Settings {
    previous...
    Enabled: true
}
```

If `previous` supplies `File`, the construction is valid. If no spread or
explicit field supplies `File` and `File` is NonDefaultable, construction is
invalid. `Retries` may use the semantic default of `int` when still omitted.

This rule is synchronized with the canonical struct defaulting rules.

### 6.4 Visibility and invariants

Struct spread follows ordinary struct construction visibility rules.

Spread must not bypass:

- inaccessible fields;
- nominal type identity;
- construction invariants;
- field-type contracts;
- ownership rules.

If the target struct cannot legally be constructed at the current source
location because required representation is inaccessible, spread does not make
that construction legal.

### 6.5 Ownership

Struct spread is a non-consuming update/construction form.

The source struct must therefore be implicitly copyable under the ordinary copy
rules. Each selected source field is copied according to those rules.

A move-only source is invalid.

Spread must not:

- partially move a struct;
- hide explicit move syntax;
- invoke a hidden clone;
- allocate merely to perform spread.

Explicit field moves remain governed by the copy/move rulebook and are not a
spread operation.

---

## 7. Copy, move, and borrowing boundary

Spread describes expansion and placement, not ownership transfer.

The compiler must resolve each expanded use through the ordinary ownership
model.

Canonical restrictions are:

```text
fixed-array call spread
    source elements must support the required non-consuming reads

fixed-array literal spread
    source elements must be implicitly copyable

struct literal spread
    source struct must be implicitly copyable
```

Spread never silently changes from copy to move according to the concrete type.

A hidden consuming-spread or partial-move-spread interpretation is invalid.

---

## 8. Allocation

Spread does not allocate by itself.

The compiler must not allocate temporary dynamic storage merely to implement
spread.

Fixed-size call and array expansion may be lowered directly.

Struct spread may be lowered to explicit field operations.

If another spread-aware destination is allocating by its own semantics, that
allocation belongs to the destination operation, not to spread.

---

## 9. Invalid sources and contexts

Unless another canonical destination rule explicitly permits them, spread is
invalid for sources such as:

- scalar values;
- strings;
- maps;
- sets;
- interfaces;
- unions;
- raw pointers;
- registers;
- move-only fixed arrays for element expansion;
- move-only structs for struct spread;
- values whose expansion count is not valid for the destination context.

Slices and owning dynamic arrays are not valid sources for fixed-array literals
or fixed-arity calls because their runtime length cannot satisfy those
compile-time arity requirements. A runtime-arity destination may define a
specific sequence-spread rule separately.

Examples:

```sec
Call(10...)
let values := [slice..., 10]
let user := User { rawPointer... }
```

---

## 10. Parsing

The parser represents postfix spread contextually:

```text
SpreadExpression
    Source
    Token
```

Canonical spread-aware parser positions include:

```text
CallArgumentList
ArrayLiteralElementList
StructLiteralEntryList
```

The parser must reject or recover diagnostically from:

- prefix spread;
- standalone spread;
- repeated spread markers;
- spread without a source expression;
- spread in a context that does not accept expansion.

Examples:

```sec
...values
values......
let value := values...
```

Recovery should synchronize at the surrounding comma or closing delimiter when
practical.

---

## 11. Semantic analysis

Sema must:

- identify the destination spread context;
- resolve the source type;
- determine expansion count when the destination requires compile-time arity;
- preserve exactly-once source evaluation;
- preserve array index order and struct declaration order;
- expand arguments before final call resolution;
- allow runtime-length expansion only into a destination such as a native typed variadic that explicitly accepts runtime arity;
- compute fixed-array result length with checked arithmetic;
- validate target element types;
- resolve struct overrides left to right;
- apply omitted-field defaults after spread/override resolution;
- reject still-missing NonDefaultable struct fields;
- validate visibility and nominal identity;
- apply copy/move/borrow rules without hidden ownership operations;
- reject hidden allocation.

---

## 12. Semantic IR and lowering

Spread semantics must be resolved before backend lowering chooses machine
operations.

Semantic IR must preserve enough information to guarantee:

- source evaluated exactly once;
- destination context;
- expansion order;
- expansion count where statically known;
- selected ownership operation for each expanded use;
- struct override resolution;
- observable evaluation order.

An implementation may retain dedicated spread operations until normalization,
or normalize spread into ordinary arguments/elements/field operations once all
semantic facts above are explicit.

MLIR and LLVM lowering must not reconstruct ownership or evaluation semantics
from source spread syntax.

---

## 13. Diagnostics

Diagnostics should identify at least:

- unsupported spread context;
- invalid source type;
- runtime expansion count used where compile-time arity is required;
- resulting call arity;
- incompatible expanded argument or element;
- move-only source restriction;
- inaccessible struct fields;
- duplicate explicit struct fields;
- omitted NonDefaultable struct fields;
- fixed-array result-length overflow.

Examples:

```text
cannot spread ref int[] into a fixed-arity call; expansion count is not known at compile time

cannot spread Resource[2] into function arguments; indexed expansion would require unsupported consuming element reads

cannot spread UserUpdate into User; spread source must have type User

spread is not valid as a standalone expression
```

---

## 14. Required tests

Valid coverage should include:

- one fixed-array spread in a function call;
- spread mixed with ordinary call arguments;
- multiple fixed-array spreads in one call;
- overload resolution after expansion;
- inferred fixed-array literal with spread;
- target-typed fixed-array literal with spread;
- multiple spreads in one fixed-array literal;
- one struct spread;
- explicit field overriding a spread-provided field;
- multiple struct spreads;
- struct spread followed by omitted-field defaulting;
- spread satisfying a NonDefaultable struct field;
- source expression evaluated exactly once;
- trailing commas after spread entries.

Invalid coverage should include:

- prefix spread;
- standalone spread;
- spread without source;
- repeated spread;
- scalar spread;
- runtime-length source into a fixed-arity call;
- call arity mismatch after expansion;
- incompatible expanded argument type;
- move-only fixed-array element call spread;
- runtime-length source in a fixed-array literal;
- fixed-array element type mismatch;
- fixed-array length overflow;
- wrong struct source type;
- inaccessible struct fields;
- missing NonDefaultable field after spread/default resolution;
- duplicate explicit struct fields;
- move-only struct spread.

---

## 15. Best practice

Use spread when it makes an already well-defined fixed expansion or same-type
struct update easier to read.

Prefer explicit code when expansion would hide ownership transfer or when a
reader would need to reason about runtime sequence length.

For struct updates, keep explicit overrides close to the spread they override:

```sec
let updated := User {
    original...
    Name: "Anna"
}
```

Do not use multiple spreads merely to simulate deep merge semantics; Sec struct
spread is deliberately shallow and nominal.

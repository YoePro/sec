# Structs

**Status:** Canonical normative rulebook
**Language version:** Sec 0.1
**Document revision:** 2.0
**Replaces:** the previous struct rulebook revision
**Created:** 2026-08-13
**Last updated:** 2026-08-13

## 1. Purpose and scope

A `struct` defines a named aggregate data type with stored fields.

Sec separates data from behavior:

- `struct` defines stored data;
- `impl` defines behavior;
- `property` defines computed or controlled member-like access;
- interfaces define behavioral contracts.

This rulebook owns the declaration, construction, defaulting, field access,
field metadata, aggregate equality, and struct-specific copy/move consequences
of structs.

It does not redefine `impl`, properties, generic rules, spread semantics,
copy/move rules, destruction, FFI layout contracts, or interface conformance.
Those areas remain governed by their own canonical rulebooks.

---

## 2. Named struct declaration

Canonical syntax:

```sec
type Coordinate struct {
    x: Meter,
    y: Meter,
    z: Meter,
}
```

A struct declaration introduced by `type` creates a nominal type.

Two separately declared struct types remain distinct even when their stored
fields have identical names and types.

Example:

```sec
type Left struct {
    value: int,
}

type Right struct {
    value: int,
}
```

`Left` and `Right` are different types. No structural conversion exists merely
because their field sets match.

An empty struct is valid:

```sec
type Marker struct {
}
```

An empty struct has no stored fields and is defaultable, trivially destructible,
and copyable unless another nominal rule such as `@noCopy` changes that
classification.

---

## 3. Struct fields

Stored fields use typed-identifier syntax:

```text
name: Type
```

Example:

```sec
type User struct {
    ID: uint64,
    Name: string,
    Active: bool,
}
```

Rules:

1. Every field has exactly one name and one resolved field type.
2. Field names must be unique within the struct.
3. Unknown field types are compile-time errors.
4. Field declaration order is semantically significant where another canonical
   rule depends on order, including default construction and destruction.
5. A stored field is data. Methods, properties, nested types, and other behavior
   do not become stored fields.
6. Field visibility follows the canonical name/scope/visibility rules.

Struct field declarations are comma-separated. A trailing comma is allowed.
Line breaks do not replace commas in the struct type declaration.

Valid:

```sec
type Point struct {
    x: int,
    y: int,
}
```

Invalid:

```sec
type Point struct {
    x: int
    y: int
}
```

A parser should report a field-specific diagnostic rather than cascading into
unrelated expression errors.

---

## 4. Struct field tags

A struct field may carry an optional raw-string field tag after its type:

```sec
type User struct {
    ID: uint64 `json:"id" db:"user_id"`,
    Name: string `json:"name"`,
    Password: string `json:"-"`,
}
```

The tag uses the Go-inspired key/value form:

```text
key:"value" key:"value"
```

Rules:

- tags are optional;
- tags belong to stored fields;
- multiple tag entries may occur in one raw tag literal;
- the compiler must not hard-code a closed list of tag keys;
- tags are metadata and do not change the Sec field type;
- tags do not change nominal struct type identity;
- tags do not independently change defaultability, copyability, ownership,
  equality, or destruction semantics;
- tags must be preserved for reflection, serialization, code generation, FFI
  tooling, or other consumers that explicitly define tag meaning;
- malformed tag syntax is a compile-time diagnostic.

Tag consumers may assign domain-specific meaning to keys such as `json`, `xml`,
`db`, `yaml`, or `csv`. Those meanings are not built into the core struct type
system merely because a tag uses one of those names.

---

## 5. Struct literals

A struct value is constructed with a named struct literal:

```sec
let user := User {
    ID: 42,
    Name: "Anna",
    Active: true,
}
```

A struct literal must name its target struct type.

The compiler validates:

- the target is a struct type;
- every explicit field exists;
- no explicit field is written more than once;
- each supplied value is assignable, movable, or otherwise constructible into
  the corresponding field according to ordinary value-transfer rules;
- every omitted field can be supplied by a spread or by its field type's
  semantic default.

Multiline struct literals may use line layout between entries. Commas remain
accepted and a trailing comma is allowed. A single-line literal must use an
unambiguous separator between entries.

Canonical multiline style may therefore be written without commas:

```sec
let user := User {
    ID: 42
    Name: "Anna"
    Active: true
}
```

or with commas:

```sec
let user := User {
    ID: 42,
    Name: "Anna",
    Active: true,
}
```

Formatting must not change the semantic meaning of the literal.

---

## 6. Omitted fields and partial literals

A struct literal does not need to list every stored field.

An omitted field is initialized from the semantic default of its declared field
type.

Example:

```sec
type Position struct {
    line: int,
    column: int,
    valid: bool,
    name: string,
}

let position := Position {
    line: 10
}
```

is semantically equivalent to a complete construction whose remaining fields
receive their type defaults:

```sec
Position {
    line: 10
    column: 0
    valid: false
    name: ""
}
```

No omitted field may remain uninitialized, undefined, or poisoned in a readable
struct value.

If an omitted field type is non-defaultable and no spread supplies that field,
the literal is invalid.

Diagnostic example:

```text
field `file` has no default value and must be initialized
```

---

## 7. Struct defaultability

A struct type is defaultable exactly when every stored field is defaultable.

Its default value is constructed field by field in declaration order using each
field type's semantic default.

Example:

```sec
type Position struct {
    line: int,
    column: int,
    valid: bool,
}
```

has the semantic default:

```sec
Position {
    line: 0
    column: 0
    valid: false
}
```

A defaultable struct may be written with an empty literal:

```sec
let position := Position {}
```

`Position {}` constructs a complete value. It does not mean uninitialized
storage.

A mutable declaration without an initializer is valid only when the struct type
is defaultable under the ordinary default-initialization rules:

```sec
let mut position: Position
```

The declaration initializes `position` to the struct default; it does not create
an uninitialized readable value.

Default construction is recursive for nested structs.

A by-value default-construction cycle is invalid.

Example conceptually:

```sec
type Invalid struct {
    next: Invalid,
}
```

cannot acquire a finite recursive default through by-value recursion.

Reference-based recursion is governed by reference/defaultability rules and is
not equivalent to by-value recursion.

---

## 8. Field-level default syntax

This rulebook defines no separate field-initializer syntax such as:

```sec
// Not canonical struct syntax in this rulebook.
type Config struct {
    timeout: Duration = Duration(30),
}
```

Canonical omitted-field behavior is based on the declared field type's semantic
default.

If another canonical rulebook introduces explicit field-level defaults, that
feature must remain compatible with existing struct literal, ownership,
construction-order, cleanup, and defaultability semantics.

---

## 9. Struct spread

Struct literals support postfix spread according to the canonical spread
rulebook:

```sec
let updated := User {
    original...
    Name: "Anna"
}
```

The struct rule does not redefine spread ownership or evaluation semantics.

For struct construction, final field resolution is conceptually:

```text
1. evaluate explicit entries and spread entries according to source evaluation order
2. resolve spread-provided fields and explicit overrides
3. reject duplicate explicit fields and other conflicts
4. initialize every remaining omitted field from its field type default
```

A spread does not bypass nominal type identity, field visibility, ownership,
copy/move rules, or non-defaultable required fields.

---

## 10. Construction and failure cleanup

Struct construction creates one complete initialized aggregate value.

If construction of a later nontrivial field fails, every already successfully
constructed owned field must be cleaned up according to the canonical
construction/destruction rules.

No failed construction may publish a partially initialized struct as a valid
source value.

Backend use of `undef`, poison, temporary aggregate slots, or partial machine
representations is permitted only when the compiler proves that no readable Sec
value can observe an uninitialized field.

---

## 11. Field access

A stored field is accessed through ordinary member syntax:

```sec
let name := user.Name
```

A field access resolves to a Place when the receiver expression provides a Place
and the field is addressable through that receiver.

Reading, assignment, moving, and borrowing of fields follow the ordinary Place,
ownership, copy/move, borrowing, and mutability rules.

Example:

```sec
user.Name = "Maria"
```

is valid only when the receiver/field place is mutable and the assignment obeys
the field type's value-transfer rules.

Struct field access must not be confused with property access. Member resolution
may ultimately select a stored field or a behavioral member, but properties are
specified by the property rulebook.

---

## 12. Copy and move classification

Struct copyability is derived according to the canonical copy/move rulebook.

A struct may be trivially or semantically copyable when all relevant fields and
nominal policies permit safe implicit copy.

A struct becomes move-only or non-copyable when required by contained fields,
resource ownership, destruction semantics, address-stability requirements, or a
nominal restriction such as `@noCopy`.

A struct literal is construction. It does not imply that its source expressions
are copied. Each field input is independently copied, moved, borrowed, or
rejected according to ordinary Sec rules.

No struct operation may hide:

- allocation;
- fallible duplication;
- external resource duplication;
- mutable alias creation;
- user-defined clone behavior.

---

## 13. Equality

An ordinary struct supports `==` and `!=` when every stored field is
semantically equality-comparable and no nominal rule forbids derived equality.

Derived struct equality compares corresponding stored fields of the same
nominal struct type using each field type's canonical equality semantics.

Field tags, properties, methods, and other non-stored behavioral members do not
participate in derived struct equality.

If any stored field is not equality-comparable, ordinary derived struct equality
is unavailable and use of `==` or `!=` is a compile-time error unless another
canonical language rule explicitly provides a valid equality operation.

Struct equality does not create structural equality between different nominal
struct types.

---

## 14. Destruction

Struct destruction is derived from the canonical destruction rulebook.

For a struct whose fields require destruction, initialized owned fields are
destroyed in reverse declaration order unless a more specialized canonical type
rule explicitly requires additional cleanup.

Example:

```sec
type Session struct {
    connection: Connection,
    buffer: Buffer,
    state: SessionState,
}
```

Normal field destruction order is:

```text
state
buffer
connection
```

Fields that are trivially destructible require no runtime destruction action.
Moved or never-initialized fields must not be destroyed as if they still contain
owned values.

---

## 15. Physical layout and ABI

Field declaration order is part of the source-level aggregate model, but an
ordinary Sec struct does not thereby acquire a stable foreign ABI layout.

A normal struct must not be assumed to be C-compatible merely because its fields
look C-compatible.

Direct FFI use requires an explicit foreign-compatible layout contract as
specified by the FFI/layout rules. Such a contract must define and validate the
relevant physical facts, including:

- field order;
- field alignment;
- field size;
- padding;
- total size;
- nested-field ABI compatibility.

Without such an explicit contract, compiler/backend representation choices must
preserve all Sec-observable semantics but are not a promise of foreign binary
layout compatibility.

Behavior added through `impl` must never alter the stored field representation
of the struct.

An explicit `extern "C" type Name struct { ... }` is the foreign-compatible
struct contract. It uses source field order and the active C ABI's natural
layout and is not implicitly Defaultable; complete explicit construction is
required. The bodyless form declares an incomplete foreign struct, which has no
known by-value size and is legal only behind `RawPtr`, or as a call-bounded
foreign `ref`/`ref mut` parameter. A final `C::flex[T]` field has no Sec
descriptor or implicit length. C bitfields use ABI-owned placement and are not
independently addressable.

---

## 16. Nested structs and associated types

A struct field may itself have a struct type.

Nested/associated type declarations under a type's behavior namespace are owned
by the canonical `impl` rulebook and are not redefined here.

A struct may refer to a qualified associated type when name resolution proves
that type exists:

```sec
type Vehicle struct {
    engine: Vehicle.Engine,
}
```

The legality, declaration placement, forward-reference behavior, and short-name
lookup of `Vehicle.Engine` belong to the `impl` and name-resolution rules.

---

## 17. Generic structs

Generic struct declarations use the canonical generics rules.

Example:

```sec
type Pair[T, U] struct {
    First: T,
    Second: U,
}
```

Defaultability, copyability, equality, destruction, and layout are resolved for
the concrete instantiation from the corresponding field types and applicable
nominal rules.

A generic declaration must not silently promise an operation that some valid
instantiations cannot support.

---

## 18. Interface conformance

When a struct declares interface conformance, its primary implementation uses
the canonical interface-conformance syntax owned by the interface and impl
rulebooks.

Example:

```sec
type FileReader struct {
    handle: FileHandle,
}

impl FileReader implements Reader {
}
```

The primary impl's `implements` clause does not add stored fields or hidden runtime
representation unless the interface representation rules explicitly require a
separate interface value at a use site.

Behavior satisfying the interface is provided through the type's allowed `impl`
surface.

---

## 19. Parser requirements

The parser must:

1. parse named struct declarations introduced by `type`;
2. require `:` between field name and field type;
3. parse comma-separated field declarations;
4. accept a trailing comma in declarations;
5. parse optional raw-string field tags;
6. parse named struct literals;
7. parse partial and empty struct literals;
8. parse struct spread entries where the spread grammar permits them;
9. recover malformed fields at a field/list boundary without cascading into
   unrelated expression diagnostics.

Suggested malformed-field diagnostic:

```text
missing ':' after struct field name "name"
```

Suggested missing-separator diagnostic:

```text
expected ',' or '}' after struct field
```

---

## 20. Semantic analysis requirements

Sema must:

- register each named struct type before resolving dependent declarations;
- resolve every field type;
- reject duplicate field names;
- preserve field declaration order;
- preserve field tags as metadata;
- resolve struct literal target types;
- reject unknown literal fields;
- reject duplicate explicit literal fields;
- apply struct spread semantics before omitted-field defaulting;
- resolve defaults for omitted fields;
- reject omission of a non-defaultable field;
- derive struct defaultability;
- derive copy/move classification;
- derive equality comparability;
- derive destruction requirements;
- preserve ownership and initialization state for every field;
- reject invalid by-value recursive layout/default cycles;
- preserve the distinction between ordinary Sec layout and explicit foreign ABI
  layout.

---

## 21. Diagnostics

Diagnostics should identify the actual struct failure rather than a downstream
backend symptom.

Important diagnostic classes include:

```text
unknown struct field type
duplicate struct field
malformed struct field
invalid struct tag
unknown struct literal field
duplicate explicit struct literal field
incompatible struct field initializer
omitted non-defaultable field
invalid struct spread source
invalid struct equality
invalid field assignment
invalid by-value recursive struct
invalid foreign struct layout
```

Where practical, diagnostics should name the struct, field, expected type, and
actual type/value category.

---

## 22. Required tests

Conformance tests must cover at least:

### Declarations

- empty struct;
- single-field struct;
- multi-field struct;
- trailing comma;
- duplicate field rejection;
- unknown field type;
- malformed field recovery;
- valid and invalid field tags.

### Literals and defaults

- complete literal;
- empty defaultable literal;
- partial literal;
- nested defaults;
- constrained named-type defaults;
- omitted non-defaultable field rejection;
- struct spread plus defaults;
- duplicate explicit literal field rejection;
- construction-failure cleanup.

### Value semantics

- copyable struct;
- move-only field causing move-only struct;
- `@noCopy` struct;
- field move/borrow behavior;
- derived equality;
- equality rejection for non-comparable fields;
- reverse field destruction order.

### Layout boundary

- ordinary struct rejected from direct FFI when no explicit compatible layout
  exists;
- explicitly laid-out foreign-compatible struct validated by the FFI/layout
  rules;
- impl/properties do not alter stored field layout.

---

## 23. Required synchronization

This rulebook must remain synchronized with the canonical rules for:

```text
default values
spread
copy and move
ownership
borrowing
references
destruction
impl
properties
interfaces
generics
FFI and explicit layout
grammar and parser recovery
LSP and diagnostics
MLIR lowering
```

The struct rulebook owns struct semantics. Adjacent rulebooks may describe how
those semantics participate in their domains, but must not silently redefine
struct construction, defaulting, nominal identity, ownership, or field
semantics.

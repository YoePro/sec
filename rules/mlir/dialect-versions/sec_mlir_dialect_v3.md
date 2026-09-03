# Sec MLIR Dialect

## Status

Normative detailed representation specification for the Sec MLIR dialect.

Current dialect schema version: `3`

This document is subordinate to:

```text
all applicable Sec language/domain rulebooks
rules/memory/layout.md
rules/types/types.md
rules/compiler/semantic_ir.md
rules/mlir/sec_mlir.md
```

It defines representation and verifier obligations.

It does not redefine Sec source-language semantics.

---

# 1. Version history

## Schema version 1

Foundation:

```text
sec namespace
!sec.named
!sec.distinct
common metadata
MLIR Location provenance
```

## Schema version 2

First Semantic IR -> Sec MLIR bridge:

```text
!sec.int
!sec.uint
!sec.float
!sec.char
!sec.rune
!sec.string
!sec.decimal
!sec.never
!sec.storage<T>

constant operations
storage operations
direct/foreign calls
func/cf integration
```

## Schema version 3

Scalar coverage and plan-aware representation completion:

```text
!sec.decimal128
active int128/int256/uint128/uint256 mapping
wide integer constant verification
decimal128 constant support
target identity metadata conventions
explicit DLTI requirement for target-resolved compiler-generated modules
```

---

# 2. Compiler-generated module markers

Schema-v3 compiler-generated modules carry:

```mlir
sec.dialect_version = 3 : i32
sec.semantic_ir_version = 1 : i32
```

Target-resolved modules additionally carry target identity metadata and an
explicit DLTI data-layout specification.

Schema-v1 and schema-v2 test modules remain parseable.

---

# 3. Target identity metadata

Reserved attributes:

```text
sec.target_os
sec.target_arch
sec.target_triple
sec.target_abi
sec.target_profile
sec.target_endianness
```

Types:

```text
StringAttr
```

Allowed `sec.target_endianness` values:

```text
little
big
```

These attributes identify the active Sec CompilationPlan.

Pointer-sized bit width is not duplicated as a `sec.*` integer attribute.

It is carried in the explicit MLIR data-layout specification.

---

# 4. DLTI requirement

Target-resolved compiler-generated Sec MLIR uses upstream DLTI.

Required Package 6 form:

```mlir
dlti.dl_spec = #dlti.dl_spec<
    #dlti.dl_entry<index, N>
>
```

where:

```text
N = active CompilationPlan canonical pointer-sized integer width
N is 32 or 64 in schema-v3 Package 6 lowering
```

Sec `int` and `uint` are not represented by MLIR `index`.

The DLTI index-width entry is a target-data carrier/query used by the scalar
resolution pass.

---

# 5. Existing identity types

Remain:

```mlir
!sec.named<"identity", base-type>
!sec.distinct<"identity", base-type>
```

Identity must not be erased by scalar layout resolution.

Their base type may be converted while the wrapper remains.

---

# 6. Semantic scalar types

Schema version 3 contains:

```text
!sec.int
!sec.uint
!sec.float
!sec.char
!sec.rune
!sec.string
!sec.decimal
!sec.decimal128
!sec.never
```

---

# 7. `!sec.int`

Target-sized signed Sec integer.

Before plan resolution:

```text
no fixed width encoded in the type
```

Under a concrete CompilationPlan:

```text
32-bit pointer-sized plan -> si32
64-bit pointer-sized plan -> si64
```

It is not MLIR `index`.

---

# 8. `!sec.uint`

Target-sized unsigned Sec integer.

Under a concrete plan:

```text
32-bit -> ui32
64-bit -> ui64
```

It is not MLIR `index`.

---

# 9. `!sec.float`

Sec source `float`.

Normative layout relationship:

```text
same semantic numeric width and native physical layout as float64
```

Plan-aware scalar resolution converts:

```text
!sec.float -> f64
```

This does not by itself define literal rounding rules.

---

# 10. `!sec.char`

Sec `char`.

Canonical native scalar representation:

```text
one byte
```

Plan-aware scalar resolution converts:

```text
!sec.char -> ui8
```

Validity rules remain Sec semantics.

---

# 11. `!sec.rune`

Sec `rune`.

Canonical native scalar representation:

```text
32-bit scalar
```

Plan-aware scalar resolution converts:

```text
!sec.rune -> ui32
```

Unicode scalar validity remains a Sec semantic obligation.

---

# 12. `!sec.string`

Unchanged high-level Sec string.

Schema version 3 does not define its physical runtime representation.

---

# 13. `!sec.decimal`

Exact Sec decimal semantic value.

Canonical lower physical components are defined by `rules/memory/layout.md` as:

```text
signed 64-bit coefficient
signed 32-bit scale
```

Schema version 3 still retains the high-level type because aggregate lowering is
not part of Package 6.

---

# 14. `!sec.decimal128`

Canonical type:

```mlir
!sec.decimal128
```

Exact Sec decimal128 semantic value.

Canonical lower physical components:

```text
signed 128-bit coefficient
signed 32-bit scale
```

It is distinct from `!sec.decimal`.

Schema version 3 retains it as a high-level type.

---

# 15. `!sec.never`

Unchanged.

No materialized scalar representation is selected by Package 6.

---

# 16. Active fixed-width integer mapping

Semantic IR active types map at high-level Sec MLIR as:

```text
int8    -> si8
int16   -> si16
int32   -> si32
int64   -> si64
int128  -> si128
int256  -> si256

uint8   -> ui8
uint16  -> ui16
uint32  -> ui32
uint64  -> ui64
uint128 -> ui128
uint256 -> ui256
```

Widths are exact.

Schema version 3 does not normalize them to signless `iN`.

---

# 17. `byte` and `bool`

High-level value mapping remains:

```text
byte -> ui8
bool -> i1
```

Important storage rule:

```text
i1 is valid as semantic SSA bool
addressable Sec bool storage is one byte
```

Therefore no schema rule may equate canonical bool storage with `memref<i1>`.

Storage lowering is governed by `rules/mlir/sec_mlir_lowering.md`.

---

# 18. `!sec.storage<T>`

Unchanged semantic storage handle.

Its element may be scalar-resolved while the storage wrapper remains.

Examples:

```text
!sec.storage<!sec.int>
    -> !sec.storage<si64> under a 64-bit plan

!sec.storage<!sec.char>
    -> !sec.storage<ui8>
```

`!sec.storage<i1>` is semantic bool storage and receives a dedicated one-byte
materialization rule during lowering.

---

# 19. Complete Semantic IR -> high-level mapping

```text
void       -> no result
never      -> !sec.never

bool       -> i1
byte       -> ui8
char       -> !sec.char
rune       -> !sec.rune
string     -> !sec.string

decimal    -> !sec.decimal
decimal128 -> !sec.decimal128

int        -> !sec.int
int8       -> si8
int16      -> si16
int32      -> si32
int64      -> si64
int128     -> si128
int256     -> si256

uint       -> !sec.uint
uint8      -> ui8
uint16     -> ui16
uint32     -> ui32
uint64     -> ui64
uint128    -> ui128
uint256    -> ui256

float      -> !sec.float
float32    -> f32
float64    -> f64
```

Named/distinct wrappers preserve identity around the mapped base.

---

# 20. Constant operations

Existing operations remain:

```text
sec.const.int
sec.const.bool
sec.const.float
sec.const.decimal
sec.const.string
```

No new decimal128-specific constant operation is added.

`sec.const.decimal` covers both exact decimal families.

---

# 21. `sec.const.int` schema-v3 verifier

Allowed plain integer result bases:

```text
!sec.int
!sec.uint
!sec.char
!sec.rune

si8
si16
si32
si64
si128
si256

ui8
ui16
ui32
ui64
ui128
ui256
```

Named/distinct wrappers around valid bases are allowed.

The value remains:

```text
StringAttr base-10 arbitrary-precision integer
```

Verifier:

```text
must parse exactly
unsigned categories reject negative
fixed-width categories reject out-of-range
no truncation
```

---

# 22. `sec.const.decimal` schema-v3 verifier

Allowed plain result bases:

```text
!sec.decimal
!sec.decimal128
```

Named/distinct wrappers are allowed.

Required attributes remain:

```text
coefficient: StringAttr
scale: IntegerAttr(i32)
lexeme: StringAttr
```

The coefficient must parse as arbitrary precision.

When the result is a concrete decimal family, the coefficient must be
representable in that family's semantic coefficient width.

No binary-float conversion.

---

# 23. Function representation

Unchanged:

```text
func.func
```

Sec target/scalar resolution may convert function parameter/result types while
preserving all Sec function metadata.

---

# 24. Call operations

Remain:

```text
sec.call.direct
sec.call.foreign
```

Package 5 may lower direct calls to `func.call`.

Package 6 scalar resolution may convert the operand/result types of either call
representation.

Foreign call identity must remain explicit.

---

# 25. Source provenance

Unchanged MLIR `Location` policy.

All scalar-resolution replacements preserve source locations.

---

# 26. Schema-v3 exclusions

Still not represented/lowered here:

```text
integer arithmetic semantic ops
float arithmetic semantic ops
decimal aggregate operations
copy/move/destruction
borrow/reference
Result/try
cleanup
aggregates
allocation
register/MMIO
concurrency
ABI lowering
LLVM dialect
```

---

# 27. Evolution rule

New target-dependent physical choices must consume the canonical Sec
CompilationPlan.

No Sec MLIR pass may derive target layout independently from:

```text
host architecture
target name spelling
heuristic LLVM defaults
```

Standard MLIR DataLayout/DLTI facilities should be used where they faithfully
carry already-resolved Sec target facts.

---

# 28. Schema-v3 completion

Schema v3 is implemented when:

```text
decimal128 type parses/prints/verifies
wide fixed integer mapping is accepted
wide integer constants verify exactly
decimal128 constants verify
target identity metadata round-trips
target-resolved generated modules carry explicit DLTI index width
schema-v1/v2 regressions remain green
```

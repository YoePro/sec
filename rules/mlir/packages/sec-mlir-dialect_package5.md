# Sec MLIR Program - Implementation Package 5

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P5`  
Package title: `Trivial Core Lowering`  
Repository: `https://github.com/YoePro/sec`  
Repository branch: `main`  
Repository sync commit used for this package: `d48035c`  
Repository sync date: `2026-08-08`  
Required Semantic IR version: `1`  
Required Sec MLIR dialect schema: `2`  
Sec MLIR dialect schema after this package: `2`

Package 5 implements the first real dialect-conversion pass inside MLIR.

It intentionally performs a **partial conversion**.

Only Sec MLIR constructs whose remaining Sec-specific obligations are completely
discharged by Packages 2-4 are lowered.

Everything else remains explicit Sec MLIR.

Package 5 does not lower to LLVM dialect and does not replace the legacy
backend.

---

# 1. Normative authority

Implementation follows:

```text
language/domain rulebooks
    ↓
rules/compiler/semantic_ir.md
    ↓
rules/mlir/sec_mlir.md
    ↓
rules/mlir/sec_mlir_dialect.md
    ↓
rules/mlir/sec_mlir_lowering.md
    ↓
implementation package
    ↓
implementation
```

At repository sync `d48035c`, `rules/mlir/sec_mlir_lowering.md` does not exist.

Before implementing Package 5, add the supplied normative file:

```text
sec_mlir_lowering_package5.md
```

to the repository as:

```text
rules/mlir/sec_mlir_lowering.md
```

Package 5 does not require a change to `rules/mlir/sec_mlir_dialect.md` because it
does not add or modify Sec dialect types or operations.

---

# 2. Why Package 5 is deliberately partial

The high-level schema-v2 dialect still contains types whose physical or
target-dependent representation is intentionally unresolved:

```text
!sec.int
!sec.uint
!sec.float
!sec.char
!sec.rune
!sec.string
!sec.decimal
!sec.named<...>
!sec.distinct<...>
```

It also contains foreign calls whose ABI lowering is intentionally unresolved:

```text
sec.call.foreign
```

Package 5 must not guess those representations.

The first lowering step is therefore limited to three cases where the
high-level semantic obligation is already complete:

```text
Sec bool constants
Package 3 trivial automatic local storage
resolved ordinary direct calls
```

This follows MLIR's normal partial dialect-conversion model: operations that are
declared illegal for a particular case must be rewritten, while other legal
high-level operations may remain in the module.

---

# 3. Package goal

After Package 5:

1. `sec-mlir-opt` contains a registered `--sec-lower-trivial-core` pass;
2. the pass uses MLIR Dialect Conversion rather than ad-hoc operation walking;
3. `sec.const.bool` becomes `arith.constant`;
4. lowerable Package 3 `!sec.storage<T>` becomes rank-0 `memref<T>`;
5. lowerable `sec.storage.declare` becomes `memref.alloca`;
6. lowerable `sec.storage.init` becomes `memref.store`;
7. lowerable `sec.storage.load` becomes `memref.load`;
8. lowerable `sec.storage.store` becomes `memref.store`;
9. `sec.call.direct` becomes `func.call`;
10. `sec.call.foreign` remains explicit Sec MLIR;
11. unresolved Sec scalar types remain untouched;
12. named/distinct type identity remains untouched;
13. no target-sized integer width is guessed;
14. no float-literal rounding rule is invented;
15. no decimal/string representation is chosen;
16. no LLVM dialect operation is created;
17. the pass is deterministic and idempotent;
18. mixed high-level/lowered MLIR verifies after the pass;
19. all Package 1-4 tests remain green.

The output remains a valid mixed-dialect MLIR module.

It is not yet "LLVM-ready MLIR".

---

# 4. Package boundary

## 4.1 In scope

Implement:

```text
rules/mlir/sec_mlir_lowering.md
SecToCore conversion library
--sec-lower-trivial-core module pass
partial dialect conversion
dynamic legality for Sec storage operations
sec.const.bool -> arith.constant
!sec.storage<T> -> memref<T> for the Package 5 lowerable subset
sec.storage.declare -> memref.alloca
sec.storage.init -> memref.store
sec.storage.load -> memref.load
sec.storage.store -> memref.store
sec.call.direct -> func.call
preservation of source locations
preservation of useful storage provenance attributes on memref.alloca
pass registration in sec-mlir-opt
lit conversion tests
negative conversion tests
idempotence tests
mixed-IR tests
real Package 4 -> Package 5 integration tests
CMake integration
full regression
```

## 4.2 Explicitly out of scope

Do not implement:

```text
LLVM dialect
LLVM IR translation
object generation
legacy backend replacement

signed/unsigned integer normalization to signless iN
!sec.int width selection
!sec.uint width selection
!sec.float representation selection
float literal rounding policy
!sec.char representation
!sec.rune representation
!sec.string representation
!sec.decimal representation
named/distinct type erasure

sec.const.int lowering
sec.const.float lowering
sec.const.decimal lowering
sec.const.string lowering

sec.call.foreign lowering
ABI lowering
calling convention lowering
link-name lowering

copy
move
destroy
cleanup
defer
borrow
reference
RawPtr

Result
Option
try
panic
checks
contracts

aggregates
allocation
arena
register/MMIO
volatile
atomics
concurrency

canonicalization as a semantic lowering requirement
CSE as a semantic lowering requirement
optimization passes
```

Do not broaden Package 5 because an upstream MLIR operation happens to make a
future representation convenient.

---

# 5. No dialect schema bump

The Sec MLIR dialect remains:

```text
schema version 2
```

Package 5 changes lowering state, not Sec dialect syntax or semantics.

Do not change:

```mlir
sec.dialect_version = 2 : i32
```

The module may still contain schema-v2 Sec types and operations after Package 5.

Do not add a `sec.lowering_stage` attribute in this package.

The operation content itself determines what has and has not yet been lowered.

---

# 6. Conversion library layout

Add:

```text
mlir/include/sec/Conversion/SecToCore/
    CMakeLists.txt
    Passes.h
    Passes.td

mlir/lib/Conversion/SecToCore/
    CMakeLists.txt
    SecToCore.cpp

mlir/test/Conversion/SecToCore/
    bool-constant.mlir
    storage.mlir
    direct-call.mlir
    mixed.mlir
    idempotent.mlir
    nonlowerable-storage.mlir
    foreign-call-remains.mlir
    metadata.mlir
    invalid-conversion-input.mlir
```

A separate source file for patterns is allowed if `SecToCore.cpp` becomes
unreasonably large:

```text
SecToCorePatterns.cpp
```

Do not add one file per trivial rewrite unless that improves clarity.

---

# 7. CMake integration

Add a conversion library, recommended target name:

```text
SecToCore
```

or a repository-consistent prefixed equivalent such as:

```text
SecMLIRSecToCore
```

It must link the MLIR components required by the implementation, including as
applicable:

```text
MLIRArithDialect
MLIRFuncDialect
MLIRControlFlowDialect
MLIRMemRefDialect
MLIRTransforms
MLIRIR
SecDialect library
```

Use current upstream MLIR CMake target names from the MLIR version used by the
project.

Do not hard-code one LLVM/MLIR source-tree layout.

Generated pass declarations from TableGen belong in the build tree.

Update `sec-mlir-opt` to register the new pass.

---

# 8. Pass definition

Canonical pass command:

```bash
sec-mlir-opt input.mlir --sec-lower-trivial-core
```

The pass is a module-level pass.

Recommended TableGen pass name:

```text
SecLowerTrivialCore
```

Recommended C++ factory:

```cpp
std::unique_ptr<mlir::Pass> createSecLowerTrivialCorePass();
```

The exact generated namespace may follow current MLIR pass conventions.

The pass must be independently invokable.

Do not hide it only inside a hard-coded pipeline.

---

# 9. Dialect Conversion is mandatory

Implement Package 5 with MLIR Dialect Conversion.

Use:

```text
ConversionTarget
RewritePatternSet
ConversionPattern / OpConversionPattern
TypeConverter where required
applyPartialConversion
```

Do not implement the package as:

```text
walk every operation
replace matching operations manually
hope the resulting module verifies
```

Reason:

Package 5 is the beginning of a multi-stage lowering pipeline. Legality must be
explicit and testable.

The conversion must fail if an operation marked illegal cannot be converted.

---

# 10. Legal target dialects

The pass must treat at least these dialects as legal:

```text
builtin
arith
func
cf
memref
```

The Sec dialect is **not globally illegal**.

Most Sec operations remain legal in this partial conversion.

Legality is selective.

---

# 11. Package 5 lowerable storage element types

A Package 5 storage element is lowerable when it is exactly one of:

```text
i1

builtin IntegerType with fixed bit width:
    8
    16
    32
    64

and signedness:
    signless
    signed
    unsigned

f32
f64
```

This includes the Package 4 representations:

```text
byte   -> ui8
int8   -> si8
int16  -> si16
int32  -> si32
int64  -> si64
uint8  -> ui8
uint16 -> ui16
uint32 -> ui32
uint64 -> ui64
float32 -> f32
float64 -> f64
bool -> i1
```

Not lowerable in Package 5:

```text
!sec.int
!sec.uint
!sec.float
!sec.char
!sec.rune
!sec.string
!sec.decimal
!sec.never
!sec.named<...>
!sec.distinct<...>
index
any aggregate/shaped/custom semantic element type
```

Do not recursively unwrap named/distinct types to make them lowerable.

Type identity must remain explicit.

---

# 12. Storage type conversion

For a lowerable element `T`:

```text
!sec.storage<T>
```

converts to a rank-0 memref:

```text
memref<T>
```

Examples:

```text
!sec.storage<i1>   -> memref<i1>
!sec.storage<si32> -> memref<si32>
!sec.storage<ui64> -> memref<ui64>
!sec.storage<f64>  -> memref<f64>
```

Rank zero is intentional.

The storage represents one scalar value, not an array of length one.

Do not lower to:

```text
memref<1xT>
```

unless a future normative change explicitly selects that representation.

Do not lower to:

```text
LLVM pointer
memref<?xT>
tensor<T>
```

---

# 13. TypeConverter

Package 5 may use a narrowly scoped `TypeConverter`.

Required behavior:

```text
!sec.storage<T> -> memref<T>
    only when T is Package 5 lowerable

all other types -> identity
```

The converter must not:

```text
convert si32 to i32
convert ui32 to i32
convert !sec.int
convert !sec.uint
convert named/distinct
convert decimal/string
```

No general target type normalization belongs in this package.

Because Package 3 storage handles are local and do not appear in function
signatures, the pass should not need broad function-signature conversion merely
to lower storage.

If the actual Package 4 implementation allows storage handles in block
arguments, use the normal Dialect Conversion signature-conversion mechanisms
rather than inserting unverified casts.

---

# 14. `sec.const.bool` lowering

Every:

```text
sec.const.bool
```

is illegal after Package 5.

It must lower to:

```text
arith.constant
```

with result:

```text
i1
```

Example:

```mlir
%0 = "sec.const.bool"() {value = true} : () -> i1
```

becomes:

```mlir
%0 = arith.constant true
```

or the canonical equivalent accepted by the current MLIR version.

Preserve the original operation location.

Do not preserve a redundant `sec.const.bool` marker after successful lowering.

This lowering is safe because schema version 2 already fixed Sec `bool` to
`i1`.

---

# 15. Integer constants remain Sec operations

Do **not** lower:

```text
sec.const.int
```

in Package 5.

Reason:

Package 4 deliberately preserves signed/unsigned fixed-width types such as
`si32` and `ui32`, while the broader Arith integer-operation pipeline primarily
uses signless integer semantics.

Signedness normalization is a distinct lowering decision and must be defined
together with the later arithmetic/target representation policy.

Package 5 must not erase or reinterpret that information merely to use
`arith.constant`.

This also keeps target-sized:

```text
!sec.int
!sec.uint
```

fully unresolved.

---

# 16. Float constants remain Sec operations

Do **not** lower:

```text
sec.const.float
```

in Package 5.

Reason:

schema version 2 preserves the original lexeme and intentionally postpones final
rounding for `!sec.float`.

Even for explicit `f32`/`f64`, the Sec language's literal rounding policy must be
normatively fixed before a lowering pass commits the lexeme to an `APFloat`
value.

Package 5 must not invent that language rule.

---

# 17. Decimal and string constants remain Sec operations

Do not lower:

```text
sec.const.decimal
sec.const.string
```

Their physical/runtime representations remain unresolved.

---

# 18. `sec.storage.declare` lowering

For:

```text
sec.storage.declare
```

whose result is lowerable:

```text
!sec.storage<T>
```

create:

```text
memref.alloca() : memref<T>
```

with rank zero.

No dynamic dimensions.

No symbol operands.

No explicit `memref.dealloc`.

The allocation is automatic storage.

Preserve the original source location.

Copy these Sec provenance attributes from the original declaration onto the
`memref.alloca`:

```text
sec.storage_id
sec.source_name
sec.storage_class
sec.mutable
```

They remain dialect attributes on a standard operation.

Do not copy attributes that are invalid on the target operation.

The conversion must not change their values.

---

# 19. Why `memref.alloca` is valid here

Package 3 storage eligible for Package 5 is:

```text
automatic local
trivial
non-owning
non-reference
non-escaping through the Package 3 call model
no observable destruction
```

Therefore stack-backed automatic storage is a semantics-preserving lower
representation for this subset.

`memref.alloca` provides automatic stack allocation scoped by MLIR's automatic
allocation scope.

Package 5 does not generalize this decision to future:

```text
escaping locals
closures
borrowed storage
aggregates
dynamic allocation
arenas
resource-owning values
```

Those require separate lowering rules.

---

# 20. `sec.storage.init` lowering

For converted storage:

```mlir
"sec.storage.init"(%slot, %value)
```

becomes rank-0:

```mlir
memref.store %value, %slot[] : memref<T>
```

Preserve location.

Do not emit a separate initialization marker.

The distinction between first initialization and later store has already been
validated by Semantic IR before this pass, and the Package 5 storage type has no
destruction or ownership behavior.

This discharge rule applies only to the Package 5 lowerable subset.

---

# 21. `sec.storage.load` lowering

For converted storage:

```mlir
%value = "sec.storage.load"(%slot)
```

becomes:

```mlir
%value = memref.load %slot[] : memref<T>
```

Preserve location.

No copy/move/borrow metadata is emitted because Package 5 lowerable storage
loads are already defined as trivial scalar copies by Package 3.

---

# 22. `sec.storage.store` lowering

For converted storage:

```mlir
"sec.storage.store"(%slot, %value)
```

becomes:

```mlir
memref.store %value, %slot[] : memref<T>
```

Preserve location.

Do not insert:

```text
destroy
copy
move
cleanup
```

Package 5 is allowed to collapse replacement to a store only because the
eligible element is trivial under the Package 3 contract.

---

# 23. Non-lowerable storage remains Sec MLIR

For example:

```text
!sec.storage<!sec.int>
!sec.storage<!sec.decimal>
!sec.storage<!sec.named<"main::ID", si32>>
```

must remain:

```text
sec.storage.declare
sec.storage.init
sec.storage.load
sec.storage.store
```

after Package 5.

The pass must not fail merely because legal non-lowerable storage remains.

This is the purpose of partial conversion.

---

# 24. Storage conversion consistency

If a storage declaration is lowerable, every schema-v2 storage use of that
storage must also be converted.

The conversion target must not allow a mixed state such as:

```text
memref.alloca
sec.storage.load
```

for one lowerable storage identity.

Use the converted operand type and dynamic legality to ensure the entire
lowerable storage chain is rewritten.

If conversion cannot complete consistently, the pass fails.

---

# 25. `sec.call.direct` lowering

Every schema-v2:

```text
sec.call.direct
```

must lower to:

```text
func.call
```

using the same target symbol, operand order and result order.

Example:

```mlir
%r = "sec.call.direct"(%a) {
    callee = @sec_fn_0,
    sec.argument_actions = ["copy-trivial"]
} : (si32) -> si32
```

becomes:

```mlir
%r = func.call @sec_fn_0(%a) : (si32) -> si32
```

Preserve location.

The target function remains the exact symbol selected by Semantic IR and
Package 4.

No symbol lookup by source name is performed.

---

# 26. Discharging `sec.argument_actions`

Schema version 2 permits only:

```text
copy-trivial
```

for Package 4 direct calls.

When lowering `sec.call.direct` to `func.call`, Package 5 removes:

```text
sec.argument_actions
```

because the only remaining action is a trivial by-value transfer and `func.call`
already represents that lower operation sufficiently.

The conversion pattern must first verify or rely on the dialect verifier that:

```text
action count == operand count
every action == "copy-trivial"
```

If any other action somehow reaches Package 5, the conversion fails.

Do not silently discard unknown actions.

---

# 27. `sec.call.foreign` remains

Do not lower:

```text
sec.call.foreign
```

in Package 5.

It remains the explicit representation of a foreign/extern boundary.

Reason:

future lowering still needs:

```text
ABI
calling convention
link name
target ABI width
foreign integer extension behavior
ownership contracts
nullability contracts
```

A normal `func.call` would not by itself represent all of those obligations.

Foreign-call lowering belongs to a later ABI package.

---

# 28. Function declarations remain `func.func`

Package 4 already uses:

```text
func.func
```

Package 5 does not rewrite function declarations or definitions.

Preserve all Sec function metadata, including:

```text
sec.function_id
sec.source_name
sec.extern
sec.unsafe
sec.link_name
sec.abi
sec.parameter_names
```

when present.

Do not remove function identity metadata simply because direct calls have
lowered to `func.call`.

---

# 29. CFG remains standard

Package 4 already uses:

```text
func.return
cf.br
cf.cond_br
MLIR block arguments
```

Package 5 leaves them unchanged.

Normal MLIR verification remains authoritative for:

```text
SSA dominance
branch target existence
branch operand arity/type
function return type
```

Do not add a duplicate Sec CFG verifier.

---

# 30. Conversion target legality

The conversion target must express at least these rules.

Always legal:

```text
builtin dialect
arith dialect
func dialect
cf dialect
memref dialect
```

Sec dialect rules:

```text
sec.const.bool
    illegal: always

sec.call.direct
    illegal: always

sec.call.foreign
    legal

sec.const.int
sec.const.float
sec.const.decimal
sec.const.string
    legal

sec.storage.declare
sec.storage.init
sec.storage.load
sec.storage.store
    dynamically legal only when they do NOT belong to a Package 5
    lowerable storage chain

all other schema-v2 Sec operations
    legal unless explicitly handled by this package
```

Unknown future Sec operations must not accidentally become illegal merely
because the Sec dialect is registered.

Package 5 is not a full Sec-dialect elimination pass.

---

# 31. Conversion failure policy

`--sec-lower-trivial-core` fails when:

```text
an illegal Package 5 operation has no successful rewrite
a lowerable storage chain cannot be converted consistently
a direct call carries an unsupported argument action
a rewrite would violate result/operand types
the resulting IR fails MLIR verification
```

It does not fail merely because legal high-level Sec operations remain.

Failure is a compiler/lowering error.

It is not a Sec source diagnostic.

---

# 32. No `unrealized_conversion_cast` residue

Package 5 must not intentionally leave:

```text
builtin.unrealized_conversion_cast
```

for its supported lowerings.

The storage conversion is closed over its Package 3 use set.

Direct call lowering does not change value types.

Bool lowering does not change result type.

If an implementation approach produces unrealized casts transiently, all such
casts introduced by this pass must be reconciled before the pass succeeds.

Add a test proving no new unrealized conversion cast remains in normal Package 5
output.

---

# 33. Pass idempotence

Running:

```bash
sec-mlir-opt input.mlir \
    --sec-lower-trivial-core \
    --sec-lower-trivial-core
```

must produce the same semantic result as running the pass once.

After one successful run there must be no remaining:

```text
sec.const.bool
sec.call.direct
Package-5-lowerable sec.storage.*
```

A second run therefore has nothing additional to lower.

Test byte-level output after canonical printing where practical.

---

# 34. Determinism

The pass must not introduce nondeterministic symbol names, block order or
attributes.

Package 5 creates no new global symbols.

`memref.alloca`, `arith.constant` and `func.call` are created at the location of
the replaced operations.

Do not use pointer addresses or unordered-map iteration to generate textual
identities.

---

# 35. Source provenance

Every replacement operation uses the source operation's MLIR `Location`.

Examples:

```text
sec.const.bool -> arith.constant
sec.storage.declare -> memref.alloca
sec.storage.init -> memref.store
sec.storage.load -> memref.load
sec.storage.store -> memref.store
sec.call.direct -> func.call
```

Storage provenance attributes copied to `memref.alloca` supplement but do not
replace MLIR locations.

---

# 36. Optimization boundary

Package 5 is a lowering package, not an optimization package.

Do not require:

```text
canonicalize
CSE
mem2reg
SROA
inlining
dead-code elimination
```

for correctness.

Tests may optionally run canonicalization after the pass to prove compatibility,
but acceptance must not depend on optimization changing semantics.

A later package may use promotion to eliminate trivial memrefs.

---

# 37. Required pass-registration behavior

`sec-mlir-opt --help` must expose:

```text
--sec-lower-trivial-core
```

The pass must also be usable inside a pass pipeline supported by the installed
MLIR version.

Example conceptual form:

```bash
sec-mlir-opt input.mlir \
    --pass-pipeline='builtin.module(sec-lower-trivial-core)'
```

Use the exact syntax supported by the chosen MLIR release.

Report the successful invocation in the completion report.

---

# 38. Required valid conversion tests

## V01 - bool constant

Input contains:

```text
sec.const.bool
```

Expected output:

```text
arith.constant
```

and no `sec.const.bool`.

## V02 - int32 storage

Input:

```text
!sec.storage<si32>
```

with declare/init/load/store.

Expected:

```text
memref<si32>
memref.alloca
memref.store
memref.load
```

No Sec storage operations remain for that storage.

## V03 - uint64 storage

Expected:

```text
memref<ui64>
```

with rank zero.

## V04 - bool storage

Expected:

```text
memref<i1>
```

## V05 - float64 storage

Expected:

```text
memref<f64>
```

## V06 - rank-zero access

Expected load/store syntax has zero indices:

```text
%v = memref.load %slot[] : memref<...>
memref.store %v, %slot[] : memref<...>
```

## V07 - provenance attributes

`memref.alloca` retains:

```text
sec.storage_id
sec.source_name
sec.storage_class
sec.mutable
```

with exact values.

## V08 - direct void call

Expected:

```text
func.call
```

with no result.

## V09 - direct value call

Result and operands preserved.

## V10 - overload symbol preserved

Two functions with lower-compatible signatures still use the exact Package 4
mapped MLIR target symbol.

No source-name resolution occurs.

## V11 - foreign call remains

Input:

```text
sec.call.foreign
```

Expected output still contains:

```text
sec.call.foreign
```

## V12 - target-sized int storage remains

Input:

```text
!sec.storage<!sec.int>
```

Expected Sec storage operations remain.

## V13 - decimal storage remains

Input:

```text
!sec.storage<!sec.decimal>
```

Expected Sec storage operations remain.

## V14 - named storage remains

Input:

```text
!sec.storage<!sec.named<"main::ID", si32>>
```

Expected Sec storage remains.

Do not unwrap the named type.

## V15 - integer constant remains

`sec.const.int` remains unchanged.

## V16 - float constant remains

`sec.const.float` remains unchanged.

## V17 - decimal constant remains

`sec.const.decimal` remains unchanged.

## V18 - string constant remains

`sec.const.string` remains unchanged.

## V19 - mixed module

A module containing:

```text
bool constant
int32 storage
target-sized int storage
direct call
foreign call
decimal constant
```

must partially lower exactly the eligible constructs and verify successfully.

---

# 39. Required negative conversion tests

## N01 - unsupported direct-call action

Construct a direct call with an action other than:

```text
copy-trivial
```

If dialect verification rejects it before the pass, that is acceptable.

The pass must never silently drop it.

## N02 - lowerable storage type mismatch

Malformed storage chain must fail dialect verification or conversion.

## N03 - converted declare with unconverted lowerable use

Create a test seam/pattern-failure case if practical.

Expected partial conversion failure.

The implementation must prove the conversion target does not permit a
half-converted lowerable storage chain.

## N04 - no unrealized casts

Normal successful Package 5 output must contain no:

```text
builtin.unrealized_conversion_cast
```

introduced by the pass.

---

# 40. Required idempotence test

Run once:

```bash
sec-mlir-opt input.mlir --sec-lower-trivial-core
```

Run twice:

```bash
sec-mlir-opt input.mlir \
    --sec-lower-trivial-core \
    --sec-lower-trivial-core
```

Expected canonical output:

```text
semantically identical
no additional Sec operations disappear on run two
```

Prefer exact FileCheck invariants rather than fragile whitespace comparison.

---

# 41. Required Package 4 integration tests

Generate high-level Sec MLIR through the Package 4 path, then lower it.

Pipeline:

```text
Sec source
    ↓
Sema
    ↓
Semantic IR
    ↓
Package 4 Sec MLIR schema v2
    ↓
sec-mlir-opt --sec-lower-trivial-core
    ↓
verified mixed core MLIR
```

Required source cases:

```text
bool return
mutable int32 local
mutable uint64 local
mutable float64 local
direct call
if/else with trivial mutable storage
nested if with trivial mutable storage
mixed direct + foreign call when a valid foreign-call fixture exists
```

The Package 5 pass must consume the Package 4 output without hand-editing.

---

# 42. Required MLIR lit files

Recommended:

```text
mlir/test/Conversion/SecToCore/bool-constant.mlir
mlir/test/Conversion/SecToCore/storage.mlir
mlir/test/Conversion/SecToCore/direct-call.mlir
mlir/test/Conversion/SecToCore/mixed.mlir
mlir/test/Conversion/SecToCore/nonlowerable-storage.mlir
mlir/test/Conversion/SecToCore/foreign-call-remains.mlir
mlir/test/Conversion/SecToCore/metadata.mlir
mlir/test/Conversion/SecToCore/idempotent.mlir
mlir/test/Conversion/SecToCore/no-unrealized-casts.mlir
```

Package 1/4 dialect tests stay separate from conversion tests.

---

# 43. Required C++ unit tests

Use lit tests as the primary test surface.

Add C++ unit tests only for logic awkward to validate through textual IR, for
example:

```text
isPackage5LowerableStorageElementType
```

if that predicate is non-trivial enough to justify direct testing.

Do not duplicate every lit test in C++.

---

# 44. `sec-mlir-opt` verification

All conversion tests run with normal verification enabled.

Do not use:

```text
--allow-unregistered-dialect
```

Do not disable the verifier.

Where supported by the current MLIR release, include:

```text
--verify-each
```

in representative pipeline tests.

---

# 45. Real MLIR build/test commands

Use the Package 1 out-of-tree build.

Representative commands:

```bash
cmake -S mlir -B build/sec-mlir -G Ninja \
    -DMLIR_DIR=<mlir-cmake-dir> \
    -DLLVM_DIR=<llvm-cmake-dir>

cmake --build build/sec-mlir

cmake --build build/sec-mlir --target check-sec-mlir
```

Also run representative conversion commands directly:

```bash
build/sec-mlir/bin/sec-mlir-opt \
    input.mlir \
    --sec-lower-trivial-core \
    --verify-each \
    -o output.mlir
```

Adjust executable location to the actual CMake layout.

Report exact commands and MLIR/LLVM version.

---

# 46. Go/compiler regression

Package 5 does not require Go compiler API changes.

Run:

```bash
go test ./...
```

Expected:

```text
PASS
```

Package 4:

```text
sec emit-sec-mlir
```

must continue to emit **high-level schema-v2 Sec MLIR**, not automatically run
Package 5.

Reason:

the high-level dump remains useful for debugging the Semantic IR -> Sec MLIR
bridge.

Do not silently change that command's output stage.

---

# 47. Legacy regression

Existing:

```text
emit-mlir
emit-llvm
build
internal/codegen/mlir
internal/codegen/llvm
```

remain unchanged.

Package 5 is not the migration point for the production backend.

---

# 48. Architecture rules

These are non-negotiable:

```text
Package 5 uses partial dialect conversion.

Sec dialect is not globally illegal.

Only fully discharged semantics are lowered.

Target-sized types remain Sec types.

Named/distinct identity remains explicit.

Decimal/string remain semantic Sec types.

Foreign calls remain Sec operations.

Trivial automatic storage may become rank-0 memref.

Package 5 memref allocation is stack automatic storage only.

No heap promotion is introduced.

No explicit dealloc is emitted for memref.alloca.

Direct ordinary calls become func.call.

Unknown argument actions are never discarded.

Locations are preserved.

Lowering correctness does not depend on optimization passes.

No LLVM dialect operation is generated.

No AST/Sema information is consulted.

No legacy codegen fallback exists.
```

---

# 49. Acceptance criteria

Package 5 is complete only when:

```text
[ ] Packages 1-4 remain green
[ ] rules/mlir/sec_mlir_lowering.md added from supplied normative file
[ ] Sec dialect schema remains version 2
[ ] SecToCore conversion library builds
[ ] --sec-lower-trivial-core is registered
[ ] pass uses applyPartialConversion
[ ] ConversionTarget legality is explicit
[ ] sec.const.bool always lowers to arith.constant
[ ] sec.const.bool is absent after successful pass
[ ] lowerable !sec.storage<T> becomes rank-0 memref<T>
[ ] lowerable sec.storage.declare becomes memref.alloca
[ ] lowerable sec.storage.init becomes memref.store
[ ] lowerable sec.storage.load becomes memref.load
[ ] lowerable sec.storage.store becomes memref.store
[ ] storage provenance attrs are preserved on memref.alloca
[ ] no memref.dealloc is generated for alloca storage
[ ] non-lowerable Sec storage remains valid Sec MLIR
[ ] named/distinct storage is not unwrapped
[ ] sec.call.direct always lowers to func.call
[ ] exact target symbol is preserved
[ ] copy-trivial action is discharged only after validation
[ ] sec.call.foreign remains Sec MLIR
[ ] sec.const.int remains Sec MLIR
[ ] sec.const.float remains Sec MLIR
[ ] sec.const.decimal remains Sec MLIR
[ ] sec.const.string remains Sec MLIR
[ ] !sec.int/!sec.uint/!sec.float remain unresolved
[ ] no signedness normalization occurs
[ ] no unrealized_conversion_cast remains from normal Package 5 conversion
[ ] pass is idempotent
[ ] pass is deterministic
[ ] normal MLIR verification remains enabled
[ ] mixed high-level/lowered output verifies
[ ] Package 4 generated output lowers without hand editing
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy compiler paths remain unchanged
[ ] no LLVM dialect operation is emitted
```

---

# 50. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. Packages 1-4 pre-status
3. rules/mlir/sec_mlir_lowering.md addition
4. files added
5. files modified
6. pass name and registration method
7. conversion library target name
8. ConversionTarget legality rules
9. TypeConverter rules
10. exact lowerable storage element predicate
11. bool constant rewrite
12. storage rewrite strategy
13. direct-call rewrite strategy
14. handling of sec.argument_actions
15. proof/statement that foreign calls remain
16. proof/statement that target-sized types remain
17. CMake configure/build commands
18. exact LLVM/MLIR version
19. check-sec-mlir result
20. representative direct conversion commands
21. Package 4 -> Package 5 integration test results
22. go test ./... result
23. idempotence result
24. deviations
25. issues discovered for Package 6
```

Do not silently broaden the pass.

---

# 51. Package 6 boundary

Package 6 should define the **scalar representation and target-data lowering
layer**.

It should not be mixed into Package 5 because it requires additional normative
decisions.

Recommended Package 6 scope:

```text
target information carried in MLIR using a defined target/data-layout model
DLTI integration where appropriate
target-sized !sec.int representation
target-sized !sec.uint representation
default !sec.float representation
fixed-width signed/unsigned normalization to signless MLIR integers
preservation of signedness semantics in operation selection/metadata
sec.const.int lowering after integer representation is fixed
sec.const.float lowering after float literal rounding is normatively fixed
function/block/call type conversion required by scalar normalization
conversion legality tests
target-width tests for 32-bit and 64-bit targets
```

Package 6 must still defer:

```text
decimal runtime representation
string runtime representation
named/distinct type erasure policy
foreign ABI lowering
copy/move/destruction
borrow/reference
Result/try
defer/cleanup
aggregates
allocation
register/MMIO
concurrency
```

The intended next invariant is:

```text
Package 5 mixed core MLIR
    ↓
target/data-layout-aware scalar conversion
    ↓
standard fixed scalar MLIR suitable for later LLVM conversion

while all still-unresolved Sec semantics remain explicit.
```

---

# 52. Upstream MLIR implementation references

Package 5 should be implemented against the exact LLVM/MLIR version selected by
the project.

Relevant upstream concepts:

- MLIR Dialect Conversion:
  `https://mlir.llvm.org/docs/DialectConversion/`
- MemRef dialect:
  `https://mlir.llvm.org/docs/Dialects/MemRef/`
- Func dialect:
  `https://mlir.llvm.org/docs/Dialects/Func/`
- Arith dialect:
  `https://mlir.llvm.org/docs/Dialects/ArithOps/`
- MLIR Data Layout modeling, relevant mainly to Package 6:
  `https://mlir.llvm.org/docs/DataLayout/`

Use current APIs from the exact installed MLIR release rather than copying API
spelling from documentation for a different release.

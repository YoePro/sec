# Sec MLIR Program - Implementation Package 6

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P6`  
Package title: `Scalar Coverage and Plan-Aware Resolution`  
Repository: `https://github.com/YoePro/sec`  
Repository branch: `main`  
Repository sync commit used for this package: `d48035c`  
Repository sync date: `2026-08-08`  
Semantic IR version: `1`  
Sec MLIR dialect schema before package: `2`  
Sec MLIR dialect schema after package: `3`  
Sec MLIR lowering specification after package: `2`

Package 6 closes two representation gaps discovered while reconciling the
earlier implementation packages with the current normative rulebooks, then adds
the first target-plan-aware scalar type resolution pass.

This package does **not** normalize integer signedness to signless MLIR types and
does **not** lower to LLVM.

---

# 1. Normative authority

Implementation follows:

```text
language/domain rulebooks
    ↓
rules/memory/layout.md
rules/types/types.md
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

Before implementing Package 6:

1. replace/update `rules/mlir/sec_mlir_dialect.md` with the supplied
   `sec_mlir_dialect_package6.md`;
2. replace/update `rules/mlir/sec_mlir_lowering.md` with the supplied
   `sec_mlir_lowering_package6.md`.

Package 6 must not alter higher-authority source-language semantics.

---

# 2. Mandatory reconciliation with current normative rules

The earlier package instructions were intentionally narrow, but two assumptions
must now be corrected before further lowering.

## 2.1 Active wide scalar types

Current `rules/types/types.md` states that these integer types are active:

```text
int128
int256
uint128
uint256
```

and that `decimal128` is also an active concrete decimal type.

Therefore Package 6 must extend the Semantic IR and Sec MLIR bridge so those
active language types are represented rather than rejected as future types.

This is a coverage correction, not a new source-language feature.

## 2.2 Addressable bool layout

Current `rules/memory/layout.md` states:

```text
an addressable bool occupies one byte;
SSA/register representation may be narrower;
i1 does not change canonical one-byte stored layout.
```

Therefore Package 5 must not lower:

```text
!sec.storage<i1>
```

to:

```text
memref<i1>
```

The Package 5 implementation must be corrected so `i1` is excluded from its
generic trivial-storage predicate.

Package 6 then performs the correct bool-storage lowering:

```text
semantic bool SSA: i1
addressable bool storage: memref<i8>
```

with explicit conversion at the storage boundary.

Do not continue with a `memref<i1>` canonical bool storage model.

---

# 3. Relevant scalar layout rules

Package 6 implements only scalar facts already fixed by `rules/memory/layout.md`.

Canonical facts:

```text
int / uint:
    canonical pointer-sized integer width of active CompilationPlan

float:
    same semantic numeric width and native physical layout as float64

char:
    one byte

rune:
    32-bit scalar representation

bool:
    SSA may use i1
    addressable storage uses one byte

fixed-width integers:
    exact declared width, including 128 and 256

decimal:
    canonical physical components { signed i64 coefficient, i32 scale }

decimal128:
    canonical physical components { signed i128 coefficient, i32 scale }
```

Package 6 resolves only the scalar types for which a standard non-aggregate MLIR
type is sufficient.

It records but does not yet lower decimal/decimal128 aggregate representation.

---

# 4. Package goal

After Package 6:

1. Semantic IR version 1 supports the currently active wide scalar types;
2. `decimal128` has an explicit high-level Sec MLIR type;
3. Package 4 type mapping covers all active scalar types;
4. `sec.const.int` verifier accepts exact 128/256-bit fixed-width types;
5. `sec.const.decimal` supports both decimal families;
6. compiler target definitions expose resolved scalar-plan facts centrally;
7. compiler-generated high-level Sec MLIR carries explicit target identity;
8. compiler-generated high-level Sec MLIR carries explicit DLTI index width;
9. the MLIR pass never relies on MLIR's default 64-bit `index` width;
10. `!sec.int` resolves from the active plan's pointer width;
11. `!sec.uint` resolves from the active plan's pointer width;
12. `!sec.float` resolves to `f64`;
13. `!sec.char` resolves to `ui8`;
14. `!sec.rune` resolves to `ui32`;
15. named/distinct identity is retained while supported base types resolve;
16. fixed-width signed/unsigned integer types remain signed/unsigned MLIR types;
17. exact integer constants whose result becomes a builtin integer lower to
    `arith.constant`;
18. bool addressable storage lowers to `memref<i8>`, never `memref<i1>`;
19. target-resolved trivial storage may then lower through the corrected
    SecToCore storage patterns;
20. direct and foreign calls remain semantically distinguishable;
21. foreign calls remain `sec.call.foreign`;
22. decimal and decimal128 remain high-level Sec types;
23. no LLVM dialect operations are emitted;
24. all earlier packages remain green after the required corrections.

---

# 5. Package boundary

## 5.1 In scope

Implement:

```text
Semantic IR support for int128
Semantic IR support for int256
Semantic IR support for uint128
Semantic IR support for uint256
Semantic IR support for decimal128

Package 3 trivial scalar storage/call support for fixed 128/256 integers
Package 3 semantic storage/call representation for decimal128 where no ownership
semantics are required

Sec MLIR dialect schema version 3
!sec.decimal128

Package 4 mappings:
    int128  -> si128
    int256  -> si256
    uint128 -> ui128
    uint256 -> ui256
    decimal128 -> !sec.decimal128

sec.const.int verifier extension to 128/256
sec.const.decimal verifier/result extension to decimal128

central resolved scalar target plan
target pointer width
target endianness identity
target OS
target architecture
LLVM target triple
selected ABI/profile identity when available

DLTI dialect registration
explicit dlti.dl_spec index-width emission

corrected Package 5 bool-storage legality

--sec-resolve-scalar-layout pass
DataLayout query usage
function signature type conversion
block argument type conversion
call operand/result type conversion
Sec storage element type conversion
named/distinct base-type conversion without erasing identity

!sec.int -> si32/si64 according to plan
!sec.uint -> ui32/ui64 according to plan
!sec.float -> f64
!sec.char -> ui8
!sec.rune -> ui32

sec.const.int -> arith.constant when result type is now a builtin integer
bool storage -> rank-zero memref<i8>
bool store/init i1 -> i8 zero-extension
bool load i8 -> i1 truncation

corrected scalar-core pass pipeline
32-bit target tests
64-bit target tests
wide integer tests
target metadata tests
full MLIR and Go regressions
```

## 5.2 Explicitly out of scope

Do not implement:

```text
integer signedness erasure to signless iN
integer arithmetic
integer comparison lowering
integer division lowering
integer shift lowering
integer cast lowering
overflow behavior lowering

float literal rounding policy
sec.const.float lowering

decimal physical aggregate lowering
decimal128 physical aggregate lowering
decimal arithmetic
decimal conversions

string lowering
string runtime representation

named/distinct identity erasure
unit erasure
contract erasure

foreign-call ABI lowering
calling convention lowering
link-name lowering

LLVM dialect
LLVM IR
object generation

ownership
copy
move
destruction
cleanup
defer
borrow
references
RawPtr

Result
Option
try
panic
runtime checks

aggregates
allocation
arena
register/MMIO
volatile
atomics
concurrency
```

Do not add placeholders for these areas.

---

# 6. Semantic IR coverage correction

Keep:

```go
const Version uint32 = 1
```

The Package 6 scalar additions are completion of the already-defined Semantic IR
version 1 type universe.

Do not bump the Semantic IR version for this additive implementation completion.

Add active canonical scalar kinds:

```text
int128
int256
uint128
uint256
decimal128
```

Required metadata:

```text
int128:
    signed = true
    bit width = 128
    target-sized = false

int256:
    signed = true
    bit width = 256
    target-sized = false

uint128:
    signed = false
    bit width = 128
    target-sized = false

uint256:
    signed = false
    bit width = 256
    target-sized = false

decimal128:
    exact decimal family
    coefficient semantic width = 128
    scale semantic width = 32
    target-sized = false
```

Do not model `decimal128` as `decimal`.

They are distinct concrete scalar types.

---

# 7. Package 2 type-table correction

Update the Package 2 canonical type table active list to include:

```text
int128
int256
uint128
uint256
decimal128
```

Type interning requirements apply normally.

Required tests:

```text
int128 interns independently from int64
int256 interns independently from int128
uint128 differs from int128
uint256 differs from uint128
decimal128 differs from decimal
```

Integer constant representation remains arbitrary precision.

No host `int64`/`uint64` truncation is permitted.

---

# 8. Package 3 scalar-subset correction

The following fixed-width wide integers are trivial scalar values for the
Package 3 storage/call subset:

```text
int128
int256
uint128
uint256
```

Add them to the Package 3 supported trivial scalar predicate.

`decimal128` may be represented by Package 3 Semantic IR storage/calls as a
scalar semantic value because Package 3 storage remains semantic storage and
does not choose its physical aggregate representation.

However:

```text
Package 5 must not lower decimal128 storage to memref
Package 6 must not lower decimal128 storage to memref
```

until the physical aggregate lowering package.

---

# 9. Dialect schema version 3

Package 6 increments:

```text
sec.dialect_version = 3 : i32
```

Reason:

the dialect type surface changes by adding `!sec.decimal128` and the schema
explicitly recognizes the currently active wide scalar mapping.

Schema versions 1 and 2 remain parseable for their regression tests.

Compiler-generated high-level Sec MLIR emitted after Package 6 uses schema 3.

---

# 10. New type: `!sec.decimal128`

Canonical type:

```mlir
!sec.decimal128
```

Meaning:

```text
exact Sec decimal128 semantic value
coefficient semantic width = 128 bits signed
scale semantic width = 32 bits
```

It does not yet define a lower aggregate MLIR representation.

It must not be replaced with:

```text
f128
f64
i128
!sec.decimal
```

It remains distinct from `!sec.decimal`.

---

# 11. Complete Package 6 high-level scalar mapping

High-level Semantic IR -> Sec MLIR mapping after Package 6:

| Semantic IR | High-level Sec MLIR |
| --- | --- |
| `void` | no result type |
| `never` | `!sec.never` |
| `bool` | `i1` |
| `byte` | `ui8` |
| `char` | `!sec.char` |
| `rune` | `!sec.rune` |
| `string` | `!sec.string` |
| `decimal` | `!sec.decimal` |
| `decimal128` | `!sec.decimal128` |
| `int` | `!sec.int` |
| `int8` | `si8` |
| `int16` | `si16` |
| `int32` | `si32` |
| `int64` | `si64` |
| `int128` | `si128` |
| `int256` | `si256` |
| `uint` | `!sec.uint` |
| `uint8` | `ui8` |
| `uint16` | `ui16` |
| `uint32` | `ui32` |
| `uint64` | `ui64` |
| `uint128` | `ui128` |
| `uint256` | `ui256` |
| `float` | `!sec.float` |
| `float32` | `f32` |
| `float64` | `f64` |

Named and distinct wrappers retain identity:

```mlir
!sec.named<"identity", mapped-base>
!sec.distinct<"identity", mapped-base>
```

---

# 12. Constant verifier correction

## 12.1 Integer constants

Extend `sec.const.int` accepted fixed-width integer bases to include:

```text
si128
si256
ui128
ui256
```

Representability checking remains arbitrary precision.

Required boundaries:

```text
si128:
    -2^127 .. 2^127 - 1

ui128:
    0 .. 2^128 - 1

si256:
    -2^255 .. 2^255 - 1

ui256:
    0 .. 2^256 - 1
```

No truncation or host-width conversion.

## 12.2 Decimal constants

`sec.const.decimal` may produce:

```text
!sec.decimal
!sec.decimal128
named/distinct wrappers over those exact bases
```

The existing attributes remain:

```text
coefficient
scale
lexeme
```

Verifier must additionally check that a constant is representable in the
semantic coefficient width of the result type when the frontend has already
resolved it as a concrete decimal family.

Do not convert through binary floating point.

---

# 13. Canonical scalar target plan

Introduce or extend the compiler's canonical layout/target model with a
read-only resolved scalar plan.

Recommended Go shape:

```go
type Endianness string

const (
    LittleEndian Endianness = "little"
    BigEndian    Endianness = "big"
)

type ResolvedScalarPlan struct {
    TargetOS         string
    TargetArch       string
    LLVMTriple       string
    ABI              string
    Profile          string
    PointerWidthBits uint16
    Endianness       Endianness
}
```

This structure belongs in the compiler layout/target layer, not in MLIR codegen.

Recommended package:

```text
internal/layout
```

If a canonical `CompilationPlan` implementation already exists after earlier
work, extend that instead of creating a competing model.

The key rule is:

```text
there must be one authoritative resolved scalar-plan source
```

Do not hard-code architecture widths inside the MLIR C++ pass.

---

# 14. Target registry integration

Extend target definitions or their resolved plan construction so target pointer
width is explicit.

Required values for concrete architectures include at least:

```text
amd64       64
arm64       64

armv6       32
armv7       32
cortex-m0   32
cortex-m3   32
cortex-m4   32
cortex-m7   32
riscv32     32
```

Do not derive the width from string suffixes inside lowering code.

Each concrete target registry entry supplies the fact.

Targets such as:

```text
rtems-any
zephyr-any
```

must not receive a guessed pointer width.

If no concrete scalar layout is resolved, target-sensitive scalar lowering must
fail explicitly.

---

# 15. Endianness

The scalar plan records native target endianness.

Package 6 does not perform endian conversion.

Ordinary scalar storage uses native target endianness according to
`rules/memory/layout.md`.

The value is retained so later explicit-layout, FFI, register and LLVM-lowering
stages use the same target plan.

No arithmetic result changes because of endianness.

---

# 16. Sec MLIR target metadata

Compiler-generated schema-v3 modules add:

```text
sec.target_os
sec.target_arch
sec.target_triple
sec.target_abi
sec.target_profile
sec.target_endianness
```

Use normal string attributes.

Emit optional ABI/profile attributes only when the resolved plan provides them.

These attributes identify the compilation plan.

Physical bit-width information used by MLIR must be represented through DLTI,
not duplicated as an independent `sec.target_pointer_width` integer attribute.

---

# 17. DLTI integration

Register/load the upstream `dlti` dialect in `sec-mlir-opt`.

Compiler-generated target-resolved schema-v3 modules must carry an explicit
data-layout spec containing the active pointer-sized/index width:

```mlir
module attributes {
    dlti.dl_spec = #dlti.dl_spec<
        #dlti.dl_entry<index, 64>
    >
}
```

or for a 32-bit plan:

```mlir
module attributes {
    dlti.dl_spec = #dlti.dl_spec<
        #dlti.dl_entry<index, 32>
    >
}
```

The emitter must derive this value from `ResolvedScalarPlan.PointerWidthBits`.

Do not rely on MLIR's default index width.

Package 6 supports only:

```text
32
64
```

as pointer-sized Sec integer widths.

Other widths require a future explicit target-profile decision.

---

# 18. Why `index` data layout is used

MLIR's data-layout model allows an explicit scoped bit width for the builtin
`index` type.

For Package 6, the compiler emits that width from the already-resolved Sec
CompilationPlan and uses the same width as the canonical pointer-sized scalar
width.

Important:

```text
Sec int is not MLIR index.
Sec uint is not MLIR index.
```

The `index` DLTI entry is used as the standard MLIR carrier/query for the
resolved machine-word width.

The lowering result remains an ordinary signed/unsigned fixed-width integer
type.

---

# 19. High-level emitter API change

The high-level Sec MLIR emitter must receive resolved target-plan information
without reading target registry globals itself.

Recommended API:

```go
func Emit(
    module *semantic.Module,
    plan layout.ResolvedScalarPlan,
) ([]byte, error)
```

or equivalent.

Allowed lowering package dependencies become:

```text
standard library
internal/ir/semantic
internal/layout
```

Still forbidden:

```text
AST
lexer
parser
Sema
legacy codegen
```

The CLI/compiler layer resolves the target plan and passes it in.

---

# 20. Target-plan validation before emission

High-level emission must reject:

```text
pointer width 0
pointer width other than 32 or 64
unknown endianness
empty target OS
empty target architecture
```

when target-resolved MLIR is requested.

Do not silently substitute host properties.

Cross-compilation must use the selected target, not the machine running the
compiler.

---

# 21. Package 5 mandatory bool-storage correction

Update `rules/mlir/sec_mlir_lowering.md` and the Package 5 implementation.

Remove `i1` from:

```text
Package 5 lowerable storage element types
```

After correction:

```text
sec.const.bool -> arith.constant i1
```

still occurs in Package 5.

But:

```text
!sec.storage<i1>
sec.storage.declare/init/load/store
```

remain high-level until Package 6.

Required regression:

```text
--sec-lower-trivial-core
```

must not produce:

```text
memref<i1>
```

for semantic bool storage.

---

# 22. New pass

Add:

```bash
--sec-resolve-scalar-layout
```

Recommended pass name:

```text
SecResolveScalarLayout
```

It is a module pass.

It consumes:

```text
verified schema-v3 Sec MLIR
explicit dlti.dl_spec
```

It must fail when the required explicit index-width entry is absent.

Do not accept MLIR's default 64-bit index width as sufficient.

---

# 23. DataLayout query rule

The pass constructs one MLIR `DataLayout` for the module scope and reuses it.

Do not repeatedly construct `DataLayout` for individual queries.

Resolve the active machine-word width from the explicit module data layout.

Required:

```text
32 -> Sec int/uint width 32
64 -> Sec int/uint width 64
```

Any other result is rejected by Package 6.

---

# 24. Scalar type conversion

The Package 6 TypeConverter performs:

```text
!sec.int   -> siN
!sec.uint  -> uiN
!sec.float -> f64
!sec.char  -> ui8
!sec.rune  -> ui32
```

where:

```text
N = explicit CompilationPlan/DLTI machine-word width
```

It does not convert:

```text
!sec.string
!sec.decimal
!sec.decimal128
!sec.never
```

Fixed-width types remain themselves:

```text
si8, si16, si32, si64, si128, si256
ui8, ui16, ui32, ui64, ui128, ui256
f32, f64
i1
```

---

# 25. No signedness normalization in Package 6

Do not convert:

```text
siN -> iN
uiN -> iN
```

in Package 6.

Reason:

Semantic IR requires signedness to remain available for later operations.

Many arithmetic MLIR operations use signless integer types and express
signed/unsigned behavior in the operation itself.

That transformation belongs to the package that introduces/lower arithmetic,
comparisons, shifts, divisions and casts together with their semantic operation
choice.

Package 6 resolves width/layout only.

---

# 26. Named/distinct base conversion

Identity must survive scalar resolution.

Example:

```mlir
!sec.named<"main::Counter", !sec.uint>
```

under a 64-bit plan becomes:

```mlir
!sec.named<"main::Counter", ui64>
```

It does **not** become:

```text
ui64
```

Likewise:

```mlir
!sec.distinct<"main::ID", !sec.int>
```

under a 32-bit plan becomes:

```mlir
!sec.distinct<"main::ID", si32>
```

Do not erase the wrapper.

---

# 27. Function signature conversion

The pass converts target-resolved scalar types in:

```text
func.func input types
func.func result types
entry-block parameters
other block parameters
```

Use MLIR Dialect Conversion signature-conversion support.

Do not reconstruct functions manually when standard conversion infrastructure
can perform the rewrite safely.

Preserve all Sec function metadata.

---

# 28. Standard CFG conversion

Block parameter conversion must preserve:

```text
cf.br arguments
cf.cond_br arguments
func.return operands
```

Normal MLIR verification must succeed after conversion.

Do not introduce `unrealized_conversion_cast` residue in successful normal
Package 6 output.

Use standard branch/signature type conversion support where available in the
selected MLIR release.

---

# 29. Direct-call conversion compatibility

The pass must work whether Package 5 has already lowered:

```text
sec.call.direct -> func.call
```

or whether a schema-v3 direct call is still present in hand-written conversion
tests.

For `func.call`:

```text
convert operand/result types with the function signature
preserve callee symbol
preserve location
```

For `sec.call.direct`:

```text
rebuild the same Sec call operation with converted operand/result types
preserve callee
preserve argument actions
preserve location
```

A later Package 5 pass may then convert it to `func.call`.

Do not perform overload resolution.

---

# 30. Foreign-call conversion compatibility

`sec.call.foreign` remains a Sec operation.

Its operand/result types may be converted according to Package 6 scalar type
rules so they continue to match the converted extern `func.func` signature.

Preserve:

```text
callee
sec.argument_actions
location
extern function metadata
sec.abi
sec.link_name
```

Do not lower to `func.call`.

Do not erase signedness.

---

# 31. Storage type resolution

For high-level semantic storage:

```text
!sec.storage<!sec.int>
```

under a 64-bit plan, convert the element to:

```text
!sec.storage<si64>
```

Likewise:

```text
!sec.storage<!sec.uint>  -> !sec.storage<uiN>
!sec.storage<!sec.float> -> !sec.storage<f64>
!sec.storage<!sec.char>  -> !sec.storage<ui8>
!sec.storage<!sec.rune>  -> !sec.storage<ui32>
```

After type resolution, non-bool storage may become eligible for the corrected
Package 5 SecToCore storage patterns.

Named/distinct element identity still prevents generic storage lowering.

---

# 32. Canonical scalar-core pipeline

Register a named pass pipeline:

```bash
--sec-lower-scalar-core
```

Conceptual order:

```text
sec-lower-trivial-core
sec-resolve-scalar-layout
sec-lower-trivial-core
```

The first Package 5 pass:

```text
lowers bool constants
lowers already-fixed trivial storage except bool
lowers direct calls
```

Package 6 then:

```text
resolves target-sized and semantic scalar types
handles canonical bool storage
converts integer constants whose types are now builtin
```

The second Package 5 pass:

```text
lowers newly eligible non-bool trivial storage
lowers any remaining direct calls
```

The exact implementation may share pattern libraries instead of literally
running the same pass twice, but observable semantics must match this ordering.

---

# 33. Integer constant lowering

After Package 6 type conversion, `sec.const.int` may lower to `arith.constant`
when its result type is exactly a builtin integer type:

```text
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

Also allow signless integer result if one appears in a hand-written valid test,
provided the existing verifier proves representability.

Use MLIR arbitrary-precision integer support.

Do not convert through host `int64`/`uint64`.

Preserve exact bits/value and source location.

---

# 34. Named/distinct integer constants remain Sec operations

Example:

```text
sec.const.int -> !sec.named<"main::ID", ui64>
```

must remain `sec.const.int`.

Reason:

`arith.constant` does not by itself preserve the nominal Sec result identity.

Do not strip the wrapper merely to lower the constant.

---

# 35. Float constant boundary

`sec.const.float` remains a Sec operation in Package 6.

Even though:

```text
!sec.float -> f64
```

the source lexeme-to-binary floating-point rounding policy is not defined by
Package 6.

Do not silently adopt MLIR's internal folding default as the Sec language
literal rule.

A later numeric-lowering package must make that policy explicit.

---

# 36. Decimal boundary

Package 6 knows these canonical physical component shapes from `layout.md`:

```text
decimal:
    coefficient signed 64
    scale signed 32

decimal128:
    coefficient signed 128
    scale signed 32
```

However the high-level types remain:

```text
!sec.decimal
!sec.decimal128
```

because Package 6 does not introduce the lower aggregate representation layer.

Do not lower them to:

```text
tuple
memref
LLVM struct
binary float
```

in this package.

---

# 37. Canonical bool storage lowering

Package 6 handles:

```text
!sec.storage<i1>
```

as a special canonical stored-bool case.

Lower the storage handle to:

```text
memref<i8>
```

rank zero.

`sec.storage.declare` becomes:

```text
memref.alloca() : memref<i8>
```

preserving:

```text
sec.storage_id
sec.source_name
sec.storage_class
sec.mutable
location
```

No `memref.dealloc` is emitted.

---

# 38. Bool initialization/store

For:

```text
i1 semantic bool value
```

stored into canonical bool storage:

```text
zero-extend i1 -> i8
memref.store i8
```

Use the current valid Arith operation for unsigned/zero extension from `i1` to
`i8`.

Canonical stored values therefore become:

```text
0x00
0x01
```

No other value is produced by safe Package 6 stores.

Preserve source locations where practical.

---

# 39. Bool load

For canonical bool storage:

```text
memref.load -> i8
truncate i8 -> i1
```

The storage representation is already guaranteed canonical by safe upstream
initialization/store paths.

Package 6 does not add an invalid-representation runtime check.

Unsafe/raw representation validation belongs to the future validity/check
lowering package.

---

# 40. P5 legacy bool-output handling

The corrected normal pipeline must never create:

```text
memref<i1>
```

for addressable Sec bool storage.

If an existing pre-correction Package 5 artifact containing a provenance-marked
bool `memref<i1>` is supplied directly to Package 6, the pass should reject it
with a clear diagnostic rather than treating it as canonical.

Recommended diagnostic:

```text
non-canonical Sec bool storage uses memref<i1>; rerun corrected
sec-lower-trivial-core from schema-v3 high-level Sec MLIR
```

Do not attempt heuristic migration of arbitrary `memref<i1>` because not every
such memref is necessarily Sec bool storage.

---

# 41. DLTI validation tests

Required:

```text
explicit index width 32 accepted
explicit index width 64 accepted
missing explicit index width rejected by Package 6 pass
index width 16 rejected
index width 128 rejected
```

Even though upstream MLIR has a default index width, the Sec pass requires an
explicit CompilationPlan-derived entry.

Cross compilation must be deterministic and independent of host width.

---

# 42. 32-bit target behavior

With:

```mlir
#dlti.dl_entry<index, 32>
```

Package 6 must resolve:

```text
!sec.int  -> si32
!sec.uint -> ui32
```

Required coverage includes at least a concrete 32-bit target such as:

```text
linux-armv7
```

and a bare-metal 32-bit target when target registry tests already support it.

---

# 43. 64-bit target behavior

With:

```mlir
#dlti.dl_entry<index, 64>
```

Package 6 must resolve:

```text
!sec.int  -> si64
!sec.uint -> ui64
```

Required target cases:

```text
linux-amd64
linux-arm64
```

where supported by the target registry.

---

# 44. Target-independent source, target-dependent MLIR

The same Semantic IR module may produce different target-resolved MLIR for two
CompilationPlans.

Example source semantic type:

```text
int
```

32-bit plan:

```text
si32
```

64-bit plan:

```text
si64
```

This difference is expected.

The Semantic IR canonical type remains target-sized and must not be mutated
globally when one output plan is resolved.

Multi-output builds resolve each plan independently.

No layout cache may leak from one target plan to another.

---

# 45. Target metadata tests

Generated schema-v3 high-level MLIR must preserve:

```text
sec.target_os
sec.target_arch
sec.target_triple
sec.target_endianness
```

and when known:

```text
sec.target_abi
sec.target_profile
```

Two outputs for different targets must contain distinct metadata and distinct
DLTI specs where their pointer widths differ.

---

# 46. Required MLIR files

Extend the existing MLIR project.

Recommended additions:

```text
mlir/test/Dialect/Sec/
    decimal128-roundtrip.mlir
    wide-integer-constants.mlir
    schema-v2-regression.mlir

mlir/test/Conversion/SecToCore/
    bool-storage-not-p5.mlir

mlir/test/Conversion/SecScalar/
    target32.mlir
    target64.mlir
    missing-layout.mlir
    invalid-layout-width.mlir
    semantic-types.mlir
    named-base.mlir
    wide-integers.mlir
    integer-constants.mlir
    bool-storage.mlir
    calls.mlir
    foreign-call-remains.mlir
    idempotent.mlir
    no-unrealized-casts.mlir
```

Recommended conversion library:

```text
mlir/include/sec/Conversion/SecScalar/
mlir/lib/Conversion/SecScalar/
```

Do not overload `SecToCore.cpp` with all target-layout logic.

---

# 47. Pass implementation requirements

Use MLIR Dialect Conversion.

Required concepts:

```text
ConversionTarget
TypeConverter
RewritePatternSet
function signature conversion
branch/block argument conversion
operation conversion patterns
applyPartialConversion or applyFullConversion only over the explicitly
selected scalar-resolution legality domain
```

Do not implement function/block type mutation with arbitrary in-place edits that
bypass conversion legality.

The pass must preserve verification throughout representative tests.

---

# 48. Legality after scalar resolution

After successful:

```text
--sec-resolve-scalar-layout
```

these types are illegal anywhere in the converted module:

```text
!sec.int
!sec.uint
!sec.float
!sec.char
!sec.rune
```

These remain legal:

```text
!sec.string
!sec.decimal
!sec.decimal128
!sec.never
!sec.named<..., converted-base>
!sec.distinct<..., converted-base>
```

`!sec.storage<T>` remains legal when its element has unresolved Sec identity or
aggregate semantics.

---

# 49. Operation legality

After Package 6 pass:

```text
sec.const.int
```

is illegal when its result is a plain builtin integer type and must have lowered
to `arith.constant`.

It remains legal when its result retains named/distinct identity.

`sec.const.float` remains legal.

`sec.const.decimal` remains legal.

`sec.const.string` remains legal.

`sec.call.foreign` remains legal.

Storage ops remain legal only where their semantic storage type is not part of
the Package 6/P5 lowerable scalar-storage subset.

---

# 50. Idempotence

Running:

```text
sec-resolve-scalar-layout
```

twice must not alter the result after the first successful conversion.

After one run, none of:

```text
!sec.int
!sec.uint
!sec.float
!sec.char
!sec.rune
```

remain.

A second run sees only already-resolved types.

The named pipeline:

```text
sec-lower-scalar-core
```

must also be idempotent.

---

# 51. No host-dependent behavior

Tests must prove that target width does not depend on the host executing
`sec-mlir-opt`.

The only source of pointer-sized scalar width is the explicit target-resolved
DLTI data attached to the module.

Do not use:

```text
sizeof(void*)
host architecture macros
LLVM host triple
process architecture
```

to resolve Sec `int`/`uint`.

---

# 52. Required Semantic IR tests

## S01

`int128`, `int256`, `uint128`, `uint256`, `decimal128` intern independently.

## S02

Wide integer exact constants survive builder/printer with no truncation.

## S03

A source `int128` return has Semantic IR bit width 128 and signed classification.

## S04

A source `uint256` parameter has Semantic IR bit width 256 and unsigned
classification.

## S05

`decimal128` remains distinct from `decimal`.

## S06

Package 3 mutable semantic storage accepts a valid wide integer scalar.

## S07

Package 3 direct-call signature accepts a valid wide integer scalar.

---

# 53. Required dialect tests

## D01

`!sec.decimal128` round-trips.

## D02

`!sec.decimal128` differs from `!sec.decimal`.

## D03

`sec.const.int` accepts maximum/minimum `si128`.

## D04

`sec.const.int` rejects one-past-range `si128`.

## D05

`sec.const.int` accepts maximum `ui256`.

## D06

`sec.const.int` rejects negative `ui256`.

## D07

`sec.const.decimal` may return `!sec.decimal128`.

## D08

Schema-v1 and schema-v2 regression files remain parseable.

---

# 54. Required Package 5 correction tests

## P5C01

`sec.const.bool` still lowers to `arith.constant i1`.

## P5C02

`!sec.storage<i1>` does not lower in `--sec-lower-trivial-core`.

## P5C03

No `memref<i1>` is generated for semantic bool storage.

## P5C04

Existing non-bool P5 trivial storage still lowers normally.

---

# 55. Required target-plan tests

## T01

Target registry resolves `linux-amd64` pointer width 64.

## T02

Target registry resolves `linux-arm64` pointer width 64.

## T03

Target registry resolves `linux-armv7` pointer width 32.

## T04

A concrete Cortex-M target resolves pointer width 32.

## T05

An `arch:any` target does not guess pointer width.

## T06

Selected target plan is independent from host architecture.

## T07

32-bit and 64-bit plans for equivalent Semantic IR do not share resolved scalar
layout cache entries.

---

# 56. Required emitter tests

## E01

Schema-v3 module marker emitted.

## E02

`dlti.dl_spec` contains explicit index width.

## E03

32-bit plan emits index width 32.

## E04

64-bit plan emits index width 64.

## E05

Target identity metadata is emitted.

## E06

`int128 -> si128`.

## E07

`uint256 -> ui256`.

## E08

`decimal128 -> !sec.decimal128`.

## E09

Emitter rejects unresolved/invalid target scalar plan.

## E10

Lowering package still imports no AST/Sema.

---

# 57. Required scalar conversion tests

## C01 - 32-bit int

Input:

```text
!sec.int
```

with explicit 32-bit DLTI.

Expected:

```text
si32
```

## C02 - 64-bit int

Expected:

```text
si64
```

## C03 - uint

32-bit:

```text
ui32
```

64-bit:

```text
ui64
```

## C04 - float

```text
!sec.float -> f64
```

## C05 - char

```text
!sec.char -> ui8
```

## C06 - rune

```text
!sec.rune -> ui32
```

## C07 - fixed wide integers

```text
si128/si256/ui128/ui256
```

remain exact and are not narrowed.

## C08 - named base

```text
!sec.named<"main::Count", !sec.int>
```

under 64-bit target becomes:

```text
!sec.named<"main::Count", si64>
```

## C09 - function signature

Parameters/results convert consistently.

## C10 - block arguments

Converted block parameter types match branch operands.

## C11 - direct call

Call operands/results remain consistent with converted target signature.

## C12 - foreign call

Operation remains `sec.call.foreign` with converted scalar signature.

## C13 - plain integer constant

Resolved plain builtin integer constant becomes `arith.constant`.

## C14 - named integer constant

Remains `sec.const.int`.

## C15 - float constant

Remains `sec.const.float`.

## C16 - decimal

Remains `!sec.decimal`.

## C17 - decimal128

Remains `!sec.decimal128`.

---

# 58. Required bool-storage tests

## B01

High-level:

```text
!sec.storage<i1>
```

lowers in Package 6 to:

```text
memref<i8>
```

not `memref<i1>`.

## B02

Initialization inserts `i1 -> i8` zero extension before store.

## B03

Subsequent store inserts the same canonical conversion.

## B04

Load produces `i8`, then converts to semantic `i1`.

## B05

Stored true value is representable as byte value `1`.

## B06

Stored false value is representable as byte value `0`.

## B07

Storage provenance attributes survive on `memref.alloca`.

## B08

A pre-correction provenance-marked `memref<i1>` input is rejected clearly.

---

# 59. Required integration pipeline tests

Generate from Sec source:

```text
Sec source
    ↓
Sema
    ↓
Semantic IR v1
    ↓
Sec MLIR schema v3 + explicit target/DLTI
    ↓
sec-lower-scalar-core
    ↓
verified mixed lower MLIR
```

Required representative source cases:

```text
int return on 32-bit plan
int return on 64-bit plan
uint return on 32/64-bit plans
float parameter/return
char parameter/return
rune parameter/return
int128 return
uint256 parameter
decimal128 constant/return remaining high-level
mutable bool local
mutable int local on 32-bit plan
mutable int local on 64-bit plan
direct call using int
foreign call using fixed-width integer
if/else using mutable bool/int storage
```

No hand-editing of generated MLIR.

---

# 60. Real MLIR verification

Run:

```bash
cmake --build build/sec-mlir
cmake --build build/sec-mlir --target check-sec-mlir
```

Representative:

```bash
sec-mlir-opt input.mlir \
    --sec-lower-scalar-core \
    --verify-each \
    -o output.mlir
```

Use exact supported pass-pipeline syntax for the installed MLIR version.

Do not use:

```text
--allow-unregistered-dialect
```

---

# 61. Go regression

Run:

```bash
go test ./...
```

No ordinary Go unit test may require a real MLIR installation.

Target-plan tests are pure Go.

External MLIR integration tests remain separate.

---

# 62. Legacy backend policy

Package 6 must not migrate:

```text
emit-mlir
emit-llvm
build
```

to the new pipeline.

Legacy backend behavior remains regression-tested.

However any shared target/layout model added in Package 6 should be written so
legacy paths can later consume it rather than maintaining competing pointer-size
rules indefinitely.

Do not perform a large legacy migration in this package.

---

# 63. Error policy

## Missing target plan

Example:

```text
Sec scalar lowering requires an explicit resolved CompilationPlan
```

## Missing DLTI index width

Example:

```text
Sec scalar lowering requires explicit dlti index width from CompilationPlan
```

## Unsupported pointer width

Example:

```text
Sec scalar lowering supports pointer-sized int/uint widths 32 and 64; got 16
```

## Invalid pre-correction bool storage

Example:

```text
non-canonical Sec bool storage uses memref<i1>
```

These are compiler/lowering configuration errors, not source-language errors.

---

# 64. Architecture rules

Non-negotiable:

```text
Active Sec scalar types must not disappear from Semantic IR coverage.

int128/int256/uint128/uint256 remain exact widths.

decimal128 remains distinct from decimal.

int and uint are pointer-sized for the active CompilationPlan.

The target width is supplied by the compiler target/layout layer.

The MLIR pass does not infer pointer width from target-name strings.

The MLIR pass does not use host pointer width.

DLTI index width is explicit; MLIR's default 64-bit index behavior is not used
as a Sec target decision.

Sec int is not MLIR index.

float resolves to f64 because the Sec layout rule defines that relationship.

char resolves to one-byte ui8.

rune resolves to ui32.

Addressable bool is one byte even when semantic SSA bool is i1.

No canonical Sec bool storage uses memref<i1>.

Signedness is not erased in Package 6.

Named/distinct identity is not erased.

Decimal types remain high-level.

Foreign calls remain explicit.

No LLVM dialect is generated.
```

---

# 65. Acceptance criteria

Package 6 is complete only when:

```text
[ ] Packages 1-5 regressions are green after corrections
[ ] rules/mlir/sec_mlir_dialect.md updated to schema v3
[ ] rules/mlir/sec_mlir_lowering.md updated to lowering spec v2
[ ] Semantic IR v1 includes int128/int256/uint128/uint256/decimal128
[ ] Package 2 type interning covers all active scalar types
[ ] Package 3 supports wide fixed integers in trivial storage/calls
[ ] !sec.decimal128 exists
[ ] Package 4 emitter maps int128/int256/uint128/uint256
[ ] Package 4 emitter maps decimal128
[ ] sec.const.int verifier supports 128/256-bit boundaries
[ ] sec.const.decimal supports decimal128
[ ] canonical resolved scalar target plan exists
[ ] target registry exposes pointer width centrally
[ ] concrete 32-bit/64-bit target widths are explicit
[ ] any-architecture targets do not guess width
[ ] high-level Sec MLIR emits target identity
[ ] high-level Sec MLIR emits explicit dlti.dl_spec index width
[ ] missing DLTI width is rejected by scalar-resolution pass
[ ] MLIR default 64-bit index width is never used as implicit Sec target data
[ ] --sec-resolve-scalar-layout registered
[ ] !sec.int resolves to si32/si64 by plan
[ ] !sec.uint resolves to ui32/ui64 by plan
[ ] !sec.float resolves to f64
[ ] !sec.char resolves to ui8
[ ] !sec.rune resolves to ui32
[ ] named/distinct wrappers preserve identity while base resolves
[ ] siN/uiN signedness remains explicit
[ ] plain builtin integer sec.const.int lowers exactly
[ ] named/distinct integer constants remain Sec
[ ] float constants remain Sec
[ ] decimal/decimal128 remain Sec
[ ] P5 no longer lowers bool storage to memref<i1>
[ ] Package 6 lowers bool storage to memref<i8>
[ ] bool init/store canonicalizes i1 to byte 0/1
[ ] bool load returns semantic i1
[ ] --sec-lower-scalar-core registered
[ ] scalar-core pipeline is idempotent
[ ] no normal unrealized_conversion_cast residue
[ ] 32-bit and 64-bit integration tests pass
[ ] wide integer integration tests pass
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] no LLVM dialect operations are generated
[ ] legacy compiler paths remain operational
```

---

# 66. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. Packages 1-5 pre-status
3. P5 bool-storage correction performed
4. files added
5. files modified
6. Semantic IR scalar types added
7. schema-v3 dialect changes
8. lowering-spec-v2 changes
9. target-plan structure/API
10. target registry fields added
11. concrete target pointer-width table implemented
12. DLTI emission strategy
13. scalar-resolution pass name
14. scalar TypeConverter rules
15. function/block/call signature conversion approach
16. wide integer constant implementation
17. bool storage implementation
18. proof that memref<i1> is not canonical bool storage
19. 32-bit target test results
20. 64-bit target test results
21. wide integer test results
22. decimal128 test results
23. CMake commands
24. exact LLVM/MLIR version
25. check-sec-mlir result
26. go test ./... result
27. end-to-end source -> scalar-core MLIR results
28. deviations
29. issues recommended for Package 7
```

---

# 67. Package 7 boundary

Package 7 should introduce the first **numeric operation lowering** layer.

Recommended scope:

```text
Semantic IR arithmetic operation family
Semantic IR comparisons
Semantic IR bitwise operations
Semantic IR shifts
Semantic IR numeric conversions

high-level Sec MLIR numeric operations

signed/unsigned operation semantics

controlled normalization:
    siN/uiN -> signless iN only when operation semantics are retained by the
    selected arith operation/predicate

integer overflow policy integration
division-by-zero policy integration
shift validation
integer cast extension/truncation choice

float literal rounding rule
sec.const.float -> arith.constant after that rule is fixed

arith lowering tests for:
    8/16/32/64/128/256
    signed
    unsigned
    pointer-sized int/uint after Package 6 resolution
```

Package 7 should still defer:

```text
decimal arithmetic lowering
string
foreign ABI
ownership
borrow/reference
Result/try
cleanup
aggregates
allocation
register/MMIO
concurrency
LLVM dialect
```

The next invariant should be:

```text
plan-resolved scalar MLIR
    ↓
semantic numeric operations choose exact signed/unsigned MLIR operations
    ↓
signless lower integer representation may be introduced without losing
language signedness semantics.
```

# Sec MLIR Program - Implementation Package 4

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P4`  
Package title: `First Semantic IR to Sec MLIR Bridge`  
Repository: `https://github.com/YoePro/sec`  
Repository branch: `main`  
Repository sync commit used for this package: `d48035c`  
Repository sync date: `2026-08-08`  
Required Semantic IR version: `1`  
Sec MLIR dialect schema introduced by this package: `2`

Package 4 is the first package that connects the canonical Semantic IR to the
Sec MLIR layer.

It must consume only verified Semantic IR.

It must not read AST, lexer, parser or Sema data.

It must not lower Sec MLIR further toward LLVM.

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
implementation package
    ↓
implementation
```

Before implementing the new dialect constructs in this package, replace or
update:

```text
rules/mlir/sec_mlir_dialect.md
```

with the supplied normative file:

```text
sec_mlir_dialect_package4.md
```

The supplied rulebook defines Sec MLIR dialect schema version 2.

Do not add dialect constructs not present in that rulebook.

---

# 2. Preconditions

Package 4 assumes Packages 1, 2 and 3 are complete and green.

Package 1 must provide:

```text
mlir/ out-of-tree project
Sec dialect registration
TableGen/ODS
sec-mlir-opt
!sec.named
!sec.distinct
dialect tests
Go Toolchain.VerifySec or equivalent
```

Package 2 must provide:

```text
canonical Semantic IR
Semantic IR version 1
canonical type table
functions
values
blocks
constants
return
source locations
verifier
deterministic printer
sec emit-ir
```

Package 3 must provide:

```text
StorageID
storage.declare
storage.init
storage.load
storage.store
DirectCall
ForeignCall
branch
conditional branch
block parameters
SSA dominance verification
storage dominance verification
resolved call identity
```

If the implemented names differ while preserving the Package 1-3 semantic
contracts, adapt to the actual implementation.

Do not re-create earlier layers.

---

# 3. Package goal

After Package 4:

1. Sec MLIR dialect schema version 2 exists;
2. Package 2/3 scalar Semantic IR types have a defined Sec MLIR mapping;
3. target-sized and otherwise semantic Sec scalar types are not prematurely
   forced into LLVM representation;
4. Semantic IR constants lower to Sec MLIR without loss of exact value data;
5. semantic local storage lowers to explicit Sec storage operations;
6. direct and foreign calls remain distinct in Sec MLIR;
7. Semantic IR branch CFG lowers to standard MLIR `cf` control flow;
8. Semantic IR function return lowers to `func.return`;
9. Semantic IR function identity and source provenance are preserved;
10. a Go lowering package consumes `*semantic.Module` and emits textual Sec MLIR;
11. that lowering package imports no AST, lexer or Sema packages;
12. generated Sec MLIR is accepted by `sec-mlir-opt`;
13. `sec emit-sec-mlir <file.sec>` exists as an explicit non-legacy dump command;
14. legacy `emit-mlir` and LLVM code generation remain unchanged;
15. no lowering to LLVM dialect occurs in this package.

The central invariant is:

```text
verified Semantic IR v1
    ↓
Semantic IR -> Sec MLIR lowering
    ↓
verified Sec MLIR dialect schema v2
```

without consulting any earlier compiler representation.

---

# 4. Package boundary

## 4.1 In scope

Implement:

```text
Sec MLIR dialect schema version 2
new semantic scalar Sec MLIR types
Sec storage handle type
Sec constant operations
Sec semantic storage operations
Sec direct-call operation
Sec foreign-call operation
dialect verifiers for all new constructs
standard func.func representation for functions
standard func.return representation for returns
standard cf.br representation for branches
standard cf.cond_br representation for conditional branches
standard MLIR block arguments
module/function semantic metadata
source-location mapping
Go Semantic IR -> Sec MLIR textual lowering
deterministic symbol mapping
deterministic block/value textual emission
sec-mlir-opt verification
sec emit-sec-mlir command
TableGen/ODS tests
Go lowering tests
CLI tests with fake toolchain where appropriate
end-to-end integration tests against real sec-mlir-opt
full Go regression
Package 1 MLIR regression
```

## 4.2 Explicitly out of scope

Do not implement:

```text
Sec MLIR -> standard scalar lowering
Sec MLIR -> LLVM dialect
LLVM IR generation from Sec MLIR
replacement of legacy compiler backends

copy
move
destroy
cleanup
defer
borrow
reference
RawPtr operations
ownership-sensitive calls

Result
Option
Ok
Err
try

runtime contract checks
bounds checks
overflow checks
panic

struct
array
slice
enum
union
property
interface
closure

allocation
arena
heap placement

register/MMIO
volatile
atomic

loops
switch
match
break
continue
unless already represented solely by Package 3 branch CFG

concurrency
spawn
await

ABI lowering
calling-convention lowering
linking
object emission

Semantic IR serialization
Semantic IR parser

AST -> Sec MLIR
Sema -> Sec MLIR
```

Do not add placeholder operations for later packages.

---

# 5. Dialect schema version

Package 4 updates:

```text
sec.dialect_version
```

for compiler-generated Sec MLIR modules to:

```mlir
sec.dialect_version = 2 : i32
```

Also emit:

```mlir
sec.semantic_ir_version = 1 : i32
```

Schema version 1 remains parseable for Package 1 tests.

No migration pass is required in Package 4.

The Semantic IR -> Sec MLIR emitter always emits dialect schema version `2`.

---

# 6. Standard MLIR dialects reused

Package 4 must reuse standard MLIR where the standard construct preserves all
remaining Sec semantics.

Use:

```text
builtin.module
func.func
func.return
cf.br
cf.cond_br
MLIR block arguments
MLIR Location
builtin integer types
builtin floating-point types
```

Do not create:

```text
sec.func
sec.return
sec.br
sec.cond_br
```

in Package 4.

Reason:

the Package 2/3 subset has no remaining Sec-specific cleanup, ownership or
failure behavior attached to those constructs.

Later packages may add higher-level Sec operations before lowering to these
standard operations when additional semantics require it.

---

# 7. New Sec MLIR types

Dialect schema version 2 adds exactly these types:

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
```

Package 1 types remain:

```text
!sec.named<"identity", base-type>
!sec.distinct<"identity", base-type>
```

## 7.1 `!sec.int`

Represents Sec target-sized signed `int`.

It must not lower to `index`.

It must not be assigned a machine width in Package 4.

## 7.2 `!sec.uint`

Represents Sec target-sized unsigned `uint`.

It must not lower to `index`.

It must not be assigned a machine width in Package 4.

## 7.3 `!sec.float`

Represents the Sec source-level `float` type when it remains distinct from the
explicit `float32` and `float64` forms.

Package 4 does not choose its final backend representation.

## 7.4 `!sec.char`

Represents Sec `char`.

Do not collapse it into `i32` merely because a later representation may use an
integer.

## 7.5 `!sec.rune`

Represents Sec `rune`.

`char` and `rune` remain distinct types.

## 7.6 `!sec.string`

Represents a Sec string value at high-level Sec MLIR.

It does not define:

```text
pointer representation
length representation
ownership
allocation
ABI
```

Package 4 supports string constants but not mutable string storage or by-value
string calls from Package 3.

## 7.7 `!sec.decimal`

Represents Sec exact decimal semantics.

It must not be lowered to binary floating point.

It must not commit to the current legacy LLVM `{ i64, i8 }` representation.

That representation is a lower-layer decision.

## 7.8 `!sec.never`

Represents Sec `never` when a type position requires it.

Package 4 does not add termination operations needed for general `never`
function bodies.

## 7.9 `!sec.storage<T>`

Represents one semantic storage identity containing values of `T`.

It is:

```text
not a pointer
not a reference
not a memref
not a stack slot
not a heap allocation
```

It exists solely to preserve the Package 3 semantic storage abstraction until a
later storage-lowering pass chooses representation.

Verifier:

```text
T must be a valid non-void MLIR type.
nested !sec.storage<!sec.storage<T>> is invalid.
```

---

# 8. Semantic IR type mapping

Package 4 uses this exact mapping.

| Semantic IR type | Sec MLIR type |
| --- | --- |
| `void` | no MLIR result type |
| `never` | `!sec.never` |
| `bool` | `i1` |
| `byte` | `ui8` |
| `char` | `!sec.char` |
| `rune` | `!sec.rune` |
| `string` | `!sec.string` |
| `decimal` | `!sec.decimal` |
| `int` | `!sec.int` |
| `int8` | `si8` |
| `int16` | `si16` |
| `int32` | `si32` |
| `int64` | `si64` |
| `uint` | `!sec.uint` |
| `uint8` | `ui8` |
| `uint16` | `ui16` |
| `uint32` | `ui32` |
| `uint64` | `ui64` |
| `float` | `!sec.float` |
| `float32` | `f32` |
| `float64` | `f64` |

A named Semantic IR scalar lowers to:

```mlir
!sec.named<"module::Name", mapped-base-type>
```

Example:

```text
Semantic IR:
    type Speed = named main::Speed base int

Sec MLIR:
    !sec.named<"main::Speed", !sec.int>
```

A distinct Semantic IR type, when later available, lowers to:

```mlir
!sec.distinct<"identity", mapped-base-type>
```

Package 4 must not manufacture distinct types because Package 2 intentionally
does not yet emit them.

---

# 9. New Sec MLIR constant operations

Schema version 2 adds:

```text
sec.const.int
sec.const.bool
sec.const.float
sec.const.decimal
sec.const.string
```

The operations preserve Semantic IR constant information instead of choosing a
backend representation.

All result-producing constant operations have one result.

All carry normal MLIR `Location`.

## 9.1 `sec.const.int`

Canonical conceptual form:

```mlir
%0 = "sec.const.int"() {
    value = "42"
} : () -> !sec.int
```

Custom assembly syntax may be defined if concise and unambiguous.

Required attribute:

```text
value: StringAttr
```

`value` contains canonical signed base-10 integer text.

The verifier must parse the string as an arbitrary-precision integer.

Allowed result semantic base categories:

```text
!sec.int
!sec.uint
!sec.char
!sec.rune
si8/si16/si32/si64
ui8/ui16/ui32/ui64
!sec.named<..., allowed-integer-base>
!sec.distinct<..., allowed-integer-base>
```

Verifier requirements:

- string is a valid integer;
- unsigned result cannot contain a negative value;
- fixed-width builtin integer results must be representable in that width;
- named/distinct verification applies recursively to the base type.

Do not truncate.

## 9.2 `sec.const.bool`

Required attribute:

```text
value: BoolAttr
```

Allowed result:

```text
i1
named/distinct type whose base is i1
```

## 9.3 `sec.const.float`

Required attribute:

```text
lexeme: StringAttr
```

The original exact source numeric spelling or equivalent Semantic IR-preserved
lexeme is retained.

Allowed result base:

```text
!sec.float
f32
f64
named/distinct over those types
```

Package 4 verifier validates syntactic floating-point form where practical but
does not perform final backend rounding.

## 9.4 `sec.const.decimal`

Required attributes:

```text
coefficient: StringAttr
scale: IntegerAttr(i32)
lexeme: StringAttr
```

Example:

```text
0.10
coefficient = "10"
scale = 2
lexeme = "0.10"
```

Allowed result base:

```text
!sec.decimal
named/distinct over !sec.decimal
```

Verifier:

```text
coefficient parses as arbitrary-precision signed integer
scale >= 0
lexeme is non-empty
```

The verifier must not require normalized scale.

## 9.5 `sec.const.string`

Required attribute:

```text
value: StringAttr
```

Allowed result base:

```text
!sec.string
named/distinct over !sec.string
```

The attribute stores the decoded semantic string.

---

# 10. New Sec storage operations

Schema version 2 adds:

```text
sec.storage.declare
sec.storage.init
sec.storage.load
sec.storage.store
```

These mirror Package 3 semantic storage.

They must not be implemented with `memref.alloca` in Package 4.

## 10.1 `sec.storage.declare`

Conceptual form:

```mlir
%slot = "sec.storage.declare"() {
    sec.storage_id = 1 : i32,
    sec.source_name = "value",
    sec.storage_class = "local-automatic",
    sec.mutable = true
} : () -> !sec.storage<!sec.int>
```

Required attributes:

```text
sec.storage_id      IntegerAttr(i32), > 0
sec.source_name     StringAttr
sec.storage_class   StringAttr
sec.mutable         BoolAttr
```

Package 4 importer emits:

```text
sec.storage_class = "local-automatic"
sec.mutable = true
```

for Package 3 mutable locals.

Verifier rejects unknown storage class values in schema version 2.

## 10.2 `sec.storage.init`

Conceptual:

```mlir
"sec.storage.init"(%slot, %value)
    : (!sec.storage<!sec.int>, !sec.int) -> ()
```

Verifier:

```text
first operand is !sec.storage<T>
second operand type is exactly T
```

The operation itself does not perform copy/move/destruction.

Schema version 2 is restricted to Package 3 trivial storage semantics.

## 10.3 `sec.storage.load`

Conceptual:

```mlir
%value = "sec.storage.load"(%slot)
    : (!sec.storage<!sec.int>) -> !sec.int
```

Verifier:

```text
input is !sec.storage<T>
result type is exactly T
```

## 10.4 `sec.storage.store`

Conceptual:

```mlir
"sec.storage.store"(%slot, %value)
    : (!sec.storage<!sec.int>, !sec.int) -> ()
```

Verifier:

```text
input 0 is !sec.storage<T>
input 1 type is exactly T
```

Schema version 2 does not support non-trivial replacement semantics.

---

# 11. New call operations

Schema version 2 adds:

```text
sec.call.direct
sec.call.foreign
```

These remain distinct even though both eventually lower to ordinary calls.

Each call stores:

```text
callee: FlatSymbolRefAttr
sec.argument_actions: ArrayAttr<StringAttr>
```

Package 4 supports only:

```text
"copy-trivial"
```

as an argument action.

The number of argument actions must equal the number of call operands.

## 11.1 `sec.call.direct`

Conceptual:

```mlir
%result = "sec.call.direct"(%arg) {
    callee = @fn_0,
    sec.argument_actions = ["copy-trivial"]
} : (!sec.int) -> !sec.int
```

Verifier must:

1. resolve `callee`;
2. require target is `func.func`;
3. require target does not have `sec.extern = true`;
4. require argument count and types match target function type;
5. require result count and types match target function type;
6. require all schema-v2 argument actions are `copy-trivial`.

## 11.2 `sec.call.foreign`

Conceptual:

```mlir
%result = "sec.call.foreign"() {
    callee = @fn_2,
    sec.argument_actions = []
} : () -> si64
```

Verifier must:

1. resolve `callee`;
2. require target is `func.func`;
3. require target has `sec.extern = true`;
4. require argument/result signature match;
5. require all schema-v2 argument actions are `copy-trivial`.

Calling convention and link-name information belong to the target function
metadata, not the call operation.

---

# 12. Module metadata

Every compiler-generated Package 4 module must contain:

```mlir
sec.dialect_version = 2 : i32
sec.semantic_ir_version = 1 : i32
sec.module_id = "..."
sec.source_files = [...]
```

Required types:

```text
sec.dialect_version     IntegerAttr(i32)
sec.semantic_ir_version IntegerAttr(i32)
sec.module_id           StringAttr
sec.source_files        ArrayAttr<StringAttr>
```

Source files must be deterministic and contain no duplicates.

The emitter must preserve Semantic IR source-file order if canonical; otherwise
sort them deterministically.

---

# 13. Function representation

Semantic IR functions lower to standard:

```text
func.func
```

Do not create `sec.func`.

Each generated `func.func` must carry:

```text
sec.function_id
sec.source_name
sec.extern
sec.unsafe
```

Optional when available:

```text
sec.link_name
sec.abi
sec.parameter_names
```

Required attribute types:

```text
sec.function_id    StringAttr
sec.source_name    StringAttr
sec.extern         BoolAttr
sec.unsafe         BoolAttr
sec.link_name      StringAttr
sec.abi            StringAttr
sec.parameter_names ArrayAttr<StringAttr>
```

Function source location becomes the `func.func` MLIR location.

Extern functions:

```text
have no body;
must have sec.extern = true;
retain sec.abi when known;
retain sec.link_name when known.
```

Non-extern functions:

```text
have a body;
must have sec.extern = false.
```

---

# 14. Deterministic MLIR symbol mapping

Semantic `FunctionID` may contain characters unsuitable or inconvenient for a
canonical MLIR symbol spelling.

Package 4 must not invent an ABI mangling scheme.

Instead perform a deterministic lowering-local mapping.

Required scheme:

```text
functions in deterministic Semantic IR function order:
    first  -> @sec_fn_0
    second -> @sec_fn_1
    third  -> @sec_fn_2
```

The exact Semantic IR FunctionID is retained in:

```text
sec.function_id
```

All calls use the mapped MLIR symbol.

Requirements:

```text
same Semantic IR module -> same MLIR symbol mapping
no map iteration order
no hash-dependent symbol names
no FunctionID parsing to infer semantics
```

Later ABI/link lowering may choose external symbol names independently.

---

# 15. Parameter and SSA mapping

Semantic IR `ValueID` is a lowering-local identity.

It does not need to survive as an MLIR attribute.

The emitter maintains:

```text
Semantic ValueID -> emitted MLIR SSA value spelling
```

Function parameters are emitted in Semantic IR parameter order.

Recommended textual names:

```text
%v0
%v1
%v2
```

Operation results continue monotonically.

Do not depend on source variable names for SSA identity.

Do not emit redundant `sec.value_id` attributes.

Reason:

MLIR SSA already owns value identity at this layer.

---

# 16. Storage mapping

Semantic IR `StorageID` lowers to the SSA result of one
`sec.storage.declare`.

The emitter maintains:

```text
Semantic StorageID -> MLIR storage SSA value
```

The Semantic StorageID is additionally retained as:

```text
sec.storage_id
```

on the declare operation for diagnostics/debugging.

There must be exactly one `sec.storage.declare` per Semantic StorageID.

Do not materialize physical storage.

---

# 17. Block and CFG mapping

Use standard MLIR blocks.

Semantic entry block is emitted first.

Recommended textual labels:

```text
^bb0
^bb1
^bb2
```

Use deterministic Semantic IR block order.

Block parameters lower to MLIR block arguments with mapped types.

Semantic:

```text
branch
```

lowers to:

```text
cf.br
```

Semantic:

```text
conditional branch
```

lowers to:

```text
cf.cond_br
```

Semantic:

```text
return
```

lowers to:

```text
func.return
```

No implicit fallthrough may be introduced.

The emitter must preserve successor and branch-argument ordering exactly.

---

# 18. Source locations

Map:

```go
semantic.Location{
    File,
    Line,
    Column,
}
```

to:

```mlir
loc("file":line:column)
```

Unknown/synthesized location maps to:

```mlir
loc(unknown)
```

Escape file names using valid MLIR string escaping.

Do not consult lexer tokens.

Do not reconstruct locations from AST.

Every generated operation for which Semantic IR carries a location must use that
location.

Compiler-synthesized bridging constructs may use:

```mlir
loc(unknown)
```

unless a more precise Semantic IR origin exists.

---

# 19. Go lowering package

Create:

```text
internal/lowering/secmlir/
```

Recommended files:

```text
doc.go
emitter.go
types.go
constants.go
functions.go
storage.go
calls.go
cfg.go
escape.go
errors.go

emitter_test.go
types_test.go
constants_test.go
storage_test.go
calls_test.go
cfg_test.go
```

Required package dependency rule:

The package may import:

```text
standard library
internal/ir/semantic
```

It must not import:

```text
internal/ast
internal/lexer
internal/parser
internal/sema
internal/codegen/llvm
internal/codegen/mlir
```

This dependency rule must have an automated test.

---

# 20. Lowering API

Recommended API:

```go
func Emit(module *semantic.Module) ([]byte, error)
```

or:

```go
func Write(w io.Writer, module *semantic.Module) error
```

Behavior:

1. reject nil module;
2. call `semantic.Verify(module)`;
3. require Semantic IR version `1`;
4. validate that all types/operations belong to the Package 4 supported subset;
5. construct deterministic type/function/block/value/storage mappings;
6. emit textual MLIR using dialect schema version `2`;
7. return emitted bytes.

The emitter must not run external tools itself.

Verification through `sec-mlir-opt` belongs to the calling compiler/toolchain
layer.

Define:

```go
type UnsupportedLoweringError struct {
    Feature string
}
```

for valid Semantic IR constructs outside Package 4.

A malformed Semantic IR module remains a verifier/internal compiler error, not
an unsupported-lowering error.

---

# 21. Why textual MLIR is permitted

The Sec frontend is implemented in Go while the Sec dialect is implemented in
the MLIR C++ infrastructure.

Package 4 may emit canonical textual MLIR directly from verified Semantic IR.

This is allowed only under these constraints:

```text
the emitter consumes Semantic IR only;
syntax is defined by rules/mlir/sec_mlir_dialect.md;
sec-mlir-opt parses and verifies the generated output;
the textual emitter contains no source-language semantic analysis;
tests cover every emitted operation/type;
future replacement by MLIR C API or another construction mechanism must not
change Sec MLIR semantics.
```

Do not introduce cgo or a C++ bridge in Package 4 merely to construct MLIR
objects in memory.

That may be reconsidered later if textual emission becomes a maintenance
problem.

---

# 22. MLIR implementation files

Extend the Package 1 MLIR tree.

Add or update:

```text
mlir/include/sec/Dialect/Sec/
    SecDialect.td
    SecTypes.td
    SecOps.td
    SecDialect.h
    SecTypes.h
    SecOps.h

mlir/lib/Dialect/Sec/
    SecDialect.cpp
    SecTypes.cpp
    SecOps.cpp

mlir/test/Dialect/Sec/
    scalar-types-roundtrip.mlir
    constants-roundtrip.mlir
    storage-roundtrip.mlir
    calls-roundtrip.mlir
    invalid-constants.mlir
    invalid-storage.mlir
    invalid-calls.mlir
    schema-v1-regression.mlir
```

Update CMake/TableGen generation accordingly.

Generated `.inc` files remain build artifacts and must not be committed.

---

# 23. Dialect registration

`SecDialect::initialize()` must register:

```text
Package 1 types
Package 4 types
Package 4 operations
```

`sec-mlir-opt` must register standard dialects required by generated Package 4
IR:

```text
func
cf
```

plus any other already-required upstream dialects.

Do not register LLVM dialect merely because it may be used later unless it is
already part of the normal generic optimizer-driver registry.

Package 4 tests must not require LLVM lowering.

---

# 24. Sec MLIR verifier requirements

In addition to ODS-generated structural checks, implement custom verification
for semantic relationships that ODS alone cannot express.

Required:

```text
integer constant arbitrary-precision syntax
fixed-width integer representability
unsigned non-negative check
constant result semantic type category
decimal coefficient/scale validity
storage<T> element type validity
storage operation element/result equality
storage class schema-v2 value
storage_id > 0
call target symbol resolution
direct call target is non-extern
foreign call target is extern
call operand/result types equal target function type
argument action count equals operand count
only copy-trivial action allowed in schema v2
```

Do not re-run Semantic IR dominance or source semantics inside the MLIR dialect
verifier.

MLIR's own SSA/CFG verifier handles ordinary SSA and branch validity.

---

# 25. Function and CFG MLIR verification

Generated output must pass normal MLIR verification for:

```text
function signatures
block argument types
SSA dominance
cf.br successor arity/types
cf.cond_br condition and successor arity/types
func.return arity/types
symbol references
```

The bridge must not disable verification.

Do not use:

```text
--allow-unregistered-dialect
```

for generated Package 4 output.

---

# 26. Compiler command

Add:

```bash
sec emit-sec-mlir <file.sec>
```

Supported:

```bash
sec emit-sec-mlir <file.sec>
sec emit-sec-mlir <file.sec> -o <file.mlir>
sec emit-sec-mlir <file.sec> -o -
sec emit-sec-mlir <file.sec> --target <os-arch>
```

Do not change the meaning of existing:

```bash
sec emit-mlir
```

in this package.

That command remains legacy until a later migration package explicitly replaces
it.

Execution pipeline:

```text
parse
target selection/filtering
Sema
Semantic IR Build
Semantic IR Verify
Sec MLIR Emit
write temporary MLIR
sec-mlir-opt verification
write requested output
stop
```

The command must not invoke:

```text
mlir-translate
clang
LLVM codegen
legacy MLIR generator
legacy LLVM generator
```

If `sec-mlir-opt` is unavailable, report a toolchain error.

Generated output must not be printed as successful output before verification
has succeeded.

---

# 27. Toolchain integration

Reuse Package 1 toolchain discovery.

If Package 1 exposes:

```go
Toolchain.VerifySec(path string) error
```

use it.

If only file-based verification exists, Package 4 may create a temporary `.mlir`
file and remove it after verification.

Do not shell-construct unsafe command strings.

Use existing process execution helpers.

Unit tests for the CLI must use a fake `sec-mlir-opt` executable through
`SEC_MLIR_BIN` or the repository's established toolchain test seam.

`go test ./...` must not require a real MLIR installation.

---

# 28. Required valid lowering examples

## V01 - integer return

Semantic source equivalent:

```sec
module main

fn Answer() int {
    return 42
}
```

Expected important MLIR facts:

```text
module sec.dialect_version = 2
func.func has sec.function_id = "main::Answer()"
return type is !sec.int
sec.const.int value = "42"
func.return uses constant result
locations are present
```

## V02 - fixed signed integer

```sec
fn Answer() int32 {
    return 42
}
```

Expected result type:

```text
si32
```

## V03 - uint

Expected:

```text
!sec.uint
```

not `index`.

## V04 - decimal exactness

```sec
fn Price() decimal {
    return 0.10
}
```

Expected attributes:

```text
coefficient = "10"
scale = 2
lexeme = "0.10"
result type = !sec.decimal
```

## V05 - named scalar

A supported named scalar over `int` lowers to:

```text
!sec.named<"main::Name", !sec.int>
```

The name must not be erased.

## V06 - mutable scalar storage

Package 3 semantic storage lowers to:

```text
sec.storage.declare
sec.storage.init
sec.storage.store
sec.storage.load
```

No `memref.alloca`.

No `llvm.alloca`.

## V07 - direct call

Expected:

```text
sec.call.direct
```

targeting deterministic MLIR function symbol.

Target `func.func` carries original exact `sec.function_id`.

## V08 - foreign call

Expected:

```text
sec.call.foreign
```

Target `func.func` has:

```text
sec.extern = true
```

and ABI/link-name metadata when Semantic IR has it.

## V09 - if/else

Semantic branch CFG lowers to:

```text
cf.cond_br
cf.br
```

with valid MLIR blocks.

## V10 - all-return if

No fake merge block is introduced by the bridge.

The bridge reproduces the Semantic IR CFG rather than reconstructing source
`if`.

## V11 - block arguments

Semantic block parameters lower to MLIR block arguments.

Incoming branch operands match in order and type.

---

# 29. Required unsupported lowering tests

The Package 4 lowering package must reject valid Semantic IR containing future
operations/types it does not understand.

Examples:

```text
copy
move
destroy
borrow
Result
try
aggregate operation
allocation
register access
concurrency
```

Expected Go error category:

```text
UnsupportedLoweringError
```

The emitter must not:

```text
skip the operation
print an unknown placeholder
lower it to an unrelated standard operation
fall back to AST or legacy codegen
```

---

# 30. Required MLIR dialect tests

## M01 - Schema v1 regression

Package 1 type-only IR still parses.

## M02 - New scalar type round-trip

Round-trip every new type:

```text
!sec.int
!sec.uint
!sec.float
!sec.char
!sec.rune
!sec.string
!sec.decimal
!sec.never
!sec.storage<!sec.int>
```

## M03 - Nested storage rejected

```text
!sec.storage<!sec.storage<!sec.int>>
```

must fail.

## M04 - Integer constant round-trip

Test:

```text
positive
negative
large arbitrary precision with !sec.int
fixed-width boundary values
```

## M05 - Fixed signed overflow rejected

`128` to `si8` rejected.

`-129` to `si8` rejected.

## M06 - Fixed unsigned overflow rejected

`256` to `ui8` rejected.

Negative to `ui8` rejected.

## M07 - Decimal round-trip

Preserve coefficient, scale and lexeme.

## M08 - String constant round-trip

Preserve decoded value.

## M09 - Storage round-trip

All four storage operations parse/print.

## M10 - Storage type mismatch rejected

Init/store/load inconsistency rejected.

## M11 - Storage ID zero rejected

## M12 - Unknown storage class rejected

## M13 - Direct call valid

Non-extern target accepted.

## M14 - Direct call to extern rejected

## M15 - Foreign call valid

Extern target accepted.

## M16 - Foreign call to non-extern rejected

## M17 - Call arity mismatch rejected

## M18 - Call argument type mismatch rejected

## M19 - Call result mismatch rejected

## M20 - Argument action count mismatch rejected

## M21 - Unknown argument action rejected

## M22 - Standard CFG accepted

Generated-style `func.func` + `cf.cond_br` + `cf.br` verifies.

## M23 - No unregistered dialect requirement

All Package 4 generated constructs verify without allow-unregistered flags.

---

# 31. Required Go lowering tests

## G01 - Dependency boundary

Automated test or source-level package dependency check proves:

```text
internal/lowering/secmlir
```

does not import:

```text
ast
lexer
parser
sema
legacy codegen
```

## G02 - Semantic Verify invoked

Pass malformed Semantic IR.

Expected emitter failure before textual MLIR is produced.

## G03 - Version mismatch rejected

Semantic IR version other than `1` rejected.

## G04 - Module metadata deterministic

Same module emits byte-identical:

```text
sec.dialect_version
sec.semantic_ir_version
sec.module_id
sec.source_files
```

## G05 - Type mapping table

Every Package 4 supported Semantic IR type maps exactly as specified.

## G06 - Named type identity

Named type remains `!sec.named`.

## G07 - Function symbol mapping deterministic

Same function order always maps:

```text
@sec_fn_0
@sec_fn_1
...
```

## G08 - FunctionID retained

MLIR function attributes contain exact Semantic FunctionID.

## G09 - Source location escaping

Paths containing spaces, quotes or backslashes produce valid MLIR string
escaping.

## G10 - Value mapping deterministic

Repeated lowering has byte-identical SSA naming.

## G11 - Storage mapping

Every Semantic StorageID maps to one `sec.storage.declare` SSA handle.

## G12 - Direct call exact target

Semantic FunctionID resolves through the precomputed function-symbol table.

No spelling lookup.

## G13 - Foreign call remains foreign

ForeignCall never emits DirectCall.

## G14 - Argument action preserved

Every Package 3 `copy-trivial` action appears in MLIR metadata.

## G15 - CFG reproduced

Semantic successor blocks and branch operands emit in exact order.

## G16 - Block parameters reproduced

Types/order are preserved.

## G17 - Unknown semantic operation rejected

Typed `UnsupportedLoweringError`.

## G18 - No legacy fallback

Test seam proves emitter does not invoke legacy MLIR/LLVM generators.

---

# 32. Required CLI tests

## C01 - emit-sec-mlir simple

Run:

```bash
sec emit-sec-mlir testdata/semantic_ir/basic_return.sec
```

using a fake accepting `sec-mlir-opt`.

Expected:

```text
exit 0
contains sec.dialect_version = 2
contains sec.const.int
contains func.return
```

## C02 - verification failure blocks output

Fake `sec-mlir-opt` returns failure.

Expected:

```text
non-zero exit
no successful MLIR output written to requested destination
```

Use temporary output then atomic rename if needed.

## C03 - file output

`-o file.mlir` writes only after verification succeeds.

## C04 - stdout output

stdout receives verified output only.

## C05 - legacy emit-mlir unchanged

Existing command behavior/tests unchanged.

## C06 - emit-ir unchanged

Package 2/3 command remains Semantic IR only.

## C07 - full Go regression

```bash
go test ./...
```

passes without real MLIR installed.

---

# 33. Required real integration tests

These tests are separate from ordinary `go test ./...`.

Build Package 1/4 MLIR project:

```bash
cmake -S mlir -B build/sec-mlir -G Ninja \
    -DMLIR_DIR=<...> \
    -DLLVM_DIR=<...>

cmake --build build/sec-mlir
cmake --build build/sec-mlir --target check-sec-mlir
```

Then build Sec compiler and run representative Package 4 inputs:

```bash
SEC_MLIR_BIN=<path-to-build-tools> \
    sec emit-sec-mlir <input.sec> -o <output.mlir>

<path-to-sec-mlir-opt> <output.mlir> -o /dev/null
```

Required representative cases:

```text
integer return
decimal return
named scalar
mutable local
direct call
foreign call where an existing valid fixture is available
if/else CFG
nested if CFG
```

Every generated file must verify.

---

# 34. No semantic reconstruction

The Package 4 emitter must not derive facts that Semantic IR should own.

Forbidden examples:

```text
looking at source function names to decide extern status
parsing FunctionID to determine parameter types
inferring named type base from type spelling
selecting a call target
rechecking overloads
deciding whether storage is mutable from a variable name
reconstructing source if-statements from blocks
choosing branch merges
inferring call argument action
```

If required data is absent from valid Package 2/3 Semantic IR, fix Semantic IR
or its builder rather than teaching the MLIR emitter to guess.

---

# 35. Error policy

Separate these failures.

## Semantic IR verifier failure

Meaning:

```text
compiler bug or invalid internal IR
```

The emitter stops.

## UnsupportedLoweringError

Meaning:

```text
valid Semantic IR construct not implemented by Package 4 lowering
```

The emitter stops without output.

## Sec MLIR dialect verification failure

Meaning:

```text
compiler lowering bug or dialect implementation bug
```

`emit-sec-mlir` reports an internal lowering/tool verification error.

Do not present it as invalid Sec source.

---

# 36. Acceptance criteria

Package 4 is complete only when:

```text
[ ] Packages 1-3 remain green
[ ] rules/mlir/sec_mlir_dialect.md updated to supplied schema version 2
[ ] compiler-generated dialect marker is 2
[ ] semantic_ir_version marker is emitted as 1
[ ] !sec.int implemented
[ ] !sec.uint implemented
[ ] !sec.float implemented
[ ] !sec.char implemented
[ ] !sec.rune implemented
[ ] !sec.string implemented
[ ] !sec.decimal implemented
[ ] !sec.never implemented
[ ] !sec.storage<T> implemented
[ ] Package 1 named/distinct types remain working
[ ] Semantic IR type mapping is exact
[ ] target-sized int/uint are not lowered to index
[ ] decimal is not lowered to binary float
[ ] char and rune remain distinct
[ ] sec.const.int implemented
[ ] sec.const.bool implemented
[ ] sec.const.float implemented
[ ] sec.const.decimal implemented
[ ] sec.const.string implemented
[ ] integer exactness/representability verifier exists
[ ] decimal exactness preserved
[ ] four sec.storage operations implemented
[ ] no memref/LLVM storage materialization occurs
[ ] sec.call.direct implemented
[ ] sec.call.foreign implemented
[ ] direct/foreign target class verified
[ ] call signature verified
[ ] argument actions preserved
[ ] functions use func.func
[ ] returns use func.return
[ ] branches use cf.br
[ ] conditions use cf.cond_br
[ ] block arguments use normal MLIR block args
[ ] module/function source locations preserved
[ ] FunctionID preserved as metadata
[ ] deterministic @sec_fn_N mapping implemented
[ ] internal/lowering/secmlir imports Semantic IR but not AST/Sema
[ ] lowering validates Semantic IR before emitting
[ ] lowering never invokes legacy codegen
[ ] unsupported future Semantic IR fails explicitly
[ ] sec emit-sec-mlir exists
[ ] output is published only after sec-mlir-opt verification
[ ] legacy emit-mlir remains unchanged
[ ] sec emit-ir remains unchanged
[ ] check-sec-mlir passes
[ ] real generated Package 4 MLIR verifies with sec-mlir-opt
[ ] go test ./... passes without requiring real MLIR
```

---

# 37. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. Package 1-3 pre-status
3. rules/mlir/sec_mlir_dialect.md update performed
4. files added
5. files modified
6. dialect schema version
7. types added
8. operations added
9. standard dialect constructs reused
10. Semantic IR type mapping implemented
11. Go lowering package dependency list
12. FunctionID -> MLIR symbol strategy
13. source-location strategy
14. sec emit-sec-mlir pipeline
15. CMake configure/build commands
16. MLIR/LLVM version used
17. check-sec-mlir result
18. Go test commands/results
19. real end-to-end generated-MLIR verification results
20. unsupported Semantic IR constructs encountered
21. deviations
22. recommendations for Package 5
```

Do not silently broaden the dialect.

---

# 38. Package 5 boundary

Package 5 should begin the first real semantic lowering **inside MLIR**.

Recommended Package 5:

```text
Sec MLIR scalar/storage/call normalization to lower standard MLIR
```

Likely scope:

```text
define target representation policy for !sec.int/!sec.uint/!sec.float
define decimal lowering boundary without yet implementing all decimal arithmetic
lower fixed semantic constants where representation is known
lower sec.storage.* for trivial local scalars to an appropriate standard
representation
lower sec.call.direct to func.call when no Sec call metadata remains necessary
lower sec.call.foreign to a lower call representation while retaining ABI data
remove schema-v2-only metadata only when its semantic obligation is discharged
add conversion target legality tests
```

Package 5 must not yet absorb:

```text
copy/move/destruction
borrowing
Result/try
defer/cleanup
aggregates
allocation
register/MMIO
concurrency
```

A later ownership package must first extend Semantic IR and Sec MLIR with those
operations.

The next invariant should be:

```text
verified Sec MLIR schema v2
    ↓
explicit dialect conversion
    ↓
verified lower MLIR

with no unresolved Package 4 Sec storage/call semantics silently discarded.
```

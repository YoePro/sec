# Sec MLIR Dialect

## Status

Normative detailed representation specification for the Sec MLIR dialect.

Current dialect schema version: `2`

This rulebook is subordinate to:

```text
rules/sec_mlir.md
rules/semantic_ir.txt
all applicable Sec language/domain rulebooks
```

It defines representation and verifier obligations.

It must not introduce or redefine Sec source-language semantics.

---

# 1. Version history

## Schema version 1

Defined the initial dialect foundation:

```text
sec namespace
!sec.named
!sec.distinct
common metadata conventions
MLIR Location source provenance
dialect version marker
```

## Schema version 2

Adds the first representation surface required to lower Semantic IR version 1
from implementation Packages 2 and 3:

```text
semantic scalar types
semantic storage handle type
exact constant operations
semantic storage operations
direct calls
foreign calls
module/function metadata conventions
standard func/cf integration rules
```

Schema version 2 still does not define ownership, borrowing, aggregates,
Result/try, cleanup, allocation, hardware or concurrency operations.

---

# 2. Authority rule

The Sec MLIR dialect represents decisions already made by higher compiler
layers.

It must not:

```text
perform source name lookup
perform overload resolution
infer ownership
invent copy or move
invent contracts
invent ABI
reinterpret source syntax
repair invalid Semantic IR
```

Invalid source is rejected before Semantic IR.

Invalid Semantic IR is rejected before Sec MLIR construction.

Sec MLIR verifiers validate dialect representation invariants.

---

# 3. Dialect identity

MLIR dialect namespace:

```text
sec
```

C++ namespace:

```text
::sec
```

The dialect is statically registered and not runtime-extensible.

---

# 4. Compiler-generated module markers

Schema version 2 compiler-generated modules carry:

```mlir
sec.dialect_version = 2 : i32
sec.semantic_ir_version = 1 : i32
```

The dialect version identifies the Sec MLIR representation schema.

The Semantic IR version identifies the input semantic representation.

Development tests may omit these markers where the test is scoped to one
individual type or operation.

Schema version 1 modules remain parseable.

No version migration pass is defined yet.

---

# 5. Common metadata

Reserved metadata names:

```text
sec.symbol_id
sec.type_id
sec.layout_ref
sec.synthesized
sec.dialect_version
sec.semantic_ir_version
sec.module_id
sec.source_files

sec.function_id
sec.source_name
sec.extern
sec.unsafe
sec.link_name
sec.abi
sec.parameter_names

sec.storage_id
sec.storage_class
sec.mutable

sec.argument_actions
```

Expected standard MLIR attribute classes:

```text
sec.symbol_id            StringAttr
sec.type_id              StringAttr
sec.layout_ref           StringAttr
sec.synthesized          BoolAttr
sec.dialect_version      IntegerAttr(i32)
sec.semantic_ir_version  IntegerAttr(i32)
sec.module_id            StringAttr
sec.source_files         ArrayAttr<StringAttr>

sec.function_id          StringAttr
sec.source_name          StringAttr
sec.extern               BoolAttr
sec.unsafe               BoolAttr
sec.link_name            StringAttr
sec.abi                  StringAttr
sec.parameter_names      ArrayAttr<StringAttr>

sec.storage_id           IntegerAttr(i32)
sec.storage_class        StringAttr
sec.mutable              BoolAttr

sec.argument_actions     ArrayAttr<StringAttr>
```

Metadata strings are opaque unless this rulebook explicitly gives a fixed set
of allowed values.

Code must not derive unrelated semantics by parsing identity strings.

---

# 6. Source provenance

Normal MLIR `Location` is the canonical source-location representation.

Example:

```mlir
loc("example.sec":12:8)
```

Unknown or synthesized origin uses:

```mlir
loc(unknown)
```

where no more precise Semantic IR location exists.

Do not duplicate ordinary file/line/column data in Sec-specific attributes.

`sec.synthesized = true` may be used when an operation needs an explicit
synthesized marker in addition to `loc(unknown)`.

---

# 7. Package 1 identity types

The following schema-version-1 types remain valid.

## 7.1 Named type

```mlir
!sec.named<"identity", base-type>
```

Parameters:

```text
identity: StringAttr
base: Type
```

Rules:

```text
identity is non-empty
base is valid
base is not NoneType
type identity includes both identity and base
named type is not implicitly interchangeable with base
```

## 7.2 Distinct type

```mlir
!sec.distinct<"identity", base-type>
```

Parameters and rules are equivalent to `!sec.named`, except the semantic class
is a distinct type rather than a named type.

The Sec MLIR layer does not infer whether a source declaration is named versus
distinct.

It receives that decision from Semantic IR.

---

# 8. Semantic scalar types

Schema version 2 adds:

```text
!sec.int
!sec.uint
!sec.float
!sec.char
!sec.rune
!sec.string
!sec.decimal
!sec.never
```

These types preserve Sec semantics that must not be erased before the
corresponding lower representation policy is defined.

---

# 9. Target-sized integer types

## 9.1 `!sec.int`

Represents Sec target-sized signed integer `int`.

It is not:

```text
MLIR index
LLVM pointer-sized integer chosen already
```

Final width is selected later from the target profile.

## 9.2 `!sec.uint`

Represents Sec target-sized unsigned integer `uint`.

The same non-commitment rule applies.

---

# 10. Default float type

`!sec.float` represents the Sec source-level `float` type when the semantic type
is distinct from explicit `float32` and `float64`.

Package 4/schema version 2 does not choose its final backend width.

Explicit `float32` and `float64` use builtin MLIR `f32` and `f64`.

---

# 11. Character types

`!sec.char` and `!sec.rune` are distinct.

They must not be collapsed into one integer type at high-level Sec MLIR.

A later lowering may choose integer representations only after all semantic
obligations are discharged.

---

# 12. String type

`!sec.string` preserves high-level Sec string identity.

Schema version 2 does not define:

```text
physical fields
pointer
length
capacity
allocation
ownership
ABI
copy policy
destruction
```

`!sec.string` may be produced by `sec.const.string`.

Other string operations are not part of schema version 2.

---

# 13. Decimal type

`!sec.decimal` represents exact Sec decimal semantics.

Schema version 2 does not define the final physical representation.

The current legacy LLVM representation must not be treated as the high-level
dialect definition.

Decimal values must not be converted to binary float merely for convenience.

---

# 14. Never type

`!sec.never` represents Sec `never` where a type is required.

Schema version 2 does not define all control-flow operations that may produce
never.

---

# 15. Storage handle type

Canonical form:

```mlir
!sec.storage<T>
```

Meaning:

```text
one semantic storage identity containing T
```

It is not:

```text
a pointer
a reference
a memref
an allocation
a stack slot
a heap slot
```

Verifier:

```text
T is a valid non-void type
T is not !sec.storage<U>
```

Storage placement is selected by later lowering.

---

# 16. Semantic IR type mapping

For Semantic IR version 1 Package 2/3 values:

```text
void      -> no MLIR value type
never     -> !sec.never
bool      -> i1
byte      -> ui8
char      -> !sec.char
rune      -> !sec.rune
string    -> !sec.string
decimal   -> !sec.decimal

int       -> !sec.int
int8      -> si8
int16     -> si16
int32     -> si32
int64     -> si64

uint      -> !sec.uint
uint8     -> ui8
uint16    -> ui16
uint32    -> ui32
uint64    -> ui64

float     -> !sec.float
float32   -> f32
float64   -> f64
```

A Semantic IR named type wraps the mapped base:

```mlir
!sec.named<"identity", mapped-base>
```

A Semantic IR distinct type wraps the mapped base:

```mlir
!sec.distinct<"identity", mapped-base>
```

The mapping is normative for schema version 2.

---

# 17. Constant operations

Schema version 2 defines:

```text
sec.const.int
sec.const.bool
sec.const.float
sec.const.decimal
sec.const.string
```

These are semantic constants.

They are not required to match the physical representation of their eventual
backend values.

---

# 18. `sec.const.int`

One result.

Required attribute:

```text
value: StringAttr
```

`value` is a signed base-10 arbitrary-precision integer spelling.

Allowed semantic result bases:

```text
!sec.int
!sec.uint
!sec.char
!sec.rune
si8
si16
si32
si64
ui8
ui16
ui32
ui64
```

A named/distinct wrapper around one of these bases is also allowed.

Verifier:

```text
value parses as arbitrary-precision integer
unsigned categories reject negative values
fixed-width builtin integer categories reject out-of-range values
named/distinct wrappers are checked recursively through their base
```

No truncation is allowed.

---

# 19. `sec.const.bool`

One result.

Required attribute:

```text
value: BoolAttr
```

Allowed result base:

```text
i1
```

Named/distinct wrappers over `i1` are allowed.

---

# 20. `sec.const.float`

One result.

Required attribute:

```text
lexeme: StringAttr
```

Allowed result base:

```text
!sec.float
f32
f64
```

Named/distinct wrappers are allowed.

The lexeme preserves Semantic IR floating constant spelling.

The schema-v2 verifier need not choose final backend rounding for `!sec.float`.

---

# 21. `sec.const.decimal`

One result.

Required attributes:

```text
coefficient: StringAttr
scale: IntegerAttr(i32)
lexeme: StringAttr
```

Allowed result base:

```text
!sec.decimal
```

Named/distinct wrappers over `!sec.decimal` are allowed.

Verifier:

```text
coefficient parses as arbitrary-precision signed integer
scale is non-negative
lexeme is non-empty
```

Scale normalization is not required.

---

# 22. `sec.const.string`

One result.

Required attribute:

```text
value: StringAttr
```

Allowed result base:

```text
!sec.string
```

Named/distinct wrappers over `!sec.string` are allowed.

The value is the decoded semantic string.

---

# 23. Semantic storage operations

Schema version 2 defines:

```text
sec.storage.declare
sec.storage.init
sec.storage.load
sec.storage.store
```

These preserve the Semantic IR storage abstraction.

They must not choose physical storage.

---

# 24. `sec.storage.declare`

No operands.

One result:

```text
!sec.storage<T>
```

Required attributes:

```text
sec.storage_id
sec.source_name
sec.storage_class
sec.mutable
```

Schema version 2 accepts storage class:

```text
local-automatic
```

The Package 3 bridge emits:

```text
sec.mutable = true
```

for mutable local storage.

Verifier:

```text
storage_id > 0
source_name may be empty only for compiler-generated/synthesized storage
storage_class == "local-automatic"
result is !sec.storage<T>
```

---

# 25. `sec.storage.init`

Operands:

```text
storage: !sec.storage<T>
value: T
```

No result.

Verifier checks exact type equality.

The operation does not define copy/move/destruction behavior beyond the
Package 3 trivial-storage subset.

---

# 26. `sec.storage.load`

Operand:

```text
storage: !sec.storage<T>
```

Result:

```text
T
```

Verifier checks exact type equality.

Schema version 2 load is valid only for the upstream Package 3 trivial-storage
subset.

The dialect operation itself does not infer whether a future non-trivial load
copies, borrows or moves.

---

# 27. `sec.storage.store`

Operands:

```text
storage: !sec.storage<T>
value: T
```

No result.

Verifier checks exact type equality.

Schema version 2 represents only Package 3 trivial replacement.

It must not be used for non-trivial ownership replacement.

---

# 28. Call operations

Schema version 2 defines:

```text
sec.call.direct
sec.call.foreign
```

Both contain:

```text
callee: FlatSymbolRefAttr
sec.argument_actions: ArrayAttr<StringAttr>
```

Schema version 2 accepts only action:

```text
copy-trivial
```

Action count equals operand count.

---

# 29. `sec.call.direct`

Represents a resolved direct Sec function call.

Verifier:

```text
callee resolves to func.func
target sec.extern is absent or false
operand count/types equal target inputs
result count/types equal target outputs
argument action count equals operand count
every action is "copy-trivial"
```

The operation does not perform overload resolution.

---

# 30. `sec.call.foreign`

Represents a resolved foreign/extern call.

Verifier:

```text
callee resolves to func.func
target sec.extern == true
operand count/types equal target inputs
result count/types equal target outputs
argument action count equals operand count
every action is "copy-trivial"
```

Calling convention and foreign symbol metadata are stored on the target
function.

---

# 31. Function representation

Use standard:

```text
func.func
```

Required compiler-generated function attributes:

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

The exact Semantic IR FunctionID is preserved in `sec.function_id`.

The MLIR symbol name is a lowering-local symbol and must not be treated as Sec
ABI mangling.

Extern functions have no body.

Non-extern functions have a body.

---

# 32. Standard control flow

Schema version 2 uses:

```text
func.return
cf.br
cf.cond_br
MLIR block arguments
```

No Sec-specific equivalents are defined.

Reason:

for the Package 2/3 subset, these standard constructs preserve all remaining
semantic obligations.

If later cleanup/ownership/failure semantics require higher-level operations,
those operations must be added before lowering to the standard constructs.

---

# 33. Function symbol mapping

The canonical Package 4 emitter uses deterministic local symbols:

```text
@sec_fn_0
@sec_fn_1
@sec_fn_2
...
```

based on deterministic Semantic IR function order.

This is not an ABI.

It is not a stable external link name.

Exact semantic identity remains in:

```text
sec.function_id
```

Foreign link identity remains in:

```text
sec.link_name
```

when available.

---

# 34. Module metadata

Compiler-generated schema-v2 modules contain:

```text
sec.dialect_version
sec.semantic_ir_version
sec.module_id
sec.source_files
```

with standard attribute classes described above.

`sec.source_files` contains deterministic unique source paths.

---

# 35. Semantic IR lowering boundary

The canonical bridge consumes validated Semantic IR only.

It may not read:

```text
AST
tokens
parser state
Sema symbol tables
overload sets
source declarations not represented in Semantic IR
```

The bridge may maintain transient maps such as:

```text
FunctionID -> MLIR SymbolRef
ValueID -> MLIR SSA value
StorageID -> MLIR storage SSA value
BlockID -> MLIR block
TypeID -> MLIR type
```

These are lowering implementation details.

---

# 36. Textual construction

Schema version 2 permits the Go compiler to emit canonical textual MLIR directly
from Semantic IR.

This does not weaken verification.

Generated output must be parsed and verified by `sec-mlir-opt`.

The textual emitter must follow this rulebook exactly.

A future in-memory MLIR construction path may replace textual emission without
changing dialect semantics.

---

# 37. Verification boundary

Sec MLIR dialect verification validates representation invariants such as:

```text
type well-formedness
constant attribute validity
fixed-width constant representability
storage type consistency
call symbol category
call signature consistency
argument action validity
```

Normal MLIR verifies:

```text
SSA dominance
block argument arity/types
branch validity
function return validity
symbol references
```

The Sec MLIR verifier does not redo:

```text
source type checking
name lookup
overload resolution
ownership analysis
borrow analysis
definite assignment
effect analysis
control-flow reachability analysis already guaranteed by Semantic IR
```

---

# 38. Schema version 2 exclusions

Not represented yet:

```text
copy
move
destroy
cleanup
defer

shared reference
mutable reference
RawPtr operations
borrow/reborrow

Result
Option
try
panic
runtime checks

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

register
MMIO
volatile
atomic

spawn
await
concurrency

ABI lowering
```

These constructs require future versions of this rulebook.

They must not be encoded using unrelated schema-v2 operations.

---

# 39. Evolution rule

Before adding any new Sec dialect type or operation:

1. the source semantics must already exist in higher-authority rules;
2. Semantic IR must represent the resolved semantics when applicable;
3. this rulebook must define the MLIR representation;
4. verifier obligations must be stated;
5. tests must exist;
6. only then may implementation be added.

Standard MLIR constructs should be reused when and only when they preserve all
remaining Sec obligations exactly.

---

# 40. Schema version 2 completion

Schema version 2 is implemented when:

```text
all schema-v1 tests remain green
new scalar types parse/print/verify
storage<T> parses/prints/verifies
constant operations preserve exact semantic data
storage operations verify exact element types
direct and foreign calls verify their target categories
func/cf integration works
compiler-generated Semantic IR v1 subset lowers deterministically
generated output passes sec-mlir-opt without unregistered-dialect mode
```

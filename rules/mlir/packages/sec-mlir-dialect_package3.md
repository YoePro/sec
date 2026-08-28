# Sec MLIR Program - Implementation Package 3

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P3`  
Package title: `Semantic IR Executable Core`  
Repository: `https://github.com/YoePro/sec`  
Repository branch: `main`  
Repository sync commit used for this package: `d48035c`  
Repository sync date: `2026-08-08`  
Semantic IR schema version: `1`

Package 3 builds directly on Package 2.

It extends the canonical Semantic IR far enough to represent useful executable
scalar programs with:

```text
mutable local storage
assignment
resolved direct calls
resolved foreign calls
multi-block control flow
if / else
```

It still does not create MLIR.

The package deliberately restricts storage and call transfer to types whose
semantics do not require the not-yet-implemented copy/move/destruction/borrow
IR.

---

# 1. Normative authority

Implementation must follow the existing authority chain:

```text
language/domain rulebooks
    ↓
rules/compiler/semantic_ir.txt
    ↓
rules/mlir/sec_mlir.md
    ↓
rules/mlir/sec_mlir_dialect.md
    ↓
implementation packages
    ↓
implementation
```

Additional directly relevant rulebooks include:

```text
rules/types/types.md
rules/declarations/functions.md
rules/memory/copy_move.md
rules/memory/ownership.md
rules/memory/borrowing.md
rules/memory/destruction.md
rules/platform/ffi.md
rules/analysis/call_graph.md
rules/analysis/effect_analysis.md
```

When Package 3 deliberately supports only a subset, valid Sec source outside
that subset must produce `UnsupportedFeatureError`.

It must not be reinterpreted as invalid Sec.

---

# 2. Preconditions

Package 3 assumes Package 2 is complete and green.

Required Package 2 capabilities:

```text
internal/ir/semantic package
Semantic IR version 1
canonical TypeTable
FunctionID
BlockID
ValueID
Location
OwnershipClass
function declarations/definitions
function parameters
basic blocks
constants
return
builder
verifier
deterministic printer
sec emit-ir
read-only Sema result access
frontend retains successful Analyzer
```

If Package 2 was implemented with different internal file names, Package 3 must
adapt to the actual implementation while preserving the public semantic
contract.

Do not reimplement Package 2 in parallel.

---

# 3. Package goal

After Package 3:

1. mutable scalar locals have explicit Semantic IR storage identity;
2. initialization, loads and replacement stores are explicit;
3. mutable source variables are not faked as ordinary immutable SSA values;
4. direct Sec calls retain the exact resolved callee;
5. foreign calls remain distinguishable from ordinary Sec calls;
6. call argument evaluation order is explicit in operation order;
7. call argument actions are explicit for the supported subset;
8. `if`, `if/else`, nested `if` and `else if` lower to explicit basic blocks;
9. all blocks terminate explicitly;
10. branch targets and branch arguments are verifiable;
11. SSA uses obey cross-block dominance;
12. storage declaration/initialization dominates every supported storage use;
13. `sec emit-ir` can dump these constructs deterministically;
14. no MLIR/LLVM tool is required;
15. ownership-sensitive/non-trivial values remain rejected until their explicit
    Semantic IR operations exist.

This package completes the first useful scalar/CFG Semantic IR foundation.

---

# 4. Package boundary

## 4.1 In scope

Implement:

```text
StorageID
local storage descriptors
storage.declare
storage.init
storage.load
storage.store
mutable local declarations with initializer
simple assignment to supported mutable locals
direct function calls
foreign function calls
exact call target retention from Sema
explicit call argument action metadata
left-to-right call argument evaluation
void calls as statements
non-void calls in value context
branch terminator
conditional branch terminator
block arguments in the IR model
if without else
if with else
nested if
else if
CFG verification
branch arity/type verification
SSA dominance verification
storage declaration dominance
storage initialization dominance
deterministic printing of storage/calls/CFG
sec emit-ir support for the new subset
Sema read-only resolved binding/call APIs
focused integration tests
full regression tests
```

## 4.2 Explicitly out of scope

Do not implement in Package 3:

```text
MLIR construction
Sec MLIR dialect changes
Semantic IR -> Sec MLIR bridge
LLVM codegen changes

typed mutable local declarations without initializer
path-sensitive maybe-uninitialized storage
reinitialization after move/discard
non-trivial replacement/destruction

copy operation
move operation
replace-owned operation
destruction
cleanup
defer
discard

borrow
reborrow
reference creation
reference return
RawPtr construction/conversion/dereference

calls requiring move-only argument transfer
calls requiring semantic copy
calls requiring borrowing
method calls
function-value calls
interface calls
intrinsic calls
generated calls
spawn/await calls
callbacks

standalone non-void call result discard

arithmetic operations
overflow checks
division checks
runtime checks
contracts
panic lowering

loops
for
while
switch
match
break
continue
fallthrough

Result
Option
Ok
Err
try

struct construction
field access
arrays
slices
enums
unions
properties
interfaces
closures

allocation
arena
heap/storage placement
register/MMIO
volatile
atomics
concurrency
ABI lowering
```

Do not add placeholders for these features.

---

# 5. Semantic IR schema version

Keep:

```go
const Version uint32 = 1
```

Package 3 is an additive implementation of the initial Semantic IR schema.

Do not increment the version merely because more operations are implemented.

A future incompatible representation change may increment the version.

---

# 6. New identifier: StorageID

Add:

```go
type StorageID uint32
```

Rules:

```text
StorageID is function-local.
StorageID values are allocated deterministically.
StorageID 0 is invalid/reserved.
StorageID is not a ValueID.
StorageID is not a pointer.
StorageID is not an address.
StorageID carries no physical stack/heap placement decision.
```

Canonical debug spelling:

```text
$1
$2
$3
```

`%N` remains reserved for SSA `ValueID`.

`^N` remains reserved for `BlockID`.

---

# 7. Local storage model

Package 3 introduces semantic local storage without deciding physical storage
placement.

Recommended record:

```go
type Storage struct {
    ID       StorageID
    Name     string
    Type     TypeID
    Mutable  bool
    Class    StorageClass
    Location Location
}
```

Package 3 defines:

```go
type StorageClass string

const (
    StorageLocalAutomatic StorageClass = "local-automatic"
)
```

Do not add stack/heap classes in this package.

`StorageLocalAutomatic` means source-level automatic local storage.

It does not mean LLVM `alloca`.

Escape/lifetime/storage-placement analysis may later choose a physical
representation without changing source semantics.

---

# 8. Types allowed in Package 3 storage

Mutable Package 3 storage is deliberately restricted to trivially copyable,
trivially destructible, non-reference scalar values whose current frontend
semantics require no hidden runtime action.

Allowed:

```text
bool
byte
char
rune

int
int8
int16
int32
int64

uint
uint8
uint16
uint32
uint64

float
float32
float64

decimal
```

A named scalar may be used only when all of these are true:

```text
its Package 2 base type is storage-allowed;
it has no contract requiring runtime validation;
it has no unit/conversion semantics that Package 3 would erase;
its copy/destruction classification is trivial;
Sema already resolved the assignment exactly.
```

Not allowed in mutable Package 3 storage:

```text
string
ref T
ref mut T
RawPtr[T]
arrays
slices
structs
unions
interfaces
function values
closures
Result
Option
resource-owning values
move-only values
values with custom destruction
```

When uncertain, reject as unsupported.

Do not infer triviality from physical size alone.

---

# 9. Storage operations

Package 3 adds four operations.

## 9.1 `storage.declare`

Conceptual:

```text
storage.declare $storage
```

Fields:

```text
StorageID
Location
```

Meaning:

- introduces the semantic storage identity at a precise control-flow point;
- does not initialize its value;
- does not allocate heap memory;
- does not imply a machine stack slot.

Verifier:

```text
storage exists in function storage table;
storage ID has not already been declared;
declaration occurs before initialization;
declaration dominates all operations referencing the storage.
```

## 9.2 `storage.init`

Conceptual:

```text
storage.init $1, %3
```

Meaning:

- establishes the first value of the storage;
- Package 3 supports exactly one initialization for a storage identity;
- input type must exactly equal storage type.

Verifier:

```text
storage is declared;
storage is mutable for Package 3 source locals;
storage has not already been initialized;
input value exists;
input type equals storage type;
declaration dominates initialization.
```

## 9.3 `storage.load`

Conceptual:

```text
%5 = storage.load $1 : !T
```

Meaning:

- reads the current scalar value;
- creates a new SSA value;
- Package 3 load action is a trivial value copy;
- it does not transfer ownership.

Result ownership:

```text
OwnershipImmediate
```

Verifier:

```text
storage exists;
storage type is Package 3 loadable;
declaration dominates load;
initialization dominates load;
result type equals storage type.
```

## 9.4 `storage.store`

Conceptual:

```text
storage.store $1, %6
```

Meaning:

- replaces the current initialized scalar value;
- Package 3 permits this only because supported storage has trivial destruction;
- no hidden destructor/copy/move operation is permitted.

Verifier:

```text
storage exists;
storage is mutable;
storage type is Package 3 storable;
declaration dominates store;
initialization dominates store;
input type equals storage type.
```

`storage.store` must not be generalized to owned values.

A later package must introduce explicit ownership-aware replacement semantics.

---

# 10. Mutable local lowering

Supported source:

```sec
let mut value := 1
value = 2
return value
```

Conceptual Semantic IR:

```text
storage $1 "value" : int mutable local-automatic

^0:
  storage.declare $1
  %0 = const.int 1 : int
  storage.init $1, %0

  %1 = const.int 2 : int
  storage.store $1, %1

  %2 = storage.load $1 : int
  return %2
```

The mutable source binding maps to `StorageID`, not directly to one SSA
`ValueID`.

Immutable locals continue to use the Package 2 SSA binding model.

---

# 11. Unsupported uninitialized mutable declarations

Valid Sec syntax such as:

```sec
let mut value: int
```

or:

```sec
int mut: value
```

remains unsupported in Package 3.

Reason:

the complete IR must distinguish:

```text
uninitialized
initialized
conditionally initialized
moved
reinitialized
destroyed
```

Package 3 intentionally avoids introducing an incomplete initialization lattice.

Expected error:

```text
semantic IR feature not implemented in package 3:
mutable local declaration without initializer
```

Do not silently insert a zero/default initializer.

---

# 12. Simple assignment

Package 3 supports assignment only when the target resolves to a supported
mutable local storage identity.

Supported:

```sec
value = expression
```

where:

```text
value is a resolved mutable local;
storage type is Package 3 storable;
expression lowers to a Package 3 supported SSA value;
Sema has already accepted the assignment.
```

Not supported:

```text
field assignment
index assignment
property assignment
compound assignment
destructuring assignment
assignment through ref
assignment through RawPtr
assignment to contract-checked value requiring try
move assignment
```

The Semantic IR builder must use Sema's resolved binding identity.

It must not resolve the target by spelling alone.

---

# 13. Resolved binding identity from Sema

Package 3 must remove any remaining need for the Semantic IR builder to repeat
local identifier name lookup.

Add a compilation-local Sema binding identity.

Recommended:

```go
type BindingID uint32
```

with a read-only public descriptor:

```go
type ResolvedBinding struct {
    ID      BindingID
    Name    string
    Kind    BindingKind
    Type    Type
    Mutable bool
}
```

Required kinds initially:

```text
parameter
local
```

Recommended API:

```go
func (a *Analyzer) ResolvedBindingOf(
    expr *ast.Identifier,
) (ResolvedBinding, bool)
```

or an equivalent read-only API.

Requirements:

```text
the mapping is recorded during Sema;
query does not perform lookup;
query does not infer;
query does not mutate Analyzer;
same declared source binding returns same BindingID;
different bindings never share BindingID in one analyzed program.
```

The Semantic IR builder maintains a transient mapping:

```text
sema.BindingID -> ValueID
```

for immutable/parameter values, or:

```text
sema.BindingID -> StorageID
```

for mutable Package 3 locals.

BindingID does not need to be stored in final Semantic IR unless useful for
debug provenance.

---

# 14. Call categories

Call-graph-relevant call categories must not be collapsed.

Package 3 implements exactly:

```text
DirectCall
ForeignCall
```

Do not represent a foreign call as an ordinary direct Sec call.

Future categories remain separate:

```text
MethodCall
FunctionValueCall
InterfaceCall
IntrinsicCall
GeneratedCall
SpawnTask
SpawnThread
SpawnProcess
CallbackInvoke
```

Those are not implemented now.

---

# 15. Resolved call target from Sema

The Semantic IR builder must never redo overload resolution.

Sema must retain the exact selected call target during successful call analysis.

Add a read-only API such as:

```go
func (a *Analyzer) ResolvedCallTarget(
    call *ast.CallExpression,
) (Function, bool)
```

If constructor/conversion/method calls share an AST call node, the API must also
expose resolved call kind so Package 3 can reject unsupported call kinds rather
than misclassify them.

Recommended:

```go
type ResolvedCallKind string

const (
    ResolvedDirectFunction ResolvedCallKind = "direct-function"
    ResolvedForeignFunction ResolvedCallKind = "foreign-function"
    ResolvedConstructor ResolvedCallKind = "constructor"
    ResolvedConversion ResolvedCallKind = "conversion"
    ResolvedMethod ResolvedCallKind = "method"
    ResolvedFunctionValue ResolvedCallKind = "function-value"
    ResolvedOther ResolvedCallKind = "other"
)
```

Only the first two are accepted by Package 3.

This result must be recorded by Sema, not reconstructed by the builder.

---

# 16. Call-safe types

Package 3 calls are restricted so they do not require unresolved ownership
semantics.

Call-safe by-value types:

```text
bool
byte
char
rune

int
int8
int16
int32
int64

uint
uint8
uint16
uint32
uint64

float
float32
float64

decimal
```

`void` is allowed as a return type.

A Package 3-compatible named scalar is allowed under the same restrictions as
Package 3 storage.

Not Package 3 call-safe:

```text
string
ref
ref mut
RawPtr
arrays
slices
structs
unions
interfaces
function values
closures
Result
Option
move-only types
resource-owning types
types requiring semantic copy
types requiring cleanup
```

The builder must reject a call if any argument or result would require a semantic
operation Package 3 does not represent.

---

# 17. Call argument actions

Each call argument records its semantic use action.

Define:

```go
type ArgumentAction string

const (
    ArgumentCopyTrivial ArgumentAction = "copy-trivial"
)
```

Package 3 supports only:

```text
copy-trivial
```

Do not add `"move"` or `"borrow"` values merely as placeholders unless the
existing Semantic IR API requires a closed enum. If present, the builder must
never emit them in Package 3.

A direct scalar by-value call therefore records that the callee receives a
trivial copy.

This preserves the distinction between language semantics and later machine
register copying.

---

# 18. DirectCall operation

Conceptual:

```text
%5 = call.direct "main::Identity(int)"(%4 [copy-trivial]) : int
```

Recommended fields:

```go
type DirectCallOp struct {
    Callee    FunctionID
    Arguments []CallArgument
    Result    *Value
    Location  Location
}
```

where:

```go
type CallArgument struct {
    Value  ValueID
    Action ArgumentAction
}
```

Rules:

```text
callee must exist;
callee must be a non-extern Sec function;
argument count must match;
argument types must exactly match already-resolved parameter types;
all Package 3 argument actions are copy-trivial;
void callee has no result;
non-void callee has exactly one result;
result type equals callee return type;
result ownership is immediate;
```

No call operation may infer conversions.

Any conversion must already exist as an explicit supported Semantic IR operation
in a later package.

---

# 19. ForeignCall operation

Conceptual:

```text
%5 = call.foreign "platform::clock()"() : int64
```

Recommended fields:

```go
type ForeignCallOp struct {
    Callee    FunctionID
    Arguments []CallArgument
    Result    *Value
    Location  Location
}
```

Rules:

```text
callee must exist;
callee must be extern;
calling convention remains on the function declaration;
unsafe validity has already been checked by Sema;
Package 3 arguments/results must be call-safe scalars;
no foreign ownership transfer is represented in Package 3;
calls requiring ownership/retention/nullability semantics are unsupported.
```

The operation kind must remain distinguishable for later call graph, effect and
FFI analysis.

---

# 20. Call evaluation order

Sec call arguments must be represented in source evaluation order.

Package 3 builder rule:

```text
lower callee target facts from Sema;
evaluate argument 1;
emit all operations for argument 1;
evaluate argument 2;
emit all operations for argument 2;
...
emit call operation last.
```

Do not sort arguments.

Do not evaluate them in map order.

Do not hoist a later argument ahead of an earlier argument.

Verifier checks argument list order structurally but does not attempt to
reconstruct source order.

Builder tests must prove the emitted operation order.

---

# 21. Call statement rules

Supported standalone call:

```sec
Noop()
```

only when return type is `void`.

A standalone non-void call remains unsupported in Package 3 because Sec discard
semantics require an explicit ownership/discard model.

Example:

```sec
Calculate()
```

Expected Package 3 IR error:

```text
semantic IR feature not implemented in package 3:
standalone non-void call/discard
```

Do not silently drop the result.

Explicit `discard` also remains outside Package 3.

---

# 22. Basic-block model

Package 2 introduced blocks.

Package 3 makes them executable CFG nodes.

Each block contains:

```text
BlockID
block parameters
ordered operations
exactly one terminator
```

Package 3 terminators:

```text
return
branch
conditional-branch
```

No implicit fallthrough exists.

---

# 23. Block parameters

Block parameters are SSA merge inputs.

Recommended representation:

```go
type BlockParameter struct {
    Value Value
}
```

Branch operations carry one operand per target block parameter.

Package 3 source lowering does not need block parameters for ordinary mutable
locals because those use explicit storage.

The verifier must nevertheless implement block parameter arity/type checks so
the IR model is ready for later SSA merges and ownership transfer.

A block parameter is defined at block entry and dominates all operations in that
block.

---

# 24. Branch operation

Conceptual:

```text
br ^3(%7, %8)
```

Recommended fields:

```go
type BranchOp struct {
    Target    BlockID
    Arguments []ValueID
    Location  Location
}
```

Verifier:

```text
target block exists;
argument count equals target parameter count;
argument types equal target parameter types;
branch is final operation in source block.
```

---

# 25. Conditional branch operation

Conceptual:

```text
cond_br %0, ^1(), ^2()
```

Recommended fields:

```go
type ConditionalBranchOp struct {
    Condition      ValueID
    TrueTarget     BlockID
    TrueArguments  []ValueID
    FalseTarget    BlockID
    FalseArguments []ValueID
    Location       Location
}
```

Verifier:

```text
condition exists;
condition type is bool;
true target exists;
false target exists;
argument arity/types match both targets;
operation terminates the block.
```

Do not encode source `if` syntax in the operation.

---

# 26. `if` lowering

## 26.1 If without else

Source:

```sec
if flag {
    Work()
}

After()
```

Conceptual CFG:

```text
^entry:
    cond_br %flag, ^then, ^merge

^then:
    call.direct Work()
    br ^merge

^merge:
    call.direct After()
    ...
```

If the then branch returns, it has no branch to merge.

## 26.2 If with else

Source:

```sec
if flag {
    Left()
} else {
    Right()
}

After()
```

Conceptual CFG:

```text
^entry:
    cond_br %flag, ^then, ^else

^then:
    call.direct Left()
    br ^merge

^else:
    call.direct Right()
    br ^merge

^merge:
    call.direct After()
    ...
```

## 26.3 Both branches terminate

Source:

```sec
if flag {
    return 1
} else {
    return 2
}
```

Do not create a fake reachable merge block.

The function has:

```text
entry
then
else
```

and both branch blocks return.

## 26.4 Nested if and else-if

Lower recursively.

An `else if` may be represented by another conditional branch region/block set.

Do not introduce a distinct Semantic IR `else-if` operation.

---

# 27. Supported conditions

Package 3 condition expressions must lower to a bool `ValueID` using operations
already supported by Package 2/3.

Examples:

```text
bool parameter
bool immutable local
bool mutable local load
bool literal
direct call returning bool
foreign call returning bool
```

Arithmetic/comparison expression lowering is not added in this package.

A semantically valid complex bool expression outside the Package 3 operation set
must produce `UnsupportedFeatureError`.

---

# 28. Mutable locals across `if`

Supported:

```sec
let mut value := 1

if flag {
    value = 2
} else {
    value = 3
}

return value
```

The one `StorageID` is visible to all blocks where the source binding is in
scope.

Both branches use `storage.store`.

The merge block uses `storage.load`.

Because initialization occurs before the conditional branch, the initialization
dominates all branch and merge uses.

Package 3 does not support a mutable local that becomes initialized only on
different branches because uninitialized declarations are excluded.

---

# 29. SSA dominance verification

Package 3 verifier must perform CFG dominance validation.

A `ValueID` may be used when:

```text
it is a function parameter;
it is a parameter of the current block;
its defining operation occurs earlier in the same block;
or its defining block dominates the using block.
```

A value defined in only one branch does not dominate the merge block.

Invalid conceptual IR:

```text
^0:
    cond_br %cond, ^1, ^2

^1:
    %x = const.int 1
    br ^3

^2:
    br ^3

^3:
    return %x
```

Verifier must reject this.

To use branch-produced SSA values at a merge, use block parameters and pass
values on every incoming edge.

Package 3 builder need not generate such merges yet, but verifier support is
required.

---

# 30. Storage dominance and initialization verification

For each Package 3 storage identity:

```text
storage.declare must dominate every storage operation;
storage.init must dominate every storage.load;
storage.init must dominate every storage.store;
exactly one storage.init is permitted.
```

Because Package 3 excludes uninitialized declaration and move/reinitialization,
ordinary dominance is sufficient for the supported subset.

Do not claim this verifier is the final ownership/initialization analysis.

Later packages will extend it with path-sensitive state.

---

# 31. CFG verification

Extend `Verify(module)` with:

```text
entry block exists;
every referenced successor exists;
every block has exactly one terminator;
no operation follows terminator;
branch argument count matches block parameters;
branch argument types match block parameters;
conditional branch condition is bool;
all ValueID operands are dominated;
all StorageID uses have dominating declaration;
all Package 3 storage loads/stores have dominating initialization;
all FunctionIDs referenced by calls exist;
call category agrees with extern/non-extern target;
call argument count/types match;
call result arity/type matches;
Package 3 call argument action is copy-trivial;
```

Recommended additional invariant:

```text
all blocks in Package 3 builder output are reachable from function entry.
```

The generic verifier may either reject unreachable blocks now or expose a
separate strict Package 3 verification mode.

The builder must not intentionally emit unreachable blocks.

---

# 32. Sema facts are authoritative

Package 3 may require adding internal Sema result tables.

At minimum retain:

```text
resolved identifier binding;
resolved call kind;
resolved exact call target;
resolved expression type.
```

The Semantic IR builder must not:

```text
search overload sets;
choose a function by argument type;
resolve a name by source spelling;
guess whether a call is foreign;
guess parameter ownership;
guess a conversion;
redo definite assignment.
```

If a required resolved fact is absent after successful Sema, treat that as a
compiler implementation gap.

Do not compensate by duplicating Sema logic in the builder.

---

# 33. Builder behavior

Package 3 builder becomes block-oriented.

Recommended internal state:

```go
type functionBuilder struct {
    function        *Function
    currentBlock    *Block

    bindings        map[sema.BindingID]bindingLocation
    nextValueID     ValueID
    nextStorageID   StorageID
    nextBlockID     BlockID
}
```

where:

```go
type bindingLocation struct {
    Value   *ValueID
    Storage *StorageID
}
```

One and only one of `Value` or `Storage` is set.

Use an explicit scope stack if needed for source lexical scopes.

Do not key semantic bindings only by identifier string.

---

# 34. Printer additions

Extend the deterministic debug printer.

Recommended shape:

```text
semantic-ir 1

module "main" {
  ...

  func "main::Choose(bool)" Choose(
      %0: !bool [immediate]
  ) -> !int {
    storage {
      $1 "value" : !int mutable local-automatic
    }

    ^0:
      storage.declare $1
      %1 = const.int 1 : !int
      storage.init $1, %1
      cond_br %0, ^1(), ^2()

    ^1:
      %2 = const.int 2 : !int
      storage.store $1, %2
      br ^3()

    ^2:
      %3 = const.int 3 : !int
      storage.store $1, %3
      br ^3()

    ^3:
      %4 = storage.load $1 : !int
      return %4 : !int
  }
}
```

Call examples:

```text
%3 = call.direct "main::Identity(int)"(
    %2 [copy-trivial]
) : !int

call.direct "main::Noop()"() : void

%4 = call.foreign "platform::ReadCounter()"() : !int64
```

Exact whitespace is implementation-defined.

Ordering and semantic information are not.

---

# 35. `sec emit-ir`

No CLI syntax change is required.

Existing:

```bash
sec emit-ir <file.sec>
sec emit-ir <file.sec> -o <file.sir>
sec emit-ir <file.sec> --target <os-arch>
```

must now support Package 3 constructs.

The pipeline remains:

```text
parse
target selection/filtering
Sema
Semantic IR build
Semantic IR verify
Semantic IR print
stop
```

It still must not invoke:

```text
sec-mlir-opt
mlir-opt
mlir-translate
clang
LLVM codegen
legacy MLIR codegen
```

---

# 36. Required valid source tests

## V01 - mutable scalar

```sec
module main

fn Main() int {
    let mut value := 1
    value = 2
    return value
}
```

Expected:

```text
one StorageID;
one storage.declare;
one storage.init;
one storage.store;
one storage.load;
```

## V02 - assignment from parameter

```sec
module main

fn Main(next: int) int {
    let mut value := 1
    value = next
    return value
}
```

Expected:

`storage.store` consumes the parameter as a `copy-trivial` value use, not as a
move.

## V03 - direct call return

```sec
module main

fn One() int {
    return 1
}

fn Main() int {
    return One()
}
```

Expected:

```text
call.direct target is exact FunctionID main::One()
```

## V04 - overloaded direct target

```sec
module main

fn Identity(value: int) int {
    return value
}

fn Identity(value: bool) bool {
    return value
}

fn Main() int {
    return Identity(7)
}
```

Expected:

call target is the `int` overload recorded by Sema.

The builder must not choose it independently.

## V05 - nested call argument order

Use supported scalar functions:

```sec
module main

fn First() int {
    return 1
}

fn Second() int {
    return 2
}

fn PairCode(a: int, b: int) int {
    return a
}

fn Main() int {
    return PairCode(First(), Second())
}
```

Expected operation order in `Main`:

```text
call.direct First
call.direct Second
call.direct PairCode
```

## V06 - void call statement

```sec
module main

fn Noop() void {
    return
}

fn Main() void {
    Noop()
    return
}
```

Expected:

void `call.direct` with no result.

## V07 - if without else

```sec
module main

fn Main(flag: bool) int {
    let mut value := 1

    if flag {
        value = 2
    }

    return value
}
```

Expected:

```text
entry -> then/merge
then -> merge
merge -> return
```

## V08 - if/else

```sec
module main

fn Main(flag: bool) int {
    let mut value := 1

    if flag {
        value = 2
    } else {
        value = 3
    }

    return value
}
```

Expected four logical blocks:

```text
entry
then
else
merge
```

## V09 - both branches return

```sec
module main

fn Main(flag: bool) int {
    if flag {
        return 1
    } else {
        return 2
    }
}
```

Expected:

```text
entry
then
else
```

No fake reachable merge block.

## V10 - nested if

```sec
module main

fn Main(first: bool, second: bool) int {
    if first {
        if second {
            return 1
        }
        return 2
    }

    return 3
}
```

Expected valid explicit CFG with no implicit fallthrough.

## V11 - mutable storage across branches

Use V08.

Verifier must prove declaration and initialization dominate both stores and the
merge load.

---

# 37. Required unsupported source tests

## U01 - uninitialized mutable local

```sec
module main

fn Main() int {
    let mut value: int
    value = 1
    return value
}
```

Expected:

```text
semantic IR feature not implemented in package 3:
mutable local declaration without initializer
```

## U02 - standalone non-void call

```sec
module main

fn Value() int {
    return 1
}

fn Main() void {
    Value()
    return
}
```

Expected:

```text
semantic IR feature not implemented in package 3:
standalone non-void call/discard
```

## U03 - method call

A valid currently supported method call must produce an unsupported Package 3
IR error, not a fake DirectCall.

## U04 - reference argument

Any call requiring `ref` or `ref mut` is unsupported.

Expected feature identifies borrow/reference call semantics.

## U05 - move-only argument

Any call that Sema classifies as consuming/moving a move-only value is
unsupported.

Do not emit `copy-trivial`.

## U06 - string by-value call

Until string ownership/call semantics are explicitly represented, reject it.

## U07 - arithmetic condition

Example:

```sec
if value > 0 {
}
```

if comparison lowering is not otherwise implemented by an earlier package.

Expected:

unsupported Package 3 scalar/comparison operation.

Do not introduce an ad hoc comparison solely for this test.

---

# 38. Required verifier unit tests

## T01 - StorageID zero rejected

## T02 - duplicate StorageID rejected

## T03 - duplicate storage.declare rejected

## T04 - storage.init before storage.declare rejected

## T05 - duplicate storage.init rejected

## T06 - storage.load before init rejected

## T07 - storage.store before init rejected

## T08 - storage type mismatch on init rejected

## T09 - storage type mismatch on store rejected

## T10 - immutable storage store rejected

## T11 - branch to missing block rejected

## T12 - branch argument count mismatch rejected

## T13 - branch argument type mismatch rejected

## T14 - cond_br non-bool condition rejected

## T15 - operation after br rejected

## T16 - operation after cond_br rejected

## T17 - cross-block non-dominating SSA use rejected

Construct:

```text
entry -> left/right -> merge
value defined only in left
merge uses value directly
```

Expected failure.

## T18 - dominated cross-block SSA use accepted

Value defined in entry and used in both child blocks.

Expected success.

## T19 - block parameter merge accepted

Construct two predecessor values and pass them to one merge block parameter.

Expected success.

## T20 - call to unknown FunctionID rejected

## T21 - DirectCall to extern function rejected

Must use ForeignCall.

## T22 - ForeignCall to non-extern function rejected

## T23 - call argument count mismatch rejected

## T24 - call argument type mismatch rejected

## T25 - call result type mismatch rejected

## T26 - non-void call missing result rejected

## T27 - void call with result rejected

## T28 - unsupported argument action rejected in Package 3 strict verification

## T29 - storage declaration in one branch used in sibling/merge rejected

Dominance must catch this.

## T30 - deterministic CFG printer

Repeated formatting is byte-identical.

---

# 39. Required Sema integration tests

## S01 - resolved binding identity

Two uses of one source local return the same `BindingID`.

Two different locals return different IDs.

## S02 - parameter binding identity

Parameter uses map to the parameter's binding identity.

## S03 - overload call target retained

The exact selected overload can be queried after `Analyze`.

## S04 - call query is read-only

Calling `ResolvedCallTarget` does not mutate analysis state and does not run
overload resolution again.

## S05 - call kind retained

Extern call is reported as foreign.

Ordinary Sec call is reported as direct function.

Constructor/conversion/method is distinguishable and therefore rejectable by
Package 3.

---

# 40. Required CLI/integration tests

## C01 - emit mutable local

```bash
sec emit-ir testdata/semantic_ir/mutable_scalar.sec
```

Expected success.

## C02 - emit direct call

Expected exact FunctionID in output.

## C03 - emit overload call

Expected selected overload identity.

## C04 - emit if/else

Expected `cond_br`, `br` and deterministic block IDs.

## C05 - emit nested if

Expected verifier success.

## C06 - unsupported uninitialized mutable

Expected exit `4` and Package 3 feature message.

## C07 - unsupported non-void discard call

Expected exit `4`.

## C08 - no backend tools

Package 3 `emit-ir` succeeds when MLIR/LLVM tools are absent.

## C09 - Go regression

Run:

```bash
go test ./...
```

Expected success.

## C10 - legacy codegen regression

Existing:

```text
internal/codegen/llvm
internal/codegen/mlir
```

tests remain green.

No existing code-generation output is migrated in Package 3.

---

# 41. Testdata

Add focused files such as:

```text
testdata/semantic_ir/
    mutable_scalar.sec
    mutable_from_parameter.sec
    direct_call.sec
    overload_call.sec
    call_argument_order.sec
    void_call.sec
    if_without_else.sec
    if_else.sec
    if_all_return.sec
    nested_if.sec
    mutable_across_if.sec

    unsupported_uninitialized_mutable.sec
    unsupported_nonvoid_discard.sec
    unsupported_method_call.sec
    unsupported_reference_call.sec
    unsupported_string_call.sec
```

Keep fixtures small enough that the expected IR can be reviewed manually.

---

# 42. Architecture rules

These are non-negotiable.

```text
Mutable source variables map to Semantic IR storage, not fake mutable SSA.

StorageID is semantic storage identity, not a physical address.

Package 3 does not decide stack versus heap.

Package 3 storage replacement is permitted only for trivial types.

Calls retain exact Sema-selected targets.

Direct and foreign calls remain distinct.

The builder never reruns overload resolution.

The builder never resolves identifier bindings by spelling alone.

Call arguments preserve left-to-right evaluation order.

Calls with unresolved copy/move/borrow behavior are rejected.

No non-void call result is silently discarded.

If lowers to explicit CFG.

Blocks never fall through implicitly.

All CFG edges are explicit terminators.

Cross-block SSA uses obey dominance.

Merge values require block parameters when ordinary dominance is insufficient.

Storage declaration and initialization must dominate Package 3 uses.

Semantic IR verification succeeds before any later MLIR stage.

Legacy AST-to-LLVM/MLIR codegen remains temporary but unchanged.
```

---

# 43. Acceptance criteria

Package 3 is complete only when all are true:

```text
[ ] Package 2 remains green
[ ] StorageID exists and is distinct from ValueID
[ ] StorageID zero is invalid
[ ] local storage descriptors exist
[ ] storage.declare exists
[ ] storage.init exists
[ ] storage.load exists
[ ] storage.store exists
[ ] mutable initialized scalar locals lower through storage
[ ] uninitialized mutable locals are explicitly unsupported
[ ] named/complex storage is rejected when semantics cannot be preserved
[ ] Sema exposes stable resolved local/parameter binding identity
[ ] builder uses binding identity rather than spelling-based lookup
[ ] Sema exposes exact resolved call target
[ ] Sema exposes resolved call kind
[ ] DirectCall exists
[ ] ForeignCall exists
[ ] DirectCall and ForeignCall cannot be confused by verifier
[ ] call arguments carry copy-trivial action in supported subset
[ ] argument operations preserve left-to-right source order
[ ] void call statement is supported
[ ] standalone non-void call remains explicitly unsupported
[ ] branch terminator exists
[ ] conditional branch terminator exists
[ ] block parameters are verifiable
[ ] if without else lowers correctly
[ ] if/else lowers correctly
[ ] nested if lowers correctly
[ ] both-return if does not create fake reachable merge
[ ] mutable initialized storage works across if branches
[ ] cross-block SSA dominance verifier exists
[ ] invalid non-dominating use is rejected
[ ] storage declaration dominance is verified
[ ] storage initialization dominance is verified
[ ] deterministic printer includes storage/call/CFG data
[ ] sec emit-ir supports Package 3 programs
[ ] sec emit-ir invokes no MLIR/LLVM backend tools
[ ] go test ./... passes
[ ] legacy LLVM codegen tests pass
[ ] legacy MLIR codegen tests pass
[ ] no Package 4+ semantic placeholder operations are added
```

---

# 44. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. Package 2 status before implementation
3. files added
4. files modified
5. StorageID/storage API
6. Sema BindingID API
7. Sema resolved call API
8. supported Package 3 storage types
9. supported Package 3 call types
10. exact call kinds emitted
11. CFG operation types
12. dominance algorithm used
13. storage initialization verification approach
14. test commands
15. test results
16. unsupported constructs encountered
17. deviations from this package
18. issues that should alter Package 4 design
```

Do not silently broaden scope.

---

# 45. Package 4 boundary

Package 4 should implement the first real:

```text
Semantic IR -> Sec MLIR bridge
```

using only the already-verifiable scalar subset from Packages 2 and 3.

Package 4 should:

```text
extend rules/mlir/sec_mlir_dialect.md before adding operations;
add the minimum Sec MLIR operation/type surface required by the bridge;
import module/function/source provenance;
import Package 2 scalar and named types;
import constants;
import Package 3 local storage operations;
import DirectCall and ForeignCall distinctly;
import return;
import branch and conditional branch;
preserve FunctionID/type/layout metadata;
verify generated Sec MLIR;
add sec emit-sec-mlir or equivalent compiler dump path;
round-trip through sec-mlir-opt;
```

Package 4 must still reject:

```text
non-trivial ownership
copy/move/destruction
references/borrows
Result/try
defer/cleanup
aggregates
allocation
register/MMIO
concurrency
```

Those require later Semantic IR and dialect packages.

The important Package 4 invariant will be:

```text
valid Package 2/3 Semantic IR
    -> valid verified Sec MLIR

without reading AST or rerunning Sema.
```

# Sec MLIR Program - Implementation Package 2

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P2`  
Package title: `Semantic IR Core Foundation`  
Repository: `https://github.com/YoePro/sec`  
Repository branch: `main`  
Repository sync commit used for this package: `d48035c`  
Repository sync date: `2026-08-08`

Package 2 is a prerequisite package for the Sec MLIR bridge.

It does **not** add MLIR lowering yet.

The reason is architectural: at repository sync `d48035c`,
`rules/semantic_ir.txt` explicitly states that Semantic IR is still a design
target. Sec MLIR is required to consume validated Semantic IR, so implementing
the bridge before a real Semantic IR exists would recreate the forbidden
AST-to-backend shortcut.

This package establishes the smallest useful canonical Semantic IR and a real
`sec emit-ir` command.

---

# 1. Normative authority

Implementation must follow this authority chain:

```text
language/domain rulebooks
    ↓
rules/semantic_ir.txt
    ↓
rules/sec_mlir.md
    ↓
rules/sec_mlir_dialect.md
    ↓
implementation packages
    ↓
implementation
```

Package 2 must not redefine source-language semantics.

The package implements only a subset of the first Semantic IR milestone
described by `rules/semantic_ir.txt`.

The remainder of that milestone is intentionally deferred to Package 3 so that
each Codex task remains reviewable.

---

# 2. Repository facts relevant to this package

At sync `d48035c`:

- `internal/sema.Analyzer` owns resolved type and function information.
- `Analyzer.TypeOf(expr)` exposes expression type information.
- `Analyzer.Types()` and `Analyzer.Functions()` expose analyzed declarations.
- `Analyzer.Analyze(program)` currently performs complete frontend semantic
  validation.
- `cmd/compiler/main.go` currently parses and analyzes successfully but discards
  the `Analyzer` object in the normal helper path.
- `internal/codegen/llvm` and `internal/codegen/mlir` consume the AST directly.
- `rules/semantic_ir.txt` requires a dedicated Semantic IR package and explicitly
  says AST nodes must not be reused as Semantic IR nodes.
- `rules/semantic_ir.txt` specifies `sec emit-ir <file.sec>` as the initial dump
  command and requires it to stop after Semantic IR generation and verification.
- `rules/sec_mlir.md` requires the future MLIR layer to consume Semantic IR,
  not AST.

Package 2 must preserve the existing LLVM and legacy MLIR paths.

---

# 3. Package goal

After Package 2:

1. `internal/ir/semantic/` exists as a real compiler layer;
2. Semantic IR owns its own module, type, function, block, value and operation
   structures;
3. no Semantic IR node is an alias for an AST node;
4. no LLVM or MLIR type appears in Semantic IR;
5. a successfully analyzed simple Sec program can be converted to Semantic IR;
6. the Semantic IR builder consumes already-resolved Sema information;
7. Semantic IR has a dedicated verifier;
8. Semantic IR has a deterministic human-readable debug printer;
9. `sec emit-ir <file.sec>` parses, analyzes, builds, verifies and prints
   Semantic IR;
10. `sec emit-ir` requires neither MLIR nor LLVM binaries;
11. unsupported-but-valid source constructs fail explicitly at the Semantic IR
    builder boundary rather than being silently approximated;
12. all existing compiler tests and legacy code generation remain operational.

This package ends before general mutable storage, calls and branching.

---

# 4. Package boundary

## 4.1 In scope

Implement:

```text
Semantic IR package skeleton
Semantic IR version = 1
module identity
source-file table
canonical scalar type table
canonical named scalar types
function declarations
function definitions
extern function declarations
function symbol identity
function parameters
basic blocks
SSA-style value identities
ownership classification field
source locations
integer constants
boolean constants
string constants
decimal constants
floating-point constants
immutable local bindings whose initializer is supported
return operations
explicit block termination
Semantic IR builder
Semantic IR verifier
deterministic textual debug printer
sec emit-ir command
frontend helper that retains the successful Analyzer
focused unit/integration tests
full Go regression
```

## 4.2 Explicitly out of scope

Do not implement in Package 2:

```text
MLIR construction
Semantic IR -> Sec MLIR bridge
changes to the Package 1 Sec dialect
MLIR lowering
LLVM lowering changes
mutable local storage
storage allocation operations
load/store operations
assignment
compound assignment
function calls
method calls
indirect calls
if control flow
loops
switch
match
Result operations
Option operations
try
defer
cleanup
copy/move operations
destruction operations
borrow operations
reference operations
RawPtr operations
aggregate construction
struct operations
array operations
slice operations
enum operations
union operations
properties
interfaces
closures
allocation
arena operations
runtime checks
contracts
panic
register/MMIO operations
volatile operations
concurrency
ABI lowering
serialization/deserialization
a parser for the Semantic IR debug format
```

Do not create fake placeholder operations for these features.

Package 3 will extend Semantic IR with executable core operations and control
flow before Package 4 introduces the Semantic IR -> Sec MLIR bridge.

---

# 5. Required source layout

Create:

```text
internal/ir/semantic/
    doc.go
    version.go
    ids.go
    location.go
    ownership.go
    types.go
    module.go
    function.go
    block.go
    value.go
    operation.go
    constant.go
    builder.go
    verifier.go
    printer.go

    types_test.go
    builder_test.go
    verifier_test.go
    printer_test.go
```

The exact number of Go files may be adjusted when two files are trivially small,
but the package must remain internally separated by responsibility.

Compiler command tests belong under the existing:

```text
cmd/compiler/
```

Do not create Semantic IR below either codegen package.

---

# 6. Semantic IR version

Define:

```go
const Version uint32 = 1
```

The version belongs to the Semantic IR representation.

It is independent of:

```text
Sec source-language version
Sec MLIR dialect schema version
LLVM version
MLIR version
```

The printer must show the Semantic IR version.

No binary serialization is introduced in Package 2.

---

# 7. Core identifiers

Define distinct Go identifier types:

```go
type TypeID uint32
type FunctionID string
type BlockID uint32
type ValueID uint32
```

Do not use raw `int` interchangeably for these identities.

## 7.1 Type IDs

Type IDs are module-local canonical table identities.

`0` is invalid/reserved.

The first valid TypeID is `1`.

## 7.2 Block IDs

Block IDs are unique within a function.

`0` may be the entry block if the implementation consistently treats block IDs
as function-local identifiers.

## 7.3 Value IDs

Value IDs are unique within a function.

Use a deterministic monotonically increasing allocator.

The textual printer displays them as:

```text
%0
%1
%2
```

## 7.4 Function IDs

Function identity must be stable for one compilation and unique across overloads.

Package 2 canonical function ID:

```text
<module>::<name>(<canonical-parameter-type-list>)
```

Examples:

```text
main::Run()
main::Print(int)
main::Print(string)
math::Add(int,int)
```

This is an internal compilation identity only.

Package 2 does not promise cross-compilation stability.

Extern `@link_name` does not replace the internal FunctionID.

Source name and link name remain separate fields.

---

# 8. Source locations

Define a Semantic IR location independent from lexer tokens:

```go
type Location struct {
    File   string
    Line   int
    Column int
}
```

Required behavior:

- zero `Location{}` means synthesized/unknown when no better origin exists;
- user-originating operations copy file/line/column from their source token;
- Semantic IR must not store `lexer.Token`;
- Semantic IR must not store AST nodes for location purposes.

Provide one frontend conversion helper outside the core data model, for example:

```go
func LocationFromToken(token lexer.Token) Location
```

If this helper imports `lexer`, keep the dependency isolated in `builder.go` or
a frontend adapter file.

Core IR structures should depend only on Semantic IR types.

---

# 9. Ownership classification

Every Semantic IR value carries an ownership classification field as required by
`rules/semantic_ir.txt`.

Package 2 defines the complete classification enum, but only emits classifications
that are semantically known for the supported subset.

Define:

```go
type OwnershipClass string

const (
    OwnershipOwned             OwnershipClass = "owned"
    OwnershipSharedReference   OwnershipClass = "shared-reference"
    OwnershipMutableReference  OwnershipClass = "mutable-reference"
    OwnershipRawPointer        OwnershipClass = "raw-pointer"
    OwnershipImmediate         OwnershipClass = "immediate"
    OwnershipCompilerTemporary OwnershipClass = "compiler-temporary"
)
```

Package 2 builder behavior:

- primitive numeric constants: `immediate`
- boolean constants: `immediate`
- character/rune constants when encountered by supported paths: `immediate`
- string literal constants: `immediate` compiler/static constant value
- function parameters:
  - non-reference scalar parameters: `immediate`
  - `ref`: `shared-reference`
  - `ref mut`: `mutable-reference`
  - `RawPtr`: `raw-pointer`
  - unsupported owning/non-scalar parameter classes cause an explicit
    `UnsupportedFeatureError` in Package 2

Do not guess ownership for unsupported types.

---

# 10. Canonical type table

Semantic IR must not embed `sema.Type` values directly.

Create a canonical type table owned by the Semantic IR module.

Recommended model:

```go
type TypeKind string

type Type struct {
    Kind       TypeKind
    Name       string
    Module     string
    Identity   string
    Base       TypeID

    Signed     bool
    BitWidth   uint16
    TargetSize bool
}
```

The exact field arrangement may differ, but it must support the requirements
below without references to AST, Sema, MLIR or LLVM objects.

## 10.1 Package 2 builtin kinds

Support these active scalar types when Sema produces them:

```text
void
never

bool
byte
char
rune
string
decimal

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
```

Do not activate planned source types that the current Sema does not yet support,
such as `int128`, `int256`, `uint128`, `uint256`, merely because the long-term
rulebook mentions them.

## 10.2 Target-sized types

`int` and `uint` must retain:

```text
TargetSize = true
```

Fixed-width types must retain their declared width.

Package 2 does not resolve the final machine width of target-sized types.

## 10.3 Named scalar types

Support named scalar types whose resolved base type is already one of the
Package 2 scalar types.

A named type record must preserve:

```text
source name
module
opaque semantic identity
base TypeID
```

Canonical identity for Package 2:

```text
<module>::<name>
```

A named type and its base type are different TypeIDs.

Two different named identities over the same base are different TypeIDs.

Do not erase contracts or unit information silently.

If a named type carries contracts or unit semantics that Package 2 cannot yet
represent completely, Semantic IR generation must return
`UnsupportedFeatureError`.

## 10.4 Distinct types

The long-term Semantic IR rules require distinct types, but the current frontend
does not expose a separate, reliable `sema.TypeKind` distinction that Package 2
can consume without inventing semantics.

Therefore Package 2 does **not** synthesize `distinct` from heuristics.

Add it only in a later package when the frontend exposes it unambiguously.

## 10.5 Type interning

The type table must intern structurally identical supported types.

Required properties:

```text
same builtin type -> same TypeID
same named identity + same base -> same TypeID
different named identity -> different TypeID
named type != base type
```

The table must provide safe lookup and return an error for invalid IDs.

---

# 11. Module representation

Recommended structure:

```go
type Module struct {
    Version     uint32
    Identity    string
    SourceFiles []string
    Types       *TypeTable
    Functions   []*Function
}
```

Requirements:

- `Version` must equal `semantic.Version`;
- module identity comes from the validated Sec module declaration;
- source files are deterministic, unique and sorted for printing;
- functions are stored in deterministic source order or a documented stable
  order;
- no LLVM module or MLIR module is referenced.

Imported core/stdlib source may exist in the combined AST.

Package 2 printer must make it possible to distinguish the requested user's
module from imported implementation functions.

Recommended builder rule:

- build the requested module's declarations and definitions;
- include an extern/import declaration only when the requested module directly
  needs it for a supported construct;
- because Package 2 has no calls yet, do not dump the entire injected core
  library simply because it was present during Sema.

This avoids unstable and excessively large `emit-ir` output.

---

# 12. Function representation

Define a Semantic IR function independent from `ast.FunctionDeclaration` and
`sema.Function`.

Recommended structure:

```go
type Function struct {
    ID          FunctionID
    Name        string
    LinkName    string
    Parameters  []Parameter
    ReturnType  TypeID
    Unsafe      bool
    Extern      bool
    ABI         string
    Entry       BlockID
    Blocks      []*Block
    Location    Location
}
```

A parameter contains at least:

```go
type Parameter struct {
    Name      string
    Value     Value
    Location  Location
}
```

A function declaration has no body blocks.

A function definition has at least one block.

Extern functions are declarations and must not have Sec body blocks.

## 12.1 Function metadata

Preserve from resolved Sema where available:

```text
module
source name
link name
extern status
ABI/calling convention
parameter resolved types
parameter reference mode
return resolved type
source location
```

Package 2 does not yet represent:

```text
allocation effects
cleanup
generic instantiation metadata
full ownership transfer modes
interface receiver metadata
```

If such metadata would affect the semantics of a function used by the supported
builder subset, fail explicitly rather than discard it.

---

# 13. Block representation

Define:

```go
type Block struct {
    ID         BlockID
    Parameters []Value
    Operations []Operation
}
```

Package 2 requires exactly one entry block for supported function definitions.

Multiple block data structures are allowed by the IR model because later
packages require them, but the Package 2 source builder does not create
multi-block control flow.

Every non-extern function block must terminate explicitly.

Package 2 terminator:

```text
return
```

---

# 14. Values

Recommended structure:

```go
type Value struct {
    ID        ValueID
    Type      TypeID
    Ownership OwnershipClass
    Location  Location
}
```

Values never store an AST expression.

Each value has exactly one definition/origin:

```text
function parameter
block parameter
operation result
```

Package 2 verifier must reject:

- duplicate ValueID in a function;
- result using invalid TypeID;
- reference to unknown ValueID;
- use before definition within a block;
- inconsistent result type.

---

# 15. Package 2 operations

Define an internal `Operation` interface or a closed tagged representation.

The representation must expose:

```text
operation kind
location
result values
operand value IDs
terminator status
```

Package 2 implements only these operation families.

## 15.1 Constant operations

Required semantic operation kinds:

```text
const.int
const.bool
const.string
const.decimal
const.float
```

It is acceptable for one Go `ConstantOp` struct to contain a `ConstantKind`
field rather than five Go structs.

### Integer constants

Do not store integer constants in `int`, `int64`, or `uint64`.

Use arbitrary precision:

```go
*big.Int
```

or an immutable equivalent representation.

The source lexeme may additionally be retained.

### Decimal constants

Decimal constants must be constructed from their source lexeme without binary
floating-point conversion.

Recommended representation:

```go
type DecimalConstant struct {
    Coefficient *big.Int
    Scale       uint32
    Lexeme      string
}
```

Examples:

```text
123.45 -> coefficient 12345, scale 2
0.10   -> coefficient 10, scale 2
```

Do not normalize away the original scale in Package 2.

### Floating-point constants

Preserve the source lexeme and resolved Sec floating type.

Do not parse through host `float64` if doing so would discard source precision
needed for later correct lowering.

### String constants

Store the decoded semantic string value.

Retaining the source lexeme for diagnostics is optional.

## 15.2 Immutable local bindings

Package 2 supports immutable local `let` declarations only when:

- they have an initializer;
- the initializer is a supported constant expression or already-defined
  parameter/immutable SSA value;
- no storage semantics are required.

The local name is a builder-time binding only.

Example:

```sec
let answer := 42
return answer
```

may become:

```text
%0 = const.int 42 : int
return %0
```

There is no `let` operation in Semantic IR.

Do not lower mutable `let mut` as SSA in Package 2.

Mutable locals require explicit storage semantics and belong to Package 3.

## 15.3 Return

Define:

```text
return
return %value
```

Requirements:

- void function: no operand;
- non-void function: exactly one operand;
- operand type exactly matches resolved Semantic IR return type;
- the return is the final operation in the block;
- no operation may appear after a return.

Package 2 does not introduce multiple return values.

---

# 16. Builder boundary

Create an explicit builder entry point.

Recommended API:

```go
func Build(program *ast.Program, analyzer *sema.Analyzer, options BuildOptions) (*Module, error)
```

`BuildOptions` should contain only information required for deterministic
module construction, for example:

```go
type BuildOptions struct {
    RequestedModule string
    SourceFiles     []string
}
```

Do not pass LLVM triples or MLIR objects merely because later stages need them.

## 16.1 Preconditions

`Build` assumes:

- parsing succeeded;
- target filtering succeeded;
- core/stdlib source resolution required for Sema has completed;
- `Analyzer.Analyze(program)` completed with zero errors.

The builder must not run a second semantic analysis.

The builder must not call name resolution itself.

The builder must consume Sema's resolved facts.

## 16.2 Sema read-only API

Add a read-only method that never invokes fallback inference:

```go
func (a *Analyzer) ResolvedTypeOf(expr ast.Expression) (Type, bool)
```

Required behavior:

- return only a type already recorded during successful analysis;
- do not call `inferExpression`;
- do not mutate analyzer state.

Keep existing:

```go
TypeOf(expr)
```

unchanged for current users.

If needed, add a second focused lookup for function declarations:

```go
func (a *Analyzer) ResolvedFunctionForDeclaration(
    decl *ast.FunctionDeclaration,
) (Function, bool)
```

This lookup must match an already-registered function declaration and must not
re-run overload resolution.

Do not expose Analyzer internal maps directly.

## 16.3 Unsupported features

Define a typed error:

```go
type UnsupportedFeatureError struct {
    Feature  string
    Location Location
}
```

A semantically valid program using a source construct not supported by Package 2
must fail with this error.

Examples:

```text
mutable local storage
function call
if statement
aggregate construction
try expression
```

Never approximate these constructs.

Never call the legacy AST-to-LLVM or AST-to-MLIR codegen as a fallback.

---

# 17. Compiler frontend refactor

Current compiler helpers discard the successful `Analyzer`.

Add a non-breaking helper that retains it.

Recommended shape:

```go
type analyzedProgram struct {
    Program  *ast.Program
    Analyzer *sema.Analyzer
}
```

and a helper equivalent to:

```go
func parseAndAnalyzeSourceForTargetWithAnalyzer(
    input string,
    sourceFile string,
    target CompilerTarget,
) analyzedProgram
```

Existing helpers may wrap this and return only `Program` so current callers do
not change behavior.

Requirements:

- no duplicate Sema pass;
- existing diagnostics remain unchanged;
- existing `emit-llvm`, legacy `emit-mlir`, `build`, `sema`, LSP tests and
  frontend tests must remain green;
- the new helper exists to establish the canonical frontend -> Semantic IR
  boundary.

---

# 18. Semantic IR verifier

Create:

```go
func Verify(module *Module) error
```

Verifier failures represent compiler bugs.

Do not turn verifier failures into source-language diagnostics.

The verifier must check at least the Package 2 invariants below.

## 18.1 Module invariants

```text
module is non-nil
version is supported
module identity is non-empty
type table exists
FunctionIDs are unique
source-file list contains no duplicates
```

## 18.2 Type invariants

```text
TypeID zero is invalid
all referenced TypeIDs exist
canonical type records are internally consistent
named type identity is non-empty
named type base exists
named type base is not itself cyclic through named base references
target-sized flag is valid only where defined
fixed numeric widths agree with type kind
```

## 18.3 Function invariants

```text
function ID is non-empty
source name is non-empty
parameter ValueIDs are unique
parameter types exist
return type exists
extern function has no body blocks
non-extern function has an entry block
entry block exists
BlockIDs are unique
```

## 18.4 Value and operation invariants

```text
ValueIDs are unique per function
every value has valid type
every value has valid ownership classification
every operand references a defined value
use-before-definition in the same block is invalid
constant result type matches constant kind
return arity matches function return type
return operand type matches function return type
return is a terminator
no operation follows a terminator
every reachable Package 2 block terminates
```

Package 2 has one source-built block, so dominance beyond same-block order is not
required yet.

The IR model must not make later multi-block verification impossible.

---

# 19. Deterministic debug printer

Create a printer API such as:

```go
func Format(module *Module) string
```

or:

```go
func Write(w io.Writer, module *Module) error
```

The output is a compiler debug format.

It is **not** Sec syntax.

It is **not** required to be parseable.

It must be deterministic.

Canonical Package 2 shape:

```text
semantic-ir 1

module "main" {
  sources {
    "sample.sec"
  }

  types {
    !1 = int target-sized
  }

  func "main::Main()" Main() -> !1 loc("sample.sec":3:1) {
    ^0:
      %0 = const.int 42 : !1 loc("sample.sec":4:12)
      return %0 : !1 loc("sample.sec":4:5)
  }
}
```

The exact whitespace may differ, but the following are mandatory:

```text
IR version
module identity
source files
canonical type table
function internal identity
function source name
parameter values and types
return type
block IDs
value IDs
operation kind
constant value
ownership classification when it is not obvious from the operation
source locations
```

Recommended: print ownership on parameter declarations and operation results in
a compact form.

Sort any map-derived output.

Never rely on Go map iteration order.

---

# 20. `sec emit-ir`

Add command:

```bash
sec emit-ir <file.sec>
```

Also support:

```bash
sec emit-ir <file.sec> -o <file.sir>
sec emit-ir <file.sec> -o -
```

Default output when `-o` is omitted:

```text
stdout
```

Optional target selection should follow current compiler conventions:

```bash
sec emit-ir <file.sec> --target <os-arch>
```

Execution order:

```text
read/collect source
parse
target validation
core/stdlib resolution
Sema
if Sema succeeds:
    Build Semantic IR
    Verify Semantic IR
    print/write Semantic IR
stop
```

The command must not invoke:

```text
mlir-opt
sec-mlir-opt
mlir-translate
clang
LLVM codegen
legacy MLIR codegen
```

Exit behavior:

```text
1  command/input/target error
2  parser error
3  Sema error
4  Semantic IR build or verifier internal error
0  success
```

Use existing compiler diagnostic conventions where practical.

A Package 2 unsupported source construct should produce a clear
`semantic IR error:` message and exit `4`.

---

# 21. Required source-level supported examples

All examples below are valid Package 2 `emit-ir` inputs.

## E01 - constant return

```sec
module main

fn Answer() int {
    return 42
}
```

## E02 - parameter return

```sec
module main

fn Identity(value: int) int {
    return value
}
```

## E03 - immutable local constant

```sec
module main

fn Answer() int {
    let value := 42
    return value
}
```

## E04 - boolean

```sec
module main

fn Ready() bool {
    return true
}
```

## E05 - decimal

```sec
module main

fn Price() decimal {
    return 12.50
}
```

The IR must preserve coefficient/scale information and source lexeme sufficiently
to distinguish the original decimal representation.

## E06 - float context

```sec
module main

fn Ratio() float64 {
    return 1.25
}
```

The resolved result must be `float64`, not `decimal`.

## E07 - void

```sec
module main

fn Noop() void {
    return
}
```

## E08 - overloaded functions

```sec
module main

fn Value(value: int) int {
    return value
}

fn Value(value: bool) bool {
    return value
}
```

The two functions must have different FunctionIDs.

---

# 22. Required rejection examples

These are rejected by Package 2 Semantic IR generation even when Sema accepts
them.

## R01 - mutable local

```sec
module main

fn Example() int {
    let mut value := 1
    value = 2
    return value
}
```

Expected Package 2 IR error:

```text
semantic IR feature not implemented in package 2: mutable local storage
```

## R02 - function call

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
semantic IR feature not implemented in package 2: function call
```

## R03 - if

```sec
module main

fn Choose(flag: bool) int {
    if flag {
        return 1
    }
    return 2
}
```

Expected:

```text
semantic IR feature not implemented in package 2: if control flow
```

These are implementation-boundary failures, not Sec language errors.

---

# 23. Required tests

## T01 - Type interning

Build repeated builtin `int`.

Expected:

```text
same TypeID
```

Build two different named scalar identities over `int`.

Expected:

```text
different TypeIDs
```

Named type and base `int` must differ.

## T02 - Invalid TypeID

Verifier receives an operation/value referring to TypeID `0` or a missing ID.

Expected:

```text
verification failure
```

## T03 - Function overload identity

Create:

```text
main::Value(int)
main::Value(bool)
```

Expected:

```text
different FunctionIDs
```

Duplicate identical FunctionID must be rejected by verifier.

## T04 - Parameter values

Build `Identity(value: int) int`.

Expected:

- parameter has ValueID;
- parameter type is canonical int TypeID;
- return references the parameter ValueID;
- no extra copy/move is invented.

## T05 - Integer constant exactness

Directly construct/test a Semantic IR integer constant greater than 64 bits.

Expected:

- exact value preserved;
- no host integer truncation.

This is a Semantic IR unit test and does not require the current Sec frontend to
accept that literal as an active builtin integer type.

## T06 - Decimal exactness

Input or direct constant test:

```text
0.10
```

Expected:

```text
coefficient = 10
scale = 2
lexeme = "0.10"
```

No binary floating-point conversion.

## T07 - Float resolved context

Source:

```sec
fn Ratio() float64 {
    return 1.25
}
```

Expected:

- constant/result type is float64;
- it is not emitted as decimal.

## T08 - Immutable local SSA binding

Source:

```sec
let value := 42
return value
```

Expected:

- one constant result value;
- return uses that same ValueID;
- no storage operation exists.

## T09 - Mutable local rejected by builder

Expected:

- typed `UnsupportedFeatureError`;
- feature identifies mutable local storage.

## T10 - Call rejected by builder

Expected typed `UnsupportedFeatureError`.

## T11 - If rejected by builder

Expected typed `UnsupportedFeatureError`.

## T12 - Missing terminator verifier failure

Construct function block with no return.

Expected verifier failure.

## T13 - Wrong return type verifier failure

Construct function returning bool but return int ValueID.

Expected verifier failure.

## T14 - Operation after terminator

Construct:

```text
return
const
```

Expected verifier failure.

## T15 - Undefined operand

Return an unknown ValueID.

Expected verifier failure.

## T16 - Duplicate ValueID

Expected verifier failure.

## T17 - Source location propagation

Compile a file with known function and literal lines.

Expected:

- function Location matches declaration;
- constant Location matches literal;
- return Location matches return token.

## T18 - Deterministic printer

Format the same module repeatedly.

Expected byte-for-byte identical output.

Construct equivalent type/function maps in different insertion orders where
maps are involved.

Expected identical output.

## T19 - `sec emit-ir` success

Build compiler and run:

```bash
sec emit-ir testdata/semantic_ir_basic.sec
```

Expected:

- exit 0;
- output starts with `semantic-ir 1`;
- contains requested module and supported function;
- no MLIR/LLVM tool is needed.

## T20 - `sec emit-ir -o`

Expected file output equals stdout form for the same module except for no
environment-dependent differences.

## T21 - `emit-ir` does not invoke MLIR/LLVM

Run command in a test environment where:

```text
mlir-opt
sec-mlir-opt
mlir-translate
clang
```

are unavailable.

Expected:

```text
emit-ir still succeeds
```

## T22 - Analyzer retained without duplicate analysis

Add a focused test around the new frontend helper.

Expected:

- one Analyzer instance is returned;
- Semantic IR builder uses it;
- no second `Analyzer.Analyze` invocation is necessary.

Implementation may expose a test seam rather than add production counters.

## T23 - Existing CLI regression

Existing `cmd/compiler` tests pass.

## T24 - Full Go regression

Run:

```bash
go test ./...
```

Expected:

```text
PASS
```

## T25 - Legacy codegen regression

Tests under:

```text
internal/codegen/llvm
internal/codegen/mlir
```

remain green.

No expected legacy output is rewritten to Semantic IR in this package.

---

# 24. Testdata

Add focused source files, for example:

```text
testdata/semantic_ir/
    basic_return.sec
    parameter_return.sec
    immutable_local.sec
    bool_return.sec
    decimal_return.sec
    float_return.sec
    void_return.sec
    overloads.sec
    unsupported_mutable.sec
    unsupported_call.sec
    unsupported_if.sec
```

Keep these tests intentionally small.

Do not reuse complex language-wide fixtures as the primary Semantic IR tests.

---

# 25. Error policy

Three failure classes must remain separate.

## Source errors

Parser/Sema diagnostics are ordinary user diagnostics.

They retain their existing IDs/severity behavior.

## Unsupported Package 2 IR feature

A valid Sec construct not yet represented by Package 2 produces
`UnsupportedFeatureError`.

This is temporary implementation incompleteness.

It must be clearly labeled as Semantic IR implementation support, not as invalid
Sec source.

## Verifier failure

A malformed Semantic IR built after successful Sema is an internal compiler
error.

CLI prefix should make this clear, for example:

```text
semantic IR verification error: ...
```

Do not silently continue to MLIR/LLVM.

---

# 26. Non-negotiable architecture rules

Package 2 must satisfy all of these:

```text
AST nodes are not Semantic IR nodes.
sema.Type is not the Semantic IR Type representation.
lexer.Token is not stored in Semantic IR.
LLVM types do not appear in Semantic IR.
MLIR types do not appear in Semantic IR.
Semantic IR construction occurs only after successful Sema.
Semantic IR does not redo source name lookup.
Semantic IR does not redo overload resolution.
Semantic IR does not infer unresolved source semantics.
unsupported semantics are rejected explicitly.
verification runs before printing from sec emit-ir.
legacy direct codegen is not used as a fallback.
```

---

# 27. Acceptance criteria

Package 2 is complete only when:

```text
[ ] internal/ir/semantic exists as an independent package
[ ] Semantic IR Version == 1
[ ] canonical scalar TypeTable exists
[ ] TypeID 0 is invalid
[ ] supported named scalar types preserve identity
[ ] function IDs distinguish overloads
[ ] function declarations/definitions exist independently of AST/Sema structs
[ ] parameters are explicit Semantic IR values
[ ] basic blocks exist
[ ] constants are explicit operations
[ ] integer constants are arbitrary precision
[ ] decimal constants are exact and lexeme-based
[ ] immutable supported locals become SSA bindings without fake storage
[ ] return is an explicit terminator
[ ] source locations contain no lexer.Token
[ ] ownership classification exists on every value
[ ] builder consumes a successful Analyzer
[ ] builder does not call Analyze again
[ ] ResolvedTypeOf or equivalent read-only Sema API exists
[ ] unsupported Package 2 constructs fail explicitly
[ ] dedicated Verify(module) exists
[ ] verifier catches all Package 2 invalid cases
[ ] deterministic debug printer exists
[ ] sec emit-ir exists
[ ] sec emit-ir needs no MLIR/LLVM installation
[ ] existing emit-llvm behavior is unchanged
[ ] existing legacy emit-mlir behavior is unchanged
[ ] existing build behavior is unchanged
[ ] go test ./... passes
```

---

# 28. Required implementation report

At completion Codex must report:

```text
1. repository HEAD implemented against
2. files added
3. files modified
4. Semantic IR public API added
5. Sema read-only API changes
6. compiler helper refactor performed
7. exact supported Package 2 source subset
8. exact unsupported-feature diagnostics
9. test commands
10. test results
11. any deviations from this package
12. any discovered semantic information that Sema does not yet expose cleanly
13. recommended Package 3 follow-up
```

Any deviation that changes Semantic IR meaning must be treated as a design
question rather than silently implemented.

---

# 29. Package 3 boundary

Package 3 should extend the Semantic IR core to complete the first executable
milestone from `rules/semantic_ir.txt`.

Intended Package 3 scope:

```text
explicit local storage
initialization
load/store
assignment
resolved direct calls
argument evaluation order
call ownership-action metadata needed by current Sema
branch operation
conditional branch
if lowering
multi-block verification
basic CFG validation
source locations on all new operations
emit-ir tests for these constructs
```

Still defer:

```text
copy/move/destruction
references
Result/try
defer/cleanup
aggregates
allocation
registers
concurrency
```

After Package 3 is green, Package 4 should implement the first
Semantic IR -> Sec MLIR bridge against the Package 1 dialect foundation.

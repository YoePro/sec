# Sec MLIR Dialect - Implementation Package 1

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-DIALECT-P1`  
Package title: `Dialect Foundation`  
Repository: `https://github.com/YoePro/sec`  
Repository branch: `main`  
Repository sync commit used for this package: `d48035c`  
Repository sync date: `2026-08-08`

This package is intentionally small enough to be implemented and reviewed as one
coherent Codex task.

It establishes the Sec MLIR dialect infrastructure and the first stable
representation primitives. It does **not** migrate the compiler to Sec MLIR yet.

---

# 1. Normative authority

Implementation must follow the existing Sec authority chain:

```text
language/domain rulebooks
    ↓
rules/semantic_ir.txt
    ↓
rules/sec_mlir.md
    ↓
rules/sec_mlir_dialect.md
    ↓
implementation
```

At repository sync `d48035c`, `rules/sec_mlir.md` already requires
`rules/sec_mlir_dialect.md`, but that detailed dialect rulebook is not present in
the repository.

Package 1 therefore includes adding the supplied `sec_mlir_dialect.md` as:

```text
rules/sec_mlir_dialect.md
```

The supplied rulebook defines only the Package 1 surface and leaves later
operations explicitly unspecified.

Codex must not invent additional Sec semantics.

---

# 2. Package goal

After Package 1:

1. the `sec` dialect exists as a real MLIR dialect;
2. it is defined declaratively with TableGen/ODS;
3. a Sec-aware MLIR driver can load and verify the dialect;
4. basic Sec type identity can survive MLIR parsing and printing;
5. common identity/layout metadata can be represented without custom ad-hoc
   encoding;
6. source locations use normal MLIR locations;
7. CMake can build the dialect against an installed/built MLIR tree;
8. automated tests prove registration, parsing, printing, verification and
   rejection of malformed Sec dialect types;
9. the existing Go compiler and legacy AST-to-LLVM-dialect generator continue
   to work unchanged;
10. no direct migration to the new Sec MLIR pipeline is attempted yet.

This is the completion boundary of Package 1.

---

# 3. Existing repository state that must be respected

The repository currently contains:

```text
internal/mlir/toolchain.go
internal/codegen/mlir/generator.go
internal/codegen/mlir/generator_test.go
```

The current `internal/codegen/mlir/generator.go` is a legacy path. It emits
LLVM-dialect MLIR directly from the AST.

The target architecture defined by `rules/sec_mlir.md` is instead:

```text
Sec source
    ↓
AST
    ↓
Sema
    ↓
Semantic IR
    ↓
Sec MLIR
    ↓
standard MLIR dialects
    ↓
LLVM dialect
    ↓
LLVM IR
```

Package 1 must **not** rewrite `internal/codegen/mlir/generator.go`.

Package 1 must **not** route normal compilation through the new dialect yet.

The legacy path may remain operational during migration, as permitted by
`rules/sec_mlir.md`.

---

# 4. Package boundary

## 4.1 In scope

Implement all of the following:

```text
Sec dialect registration
TableGen/ODS setup
out-of-tree CMake integration
generated dialect/type declarations
generated dialect/type definitions
Sec-aware MLIR test/utility driver
custom Sec named type
custom Sec distinct type
common Sec metadata key conventions
source-location policy
dialect version marker policy
basic dialect verification
parser/printer round-trip tests
negative verifier/parser tests
C++ unit or lit tests as appropriate
Go regression tests
documentation generation target if practical
```

## 4.2 Explicitly out of scope

Do not implement any of the following in Package 1:

```text
Semantic IR -> Sec MLIR importer
Sec MLIR -> standard MLIR lowering
Sec MLIR -> LLVM lowering
ownership operations
copy/move operations
destruction
cleanup regions
defer operations
reference operations
RawPtr operations
storage-domain operations
generation/epoch operations
bounds checks
contract checks
panic lowering
Result operations
Option operations
try propagation
struct operations
array operations
slice operations
enum operations
union operations
interface dispatch
property operations
allocation operations
arena operations
string operations
register/MMIO operations
volatile operations
concurrency operations
ABI lowering
optimizer passes
canonicalization beyond what is necessary for Package 1
replacement of the existing Go MLIR generator
```

Do not add placeholder semantic operations for later packages.

---

# 5. Implementation layout

Create a new MLIR implementation subtree at repository root:

```text
mlir/
    CMakeLists.txt

    include/sec/Dialect/Sec/
        CMakeLists.txt
        SecDialect.td
        SecTypes.td
        SecDialect.h
        SecTypes.h

    lib/Dialect/Sec/
        CMakeLists.txt
        SecDialect.cpp
        SecTypes.cpp

    tools/sec-mlir-opt/
        CMakeLists.txt
        sec-mlir-opt.cpp

    test/
        CMakeLists.txt
        lit.cfg.py
        lit.site.cfg.py.in

        Dialect/Sec/
            dialect-loading.mlir
            named-type-roundtrip.mlir
            distinct-type-roundtrip.mlir
            metadata-roundtrip.mlir
            invalid-named-type.mlir
            invalid-distinct-type.mlir
```

The exact split of generated `.inc` includes may follow current MLIR best
practice, but the logical organization above must be preserved.

Do not place the C++ dialect implementation inside `internal/`.

The Go tree remains the Sec frontend/tooling tree.

The new `mlir/` tree is the C++ MLIR layer.

---

# 6. Build integration

## 6.1 Build model

Implement Sec MLIR as an out-of-tree MLIR project.

Configuration must accept an existing MLIR installation/build through
`MLIR_DIR`.

Example supported configuration:

```bash
cmake -S mlir -B build/sec-mlir -G Ninja \
    -DMLIR_DIR=/path/to/llvm-project/build/lib/cmake/mlir \
    -DLLVM_DIR=/path/to/llvm-project/build/lib/cmake/llvm
```

Build:

```bash
cmake --build build/sec-mlir
```

Test:

```bash
cmake --build build/sec-mlir --target check-sec-mlir
```

Do not vendor LLVM or MLIR into the Sec repository in this package.

Do not hard-code one local developer path such as `~/mlir/...` in CMake.

## 6.2 Required CMake behavior

The top-level MLIR CMake project must:

- locate LLVM and MLIR through their CMake packages;
- add the MLIR CMake module path;
- include the standard MLIR CMake helpers;
- configure generated headers into the build tree;
- build the Sec dialect library;
- build `sec-mlir-opt`;
- configure lit tests;
- expose `check-sec-mlir`.

Use current upstream MLIR CMake helpers such as `add_mlir_dialect`,
`mlir_tablegen`, `add_mlir_library`, and MLIR lit integration where appropriate.

Do not copy generated TableGen output into source control.

---

# 7. Dialect definition

## 7.1 Namespace

MLIR dialect namespace:

```text
sec
```

C++ namespace:

```text
::sec
```

The dialect must be non-extensible in Package 1.

Do not enable runtime-defined Sec operations or types.

## 7.2 Version

Package 1 defines dialect schema version:

```text
1
```

The canonical module marker is a normal MLIR integer attribute:

```mlir
sec.dialect_version = 1 : i32
```

The Sec-aware driver must accept modules without this marker for hand-written
unit tests, but any future compiler-generated Sec MLIR module will be required
to carry it.

Package 1 must document the version marker but must not add migration logic.

---

# 8. Package 1 Sec types

Package 1 implements exactly two custom Sec types.

## 8.1 Named type

Canonical textual form:

```mlir
!sec.named<"type-id", base-type>
```

Example:

```mlir
!sec.named<"main::Speed", i64>
```

Meaning:

- preserves Sec named-type identity;
- retains the already-resolved underlying representation type;
- does not authorize implicit conversion to or from the base type;
- does not perform contract validation;
- does not perform unit conversion;
- does not determine ABI layout;
- does not imply ownership.

Required parameters:

```text
identity: StringAttr
base: Type
```

Verifier requirements:

- identity must not be empty;
- base type must exist;
- base type must not be `NoneType`;
- the type must round-trip through MLIR parse/print.

Do not assign semantics based on the spelling of `identity`.

## 8.2 Distinct type

Canonical textual form:

```mlir
!sec.distinct<"type-id", base-type>
```

Example:

```mlir
!sec.distinct<"main::CustomerID", i64>
```

Meaning:

- preserves complete Sec distinct-type identity;
- retains the already-resolved underlying representation type;
- does not authorize implicit conversion to or from the base type;
- does not imply ownership;
- does not determine ABI layout.

Required parameters:

```text
identity: StringAttr
base: Type
```

Verifier requirements:

- identity must not be empty;
- base type must exist;
- base type must not be `NoneType`;
- the type must round-trip through MLIR parse/print.

## 8.3 No other Sec types in Package 1

Do not add:

```text
!sec.ref
!sec.ref_mut
!sec.raw_ptr
!sec.result
!sec.option
!sec.string
!sec.decimal
!sec.array
!sec.slice
!sec.struct
!sec.enum
!sec.union
!sec.interface
!sec.register
!sec.owned
```

Those belong to later packages.

---

# 9. Common metadata conventions

Package 1 uses normal MLIR attributes for shared metadata rather than creating
custom Sec attribute classes prematurely.

The following attribute names are reserved:

```text
sec.symbol_id
sec.type_id
sec.layout_ref
sec.synthesized
sec.dialect_version
```

Expected value types:

```text
sec.symbol_id       StringAttr
sec.type_id         StringAttr
sec.layout_ref      StringAttr
sec.synthesized     BoolAttr
sec.dialect_version IntegerAttr(i32)
```

Example:

```mlir
func.func @example(
    %value: !sec.named<"main::Speed", i64>
) -> !sec.named<"main::Speed", i64>
attributes {
    sec.symbol_id = "main::example#0",
    sec.layout_ref = "layout:main::Speed"
} {
    return %value : !sec.named<"main::Speed", i64>
}
```

These strings are opaque compiler identities.

Package 1 must not define their future cross-compilation stability format.

That format belongs to Semantic IR and later importer work.

---

# 10. Source provenance

Use MLIR `Location` as the primary source-location representation.

Package 1 must support and preserve normal locations such as:

```mlir
loc("example.sec":12:8)
```

Do not create a duplicate Sec-specific file/line/column attribute.

For compiler-synthesized objects, reserve:

```text
sec.synthesized = true
```

Future packages may add richer origin metadata when required, but Package 1 must
not invent it.

Parser/printer tests must demonstrate that a Sec type used in an operation with
a source location preserves the location through a round trip.

---

# 11. Sec-aware MLIR driver

Create:

```text
sec-mlir-opt
```

The tool exists because stock `mlir-opt` does not know the Sec dialect unless the
dialect is linked and registered.

The driver must:

- register the Sec dialect;
- register the standard dialects needed by Package 1 tests;
- use normal MLIR command-line parsing;
- support parsing, verification, printing and ordinary pass-manager behavior;
- fail on malformed registered Sec types;
- fail on unknown unregistered dialect constructs unless the user explicitly
  enables MLIR's normal unregistered-dialect behavior.

Do not implement Sec lowering passes in this package.

The tool should be based on normal upstream MLIR optimizer-driver facilities
rather than a custom parser.

---

# 12. Go integration

`internal/mlir/toolchain.go` currently knows how to run `mlir-opt` and
`mlir-translate`.

Package 1 must add a non-breaking method for invoking the Sec-aware tool.

Recommended API:

```go
func (t Toolchain) VerifySec(mlirPath string) error
```

Behavior:

```text
run sec-mlir-opt <file> -o /dev/null
```

The implementation must use the existing `Toolchain.run` and `toolPath`
mechanisms.

`SEC_MLIR_BIN` remains the binary-directory override.

Do not change the behavior of:

```go
Verify
TranslateToLLVMIR
```

Do not route existing compilation through `VerifySec` yet.

Add Go tests that use a temporary fake executable or another repository-consistent
test mechanism so the test does not require a developer's real MLIR installation.

All Go code must remain English.

---

# 13. TableGen requirements

Use TableGen/ODS as the canonical type definition source.

At minimum:

```text
SecDialect.td
SecTypes.td
```

must generate the relevant C++ declarations/definitions.

Do not manually duplicate type parsers or printers when declarative assembly
format can express the required syntax cleanly.

If custom parse/print code is needed due to current MLIR limitations, keep it
small and covered by round-trip tests.

Generated documentation should be enabled if it can be done without introducing
extra build fragility.

---

# 14. Verification requirements

Package 1 verification is structural, not source-semantic.

The Sec dialect verifier must reject malformed Package 1 representations.

Required invalid cases:

1. empty named type identity;
2. empty distinct type identity;
3. `NoneType` used as the base type if it can be constructed in textual IR;
4. malformed textual syntax;
5. unknown Sec type mnemonic.

Required valid cases:

1. named type with integer base;
2. named type with floating base;
3. distinct type with integer base;
4. two different identities with the same base type remain different MLIR
   types;
5. same identity and same base type uniquify as the same MLIR type;
6. nested use in `func.func` signatures;
7. source location survives parsing and printing;
8. reserved metadata attributes survive parsing and printing.

The verifier must not perform:

```text
Sec name lookup
contract checking
ownership checking
borrow checking
layout calculation
ABI calculation
unit algebra
```

Those decisions must already belong to Semantic IR or later defined lowering
passes.

---

# 15. Required tests

## T01 - Dialect registration

Input:

```mlir
module attributes {sec.dialect_version = 1 : i32} {
}
```

Command:

```bash
sec-mlir-opt test.mlir -o -
```

Expected:

- exit code 0;
- module prints successfully.

## T02 - Named type round trip

Input includes:

```mlir
!sec.named<"main::Speed", i64>
```

Expected:

- parse succeeds;
- print succeeds;
- printed IR parses again;
- type text remains semantically equivalent.

## T03 - Distinct type round trip

Input includes:

```mlir
!sec.distinct<"main::CustomerID", i64>
```

Expected:

- parse succeeds;
- print succeeds;
- printed IR parses again.

## T04 - Identity separation

Construct:

```text
!sec.named<"A", i64>
!sec.named<"B", i64>
```

Expected:

- MLIR type equality reports false.

Construct two copies of:

```text
!sec.named<"A", i64>
```

Expected:

- MLIR type equality reports true.

This may be a C++ unit test if equality is awkward to prove with FileCheck.

## T05 - Empty named identity rejected

Attempt to parse/build:

```mlir
!sec.named<"", i64>
```

Expected:

- verification/parser failure;
- diagnostic refers to non-empty identity.

## T06 - Empty distinct identity rejected

Attempt:

```mlir
!sec.distinct<"", i64>
```

Expected:

- verification/parser failure.

## T07 - Metadata round trip

Use:

```text
sec.symbol_id
sec.type_id
sec.layout_ref
sec.synthesized
sec.dialect_version
```

Expected:

- attributes survive parse/print.

## T08 - Source location round trip

Use:

```mlir
loc("sample.sec":12:8)
```

Expected:

- location survives parse/print.

## T09 - Unknown Sec type rejected

Input contains an unregistered type such as:

```mlir
!sec.future_type
```

Expected:

- normal invocation rejects it.

## T10 - No accidental unregistered-dialect acceptance

Run a module containing:

```mlir
"unknown.op"() : () -> ()
```

without an unregistered-dialect option.

Expected:

- failure.

## T11 - Go toolchain Sec verifier command

Test `Toolchain.VerifySec`.

Expected command target:

```text
sec-mlir-opt
```

Expected:

- existing timeout/error behavior is preserved;
- no external MLIR installation is required by the unit test.

## T12 - Go regression

Run:

```bash
go test ./...
```

Expected:

- all existing tests pass.

## T13 - Legacy MLIR generator regression

Existing tests under:

```text
internal/codegen/mlir
```

must remain green.

No expected output should be rewritten merely to use the Sec dialect in this
package.

## T14 - C++ build

Run:

```bash
cmake --build build/sec-mlir
```

Expected:

- no generated-source files required from the source tree;
- `sec-mlir-opt` links successfully.

## T15 - Lit suite

Run:

```bash
cmake --build build/sec-mlir --target check-sec-mlir
```

Expected:

- all Package 1 dialect tests pass.

---

# 16. Acceptance criteria

Package 1 is complete only when all of the following are true:

```text
[ ] rules/sec_mlir_dialect.md exists and matches the supplied Package 1 rulebook
[ ] mlir/ configures against an external MLIR installation/build
[ ] TableGen generates Sec dialect/type code
[ ] Sec dialect registers successfully
[ ] sec-mlir-opt builds
[ ] !sec.named parses, verifies and prints
[ ] !sec.distinct parses, verifies and prints
[ ] malformed Package 1 Sec types are rejected
[ ] source locations survive round-trip
[ ] reserved metadata attributes survive round-trip
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy internal/codegen/mlir tests pass unchanged in meaning
[ ] normal compilation is not yet redirected through the new dialect
[ ] no Package 2+ semantic operations were added as placeholders
```

---

# 17. Required implementation report

At completion Codex must report:

```text
1. files added
2. files modified
3. exact MLIR/LLVM version used for the successful build
4. CMake configure command used
5. build command used
6. test commands used
7. test results
8. any deviations from this package
9. any required follow-up work for Package 2
```

Any deviation that changes the dialect surface must be treated as a design issue,
not silently implemented.

---

# 18. Package 2 boundary

Package 2 should begin only after Package 1 is green.

The intended next package is:

```text
Semantic IR -> high-level Sec MLIR import foundation
```

Likely Package 2 responsibilities:

```text
Semantic IR module bridge
function/symbol import
resolved scalar type import
named/distinct type import
source-location import
identity/layout metadata import
minimal function body/control-flow import
verification after import
Sec MLIR dump path from the Go compiler
```

Package 2 must still avoid ownership/aggregate/error lowering unless the imported
Semantic IR requires a small explicit subset and the normative dialect rulebook
has been extended first.

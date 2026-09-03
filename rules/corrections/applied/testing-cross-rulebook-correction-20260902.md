# Testing Cross-Rulebook Correction

- **Status:** Applied
- **Created:** 2026-09-02
- **Last updated:** 2026-09-02
- **Applied:** 2026-09-02
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `814a584`
- **Primary new rulebook:** `rules/tooling/testing.md`
- **Intended correction area:** `rules/corrections/`
- **Classification:** Normative synchronization required by the new canonical testing rulebook and by the already-canonical direct standard-library import model.

## 1. Purpose

### 1.1 Scope

This correction records cross-rulebook changes required when `rules/tooling/testing.md` is added.

It also corrects the remaining obsolete `std/...` standard-library import examples in the module/project rulebooks.

### 1.2 Durable authority

After these corrections are merged into the owning canonical rulebooks, the owning rulebooks become the durable authority and this correction may be archived according to repository governance.

---

## 2. `rules/projects/modules.md` — standard-library import root correction

### 2.1 Current conflict

The current module rulebook states that Sec 0.1 has compiler-reserved import roots:

```text
std
platform
```

and gives:

```sec
import "std/io"
```

as a standard-library example.

This conflicts with the canonical standard-library rule, which uses direct logical module paths such as:

```sec
import "fmt"
import "io"
import "net/http"
```

without a `std/` source prefix.

### 2.2 Corrected rule

The `std` source import root is removed from Sec source syntax.

Standard-library modules use their canonical logical module paths directly.

Examples:

```sec
import "io"
import "fmt"
import "net/http"
```

The compiler resolves those names through the canonical standard-library namespace according to the project/module resolution rules.

A project-local or external module must not silently replace a canonical standard-library module with the same logical path.

### 2.3 Platform root remains separate

The compiler-reserved:

```text
platform
```

root remains platform-specific and is not affected by this correction unless a later platform rule changes it.

### 2.4 Required module synchronization

Update at least:

```text
modules.md section 8 Import roots
all std/... examples
import-resolution text that assumes std is source-visible
```

---

## 3. `rules/projects/projects.txt` — standard-library import correction

### 3.1 Current conflict

The project rulebook also describes:

```text
std
platform
```

as source-visible reserved import roots.

### 3.2 Corrected rule

Synchronize the project rulebook with the direct standard-library import model:

```sec
import "io"
import "net/http"
```

not:

```sec
import "std/io"
import "std/net/http"
```

### 3.3 Project resolution

The project model may maintain an internal compiler distinction between:

```text
core
standard library
project
dependency
platform
```

without exposing `std/` as a mandatory source path prefix.

---

## 4. `rules/projects/modules.md` — test-mode module participation

### 4.1 Production ModuleInstance

An ordinary production ModuleInstance excludes:

```text
*_test.sec
```

### 4.2 Test ModuleInstance

A TestCompilationPlan may construct a test view of a production module containing:

```text
ordinary production .sec files
same-directory *_test.sec files
```

### 4.3 Visibility

Same-directory test files receive ordinary same-module visibility.

They may access module-internal declarations.

They do not receive access to declarations that remain source-file-private in another source file.

### 4.4 Dependency direction

Production declarations must remain valid without test-only declarations.

The module dependency model must represent:

```text
Production -> Production
Test -> Production
Test -> Test
```

and reject:

```text
Production -> Test
```

---

## 5. `rules/projects/projects.txt` — project test source tree

### 5.1 Reserved test tree

Add project-root:

```text
tests/
```

as the canonical test-only source tree for black-box and integration-test modules.

### 5.2 Production exclusion

Source below `tests/` does not participate in ordinary production builds.

### 5.3 Module semantics

Directories below `tests/` follow ordinary module naming and import rules except that their source universe exists only in TestCompilationPlan.

### 5.4 Public API boundary

Test-only modules below `tests/` access production modules through ordinary imports and ordinary external visibility.

No friend visibility is implied.

---

## 6. `rules/projects/projects.txt` — TestCompilationPlan

### 6.1 Compilation mode

Extend the conceptual CompilationPlan with an explicit compilation mode sufficient to distinguish at least:

```text
Production
Test
```

Do not model this as ad-hoc source inclusion performed after production compilation.

### 6.2 Test plan additions

A TestCompilationPlan may additionally contain:

```text
selected *_test.sec source
tests/ test-only modules
test-only dependencies
compiler-known testing support
generated test harness roots
test execution metadata
```

### 6.3 Existing `kind = "test"`

The existing project target kind:

```text
test
```

must not be interpreted as a requirement for ordinary source tests.

`sec test` and `*_test.sec` do not require a manifest target whose kind is `test`.

If `kind = "test"` is retained, the project rulebook must define it as an explicitly buildable test-oriented target/artifact concept separate from ordinary source test discovery.

It must not grant test visibility or replace TestCompilationPlan semantics by itself.

---

## 7. `rules/library/core-library.md` — compiler-known testing context

### 7.1 New compiler-known context surface

Add the test-context compiler-known namespace:

```text
testing
```

with the Sec 0.1 surface defined by `rules/tooling/testing.md`.

### 7.2 No stdlib import

`testing` is not an implicit import of a standard-library module.

It is compiler-known test-context functionality, analogous in authority to other compiler-owned core operations whose semantics cannot be represented as an ordinary unprivileged library call alone.

### 7.3 Production unavailability

Ordinary production source must not acquire test semantics by importing or declaring an unrelated module named `testing`.

The compiler-owned test identity is tied to TestCompilationPlan context.

---

## 8. `rules/foundations/grammar.md` — `test` declaration

### 8.1 New top-level declaration form

Add:

```text
TestDeclaration
    := "test" StringLiteral Block
```

subject to the canonical restrictions in `rules/tooling/testing.md`.

### 8.2 Context

The spelling `test` is recognized in top-level declaration position for test source.

Lexical/reserved-word treatment must be synchronized with the canonical lexical strategy rather than inferred independently by parser, formatter and editor grammar.

---

## 9. `rules/errors/errorhandling.md` — test propagation boundary

### 9.1 New compiler-known boundary

A top-level test invocation and a subtest invocation are compiler-known error-propagation boundaries.

### 9.2 Unexpected `Err`

An otherwise-unhandled `Err(errorValue)` propagated through ordinary `try` to the test boundary:

```text
terminates the current invocation
runs ordinary cleanup
records Failed
reports the unexpected error
```

It does not require a source-visible `Result` return type on the `test` declaration.

### 9.3 Expected error

Expected errors continue to use the canonical `try`/`match` semantics.

No parallel test-specific error type system is introduced.

---

## 10. Cleanup rulebooks — controlled test termination

### 10.1 Affected rulebooks

Synchronize:

```text
rules/control-flow/defer.md
rules/memory/destruction.md
```

### 10.2 Required behavior

Controlled test termination through:

```text
testing.Pass
testing.Fail
testing.Skip
failed testing.Require
failed testing.RequireEqual
unexpected Err reaching the test boundary
```

must execute ordinary cleanup for the invocation.

Testing does not create an escape from deterministic destruction or registered `defer`.

---

## 11. `rules/tooling/lsp.md` — test discovery and execution integration

### 11.1 Compiler workspace authority

The compiler workspace is the sole source of truth for:

```text
test declaration discovery
test identity
test source range
test category
test compilation diagnostics
```

### 11.2 Test compilation view

The LSP must support a test compilation view in addition to production analysis so `*_test.sec`, `tests/` modules and `testing.*` resolve correctly.

### 11.3 Runnable metadata

Expose enough compiler-owned metadata for clients to provide:

```text
Run Test
Debug Test
Run Tests in File
Run Tests in Module
Run Project Tests
```

where the client supports those actions.

### 11.4 Editor-native testing UI

A client such as the VS Code extension may integrate compiler/LSP test metadata with the editor's native testing UI.

The extension must not independently parse Sec source to define test semantics.

### 11.5 Canonical execution

Editor-run tests use the same TestCompilationPlan and test selection semantics as `sec test`.

No editor-specific semantic runner is permitted.

---

## 12. `rules/tooling/formatter.md`

### 12.1 Required syntax support

Add formatting support for:

```sec
test "name" {
    ...
}
```

and ordinary `testing.*` calls.

### 12.2 Test file handling

`*_test.sec` is normal Sec input to `sec fmt`.

---

## 13. `rules/tooling/diagnostics.txt`

### 13.1 Required diagnostic classes

Add stable diagnostic definitions for at least:

```text
test outside *_test.sec
nested test declaration
empty test name
duplicate test identity
test parameters
test return type
return value from test
production use of test-only declaration
testing namespace outside test context
invalid Expect/Require boolean argument
invalid equality helper operands
invalid subtest name
duplicate sibling subtest identity
```

### 13.2 Runtime test records

Runtime test failure records are structured test results and must not be misclassified as compile-time diagnostics.

---

## 14. Compiler pipeline and Semantic IR

### 14.1 Affected rulebooks

Synchronize:

```text
rules/compiler/compiler_pipeline.md
rules/compiler/semantic_ir.md
```

### 14.2 Required concepts

The compiler must retain explicit:

```text
CompilationMode.Test
TestDeclaration identity
test-boundary identity
selected harness roots
test-only source provenance
subtest/result semantics where represented before lowering
```

### 14.3 No function-name inference

No compiler stage may reconstruct test identity from a generated function name after the frontend has already resolved the test declaration.

---

## 15. Execution and debug integration

### 15.1 Future owning rules

Future debug/execution-provider rules must permit TestCompilationPlan artifacts to be:

```text
deployed
started
reset
debugged
observed
```

where the environment provides those capabilities.

### 15.2 Transport boundary

The physical test-result transport is owned by the execution/debug provider.

It is not defined by Sec source semantics.

### 15.3 LSP Debug Test

Editor `Debug Test` uses the same selected test semantics with an execution provider that additionally supports debugging.

---

## 16. `language-rulebook-status.md`

### 16.1 Add written rulebook

Add:

```text
tooling/testing.md
```

to the canonical written set.

Suggested tooling table entry:

```text
| `tooling/testing.md` | **Written** | Canonical source-level `test`, `*_test.sec`, `sec test`, testing.*, subtests, integration tests, TestCompilationPlan, execution-provider and editor-integration semantics. Implementation progress is tracked by `tooling.language-testing`. |
```

### 16.2 Keep compiler testing separate

Do not replace:

```text
compiler_testing.md
```

with `tooling/testing.md`.

The planned compiler-testing rulebook still owns verification of the Sec compiler implementation itself.

---

## 17. `rules/README.md`

### 17.1 Directory map

Extend the `tooling/` description to include source-level testing/toolchain integration.

Suggested scope:

```text
Diagnostics, formatter, source-level testing, and LSP/editor integration.
```

---

## 18. Legacy testing sketches

### 18.1 Obsolete direction

Any earlier non-canonical design that defines tests primarily as:

```text
@test
fn TestSomething(...)
```

or treats `_test.sec` as merely optional while the attribute is the source of truth is superseded by:

```sec
test "name" {
}
```

inside:

```text
*_test.sec
```

### 18.2 Benchmark sketches

Earlier non-canonical `@benchmark` sketches do not define Sec 0.1 benchmark syntax.

Benchmarking and fuzzing remain future design areas until separately specified.

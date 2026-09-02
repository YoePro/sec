# Sec Testing

- **Status:** Normative
- **Created:** 2026-09-02
- **Last updated:** 2026-09-02
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `814a584`
- **Canonical path:** `rules/tooling/testing.md`
- **Replaces:** No canonical rulebook. This rulebook supersedes earlier non-canonical testing sketches where they conflict.
- **Related rulebooks:** `rules/projects/projects.txt`, `rules/projects/modules.md`, `rules/library/core-library.md`, `rules/errors/errorhandling.md`, `rules/errors/panic.md`, `rules/control-flow/defer.md`, `rules/memory/destruction.md`, `rules/declarations/lambda-functions.md`, `rules/compiler/compiler_pipeline.txt`, `rules/compiler/semantic_ir.txt`, `rules/tooling/diagnostics.txt`, `rules/tooling/formatter.md`, `rules/tooling/lsp.md`

## 1. Purpose

### 1.1 Scope

This rulebook defines source-level testing and the canonical `sec test` model for Sec 0.1.

It defines:

```text
test source files
test declarations
test identity
white-box tests
black-box and integration tests
test-only declarations and dependencies
test compilation
test discovery and selection
test outcomes
the compiler-known testing namespace
fatal and non-fatal expectations
error propagation at test boundaries
subtests
cleanup behavior
test isolation
test execution ordering
generated test harnesses
hosted and embedded execution providers
structured test results
CLI behavior
LSP and editor integration
```

### 1.2 Design goal

Testing must be easy enough that adding a useful test normally requires only:

```sec
test "Add returns the sum" {
    testing.ExpectEqual(4, Add(2, 2))
}
```

followed by:

```text
sec test
```

Testing must use ordinary Sec semantics wherever possible.

Testing must not require:

```text
a third-party test runner
a user-written main function
a hidden stdlib import
runtime reflection
dynamic test registration
garbage collection
a heap
an operating system
a filesystem
threads
a permanent testing runtime in production binaries
```

### 1.3 Non-goals

This rulebook does not define:

```text
compiler implementation self-tests
compiler unit-test organization in Go or C++
benchmark semantics
fuzz semantics
mocking frameworks
BDD syntax
snapshot testing
golden-file DSLs
system or end-to-end process orchestration
a debug wire protocol
a test-result byte protocol
a universal test timeout
a universal filesystem sandbox
```

Compiler self-testing remains the responsibility of the planned `compiler_testing.md`.

Benchmarking and fuzzing are future design areas and are not source-language features defined by Sec 0.1.

---

## 2. Normative ownership

### 2.1 Testing owns source test semantics

This rulebook is the canonical owner of:

```text
test declarations
test-file participation
test outcomes
testing.*
subtests
test compilation semantics
sec test selection semantics
test execution result semantics
```

### 2.2 Other rulebooks retain their own semantics

Testing does not redefine:

```text
ordinary equality
Result and Err
try
match
return
defer
ownership
borrowing
destruction
panic
assert
module visibility
imports
target profiles
debug transport
```

A test body uses those ordinary Sec rules except where this rulebook explicitly defines a test boundary.

### 2.3 Compiler testing is separate

Source-language testing:

```text
rules/tooling/testing.md
```

is distinct from compiler verification infrastructure:

```text
compiler_testing.md
```

The latter may define parser tests, Sema tests, invalid fixtures, IR tests, MLIR tests, backend tests and regression matrices for the compiler implementation.

---

## 3. Core terminology

### 3.1 Test compilation

A **TestCompilationPlan** is a CompilationPlan whose compilation mode is `Test`.

Conceptually:

```text
CompilationMode:
    Production
    Test
```

The internal representation should remain extensible to future compilation modes without reducing test mode to a boolean flag.

### 3.2 Test invocation

A **test invocation** is one execution of either:

```text
a top-level test declaration
a subtest created by testing.Run
```

Each invocation has its own outcome, diagnostics, cleanup boundary and hierarchical test identity.

### 3.3 Test outcome

A completed test invocation has one primary outcome:

```text
Passed
Failed
Skipped
```

Selection state and infrastructure failures are not test outcomes.

For example:

```text
NotSelected
CompilationFailed
ExecutionUnavailable
ExecutionFailed
Timeout
TargetReset
```

must not be represented as `Skipped`.

### 3.4 White-box test

A white-box test is declared in a `*_test.sec` file in an ordinary production module directory.

It participates in the same module during test compilation.

### 3.5 Integration test

A black-box or integration test is declared below the project-root `tests/` source tree.

It belongs to a test-only module and accesses production modules through normal imports and visibility.

---

## 4. Test source files

### 4.1 Canonical filename

Source test files use:

```text
*_test.sec
```

Examples:

```text
request_test.sec
parser_test.sec
regression_test.sec
```

The filename does not need to correspond one-to-one with a production source file.

### 4.2 Test declaration location

A `test` declaration is legal only in a `*_test.sec` file.

Invalid:

```sec
// request.sec

test "Request parses GET" {
}
```

Expected diagnostic:

```text
test declarations are only allowed in *_test.sec files
```

### 4.3 Production builds exclude test files

Ordinary production compilation does not include `*_test.sec` files.

This includes at least:

```text
sec build
sec run
ordinary library compilation
ordinary firmware compilation
```

Test source must not add production declarations, dependencies, initialization or emitted code to a production artifact.

### 4.4 Test builds include selected test files

A TestCompilationPlan includes:

```text
selected production source
selected *_test.sec source
required test-only helpers
required test-only dependencies
generated test harness support
```

### 4.5 Formatter and editor support

`*_test.sec` is ordinary Sec source for lexical, parser, formatter and editor purposes.

The formatter must format it as Sec source.

The LSP must analyze it using the appropriate test compilation view.

---

## 5. Test declaration syntax

### 5.1 Canonical form

A top-level test declaration is:

```sec
test "Request parses GET" {
    ...
}
```

### 5.2 Grammar

The canonical grammar is conceptually:

```text
TestDeclaration
    := "test" StringLiteral Block
```

`test` is recognized in top-level declaration position.

The grammar rulebook remains responsible for the complete integrated grammar and lexical classification.

### 5.3 Top-level only

`test` is a module-level declaration.

It is not legal inside:

```text
fn
impl
property
test
lambda
local block
```

Nested test execution uses `testing.Run`, not nested `test` declarations.

### 5.4 Name requirement

The test name must be a non-empty compile-time string literal.

Valid:

```sec
test "empty input is rejected" {
}
```

Invalid:

```sec
test "" {
}
```

A runtime expression cannot be used as a top-level test name.

### 5.5 No parameters

A top-level test declaration has no source-visible parameters.

Invalid design:

```sec
test "example"(context: TestContext) {
}
```

The programmer is not required to receive a test-context argument.

### 5.6 No source-visible return type

A top-level test declaration has no source-visible return type.

Invalid:

```sec
test "example" bool {
}
```

Invalid:

```sec
test "example" Result[void, error] {
}
```

The generated harness owns the test outcome.

### 5.7 Not callable as an ordinary function

A test declaration is not an ordinary source-level function value.

User code cannot:

```text
call a top-level test by name
store it in a callable value
take ownership of it
export it as an ordinary API
```

The compiler may lower it to an internal callable entry point.

---

## 6. Test identity

### 6.1 Top-level identity

A top-level test identity consists of:

```text
TestModuleIdentity
TestName
```

The identity must be stable for a stable source declaration and CompilationPlan.

### 6.2 Unique names

Two top-level test declarations in the same test module must not have the same test name.

The compiler must diagnose duplicate top-level test identities.

### 6.3 Same names in different modules

Different modules may declare the same human-readable test name.

Tooling must use the complete test identity when ambiguity exists.

### 6.4 Hierarchical identity

Subtests extend the parent identity with a path component.

Conceptually:

```text
module :: top-level test :: subtest :: nested subtest
```

The internal representation should use structured path components rather than requiring string concatenation to define semantic identity.

---

## 7. White-box tests

### 7.1 Same-directory participation

A file such as:

```text
net/http/request_test.sec
```

participates in the same module as the ordinary Sec files in:

```text
net/http/
```

during TestCompilationPlan construction.

### 7.2 Module declaration

The test file declares the same module name required by the directory.

Example:

```sec
module http
```

### 7.3 Module-internal visibility

A white-box test may access module-internal declarations according to the ordinary module visibility rules.

Testing does not create a separate friend mechanism.

### 7.4 Source-file-private visibility remains private

A source-file-private declaration remains visible only from its declaring source file.

Moving test code into a separate `*_test.sec` file does not grant access to source-file-private declarations.

Testing must not weaken the canonical distinction between:

```text
module-internal
source-file-private
```

### 7.5 Test-only declarations

A white-box `*_test.sec` file may contain ordinary declarations used only by tests.

Examples include:

```text
helper functions
fixture types
test-case structs
test-only constants
test-only impl support where otherwise legal
```

These declarations exist only in TestCompilationPlan.

---

## 8. Integration and black-box tests

### 8.1 Project test source tree

The project-root directory:

```text
tests/
```

is the canonical source tree for black-box and integration tests.

It participates only in test compilation.

### 8.2 Ordinary module rules

Directories below `tests/` form test-only modules according to the ordinary module rules.

Files directly in `tests/` may form the test-root module according to the project/module naming rules.

### 8.3 Test files remain `*_test.sec`

Integration test declarations still occur only in:

```text
*_test.sec
```

The `tests/` directory does not make an ordinary `.sec` file containing `test` declarations legal.

### 8.4 External visibility

An integration-test module sees production modules as an external consumer sees them.

It must use ordinary imports.

Example:

```sec
import "net/http"

test "public request API parses GET" {
    let request := try http.ParseRequest("GET / HTTP/1.1")

    testing.ExpectEqual(http.Method.GET, request.Method)
}
```

### 8.5 No test-friend visibility

Sec 0.1 does not define:

```text
friend tests
@test-visible
internal-visible-to-tests
```

Use same-module white-box tests when module-internal access is required.

### 8.6 Multi-module integration

An integration test may import and exercise several production modules.

The compiler must not infer that an integration test semantically belongs to one production module merely because it imports that module.

### 8.7 Black-box does not imply process isolation

Black-box testing means external Sec API visibility.

It does not by itself require:

```text
a separate operating-system process
network transport
deployment
container isolation
fresh machine state
```

System and end-to-end orchestration are future tooling concerns.

---

## 9. Test body semantics

### 9.1 Ordinary Sec code

A test body supports ordinary Sec statements and expressions.

This includes, where otherwise legal:

```text
let
if
for
while
match
try
defer
function calls
methods
properties
collections
ownership
borrowing
concurrency
hardware access
```

Testing does not define a mini-language.

### 9.2 Normal completion

Reaching the end of a test body without a recorded failure produces `Passed`.

### 9.3 Bare return

A bare:

```sec
return
```

may terminate the current test invocation normally.

It does not return a source-visible value.

The resulting outcome is `Passed` unless a failure was already recorded.

### 9.4 Return values are forbidden

A test cannot return:

```text
true
false
Ok(...)
Err(...)
another value
```

through ordinary `return value` syntax.

### 9.5 Failure state is monotonic

Once the current invocation has recorded a failure, later normal completion or `testing.Pass()` must not erase that failure.

This prevents a later control path from accidentally converting a failed test to `Passed`.

### 9.6 Skip does not erase failure

If an invocation has already recorded a failure, a later skip request does not erase the failure.

The primary outcome remains `Failed`.

Tooling may retain the skip request as additional diagnostic information.

---

## 10. Error propagation at a test boundary

### 10.1 Test boundary supports `try`

A test invocation is a compiler-known error-propagation boundary.

This permits ordinary concise test code:

```sec
test "valid request parses" {
    let request := try ParseRequest("GET / HTTP/1.1")

    testing.ExpectEqual(Method.GET, request.Method)
}
```

### 10.2 Unexpected propagated error

If an unhandled `Err(errorValue)` propagates to the current test boundary:

```text
the current invocation terminates
ordinary cleanup executes
the current invocation becomes Failed
the error value is reported as an unexpected error
```

### 10.3 Error identity must be preserved

Diagnostics should preserve the concrete error identity and payload information available through the ordinary Sec error model.

A test failure caused by:

```text
ParseError.InvalidHeader
```

must not be reduced to an opaque generic failure when the error identity is known.

### 10.4 Expected errors use ordinary Sec error handling

Testing does not define `ExpectError`.

Expected errors are tested using ordinary `try` handlers or `match`.

Example:

```sec
test "invalid header returns InvalidHeader" {
    try ParseHeader(input) {
        Err(ParseError.InvalidHeader) => {
            testing.Pass()
        }
    }

    testing.Fail("expected ParseError.InvalidHeader")
}
```

### 10.5 Explicit Result inspection

When both success and error paths are the subject of the test, ordinary `match` remains appropriate.

Testing must not replace the canonical Result model with a second test-specific error API.

---

## 11. Compiler-known `testing` namespace

### 11.1 Availability

During test compilation, the compiler provides the compiler-known namespace:

```text
testing
```

No import is required.

### 11.2 Not a hidden stdlib import

The availability of `testing` is part of test compilation semantics.

It is not equivalent to:

```sec
import "testing"
```

and does not violate the rule that ordinary stdlib modules are imported explicitly.

### 11.3 Test-context only

The compiler-known testing namespace is available only where test compilation semantics permit its use.

Production code must not depend on `testing`.

### 11.4 Canonical 0.1 surface

Sec 0.1 defines:

```sec
testing.Pass()

testing.Fail(message: string)

testing.Skip(message: string)

testing.Log(message: string)

testing.Expect(condition: bool)
testing.Expect(condition: bool, message: string)

testing.Require(condition: bool)
testing.Require(condition: bool, message: string)

testing.ExpectEqual(expected, actual)
testing.ExpectEqual(expected, actual, message: string)

testing.RequireEqual(expected, actual)
testing.RequireEqual(expected, actual, message: string)

testing.Run(name, body)

testing.TestResult
```

The exact compiler-internal implementation may use generated support or core declarations.

The source semantics are defined here.

---

## 12. `testing.Pass`

### 12.1 Meaning

```sec
testing.Pass()
```

requests successful termination of the current test invocation.

### 12.2 Terminal control flow

`testing.Pass()` is terminal for the current test invocation.

Sema must treat following code in the same reachable path according to the canonical terminating-control-flow rules.

### 12.3 Cleanup

Controlled termination through `testing.Pass()` performs ordinary cleanup before leaving the invocation.

### 12.4 Existing failure

If a failure has already been recorded in the current invocation, `testing.Pass()` terminates the invocation but does not erase the failure.

The resulting primary outcome remains `Failed`.

### 12.5 Normal tests do not require Pass

A test that reaches its end normally already passes.

`testing.Pass()` is intended primarily for early successful termination, including expected-error branches.

---

## 13. `testing.Fail`

### 13.1 Signature

```sec
testing.Fail(message: string)
```

requires a message.

### 13.2 Meaning

`testing.Fail`:

```text
records a failure
records the message
terminates the current invocation
performs ordinary controlled cleanup
```

### 13.3 No argument-free form

Sec 0.1 does not define:

```sec
testing.Fail()
```

A deliberate explicit failure should state why it occurred.

---

## 14. `testing.Skip`

### 14.1 Signature

```sec
testing.Skip(message: string)
```

requires a message.

### 14.2 Meaning

If the invocation has not already failed, `testing.Skip`:

```text
records Skipped
records the reason
terminates the current invocation
performs ordinary controlled cleanup
```

### 14.3 Runtime conditions

Skip may be selected from ordinary runtime control flow.

Example:

```sec
if !device.IsPresent {
    testing.Skip("device is not present")
}
```

### 14.4 Compile-time exclusion is separate

A test not selected for the active target or CompilationPlan is not `Skipped` merely because it was not compiled or selected.

Selection and test outcomes remain separate.

---

## 15. `testing.Log`

### 15.1 Meaning

```sec
testing.Log(message)
```

records diagnostic information associated with the current test invocation.

### 15.2 No outcome change

Logging does not:

```text
pass
fail
skip
terminate
```

the invocation.

### 15.3 Not ordinary stdout

`testing.Log` is structured test diagnostic output.

It is distinct from ordinary program stdout or stderr.

Execution providers may transport `testing.Log` without providing normal process I/O.

### 15.4 Formatting

Testing does not define a `Logf` formatting language in Sec 0.1.

The argument is a normal `string`.

Normal Sec formatting facilities may be used to construct the message when available.

---

## 16. `testing.Expect`

### 16.1 Non-fatal expectation

```sec
testing.Expect(condition)
```

requires a `bool`.

If the condition is false:

```text
a failure is recorded
the current invocation continues
```

### 16.2 Optional message

The overload:

```sec
testing.Expect(condition, message)
```

records the supplied string as additional context.

### 16.3 No expression-stringification requirement

Sec 0.1 does not require the compiler to reconstruct or stringify the source expression for `condition`.

At minimum, a failed expectation reports:

```text
test identity
source location
expectation failure
optional message
```

---

## 17. `testing.Require`

### 17.1 Fatal requirement

```sec
testing.Require(condition)
```

requires a `bool`.

If the condition is false:

```text
a failure is recorded
the current invocation terminates
ordinary cleanup executes
```

### 17.2 Optional message

The overload:

```sec
testing.Require(condition, message)
```

records the message as additional context.

### 17.3 Purpose

`Require` is intended when continued test execution is not meaningful or would make later diagnostics misleading.

---

## 18. Equality expectations

### 18.1 Canonical argument order

Equality helpers use:

```text
expected
actual
```

in that order.

### 18.2 Non-fatal equality

```sec
testing.ExpectEqual(expected, actual)
```

records a failure and continues when the values are not equal.

### 18.3 Fatal equality

```sec
testing.RequireEqual(expected, actual)
```

records a failure and terminates the current invocation when the values are not equal.

### 18.4 Optional message

Both equality helpers permit an additional message string.

### 18.5 Ordinary Sec equality

Testing equality uses the canonical Sec equality rules.

`testing.ExpectEqual(a, b)` is legal only where the corresponding equality comparison is semantically legal.

Testing does not introduce:

```text
implicit conversions
deep comparison outside normal Sec equality
special pointer equality
automatic approximate floating-point equality
```

### 18.6 Evaluate once

The expected expression and actual expression are each evaluated exactly once.

Diagnostic reporting must use the values established by that evaluation.

Testing must not re-evaluate user expressions to produce diagnostics.

---

## 19. Diagnostic value rendering

### 19.1 Structured expected and actual values

When representable, an equality failure should report:

```text
expected
actual
type information where useful
source location
test identity
optional message
```

### 19.2 Compiler-known rendering

The compiler or harness may provide direct diagnostic representations for compiler-known types such as:

```text
bool
integers
floating-point values
runes
strings
enums
simple Option/Result discriminants
```

### 19.3 Enum identity

When enum identity is known, diagnostics should prefer the semantic enum value:

```text
Method.GET
```

over an underlying integer representation.

### 19.4 Formatting is not required for equality legality

A type may be legally comparable even when the test system cannot fully render its values.

A lack of diagnostic formatting support must not by itself make `ExpectEqual` illegal.

A fallback diagnostic may state:

```text
values are not equal
type: SomeType
```

### 19.5 No testing-specific formatting protocol

Testing does not define a separate `Display`, `TestString`, `DebugString` or formatting interface.

It may use canonical Sec formatting facilities where available.

---

## 20. Assertions and testing

### 20.1 `assert` remains ordinary program semantics

The language-level:

```sec
assert condition
assert condition, "message"
```

is not redefined as `testing.Expect`.

### 20.2 Separate purpose

`assert` expresses a program/runtime assertion.

`testing.Expect` and `testing.Require` communicate a test outcome to the test harness.

### 20.3 Assertion failure during a test

Where the execution environment can identify an ordinary assertion failure, the test runner should associate it with the current test invocation and report it distinctly from a `testing.Expect` failure.

Testing must not change the canonical panic/runtime-check semantics of `assert`.

---

## 21. Cleanup and controlled termination

### 21.1 Cleanup applies

The following controlled exits from a test invocation participate in ordinary Sec cleanup:

```text
normal completion
bare return
propagated unexpected Err
testing.Pass
testing.Fail
testing.Require failure
testing.RequireEqual failure
testing.Skip
```

### 21.2 Destruction

Owned values are destroyed according to the ordinary destruction rules.

Testing does not bypass exact-once destruction.

### 21.3 `defer`

Deferred operations registered by the test body execute according to the canonical `defer` and destruction ordering rules.

### 21.4 No hidden teardown

Testing does not implicitly:

```text
close arbitrary resources
restore hardware registers
delete files
rollback databases
restore global state
```

unless such cleanup follows from ordinary owned values, explicit `defer`, or execution-provider isolation.

---

## 22. Subtests

### 22.1 Purpose

Subtests provide independently identified nested test invocations, especially for table-driven tests and repeated cases.

### 22.2 `testing.Run`

Subtests are created through the compiler-known operation:

```text
testing.Run(name, body)
```

The exact callable spelling of `body` follows the canonical callable/lambda syntax.

Testing does not introduce a second lambda syntax.

### 22.3 Runtime name

Unlike a top-level test name, a subtest name may be a runtime string.

This permits table-driven code to derive the case name from runtime test-case data.

### 22.4 Nested invocation context

The body supplied to `testing.Run` is analyzed and executed as a nested test invocation.

Within that body:

```text
testing.Pass
testing.Fail
testing.Skip
testing.Require
unexpected Err propagation
```

terminate the current subtest invocation rather than the parent invocation.

### 22.5 Synchronous execution

`testing.Run` is synchronous.

Before `testing.Run` returns:

```text
the child body has finished
the child outcome is known
child cleanup has completed
```

### 22.6 Result

`testing.Run` returns:

```text
testing.TestResult
```

with the values:

```text
Passed
Failed
Skipped
```

Ignoring the returned value is legal.

### 22.7 Failure propagation to parent

If any child invocation fails, the parent invocation is marked failed.

The parent body may continue after `testing.Run` returns.

### 22.8 Child skip

A skipped child does not automatically fail or skip the parent.

If the entire parent should be skipped, the parent must explicitly request `testing.Skip`.

### 22.9 Duplicate sibling names

Two sibling subtests created by one parent invocation must not have the same runtime name.

Because names may be runtime values, duplicate detection may occur at runtime in the generated harness.

A duplicate subtest identity is a test-harness failure attributed to the parent invocation.

### 22.10 Empty runtime name

A subtest runtime name must not be empty.

An empty name is an invalid subtest definition and fails the relevant invocation.

### 22.11 Nested subtests

A subtest may create further subtests.

There is no language-defined fixed nesting depth.

Implementation/resource limits must be diagnosed explicitly rather than silently changing identity.

---

## 23. Test execution ordering

### 23.1 No semantic ordering dependency

Independent top-level tests must not rely on a particular execution order.

Source-file order and filesystem enumeration order do not establish test execution dependencies.

### 23.2 Initial implementation recommendation

A Sec 0.1 runner may execute top-level tests sequentially for simplicity and reproducibility.

This is an implementation strategy, not a language guarantee.

### 23.3 Future parallel scheduling

A future runner may execute independent top-level tests in parallel without changing source semantics.

Sec 0.1 does not define:

```text
testing.Parallel
```

### 23.4 Ordinary concurrency remains available

Test code may use Sec's ordinary concurrency facilities where supported.

Testing does not create a separate ownership, race or synchronization model.

### 23.5 Subtests are synchronous

`testing.Run` itself remains synchronous even if a future top-level runner schedules different parents concurrently.

---

## 24. Isolation

### 24.1 Guaranteed local isolation

Each test invocation has ordinary independent local scope and cleanup:

```text
local values
owned local resources
defer registrations
borrow scopes
```

### 24.2 No guaranteed process isolation

Sec testing does not guarantee a fresh process for each test.

### 24.3 No guaranteed global reset

Testing does not automatically reset:

```text
mutable module state
filesystem state
environment variables
network services
database state
hardware state
remote resources
```

### 24.4 Provider isolation

An execution provider may offer stronger isolation such as:

```text
fresh process
target reset
fresh simulator
fresh emulator
```

Such isolation is a provider capability and must not be assumed by portable test semantics.

---

## 25. Test-only dependency direction

### 25.1 Production must remain independently valid

Production source must compile and resolve without test-only declarations.

A production declaration must not be made valid only because a `*_test.sec` file defines a missing helper.

### 25.2 Allowed dependency direction

The semantic dependency direction is:

```text
Production -> Production

Test -> Production
Test -> Test
```

The following is forbidden:

```text
Production -> Test
```

### 25.3 Test-only imports

Imports declared only by test source participate only in the TestCompilationPlan.

They must not become production dependencies.

### 25.4 Test dependency category

The project model must be able to distinguish:

```text
production dependency
test-only dependency
```

The exact manifest syntax is owned by the project rulebook and may be finalized separately.

---

## 26. TestCompilationPlan

### 26.1 First-class compilation mode

`sec test` constructs a TestCompilationPlan rather than running an ordinary production build followed by an unrelated external runner.

### 26.2 Production graph

The production graph is still validated as a production graph.

Test-only names must not satisfy production references.

### 26.3 Selected test graph

The test graph may additionally contain:

```text
same-module *_test.sec
tests/ test-only modules
test-only dependencies
compiler-known testing support
generated harness code
```

### 26.4 Target and variant

A TestCompilationPlan uses the canonical target, variant, profile, ABI and platform resolution model.

Testing must not infer target behavior from the compiler host.

### 26.5 No test initialization leakage

Test-only declarations and generated harness support do not participate in production initialization, linking or artifacts.

---

## 27. Test discovery

### 27.1 Compiler-owned discovery

The compiler workspace is the sole semantic source of truth for test discovery.

Tooling must not independently parse text looking for names beginning with `Test` or for test-like strings.

### 27.2 Top-level discovery

The compiler discovers top-level `test` declarations from selected `*_test.sec` files.

### 27.3 No runtime reflection

Discovery does not require:

```text
reflection
constructor registration
global runtime registries
linker-section scanning as source semantics
```

### 27.4 Static harness

The compiler may generate a static harness containing only the selected tests and required support.

### 27.5 Dynamic subtest discovery

Subtests created by `testing.Run` are discovered during execution.

An IDE may know top-level tests before execution without knowing every dynamically named subtest in advance.

---

## 28. `sec test`

### 28.1 Canonical command

The canonical user command is:

```text
sec test
```

### 28.2 Project default

From the active project, `sec test` selects the project's ordinary test scope:

```text
white-box tests in project production modules
project integration tests below tests/
```

It must not automatically run the standard library's own internal tests merely because stdlib modules are imported.

### 28.3 Scoped selection

The CLI must support restricting test execution to a selected test scope.

Exact general module/path selector syntax is owned by the canonical CLI/project model.

Testing must not invent a competing module-selection grammar.

### 28.4 Dependency tests

Testing a selected production module builds required dependencies but does not automatically run the dependencies' own tests.

### 28.5 Integration-test selection

Module-scoped white-box testing must not infer related integration tests merely by scanning imports under `tests/`.

Integration tests have their own test-module identities and are selected through the normal test selection model.

### 28.6 Name filtering

The test runner must provide deterministic filtering by test identity or test name.

The exact pattern language is tooling-versioned and is not normative Sec 0.1 source semantics.

### 28.7 Selection before final linking

Where practical, test selection occurs before final linking so that a selected single test does not require every project test body to be included in a constrained target artifact.

This is especially important for embedded targets.

---

## 29. Runner behavior

### 29.1 Continue by default

A controlled test failure does not stop the entire selected suite by default.

The runner continues with other selected top-level tests when the execution environment remains usable.

### 29.2 Fail fast

A CLI fail-fast policy may stop scheduling new top-level tests after a failure.

Fail-fast policy must not silently alter source-level parent/subtest control flow in the middle of an already executing test invocation.

### 29.3 Deterministic presentation

For the same test set and equivalent results, presentation order should be deterministic even if a future implementation executes tests concurrently.

### 29.4 No selected tests

Selecting or discovering zero tests is not itself a source-language test failure.

The runner must report the condition clearly.

CLI policy may distinguish an explicit filter that matched nothing from a project that simply contains no tests.

---

## 30. Exit status and aggregate result

### 30.1 Successful test command

The command returns success when all completed selected tests are:

```text
Passed
Skipped
```

and no compilation or execution infrastructure failure occurred.

### 30.2 All skipped

A run in which every executed test is `Skipped` is not automatically a failure.

The summary must make the absence of passed tests visible.

### 30.3 Unsuccessful command

The command returns non-zero when at least one selected test fails or the requested test run cannot be completed successfully.

### 30.4 Distinguish failure categories

Human-readable and structured output must distinguish at least:

```text
test failure
test compilation failure
execution unavailable
execution infrastructure failure
target fault/reset/timeout where known
```

A non-zero shell exit code does not justify collapsing these into one diagnostic category.

---

## 31. Test result reporting

### 31.1 Minimum test states

The runner must surface:

```text
PASS
FAIL
SKIP
```

for completed test invocations.

### 31.2 Required identity

A reported test result must be traceable to:

```text
test identity
module/test path
source declaration
```

### 31.3 Assertion failure information

Where applicable, a failure record contains:

```text
source location
failure kind
message
expected value
actual value
unexpected error
subtest identity
```

### 31.4 Multiple expectations

Several failed `testing.Expect` or `testing.ExpectEqual` calls in one invocation may produce several failure records.

The invocation has one primary outcome: `Failed`.

### 31.5 Structured result model

The harness and runner must use structured result data internally.

Human-readable output must not be the sole semantic representation of test results.

This permits future:

```text
JSON output
CI formats
JUnit-style export
IDE integration
hardware-farm reporting
```

without changing test semantics.

---

## 32. Test harness generation

### 32.1 Compiler-generated harness

The programmer does not write a test `main`.

The compiler generates the harness required to invoke selected tests.

### 32.2 No mandatory runtime registry

The harness may contain a statically generated set of selected test entries.

Sec testing must not require runtime test registration.

### 32.3 Production runtime independence

Ordinary Sec programs do not need to carry:

```text
test registry
test result collector
testing formatter
test transport
```

Testing support belongs only to TestCompilationPlan artifacts.

### 32.4 Runtime-free compatibility

A minimal test harness must be implementable on a runtime-free target.

It must not intrinsically require:

```text
heap allocation
filesystem
threads
operating system
garbage collection
reflection
```

---

## 33. Semantic IR and lowering

### 33.1 Explicit semantic identity

The frontend and canonical compiler representation must retain explicit test metadata before backend lowering.

Conceptually:

```text
TestDeclaration {
    Identity
    Name
    SourceLocation
    Entry
}
```

### 33.2 Test boundary

Semantic IR must preserve the fact that a test or subtest invocation is a test boundary for:

```text
unexpected Err propagation
Pass
Fail
Skip
Require termination
result attribution
cleanup
```

### 33.3 No backend reconstruction

MLIR or LLVM lowering must not reconstruct test semantics from function names or symbol spelling.

### 33.4 Generated support

Generated harness operations may lower to ordinary calls, target-specific support or compiler-known operations as appropriate.

Lowering must preserve the source-defined outcome and cleanup semantics.

---

## 34. Hosted and embedded execution

### 34.1 Target-independent source semantics

The same:

```sec
test "UART initializes" {
    ...
}
```

may execute as:

```text
host executable
simulator test image
emulator test image
RTOS test image
bare-metal firmware
```

when the selected target and execution provider support it.

### 34.2 Target code remains target code

Cross-compiled tests execute under the selected target's real semantic and ABI assumptions.

A target test is not silently recompiled for the host merely because the host runs `sec test`.

### 34.3 Separate test artifact

A test build produces a test artifact separate from the production artifact.

Test-only code may be present in the test artifact without increasing the production binary.

### 34.4 Selected-test artifacts

An execution provider may build or deploy only the selected test set plus required production code and harness support.

---

## 35. Test execution providers

### 35.1 Execution-provider abstraction

Test execution after compilation is performed through a compiler/tooling execution provider.

Conceptually the provider may support capabilities such as:

```text
deploy
start
reset
collect test results
interrupt execution
debug
report faults
```

### 35.2 Provider is not necessarily a debugger

A provider may be:

```text
native host runner
simulator
emulator
debug probe integration
hardware farm
target-specific launcher
```

Debugging is an optional provider capability.

### 35.3 Buildable versus executable

A target may support test artifact compilation while no local execution provider is configured.

The toolchain must distinguish:

```text
test artifact built successfully
```

from:

```text
tests executed successfully
```

### 35.4 Result transport

Testing defines logical test-result events.

It does not define the physical transport.

Possible provider transports include:

```text
debug-probe channel
semihosting
debug monitor
simulator channel
shared memory
mailbox
UART chosen explicitly by the provider
target-defined transport
```

### 35.5 Tested peripherals must not be implicitly consumed

Testing must not assume that an application peripheral such as a UART is available for test reporting.

The execution provider owns its reporting resources.

A test result transport must not silently interfere with the hardware resource being tested.

---

## 36. Embedded result model

### 36.1 Logical events

A target harness may logically report events such as:

```text
TestRunStarted
TestStarted
AssertionFailed
TestPassed
TestFailed
TestSkipped
TestRunFinished
```

The exact event encoding is not defined here.

### 36.2 Compact target identity

The target artifact need not carry full human-readable test names and file paths.

The compiler may assign compact static identities such as:

```text
TestId
AssertionId
```

while the host retains the corresponding source metadata.

### 36.3 Runtime values

Actual runtime values required for diagnostics may be transported in a target-appropriate structured representation.

The testing model does not require a general reflection runtime.

### 36.4 Heap-free target reporting

A conforming minimal embedded test harness must be capable of reporting ordinary outcomes without requiring hidden heap allocation.

---

## 37. Faults, reset and timeout

### 37.1 Controlled failure versus execution fault

A failure produced by:

```text
testing.Expect
testing.Fail
unexpected Err
```

is distinct from:

```text
processor fault
unhandled panic
process crash
target reset
lost execution provider
```

### 37.2 Fault attribution

Where the provider can identify the active test, an execution fault should be attributed to that test identity and reported as an execution fault rather than fabricated as an ordinary expectation failure.

### 37.3 Reset

If the target resets during a test, the provider should report the reset and active test identity where known.

### 37.4 Timeout policy

A universal source-language timeout is not defined for Sec 0.1.

Timeout policy belongs to the runner, project configuration or execution provider.

### 37.5 Timeout outcome

A timeout is an execution failure category, not `Skipped`.

### 37.6 Recovery and continuation

After a fault, reset or timeout, the execution provider determines whether:

```text
execution can continue
the target must be restarted
the artifact must be redeployed
the remaining tests cannot be executed
```

The testing model does not require every selected test to run in one process, one boot or one firmware invocation.

---

## 38. Test environment and resources

### 38.1 Environment capabilities

Tests may use ordinary platform/environment features when the selected target provides them.

Examples include:

```text
filesystem
network
clock
environment variables
hardware devices
```

Testing does not guarantee that these exist.

### 38.2 Current working directory

The current working directory is not part of portable Sec test semantics.

A portable test must not rely on the runner choosing the source-module directory as its working directory unless an explicit project/tooling contract provides that behavior.

### 38.3 No magic `testdata` semantics in 0.1

Sec 0.1 does not define:

```text
automatic testdata discovery
automatic test resource embedding
testing.TestData(...)
```

A future resource model may provide portable host/embedded test resources.

### 38.4 Environment variables

Environment variables use ordinary platform APIs where available.

Testing does not create source-visible magic variables such as:

```text
SEC_TEST_NAME
SEC_TEST_ROOT
```

### 38.5 CLI runner arguments

Arguments passed to `sec test` control the test runner unless explicitly defined otherwise.

They do not automatically become program arguments visible to each test invocation.

---

## 39. LSP and editor integration

### 39.1 Compiler workspace is authoritative

The compiler workspace discovers test declarations and produces canonical test identities and source ranges.

The LSP must consume this compiler-owned information.

### 39.2 No duplicate parser in the editor

An editor extension must not implement an independent Sec test parser to discover:

```text
*_test.sec
test "..."
```

when compiler workspace metadata is available.

### 39.3 Test compilation view

The LSP must be able to analyze:

```text
production compilation view
test compilation view
```

so that test-only declarations, visibility, imports and compiler-known `testing` members are understood without contaminating production analysis.

### 39.4 Runnable test metadata

The LSP/editor integration should expose at least:

```text
test identity
test name
module
source range
test category
```

where test category distinguishes same-module white-box tests from project integration-test modules.

### 39.5 Run and debug actions

Where supported by the client, the editor integration should provide:

```text
Run Test
Debug Test
Run Tests in File
Run Tests in Module
Run Project Tests
```

using the canonical compiler test selection model.

### 39.6 Standard LSP versus editor-native UI

Standard LSP features such as CodeLens may expose runnable test commands.

A richer editor-native testing UI may be implemented by the editor extension/client.

The client remains thin: discovery and semantics come from the compiler workspace.

### 39.7 Same execution path

Editor-triggered test execution must use the same TestCompilationPlan and result semantics as `sec test`.

An editor must not define a second test runner with different language behavior.

### 39.8 Result presentation

The IDE should surface:

```text
Passed
Failed
Skipped
failure source locations
expected/actual data where available
unexpected errors
logs
execution faults
```

through the editor's available test UI.

---

## 40. Diagnostics

### 40.1 Required parser and Sema diagnostics

Diagnostics must cover at least:

```text
test declaration outside *_test.sec
test declaration not at top level
empty top-level test name
duplicate top-level test name
test with parameters
test with source-visible return type
returning a value from a test
production dependency on test-only declaration
invalid use of testing outside test context
invalid testing.Expect argument type
invalid testing.Require argument type
illegal equality passed to ExpectEqual/RequireEqual
empty subtest name
duplicate sibling subtest name
```

### 40.2 Good diagnostic structure

Diagnostics should state:

```text
what test rule was violated
the relevant test identity when known
the source location
related declaration locations when useful
a concrete correction when one is safe
```

### 40.3 Failure diagnostics are not compiler diagnostics

A source program may compile successfully and then produce test failure records.

The implementation must distinguish compile-time diagnostics from runtime test-result diagnostics.

---

## 41. Formatter requirements

### 41.1 Canonical syntax

The formatter must support:

```sec
test "name" {
    ...
}
```

as a top-level declaration.

### 41.2 Ordinary block formatting

The test body uses ordinary Sec block formatting.

### 41.3 No name rewriting

The formatter must not rewrite the test name string.

### 41.4 Testing calls

Calls to `testing.*` use ordinary call formatting.

No testing-specific alternate indentation or assertion DSL is required.

---

## 42. Implementation requirements

### 42.1 Parser

The frontend must parse and retain a distinct test declaration node or equivalent canonical declaration identity.

It must not infer tests from function names.

### 42.2 Source loader

The source loader must select `*_test.sec` only for test compilation.

It must preserve production validity independently of test-only source.

### 42.3 Sema

Sema must implement:

```text
test context
compiler-known testing namespace
test error boundary
terminal testing operations
non-fatal failure recording model
test-only dependency direction
test identity
subtest semantics
```

### 42.4 Compiler workspace

The shared compiler workspace must provide test discovery and test views reusable by:

```text
sec test
LSP
VS Code/editor integration
future build systems
```

### 42.5 Harness

The compiler must generate a deterministic harness for the selected test set.

### 42.6 Lowering

Lowering must preserve:

```text
test boundaries
cleanup
unexpected-error outcomes
terminal test operations
subtest parent/child identity
result attribution
```

### 42.7 Target support

Test execution support is target/provider coverage.

A source test feature is not considered universally implemented merely because hosted native execution exists.

---

## 43. Required implementation tests

### 43.1 Parser tests

Required parser coverage includes:

```text
valid top-level test
invalid test in ordinary .sec file
invalid nested test
empty name
test parameters rejected
test return type rejected
parser recovery after malformed test declaration
formatter round-trip
```

### 43.2 Sema tests

Required Sema coverage includes:

```text
same-module module-internal access
source-file-private access remains rejected
production cannot depend on test-only helper
test may depend on production helper
test-only import selection
testing namespace only in test context
bare return
return value rejection
unexpected Err to test failure
expected Err handling
Pass
Fail
Skip
Expect
Require
ExpectEqual
RequireEqual
failure monotonicity
cleanup on every controlled terminal path
```

### 43.3 Subtest tests

Required coverage includes:

```text
synchronous Run
hierarchical identity
Passed child
Failed child marks parent failed
Skipped child does not skip parent
nested child
duplicate runtime sibling name
empty runtime name
cleanup before Run returns
Pass/Fail/Skip apply to current child
```

### 43.4 Discovery tests

Required coverage includes:

```text
*_test.sec excluded from production
*_test.sec included in test mode
project-root sec test discovers project tests
dependency tests are not automatically executed
integration tests below tests/
stable test identity
deterministic filtering
single-test selection before final linking where supported
```

### 43.5 CLI tests

Required coverage includes:

```text
all pass
one fail
all skipped
mixed pass/skip
compilation failure
execution unavailable
fail-fast scheduling
zero selected tests
structured result preservation
exit status
```

### 43.6 LSP tests

Required coverage includes:

```text
test discovery from compiler workspace
test CodeLens metadata where supported
Run Test selection
Debug Test selection
same-module test diagnostics
integration-test diagnostics
test-only imports
stable source ranges
no editor-side duplicate parsing requirement
```

### 43.7 Embedded/provider tests

Where a provider exists, required coverage includes:

```text
test artifact deployment
compact TestId mapping
pass/fail/skip transport
assertion source attribution
target fault attribution
target reset reporting
timeout reporting
provider restart behavior
selected-test artifact
no hidden heap requirement for minimal harness
```

---

## 44. Future testing facilities

### 44.1 Benchmarking

Benchmarking is an intended future testing/tooling area.

Sec 0.1 does not define:

```text
benchmark declaration syntax
benchmark timing model
iteration model
warmup
statistical reporting
sec benchmark command semantics
```

These decisions require a separate future design.

### 44.2 Fuzzing

Fuzz testing is an intended future testing/tooling area.

Sec 0.1 does not define:

```text
fuzz declaration syntax
input-generation model
seed corpus
shrinking
coverage guidance
reproduction format
sec fuzz command semantics
```

These decisions require a separate future design.

### 44.3 Executable documentation examples

Executable documentation examples are a useful future feature but belong at the boundary between testing, documentation and tooling.

They are not defined by Sec 0.1 testing.

---

## 45. Required cross-rulebook synchronization

### 45.1 Grammar

`rules/foundations/grammar.md` must add the canonical top-level `test` declaration grammar.

### 45.2 Core library

`rules/library/core-library.md` must reserve the compiler-known test-context `testing` namespace and distinguish it from ordinary stdlib imports.

### 45.3 Modules

`rules/projects/modules.md` must define test-mode file participation and remain consistent with the direct logical standard-library import model.

### 45.4 Projects

`rules/projects/projects.txt` must define:

```text
project-root tests/ source tree
TestCompilationPlan integration
test-only dependencies
ordinary sec test not requiring a manifest test target
```

### 45.5 Error handling

`rules/errors/errorhandling.md` must recognize test and subtest invocations as compiler-known `try` propagation boundaries with test-failure outcomes.

### 45.6 Cleanup

`rules/control-flow/defer.md` and `rules/memory/destruction.md` must remain consistent with controlled test termination executing ordinary cleanup.

### 45.7 Compiler pipeline and Semantic IR

`rules/compiler/compiler_pipeline.txt` and `rules/compiler/semantic_ir.txt` must preserve test identity, test boundaries, selected harness roots and test compilation mode before lowering.

### 45.8 Diagnostics and formatter

`rules/tooling/diagnostics.txt` and `rules/tooling/formatter.md` must recognize testing syntax and diagnostics.

### 45.9 LSP

`rules/tooling/lsp.md` must expose compiler-owned test discovery and editor execution integration.

### 45.10 Debug and execution

Future debug/execution-provider rules must support structured test execution where the selected environment provides such a provider.

The result transport is not Sec source semantics.

### 45.11 Rulebook inventory

`language-rulebook-status.md` must add:

```text
tooling/testing.md
```

as a written rulebook and must keep the planned compiler-internal `compiler_testing.md` distinct.

---

## 46. Governance

### 46.1 Mutable implementation state

Current implementation progress does not belong in this rulebook.

It belongs in:

```text
implementation-status.yaml
```

### 46.2 Suggested integration identity

The canonical ledger should contain an integration entry equivalent to:

```text
tooling.language-testing
```

### 46.3 Completion rule

Testing is not end-to-end implemented merely because one layer exists.

A complete claimed target slice requires the applicable parts of:

```text
source discovery
parser
Sema
test compilation
harness generation
lowering/linking
execution provider
result collection
CLI
diagnostics
LSP/editor integration where claimed
```

### 46.4 Roadmap relationship

The Sec 0.1 goal is to define the first complete language revision, including the testing semantics in this rulebook.

Dogfooding and external feedback in the Sec 0.2 phase may refine ergonomic details through the normal correction process.

Future benchmark, fuzz, profiling and documentation work does not change the Sec 0.1 test contract unless a later normative revision explicitly does so.

---

## 47. Normative summary

### 47.1 Source model

Sec tests use:

```text
*_test.sec
test "name" { ... }
sec test
```

White-box tests live beside production code.

Integration tests live below project-root `tests/` and use public imports.

### 47.2 Core testing surface

Test compilation provides compiler-known:

```text
testing.Pass
testing.Fail
testing.Skip
testing.Log
testing.Expect
testing.Require
testing.ExpectEqual
testing.RequireEqual
testing.Run
testing.TestResult
```

without an import.

### 47.3 Ordinary language semantics

Tests use ordinary Sec:

```text
types
equality
Result/Err
try/match
ownership
borrowing
defer
destruction
concurrency
hardware access
```

except for the explicit test boundary defined here.

### 47.4 Execution model

Top-level tests are compiler-discovered.

Subtests are nested synchronous invocations.

Controlled failures do not normally abort the suite.

Tests do not semantically depend on execution order or automatic external-state reset.

### 47.5 Tooling model

The compiler generates the test harness and owns test discovery.

`sec test`, LSP and editor integrations consume the same compiler TestCompilationPlan semantics.

### 47.6 Embedded model

Testing is not host-only.

A target-specific execution provider may deploy a test artifact and transport structured results through debug, simulator, hardware-farm or other platform-defined mechanisms without changing source test semantics.

### 47.7 Future work

Benchmarking, fuzzing, executable documentation examples and system/E2E orchestration remain future design areas.

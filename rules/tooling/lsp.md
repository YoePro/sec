# Language Server Protocol

## Status

This document is the canonical rulebook for the Sec language server.

The language server already exists and provides a useful initial feature set.

This rulebook:

- records the current implementation accurately;
- defines the required architecture;
- defines the complete target feature set;
- defines the relationship between the compiler, formatter, fix engine, and LSP;
- defines Sec-specific extensions;
- defines future knowledge-pack integration;
- provides an implementation plan for Codex.

The canonical executable remains:

```text
lsp-sec
```

The current entry point is:

```text
cmd/lsp
```

The implementation must be reorganized without discarding working behavior.

---

# Vision

The Sec language server must be better than any competing language server in
the practical quality of its assistance.

This means more than feature count.

The server must be:

- more accurate;
- more explanatory;
- more aware of ownership and effects;
- more useful on incomplete source;
- more capable of safe correction;
- more capable of structural refactoring;
- more target-aware;
- more transparent about consequences;
- more tightly integrated with the compiler;
- more predictable;
- more responsive;
- more deterministic.

The server must expose the knowledge already produced by the Sec compiler
instead of recreating a weaker editor-specific approximation.

The language server must never silently invent language semantics.

---

# Fundamental rules

## One semantic source of truth

The compiler workspace is the sole owner of:

```text
lexical truth
syntax truth
AST truth
name resolution
type resolution
generic resolution
overload resolution
contract analysis
unit analysis
ownership analysis
borrow analysis
lifetime analysis
effect analysis
call graph analysis
stack analysis
allocation analysis
escape analysis
ISR analysis
concurrency analysis
target analysis
diagnostic truth
```

The LSP must not maintain a separate lexer, parser, AST, semantic analyzer,
type system, ownership model, or diagnostic system.

Every diagnostic presented by the LSP must originate from the same diagnostic
model used by:

```text
sec check
sec build
sec run
sec test
sec fmt --fix
```

The LSP is a protocol adapter and interactive client of compiler services.

The compiler owns one canonical semantic call graph for each concrete
`CompilationPlan`. The LSP consumes that graph and its compiler-produced
analysis views. It must not construct, cache as semantic truth, or incrementally
maintain a second LSP-specific call graph.

---

## Shared formatter

LSP formatting must call the same formatter implementation as:

```text
sec fmt
```

LSP safe fixes must call the same fix engine as:

```text
sec fmt --fix
```

The LSP may expose all formatter functionality or a negotiated subset.

It must not contain an independent formatter implementation.

The editor extension must not contain Sec formatting rules.

---

## Complete LSP support

The Sec language server targets the complete Language Server Protocol 3.18
surface.

The server must:

- implement the complete base protocol correctly;
- negotiate capabilities correctly;
- implement every applicable standard language and workspace capability;
- respond correctly to supported and unsupported optional requests;
- never advertise a capability that is not functionally implemented;
- never crash because a client sends a valid but unknown future enum value;
- preserve compatibility with clients supporting earlier negotiated subsets.

Features not meaningful for Sec must still follow protocol rules.

They may remain unadvertised or return a valid empty result as required by the
protocol.

---

## Error-tolerant editing

The parser and compiler workspace must preserve useful structure around
recoverable syntax errors.

A recoverable error must not unnecessarily remove:

```text
semantic highlighting
completion
hover
document symbols
navigation
references
formatting of unaffected regions
code actions
```

Where a syntax repair is unambiguous, the compiler must produce a structured,
machine-applicable fix.

Example:

```sec
fn myfunc(a string) void {
}
```

Diagnostic:

```text
expected `:` between parameter name `a` and type `string`
```

Fix:

```sec
fn myfunc(a: string) void {
}
```

The fix may be applied:

- explicitly through a quick fix;
- on save when safe fixes on save are enabled;
- by `sec fmt --fix`.

Ordinary `sec fmt` does not apply this repair.

---

## Expose all compiler knowledge

Every useful fact available from the compiler pipeline should be exposable
through standard LSP features or namespaced Sec extensions.

This includes facts from:

```text
lexer
parser
AST
Sema
Semantic IR
call graph
ownership analysis
borrow analysis
lifetime analysis
contract analysis
unit analysis
effect analysis
allocation analysis
escape analysis
stack analysis
ISR analysis
concurrency analysis
target analysis
lowering analysis
```

The LSP must not wait for a separate editor-specific reimplementation of an
analysis that already exists in the compiler.

---

## No silent semantic edits

The language server must never silently change program meaning.

Automatic text changes are divided into:

```text
canonical formatting
syntax normalization
safe compiler fix
code cleanup
refactoring
behavior-changing transformation
```

Each category has separate permissions.

The LSP must explain ownership-changing and behavior-changing edits before
application.

---

# Current implementation status

The current implementation is substantial and must be preserved during
reorganization.

The status below reflects the repository implementation in `cmd/lsp` and
`vscode`.

---

## Implemented

### Transport and lifecycle

Implemented:

- JSON-RPC transport over stdio;
- `Content-Length` message framing;
- `initialize`;
- `initialized`;
- `shutdown`;
- `exit`;
- JSON-RPC success responses;
- JSON-RPC error responses;
- explicit `null` results.

### Document synchronization

Implemented:

- `textDocument/didOpen`;
- `textDocument/didChange`;
- in-memory open-document text;
- full-document synchronization;
- debounced diagnostics;
- a current diagnostic delay of 600 milliseconds.

### Diagnostics

Implemented:

- lexer/parser execution through compiler packages;
- Sema execution through the compiler Sema package;
- core source loading;
- same-directory, same-module source assembly through the compiler lexer,
  parser, AST, and Sema pipeline;
- open-document overlays for active and sibling module files;
- dependent diagnostic refresh when a sibling module document opens, changes,
  saves, or closes;
- project import loading;
- stdlib import loading for currently recognized modules;
- transitive import loading;
- Sema diagnostic IDs;
- Sema severity mapping;
- Sema help text in LSP diagnostics;
- parser diagnostics mapped to a generic parser diagnostic ID;
- push diagnostics through `textDocument/publishDiagnostics`.

### Formatting

Implemented:

- full-document formatting;
- line-ending preservation;
- indentation;
- whitespace normalization;
- grouped import indentation;
- switch and select indentation;
- typed declaration-group indentation;
- single-line function signature normalization;
- declaration-list normalization;
- unambiguous `func` to `fn` normalization;
- canonical `@noCopy` line placement through the shared formatter;
- protection against rewriting a normal identifier or call named `func`;
- whole-document replacement edits;
- format-on-save enabled by default in the VS Code extension.

### Completion

Implemented:

- global completion;
- member completion;
- completion trigger on `.`;
- keyword completion;
- type-position completion;
- intrinsic type completion;
- core type completion;
- contract-modifier completion;
- member completion for fields, properties, events, and methods;
- type-qualified completion for enum members, union variants, and legal static
  struct/union members;
- completion for `self`;
- expected return-type filtering;
- limited completion recovery for incomplete member expressions;
- limited completion recovery for incomplete `if` conditions;
- survival of incomplete function source in tested cases.

### Hover

Implemented:

- markdown hover;
- symbol hover;
- documentation-comment hover;
- function hover;
- field hover;
- property hover;
- event hover;
- method hover;
- `self` member resolution;
- type-aware hover through Sema;
- compiler-owned call-graph summaries for resolved functions and methods,
  including incoming/outgoing call-site counts, current root reachability, and
  represented same-stack recursion components;
- task-spawn and thread-start edge counts plus derived entry-root reachability;
- source-ordered direct Arena effects, propagated `MayAllocate`, and the
  compiler-owned shortest synchronous allocation cause path.

### Navigation

Implemented:

- `textDocument/definition` capability advertisement and request handling;
- compiler-owned use-to-declaration bindings produced during Sema;
- definitions for local variables and parameters;
- definitions for named types and generic type parameters;
- definitions for resolved functions and the selected overload;
- definitions for fields, register fields, properties, events, enum values,
  union variants, and resolved methods, including `self` member access;
- cross-file definitions in imported project, core, and stdlib sources;
- exact unqualified declaration ranges after imported declarations are
  semantically qualified;
- `null` results for unresolved and tested incomplete source instead of guessed
  destinations;
- `textDocument/documentHighlight` capability advertisement and request
  handling;
- semantic same-document highlights based on Sema declaration bindings rather
  than identifier spelling;
- separate highlights for shadowed declarations and resolved `self` members;
- LSP `Write` classification for declarations and direct assignment targets,
  with other occurrences classified as `Read`;
- empty highlight results for unresolved, ambiguous, and tested incomplete
  source;
- `textDocument/prepareCallHierarchy`, `callHierarchy/incomingCalls`, and
  `callHierarchy/outgoingCalls` capability advertisement and request handling;
- compiler-owned callable and call-site identities consumed by call hierarchy;
- grouped incoming and outgoing synchronous direct-call results for functions
  and statically resolved methods;
- defensive empty call-hierarchy results for stale, unresolved, or tested
  incomplete source;
- current `ProgramEntryRoot` reachability and same-stack recursion information
  from the compiler graph, reused by hover rather than recomputed in the LSP;
- task-spawn and thread-start call-hierarchy entries that preserve dispatch and
  execution relation in item data and do not merge with synchronous calls to
  the same target.

The initial implementation resolves from the current immutable document text
and compiler-loaded source set. A persistent workspace symbol index,
declaration/type-definition distinction, implementation navigation, references,
rename, indirect/open call targets, non-synchronous relationships beyond the
implemented task/thread spawn edges, root classes beyond program/task/thread
entry, complete path-sensitive graph pruning, per-`CompilationPlan` graph
snapshots, and richer copy/move/borrow occurrence classification remain pending.

### Document symbols

Implemented:

- module symbols;
- type symbols;
- struct symbols;
- union symbols;
- enum symbols;
- enum-member symbols;
- interface symbols;
- function symbols;
- variable symbols;
- grouped declaration symbols;
- impl symbols;
- property symbols;
- event symbols;
- nested symbol children;
- selection ranges contained by symbol ranges.

### Semantic tokens

Implemented:

- `textDocument/semanticTokens/full`;
- lexical fallback when semantic analysis is unavailable;
- semantic classification using compiler types, functions, and symbols;
- standard token types including namespace, type, enum, interface, struct,
  parameter, variable, property, event, function, method, keyword, comment,
  string, number, and operator;
- standard `variable` token classification with the `readonly` modifier for
  immutable bindings at both their declaration and resolved use sites;
- declaration and static modifiers;
- compiler-known attribute-name classification as `modifier`, including the
  implemented `@noCopy` path;
- resilience against selected panics during active editing.

`new` is classified as a keyword. Bare lifecycle `init` and `free` members are
recognized contextually inside impl bodies without changing the classification
of an ordinary function named `init`.

Hover and signature help for an initializer show its parameters, constructed
type, and optional construction error separately. Hover on `new Type(args...)`
shows the selected init overload and whether `try`/handling is required.
Definition/references connect lifecycle construction to the selected explicit
init (or identify the permitted implicit path). Completion offers `init` and
`free` only where valid in an impl body and offers `new` at expression sites.

### Project and source integration

Implemented:

- core loading from `sec/core`;
- project-root discovery through `.sec/sec.toml`;
- project source import resolution;
- grouped imports;
- platform import resolution;
- loading all `.sec` files in a module directory;
- several current stdlib module mappings;
- short import qualifier rewriting;
- transitive source imports.

### VS Code extension

Implemented:

- VS Code language activation;
- `.sec` and `.se` file recognition;
- TextMate grammar;
- language configuration;
- language-client startup;
- configurable language-server executable path;
- bundled executable lookup;
- filesystem watcher;
- output channel;
- format-on-save default;
- semantic token color customizations for current unit scopes.

### Tests

Implemented tests cover, among other areas:

- compiler diagnostics;
- stable Sema diagnostic IDs;
- help text;
- core loading;
- stdlib imports;
- transitive imports;
- project-root imports;
- platform imports;
- grouped imports;
- completion;
- member completion;
- expected return types;
- contract completion;
- hover;
- documentation comments;
- `self` members;
- document symbols;
- semantic tokens;
- incomplete-source resilience;
- formatter behavior;
- typed declaration groups;
- unambiguous `func` normalization.

---

## Partially implemented

### Shared compiler workspace

The LSP uses the compiler's lexer, parser, AST, Sema, and diagnostics packages.

However, it still directly orchestrates compilation independently inside
`cmd/lsp`.

The LSP currently constructs:

```text
lexer
parser
core source inclusion
import inclusion
Sema analyzer
```

for multiple features.

This must move into a shared compiler workspace and snapshot API.

### Diagnostics

Sema diagnostics are structured.

Parser diagnostics are currently plain strings whose positions are recovered by
parsing message text.

Parser diagnostics must become structured compiler diagnostics containing:

```text
stable ID
source range
severity
message
notes
help
machine-applicable fixes
recovery metadata
```

### Formatter

The formatter is currently implemented directly inside `cmd/lsp/main.go`.

It is not yet shared with `sec fmt`.

The implementation is useful but architecturally misplaced.

### Incomplete-source handling

Completion and semantic tokens contain selected recovery behavior.

The central LSP parse paths now consume the parser's `ParseResult`, including
structured diagnostics, recovery context, episode association and error state.
The partial AST retains failed declarations, statements, impl members,
match/handler patterns, and selected invalid expressions/types. Episode-aware
deduplication reduces duplicate parser diagnostics before publication. A
shared delimiter stack now keeps migrated recovery scans inside their nested
parenthesis, bracket, and brace boundaries and exposes their skipped ranges to
tooling.

The general parser and compiler workspace do not yet provide a complete
error-tolerant syntax tree.

After a non-fatal parser error, the LSP may continue semantic analysis over
retained declarations and unaffected subtrees. It must not publish semantic
diagnostics whose primary location lies inside a parser recovery range, and a
local malformed body must not hide independent declaration diagnostics in
following siblings.

### Project loading

Project and stdlib loading exist but remain feature-specific and partly
hardcoded.

The final compiler workspace must resolve all core, stdlib, project, platform,
target, and generated sources through the canonical module system.

### Document snapshots

Open-document text now has a versioned snapshot store. `didOpen` and full-text
`didChange` updates record versions and reject stale updates. Feature handlers
read immutable snapshots. `didClose` removes the overlay and clears published
diagnostics; `didSave` refreshes diagnostics. `willSave` and
`willSaveWaitUntil` are supported, with the latter currently returning no edits.
Incremental edits and stale-result suppression remain pending.

### Protocol transport and types

JSON-RPC framing and envelope types now live in `internal/lsp/protocol`.
Request routing and most LSP feature parameter types remain in `cmd/lsp` until
the server and feature packages are extracted.

The types are still handwritten. This is sufficient for the present subset but
not for complete LSP 3.18 coverage.

---

## Not implemented

The following major areas are not yet implemented:

- complete LSP 3.18 protocol support;
- generated LSP protocol types;
- capability-aware protocol routing;
- dynamic registration;
- incremental document synchronization;
- document version validation;
- stale-result suppression;
- request cancellation;
- progress reporting;
- partial results;
- work-done progress;
- pull diagnostics;
- workspace diagnostics;
- diagnostic related information;
- diagnostic tags;
- diagnostic data payloads;
- code action resolve;
- safe fix engine;
- fix-all;
- safe fixes on save;
- missing-colon repair;
- declaration;
- type definition;
- implementation navigation;
- references;
- rename;
- prepare rename;
- signature help;
- workspace symbols;
- workspace-symbol resolve;
- code lens;
- code-lens resolve;
- inlay hints;
- inlay-hint resolve;
- inline values;
- inline completion;
- compiler-owned call-graph queries and per-`CompilationPlan` graph snapshots;
- indirect target sets, open callable contracts, root reachability, and
  call-graph cause paths;
- type hierarchy;
- folding ranges;
- selection ranges;
- linked editing;
- document links;
- range formatting;
- on-type formatting;
- semantic-token range requests;
- semantic-token delta requests;
- import code actions;
- organize imports;
- refactoring;
- code cleanup;
- target switching;
- multi-target diagnostics;
- ownership visualization;
- contract-proof visualization;
- unit metadata hover;
- compiler-analysis code lenses;
- Sec-specific protocol requests;
- knowledge packs.

---

# Architecture

## Required package structure

The implementation should move toward:

```text
cmd/lsp/
    main.go

internal/lsp/
    protocol/
    server/
    features/
    sec/

internal/compiler/workspace/
internal/compiler/snapshot/
internal/formatter/
internal/fixes/
internal/refactor/
```

Suggested feature files:

```text
internal/lsp/features/
    completion.go
    hover.go
    signature_help.go
    diagnostics.go
    formatting.go
    code_actions.go
    definition.go
    references.go
    rename.go
    document_symbols.go
    workspace_symbols.go
    semantic_tokens.go
    inlay_hints.go
    code_lens.go
    call_hierarchy.go
    type_hierarchy.go
    folding.go
    selection.go
    links.go
```

Suggested Sec-specific files:

```text
internal/lsp/sec/
    ownership.go
    contracts.go
    units.go
    targets.go
    analysis.go
    lowering.go
```

`cmd/lsp/main.go` should eventually contain only:

```text
process startup
stdio selection
server construction
logging setup
exit status
```

---

## Compiler workspace

A long-lived compiler workspace owns:

```text
workspace folders
project manifests
targets
source files
open-document overlays
core declarations
stdlib indexes
module graph
syntax trees
semantic snapshots
symbol indexes
dependency graph
analysis caches
diagnostic configuration
```

The LSP does not construct isolated frontend pipelines per feature.

Conceptual API:

```text
workspace.OpenDocument(uri, version, text)
workspace.ChangeDocument(uri, version, edits)
workspace.CloseDocument(uri)
workspace.Snapshot(uri, version, target)
workspace.Check(snapshot)
workspace.Complete(snapshot, position)
workspace.Hover(snapshot, position)
workspace.References(snapshot, position)
workspace.Format(snapshot, options)
workspace.Fixes(snapshot, range)
workspace.Refactor(snapshot, request)
```

The same workspace services should be reusable by CLI tools.

---

## Snapshots

Every request operates on an immutable workspace snapshot.

A snapshot includes:

```text
document versions
project manifest version
active target
output variant
CompilationPlan identity
profile
feature set
diagnostic policy
core version
stdlib version
module graph version
call graph version
```

Results from an older snapshot must not overwrite results for a newer document
version.

Analysis is deterministic for a given snapshot.

Call-graph results are valid only for the exact snapshot and
`CompilationPlan` from which the compiler produced them. A target switch,
active implementation-variant change, relevant declaration change, or changed
indirect target set invalidates affected graph facts. Partial or completed
results from an older graph snapshot must not overwrite newer results.

---

## Incremental analysis

The workspace must invalidate only affected results.

Dependency tracking should distinguish:

```text
lexical change
local syntax change
signature change
type-shape change
impl change
contract change
unit change
import change
target change
manifest change
core or stdlib change
```

The system may reuse:

```text
unchanged tokens
unchanged syntax subtrees
unchanged declarations
unchanged generic instantiations
unchanged symbol indexes
unchanged call graph regions
unchanged target analyses
```

Correctness takes priority over reuse.

---

# LSP protocol requirements

## Protocol version

The target protocol version is:

```text
LSP 3.18
```

Protocol types should be generated from the official LSP 3.18 meta model where
practical.

Generated enum handling must preserve unknown values when the protocol permits
future extension.

---

## Base protocol

Required:

- correct headers;
- UTF-8 JSON payloads;
- request IDs;
- notifications;
- responses;
- error responses;
- batch behavior where supported by the chosen JSON-RPC profile;
- cancellation;
- invalid-request handling;
- method-not-found handling;
- internal-error handling;
- server-not-initialized handling.

One malformed request must not terminate the server unless transport integrity is
lost.

---

## Lifecycle

Required:

```text
initialize
initialized
shutdown
exit
```

The server must reject invalid lifecycle ordering according to the protocol.

Initialization must inspect client capabilities before advertising server
capabilities.

---

## Document synchronization

Required:

```text
didOpen
didChange
didClose
didSave
willSave
willSaveWaitUntil
```

Incremental synchronization should be the preferred mode.

Full synchronization may remain as a negotiated fallback.

Document versions are mandatory for change ordering.

---

## Language features

The server must implement every applicable standard feature:

```text
completion
completion resolve
hover
signature help
declaration
definition
type definition
implementation
references
document highlight
document symbols
workspace symbols
workspace symbol resolve
code actions
code action resolve
code lens
code lens resolve
formatting
range formatting
on-type formatting
rename
prepare rename
linked editing
folding ranges
selection ranges
document links
document link resolve
semantic tokens full
semantic tokens delta
semantic tokens range
inlay hints
inlay hint resolve
inline values
inline completion
call hierarchy
type hierarchy
document diagnostics
workspace diagnostics
```

Features must use compiler facts and shared indexes.

---

## Workspace features

Required where applicable:

```text
workspace folders
configuration changes
watched files
create file operations
rename file operations
delete file operations
workspace symbols
workspace diagnostics
execute command
apply edit
```

Workspace edits must be version-aware.

---

## Window and progress features

Required:

```text
show message
show message request
log message
show document
work-done progress
partial results
```

Long analyses must support cancellation and progress reporting.

Call hierarchy, root reachability, path explanations, and graph visualization
may return partial results when the client supports them. Every result must
identify its snapshot and active `CompilationPlan`; cancellation must stop
further traversal without publishing an incomplete result as a complete closed
target set or complete cause path.

---

# Responsiveness model

The LSP must remain interactive while deep analysis continues.

## Immediate tier

Target response:

```text
within one editor frame where practical
```

Includes:

- lexical tokens;
- bracket and delimiter knowledge;
- local syntax diagnostics;
- obvious syntax fixes;
- basic local completion;
- local document symbols.

## Fast tier

Includes:

- local Sema;
- type-aware completion;
- hover;
- signature help;
- ownership state;
- contract state;
- unit resolution;
- local code actions.

## Deep tier

Includes:

- workspace references;
- call graph;
- stack analysis;
- allocation paths;
- escape analysis;
- all-target analysis;
- race analysis;
- deadlock analysis;
- large refactorings.

Deep results arrive asynchronously but are tied to the exact snapshot.

---

# Error-tolerant syntax model

## Recovery nodes

The parser must support explicit recovery structures such as:

```text
MissingToken
ErrorExpression
ErrorStatement
RecoveredDeclaration
RecoveredParameter
RecoveredType
RecoveredBlock
```

Conceptual recovered parameter:

```text
Parameter {
    Name: a
    MissingColon: true
    Type: string
}
```

The recovered node lets the LSP understand intended structure while preserving
the error.

---

## Stable syntax identity

Unchanged declarations and subtrees should retain stable identities across
nearby edits where practical.

This improves:

- semantic-token deltas;
- diagnostics stability;
- code-lens stability;
- symbol navigation;
- refactoring previews;
- incremental analysis.

---

## Recovery limits

Recovery must not fabricate arbitrary semantics.

A recovered fact must record confidence:

```text
exact
unambiguous recovery
probable recovery
unknown
```

Only exact and unambiguous recoveries may produce automatic safe fixes.

---

# Formatting and fixing

## Shared pipeline

The canonical edit pipeline is:

```text
source snapshot
    -> optional safe fixes
    -> structured edits
    -> canonical formatter
    -> text edits
```

A code action or refactoring must not hand-format its generated text.

It constructs a structured edit and delegates canonical output to the formatter.

---

## Ordinary formatting

Used by:

```text
sec fmt
textDocument/formatting
textDocument/rangeFormatting
format on save
```

Ordinary formatting:

- preserves semantics;
- does not require successful Sema;
- preserves comments;
- preserves literal values;
- preserves string contents;
- preserves ordinary copy versus move syntax;
- produces canonical Sec style;
- is idempotent.

It may normalize accepted, unambiguous noncanonical syntax.

---

## Syntax normalization without `--fix`

The formatter may normalize:

```text
func -> fn
x++  -> x += 1
x--  -> x -= 1
```

only when the intended construct is unambiguous.

Example:

```sec
func Run() void {
}
```

becomes:

```sec
fn Run() void {
}
```

A call or identifier named `func` remains unchanged.

Example:

```sec
func(callback)
```

remains unchanged.

Increment and decrement are statement-only aliases.

They do not produce a value.

The resulting compound assignment must still satisfy mutability, contract, and
`try` requirements.

---

## Safe fixes

Used by:

```text
sec fmt --fix
LSP quick fix
LSP fix all
safe fixes on save
```

Safe fixes may use Sema.

A fix is safe only when:

- the intended correction is unique;
- the corrected program preserves the apparent intent;
- no unrelated overload changes;
- no ownership effect is hidden;
- no new required user choice appears;
- affected source ranges are exact;
- the result can be formatted canonically.

Examples:

```text
insert missing parameter colon
replace invalid copy syntax with explicit move syntax when move is the only valid operation
add a required import when exactly one canonical module provides the symbol
add a missing comma when grammar and surrounding structure are unambiguous
```

---

## Missing parameter colon

Input:

```sec
fn myfunc(a string) void {
}
```

Diagnostic:

```text
expected `:` between parameter name `a` and type `string`
```

Quick fix:

```text
Insert missing `:`
```

Result:

```sec
fn myfunc(a: string) void {
}
```

Ordinary `sec fmt` does not apply this fix.

`sec fmt --fix` may apply it.

Safe fixes on save may apply it.

Optional inline correction may apply it while typing when enabled.

---

## Ownership fix

Input:

```sec
let destination := source
```

when the resolved copy classification does not permit copying `source` and an
explicit ownership transfer is legal.

Quick fix:

```sec
let destination :<- source
```

The fix is considered automatically safe only when no later source use becomes
invalid.

When later uses exist, the action must explain the consequence and be classified
as a refactoring or multi-edit fix rather than a trivial safe fix.

Ordinary formatting must never change:

```sec
let value := Function()
```

to:

```sec
let value :<- Function()
```

Direct construction from a temporary already uses the canonical form.

---

## Save behavior

Recommended defaults:

```text
format on save
    enabled

safe fixes on save
    enabled

code cleanup on save
    disabled

automatic semantic edits while typing
    disabled
```

Users may configure the categories independently.

An editor client must not hide which fixes were applied.

---

# Diagnostics

## Shared diagnostic model

Compiler diagnostics must contain:

```text
stable ID
phase
category
default severity
configured severity
primary source range
message
notes
related locations
help
machine-applicable fixes
target applicability
analysis confidence
data payload
```

The LSP maps this structure to protocol diagnostics.

It must not parse compiler message text to rediscover locations or IDs.

---

## Explanation quality

A diagnostic should explain the cause chain.

Example:

```text
This call is not ISR-safe
because it calls Log
which calls Write
which may block in platform/linux/file.sec
```

Every relevant chain entry should be navigable.

---

## Diagnostic origin

The client must be able to distinguish:

```text
lexer
parser
Sema
ownership analysis
borrow analysis
contract analysis
unit analysis
effect analysis
target analysis
compiler analysis
code-quality analysis
future knowledge pack
```

---

## Safety and advisory diagnostics

Mandatory language-safety errors cannot be disabled.

Advisory diagnostics use the configurable diagnostics model:

```text
off
info
warning
error
```

Examples include:

```text
style.merge-compatible-declarations
style.multiple-primary-types-per-file
style.type-file-name-mismatch
maintainability.large-function
ownership.implicit-discarded-result
```

---

## Diagnostic preview

Before applying a fix, the LSP should show:

```text
files changed
ranges changed
symbols affected
ownership consequences
new diagnostics introduced
diagnostics resolved
target variants affected
```

---

# Completion

Completion must be semantic and context-sensitive.

It considers:

```text
syntax position
expected type
scope
visibility
imports
target
profile
generic constraints
receiver type
receiver mutability
ownership state
borrow state
contracts
units
effects
availability
```

---

## Completion ranking

Prefer:

1. exact valid symbols in scope;
2. members valid for the resolved receiver;
3. expected-type matches;
4. already imported symbols;
5. auto-import candidates;
6. syntax keywords valid at the position;
7. snippets and templates.

Invalid or unavailable candidates should not appear as ordinary top-ranked
completion items.

---

## Unavailable completion explanation

The LSP may expose a secondary explanation for unavailable items.

Examples:

```text
Advance()
Unavailable because the receiver is immutable.
```

```text
Send()
Unavailable because the value was moved at line 12.
```

```text
Method()
Unavailable on the selected target.
```

---

## Completion resolve

Expensive information may be filled through completion-item resolve:

```text
documentation
effect summary
error variants
ownership behavior
target support
unit metadata
contract metadata
```

---

## Auto-import

The LSP may index unimported core and stdlib declarations for completion.

It must not make those symbols silently visible to Sema.

Instead it provides an additional text edit or code action inserting the
canonical import.

Example:

```sec
import "units"
```

Sec does not require a `std/units` source spelling.

---

# Hover

Hover should present the most useful compiler-known information.

General hover may include:

```text
declaration
resolved type
documentation
module
visibility
generic instantiation
ownership classification
availability state
borrow state
contracts
units
effects
error variants
target availability
source location
```

For a resolved callable, hover may additionally expose compiler-owned call-graph
facts:

```text
direct callers and callees
possible indirect callers and targets
closed target set or open callable contract
roots reaching the callable
execution relationships
same-stack recursion component
task, thread, or process creation relationships
task spawn origin
await or join synchronization origin where known
continuation information where known
effect, panic, allocation, stack, and unsafe-provenance paths
active target variant and CompilationPlan
```

The presentation must distinguish proven direct facts, conservative possible
facts, and open-world contracts. Unknown facts must not be displayed as closed
or complete.

Unknown facts must be omitted or explicitly marked unavailable.

The LSP must never invent metadata that the compiler has not resolved.

Hover on `try` and its operand consumes Sema's resolved revision-2 facts. It
shows the protected carrier/operation, success type, locally handled states,
unhandled states that propagate, propagation target, concrete-to-`error`
widening, and remaining panic effect where relevant. For Option it explains
Some success and None propagation/recovery; it must not claim Option cannot use
try.

Carrier-incompatible handler diagnostics explain the actual carrier and offer
the correct Result or Option patterns. Completion and hover expose `.Ok()` and
`.Err()` as consuming projections returning Option, and `OkRef`/`ErrRef` as
shared borrowed properties. Later-use diagnostics link to a consuming
projection. Fallible-setter hover/signature help shows its explicit error type,
and open-error hover may distinguish static type `error` from a concrete error
identity proven by Sema. The LSP never reconstructs a parallel Result, Option,
error, or ownership model.

## Default information

Hover and type completion expose the shared compiler-resolved default value and
its origin, including explicit type defaults, range-derived defaults and the
first value of `in [...]`. Non-defaultable types are reported as having no
default.

For omitted defaultable struct fields, inlay information may show the resolved
value without modifying source. Explicit code actions may expand defaulted
fields, insert an explicit default value, or declare a named-type default. These
are refactorings and must not run as ordinary formatting. The LSP must call the
same `DefaultValueOf` semantics as Sema rather than reimplement selection.

---

# Signature help

Signature help includes:

```text
overload candidates
active parameter
parameter names
parameter types
ownership behavior
ref or ref mut behavior
contracts
units
default values when supported
effects
error return
documentation
```

A consuming move-only parameter should be clearly marked.

---

# Navigation and references

Required:

```text
go to declaration
go to definition
go to type definition
go to implementation
find references
document highlight
```

Navigation must work across:

```text
project modules
core
stdlib
platform sources
generated sources
target-specific variants
```

References distinguish:

```text
read
write
copy
move
borrow
mutable borrow
call
type use
implementation
override or interface implementation
```

---

# Rename

Rename must be semantic.

It must:

- resolve the exact symbol;
- respect visibility;
- update all valid references;
- avoid unrelated equal spellings;
- preserve contextual `x`;
- preserve contextual `set`;
- respect generated and read-only sources;
- update documentation links where supported;
- preview multi-file changes.

Prepare-rename must reject keywords, invalid scopes, and non-renamable compiler
symbols.

---

# Semantic tokens

Semantic tokens should distinguish Sec-specific meaning when client capability
allows it.

Useful custom token types or modifiers may include:

```text
unit
contract
moved
borrowed
mutableBorrow
mustUse
targetSpecific
hardwareRegister
interrupt
unsafe
deprecated
```

Fallback clients use standard token types and modifiers.

Moved or unavailable bindings may be visually de-emphasized.

The semantic meaning remains available independently of color configuration.

---

# Inlay hints

Inlay hints should be individually configurable.

Possible categories:

```text
inferred type
parameter name
copy
move
consuming parameter
borrow lifetime
generic instantiation
resolved overload
unit conversion
contract check
error type
allocation
target
```

Call navigation and references also distinguish:

```text
direct call
interface dispatch
function-value dispatch
closure invocation
foreign call or callback entry
compiler-generated or intrinsic invocation
deferred invocation
destruction
task spawn
thread start
process launch
interrupt entry
await or join synchronization
```

Examples:

```text
resource /* moved */
```

```text
value /* copied */
```

Hints must not overwhelm the source by default.

Ownership hints for non-obvious consumption are high priority.

---

# Code lens

Code lenses may expose:

```text
reference count
implementation count
test count
stack estimate
maximum call-path stack
allocation count
blocking status
panic paths
ISR safety
target validity
generic specializations
callers
callees
```

Call-graph lenses use the compiler-owned graph for the active
`CompilationPlan`. They may show direct and possible counts separately, mark an
open callable contract, identify roots, and expose same-stack recursion, spawn
origins, synchronization origins, and available cause paths. A possible or
open-world relationship must not be presented as a confirmed direct call.

Example:

```text
Stack: 416 bytes local, 1.8 KiB maximum path
Allocations: none
Blocking: no
Targets: 3 valid, 1 invalid
```

Expensive lenses may load lazily.

---

# Shaped values

Completion and hover for shaped values consume the canonical compiler-known
member registry and resolved Sema facts. The LSP must not maintain an
independent shaped API table.

Hover presents the exact resolved expression/member type and exact property or
call return type. Where useful for correct use it additionally presents `Rank`,
`Shape`, `Len`, whether shape knowledge is static or runtime, `IsContiguous`,
`Layout`, `MemorySpace`, an unsafe `Ptr` requirement, and any runtime shape
check requiring `try`.

Hover must preserve the distinction between normal absence and failure. For
example, `Normalize() -> Option[vector[float32, 3]]` states that zero magnitude
produces `None`, while a storage-producing operation displays its exact
`Result[..., StorageError]` and principal failure condition.

When a shaped operation requires `try` only because compatibility is not
statically proven, hover states the runtime shape condition rather than implying
allocation failure. When Sema proves compatibility, hover presents the
infallible result. It must never invent shaped facts that Sema has not resolved.

---

# Ownership support

Ownership is a first-class editor feature.

## Hover state

Example:

```text
Type: Buffer
Ownership: move-only
Owner: buffer
State: Available
Active borrows: 0
Destruction: end of scope
```

After move:

```text
Type: Buffer
State: Moved
Moved to: activeBuffer
Move location: line 18
```

---

## Partial values

For a partially available struct:

```text
session
State: Partially available

Available:
    name
    timeout

Moved:
    file
```

The LSP should explain which operations require the complete value.

---

## Ownership actions

Possible actions:

```text
Go to move
Reinitialize binding
Change copy to move
Borrow instead
Change parameter to ref
Change parameter to ref mut
Show borrow holders
Show destruction point
Show ownership path
```

---

## Ownership preview

An ownership-changing action must explain its effects.

Example:

```text
Change `:=` to `:<-`

source becomes unavailable after line 14
later use at line 21 would become invalid
```

Such an action is not classified as automatically safe unless all consequences
are resolved.

---

## Match ownership and coverage

The LSP consumes compiler-resolved match facts and does not implement a
parallel pattern, ownership, borrow, or exhaustiveness analyzer.

Hover on a payload or field binding exposes its resolved type and binding mode:

```text
copy
move
shared borrow
mutable borrow
temporary forwarding
```

For a guarded move-only binding, tooling distinguishes a prospective move from
a committed move and may display `moves payload if this arm is selected`.
Pattern success alone must not make the subject appear moved.

After a match, hover and mandatory diagnostics expose compiler-produced
`Moved`, `Partially available`, or `Conditionally available` state and retain
related locations for the affected Place, moving arm, ownership binding, move
commit, later invalid use, and merge reason where available.

Configurable inlay hints may show `copy`, `move`, `ref`, `ref mut`, or `moves if
selected`. Disabling hints never disables ownership analysis or diagnostics.

Exhaustiveness diagnostics and generated-match actions use the resolved
coverage domain, including ordinary enum numeric value classes, open bit-enum
domains, guarded residual coverage, and reachable compiler-known union `empty`
state. Generated patterns must not introduce direct bool, general literal,
range, nested recursive, or ordinary struct-subject patterns.

Completion and code actions must not recommend payload use that is invalid in
the compiler-resolved ownership state.

---

# Contracts

The LSP should display:

```text
nominal type
base type
contract
compile-time proof status
runtime-check requirement
failing expression
valid range or set
```

Example:

```text
Type: Percent
Base: int
Contract: range 0..100
Proof: runtime check required
```

Code actions may:

```text
insert required try handling
convert literal to valid value
show contract definition
find contract uses
explain proof failure
```

---

# Units

The LSP must use the compiler's canonical unit model.

It may know that a unit symbol is available from the standard-library module:

```sec
import "units"
```

When a unit is not imported, the LSP offers an import action.

It must not silently inject unit symbols into compiler scope.

---

## Unit hover

When compiler metadata exists, hover may show:

```text
Name
Symbol
Numeric carrier
Default numeric representation
Category
Dimension
Kind
Scale
Offset
Origin
System
Transform
Logarithmic base, factor, and reference
Named or structural identity
Canonical form
Point or difference role
Conversion exactness
Definition
Source module
```

Example:

```text
Name: metre per second
Symbol: m/s
Dimension: Length × Time⁻¹
Scale: 1
Offset: 0
System: SI
```

When metadata is not implemented, show only verified facts.
Unknown Kind is shown as unknown and is never guessed from dimension. Tooling
preserves named versus structural source identity and does not rewrite `<m/s>`
to `<mps>`.

Example:

```text
Resolved symbol: m
Declared in: units
Dimension metadata: not yet available
```

---

## Unit actions

Possible actions:

```text
Import "units"
Convert to compatible unit
Show dimension derivation
Go to unit definition
Find unit references
Show exact scale and offset
```

The LSP surfaces compiler diagnostics for incompatible dimension or Kind,
point/difference and Origin misuse, invalid affine/logarithmic arithmetic,
lossy conversion, deprecated units, and derived currency warnings. Expressions
such as `<EUR/s>` and `<SEK/EUR>` remain valid when warned about.

Code actions may offer a compiler-proven fixed conversion, declared conversion
function, in-scope factor-provided conversion, or exact carrier change. They
must not invent an exchange rate or rounding policy. Unit algebra and derivation
facts come from Sema rather than an LSP-owned implementation.

---

# Target-aware analysis

The LSP must understand the active:

```text
project
logical target
output variant
OS
architecture
profile
features
diagnostic policy
```

Users must be able to change the active target without restarting the server.

---

## Multi-target diagnostics

A diagnostic records applicability:

```text
all targets
one target
selected outputs
one architecture
one profile
```

Example:

```text
linux/amd64: valid
linux/arm64: valid
baremetal/cortex-m4: invalid
    allocation path reaches allocator.New
```

---

## Target status

The LSP should provide:

```text
target selection
target status panel
per-target diagnostics
target comparison
active conditional sources
platform symbol navigation
```

---

# Compiler analysis

Everything produced by compiler analysis should be available in the editor.

## Call graph

The canonical semantics are defined by `call_graph.md`. The LSP exposes
compiler-owned graph facts and does not define another graph model.

Required callable queries include:

```text
direct callers
direct callees
possible indirect callers
possible indirect targets
closed target sets
open callable contracts
roots reaching a callable
complete cause paths where available
same-stack recursion components
task-spawn relationships
thread-start relationships
process-launch relationships
interrupt and callback entry relationships
await and join synchronization origins where known
continuation information where known
effect paths
panic paths
allocation paths
stack paths and estimates
unsafe-provenance paths
active target variant and CompilationPlan
```

### Relationship classification

Call hierarchy, hover, code lens, navigation, and graph views preserve both the
dispatch kind and the execution relationship. They distinguish:

```text
direct
interface
function value
closure
foreign
intrinsic
compiler generated
deferred
destruction
task
thread
process
interrupt
callback
```

Same-stack calls are not conflated with task-, thread-, or process-creation
edges. Consequently, same-stack recursion is reported separately from task
spawn cycles, thread-start cycles, and process-launch cycles.

### Call hierarchy

Incoming-call results distinguish:

```text
confirmed direct caller
possible indirect caller
open-world caller contract
implicit destructor caller
deferred caller
task spawner
thread starter
process launcher
foreign callback entry
interrupt root
```

Outgoing-call results distinguish direct callees, closed possible-target sets,
and open callable contracts. An open target set must retain its compiler-proven
callable contract instead of being shown as an empty or complete target set.

### Root and path explanations

For a callable or diagnostic, the LSP may expose all known root classes that
reach it and the compiler-owned cause path for an effect, panic, allocation,
stack contribution, unsafe boundary, reference escape, or ownership fact. Path
steps retain implicit edges and execution-context boundaries.

Task-spawn origin, and `await` or `join` synchronization origin where known,
must be navigable. Await and join synchronization are not displayed as ordinary
call edges to the task body. Continuation information is shown as a separate
synchronization fact.

### Target and snapshot requirements

Every graph response belongs to one concrete `CompilationPlan` and active
target variant. Cross-plan comparison is presented as comparison of separate
graphs, never as one merged runtime graph.

Deep graph requests support cancellation, work-done progress, and partial
results. Partial target discovery must remain visibly partial. The LSP must not
label a target set closed, a path complete, or a root set exhaustive until the
compiler analysis for that snapshot has established that fact.

---

## Stack analysis

Display:

```text
semantic local estimate
final target frame
maximum path
recursive uncertainty
interrupt contribution
task entry contribution
```

---

## Allocation and escape analysis

Display:

```text
allocation sites
allocator used
escape reason
ownership chain
heap-after-init violations
arena lifetime
no-allocation violations
```

---

## Effect analysis

Display:

```text
may allocate
may block
may panic
may perform I/O
may use unsafe
may access volatile memory
may call foreign code
may acquire locks
```

Cause chains must be navigable.

---

## ISR analysis

Display:

```text
ISR safe or unsafe
stack estimate
blocking chain
allocation chain
lock chain
shared mutation
non-reentrant call
unsafe call
```

---

## Concurrency analysis

When implemented, expose:

```text
task and thread transferability
channel ownership transfer
race candidates
deadlock candidates
lock order
join and await dependencies
cancellation paths
structured-concurrency violations
```

A heuristic finding must be marked as heuristic.

---

## Lowering analysis

Optional advanced views may show:

```text
Semantic IR
MLIR
LLVM IR
layout
ABI
target instruction summary
```

These are debugging and optimization views.

They do not redefine source semantics.

---

# Better code analysis

Better code analysis identifies valid but improvable code.

It produces advisory diagnostics and refactoring actions.

It is distinct from formatting and compiler-error correction.

---

## Analysis categories

```text
style
readability
maintainability
ownership
performance
allocation
control flow
API design
file organization
target portability
hardware safety
```

---

## Merge compatible declarations

Example:

```sec
let mut Audi: Car
let mut Saab: Car
let mut Volvo: Car
let mut Skoda: Car
```

The canonical compact form is:

```sec
Car mut: Audi, Saab, Volvo, Skoda
```

Suggested diagnostic:

```text
style.merge-compatible-declarations
```

Default severity:

```text
info
```

Suggested action:

```text
Merge compatible declarations
```

The action is allowed only when:

- declarations are in the same scope;
- declarations have the exact same type;
- declarations have the same mutability;
- declarations have compatible storage and attributes;
- evaluation order remains unchanged;
- comments and documentation remain correctly attached;
- no visibility or lifetime boundary changes;
- no initializer dependency changes meaning.

Ordinary formatting does not perform this transformation.

---

## Typed declaration tables

Example:

```sec
type TokenType string

TokenType (
    ILLEGAL := "ILLEGAL",
    EOF := "EOF",
    IDENT := "IDENT",
    INT := "INT",
)
```

The LSP should understand this as a related declaration table.

It may provide:

```text
sort declaration table
add declaration
rename declaration
find declaration references
convert compatible declarations to table
split table
```

Sorting must not occur automatically unless declaration order is semantically
irrelevant and the user explicitly requests it.

---

## Function analysis

Possible advisory findings:

```text
large function
high nesting
many exit paths
repeated condition
extractable region
duplicated expression
unnecessary temporary
unnecessary copy
avoidable allocation
```

A large-function diagnostic should provide evidence.

Example:

```text
Statements: 146
Branches: 31
Maximum nesting: 7
Ownership exits: 11
Suggested extraction regions: 3
```

---

# File and type organization

## Primary type per file

The recommended Sec organization follows the broad convention used by several
languages and codebases:

> One primary public nominal type per source file.

The same file should normally contain:

```text
the primary type
its ordinary impl block
closely related private helper declarations
nested or subordinate types
direct documentation
```

Example:

```text
Car.sec
    type Car struct
    impl Car
```

This is a code-quality recommendation, not a mandatory language rule.

---

## File organization diagnostics

Suggested advisory diagnostics:

```text
style.multiple-primary-types-per-file
style.type-file-name-mismatch
style.impl-separated-from-type
style.unrelated-declaration-in-file
```

Default severity:

```text
info
```

---

## File refactorings

Required actions:

```text
Move type to new file
Move type and impl to new file
Split file by primary types
Move impl next to type
Rename file to match type
Move type to another module
Move related private helpers
```

Moving within the same module is simpler than moving between modules.

Cross-module moves must update:

```text
imports
qualifiers
visibility
internal boundaries
references
tests
generated metadata
```

Multi-file moves require preview.

---

# Refactoring

Refactoring is a core language-server responsibility.

It must use compiler identity and structured edits.

---

## Required refactorings

Initial and planned actions include:

```text
Rename symbol
Extract local
Extract constant
Extract function
Extract method
Extract type
Extract interface
Inline local
Inline function
Move declaration
Move type to file
Move type to module
Split file by types
Merge compatible declarations
Split declaration group
Change parameter to ref
Change parameter to ref mut
Convert copy to move
Convert move to borrow
Introduce named type
Generate exhaustive match
Generate switch cases
Generate Result handling
Generate impl
Generate interface implementation
Generate property
Generate constructor
Organize imports
Remove unused imports
```

---

## Refactoring safety classes

Every action is classified.

### Formatting

No semantic change.

May run automatically.

### Syntax normalization

Unambiguous canonicalization.

May run automatically.

### Safe semantic fix

Compiler-proven unique correction.

May run on save when enabled.

### Code cleanup

Valid-code improvement preserving behavior.

Requires configured cleanup or explicit action.

### Structural refactoring

Changes declarations or file organization while preserving behavior.

Requires explicit action and preview for multi-file edits.

### Behavior-changing refactoring

Changes ownership, API, effects, or runtime behavior.

Always requires explicit confirmation and consequence preview.

### Potentially lossy action

May drop information, comments, compatibility, or behavior.

Must never run automatically.

---

## Refactoring preview

Preview must show:

```text
files created
files deleted
files renamed
edits
symbols moved
references updated
imports changed
ownership effects
API effects
targets affected
diagnostics resolved
diagnostics introduced
```

---

## Refactoring examples

### Move type to file

Preview:

```text
Create: Car.sec
Move: type Car
Move: impl Car
Keep: __CarParser
Update imports: 0
Module remains: vehicles
```

### Change parameter to ref

Preview:

```text
Call sites affected: 17
Moves removed: 13
Copies removed: 4
Lifetime conflicts introduced: 1
```

### Merge declarations

Preview:

```text
4 declarations become 1 declaration group
No evaluation-order changes
No comments moved
No symbol-identity changes
```

---

# Standard-library awareness

The compiler workspace must index the complete standard library.

The LSP may use this index for:

```text
completion
auto-import
hover
documentation
navigation
references
effects
units
contracts
target availability
```

Import spelling follows Sec module rules.

Examples:

```sec
import "fmt"
import "units"
```

The LSP must not require artificial `std/` prefixes when the language does not.

Current hardcoded module mappings must be replaced by canonical module discovery.

---

# Sec-specific protocol extensions

Standard LSP features must be used wherever they are sufficient.

Rich Sec-specific data uses the namespace:

```text
sec/...
```

Possible requests:

```text
sec/ownershipState
sec/ownershipPath
sec/borrowGraph
sec/contractProof
sec/unitInfo
sec/targetStatus
sec/callGraph
sec/stackUsage
sec/allocationPaths
sec/escapePaths
sec/effectChain
sec/isrSafety
sec/dataRaceExplanation
sec/deadlockExplanation
sec/loweringPreview
sec/semanticIR
sec/mlir
```

Every extension must:

- be versioned;
- use capability negotiation;
- degrade gracefully;
- identify the workspace snapshot;
- use compiler-owned facts;
- remain optional for ordinary LSP clients.

---

# Knowledge Packs

## Status

Knowledge Packs are a future feature.

They are not required for Sec 0.1.

The LSP architecture must leave a clean integration point, but no complete
Knowledge Pack system is required before 0.1.

A first public implementation may be targeted for Sec 1.0.

---

## Future purpose

A Knowledge Pack provides versioned domain knowledge that enriches compiler and
LSP analysis.

Potential domains include:

```text
microcontrollers
boards
operating systems
system calls
communication protocols
databases
network equipment
company APIs
safety standards
industry rules
```

Example future pack:

```text
STM32F411
```

It may contain:

```text
memory map
registers
bit fields
interrupts
clock tree
DMA mappings
pin alternate functions
package constraints
silicon revisions
errata
documentation references
```

---

## Future rules

A Knowledge Pack may enrich:

```text
hover
completion
diagnostics
navigation
code actions
target validation
documentation
```

It may not:

- redefine Sec syntax;
- replace Sema;
- override compiler errors;
- change ownership rules;
- silently add imports;
- silently add symbols to source scope;
- claim unknown facts as proven.

Knowledge Pack results must identify:

```text
source
version
applicability
confidence
```

Declarative packs are preferred over executable plugins.

Executable analyzer plugins, if ever supported, require a separate security and
sandboxing design.

---

# Privacy, security, and telemetry

The language server must work fully offline.

Source code must not be transmitted externally by default.

Telemetry is disabled by default unless a later explicit project decision
changes that rule.

Any future telemetry must be:

- opt-in;
- documented;
- inspectable;
- free of source text;
- free of identifiers;
- free of file paths unless explicitly approved.

Knowledge Packs must not imply network access.

---

# Logging

Logging levels should include:

```text
off
error
warning
info
debug
trace
```

Protocol trace logging must be independently configurable.

Logs must avoid dumping entire source documents by default.

Internal panics should produce:

```text
request method
snapshot version
internal stack trace in server log
safe client-facing error
```

The server should remain alive when possible.

---

# Configuration

Recommended configuration areas:

```text
sec.languageServer.path
sec.languageServer.trace
sec.formatOnSave
sec.safeFixesOnSave
sec.codeCleanupOnSave
sec.inlineSafeFixes
sec.inlayHints.types
sec.inlayHints.ownership
sec.inlayHints.parameters
sec.analysis.stack
sec.analysis.allocations
sec.analysis.effects
sec.analysis.concurrency
sec.activeTarget
sec.diagnostics
```

Configuration changes should update the workspace without restarting the server
where possible.

Closure and callable-flow analysis depth is selected in the project manifest:

```toml
[analysis]
lsp_depth = "interactive"
```

Supported values are `interactive`, `standard`, and `deep`. The default is
`interactive` when the section, key, or project manifest is absent. The setting
changes resource budget and optional precision, never source-language semantics
or which safety proofs are required.

The server must re-read this value after project-configuration notifications or
watched-file changes, invalidate analysis facts produced under the old depth,
and refresh affected open-document diagnostics without a server restart.

When pitfall analysis is implemented, its optional configuration and result
filtering follow `pitfall_analysis.md`. Configuration reload must invalidate
affected pitfall state and refresh diagnostics and code actions without a
server restart. Optional pitfall settings never suppress the normative compiler
diagnostic that owns a proven language error.

---

# Testing

## Protocol tests

Required:

- initialization negotiation;
- lifecycle ordering;
- valid and invalid JSON-RPC;
- cancellation;
- progress;
- document versions;
- incremental changes;
- stale-result suppression;
- unknown enum values;
- unknown methods;
- client capability subsets.

---

## Feature tests

Each feature requires:

```text
unit tests
golden tests
multi-file tests
incomplete-source tests
Unicode position tests
target-specific tests
cancellation tests
stale-snapshot tests
```

---

## Shared compiler tests

Verify that:

```text
sec check
LSP diagnostics
sec fmt --fix diagnostics
```

produce the same IDs, ranges, messages, and fixes for the same snapshot.

---

## Formatter integration tests

Verify:

- CLI and LSP produce byte-identical formatting;
- ordinary formatting never applies semantic fixes;
- `--fix` and LSP safe fixes share edits;
- formatting is idempotent;
- comments survive;
- incomplete unaffected regions remain formatable.

---

## Refactoring tests

Every refactoring tests:

```text
symbol identity
references
imports
comments
formatting
ownership state
target variants
API changes
rollback on failure
```

Multi-file edits must be atomic from the client's perspective.

---

## Performance tests

Measure:

```text
startup
first diagnostics
incremental diagnostics
completion latency
hover latency
workspace reference latency
memory use
large-workspace behavior
cancellation latency
```

Performance regressions require tracked baselines.

---

# Required synchronization

This rulebook requires synchronization with:

```text
formatter.md
diagnostics.txt
compiler.txt
compiler_pipeline.txt
compiler_analysis.txt
semantic_ir.txt
projects.txt
modules.md
parser_recovery.md
incremental_compilation.md
compiler_testing.md
ownership.md
copy_move.md
borrowing.txt
lifetime_analysis.md
contracts.md
types/units.md
rules/platform/target_profiles.md
platform_model.md
call_graph.md
stack_analysis.md
escape_analysis.md
pitfall_analysis.md
effect_analysis.md
isr_analysis.md
data_races.md
deadlock_analysis.md
language-rulebook-status.md
rules_implementations.txt
```

Files that do not yet exist remain planned dependencies.

---

# Appendix A — Codex implementation plan

## A.1 Preserve working behavior

Before moving code, create coverage for all current behavior.

Do not regress:

```text
diagnostics
core loading
imports
completion
hover
document symbols
semantic tokens
formatting
VS Code startup
format on save
```

---

## A.2 Move protocol transport

Create:

```text
internal/lsp/protocol
internal/lsp/server
```

Move:

```text
JSON-RPC framing
message types
request routing
responses
notifications
capability negotiation
lifecycle
```

Keep `cmd/lsp/main.go` as bootstrap.

---

## A.3 Generate protocol types

Use the LSP 3.18 meta model where practical.

Requirements:

- generated files are reproducible;
- unknown extensible enum values are preserved;
- optional and nullable fields follow the protocol exactly;
- no editor-specific assumptions in protocol types;
- generation command is documented and tested.

---

## A.4 Create compiler workspace

Create:

```text
internal/compiler/workspace
internal/compiler/snapshot
```

Move LSP-specific direct calls to lexer, parser, and Sema behind shared workspace
services.

The workspace must be reusable by CLI commands.

---

## A.5 Unify source loading

Replace LSP-specific core and import inclusion with the canonical module loader.

Support:

```text
core
stdlib
project modules
platform modules
target variants
generated sources
open-document overlays
```

Remove permanent hardcoded stdlib module switches when canonical discovery is
available.

---

## A.6 Structured parser diagnostics

Replace parser error strings with structured diagnostics.

Add:

```text
stable IDs
ranges
notes
help
fixes
recovery nodes
```

Do not parse line and column from human-readable error text.

---

## A.7 Error-tolerant syntax tree

Implement recovery nodes and continue analysis after local recoverable errors.

First required recovery:

```sec
fn myfunc(a string) void {
}
```

Recovered as a parameter with a missing colon.

Provide a machine-applicable insertion fix.

---

## A.8 Shared formatter

Move `formatSource` and supporting logic from `cmd/lsp` to:

```text
internal/formatter
```

Use it from:

```text
sec fmt
LSP document formatting
LSP range formatting
LSP on-type formatting
refactoring output
```

Preserve all current formatter tests.

The formatter-rulebook filename migration to `rules/tooling/formatter.md` is complete.

---

## A.9 Shared fix engine

Create:

```text
internal/fixes
```

Consume compiler diagnostics containing structured edits.

Support:

```text
single fix
fix all in document
fix all in workspace
safe fixes on save
sec fmt --fix
```

Do not duplicate fix logic in LSP handlers.

---

## A.10 Document snapshots

Implement:

```text
versioned documents
incremental edits
immutable snapshots
stale result rejection
didClose
didSave
```

Protect all document access through the snapshot model.

---

## A.11 Cancellation and progress

Implement:

```text
$/cancelRequest
workDoneProgress
partial results
```

Every deep workspace analysis must periodically check cancellation.

---

## A.12 Split features

Move features into `internal/lsp/features`.

Each feature receives:

```text
server context
client capabilities
workspace snapshot
request parameters
```

It must not instantiate compiler frontend phases directly.

---

## A.13 Complete diagnostics

Add:

```text
related information
tags
data
code descriptions where useful
pull diagnostics
workspace diagnostics
result IDs
unchanged reports
```

Retain push diagnostics for clients requiring them.

---

## A.14 Implement code actions

Initial code actions:

```text
insert missing colon
add missing import
change invalid copy to move
handle Result
remove unused import
merge compatible declarations
move primary type to file
```

Classify every action by safety class.

Current implementation includes the user-invoked `S1007` quick fix for a
proven invalid copy of a named value, including move-only, compiler-known
non-copyable, and generic values without proven copyability. It replaces only
the diagnosed source statement's `:=` or `=` operator with `:<-` or `<-`,
respectively, and states that the source becomes unavailable. It is not applied
automatically.

---

## A.15 Implement navigation

Initial `textDocument/definition` support is implemented through Sema-owned
use-to-declaration bindings. It covers local and cross-file declarations,
members, and selected overloads without an LSP-owned name resolver.

Initial `textDocument/documentHighlight` support is also implemented through
those bindings. It finds exact same-document occurrences, keeps shadowed names
separate, and classifies declarations and direct assignment targets as writes.

Initial standard call hierarchy is implemented over the compiler-owned direct
call graph. It covers validated synchronous direct calls and statically resolved
methods in the current source snapshot. Hover also consumes initial
program/task/thread root reachability and same-stack SCC results. Task and
thread spawn relationships retain their execution classification and remain
outside same-stack recursion. Indirect targets, other execution boundaries,
specializations, additional root classes, and complete `CompilationPlan`
identity remain pending.

Remaining order:

```text
declaration
type definition
implementation
references
rename
```

Build a shared workspace symbol index.

---

## A.16 Implement signature help and inlay hints

Use Sema-resolved overloads.

Prioritize ownership and parameter hints.

---

## A.17 Implement refactoring engine

Create:

```text
internal/refactor
```

Initial refactorings:

```text
rename
move type to file
split file by primary types
merge declarations
extract function
change parameter to ref
```

All multi-file actions require preview metadata.

---

## A.18 Implement compiler-analysis views

Add code lenses and Sec-specific endpoints incrementally as compiler analyses
become available.

Do not create weaker duplicate analyses inside LSP.

---

## A.19 Units integration

Replace hardcoded unit presentation with compiler unit metadata.

Implement auto-import:

```sec
import "units"
```

Do not make unimported units silently visible.

---

## A.20 Target integration

Load active target and output variant from the canonical project model.

Add target switching and per-target diagnostics.

---

## A.21 VS Code settings

Add separate settings for:

```text
format on save
safe fixes on save
code cleanup on save
inline safe fixes
active target
inlay hints
analysis features
trace
```

Keep client logic thin.

---

## A.22 Knowledge Pack reservation

Add only stable extension points and capability names.

Do not implement a complete Knowledge Pack system for Sec 0.1.

Mark Knowledge Packs as future in status documents.

---

## A.23 Status updates

Update `language-rulebook-status.md`:

```text
lsp.md
    Written

knowledge_packs.md
    Future or deferred candidate
```

Remove `lsp.md` from Candidate.

Add it to the canonical written set.

Update `rules_implementations.txt` with the current implemented, partial, and
missing LSP features.

---

## A.24 Recommended implementation order

```text
1. Freeze current behavior with tests.
2. Move formatter to internal/formatter.
3. Create generated protocol types and server package.
4. Create compiler workspace and snapshots.
5. Move current features to internal/lsp/features.
6. Implement document versions and incremental sync.
7. Implement structured parser diagnostics and recovery.
8. Create shared fix engine.
9. Implement code actions and safe fixes on save.
10. Implement navigation and references.
11. Implement rename and refactoring engine.
12. Implement complete LSP 3.18 capability set.
13. Add ownership, contract, unit, target, and analysis views.
14. Optimize incremental performance.
15. Reserve, but do not implement, Knowledge Packs.
```

---

# Design summary

## Enum domains and conversions

Enum hover should identify ordinary enums as closed over their declared numeric
value classes and bit-backed enums as open over every value representable by
`bit[N]`. For hardware enums it should make clear that unnamed patterns may
occur at runtime.

When declared members do not exhaust an open bit-backed enum, match diagnostics
should explain the uncovered hardware encodings and may suggest a final `_`
fallback. Checked runtime integer-to-enum conversion hover and signatures show
fallibility and the `EnumValueError` family so that the reason for `try` is
visible.

---

The Sec language server is an interactive view of the real compiler.

It does not own a parallel frontend or diagnostic system.

It uses the same formatter as `sec fmt` and the same fix engine as
`sec fmt --fix`.

It targets complete LSP 3.18 support.

It remains useful on incomplete source through structured parser recovery.

It exposes ownership, contracts, units, targets, effects, stack, allocation,
escape, ISR, concurrency, and lowering knowledge whenever the compiler provides
that knowledge.

It treats refactoring as a first-class capability.

It supports code-quality analysis such as declaration merging and one primary
type per file without confusing those improvements with formatting.

It explains consequences before applying ownership-changing or multi-file edits.

Knowledge Packs are reserved as a future enrichment system and are not required
for Sec 0.1.

## Ownership revision 2 integration

The LSP consumes the same canonical ownership facts as Sema; it does not build a
parallel ownership state machine. Hover, diagnostics, semantic tokens, and code
actions may expose place availability (including partial and conditional
states), unavailability reason and source location, copy-versus-consume,
required `<-` markers, capture mode, and refinement from `is available` / `is
not available`.

When compiler facts make a repair unambiguous, the LSP may suggest forms such as
`Consume(<-resource)`, `capture(<-resource)`, an appropriate borrow capture, or
`discard place` for explicit convergence. It must also explain when selected
target policy forbids required runtime ownership bookkeeping. These actions use
programmer-facing mentor language and are offered only when Sema proves their
ownership consequence.

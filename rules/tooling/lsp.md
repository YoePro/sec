# Language Server Protocol

- **Status:** Normative
- **Created:** 2026-07-31
- **Last updated:** 2026-11-08 (stable chapter, subsection, and paragraph numbering)
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/tooling/lsp.md`
- **Repository baseline reviewed:** `50655d3bc5d2e2596935dc5609cb1db1387e0cec`
- **Related governance:** `governance/tooling_lsp.yaml`
- **Related rulebooks:** `rules/tooling/formatter.md`, `rules/tooling/diagnostics.md`, `rules/compiler/compiler_analysis.md`, `rules/compiler/parser_recovery.md`, `rules/compiler/semantic_ir.md`, `rules/projects/projects.md`, `rules/projects/modules.md`, `rules/analysis/call_graph.md`, `rules/analysis/stack_analysis.md`, `rules/analysis/effect_analysis.md`, `rules/analysis/escape_analysis.md`

---

## § 1 Status

**§ 1(1)** This document is the canonical rulebook for the Sec language server.

**§ 1(2)** The language server already exists and provides a useful initial feature set.

**§ 1(3)** This rulebook:

- records the current implementation accurately;
- defines the required architecture;
- defines the complete target feature set;
- defines the relationship between the compiler, formatter, fix engine, and LSP;
- defines Sec-specific extensions;
- defines future knowledge-pack integration;
- provides an implementation plan for Codex.

**§ 1(4)** The canonical executable remains:

```text
lsp-sec
```

**§ 1(5)** The current entry point is:

```text
cmd/lsp
```

**§ 1(6)** The implementation must be reorganized without discarding working behavior.

---

## § 2 Vision

**§ 2(1)** The Sec language server must be better than any competing language server in
the practical quality of its assistance.

**§ 2(2)** This means more than feature count.

**§ 2(3)** The server must be:

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

**§ 2(4)** The server must expose the knowledge already produced by the Sec compiler
instead of recreating a weaker editor-specific approximation.

**§ 2(5)** The language server must never silently invent language semantics.

---

## § 3 Fundamental rules

### § 3.1 One semantic source of truth

**§ 3.1(1)** The compiler workspace is the sole owner of:

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

**§ 3.1(2)** The LSP must not maintain a separate lexer, parser, AST, semantic analyzer,
type system, ownership model, or diagnostic system.

**§ 3.1(3)** Every diagnostic presented by the LSP must originate from the same diagnostic
model used by:

```text
sec check
sec build
sec run
sec test
sec fmt --fix
```

**§ 3.1(4)** The LSP is a protocol adapter and interactive client of compiler services.

**§ 3.1(5)** The compiler owns one canonical semantic call graph for each concrete
`CompilationPlan`. The LSP consumes that graph and its compiler-produced
analysis views. It must not construct, cache as semantic truth, or incrementally
maintain a second LSP-specific call graph.

---

### § 3.2 Shared formatter

**§ 3.2(1)** LSP formatting must call the same formatter implementation as:

```text
sec fmt
```

**§ 3.2(2)** LSP safe fixes must call the same fix engine as:

```text
sec fmt --fix
```

**§ 3.2(3)** LSP formatting resolves its effective options through the same configuration code as `sec fmt`: the document's project `.sec/sec.toml` `[format]` table, or the formatter defaults for a stand-alone file. An invalid configuration produces no edit (`formatter.md` § 4(18)–(22); MD-036/MD-054, 2026-10-12).

**§ 3.2(4)** The `sec fmt` directory selection and bulk malformed-file policy (`formatter.md` §§ 28(9)–28(13), MD-037) apply only to the CLI. The LSP formats the open document with the same formatter and configuration, under its own § 25 behavior.

**§ 3.2(5)** The LSP may expose all formatter functionality or a negotiated subset.

**§ 3.2(6)** It must not contain an independent formatter implementation.

**§ 3.2(7)** The editor extension must not contain Sec formatting rules.

---

### § 3.3 Complete LSP support

**§ 3.3(1)** The Sec language server targets the complete Language Server Protocol 3.18
surface.

**§ 3.3(2)** The server must:

- implement the complete base protocol correctly;
- negotiate capabilities correctly;
- implement every applicable standard language and workspace capability;
- respond correctly to supported and unsupported optional requests;
- never advertise a capability that is not functionally implemented;
- never crash because a client sends a valid but unknown future enum value;
- preserve compatibility with clients supporting earlier negotiated subsets.

**§ 3.3(3)** Features not meaningful for Sec must still follow protocol rules.

**§ 3.3(4)** They may remain unadvertised or return a valid empty result as required by the
protocol.

---

### § 3.4 Error-tolerant editing

**§ 3.4(1)** The parser and compiler workspace must preserve useful structure around
recoverable syntax errors.

**§ 3.4(2)** A recoverable error must not unnecessarily remove:

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

**§ 3.4(3)** Where a syntax repair is unambiguous, the compiler must produce a structured,
machine-applicable fix.

**§ 3.4(4)** Example:

```sec
fn myfunc(a string) void {
}
```

**§ 3.4(5)** Diagnostic:

```text
expected `:` between parameter name `a` and type `string`
```

**§ 3.4(6)** Fix:

```sec
fn myfunc(a: string) void {
}
```

**§ 3.4(7)** The fix may be applied:

- explicitly through a quick fix;
- on save when safe fixes on save are enabled;
- by `sec fmt --fix`.

**§ 3.4(8)** Ordinary `sec fmt` does not apply this repair.

---

### § 3.5 Expose all compiler knowledge

**§ 3.5(1)** Every useful fact available from the compiler pipeline should be exposable
through standard LSP features or namespaced Sec extensions.

**§ 3.5(2)** This includes facts from:

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

**§ 3.5(3)** The LSP must not wait for a separate editor-specific reimplementation of an
analysis that already exists in the compiler.

---

### § 3.6 No silent semantic edits

**§ 3.6(1)** The language server must never silently change program meaning.

**§ 3.6(2)** Automatic text changes are divided into:

```text
canonical formatting
syntax normalization
safe compiler fix
code cleanup
refactoring
behavior-changing transformation
```

**§ 3.6(3)** Each category has separate permissions.

**§ 3.6(4)** The LSP must explain ownership-changing and behavior-changing edits before
application.

---

## § 4 Current implementation status

**§ 4(1)** The current implementation is substantial and must be preserved during
reorganization.

**§ 4(2)** The status below reflects the repository implementation in `cmd/lsp` and
`vscode`.

---

### § 4.1 Implemented

### Transport and lifecycle

**§ 4.1(1)** Implemented:

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

**§ 4.1(2)** Implemented:

- `textDocument/didOpen`;
- `textDocument/didChange`;
- in-memory open-document text;
- full-document synchronization;
- debounced diagnostics;
- a current diagnostic delay of 600 milliseconds.

### Diagnostics

**§ 4.1(3)** Implemented:

- lexer/parser execution through compiler packages;
- Sema execution through the compiler Sema package;
- core source loading;
- same-directory, same-module source assembly through the compiler lexer,
  parser, AST, and Sema pipeline;
- target-aware source selection (2026-10-04): one analysis sees only the
  sibling and imported files whose `#target` matches the active target, as the
  compiler does, so platform-specific files of one module (for example
  `file.linux.amd64.sec` and `file.windows.amd64.sec` in `stdlib/os`) may
  declare the same names; the active target is the document's own `#target`,
  otherwise the project target when all its variants share one OS and
  architecture, otherwise the host target;
- open-document overlays for active and sibling module files;
- preservation of valid declarations from sibling files with recoverable syntax
  errors, including bodyless function stubs; sibling syntax errors are reported
  on their own document instead of hiding their declarations from other files;
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

**§ 4.1(4)** Implemented:

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
- minimal enclosing document-formatting edits from the shared formatter, with unchanged prefix/suffix excluded and UTF-16 positions;
- format-on-save enabled by default in the VS Code extension.

### Completion

**§ 4.1(5)** Implemented:

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

**§ 4.1(6)** Type-qualified completion for a string-backed enum exposes its declared
members exactly like an integer-backed closed enum. Hover shows the nominal enum
type, `string` underlying representation, closed domain, and declared constant
value. Explicit `static let` members are readonly type-qualified members;
non-static `let` members require an instance receiver. Tooling must keep these
categories distinct and must never offer a remove-static rewrite or a redundant
modifier diagnostic for an implementation binding.

### Hover

**§ 4.1(7)** Implemented:

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

**§ 4.1(8)** Implemented:

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

**§ 4.1(9)** The initial implementation resolves from the current immutable document text
and compiler-loaded source set. A persistent workspace symbol index,
declaration/type-definition distinction, implementation navigation, references,
rename, indirect/open call targets, non-synchronous relationships beyond the
implemented task/thread spawn edges, root classes beyond program/task/thread
entry, complete path-sensitive graph pruning, per-`CompilationPlan` graph
snapshots, and richer copy/move/borrow occurrence classification remain pending.

### Document symbols

**§ 4.1(10)** Implemented:

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
- nested symbol children, including struct-type fields and enum and union
  members of type declarations;
- selection ranges contained by symbol ranges.

### Interface conformance code lens

**§ 4.1(11)** Implemented (2026-10-04): `textDocument/codeLens` places one lens on every
interface declared in the document. The module is assembled and analyzed once
per target selected by its `#target` files (or for the active target when it has
none), each with that target's scalar plan, and the lens summarizes the result:
`✓ Handle conforms on 18 targets`, or `✗ conforms on 16 of 18 targets —
freebsd-armv7: Open, Close; …` naming each failing target with its S1106/S1107
members, and targets without an implementation. Its command
(`sec.showLocations`) lists the failing implementation members, or the
implementing types when every target conforms. Results are cached until the
source/import closure, project configuration or analysis epoch changes.

**§ 4.1(12)** As of 2026-10-14, listing code lenses only parses the active snapshot and returns
interface-name ranges with versioned declaration/snapshot data and no command.
`codeLensProvider.resolveProvider` advertises `codeLens/resolve`; resolving the
item runs the existing compiler-owned cross-target conformance projection on
the bounded read-only worker pool, using current module sources and its shared
cache. Concurrent resolves with the same source key share one in-flight
computation; failed computations publish no partial report and are not cached
as success. No second conformance engine or worker pool is introduced. Exact name,
range, URI and source fingerprint validate the declaration; malformed items
return `InvalidParams`, while changed or closed active documents return
`ContentModified`, including changes arriving during analysis. Client-supplied
commands are recomputed from compiler facts. Queued cancellation and edits can
skip obsolete resolve work, and formatting may overtake it. Cancellation inside
an already-running compiler analysis remains separate scheduling work.

**§ 4.1(13)** As of 2026-11-03, implementing-type declarations (including enums and named
scalar types) also show their explicit conformance across the selected targets.
A successful lens names the type and concrete interfaces, for example
`✓ Handle (Native, Writable) conforms on 2 targets`. A failing lens identifies
the type, target and interface-qualified member, for example
`Handle: ✗ conforms on 1 of 2 targets — windows-amd64: Native.Size`.
An interface declared on one target but absent on another is reported as
`Native (no declared conformance)`; another valid interface cannot mask it.

**§ 4.1(14)** Enumeration adds type-name candidates without loading targets or invoking Sema.
Resolving a candidate with no explicit compiler-owned conformance on any selected
target returns no command. Type views match exact module/type identities, reuse
the same cached target report and worker pool, and validate the source snapshot
and declaration kind. Their command lists failing members, or the implemented
interface declarations when every target conforms. Unresolved compiler facts
are never presented as successful conformance.

**§ 4.1(15)** As of 2026-11-05, whole-program `sec analyse` and `sec analyse --all` also
project these compiler-owned conformance facts across distinct physical targets
from same-module source directives and the owning Project Target's Variants.
An explicit `--target` or explicitly selected target-directed source limits
the report to that platform. Function-focused and individually selected analyses
keep their existing scope. The CLI reuses ordinary source/import/project
validation and each target's scalar plan; it does not discover methods or infer
conformance from names or diagnostic text.

**§ 4.1(16)** `--verbose` lists the type/module, concrete interface, target, proof state and
failing members; `--json` adds a `conformances` array with declaration spans and
member diagnostic IDs, messages and spans. Missing explicit declarations and
unproven facts are reported as unproven findings, never success. Conformance
errors retain their registered diagnostics and reject the command with exit 3;
unrelated semantic errors still reject analysis at the ordinary validation gate.
Clean conformance remains silent in the default human presentation.

### Workspace symbols

**§ 4.1(17)** Implemented (`workspace/symbol`, VS Code "Go to Symbol in Workspace"):

- `workspaceSymbolProvider` and workspace-folder tracking from `initialize`
  (`workspaceFolders`, falling back to `rootUri`/`rootPath`) and
  `workspace/didChangeWorkspaceFolders`;
- the declarations of every `.sec` source under the workspace folders and of
  every open document, including files that are not open; hidden directories,
  `testdata` fixtures, and `node_modules` are not workspace source;
- workspace folders inside a Project resolve to the enclosing manifest's
  `ProjectRoot` through the shared project model, so unopened sibling Modules
  remain searchable. Project source locations follow `projects.md` §§13–15:
  root `bin/` and `.sec/` storage and direct files in `cmd/` or an `internal`
  marker are excluded; ordinary nested `bin/` directories remain source.
  Standalone folders and independent Projects below an umbrella folder are
  supported. Invalid manifests and forbidden nested Projects never authorize
  a standalone fallback. Explicitly open documents remain included for editing;
- source selection and lazy resolve rediscover current project boundaries per
  request; manifest addition, repair, invalidation and removal affect cached
  declarations without requiring a server restart. Resolve uses the same
  source-membership facts without recursively traversing directories;
- the same declaration projection as document symbols: types, structs, unions,
  enums, interfaces, functions, units, module-scope variables, and their
  members, fields, properties, events, and enum and union members; impl
  members appear under the implemented type; module headers, `init`, and
  `free` are not workspace symbols;
- separate `impl extends Type` header symbols, including generic targets and
  multiple fragments in one file; searching `impl extends Widget` lists the
  indexed fragments for that target with their module containers and source
  locations. Headers follow the target's visibility, including qualified
  private names, and support the same unsaved snapshots and lazy resolution
  as other workspace symbols;
- complete `SymbolInformation` locations for clients without lazy-range support,
  and `WorkspaceSymbol` URI-only locations with declaration snapshot data for
  clients advertising `workspace.symbol.resolveSupport` for `location.range`;
- locations at the exact declaration name with the
  container: the module path for top-level declarations and the owning type
  for members;
- case-insensitive subsequence matching ranked exact, prefix, substring, then
  scattered subsequence, and ordered deterministically by name, container,
  file, and line within a rank;
- visibility from the editor context, because the request carries no
  document: a module-internal (`_`) name is offered only while an open
  document belongs to its module, and a private (`__`) name only while its
  own source file is open;
- read-only trusted core declarations ranked after workspace declarations and
  marked `core (read-only)` in their container name;
- per-file declaration summaries cached by on-disk size and modification time
  or by the open snapshot's content hash, so only changed files are
  re-parsed;
- `workspaceSymbol/resolve`, advertised by `workspaceSymbolProvider.resolveProvider`,
  fills the exact declaration range from the shared document-symbol projection.
  It preserves distinct overloads and member containers, validates source content
  (including edits preserving disk size and timestamp), and rechecks current
  workspace membership and editor-context visibility. Stale content returns
  `ContentModified`; malformed or no-longer-accessible symbols return
  `InvalidParams`. It uses the selected file's cached summary without another
  semantic analysis or workspace traversal. Resolve is a read-only request that
  formatting may overtake.

**§ 4.1(18)** Pending: generated and dependency read-only provenance once the shared compiler
workspace exposes it, and moving declaration summaries into that shared index.

### Semantic tokens

**§ 4.1(19)** Implemented:

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

**§ 4.1(20)** Every multiline lexer token that receives semantic highlighting is encoded as
separate single-line semantic tokens. In particular, every non-empty physical
line of an ordinary or documentation block comment receives the same
comment classification and modifiers as its enclosing lexer token. Segment
starts and lengths use the protocol's UTF-16 code-unit coordinates, including
when astral Unicode scalars occur before or inside the segment.

**§ 4.1(21)** `new` is classified as a keyword. Bare lifecycle `init` and `free` members are
recognized contextually inside impl bodies without changing the classification
of an ordinary function named `init`.

**§ 4.1(22)** Hover and signature help for an initializer show its parameters, constructed
type, and optional construction error separately. Hover on `new Type(args...)`
shows the selected init overload and whether `try`/handling is required.
Definition/references connect lifecycle construction to the selected explicit
init (or identify the permitted implicit path). Completion offers `init` and
`free` only where valid in an impl body and offers `new` at expression sites.

### Project and source integration

**§ 4.1(23)** Implemented:

- core loading from `sec/core`;
- project-root discovery through `.sec/sec.toml`;
- project source import resolution;
- grouped imports;
- platform import resolution;
- loading all `.sec` files in a module directory;
- canonical stdlib file and directory-module resolution under `sec/stdlib`;
- short import qualifier rewriting;
- transitive source imports;
- mandatory `S1051` diagnostics for unresolved canonical imports at the exact
  source path, including actionable import-root guidance.

### VS Code extension

**§ 4.1(24)** Implemented:

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

**§ 4.1(25)** Implemented tests cover, among other areas:

- compiler diagnostics;
- stable Sema diagnostic IDs;
- help text;
- core loading;
- stdlib imports;
- transitive imports;
- project-root imports;
- platform imports;
- grouped imports;
- unresolved canonical imports in individual and batched diagnostics;
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

### § 4.2 Partially implemented

### Shared compiler workspace

**§ 4.2(1)** The LSP uses the compiler's lexer, parser, AST, Sema, and diagnostics packages.

**§ 4.2(2)** However, it still directly orchestrates compilation independently inside
`cmd/lsp`.

**§ 4.2(3)** The LSP currently constructs:

```text
lexer
parser
core source inclusion
import inclusion
Sema analyzer
```

**§ 4.2(4)** for multiple features.

**§ 4.2(5)** This must move into a shared compiler workspace and snapshot API.

### Diagnostics

**§ 4.2(6)** Sema diagnostics are structured.

**§ 4.2(7)** Parser diagnostics are currently plain strings whose positions are recovered by
parsing message text.

**§ 4.2(8)** Parser diagnostics must become structured compiler diagnostics containing:

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

**§ 4.2(9)** The formatter is currently implemented directly inside `cmd/lsp/main.go`.

**§ 4.2(10)** It is not yet shared with `sec fmt`.

**§ 4.2(11)** The implementation is useful but architecturally misplaced.

### Incomplete-source handling

**§ 4.2(12)** Completion and semantic tokens contain selected recovery behavior.

**§ 4.2(13)** The central LSP parse paths now consume the parser's `ParseResult`, including
structured diagnostics, recovery context, episode association and error state.
The partial AST retains failed declarations, statements, impl members,
match/handler patterns, and selected invalid expressions/types. Episode-aware
deduplication reduces duplicate parser diagnostics before publication. A
shared delimiter stack now keeps migrated recovery scans inside their nested
parenthesis, bracket, and brace boundaries and exposes their skipped ranges to
tooling.

**§ 4.2(14)** The general parser and compiler workspace do not yet provide a complete
error-tolerant syntax tree.

**§ 4.2(15)** After a non-fatal parser error, the LSP may continue semantic analysis over
retained declarations and unaffected subtrees. It must not publish semantic
diagnostics whose primary location lies inside a parser recovery range, and a
local malformed body must not hide independent declaration diagnostics in
following siblings.

### Project loading

**§ 4.2(16)** Project and stdlib loading exist but remain feature-specific and partly
hardcoded.

**§ 4.2(17)** The final compiler workspace must resolve all core, stdlib, project, platform,
target, and generated sources through the canonical module system.

### Document snapshots

**§ 4.2(18)** Open-document text now has a versioned snapshot store. `didOpen` and full-text
`didChange` updates record versions and reject stale updates. Feature handlers
read immutable snapshots. `didClose` removes the overlay and clears published
diagnostics; `didSave` refreshes diagnostics. `willSave` and
`willSaveWaitUntil` are supported, with the latter currently returning no edits.
Incremental edits and stale-result suppression remain pending.

### Protocol transport and types

**§ 4.2(19)** JSON-RPC framing and envelope types now live in `internal/lsp/protocol`.
Request routing and most LSP feature parameter types remain in `cmd/lsp` until
the server and feature packages are extracted.

**§ 4.2(20)** The types are still handwritten. This is sufficient for the present subset but
not for complete LSP 3.18 coverage.

---

### § 4.3 Not implemented

**§ 4.3(1)** The following major areas are not yet implemented:

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
- code lens;
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

## § 5 Architecture

### § 5.1 Required package structure

**§ 5.1(1)** The implementation should move toward:

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

**§ 5.1(2)** Suggested feature files:

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

**§ 5.1(3)** Suggested Sec-specific files:

```text
internal/lsp/sec/
    ownership.go
    contracts.go
    units.go
    targets.go
    analysis.go
    lowering.go
```

**§ 5.1(4)** `cmd/lsp/main.go` should eventually contain only:

```text
process startup
stdio selection
server construction
logging setup
exit status
```

---

### § 5.2 Compiler workspace

**§ 5.2(1)** A long-lived compiler workspace owns:

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

**§ 5.2(2)** The LSP does not construct isolated frontend pipelines per feature.

**§ 5.2(3)** Conceptual API:

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

**§ 5.2(4)** The same workspace services should be reusable by CLI tools.

---

### § 5.3 Snapshots

**§ 5.3(1)** Every request operates on an immutable workspace snapshot.

**§ 5.3(2)** A snapshot includes:

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

**§ 5.3(3)** Results from an older snapshot must not overwrite results for a newer document
version.

**§ 5.3(4)** Analysis is deterministic for a given snapshot.

**§ 5.3(5)** Call-graph results are valid only for the exact snapshot and
`CompilationPlan` from which the compiler produced them. A target switch,
active implementation-variant change, relevant declaration change, or changed
indirect target set invalidates affected graph facts. Partial or completed
results from an older graph snapshot must not overwrite newer results.

---

### § 5.4 Incremental analysis

**§ 5.4(1)** The workspace must invalidate only affected results.

**§ 5.4(2)** Dependency tracking should distinguish:

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

**§ 5.4(3)** The system may reuse:

```text
unchanged tokens
unchanged syntax subtrees
unchanged declarations
unchanged generic instantiations
unchanged symbol indexes
unchanged call graph regions
unchanged target analyses
```

**§ 5.4(4)** Correctness takes priority over reuse.

---

## § 6 LSP protocol requirements

### § 6.1 Protocol version

**§ 6.1(1)** The target protocol version is:

```text
LSP 3.18
```

**§ 6.1(2)** Protocol types should be generated from the official LSP 3.18 meta model where
practical.

**§ 6.1(3)** Generated enum handling must preserve unknown values when the protocol permits
future extension.

---

### § 6.2 Base protocol

**§ 6.2(1)** Required:

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

**§ 6.2(2)** One malformed request must not terminate the server unless transport integrity is
lost.

---

### § 6.3 Lifecycle

**§ 6.3(1)** Required:

```text
initialize
initialized
shutdown
exit
```

**§ 6.3(2)** The server must reject invalid lifecycle ordering according to the protocol.

**§ 6.3(3)** Initialization must inspect client capabilities before advertising server
capabilities.

---

### § 6.4 Document synchronization

**§ 6.4(1)** Required:

```text
didOpen
didChange
didClose
didSave
willSave
willSaveWaitUntil
```

**§ 6.4(2)** Incremental synchronization should be the preferred mode.

**§ 6.4(3)** Full synchronization may remain as a negotiated fallback.

**§ 6.4(4)** Document versions are mandatory for change ordering.

---

### § 6.5 Language features

**§ 6.5(1)** The server must implement every applicable standard feature:

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

**§ 6.5(2)** Features must use compiler facts and shared indexes.

---

### § 6.6 Workspace features

**§ 6.6(1)** Required where applicable:

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

**§ 6.6(2)** Workspace edits must be version-aware.

---

### § 6.7 Window and progress features

**§ 6.7(1)** Required:

```text
show message
show message request
log message
show document
work-done progress
partial results
```

**§ 6.7(2)** Long analyses must support cancellation and progress reporting.

**§ 6.7(3)** Call hierarchy, root reachability, path explanations, and graph visualization
may return partial results when the client supports them. Every result must
identify its snapshot and active `CompilationPlan`; cancellation must stop
further traversal without publishing an incomplete result as a complete closed
target set or complete cause path.

---

## § 7 Responsiveness model

**§ 7(1)** The LSP must remain interactive while deep analysis continues.

**§ 7(2)** Diagnostic refreshes for open siblings are coalesced into one module job.
A job analyzes one captured overlay and routes compiler diagnostics to each
source document; it must not repeat whole-module analysis for each open file.
Superseded jobs and results are discarded, publications carry document versions,
and shutdown prevents pending diagnostic publication.

### § 7.1 Immediate tier

**§ 7.1(1)** Target response:

```text
within one editor frame where practical
```

**§ 7.1(2)** Includes:

- lexical tokens;
- bracket and delimiter knowledge;
- local syntax diagnostics;
- obvious syntax fixes;
- basic local completion;
- local document symbols.

### § 7.2 Fast tier

**§ 7.2(1)** Includes:

- local Sema;
- type-aware completion;
- hover;
- signature help;
- ownership state;
- contract state;
- unit resolution;
- local code actions.

### § 7.3 Deep tier

**§ 7.3(1)** Includes:

- workspace references;
- call graph;
- stack analysis;
- allocation paths;
- escape analysis;
- all-target analysis;
- race analysis;
- deadlock analysis;
- large refactorings.

**§ 7.3(2)** Deep results arrive asynchronously but are tied to the exact snapshot.

---

## § 8 Error-tolerant syntax model

### § 8.1 Recovery nodes

**§ 8.1(1)** The parser must support explicit recovery structures such as:

```text
MissingToken
ErrorExpression
ErrorStatement
RecoveredDeclaration
RecoveredParameter
RecoveredType
RecoveredBlock
```

**§ 8.1(2)** Conceptual recovered parameter:

```text
Parameter {
    Name: a
    MissingColon: true
    Type: string
}
```

**§ 8.1(3)** The recovered node lets the LSP understand intended structure while preserving
the error.

---

### § 8.2 Stable syntax identity

**§ 8.2(1)** Unchanged declarations and subtrees should retain stable identities across
nearby edits where practical.

**§ 8.2(2)** This improves:

- semantic-token deltas;
- diagnostics stability;
- code-lens stability;
- symbol navigation;
- refactoring previews;
- incremental analysis.

---

### § 8.3 Recovery limits

**§ 8.3(1)** Recovery must not fabricate arbitrary semantics.

**§ 8.3(2)** A recovered fact must record confidence:

```text
exact
unambiguous recovery
probable recovery
unknown
```

**§ 8.3(3)** Only exact and unambiguous recoveries may produce automatic safe fixes.

---

## § 9 Formatting and fixing

### § 9.1 Shared pipeline

**§ 9.1(1)** The canonical edit pipeline is:

```text
source snapshot
    -> optional safe fixes
    -> structured edits
    -> canonical formatter
    -> text edits
```

**§ 9.1(2)** A code action or refactoring must not hand-format its generated text.

**§ 9.1(3)** It constructs a structured edit and delegates canonical output to the formatter.

---

### § 9.2 Ordinary formatting

**§ 9.2(1)** Used by:

```text
sec fmt
textDocument/formatting
textDocument/rangeFormatting
format on save
```

**§ 9.2(2)** Ordinary formatting:

- preserves semantics;
- does not require successful Sema;
- preserves comments;
- preserves literal values;
- preserves string contents;
- preserves ordinary copy versus move syntax;
- produces canonical Sec style;
- is idempotent.

**§ 9.2(3)** It may normalize accepted, unambiguous noncanonical syntax.

---

### § 9.3 Syntax normalization without `--fix`

**§ 9.3(1)** The formatter may normalize:

```text
func -> fn
x++  -> x += 1
x--  -> x -= 1
```

**§ 9.3(2)** only when the intended construct is unambiguous.

**§ 9.3(3)** Example:

```sec
func Run() void {
}
```

**§ 9.3(4)** becomes:

```sec
fn Run() void {
}
```

**§ 9.3(5)** A call or identifier named `func` remains unchanged.

**§ 9.3(6)** Example:

```sec
func(callback)
```

**§ 9.3(7)** remains unchanged.

**§ 9.3(8)** Increment and decrement are statement-only aliases.

**§ 9.3(9)** They do not produce a value.

**§ 9.3(10)** The resulting compound assignment must still satisfy mutability, contract, and
`try` requirements.

---

### § 9.4 Safe fixes

**§ 9.4(1)** Used by:

```text
sec fmt --fix
LSP quick fix
LSP fix all
safe fixes on save
```

**§ 9.4(2)** Safe fixes may use Sema.

**§ 9.4(3)** A fix is safe only when:

- the intended correction is unique;
- the corrected program preserves the apparent intent;
- no unrelated overload changes;
- no ownership effect is hidden;
- no new required user choice appears;
- affected source ranges are exact;
- the result can be formatted canonically.

**§ 9.4(4)** Examples:

```text
insert missing parameter colon
replace invalid copy syntax with explicit move syntax when move is the only valid operation
add a required import when exactly one canonical module provides the symbol
add a missing comma when grammar and surrounding structure are unambiguous
```

---

### § 9.5 Missing comma

**§ 9.5(1)** Implemented (2026-10-03). A P2002 missing-comma diagnostic whose repair the
parser proved — a struct field, parameter, call argument, or array element
that begins on a later line than the previous item — offers:

```text
Insert missing ','
```

**§ 9.5(2)** The edit inserts `,` directly after the previous item and before any trailing
comment, exactly as the opt-in formatter correction of
`rules/tooling/formatter.md` § 27(37). Same-line adjacency gets no quick fix.

---

### § 9.6 Missing parameter colon

**§ 9.6(1)** Input:

```sec
fn myfunc(a string) void {
}
```

**§ 9.6(2)** Diagnostic:

```text
expected `:` between parameter name `a` and type `string`
```

**§ 9.6(3)** Quick fix:

```text
Insert missing `:`
```

**§ 9.6(4)** Result:

```sec
fn myfunc(a: string) void {
}
```

**§ 9.6(5)** Ordinary `sec fmt` does not apply this fix.

**§ 9.6(6)** `sec fmt --fix` may apply it.

**§ 9.6(7)** Safe fixes on save may apply it.

**§ 9.6(8)** Optional inline correction may apply it while typing when enabled.

---

### § 9.7 Ownership fix

**§ 9.7(1)** Input:

```sec
let destination := source
```

**§ 9.7(2)** when the resolved copy classification does not permit copying `source` and an
explicit ownership transfer is legal.

**§ 9.7(3)** Quick fix:

```sec
let destination :<- source
```

**§ 9.7(4)** The fix is considered automatically safe only when no later source use becomes
invalid.

**§ 9.7(5)** When later uses exist, the action must explain the consequence and be classified
as a refactoring or multi-edit fix rather than a trivial safe fix.

**§ 9.7(6)** Ordinary formatting must never change:

```sec
let value := Function()
```

**§ 9.7(7)** to:

```sec
let value :<- Function()
```

**§ 9.7(8)** Direct construction from a temporary already uses the canonical form.

---

### § 9.8 Save behavior

**§ 9.8(1)** Recommended defaults:

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

**§ 9.8(2)** Users may configure the categories independently.

**§ 9.8(3)** An editor client must not hide which fixes were applied.

---

## § 10 Diagnostics

### § 10.1 Shared diagnostic model

**§ 10.1(1)** Compiler diagnostics must contain:

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

**§ 10.1(2)** The LSP maps this structure to protocol diagnostics.

**§ 10.1(3)** Protocol positions use the client's negotiated position encoding. While the
server does not negotiate another encoding, every feature maps compiler
Unicode-scalar columns and token widths to the protocol's UTF-16 code-unit
default. Astral Unicode scalars therefore occupy two protocol character units
even though the lexer counts them as one source column. The shared conversion
applies independently to both ends of multiline diagnostic, hover, navigation,
highlight, call-hierarchy, document-symbol, edit, and semantic-token ranges.
Incoming hover, completion, signature, navigation, and edit positions undergo
the inverse UTF-16-to-byte conversion. Both directions treat LF, CRLF, and bare
CR as one physical line ending and must not reinterpret compiler scalar columns
as protocol columns.

**§ 10.1(4)** It must not parse compiler message text to rediscover locations or IDs.

---

### § 10.2 Explanation quality

**§ 10.2(1)** A diagnostic should explain the cause chain.

**§ 10.2(2)** Example:

```text
This call is not ISR-safe
because it calls Log
which calls Write
which may block in platform/linux/file.sec
```

**§ 10.2(3)** Every relevant chain entry should be navigable.

---

### § 10.3 Diagnostic origin

**§ 10.3(1)** The client must be able to distinguish:

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

### § 10.4 Safety and advisory diagnostics

**§ 10.4(1)** Mandatory language-safety errors cannot be disabled.

**§ 10.4(2)** Advisory diagnostics use the configurable diagnostics model:

```text
off
info
warning
error
```

**§ 10.4(3)** Examples include:

```text
style.merge-compatible-declarations
style.multiple-primary-types-per-file
style.type-file-name-mismatch
maintainability.large-function
ownership.implicit-discarded-result
```

---

### § 10.5 Diagnostic preview

**§ 10.5(1)** Before applying a fix, the LSP should show:

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

## § 11 Completion

**§ 11(1)** Completion must be semantic and context-sensitive.

**§ 11(2)** Member completion uses the receiver at the active document cursor, including
selectors nested in calls and `Ok(...)` / `Err(...)`. Same-module declarations
from disk and unsaved overlays participate even when other declarations have
recoverable syntax errors.

**§ 11(3)** It considers:

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

### § 11.1 Completion ranking

**§ 11.1(1)** Prefer:

1. exact valid symbols in scope;
2. members valid for the resolved receiver;
3. expected-type matches;
4. already imported symbols;
5. auto-import candidates;
6. syntax keywords valid at the position;
7. snippets and templates.

**§ 11.1(2)** Invalid or unavailable candidates should not appear as ordinary top-ranked
completion items.

**§ 11.1(3)** Expected-type member filtering (decision 2026-10-03): when a member selector is
written directly as a returned value — in a function or method body, or in a
property getter, whose expected type is the property's declared type — member
completion offers only members whose value type matches the expected type:
fields, properties, events, and compiler-known members by their type, methods
by any overload's return type, and enum members and union variants by their
owning type. Members of other types are not offered there, even as
intermediate steps of a longer selector chain. Inside a property getter, the
property itself is not offered on `self`, because returning it would recurse.
Outside a direct return value, member completion is unfiltered.

**§ 11.1(4)** The same expected-type rule applies to subject and operand positions
(decision 2026-10-03):

- the right operand of `==` or `!=` completes only values of the left
  operand's type; after `if self.OpenMode == ` an enum-typed left operand
  offers its qualified members (`FileMode.OpenExisting`) plus visible symbols
  and functions of that type, and `== self.` offers only members of that type;
- a `case` item of a subject `switch` offers the subject type's values, with
  enum members already named by another case left out;
- the pattern at the start of a `match` arm offers the subject's variants not
  yet covered by an unguarded arm — qualified enum members, bare union and
  Option variants, `Ok` and `Err` for a Result — followed by `_`;
- the variant position of a non-binding `is` or `is not` test offers only
  the Sema-resolved subject's union variants, `Some`/`None` for Option, or
  `Ok`/`Err` for Result, filtered by the typed prefix. A matching type
  qualifier may precede the variant. This works in if/while headers and
  ordinary boolean expression positions, including unfinished headers,
  member and call subjects, and negated tests. It introduces no payload
  binding or wildcard and invents no variants for unknown or non-union
  subjects. Strings, comments, completed tests, and ordinary members named
  `is` retain their ordinary completion behavior (2026-10-14).

**§ 11.1(5)** Within a `select` operation position, completion offers only canonical
selectable operations valid for the resolved type and context. In Sec 0.1 it
must not offer `TrySend`, `TryReceive`, `TryLock`, `TryAcquire`, or
`CancelRequested` as primitive select candidates. Hover on a branch binding
shows the selected operation's exact ordinary result type, and navigation
resolves to the owning ordinary operation or type declaration rather than a
duplicate select-only declaration.

---

### § 11.2 Unavailable completion explanation

**§ 11.2(1)** The LSP may expose a secondary explanation for unavailable items.

**§ 11.2(2)** Examples:

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

### § 11.3 Completion resolve

**§ 11.3(1)** Expensive information may be filled through completion-item resolve:

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

### § 11.4 Auto-import

**§ 11.4(1)** The LSP may index unimported core and stdlib declarations for completion.

**§ 11.4(2)** It must not make those symbols silently visible to Sema.

**§ 11.4(3)** Instead it provides an additional text edit or code action inserting the
canonical import.

**§ 11.4(4)** Example:

```sec
import "units"
```

**§ 11.4(5)** Sec does not require a `std/units` source spelling.

---

## § 12 Hover

**§ 12(1)** Hover should present the most useful compiler-known information.

**§ 12(2)** Hover on a string `+` or `+=` states the source-selected failure mode from Sema's
plan and separates ordinary materialization, which may panic, from `try`, which
reports `StringError`, and from an unhandled `Result` operand, which is invalid
(MD-039). Its `try` quick fixes and refactors are offered only when analysis of the
edited source proves them; `+=` with an unhandled operand offers both `try`
placements, naming that the operand's `try` leaves the append in panic mode.

**§ 12(3)** General hover may include:

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

**§ 12(4)** For a resolved callable, hover may additionally expose compiler-owned call-graph
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

**§ 12(5)** The presentation must distinguish proven direct facts, conservative possible
facts, and open-world contracts. Unknown facts must not be displayed as closed
or complete.

**§ 12(6)** Unknown facts must be omitted or explicitly marked unavailable.

**§ 12(7)** The LSP must never invent metadata that the compiler has not resolved.

**§ 12(8)** Hover on `try` and its operand consumes Sema's resolved revision-2 facts. It
shows the protected carrier/operation, success type, locally handled states,
unhandled states that propagate, propagation target, concrete-to-`error`
widening, and remaining panic effect where relevant. For Option it explains
Some success and None propagation/recovery; it must not claim Option cannot use
try.

**§ 12(9)** Focused coverage includes single and multiple partial Result handlers, guarded
catch-all handlers, guarded handlers followed by variant handlers, exhaustive
catch-all fallback, and residual concrete-to-`error` widening. Checked arithmetic,
indexing, and narrowing conversions are covered at both the `try` keyword and
protected operation. Invalid operands, incompatible enclosing error channels,
and invalid handler recovery values do not receive a successful try-flow hover;
valid neighboring tries remain visible in a document with unrelated errors.
Handler presentation requires a completed Sema handler plan, and incompatible
Result propagation does not publish a completed Sema propagation fact.

**§ 12(10)** Carrier-incompatible handler diagnostics explain the actual carrier and offer
the correct Result or Option patterns. Completion and hover expose `.Ok()` and
`.Err()` as consuming projections returning Option, and `OkRef`/`ErrRef` as
shared borrowed properties. Later-use diagnostics link to a consuming
projection. Fallible-setter hover/signature help shows its explicit error type,
and open-error hover may distinguish static type `error` from a concrete error
identity proven by Sema. The LSP never reconstructs a parallel Result, Option,
error, or ownership model.

**§ 12(11)** Hover on `await worker` consumes Sema's resolved operand specialization and
always reports `TaskOutcome[T]` with the concrete `T`; it must not display bare
`T` or `Result[T, TaskError]`. Hover, completion, and navigation expose the
exact closed `TaskError` members `OutOfMemory`, `ResourceLimit`,
`ExecutorUnavailable`, and `NativeFailure`. `PanicInfo` navigation resolves to
the single canonical panic declaration. Await-related examples and code actions
use `RequestCancel()` and never synthesize legacy `cancel()`.

### § 12.1 Default information

**§ 12.1(1)** Hover and type completion expose the shared compiler-resolved default value and
its origin, including explicit type defaults, range-derived defaults and the
first value of `in [...]`. Non-defaultable types are reported as having no
default.

**§ 12.1(2)** For omitted defaultable struct fields, inlay information may show the resolved
value without modifying source. Explicit code actions may expand defaulted
fields, insert an explicit default value, or declare a named-type default. These
are refactorings and must not run as ordinary formatting. The LSP must call the
same `DefaultValueOf` semantics as Sema rather than reimplement selection.

**§ 12.1(3)** An error parameter (MD-120) is presented so that a constrained unknown `E: error`, the explicit root `error` and a concrete instantiation stay distinct; a specialized location shows its concrete argument. Completion in a template offers only whole-channel `Err(...)` bindings, never variants of an unknown `E`.

---

## § 13 Signature help

**§ 13(1)** Signature help includes:

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

**§ 13(2)** A consuming move-only parameter should be clearly marked.

---

## § 14 Navigation and references

**§ 14(1)** Required:

```text
go to declaration
go to definition
go to type definition
go to implementation
find references
document highlight
```

**§ 14(2)** Navigation must work across:

```text
project modules
core
stdlib
platform sources
generated sources
target-specific variants
```

**§ 14(3)** References distinguish:

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

## § 15 Rename

**§ 15(1)** Rename must be semantic.

**§ 15(2)** It must:

- resolve the exact symbol;
- respect visibility;
- update all valid references;
- avoid unrelated equal spellings;
- preserve contextual `x`;
- preserve contextual `set`;
- respect generated and read-only sources;
- update documentation links where supported;
- preview multi-file changes.

**§ 15(3)** Prepare-rename must reject keywords, invalid scopes, and non-renamable compiler
symbols.

---

## § 16 Semantic tokens

**§ 16(1)** Semantic tokens should distinguish Sec-specific meaning when client capability
allows it.

**§ 16(2)** Useful custom token types or modifiers may include:

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

**§ 16(3)** Fallback clients use standard token types and modifiers.

**§ 16(4)** Moved or unavailable bindings may be visually de-emphasized.

**§ 16(5)** The semantic meaning remains available independently of color configuration.

---

## § 17 Inlay hints

**§ 17(1)** Inlay hints should be individually configurable.

**§ 17(2)** Implemented (2026-10-04), `textDocument/inlayHint` from Sema facts:

- inferred type (`sec.inlayHints.types`): `: T` after the name of a `let`
  without a declared type, from the resolved binding; left out when the
  initializer already spells the type (struct literal, `new`, or a conversion
  call named like the type);
- inferred loop/pattern/Option-if binding types (`sec.inlayHints.types`,
  2026-10-14): `: T` after each resolved non-discard loop name, match payload
  binding, positive `is Some(binding)` name, and plain `Err(name)` try-handler
  binding. The exact declaration/navigation fact supplies the type and borrow
  authority, so shadowed names stay distinct. These hints use the same active-file
  and requested-range filters and type-category setting as let hints. Unsupported
  or unresolved patterns acquire no guessed binding type; underlying shallow
  field-pattern analysis remains tracked by `frontend.match-v2` in
  `governance/control_flow.yaml`;
- parameter name (`sec.inlayHints.parameters`): `name:` before each argument
  of a Sema-resolved call with two or more arguments, stopping at a variadic
  parameter or spread argument and left out where the argument's final name
  already equals the parameter name;
- omitted struct-field defaults (`sec.inlayHints.defaults`, 2026-10-14): a
  `defaults: field: value, …` preview immediately before a source literal's
  closing brace, from the validated struct construction plan's default-sourced
  final fields and the shared rendering of its stored `DefaultValueOf` results.
  Explicit fields and spread-supplied fields are excluded. Nested literals get
  their own hints; synthetic defaults and invalid/incomplete literals get none.
  Preview labels are limited to eight fields and 160 Unicode scalars, with
  individual values limited to 80 scalars and ellipsis denoting omitted preview
  text. No source edit or inferred semantic default is introduced;
- these categories are on by default and individually switchable through
  `initializationOptions.inlayHints` and `workspace/didChangeConfiguration`
  (`sec.inlayHints.*` in VS Code); hints are restricted to the requested range
  and never change source.

**§ 17(3)** Ownership hints (`sec.inlayHints.ownership`, 2026-10-04) show only effects
the source does not spell, from Sema facts:

- `ref` or `ref mut` before a plain place passed to a reference parameter,
  the implicit call-bounded borrow of `rules/memory/borrowing.md` § 15.2;
- `moves if selected` after a by-value match payload binding whose resolved
  action moves a move-only payload, placed on the binding and never on the
  subject;
- `move` before a reusable source implicitly transferred through a terminal
  return or its owning construction path (`copy_move.md` §§ 9.1–9.2, 10),
  and before a receiver consumed by an accepted method call, including owned
  `Result.Ok()` / `Err()` (`errorhandling.md` §§ 6.1, 29). These hints project
  Sema's validated transfer plans and per-call receiver-move facts, including
  partial Places; the LSP never infers consumption from a method's name.
  Explicit source `<-` markers, ordinary copying returns, fresh receivers,
  non-consuming methods and rejected receiver operations get no `move` hint;
- `copy` before a plain place passed by value or used as a `let` initializer
  when its type copies semantically, and after a semantically copying match
  binding.

**§ 17(4)** Explicit `<-` and `ref` forms, temporaries, existing references, and trivial
copies get no hint.

**§ 17(5)** Inlay hint resolve (2026-10-29): `inlayHintProvider.resolveProvider`
advertises `inlayHint/resolve`. Hints retain stable label parts and opaque data
for the exact source snapshot. Resolve adds Markdown tooltips and declaration
locations from the existing resolved binding type or selected call parameter;
imported declarations retain their own URI and range. Types without one physical
declaration do not acquire a guessed target. Ownership and default-preview hints
explain the already resolved effect or omitted defaults without changing source.

**§ 17(6)** The client’s `textDocument.inlayHint.resolveSupport.properties` controls lazy
`tooltip`, `label.tooltip` and `label.location` properties independently;
unsupported lazy properties are supplied when listing hints. Resolve regenerates
the current hint using current compiler facts and settings, preserves its label,
position, kind and padding, and ignores client-supplied decorations. Malformed
or unmatched hints are rejected; changed or closed source snapshots require a
fresh hint request. Resolve uses the read-only worker queue and is superseded by
queued document changes or close notifications.

**§ 17(7)** Pending: generic instantiation, resolved
overload, unit conversion, contract, error type, allocation and target hints.

**§ 17(8)** Possible categories:

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

**§ 17(9)** Call navigation and references also distinguish:

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

**§ 17(10)** Examples:

```text
resource /* moved */
```

```text
value /* copied */
```

**§ 17(11)** Hints must not overwhelm the source by default.

**§ 17(12)** Ownership hints for non-obvious consumption are high priority.

---

## § 18 Code lens

**§ 18(1)** Code lenses may expose:

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

**§ 18(2)** Generic declarations, concrete specialization identity, substitution context,
and generic diagnostics come from the compiler workspace and the canonical
boundary in `rules/compiler/generics_lowering.md`. The LSP must not implement an
independent generic-lowering engine. Navigation from a concrete use resolves to
its source generic declaration; concrete-specialization inspection may be
presented separately, with origin-module provenance retained when available.

**§ 18(3)** Call-graph lenses use the compiler-owned graph for the active
`CompilationPlan`. They may show direct and possible counts separately, mark an
open callable contract, identify roots, and expose same-stack recursion, spawn
origins, synchronization origins, and available cause paths. A possible or
open-world relationship must not be presented as a confirmed direct call.

**§ 18(4)** Example:

```text
Stack: 416 bytes local, 1.8 KiB maximum path
Allocations: none
Blocking: no
Targets: 3 valid, 1 invalid
```

**§ 18(5)** Expensive lenses may load lazily.

---

## § 19 Shaped values

**§ 19(1)** Completion and hover for shaped values consume the canonical compiler-known
member registry and resolved Sema facts. The LSP must not maintain an
independent shaped API table.

**§ 19(2)** Hover presents the exact resolved expression/member type and exact property or
call return type. Where useful for correct use it additionally presents `Rank`,
`Shape`, `Len`, whether shape knowledge is static or runtime, `IsContiguous`,
`Layout`, `MemorySpace`, an unsafe `Ptr` requirement, and any runtime shape
check requiring `try`.

**§ 19(3)** Hover must preserve the distinction between normal absence and failure. For
example, `Normalize() -> Option[vector[float32, 3]]` states that zero magnitude
produces `None`, while a storage-producing operation displays its exact
`Result[..., StorageError]` and principal failure condition.

**§ 19(4)** When a shaped operation requires `try` only because compatibility is not
statically proven, hover states the runtime shape condition rather than implying
allocation failure. When Sema proves compatibility, hover presents the
infallible result. It must never invent shaped facts that Sema has not resolved.

---

## § 20 Ownership support

**§ 20(1)** Ownership is a first-class editor feature.

### § 20.1 Hover state

**§ 20.1(1)** Example:

```text
Type: Buffer
Ownership: move-only
Owner: buffer
State: Available
Active borrows: 0
Destruction: end of scope
```

**§ 20.1(2)** After move:

```text
Type: Buffer
State: Moved
Moved to: activeBuffer
Move location: line 18
```

---

### § 20.2 Partial values

**§ 20.2(1)** For a partially available struct:

```text
session
State: Partially available

Available:
    name
    timeout

Moved:
    file
```

**§ 20.2(2)** The LSP should explain which operations require the complete value.

---

### § 20.3 Ownership actions

**§ 20.3(1)** Possible actions:

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

### § 20.4 Ownership preview

**§ 20.4(1)** An ownership-changing action must explain its effects.

**§ 20.4(2)** Example:

```text
Change `:=` to `:<-`

source becomes unavailable after line 14
later use at line 21 would become invalid
```

**§ 20.4(3)** Such an action is not classified as automatically safe unless all consequences
are resolved.

---

### § 20.5 Match ownership and coverage

**§ 20.5(1)** The LSP consumes compiler-resolved match facts and does not implement a
parallel pattern, ownership, borrow, or exhaustiveness analyzer.

**§ 20.5(2)** Hover on a payload or field binding exposes its resolved type and binding mode:

```text
copy
move
shared borrow
mutable borrow
temporary forwarding
```

**§ 20.5(3)** For a guarded move-only binding, tooling distinguishes a prospective move from
a committed move and may display `moves payload if this arm is selected`.
Pattern success alone must not make the subject appear moved.

**§ 20.5(4)** After a match, hover and mandatory diagnostics expose compiler-produced
`Moved`, `Partially available`, or `Conditionally available` state and retain
related locations for the affected Place, moving arm, ownership binding, move
commit, later invalid use, and merge reason where available.

**§ 20.5(5)** Configurable inlay hints may show `copy`, `move`, `ref`, `ref mut`, or `moves if
selected`. Disabling hints never disables ownership analysis or diagnostics.

**§ 20.5(6)** Exhaustiveness diagnostics and generated-match actions use the resolved
coverage domain, including ordinary enum underlying value classes, open bit-enum
domains, guarded residual coverage, and reachable compiler-known union `empty`
state. Generated patterns must not introduce direct bool, general literal,
range, nested recursive, or ordinary struct-subject patterns.

**§ 20.5(7)** Completion and code actions must not recommend payload use that is invalid in
the compiler-resolved ownership state.

### Error patterns, inactive arms and `Some` (MD-029/MD-042/MD-102)

**§ 20.5(8)** The parsed contextual `Err(.Variant...)` form, its resolved qualified variant
and its payload-binding sites keep source locations for hover, diagnostics,
references and go-to-definition. After `Err(.` completion offers the variants
of the one concrete error type supplied by the protected channel or `Result`
subject, never a workspace-wide name search; for an open `error` channel or a
heterogeneous context it suggests a qualified concrete type instead. Hover on a
bare payload-bearing variant may show the omitted payload fields, and hover on
an inactive arm may show the proof; both come from compiler facts and never
rewrite source. Quick fixes for the omitted-payload and inactive-arm
Information diagnostics are optional and semantic-fact driven. Hover on a
`Some(value)` shows the selected `Option[T]` and constructor. CLI, JSON and LSP
report the same advisory identities
(`rules/tooling/diagnostics.md` § 24(4e);
`rules/corrections/applied/md029-md042-md102-pattern-reachability-some-correction-20261028.md`
§§ 10(4), 15(4)).

### Register shadow knowledge (MD-113)

**§ 20.5(9)** Amendment 2026-10-30 (`rules/corrections/applied/MD-113-shadow-knowledge-across-calls-decision-20261030.md` §§ 4(8), 7(7), 9(7)). Completion, hover and
go-to-definition for `self.IsShadowKnown(T.Field...)` use Sema's selector
resolution: the selector links to the final field and shows the whole path, and
no LSP-only parser exists. Reserved-name diagnostics for `Read`, `Write` and
`IsShadowKnown` and their completion come from the compiler-known registry.
Hover on a register `impl` member shows its published static contract: whether
a returned value comes from shadow or from a hardware observation, its
`RequiresKnown` / `EnsuresKnown` / conditional summaries, and whether it may
touch external hardware. This is static API information, never runtime
reflection, and a `Known` shadow value is never presented as current hardware
state.

### Hardware fact declarations (MD-111)

**§ 20.5(10)** Amendment 2026-10-30 (`rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md` § 13(3)). Completion and hover for
`hardware region`, `hardware resource` and `hardware endpoint` members come from
the compiler's metadata schema: allowed members, exact types, requiredness,
enum cases and their meaning, with no separate LSP or formatter copy. Navigation
resolves hardware declaration names, `@address` endpoint paths, `Project.<name>`
exports and manifest override selectors to their canonical identities. The
server reports the selected target address domain, effective property
provenance (source or project override, with the changed-value warning) and
alias facts for bindings.

---

## § 21 Contracts

**§ 21(1)** The LSP should display:

```text
nominal type
base type
contract
compile-time proof status
runtime-check requirement
failing expression
valid range or set
```

**§ 21(2)** Example:

```text
Type: Percent
Base: int
Contract: range 0..100
Proof: runtime check required
```

**§ 21(3)** Code actions may:

```text
insert required try handling
convert literal to valid value
show contract definition
find contract uses
explain proof failure
```

---

## § 22 Units

**§ 22(1)** The LSP must use the compiler's canonical unit model.

**§ 22(2)** It may know that a unit symbol is available from the standard-library module:

```sec
import "units"
```

**§ 22(3)** When a unit is not imported, the LSP offers an import action.

**§ 22(4)** It must not silently inject unit symbols into compiler scope.

---

### § 22.1 Unit hover

**§ 22.1(1)** When compiler metadata exists, hover may show:

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

**§ 22.1(2)** Example:

```text
Name: metre per second
Symbol: m/s
Dimension: Length × Time⁻¹
Scale: 1
Offset: 0
System: SI
```

**§ 22.1(3)** When metadata is not implemented, show only verified facts.
Unknown Kind is shown as unknown and is never guessed from dimension. Tooling
preserves named versus structural source identity and does not rewrite `<m/s>`
to `<mps>`.

**§ 22.1(4)** Example:

```text
Resolved symbol: m
Declared in: units
Dimension metadata: not yet available
```

---

### § 22.2 Unit actions

**§ 22.2(1)** Possible actions:

```text
Import "units"
Convert to compatible unit
Show dimension derivation
Go to unit definition
Find unit references
Show exact scale and offset
```

**§ 22.2(2)** The LSP surfaces compiler diagnostics for incompatible dimension or Kind,
point/difference and Origin misuse, invalid affine/logarithmic arithmetic,
lossy conversion, deprecated units, and derived currency warnings. Expressions
such as `<EUR/s>` and `<SEK/EUR>` remain valid when warned about.

**§ 22.2(3)** Code actions may offer a compiler-proven fixed conversion, declared conversion
function, in-scope factor-provided conversion, or exact carrier change. They
must not invent an exchange rate or rounding policy. Unit algebra and derivation
facts come from Sema rather than an LSP-owned implementation.

---

## § 23 Target-aware analysis

**§ 23(1)** The LSP must understand the active:

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

**§ 23(2)** Users must be able to change the active target without restarting the server.

---

### § 23.1 Multi-target diagnostics

**§ 23.1(1)** Implemented sharing (2026-11-04): background target comparison and interface/type
conformance lenses use one common per-target Sema snapshot service. Identical
active-document, prepared source/import closure, effective target/configuration
and analysis-epoch inputs share an in-flight computation and completed result.
The lens presentation cache follows the same source/plan fingerprint.

**§ 23.1(2)** Snapshots contain detached compiler diagnostics, warnings and structured
conformance facts. Readers receive independent copies; mutable analyzers are
never shared between consumers. Completed snapshots are bounded to 32 entries
and 8 MiB, and failed computations publish no successful empty snapshot. Source
preparation/parsing still runs per request; parsing caches, incremental analysis
and cross-target recomputation while typing remain separate work.

**§ 23.1(3)** A diagnostic records applicability:

```text
all targets
one target
selected outputs
one architecture
one profile
```

**§ 23.1(4)** Example:

```text
linux/amd64: valid
linux/arm64: valid
baremetal/cortex-m4: invalid
    allocation path reaches allocator.New
```


---

### § 23.2 Target status

**§ 23.2(1)** The LSP should provide:

```text
target selection
target status panel
per-target diagnostics
target comparison
active conditional sources
platform symbol navigation
```

---

## § 24 Compiler analysis

**§ 24(1)** Everything produced by compiler analysis should be available in the editor.

### § 24.1 Call graph

**§ 24.1(1)** The canonical semantics are defined by `call_graph.md`. The LSP exposes
compiler-owned graph facts and does not define another graph model.

**§ 24.1(2)** Required callable queries include:

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

**§ 24.1(3)** Call hierarchy, hover, code lens, navigation, and graph views preserve both the
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

**§ 24.1(4)** Same-stack calls are not conflated with task-, thread-, or process-creation
edges. Consequently, same-stack recursion is reported separately from task
spawn cycles, thread-start cycles, and process-launch cycles.

### Call hierarchy

**§ 24.1(5)** Incoming-call results distinguish:

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

**§ 24.1(6)** Outgoing-call results distinguish direct callees, closed possible-target sets,
and open callable contracts. An open target set must retain its compiler-proven
callable contract instead of being shown as an empty or complete target set.

### Root and path explanations

**§ 24.1(7)** For a callable or diagnostic, the LSP may expose all known root classes that
reach it and the compiler-owned cause path for an effect, panic, allocation,
stack contribution, unsafe boundary, reference escape, or ownership fact. Path
steps retain implicit edges and execution-context boundaries.

**§ 24.1(8)** Task-spawn origin, and `await` or `join` synchronization origin where known,
must be navigable. Await and join synchronization are not displayed as ordinary
call edges to the task body. Continuation information is shown as a separate
synchronization fact.

### Target and snapshot requirements

**§ 24.1(9)** Every graph response belongs to one concrete `CompilationPlan` and active
target variant. Cross-plan comparison is presented as comparison of separate
graphs, never as one merged runtime graph.

**§ 24.1(10)** Deep graph requests support cancellation, work-done progress, and partial
results. Partial target discovery must remain visibly partial. The LSP must not
label a target set closed, a path complete, or a root set exhaustive until the
compiler analysis for that snapshot has established that fact.

---

### § 24.2 Stack analysis

**§ 24.2(1)** Display:

```text
semantic local estimate
final target frame
maximum path
recursive uncertainty
interrupt contribution
task entry contribution
```

---

### § 24.3 Allocation and escape analysis

**§ 24.3(1)** Display:

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

### § 24.4 Effect analysis

**§ 24.4(1)** Display:

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

**§ 24.4(2)** Cause chains must be navigable.

---

### § 24.5 ISR analysis

**§ 24.5(1)** Display:

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

### § 24.6 Concurrency analysis

**§ 24.6(1)** When implemented, expose:

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

**§ 24.6(2)** A heuristic finding must be marked as heuristic.

---

### § 24.7 Lowering analysis

**§ 24.7(1)** Optional advanced views may show:

```text
Semantic IR
MLIR
LLVM IR
layout
ABI
target instruction summary
```

**§ 24.7(2)** These are debugging and optimization views.

**§ 24.7(3)** They do not redefine source semantics.

---

## § 25 Better code analysis

**§ 25(1)** Better code analysis identifies valid but improvable code.

**§ 25(2)** It produces advisory diagnostics and refactoring actions.

**§ 25(3)** It is distinct from formatting and compiler-error correction.

---

### § 25.1 Analysis categories

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

### § 25.2 Merge compatible declarations

**§ 25.2(1)** Example:

```sec
let mut Audi: Car
let mut Saab: Car
let mut Volvo: Car
let mut Skoda: Car
```

**§ 25.2(2)** The canonical compact form is:

```sec
Car mut: Audi, Saab, Volvo, Skoda
```

**§ 25.2(3)** Suggested diagnostic:

```text
style.merge-compatible-declarations
```

**§ 25.2(4)** Default severity:

```text
info
```

**§ 25.2(5)** Suggested action:

```text
Merge compatible declarations
```

**§ 25.2(6)** The action is allowed only when:

- declarations are in the same scope;
- declarations have the exact same type;
- declarations have the same mutability;
- declarations have compatible storage and attributes;
- evaluation order remains unchanged;
- comments and documentation remain correctly attached;
- no visibility or lifetime boundary changes;
- no initializer dependency changes meaning.

**§ 25.2(7)** Ordinary formatting does not perform this transformation.

---

### § 25.3 Typed declaration tables

**§ 25.3(1)** Example:

```sec
type TokenType string

TokenType (
    ILLEGAL := "ILLEGAL",
    EOF := "EOF",
    IDENT := "IDENT",
    INT := "INT",
)
```

**§ 25.3(2)** The LSP should understand this as a related declaration table.

**§ 25.3(3)** It may provide:

```text
sort declaration table
add declaration
rename declaration
find declaration references
convert compatible declarations to table
split table
```

**§ 25.3(4)** Sorting must not occur automatically unless declaration order is semantically
irrelevant and the user explicitly requests it.

---

### § 25.4 Function analysis

**§ 25.4(1)** Possible advisory findings:

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

**§ 25.4(2)** A large-function diagnostic should provide evidence.

**§ 25.4(3)** Example:

```text
Statements: 146
Branches: 31
Maximum nesting: 7
Ownership exits: 11
Suggested extraction regions: 3
```

---

## § 26 File and type organization

### § 26.1 Primary type per file

**§ 26.1(1)** The recommended Sec organization follows the broad convention used by several
languages and codebases:

> One primary public nominal type per source file.

**§ 26.1(2)** The same file should normally contain:

```text
the primary type
its ordinary impl block
closely related private helper declarations
nested or subordinate types
direct documentation
```

**§ 26.1(3)** Example:

```text
Car.sec
    type Car struct
    impl Car
```

**§ 26.1(4)** This is a code-quality recommendation, not a mandatory language rule.

---

### § 26.2 File organization diagnostics

**§ 26.2(1)** Suggested advisory diagnostics:

```text
style.multiple-primary-types-per-file
style.type-file-name-mismatch
style.impl-separated-from-type
style.unrelated-declaration-in-file
```

**§ 26.2(2)** Default severity:

```text
info
```

---

### § 26.3 File refactorings

**§ 26.3(1)** Required actions:

```text
Move type to new file
Move type and impl to new file
Split file by primary types
Move impl next to type
Rename file to match type
Move type to another module
Move related private helpers
```

**§ 26.3(2)** Moving within the same module is simpler than moving between modules.

**§ 26.3(3)** Cross-module moves must update:

```text
imports
qualifiers
visibility
internal boundaries
references
tests
generated metadata
```

**§ 26.3(4)** Multi-file moves require preview.

---

## § 27 Refactoring

**§ 27(1)** Refactoring is a core language-server responsibility.

**§ 27(2)** It must use compiler identity and structured edits.

---

### § 27.1 Required refactorings

**§ 27.1(1)** Initial and planned actions include:

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

### § 27.2 Refactoring safety classes

**§ 27.2(1)** Every action is classified.

### Formatting

**§ 27.2(2)** No semantic change.

**§ 27.2(3)** May run automatically.

### Syntax normalization

**§ 27.2(4)** Unambiguous canonicalization.

**§ 27.2(5)** May run automatically.

### Safe semantic fix

**§ 27.2(6)** Compiler-proven unique correction.

**§ 27.2(7)** May run on save when enabled.

### Code cleanup

**§ 27.2(8)** Valid-code improvement preserving behavior.

**§ 27.2(9)** Requires configured cleanup or explicit action.

### Structural refactoring

**§ 27.2(10)** Changes declarations or file organization while preserving behavior.

**§ 27.2(11)** Requires explicit action and preview for multi-file edits.

### Behavior-changing refactoring

**§ 27.2(12)** Changes ownership, API, effects, or runtime behavior.

**§ 27.2(13)** Always requires explicit confirmation and consequence preview.

### Potentially lossy action

**§ 27.2(14)** May drop information, comments, compatibility, or behavior.

**§ 27.2(15)** Must never run automatically.

---

### § 27.3 Refactoring preview

**§ 27.3(1)** Preview must show:

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

### § 27.4 Refactoring examples

### Move type to file

**§ 27.4(1)** Preview:

```text
Create: Car.sec
Move: type Car
Move: impl Car
Keep: __CarParser
Update imports: 0
Module remains: vehicles
```

### Change parameter to ref

**§ 27.4(2)** Preview:

```text
Call sites affected: 17
Moves removed: 13
Copies removed: 4
Lifetime conflicts introduced: 1
```

### Merge declarations

**§ 27.4(3)** Preview:

```text
4 declarations become 1 declaration group
No evaluation-order changes
No comments moved
No symbol-identity changes
```

---

## § 28 Standard-library awareness

**§ 28(1)** The compiler workspace must index the complete standard library.

**§ 28(2)** The LSP may use this index for:

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

**§ 28(3)** Import spelling follows Sec module rules.

**§ 28(4)** Examples:

```sec
import "fmt"
import "units"
```

**§ 28(5)** The LSP must not require artificial `std/` prefixes when the language does not.

**§ 28(6)** Current hardcoded module mappings must be replaced by canonical module discovery.

---

## § 29 Sec-specific protocol extensions

**§ 29(1)** Standard LSP features must be used wherever they are sufficient.

**§ 29(2)** Rich Sec-specific data uses the namespace:

```text
sec/...
```

**§ 29(3)** Possible requests:

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

**§ 29(4)** Every extension must:

- be versioned;
- use capability negotiation;
- degrade gracefully;
- identify the workspace snapshot;
- use compiler-owned facts;
- remain optional for ordinary LSP clients.

---

## § 30 Knowledge Packs

### § 30.1 Status

**§ 30.1(1)** Knowledge Packs are a future feature.

**§ 30.1(2)** They are not required for Sec 0.1.

**§ 30.1(3)** The LSP architecture must leave a clean integration point, but no complete
Knowledge Pack system is required before 0.1.

**§ 30.1(4)** A first public implementation may be targeted for Sec 1.0.

---

### § 30.2 Future purpose

**§ 30.2(1)** A Knowledge Pack provides versioned domain knowledge that enriches compiler and
LSP analysis.

**§ 30.2(2)** Potential domains include:

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

**§ 30.2(3)** Example future pack:

```text
STM32F411
```

**§ 30.2(4)** It may contain:

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

### § 30.3 Future rules

**§ 30.3(1)** A Knowledge Pack may enrich:

```text
hover
completion
diagnostics
navigation
code actions
target validation
documentation
```

**§ 30.3(2)** It may not:

- redefine Sec syntax;
- replace Sema;
- override compiler errors;
- change ownership rules;
- silently add imports;
- silently add symbols to source scope;
- claim unknown facts as proven.

**§ 30.3(3)** Knowledge Pack results must identify:

```text
source
version
applicability
confidence
```

**§ 30.3(4)** Declarative packs are preferred over executable plugins.

**§ 30.3(5)** Executable analyzer plugins, if ever supported, require a separate security and
sandboxing design.

---

## § 31 Privacy, security, and telemetry

**§ 31(1)** The language server must work fully offline.

**§ 31(2)** Source code must not be transmitted externally by default.

**§ 31(3)** Telemetry is disabled by default unless a later explicit project decision
changes that rule.

**§ 31(4)** Any future telemetry must be:

- opt-in;
- documented;
- inspectable;
- free of source text;
- free of identifiers;
- free of file paths unless explicitly approved.

**§ 31(5)** Knowledge Packs must not imply network access.

---

## § 32 Logging

**§ 32(1)** Logging levels should include:

```text
off
error
warning
info
debug
trace
```

**§ 32(2)** Protocol trace logging must be independently configurable.

**§ 32(3)** Logs must avoid dumping entire source documents by default.

**§ 32(4)** Internal panics should produce:

```text
request method
snapshot version
internal stack trace in server log
safe client-facing error
```

**§ 32(5)** The server should remain alive when possible.

---

## § 33 Configuration

**§ 33(1)** Recommended configuration areas:

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

**§ 33(2)** Configuration changes should update the workspace without restarting the server
where possible.

**§ 33(3)** Closure and callable-flow analysis depth is selected in the project manifest:

```toml
[analysis]
lsp_depth = "interactive"
```

**§ 33(4)** Supported values are `interactive`, `standard`, and `deep`. The default is
`interactive` when the section, key, or project manifest is absent. The setting
changes resource budget and optional precision, never source-language semantics
or which safety proofs are required.

**§ 33(5)** The server must re-read this value after project-configuration notifications or
watched-file changes, invalidate analysis facts produced under the old depth,
and refresh affected open-document diagnostics without a server restart.

**§ 33(6)** When pitfall analysis is implemented, its optional configuration and result
filtering follow `pitfall_analysis.md`. Configuration reload must invalidate
affected pitfall state and refresh diagnostics and code actions without a
server restart. Optional pitfall settings never suppress the normative compiler
diagnostic that owns a proven language error.

---

## § 34 Testing

### § 34.1 Protocol tests

**§ 34.1(1)** Required:

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

### § 34.2 Feature tests

**§ 34.2(1)** Each feature requires:

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

### § 34.3 Shared compiler tests

**§ 34.3(1)** Verify that:

```text
sec check
LSP diagnostics
sec fmt --fix diagnostics
```

**§ 34.3(2)** produce the same IDs, ranges, messages, and fixes for the same snapshot.

---

### § 34.4 Formatter integration tests

**§ 34.4(1)** Verify:

- CLI and LSP produce byte-identical formatting;
- ordinary formatting never applies semantic fixes;
- `--fix` and LSP safe fixes share edits;
- formatting is idempotent;
- comments survive;
- incomplete unaffected regions remain formatable.

---

### § 34.5 Refactoring tests

**§ 34.5(1)** Every refactoring tests:

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

**§ 34.5(2)** Multi-file edits must be atomic from the client's perspective.

---

### § 34.6 Performance tests

**§ 34.6(1)** Measure:

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

**§ 34.6(2)** Performance regressions require tracked baselines.

---

## § 35 Required synchronization

**§ 35(1)** This rulebook requires synchronization with:

```text
formatter.md
diagnostics.md
compiler.md
compiler_pipeline.md
compiler_analysis.md
semantic_ir.md
projects.md
modules.md
parser_recovery.md
compiler/incremental_compilation.md
compiler_testing.md
ownership.md
copy_move.md
borrowing.md
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
```

**§ 35(2)** Files that do not yet exist remain planned dependencies.

---

## Appendix A — Codex implementation plan

### Appendix A.1 Preserve working behavior

**Appendix A.1(1)** Before moving code, create coverage for all current behavior.

**Appendix A.1(2)** Do not regress:

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

### Appendix A.2 Move protocol transport

**Appendix A.2(1)** Create:

```text
internal/lsp/protocol
internal/lsp/server
```

**Appendix A.2(2)** Move:

```text
JSON-RPC framing
message types
request routing
responses
notifications
capability negotiation
lifecycle
```

**Appendix A.2(3)** Keep `cmd/lsp/main.go` as bootstrap.

---

### Appendix A.3 Generate protocol types

**Appendix A.3(1)** Use the LSP 3.18 meta model where practical.

**Appendix A.3(2)** Requirements:

- generated files are reproducible;
- unknown extensible enum values are preserved;
- optional and nullable fields follow the protocol exactly;
- no editor-specific assumptions in protocol types;
- generation command is documented and tested.

---

### Appendix A.4 Create compiler workspace

**Appendix A.4(1)** Create:

```text
internal/compiler/workspace
internal/compiler/snapshot
```

**Appendix A.4(2)** Move LSP-specific direct calls to lexer, parser, and Sema behind shared workspace
services.

**Appendix A.4(3)** The workspace must be reusable by CLI commands.

---

### Appendix A.5 Unify source loading

**Appendix A.5(1)** Replace LSP-specific core and import inclusion with the canonical module loader.

**Appendix A.5(2)** Support:

```text
core
stdlib
project modules
platform modules
target variants
generated sources
open-document overlays
```

**Appendix A.5(3)** Remove permanent hardcoded stdlib module switches when canonical discovery is
available.

---

### Appendix A.6 Structured parser diagnostics

**Appendix A.6(1)** Replace parser error strings with structured diagnostics.

**Appendix A.6(2)** Add:

```text
stable IDs
ranges
notes
help
fixes
recovery nodes
```

**Appendix A.6(3)** Do not parse line and column from human-readable error text.

---

### Appendix A.7 Error-tolerant syntax tree

**Appendix A.7(1)** Implement recovery nodes and continue analysis after local recoverable errors.

**Appendix A.7(2)** First required recovery:

```sec
fn myfunc(a string) void {
}
```

**Appendix A.7(3)** Recovered as a parameter with a missing colon.

**Appendix A.7(4)** Provide a machine-applicable insertion fix.

---

### Appendix A.8 Shared formatter

**Appendix A.8(1)** Move `formatSource` and supporting logic from `cmd/lsp` to:

```text
internal/formatter
```

**Appendix A.8(2)** Use it from:

```text
sec fmt
LSP document formatting
LSP range formatting
LSP on-type formatting
refactoring output
```

**Appendix A.8(3)** Preserve all current formatter tests.

**Appendix A.8(4)** The formatter-rulebook filename migration to `rules/tooling/formatter.md` is complete.

---

### Appendix A.9 Shared fix engine

**Appendix A.9(1)** Create:

```text
internal/fixes
```

**Appendix A.9(2)** Consume compiler diagnostics containing structured edits.

**Appendix A.9(3)** Support:

```text
single fix
fix all in document
fix all in workspace
safe fixes on save
sec fmt --fix
```

**Appendix A.9(4)** Do not duplicate fix logic in LSP handlers.

---

### Appendix A.10 Document snapshots

**Appendix A.10(1)** Implement:

```text
versioned documents
incremental edits
immutable snapshots
stale result rejection
didClose
didSave
```

**Appendix A.10(2)** Protect all document access through the snapshot model.

---

### Appendix A.11 Cancellation and progress

**Appendix A.11(1)** Implement:

```text
$/cancelRequest
workDoneProgress
partial results
```

**Appendix A.11(2)** Every deep workspace analysis must periodically check cancellation.

---

### Appendix A.12 Split features

**Appendix A.12(1)** Move features into `internal/lsp/features`.

**Appendix A.12(2)** Each feature receives:

```text
server context
client capabilities
workspace snapshot
request parameters
```

**Appendix A.12(3)** It must not instantiate compiler frontend phases directly.

---

### Appendix A.13 Complete diagnostics

**Appendix A.13(1)** Add:

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

**Appendix A.13(2)** Retain push diagnostics for clients requiring them.

---

### Appendix A.14 Implement code actions

**Appendix A.14(1)** Initial code actions:

```text
insert missing colon
add missing import
change invalid copy to move
handle Result
remove unused import
merge compatible declarations
move primary type to file
```

**Appendix A.14(2)** Classify every action by safety class.

**Appendix A.14(3)** Current implementation includes the user-invoked `S1007` quick fix for a
proven invalid copy of a named value, including move-only, compiler-known
non-copyable, and generic values without proven copyability. It replaces only
the diagnosed source statement's `:=` or `=` operator with `:<-` or `<-`,
respectively, and states that the source becomes unavailable. It is not applied
automatically.

---

### Appendix A.15 Implement navigation

**Appendix A.15(1)** Initial `textDocument/definition` support is implemented through Sema-owned
use-to-declaration bindings. It covers local and cross-file declarations,
members, and selected overloads without an LSP-owned name resolver.

**Appendix A.15(2)** Initial `textDocument/documentHighlight` support is also implemented through
those bindings. It finds exact same-document occurrences, keeps shadowed names
separate, and classifies declarations and direct assignment targets as writes.

**Appendix A.15(3)** Initial standard call hierarchy is implemented over the compiler-owned direct
call graph. It covers validated synchronous direct calls and statically resolved
methods in the current source snapshot. Hover also consumes initial
program/task/thread root reachability and same-stack SCC results. Task and
thread spawn relationships retain their execution classification and remain
outside same-stack recursion. Indirect targets, other execution boundaries,
specializations, additional root classes, and complete `CompilationPlan`
identity remain pending.

**Appendix A.15(4)** Remaining order:

```text
declaration
type definition
implementation
references
rename
```

**Appendix A.15(5)** Build a shared workspace symbol index.

---

### Appendix A.16 Implement signature help and inlay hints

**Appendix A.16(1)** Use Sema-resolved overloads.

**Appendix A.16(2)** Prioritize ownership and parameter hints.

---

### Appendix A.17 Implement refactoring engine

**Appendix A.17(1)** Create:

```text
internal/refactor
```

**Appendix A.17(2)** Initial refactorings:

```text
rename
move type to file
split file by primary types
merge declarations
extract function
change parameter to ref
```

**Appendix A.17(3)** All multi-file actions require preview metadata.

---

### Appendix A.18 Implement compiler-analysis views

**Appendix A.18(1)** Add code lenses and Sec-specific endpoints incrementally as compiler analyses
become available.

**Appendix A.18(2)** Do not create weaker duplicate analyses inside LSP.

---

### Appendix A.19 Units integration

**Appendix A.19(1)** Replace hardcoded unit presentation with compiler unit metadata.

**Appendix A.19(2)** Implement auto-import:

```sec
import "units"
```

**Appendix A.19(3)** Do not make unimported units silently visible.

---

### Appendix A.20 Target integration

**Appendix A.20(1)** Load active target and output variant from the canonical project model.

**Appendix A.20(2)** Add target switching and per-target diagnostics.

---

### Appendix A.21 VS Code settings

**Appendix A.21(1)** Add separate settings for:

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

**Appendix A.21(2)** Keep client logic thin.

---

### Appendix A.22 Knowledge Pack reservation

**Appendix A.22(1)** Add only stable extension points and capability names.

**Appendix A.22(2)** Do not implement a complete Knowledge Pack system for Sec 0.1.

**Appendix A.22(3)** Mark Knowledge Packs as future in status documents.

---

### Appendix A.23 Status updates

**Appendix A.23(1)** Update `language-rulebook-status.md`:

```text
lsp.md
    Written

knowledge_packs.md
    Future or deferred candidate
```

**Appendix A.23(2)** Remove `lsp.md` from Candidate.

**Appendix A.23(3)** Add it to the canonical written set.

**Appendix A.23(4)** Record the current implemented, partial, and missing LSP features in
`governance/tooling_lsp.yaml`.

---

### Appendix A.24 Recommended implementation order

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

## § 36 Design summary

### § 36.1 Enum domains and conversions

**§ 36.1(1)** Enum hover should identify ordinary enums as closed over their declared numeric
value classes and bit-backed enums as open over every value representable by
`bit[N]`. For hardware enums it should make clear that unnamed patterns may
occur at runtime.

**§ 36.1(2)** When declared members do not exhaust an open bit-backed enum, match diagnostics
should explain the uncovered hardware encodings and may suggest a final `_`
fallback. Checked runtime integer-to-enum conversion hover and signatures show
fallibility and the `EnumValueError` family so that the reason for `try` is
visible.

---

**§ 36.1(3)** The Sec language server is an interactive view of the real compiler.

**§ 36.1(4)** It does not own a parallel frontend or diagnostic system.

**§ 36.1(5)** It uses the same formatter as `sec fmt` and the same fix engine as
`sec fmt --fix`.

**§ 36.1(6)** It targets complete LSP 3.18 support.

**§ 36.1(7)** It remains useful on incomplete source through structured parser recovery.

**§ 36.1(8)** It exposes ownership, contracts, units, targets, effects, stack, allocation,
escape, ISR, concurrency, and lowering knowledge whenever the compiler provides
that knowledge.

**§ 36.1(9)** It treats refactoring as a first-class capability.

**§ 36.1(10)** It supports code-quality analysis such as declaration merging and one primary
type per file without confusing those improvements with formatting.

**§ 36.1(11)** It explains consequences before applying ownership-changing or multi-file edits.

**§ 36.1(12)** Knowledge Packs are reserved as a future enrichment system and are not required
for Sec 0.1.

### § 36.2 Ownership revision 2 integration

**§ 36.2(1)** The LSP consumes the same canonical ownership facts as Sema; it does not build a
parallel ownership state machine. Hover, diagnostics, semantic tokens, and code
actions may expose place availability (including partial and conditional
states), unavailability reason and source location, copy-versus-consume,
required `<-` markers, capture mode, and refinement from `is available` / `is
not available`.

**§ 36.2(2)** When compiler facts make a repair unambiguous, the LSP may suggest forms such as
`Consume(<-resource)`, `capture(<-resource)`, an appropriate borrow capture, or
`discard place` for explicit convergence. It must also explain when selected
target policy forbids required runtime ownership bookkeeping. These actions use
programmer-facing mentor language and are offered only when Sema proves their
ownership consequence.

### § 36.3 Source-test discovery and execution integration

**§ 36.3(1)** The compiler workspace is the sole authority for test declaration discovery,
stable identity, source range, category, and test-compilation diagnostics. The
LSP supports an explicit `TestCompilationPlan` view so same-directory
`*_test.sec`, project `tests/` modules, and compiler-known `testing.*` resolve
without contaminating production analysis.

**§ 36.3(2)** The server exposes compiler-owned runnable metadata for Run Test, Debug Test,
tests in a file/module, and project tests where the client supports those
actions. Editor extensions may map this metadata to a native testing UI but
must not parse Sec independently or invent an editor-specific runner. Every
editor invocation uses the same discovery, selection, plan, harness, cleanup,
and execution-provider semantics as `sec test` in
`rules/tooling/testing.md`.

### § 36.4 Compile-time evaluation integration

**§ 36.4(1)** The LSP consumes compiler-owned CTE values, provenance, and diagnostics and
must not implement a separate evaluator. Diagnostics distinguish
`NotEvaluable`, `EvaluationFailed`, `ResourceLimit`, `DependencyCycle`,
`ResultNotMaterializable`, `ImplementationGap`,
`CompilerInvariantFailure`, and `Cancelled`, while retaining the owning
language error for `SemanticOperationFailed`. Cancellation of obsolete editor
work is not a source diagnostic.

**§ 36.4(2)** The LSP consumes the same compiler-known registry as Sema. Hover/completion
shows `value.SizeOf` and `TypeName.SizeOf` as authoritative read-only
`uint` properties and never advertises global `SizeOf(TypeName)`.
`ToString() Result[string, StringError]` is shown as a compiler-provided fallback unless an
eligible exact user replacement resolves first. Tooling may identify fallback,
authoritative, privileged-core-backed, and ordinary replacement categories, but
must not infer compiler authority or source visibility from underscore spelling.

### § 36.5 Channel v2 integration

**§ 36.5(1)** The LSP consumes the exact compiler-known ordinary-channel identities from
`rules/concurrency/channels.md`. Hover exposes the canonical declarations and
variant payloads for `Channel[T]`, its endpoints, options, statistics, tickets,
send/receive results, revocation results, and message dispositions.

**§ 36.5(2)** Completion uses `Tx` and `Rx`, offers `Revoke()` on `MessageTicket[T]` rather
than `Sender[T]`, distinguishes blocking and non-blocking result families, and
does not expose the removed `Priority` capability. `Statistics()` is offered
only when the resolved channel capability facts permit it, or is annotated with
that capability requirement. Navigation treats compiler-known/source-visible
channel identities as the same semantic symbols consumed by Sema.

### § 36.6 Thread v2 integration

**§ 36.6(1)** Hover, completion, and navigation consume the exact CamelCase portable thread
surface from `rules/concurrency/threads.md`. Tooling exposes
`ThreadSetting[T]`, `ThreadPriority`, `ThreadStartMode`, `ThreadStatus`,
`ThreadTerminationKind`, `ThreadTermination`, `ThreadSpawnError`,
`ThreadStartError`, `ThreadConfig`, non-generic `ThreadStorage`, and `CpuSet`
with their exact declarations and ownership constraints.

**§ 36.6(2)** Completion does not offer lowercase legacy thread members, portable
`SetPriority`/`SetAffinity`, `ThreadSchedulingError`,
`ThreadTerminationError`, or `ThreadObserver.Wait`/observer select. Navigation
from `ThreadPlatform` resolves to the selected target's concrete declaration.
Hover distinguishes configuration preference from requirement, reports
explicit-storage borrow lifetime, and shows terminal `Value`, `Panic`, or
`Termination` availability only for the matching joined status.

### § 36.7 Scheduling v2 integration

**§ 36.7(1)** Hover for `Task.Yield()` identifies a logical task scheduling point, a
current-task cancellation point, and the absence of a synchronization edge.
Hover for `Thread.Yield()` identifies a physical scheduler hint that is neither
cancellation-aware nor synchronizing. Completion and diagnostics use the real
execution context: ordinary `main` has attached `ThreadContext` but no implicit
`TaskContext`. Tooling does not expose portable `ThreadSchedulingError`,
`Thread.SetPriority`, or `Thread.SetAffinity` surfaces.

### § 36.8 Thread-local v2 integration

**§ 36.8(1)** Hover, completion, and navigation consume the exact identities from
`rules/concurrency/thread_local.md`. `ThreadLocal[T]` exposes both constructors
and exactly `Borrow`, `BorrowMut`, and `Replace`; tooling does not suggest the
removed `Value`, `value`, `Take`, `Get`, or `Set` surfaces.

**§ 36.8(2)** Tooling exposes `ThreadAttachError` with exactly `AlreadyAttached`,
`ResourceUnavailable`, and `NativeFailure`, and exposes the owned
`ThreadAttachment` with its `Context` property and lifecycle-destruction
semantics. Navigation from `ThreadAttachment.Context` resolves to canonical
`ThreadContext`. Diagnostics retain the TLS key, borrow creation, and invalid
suspension/escape locations where available.

### § 36.9 Debug-information source-coordinate integration

**§ 36.9(1)** LSP and debug information consume compatible canonical source-file and range
identities. Protocol UTF-16 conversion remains an LSP transport concern;
`rules/compiler/debug_information.md` consumes compiler source coordinates and
lowers them to the selected physical debug format. A future debugger plugin may
reuse compiler/LSP naming and type-display services without requiring `Full` to
embed an LSP database.

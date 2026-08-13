# Parser Recovery

## Status

This document is the canonical parser-recovery rulebook for Sec.

It defines:

- parser behavior after invalid or incomplete source;
- recovery invariants;
- recovery diagnostics;
- virtual missing tokens;
- invalid AST nodes;
- synchronization boundaries;
- context-specific recovery;
- interaction with Sema, formatter, and LSP;
- current implementation status;
- implementation requirements and tests.

This document does not redefine the canonical grammar.

Canonical syntax is defined by:

```text
grammar.md
lexical_structure.md
operators.md
```

Specialized rulebooks define semantic validity after parsing.

Repository baseline reviewed:

```text
branch: main
review date: 2026-08-07
```

The repository may contain small unpublished changes after this review.

Such changes may update an implementation-status item, but they do not override
the normative recovery rules in this document.

---

# Purpose

The Sec parser must remain useful when source code is:

```text
incomplete
temporarily malformed during editing
written with one local syntax mistake
using reserved syntax
using compatibility syntax
truncated at end of file
damaged inside one declaration or block
```

A syntax error should not normally prevent the parser from finding later valid
declarations and statements.

Recovery exists to:

- report the primary syntax problem;
- preserve as much source structure as possible;
- avoid cascading diagnostics;
- allow later code to be parsed;
- support LSP features on incomplete files;
- support formatter and refactoring tools without inventing semantics.

Recovery does not make invalid source valid.

---

# Core recovery principle

> Recovery may preserve incomplete syntax, but it must never silently reinterpret
> invalid source as a different valid program.

Example:

```sec
fn Add(left int, right: int) int {
    return left + right
}
```

The parser may identify that `:` is missing after `left`.

It may create a virtual missing colon and continue parsing the parameter.

It must not reinterpret:

```sec
left int
```

as another declaration form.

---

# Compiler result

The parser should return a parse result even when syntax errors exist.

Conceptual result:

```go
type ParseResult struct {
    Program     *ast.Program
    Diagnostics []Diagnostic
    HasErrors   bool
    Fatal       bool
}
```

Exact Go names may differ.

A normal batch compilation must not proceed to code generation when parser
errors exist.

Tooling may continue with partial semantic analysis of valid subtrees.

---

# Terminology

## Syntax error

A syntax error is source that cannot be parsed as canonical or accepted
compatibility syntax.

Examples:

```text
missing delimiter
unexpected delimiter
missing operand
missing field colon
missing match arrow
unterminated block
invalid declaration member
misplaced keyword
assignment in expression position
```

## Recovery

Recovery is the parser's controlled continuation after a syntax error.

## Repair

A repair is the parser's internal model of the smallest assumed correction
needed to continue.

A repair may be:

```text
insert one missing structural token
skip one unexpected token
skip a malformed region
close a truncated construct at end of file
```

The parser does not modify the user's source.

## Virtual token

A virtual token represents a required token missing from source.

Example:

```text
MissingToken(":")
```

A virtual token has a zero-width source position.

## Synchronization point

A synchronization point is a token or grammar boundary where parsing can safely
resume.

Examples:

```text
next top-level declaration
next statement at the current block depth
comma in a list
closing delimiter
next switch case
next impl member
```

## Invalid node

An invalid node is an AST node preserving malformed source and partial children.

It is not a valid semantic construct.

## Cascading diagnostic

A cascading diagnostic is a secondary error caused only by an earlier syntax
error and poor recovery.

## Fatal parser failure

A fatal parser failure means the parser cannot safely continue.

Ordinary source syntax errors are not fatal.

---

# Recovery goals

The parser should:

1. always make progress;
2. preserve source order;
3. preserve surrounding valid syntax;
4. emit one primary diagnostic for one local cause;
5. avoid crossing enclosing grammar boundaries;
6. avoid fabricating semantic values;
7. preserve explicit copy and move spelling;
8. preserve comments and skipped source ranges;
9. return stable partial nodes for tooling;
10. recover deterministically;
11. behave identically in batch and tooling parsing;
12. never panic because source is invalid.

---

# Non-goals

Sec 0.1 recovery does not require:

```text
global minimum-edit repair
probabilistic intent inference
automatic rewriting of source
semantic type guessing
overload-based grammar repair
macro expansion recovery
full incremental parsing
perfect formatting of arbitrary broken source
```

A later parser may use a more advanced repair algorithm.

The observable invariants in this document must remain unchanged.

---

# Recovery invariants

## Progress

Every parser loop must either:

- consume at least one real token;
- insert one bounded virtual token and change parser state;
- return to its caller;
- stop at end of file.

No recovery path may repeatedly inspect the same token in the same state.

## Enclosing-boundary safety

Recovery inside a construct must not consume the closing delimiter belonging to
an outer construct.

Example:

```sec
fn Test() void {
    let value := Call(
        first,
        second

    return
}
```

Argument-list recovery may stop before `return` or the function `}`.

It must not consume the complete function body looking for `)`.

## No silent reinterpretation

A repair must not change the likely grammatical category of source without a
diagnostic.

Invalid:

```sec
while running = true {
}
```

The parser may preserve `running` as a partial condition for tooling.

It must emit an error for assignment in condition position.

It must not compile the source as though the user wrote:

```sec
while running {
}
```

## No semantic repair

The parser must not decide:

```text
which type was intended
which variable was intended
which overload was intended
which operator was intended
whether a value should be copied or moved
whether a contract should succeed
```

## Determinism

Given the same token stream and parser version, recovery must produce:

```text
the same diagnostics
the same invalid nodes
the same virtual tokens
the same synchronization result
```

## Bounded damage

One malformed member should not normally invalidate later siblings.

Examples:

```text
one struct field
one enum value
one function parameter
one argument
one match arm
one switch case
one impl member
one top-level declaration
```

## Error blocks code generation

A recovered AST is not proof of a valid program.

Any parser diagnostic with error severity blocks ordinary lowering and code
generation.

---

# Current implementation status

## Implemented

The current parser already implements several important recovery foundations.

### Parser continues after errors

`ParseProgram` continues until EOF.

When a parse function returns `nil`, the parser calls `skipStatement()` and
attempts to resume at a statement-start token or a closing brace.

Statement blocks use the same general pattern.

### Error and warning collection

The parser currently collects:

```go
errors   []string
warnings []string
```

and exposes:

```go
Errors() []string
Warnings() []string
```

The canonical migration API now returns:

```go
type ParseResult struct {
    Program     *ast.Program
    Diagnostics []Diagnostic
    Warnings    []string
    Recovery    []RecoveryEvent
    HasErrors   bool
    Fatal       bool
}
```

`ParseProgram`, `Errors`, `Warnings`, and `Diagnostics` remain compatibility
APIs while compiler and tooling consumers migrate. Central CLI and LSP parse
paths consume `ParseResult`.

### Speculative parsing rollback

Typed-declaration lookahead snapshots:

```text
lexer state
current token
lookahead token
error count
warning count
```

and restores them after speculative parsing.

This prevents diagnostics from a rejected speculative branch from escaping.

### Existing invalid statement node

The AST currently contains:

```go
type InvalidStatement struct {
    Token   lexer.Token
    Message string
}
```

It is used for selected invalid or reserved statement forms.

Failed top-level statements and failed statements in ordinary function/block
bodies are also retained as `InvalidStatement` nodes with diagnostic ID,
message, start/end tokens, and skipped-token count.

Examples include:

```text
unsupported `free`
unexpected `else`
```

Malformed declaration-shaped top-level input is retained separately as
`InvalidDeclaration`. Disallowed or malformed `impl` entries are retained as
`InvalidMember`, including their recovery range, while their existing semantic
diagnostic phase remains unchanged.

### Initial invalid expression and type retention

The AST now contains `InvalidExpression` with recovery metadata. Pratt-parser
failures without a prefix parser retain this node instead of returning only
`nil`.

`TypeReference` can be marked invalid and carry the same recovery metadata.
This is wired for parenthesized, reference, function and prefix sequence type
paths. Remaining specialized type parsers still require migration.

Sema maps these invalid nodes directly to its invalid/error type and does not
invent dependent semantics.

### Initial recovery event stream

The parser records structured recovery events for:

```text
unambiguous virtual missing-token insertion
statement-level skipped token ranges
malformed impl-member skipped ranges
malformed match-arm and try-handler skipped ranges
malformed struct-field, switch-clause, and select-branch skipped ranges
compiler-directive, generic-parameter, declaration-rest, malformed for-header,
attribute-argument, property-remainder, and balanced-block skipped ranges
```

Diagnostics and their associated events carry a stable recovery context and
episode number. Integrated contexts distinguish top-level, ordinary block,
declaration-member, match-arm, and try-handler recovery.

Speculative parser rollback also removes repair events associated with rolled
back diagnostics and reconstructs deduplication and episode numbering state.

### Diagnostic limit

The parser emits at most 100 diagnostics for one parse. When the cap is reached,
the final diagnostic is the stable `P2015` recovery-limit diagnostic and later
parser diagnostics are suppressed. Speculative rollback restores the limit
state when the speculative diagnostics are discarded.

### Brace-depth skipping

The parser has brace-aware helpers including:

```text
skipCurrentBlock
skipBraceBlock
skipPropertyRemainder
```

These avoid stopping at the first nested closing brace.

### Target directive recovery

Malformed `#target(...)` syntax can skip to the closing parenthesis.

### Generic parameter recovery

Malformed generic parameter lists can skip toward:

```text
]
{
(
EOF
```

### Malformed loop-header recovery

A malformed `for` header can skip to its body brace and still parse the body.

The current tests verify recovery after:

```text
condition-only for syntax
C-style for syntax
missing iterable syntax
```

### Switch recovery

Malformed switch clauses can synchronize at:

```text
case
default
}
EOF
```

The parser can continue to later cases.

It also gives a focused diagnostic when `||` is used where comma-separated case
items were likely intended.

### Select recovery

Malformed select branches can synchronize toward:

```text
after
default
probable identifier branch
await branch
}
EOF
```

### Struct field recovery

Malformed struct declaration fields can synchronize at:

```text
,
}
EOF
```

The current tests verify that:

- a missing field colon produces one error;
- a later comma-separated field is retained;
- later top-level declarations are retained.

### Impl-member recovery

Malformed impl members use:

```text
brace depth
source line change
known impl member starts
closing impl brace
```

to locate a later member.

### Try-handler closing-brace recovery

The parser detects selected statement starts after a try handler block and can
report a missing `}` while preserving the following statement.

The current tests verify that a following `return` remains in the function body.

### Unexpected else recovery

An `else` without a matching `if` produces an invalid statement.

When followed by a block, the parser consumes that block so its contents do not
become unrelated outer statements.

### Unterminated block diagnostics

Several parser paths detect EOF before a required closing brace and report an
unterminated construct.

---

# Partly implemented

## Structured diagnostics are initially implemented

The parser now retains compatibility string errors and parallel structured
diagnostics carrying:

```text
diagnostic ID
message
primary token
expected tokens
unexpected token
```

Speculative rollback removes both representations transactionally.

Initial focused paths use `P2002`, `P2003`, `P2007`, `P2008`, `P2011` and
`P2013`. Most parser call sites still use fallback `P2001` and do not yet
provide canonical source ranges, related locations, recovery events or fixes.

A parser error is generally only:

```text
formatted message
embedded line and column
```

The diagnostics registry contains the general parser ID:

```text
P2001 parser.syntax-error
```

but individual parser errors are not emitted as structured diagnostics with
stable IDs, ranges, expected tokens, related locations, or fixes.

## AST recovery nodes remain incomplete

The parser now retains `InvalidStatement`, `InvalidDeclaration`,
`InvalidExpression`, `InvalidPattern`, `InvalidMember`, and invalid
`TypeReference` nodes with recovery metadata on integrated paths.

Canonical AST representation is still absent for:

```text
missing token
skipped token range
```

Many recoverable parse functions therefore return `nil`.

Returning `nil` loses local syntax structure and forces broader skipping.

## General synchronization set is incomplete

The current `isStatementStart` helper omits several valid statement or
declaration starts.

Examples include some of:

```text
#
unit
extern
try
discard
cancel
else
fallthrough
spawn
await
@
ref
self
[
comments
```

Some forms are reached through identifiers or type lookahead and cannot be
represented by a simple token-only set.

The current general `skipStatement()` can therefore skip too far.

## Match-arm recovery is partial

Malformed match arms are retained with `InvalidPattern`, and recovery stops
before a likely pattern on a later line or the closing brace. Parenthesis,
bracket, and brace depth is respected locally. Grammar-derived synchronization
and same-line sibling recovery remain pending.

## Try-handler recovery is partial

Malformed try handlers are retained with `InvalidPattern`, and the same local,
delimiter-aware sibling synchronization preserves likely later handlers.

The special missing-closing-brace detection recognizes only a subset of
possible following statement starts.

## Missing tokens are not represented

The parser can diagnose a missing token but does not preserve a virtual token in
the AST.

Tooling cannot reliably distinguish:

```text
token present in source
token assumed by recovery
token absent and node abandoned
```

## Skipped tokens are only partly preserved

General statement recovery and the migrated delimiter-aware helpers record the
skipped token range. These currently include impl members, match/try siblings,
struct fields, switch/select entries, compiler directives, generic parameters,
declaration tails, malformed for headers, attribute arguments, property
remainders, and balanced blocks. Other specialized helpers still advance
without recording it.

Formatter, LSP, and diagnostics cannot always reconstruct the malformed region
from AST data alone.

## Source spans are incomplete

Lexer tokens currently contain:

```text
file
line
column
lexeme
token type
```

They do not carry canonical start and end offsets.

Accurate zero-width insertion ranges and multi-token diagnostic ranges therefore
need additional source-position support.

## Recovery episodes and cascading-diagnostic suppression are partial

The parser starts a numbered episode at the first diagnostic and ends it at
integrated stable statement, member, arm, handler, delimiter, or skip
boundaries. Diagnostics with identical ID, primary token range, and recovery
context are deduplicated. A second diagnostic at the same primary location and
context within the active episode is suppressed as a same-cause cascade.

Broader causal suppression across different token locations and full context
coverage for every specialized parser remain pending.

## Unterminated constructs often return nil

Several block parsers report an unterminated block and then return `nil`.

This can discard otherwise useful partial block contents.

## Assignment-condition recovery

Assignment in a `while` condition emits structured `P2013` as an error and
blocks batch compilation. The recovered node currently preserves the left
expression while retaining and reporting the unexpected assignment token.

## Comments are not consistently retained

Top-level comments may become AST nodes.

Many block parsers skip comments.

A recovered formatter or documentation tool therefore lacks one consistent
full-fidelity source model.

## Recovery and Sema are not formally separated

There is no canonical rule for:

```text
whether Sema visits an invalid node
what type an invalid expression has
whether an invalid declaration introduces a symbol
how ownership state flows past invalid syntax
which diagnostics are suppressed beneath invalid syntax
```

---

# Not implemented

The following canonical recovery infrastructure is not yet implemented fully:

```text
MissingToken representation
SkippedTokenRange representation
ErrorType propagation
delimiter-stack migration for remaining specialized helpers
shared grammar-derived synchronization sets
cross-location recovery-cascade suppression
full-fidelity malformed-source token retention
safe-fix metadata
recovery-aware formatter behavior
recovery-aware LSP completion context
property-based recovery tests
mutation-based recovery tests
parser progress assertions
```

---

# Parse result

The parser should return one result object.

Conceptual form:

```go
type ParseResult struct {
    Program     *ast.Program
    Diagnostics []diagnostics.Diagnostic
    Recovery    []RecoveryEvent
    HasErrors   bool
    Fatal       bool
}
```

The exact package layout may differ.

The result must distinguish:

```text
valid parse with no diagnostics
valid parse with compatibility warning
recovered parse with errors
fatal parser failure
```

---

# Structured parser diagnostic

Conceptual diagnostic:

```go
type Diagnostic struct {
    ID              string
    Severity        Severity
    Message         string
    PrimaryRange    SourceRange
    Related         []RelatedLocation
    ExpectedTokens  []lexer.TokenType
    UnexpectedToken *lexer.Token
    Fixes           []Fix
}
```

Parser code must not embed the only copy of source location into a message
string.

Human-readable messages may still include token spellings.

---

# Parser diagnostic IDs

Existing:

```text
P2001 parser.syntax-error
```

`P2001` remains the fallback for an unclassified parser error.

Add stable focused parser diagnostics.

Recommended initial allocation:

| ID | Symbolic name | Purpose |
|---|---|---|
| `P2001` | `parser.syntax-error` | fallback parser error |
| `P2002` | `parser.missing-token` | one required token is absent |
| `P2003` | `parser.unexpected-token` | token is invalid in current context |
| `P2004` | `parser.unterminated-delimiter` | `)`, `]`, or `}` missing before EOF or boundary |
| `P2005` | `parser.invalid-declaration` | declaration cannot be completed |
| `P2006` | `parser.invalid-statement` | statement cannot be completed |
| `P2007` | `parser.invalid-expression` | expression cannot be completed |
| `P2008` | `parser.invalid-type-reference` | type reference cannot be completed |
| `P2009` | `parser.invalid-pattern` | match or handler pattern cannot be completed |
| `P2010` | `parser.missing-separator` | comma or other separator is absent |
| `P2011` | `parser.misplaced-keyword` | keyword is valid only in another grammar context |
| `P2012` | `parser.reserved-syntax` | reserved spelling has no implemented form |
| `P2013` | `parser.invalid-assignment-expression` | assignment appears where expression is required |
| `P2014` | `parser.chained-comparison` | comparison chaining is invalid |
| `P2015` | `parser.recovery-limit` | parser stopped issuing additional syntax diagnostics |
| `P2016` | `parser.unexpected-end-of-file` | source ends before required construct completion |
| `P2017` | `parser.invalid-block-member` | declaration or statement is invalid in a body |
| `P2018` | `parser.compatibility-syntax` | accepted noncanonical syntax requires migration |

IDs identify parser rules, not severity.

Compatibility syntax may default to warning or information.

Mandatory syntax violations remain errors.

---

# Recovery event

A recovery event records what the parser did after a diagnostic.

Conceptual form:

```go
type RecoveryEvent struct {
    DiagnosticID string
    Context      RecoveryContext
    Unexpected   lexer.Token
    Inserted     []MissingToken
    Skipped      []lexer.Token
    ResumeToken  lexer.Token
}
```

Recovery events support:

```text
debugging
tests
LSP behavior
formatter behavior
parser telemetry
bootstrap comparison
```

They do not become source-language semantics.

---

# Virtual missing tokens

## Purpose

A missing token allows the parser to preserve a surrounding node without
pretending that the source contained the token.

Example:

```sec
type Point struct {
    x int,
    y: int,
}
```

Recovery may create:

```text
StructField
    name: x
    colon: MissingToken(":")
    type: int
```

## Representation

Conceptual form:

```go
type MissingToken struct {
    Expected lexer.TokenType
    Anchor   SourcePosition
    Before   lexer.Token
}
```

A missing token:

- has zero width;
- records the expected token kind;
- records its insertion anchor;
- is marked synthetic;
- is never returned by the lexer;
- is never printed as though present in source;
- may support an explicit code action.

## Safe insertion categories

The parser may insert one missing structural token when the expected choice is
unique and local.

Typical safe categories:

```text
:
,
)
]
}
=>
in
{
```

The exact insertion is context-dependent.

## Tokens not guessed

Recovery must not invent:

```text
identifier
literal
type name
member name
operator operand
arbitrary operator
ABI string
module path
import path
function body expression
```

When such content is missing, create an invalid node and synchronize.

## No repeated insertion

At one source position, the parser must not repeatedly insert the same token
without consuming input or returning.

---

# Unexpected token handling

## Single-token deletion

The parser may skip one unexpected punctuation token when:

- the surrounding grammar is otherwise unambiguous;
- skipping does not cross a delimiter boundary;
- the token is preserved in recovery metadata;
- one diagnostic is emitted.

Example:

```sec
Call(first,, second)
```

The second comma may be skipped.

## Keyword deletion

The parser should not silently delete a keyword that can begin another
declaration or statement.

It should synchronize before that keyword when possible.

## Token substitution

The parser does not silently substitute one real token for another.

Example:

```sec
Value: 10
```

inside an enum may be accepted as explicit compatibility syntax and diagnosed
accordingly.

That is not ordinary recovery substitution.

---

# Invalid AST nodes

Recoverable malformed source should normally produce an invalid node rather than
`nil`.

## Required node families

Add:

```text
InvalidDeclaration
InvalidStatement
InvalidExpression
InvalidTypeReference
InvalidPattern
InvalidMember
```

`InvalidStatement` already exists and should be extended.

## Common recovery metadata

Conceptual form:

```go
type RecoveryInfo struct {
    DiagnosticID string
    Unexpected   *lexer.Token
    Expected     []lexer.TokenType
    Inserted     []MissingToken
    Skipped      []lexer.Token
    Range        SourceRange
}
```

Each invalid node should retain:

```text
start token
source range
recovery information
successfully parsed children
raw or skipped tokens
```

## Invalid declaration

An invalid declaration may retain:

```text
declaration keyword
name
generic parameters
partial type
partial body
```

## Invalid expression

An invalid expression may retain:

```text
left operand
operator
partial right operand
opening delimiter
parsed arguments
```

## Invalid type reference

An invalid type reference may retain:

```text
base name
generic arguments
array suffixes
reference modifier
unit annotation
```

## Invalid pattern

An invalid pattern may retain:

```text
variant name
partial payload binding
guard
```

## Nil use

A parse function may return `nil` only when:

- its caller has not committed to that grammar alternative;
- speculative parsing is being rolled back;
- a fatal parser invariant prevents node construction.

After a construct has been committed, recoverable errors should produce a
partial or invalid node.

---

# Error type

Sema should assign invalid expressions and invalid type references an internal
error type.

Conceptual:

```text
ErrorType
```

`ErrorType`:

- is not a user-visible Sec type;
- is compatible enough to prevent cascades;
- does not satisfy ordinary contracts;
- cannot reach lowering;
- carries no ownership resource;
- does not authorize unsafe operations;
- does not create a valid overload candidate.

---

# Parser error episodes

A recovery episode begins when the parser emits a primary syntax diagnostic.

It ends when the parser:

- reaches a synchronization point;
- successfully consumes the expected closing delimiter;
- completes the current invalid node;
- returns to a stable caller boundary.

Within one episode, secondary diagnostics caused solely by the same missing
token should be suppressed.

Example:

```sec
fn Test(value int result: int) void {
}
```

The parser should not emit independent errors for every token after the first
missing parameter punctuation if one local repair explains them.

---

# Diagnostic deduplication

Diagnostics with the same:

```text
ID
primary source range
recovery context
```

should not be emitted more than once.

Speculative parse diagnostics must be rolled back entirely.

A diagnostic from an abandoned parser branch must never reach the user.

---

# Diagnostic limit

The parser should cap syntax diagnostics per file.

Recommended default:

```text
100 parser errors
```

After the cap:

- emit `P2015 parser.recovery-limit` once;
- continue only enough to find EOF safely;
- do not emit further parser errors;
- still return the partial program;
- mark the result as containing errors.

The cap is an implementation policy, not source-language semantics.

---

# Fatal conditions

The parser may mark a result fatal only for conditions such as:

```text
lexer fails to advance
corrupt token stream
impossible delimiter-stack state
internal parser invariant violation
resource exhaustion
```

An unterminated user block or malformed expression is not fatal.

---

# Delimiter tracking

Recovery must track nesting for:

```text
(...)
[...]
{...}
```

A context recovery function must ignore synchronization tokens inside deeper
nested delimiters.

Example:

```sec
Call(
    Other(first, second),
    third
)
```

The comma inside `Other(...)` is not a synchronization comma for the outer
argument if recovery is currently inside the nested call.

## Mismatched closer

When the parser expects one closer and finds another:

```sec
Call(value]
```

it should:

1. report the mismatched closer;
2. preserve the real `]`;
3. create a missing `)` only when doing so does not consume an outer boundary;
4. allow the caller to process `]` if it belongs to an outer construct.

---

# Synchronization model

Synchronization sets are grammar-context-specific.

Do not use one global panic-mode set for every parse failure.

Each parser context must define:

```text
hard boundaries
sibling starts
separators
closing delimiters
tokens that must not be consumed
```

---

# Compilation-unit synchronization

## Hard boundaries

At top level, recover toward:

```text
#target start
module
import
type
unit
enum
interface
extern
fn
struct
impl
let
static
unsafe
EOF
```

Contextual top-level declarations may require predicate-based detection.

## Rule

A malformed top-level declaration must not consume the next probable top-level
declaration at brace depth zero.

## Current implementation gap

The current `isStatementStart` list is incomplete and should be replaced by
grammar-derived declaration and statement predicates.

---

# General statement synchronization

Within a block, synchronize toward:

```text
}
let
typed declaration start
return
if
for
while
switch
match
select
break
continue
fallthrough
defer
discard
cancel
unsafe
asm
try
spawn
await
@
ref
self
identifier expression start
array expression or typed declaration start
comment
EOF
```

Because identifiers can begin either expressions or typed declarations,
statement synchronization cannot be only a static token set.

Use:

```text
token kind
line position
delimiter depth
typed-declaration lookahead
current grammar context
```

## Same-block requirement

A synchronization candidate must be at the current block's delimiter depth.

---

# Block recovery

When a block is missing `}`:

- preserve parsed statements;
- create a virtual missing `}`;
- emit an unterminated-block diagnostic;
- stop before a token known to start an enclosing sibling when such a boundary
  is reliable;
- stop at EOF otherwise.

Do not discard the partial block by returning `nil`.

## Function boundary heuristic

At module scope, a probable top-level declaration at brace depth zero may close
a truncated function block virtually.

This heuristic is permitted only when the parser can prove it is no longer
inside a nested delimiter.

---

# Declaration-header recovery

Declaration headers include:

```text
type
function
interface
impl
enum
union
register
unit
static
extern
```

Recover toward:

```text
opening body brace
end of declaration line where body is not required
next top-level declaration
closing enclosing brace
EOF
```

A malformed header should produce an invalid declaration preserving:

```text
keyword
name when parsed
generic parameters when parsed
partial type information
```

---

# Import recovery

## Single import

Recover toward:

```text
string path
next top-level declaration
EOF
```

Do not invent an import path.

## Import group

Inside `import (...)`, synchronize at:

```text
next string literal
probable alias followed by string literal
)
EOF
```

An invalid import item should not discard later valid import items.

A missing `)` may be inserted before the next top-level declaration at depth
zero.

---

# Compiler-directive recovery

Inside `#target(...)`, synchronize at:

```text
,
)
next top-level declaration
EOF
```

Unknown argument names remain part of one invalid directive node.

Do not interpret malformed directive tokens as ordinary code inside the
directive.

---

# Generic parameter recovery

Inside generic parameter declarations, synchronize at:

```text
,
]
(
{
implements
type contract start
default
EOF
```

A malformed generic parameter should preserve later comma-separated parameters
when possible.

The current helper stops at `]`, `{`, or `(` and should be extended.

---

# Generic argument recovery

Inside generic or collection type arguments, synchronize at:

```text
,
]
)
{
EOF
```

The parser must retain whether an argument position expects:

```text
type argument
constant argument
either, pending Sema
```

Do not silently convert an invalid constant argument into a type argument.

---

# Parameter-list recovery

Inside a function or lambda parameter list, synchronize at:

```text
,
)
return type start
{
EOF
```

## Missing colon

Example:

```sec
fn Add(left int, right: int) int {
}
```

When an identifier is followed by a valid type start, insert a virtual `:` and
continue.

Emit one missing-token diagnostic.

## Missing parameter name

Example:

```sec
fn Add(: int) int {
}
```

Do not invent a name.

Create an invalid parameter node and synchronize at comma or `)`.

## Missing type

Example:

```sec
fn Add(left:, right: int) int {
}
```

Do not invent a type.

Create an invalid type reference and synchronize at comma or `)`.

## Missing comma

When the next tokens clearly begin another `Identifier ":"` parameter, insert a
virtual comma.

---

# Argument-list recovery

Inside a call, synchronize at:

```text
,
)
]
}
=>
EOF
```

A missing argument expression creates an invalid expression.

Example:

```sec
Call(first, , third)
```

The parser should preserve argument positions:

```text
first
InvalidExpression
third
```

This is useful for parameter completion.

---

# Array-literal recovery

Inside an array literal, synchronize at:

```text
,
]
}
EOF
```

Preserve later elements after one malformed element.

A missing comma may be inserted only when the next token clearly begins another
element at the same bracket depth.

---

# Type-reference recovery

Type-reference recovery must account for:

```text
qualified names
generic arguments
constant shape arguments
unit annotations
array suffixes
reference modifiers
function types
parenthesized types
```

Synchronize at grammar boundaries such as:

```text
,
)
]
{
}
:
:=
<-
implements
contract start
default
EOF
```

Do not consume an assignment initializer while searching for a missing type
delimiter.

---

# Type-contract recovery

## Range contract

Synchronize at:

```text
..
..<
next contract start
default
opening type body
next declaration
EOF
```

At least one bound is required.

Do not invent a numeric bound.

## `in [...]`

Synchronize individual values at:

```text
,
]
next contract start
default
EOF
```

Preserve later values.

## Value contract

For contracts such as `multipleOf`, a missing value creates an invalid
expression attached to the contract.

---

# Default-clause recovery

After `default`, an expression is required.

If absent:

- create an invalid default expression;
- retain the type declaration;
- synchronize at type-body start, next top-level declaration, closing enclosing
  brace, or EOF.

Do not substitute the implicit type default.

The declaration remains invalid until corrected.

---

# Struct declaration recovery

Inside a struct body, synchronize at:

```text
,
}
next probable field start on a later line
EOF
```

## Missing field colon

Example:

```sec
type Point struct {
    x int,
    y: int,
}
```

When `int` is a valid type start, create a virtual `:`.

Preserve the field as a recovered field rather than dropping it.

The current parser reports the error and may skip the malformed field.

The canonical model retains it as a recovered field node.

## Missing comma

Declared struct fields require commas.

When the next line clearly begins `Identifier ":"`, insert a virtual comma and
continue.

## Invalid field type

Create an invalid type-reference field and synchronize at comma or `}`.

## Invalid tag

Preserve:

```text
field name
field type
raw tag token
```

Attach a tag diagnostic.

Do not discard the whole struct.

## Unterminated struct

Preserve parsed fields and insert a virtual `}` at a safe outer boundary or EOF.

---

# Struct literal recovery

Inside a struct literal, synchronize at:

```text
,
line-start next field
}
EOF
```

Struct literals permit newline-separated fields according to `grammar.md`.

## Missing colon

When the left expression is a simple identifier and the following token starts
an expression, a missing `:` may be inserted.

## Invalid item

Preserve an invalid struct-literal item.

Do not discard later fields or spreads.

## Missing closing brace

Preserve parsed items and create a virtual `}` at a reliable enclosing boundary.

---

# Enum recovery

Inside an enum body, synchronize at:

```text
,
line-start next identifier
}
EOF
```

## Missing value name

Create an invalid enum member and continue to the next separator.

## Invalid initializer

Preserve the member name and attach an invalid expression.

## Colon compatibility

`Value: expression` may be accepted as compatibility syntax.

It must produce the canonical compatibility diagnostic and formatter fix:

```sec
Value = expression
```

It is not ordinary missing-token recovery.

## Unterminated enum

Preserve parsed values and create a virtual `}` at a safe boundary.

---

# Union recovery

Inside a union body, synchronize at:

```text
,
line-start next variant name
}
EOF
```

Preserve:

```text
variant name
payload opening delimiter
partial payload type or fields
```

A malformed payload must not discard later variants.

---

# Reversed type declaration order

The parser recognizes declaration-kind-before-name mistakes such as:

```sec
type struct User { ... }
type union State { ... }
type register Status[8] { ... }
```

These forms remain syntax errors. The parser emits `P2011` and explains the
canonical `type Name kind` order. Because the intended ordering is unique,
fix-enabled formatting may rewrite them while ordinary formatting preserves
the invalid source around the diagnostic.

---

# Register recovery

Inside a register body, synchronize at:

```text
,
line-start next field or `_`
}
EOF
```

Preserve field width syntax where parsed.

Width-total validation remains Sema's responsibility.

---

# Interface recovery

Inside an interface body, synchronize at:

```text
fn
property
contextual event
}
EOF
```

An invalid member becomes `InvalidMember`.

It must not cause the complete interface declaration to return `nil`.

Property requirements synchronize at:

```text
get
set
}
```

---

# Impl recovery

Inside an impl body, synchronize at:

```text
type
unit
enum
fn
free
property
contextual event
static
}
EOF
```

The current implementation's line- and depth-aware recovery is a useful base.

Expand it to:

- preserve invalid members;
- include every canonical impl-member start;
- avoid relying only on line change;
- retain skipped tokens;
- support nested delimiters.

An invalid local `let` inside impl should remain an invalid member with a
structured diagnostic.

---

# Property recovery

Inside a property body, synchronize at:

```text
get
set
try set
}
EOF
```

A malformed getter must not discard a later setter.

A malformed setter must not discard a later getter.

## Missing setter parameter

Do not invent the parameter name.

Create an invalid setter node, consume or preserve its body, and continue at the
next accessor or property `}`.

## Duplicate accessor

Duplicate accessors are semantic or structural property errors.

Preserve both accessors so tooling can show both source ranges.

---

# Event recovery

Event declarations synchronize at:

```text
using
next impl member
}
EOF
```

A missing storage name must not consume the next impl member.

Do not silently skip an event body unless event-body syntax becomes canonical.

---

# Function recovery

Function declaration recovery must preserve as much as possible:

```text
name
generic parameters
parameters
return type
body
```

## Missing function name

Create an invalid declaration.

Synchronize at generic parameters, `(`, return type, body, or next declaration.

Do not create a public anonymous named function.

## Missing return type

The return type is required.

Create an invalid type reference before the body.

Do not infer `void`.

## Missing body

For an ordinary function, create a missing-body diagnostic.

Do not consume the next top-level declaration as the body.

Interface and extern declarations follow their specialized rules.

---

# Extern recovery

Recover around:

```text
ABI string
fn
name
parameter list
return type
optional canonical body form
next declaration
EOF
```

Do not invent an ABI string.

Preserve enough information for FFI diagnostics.

---

# Let-declaration recovery

Synchronize each declarator at:

```text
,
next statement start at same depth
}
EOF
```

Preserve:

```text
let token
mutability
name
type
copy or move initializer operator
partial initializer
```

## Missing name

Do not invent an identifier.

Create an invalid declarator and continue at comma or statement boundary.

## Missing type after colon

Create an invalid type reference.

Do not reinterpret the following initializer as a type.

## Missing initializer for immutable binding

This is a language diagnostic after the declaration shape is parsed.

Keep the declaration node.

## Copy versus move

Recovery must never replace:

```text
:=
:<-
=
<-
```

with another ownership operator silently.

---

# Type-first declaration recovery

Type-first declaration recovery must distinguish:

```text
type reference
mut
colon
declarator list
```

Speculative parsing must remain transactional.

When the type-first interpretation fails and expression parsing is viable, the
parser may roll back without diagnostics.

After committing at `mut` or `:`, errors produce an invalid declaration rather
than rolling back to expression parsing.

---

# Assignment recovery

Preserve:

```text
target
operator spelling
right expression
```

## Missing right expression

Create `InvalidExpression` as the source.

Do not drop the assignment target.

## Assignment in expression position

Emit:

```text
P2013 parser.invalid-assignment-expression
```

Do not reinterpret it as only the left expression for compilation.

The AST may retain an invalid assignment-expression node for tooling.

## Chained assignment

Preserve the first assignment statement and attach an invalid right expression
or dedicated invalid chain node.

Do not assign associativity.

---

# If recovery

## Missing condition

Create an invalid condition expression.

Preserve and parse the consequence block.

The current parser preserves an `IfStatement` with a nil condition.

Canonical AST should use `InvalidExpression` instead of nil.

## Missing consequence brace

When a block is clearly intended, create a virtual `{` only when a matching
body boundary can be established safely.

Otherwise preserve an invalid if statement and synchronize at:

```text
else
next statement
}
EOF
```

## Else recovery

An unmatched `else` remains an invalid statement.

Its body should be consumed as part of that invalid statement.

Do not expose its inner statements at the surrounding scope.

---

# For recovery

The current malformed-header recovery is retained conceptually.

Synchronize at:

```text
in
step
{
}
EOF
```

Preserve bindings, iterable, step, and body where parsed.

Unsupported forms receive focused diagnostics:

```text
condition-only for
C-style for
```

Recovery may still parse their body.

It must not compile them as canonical loops.

---

# While recovery

## Missing condition

Use `InvalidExpression`.

Preserve the body.

## Assignment condition

Emit an error, not only a warning.

Preserve the assignment-shaped invalid condition for tooling.

Do not compile using only the left operand.

## Missing block

Synchronize before the next statement or closing outer brace.

---

# Switch recovery

The existing case/default synchronization is canonical in principle.

## Case item

A malformed case item should create an invalid case item and continue at:

```text
,
:
case
default
}
EOF
```

## Missing colon

When the case item is complete and a statement clearly follows, insert a virtual
colon.

## Duplicate default

Preserve every default clause.

Sema or parser validation reports duplicates with related locations.

## Default order

A case after default is retained as source but diagnosed as invalid ordering.

---

# Match recovery

Match recovery must improve on the current close-brace-only skipping.

Synchronize a malformed arm at:

```text
=>
next probable pattern at the same brace depth and a later line
}
EOF
```

Use a dedicated pattern parser.

## Missing arrow

When the pattern is complete and the next token begins an arm body, insert a
virtual `=>`.

## Invalid pattern

Create `InvalidPattern`.

Do not parse the pattern as an unrestricted valid expression.

## Invalid body

Create `InvalidExpression` or invalid block body while preserving later arms.

## Exhaustiveness

Exhaustiveness is Sema's responsibility.

Invalid arms do not count as proof of exhaustiveness.

---

# Try-handler recovery

Try handlers use the same pattern and arm concepts as match.

Synchronize at:

```text
=>
next probable handler pattern at the same brace depth and a later line
}
next statement outside a missing closing brace
EOF
```

The current special recovery-start list must be replaced by the shared
statement-start predicate.

A malformed first handler must not discard later handlers.

---

# Return recovery

A return statement ends before:

```text
}
case
default
later-line statement start where no expression continuation exists
EOF
```

A malformed return expression becomes `InvalidExpression`.

Do not consume the next statement as part of the return.

---

# Defer recovery

After `defer`, canonical forms are:

```text
block
return
```

When neither appears:

- create an invalid defer statement;
- synchronize at next statement or closing brace;
- do not treat an arbitrary following statement as deferred.

---

# Discard recovery

After `discard`, an expression is required.

When absent:

- retain a `DiscardStatement` with `InvalidExpression`;
- stop before the closing brace or next statement;
- emit one focused diagnostic.

---

# Select recovery

The existing branch synchronization is a foundation.

Synchronize at:

```text
=>
after
default
next probable branch expression at same depth and later line
}
EOF
```

## Missing arrow

When the branch operation is complete and a block follows, insert virtual `=>`.

## Invalid binding branch

Preserve:

```text
binding name
:=
operation expression
body
```

## Default and timeout

A malformed default or after branch must not discard later branches.

---

# Unsafe recovery

A missing unsafe block produces an invalid unsafe statement.

Do not apply unsafe context to following unrelated statements.

The unsafe context begins only after a real or safely inserted opening brace and
ends at the corresponding real or virtual closing brace.

---

# Assembly recovery

Structured asm recovery must synchronize at:

```text
inputs
outputs
clobbers
}
EOF
```

Within a section, synchronize at:

```text
,
next line item
next section
}
EOF
```

Preserve the raw template and successfully parsed operands.

Do not let malformed asm consume the rest of the function.

---

# Expression recovery

Expression recovery must be precedence-aware.

Synchronization may stop at tokens that cannot continue the current expression:

```text
,
)
]
}
:
=>
case
default
statement boundary
EOF
```

The parser must still respect nested delimiters.

## Missing prefix operand

Example:

```sec
let value := -
```

Create an invalid right operand.

## Missing infix right operand

Example:

```sec
let value := left +
```

Preserve:

```text
left
+
InvalidExpression
```

## Unexpected operator

Example:

```sec
left + * right
```

Do not invent the intended operator.

Preserve the unexpected operator in the invalid expression.

## Grouped expression

A missing `)` may be inserted before a clear enclosing boundary.

## Postfix expression

Malformed call, index, slice, generic call, member access, or typed construction
should preserve the valid left expression and create an invalid postfix node.

---

# Member-access recovery

Example:

```sec
value.
```

Do not invent a member name.

Create an invalid member expression and stop at the expression boundary.

Example:

```sec
value..other
```

Do not silently reinterpret range syntax as member access or vice versa.

---

# Index and slice recovery

Inside brackets, synchronize at:

```text
]
,
range operator
outer expression boundary
EOF
```

Preserve:

```text
base expression
opening bracket
start expression
range operator
end expression
closing token or missing token
```

A missing index expression creates `InvalidExpression`.

A missing range bound may be valid depending on slice grammar.

---

# Range recovery

Ranges are contextual.

When a range appears where no range is permitted, preserve an invalid expression
and emit the contextual range diagnostic.

Do not create a first-class range value in Sec 0.1.

A malformed range should not consume the next statement while searching for a
bound.

---

# Contextual `x` recovery

After contextual matrix `x` is implemented:

- parse it only in valid infix position;
- preserve ordinary identifier `x`;
- a missing right operand creates an invalid infix expression;
- recovery must not reclassify an identifier named `x` outside infix context.

---

# Reserved syntax

Reserved syntax should receive focused diagnostics.

Examples:

```text
?
free
panic
assert
```

Reserved syntax is not an unknown-token lexer failure.

Preserve a dedicated invalid or reserved node where it helps tooling.

`?` remains without Sec 0.1 meaning.

`require` is an ordinary identifier spelling and is not handled by reserved
syntax recovery outside any future explicitly contextual grammar role.

Recovery must not assume ternary or Rust-style propagation semantics.

---

# Compatibility syntax

Compatibility syntax is parsed intentionally.

It is not an error-recovery guess.

Examples currently include selected forms such as:

```text
enum colon initializers
standalone `struct Name`
prefix sequence types
explicit `ref self`
assigned named-type syntax
```

Compatibility nodes or diagnostics should record:

```text
source form
canonical replacement
safe-fix range
```

A formatter may normalize only when the transformation is proven
semantics-preserving.

---

# Speculative parsing

Speculative parsing must be transactional.

The parser currently snapshots lexer position, tokens, error length, and warning
length for typed-declaration lookahead.

The canonical transaction must include every mutable parser state:

```text
lexer/token cursor
current token
lookahead token
diagnostics
warnings
recovery events
virtual tokens
delimiter stack
context stack
parser mode flags
partially appended AST nodes
```

## Commit

A speculative branch commits only when its distinguishing grammar token is
confirmed.

Examples:

```text
typed declaration colon
typed declaration mut marker
generic call followed by call parentheses
typed declaration group shape
```

## Rollback

Rollback must leave no:

```text
diagnostic
warning
invalid node
virtual token
skipped token
context flag
```

from the rejected branch.

---

# Parser context stack

The parser should maintain explicit recovery contexts.

Conceptual values:

```text
CompilationUnit
ImportGroup
TypeDeclaration
GenericParameters
StructBody
EnumBody
UnionBody
RegisterBody
InterfaceBody
ImplBody
PropertyBody
FunctionParameters
FunctionBody
StatementBlock
Expression
ArgumentList
ArrayLiteral
StructLiteral
SwitchBody
SwitchCase
MatchBody
MatchArm
TryHandlerBody
SelectBody
SelectBranch
AsmBody
AsmSection
```

Context drives:

```text
expected tokens
synchronization set
diagnostic wording
safe insertion choices
LSP completion
```

---

# Source ranges

Canonical source positions should support:

```text
file
start byte or rune offset
end byte or rune offset
start line
start column
end line
end column
```

The exact internal offset unit must be consistent.

LSP conversion to UTF-16 positions happens at the LSP boundary.

A virtual token has:

```text
start == end
```

at its insertion anchor.

Skipped source has a real nonempty range.

---

# Token and trivia retention

Recovery must preserve access to:

```text
all real tokens
comments
invalid lexer tokens
skipped tokens
virtual tokens
```

This may be implemented through:

```text
token buffer
lossless syntax tree
AST plus recovery ranges and original source
```

A fully lossless CST is not mandated for Sec 0.1.

The chosen model must support formatter and LSP requirements.

---

# Lexer errors

An `ILLEGAL` lexer token is not silently skipped.

The parser should create a lexical diagnostic and preserve the token.

Examples:

```text
unknown character
unterminated block comment
unterminated string
invalid character literal
```

The lexer must always advance after emitting an illegal token.

If it does not advance, the parser marks a fatal internal failure.

---

# Interaction with Sema

## Batch compilation

When parser errors exist:

- compilation fails;
- code generation is not attempted;
- Sema may run only to provide additional independent diagnostics where safe.

## Tooling analysis

For LSP, Sema may analyze valid subtrees.

It must:

- skip invalid declarations when symbol identity is unavailable;
- assign `ErrorType` to invalid expressions and type references;
- suppress diagnostics caused only by `ErrorType`;
- preserve symbols from valid parts of a recovered declaration when stable;
- avoid ownership-state changes caused by invalid expressions;
- avoid treating virtual syntax as user-confirmed semantic intent.

## Invalid symbols

An invalid declaration must not introduce a normal visible symbol unless its
name and declaration category are unambiguous.

When introduced for tooling, mark it invalid so it cannot participate as a
normal overload or type candidate.

## Ownership

Invalid syntax does not:

```text
move a value
borrow a value
destroy a value
allocate
call FFI
establish a resource state
```

Ownership analysis may conservatively mark affected flow unknown for later
diagnostic suppression.

---

# Interaction with formatter

## Valid surrounding code

The formatter may format valid regions surrounding an invalid node.

## Invalid region

The ordinary formatter should preserve the original token spelling and order
inside an invalid region as much as possible.

It must not print virtual tokens into source automatically.

## Explicit fixes

A code action may insert a virtual token into source after user selection.

## No semantic guessing

The formatter must not:

```text
invent an identifier
choose an operator
change copy to move
change move to copy
choose a type
delete a malformed declaration
```

## Stability

Formatting a recovered file must not move diagnostics to unrelated source
locations unnecessarily.

---

# Interaction with LSP

The LSP should use recovery contexts for:

```text
completion
signature help
hover on valid subtrees
document symbols
folding ranges
semantic tokens
code actions
outline
navigation
```

## Completion

Examples:

```sec
fn Add(left: |
```

The context indicates a type is expected.

```sec
Point {
    |
}
```

The context indicates a struct field or spread is expected.

```sec
match value {
    Pattern |
}
```

The context may suggest `=>` and guards.

## Hover

Hover is available on valid nodes inside a recovered declaration.

No fabricated type should be shown for an invalid node.

## Document symbols

A recovered function with a valid name should remain visible in the outline even
when its body is malformed.

## Semantic tokens

Tokens retain lexical classification.

Semantic classification is provided only where resolution is reliable.

## Diagnostics

The LSP receives structured ranges and IDs.

It must not parse line and column back out of message strings.

---

# Safe fixes

A parser diagnostic may carry a machine-applicable fix only when intent is
unique.

Typical safe fixes:

```text
insert missing `:`
insert missing `,`
insert missing `)`
insert missing `]`
insert missing `}`
insert missing `=>`
replace enum `:` with `=`
replace C-style for with no automatic fix unless mechanically safe
remove one duplicated comma
```

Not automatically safe:

```text
invent identifier
invent expression
choose type
choose operator
choose copy or move
choose match variant
choose function body
```

A diagnostic may offer a nonautomatic suggestion without a machine edit.

---

# Recovery quality rules

## Prefer local repair

Try one local structural repair before region skipping.

## Prefer preserving siblings

A repair preserving later list items or body members is better than abandoning
the complete parent.

## Prefer fewer assumptions

Inserting a missing comma is safer than inventing an expression.

## Prefer grammar context

Use the current grammar production rather than a generic token list.

## Prefer no cascade

After repairing one missing delimiter, suppress errors that are only artifacts
of the same repair.

## Never prefer successful misparse

A recovered parse that changes the apparent program is worse than an invalid
node.

---

# Recovery algorithm

Sec 0.1 may use deterministic recursive-descent recovery.

Recommended order at a failed expectation:

```text
1. If expected token is present, consume it.
2. If one safe structural insertion is uniquely justified, create a virtual
   token and continue.
3. If current token is one isolated unexpected separator, preserve and skip it.
4. If a synchronization token is already present, return without consuming an
   outer boundary.
5. Otherwise skip tokens with delimiter-depth tracking until a context-specific
   synchronization point.
6. Create or complete an invalid node.
7. Guarantee progress before re-entering the same parse loop.
```

A global minimum-cost parser repair algorithm is not required.

---

# Current implementation corrections

## Replace string diagnostics

Refactor:

```go
errors   []string
warnings []string
```

toward structured diagnostics.

Compatibility accessors may format strings temporarily for existing tests.

## Extend invalid AST

Keep `InvalidStatement`.

Add the other invalid-node categories and common recovery metadata.

## Replace global statement-start helper

Generate or centralize:

```text
top-level declaration starts
block statement starts
expression starts
type starts
impl member starts
interface member starts
property accessor starts
pattern starts
```

Do not let these tables drift independently from `grammar.md`.

## Preserve partial blocks

Change unterminated-block paths to return partial blocks with a virtual closer
instead of returning nil.

## Improve arm recovery

Replace `skipMatchArm()` and `skipTryHandler()` with sibling-aware recovery.

## Record skipped ranges

Every skip helper records the skipped tokens or source range.

## Correct while assignment

Implemented for `while`: assignment-in-condition recovery is a structured
`P2013` parser error.

Do not treat the recovered left operand as a valid complete condition for batch
compilation.

---

# Implementation status matrix

| Recovery area | Current status |
|---|---|
| continue to EOF after ordinary parser error | Implemented |
| string error collection | Implemented |
| structured parser diagnostics | Partly implemented; compatibility strings retained |
| general `P2001` registry entry | Implemented |
| focused parser diagnostic IDs | Registered; initial focused paths wired, broader migration pending |
| speculative lexer rollback | Implemented |
| speculative diagnostic rollback | Implemented for strings, structured diagnostics, dedup keys and deterministic episode numbering |
| speculative recovery-event rollback | Implemented for diagnostic-associated events |
| `InvalidStatement` | Implemented |
| invalid declaration node | Implemented for failed top-level declaration-shaped statements |
| invalid expression node | Implemented for Pratt prefix failures; broader retention pending |
| invalid type node | Partly implemented through invalid `TypeReference` metadata |
| invalid pattern node | Implemented for malformed match arms and try handlers |
| invalid member node | Implemented for disallowed impl members |
| missing-token node | Not implemented |
| skipped-token retention | Implemented for general statements and migrated delimiter-aware helpers; remaining specialized helpers pending |
| struct field synchronization | Implemented, node retention partial |
| malformed for-header recovery | Implemented |
| switch case synchronization | Implemented |
| select branch synchronization | Implemented |
| impl member synchronization | Delimiter-aware with retained invalid members; start set remains incomplete |
| target directive synchronization | Implemented |
| generic parameter synchronization | Implemented, coarse |
| unexpected else preservation | Implemented |
| missing try-handler brace recovery | Implemented for selected starts |
| sibling-aware match-arm recovery | Implemented for likely later-line patterns; same-line and grammar-derived sets pending |
| sibling-aware try-handler recovery | Implemented for likely later-line patterns; same-line and grammar-derived sets pending |
| partial unterminated block retention | Not implemented consistently |
| delimiter stack across all helpers | Shared stack implemented and used by major skip helpers; remaining ad hoc scans pending migration |
| recovery episode suppression | Implemented at integrated stable boundaries with same-location cascade suppression; broader causal suppression pending |
| parser diagnostic cap | Implemented at 100 diagnostics with one terminal `P2015` |
| recovery-aware formatter | Partly implemented or planned |
| recovery-aware LSP | Partly implemented or planned |
| progress fuzzing | Not implemented |
| recovery golden fixtures | Partly implemented |

---

# Tests

## General invariants

Every recovery test should verify:

```text
diagnostic count
diagnostic ID
primary range
recovery action
later sibling preservation
later statement preservation
later top-level declaration preservation
no panic
parser reaches EOF
stable AST shape
```

## One-error tests

For one local error, prefer exactly one primary parser diagnostic.

Examples:

```text
missing struct field colon
missing parameter comma
missing case colon
missing match arrow
missing closing parenthesis
```

## Cascade tests

Verify that one missing token does not produce unrelated errors on:

```text
next field
next parameter
next statement
next case
next arm
next declaration
```

## Progress tests

Feed repeated malformed tokens:

```text
}}}}}
,,,,,
?????
++++
```

The parser must terminate.

## Truncation tests

Truncate valid files after every token.

For every prefix:

- parser must not panic;
- parser must terminate;
- diagnostics must remain bounded;
- partial AST must remain internally valid.

## Mutation tests

From valid fixtures:

```text
delete one token
duplicate one token
replace one delimiter
move one closing brace
insert one keyword
```

Verify bounded diagnostics and later-node preservation.

## Fuzz tests

Fuzz:

```text
lexer input
parser token streams
nested delimiters
long malformed lists
long malformed expressions
Unicode identifiers
unterminated comments and strings
```

## Tooling tests

Verify completion and symbols in incomplete source.

Examples:

```sec
fn Test(value: |
```

```sec
type Point struct {
    x: |
}
```

```sec
match value {
    Some(value) |
}
```

---

# Required recovery fixtures

Create:

```text
parser_recovery_valid_context.sec
parser_recovery_missing_tokens.sec
parser_recovery_unexpected_tokens.sec
parser_recovery_declarations.sec
parser_recovery_statements.sec
parser_recovery_expressions.sec
parser_recovery_types.sec
parser_recovery_structs.sec
parser_recovery_impl.sec
parser_recovery_switch.sec
parser_recovery_match.sec
parser_recovery_try.sec
parser_recovery_select.sec
parser_recovery_asm.sec
parser_recovery_truncated.sec
parser_recovery_cascades.sec
```

Each malformed example should state:

```sec
/* Expected primary diagnostic: P....
 * Expected recovery: ...
 * Expected preserved node: ...
 */
```

---

# Parser bootstrap requirements

The Sec implementation of the parser should use the same recovery model.

It must have explicit types for:

```text
Diagnostic
RecoveryContext
RecoveryEvent
MissingToken
InvalidExpression
InvalidStatement
InvalidDeclaration
InvalidTypeReference
InvalidPattern
```

Parser helper methods should return structured results rather than relying only
on nullable nodes.

Conceptual form:

```sec
type ParseNodeResult[T] union {
    Parsed(T)
    Recovered(T)
    Failed
}
```

This is illustrative.

The exact bootstrap API may use another explicit type.

The important distinction is:

```text
valid node
recovered partial node
unrecoverable branch failure
```

---

# Required synchronization

This document must remain synchronized with:

```text
grammar.md
lexical_structure.md
operators.md
diagnostics.txt
lsp.md
formatter.md
semantic_ir.txt
compiler_pipeline.txt
types.md
functions.txt
struct.md
enums.md
unions.md
declarations/interfaces.md
impl.md
properties.md
collections.md
flowcontrol_if.txt
flowcontrol_for.txt
flowcontrol_while.txt
flowcontrol_switch.txt
flowcontrol_match.txt
errorhandling.txt
inline_assembly.md
language-rulebook-status.md
rules_implementations.txt
```

---

# Appendix A — Codex implementation plan

## A.1 Add the rulebook

Implemented.

Add:

```text
rules/compiler/parser_recovery.md
```

Update:

```text
language-rulebook-status.md
rules/compiler/rules_implementations.txt
```

Mark the rulebook as Written.

Do not mark the recovery implementation complete.

---

## A.2 Introduce structured parser diagnostics

Partly implemented. The parser and LSP have a structured compatibility path;
source ranges, related locations, fixes and broad focused-ID migration remain.

Replace parser-owned string slices with structured diagnostics.

Keep temporary compatibility formatting for existing tests.

Register the focused parser IDs from this document.

Add:

```text
primary source range
expected token set
unexpected token
related location
safe fixes
```

---

## A.3 Add source offsets

Extend lexer tokens or a source map with stable start and end offsets.

Retain line and column.

Use source-map conversion for LSP UTF-16 coordinates.

---

## A.4 Add parser context stack

Track the current grammar recovery context.

Use it for:

```text
diagnostics
synchronization
completion
missing-token insertion
```

---

## A.5 Add delimiter stack

Track `()`, `[]`, and `{}` nesting.

All skip functions must respect the current delimiter depth.

Add parser assertions for underflow and nonprogress.

---

## A.6 Add invalid nodes

Add:

```text
InvalidDeclaration
InvalidExpression
InvalidTypeReference
InvalidPattern
InvalidMember
```

Extend `InvalidStatement`.

Add common `RecoveryInfo`.

Update visitors, formatter, Sema, and LSP.

---

## A.7 Add virtual tokens

Represent safe missing structural tokens explicitly.

Do not mutate the lexer's real token stream.

Add code-action generation from missing-token metadata.

---

## A.8 Centralize grammar start predicates

Create shared definitions for:

```text
declaration start
statement start
expression start
type start
pattern start
impl member start
interface member start
property accessor start
```

Update all parser paths and tests.

This fixes the current incomplete general recovery set.

---

## A.9 Preserve partial blocks

Update block parsers to return partial blocks on EOF or a reliable outer
boundary.

Attach a virtual closing brace and diagnostic.

Do not drop already parsed statements.

---

## A.10 Refactor skip helpers

Replace ad hoc functions with context-aware helpers.

Conceptual APIs:

```go
func (p *Parser) recoverTo(context RecoveryContext, extra ...TokenPredicate)
func (p *Parser) recoverList(close TokenType, itemStart TokenPredicate)
func (p *Parser) recoverBlock(memberStart TokenPredicate)
```

Specialized wrappers may remain.

Every helper records skipped source.

---

## A.11 Improve lists

Implement item-preserving recovery for:

```text
imports
generic parameters
type arguments
parameters
arguments
array elements
struct fields
struct literal items
enum values
union variants
register fields
```

---

## A.12 Improve arms and branches

Implement sibling-aware recovery for:

```text
match arms
try handlers
switch cases
select branches
property accessors
asm sections
```

Do not scan directly to the parent closing brace after one malformed sibling.

---

## A.13 Correct invalid condition recovery

Implemented for `while` conditions.

Assignment in condition is an error.

Retain partial syntax for tooling.

Block batch compilation.

---

## A.14 Integrate Sema

Add `ErrorType`.

Define visitor behavior for every invalid node.

Suppress dependent cascades.

Do not allow invalid nodes into Semantic IR.

---

## A.15 Integrate formatter

Preserve malformed regions.

Do not emit virtual tokens automatically.

Support explicit safe fixes.

---

## A.16 Integrate LSP

Use recovery contexts for completion and signature help.

Expose structured diagnostics and code actions.

Keep valid document symbols from recovered declarations.

---

## A.17 Add recovery limits

Add:

```text
diagnostic cap
recovery-event cap if needed
maximum skipped region accounting
progress assertions
```

Emit `P2015` once when the diagnostic cap is reached.

---

## A.18 Convert existing tests

Migrate exact string comparisons toward:

```text
diagnostic ID
range
message fragment
recovery shape
```

Message text may evolve without breaking semantic diagnostic tests.

Keep selected golden-message tests for user-facing quality.

---

## A.19 Add fuzz and mutation tests

Add deterministic seeds reproducing every fixed recovery bug.

Run parser fuzzing in CI with a bounded corpus and time budget.

---

## A.20 Recommended implementation order

```text
1. Add structured source ranges and diagnostics.
2. Add recovery contexts and centralized start predicates.
3. Add delimiter tracking and progress assertions.
4. Add MissingToken and RecoveryInfo.
5. Add invalid expression/type/declaration/pattern nodes.
6. Preserve partial unterminated blocks.
7. Refactor list recovery.
8. Refactor match, try, switch, select, property, and asm recovery.
9. Add ErrorType and Sema cascade suppression.
10. Integrate formatter and LSP.
11. Add diagnostic cap.
12. Add truncation, mutation, and fuzz tests.
```

---

# Design summary

Sec uses deterministic, context-specific parser recovery.

The parser returns a partial program after ordinary syntax errors.

Parser errors block normal compilation and code generation.

Tooling may analyze valid subtrees.

Recovery never silently changes invalid source into another valid program.

The parser may insert a unique missing structural token virtually.

It does not invent identifiers, expressions, types, or operators.

Recoverable malformed constructs produce invalid AST nodes rather than nil.

Synchronization respects delimiter depth and grammar context.

One local syntax cause should normally produce one primary diagnostic.

Skipped tokens and virtual tokens remain distinguishable from real valid syntax.

Speculative parsing is fully transactional.

Sema uses an internal error type to suppress dependent cascades.

The formatter preserves malformed regions.

The LSP uses recovery context for completion, symbols, and safe fixes.

The current parser already has useful local recovery for structs, loops,
switches, selects, impl members, target directives, and selected try errors.

The main implementation work is to make that recovery structured, consistent,
sibling-preserving, and tooling-aware.

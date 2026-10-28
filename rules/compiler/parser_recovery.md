# Parser Recovery

- **Status:** Normative
- **Created:** 2026-08-07
- **Last updated:** 2026-10-28 (revision 2.0: first numbered revision; implementation status, fixtures and the implementation plan moved to governance; MD-003, MD-029, MD-042 and MD-102 synchronization; `rules/corrections/sec-compiler-rulebooks-modernization-pass1-20261028.md`)
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/compiler/parser_recovery.md`
- **Repository baseline reviewed:** `7be8777`
- **Implementation governance:** `governance/parser.yaml` (`frontend.parser-recovery-result`, `frontend.grammar-conformance`, `frontend.parser-long-term`)
- **Accepted grammar owners:** `rules/foundations/grammar.md`, `rules/foundations/lexical_structure.md`, `rules/foundations/operators.md` and the specialized feature rulebooks
- **Related rulebooks:** `rules/tooling/diagnostics.md`, `rules/tooling/lsp.md`, `rules/tooling/formatter.md`, `rules/compiler/semantic_ir.md`, `rules/compiler/compiler_pipeline.md`, `rules/compiler/compiler.md`, `rules/types/types.md`, `rules/declarations/functions.md`, `rules/declarations/struct.md`, `rules/declarations/enums.md`, `rules/declarations/unions.md`, `rules/declarations/interfaces.md`, `rules/declarations/impl.md`, `rules/declarations/properties.md`, `rules/collections/collections.md`, `rules/control-flow/flowcontrol_if.md`, `rules/control-flow/flowcontrol_for.md`, `rules/control-flow/flowcontrol_while.md`, `rules/control-flow/flowcontrol_switch.md`, `rules/control-flow/flowcontrol_match.md`, `rules/errors/errorhandling.md`, `rules/errors/panic.md`, `rules/platform/inline_assembly.md`

---

## § 1 Purpose

§ 1(1) The Sec parser must remain useful when source code is:

```text
incomplete
temporarily malformed during editing
written with one local syntax mistake
using reserved or legacy syntax
truncated at end of file
damaged inside one declaration or block
```

§ 1(2) A syntax error should not normally prevent the parser from finding later valid
declarations and statements.

§ 1(3) Recovery exists to:

- report the primary syntax problem;
- preserve as much source structure as possible;
- avoid cascading diagnostics;
- allow later code to be parsed;
- support LSP features on incomplete files;
- support formatter and refactoring tools without inventing semantics.

§ 1(4) Recovery does not make invalid source valid.

§ 1(5) Parser recovery describes the treatment of incomplete and malformed source
syntax under the selected Sec language version. It must not broaden or narrow
the accepted grammar independently of `rules/foundations/grammar.md` or a
specialized active feature rulebook. On conflict, accepted grammar and semantic
validity are determined by those owners; this rulebook owns source
preservation, diagnostic recovery and parser progress.

§ 1(6) This rulebook does not record implementation status. Implemented, partial and
remaining recovery work is tracked by the governance entries named in the
header.

---

## § 2 Core recovery principle

§ 2(1) Recovery may preserve incomplete syntax, but it must never silently reinterpret
invalid source as a different valid program.

§ 2(2) Example:

```sec
fn Add(left int, right: int) int {
    return left + right
}
```

§ 2(3) The parser may identify that `:` is missing after `left`.

§ 2(4) It may create a virtual missing colon and continue parsing the parameter.

§ 2(5) It must not reinterpret:

```sec
left int
```

as another declaration form.

---

## § 3 Compiler result

§ 3(1) The parser returns a parse result even when syntax errors exist (§ 8).

§ 3(2) A normal batch compilation must not proceed to code generation when parser
errors exist.

§ 3(3) Tooling may continue with partial semantic analysis of valid subtrees.

---

## § 4 Terminology

### § 4.1 Syntax error

§ 4.1(1) A syntax error is source that cannot be parsed as canonical syntax of the
selected language version.

§ 4.1(2) Examples:

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

### § 4.2 Recovery

§ 4.2(1) Recovery is the parser's controlled continuation after a syntax error.

### § 4.3 Repair

§ 4.3(1) A repair is the parser's internal model of the smallest assumed correction
needed to continue.

§ 4.3(2) A repair may be:

```text
insert one missing structural token
skip one unexpected token
skip a malformed region
close a truncated construct at end of file
```

§ 4.3(3) The parser does not modify the user's source.

### § 4.4 Virtual token

§ 4.4(1) A virtual token represents a required token missing from source.

§ 4.4(2) Example:

```text
MissingToken(":")
```

§ 4.4(3) A virtual token has a zero-width source position.

### § 4.5 Synchronization point

§ 4.5(1) A synchronization point is a token or grammar boundary where parsing can safely
resume.

§ 4.5(2) Examples:

```text
next top-level declaration
next statement at the current block depth
comma in a list
closing delimiter
next switch case
next impl member
```

### § 4.6 Invalid node

§ 4.6(1) An invalid node is an AST node preserving malformed source and partial children.

§ 4.6(2) It is not a valid semantic construct.

### § 4.7 Cascading diagnostic

§ 4.7(1) A cascading diagnostic is a secondary error caused only by an earlier syntax
error and poor recovery.

### § 4.8 Fatal parser failure

§ 4.8(1) A fatal parser failure means the parser cannot safely continue.

§ 4.8(2) Ordinary source syntax errors are not fatal.

---

## § 5 Recovery goals

§ 5(1) The parser should:

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

## § 6 Non-goals

§ 6(1) Sec 0.1 recovery does not require:

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

§ 6(2) A later parser may use a more advanced repair algorithm.

§ 6(3) The observable invariants in this document must remain unchanged.

---

## § 7 Recovery invariants

### § 7.1 Progress

§ 7.1(1) Every parser loop must either:

- consume at least one real token;
- insert one bounded virtual token and change parser state;
- return to its caller;
- stop at end of file.

§ 7.1(2) No recovery path may repeatedly inspect the same token in the same state.

### § 7.2 Enclosing-boundary safety

§ 7.2(1) Recovery inside a construct must not consume the closing delimiter belonging to
an outer construct.

§ 7.2(2) Example:

```sec
fn Test() void {
    let value := Call(
        first,
        second

    return
}
```

§ 7.2(3) Argument-list recovery may stop before `return` or the function `}`.

§ 7.2(4) It must not consume the complete function body looking for `)`.

### § 7.3 No silent reinterpretation

§ 7.3(1) A repair must not change the likely grammatical category of source without a
diagnostic.

§ 7.3(2) Invalid:

```sec
while running = true {
}
```

§ 7.3(3) The parser may preserve `running` as a partial condition for tooling.

§ 7.3(4) It must emit an error for assignment in condition position.

§ 7.3(5) It must not compile the source as though the user wrote:

```sec
while running {
}
```

### § 7.4 No semantic repair

§ 7.4(1) The parser must not decide:

```text
which type was intended
which variable was intended
which overload was intended
which operator was intended
whether a value should be copied or moved
whether a contract should succeed
```

### § 7.5 Determinism

§ 7.5(1) Given the same token stream and parser version, recovery must produce:

```text
the same diagnostics
the same invalid nodes
the same virtual tokens
the same synchronization result
```

### § 7.6 Bounded damage

§ 7.6(1) One malformed member should not normally invalidate later siblings.

§ 7.6(2) Examples:

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

### § 7.7 Error blocks code generation

§ 7.7(1) A recovered AST is not proof of a valid program.

§ 7.7(2) Any parser diagnostic with error severity blocks ordinary lowering and code
generation.

---

## § 8 Parse result

§ 8(1) The parser returns one result for each parsed source. The result contains:

```text
the partial program
structured diagnostics
recovery events
whether any error was reported
whether parsing failed fatally
```

§ 8(2) The result must distinguish:

```text
valid parse with no diagnostics
valid parse with an advisory diagnostic
recovered parse with errors
fatal parser failure
```

§ 8(3) These are observable contract fields. The implementation language, package
and field names are not normative.

---

## § 9 Structured parser diagnostic

§ 9(1) A parser diagnostic is structured data with:

```text
stable registered ID
severity from the diagnostic registry
message
primary source range
related locations
expected tokens where known
the unexpected token where present
safe fixes where justified
```

§ 9(2) Parser code must not embed the only copy of a source location into a message
string.

§ 9(3) Human-readable messages may still include token spellings.

---

## § 10 Parser diagnostic IDs

§ 10(1) Parser diagnostics have stable registered IDs in the `P2xxx` namespace,
structured primary ranges, related locations, notes and help as defined by
`rules/tooling/diagnostics.md`. The registered catalog (`sec diagnostics`) is
the authority for each ID, symbolic name, default severity and
mandatory/configurable status; this rulebook does not allocate or redefine
them.

§ 10(2) `P2001` / `parser.syntax-error` is the fallback for a parser error that has no
focused classification.

§ 10(3) IDs identify parser rules, not severity.

§ 10(4) Mandatory syntax violations remain errors. A legacy or noncanonical form that
has a focused registered diagnostic is reported with that ID (§ 70).

§ 10(5) A recovery event records the parser's treatment of invalid syntax. It does not
insert a valid token into the actual source or change the program's semantic
status.

---

## § 11 Recovery event

§ 11(1) A recovery event records what the parser did after a diagnostic:

```text
the diagnostic it belongs to
the recovery context
the unexpected token
virtual tokens inserted
tokens or source ranges skipped
the token where parsing resumed
```

§ 11(2) Recovery events support:

```text
debugging
tests
LSP behavior
formatter behavior
parser telemetry
comparison between parser implementations
```

§ 11(3) They do not become source-language semantics.

---

## § 12 Virtual missing tokens

### § 12.1 Purpose

§ 12.1(1) A missing token allows the parser to preserve a surrounding node without
pretending that the source contained the token.

§ 12.1(2) Example:

```sec
type Point struct {
    x int,
    y: int,
}
```

§ 12.1(3) Recovery may create:

```text
StructField
    name: x
    colon: MissingToken(":")
    type: int
```

### § 12.2 Representation

§ 12.2(1) A missing token records:

```text
the expected token kind
its insertion anchor position
the real token it precedes
```

§ 12.2(2) A missing token:

- has zero width;
- records the expected token kind;
- records its insertion anchor;
- is marked synthetic;
- is never returned by the lexer;
- is never printed as though present in source;
- may support an explicit code action.

### § 12.3 Safe insertion categories

§ 12.3(1) The parser may insert one missing structural token when the expected choice is
unique and local.

§ 12.3(2) Typical safe categories:

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

§ 12.3(3) The exact insertion is context-dependent.

### § 12.4 Tokens not guessed

§ 12.4(1) Recovery must not invent:

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

§ 12.4(2) When such content is missing, create an invalid node and synchronize.

### § 12.5 No repeated insertion

§ 12.5(1) At one source position, the parser must not repeatedly insert the same token
without consuming input or returning.

---

## § 13 Unexpected token handling

### § 13.1 Single-token deletion

§ 13.1(1) The parser may skip one unexpected punctuation token when:

- the surrounding grammar is otherwise unambiguous;
- skipping does not cross a delimiter boundary;
- the token is preserved in recovery metadata;
- one diagnostic is emitted.

§ 13.1(2) Example:

```sec
Call(first,, second)
```

§ 13.1(3) The second comma may be skipped.

### § 13.2 Keyword deletion

§ 13.2(1) The parser should not silently delete a keyword that can begin another
declaration or statement.

§ 13.2(2) It should synchronize before that keyword when possible.

### § 13.3 Token substitution

§ 13.3(1) The parser does not silently substitute one real token for another.

§ 13.3(2) Example:

```sec
Value: 10
```

inside an enum is invalid legacy syntax diagnosed with `P2028`
(§ 38).

§ 13.3(3) That is not ordinary recovery substitution.

---

## § 14 Invalid AST nodes

§ 14(1) Recoverable malformed source should normally produce an invalid node rather than
`nil`.

### § 14.1 Required node families

§ 14.1(1) Add:

```text
InvalidDeclaration
InvalidStatement
InvalidExpression
InvalidTypeReference
InvalidPattern
InvalidMember
```

### § 14.2 Common recovery metadata

§ 14.2(1) Recovery metadata records:

```text
the diagnostic ID
the unexpected token
the expected tokens
virtual tokens inserted
tokens skipped
the source range
```

§ 14.2(2) Each invalid node should retain:

```text
start token
source range
recovery information
successfully parsed children
raw or skipped tokens
```

### § 14.3 Invalid declaration

§ 14.3(1) An invalid declaration may retain:

```text
declaration keyword
name
generic parameters
partial type
partial body
```

### § 14.4 Invalid expression

§ 14.4(1) An invalid expression may retain:

```text
left operand
operator
partial right operand
opening delimiter
parsed arguments
```

### § 14.5 Invalid type reference

§ 14.5(1) An invalid type reference may retain:

```text
base name
generic arguments
array suffixes
reference modifier
unit annotation
```

### § 14.6 Invalid pattern

§ 14.6(1) An invalid pattern may retain:

```text
variant name
partial payload binding
guard
```

### § 14.7 Nil use

§ 14.7(1) A parse function may return `nil` only when:

- its caller has not committed to that grammar alternative;
- speculative parsing is being rolled back;
- a fatal parser invariant prevents node construction.

§ 14.7(2) After a construct has been committed, recoverable errors should produce a
partial or invalid node.

---

## § 15 Error type

§ 15(1) Sema should assign invalid expressions and invalid type references an internal
error type.

§ 15(2) Conceptual:

```text
ErrorType
```

§ 15(3) `ErrorType`:

- is not a user-visible Sec type;
- is compatible enough to prevent cascades;
- does not satisfy ordinary contracts;
- cannot reach lowering;
- carries no ownership resource;
- does not authorize unsafe operations;
- does not create a valid overload candidate.

---

## § 16 Parser error episodes

§ 16(1) A recovery episode begins when the parser emits a primary syntax diagnostic.

§ 16(2) It ends when the parser:

- reaches a synchronization point;
- successfully consumes the expected closing delimiter;
- completes the current invalid node;
- returns to a stable caller boundary.

§ 16(3) Within one episode, secondary diagnostics caused solely by the same missing
token should be suppressed.

§ 16(4) Example:

```sec
fn Test(value int result: int) void {
}
```

§ 16(5) The parser should not emit independent errors for every token after the first
missing parameter punctuation if one local repair explains them.

---

## § 17 Diagnostic deduplication

§ 17(1) Diagnostics with the same:

```text
ID
primary source range
recovery context
```

should not be emitted more than once.

§ 17(2) Speculative parse diagnostics must be rolled back entirely.

§ 17(3) A diagnostic from an abandoned parser branch must never reach the user.

---

## § 18 Diagnostic limit

§ 18(1) The parser should cap syntax diagnostics per file.

§ 18(2) Recommended default:

```text
100 parser errors
```

§ 18(3) After the cap:

- emit `P2015 parser.recovery-limit` once;
- continue only enough to find EOF safely;
- do not emit further parser errors;
- still return the partial program;
- mark the result as containing errors.

§ 18(4) The cap is an implementation policy, not source-language semantics.

---

## § 19 Fatal conditions

§ 19(1) The parser may mark a result fatal only for conditions such as:

```text
lexer fails to advance
corrupt token stream
impossible delimiter-stack state
internal parser invariant violation
resource exhaustion
```

§ 19(2) An unterminated user block or malformed expression is not fatal.

---

## § 20 Delimiter tracking

§ 20(1) Recovery must track nesting for:

```text
(...)
[...]
{...}
```

§ 20(2) A context recovery function must ignore synchronization tokens inside deeper
nested delimiters.

§ 20(3) Example:

```sec
Call(
    Other(first, second),
    third
)
```

§ 20(4) The comma inside `Other(...)` is not a synchronization comma for the outer
argument if recovery is currently inside the nested call.

### § 20.1 Mismatched closer

§ 20.1(1) When the parser expects one closer and finds another:

```sec
Call(value]
```

it should:

1. report the mismatched closer;
2. preserve the real `]`;
3. create a missing `)` only when doing so does not consume an outer boundary;
4. allow the caller to process `]` if it belongs to an outer construct.

---

## § 21 Synchronization model

§ 21(1) Synchronization sets are grammar-context-specific.

§ 21(2) Do not use one global panic-mode set for every parse failure.

§ 21(3) Each parser context must define:

```text
hard boundaries
sibling starts
separators
closing delimiters
tokens that must not be consumed
```

---

## § 22 Compilation-unit synchronization

### § 22.1 Hard boundaries

§ 22.1(1) At top level, recover toward:

```text
@target / @when attribute-set start
legacy #target start (compatibility form, compiler.md § 15)
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

§ 22.1(2) Contextual top-level declarations may require predicate-based detection.

### § 22.2 Rule

§ 22.2(1) A malformed top-level declaration must not consume the next probable top-level
declaration at brace depth zero.

### § 22.3 Grammar-derived start sets

§ 22.3(1) Declaration and statement start sets used for synchronization come from
canonical grammar and lexer metadata, not from an independently maintained
list that becomes stale when new syntax is introduced. `#target` is a legacy
compatibility form and is not advertised as canonical source syntax.

---

## § 23 General statement synchronization

§ 23(1) Within a block, synchronize toward:

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

§ 23(2) Because identifiers can begin either expressions or typed declarations,
statement synchronization cannot be only a static token set.

§ 23(3) Use:

```text
token kind
line position
delimiter depth
typed-declaration lookahead
current grammar context
```

### § 23.1 Same-block requirement

§ 23.1(1) A synchronization candidate must be at the current block's delimiter depth.

---

## § 24 Block recovery

§ 24(1) When a block is missing `}`:

- preserve parsed statements;
- create a virtual missing `}`;
- emit an unterminated-block diagnostic;
- stop before a token known to start an enclosing sibling when such a boundary
  is reliable;
- stop at EOF otherwise.

§ 24(2) Do not discard the partial block by returning `nil`.

### § 24.1 Function boundary heuristic

§ 24.1(1) At module scope, a probable top-level declaration at brace depth zero may close
a truncated function block virtually.

§ 24.1(2) This heuristic is permitted only when the parser can prove it is no longer
inside a nested delimiter.

---

## § 25 Declaration-header recovery

§ 25(1) Declaration headers include:

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

§ 25(2) Recover toward:

```text
opening body brace
end of declaration line where body is not required
next top-level declaration
closing enclosing brace
EOF
```

§ 25(3) A malformed header should produce an invalid declaration preserving:

```text
keyword
name when parsed
generic parameters when parsed
partial type information
```

---

## § 26 Import recovery

### § 26.1 Single import

§ 26.1(1) Recover toward:

```text
string path
next top-level declaration
EOF
```

§ 26.1(2) Do not invent an import path.

### § 26.2 Import group

§ 26.2(1) Inside `import (...)`, synchronize at:

```text
next string literal
probable alias followed by string literal
)
EOF
```

§ 26.2(2) An invalid import item should not discard later valid import items.

§ 26.2(3) A missing `)` may be inserted before the next top-level declaration at depth
zero.

---

## § 27 Compiler-directive recovery

§ 27(1) Inside `#target(...)`, synchronize at:

```text
,
)
next top-level declaration
EOF
```

§ 27(2) Unknown argument names remain part of one invalid directive node.

§ 27(3) Do not interpret malformed directive tokens as ordinary code inside the
directive.

---

## § 28 Generic parameter recovery

§ 28(1) Inside generic parameter declarations, synchronize at:

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

§ 28(2) A malformed generic parameter should preserve later comma-separated parameters
when possible.

---

## § 29 Generic argument recovery

§ 29(1) Inside generic or collection type arguments, synchronize at:

```text
,
]
)
{
EOF
```

§ 29(2) The parser must retain whether an argument position expects:

```text
type argument
constant argument
either, pending Sema
```

§ 29(3) Do not silently convert an invalid constant argument into a type argument.

---

## § 30 Parameter-list recovery

§ 30(1) Inside a function or lambda parameter list, synchronize at:

```text
,
)
return type start
{
EOF
```

### § 30.1 Missing colon

§ 30.1(1) Example:

```sec
fn Add(left int, right: int) int {
}
```

§ 30.1(2) When an identifier is followed by a valid type start, insert a virtual `:` and
continue.

§ 30.1(3) Emit one missing-token diagnostic.

### § 30.2 Missing parameter name

§ 30.2(1) Example:

```sec
fn Add(: int) int {
}
```

§ 30.2(2) Do not invent a name.

§ 30.2(3) Create an invalid parameter node and synchronize at comma or `)`.

### § 30.3 Missing type

§ 30.3(1) Example:

```sec
fn Add(left:, right: int) int {
}
```

§ 30.3(2) Do not invent a type.

§ 30.3(3) Create an invalid type reference and synchronize at comma or `)`.

### § 30.4 Missing comma

§ 30.4(1) When the next tokens clearly begin another `Identifier ":"` parameter, insert a
virtual comma.

---

## § 31 Argument-list recovery

§ 31(1) Inside a call, synchronize at:

```text
,
)
]
}
=>
EOF
```

§ 31(2) A missing argument expression creates an invalid expression.

§ 31(3) Example:

```sec
Call(first, , third)
```

§ 31(4) The parser should preserve argument positions:

```text
first
InvalidExpression
third
```

§ 31(5) This is useful for parameter completion.

---

## § 32 Array-literal recovery

§ 32(1) Inside an array literal, synchronize at:

```text
,
]
}
EOF
```

§ 32(2) Preserve later elements after one malformed element.

§ 32(3) A missing comma may be inserted only when the next token clearly begins another
element at the same bracket depth.

---

## § 33 Type-reference recovery

§ 33(1) Type-reference recovery must account for:

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

§ 33(2) Synchronize at grammar boundaries such as:

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

§ 33(3) Do not consume an assignment initializer while searching for a missing type
delimiter.

---

## § 34 Type-contract recovery

### § 34.1 Range contract

§ 34.1(1) Synchronize at:

```text
..
..<
next contract start
default
opening type body
next declaration
EOF
```

§ 34.1(2) At least one bound is required.

§ 34.1(3) Do not invent a numeric bound.

### § 34.2 `in [...]`

§ 34.2(1) Synchronize individual values at:

```text
,
]
next contract start
default
EOF
```

§ 34.2(2) Preserve later values.

### § 34.3 Value contract

§ 34.3(1) For contracts such as `multipleOf`, a missing value creates an invalid
expression attached to the contract.

---

## § 35 Default-clause recovery

§ 35(1) After `default`, an expression is required.

§ 35(2) If absent:

- create an invalid default expression;
- retain the type declaration;
- synchronize at type-body start, next top-level declaration, closing enclosing
  brace, or EOF.

§ 35(3) Do not substitute the implicit type default.

§ 35(4) The declaration remains invalid until corrected.

---

## § 36 Struct declaration recovery

§ 36(1) Inside a struct body, synchronize at:

```text
,
}
next probable field start on a later line
EOF
```

### § 36.1 Missing field colon

§ 36.1(1) Example:

```sec
type Point struct {
    x int,
    y: int,
}
```

§ 36.1(2) When `int` is a valid type start, create a virtual `:`.

§ 36.1(3) Preserve the field as a recovered field rather than dropping it.

§ 36.1(4) The malformed field is retained as a recovered field node.

### § 36.2 Missing comma

§ 36.2(1) Declared struct fields require commas.

§ 36.2(2) When the next line clearly begins `Identifier ":"`, insert a virtual comma and
continue.

§ 36.2(3) A comment after the field is trivia and is not a separator; the same proof
applies to the first token after the comment.

§ 36.2(4) The same later-line repair applies to parameters (the next line begins
`Identifier ":"`, `ref`, or `->`), call arguments, and array literal elements
(the next line begins an expression). The repair records the token after
which the virtual comma sits, so the formatter Language Correction
(`rules/tooling/formatter.md` § 27(37)) and the LSP quick fix insert exactly
that comma. Same-line adjacency is reported without a repair.

### § 36.3 Invalid field type

§ 36.3(1) Create an invalid type-reference field and synchronize at comma or `}`.

### § 36.4 Invalid tag

§ 36.4(1) Preserve:

```text
field name
field type
raw tag token
```

§ 36.4(2) Attach a tag diagnostic.

§ 36.4(3) Do not discard the whole struct.

### § 36.5 Unterminated struct

§ 36.5(1) Preserve parsed fields and insert a virtual `}` at a safe outer boundary or EOF.

---

## § 37 Struct literal recovery

§ 37(1) Inside a struct literal, synchronize at:

```text
,
line-start next field
}
EOF
```

§ 37(2) Struct literals permit newline-separated fields according to `grammar.md`.

### § 37.1 Missing colon

§ 37.1(1) When the left expression is a simple identifier and the following token starts
an expression, a missing `:` may be inserted.

### § 37.2 Invalid item

§ 37.2(1) Preserve an invalid struct-literal item.

§ 37.2(2) Do not discard later fields or spreads.

### § 37.3 Missing closing brace

§ 37.3(1) Preserve parsed items and create a virtual `}` at a reliable enclosing boundary.

---

## § 38 Enum recovery

§ 38(1) Inside an enum body, synchronize at:

```text
,
line-start next identifier
}
EOF
```

### § 38.1 Missing value name

§ 38.1(1) Create an invalid enum member and continue to the next separator.

### § 38.2 Invalid initializer

§ 38.2(1) Preserve the member name and attach an invalid expression.

### § 38.3 Legacy colon initializer

§ 38.3(1) `Value: expression` is invalid legacy syntax. It is reported with the focused
registered diagnostic `P2028` / `parser.legacy-enum-colon-initializer`, blocks
artifact publication and is not accepted as a soft warning. Recovery retains
the member name and the initializer for tooling. An explicitly enabled
Language Correction (`rules/tooling/formatter.md` §§ 26–27) may rewrite it to:

```sec
Value = expression
```

§ 38.3(2) It is not ordinary missing-token recovery.

### § 38.4 Unterminated enum

§ 38.4(1) Preserve parsed values and create a virtual `}` at a safe boundary.

---

## § 39 Union recovery

§ 39(1) Inside a union body, synchronize at:

```text
,
line-start next variant name
}
EOF
```

§ 39(2) Preserve:

```text
variant name
payload opening delimiter
partial payload type or fields
```

§ 39(3) A malformed payload must not discard later variants.

---

## § 40 Reversed type declaration order

§ 40(1) The parser recognizes declaration-kind-before-name mistakes such as:

```sec
type struct User { ... }
type union State { ... }
type register Status[8] { ... }
```

§ 40(2) These forms remain syntax errors. The parser emits `P2011` and explains the
canonical `type Name kind` order. Because the intended ordering is unique,
fix-enabled formatting may rewrite them while ordinary formatting preserves
the invalid source around the diagnostic.

---

## § 41 Register recovery

§ 41(1) Inside a register body, synchronize at:

```text
,
line-start next field or `_`
}
EOF
```

§ 41(2) Preserve field width syntax where parsed.

§ 41(3) Width-total validation remains Sema's responsibility.

---

## § 42 Interface recovery

§ 42(1) Inside an interface body, synchronize at:

```text
fn
property
contextual event
}
EOF
```

§ 42(2) An invalid member becomes `InvalidMember`.

§ 42(3) It must not cause the complete interface declaration to return `nil`.

§ 42(4) Property requirements synchronize at:

```text
get
set
}
```

---

## § 43 Impl recovery

§ 43(1) Inside an impl body, synchronize at:

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

§ 43(2) Impl-member recovery is line- and depth-aware and must:

- preserve invalid members;
- include every canonical impl-member start;
- avoid relying only on line change;
- retain skipped tokens;
- support nested delimiters.

§ 43(3) An invalid local `let` inside impl should remain an invalid member with a
structured diagnostic.

---

## § 44 Property recovery

§ 44(1) Inside a property body, synchronize at:

```text
get
set
try set
}
EOF
```

§ 44(2) A malformed getter must not discard a later setter.

§ 44(3) A malformed setter must not discard a later getter.

### § 44.1 Missing setter parameter

§ 44.1(1) Do not invent the parameter name.

§ 44.1(2) Create an invalid setter node, consume or preserve its body, and continue at the
next accessor or property `}`.

### § 44.2 Duplicate accessor

§ 44.2(1) Duplicate accessors are semantic or structural property errors.

§ 44.2(2) Preserve both accessors so tooling can show both source ranges.

---

## § 45 Event recovery

§ 45(1) Event declarations synchronize at:

```text
using
next impl member
}
EOF
```

§ 45(2) A missing storage name must not consume the next impl member.

§ 45(3) Do not silently skip an event body unless event-body syntax becomes canonical.

---

## § 46 Function recovery

§ 46(1) Function declaration recovery must preserve as much as possible:

```text
name
generic parameters
parameters
return type
body
```

### § 46.1 Missing function name

§ 46.1(1) Create an invalid declaration.

§ 46.1(2) Synchronize at generic parameters, `(`, return type, body, or next declaration.

§ 46.1(3) Do not create a public anonymous named function.

### § 46.2 Missing return type

§ 46.2(1) The return type is required.

§ 46.2(2) Create an invalid type reference before the body.

§ 46.2(3) Do not infer `void`.

### § 46.3 Missing body

§ 46.3(1) For an ordinary function, create a missing-body diagnostic.

§ 46.3(2) Do not consume the next top-level declaration as the body.

§ 46.3(3) Interface and extern declarations follow their specialized rules.

---

## § 47 Extern recovery

§ 47(1) Recover around:

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

§ 47(2) Do not invent an ABI string.

§ 47(3) Preserve enough information for FFI diagnostics.

---

## § 48 Let-declaration recovery

§ 48(1) Synchronize each declarator at:

```text
,
next statement start at same depth
}
EOF
```

§ 48(2) Preserve:

```text
let token
mutability
name
type
copy or move initializer operator
partial initializer
```

### § 48.1 Missing name

§ 48.1(1) Do not invent an identifier.

§ 48.1(2) Create an invalid declarator and continue at comma or statement boundary.

### § 48.2 Missing type after colon

§ 48.2(1) Create an invalid type reference.

§ 48.2(2) Do not reinterpret the following initializer as a type.

### § 48.3 Missing initializer for immutable binding

§ 48.3(1) This is a language diagnostic after the declaration shape is parsed.

§ 48.3(2) Keep the declaration node.

### § 48.4 Copy versus move

§ 48.4(1) Recovery must never replace:

```text
:=
:<-
=
<-
```

with another ownership operator silently.

---

## § 49 Type-first declaration recovery

§ 49(1) Type-first declaration recovery must distinguish:

```text
type reference
mut
colon
declarator list
```

§ 49(2) Speculative parsing must remain transactional.

§ 49(3) When the type-first interpretation fails and expression parsing is viable, the
parser may roll back without diagnostics.

§ 49(4) After committing at `mut` or `:`, errors produce an invalid declaration rather
than rolling back to expression parsing.

---

## § 50 Assignment recovery

§ 50(1) Preserve:

```text
target
operator spelling
right expression
```

### § 50.1 Missing right expression

§ 50.1(1) Create `InvalidExpression` as the source.

§ 50.1(2) Do not drop the assignment target.

### § 50.2 Assignment in expression position

§ 50.2(1) Emit:

```text
P2013 parser.invalid-assignment-expression
```

§ 50.2(2) Do not reinterpret it as only the left expression for compilation.

§ 50.2(3) The AST may retain an invalid assignment-expression node for tooling.

### § 50.3 Chained assignment

§ 50.3(1) Preserve the first assignment statement and attach an invalid right expression
or dedicated invalid chain node.

§ 50.3(2) Do not assign associativity.

---

## § 51 If recovery

### § 51.1 Missing condition

§ 51.1(1) Create an invalid condition expression.

§ 51.1(2) Preserve and parse the consequence block.

§ 51.1(3) The recovered `if` keeps an invalid condition expression, never a nil condition.

### § 51.2 Missing consequence brace

§ 51.2(1) When a block is clearly intended, create a virtual `{` only when a matching
body boundary can be established safely.

§ 51.2(2) Otherwise preserve an invalid if statement and synchronize at:

```text
else
next statement
}
EOF
```

### § 51.3 Else recovery

§ 51.3(1) An unmatched `else` remains an invalid statement.

§ 51.3(2) Its body should be consumed as part of that invalid statement.

§ 51.3(3) Do not expose its inner statements at the surrounding scope.

---

## § 52 For recovery

§ 52(1) Synchronize at:

```text
in
step
{
}
EOF
```

§ 52(2) Preserve bindings, iterable, step, and body where parsed.

§ 52(3) Unsupported forms receive focused diagnostics:

```text
condition-only for
C-style for
```

§ 52(4) Recovery may still parse their body.

§ 52(5) It must not compile them as canonical loops.

§ 52(6) The infinite loop `for { ... }` and numeric and range iteration are legal
(`rules/control-flow/flowcontrol_for.md`) and are never reported as unsupported
forms; "condition-only for" means `for condition { ... }`, whose canonical
spelling is `while`.

---

## § 53 While recovery

### § 53.1 Missing condition

§ 53.1(1) Use `InvalidExpression`.

§ 53.1(2) Preserve the body.

### § 53.2 Assignment condition

§ 53.2(1) Emit an error, not only a warning.

§ 53.2(2) Preserve the assignment-shaped invalid condition for tooling.

§ 53.2(3) Do not compile using only the left operand.

### § 53.3 Missing block

§ 53.3(1) Synchronize before the next statement or closing outer brace.

---

## § 54 Switch recovery

§ 54(1) The existing case/default synchronization is canonical in principle.

### § 54.1 Case item

§ 54.1(1) A malformed case item should create an invalid case item and continue at:

```text
,
:
case
default
}
EOF
```

### § 54.2 Missing colon

§ 54.2(1) When the case item is complete and a statement clearly follows, insert a virtual
colon.

### § 54.3 Duplicate default

§ 54.3(1) Preserve every default clause.

§ 54.3(2) Sema or parser validation reports duplicates with related locations.

### § 54.4 Default order

§ 54.4(1) A case after default is retained as source but diagnosed as invalid ordering.

---

## § 55 Match recovery

§ 55(1) Match-arm recovery is sibling-aware; it does not skip to the closing brace of
the `match`.

§ 55(2) Synchronize a malformed arm at:

```text
=>
next probable pattern at the same brace depth and a later line
}
EOF
```

§ 55(3) Use a dedicated pattern parser.

### § 55.1 Missing arrow

§ 55.1(1) When the pattern is complete and the next token begins an arm body, insert a
virtual `=>`.

### § 55.2 Invalid pattern

§ 55.2(1) Create `InvalidPattern`.

§ 55.2(2) Do not parse the pattern as an unrestricted valid expression.

### § 55.3 Invalid body

§ 55.3(1) Create `InvalidExpression` or invalid block body while preserving later arms.

### § 55.4 Error patterns

§ 55.4(1) For `Result`/error patterns recovery distinguishes an `Err` carrier pattern, a
concretely qualified error variant, a contextually shortened `.Variant` when
its enclosing error type is known, and an optional single shallow payload or
field-pattern binding (`rules/foundations/grammar.md` "Try expression",
`rules/errors/errorhandling.md` § 15.3, MD-029). These source forms are not
rejected merely because the general nested-pattern grammar remains closed. A
malformed payload pattern retains the outer `Err` arm or handler, its source
location and the following arms. A missing or ambiguous qualification is a
semantic or type-resolution concern unless the syntax itself is malformed. The
braces of a field pattern inside `Err(...)` are not a handler list, `match`
body or arm body.

### § 55.5 Exhaustiveness

§ 55.5(1) Exhaustiveness is Sema's responsibility.

§ 55.5(2) Invalid arms do not count as proof of exhaustiveness.

§ 55.5(3) Proof that a well-formed arm cannot execute does not make its syntax an invalid
pattern. Parser recovery never emits an unreachable-arm error from inferred
active variants. The semantic diagnostic layer distinguishes source-order full
coverage, which is invalid when the owning match rule says so, from a valid arm
excluded only by a proven subject state, which is Information under MD-042
(`rules/control-flow/flowcontrol_match.md` § 25A).

---

## § 56 Try-handler recovery

§ 56(1) Try handlers use the same pattern and arm concepts as match.

§ 56(2) Synchronize at:

```text
=>
next probable handler pattern at the same brace depth and a later line
}
next statement outside a missing closing brace
EOF
```

§ 56(3) Handler synchronization uses the shared grammar-derived statement-start
predicate, not a private start list.

§ 56(4) The error-pattern forms and their recovery follow § 55.

§ 56(5) A malformed first handler must not discard later handlers.

---

## § 57 Return recovery

§ 57(1) A return statement ends before:

```text
}
case
default
later-line statement start where no expression continuation exists
EOF
```

§ 57(2) A malformed return expression becomes `InvalidExpression`.

§ 57(3) Do not consume the next statement as part of the return.

---

## § 58 Defer recovery

§ 58(1) After `defer`, canonical forms are:

```text
block
return
```

§ 58(2) When neither appears:

- create an invalid defer statement;
- synchronize at next statement or closing brace;
- do not treat an arbitrary following statement as deferred.

---

## § 59 Discard recovery

§ 59(1) After `discard`, an expression is required.

§ 59(2) When absent:

- retain a `DiscardStatement` with `InvalidExpression`;
- stop before the closing brace or next statement;
- emit one focused diagnostic.

---

## § 60 Select recovery

§ 60(1) The existing branch synchronization is a foundation.

§ 60(2) Synchronize at:

```text
=>
after
default
next probable branch expression at same depth and later line
}
EOF
```

### § 60.1 Missing arrow

§ 60.1(1) When the branch operation is complete and a block follows, insert virtual `=>`.

### § 60.2 Invalid binding branch

§ 60.2(1) Preserve:

```text
binding name
:=
operation expression
body
```

### § 60.3 Default and timeout

§ 60.3(1) A malformed default or after branch must not discard later branches.

---

## § 61 Unsafe recovery

§ 61(1) A missing unsafe block produces an invalid unsafe statement.

§ 61(2) Do not apply unsafe context to following unrelated statements.

§ 61(3) The unsafe context begins only after a real or safely inserted opening brace and
ends at the corresponding real or virtual closing brace.

---

## § 62 Assembly recovery

§ 62(1) Structured asm recovery must synchronize at:

```text
inputs
outputs
clobbers
}
EOF
```

§ 62(2) Within a section, synchronize at:

```text
,
next line item
next section
}
EOF
```

§ 62(3) Preserve the raw template and successfully parsed operands.

§ 62(4) Do not let malformed asm consume the rest of the function.

---

## § 63 Expression recovery

§ 63(1) Expression recovery must be precedence-aware.

§ 63(2) Synchronization may stop at tokens that cannot continue the current expression:

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

§ 63(3) The parser must still respect nested delimiters.

### § 63.1 Missing prefix operand

§ 63.1(1) Example:

```sec
let value := -
```

§ 63.1(2) Create an invalid right operand.

### § 63.2 Missing infix right operand

§ 63.2(1) Example:

```sec
let value := left +
```

§ 63.2(2) Preserve:

```text
left
+
InvalidExpression
```

### § 63.3 Unexpected operator

§ 63.3(1) Example:

```sec
left + * right
```

§ 63.3(2) Do not invent the intended operator.

§ 63.3(3) Preserve the unexpected operator in the invalid expression.

### § 63.4 Grouped expression

§ 63.4(1) A missing `)` may be inserted before a clear enclosing boundary.

### § 63.5 Postfix expression

§ 63.5(1) Malformed call, index, slice, generic call, member access, or typed construction
should preserve the valid left expression and create an invalid postfix node.

---

## § 64 Member-access recovery

§ 64(1) Example:

```sec
value.
```

§ 64(2) Do not invent a member name.

§ 64(3) Create an invalid member expression and stop at the expression boundary.

§ 64(4) Example:

```sec
value..other
```

§ 64(5) Do not silently reinterpret range syntax as member access or vice versa.

---

## § 65 Index and slice recovery

§ 65(1) Inside brackets, synchronize at:

```text
]
,
range operator
outer expression boundary
EOF
```

§ 65(2) Preserve:

```text
base expression
opening bracket
start expression
range operator
end expression
closing token or missing token
```

§ 65(3) A missing index expression creates `InvalidExpression`.

§ 65(4) A missing range bound may be valid depending on slice grammar.

---

## § 66 Range recovery

§ 66(1) Ranges are contextual.

§ 66(2) When a range appears where no range is permitted, preserve an invalid expression
and emit the contextual range diagnostic.

§ 66(3) Do not create a first-class range value in Sec 0.1.

§ 66(4) A malformed range should not consume the next statement while searching for a
bound.

---

## § 67 Contextual `x` recovery

§ 67(1) Contextual matrix `x` is an active infix operator (`rules/foundations/operators.md`,
`rules/foundations/grammar.md`):

- it is parsed as an operator only in valid infix position;
- an ordinary identifier named `x` keeps its identifier meaning;
- a missing right operand creates an invalid infix expression that retains the
  left operand and the operator;
- recovery must not reclassify an identifier named `x` outside infix context.

---

## § 68 Contextual Option constructors

§ 68(1) A syntactically valid one-argument `Some(expression)` is not rejected at parse
time as an unknown function merely because no `Option` target type is
immediately available. Semantic analysis determines whether context or the
payload supplies a legal `Option[T]` and separately validates ownership,
borrowing and lifetime (`rules/types/types.md` "Contextual and self-inferred
`Some(value)`", MD-102). `None()` remains a syntactically recognizable but
invalid use of the absence value and receives the canonical focused diagnostic
(`S1157`, `rules/tooling/diagnostics.md` § 24(4f)).

---

## § 69 Reserved syntax

§ 69(1) Reserved and context-restricted spellings receive focused diagnostics, not
generic token failures (`rules/foundations/grammar.md` "Reserved syntax"):

- `?` is reserved and has no meaning in Sec 0.1. Recovery diagnoses it under
  the selected language version and does not assume ternary or Rust-style
  propagation semantics. This rulebook does not decide that a later language
  version may never give `?` a meaning; such syntax is governed by its own
  future rule.
- `free` is valid only as the `FreeDeclaration` impl member; elsewhere it is
  diagnosed in its grammar context.
- `require` is a general keyword and is not a valid identifier.

§ 69(2) Reserved syntax is not an unknown-token lexer failure.

§ 69(3) Preserve a dedicated invalid or reserved node where it helps tooling.

§ 69(4) Parser synchronization recognizes the active constructs `panic` and `assert`
(`rules/errors/panic.md`, with their focused assertion diagnostics), the legal
infinite `for`, the selected source attributes `@target` / `@when`, and
contextual matrix `x` according to their owning rules. A currently valid
construct is never treated as reserved merely because an earlier revision of
this rulebook predated it.

---

## § 70 Legacy and compatibility syntax

§ 70(1) Recognizable legacy Sec syntax is invalid unless an active canonical owner
explicitly defines a migration exception (`rules/foundations/grammar.md`
"Canonical syntax and legacy forms", MD-003). A recognized legacy form is not
made canonical by its recovery representation: it remains a parse or
validation error and blocks artifact publication.

§ 70(2) Legacy forms receive focused registered diagnostics where the intended
migration can be determined reliably, for example:

```text
P2023 parser.legacy-assigned-type
P2026 parser.prefix-sequence-type
P2027 parser.future-struct-declaration
P2028 parser.legacy-enum-colon-initializer
P2034 parser.legacy-try-match-wrapper
P2035 parser.removed-uppercase-foreign-qualifier
```

§ 70(3) The registry remains the authority for the complete list
(§ 10).

§ 70(4) The legacy `#target` directive is a compatibility form governed by
`rules/compiler/compiler.md` § 15; it is never advertised as canonical syntax.

§ 70(5) Legacy-form diagnostics or nodes record:

```text
source form
canonical replacement
safe-fix range
```

§ 70(6) Default formatting never converts invalid legacy syntax into canonical syntax.
A Language Correction (`rules/tooling/formatter.md` §§ 26–27) may rewrite a
legacy form only where the canonical migration contract proves the correction
unambiguous and meaning-preserving.

---

## § 71 Speculative parsing

§ 71(1) Speculative parsing must be transactional.

§ 71(2) The canonical transaction must include every mutable parser state:

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

### § 71.1 Commit

§ 71.1(1) A speculative branch commits only when its distinguishing grammar token is
confirmed.

§ 71.1(2) Examples:

```text
typed declaration colon
typed declaration mut marker
generic call followed by call parentheses
typed declaration group shape
```

### § 71.2 Rollback

§ 71.2(1) Rollback must leave no:

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

## § 72 Parser context stack

§ 72(1) The parser should maintain explicit recovery contexts.

§ 72(2) Conceptual values:

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

§ 72(3) Context drives:

```text
expected tokens
synchronization set
diagnostic wording
safe insertion choices
LSP completion
```

---

## § 73 Source ranges

§ 73(1) Canonical source positions should support:

```text
file
start byte or rune offset
end byte or rune offset
start line
start column
end line
end column
```

§ 73(2) The exact internal offset unit must be consistent.

§ 73(3) LSP conversion to UTF-16 positions happens at the LSP boundary.

§ 73(4) A virtual token has:

```text
start == end
```

at its insertion anchor.

§ 73(5) Skipped source has a real nonempty range.

---

## § 74 Token and trivia retention

§ 74(1) Recovery must preserve access to:

```text
all real tokens
comments
invalid lexer tokens
skipped tokens
virtual tokens
```

§ 74(2) This may be implemented through:

```text
token buffer
lossless syntax tree
AST plus recovery ranges and original source
```

§ 74(3) A fully lossless CST is not mandated for Sec 0.1.

§ 74(4) The chosen model must support formatter and LSP requirements.

---

## § 75 Lexer errors

§ 75(1) An `ILLEGAL` lexer token is not silently skipped.

§ 75(2) The parser should create a lexical diagnostic and preserve the token.

§ 75(3) Examples:

```text
unknown character
unterminated block comment
unterminated string
invalid character literal
```

§ 75(4) The lexer must always advance after emitting an illegal token.

§ 75(5) If it does not advance, the parser marks a fatal internal failure.

---

## § 76 Interaction with Sema

### § 76.1 Batch compilation

§ 76.1(1) When parser errors exist:

- compilation fails;
- code generation is not attempted;
- Sema may run only to provide additional independent diagnostics where safe.

### § 76.2 Tooling analysis

§ 76.2(1) For LSP, Sema may analyze valid subtrees.

§ 76.2(2) It must:

- skip invalid declarations when symbol identity is unavailable;
- assign `ErrorType` to invalid expressions and type references;
- suppress diagnostics caused only by `ErrorType`;
- preserve symbols from valid parts of a recovered declaration when stable;
- avoid ownership-state changes caused by invalid expressions;
- avoid treating virtual syntax as user-confirmed semantic intent.

### § 76.3 Invalid symbols

§ 76.3(1) An invalid declaration must not introduce a normal visible symbol unless its
name and declaration category are unambiguous.

§ 76.3(2) When introduced for tooling, mark it invalid so it cannot participate as a
normal overload or type candidate.

### § 76.4 Ownership

§ 76.4(1) Invalid syntax does not:

```text
move a value
borrow a value
destroy a value
allocate
call FFI
establish a resource state
```

§ 76.4(2) Ownership analysis may conservatively mark affected flow unknown for later
diagnostic suppression.

---

## § 77 Interaction with formatter

### § 77.1 Valid surrounding code

§ 77.1(1) The formatter may format valid regions surrounding an invalid node.

### § 77.2 Invalid region

§ 77.2(1) The ordinary formatter should preserve the original token spelling and order
inside an invalid region as much as possible.

§ 77.2(2) It must not print virtual tokens into source automatically.

### § 77.3 Explicit fixes

§ 77.3(1) A code action may insert a virtual token into source after user selection.

### § 77.4 No semantic guessing

§ 77.4(1) The formatter must not:

```text
invent an identifier
choose an operator
change copy to move
change move to copy
choose a type
delete a malformed declaration
```

### § 77.5 Stability

§ 77.5(1) Formatting a recovered file must not move diagnostics to unrelated source
locations unnecessarily.

---

## § 78 Interaction with LSP

§ 78(1) The LSP should use recovery contexts for:

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

### § 78.1 Completion

§ 78.1(1) Examples:

```sec
fn Add(left: |
```

§ 78.1(2) The context indicates a type is expected.

```sec
Point {
    |
}
```

§ 78.1(3) The context indicates a struct field or spread is expected.

```sec
match value {
    Pattern |
}
```

§ 78.1(4) The context may suggest `=>` and guards.

### § 78.2 Hover

§ 78.2(1) Hover is available on valid nodes inside a recovered declaration.

§ 78.2(2) No fabricated type should be shown for an invalid node.

### § 78.3 Document symbols

§ 78.3(1) A recovered function with a valid name should remain visible in the outline even
when its body is malformed.

### § 78.4 Semantic tokens

§ 78.4(1) Tokens retain lexical classification.

§ 78.4(2) Semantic classification is provided only where resolution is reliable.

### § 78.5 Diagnostics

§ 78.5(1) The LSP receives structured ranges and IDs.

§ 78.5(2) It must not parse line and column back out of message strings.

---

## § 79 Safe fixes

§ 79(1) A parser diagnostic may carry a machine-applicable fix only when intent is
unique.

§ 79(2) Typical safe fixes:

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

§ 79(3) Not automatically safe:

```text
invent identifier
invent expression
choose type
choose operator
choose copy or move
choose match variant
choose function body
```

§ 79(4) A diagnostic may offer a nonautomatic suggestion without a machine edit.

---

## § 80 Recovery quality rules

### § 80.1 Prefer local repair

§ 80.1(1) Try one local structural repair before region skipping.

### § 80.2 Prefer preserving siblings

§ 80.2(1) A repair preserving later list items or body members is better than abandoning
the complete parent.

### § 80.3 Prefer fewer assumptions

§ 80.3(1) Inserting a missing comma is safer than inventing an expression.

### § 80.4 Prefer grammar context

§ 80.4(1) Use the current grammar production rather than a generic token list.

### § 80.5 Prefer no cascade

§ 80.5(1) After repairing one missing delimiter, suppress errors that are only artifacts
of the same repair.

### § 80.6 Never prefer successful misparse

§ 80.6(1) A recovered parse that changes the apparent program is worse than an invalid
node.

---

## § 81 Recovery algorithm

§ 81(1) Sec 0.1 may use deterministic recursive-descent recovery.

§ 81(2) Recommended order at a failed expectation:

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

§ 81(3) A global minimum-cost parser repair algorithm is not required.

---

## § 82 Conformance requirements

§ 82(1) A conforming parser satisfies the following observable requirements. The
concrete tests, fixtures, fuzz campaigns and their status are tracked by the
governance entries named in the header.

### § 82.1 General invariants

§ 82.1(1) Recovery behavior is verified for:

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

### § 82.2 One-error tests

§ 82.2(1) For one local error, prefer exactly one primary parser diagnostic.

§ 82.2(2) Examples:

```text
missing struct field colon
missing parameter comma
missing case colon
missing match arrow
missing closing parenthesis
```

### § 82.3 Cascade tests

§ 82.3(1) Verify that one missing token does not produce unrelated errors on:

```text
next field
next parameter
next statement
next case
next arm
next declaration
```

### § 82.4 Progress tests

§ 82.4(1) Feed repeated malformed tokens:

```text
}}}}}
,,,,,
?????
++++
```

§ 82.4(2) The parser must terminate.

### § 82.5 Truncation tests

§ 82.5(1) Truncate valid files after every token.

§ 82.5(2) For every prefix:

- parser must not panic;
- parser must terminate;
- diagnostics must remain bounded;
- partial AST must remain internally valid.

### § 82.6 Mutation tests

§ 82.6(1) From valid fixtures:

```text
delete one token
duplicate one token
replace one delimiter
move one closing brace
insert one keyword
```

§ 82.6(2) Verify bounded diagnostics and later-node preservation.

### § 82.7 Robustness inputs

§ 82.7(1) The progress and termination requirements also hold for:

```text
lexer input
parser token streams
nested delimiters
long malformed lists
long malformed expressions
Unicode identifiers
unterminated comments and strings
```

### § 82.8 Tooling tests

§ 82.8(1) Verify completion and symbols in incomplete source.

§ 82.8(2) Examples:

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

## § 83 Implementation independence

§ 83(1) No implementation-language package name, struct layout, pass name,
optimization choice or current status is normative unless an owning compiler
architecture rule explicitly makes that fact an externally required interface.
The compiler may organize recovery internally in different ways provided the
observable contract and the progress and proof invariants of this rulebook are
preserved.

§ 83(2) Every parser implementation, including a self-hosted Sec parser, follows the
same recovery model and distinguishes:

```text
valid node
recovered partial node
unrecoverable branch failure
```

---

## § 84 Design summary

§ 84(1) Sec uses deterministic, context-specific parser recovery under the selected
language version.

§ 84(2) The parser returns a partial program after ordinary syntax errors.

§ 84(3) Parser errors block normal compilation and code generation.

§ 84(4) Tooling may analyze valid subtrees.

§ 84(5) Recovery never silently changes invalid source into another valid program.

§ 84(6) The parser may insert a unique missing structural token virtually.

§ 84(7) It does not invent identifiers, expressions, types, or operators.

§ 84(8) Recoverable malformed constructs produce invalid AST nodes rather than nil.

§ 84(9) Synchronization respects delimiter depth and grammar context and uses
grammar-derived start sets.

§ 84(10) One local syntax cause should normally produce one primary diagnostic with a
stable registered ID.

§ 84(11) Skipped tokens and virtual tokens remain distinguishable from real valid syntax.

§ 84(12) Speculative parsing is fully transactional.

§ 84(13) Sema uses an internal error type to suppress dependent cascades.

§ 84(14) The formatter preserves malformed regions and never makes a semantics-changing
repair.

§ 84(15) The LSP uses recovery context for completion, symbols, and safe fixes.

§ 84(16) Legacy syntax stays invalid, and currently valid constructs are never treated
as reserved.

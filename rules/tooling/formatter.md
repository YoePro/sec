# Formatter

## Status

This document is the canonical formatter rulebook for Sec. The former legacy
text rulebook has been replaced and is no longer canonical.

The formatter follows a gofmt-like philosophy:

- Sec has one canonical source style;
- formatting is deterministic;
- ordinary formatting is not user-configurable;
- equivalent source should converge to the same output;
- formatting must be safe to run repeatedly;
- the formatter, compiler, and language server must share implementation;
- safe source repair is separated from ordinary formatting.

This rulebook supersedes earlier statements that formatting always requires
fully valid syntax.

The formatter should format valid source and preserve useful formatting around
recoverable syntax errors.

For defaults and empty collections, ordinary formatting must:

- preserve explicit named-type `default` clauses;
- preserve the semantic source order of every `in [...]` list;
- preserve omitted struct fields rather than expanding them;
- format `list[T] {}` and `list[T, Capacity] {}` as empty collection literals,
  distinct from named-struct literals;
- never insert explicit default values.

Expansion of omitted fields or insertion/declaration of defaults belongs to an
explicit LSP/refactoring action. No special configurable default style is
introduced.

---

# Current implementation status

The repository contains a working initial shared formatter in:

```text
internal/formatter
```

The LSP calls this package for document formatting. The CLI and fix engine are
not integrated yet.

## Implemented

The current LSP formatter already implements:

- full-document formatting through `textDocument/formatting`;
- whole-document replacement edits;
- preservation of LF versus CRLF line endings;
- conversion of tabs in leading and ordinary source whitespace to four spaces;
- removal of trailing horizontal whitespace;
- exactly one final newline for non-empty formatted source;
- collapsing repeated blank lines;
- indentation based on braces, parentheses, and brackets;
- indentation of `switch` cases;
- indentation of `select` branches;
- indentation of grouped imports;
- indentation of parenthesized type-first declaration groups;
- indentation of nested blocks;
- single-line function-parameter comma spacing;
- single-line `let` declaration comma spacing;
- unambiguous `func` to `fn` normalization;
- canonical placement of an inline `@noCopy` attribute on its own line;
- preservation of ordinary identifiers and calls named `func`;
- format-on-save integration in the VS Code extension;
- tests for switch, select, grouped imports, declaration groups, function
  signatures, bootstrap lexer source, and `func` normalization.

## Partially implemented

The current formatter is partially implemented in these areas:

- comment preservation is primarily line-based rather than trivia-aware;
- indentation is inferred from source text rather than a lossless syntax tree;
- malformed and incomplete source can sometimes be formatted, but recovery is
  not systematic;
- function and declaration normalization is limited to selected single-line
  forms;
- the LSP emits one whole-document edit rather than minimal edits;
- grouped declarations are indented but not comprehensively aligned;
- line comments retain indentation but are not aligned into declaration
  columns;
- struct tags are preserved as source text but are not aligned;
- ordinary formatting and safe fixing do not yet share a structured edit model.

## Not implemented

The following are not yet implemented:

- a shared `internal/fixes` package;
- the complete `sec fmt` command model defined here;
- `sec fmt --fix`;
- `sec fmt --check`;
- recursive directory and project formatting;
- range formatting;
- on-type formatting;
- minimal text edits;
- an AST- and trivia-aware printer;
- stable formatting of all recoverable syntax;
- `x++` to `x += 1`;
- `x--` to `x -= 1`;
- missing-colon repair;
- struct-field column alignment;
- struct-tag column alignment;
- end-of-line comment alignment;
- declaration-table alignment;
- multiline width-based layout;
- canonical import sorting policy;
- structured formatter diagnostics;
- formatter-disable directives;
- formatter fuzz testing;
- formatter idempotence testing across the full grammar.

---

# Purpose

The formatter converts Sec source into the canonical source representation.

Its responsibilities include:

```text
indentation
spacing
line breaks
blank lines
delimiter placement
comment placement
comment alignment
struct tag alignment
canonical spelling of accepted noncanonical syntax
stable printing of recoverable syntax
```

The formatter is not:

```text
a type checker
a replacement for Sema
a general refactoring engine
a code-style configuration framework
an optimizer
an ownership inference engine
```

---

# Formatter layers

Sec distinguishes three source-transformation layers.

## Canonical formatting

Used by:

```text
sec fmt
LSP document formatting
LSP range formatting
LSP on-type formatting
format on save
```

Canonical formatting:

- does not change program semantics;
- does not require successful Sema;
- may use a tolerant syntax tree;
- preserves comments and documentation;
- preserves identifier spelling;
- preserves literal values;
- preserves string and raw-string contents;
- preserves copy versus move syntax;
- is deterministic and idempotent.

## Syntax normalization

Syntax normalization converts an accepted, unambiguous noncanonical spelling
into canonical Sec spelling.

It may run as part of ordinary formatting.

Initial normalization rules include:

```text
func -> fn
x++  -> x += 1
x--  -> x -= 1
```

Normalization is allowed only when the parser proves the intended construct.

## Safe fixing

Used by:

```text
sec fmt --fix
LSP quick fixes
LSP fix all
safe fixes on save
```

Safe fixing may repair invalid source when the compiler proves one unique,
machine-applicable correction.

Examples include:

```text
insert a missing parameter colon
replace invalid copy initialization with explicit move initialization
replace invalid copy assignment with explicit move assignment
insert an unambiguous missing comma
insert one canonical import when exactly one module provides the symbol
rewrite a proven reversed type declaration from `type kind Name` to
    `type Name kind`
```

For the contextual word `register`, the fix requires a proving register shape
such as `type register Name[Width]`. It must not rewrite an ordinary identifier
named `register` when that shape is absent.

Safe fixing runs before canonical formatting.

---

# Shared implementation

The canonical implementation must live in reusable packages.

Target structure:

```text
internal/formatter/
internal/fixes/
```

The following must call the same formatter:

```text
sec fmt
sec fmt --check
sec fmt --stdin
LSP document formatting
LSP range formatting
LSP on-type formatting
refactoring output
generated source output
```

The following must call the same fix engine:

```text
sec fmt --fix
LSP quick fix
LSP fix all
safe fixes on save
```

No formatting rules may be duplicated in:

```text
cmd/lsp
the VS Code extension
another editor extension
a refactoring implementation
a code generator
```

---

# Command model

Required CLI forms:

```text
sec fmt <path>
sec fmt --check <path>
sec fmt --stdin
sec fmt --fix <path>
sec fmt --fix --check <path>
```

A path may identify:

```text
a `.sec` file
a directory
a project root
```

## `sec fmt <path>`

Formats source and rewrites changed files in place.

It applies:

```text
canonical formatting
syntax normalization
```

It does not apply semantic fixes.

## `sec fmt --check <path>`

Does not modify files.

It exits non-zero when any selected source file is not canonical.

It reports affected files.

## `sec fmt --stdin`

Reads Sec source from standard input and writes formatted source to standard
output.

A virtual filename may be supplied later for module and diagnostic context.

## `sec fmt --fix <path>`

Applies:

```text
safe machine-proven fixes
canonical formatting
```

It must report applied fixes.

It must not apply code cleanup or discretionary refactoring.

## `sec fmt --fix --check <path>`

Does not modify files.

It exits non-zero when safe fixes or canonical formatting would change source.

---

# File selection

Directory and project formatting includes:

```text
*.sec
```

Legacy `.se` support may remain while the extension is supported by the project.

The formatter must ignore:

```text
build outputs
vendor or dependency caches
generated files marked read-only
directories excluded by project configuration
```

Generated files may opt into formatting through explicit metadata.

---

# Canonical configuration

Ordinary formatting is not configurable.

The following are language decisions, not per-project preferences:

```text
indent width
brace placement
operator spacing
comma placement
comment spacing
alignment rules
canonical keyword spelling
final newline
```

Project configuration may control only operational behavior such as:

```text
which files are selected
whether generated files are included
whether safe fixes run on save
whether code cleanup runs on save
```

---

# Source model

The final formatter must operate on a lossless, error-tolerant syntax model.

It must retain:

```text
tokens
whitespace trivia
line comments
block comments
documentation comments
raw strings
source ranges
missing-token recovery nodes
error nodes
```

The AST alone is not sufficient when it does not preserve all comments and
trivia.

A temporary line-based implementation may remain during migration, but the
canonical architecture is syntax-tree and trivia aware.

---

# General invariants

Formatted source must satisfy:

```text
deterministic output
idempotence
semantic preservation
comment preservation
literal preservation
stable line endings
exactly one final newline
no trailing whitespace
no emitted tabs
```

Formally:

```text
Format(Format(source)) == Format(source)
```

For valid source:

```text
Parse(source) and Parse(Format(source))
```

must have equivalent language semantics.

---

# Line endings

The formatter preserves the file's established line-ending style when it is
consistent:

```text
LF
CRLF
```

Mixed line endings are normalized to the dominant style.

When no dominant style exists, use LF.

`sec fmt --stdin` uses LF unless an explicit line-ending mode is later provided.

A formatted non-empty file ends with exactly one line ending.

An empty file remains empty or ends with one newline according to the final CLI
policy; the implementation must use one deterministic rule.

---

# Indentation

Indentation uses four spaces.

Tabs are never emitted for indentation or alignment.

Indentation increases inside multiline:

```text
blocks
parenthesized groups
bracketed groups
multiline argument lists
multiline parameter lists
multiline literals
multiline declaration groups
```

Delimiter characters inside:

```text
strings
raw strings
character literals
comments
```

do not affect indentation.

Example:

```sec
fn main() int {
    if ready {
        return 0
    }

    return 1
}
```

---

# Blank lines

The formatter emits no repeated empty lines.

Two or more consecutive empty lines become one.

Blank lines normally separate:

```text
target directives from module
module from imports
import groups from declarations
top-level declarations
functions
major logical statement groups when a blank line already exists
```

No blank line is emitted:

```text
immediately after an opening brace
immediately before a closing brace
between a documentation comment and its declaration
between a standalone attached comment and its declaration
between `}` and `else`
between `}` and another required continuation
```

The formatter preserves a single intentional blank line inside a function when
it separates logical sections.

It does not invent many blank lines based on semantic analysis.

---

# Braces

Opening braces remain on the same line as the construct they belong to.

Canonical:

```sec
if ready {
}
```

Not canonical:

```sec
if ready
{
}
```

This applies to:

```text
functions
structs
unions
enums
interfaces
impl blocks
properties
getters
setters
unsafe blocks
asm blocks
if
else
for
while
switch
select
match
try handlers
struct literals
```

`else` remains on the same line as the preceding closing brace:

```sec
if ready {
    Run()
} else {
    Stop()
}
```

---

# General spacing

No space before:

```text
,
:
)
]
}
.
```

One space after:

```text
,
:
```

except where alignment or a compact grammar form defines a specific rule.

Binary and assignment operators have one space on both sides:

```sec
a + b
a == b
a && b
value := 10
value :<- source
value = other
value <- source
value += 1
left x right
```

Unary operators have no following space:

```sec
!enabled
-value
```

Member access has no spaces:

```sec
value.field
Vehicle.FuelType.diesel
```

Function calls have no space before `(`:

```sec
Add(1, 2)
Color(1)
```

Function declarations have no space between name and parameter list:

```sec
fn Add(a: int, b: int) int {
}
```

---

# Ownership tokens

Canonical move initialization:

```sec
let destination :<- source
```

Canonical typed move initialization:

```sec
let destination: Buffer <- source
```

Canonical move assignment:

```sec
destination <- source
```

Rules:

- one space before and after `:<-`;
- one space before and after `<-`;
- `:<-` remains one token;
- ordinary formatting preserves `:=` versus `:<-`;
- ordinary formatting preserves `=` versus `<-`;
- ordinary formatting never infers ownership transfer.

The formatter must never change:

```sec
let destination := source
```

to:

```sec
let destination :<- source
```

based on type information.

That correction belongs to the safe fix engine.

The formatter must never change:

```sec
let value := CreateBuffer()
```

to:

```sec
let value :<- CreateBuffer()
```

The first form is canonical direct initialization from a temporary.

---

# Accepted syntax normalization

The formatter may normalize accepted noncanonical source only when the parser
identifies the intended construct exactly.

## Function keyword

Input:

```sec
func Run() void {
}
```

Output:

```sec
fn Run() void {
}
```

The normalization applies only to a complete, body-bearing function declaration
or another parser-confirmed function declaration form.

The formatter must leave these unchanged:

```sec
func(callback)
let func := callback
object.func()
```

unless separate language rules reject them.

## Increment

Input:

```sec
count++
```

Output:

```sec
count += 1
```

## Decrement

Input:

```sec
count--
```

Output:

```sec
count -= 1
```

`++` and `--` are statement-only accepted aliases.

They do not return a value.

Invalid:

```sec
let old := count++
```

The formatter must not convert invalid expression use into a different
expression.

The canonical compound assignment remains subject to:

```text
mutability
type checking
contracts
fallible assignment
required `try`
```

The normalization does not bypass those rules.

---

# Future normalization registry

The implementation should maintain an explicit registry of accepted
noncanonical spellings.

Each entry records:

```text
source pattern
canonical pattern
parser confidence requirement
whether ordinary formatting may apply it
whether `--fix` is required
diagnostic ID
tests
```

Candidate future normalizations must not be enabled merely because they are
common in another language.

Examples requiring separate decisions include:

```text
function -> fn
def -> fn
var -> let mut
const -> let
elif -> else if
elseif -> else if
C-style array spelling
semicolon removal
and/or/not aliases
nil/null aliases
```

No candidate is canonical until its own language decision is recorded.

---

# Safe syntax fixes

Safe syntax fixes repair invalid source through the shared fix engine.

They are not ordinary formatting.

## Missing parameter colon

Input:

```sec
fn Parse(value string) Token {
}
```

Diagnostic:

```text
expected `:` between parameter name `value` and type `string`
```

Safe result:

```sec
fn Parse(value: string) Token {
}
```

## Missing typed-binding colon

Input:

```sec
let value int := 1
```

When the parser proves the declaration intent, a safe fix may produce:

```sec
let value: int := 1
```

This rule must not run where the token sequence has another valid meaning.

## Declaration assignment token

Input:

```sec
let value = 1
```

When declaration intent is exact and no alternative grammar applies, a safe fix
may produce:

```sec
let value := 1
```

The same applies to:

```sec
let mut value = 1
```

becoming:

```sec
let mut value := 1
```

This is a `--fix` operation, not ordinary formatting.

## Missing comma

A missing comma may be inserted in:

```text
struct fields
enum values
multiline declaration tables
multiline literals
multiline arguments
```

only when the recovery tree proves the intended item boundary.

## Move correction

Input:

```sec
let destination := source
```

where the resolved copy classification does not permit copying `source` and an
explicit ownership transfer is legal.

Safe fix:

```sec
let destination :<- source
```

The fix is automatically safe only when:

- move is legal;
- no later source use becomes invalid;
- no borrow conflict exists;
- no overload changes;
- no conversion changes;
- source and destination do not conflict.

Otherwise it is an explicit refactoring with consequence preview.

---

# Comments

Sec supports:

```text
// line comments
/* block comments */
/** documentation comments */
```

The formatter preserves comment text.

It may change only:

```text
indentation
surrounding whitespace
alignment
canonical documentation-comment framing
```

It must not reflow ordinary comment prose by default.

---

# Comment attachment

A comment is attached according to source position and blank lines.

## Leading comment

A standalone comment group immediately before a declaration or statement,
without an empty line, belongs to that declaration or statement.

```sec
// Opens the input file.
let file := OpenFile()
```

## Trailing comment

A line comment after code belongs to that source item.

```sec
let retryCount := 3  // Maximum retry attempts.
```

## Detached comment

A blank line separates a comment group from following code.

Detached comments retain their relative source position.

## Documentation comment

A `/** ... */` comment belongs to the declaration immediately following it.

No blank line is emitted between them.

---

# Line comments

Standalone line comments use the same indentation as surrounding source.

Example:

```sec
fn main() void {
    // Prepare output.
    let message := "hello"

    // Print output.
    fmt.println(message)
}
```

A trailing line comment has at least two spaces before `//`:

```sec
let value := 10  // Explanation.
```

Within an alignment group, trailing comments align to one column.

---

# Block comments

Single-line block comments remain single-line when their text fits:

```sec
/* explanation */
```

Multiline ordinary block comments preserve text and line structure.

Their outer indentation follows the containing construct.

The formatter must not convert ordinary block comments into documentation
comments.

---

# Documentation comments

Canonical form:

```sec
/**
 * Returns true when the value is positive.
 *
 * @param value value to inspect
 * @return true when value is greater than zero
 */
fn IsPositive(value: int) bool {
    return value > 0
}
```

Rules:

- documentation comments use `/** ... */`;
- each interior line begins with the current indentation, `*`, and optional
  text;
- the closing `*/` aligns with the opening `/**`;
- no blank line separates documentation from its declaration;
- comment text and tags are preserved;
- ordinary formatting does not reorder documentation tags.

Accepted documentation tag names are defined by the documentation rulebook.

---

# Alignment

Alignment is part of canonical formatting for declaration-like consecutive
groups.

The formatter uses spaces, never tabs.

Alignment must remain local and predictable.

It must not create enormous whitespace because one unrelated line is very long.

---

# Alignment groups

An alignment group consists of consecutive compatible single-line items.

A group ends at:

```text
a blank line
a standalone comment
a documentation comment
a multiline item
a preprocessor or target boundary
a different declaration shape
a nested block boundary
```

A trailing comment does not end the group.

A struct field without a tag may remain in the same group as tagged fields.

---

# Struct field formatting

Struct fields are one per line.

Every field ends with a comma.

Basic canonical form:

```sec
type User struct {
    active: bool,
    name: string,
    age: Age,
}
```

Within one alignment group, field names and field type starts align.

Example:

```sec
type User struct {
    ID:       int,
    Name:     string,
    Password: string,
}
```

The colon remains immediately after the field name.

Spaces after the colon align the type column.

---

# Struct tags

Struct tags use raw-string syntax after the complete field type and before the
field comma.

Example:

```sec
type User struct {
    ID:       int    `json:"id" xml:"id"`,
    Name:     string `json:"name" xml:"name"`,
    Password: string `json:"-"`,
}
```

Within one field group:

- tag starts align to one column;
- fields without tags reserve the tag column only when needed to align trailing
  comments;
- tag contents are preserved exactly;
- tag key order is preserved;
- tag values are preserved;
- ordinary formatting does not invent tags;
- ordinary formatting does not sort tags.

Example with one untagged field:

```sec
type User struct {
    ID:        int    `json:"id"`,
    Name:      string `json:"name"`,
    CacheOnly: bool,
}
```

No trailing whitespace is emitted on the untagged field.

---

# Struct trailing comments

Trailing line comments on consecutive struct fields align.

Canonical example:

```sec
type User struct {
    ID:       int    `json:"id"`,    // Stable database identifier.
    Name:     string `json:"name"`,  // Display name.
    Password: string `json:"-"`,     // Never serialize.
}
```

The formatter aligns these conceptual columns:

```text
field name and colon
type
struct tag
comma
trailing comment
```

The comma remains part of the field syntax and occurs before the trailing
comment.

When a field has no tag:

```sec
type User struct {
    ID:      int    `json:"id"`,  // Stable identifier.
    Enabled: bool,                // Runtime state only.
}
```

The trailing comments still align.

---

# Struct alignment limits

Alignment is not applied across:

```text
blank lines
documentation comments
standalone comments
multiline field types
multiline tags
conditional source boundaries
```

Example:

```sec
type Config struct {
    ID:   int,     // Identity.
    Name: string,  // Display name.

    // Network configuration.
    Endpoint: string `json:"endpoint"`,
    Timeout:  int    `json:"timeout"`,
}
```

The two sections form separate alignment groups.

If one field is multiline, it is formatted independently.

The implementation may define a maximum alignment expansion to avoid
pathological whitespace, but that threshold is language-defined and not
user-configurable.

Until such a threshold is chosen, align the complete local group.

---

# Enum alignment

Enum values remain one per line and end with commas.

Simple values:

```sec
enum Direction {
    north,
    east,
    south,
    west,
}
```

Initialized values may align assignment operators within one group:

```sec
enum Permission uint {
    none    = 0,
    read    = 1 << iota,
    write   = 1 << iota,
    execute = 1 << iota,
}
```

Trailing comments may align:

```sec
enum Status int {
    New      = 0,   // Created but not processed.
    Invoiced = 1,   // Invoice generated.
    Paid     = 10,  // Payment completed.
}
```

Alignment must preserve enum order.

---

# Typed declaration groups

Parenthesized type-first declaration groups are formatted as:

```sec
TokenType (
    ILLEGAL := "ILLEGAL",
    EOF     := "EOF",
    IDENT   := "IDENT",
    INT     := "INT",
)
```

Within one group:

- names align;
- initialization operators align;
- initializer starts align;
- commas remain;
- trailing comments align when present.

Example:

```sec
TokenType (
    ILLEGAL := "ILLEGAL",  // Invalid token.
    EOF     := "EOF",      // End of input.
    IDENT   := "IDENT",    // Identifier.
)
```

The formatter preserves declaration order.

Sorting is a separate code action.

---

# Type-first mutable declarations

Canonical single-line form:

```sec
Car mut: Audi, Saab, Volvo, Skoda
```

The formatter may wrap a long declaration according to future line-width rules.

It does not automatically merge separate declarations into this form.

Merging is code cleanup or refactoring.

---

# General trailing-comment alignment

Trailing comments may align in compatible local groups such as:

```text
struct fields
enum values
typed declaration groups
register fields
simple consecutive variable declarations
```

They do not align across unrelated statements merely because they are adjacent.

Example not automatically aligned as one group:

```sec
let input := Read()
Process(input)  // Performs validation.
```

---

# Imports

Single imports are one per line:

```sec
import "fmt"
import "io"
```

Grouped imports:

```sec
import (
    "fmt"
    sys "platform/linux/amd64"
)
```

Each grouped entry is indented four spaces.

Import order is preserved by ordinary formatting until an explicit canonical
sorting rule is approved.

`Organize imports` is a code action and code-cleanup operation.

It may:

```text
remove unused imports
merge compatible groups
sort according to the approved policy
add required imports
```

Ordinary formatting does not remove or reorder imports.

---

# Top-level layout

The formatter preserves top-level declaration order.

Typical layout:

```sec
#target(os: "linux", arch: "amd64")

module main

import "fmt"

type Percent int range 0..100

fn main() int {
    return 0
}
```

One primary public type per file is a code-quality recommendation, not a
formatter transformation.

Moving declarations to files is a refactoring.

---

# Functions

Canonical:

```sec
fn Add(a: int, b: int) int {
    return a + b
}
```

Unsafe function:

```sec
unsafe fn _rawSyscall3(number: uint, arg1: uint, arg2: uint, arg3: uint) int {
    asm {
        "syscall"
    }
}
```

Parameters use:

```text
name: Type
```

Single-line parameter lists use comma and one space.

Single-line signatures do not have a trailing parameter comma.

---

# Multiline function parameters

Once a signature is broken across lines, use one parameter per line:

```sec
fn CreateUser(
    name: string,
    email: string,
    enabled: bool,
) Result[User, CreateError] {
}
```

Multiline parameter lists use a trailing comma.

The closing `)` aligns with the start of `fn`.

Width-based breaking belongs to the line-width section.

---

# Calls

Short call:

```sec
CreateUser(name, email, true)
```

Multiline call:

```sec
CreateUser(
    name,
    email,
    true,
)
```

Multiline argument lists use one argument per line and trailing commas.

The formatter must preserve evaluation order.

---

# Variables

Canonical inferred declarations:

```sec
let value := 10
let mut value := 10
```

Canonical typed declarations:

```sec
let value: int := 10
let mut value: int
let mut value: int := 10
```

Multiple declarations:

```sec
let a := 1, b := "hello", c := true
let mut a := 1, b := "hello", c := false
```

Type-first declarations:

```sec
int mut: a, b, c
float: a := 5.4, pi := 3.14
```

Move declarations:

```sec
let destination :<- source
let destination: Buffer <- source
```

---

# Types

Named type:

```sec
type Percent int range 0..100
```

Unit-bearing type:

```sec
type Money decimal<SEK>
```

Generic type:

```sec
type Box[T] struct {
    value: T,
}
```

The formatter does not change type meaning or normalize unit expressions beyond
operator and delimiter spacing.

Unit declarations preserve the optional default numeric carrier and category;
the formatter does not insert redundant `decimal`. Compiler-known unit metadata
uses canonical PascalCase:

```text
LongName Symbol BaseUnit Status Dimension Kind Scale System Transform Offset
Origin LogBase LogFactor Reference
```

Dimension vectors use compact exponent notation with ordinary comma spacing,
for example `[length^1, time^-1]`. Structural annotations use compact operator
spacing such as `<kg*m/s^2>` while preserving source factor order and required
parentheses.

Formatting never replaces a named unit with a structural expression or the
reverse, reorders factors for semantic canonicalization, changes a carrier, or
invents/removes a conversion.

---

# Struct literals

Short struct literals may remain on one line when concise:

```sec
let point := Point { start: 0, size: 1 }
```

The exact space between type name and `{` follows the canonical struct-literal
grammar. Once locked, the parser and formatter must use one spelling
consistently.

Multiline form:

```sec
let user := User {
    ID: 1,
    Name: "Ada",
}
```

Multiline fields end with commas.

Struct literal field order is preserved.

---

# Impl blocks and properties

Canonical:

```sec
impl Vehicle {
    property TopSpeed: Speed {
        get {
            return _speed
        }

        set value {
            _speed = value
        }
    }
}
```

Nested types and enums follow their ordinary formatting rules.

No unnecessary `self` parameter is inserted.

Lifecycle members and construction retain their distinct canonical forms:

```sec
impl Buffer {
    init(size: uint) AllocationError {
    }

    free {
    }
}

let buffer := try new Buffer(4096)
```

`init` is formatted without `fn`; its trailing type is not described as a
return type. The formatter must never rewrite `Type(value)` to `new Type(value)`
or the reverse, and must not imply heap allocation. Nested impls use ordinary
impl-block indentation. Explicit receiver parameters are not canonical output.

---

# Control flow

## If

```sec
if value {
    return 1
} else {
    return 0
}
```

Optional redundant outer parentheses may be removed:

```sec
if (value) {
}
```

becomes:

```sec
if value {
}
```

only when precedence and readability remain clear.

## For

```sec
for i in 0..10 {
    continue
}
```

Infinite loop:

```sec
for {
    Run()
}
```

## While

```sec
while running {
    Run()
}
```

## Switch

```sec
switch value {
    case 0:
        return 0
    case 1:
        return 1
    default:
        return -1
}
```

Cases are indented one level from `switch`.

Case bodies are indented one additional level.

## Select

```sec
select {
    value := receiver.Receive() => {
        Use(value)
    }
    after timeout => {
        return
    }
    default => {
        return
    }
}
```

## Match

Expression form:

```sec
return match result {
    Ok(value) => value
    Err(error) => 0
}
```

Block form:

```sec
match result {
    Ok(value) => {
        Use(value)
    }
    Err(error) => {
        return
    }
}
```

---

# `discard`

Canonical:

```sec
discard value
discard Calculate()
```

Exactly one space follows `discard`.

The formatter preserves the keyword, formats the operand using ordinary
expression rules, and keeps `discard` as a statement rather than a function
call. It must preserve evaluation order and ownership semantics.

The formatter does not insert explicit `discard` for ordinary implicit call
results and does not remove an explicit `discard` from source.

Diagnostic configuration must not make an otherwise valid file fail to format
or change canonical formatting. Insertion of explicit discard is a separately
requested semantic code action, not an unconditional formatter rewrite.

---

# Contextual `x`

The matrix multiplication operator is formatted as a binary operator:

```sec
let result := left x right
```

An identifier named `x` is formatted as an ordinary identifier:

```sec
let x := 10
Use(x)
```

The formatter relies on parser context.

It must not rewrite identifier `x` as an operator or vice versa.

---

# Contextual `set`

The contextual spelling `set` is preserved according to parser context.

Type use:

```sec
let values: set[int]
```

Property setter:

```sec
set value {
}
```

The formatter does not treat ordinary invalid attempts to declare a symbol named
`set` as a formatting problem.

---

# Operators and parentheses

The formatter follows the canonical operator precedence table.

It may remove redundant outer parentheses only when:

- meaning is unchanged;
- parser recovery is not involved;
- readability is not reduced.

It must preserve parentheses that affect precedence:

```sec
(1 + 2) * 3
```

It should preserve clarifying parentheses around mixed boolean and comparison
expressions when removal would reduce readability.

---

# Line width

The first formatter implementation may avoid forced width-based wrapping.

The target model should use a soft width, not a hard syntax limit.

A future canonical soft target may be:

```text
100 or 120 columns
```

The exact value requires a separate language decision.

Until then:

- preserve already sensible multiline structure;
- break only constructs with canonical multiline forms;
- never truncate or reflow string content;
- never force trailing comments into unreadable columns merely to align them.

Alignment and line width must cooperate.

When aligned trailing comments would exceed the future soft target, the
formatter may place the comment on the preceding line or keep a minimal
two-space separation according to a future exact rule.

---

# Incomplete and recoverable source

The formatter should format unaffected structure even when source contains
recoverable errors.

Examples:

```text
missing parameter colon
missing comma
unfinished expression
unfinished member access
unfinished function body
```

Rules:

- never delete unknown tokens;
- never invent a semantic expression without a compiler fix;
- preserve error-node text;
- format surrounding valid blocks;
- apply syntax normalization only when the recovered construct is unambiguous;
- require `--fix` for inserted missing syntax.

---

# Range formatting

LSP range formatting must use the shared formatter.

The formatter expands the requested range to complete safe syntax boundaries.

Possible expansion units:

```text
statement
declaration
comment group
parameter list
argument list
struct field group
block
```

Range formatting must not return edits outside the expanded range except when
required to maintain a syntactically complete attached comment or delimiter.

---

# On-type formatting

Initial trigger candidates:

```text
}
)
]
,
:
newline
```

On-type formatting should be conservative and fast.

It may:

```text
indent the current line
align a completed local group
place `else`
format a completed field
format a completed case
```

It must not run semantic fixes unless the user enabled inline safe fixes.

---

# Minimal edits

The shared formatter should be able to produce:

```text
full formatted text
minimal text edits
```

CLI may rewrite the file.

LSP should prefer minimal, non-overlapping edits when practical.

Edits must use the client's negotiated position encoding.

---

# Generated code

Generated Sec code must be canonical.

Generators should construct syntax or structured source and call the shared
formatter.

They must not embed a separate style printer.

A generated-file marker may disable manual refactoring while still allowing
formatting.

---

# Formatter directives

Sec 0.1 does not require formatter-disable directives.

A future design may support narrowly scoped directives such as:

```text
format off
format on
```

Only if concrete cases require them.

Directives must not become a general escape from canonical style.

---

# Diagnostics

Formatter and fix diagnostics require stable IDs.

Suggested rules:

```text
format.noncanonical-source
format.unrecoverable-region
format.unsafe-fix-refused
format.ambiguous-normalization
format.generated-file-read-only
```

Syntax fixes retain their parser or Sema diagnostic IDs.

`sec fmt --check` should report files and optionally first differing ranges
without pretending formatting differences are language errors.

---

# Safety classification

Every transformation is classified as:

```text
Formatting
SyntaxNormalization
SafeFix
CodeCleanup
StructuralRefactoring
BehaviorChanging
PotentiallyLossy
```

Ordinary formatter accepts only:

```text
Formatting
SyntaxNormalization
```

`--fix` accepts:

```text
Formatting
SyntaxNormalization
SafeFix
```

No automatic mode accepts:

```text
BehaviorChanging
PotentiallyLossy
```

---

# Better code analysis boundary

The formatter does not automatically perform improvements such as:

```text
merge adjacent declarations
split files by primary type
extract function
rename symbols
sort declaration tables
convert move to borrow
remove an apparently unnecessary temporary
```

Those are diagnostics, code cleanup, or refactoring.

Example:

```sec
let mut Audi: Car
let mut Saab: Car
let mut Volvo: Car
let mut Skoda: Car
```

may receive a refactoring to:

```sec
Car mut: Audi, Saab, Volvo, Skoda
```

Ordinary formatting preserves the original declaration structure.

---

# Tests

## Golden tests

Every syntax construct requires input and expected-output files.

Include:

```text
valid canonical source
valid noncanonical source
recoverable source
comments
raw strings
Unicode identifiers
line endings
nested constructs
```

## Idempotence

Every formatter test must verify:

```text
Format(output) == output
```

## Semantic preservation

For valid source, compare canonical AST or semantic representation before and
after formatting.

## Comment preservation

Verify:

```text
comment count
comment text
attachment
relative order
documentation ownership
```

## Struct alignment

Required tests:

```sec
type User struct {
ID:int `json:"id"`,// Stable identifier.
Name:string `json:"name"`, // Display name.
Password:string `json:"-"`,// Never serialize.
}
```

Expected:

```sec
type User struct {
    ID:       int    `json:"id"`,    // Stable identifier.
    Name:     string `json:"name"`,  // Display name.
    Password: string `json:"-"`,     // Never serialize.
}
```

Test fields with:

```text
no tag
one tag
multiple tags
no comment
trailing comment
blank-line group break
standalone comment group break
documentation comment
long type
multiline type
```

## Typed declaration alignment

Input:

```sec
TokenType (
ILLEGAL:="ILLEGAL",
EOF:="EOF",
IDENT:="IDENT",
)
```

Expected:

```sec
TokenType (
    ILLEGAL := "ILLEGAL",
    EOF     := "EOF",
    IDENT   := "IDENT",
)
```

## Normalization tests

Test:

```text
func declaration -> fn
func call remains func
x++ -> x += 1
x-- -> x -= 1
increment expression remains diagnostic
```

## Ownership tests

Verify ordinary formatting preserves:

```text
:=
:<-
=
<-
```

Verify `--fix` changes copy syntax only through a proven diagnostic fix.

## Fuzzing

Fuzz:

```text
lexer token streams
valid syntax trees
recoverable source
comments
raw strings
Unicode
nested delimiters
```

The formatter must not panic.

---

# Required synchronization

This rulebook must remain synchronized with:

```text
lsp.md
ownership.md
copy_move.md
lexical_structure.md
grammar.md
operators.md
types.md
struct.md
enum rules
register rules
comments and documentation rules
imports and modules rules
diagnostics.txt
compiler_pipeline.txt
parser recovery rules
language-rulebook-status.md
rules_implementations.txt
```

---

# Appendix A — Codex implementation plan

## A.1 Rename the rulebook

The filename migration is complete. `rules/tooling/formatter.md` is canonical,
repository references are updated, and no duplicate canonical file remains.

## A.2 Preserve current tests

Before moving code, preserve the formatter tests currently located in:

```text
cmd/lsp/main_test.go
```

Move or duplicate them into the shared formatter test package before deleting
LSP-local functions.

## A.3 Create shared formatter package

Create:

```text
internal/formatter
```

Initial API:

```go
type Options struct {
    Fix bool
}

type Result struct {
    Text        string
    Edits       []TextEdit
    Diagnostics []diagnostics.Diagnostic
}

func Format(source Source, options Options) Result
func FormatRange(source Source, target SourceRange, options Options) Result
```

Exact Go types may follow existing compiler source abstractions.

## A.4 Move current implementation

Move these responsibilities from `cmd/lsp`:

```text
formatSource
function signature normalization
let declaration normalization
indentation helpers
branch indentation helpers
comma splitting helpers
func normalization
```

The LSP must call the shared package.

## A.5 Build lossless syntax and trivia support

Extend lexer/parser infrastructure or add a formatter syntax layer retaining:

```text
all tokens
comments
whitespace trivia
missing tokens
error nodes
source ranges
```

Do not infer comment attachment from trimmed lines in the final implementation.

## A.6 Add alignment engine

Implement a generic local alignment engine.

Inputs:

```text
alignment group
column cells
minimum spacing
group boundaries
```

Initial clients:

```text
struct fields
struct tags
trailing comments
enum initializers
typed declaration groups
register fields
```

Use spaces only.

## A.7 Implement struct alignment

Parse each single-line field into cells:

```text
nameColon
typeAndContracts
tag
comma
comment
```

Align compatible fields.

Preserve tag raw text exactly.

Preserve comment text exactly.

Do not align across group boundaries.

## A.8 Add ownership tokens

Update lexer and formatter for:

```text
:<-
<-
```

Preserve ordinary versus move syntax.

## A.9 Add increment/decrement normalization

Recognize parser-confirmed statement-only:

```text
postfix ++
postfix --
```

Print as:

```text
+= 1
-= 1
```

Do not add increment/decrement expression semantics.

## A.10 Shared fix engine

Create or use:

```text
internal/fixes
```

Formatter with `Fix: true` applies only diagnostics marked:

```text
machine applicable
safe
unambiguous
```

Then formats the result.

## A.11 Implement missing-colon fix

Add structured parser recovery and fix data for:

```sec
fn Parse(value string) Token {
}
```

The fix inserts exactly one colon at the compiler-provided position.

## A.12 CLI integration

Implement:

```text
sec fmt
sec fmt --check
sec fmt --stdin
sec fmt --fix
```

Ensure exit codes are documented and tested.

## A.13 LSP integration

Replace LSP-local formatting with calls to the shared formatter.

Implement:

```text
document formatting
range formatting
on-type formatting
safe fixes on save
```

## A.14 Update VS Code extension

Keep formatting settings thin.

The extension must not implement alignment or syntax normalization.

Expose separate settings for:

```text
format on save
safe fixes on save
code cleanup on save
inline safe fixes
```

## A.15 Update implementation tracker

Record:

```text
current LSP formatter behavior implemented
shared formatter package pending
struct alignment pending
fix engine pending
CLI pending or current actual status
```

Do not mark a feature implemented until tests exercise the shared path.

---

# Design summary

Sec has one canonical formatter.

The LSP, CLI, fix engine, refactorings, and generators share it.

Ordinary formatting preserves semantics and may normalize only parser-proven
noncanonical syntax such as:

```text
func -> fn
x++ -> x += 1
x-- -> x -= 1
```

Invalid source repairs require `sec fmt --fix` or an LSP safe fix.

Struct fields use gofmt-like local alignment for:

```text
field names and types
struct tags
commas
trailing line comments
```

Comments, tag text, declaration order, copy syntax, and move syntax are
preserved.

Code-quality improvements and refactorings remain separate from formatting.

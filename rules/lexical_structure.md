# Lexical Structure

## Purpose

This rulebook defines how Sec source text is converted into lexical tokens.

It covers:

- source encoding;
- line endings;
- whitespace;
- comments;
- identifiers;
- reserved words;
- compiler-known type names;
- contextual words and operators;
- literals;
- operators and punctuation;
- token boundaries;
- lexical diagnostics;
- source positions;
- formatter and tooling requirements.

This rulebook defines lexical structure only.

Parsing, name resolution, type checking, operator precedence, and runtime
semantics are defined by their respective rulebooks.

The lexer always tokenizes `default` as a keyword. The parser resolves its
context, including switch/select default branches, the named-type default clause
and any other separately defined default context. This contextual parsing does
not change lexical meaning.

The canonical explicit zero literals are `0c` for `char` and `0r` for `rune`;
they must not be documented as empty character source literals.

---

# 1. Source files

## 1.1 Encoding

Sec source files use UTF-8.

Invalid UTF-8 is a lexical error.

A UTF-8 byte-order mark is permitted only as the first bytes of a source file.

The compiler ignores that initial byte-order mark.

A byte-order mark anywhere else is an invalid source character.

The formatter removes an initial byte-order mark when rewriting a file.

---

## 1.2 Source extension

Ordinary Sec source files use:

```text
.sec
```

Other files such as manifests, generated MLIR, and rulebooks are not governed by
this lexical rule.

---

## 1.3 Unicode scalar values

The lexer processes Unicode scalar values rather than raw bytes.

A diagnostic source position must identify:

- file;
- one-based line;
- one-based Unicode-scalar column;
- byte offset where required by tooling.

A tab counts as one source character for lexical column tracking.

Editor display columns may differ according to tab width.

Diagnostics and LSP ranges must preserve exact byte offsets so tools can map
positions correctly.

---

# 2. Line endings

The accepted source line endings are:

```text
LF      \n
CRLF    \r\n
CR      \r
```

The lexer treats all three as line breaks.

A CRLF pair counts as one line break.

The formatter writes LF line endings by default unless an explicit formatter
configuration preserves another repository convention.

Formatted files end with exactly one newline.

---

# 3. Whitespace

The following characters are lexical whitespace outside literals:

```text
space       U+0020
tab         U+0009
line feed   U+000A
carriage return U+000D
```

Other Unicode spacing characters are not ordinary Sec whitespace.

This restriction prevents visually invisible source differences.

An unsupported spacing character outside a literal or comment is a lexical
error with a diagnostic identifying its Unicode code point.

Whitespace separates tokens when required.

Whitespace is otherwise not semantically significant.

Examples:

```sec
let value := 10

let    other    :=    20
```

Both declarations have the same meaning.

The formatter applies canonical spacing and indentation.

---

# 4. Newlines and statements

A newline is whitespace.

It is not automatically inserted as a statement terminator token.

Statement boundaries are determined by grammar.

Sec source does not require semicolons after ordinary statements:

```sec
let value := 10
value += 1
return value
```

The semicolon character is not a valid ordinary statement terminator in Sec
0.1.

Invalid:

```sec
let value := 10;
```

The lexer may still produce a semicolon token so the parser can issue a focused
diagnostic:

```text
semicolon is not used as a statement terminator in Sec
```

This also supports parser recovery for malformed foreign-style source.

A future rule may assign a specialized use to `;`, but no such use is defined
by Sec 0.1.

---

# 5. Comments

## 5.1 Line comments

A line comment begins with:

```text
//
```

and continues until the next line ending or end of file.

Example:

```sec
let value := 10 // current value
```

The line ending is not part of the comment token.

---

## 5.2 Block comments

A block comment begins with:

```text
/*
```

and ends with the matching:

```text
*/
```

Example:

```sec
/*
 * This is a block comment.
 */
```

Block comments may span lines.

---

## 5.3 Nested block comments

Block comments may nest.

Example:

```sec
/*
 * Outer comment.
 *
 * /*
 *  * Inner comment.
 *  */
 *
 * Outer comment continues.
 */
```

Each opening `/*` increases nesting depth.

Each closing `*/` decreases nesting depth.

The block comment ends when depth returns to zero.

An unterminated block comment is one lexical error beginning at the opening
delimiter.

---

## 5.4 Documentation comments

A block comment beginning with:

```text
/**
```

is a documentation comment.

Example:

```sec
/**
 * Returns the current temperature.
 */
fn CurrentTemperature() Temperature {
}
```

Documentation comments are lexically comments.

Their attachment to declarations and supported documentation tags are tooling
rules, not separate token syntax.

The lexer must preserve enough source information for:

- formatter attachment;
- LSP hover text;
- generated documentation;
- source mapping.

A sequence beginning with `///` is currently an ordinary line comment.

Sec 0.1 does not assign separate documentation semantics to `///`.

---

## 5.5 Comment preservation

Comments are not part of expression grammar.

The frontend must nevertheless retain comments or comment source ranges for:

- formatting;
- documentation generation;
- editor tooling;
- source-to-source transformations.

Comments separate surrounding tokens.

Example:

```sec
value/* comment */+1
```

is lexically equivalent to:

```sec
value + 1
```

A comment cannot be inserted inside a multi-character token.

Example:

```sec
1./* comment */.<10
```

does not contain the `..<` token.

---

# 6. Identifiers

## 6.1 Identifier form

An identifier begins with:

- a Unicode letter; or
- underscore `_`.

Subsequent characters may be:

- Unicode letters;
- ASCII digits `0` through `9`;
- underscore `_`.

Conceptual grammar:

```text
IdentifierStart
    := UnicodeLetter
     | "_"

IdentifierContinue
    := UnicodeLetter
     | ASCIIDigit
     | "_"

Identifier
    := IdentifierStart { IdentifierContinue }
```

Examples:

```sec
value
currentValue
HTTPServer
_speed
__state
Ω
Σ
μs
Meter2
```

Invalid identifier characters include:

- hyphen;
- whitespace;
- punctuation;
- emoji;
- combining marks outside a normalized identifier;
- non-ASCII digits.

Examples:

```text
current-value
value name
value🙂
```

These do not form one valid identifier.

---

## 6.2 Unicode normalization

Every identifier must already be encoded in Unicode Normalization Form C:

```text
NFC
```

The compiler does not silently rewrite an identifier into NFC.

A non-NFC identifier is rejected with a diagnostic showing the normalized
spelling.

Reason:

- visually identical identifiers must not create different symbols;
- linker and tooling names must be stable;
- source search and diagnostics must remain predictable.

Identifier comparison is:

- case-sensitive;
- based on exact NFC Unicode scalar values;
- not locale-sensitive.

Examples:

```text
Value
value
```

are different identifiers.

The compiler may warn about visually confusable identifiers, but confusable
detection does not alter symbol identity.

Mixed scripts are allowed because Sec units and scientific code may validly use
names such as:

```sec
μs
Ω
```

---

## 6.3 Bare underscore

The spelling:

```text
_
```

is a dedicated contextual symbol.

It is not an ordinary user binding name.

Its meaning is determined by grammar.

Existing meanings include:

- ignored match pattern;
- reserved unnamed register bits;
- explicit discard positions defined by another rule.

The lexer must distinguish bare `_` from identifiers beginning with underscore:

```text
_
_name
__name
```

`_name` and `__name` are ordinary identifiers with visibility semantics defined
by:

```text
names_scopes_visibility.md
```

Bare `_` must not be treated as a declaration that suppresses every unrelated
diagnostic.

---

## 6.4 No escaped identifiers

Sec does not support escaped identifiers.

A reserved word cannot be used as an identifier by quoting or escaping it.

Invalid conceptual forms:

```text
`match`
\match
@"match"
```

Raw-string syntax does not create an identifier.

This keeps diagnostics, formatting, code search, and symbol naming direct.

---

# 7. Reserved words

Reserved words cannot be used as names for:

- variables;
- constants;
- functions;
- parameters;
- fields;
- properties;
- events;
- types;
- modules;
- imports;
- generic parameters;
- labels;
- aliases.

The reservation applies even when a word is only meaningful in a particular
grammar context.

---

## 7.1 General language keywords

The general Sec keyword set includes:

```text
after
arena
asm
assert
await
break
cancel
capture
case
continue
default
defer
discard
else
enum
extern
fallthrough
false
fn
for
free
get
if
impl
implements
import
in
interface
let
match
module
mut
panic
property
range
ref
require
return
sec
select
self
spawn
static
struct
switch
true
try
type
union
unit
unsafe
where
while
```

Some listed words are not yet fully implemented.

Their reservation prevents source compatibility problems while their rulebooks
are completed.

---

## 7.2 Context-specific reserved words

The following words are reserved but interpreted only in specific contexts:

```text
set
task
thread
process
```

Examples:

```sec
let values: set[int]

property Value: int {
    set value {
    }
}

let worker := try spawn task Work()
let worker := try spawn thread Work()
```

Although the parser uses context to determine their role, they are not valid
user declaration names.

Invalid:

```sec
let set: int := 3
let task := 1

fn thread() void {
}
```

`process` remains reserved even while process spawning is deferred.

---

## 7.3 Contract words

Compiler-known contract spellings are reserved:

```text
even
finite
multipleOf
notEmpty
odd
unique
```

Additional contract words must be added to this rulebook when they become
canonical.

A contract spelling is not available as a user declaration name merely because
it appears only after a type.

---

## 7.4 Modifiers

Language modifiers are reserved declaration names.

This includes words such as:

```text
arena
extern
free
mut
ref
static
unsafe
task
thread
process
```

A modifier may be lexed contextually, but it cannot be reused as a user symbol.

---

# 8. Compiler-known lowercase type names

Fundamental types and first-class compiler-known type constructors use lowercase
names.

They cannot be redeclared or used as names for variables, functions, fields,
parameters, modules, or user-defined types.

The initial reserved type-name set includes:

```text
any
bool
byte
char
decimal
decimal128
float
float32
float64
int
int8
int16
int32
int64
int128
int256
uint
uint8
uint16
uint32
uint64
uint128
uint256
rune
string
void

bit
register

list
map
set
vector
matrix
tensor
tensor_view
```

Examples:

```sec
let value: int := 10
let values: list[int]
let pixels: matrix[Pixel, 480, 640]
```

Invalid:

```sec
let int := 10

fn map() void {
}

type string struct {
}
```

The lexer may represent built-in type names as dedicated token kinds or as
reserved identifier-like tokens.

The parser and Sema behavior must be identical either way.

Nominal core types remain ordinary uppercase identifiers:

```sec
Result[T, E]
Option[T]
Task[T]
Thread[T]
Shape[Rank]
Strides[Rank]
```

They are not lexical keywords.

They are unavailable for conflicting declarations because core symbols are
predeclared before user code.

---

# 9. Contextual spelling `set`

The spelling:

```text
set
```

has two language roles.

## 9.1 Built-in collection type

In type position followed by generic arguments:

```sec
set[int]
set[UserID, 128]
```

it denotes the built-in set type constructor.

---

## 9.2 Property setter

Inside property accessor grammar:

```sec
property Value: int {
    set value {
    }
}
```

it introduces an infallible setter.

After `try` in the same grammar:

```sec
property Value: Percent {
    try set value {
    }
}
```

it introduces a fallible setter.

---

## 9.3 Reservation

`set` is contextually interpreted but lexically reserved.

It cannot be an ordinary identifier:

```sec
let set := 1 // invalid
```

The parser must not require two different source spellings for collection set
and property set.

The formatter and syntax highlighter determine presentation from syntactic
context.

---

# 10. Contextual operator `x`

The spelling:

```text
x
```

is not a reserved word.

It remains a valid ordinary identifier:

```sec
let x := 10

fn Scale(x: float) float {
    return x * 2.0
}
```

Between expressions in infix operator position, `x` denotes shaped
linear-algebraic multiplication:

```sec
let result := left x right
```

The lexer emits ordinary identifier spelling for `x`.

The parser recognizes it as a contextual operator only when:

- a complete left expression precedes it;
- a valid right expression can follow it;
- it is not in declaration-name position;
- it is not part of member access;
- it is not the callee at the beginning of a call expression.

Examples:

```sec
x(value)          // call the function or callable named x
left x right      // contextual matrix-multiplication operator
object.x          // member named x
```

Whether the operands support matrix multiplication is checked by Sema.

The formatter writes one space around operator `x`:

```sec
left x right
```

No whitespace is required for lexical recognition, but canonical formatting
always inserts it.

---

# 11. Boolean literals

The boolean literals are:

```text
true
false
```

They are reserved tokens, not identifiers.

There is no `nil` keyword.

`None`, `Some`, `Ok`, and `Err` are nominal variant or constructor names, not
keywords.

---

# 12. Numeric literals

## 12.1 General rule

A sign is not part of a numeric literal.

Examples:

```sec
-10
+10
```

are tokenized as unary operator followed by numeric literal.

Numeric literals remain untyped until context and Sema determine their type,
subject to explicit family suffixes.

---

## 12.2 Decimal integer literals

A decimal integer literal contains ASCII digits:

```sec
0
9
123
1000000
```

Leading zeroes are permitted but have no octal meaning:

```sec
0010
```

is decimal ten.

Octal requires the explicit `0o` prefix.

---

## 12.3 Base-prefixed integer literals

Binary:

```sec
0b1000
0B1000
```

Octal:

```sec
0o10
0O10
```

Hexadecimal:

```sec
0xFF
0Xff
```

The prefix is case-insensitive.

Digits after the prefix must be valid for that base.

A base prefix with no following digit is one malformed numeric literal.

Invalid:

```sec
0b
0o9
0x
0xGG
```

The lexer should consume the maximal malformed numeric candidate and issue one
focused diagnostic rather than split it into misleading tokens.

---

## 12.4 Fractional literals

A fractional literal uses a period as decimal separator:

```sec
3.14
0.5
.5
```

Locale-specific comma decimal syntax is not accepted.

Invalid:

```sec
3,14
```

A trailing period does not form a numeric literal:

```sec
1.
```

Write:

```sec
1.0
```

This prevents ambiguity with member access and range tokens.

---

## 12.5 Exponent notation

Scientific exponent notation is supported for decimal-form literals:

```sec
1e3
1E3
1.5e-2
.5E+4
```

The exponent contains:

- `e` or `E`;
- optional `+` or `-`;
- one or more ASCII decimal digits.

An exponent literal without an explicit `f` suffix follows Sec's ordinary exact
decimal literal inference.

Examples:

```sec
let exact := 1.25e-3
let floating := 1.25e-3f
```

Exponent notation is not permitted on binary, octal, or hexadecimal literals.

---

## 12.6 Digit separators

Underscore may separate digits for readability:

```sec
1_000_000
0b1111_0000
0o755_000
0xFFFF_FFFF
1_234.567_890
1.25e1_000
```

A digit separator:

- must occur between two valid digits of the current numeric component;
- has no semantic value;
- may appear multiple times only when each underscore remains between digits.

Invalid:

```sec
_100
100_
1__000
0x_FF
1_.5
1._5
1e_3
1e3_
```

An underscore after a completed literal begins an identifier only when token
separation makes that unambiguous.

The formatter may preserve valid digit grouping.

It must not silently change a literal's value.

---

## 12.7 Numeric family suffixes

A one-letter suffix may select the numeric family:

```text
i    signed integer family
u    unsigned integer family
f    binary floating-point family
d    exact decimal family
c    char scalar type
r    rune scalar type
```

Examples:

```sec
8i
8u
8f
8d
65c
65r
1.5f
1.5d
1e3f
1e3d
```

The suffix selects a family, not a fixed width.

Width is selected by context or explicit type:

```sec
let small: int8 := 8i
let wide: int128 := 8i
let exact: decimal128 := 8d
```

`c` and `r` apply only to integer literals and select the exact `char` and
`rune` scalar types. Their value must be a Unicode scalar value: within
`0..U+10FFFF` and outside the surrogate range `U+D800..U+DFFF`.

Binary and octal literals accept `i`, `u`, `c`, or `r`. Hexadecimal literals
accept `i`, `u`, or `r`. The spelling `c` is already a hexadecimal digit, so
`0x41c` is one unsuffixed hexadecimal literal rather than `0x41` with a char
suffix. Write hexadecimal character values in decimal, binary, or octal form
when using the `c` suffix.

Fractional and exponent forms accept only `f` or `d`.

Invalid:

```sec
0b10d
1.5c
1e3r
```

A suffix must be adjacent to the literal.

---

## 12.8 Range disambiguation

The range tokens are:

```text
..     inclusive range
..<    exclusive upper bound
```

Examples:

```sec
1..10
1..<10
```

The lexer must not interpret the first period as a fractional point when it is
followed by another period.

Tokenization:

```text
1..10
```

becomes:

```text
INT("1")
RANGE("..")
INT("10")
```

A leading range remains valid tokenization:

```sec
..10
```

Whether it is valid grammar depends on context.

---

## 12.9 Exact source lexeme

The compiler must retain the original numeric lexeme through the stage where
exact interpretation is required.

In particular, decimal literals must not be converted through binary floating
point before exact decimal construction.

Digit separators are removed only during semantic numeric interpretation.

---

# 13. Character literals

A character literal uses single quotes:

```sec
'A'
'\n'
'Ω'
```

After escape processing, a character literal must contain exactly one Unicode
scalar value.

Invalid:

```sec
''
'AB'
```

A newline cannot occur directly inside a character literal.

Supported escapes are defined in the escape section below.

Whether the literal initializes `char`, `rune`, or another compatible type is
determined by Sema and the type rules. It is a `char` by default, but becomes
a `rune` when a `rune` is expected, when compared directly to a `rune` value,
or as a `rune` `switch` case. Thus `if ch == '$'` is valid when `ch` is a
`rune`.

An unterminated character literal is one lexical error beginning at its opening
quote.

---

# 14. String literals

## 14.1 Ordinary strings

An ordinary string uses double quotes:

```sec
"hello"
"hello\nworld"
```

Ordinary string literals cannot contain an unescaped physical newline.

An ordinary string is decoded through the escape rules.

An unterminated ordinary string is one lexical error beginning at its opening
quote.

---

## 14.2 Raw strings

A raw string uses backticks:

```sec
`raw text`
```

Raw strings:

- may span physical lines;
- do not process backslash escapes;
- preserve their content exactly apart from source line-ending normalization
  chosen by the frontend;
- end at the next backtick.

Example:

```sec
let text := `first line
second line`
```

A raw string cannot directly contain a backtick.

Sec 0.1 defines no doubled-backtick escape.

Raw strings are also used by grammar rules that attach metadata or annotations
to declarations:

```sec
field: int `json:"field"`
```

The consuming grammar determines whether a raw-string token is a normal string
expression or declaration metadata.

---

## 14.3 Interpolated strings

An interpolated string begins with:

```text
$"
```

and ends with the matching unescaped double quote.

Example:

```sec
$"Hello {name}"
```

An unescaped single `{` begins an interpolation expression.

The matching `}` ends that interpolation.

Nested braces inside the expression are balanced according to normal Sec
expression grammar.

Literal braces are written by doubling them:

```sec
$"{{value}}"
```

which produces:

```text
{value}
```

String escapes remain available in the non-expression portions.

Interpolated strings cannot contain an unescaped physical newline.

The frontend may initially tokenize the entire interpolated string as one token,
but parser-visible interpolation expressions must retain accurate nested source
ranges and normal Sec expression diagnostics.

---

## 14.4 Byte strings

Sec 0.1 does not define a byte-string literal syntax.

A byte buffer is created through:

- a byte array;
- a collection constructor;
- string conversion;
- an explicit core or stdlib API.

Any existing lexer token named `BYTES` without source syntax is an implementation
placeholder and must not define language behavior.

A future byte-string syntax requires an explicit rulebook update.

---

# 15. Escapes

The following escapes are valid in ordinary strings, interpolated string text,
and character literals where applicable:

```text
\\          backslash
\"          double quote
\'          single quote
\n          line feed
\r          carriage return
\t          horizontal tab
\0          zero
\xNN        exactly two hexadecimal digits
\u{H...}    one to six hexadecimal digits naming a Unicode scalar value
```

Examples:

```sec
"\n"
"\x41"
"\u{03A9}"
'\u{03BC}'
```

`\xNN` contributes the scalar value represented by the byte value.

`\u{...}` must denote a valid Unicode scalar value.

It must not denote:

- a surrogate code point;
- a value above `U+10FFFF`;
- an empty digit sequence.

Unknown or malformed escapes are lexical errors.

A backslash followed by a physical newline is not a line-continuation escape in
Sec 0.1.

---

# 16. Operators and punctuation

The lexer recognizes the longest valid token at the current source position.

## 16.1 Assignment and arrows

```text
=       assignment
:=      declaration assignment
<-      move assignment
:<-     move declaration assignment
=>      arm or mapping arrow
```

---

## 16.2 Arithmetic

```text
+       addition or unary plus
-       subtraction or unary minus
*       multiplication
/       division
%       remainder
```

Compound assignment:

```text
+=
-=
*=
/=
%=
```

The contextual matrix-multiplication operator `x` is lexically an identifier and
is described separately.

---

## 16.3 Comparison and logical operators

```text
==
!=
<
<=
>
>=
&&
||
!
```

---

## 16.4 Bitwise operators

```text
&
|
^
~
<<
>>
```

Compound forms:

```text
&=
|=
^=
<<=
>>=
```

---

## 16.5 Range and spread

```text
.       member or qualified access
..      inclusive range
..<     exclusive upper-bound range
...     spread
```

Longest-match order for period-prefixed tokens is:

```text
...
..<
..
.
```

A period followed immediately by a decimal digit may begin a fractional
literal:

```sec
.5
```

---

## 16.6 Delimiters and punctuation

```text
,       comma
:       colon
?       question mark
@       attribute marker
#       reserved directive marker

( )
{ }
[ ]
```

`@` begins attribute syntax as defined by the future attribute rulebook.

`#` is reserved for compiler or source-directive syntax.

Until another canonical rule assigns a use, `#` outside a literal or comment is
a parser error rather than an ordinary user-defined operator.

Semicolon is lexed only for focused rejection and recovery as described above.

---

# 17. Longest-match rule

When several tokens share a prefix, the lexer chooses the longest valid token.

Examples:

```text
... before ..
..< before ..
<<= before <<
>>= before >>
+= before +
:<- before :
:= before :
<- before <
=> before =
```

The longest-match rule does not cross:

- whitespace;
- comments;
- end of file;
- literal boundaries.

Malformed composite punctuation must produce focused tokenization rather than
silently changing meaning.

---

# 18. Token boundaries

Two adjacent identifier characters form one identifier.

Example:

```text
value1
```

is one identifier.

A reserved word followed by an identifier character is one identifier when the
full spelling is not reserved:

```text
letValue
```

is an identifier, not `let` followed by `Value`.

A numeric literal followed immediately by a Unicode letter is valid only when
the letter is a recognized numeric suffix and the remaining spelling is valid.

Invalid:

```text
10meters
```

This must not silently tokenize as `10` and `meters`.

Write an explicit separator or unit conversion according to the unit rules.

---

# 19. Attributes and annotations

The lexical attribute marker is:

```text
@
```

Example conceptual form:

```sec
@noAlloc
fn Poll() void {
}
```

The lexer treats the attribute name as an identifier or reserved attribute word
according to the final attribute grammar.

Attributes are not comments.

Raw declaration metadata such as:

```sec
field: int `json:"field"`
```

uses raw-string tokens and is distinct from `@` attributes.

---

# 20. Lexical errors

A lexical error must:

- identify the source range;
- preserve lexer progress where safe;
- avoid cascading into character-by-character errors;
- emit a stable lexer diagnostic ID;
- continue from a reliable token boundary when possible.

Required lexical error categories include:

```text
invalid UTF-8
invalid source character
unsupported Unicode whitespace
non-NFC identifier
invalid identifier character
unterminated block comment
unterminated ordinary string
unterminated raw string
unterminated interpolated string
unterminated character literal
invalid escape sequence
invalid Unicode escape
invalid character literal length
malformed numeric literal
invalid base digit
missing exponent digits
invalid digit separator
invalid numeric suffix
unexpected byte-order mark
```

An illegal source character should produce one token or diagnostic and then
advance by one Unicode scalar value unless a larger malformed token has already
been recognized.

---

# 21. Lexer and parser boundary

The lexer is responsible for:

- source decoding;
- whitespace;
- comments;
- identifier spelling;
- reserved-word recognition;
- literal boundaries;
- escape validation;
- operator and punctuation tokens;
- source locations;
- malformed-token diagnostics.

The parser is responsible for:

- grammatical role;
- contextual `set`;
- contextual operator `x`;
- declaration structure;
- type-argument structure;
- expression precedence;
- statement boundaries;
- property accessor context;
- attribute structure;
- raw metadata placement.

Sema is responsible for:

- name resolution;
- reserved symbol conflicts not enforced lexically;
- type validity;
- literal range;
- contextual literal type;
- matrix-operator operand validity;
- contract validity;
- annotation meaning.

The implementation may move a validation earlier when that improves diagnostics,
but the user-visible semantics must remain the same.

---

# 22. Formatter requirements

The formatter must:

- preserve comments;
- preserve literal values;
- preserve raw-string content;
- write canonical spaces around operators;
- write `left x right`;
- remove trailing whitespace;
- write one final newline;
- avoid semicolon insertion;
- preserve valid digit separators unless a numeric formatting rule explicitly
  canonicalizes them;
- keep declaration metadata attached to its declaration;
- preserve Unicode identifier spelling exactly after NFC validation.

The formatter must not rename identifiers or normalize them silently.

---

# 23. LSP and syntax-highlighting requirements

Tooling must distinguish:

- hard keyword;
- contextual reserved word;
- built-in lowercase type;
- nominal type;
- ordinary identifier;
- contextual operator `x`;
- property `set`;
- collection type `set`;
- bare `_`;
- visibility-prefixed identifier;
- literal;
- comment;
- documentation comment;
- attribute;
- raw declaration metadata.

Semantic highlighting may refine the lexer's initial token classification.

For example, lexer spelling `x` may begin as an identifier and later be
highlighted as an operator after parsing.

---

# 24. Implementation status

## Implemented

The current lexer already provides substantial support for:

- Unicode-rune input processing;
- one-based line and column tracking;
- identifier starts using underscore, ASCII letters, and Unicode letters;
- ASCII digits after identifier start;
- keyword lookup for a large current subset;
- decimal integer literals;
- fractional literals including leading-period forms such as `.5`;
- binary, octal, and hexadecimal integer prefixes;
- numeric family suffixes `i`, `u`, `f`, `d`, `c`, and `r`;
- ordinary strings;
- character-literal tokenization;
- backtick raw strings;
- `$"..."` interpolated-string tokenization;
- line comments;
- nested block comments;
- comment tokens;
- LSP semantic tokens for keywords, variables, types, functions, methods,
  comments, literals and operators;
- LSP member completion resolves `self` to its enclosing impl target, including
  fields, register fields, properties, events and instance methods;
- LSP member completion retains inferred local types from function and method
  return values, including while an `if` condition is incomplete in the editor;
- LSP hover resolves `self`, `self.field`, `self.property`, `self.event` and
  `self.Method(...)` against the enclosing impl target, displaying member type
  information or method signatures and adjacent `/** ... */` method docs;
- semantic-token fallback to lexical classification when semantic analysis
  encounters an incomplete editor AST;
- LSP member-completion traversal ignores typed-nil AST nodes produced during
  parser recovery, preventing completion requests from terminating the server;
- LSP document outline for module, type, enum, interface, function, struct,
  variable and impl members;
- LSP hover information from `/** ... */` documentation comments immediately
  above function declarations;
- common arithmetic, logical, bitwise, range, spread, delimiter, and assignment
  tokens;
- longest matching for existing multi-character punctuation;
- `ILLEGAL` tokens for some unterminated literals and comments;
- lexer snapshot and restore.

These implementation facts do not override the normative rules above.

---

## Partially implemented

The following currently exist but require synchronization or stronger
validation:

- keyword inventory;
- contextual `set`;
- bare `_` tokenization;
- character-literal content validation;
- escape validation;
- interpolation parsing;
- malformed base-literal diagnostics;
- comment attachment for formatter and documentation;
- CR and CRLF line accounting;
- built-in type-name reservation;
- parser rejection of semicolons;
- source position integration across lexer, parser, diagnostics, and LSP;
- documentation-comment support in LSP hover without AST-level comment
  attachment.
- semantic validation of character literals, including exactly one decoded
  scalar and the documented `\\`, `\"`, `\'`, `\n`, `\r`, `\t`, `\0`, `\xNN`, and
  `\u{...}` escape forms.

The current repository keyword notes and lexer keyword switch are not fully
synchronized.

This rulebook becomes the canonical lexical inventory.

---

## Not implemented

The following rules are not yet fully implemented:

- explicit UTF-8 validation;
- initial BOM handling;
- rejection of BOM elsewhere;
- NFC validation of identifiers;
- diagnostics for unsupported Unicode whitespace;
- confusable-identifier warnings;
- scientific exponent literals;
- numeric digit separators;
- maximal malformed numeric-token recovery;
- strict suffix validation by base and literal family;
- lexer-level escape diagnostics; character-literal escape validation currently
  occurs during semantic analysis;
- balanced interpolation expression lexing;
- doubled literal braces in interpolated strings;
- contextual `x` operator parsing;
- built-in `list`, `map`, `set`, `vector`, `matrix`, `tensor`, and
  `tensor_view` lexical/parser integration;
- complete reserved contract-word inventory;
- documentation-comment attachment in the parser and AST;
- dedicated lexical tests for every invalid category;
- stable lexer diagnostic IDs for all categories;
- formatter and complete LSP support for all contextual classifications in this
  rulebook.

The existing unused `BYTES` token does not represent a Sec 0.1 source feature.

The current bare-underscore path must be reviewed so `_` is not accidentally
classified as an ordinary identifier.

---

# 25. Required tests

The lexer test suite must include valid and invalid coverage for:

```text
UTF-8 and BOM
LF, CRLF, and CR
ASCII whitespace
unsupported Unicode whitespace
ASCII identifiers
Unicode identifiers
NFC and non-NFC identifiers
bare _
_name
__name
reserved words
built-in type names
contextual set
identifier x
operator x
line comments
nested block comments
documentation comments
ordinary strings
raw strings
interpolated strings
character literals
escapes
integer bases
fractions
leading-period fractions
exponents
digit separators
suffixes
ranges
spread
longest-match punctuation
semicolons
illegal characters
unterminated tokens
source positions
```

Invalid integration cases must document:

```sec
/* Expected error: ...
 * Reason: ...
 */
```

Lexer unit tests must verify:

- token type;
- source lexeme;
- byte range;
- line;
- column;
- recovery token after an error.

---

# 26. Required synchronization

This rulebook must be synchronized with:

```text
names_scopes_visibility.md
types.txt
contracts.md
properties.txt
collections-shaped-types.md
registers.txt
enums.txt
units.txt
functions.txt
functions_lambda.txt
attributes.md
operators.md
grammar.md
formatter.md
diagnostics.txt
parser_recovery.md
core-library.md
stdlib.md
compiler_pipeline.txt
rules_implementations.txt
VS Code grammar
LSP token classification
lexer implementation
lexer tests
parser implementation
parser tests
```

When a new keyword, modifier, contract word, compiler-known lowercase type, or
punctuation token is introduced, this rulebook and the tooling inventories must
be updated in the same change.

---

# 27. Design summary

Sec source is UTF-8 and Unicode-aware, but intentionally strict about invisible
or ambiguous source text.

Identifiers are case-sensitive NFC spellings.

Keywords, modifiers, contract words, and compiler-known lowercase type names
cannot be reused as declarations.

`set` is reserved and interpreted contextually as either a collection type or a
property setter.

`x` remains an ordinary identifier spelling and becomes matrix multiplication
only in infix expression context.

Comments may nest, literals retain exact source information, semicolons do not
terminate statements, and the lexer always prefers the longest valid token.

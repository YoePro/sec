# Names, Scopes, and Visibility

## Purpose

This rulebook defines:

- valid declaration names;
- the unified declaration namespace;
- overload groups;
- lexical and member scopes;
- shadowing;
- visibility prefixes;
- contextual language spellings;
- name lookup;
- forward references;
- naming requirements for user-defined types;
- compiler and tooling requirements.

The purpose is to keep Sec source predictable.

A reader, compiler diagnostic, debugger, formatter, and language server should
resolve a name in the same way.

---

# 1. Identifier model

An identifier names a language declaration or binding.

Examples include:

```text
types
functions
constants
variables
parameters
generic parameters
fields
properties
methods
events
nested types
imports
module aliases
pattern bindings
loop bindings
```

An identifier spelling must satisfy the lexical identifier grammar.

The canonical character-level grammar belongs in `lexical_structure.md`.

This rulebook defines the semantic use of identifier spellings.

---

# 2. One declaration namespace per scope

Sec uses one declaration namespace inside each scope.

Types and values do not occupy separate namespaces.

The following declarations conflict when they use the same name in the same
scope:

```text
type declarations
functions
function overload groups
constants
variables
imports
module aliases
interfaces
units where their rule exposes a declaration name
```

Invalid:

```sec
type User struct {
    id: uint64,
}

fn User() int {
    return 1
}
```

Invalid:

```sec
type User struct {
    id: uint64,
}

let User := 10
```

Invalid:

```sec
const Count := 10

fn Count() int {
    return 10
}
```

A programmer must not need to know whether a name is used in a type position or
a value position to discover which declaration it denotes.

Qualified names remain distinct:

```sec
storage.Reader
network.Reader
```

---

# 3. Module namespace

All source files belonging to one module contribute to one module namespace.

A declaration in one file may conflict with a declaration in another file of
the same module.

Source-file boundaries do not create separate public or module-internal
namespaces.

The compiler must collect the complete module declaration surface before
analyzing function and method bodies.

This permits forward references while still detecting duplicate names across
files.

Example:

```text
module storage

reader.sec:
    type Reader struct { ... }

factory.sec:
    fn CreateReader() Reader { ... }
```

The physical file name has no effect on the declaration name.

Module identity and import-path resolution are defined by the module and project
rulebooks.

---

# 4. Function overload groups

Functions may be overloaded when the function rules permit it.

All overloads with one name form one namespace entry:

```sec
fn Format(value: int) string {
    return value.ToString()
}

fn Format(value: int, pattern: string) Result[string, FormatError] {
    return value.ToString(pattern)
}
```

An overload group occupies the name `Format`.

No non-function declaration may use that name in the same scope.

Invalid:

```sec
type Format struct {
}

fn Format(value: int) string {
    return value.ToString()
}
```

Two overloads must have distinct parameter signatures.

The return type never participates in overload resolution.

Invalid:

```sec
fn Read() int {
    return 1
}

fn Read() string {
    return "one"
}
```

Methods follow the same overload-group rule inside a type's member namespace.

---

# 5. Member namespace

A named type has one member namespace.

The following member categories share that namespace:

```text
fields
properties
methods
method overload groups
associated functions
constants
events
nested types
nested enums
nested unions
nested units
```

A member name may not be reused by a different member category.

Invalid:

```sec
type Person struct {
    Name: string,
}

impl Person {
    fn Name() string {
        return self.Name
    }
}
```

Invalid:

```sec
type Device struct {
}

impl Device {
    property Status: int {
        get {
            return 0
        }
    }

    enum Status {
        Ready,
        Failed,
    }
}
```

Method overloads are allowed when their parameter signatures are distinct.

Member conflicts are checked across the complete type declaration and its
ordinary `impl` block.

A separate source file does not create a separate member namespace.

---

# 6. Nested declarations

A nested type belongs to its containing type's member namespace.

Outside the containing type and its `impl`, the qualified name is required:

```sec
Vehicle.FuelType
```

Inside the owning type's `impl`, the short nested name may be used when it is
unambiguous:

```sec
impl Vehicle {
    fn DefaultFuel() FuelType {
        return FuelType.Electric
    }
}
```

A nested type must not leak into the surrounding module namespace.

Nested declaration support remains governed by `impl.txt`.

---

# 7. Lexical scopes

The following constructs introduce lexical scopes:

```text
module
type declaration
impl block
function or method
property getter
property setter
ordinary block
if branch
switch case
match arm
loop body
closure body
pattern-binding region
generic parameter list
```

A name declared in a lexical scope is visible from its declaration point unless
another rule explicitly registers the complete declaration surface before body
analysis.

Function, type, and impl-member surfaces are registered before bodies are
analyzed.

Local variables normally become visible only after their declaration has been
successfully analyzed.

A variable is not visible inside its own initializer.

Invalid:

```sec
let value := value + 1
```

---

# 8. Shadowing

Sec forbids shadowing of a visible declaration.

A declaration must not hide a name from an enclosing lexical scope.

Invalid:

```sec
let count := 10

if enabled {
    let count := 20
}
```

Invalid:

```sec
fn Process(value: int) void {
    for value in values {
    }
}
```

Invalid:

```sec
type Packet struct {
}

fn Decode() void {
    let Packet := 10
}
```

The rule applies to:

```text
locals
parameters
loop bindings
pattern bindings
generic parameters
nested declarations
imports and aliases
```

The compiler must identify the hidden declaration and its source location.

Reusing a name in disjoint sibling scopes is valid when neither declaration is
visible from the other:

```sec
if condition {
    let result := First()
} else {
    let result := Second()
}
```

Compiler-provided contextual bindings such as `self`, a property setter value
parameter, or a match payload binding remain subject to explicit grammar and
must not silently replace a visible user declaration.

---

# 9. Reserved language names

Keywords and modifiers may not be used as declaration names.

Invalid examples include:

```sec
let mut := 3
fn unsafe() void {
}
type match int
```

The restriction applies to:

```text
variables
constants
parameters
functions
types
interfaces
fields
properties
methods
events
generic parameters
imports and aliases
```

Compiler-known fundamental type names and built-in type constructors are also
reserved language names.

Examples include:

```text
int
string
list
map
set
vector
matrix
tensor
tensor_view
```

Invalid:

```sec
let list := 3
fn map() void {
}
type int uint64
```

The canonical complete set of reserved words and reserved language names belongs
in `lexical_structure.md`.

---

# 10. Contextual spelling: set

`set` has more than one grammar role.

In a type position followed by generic arguments, it is the first-class
collection type constructor:

```sec
let values: set[int]
```

Inside a property declaration, it introduces an infallible property setter:

```sec
property Value: int {
    set value {
        _value = value
    }
}
```

Inside a property declaration, `try set` introduces a fallible setter:

```sec
property Value: int {
    try set value {
        _value = value
        return Ok()
    }
}
```

The parser resolves `set` from grammar context.

`set` is nevertheless a reserved language spelling and may not be declared as
an identifier.

Invalid:

```sec
let set: int := 3
```

Invalid:

```sec
fn set() void {
}
```

The contextual treatment prevents grammar collision between the type
constructor and property syntax.

It does not make `set` available as a user declaration name.

---

# 11. Contextual operator spelling: x

`x` is an ordinary identifier spelling in identifier position:

```sec
let x := 10
```

It is a matrix-multiplication operator in a valid infix operator position:

```sec
let result := left x right
```

`x` is not a keyword, modifier, built-in type name, or generally reserved
declaration name.

The parser resolves the operator from expression grammar and surrounding
operands.

Valid:

```sec
fn Scale(x: float32) float32 {
    return x * 2.0
}
```

The formatter must preserve the distinction:

```sec
let x := 10
let result := left x right
```

The operator semantics and precedence belong in `operators.md` and
`collections-shaped-types.md`.

---

# 12. Visibility prefixes

Sec encodes declaration visibility in the identifier prefix.

The canonical model is:

```text
Name
    public

_name
    module internal

__name
    private
```

The prefix is part of the identifier spelling.

It is not a separate modifier.

---

## 12.1 Public names

A name without a leading underscore is public when declared in a scope that can
export names.

Examples:

```sec
type Reader struct {
}

fn Open() Result[Reader, IOError] {
}
```

Public visibility remains subject to project-level access rules.

For example, a module located behind an `internal` project boundary remains
unavailable to prohibited importers even though it contains public names.

---

## 12.2 Module-internal names

A name beginning with one underscore is visible throughout the declaring module
but is not exported from that module.

Examples:

```sec
type _BufferState struct {
}

fn _ValidateHeader() bool {
    return true
}
```

All source files belonging to the same module may use the name.

External importers may not access it.

---

## 12.3 Private names

A name beginning with two underscores is private to its declaring owner.

At module scope, the declaring owner is the source file:

```sec
fn __ReadHeader() void {
}
```

The name is unavailable from other files, including other files in the same
module.

For a type member, the declaring owner is the type and its ordinary `impl`:

```sec
type Counter struct {
    __value: int,
}

impl Counter {
    fn Increment() void {
        __value += 1
    }
}
```

Another type in the same file or module may not access `Counter.__value`.

For a nested declaration, private access remains inside the owning declaration
and its implementation context.

---

## 12.4 Local bindings

Local bindings already have lexical scope.

A leading `_` or `__` does not suppress unused-variable diagnostics and does not
turn a local into a discard binding.

Examples:

```sec
let _temporary := Calculate()
let __state := ReadState()
```

These are real local variables.

They must be used or explicitly discarded according to the diagnostics and
discard rules.

Bare `_` remains a context-specific grammar form and is not a general-purpose
identifier.

Its distinct uses include:

```text
match ignore patterns
reserved unnamed register bits
other explicitly defined grammar contexts
```

---

# 13. Type naming

Every user-defined nominal type must begin with an uppercase letter after any
visibility prefix.

Valid:

```sec
type Percent int

type Person struct {
    name: string,
}

type Response union {
}

enum Color int {
    Red = 1,
}

type Control register[32] {
    Enabled: bit,
    _: bit[31],
}

interface Reader {
    fn Read() Result[string, IOError]
}
```

Valid visibility-prefixed types:

```sec
type _ModuleState struct {
}

type __PrivateState struct {
}
```

Invalid:

```sec
type percent int
type person struct {
}
interface reader {
}
```

The rule applies to:

```text
named scalar types
distinct types
structs
unions
enums
registers
interfaces
error types
generic nominal types
nested nominal types
core nominal types
standard-library nominal types
```

Compiler-known fundamental and first-class lowercase types are exempt:

```text
int
string
list
map
set
vector
matrix
tensor
tensor_view
```

Unit symbols are governed by the unit naming rules and are not forced into the
nominal-type capitalization rule.

The uppercase requirement is a compiler error, not merely a formatter style.

This deliberate distinction makes built-in language types and user-defined
nominal types immediately recognizable in source, diagnostics, and debugger
output.

---

# 14. Other naming styles

Sec 0.1 does not impose a universal capitalization style on:

```text
functions
methods
fields
properties
constants
variables
parameters
modules
enum values
```

Those declarations must still satisfy:

- uniqueness;
- no shadowing;
- reserved-name restrictions;
- visibility-prefix rules;
- any category-specific rulebook.

The formatter must not rename declarations.

A later style guide may recommend conventions without changing name identity.

---

# 15. Generic parameters

Generic type parameters are type names and must begin with an uppercase letter:

```sec
type Pair[T, U] struct {
    first: T,
    second: U,
}
```

Compile-time value parameters follow the naming rule of ordinary values unless a
specific generic rule states otherwise.

Generic parameters share the declaration's generic parameter scope.

Duplicate generic parameter names are invalid.

A generic parameter must not shadow:

- another generic parameter;
- a visible type;
- a visible value;
- a member name made directly visible by the declaration grammar.

---

# 16. Imports and aliases

Imported module names and import aliases enter the current module declaration
namespace.

They must not conflict with:

```text
types
functions
constants
other imports
module aliases
```

Invalid conceptually:

```sec
import storage

type storage struct {
}
```

Qualification resolves names inside imported modules:

```sec
storage.Reader
storage.Open()
```

Import aliases must satisfy the same identifier and reserved-name rules as other
declarations.

Import visibility and project-internal access are defined by the module and
project rulebooks.

---

# 17. Name lookup order

Unqualified lookup proceeds from the innermost valid scope outward.

Conceptually:

1. active pattern, loop, closure, or block scope;
2. function or property scope;
3. generic parameter scope;
4. owning type and impl member scope;
5. module scope;
6. imported names and aliases;
7. always-available core declarations.

Because shadowing is forbidden, finding the same unqualified declaration name at
multiple visible lexical levels is normally a prior semantic error.

Member access uses the resolved receiver type:

```sec
value.Member
Type.AssociatedFunction()
Module.Declaration
```

The parser preserves ordinary member syntax.

Sema determines whether the member is a:

```text
field
property
method
associated function
enum value
nested type
event
compiler intrinsic
core member
```

---

# 18. Forward references

The compiler registers declaration surfaces before body analysis where the
language permits forward references.

At module level, it registers at least:

```text
types
interfaces
functions and overload groups
constants where supported
imports and aliases
```

For a type and its impl, it registers at least:

```text
fields
nested declarations
properties
methods and overload groups
events
constants where supported
```

This allows:

- calling a function declared later in another file of the same module;
- methods calling methods declared later;
- properties referring to properties declared later;
- type references across files in one module.

Local variables do not receive forward-reference behavior.

---

# 19. Name identity

A declaration's semantic identity includes its qualified owner path.

Examples:

```text
storage.Reader
network.Reader
Vehicle.FuelType
Result.Ok
```

Function overload identity additionally includes the parameter signature.

Return type is excluded from overload identity.

Visibility prefixes remain part of the source name and symbol identity unless
the ABI rulebook defines a mangling transformation.

Compiler-generated hidden names must use a namespace that cannot collide with
valid user declarations.

The compiler must not rely only on a user-accessible `__` prefix for hidden
symbols.

---

# 20. Diagnostics

Required diagnostics include:

```text
duplicate declaration User in module storage
```

```text
function User conflicts with type User declared here
```

```text
member Name conflicts with field Name on type Person
```

```text
local declaration count shadows visible declaration count
```

```text
set is a reserved language name and cannot be declared as a variable
```

```text
mut is a keyword and cannot be used as a function name
```

```text
user-defined type percent must begin with an uppercase letter
```

```text
module-internal declaration _Open is not visible outside module storage
```

```text
private declaration __ReadHeader is not visible outside its source file
```

Diagnostics must:

- use stable IDs;
- identify both declarations for conflicts;
- distinguish duplicate names from overload conflicts;
- distinguish inaccessible names from unknown names;
- preserve the original source spelling.

---

# 21. Parser, AST, and Sema

The lexer and parser must distinguish:

```text
globally reserved words
reserved language names
contextual language spellings
ordinary identifiers
contextual operator positions
```

The parser must resolve:

- property `set`;
- collection type `set[...]`;
- infix `x`;
- identifier `x`.

The AST should preserve identifier spelling without encoding visibility as a
separate source token.

Sema derives visibility from the prefix and records:

```text
Public
ModuleInternal
Private
LexicalLocal
```

Sema must maintain:

- one declaration namespace per scope;
- overload-group entries;
- complete member namespaces;
- source-file ownership for module-scope private declarations;
- owning-type identity for private members;
- shadowing checks;
- reserved-name checks;
- type-capitalization checks.

---

# 22. Formatter and tooling

The formatter must not change identifier spelling or capitalization.

It must format matrix multiplication with spaces:

```sec
left x right
```

It must preserve identifier uses of `x`:

```sec
let x := 10
```

Syntax highlighting and LSP tokenization must classify `set` and `x` according
to context rather than spelling alone.

Rename operations must reject a target name when it would:

- collide in the unified namespace;
- create an invalid overload;
- shadow a visible declaration;
- use a reserved language name;
- violate the uppercase nominal-type rule;
- change intended visibility through `_` or `__`.

Debugger and diagnostic displays should use fully qualified names when ambiguity
is possible.

---

# 23. Implementation status

## Implemented

Existing compiler rules already describe or partially implement:

- duplicate member checks across fields, properties, methods, and nested types;
- top-level module declaration namespace conflicts across types, units, enums,
  interfaces, function overload groups and module-level variables;
- method and property registration before body analysis;
- qualified nested type lookup;
- one module namespace across module source files in the project model;
- function overload selection by parameter signature;
- public, module-internal, and private visibility categories in existing design
  material.

These existing behaviors must be audited against this consolidated rulebook.

## Not implemented

This rulebook is not considered fully implemented until the compiler provides:

- complete unified declaration namespace for all declaration categories and
  nested lexical scopes;
- complete duplicate checks across module files for imports, aliases and other
  declarations not yet covered by the top-level module namespace pass;
- no-shadowing checks;
- reserved keyword, modifier, and built-in-name checks for every declaration
  category;
- contextual `set` parsing without allowing `set` declarations;
- contextual `x` operator parsing while retaining identifier `x`;
- mandatory uppercase nominal-type diagnostics;
- complete `_` and `__` visibility enforcement;
- private source-file ownership;
- complete import-alias conflict checks;
- formatter, syntax grammar, and LSP contextual support;
- valid and invalid integration tests.

---

# 24. Required synchronization

This rulebook must be synchronized with:

```text
lexical_structure.md
grammar.md
operators.md
types.txt
variables_contracts.txt
functions.txt
functions_lambda.txt
generics.txt
interfaces.txt
impl.txt
properties.txt
struct.txt
enums.txt
unions.txt
registers.txt
units.txt
collections-shaped-types.md
flowcontrol_for.txt
flowcontrol_match.txt
projects.txt
modules.md
linking.md
diagnostics.txt
formatter.txt
semantic_ir.txt
core-library.md
stdlib.md
VS Code grammar
LSP implementation
debug information
```

The living `language-rulebook-status.md` must mark this rulebook as written when
it is added to the repository.

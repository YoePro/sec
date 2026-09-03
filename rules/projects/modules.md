# Sec Modules

- **Status:** Normative
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/projects/modules.md`

## 1. Purpose

This rulebook defines the canonical Sec 0.1 module model.

It owns:

- source-module identity;
- source-directory and source-file membership;
- module declarations;
- import paths and import bindings;
- import aliases;
- module access through `internal`;
- module dependency graph construction;
- import-cycle rejection;
- cross-module declaration identity;
- public module surfaces;
- separate-compilation requirements for module resolution;
- deterministic and incremental module processing.

This rulebook does not define:

- project-manifest syntax, Targets, Variants, or build-output layout;
- static or runtime initialization order;
- deinitialization order;
- ABI representation;
- link-time symbol spelling, mangling, dead stripping, or artifact composition;
- package/dependency acquisition semantics not yet defined for Sec 0.1.

Those concerns belong to their respective project, initialization, ABI, linking,
and future dependency rulebooks.

---

## 2. Core terminology

### 2.1 ModuleName

`ModuleName` is the source-level identifier declared by a source module and used
as its default qualifier when imported.

Example:

```sec
module orders
```

The `ModuleName` is `orders`.

A module declaration contains exactly one identifier.

Dotted module declarations are invalid:

```sec
module domain.orders
module internal.storage
module platform.linux.amd64
```

### 2.2 ModuleIdentity

`ModuleIdentity` is the compiler's canonical semantic identity for one logical
source module.

A `ModuleIdentity` is derived from at least:

```text
ImportRootIdentity
CanonicalImportPath
```

For project-local modules, the project identity participates in the import-root
identity.

Two modules may have the same short `ModuleName` while having different
`ModuleIdentity` values.

Example:

```text
sales/order
customer/order
```

Both directories may contain:

```sec
module order
```

They remain distinct modules because their canonical identities differ.

The compiler must never identify modules only by their short source-level name.

### 2.3 ModuleInstance

`ModuleInstance` is a `ModuleIdentity` as selected and analyzed under one
`CompilationPlan`.

The same logical module may therefore have more than one `ModuleInstance` when
plan-dependent source selection or semantic completion differs between build
Variants.

A plan-specific instance does not become a different logical `ModuleIdentity`
merely because it is compiled for another target platform.

### 2.4 ModuleSurface

`ModuleSurface` is the canonical semantic information that an importer needs in
order to resolve and validate uses of a module without requiring ordinary
implementation bodies to be reanalyzed.

Conceptually, a surface contains at least:

```text
ModuleIdentity
PublicDeclarations
SemanticSignatures
RequiredTypeInformation
SurfaceFingerprint
```

The exact in-memory and serialized representation is implementation-defined.

A `ModuleSurface` is semantic. It does not define ABI representation or binary
linkage names.

---

## 3. Source directory and module membership

One source directory forms one source module.

All applicable `.sec` files directly belonging to that directory contribute to
one shared module declaration namespace.

Example:

```text
domain/orders/
    order.sec
    status.sec
    validation.sec
```

Every file declares:

```sec
module orders
```

Source-file boundaries do not create separate public or module-internal
namespaces.

The normal module name is the final source-directory component.

Examples:

```text
domain/orders/                 -> module orders
services/database/             -> module database
internal/storage/              -> module storage
services/internal/transport/   -> module transport
```

Project-root source modules follow the root-module rules defined by the project
rulebook.

---

## 4. Module declaration

Every ordinary Sec source file declares the source module to which it belongs.

The declaration must match the `ModuleName` required for its module directory.

Invalid:

```text
orders/
    order.sec       -> module orders
    status.sec      -> module status
```

The compiler must diagnose inconsistent module declarations before ordinary
body analysis.

A module declaration is not a filesystem path and does not contain directory
hierarchy, `internal` markers, or compiler-reserved import roots.

---

## 5. Shared declaration namespace

All source files of one module contribute declarations to one shared module
namespace.

Example:

`order.sec`:

```sec
module orders

struct CustomerOrder {
    Id: uint64,
}
```

`create.sec`:

```sec
module orders

fn CreateCustomerOrder() CustomerOrder {
    return CustomerOrder {}
}
```

No import is required between source files belonging to the same module.

Duplicate-declaration checks operate over the complete module namespace and
follow the declaration-specific rules for overload groups, `impl`, `impl
extends`, nested declarations, and every other explicitly mergeable declaration
form.

Source-file order and filesystem enumeration order have no semantic meaning.
A declaration may refer to another declaration in the same module regardless of
which participating source file is parsed first.

The compiler must collect the complete module declaration surface before
ordinary function and method bodies require full name resolution.

---

## 6. Source-file-private and module-internal declarations

Visibility prefixes retain their canonical visibility meaning:

```text
Name       Public
_name      ModuleInternal
__name     SourceFilePrivate
```

A module-internal declaration may be used by any source file of the same
module.

A source-file-private declaration may be used only from its declaring source
file, subject to the canonical name/scope rules.

Module access restrictions and declaration visibility are separate checks.

A public declaration inside a module that an importer is not permitted to
import does not make that module importable.

---

## 7. Import paths

Sec imports use canonical logical import paths rather than arbitrary filesystem
paths.

Example:

```sec
import "domain/orders"
```

Canonical import paths:

- use `/` as the separator on every host platform;
- are case-sensitive;
- do not contain `.` or `..` traversal components;
- are not absolute host paths;
- do not use host-specific separators;
- do not depend on the compiler host's filesystem case behavior.

Invalid examples:

```sec
import "../orders"
import "./orders"
import "/srv/project/orders"
```

A project whose physical source tree cannot represent its distinct canonical
paths on the active host has a source-layout error. The compiler must not change
module identity to accommodate host filesystem case folding.

---

## 8. Import roots

Import resolution first selects one canonical import root.

Sec 0.1 includes the compiler-reserved platform root:

```text
platform
```

Standard-library modules instead use their canonical logical paths directly.
The compiler resolves those paths through the standard-library namespace before
project/dependency fallback, and a project-local module must not silently
replace a canonical standard-library module with the same path.

Example:

```sec
import "io"
import "net/http"
import "platform/linux/amd64"
```

If a reserved-root import does not resolve, the compiler must not retry the same
path as a project-local import.

Ordinary non-reserved imports resolve within the active project source root.

Future dependency/package roots may extend the import-root model without
changing the `ModuleIdentity` principle defined here.

---

## 9. Import resolution

Resolving an import must produce exactly one canonical `ModuleIdentity` or a
resolution diagnostic.

A resolved destination must:

- exist in the selected source universe;
- be a valid Sec source module;
- satisfy the applicable directory/module-name rule;
- satisfy project and `internal` access restrictions;
- be selected for the active `CompilationPlan` where source selection is
  plan-dependent.

The import path identifies the module location and canonical identity.

The resolved source module declaration identifies the default source qualifier.

Example:

```sec
import "customer/order"
```

If the imported module declares:

```sec
module order
```

the default qualifier is:

```sec
order.CreateCustomerOrder()
```

The canonical import path does not become a dotted source qualifier.

---

## 10. Import bindings are source-file-local

Import bindings belong to the source file that declares them.

Example:

`create.sec`:

```sec
module orders

import "database"

fn Create() void {
    database.Insert()
}
```

A different source file in module `orders` does not obtain the qualifier
`database` automatically. It must declare its own import before using that
external module qualifier.

This establishes the canonical distinction:

```text
Module declarations     module-wide
Import bindings         source-file-local
```

The dependency graph of a module is the union of the resolved imports declared
by all source files participating in its active `ModuleInstance`.

---

## 11. Import aliases and qualifier collisions

An import alias creates only a local binding in the declaring source file.

Example:

```sec
import customerOrder "customer/order"
```

Usage:

```sec
customerOrder.CreateCustomerOrder()
```

The alias does not change:

- `ModuleIdentity`;
- declaration identity;
- module namespace membership;
- linking identity;
- the imported module's own `ModuleName`.

Different modules may have the same `ModuleName`, but every imported module
binding in one source-file scope must have a unique local qualifier.

Example:

```sec
import (
    "sales/order"
    customerOrder "customer/order"
)
```

Usage:

```sec
order.Ascending()
customerOrder.CreateCustomerOrder()
```

Two imports that would introduce the same local qualifier require explicit
aliasing.

The same canonical `ModuleIdentity` must not be imported more than once by one
source file, even through different aliases.

Import and alias names participate in the canonical unified namespace and
shadowing rules for the source scope in which they are declared.

---

## 12. No implicit name injection or re-export

Importing a module does not inject that module's declarations as unqualified
names into the importer.

After:

```sec
import "storage"
```

use:

```sec
storage.Open()
```

not an unqualified `Open()` solely because `storage` declares it.

Ordinary imports are never implicitly re-exported.

If module `A` imports module `B`, a module importing `A` does not thereby gain a
binding for `B`.

Explicit re-export syntax and semantics are not defined for Sec 0.1. A later
rulebook revision may introduce an explicit re-export mechanism without
changing the rule that ordinary imports are not re-exports.

Selective imports and wildcard imports are not part of Sec 0.1.

---

## 13. `internal` module access

The path segment `internal` is an access-control marker, not a source module
name.

It remains part of the canonical import path and therefore participates in
`ModuleIdentity`, but it does not appear in the source module declaration or
default source qualifier.

Example:

```text
services/internal/transport/
```

contains:

```sec
module transport
```

A module below an `internal` directory may be imported only by modules below the
directory that immediately contains that `internal` directory.

A root-level `internal` directory is private to the complete Sec project.

Import resolution enforces this access rule before imported declaration
visibility is considered.

A public declaration does not make an otherwise inaccessible internal module
importable.

---

## 14. Self-imports, duplicate imports, and cycles

A module must not import itself.

A source file must not import the same canonical `ModuleIdentity` more than
once.

The Sec 0.1 module import graph is acyclic.

Any import cycle is a semantic error.

Example:

```text
orders -> billing -> inventory -> orders
```

Cycle detection operates on resolved `ModuleIdentity` edges rather than import
aliases or source spellings.

A valid module graph is therefore a directed acyclic graph after self-import
and duplicate-import validation.

### 14.1 Required cycle algorithm

A conforming implementation may use DFS cycle detection, SCC decomposition, or
another equivalent deterministic algorithm.

A recommended implementation is:

```text
1. Resolve every source-file import to ModuleIdentity.
2. Merge file import edges into the ModuleInstance dependency set.
3. Reject self-edges.
4. Build the directed ModuleGraph.
5. Compute strongly connected components.
6. Reject every SCC containing more than one ModuleIdentity.
7. Produce one deterministic representative cycle for each reported SCC.
8. Topologically order the remaining acyclic graph.
```

Diagnostics should show a short, deterministic cycle and source locations for
its import edges rather than emitting an unbounded path for a large SCC.

---

## 15. ModuleGraph and CompilationPlan

`ModuleIdentity` is logical and stable across platform Variants when the same
logical source module is being compiled.

`ModuleInstance` and the selected `ModuleGraph` are plan-scoped where canonical
source selection differs by `CompilationPlan`.

The compiler must not model a multi-Variant build as one graph containing a
polymorphic architecture or platform identity.

Each concrete `CompilationPlan` receives its applicable module-instance graph.
Target-independent parsing and semantic work may be reused between plans only
when the reuse is sound under the relevant dependency model.

Sec 0.1 does not define conditional-import source syntax.

---

## 16. Public module surface

A module's ordinary external semantic surface consists of its public
module-scope declarations and the semantic information required to use them.

Imported module bindings are not automatically part of that surface.

A public declaration must not expose an inaccessible declaration or type in the
semantic signature required by an external importer unless another canonical
language feature explicitly defines such opacity.

This requirement applies where relevant to:

- parameter types;
- return types;
- public fields;
- public properties;
- public interface relationships;
- generic constraints and arguments;
- other declaration identities needed to understand the public signature.

The owning declaration/type rulebooks define the exact semantic content of
those signatures.

---

## 17. Cross-module declaration identity

A declaration's cross-module semantic identity incorporates its canonical
`ModuleIdentity`.

A display form such as:

```text
order.Create
```

is not sufficient as the compiler's unique semantic identity when two distinct
modules may both be named `order`.

Conceptually:

```text
CrossModuleDeclarationIdentity =
    ModuleIdentity + DeclarationIdentity
```

Additional identity components required by overloads, generics, nested owners,
or other declaration forms remain governed by their owning rulebooks.

This semantic identity must remain available to later ABI and linking stages.
Those stages may derive binary names from it but must not redefine which module
owns the declaration.

---

## 18. ModuleGraph is not the initialization graph

The module import graph represents semantic import dependencies.

It does not by itself define:

- static initialization order;
- runtime startup order;
- cross-module initialization failure behavior;
- deinitialization order.

The initialization rulebook consumes canonical module and dependency facts and
may define additional initialization-specific constraints.

The module graph and initialization/deinitialization graphs must not be treated
as automatically identical merely because imports contribute dependency facts.

---

## 19. Entry modules and source discovery

A project Target may designate an entry module according to the project rules.

Entry status is an additional project/build role and does not alter ordinary
module namespace, visibility, identity, or import semantics.

The complete source graph for a Target is discovered by following canonical
imports from the selected entry/source roots and by applying canonical
CompilationPlan source selection.

A project directory that is not selected into the active source graph is not
compiled merely because it contains `.sec` files somewhere below the project
root.

Compiler-provided standard-library modules and reserved roots such as `platform` use the same
`ModuleIdentity`, `ModuleName`, `ModuleInstance`, and `ModuleSurface` concepts as
project-local modules. Their distinction lies in their import-root identity,
source-selection ownership, and access rules.

### 19.1 Test-compilation module participation

An ordinary production `ModuleInstance` excludes every `*_test.sec` file.
A `TestCompilationPlan` may construct a test view containing the ordinary
production files plus same-directory `*_test.sec` files. Those test files have
ordinary same-module visibility, but no additional access to declarations that
remain private to another source file.

The dependency direction is `Test -> Production` and `Test -> Test`.
Production declarations must remain valid without test-only declarations, so
`Production -> Test` is invalid. Project-root `tests/` modules are test-only
modules and use ordinary imports and external visibility.

---

## 20. Separate compilation and incremental processing

Module resolution must support separate and incremental compilation without
making source-file ordering part of semantics.

An importer should be able to perform ordinary cross-module name and type
resolution from a compatible canonical `ModuleSurface` without reanalyzing the
imported module's ordinary implementation bodies.

A `ModuleSurface` must have a deterministic semantic fingerprint.

Implementation-only changes that preserve the semantic surface need not
invalidate importer name/type resolution.

Changes that affect any importer-visible semantic fact must invalidate the
corresponding dependent resolution.

At minimum, changes to the following may affect the surface:

```text
ModuleIdentity
public declaration set
public declaration visibility
public semantic signatures
publicly required type information
```

Other rulebooks may define additional exported summaries required for generics,
analysis, optimization, or lowering. Those summaries do not change the module
identity rules defined here.

### 20.1 Recommended incremental dependency model

Conceptually:

```text
SourceFile
    -> ModuleInstance
    -> ModuleSurface
    -> importing ModuleInstances
```

When one source file changes, an implementation should:

```text
1. Reparse the changed file.
2. Recompute its file-local imports and declarations.
3. Reassemble affected module declarations and imports.
4. Recompute the ModuleSurface fingerprint.
5. Recompute ModuleGraph edges if imports changed.
6. Re-run cycle validation where affected.
7. Propagate importer invalidation only when exported semantic dependencies changed.
```

Conservative over-invalidation is valid. Reuse of stale semantic surfaces is
not valid.

---

## 21. Diagnostics

A conforming implementation must provide deterministic diagnostics for at least:

- missing module declaration;
- module declaration inconsistent with its source directory;
- inconsistent module declarations among files in one directory;
- unresolved import;
- malformed or non-canonical import path;
- illegal `internal` import;
- duplicate import of the same `ModuleIdentity` in one source file;
- self-import;
- import qualifier or alias collision;
- import cycle;
- duplicate module-scope declarations according to the owning declaration rules;
- public semantic surface exposing an inaccessible declaration or type.

Import diagnostics should preserve:

```text
import source location
original import spelling
canonical ModuleIdentity where resolution succeeded
local qualifier or alias
access-control cause where applicable
CompilationPlan identity where the result is plan-dependent
```

Cycle diagnostics must show a deterministic representative cycle and navigable
import sites.

When the same semantic issue occurs in several CompilationPlans, tooling may
group presentation while retaining distinct plan-scoped results.

---

## 22. Compiler implementation model

A recommended compiler pipeline for modules is:

```text
1. Determine source files selected for the active CompilationPlan.
2. Parse module declarations and imports for selected files.
3. Group files into source modules by canonical source location.
4. Validate module-name agreement and directory/module rules.
5. Assign canonical ModuleIdentity values.
6. Resolve every source-file import to ModuleIdentity.
7. Validate import-root and `internal` access rules.
8. Build file-local import-binding tables.
9. Merge resolved import edges into ModuleInstance dependency sets.
10. Reject duplicate imports and self-imports.
11. Build the ModuleGraph and reject cycles.
12. Topologically order the valid graph.
13. Collect complete module declaration namespaces.
14. Validate duplicate declarations and visibility through owning rulebooks.
15. Build canonical ModuleSurface values.
16. Resolve cross-module symbol references.
17. Analyze ordinary bodies.
```

Implementations may combine or reorder steps when the same observable semantics
and diagnostics are preserved.

The compiler must not:

- derive module identity solely from the short `ModuleName`;
- use import aliases as graph identities;
- infer dotted source-module names from directory paths;
- let an import declared in one source file implicitly bind names in another;
- allow filesystem enumeration order to affect declaration or import results;
- recover from an unresolved reserved root by silently searching another root;
- permit cyclic imports through partial or order-dependent resolution;
- let later linking manufacture a different semantic module identity.

---

## 23. LSP and tooling requirements

The LSP must use the same canonical module resolver and `CompilationPlan`
selection model as command-line compilation.

For each source file, tooling must be able to determine:

```text
ModuleIdentity
ModuleName
ModuleInstance
file-local import bindings
module-wide declaration surface
imported ModuleIdentity dependencies
```

Edits to one source file must invalidate the file-local binding state
immediately and invalidate module/importer state according to dependency and
surface changes.

If an import edit introduces a cycle, the LSP should report the cycle without
requiring restart.

Rename and completion tooling must respect:

- file-local aliases;
- module-wide declarations;
- `internal` accessibility;
- canonical module identity;
- unified declaration namespace rules.

A qualifier rename changes the local alias/binding only unless the operation is
explicitly a module/declaration rename under separate project/refactoring rules.

---

## 24. Required validation tests

The implementation test suite must include at least:

### Module assembly

- one-file module;
- multi-file module;
- inconsistent module declarations;
- file-order independence;
- cross-file forward references;
- valid and invalid duplicate declarations.

### Import binding

- ordinary import;
- alias import;
- two modules with the same `ModuleName` using aliases;
- file-local import not visible from a sibling source file;
- different aliases for the same dependency in different source files;
- duplicate import in one source file;
- qualifier collision;
- self-import.

### Resolution

- project-local import;
- direct canonical standard-library import such as `io` or `net/http`;
- `platform` import;
- unknown reserved-root import without project fallback;
- case-sensitive path handling;
- rejected relative/absolute path forms;
- valid and invalid `internal` access.

### Graph

- acyclic chain;
- diamond dependency;
- direct cycle;
- indirect cycle;
- deterministic cycle diagnostic;
- plan-specific module graph where source selection differs.

### Surface and incremental behavior

- public surface creation;
- public signature exposing inaccessible type;
- implementation-only edit preserving surface fingerprint;
- public-signature edit changing surface fingerprint;
- import edit invalidating affected graph state;
- unrelated module edit preserving importer caches where supported.

### Determinism

Equivalent builds must produce identical module identities, graph edges,
surface fingerprints, and diagnostic ordering regardless of source-file
enumeration, map iteration, or compiler worker scheduling.

---

## 25. Completion criteria

Sec 0.1 module support is implementation-complete when the compiler can
reliably and deterministically transform the selected project source universe
into:

```text
validated ModuleInstances
canonical ModuleIdentities
file-local import bindings
an acyclic ModuleGraph
module-wide declaration namespaces
public ModuleSurfaces
stable cross-module declaration identities
```

and when:

- `internal` access is enforced during import resolution;
- module and import semantics are independent of host filesystem ordering and
  path conventions;
- cycle diagnostics are deterministic and navigable;
- separate-compilation surfaces support ordinary importer resolution;
- LSP edits update module/import state without restart;
- no later ABI or linking stage must rediscover module ownership or identity.

---

## 26. Related normative rulebooks

This rulebook must remain semantically consistent with at least:

```text
rules/projects/projects.txt
rules/foundations/grammar.md
rules/foundations/names_scopes_visibility.md
rules/foundations/lexical_structure.md
rules/declarations/functions.md
rules/declarations/generics.md
rules/declarations/interfaces.md
rules/declarations/impl.md
rules/declarations/properties.md
rules/compiler/compiler_pipeline.md
rules/compiler/semantic_ir.md
rules/platform/platform_model.md
rules/tooling/diagnostics.txt
rules/tooling/lsp.md
rules/tooling/testing.md
```

`rules/compiler/initialization.md`, future linking, dependency/package, and
incremental-compilation rules consume the module identities and surfaces defined
here and must not redefine their source-level meaning. The ModuleGraph is not a
runtime initialization graph.

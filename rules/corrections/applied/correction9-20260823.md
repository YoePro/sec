# Correction 9 — compiler-known `impl` privilege can be forged by source-file path

## Audit context

- Repository: `github.com/YoePro/sec`
- Repository baseline: `c515862`
- Audited: `2026-08-19`
- Primary rulebook: `rules/declarations/impl.md`
- Document revision: **2.0**
- Created: `2026-08-13`
- Last updated: `2026-08-14`

## Classification

Implementation bug.

This is not a language-rule change.

Revision 2.0 permits compiler/core code to define privileged implementations for
compiler-known or fundamental types only where the core-library rules permit
that privilege.

It explicitly requires:

> User code must not use that privilege to extend compiler-known types.

Current Sema decides whether a compiler-known implementation is privileged from
the source token's **path string**. A user-controlled source file can therefore
obtain core privilege merely by being placed below a directory named
`sec/core`.

---

## Normative requirement

`rules/declarations/impl.md` revision 2.0 establishes:

- ordinary `impl` targets a user-defined named nominal type;
- compiler/core code may receive an explicit privilege to implement
  compiler-known/fundamental types;
- user code must not receive that privilege;
- the privilege is a semantic/coherence boundary, not a filename convention.

For example, ordinary user code must not be able to add members globally to:

```text
string
int
Result
Option
RawPtr
...
```

simply by choosing a source location that looks like the compiler's core tree.

---

# Bug — privilege is granted from an untrusted path substring

## Affected code

`internal/sema/analyzer.go`

The relevant check is:

```go
func isAllowedCoreBuiltinImpl(target string, token lexer.Token) bool {
    if token.File == "" {
        return false
    }

    path := filepath.ToSlash(filepath.Clean(token.File))

    if !strings.Contains(path, "/sec/core/") &&
       !strings.HasPrefix(path, "sec/core/") {
        return false
    }

    return isCoreBuiltinImplTarget(target)
}
```

`validateImplTarget()` then permits a non-named compiler-known target when this
function returns true:

```go
if !target.Named &&
   target.Kind != InvalidType &&
   !isAllowedCoreBuiltinImpl(impl.Target.Name, impl.Target.Token) {
    ...
}
```

The file path is therefore serving as the authorization mechanism.

---

# Exploit by ordinary source layout

A user can create a source file with a path such as:

```text
my-project/sec/core/string.sec
```

or:

```text
/tmp/work/sec/core/string.sec
```

and place ordinary user code in it:

```sec
module app

impl string {
    fn UserDefinedGlobalExtension() int {
        return 42
    }
}
```

For a token whose `File` is:

```text
my-project/sec/core/string.sec
```

the normalized path contains:

```text
/sec/core/
```

and `isAllowedCoreBuiltinImpl("string", ...)` returns true.

There is no additional proof in `validateImplTarget()` that:

- this file came from the compiler-owned core package;
- the compiler selected it as trusted core source;
- the module is the canonical core module for the target;
- the source root is the compiler installation's designated core root;
- the file was loaded through a trusted core-library loading path rather than
  supplied by the user.

Thus source provenance can be forged by directory naming.

---

# CLI/source-loading relevance

The compiler CLI accepts user-provided files, directories, and globs for
analysis, and explicit user-provided source files for build/emission commands.

Therefore `token.File` cannot itself be treated as trusted authority.

Even if the normal project layout happens not to use `sec/core`, an authorization
rule must not rely on the absence of a user choosing that pathname.

---

# Existing tests do not protect the boundary

The current tests establish:

1. `sec/core/string.sec` may implement builtin `string`;
2. `app/string.sec` may not.

That verifies only the path heuristic itself.

It does **not** verify that an ordinary user source root containing a nested
`sec/core` directory is denied privilege.

The positive test also constructs the trusted-looking path directly:

```go
filepath.Join("sec", "core", "string.sec")
```

which makes the test unable to distinguish:

```text
compiler-selected trusted core source
```

from:

```text
user source with a trusted-looking pathname
```

---

# Required correction

## 1. Make trust explicit compiler metadata

The authorization decision must consume trusted source provenance supplied by
the compiler/source loader.

For example, each parsed source unit may carry an immutable provenance category:

```text
UserSource
CoreSource
StdlibSource
PlatformSource
GeneratedTrustedSource
...
```

The exact representation belongs to compiler architecture, but it must be
assigned by a trusted loader/compilation plan rather than inferred by Sema from a
path substring.

`impl` privilege should then require an explicit compiler-owned capability such
as:

```text
source provenance == CoreSource
AND target is explicitly core-extendable
```

## 2. Do not infer authorization from pathname

Remove authorization logic based solely on:

```go
strings.Contains(path, "/sec/core/")
strings.HasPrefix(path, "sec/core/")
```

A canonical path may still be used for diagnostics, source identity, cache keys,
or provenance validation by the source loader, but it must not itself confer
semantic privilege after the source has entered Sema.

## 3. Keep the target allowlist explicit

The existing `isCoreBuiltinImplTarget()` concept is useful as one half of the
boundary:

```text
trusted source
AND
explicitly permitted compiler-known target
```

Do not replace it with a blanket "all core files may implement any intrinsic
type" rule.

The target allowlist should ultimately be governed by the canonical
compiler-known/core rulebooks rather than duplicated ad hoc.

## 4. Preserve user/core module coherence

Where the owning core rulebook assigns particular intrinsic implementations to
particular core modules, validate that relationship from semantic module
identity plus trusted provenance.

Do not use a module name alone as authorization either; user source can write a
module declaration.

## 5. Propagate provenance through assembled programs

The privilege must remain correct when:

- multiple files are assembled;
- core and user modules are analyzed together;
- LSP analyzes workspace plus core sources;
- generated sources participate;
- paths are relative or absolute;
- symlinks are involved;
- the project itself has directories named `sec` or `core`.

Authorization must be a property of the source unit selected by the compiler,
not the spelling of `Token.File`.

---

# Required regression tests

Add tests covering at least:

1. compiler-selected trusted core `string` source may `impl string`;
2. ordinary `app/string.sec` may not `impl string`;
3. user source at `project/sec/core/string.sec` may not `impl string`;
4. user source at an absolute `/tmp/project/sec/core/string.sec` may not gain
   privilege;
5. a user-declared module named `string` does not gain core privilege;
6. a user-declared module named `core` does not gain core privilege;
7. a user source path containing multiple `sec/core` path fragments remains
   unprivileged;
8. symlink/path normalization cannot convert user provenance into core
   provenance;
9. trusted core provenance for a target not in the explicit allowlist is
   rejected;
10. trusted core provenance plus an explicitly permitted compiler-known target is
    accepted;
11. LSP and command-line Sema use the same provenance boundary;
12. assembled core + user programs preserve provenance independently per source
    file.

---

# Governance note

`frontend.impl-extensions` and `frontend.impl-lifecycle-construction` do not
currently record this privilege boundary.

Do not downgrade ordinary user-defined `impl`, extension composition, or
lifecycle construction broadly.

Add a focused partial item for compiler-known/core implementation authority.

There is also a separate governance wording correction discovered in the same
audit:

Current lifecycle governance says:

```text
zero-argument implicit new construction when canonical default construction
exists and no explicit init group exists
```

The code and revision-2.0 rule are actually more complete than that statement.
Current Sema permits implicit `new Type()` when:

- the type is defaultable; and
- no matching zero-argument `init()` exists,

even if other `init(...)` overloads exist.

The governance implemented wording should be corrected to reflect the actual
rule rather than downgraded.

## Applied 2026-08-23

The AST program now carries compiler-supplied source provenance. Compiler and
LSP core loading mark files only after canonical-root and symlink resolution,
and Sema grants compiler-known implementation authority only to sources marked
as trusted core. Path spelling is no longer an authorization mechanism. Focused
tests cover trusted core and forged core-looking paths.

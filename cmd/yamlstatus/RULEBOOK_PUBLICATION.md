# Rulebook HTML publication

Owner: `governance.yamlstatus-rulebook-publication` in
`governance/governance.yaml`.

## Generate or validate the site

Use `yamlstatus -?` for general syntax and `yamlstatus help books` (or
`yamlstatus books --help`) for all publication options. Help does not generate
files or require a source checkout.

Build the tool with portable source paths:

```sh
go build -trimpath -o bin/yamlstatus ./cmd/yamlstatus
```

From a checkout, generate into a separate destination:

```sh
bin/yamlstatus books --outdir ../sec-rulebooks
bin/yamlstatus books --indir "$SEC_CHECKOUT" --outdir "$SEC_BOOK_OUTPUT"
bin/yamlstatus books --indir "$SEC_CHECKOUT" --check
```

`--indir` defaults to the current directory and names the **checkout root**,
containing `rules/README.md` and `language-rulebook-status.md`. `--outdir` is
required for generation. It can be a new directory, including new parent
directories, but must be outside the source checkout. `--check` renders and
validates entirely in memory; it does not create output, lock or staging files.
Open the destination's `index.html` locally or serve that directory using any
static web server. No Go server or browser-side JavaScript is needed.

Links are relative by default, so the same output works locally and under a
web-server subdirectory. To explicitly target a deployment URL, use
`--base-url /books/` or `--base-url https://example.com/books/`.

For cron, use an absolute binary path and explicit absolute source/destination
paths; cron need not start in the checkout. For example:

```cron
SEC_CHECKOUT=/srv/sec
SEC_BOOK_OUTPUT=/var/www/sec-rulebooks
15 3 * * * /usr/local/bin/yamlstatus books --indir "$SEC_CHECKOUT" --outdir "$SEC_BOOK_OUTPUT" >> /var/log/sec-rulebooks.log 2>&1
```

CI can run the same `books --check`, generate with `books --outdir` into an
artifact directory outside the checkout, then archive that directory. Generation
does not upload files or change hosting configuration. Failures return nonzero;
successful runs print stable book/file counts. A per-destination lock rejects
overlapping runs instead of allowing two cron processes to replace each other.

## Inventory, navigation and references

Only explicit `.md`/`.txt` rows marked Written (including sync-required books)
or Living in `language-rulebook-status.md` are published. Duplicate references
produce one book. Categories/order/descriptions come from the directory map in
`rules/README.md`. Missing selected sources and unknown categories are errors.
Planned, Covered, Deferred, Candidate, repository-sync-pending and other inactive
entries appear as unavailable in the index; implementation artifacts and
directory-only inventory entries do not become book pages. Corrections,
packages, historical snapshots and amendments are not discovered recursively.

The output contains a main index, category indexes and one HTML file per book.
Page metadata and headings supply the title, document revision, status,
breadcrumbs and contents list. Heading IDs retain Unicode and section numbers;
duplicate headings receive stable numeric suffixes. Explicit IDs/named anchors
are preserved, collisions are diagnosed, and the source's first title appears
once in the page header with its anchor.

Markdown/HTML book links are resolved against the source book, canonical paths
and unambiguous bare filenames, then rewritten to the generated HTML while
preserving queries/fragments. Unambiguous book filenames in prose and inline
code also become links. Fenced examples and plain-text books stay literal.
References to inactive/history/repository-only files remain labelled text rather
than broken public links. Unknown local book links and missing anchors fail
validation. No external URLs are fetched or checked.

Local PNG, JPEG, GIF, WebP, AVIF and ICO images inside `rules/` are copied to
`assets/` and rewritten. Unsupported formats, missing images and escaping paths
are errors. Other repository files are not copied. Source-book external image
URLs remain external; rendering itself makes no network requests.

Both `--check` and generation validate the final HTML links, anchors and asset
targets, including generated navigation. Diagnostics identify the source or
generated page; unique source references include their line number. Output is
not replaced if validation fails.

## Rebuilding an existing destination

All pages/resources are prepared before installation, then written into a fresh
private sibling staging directory. The `.sec-rulebooks.json` ownership manifest
records generated paths and SHA-256 checksums. Pages and manifest bytes are
deterministic: no build time, random value or machine-specific source path is
embedded. The manifest must remain with the site.

On regeneration, unchanged previously generated files are replaced and stale
generated files are removed. Unrelated regular files/directories are preserved,
including file permissions/modification times and directory permissions. A
manually modified generated file, invalid manifest, symlink or unrelated file
at a generated destination causes an error and leaves the installed site alone.
Directory replacement restores the old directory if installation returns an
error, so pages are not installed one at a time. There is no upload/deployment
integration and no promise of an uninterrupted directory path during the swap.

## Implemented: body rendering

`renderRulebookBody` accepts a source filename and UTF-8 bytes and returns a
sanitized HTML fragment. It does not read files, fetch links, write output or
interpret examples. The caller supplies the bytes. Invalid UTF-8 and unsupported
extensions return a contextual error without partial HTML.

Markdown uses **Goldmark v1.8.7** (MIT), with GitHub Flavored Markdown enabled.
It supports CommonMark prose, Unicode, tables, fenced code and language labels,
nested lists, blockquotes, links, images and strikethrough. GFM task-list text is
preserved; interactive checkbox controls are stripped by the HTML policy.

Goldmark was selected for its documented CommonMark/GFM support, structured AST
for future link/navigation work, pure Go implementation and permissive license.
See its [upstream documentation](https://github.com/yuin/goldmark/tree/v1.8.7)
and [license](https://github.com/yuin/goldmark/blob/v1.8.7/LICENSE).

Raw HTML is parsed and then the **entire rendered fragment** passes through
**bluemonday v1.0.27** (BSD-3-Clause), using its UGC allowlist plus a narrowly
restricted `language-*` class on code elements. Harmless markup such as line
breaks, details/summary and explicit anchors is retained. Scripts, event
handlers, inline styles, executable URL schemes and embedded frames are
removed. No unsafe rendering switch is exposed. See the sanitizer's
[documentation](https://github.com/microcosm-cc/bluemonday/tree/v1.0.27) and
[license](https://github.com/microcosm-cc/bluemonday/blob/v1.0.27/LICENSE.md).
Future page templates must escape metadata with `html/template`; sanitizing
the body does not sanitize subsequently appended markup.

Both current `.txt` rulebooks, `rules/mlir/mlir.txt` and
`rules/mlir/mlir-optimize.txt`, render as escaped preformatted text. Their mixed
prose, indentation and literal diagrams are preserved without guessing a
Markdown dialect. This format policy applies to any selected `.txt` book;
it does not automatically select files for publication.

## Implemented: HTML presentation

`renderRulebookPage` builds a complete HTML document from source bytes and
caller-provided title, category, revision, documentation status and navigation.
The body always passes through `renderRulebookBody`; page metadata is escaped
by `html/template`. Invalid titles, unsupported navigation URL schemes and
body-rendering errors return no partial page.

`rulebook_page.html` and `rulebook_page.css` are embedded separately from Go.
The page uses the Sec manual's header/sidebar/breadcrumb/content/footer layout
with locally packaged green styling and system fonts. No page-chrome image,
stylesheet, font or script needs a network request. Images explicitly linked
inside a source book follow the resource-copy/link policy above.

The presentation supports supplied navigation groups, current-page markers,
breadcrumbs and an optional contents list. The layout includes a keyboard skip
link, visible focus indicators, semantic navigation landmarks, responsive
mobile navigation, and locally scrollable code blocks and wide tables. Metadata
labels distinguish documentation status from implementation progress.

The caller must supply links relative to the generated page (including its
deployment subpath). Presentation validates URL syntax and schemes, but does
not establish that a target exists. Automatic inventory selection, navigation
generation, heading anchors and destination validation are supplied by the
publication command described above.

## Implemented: confined publication workspace

`openRulebookWorkspace` opens existing source and output/staging directories.
It resolves root symlink aliases and rejects identical or nested roots in
either direction, so generated output cannot be placed inside the sources
and the source tree cannot be inside the output tree. Directory access uses
Go's `os.Root`, which checks confinement during filesystem operations.

`ReadSource` reads regular local book/resource files; relative symlinks work
only when their target stays inside the source root. `RenderPage` combines
that read with the existing metadata escaping and final-body sanitization.
It returns bytes for the caller to validate before publication.

All IO paths must be decoded, slash-separated relative filesystem paths.
Absolute paths, URLs, drive prefixes, backslashes, empty/dot/parent components,
control characters, invalid UTF-8 and percent escapes are rejected. The link
resolver must decode URL paths before supplying a filesystem path; the
workspace never repeatedly decodes or cleans an ambiguous path into a target.

`WriteArtifact` creates new files beneath the output root, creating parent
directories as needed. Exclusive creation rejects existing files, leaf
symlinks and hard links, so it cannot overwrite a source or unrelated file.
Escaping parent symlinks are rejected by `os.Root`. Failed writes remove their
new partial artifact. Rebuilds use the fresh staging directory and manifest
replacement described above. The workspace itself never downloads resources, executes
source examples or deploys output.

The existing statistics `-html` output is unchanged. Documentation publication
does not establish that the corresponding Sec language behavior is implemented.

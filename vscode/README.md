# SEC Language Extension

This repository contains a minimal Visual Studio Code extension for the SEC language.

## Features

- Syntax highlighting for `.sec` and `.se` files
- SEC file icons in light and dark VS Code themes
- Comment support for `//` line comments and `/* ... */` block comments
- Keyword highlighting for SEC language constructs
- Current grammar coverage for modules, imports, target directives, functions,
  structs, impl blocks, properties, enums, match/switch/if/for/while, units,
  numeric literal suffixes and common operators
- Unit highlighting for `decimal<m>`, shorthand `<m>`, unit declarations,
  categories (`physical`, `currency`, `other`) and metadata such as `Dimension`,
  `Scale`, `System`, `LongName`, `Symbol`, `BaseUnit`, and `Status`
- Type highlighting for built-in SEC types
- String interpolation highlighting for `$"..."`
- Format on save through the SEC language server.

## Language Highlights

SEC is a compiled language with a strong static and semantic type system. The syntax supports:

- `let` / `let mut`
- `type` declarations with range contracts and units of measure
- generics like `Result[T,E]`, `Vec[T]`, `Map[K,V]`, `Option[T]`
- explicit unsafe code
- properties, methods, interfaces, and delegation
- `void`, `decimal`, `decimal128`, `(u)int128`, `(u)int256`, `rune`, and other primitive types

## Language Server Direction

The extension currently provides TextMate syntax highlighting only.

The intended source-code server path is a Sec Language Server Protocol (LSP)
process, for example `sec lsp` or `sec-lsp`, that reuses the compiler's lexer,
parser, formatter, semantic analyzer and diagnostics. The VS Code extension
should talk to that server over stdio instead of reimplementing compiler logic
in TypeScript.

This repository now contains a first server entrypoint at `cmd/lsp`. Build it
with:

```bash
go build -o bin/lsp-sec ./cmd/lsp
GOOS=windows GOARCH=amd64 go build -o bin/lsp-sec.exe ./cmd/lsp
```

When the extension is installed/copied, put the language-server binaries in the
extension root:

```text
~/.vscode-server/extensions/sec-lang.sec-syntax/bin/lsp-sec
C:\Users\Accountname\.vscode\extensions\sec-lang.sec-syntax\bin\lsp-sec.exe
```

The extension first checks the `sec.languageServer.path` setting. If that is
empty, it looks for `bin/lsp-sec` or `bin/lsp-sec.exe` inside the extension. In
repo-development mode it also checks `../bin/lsp-sec`.

Example user setting on Windows:

```json
{
  "sec.languageServer.path": "C:\\Users\\Accountname\\.vscode\\extensions\\sec-lang.sec-syntax\\bin\\lsp-sec.exe"
}
```

Initial LSP features should be:

- parse and sema diagnostics with file-aware locations
- warnings for deprecated or obsolete units
- document formatting on save through the language server
- hover text for unit metadata such as `LongName`, `Symbol`, `Dimension`, and
  `Scale`
- completion for keywords, units, types, functions, enum values, and metadata
  fields

## Files

- `package.json` - VS Code extension manifest
- `language-configuration.json` - bracket and comment rules
- `images/sec-language.svg` - default language file icon
- `syntaxes/sec.tmLanguage.json` - TextMate grammar for syntax highlighting

## Usage

1. Open the repository in VS Code.
2. Run the `Extensions: Load Extension` command or use the VS Code Extension Development Host.
3. Open a `.sec` file and verify syntax highlighting.

## Contributing

Contributions are welcome. Please open issues or pull requests for grammar improvements, support for additional SEC syntax, or language server integration.

### Optional parameter analysis

`sec.analysis.parameters.hover` selects `off` (default), `concise`, or
`detailed`. Concise hover reports the resolved parameter's access, lifetime,
ownership demand, or incomplete analysis. Detailed hover includes capability
requirements, precision, and the compiler's narrowing candidates and blockers.
This information is available on parameter declarations/uses and callable
hovers; it comes from Sema, including transitive call demand.

`sec.analysis.parameters.advisories` selects `off` (default), `info`, `warning`,
or `error` for A2001 large-value parameter advice. This setting affects LSP
presentation only. Language-safety errors remain visible and compiler validity
is unchanged. Changes take effect without restarting the language server and
refresh diagnostics on open documents. The analysis depth continues to follow
`[analysis].lsp_depth` in `.sec/sec.toml` (`interactive`, `standard`, or `deep`).

Other LSP clients can send the same settings at initialization as
`{"analysis":{"parameters":{"hover":"concise","advisories":"info"}}}`,
or in `workspace/didChangeConfiguration` under `settings.sec`. Partial updates
preserve unspecified choices; unsupported values preserve previous choices.

# SEC Language Extension

This repository contains a minimal Visual Studio Code extension for the SEC language.

## Features

- Syntax highlighting for `.sec` files
- Comment support for `//` line comments and `/* ... */` block comments
- Keyword highlighting for SEC language constructs
- Type highlighting for built-in SEC types
- String interpolation highlighting for `$"..."`

## Language Highlights

SEC is a compiled language with a strong static and semantic type system. The syntax supports:

- `let` / `let mut`
- `type` declarations with range contracts and units of measure
- generics like `Result[T,E]`, `Vec[T]`, `Map[K,V]`, `Option[T]`
- explicit unsafe code
- properties, methods, interfaces, and delegation
- `void`, `decimal`, `bytes`, `rune`, and other primitive types

## Files

- `package.json` - VS Code extension manifest
- `language-configuration.json` - bracket and comment rules
- `syntaxes/sec.tmLanguage.json` - TextMate grammar for syntax highlighting

## Usage

1. Open the repository in VS Code.
2. Run the `Extensions: Load Extension` command or use the VS Code Extension Development Host.
3. Open a `.sec` file and verify syntax highlighting.

## Contributing

Contributions are welcome. Please open issues or pull requests for grammar improvements, support for additional SEC syntax, or language server integration.

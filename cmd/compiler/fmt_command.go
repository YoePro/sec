package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sec/internal/formatconfig"
	secformatter "sec/internal/formatter"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// formatCheckError reports every selected file that the shared formatter would
// change, without classifying formatting differences as language errors.
// Rules: rules/tooling/formatter.md — Command model, `sec fmt --check <path>`;
// Diagnostics.
type formatCheckError struct {
	paths     []string
	malformed []string
}

// Error renders stable formatting diagnostic IDs and affected filenames.
// Rules: rules/tooling/formatter.md — Diagnostics.
func (e *formatCheckError) Error() string {
	var lines []string
	for _, path := range e.paths {
		lines = append(lines, fmt.Sprintf("%s: format.noncanonical-source: formatting would change this file", path))
	}
	for _, path := range e.malformed {
		lines = append(lines, fmt.Sprintf("%s: format.malformed-source: formatter preserved malformed or incomplete source", path))
	}
	return strings.Join(lines, "\n")
}

// runFmtCommand formats explicit files and recursively selected directories
// in place (MD-037), checks them without writes, or formats standard input to
// standard output.
// Check differences return an error so the CLI exits non-zero after reporting
// all affected files. LSP uses the same formatter.
// Rules: rules/tooling/formatter.md — Command model, `sec fmt <path>` and
// `sec fmt --check <path>` and `sec fmt --stdin`; Shared implementation; Diagnostics.
//
// Effective options follow formatter.md § 4(18)–(23): explicit options of this
// invocation, then the file's project `[format]` table, then the defaults.
// Every file's configuration is validated before any file is written.
func runFmtCommand(args []string) error {
	var paths []string
	check, stdin, literalPaths := false, false, false
	var overrides formatconfig.Overrides
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case literalPaths:
			paths = append(paths, arg)
		case arg == "--":
			literalPaths = true
		case arg == "--check":
			check = true
		case arg == "--stdin":
			stdin = true
		case arg == "--fix":
			// rules/tooling/lsp.md — "Shared formatter": `sec fmt --fix`
			// applies the opt-in Language Corrections of formatter.md §§ 26–27.
			enabled := true
			overrides.LanguageCorrections = &enabled
		case arg == "--indentation-width" || strings.HasPrefix(arg, "--indentation-width="):
			raw, inline := strings.CutPrefix(arg, "--indentation-width=")
			if !inline {
				if index+1 >= len(args) {
					return fmt.Errorf("--indentation-width requires a value")
				}
				index++
				raw = args[index]
			}
			width, err := strconv.Atoi(raw)
			if err != nil {
				return fmt.Errorf("--indentation-width must be an integer, got %q", raw)
			}
			overrides.IndentationWidth = &width
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown fmt option: %s", arg)
		default:
			paths = append(paths, arg)
		}
	}
	if stdin {
		if check || len(paths) > 0 {
			return fmt.Errorf("--stdin cannot be combined with --check or source file paths")
		}
		settings, err := formatconfig.ForSource("", overrides)
		if err != nil {
			return err
		}
		return formatStdinWithOptions(os.Stdin, os.Stdout, settings.Options)
	}
	if len(paths) == 0 {
		return fmt.Errorf("expected at least one source file or directory")
	}
	// Preflight (formatter.md § 28(13)): select and read every file and
	// resolve every configuration before the first write.
	selected, err := selectFmtSources(paths)
	if err != nil {
		return err
	}
	options := make([]secformatter.Options, len(selected))
	for index, file := range selected {
		settings, err := formatconfig.ForSource(file.absolute, overrides)
		if err != nil {
			return err
		}
		options[index] = settings.Options
	}
	// A malformed file is left unchanged and reported, and its valid
	// siblings are still formatted.
	var changed, malformed []string
	for index, file := range selected {
		if err := file.validate(); err != nil {
			return err
		}
		result := formatSelectedSource(file, options[index])
		if result.Malformed {
			malformed = append(malformed, file.display)
			continue
		}
		formatted := result.Text
		if formatted == string(file.input) {
			continue
		}
		if check {
			changed = append(changed, file.display)
			continue
		}
		if err := replaceFormattedFile(file, []byte(formatted)); err != nil {
			return err
		}
	}
	if len(changed) > 0 || len(malformed) > 0 {
		return &formatCheckError{paths: changed, malformed: malformed}
	}
	return nil
}

// formatStdin formats a stream through the shared formatter with LF line endings
// and propagates read/write failures to the CLI.
// Rules: rules/tooling/formatter.md — Command model, `sec fmt --stdin`;
// Shared implementation; rules/foundations/lexical_structure.md — §2 "Line endings".
func formatStdin(input io.Reader, output io.Writer) error {
	return formatStdinWithOptions(input, output, formatconfig.Defaults().Options)
}

// formatStdinWithOptions formats a stream with the selected formatter
// options, such as the opt-in Language Corrections of `--fix`.
func formatStdinWithOptions(input io.Reader, output io.Writer, options secformatter.Options) error {
	source, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	text := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(string(source))
	formatted := secformatter.Format(secformatter.Source{Text: text}, options).Text
	n, err := io.WriteString(output, formatted)
	if err == nil && n != len(formatted) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fmt.Errorf("write stdout: %w", err)
	}
	return nil
}

// replaceFormattedFile atomically installs a successful format only while the
// selected file still matches its preflight snapshot, preserving permissions.
// Rules: rules/tooling/formatter.md — §28(12)–(13).
func replaceFormattedFile(file fmtSelection, contents []byte) error {
	path, mode := file.absolute, file.mode
	temporary, err := os.CreateTemp(filepath.Dir(path), ".sec-fmt-*")
	if err != nil {
		return fmt.Errorf("%s: create temporary file: %w", path, err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("%s: preserve permissions: %w", path, err)
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("%s: write formatted source: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("%s: close formatted source: %w", path, err)
	}
	if err := file.validate(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("%s: replace source: %w", path, err)
	}
	removeTemporary = false
	return nil
}

// formatSelectedSource implements the CLI's whole-file malformed-input policy.
// Ordinary formatting never repairs parser-uncertain syntax. Explicit language
// corrections may repair recognized forms, but must leave a valid complete
// parse before a source can be replaced. LSP safe-region formatting is separate.
// Rules: rules/tooling/formatter.md — §§25,26–27,28(13).
func formatSelectedSource(file fmtSelection, options secformatter.Options) secformatter.Result {
	valid := func(text string) bool {
		p := parser.New(lexer.NewWithFile(text, file.display))
		p.ParseProgram()
		return len(p.Errors()) == 0
	}
	original := string(file.input)
	if !options.Fix && !valid(original) {
		return secformatter.Result{Text: original, Malformed: true}
	}
	result := secformatter.Format(secformatter.Source{Text: original}, options)
	if !result.Malformed && !valid(result.Text) {
		return secformatter.Result{Text: original, Malformed: true}
	}
	return result
}

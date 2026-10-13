package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/project"
	"sec/internal/sema"
)

// initTargetSource chooses ordinary source directories, keeping executable
// entries below cmd and generated tests in their own ordinary source module.
// Rules: rules/projects/projects.md — §§13,15(2),18,39; modules.md — §§3,4.
func initTargetSource(kind, name string) string {
	if kind == "command" || kind == "firmware" {
		if name == "internal" {
			name += "_entry"
		}
		return "cmd/" + name
	}
	module := strings.ReplaceAll(name, "-", "_")
	if !project.ValidModuleName(module) || module == "internal" || module == "bin" || module == "cmd" || module == "platform" {
		module += "_module"
	}
	if kind == "test" {
		return "tests/" + module
	}
	return module
}

// validateInitTargetSource rejects host/traversal/reserved paths before planning.
// Rules: rules/projects/projects.md — §§15,17(5); modules.md — §7.
func validateInitTargetSource(source string) error {
	if source == "." {
		return nil
	}
	parts := strings.Split(source, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || part == ".sec" || strings.ContainsAny(part, "\\:\x00\r\n") {
			return fmt.Errorf("invalid Target source path %q", source)
		}
	}
	if parts[0] == "bin" || parts[0] == "platform" || source == "cmd" || parts[len(parts)-1] == "internal" {
		return fmt.Errorf("reserved Target source path %q", source)
	}
	return nil
}

// initTargetStarter follows the callable entry contract for each Target kind;
// library and test products do not receive a source main or friend visibility.
// Rules: rules/compiler/initialization.md — §§20,25–27; projects.md — §§18,39; testing.md — §5.
func initTargetStarter(manifest *project.Manifest, kind, source string) (string, string, string, error) {
	module := filepath.Base(filepath.FromSlash(source))
	if kind == "command" || kind == "firmware" {
		module = "main"
	} else if source == "." {
		var err error
		module, err = manifest.RootModuleName()
		if err != nil {
			return "", "", "", err
		}
	}
	if !project.ValidModuleName(module) {
		return "", "", "", fmt.Errorf("Target source must end in a valid ModuleName")
	}
	header := "module " + module + "\n"
	switch kind {
	case "command":
		return module, "main.sec", defaultMainSource(), nil
	case "firmware":
		return module, "main.sec", header + "\nfn main() void {\n}\n", nil
	case "test":
		return module, module + "_test.sec", header + "\ntest \"initial test\" {\n}\n", nil
	default:
		return module, module + ".sec", header, nil
	}
}

// validateInitEntrySignature delegates callable compatibility to the compiler's
// entry checker. Bodies are omitted: init preserves user implementation code
// and does not attempt to resolve its dependency graph or prove body semantics.
// Rules: rules/compiler/initialization.md — §§20–22,25; projects.md — §42.
func validateInitEntrySignature(facts initModuleSourceFacts, module, kind string) error {
	program := &ast.Program{Statements: []ast.Statement{&ast.ModuleStatement{Path: module}}}
	for _, declaration := range facts.mainDeclarations {
		signature := *declaration
		signature.Body = nil
		program.Statements = append(program.Statements, &signature)
	}
	analyzer := sema.NewAnalyzer()
	_ = analyzer.Analyze(program)
	failures := analyzer.ValidateProgramEntry(sema.ProgramTargetKind(kind), module, lexer.Token{})
	if len(failures) != 0 {
		return fmt.Errorf("init entry conflict: %s", failures[0].Message)
	}
	return nil
}

// existingInitEntryModule preserves an already declared ordinary entry module
// whose name matches its directory; main is the canonical generated default.
// Rules: rules/projects/projects.md — §15.2(4),§42; modules.md — §§3–4.
func existingInitEntryModule(directory, fallback string) string {
	data, err := os.ReadFile(filepath.Join(directory, "main.sec"))
	if err != nil {
		return fallback
	}
	p := parser.New(lexer.New(string(data)))
	program := p.Parse().Program
	name := programModulePath(program)
	if len(p.Errors()) == 0 && name == filepath.Base(directory) && project.ValidModuleName(name) {
		return name
	}
	return fallback
}

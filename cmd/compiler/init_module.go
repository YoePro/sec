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
)

// validateInitModulePath applies logical path spelling and reserved project roots.
// Internal is an ordinary access marker and does not invalidate a module path.
// Rules: rules/projects/projects.md — §15,§38(4–6); rules/projects/modules.md — §§2.1,7,8.
func validateInitModulePath(path string) (string, error) {
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "\\:\x00\r\n") {
			return "", fmt.Errorf("invalid module path %q: use a relative logical path with / separators and no empty or traversal segments", path)
		}
		if part == ".sec" {
			return "", fmt.Errorf("module path cannot enter the .sec metadata directory")
		}
	}
	if parts[0] == "bin" || parts[0] == "platform" || (parts[0] == "cmd" && len(parts) == 1) {
		return "", fmt.Errorf("module path %q uses a reserved project or import root", path)
	}
	name := parts[len(parts)-1]
	if name == "internal" {
		return "", fmt.Errorf("internal is an access marker; create a child module below it")
	}
	if !project.ValidModuleName(name) {
		return "", fmt.Errorf("module path %q must end in a valid ModuleName identifier", path)
	}
	return name, nil
}

// initModule preflights the complete one-file change, preserves compatible user
// sources, and creates no Target or manifest. Creation uses exclusive writes;
// failures remove only files/directories created by this operation.
// Rules: rules/projects/projects.md — §§38,40,42,43,45; rules/projects/modules.md — §§3,4,7.
func initModule(options initCommandOptions) error {
	name, err := validateInitModulePath(options.ModulePath)
	if err != nil {
		return err
	}
	manifest, ok, err := discoverInitManifest(options.ProjectDir)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("sec init module requires an existing project with .sec/sec.toml")
	}
	if _, err = readInitIdentity(manifest); err != nil {
		return err
	}
	root := filepath.Dir(filepath.Dir(manifest))
	destination := filepath.Join(root, filepath.FromSlash(options.ModulePath))
	// Refuse symlinks and nested boundaries before creating any directories.
	for current := root; ; {
		if current != root && fileExists(filepath.Join(current, ".sec", "sec.toml")) {
			return fmt.Errorf("nested project manifest at %s", current)
		}
		info, statErr := os.Lstat(current)
		if statErr == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
			return fmt.Errorf("module directory %s is not an ordinary directory", current)
		}
		if statErr != nil && !os.IsNotExist(statErr) {
			return statErr
		}
		if current == destination {
			break
		}
		relative, _ := filepath.Rel(current, destination)
		current = filepath.Join(current, strings.Split(relative, string(filepath.Separator))[0])
	}

	if _, err := inspectInitModuleSources(destination, name); err != nil {
		return err
	}
	sourcePath := filepath.Join(destination, name+".sec")
	existed := fileExists(sourcePath)
	plan := &initPlan{}
	if err := plan.addFile(sourcePath, []byte("module "+name+"\n"), true); err != nil {
		return err
	}
	if err := executeInitPlan(plan, options.DryRun); err != nil {
		return err
	}
	if !options.DryRun {
		if existed {
			fmt.Printf("Sec module %s already exists\n", options.ModulePath)
		} else {
			fmt.Printf("initialized Sec module %s in %s\n", options.ModulePath, root)
		}
	}
	return nil
}

type initModuleSourceFacts struct {
	mainFunctions    int
	hasMainSource    bool
	mainDeclarations []*ast.FunctionDeclaration
}

// inspectInitModuleSources checks every existing source member for compatible
// parsing and a single matching module declaration before planning writes.
// Rules: rules/projects/modules.md — §§3,4; rules/projects/projects.md — §§42,43.
func inspectInitModuleSources(destination, name string) (initModuleSourceFacts, error) {
	facts := initModuleSourceFacts{}
	entries, err := os.ReadDir(destination)
	if err != nil && !os.IsNotExist(err) {
		return facts, err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sec") {
			continue
		}
		if entry.IsDir() {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return facts, fmt.Errorf("source conflict at %s", filepath.Join(destination, entry.Name()))
		}
		data, readErr := os.ReadFile(filepath.Join(destination, entry.Name()))
		if readErr != nil {
			return facts, readErr
		}
		l := lexer.New(string(data))
		p := parser.New(l)
		program := p.Parse().Program
		if entry.Name() == "main.sec" {
			facts.hasMainSource = true
		}
		moduleCount := 0
		for _, statement := range program.Statements {
			if fn, ok := statement.(*ast.FunctionDeclaration); ok && fn.Name != nil && fn.Name.Value == "main" {
				facts.mainFunctions++
				facts.mainDeclarations = append(facts.mainDeclarations, fn)
			}
			if _, ok := statement.(*ast.ModuleStatement); ok {
				moduleCount++
			}
		}
		if len(p.Errors()) != 0 || len(l.Diagnostics()) != 0 || moduleCount != 1 || programModulePath(program) != name {
			return facts, fmt.Errorf("source conflict: %s must declare module %s and parse successfully", filepath.Join(destination, entry.Name()), name)
		}
	}
	return facts, nil
}

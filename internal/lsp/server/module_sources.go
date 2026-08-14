package server

import (
	"os"
	"path/filepath"
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// SourceOverlay is an immutable compiler-input view for open documents. Keys
// are normalized filesystem paths and values are unsaved source snapshots.
type SourceOverlay map[string]string

func NormalizeSourcePath(path string) string {
	if path == "" {
		return ""
	}
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	return filepath.Clean(path)
}

func ReadSource(path string, overlay SourceOverlay) ([]byte, error) {
	if text, ok := overlay[NormalizeSourcePath(path)]; ok {
		return []byte(text), nil
	}
	return os.ReadFile(path)
}

// ParseSource runs the canonical compiler lexer/parser over an overlay or the
// on-disk source. Incomplete siblings are excluded from the combined module;
// their own document analysis remains responsible for recovery diagnostics.
func ParseSource(path string, overlay SourceOverlay) (*ast.Program, bool) {
	data, err := ReadSource(path, overlay)
	if err != nil {
		return nil, false
	}
	result := parser.New(lexer.NewWithFile(string(data), path)).Parse()
	if result.HasErrors {
		return nil, false
	}
	return result.Program, true
}

// AssembleModule replaces the active single-file statement list with the
// deterministic set of valid sibling files declaring the same module.
func AssembleModule(active *ast.Program, sourceFile string, overlay SourceOverlay) {
	module := ProgramModule(active)
	if active == nil || module == "" || filepath.Ext(sourceFile) != ".sec" {
		return
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(sourceFile), "*.sec"))
	if err != nil {
		return
	}
	seen := make(map[string]bool, len(matches))
	for _, match := range matches {
		seen[NormalizeSourcePath(match)] = true
	}
	sourceDir := NormalizeSourcePath(filepath.Dir(sourceFile))
	for path := range overlay {
		normalized := NormalizeSourcePath(path)
		if filepath.Ext(normalized) != ".sec" || NormalizeSourcePath(filepath.Dir(normalized)) != sourceDir || seen[normalized] {
			continue
		}
		seen[normalized] = true
		matches = append(matches, normalized)
	}
	activePath := NormalizeSourcePath(sourceFile)
	seenActive := false
	sort.Strings(matches)
	statements := make([]ast.Statement, 0, len(active.Statements))
	for _, match := range matches {
		if NormalizeSourcePath(match) == activePath {
			statements = append(statements, active.Statements...)
			seenActive = true
			continue
		}
		sibling, ok := ParseSource(match, overlay)
		if !ok || ProgramModule(sibling) != module {
			continue
		}
		statements = append(statements, sibling.Statements...)
	}
	if !seenActive {
		statements = append(statements, active.Statements...)
	}
	active.Statements = statements
}

func ProgramModule(program *ast.Program) string {
	if program == nil {
		return ""
	}
	for _, statement := range program.Statements {
		if module, ok := statement.(*ast.ModuleStatement); ok && module != nil {
			return module.Path
		}
	}
	return ""
}

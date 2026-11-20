package traceability

import (
	"bufio"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// update rewrites the baseline without the functions that now carry a
// reference or no longer exist; it never adds an entry.
var update = flag.Bool("update", false, "shrink testdata/baseline.txt to the still-unreferenced functions")

// semanticRoots are the semantic packages the policy names first: parser,
// Sema, Semantic IR, lowering, ABI/layout and diagnostics.
var semanticRoots = []string{
	"internal/parser", "internal/sema", "internal/ir/semantic",
	"internal/lowering", "internal/layout", "internal/diagnostics",
}

// rulebookReference accepts the documented reference forms: a rulebook
// path, a `Rules:` block, a section sign, a missing-decision ID or a
// package rulebook.
var rulebookReference = regexp.MustCompile(`rules/|Rules:|§|MD-[0-9]+|package[0-9]+\.md`)

const baselinePath = "testdata/baseline.txt"

// unreferencedFunctions lists every function and method of the semantic
// packages whose doc comment cites no rulebook, keyed by package directory,
// receiver and name so that moving a function between files keeps its key.
func unreferencedFunctions(t *testing.T) map[string]bool {
	t.Helper()
	found := map[string]bool{}
	for _, root := range semanticRoots {
		err := filepath.Walk(filepath.Join("..", "..", root), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && strings.HasPrefix(info.Name(), "testdata") {
				return filepath.SkipDir
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
			if err != nil {
				return err
			}
			dir := filepath.ToSlash(filepath.Dir(strings.TrimPrefix(path, filepath.Join("..", "..")+string(filepath.Separator))))
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Doc != nil && rulebookReference.MatchString(fn.Doc.Text()) {
					continue
				}
				found[dir+":"+functionName(fn)] = true
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return found
}

func functionName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	receiver := fn.Recv.List[0].Type
	pointer := ""
	if star, ok := receiver.(*ast.StarExpr); ok {
		pointer, receiver = "*", star.X
	}
	if index, ok := receiver.(*ast.IndexExpr); ok {
		receiver = index.X
	}
	if index, ok := receiver.(*ast.IndexListExpr); ok {
		receiver = index.X
	}
	name := "?"
	if ident, ok := receiver.(*ast.Ident); ok {
		name = ident.Name
	}
	return "(" + pointer + name + ")." + fn.Name.Name
}

func readBaseline(t *testing.T) map[string]bool {
	t.Helper()
	file, err := os.Open(baselinePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	baseline := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			baseline[line] = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return baseline
}

// TestRulebookTraceability requires every new or renamed semantic function to
// cite its rulebook; the functions written before the rule are the baseline,
// which only shrinks. A function that gains a reference is removed from the
// baseline with -update.
// Rules: rules/README.md — "Implementation traceability"; governance.source-rulebook-traceability.
func TestRulebookTraceability(t *testing.T) {
	current := unreferencedFunctions(t)
	baseline := readBaseline(t)
	var added, resolved []string
	for key := range current {
		if !baseline[key] {
			added = append(added, key)
		}
	}
	for key := range baseline {
		if !current[key] {
			resolved = append(resolved, key)
		}
	}
	sort.Strings(added)
	sort.Strings(resolved)
	if *update {
		var keep []string
		for key := range baseline {
			if current[key] {
				keep = append(keep, key)
			}
		}
		sort.Strings(keep)
		header := "# Semantic functions written before the traceability rule that do not cite a rulebook yet.\n# Add the reference and run: go test ./internal/traceability -run TestRulebookTraceability -update\n# Entries are only ever removed; never add one.\n"
		if err := os.WriteFile(baselinePath, []byte(header+strings.Join(keep, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("baseline shrank by %d to %d entries", len(resolved), len(keep))
	}
	if len(added) > 0 {
		t.Errorf("%d new semantic functions cite no rulebook in their doc comment (rules/README.md, \"Implementation traceability\"); add a `Rules:` line, a rulebook path or a § reference:\n  %s",
			len(added), strings.Join(added, "\n  "))
	}
	if len(resolved) > 0 && !*update {
		t.Logf("%d baseline entries now cite a rulebook or no longer exist; shrink the baseline with -update", len(resolved))
	}
}

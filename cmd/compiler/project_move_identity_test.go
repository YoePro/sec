package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const projectMoveManifest = `[project]
name = "app"
module = "main"
uuid = "e3856637-43bf-4166-bf83-07edd6e03c5d"

[version]
major = 1
minor = 0
revision = 0
build = 0
format = "[major].[minor].[revision]"
`

const projectMoveSource = `module main

static let mut counter: int := 0

fn Twice(x: int) int {
    return x * 2
}

fn main() int {
    counter = Twice(21)
    return 0
}
`

func writeProjectMoveFixture(t *testing.T, root string) string {
	t.Helper()
	for path, text := range map[string]string{
		filepath.Join(root, ".sec", "sec.toml"): projectMoveManifest,
		filepath.Join(root, "main.sec"):         projectMoveSource,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(root, "main.sec")
}

// TestProjectMoveKeepsEmittedOutput moves a Project to another directory and
// invokes the compiler through absolute paths: the Semantic IR (text and
// JSON), Sec MLIR and LLVM IR are byte-identical and contain no directory of
// either location. Source identities are Project-relative, so caches keyed on
// emitted output survive a Project directory move.
// Rules: rules/projects/projects.md — §§ 34, 50(1); rules/compiler/semantic_ir.md — § 5(1).
func TestProjectMoveKeepsEmittedOutput(t *testing.T) {
	base := t.TempDir()
	first := writeProjectMoveFixture(t, filepath.Join(base, "a", "app"))
	moved := writeProjectMoveFixture(t, filepath.Join(base, "b", "deeper", "path", "app"))
	commands := [][]string{{"emit-ir"}, {"emit-ir", "--json"}}
	if os.Getenv("SEC_MLIR_BIN") != "" {
		commands = append(commands, []string{"emit-sec-mlir"}, []string{"emit-llvm"})
	}
	for _, command := range commands {
		emit := func(file string) string {
			args := append(append([]string{command[0], file}, command[1:]...), "-o", "-")
			stdout, stderr, code := runCLIForDiagnostics(t, args...)
			if code != 0 {
				t.Fatalf("%v %s: exit %d\n%s", command, file, code, stderr)
			}
			return stdout
		}
		before, after := emit(first), emit(moved)
		if before != after {
			t.Errorf("%v differs after a Project directory move:\n%s\n---\n%s", command, before, after)
		}
		if strings.Contains(before, base) {
			t.Errorf("%v embeds the Project directory:\n%s", command, before)
		}
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func renameAt(t *testing.T, uri string, text string, needle string, occurrence int, newName string, overlay sourceOverlay) (workspaceEdit, error) {
	t.Helper()
	offset := -1
	for index := 0; index <= occurrence; index++ {
		next := strings.Index(text[offset+1:], needle)
		if next < 0 {
			t.Fatalf("needle %q occurrence %d not found", needle, occurrence)
		}
		offset += next + 1
	}
	return renameForSource(uri, text, offsetPosition(text, offset), newName, overlay)
}

// Rename resolves the exact symbol through Sema: every reference bound to the
// declaration is renamed, an equal spelling bound to another declaration is
// untouched, and the renamed program analyzes without new errors.
//
// Rules:
//   - rules/tooling/lsp.md — "Rename"
func TestRenameIsSemantic(t *testing.T) {
	uri := "file:///rename.sec"
	text := "module main\n\nfn Total(count: int) int {\n    let doubled := count * 2\n    return doubled + count\n}\n\nfn Other(count: int) int {\n    return count\n}\n\nfn Use() int {\n    return Total(1) + Total(2)\n}\n"
	edit, err := renameAt(t, uri, text, "count", 1, "amount", sourceOverlay{})
	if err != nil {
		t.Fatalf("rename local parameter: %v", err)
	}
	renamed := applyTextEdits(text, edit.Changes[uri])
	if !strings.Contains(renamed, "fn Total(amount: int) int {\n    let doubled := amount * 2\n    return doubled + amount") ||
		!strings.Contains(renamed, "fn Other(count: int) int {\n    return count") {
		t.Fatalf("renamed parameter:\n%s", renamed)
	}
	if diagnostics := analyze(uri, renamed); len(diagnostics) != 0 {
		t.Fatalf("renamed program reports %+v", diagnostics)
	}

	edit, err = renameAt(t, uri, text, "Total", 0, "Sum", sourceOverlay{})
	if err != nil {
		t.Fatalf("rename function: %v", err)
	}
	renamed = applyTextEdits(text, edit.Changes[uri])
	if strings.Contains(renamed, "Total") || strings.Count(renamed, "Sum(") != 3 {
		t.Fatalf("renamed function:\n%s", renamed)
	}
}

// Prepare-rename rejects keywords, compiler-known members, and positions that
// are not identifiers; rename rejects invalid and reserved names, a changed
// visibility prefix, and an edit that would introduce a collision.
//
// Rules:
//   - rules/tooling/lsp.md — "Rename" (prepare-rename rejections, semantic safety)
func TestRenameRejectsUnsafeRequests(t *testing.T) {
	uri := "file:///rename.sec"
	text := "module main\n\nfn Measure(values: int[]) uint {\n    let first := 1\n    let second := 2\n    discard first\n    discard second\n    return values.Len\n}\n\nfn _Helper() int {\n    return 1\n}\n"
	for _, needle := range []string{"fn Measure", "Len", "return values"} {
		offset := strings.Index(text, needle)
		if _, err := prepareRenameForSource(uri, text, offsetPosition(text, offset), sourceOverlay{}); err == nil {
			t.Fatalf("prepare-rename accepted %q", needle)
		}
	}
	if result, err := prepareRenameForSource(uri, text, offsetPosition(text, strings.Index(text, "first")), sourceOverlay{}); err != nil || result.Placeholder != "first" {
		t.Fatalf("prepare-rename of a local = %+v, %v", result, err)
	}
	for _, test := range []struct {
		needle  string
		newName string
		reason  string
	}{
		{needle: "first", newName: "not valid", reason: "not a valid Sec identifier"},
		{needle: "first", newName: "match", reason: "not a valid Sec identifier"},
		{needle: "first", newName: "task", reason: "reserved"},
		{needle: "_Helper", newName: "Helper", reason: "visibility prefix"},
		{needle: "first", newName: "second", reason: "would introduce an error"},
	} {
		if _, err := renameAt(t, uri, text, test.needle, 0, test.newName, sourceOverlay{}); err == nil || !strings.Contains(err.Error(), test.reason) {
			t.Fatalf("rename %s -> %s error = %v, want %q", test.needle, test.newName, err, test.reason)
		}
	}
}

// A module-level declaration used from another file of the same module is
// renamed in both files in one workspace edit.
func TestRenameSpansModuleFiles(t *testing.T) {
	dir := t.TempDir()
	declarations := filepath.Join(dir, "declarations.sec")
	usage := filepath.Join(dir, "usage.sec")
	if err := os.WriteFile(declarations, []byte("module shapes\n\nfn Area(width: int) int {\n    return width * width\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	usageText := "module shapes\n\nfn Twice(width: int) int {\n    return Area(width) * 2\n}\n"
	if err := os.WriteFile(usage, []byte(usageText), 0o644); err != nil {
		t.Fatal(err)
	}
	edit, err := renameAt(t, uriFromPath(usage), usageText, "Area", 0, "Square", sourceOverlay{})
	if err != nil {
		t.Fatalf("rename across files: %v", err)
	}
	if len(edit.Changes[uriFromPath(usage)]) != 1 || len(edit.Changes[uriFromPath(declarations)]) != 1 {
		t.Fatalf("workspace edit = %+v, want one edit in each module file", edit.Changes)
	}
}

// Rename validation consumes the compiler's own namespace, shadowing,
// overload and capitalization rules by re-analyzing the renamed program:
// shadowing an outer binding, colliding with a same-signature function and
// lowercasing a nominal type are rejected with the Sema diagnostic, while
// joining a different-signature overload set stays valid.
//
// Rules:
//   - rules/tooling/lsp.md — § 15 "Rename"
//   - rules/foundations/names_scopes_visibility.md — no shadowing, one namespace per scope
func TestRenameConsumesSemaNamespaceRules(t *testing.T) {
	uri := "file:///rename_rules.sec"
	for _, test := range []struct {
		text, needle, newName, reason string
	}{
		{"module main\n\nfn F(outer: int) int {\n    if outer > 0 {\n        let inner := 1\n        return inner\n    }\n    return outer\n}\n", "inner", "outer", `variable "outer" already declared`},
		{"module main\n\nfn A(v: int) int {\n    return v\n}\n\nfn B(v: int) int {\n    return v\n}\n\nfn Use() int {\n    return A(1) + B(2)\n}\n", "B(v", "A", `duplicate function "A" with same signature`},
		{"module main\n\ntype Point struct {\n    x: int,\n}\n\nfn Use(p: Point) int {\n    return p.x\n}\n", "Point struct", "point", "must begin with an uppercase letter"},
		{"module main\n\nfn A(v: int) int {\n    return v\n}\n\nfn B(v: string) int {\n    return 1\n}\n\nfn Use() int {\n    return A(1) + B(\"x\")\n}\n", "B(v", "A", ""},
	} {
		_, err := renameAt(t, uri, test.text, test.needle, 0, test.newName, sourceOverlay{})
		if test.reason == "" {
			if err != nil {
				t.Fatalf("rename %s -> %s rejected: %v", test.needle, test.newName, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), test.reason) {
			t.Fatalf("rename %s -> %s error = %v, want %q", test.needle, test.newName, err, test.reason)
		}
	}
}

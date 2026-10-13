package project

import (
	"fmt"
	"strings"
	"testing"
)

// Rules: rules/projects/projects.md — §3.3,§7,§50(1).
func TestProjectUUIDVersionsAndCanonicalSpelling(t *testing.T) {
	for version := 1; version <= 8; version++ {
		value := fmt.Sprintf("550e8400-e29b-%x1d4-a716-446655440000", version)
		uuid, err := ParseProjectUUID(value)
		if err != nil || uuid.String() != value {
			t.Fatal(value, uuid, err)
		}
	}
	if uuid, err := ParseProjectUUID("ffffffff-ffff-ffff-ffff-ffffffffffff"); err != nil || uuid.String() != "ffffffff-ffff-ffff-ffff-ffffffffffff" {
		t.Fatal(uuid, err)
	}
	for _, value := range []string{"", "00000000-0000-0000-0000-000000000000", "550E8400-e29b-41d4-a716-446655440000", "550e8400e29b41d4a716446655440000", "550e8400-e29b-91d4-a716-446655440000", "550e8400-e29b-41d4-0716-446655440000", "550e8400-e29b-41d4-c716-446655440000", "550e8400-e29b-41d4-a716-44665544000z"} {
		if _, err := ParseProjectUUID(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

// Rules: rules/projects/projects.md — §7(1–7),§48(2).
func TestManifestIdentityValidation(t *testing.T) {
	valid := "[project]\nname = ' Human # Name '\nuuid = \"550e8400-e29b-41d4-a716-446655440000\"\nmodule = 'orders'\n[version]\nmajor = 0\n"
	identity, errors := ParseManifestIdentity(valid)
	if len(errors) != 0 || identity.Name != " Human # Name " || identity.Module != "orders" || len(identity.Warnings) != 1 || identity.Warnings[0].Line != 2 {
		t.Fatal(identity, errors)
	}
	cases := []struct{ text, want string }{
		{"", "[project]"},
		{`[project]
name = "bad\x41"
uuid = "550e8400-e29b-41d4-a716-446655440000"`, "invalid TOML string escape"},
		{"[project]\nname = 'a'b'\n", "invalid TOML literal string"},
		{"[project]\n", "required field name"},
		{strings.Replace(valid, "name = ' Human # Name '", "name = ''", 1), "non-empty"},
		{strings.Replace(valid, "name = ' Human # Name '", "name = 42", 1), "expected a string"},
		{strings.Replace(valid, "550e8400-e29b-41d4-a716-446655440000", "00000000-0000-0000-0000-000000000000", 1), "non-nil"},
		{valid + "[project]\n", "second one"},
		{strings.Replace(valid, "module = 'orders'", "name = 'other'", 1), "duplicate project field name"},
		{strings.Replace(valid, "module = 'orders'", "module = 'orders.items'", 1), "ModuleName"},
		{strings.Replace(valid, "name = ' Human # Name '", "name = '''fake\n[project]\n'''", 1), "multiline TOML"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			_, errors := ParseManifestIdentity(tc.text)
			found := false
			for _, err := range errors {
				found = found || strings.Contains(err.Message, tc.want)
			}
			if !found {
				t.Fatal(tc.text, errors)
			}
		})
	}
}

// Rules: rules/projects/modules.md — §2.1,§4; rules/foundations/grammar.md — ModuleDeclaration.
func TestValidModuleName(t *testing.T) {
	for _, name := range []string{"orders", "storage", "räksmörgås", "type"} {
		if !ValidModuleName(name) {
			t.Fatal(name)
		}
	}
	for _, name := range []string{"", "orders.items", "orders/items", " orders", "a\nmodule b", "ra\u0308ks", "x {}", "1abc"} {
		if ValidModuleName(name) {
			t.Fatal(name)
		}
	}
}

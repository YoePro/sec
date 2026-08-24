package modules

import "testing"

func TestValidateImportPath(t *testing.T) {
	valid := []string{"orders", "domain/orders", "std/io", "platform/linux/amd64", "Mixed/Case"}
	for _, path := range valid {
		if err := ValidateImportPath(path); err != nil {
			t.Errorf("ValidateImportPath(%q): %v", path, err)
		}
	}
	invalid := []string{"", "/orders", "orders/", "domain//orders", ".", "./orders", "../orders", "domain/../orders", `domain\\orders`, "C:/orders"}
	for _, path := range invalid {
		if err := ValidateImportPath(path); err == nil {
			t.Errorf("ValidateImportPath(%q) unexpectedly succeeded", path)
		}
	}
}

func TestModuleIdentityIncludesImportRoot(t *testing.T) {
	first, err := NewModuleIdentity("project:first", "order")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewModuleIdentity("project:second", "order")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || first.String() == second.String() {
		t.Fatalf("different roots produced equal identities: %v and %v", first, second)
	}
}

func TestCrossModuleDeclarationIdentityUsesCanonicalModuleOwner(t *testing.T) {
	firstModule, err := NewModuleIdentity("project:first", "order")
	if err != nil {
		t.Fatal(err)
	}
	secondModule, err := NewModuleIdentity("project:second", "order")
	if err != nil {
		t.Fatal(err)
	}
	first, err := NewCrossModuleDeclarationIdentity(firstModule, "fn:Create")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewCrossModuleDeclarationIdentity(secondModule, "fn:Create")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || first.StableKey() == second.StableKey() {
		t.Fatalf("different canonical module owners produced the same declaration identity: %v and %v", first, second)
	}
	if got, want := first.String(), "project:first:order::fn:Create"; got != want {
		t.Fatalf("identity display = %q, want %q", got, want)
	}
	if got, want := first.StableKey(), "13:project:first5:order9:fn:Create"; got != want {
		t.Fatalf("stable key = %q, want %q", got, want)
	}
}

func TestCrossModuleDeclarationIdentityRejectsIncompleteIdentity(t *testing.T) {
	validModule, err := NewModuleIdentity("project:test", "order")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name        string
		module      ModuleIdentity
		declaration string
	}{
		{name: "missing root", module: ModuleIdentity{CanonicalImportPath: "order"}, declaration: "fn:Create"},
		{name: "invalid path", module: ModuleIdentity{ImportRootIdentity: "project:test", CanonicalImportPath: "../order"}, declaration: "fn:Create"},
		{name: "missing declaration", module: validModule},
		{name: "NUL declaration", module: validModule, declaration: "fn:\x00Create"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewCrossModuleDeclarationIdentity(test.module, test.declaration); err == nil {
				t.Fatal("invalid cross-module declaration identity unexpectedly succeeded")
			}
		})
	}
}
